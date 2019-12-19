// Package helpers contains helper functions outlined in ETH2.0 spec beacon chain spec
package helpers

import (
	"fmt"
	"sort"

	"github.com/pkg/errors"
	ethpb "github.com/prysmaticlabs/ethereumapis/eth/v1alpha1"
	"github.com/prysmaticlabs/go-bitfield"
	"github.com/prysmaticlabs/prysm/beacon-chain/cache"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	"github.com/prysmaticlabs/prysm/shared/featureconfig"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/sliceutil"
)

var committeeCache = cache.NewCommitteesCache()

// CommitteeCountAtSlot returns the number of crosslink committees of a slot.
//
// Spec pseudocode definition:
//   def get_committee_count_at_slot(state: BeaconState, slot: Slot) -> uint64:
//    """
//    Return the number of committees at ``slot``.
//    """
//    epoch = compute_epoch_at_slot(slot)
//    return max(1, min(
//        MAX_COMMITTEES_PER_SLOT,
//        len(get_active_validator_indices(state, epoch)) // SLOTS_PER_EPOCH // TARGET_COMMITTEE_SIZE,
//    ))
func CommitteeCountAtSlot(state *pb.BeaconState, slot uint64) (uint64, error) {
	epoch := SlotToEpoch(slot)
	count, err := ActiveValidatorCount(state, epoch)
	if err != nil {
		return 0, errors.Wrap(err, "could not get active count")
	}
	var committeePerSlot = count / params.BeaconConfig().SlotsPerEpoch / params.BeaconConfig().TargetCommitteeSize
	if committeePerSlot > params.BeaconConfig().MaxCommitteesPerSlot {
		return params.BeaconConfig().MaxCommitteesPerSlot, nil
	}
	if committeePerSlot == 0 {
		return 1, nil
	}
	return committeePerSlot, nil
}

// BeaconCommittee returns the crosslink committee of a given slot and committee index.
//
// Spec pseudocode definition:
//   def get_beacon_committee(state: BeaconState, slot: Slot, index: CommitteeIndex) -> Sequence[ValidatorIndex]:
//    """
//    Return the beacon committee at ``slot`` for ``index``.
//    """
//    epoch = compute_epoch_at_slot(slot)
//    committees_per_slot = get_committee_count_at_slot(state, slot)
//    epoch_offset = index + (slot % SLOTS_PER_EPOCH) * committees_per_slot
//    return compute_committee(
//        indices=get_active_validator_indices(state, epoch),
//        seed=get_seed(state, epoch, DOMAIN_BEACON_ATTESTER),
//        index=epoch_offset,
//        count=committees_per_slot * SLOTS_PER_EPOCH,
//    )
func BeaconCommittee(state *pb.BeaconState, slot uint64, index uint64) ([]uint64, error) {
	epoch := SlotToEpoch(slot)
	if featureconfig.Get().EnableNewCache {
		seed, err := Seed(state, epoch, params.BeaconConfig().DomainBeaconAttester)
		if err != nil {
			return nil, errors.Wrap(err, "could not get seed")
		}

		indices, err := committeeCache.Committee(slot, seed, index)
		if err != nil {
			return nil, errors.Wrap(err, "could not interface with committee cache")
		}
		if indices != nil {
			return indices, nil
		}
	}

	committeesPerSlot, err := CommitteeCountAtSlot(state, slot)
	if err != nil {
		return nil, errors.Wrap(err, "could not get committee count at slot")
	}
	epochOffset := index + (slot%params.BeaconConfig().SlotsPerEpoch)*committeesPerSlot
	count := committeesPerSlot * params.BeaconConfig().SlotsPerEpoch

	seed, err := Seed(state, epoch, params.BeaconConfig().DomainBeaconAttester)
	if err != nil {
		return nil, errors.Wrap(err, "could not get seed")
	}

	indices, err := ActiveValidatorIndices(state, epoch)
	if err != nil {
		return nil, errors.Wrap(err, "could not get active indices")
	}

	if featureconfig.Get().EnableNewCache {
		if err := UpdateCommitteeCache(state); err != nil {
			return nil, errors.Wrap(err, "could not update committee cache")
		}
	}

	return ComputeCommittee(indices, seed, epochOffset, count)
}

// BeaconCommitteeWithoutCache returns the crosslink committee of a given slot and committee index without the
// usage of committee cache.
// TODO(3603): Delete this function when issue 3603 closes.
func BeaconCommitteeWithoutCache(state *pb.BeaconState, slot uint64, index uint64) ([]uint64, error) {
	epoch := SlotToEpoch(slot)

	committeesPerSlot, err := CommitteeCountAtSlot(state, slot)
	if err != nil {
		return nil, errors.Wrap(err, "could not get committee count at slot")
	}
	epochOffset := index + (slot%params.BeaconConfig().SlotsPerEpoch)*committeesPerSlot
	count := committeesPerSlot * params.BeaconConfig().SlotsPerEpoch

	seed, err := Seed(state, epoch, params.BeaconConfig().DomainBeaconAttester)
	if err != nil {
		return nil, errors.Wrap(err, "could not get seed")
	}

	indices, err := ActiveValidatorIndices(state, epoch)
	if err != nil {
		return nil, errors.Wrap(err, "could not get active indices")
	}

	return ComputeCommittee(indices, seed, epochOffset, count)
}

// ComputeCommittee returns the requested shuffled committee out of the total committees using
// validator indices and seed.
//
// Spec pseudocode definition:
//  def compute_committee(indices: Sequence[ValidatorIndex],
//                      seed: Hash,
//                      index: uint64,
//                      count: uint64) -> Sequence[ValidatorIndex]:
//    """
//    Return the committee corresponding to ``indices``, ``seed``, ``index``, and committee ``count``.
//    """
//    start = (len(indices) * index) // count
//    end = (len(indices) * (index + 1)) // count
//    return [indices[compute_shuffled_index(ValidatorIndex(i), len(indices), seed)] for i in range(start, end)
func ComputeCommittee(
	indices []uint64,
	seed [32]byte,
	index uint64,
	count uint64,
) ([]uint64, error) {
	validatorCount := uint64(len(indices))
	start := sliceutil.SplitOffset(validatorCount, count, index)
	end := sliceutil.SplitOffset(validatorCount, count, index+1)

	// Save the shuffled indices in cache, this is only needed once per epoch or once per new committee index.
	shuffledIndices := make([]uint64, end-start)
	for i := start; i < end; i++ {
		permutedIndex, err := ShuffledIndex(i, validatorCount, seed)
		if err != nil {
			return []uint64{}, errors.Wrapf(err, "could not get shuffled index at index %d", i)
		}
		shuffledIndices[i-start] = indices[permutedIndex]
	}

	return shuffledIndices, nil
}

// AttestingIndices returns the attesting participants indices from the attestation data. The
// committee is provided as an argument rather than a direct implementation from the spec definition.
// Having the committee as an argument allows for re-use of beacon committees when possible.
//
// Spec pseudocode definition:
//   def get_attesting_indices(state: BeaconState,
//                          data: AttestationData,
//                          bits: Bitlist[MAX_VALIDATORS_PER_COMMITTEE]) -> Set[ValidatorIndex]:
//    """
//    Return the set of attesting indices corresponding to ``data`` and ``bits``.
//    """
//    committee = get_beacon_committee(state, data.slot, data.index)
//    return set(index for i, index in enumerate(committee) if bits[i])
func AttestingIndices(bf bitfield.Bitfield, committee []uint64) ([]uint64, error) {
	indices := make([]uint64, 0, len(committee))
	indicesSet := make(map[uint64]bool)
	for i, idx := range committee {
		if !indicesSet[idx] {
			if bf.BitAt(uint64(i)) {
				indices = append(indices, idx)
			}
		}
		indicesSet[idx] = true
	}
	return indices, nil
}

// CommitteeAssignmentContainer represents a committee, index, and attester slot for a given epoch.
type CommitteeAssignmentContainer struct {
	Committee      []uint64
	AttesterSlot   uint64
	CommitteeIndex uint64
}

// CommitteeAssignments is a map of validator indices pointing to the appropriate committee
// assignment for the given epoch.
//
// 1. Determine the proposer validator index for each slot.
// 2. Compute all committees.
// 3. Determine the attesting slot for each committee.
// 4. Construct a map of validator indices pointing to the respective committees.
func CommitteeAssignments(state *pb.BeaconState, epoch uint64) (map[uint64]*CommitteeAssignmentContainer, map[uint64]uint64, error) {
	if epoch > NextEpoch(state) {
		return nil, nil, fmt.Errorf(
			"epoch %d can't be greater than next epoch %d",
			epoch, 
			NextEpoch(state),
		)
	}

	// Track which slot has which proposer.
	startSlot := StartSlot(epoch)
	proposerIndexToSlot := make(map[uint64]uint64)
	for slot := startSlot; slot < startSlot+params.BeaconConfig().SlotsPerEpoch; slot++ {
		state.Slot = slot
		i, err := BeaconProposerIndex(state)
		if err != nil {
			return nil, nil, errors.Wrapf(err, "could not check proposer at slot %d", state.Slot)
		}
		proposerIndexToSlot[i] = slot
	}

	// Each slot in an epoch has a different set of committees. This value is derived from the
	// active validator set, which does not change.
	numCommitteesPerSlot, err := CommitteeCountAtSlot(state, StartSlot(epoch))
	if err != nil {
		return nil, nil, err
	}

	validatorIndexToCommittee := make(map[uint64]*CommitteeAssignmentContainer)

	// Compute all committees for all slots.
	for i := uint64(0); i < params.BeaconConfig().SlotsPerEpoch; i++ {
		// Compute committees.
		for j := uint64(0); j < numCommitteesPerSlot; j++ {
			slot := startSlot + i
			committee, err := BeaconCommittee(state, slot, j /*committee index*/)
			if err != nil {
				return nil, nil, err
			}

			cac := &CommitteeAssignmentContainer{
				Committee:      committee,
				CommitteeIndex: j,
				AttesterSlot:   slot,
			}
			for _, vID := range committee {
				validatorIndexToCommittee[vID] = cac
			}
		}
	}

	return validatorIndexToCommittee, proposerIndexToSlot, nil
}

// CommitteeAssignment is used to query committee assignment from
// current and previous epoch.
//
// Deprecated: Consider using CommitteeAssignments, especially when computing more than one
// validator assignment as this method is O(n^2) in computational complexity. This method exists to
// ensure spec definition conformance and otherwise should probably not be used.
//
// Spec pseudocode definition:
//   def get_committee_assignment(state: BeaconState,
//                             epoch: Epoch,
//                             validator_index: ValidatorIndex
//                             ) -> Optional[Tuple[Sequence[ValidatorIndex], CommitteeIndex, Slot]]:
//    """
//    Return the committee assignment in the ``epoch`` for ``validator_index``.
//    ``assignment`` returned is a tuple of the following form:
//        * ``assignment[0]`` is the list of validators in the committee
//        * ``assignment[1]`` is the index to which the committee is assigned
//        * ``assignment[2]`` is the slot at which the committee is assigned
//    Return None if no assignment.
//    """
//    next_epoch = get_current_epoch(state) + 1
//    assert epoch <= next_epoch
//
//    start_slot = compute_start_slot_at_epoch(epoch)
//    for slot in range(start_slot, start_slot + SLOTS_PER_EPOCH):
//        for index in range(get_committee_count_at_slot(state, Slot(slot))):
//            committee = get_beacon_committee(state, Slot(slot), CommitteeIndex(index))
//            if validator_index in committee:
//                return committee, CommitteeIndex(index), Slot(slot)
//    return None
func CommitteeAssignment(
	state *pb.BeaconState,
	epoch uint64,
	validatorIndex uint64,
) ([]uint64, uint64, uint64, uint64, error) {

	if epoch > NextEpoch(state) {
		return nil, 0, 0, 0, fmt.Errorf(
			"epoch %d can't be greater than next epoch %d",
			epoch, NextEpoch(state))
	}

	// Track which slot has which proposer.
	startSlot := StartSlot(epoch)
	proposerIndexToSlot := make(map[uint64]uint64)
	for slot := startSlot; slot < startSlot+params.BeaconConfig().SlotsPerEpoch; slot++ {
		state.Slot = slot
		i, err := BeaconProposerIndex(state)
		if err != nil {
			return nil, 0, 0, 0, errors.Wrapf(err, "could not check proposer at slot %d", state.Slot)
		}
		proposerIndexToSlot[i] = slot
	}

	for slot := startSlot; slot < startSlot+params.BeaconConfig().SlotsPerEpoch; slot++ {
		countAtSlot, err := CommitteeCountAtSlot(state, slot)
		if err != nil {
			return nil, 0, 0, 0, errors.Wrapf(err, "could not get committee count at slot %d", slot)
		}
		for i := uint64(0); i < countAtSlot; i++ {
			committee, err := BeaconCommittee(state, slot, i)
			if err != nil {
				return nil, 0, 0, 0, errors.Wrapf(err, "could not get crosslink committee at slot %d", slot)
			}
			for _, v := range committee {
				if validatorIndex == v {
					proposerSlot, _ := proposerIndexToSlot[v]
					return committee, i, slot, proposerSlot, nil
				}
			}
		}
	}
	return []uint64{}, 0, 0, 0, fmt.Errorf("validator with index %d not found in assignments", validatorIndex)
}

// VerifyBitfieldLength verifies that a bitfield length matches the given committee size.
func VerifyBitfieldLength(bf bitfield.Bitfield, committeeSize uint64) error {
	if bf.Len() != committeeSize {
		return fmt.Errorf(
			"wanted participants bitfield length %d, got: %d",
			committeeSize,
			bf.Len())
	}
	return nil
}

// VerifyAttestationBitfieldLengths verifies that an attestations aggregation and custody bitfields are
// a valid length matching the size of the committee.
func VerifyAttestationBitfieldLengths(bState *pb.BeaconState, att *ethpb.Attestation) error {
	committee, err := BeaconCommittee(bState, att.Data.Slot, att.Data.CommitteeIndex)
	if err != nil {
		return errors.Wrap(err, "could not retrieve beacon committees")
	}

	if committee == nil {
		return errors.New("no committee exist for this attestation")
	}

	if err := VerifyBitfieldLength(att.AggregationBits, uint64(len(committee))); err != nil {
		return errors.Wrap(err, "failed to verify aggregation bitfield")
	}
	if err := VerifyBitfieldLength(att.CustodyBits, uint64(len(committee))); err != nil {
		return errors.Wrap(err, "failed to verify custody bitfield")
	}
	return nil
}

// ShuffledIndices uses input beacon state and returns the shuffled indices of the input epoch,
// the shuffled indices then can be used to break up into committees.
func ShuffledIndices(state *pb.BeaconState, epoch uint64) ([]uint64, error) {
	seed, err := Seed(state, epoch, params.BeaconConfig().DomainBeaconAttester)
	if err != nil {
		return nil, errors.Wrapf(err, "could not get seed for epoch %d", epoch)
	}

	indices := make([]uint64, 0, len(state.Validators))
	for i, v := range state.Validators {
		if IsActiveValidator(v, epoch) {
			indices = append(indices, uint64(i))
		}
	}

	validatorCount := uint64(len(indices))
	shuffledIndices := make([]uint64, validatorCount)
	for i := 0; i < len(shuffledIndices); i++ {
		permutedIndex, err := ShuffledIndex(uint64(i), validatorCount, seed)
		if err != nil {
			return []uint64{}, errors.Wrapf(err, "could not get shuffled index at index %d", i)
		}
		shuffledIndices[i] = indices[permutedIndex]
	}

	return shuffledIndices, nil
}

// UpdateCommitteeCache gets called at the beginning of every epoch to cache the committee shuffled indices
// list with committee index and epoch number. It caches the shuffled indices for current epoch and next epoch.
func UpdateCommitteeCache(state *pb.BeaconState) error {
	currentEpoch := CurrentEpoch(state)
	for _, epoch := range []uint64{currentEpoch, currentEpoch + 1} {
		shuffledIndices, err := ShuffledIndices(state, epoch)
		if err != nil {
			return err
		}
		count, err := CommitteeCountAtSlot(state, StartSlot(epoch))
		if err != nil {
			return err
		}
		seed, err := Seed(state, epoch, params.BeaconConfig().DomainBeaconAttester)
		if err != nil {
			return err
		}

		// Store the sorted indices as well as shuffled indices. In current spec,
		// sorted indices is required to retrieve proposer index. This is also
		// used for failing verify signature fallback.
		sortedIndices := make([]uint64, len(shuffledIndices))
		copy(sortedIndices, shuffledIndices)
		sort.Slice(sortedIndices, func(i, j int) bool {
			return sortedIndices[i] < sortedIndices[j]
		})

		if err := committeeCache.AddCommitteeShuffledList(&cache.Committees{
			ShuffledIndices: shuffledIndices,
			CommitteeCount:  count * params.BeaconConfig().SlotsPerEpoch,
			Seed:            seed,
			SortedIndices:   sortedIndices,
		}); err != nil {
			return err
		}
	}

	return nil
}
