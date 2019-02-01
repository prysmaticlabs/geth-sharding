// Package epoch contains epoch processing libraries. These libraries
// process new balance for the validators, justify and finalize new
// check points, shuffle and reassign validators to different slots and
// shards.
package epoch

import (
	"bytes"
	"fmt"
	"math"

	block "github.com/prysmaticlabs/prysm/beacon-chain/core/blocks"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/validators"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	b "github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/prysmaticlabs/prysm/shared/params"
)

// Attestations returns the pending attestations of slots in the epoch
// (state.slot-EPOCH_LENGTH...state.slot-1), not attestations that got
// included in the chain during the epoch.
//
// Spec pseudocode definition:
//   return [a for a in state.latest_attestations if
//   	current_epoch == slot_to_epoch(a.data.slot)
func Attestations(state *pb.BeaconState) []*pb.PendingAttestationRecord {
	var thisEpochAttestations []*pb.PendingAttestationRecord
	currentEpoch := helpers.CurrentEpoch(state)

	for _, attestation := range state.LatestAttestations {
		if currentEpoch == helpers.SlotToEpoch(attestation.Data.Slot) {
			thisEpochAttestations = append(thisEpochAttestations, attestation)
		}
	}
	return thisEpochAttestations
}

// BoundaryAttestations returns the pending attestations from
// the epoch's boundary block.
//
// Spec pseudocode definition:
//   return [a for a in this_epoch_attestations if a.data.epoch_boundary_root ==
//   get_block_root(state, state.slot-EPOCH_LENGTH) and a.justified_slot ==
//   state.justified_slot]
func BoundaryAttestations(
	state *pb.BeaconState,
	thisEpochAttestations []*pb.PendingAttestationRecord,
) ([]*pb.PendingAttestationRecord, error) {
	epochLength := params.BeaconConfig().EpochLength
	var boundarySlot uint64
	var boundaryAttestations []*pb.PendingAttestationRecord

	for _, attestation := range thisEpochAttestations {

		// If boundary slot is less than epoch length, then it would
		// result in a negative number. Therefore we should
		// default boundarySlot = 0 in this case.
		if state.Slot > epochLength {
			boundarySlot = state.Slot - epochLength
		}

		boundaryBlockRoot, err := block.BlockRoot(state, boundarySlot)
		if err != nil {
			return nil, err
		}

		attestationData := attestation.Data
		sameRoot := bytes.Equal(attestationData.JustifiedBlockRootHash32, boundaryBlockRoot)
		sameSlotNum := attestationData.JustifiedSlot == state.JustifiedSlot
		if sameRoot && sameSlotNum {
			boundaryAttestations = append(boundaryAttestations, attestation)
		}
	}
	return boundaryAttestations, nil
}

// PrevAttestations returns the attestations of the previous epoch
// (state.slot - 2 * EPOCH_LENGTH...state.slot - EPOCH_LENGTH).
//
// Spec pseudocode definition:
//   return [a for a in state.latest_attestations if
//   	previous_epoch == slot_to_epoch(a.data.slot)].
func PrevAttestations(state *pb.BeaconState) []*pb.PendingAttestationRecord {
	var prevEpochAttestations []*pb.PendingAttestationRecord
	prevEpoch := helpers.PrevEpoch(state)

	for _, attestation := range state.LatestAttestations {
		if prevEpoch == helpers.SlotToEpoch(attestation.Data.Slot) {
			prevEpochAttestations = append(prevEpochAttestations, attestation)
		}
	}

	return prevEpochAttestations
}

// PrevJustifiedAttestations returns the justified attestations
// of the previous 2 epochs.
//
// Spec pseudocode definition:
//   return [a for a in this_epoch_attestations + previous_epoch_attestations
//   if a.justified_slot == state.previous_justified_slot]
func PrevJustifiedAttestations(
	state *pb.BeaconState,
	thisEpochAttestations []*pb.PendingAttestationRecord,
	prevEpochAttestations []*pb.PendingAttestationRecord,
) []*pb.PendingAttestationRecord {

	var prevJustifiedAttestations []*pb.PendingAttestationRecord
	epochAttestations := append(thisEpochAttestations, prevEpochAttestations...)

	for _, attestation := range epochAttestations {
		if attestation.Data.JustifiedSlot == state.PreviousJustifiedSlot {
			prevJustifiedAttestations = append(prevJustifiedAttestations, attestation)
		}
	}
	return prevJustifiedAttestations
}

// PrevBoundaryAttestations returns the boundary attestations
// at the start of the previous epoch.
//
// Spec pseudocode definition:
//   return [a for a in previous_epoch_justified_attestations
// 	 if a.epoch_boundary_root == get_block_root(state, state.slot - 2 * EPOCH_LENGTH)]
func PrevBoundaryAttestations(
	state *pb.BeaconState,
	prevEpochJustifiedAttestations []*pb.PendingAttestationRecord,
) ([]*pb.PendingAttestationRecord, error) {
	var earliestSlot uint64

	// If the state slot is less than 2 * epochLength, then the earliestSlot would
	// result in a negative number. Therefore we should default to
	// earliestSlot = 0 in this case.
	if state.Slot > 2*params.BeaconConfig().EpochLength {
		earliestSlot = state.Slot - 2*params.BeaconConfig().EpochLength
	}

	var prevBoundaryAttestations []*pb.PendingAttestationRecord
	prevBoundaryBlockRoot, err := block.BlockRoot(state,
		earliestSlot)
	if err != nil {
		return nil, err
	}
	for _, attestation := range prevEpochJustifiedAttestations {
		if bytes.Equal(attestation.Data.EpochBoundaryRootHash32, prevBoundaryBlockRoot) {
			prevBoundaryAttestations = append(prevBoundaryAttestations, attestation)
		}
	}
	return prevBoundaryAttestations, nil
}

// PrevHeadAttestations returns the pending attestations from
// the canonical beacon chain.
//
// Spec pseudocode definition:
//   return [a for a in previous_epoch_attestations
//   if a.beacon_block_root == get_block_root(state, a.slot)]
func PrevHeadAttestations(
	state *pb.BeaconState,
	prevEpochAttestations []*pb.PendingAttestationRecord,
) ([]*pb.PendingAttestationRecord, error) {

	var headAttestations []*pb.PendingAttestationRecord
	for _, attestation := range prevEpochAttestations {
		canonicalBlockRoot, err := block.BlockRoot(state, attestation.Data.Slot)
		if err != nil {
			return nil, err
		}

		attestationData := attestation.Data
		if bytes.Equal(attestationData.BeaconBlockRootHash32, canonicalBlockRoot) {
			headAttestations = append(headAttestations, attestation)
		}
	}
	return headAttestations, nil
}

// TotalBalance returns the total balance at stake of the validators
// from the shard committee regardless of validators attested or not.
//
// Spec pseudocode definition:
//    Let total_balance =
//    sum([get_effective_balance(state, i) for i in active_validator_indices])
func TotalBalance(
	state *pb.BeaconState,
	activeValidatorIndices []uint64) uint64 {

	var totalBalance uint64
	for _, index := range activeValidatorIndices {
		totalBalance += validators.EffectiveBalance(state, index)
	}

	return totalBalance
}

// InclusionSlot returns the slot number of when the validator's
// attestation gets included in the beacon chain.
//
// Spec pseudocode definition:
//    Let inclusion_slot(state, index) =
//    a.slot_included for the attestation a where index is in
//    get_attestation_participants(state, a.data, a.participation_bitfield)
//    If multiple attestations are applicable, the attestation with
//    lowest `slot_included` is considered.
func InclusionSlot(state *pb.BeaconState, validatorIndex uint64) (uint64, error) {
	lowestSlotIncluded := uint64(math.MaxUint64)
	for _, attestation := range state.LatestAttestations {
		participatedValidators, err := validators.AttestationParticipants(state, attestation.Data, attestation.ParticipationBitfield)
		if err != nil {
			return 0, fmt.Errorf("could not get attestation participants: %v", err)
		}
		for _, index := range participatedValidators {
			if index == validatorIndex {
				if attestation.SlotIncluded < lowestSlotIncluded {
					lowestSlotIncluded = attestation.SlotIncluded
				}
			}
		}
	}
	if lowestSlotIncluded == math.MaxUint64 {
		return 0, fmt.Errorf("could not find inclusion slot for validator index %d", validatorIndex)
	}
	return lowestSlotIncluded, nil
}

// InclusionDistance returns the difference in slot number of when attestation
// gets submitted and when it gets included.
//
// Spec pseudocode definition:
//    Let inclusion_distance(state, index) =
//    a.slot_included - a.data.slot where a is the above attestation same as
//    inclusion_slot
func InclusionDistance(state *pb.BeaconState, validatorIndex uint64) (uint64, error) {

	for _, attestation := range state.LatestAttestations {
		participatedValidators, err := validators.AttestationParticipants(state, attestation.Data, attestation.ParticipationBitfield)
		if err != nil {
			return 0, fmt.Errorf("could not get attestation participants: %v", err)
		}

		for _, index := range participatedValidators {
			if index == validatorIndex {
				return attestation.SlotIncluded - attestation.Data.Slot, nil
			}
		}
	}
	return 0, fmt.Errorf("could not find inclusion distance for validator index %d", validatorIndex)
}

// AttestingValidators returns the validators of the winning root.
//
// Spec pseudocode definition:
//    Let `attesting_validators(shard_committee)` be equal to
//    `attesting_validator_indices(shard_committee, winning_root(shard_committee))` for convenience
func AttestingValidators(
	state *pb.BeaconState,
	shard uint64, thisEpochAttestations []*pb.PendingAttestationRecord,
	prevEpochAttestations []*pb.PendingAttestationRecord) ([]uint64, error) {

	root, err := winningRoot(
		state,
		shard,
		thisEpochAttestations,
		prevEpochAttestations)
	if err != nil {
		return nil, fmt.Errorf("could not get winning root: %v", err)
	}

	indices, err := validators.AttestingValidatorIndices(
		state,
		shard,
		root,
		thisEpochAttestations,
		prevEpochAttestations)
	if err != nil {
		return nil, fmt.Errorf("could not get attesting validator indices: %v", err)
	}

	return indices, nil
}

// TotalAttestingBalance returns the total balance at stake of the validators
// attested to the winning root.
//
// Spec pseudocode definition:
//    Let total_balance(shard_committee) =
//    sum([get_effective_balance(state, i) for i in shard_committee.committee])
func TotalAttestingBalance(
	state *pb.BeaconState,
	shard uint64,
	thisEpochAttestations []*pb.PendingAttestationRecord,
	prevEpochAttestations []*pb.PendingAttestationRecord) (uint64, error) {

	var totalBalance uint64
	attestedValidatorIndices, err := AttestingValidators(state, shard, thisEpochAttestations, prevEpochAttestations)
	if err != nil {
		return 0, fmt.Errorf("could not get attesting validator indices: %v", err)
	}

	for _, index := range attestedValidatorIndices {
		totalBalance += validators.EffectiveBalance(state, index)
	}

	return totalBalance, nil
}

// SinceFinality calculates and returns how many epoch has it been since
// a finalized slot.
//
// Spec pseudocode definition:
//    epochs_since_finality = (state.slot - state.finalized_slot) // EPOCH_LENGTH
func SinceFinality(state *pb.BeaconState) uint64 {
	return (state.Slot - state.FinalizedSlot) / params.BeaconConfig().EpochLength
}

// winningRoot returns the shard block root with the most combined validator
// effective balance. The ties broken by favoring lower shard block root values.
//
// Spec pseudocode definition:
//   Let winning_root(crosslink_committee) be equal to the value of shard_block_root
//   such that sum([get_effective_balance(state, i)
//   for i in attesting_validator_indices(crosslink_committee, shard_block_root)])
//   is maximized (ties broken by favoring lower shard_block_root values)
func winningRoot(
	state *pb.BeaconState,
	shard uint64,
	thisEpochAttestations []*pb.PendingAttestationRecord,
	prevEpochAttestations []*pb.PendingAttestationRecord) ([]byte, error) {

	var winnerBalance uint64
	var winnerRoot []byte
	var candidateRoots [][]byte
	attestations := append(thisEpochAttestations, prevEpochAttestations...)

	for _, attestation := range attestations {
		if attestation.Data.Shard == shard {
			candidateRoots = append(candidateRoots, attestation.Data.ShardBlockRootHash32)
		}
	}

	for _, candidateRoot := range candidateRoots {
		indices, err := validators.AttestingValidatorIndices(
			state,
			shard,
			candidateRoot,
			thisEpochAttestations,
			prevEpochAttestations)
		if err != nil {
			return nil, fmt.Errorf("could not get attesting validator indices: %v", err)
		}

		var rootBalance uint64
		for _, index := range indices {
			rootBalance += validators.EffectiveBalance(state, index)
		}

		if rootBalance > winnerBalance ||
			(rootBalance == winnerBalance && b.LowerThan(candidateRoot, winnerRoot)) {
			winnerBalance = rootBalance
			winnerRoot = candidateRoot
		}
	}
	return winnerRoot, nil
}

// randaoMix returns the randao mix (xor'ed seed)
// of a given slot. It is used to shuffle validators.
//
// Spec pseudocode definition:
//   def get_randao_mix(state: BeaconState,
//                   slot: int) -> Hash32:
//    """
//    Returns the randao mix at a recent ``slot``.
//    """
//    assert state.slot <= slot + LATEST_RANDAO_MIXES_LENGTH
//	  assert slot < state.slot
//    return state.latest_block_roots[slot % LATEST_RANDAO_MIXES_LENGTH]
func randaoMix(state *pb.BeaconState, slot uint64) ([]byte, error) {
	var lowerBound uint64
	if state.Slot > config.LatestRandaoMixesLength {
		lowerBound = state.Slot - config.LatestRandaoMixesLength
	}
	upperBound := state.Slot
	if lowerBound > slot || slot >= upperBound {
		return nil, fmt.Errorf("input randaoMix slot %d out of bounds: %d <= slot < %d",
			slot, lowerBound, upperBound)
	}
	return state.LatestRandaoMixesHash32S[slot%config.LatestRandaoMixesLength], nil
}
