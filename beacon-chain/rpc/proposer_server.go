package rpc

import (
	"context"
	"errors"
	"fmt"
	"math/big"

	"github.com/prysmaticlabs/prysm/beacon-chain/core/blocks"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/state"
	"github.com/prysmaticlabs/prysm/beacon-chain/db"
	pbp2p "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/rpc/v1"
	"github.com/prysmaticlabs/prysm/shared/blockutil"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/prysmaticlabs/prysm/shared/featureconfig"
	"github.com/prysmaticlabs/prysm/shared/hashutil"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/trieutil"
	"github.com/sirupsen/logrus"
)

// ProposerServer defines a server implementation of the gRPC Proposer service,
// providing RPC endpoints for computing state transitions and state roots, proposing
// beacon blocks to a beacon node, and more.
type ProposerServer struct {
	beaconDB           *db.BeaconDB
	chainService       chainService
	powChainService    powChainService
	operationService   operationService
	canonicalStateChan chan *pbp2p.BeaconState
}

// RequestBlock is called by a proposer during its assigned slot to request a block to sign
// by passing in the slot and the signed randao reveal of the slot.
func (ps *ProposerServer) RequestBlock(ctx context.Context, req *pb.BlockRequest) (*pbp2p.BeaconBlock, error) {

	// Retrieve the parent block as the current head of the canonical chain
	parent, err := ps.beaconDB.ChainHead()
	if err != nil {
		return nil, fmt.Errorf("could not get canonical head block: %v", err)
	}

	parentRoot, err := blockutil.BlockSigningRoot(parent)
	if err != nil {
		return nil, fmt.Errorf("could not get parent block signing root: %v", err)
	}

	// Construct block body
	// Pack ETH1 deposits which have not been included in the beacon chain
	eth1Data, err := ps.eth1Data(ctx)
	if err != nil {
		return nil, fmt.Errorf("could not get ETH1 data: %v", err)
	}

	// Pack ETH1 deposits which have not been included in the beacon chain.
	deposits, err := ps.deposits(ctx)
	if err != nil {
		return nil, fmt.Errorf("could not get eth1 deposits: %v", err)
	}

	// Pack aggregated attestations which have not been included in the beacon chain.
	attestations, err := ps.attestations(ctx)
	if err != nil {
		return nil, fmt.Errorf("could not get pending attestations: %v", err)
	}

	// Use zero hash as stub for state root to compute later.
	stateRoot := params.BeaconConfig().ZeroHash[:]

	blk := &pbp2p.BeaconBlock{
		Slot:       req.Slot,
		ParentRoot: parentRoot[:],
		StateRoot:  stateRoot,
		Body: &pbp2p.BeaconBlockBody{
			Eth1Data:     eth1Data,
			Deposits:     deposits,
			Attestations: attestations,
			// TODO(2766): Implement rest of the retrievals for beacon block operations
			ProposerSlashings: nil,
			AttesterSlashings: nil,
			VoluntaryExits:    nil,
		},
	}

	if !featureconfig.FeatureConfig().EnableComputeStateRoot {
		// Compute state root with the newly constructed block.
		stateRoot, err = ps.computeStateRoot(ctx, blk)
		if err != nil {
			return nil, fmt.Errorf("could not get compute state root: %v", err)
		}
		blk.StateRoot = stateRoot
	}

	return blk, nil
}

// ProposeBlock is called by a proposer during its assigned slot to create a block in an attempt
// to get it processed by the beacon node as the canonical head.
func (ps *ProposerServer) ProposeBlock(ctx context.Context, blk *pbp2p.BeaconBlock) (*pb.ProposeResponse, error) {
	root, err := blockutil.BlockSigningRoot(blk)
	if err != nil {
		return nil, fmt.Errorf("could not tree hash block: %v", err)
	}
	log.WithField("blockRoot", fmt.Sprintf("%#x", bytesutil.Trunc(root[:]))).Debugf(
		"Block proposal received via RPC")

	beaconState, err := ps.chainService.ReceiveBlock(ctx, blk)
	if err != nil {
		return nil, fmt.Errorf("could not process beacon block: %v", err)
	}

	if err := ps.beaconDB.UpdateChainHead(ctx, blk, beaconState); err != nil {
		return nil, fmt.Errorf("failed to update chain: %v", err)

	}
	ps.chainService.UpdateCanonicalRoots(blk, root)
	log.WithFields(logrus.Fields{
		"headRoot": fmt.Sprintf("%#x", bytesutil.Trunc(root[:])),
		"headSlot": blk.Slot,
	}).Info("Chain head block and state updated")

	return &pb.ProposeResponse{BlockRoot: root[:]}, nil
}

// attestations retrieves aggregated attestations kept in the beacon node's operations pool which have
// not yet been included into the beacon chain. Proposers include these pending attestations in their
// proposed blocks when performing their responsibility. If desired, callers can choose to filter pending
// attestations which are ready for inclusion. That is, attestations that satisfy:
// attestation.slot + MIN_ATTESTATION_INCLUSION_DELAY <= state.slot.
func (ps *ProposerServer) attestations(ctx context.Context) ([]*pbp2p.Attestation, error) {
	beaconState, err := ps.beaconDB.HeadState(ctx)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve beacon state: %v", err)
	}
	atts, err := ps.operationService.PendingAttestations(ctx)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve pending attest ations from operations service: %v", err)
	}
	beaconState.Slot++

	var attsReadyForInclusion []*pbp2p.Attestation
	for _, att := range atts {
		slot, err := helpers.AttestationDataSlot(beaconState, att.Data)
		if err != nil {
			return nil, fmt.Errorf("could not get attestation slot: %v", err)
		}
		if slot+params.BeaconConfig().MinAttestationInclusionDelay <= beaconState.Slot {
			attsReadyForInclusion = append(attsReadyForInclusion, att)
		}
	}

	validAtts := make([]*pbp2p.Attestation, 0, len(attsReadyForInclusion))
	for _, att := range attsReadyForInclusion {
		slot, err := helpers.AttestationDataSlot(beaconState, att.Data)
		if err != nil {
			return nil, fmt.Errorf("could not get attestation slot: %v", err)
		}

		if _, err := blocks.ProcessAttestation(beaconState, att, false); err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}

			log.WithError(err).WithFields(logrus.Fields{
				"slot":     slot,
				"headRoot": fmt.Sprintf("%#x", bytesutil.Trunc(att.Data.BeaconBlockRoot))}).Info(
				"Deleting failed pending attestation from DB")
			if err := ps.beaconDB.DeleteAttestation(att); err != nil {
				return nil, fmt.Errorf("could not delete failed attestation: %v", err)
			}
			continue
		}
		canonical, err := ps.operationService.IsAttCanonical(ctx, att)
		if err != nil {
			// Delete attestation that failed to verify as canonical.
			if err := ps.beaconDB.DeleteAttestation(att); err != nil {
				return nil, fmt.Errorf("could not delete failed attestation: %v", err)
			}
			return nil, fmt.Errorf("could not verify canonical attestation: %v", err)
		}
		// Skip the attestation if it's not canonical.
		if !canonical {
			continue
		}

		validAtts = append(validAtts, att)
	}

	return validAtts, nil
}

// Eth1Data is a mechanism used by block proposers vote on a recent Ethereum 1.0 block hash and an
// associated deposit root found in the Ethereum 1.0 deposit contract. When consensus is formed,
// state.latest_eth1_data is updated, and validator deposits up to this root can be processed.
// The deposit root can be calculated by calling the get_deposit_root() function of
// the deposit contract using the post-state of the block hash.
//
// TODO(#2307): Refactor for v0.6.
func (ps *ProposerServer) eth1Data(ctx context.Context) (*pbp2p.Eth1Data, error) {
	return nil, nil
}

// computeStateRoot computes the state root after a block has been processed through a state transition and
// returns it to the validator client.
func (ps *ProposerServer) computeStateRoot(ctx context.Context, block *pbp2p.BeaconBlock) ([]byte, error) {

	beaconState, err := ps.beaconDB.HeadState(ctx)
	if err != nil {
		return nil, fmt.Errorf("could not get beacon state: %v", err)
	}

	s, err := state.ExecuteStateTransition(
		ctx,
		beaconState,
		block,
		state.DefaultConfig(),
	)
	if err != nil {
		return nil, fmt.Errorf("could not execute state transition for state root %v", err)
	}

	root, err := hashutil.HashProto(s)
	if err != nil {
		return nil, fmt.Errorf("could not tree hash beacon state: %v", err)
	}

	log.WithField("beaconStateRoot", fmt.Sprintf("%#x", root)).Debugf("Computed state hash")

	return root[:], nil
}

// deposits returns a list of pending deposits that are ready for
// inclusion in the next beacon block.
func (ps *ProposerServer) deposits(ctx context.Context) ([]*pbp2p.Deposit, error) {
	bNum := ps.powChainService.LatestBlockHeight()
	if bNum == nil {
		return nil, errors.New("latest PoW block number is unknown")
	}
	// Only request deposits that have passed the ETH1 follow distance window.
	bNum = bNum.Sub(bNum, big.NewInt(int64(params.BeaconConfig().Eth1FollowDistance)))
	allDeps := ps.beaconDB.AllDeposits(ctx, bNum)
	if len(allDeps) == 0 {
		return nil, nil
	}

	// Need to fetch if the deposits up to the state's latest eth 1 data matches
	// the number of all deposits in this RPC call. If not, then we return nil.
	beaconState, err := ps.beaconDB.HeadState(ctx)
	if err != nil {
		return nil, fmt.Errorf("could not fetch beacon state: %v", err)
	}
	h := bytesutil.ToBytes32(beaconState.LatestEth1Data.BlockHash)
	_, latestEth1DataHeight, err := ps.powChainService.BlockExists(ctx, h)
	if err != nil {
		return nil, fmt.Errorf("could not fetch eth1data height: %v", err)
	}
	// If the state's latest eth1 data's block hash has a height of 100, we fetch all the deposits up to height 100.
	// If this doesn't match the number of deposits stored in the cache, the generated trie will not be the same and
	// root will fail to verify. This can happen in a scenario where we perhaps have a deposit from height 101,
	// so we want to avoid any possible mismatches in these lengths.
	upToLatestEth1DataDeposits := ps.beaconDB.AllDeposits(ctx, latestEth1DataHeight)
	if len(upToLatestEth1DataDeposits) != len(allDeps) {
		return nil, nil
	}
	depositData := [][]byte{}
	for _, dep := range upToLatestEth1DataDeposits {
		depHash, err := hashutil.DepositHash(dep.Data)
		if err != nil {
			return nil, fmt.Errorf("coulf not hash deposit data %v", err)
		}
		depositData = append(depositData, depHash[:])
	}

	depositTrie, err := trieutil.GenerateTrieFromItems(depositData, int(params.BeaconConfig().DepositContractTreeDepth))
	if err != nil {
		return nil, fmt.Errorf("could not generate historical deposit trie from deposits: %v", err)
	}

	allPendingContainers := ps.beaconDB.PendingContainers(ctx, bNum)

	// Deposits need to be received in order of merkle index root, so this has to make sure
	// deposits are sorted from lowest to highest.
	var pendingDeps []*db.DepositContainer
	for _, dep := range allPendingContainers {
		if uint64(dep.Index) >= beaconState.DepositIndex {
			pendingDeps = append(pendingDeps, dep)
		}
	}

	for i := range pendingDeps {
		// Don't construct merkle proof if the number of deposits is more than max allowed in block.
		if uint64(i) == params.BeaconConfig().MaxDeposits {
			break
		}
		pendingDeps[i].Deposit, err = constructMerkleProof(depositTrie, pendingDeps[i].Index, pendingDeps[i].Deposit)
		if err != nil {
			return nil, err
		}
	}
	// Limit the return of pending deposits to not be more than max deposits allowed in block.
	var pendingDeposits []*pbp2p.Deposit
	for i := 0; i < len(pendingDeps) && i < int(params.BeaconConfig().MaxDeposits); i++ {
		pendingDeposits = append(pendingDeposits, pendingDeps[i].Deposit)
	}
	return pendingDeposits, nil
}
