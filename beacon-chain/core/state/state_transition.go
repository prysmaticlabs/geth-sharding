package state

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/incentives"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/randao"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/types"
	v "github.com/prysmaticlabs/prysm/beacon-chain/core/validators"
	"github.com/prysmaticlabs/prysm/beacon-chain/utils"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	"github.com/prysmaticlabs/prysm/shared/params"
)

// NewStateTransition computes the new beacon state.
// DEPRECATED: Will be removed soon.
// This function takes in the previous beacon stat, beacon block, and its parent slot.
// This method is called during a cycle transition. We also check for validator
// set change transition and compute for new committees if necessary during this transition.
func NewStateTransition(
	st *types.BeaconState,
	block *pb.BeaconBlock,
	parentSlot uint64,
	blockVoteCache utils.BlockVoteCache,
) (*types.BeaconState, error) {
	var lastStateRecalculationSlotCycleBack uint64
	var err error

	newState := st.CopyState()
	justifiedStreak := st.JustifiedStreak()
	justifiedSlot := st.LastJustifiedSlot()
	finalizedSlot := st.LastFinalizedSlot()
	timeSinceFinality := block.GetSlot() - newState.LastFinalizedSlot()
	newState.SetValidatorRegistry(v.CopyValidatorRegistry(newState.ValidatorRegistry()))

	newState.ClearAttestations(st.LastStateRecalculationSlot())
	// Derive the new set of recent block hashes.
	recentBlockHashes, err := st.CalculateNewBlockHashes(block, parentSlot)
	if err != nil {
		return nil, err
	}
	newState.SetLatestBlockHashes(recentBlockHashes)
	if err != nil {
		return nil, fmt.Errorf("failed to update recent block hashes: %v", err)
	}

	var blockRandao [32]byte
	copy(blockRandao[:], block.GetRandaoRevealHash32())
	newRandao := createRandaoMix(blockRandao, st.RandaoMix())
	newState.SetRandaoMix(newRandao[:])

	// The changes below are only applied if this is a cycle transition.
	if block.GetSlot()%params.BeaconConfig().CycleLength == 0 {
		if st.LastStateRecalculationSlot() < params.BeaconConfig().CycleLength {
			lastStateRecalculationSlotCycleBack = 0
		} else {
			lastStateRecalculationSlotCycleBack = st.LastStateRecalculationSlot() - params.BeaconConfig().CycleLength
		}

		// walk through all the slots from LastStateRecalculationSlot - cycleLength to
		// LastStateRecalculationSlot - 1.
		for i := uint64(0); i < params.BeaconConfig().CycleLength; i++ {
			var blockVoteBalance uint64

			slot := lastStateRecalculationSlotCycleBack + i
			blockHash := recentBlockHashes[i]

			blockVoteBalance, validators := incentives.TallyVoteBalances(
				common.BytesToHash(blockHash),
				blockVoteCache,
				newState.ValidatorRegistry(),
				v.ActiveValidatorIndices(newState.ValidatorRegistry()),
				v.TotalActiveValidatorBalance(newState.ValidatorRegistry()),
				timeSinceFinality,
			)

			newState.SetValidatorRegistry(validators)

			justifiedSlot, finalizedSlot, justifiedStreak = FinalizeAndJustifySlots(
				slot,
				justifiedSlot,
				finalizedSlot,
				justifiedStreak,
				blockVoteBalance,
				v.TotalActiveValidatorBalance(st.ValidatorRegistry()),
			)
		}

		crossLinks, err := crossLinkCalculations(
			newState,
			st.PendingAttestations(),
			block.GetSlot(),
		)
		if err != nil {
			return nil, err
		}

		newState.SetCrossLinks(crossLinks)

		newState.SetLastJustifiedSlot(justifiedSlot)
		newState.SetLastFinalizedSlot(finalizedSlot)
		newState.SetJustifiedStreak(justifiedStreak)

		// Exit the validators when their balance fall below min online deposit size.
		newState.SetValidatorRegistry(v.CheckValidatorMinDeposit(newState.ValidatorRegistry(), block.GetSlot()))

		// Entering new validator set change transition.
		if newState.IsValidatorSetChange(block.GetSlot()) {
			newState.SetValidatorRegistryLastChangeSlot(newState.LastStateRecalculationSlot())
			var parentRoot [32]byte
			copy(parentRoot[:], block.GetParentRootHash32())
			shardAndCommitteesForSlots, err := validatorSetRecalculations(
				newState.ShardAndCommitteesForSlots(),
				newState.ValidatorRegistry(),
				parentRoot,
			)
			if err != nil {
				return nil, err
			}
			newState.SetShardAndCommitteesForSlots(shardAndCommitteesForSlots)

			period := block.GetSlot() / params.BeaconConfig().MinWithdrawalPeriod
			totalPenalties := newState.PenalizedETH(period)
			newState.SetValidatorRegistry(v.ChangeValidatorRegistry(block.GetSlot(), totalPenalties, newState.ValidatorRegistry()))
		}
	}
	newState.SetLastStateRecalculationSlot(newState.LastStateRecalculationSlot() + 1)
	return newState, nil
}

// ExecuteStateTransition defines the procedure for a state transition function.
// Spec:
//  We now define the state transition function. At a high level the state transition is made up of two parts:
//  - The per-slot transitions, which happens every slot, and only affects a parts of the state.
//  - The per-epoch transitions, which happens at every epoch boundary (i.e. state.slot % EPOCH_LENGTH == 0), and affects the entire state.
//  The per-slot transitions generally focus on verifying aggregate signatures and saving temporary records relating to the per-slot
//  activity in the BeaconState. The per-epoch transitions focus on the validator registry, including adjusting balances and activating
//  and exiting validators, as well as processing crosslinks and managing block justification/finalization.
func ExecuteStateTransition(
	beaconState *types.BeaconState,
	block *pb.BeaconBlock,
) (*types.BeaconState, error) {

	var err error

	newState := beaconState.CopyState()

	currentSlot := newState.Slot()
	newState.SetSlot(currentSlot + 1)

	newState, err = randao.UpdateRandaoLayers(newState, newState.Slot())
	if err != nil {
		return nil, fmt.Errorf("unable to update randao layer %v", err)
	}

	newHashes, err := newState.CalculateNewBlockHashes(block, currentSlot)
	if err != nil {
		return nil, fmt.Errorf("unable to calculate recent blockhashes")
	}

	newState.SetLatestBlockHashes(newHashes)

	if block != nil {
		newState = ProcessBlock(newState, block)
		if newState.Slot()%params.BeaconConfig().EpochLength == 0 {
			newState = NewEpochTransition(newState)
		}
	}

	return newState, nil
}

// NewEpochTransition describes the per epoch operations that are performed on the
// beacon state.
func NewEpochTransition(state *types.BeaconState) *types.BeaconState {
	// TODO(#1074): This will encompass all the related logic to epoch transitions.
	return state
}

// crossLinkCalculations checks if the proposed shard block has recevied
// 2/3 of the votes. If yes, we update crosslink record to point to
// the proposed shard block with latest beacon chain slot numbers.
func crossLinkCalculations(
	st *types.BeaconState,
	pendingAttestations []*pb.AggregatedAttestation,
	currentSlot uint64,
) ([]*pb.CrosslinkRecord, error) {
	slot := st.LastStateRecalculationSlot() + params.BeaconConfig().CycleLength
	crossLinkRecords := st.LatestCrosslinks()
	for _, attestation := range pendingAttestations {
		shardCommittees, err := v.GetShardAndCommitteesForSlot(
			st.ShardAndCommitteesForSlots(),
			st.LastStateRecalculationSlot(),
			attestation.GetSlot(),
		)
		if err != nil {
			return nil, err
		}

		indices, err := v.AttesterIndices(shardCommittees, attestation)
		if err != nil {
			return nil, err
		}

		totalBalance, voteBalance, err := v.VotedBalanceInAttestation(st.ValidatorRegistry(), indices, attestation)
		if err != nil {
			return nil, err
		}

		newValidatorSet, err := incentives.ApplyCrosslinkRewardsAndPenalties(
			crossLinkRecords,
			currentSlot,
			indices,
			attestation,
			st.ValidatorRegistry(),
			v.TotalActiveValidatorBalance(st.ValidatorRegistry()),
			totalBalance,
			voteBalance,
		)
		if err != nil {
			return nil, err
		}
		st.SetValidatorRegistry(newValidatorSet)
		crossLinkRecords = UpdateLatestCrosslinks(slot, voteBalance, totalBalance, attestation, crossLinkRecords)
	}
	return crossLinkRecords, nil
}

// validatorSetRecalculation recomputes the validator set.
func validatorSetRecalculations(
	shardAndCommittesForSlots []*pb.ShardAndCommitteeArray,
	validators []*pb.ValidatorRecord,
	seed [32]byte,
) ([]*pb.ShardAndCommitteeArray, error) {
	lastSlot := len(shardAndCommittesForSlots) - 1
	lastCommitteeFromLastSlot := len(shardAndCommittesForSlots[lastSlot].ArrayShardAndCommittee) - 1
	crosslinkLastShard := shardAndCommittesForSlots[lastSlot].ArrayShardAndCommittee[lastCommitteeFromLastSlot].Shard
	crosslinkNextShard := (crosslinkLastShard + 1) % params.BeaconConfig().ShardCount

	newShardCommitteeArray, err := v.ShuffleValidatorRegistryToCommittees(
		seed,
		validators,
		crosslinkNextShard,
	)
	if err != nil {
		return nil, err
	}

	return append(shardAndCommittesForSlots[params.BeaconConfig().CycleLength:], newShardCommitteeArray...), nil
}

// createRandaoMix sets the block randao seed into a beacon state randao. This function
// XOR's the current state randao with the block's randao value added by the
// proposer.
func createRandaoMix(blockRandao [32]byte, beaconStateRandao [32]byte) [32]byte {
	for i, b := range blockRandao {
		beaconStateRandao[i] ^= b
	}
	return beaconStateRandao
}
