package validator

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"reflect"
	"time"

	fastssz "github.com/ferranbt/fastssz"
	"github.com/pkg/errors"
	types "github.com/prysmaticlabs/eth2-types"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/blocks"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/feed"
	blockfeed "github.com/prysmaticlabs/prysm/beacon-chain/core/feed/block"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/state"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/state/interop"
	iface "github.com/prysmaticlabs/prysm/beacon-chain/state/interface"
	dbpb "github.com/prysmaticlabs/prysm/proto/beacon/db"
	ethpb "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/proto/eth/v1alpha1/wrapper"
	"github.com/prysmaticlabs/prysm/proto/interfaces"
	attaggregation "github.com/prysmaticlabs/prysm/shared/aggregation/attestations"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/prysmaticlabs/prysm/shared/featureconfig"
	"github.com/prysmaticlabs/prysm/shared/hashutil"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/rand"
	"github.com/prysmaticlabs/prysm/shared/trieutil"
	"github.com/sirupsen/logrus"
	"go.opencensus.io/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// eth1DataNotification is a latch to stop flooding logs with the same warning.
var eth1DataNotification bool

const eth1dataTimeout = 2 * time.Second

// skipcq: SCC-U1000
type eth1DataSingleVote struct {
	eth1Data    *ethpb.Eth1Data
	blockHeight *big.Int
}

// skipcq: SCC-U1000
type eth1DataAggregatedVote struct {
	data  eth1DataSingleVote
	votes int
}

// BlockData required to create a beacon block.
type BlockData struct {
	ParentRoot        []byte
	Graffiti          [32]byte
	ProposerIdx       types.ValidatorIndex
	Eth1Data          *ethpb.Eth1Data
	Deposits          []*ethpb.Deposit
	Attestations      []*ethpb.Attestation
	ProposerSlashings []*ethpb.ProposerSlashing
	AttesterSlashings []*ethpb.AttesterSlashing
	VoluntaryExits    []*ethpb.SignedVoluntaryExit
}

// GetBlock is called by a proposer during its assigned slot to request a block to sign
// by passing in the slot and the signed randao reveal of the slot.
func (vs *Server) GetBlock(ctx context.Context, req *ethpb.BlockRequest) (*ethpb.BeaconBlock, error) {
	ctx, span := trace.StartSpan(ctx, "ProposerServer.GetBlock")
	defer span.End()
	span.AddAttributes(trace.Int64Attribute("slot", int64(req.Slot)))

	blkData, err := vs.BuildBlockData(ctx, req)
	if err != nil {
		return nil, err
	}

	// Use zero hash as stub for state root to compute later.
	stateRoot := params.BeaconConfig().ZeroHash[:]

	blk := &ethpb.BeaconBlock{
		Slot:          req.Slot,
		ParentRoot:    blkData.ParentRoot,
		StateRoot:     stateRoot,
		ProposerIndex: blkData.ProposerIdx,
		Body: &ethpb.BeaconBlockBody{
			Eth1Data:          blkData.Eth1Data,
			Deposits:          blkData.Deposits,
			Attestations:      blkData.Attestations,
			RandaoReveal:      req.RandaoReveal,
			ProposerSlashings: blkData.ProposerSlashings,
			AttesterSlashings: blkData.AttesterSlashings,
			VoluntaryExits:    blkData.VoluntaryExits,
			Graffiti:          blkData.Graffiti[:],
		},
	}

	// Compute state root with the newly constructed block.
	stateRoot, err = vs.ComputeStateRoot(
		ctx, wrapper.WrappedPhase0SignedBeaconBlock(
			&ethpb.SignedBeaconBlock{Block: blk, Signature: make([]byte, 96)},
		),
	)
	if err != nil {
		interop.WriteBlockToDisk(wrapper.WrappedPhase0SignedBeaconBlock(&ethpb.SignedBeaconBlock{Block: blk}), true /*failed*/)
		return nil, status.Errorf(codes.Internal, "Could not compute state root: %v", err)
	}
	blk.StateRoot = stateRoot

	return blk, nil
}

// BuildBlockData for creating a new beacon block, so that this method can be shared across forks.
func (vs *Server) BuildBlockData(ctx context.Context, req *ethpb.BlockRequest) (*BlockData, error) {
	ctx, span := trace.StartSpan(ctx, "ProposerServer.BuildBlockData")
	defer span.End()

	if vs.SyncChecker.Syncing() {
		return nil, status.Errorf(codes.Unavailable, "Syncing to latest head, not ready to respond")
	}

	// Retrieve the parent block as the current head of the canonical chain.
	parentRoot, err := vs.HeadFetcher.HeadRoot(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Could not retrieve head root: %v", err)
	}

	head, err := vs.HeadFetcher.HeadState(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Could not get head state %v", err)
	}

	if featureconfig.Get().EnableNextSlotStateCache {
		head, err = state.ProcessSlotsUsingNextSlotCache(ctx, head, parentRoot, req.Slot)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "Could not advance slots to calculate proposer index: %v", err)
		}
	} else {
		head, err = state.ProcessSlots(ctx, head, req.Slot)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "Could not advance slot to calculate proposer index: %v", err)
		}
	}

	eth1Data, err := vs.eth1DataMajorityVote(ctx, head)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Could not get ETH1 data: %v", err)
	}

	// Pack ETH1 Deposits which have not been included in the beacon chain.
	deposits, err := vs.deposits(ctx, head, eth1Data)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Could not get ETH1 Deposits: %v", err)
	}

	// Pack aggregated Attestations which have not been included in the beacon chain.
	atts, err := vs.packAttestations(ctx, head)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Could not get Attestations to pack into block: %v", err)
	}

	graffiti := bytesutil.ToBytes32(req.Graffiti)

	// Calculate new proposer index.
	idx, err := helpers.BeaconProposerIndex(head)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Could not calculate proposer index %v", err)
	}

	return &BlockData{
		ParentRoot:        parentRoot,
		Graffiti:          graffiti,
		ProposerIdx:       idx,
		Eth1Data:          eth1Data,
		Deposits:          deposits,
		Attestations:      atts,
		ProposerSlashings: vs.SlashingsPool.PendingProposerSlashings(ctx, head, false /*noLimit*/),
		AttesterSlashings: vs.SlashingsPool.PendingAttesterSlashings(ctx, head, false /*noLimit*/),
		VoluntaryExits:    vs.ExitPool.PendingExits(head, req.Slot, false /*noLimit*/),
	}, nil
}

// ProposeBlock is called by a proposer during its assigned slot to create a block in an attempt
// to get it processed by the beacon node as the canonical head.
func (vs *Server) ProposeBlock(ctx context.Context, rBlk *ethpb.SignedBeaconBlock) (*ethpb.ProposeResponse, error) {
	blk := wrapper.WrappedPhase0SignedBeaconBlock(rBlk)
	return vs.ProposeBlockGeneric(ctx, blk)
}

// ProposeBlockGeneric performs the core post-block creation actions once a block proposal is received.
func (vs *Server) ProposeBlockGeneric(ctx context.Context, blk interfaces.SignedBeaconBlock) (*ethpb.ProposeResponse, error) {
	ctx, span := trace.StartSpan(ctx, "ProposerServer.proposeBlock")
	defer span.End()
	root, err := blk.Block().HashTreeRoot()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Could not tree hash block: %v", err)
	}

	// Do not block proposal critical path with debug logging or block feed updates.
	defer func() {
		log.WithField("blockRoot", fmt.Sprintf("%#x", bytesutil.Trunc(root[:]))).Debugf(
			"Block proposal received via RPC")
		vs.BlockNotifier.BlockFeed().Send(&feed.Event{
			Type: blockfeed.ReceivedBlock,
			Data: &blockfeed.ReceivedBlockData{SignedBlock: blk},
		})
	}()

	// Broadcast the new block to the network.
	if err := vs.P2P.Broadcast(ctx, blk.Proto()); err != nil {
		return nil, status.Errorf(codes.Internal, "Could not broadcast block: %v", err)
	}
	log.WithFields(logrus.Fields{
		"blockRoot": hex.EncodeToString(root[:]),
	}).Debug("Broadcasting block")

	if err := vs.BlockReceiver.ReceiveBlock(ctx, blk, root); err != nil {
		return nil, status.Errorf(codes.Internal, "Could not process beacon block: %v", err)
	}

	return &ethpb.ProposeResponse{
		BlockRoot: root[:],
	}, nil
}

// eth1DataMajorityVote determines the appropriate eth1data for a block proposal using
// an algorithm called Voting with the Majority. The algorithm works as follows:
//  - Determine the timestamp for the start slot for the eth1 voting period.
//  - Determine the earliest and latest timestamps that a valid block can have.
//  - Determine the first block not before the earliest timestamp. This block is the lower bound.
//  - Determine the last block not after the latest timestamp. This block is the upper bound.
//  - If the last block is too early, use current eth1data from the beacon state.
//  - Filter out votes on unknown blocks and blocks which are outside of the range determined by the lower and upper bounds.
//  - If no blocks are left after filtering votes, use eth1data from the latest valid block.
//  - Otherwise:
//    - Determine the vote with the highest count. Prefer the vote with the highest eth1 block height in the event of a tie.
//    - This vote's block is the eth1 block to use for the block proposal.
func (vs *Server) eth1DataMajorityVote(ctx context.Context, beaconState iface.BeaconState) (*ethpb.Eth1Data, error) {
	ctx, cancel := context.WithTimeout(ctx, eth1dataTimeout)
	defer cancel()

	slot := beaconState.Slot()
	votingPeriodStartTime := vs.slotStartTime(slot)

	if vs.MockEth1Votes {
		return vs.mockETH1DataVote(ctx, slot)
	}
	if !vs.Eth1InfoFetcher.IsConnectedToETH1() {
		return vs.randomETH1DataVote(ctx)
	}
	eth1DataNotification = false

	eth1FollowDistance := params.BeaconConfig().Eth1FollowDistance
	earliestValidTime := votingPeriodStartTime - 2*params.BeaconConfig().SecondsPerETH1Block*eth1FollowDistance
	latestValidTime := votingPeriodStartTime - params.BeaconConfig().SecondsPerETH1Block*eth1FollowDistance

	lastBlockByEarliestValidTime, err := vs.Eth1BlockFetcher.BlockByTimestamp(ctx, earliestValidTime)
	if err != nil {
		log.WithError(err).Error("Could not get last block by earliest valid time")
		return vs.randomETH1DataVote(ctx)
	}
	// Increment the earliest block if the original block's time is before valid time.
	// This is very likely to happen because BlockTimeByHeight returns the last block AT OR BEFORE the specified time.
	if lastBlockByEarliestValidTime.Time < earliestValidTime {
		lastBlockByEarliestValidTime.Number = big.NewInt(0).Add(lastBlockByEarliestValidTime.Number, big.NewInt(1))
	}

	lastBlockByLatestValidTime, err := vs.Eth1BlockFetcher.BlockByTimestamp(ctx, latestValidTime)
	if err != nil {
		log.WithError(err).Error("Could not get last block by latest valid time")
		return vs.randomETH1DataVote(ctx)
	}
	if lastBlockByLatestValidTime.Time < earliestValidTime {
		return vs.HeadFetcher.HeadETH1Data(), nil
	}

	lastBlockDepositCount, lastBlockDepositRoot := vs.DepositFetcher.DepositsNumberAndRootAtHeight(ctx, lastBlockByLatestValidTime.Number)
	if lastBlockDepositCount == 0 {
		return vs.ChainStartFetcher.ChainStartEth1Data(), nil
	}

	if lastBlockDepositCount >= vs.HeadFetcher.HeadETH1Data().DepositCount {
		hash, err := vs.Eth1BlockFetcher.BlockHashByHeight(ctx, lastBlockByLatestValidTime.Number)
		if err != nil {
			log.WithError(err).Error("Could not get hash of last block by latest valid time")
			return vs.randomETH1DataVote(ctx)
		}
		return &ethpb.Eth1Data{
			BlockHash:    hash.Bytes(),
			DepositCount: lastBlockDepositCount,
			DepositRoot:  lastBlockDepositRoot[:],
		}, nil
	}
	return vs.HeadFetcher.HeadETH1Data(), nil
}

func (vs *Server) slotStartTime(slot types.Slot) uint64 {
	startTime, _ := vs.Eth1InfoFetcher.Eth2GenesisPowchainInfo()
	return helpers.VotingPeriodStartTime(startTime, slot)
}

// skipcq: SCC-U1000
func (vs *Server) inRangeVotes(ctx context.Context,
	beaconState iface.ReadOnlyBeaconState,
	firstValidBlockNumber, lastValidBlockNumber *big.Int) ([]eth1DataSingleVote, error) {

	currentETH1Data := vs.HeadFetcher.HeadETH1Data()

	var inRangeVotes []eth1DataSingleVote
	for _, eth1Data := range beaconState.Eth1DataVotes() {
		exists, height, err := vs.BlockFetcher.BlockExistsWithCache(ctx, bytesutil.ToBytes32(eth1Data.BlockHash))
		if err != nil {
			log.Warningf("Could not fetch eth1data height for received eth1data vote: %v", err)
		}
		// Make sure we don't "undo deposit progress". See https://github.com/ethereum/eth2.0-specs/pull/1836
		if eth1Data.DepositCount < currentETH1Data.DepositCount {
			continue
		}
		// firstValidBlockNumber.Cmp(height) < 1 filters out all blocks before firstValidBlockNumber
		// lastValidBlockNumber.Cmp(height) > -1 filters out all blocks after lastValidBlockNumber
		// These filters result in the range [firstValidBlockNumber, lastValidBlockNumber]
		if exists && firstValidBlockNumber.Cmp(height) < 1 && lastValidBlockNumber.Cmp(height) > -1 {
			inRangeVotes = append(inRangeVotes, eth1DataSingleVote{eth1Data: eth1Data, blockHeight: height})
		}
	}

	return inRangeVotes, nil
}

// skipcq: SCC-U1000
func chosenEth1DataMajorityVote(votes []eth1DataSingleVote) eth1DataAggregatedVote {
	var voteCount []eth1DataAggregatedVote
	for _, singleVote := range votes {
		newVote := true
		for i, aggregatedVote := range voteCount {
			aggregatedData := aggregatedVote.data
			if reflect.DeepEqual(singleVote.eth1Data, aggregatedData.eth1Data) {
				voteCount[i].votes++
				newVote = false
				break
			}
		}

		if newVote {
			voteCount = append(voteCount, eth1DataAggregatedVote{data: singleVote, votes: 1})
		}
	}
	if len(voteCount) == 0 {
		return eth1DataAggregatedVote{}
	}
	currentVote := voteCount[0]
	for _, aggregatedVote := range voteCount[1:] {
		// Choose new eth1data if it has more votes or the same number of votes with a bigger block height.
		if aggregatedVote.votes > currentVote.votes ||
			(aggregatedVote.votes == currentVote.votes &&
				aggregatedVote.data.blockHeight.Cmp(currentVote.data.blockHeight) == 1) {
			currentVote = aggregatedVote
		}
	}

	return currentVote
}

func (vs *Server) mockETH1DataVote(ctx context.Context, slot types.Slot) (*ethpb.Eth1Data, error) {
	if !eth1DataNotification {
		log.Warn("Beacon Node is no longer connected to an ETH1 chain, so ETH1 data votes are now mocked.")
		eth1DataNotification = true
	}
	// If a mock eth1 data votes is specified, we use the following for the
	// eth1data we provide to every proposer based on https://github.com/ethereum/eth2.0-pm/issues/62:
	//
	// slot_in_voting_period = current_slot % SLOTS_PER_ETH1_VOTING_PERIOD
	// Eth1Data(
	//   DepositRoot = hash(current_epoch + slot_in_voting_period),
	//   DepositCount = state.eth1_deposit_index,
	//   BlockHash = hash(hash(current_epoch + slot_in_voting_period)),
	// )
	slotInVotingPeriod := slot.ModSlot(params.BeaconConfig().SlotsPerEpoch.Mul(uint64(params.BeaconConfig().EpochsPerEth1VotingPeriod)))
	headState, err := vs.HeadFetcher.HeadState(ctx)
	if err != nil {
		return nil, err
	}
	var enc []byte
	enc = fastssz.MarshalUint64(enc, uint64(helpers.SlotToEpoch(slot))+uint64(slotInVotingPeriod))
	depRoot := hashutil.Hash(enc)
	blockHash := hashutil.Hash(depRoot[:])
	return &ethpb.Eth1Data{
		DepositRoot:  depRoot[:],
		DepositCount: headState.Eth1DepositIndex(),
		BlockHash:    blockHash[:],
	}, nil
}

func (vs *Server) randomETH1DataVote(ctx context.Context) (*ethpb.Eth1Data, error) {
	if !eth1DataNotification {
		log.Warn("Beacon Node is no longer connected to an ETH1 chain, so ETH1 data votes are now random.")
		eth1DataNotification = true
	}
	headState, err := vs.HeadFetcher.HeadState(ctx)
	if err != nil {
		return nil, err
	}

	// set random roots and block hashes to prevent a majority from being
	// built if the eth1 node is offline
	randGen := rand.NewGenerator()
	depRoot := hashutil.Hash(bytesutil.Bytes32(randGen.Uint64()))
	blockHash := hashutil.Hash(bytesutil.Bytes32(randGen.Uint64()))
	return &ethpb.Eth1Data{
		DepositRoot:  depRoot[:],
		DepositCount: headState.Eth1DepositIndex(),
		BlockHash:    blockHash[:],
	}, nil
}

// ComputeStateRoot computes the state root after a block has been processed through a state transition and
// returns it to the validator client.
func (vs *Server) ComputeStateRoot(ctx context.Context, block interfaces.SignedBeaconBlock) ([]byte, error) {
	beaconState, err := vs.StateGen.StateByRoot(ctx, bytesutil.ToBytes32(block.Block().ParentRoot()))
	if err != nil {
		return nil, errors.Wrap(err, "could not retrieve beacon state")
	}
	root, err := state.CalculateStateRoot(
		ctx,
		beaconState,
		block,
	)
	if err != nil {
		return nil, errors.Wrapf(err, "could not calculate state root at slot %d", beaconState.Slot())
	}

	log.WithField("beaconStateRoot", fmt.Sprintf("%#x", root)).Debugf("Computed state root")
	return root[:], nil
}

// deposits returns a list of pending deposits that are ready for inclusion in the next beacon
// block. Determining deposits depends on the current eth1data vote for the block and whether or not
// this eth1data has enough support to be considered for deposits inclusion. If current vote has
// enough support, then use that vote for basis of determining deposits, otherwise use current state
// eth1data.
func (vs *Server) deposits(
	ctx context.Context,
	beaconState iface.BeaconState,
	currentVote *ethpb.Eth1Data,
) ([]*ethpb.Deposit, error) {
	ctx, span := trace.StartSpan(ctx, "ProposerServer.Deposits")
	defer span.End()

	if vs.MockEth1Votes || !vs.Eth1InfoFetcher.IsConnectedToETH1() {
		return []*ethpb.Deposit{}, nil
	}
	// Need to fetch if the Deposits up to the state's latest eth 1 data matches
	// the number of all Deposits in this RPC call. If not, then we return nil.
	canonicalEth1Data, canonicalEth1DataHeight, err := vs.canonicalEth1Data(ctx, beaconState, currentVote)
	if err != nil {
		return nil, err
	}

	_, genesisEth1Block := vs.Eth1InfoFetcher.Eth2GenesisPowchainInfo()
	if genesisEth1Block.Cmp(canonicalEth1DataHeight) == 0 {
		return []*ethpb.Deposit{}, nil
	}

	// If there are no pending Deposits, exit early.
	allPendingContainers := vs.PendingDepositsFetcher.PendingContainers(ctx, canonicalEth1DataHeight)
	if len(allPendingContainers) == 0 {
		return []*ethpb.Deposit{}, nil
	}

	depositTrie, err := vs.depositTrie(ctx, canonicalEth1Data, canonicalEth1DataHeight)
	if err != nil {
		return nil, errors.Wrap(err, "could not retrieve deposit trie")
	}

	// Deposits need to be received in order of merkle index root, so this has to make sure
	// Deposits are sorted from lowest to highest.
	var pendingDeps []*dbpb.DepositContainer
	for _, dep := range allPendingContainers {
		if uint64(dep.Index) >= beaconState.Eth1DepositIndex() && uint64(dep.Index) < canonicalEth1Data.DepositCount {
			pendingDeps = append(pendingDeps, dep)
		}
	}

	for i := range pendingDeps {
		// Don't construct merkle proof if the number of Deposits is more than max allowed in block.
		if uint64(i) == params.BeaconConfig().MaxDeposits {
			break
		}
		pendingDeps[i].Deposit, err = constructMerkleProof(depositTrie, int(pendingDeps[i].Index), pendingDeps[i].Deposit)
		if err != nil {
			return nil, err
		}
	}
	// Limit the return of pending Deposits to not be more than max Deposits allowed in block.
	var pendingDeposits []*ethpb.Deposit
	for i := uint64(0); i < uint64(len(pendingDeps)) && i < params.BeaconConfig().MaxDeposits; i++ {
		pendingDeposits = append(pendingDeposits, pendingDeps[i].Deposit)
	}
	return pendingDeposits, nil
}

// canonicalEth1Data determines the canonical eth1data and eth1 block height to use for determining deposits.
func (vs *Server) canonicalEth1Data(
	ctx context.Context,
	beaconState iface.BeaconState,
	currentVote *ethpb.Eth1Data) (*ethpb.Eth1Data, *big.Int, error) {

	var eth1BlockHash [32]byte

	// Add in current vote, to get accurate vote tally
	if err := beaconState.AppendEth1DataVotes(currentVote); err != nil {
		return nil, nil, errors.Wrap(err, "could not append eth1 data votes to state")
	}
	hasSupport, err := blocks.Eth1DataHasEnoughSupport(beaconState, currentVote)
	if err != nil {
		return nil, nil, errors.Wrap(err, "could not determine if current eth1data vote has enough support")
	}
	var canonicalEth1Data *ethpb.Eth1Data
	if hasSupport {
		canonicalEth1Data = currentVote
		eth1BlockHash = bytesutil.ToBytes32(currentVote.BlockHash)
	} else {
		canonicalEth1Data = beaconState.Eth1Data()
		eth1BlockHash = bytesutil.ToBytes32(beaconState.Eth1Data().BlockHash)
	}
	_, canonicalEth1DataHeight, err := vs.Eth1BlockFetcher.BlockExists(ctx, eth1BlockHash)
	if err != nil {
		return nil, nil, errors.Wrap(err, "could not fetch eth1data height")
	}
	return canonicalEth1Data, canonicalEth1DataHeight, nil
}

func (vs *Server) depositTrie(ctx context.Context, canonicalEth1Data *ethpb.Eth1Data, canonicalEth1DataHeight *big.Int) (*trieutil.SparseMerkleTrie, error) {
	ctx, span := trace.StartSpan(ctx, "ProposerServer.depositTrie")
	defer span.End()

	var depositTrie *trieutil.SparseMerkleTrie

	finalizedDeposits := vs.DepositFetcher.FinalizedDeposits(ctx)
	depositTrie = finalizedDeposits.Deposits
	upToEth1DataDeposits := vs.DepositFetcher.NonFinalizedDeposits(ctx, canonicalEth1DataHeight)
	insertIndex := finalizedDeposits.MerkleTrieIndex + 1

	for _, dep := range upToEth1DataDeposits {
		depHash, err := dep.Data.HashTreeRoot()
		if err != nil {
			return nil, errors.Wrap(err, "could not hash deposit data")
		}
		depositTrie.Insert(depHash[:], int(insertIndex))
		insertIndex++
	}
	valid, err := vs.validateDepositTrie(depositTrie, canonicalEth1Data)
	// Log a warning here, as the cached trie is invalid.
	if !valid {
		log.Warnf("Cached deposit trie is invalid, rebuilding it now: %v", err)
		return vs.rebuildDepositTrie(ctx, canonicalEth1Data, canonicalEth1DataHeight)
	}

	return depositTrie, nil
}

// rebuilds our deposit trie by recreating it from all processed deposits till
// specified eth1 block height.
func (vs *Server) rebuildDepositTrie(ctx context.Context, canonicalEth1Data *ethpb.Eth1Data, canonicalEth1DataHeight *big.Int) (*trieutil.SparseMerkleTrie, error) {
	ctx, span := trace.StartSpan(ctx, "ProposerServer.rebuildDepositTrie")
	defer span.End()

	deposits := vs.DepositFetcher.AllDeposits(ctx, canonicalEth1DataHeight)
	trieItems := make([][]byte, 0, len(deposits))
	for _, dep := range deposits {
		depHash, err := dep.Data.HashTreeRoot()
		if err != nil {
			return nil, errors.Wrap(err, "could not hash deposit data")
		}
		trieItems = append(trieItems, depHash[:])
	}
	depositTrie, err := trieutil.GenerateTrieFromItems(trieItems, params.BeaconConfig().DepositContractTreeDepth)
	if err != nil {
		return nil, err
	}

	valid, err := vs.validateDepositTrie(depositTrie, canonicalEth1Data)
	// Log an error here, as even with rebuilding the trie, it is still invalid.
	if !valid {
		log.Errorf("Rebuilt deposit trie is invalid: %v", err)
	}
	return depositTrie, nil
}

// validate that the provided deposit trie matches up with the canonical eth1 data provided.
func (vs *Server) validateDepositTrie(trie *trieutil.SparseMerkleTrie, canonicalEth1Data *ethpb.Eth1Data) (bool, error) {
	if trie.NumOfItems() != int(canonicalEth1Data.DepositCount) {
		return false, errors.Errorf("wanted the canonical count of %d but received %d", canonicalEth1Data.DepositCount, trie.NumOfItems())
	}
	rt := trie.HashTreeRoot()
	if !bytes.Equal(rt[:], canonicalEth1Data.DepositRoot) {
		return false, errors.Errorf("wanted the canonical deposit root of %#x but received %#x", canonicalEth1Data.DepositRoot, rt)
	}
	return true, nil
}

// in case no vote for new eth1data vote considered best vote we
// default into returning the latest deposit root and the block
// hash of eth1 block hash that is FOLLOW_DISTANCE back from its
// latest block.
func (vs *Server) defaultEth1DataResponse(ctx context.Context, currentHeight *big.Int) (*ethpb.Eth1Data, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	eth1FollowDistance := int64(params.BeaconConfig().Eth1FollowDistance)
	ancestorHeight := big.NewInt(0).Sub(currentHeight, big.NewInt(eth1FollowDistance))
	blockHash, err := vs.Eth1BlockFetcher.BlockHashByHeight(ctx, ancestorHeight)
	if err != nil {
		return nil, errors.Wrap(err, "could not fetch ETH1_FOLLOW_DISTANCE ancestor")
	}
	// Fetch all historical Deposits up to an ancestor height.
	depositsTillHeight, depositRoot := vs.DepositFetcher.DepositsNumberAndRootAtHeight(ctx, ancestorHeight)
	if depositsTillHeight == 0 {
		return vs.ChainStartFetcher.ChainStartEth1Data(), nil
	}
	// // Make sure we don't "undo deposit progress". See https://github.com/ethereum/eth2.0-specs/pull/1836
	currentETH1Data := vs.HeadFetcher.HeadETH1Data()
	if depositsTillHeight < currentETH1Data.DepositCount {
		return currentETH1Data, nil
	}
	return &ethpb.Eth1Data{
		DepositRoot:  depositRoot[:],
		BlockHash:    blockHash[:],
		DepositCount: depositsTillHeight,
	}, nil
}

// This filters the input Attestations to return a list of valid Attestations to be packaged inside a beacon block.
func (vs *Server) filterAttestationsForBlockInclusion(ctx context.Context, st iface.BeaconState, atts []*ethpb.Attestation) ([]*ethpb.Attestation, error) {
	ctx, span := trace.StartSpan(ctx, "ProposerServer.filterAttestationsForBlockInclusion")
	defer span.End()

	validAtts, invalidAtts := proposerAtts(atts).filter(ctx, st)
	if err := vs.deleteAttsInPool(ctx, invalidAtts); err != nil {
		return nil, err
	}
	deduped, err := validAtts.dedup()
	if err != nil {
		return nil, err
	}
	sorted, err := deduped.sortByProfitability()
	if err != nil {
		return nil, err
	}
	return sorted.limitToMaxAttestations(), nil
}

// The input Attestations are processed and seen by the node, this deletes them from pool
// so proposers don't include them in a block for the future.
func (vs *Server) deleteAttsInPool(ctx context.Context, atts []*ethpb.Attestation) error {
	ctx, span := trace.StartSpan(ctx, "ProposerServer.deleteAttsInPool")
	defer span.End()

	for _, att := range atts {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if helpers.IsAggregated(att) {
			if err := vs.AttPool.DeleteAggregatedAttestation(att); err != nil {
				return err
			}
		} else {
			if err := vs.AttPool.DeleteUnaggregatedAttestation(att); err != nil {
				return err
			}
		}
	}
	return nil
}

func constructMerkleProof(trie *trieutil.SparseMerkleTrie, index int, deposit *ethpb.Deposit) (*ethpb.Deposit, error) {
	proof, err := trie.MerkleProof(index)
	if err != nil {
		return nil, errors.Wrapf(err, "could not generate merkle proof for deposit at index %d", index)
	}
	// For every deposit, we construct a Merkle proof using the powchain service's
	// in-memory Deposits trie, which is updated only once the state's LatestETH1Data
	// property changes during a state transition after a voting period.
	deposit.Proof = proof
	return deposit, nil
}

func (vs *Server) packAttestations(ctx context.Context, latestState iface.BeaconState) ([]*ethpb.Attestation, error) {
	ctx, span := trace.StartSpan(ctx, "ProposerServer.packAttestations")
	defer span.End()

	atts := vs.AttPool.AggregatedAttestations()
	atts, err := vs.filterAttestationsForBlockInclusion(ctx, latestState, atts)
	if err != nil {
		return nil, errors.Wrap(err, "could not filter Attestations")
	}

	// If there is any room left in the block, consider unaggregated Attestations as well.
	numAtts := uint64(len(atts))
	if numAtts < params.BeaconConfig().MaxAttestations {
		uAtts, err := vs.AttPool.UnaggregatedAttestations()
		if err != nil {
			return nil, errors.Wrap(err, "could not get unaggregated Attestations")
		}
		uAtts, err = vs.filterAttestationsForBlockInclusion(ctx, latestState, uAtts)
		if err != nil {
			return nil, errors.Wrap(err, "could not filter Attestations")
		}
		atts = append(atts, uAtts...)

		attsByDataRoot := make(map[[32]byte][]*ethpb.Attestation, len(atts))
		for _, att := range atts {
			attDataRoot, err := att.Data.HashTreeRoot()
			if err != nil {
				return nil, err
			}
			attsByDataRoot[attDataRoot] = append(attsByDataRoot[attDataRoot], att)
		}

		attsForInclusion := proposerAtts(make([]*ethpb.Attestation, 0))
		for _, as := range attsByDataRoot {
			as, err := attaggregation.Aggregate(as)
			if err != nil {
				return nil, err
			}
			attsForInclusion = append(attsForInclusion, as...)
		}
		deduped, err := attsForInclusion.dedup()
		if err != nil {
			return nil, err
		}
		sorted, err := deduped.sortByProfitability()
		if err != nil {
			return nil, err
		}
		atts = sorted.limitToMaxAttestations()
	}
	return atts, nil
}
