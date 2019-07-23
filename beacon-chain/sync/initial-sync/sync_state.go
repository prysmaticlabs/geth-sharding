package initialsync

import (
	"context"

	peer "github.com/libp2p/go-libp2p-peer"
	"github.com/prysmaticlabs/go-ssz"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/blocks"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/validators"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/prysmaticlabs/prysm/shared/p2p"
	"go.opencensus.io/trace"
)

func (s *InitialSync) processState(msg p2p.Message, chainHead *pb.ChainHeadResponse) error {
	ctx, span := trace.StartSpan(msg.Ctx, "beacon-chain.sync.initial-sync.processState")
	defer span.End()
	data := msg.Data.(*pb.BeaconStateResponse)
	finalizedState := data.FinalizedState
	recState.Inc()

	// save a block with an empty body.
	blockWithNoBody := blocks.BlockFromHeader(finalizedState.LatestBlockHeader)
	if err := s.db.SaveFinalizedState(finalizedState); err != nil {
		log.Errorf("Unable to set received last finalized state in db: %v", err)
		return nil
	}

	if err := s.db.SaveFinalizedBlock(blockWithNoBody); err != nil {
		log.Errorf("Could not save finalized block %v", err)
		return nil
	}

	if err := s.db.SaveBlock(blockWithNoBody); err != nil {
		log.Errorf("Could not save block %v", err)
		return nil
	}

	finalizedBlockRoot, err := ssz.HashTreeRoot(finalizedState.LatestBlockHeader)
	if err != nil {
		log.Errorf("Could not hash finalized block %v", err)
		return nil
	}

	if err := s.db.SaveHistoricalState(ctx, finalizedState, finalizedBlockRoot); err != nil {
		log.Errorf("Could not save new historical state: %v", err)
		return nil
	}

	if err := s.db.SaveAttestationTarget(ctx, &pb.AttestationTarget{
		Slot:            finalizedState.LatestBlockHeader.Slot,
		BeaconBlockRoot: finalizedBlockRoot[:],
		ParentRoot:      finalizedState.LatestBlockHeader.ParentRoot,
	}); err != nil {
		log.Errorf("Could not to save attestation target: %v", err)
		return nil
	}

	if err := s.db.SaveJustifiedState(finalizedState); err != nil {
		log.Errorf("Could not set beacon state for initial sync %v", err)
		return nil
	}

	if err := s.db.SaveJustifiedBlock(blockWithNoBody); err != nil {
		log.Errorf("Could not save finalized block %v", err)
		return nil
	}

	exists, _, err := s.powchain.BlockExists(ctx, bytesutil.ToBytes32(finalizedState.Eth1Data.BlockHash))
	if err != nil {
		log.Errorf("Unable to get powchain block %v", err)
	}

	if !exists {
		log.Error("Latest ETH1 block doesn't exist in the pow chain")
		return nil
	}

	s.db.PrunePendingDeposits(ctx, int(finalizedState.Eth1DepositIndex))

	if err := s.db.UpdateChainHead(ctx, blockWithNoBody, finalizedState); err != nil {
		log.Errorf("Could not update chain head: %v", err)
		return nil
	}

	validators.InitializeValidatorStore(finalizedState)

	s.stateReceived = true
	log.Debugf(
		"Successfully saved beacon state with the last finalized slot: %d",
		finalizedState.Slot,
	)
	log.WithField("peer", msg.Peer.Pretty()).Info("Requesting batch blocks from peer")
	s.requestBatchedBlocks(ctx, finalizedBlockRoot[:], chainHead.CanonicalBlockRoot, msg.Peer)

	return nil
}

// requestStateFromPeer requests for the canonical state, finalized state, and justified state from a peer.
func (s *InitialSync) requestStateFromPeer(ctx context.Context, lastFinalizedRoot [32]byte, peer peer.ID) error {
	ctx, span := trace.StartSpan(ctx, "beacon-chain.sync.initial-sync.requestStateFromPeer")
	defer span.End()
	stateReq.Inc()
	return s.p2p.Send(ctx, &pb.BeaconStateRequest{
		FinalizedStateRootHash32S: lastFinalizedRoot[:],
	}, peer)
}
