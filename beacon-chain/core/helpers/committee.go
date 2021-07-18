// Package helpers contains helper functions outlined in the Ethereum Beacon Chain spec, such as
// computing committees, randao, rewards/penalties, and more.
package helpers

import (
	"bytes"
	"fmt"
	"sort"

	"github.com/pkg/errors"
	types "github.com/prysmaticlabs/eth2-types"
	"github.com/prysmaticlabs/go-bitfield"
	"github.com/prysmaticlabs/prysm/beacon-chain/cache"
	iface "github.com/prysmaticlabs/prysm/beacon-chain/state/interface"
	ethpb "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/prysmaticlabs/prysm/shared/hashutil"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/sliceutil"
	log "github.com/sirupsen/logrus"
)

var committeeCache = cache.NewCommitteesCache()
var proposerIndicesCache = cache.NewProposerIndicesCache()
var syncCommitteeCache = cache.NewSyncCommittee()

// SlotCommitteeCount returns the number of crosslink committees of a slot. The
// active validator count is provided as an argument rather than a imported implementation
// from the spec definition. Having the active validator count as an argument allows for
// cheaper computation, instead of retrieving head state, one can retrieve the validator
// count.
//
//
// Spec pseudocode definition:
//   def get_committee_count_per_slot(state: BeaconState, epoch: Epoch) -> uint64:
//    """
//    Return the number of committees in each slot for the given ``epoch``.
//    """
//    return max(uint64(1), min(
//        MAX_COMMITTEES_PER_SLOT,
//        uint64(len(get_active_validator_indices(state, epoch))) // SLOTS_PER_EPOCH // TARGET_COMMITTEE_SIZE,
//    ))
func SlotCommitteeCount(activeValidatorCount uint64) uint64 {
	var committeePerSlot = activeValidatorCount / uint64(params.BeaconConfig().SlotsPerEpoch) / params.BeaconConfig().TargetCommitteeSize

	if committeePerSlot > params.BeaconConfig().MaxCommitteesPerSlot {
		return params.BeaconConfig().MaxCommitteesPerSlot
	}
	if committeePerSlot == 0 {
		return 1
	}

	return committeePerSlot
}

// BeaconCommitteeFromState returns the crosslink committee of a given slot and committee index. This
// is a spec implementation where state is used as an argument. In case of state retrieval
// becomes expensive, consider using BeaconCommittee below.
//
// Spec pseudocode definition:
//   def get_beacon_committee(state: BeaconState, slot: Slot, index: CommitteeIndex) -> Sequence[ValidatorIndex]:
//    """
//    Return the beacon committee at ``slot`` for ``index``.
//    """
//    epoch = compute_epoch_at_slot(slot)
//    committees_per_slot = get_committee_count_per_slot(state, epoch)
//    return compute_committee(
//        indices=get_active_validator_indices(state, epoch),
//        seed=get_seed(state, epoch, DOMAIN_BEACON_ATTESTER),
//        index=(slot % SLOTS_PER_EPOCH) * committees_per_slot + index,
//        count=committees_per_slot * SLOTS_PER_EPOCH,
//    )
func BeaconCommitteeFromState(state iface.ReadOnlyBeaconState, slot types.Slot, committeeIndex types.CommitteeIndex) ([]types.ValidatorIndex, error) {
	epoch := SlotToEpoch(slot)
	seed, err := Seed(state, epoch, params.BeaconConfig().DomainBeaconAttester)
	if err != nil {
		return nil, errors.Wrap(err, "could not get seed")
	}

	indices, err := committeeCache.Committee(slot, seed, committeeIndex)
	if err != nil {
		return nil, errors.Wrap(err, "could not interface with committee cache")
	}
	if indices != nil {
		return indices, nil
	}

	activeIndices, err := ActiveValidatorIndices(state, epoch)
	if err != nil {
		return nil, errors.Wrap(err, "could not get active indices")
	}

	return BeaconCommittee(activeIndices, seed, slot, committeeIndex)
}

// BeaconCommittee returns the crosslink committee of a given slot and committee index. The
// validator indices and seed are provided as an argument rather than a imported implementation
// from the spec definition. Having them as an argument allows for cheaper computation run time.
func BeaconCommittee(
	validatorIndices []types.ValidatorIndex,
	seed [32]byte,
	slot types.Slot,
	committeeIndex types.CommitteeIndex,
) ([]types.ValidatorIndex, error) {
	indices, err := committeeCache.Committee(slot, seed, committeeIndex)
	if err != nil {
		return nil, errors.Wrap(err, "could not interface with committee cache")
	}
	if indices != nil {
		return indices, nil
	}

	committeesPerSlot := SlotCommitteeCount(uint64(len(validatorIndices)))

	epochOffset := uint64(committeeIndex) + uint64(slot.ModSlot(params.BeaconConfig().SlotsPerEpoch).Mul(committeesPerSlot))
	count := committeesPerSlot * uint64(params.BeaconConfig().SlotsPerEpoch)

	return ComputeCommittee(validatorIndices, seed, epochOffset, count)
}

// ComputeCommittee returns the requested shuffled committee out of the total committees using
// validator indices and seed.
//
// Spec pseudocode definition:
//  def compute_committee(indices: Sequence[ValidatorIndex],
//                      seed: Bytes32,
//                      index: uint64,
//                      count: uint64) -> Sequence[ValidatorIndex]:
//    """
//    Return the committee corresponding to ``indices``, ``seed``, ``index``, and committee ``count``.
//    """
//    start = (len(indices) * index) // count
//    end = (len(indices) * uint64(index + 1)) // count
//    return [indices[compute_shuffled_index(uint64(i), uint64(len(indices)), seed)] for i in range(start, end)]
func ComputeCommittee(
	indices []types.ValidatorIndex,
	seed [32]byte,
	index, count uint64,
) ([]types.ValidatorIndex, error) {
	validatorCount := uint64(len(indices))
	start := sliceutil.SplitOffset(validatorCount, count, index)
	end := sliceutil.SplitOffset(validatorCount, count, index+1)

	if start > validatorCount || end > validatorCount {
		return nil, errors.New("index out of range")
	}

	// Save the shuffled indices in cache, this is only needed once per epoch or once per new committee index.
	shuffledIndices := make([]types.ValidatorIndex, len(indices))
	copy(shuffledIndices, indices)
	// UnshuffleList is used here as it is an optimized implementation created
	// for fast computation of committees.
	// Reference implementation: https://github.com/protolambda/eth2-shuffle
	shuffledList, err := UnshuffleList(shuffledIndices, seed)
	if err != nil {
		return nil, err
	}

	return shuffledList[start:end], nil
}

// CommitteeAssignmentContainer represents a committee, index, and attester slot for a given epoch.
type CommitteeAssignmentContainer struct {
	Committee      []types.ValidatorIndex
	AttesterSlot   types.Slot
	CommitteeIndex types.CommitteeIndex
}

// CommitteeAssignments is a map of validator indices pointing to the appropriate committee
// assignment for the given epoch.
//
// 1. Determine the proposer validator index for each slot.
// 2. Compute all committees.
// 3. Determine the attesting slot for each committee.
// 4. Construct a map of validator indices pointing to the respective committees.
func CommitteeAssignments(
	state iface.BeaconState,
	epoch types.Epoch,
) (map[types.ValidatorIndex]*CommitteeAssignmentContainer, map[types.ValidatorIndex][]types.Slot, error) {
	nextEpoch := NextEpoch(state)
	if epoch > nextEpoch {
		return nil, nil, fmt.Errorf(
			"epoch %d can't be greater than next epoch %d",
			epoch,
			nextEpoch,
		)
	}

	// We determine the slots in which proposers are supposed to act.
	// Some validators may need to propose multiple times per epoch, so
	// we use a map of proposer idx -> []slot to keep track of this possibility.
	startSlot, err := StartSlot(epoch)
	if err != nil {
		return nil, nil, err
	}
	proposerIndexToSlots := make(map[types.ValidatorIndex][]types.Slot, params.BeaconConfig().SlotsPerEpoch)
	// Proposal epochs do not have a look ahead, so we skip them over here.
	validProposalEpoch := epoch < nextEpoch
	for slot := startSlot; slot < startSlot+params.BeaconConfig().SlotsPerEpoch && validProposalEpoch; slot++ {
		// Skip proposer assignment for genesis slot.
		if slot == 0 {
			continue
		}
		if err := state.SetSlot(slot); err != nil {
			return nil, nil, err
		}
		i, err := BeaconProposerIndex(state)
		if err != nil {
			return nil, nil, errors.Wrapf(err, "could not check proposer at slot %d", state.Slot())
		}
		proposerIndexToSlots[i] = append(proposerIndexToSlots[i], slot)
	}

	activeValidatorIndices, err := ActiveValidatorIndices(state, epoch)
	if err != nil {
		return nil, nil, err
	}
	// Each slot in an epoch has a different set of committees. This value is derived from the
	// active validator set, which does not change.
	numCommitteesPerSlot := SlotCommitteeCount(uint64(len(activeValidatorIndices)))
	validatorIndexToCommittee := make(map[types.ValidatorIndex]*CommitteeAssignmentContainer, params.BeaconConfig().SlotsPerEpoch.Mul(numCommitteesPerSlot))

	// Compute all committees for all slots.
	for i := types.Slot(0); i < params.BeaconConfig().SlotsPerEpoch; i++ {
		// Compute committees.
		for j := uint64(0); j < numCommitteesPerSlot; j++ {
			slot := startSlot + i
			committee, err := BeaconCommitteeFromState(state, slot, types.CommitteeIndex(j) /*committee index*/)
			if err != nil {
				return nil, nil, err
			}

			cac := &CommitteeAssignmentContainer{
				Committee:      committee,
				CommitteeIndex: types.CommitteeIndex(j),
				AttesterSlot:   slot,
			}
			for _, vIndex := range committee {
				validatorIndexToCommittee[vIndex] = cac
			}
		}
	}

	return validatorIndexToCommittee, proposerIndexToSlots, nil
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

// VerifyAttestationBitfieldLengths verifies that an attestations aggregation bitfields is
// a valid length matching the size of the committee.
func VerifyAttestationBitfieldLengths(state iface.ReadOnlyBeaconState, att *ethpb.Attestation) error {
	committee, err := BeaconCommitteeFromState(state, att.Data.Slot, att.Data.CommitteeIndex)
	if err != nil {
		return errors.Wrap(err, "could not retrieve beacon committees")
	}

	if committee == nil {
		return errors.New("no committee exist for this attestation")
	}

	if err := VerifyBitfieldLength(att.AggregationBits, uint64(len(committee))); err != nil {
		return errors.Wrap(err, "failed to verify aggregation bitfield")
	}
	return nil
}

// Returns the active indices and the total active balance of the validators in input `state` and during input `epoch`.
func activeIndicesAndBalance(state iface.ReadOnlyBeaconState, epoch types.Epoch) ([]types.ValidatorIndex, uint64, error) {
	balances := uint64(0)
	indices := make([]types.ValidatorIndex, 0, state.NumValidators())
	if err := state.ReadFromEveryValidator(func(idx int, val iface.ReadOnlyValidator) error {
		if IsActiveValidatorUsingTrie(val, epoch) {
			balances += val.EffectiveBalance()
			indices = append(indices, types.ValidatorIndex(idx))
		}
		return nil
	}); err != nil {
		return nil, 0, err
	}

	return indices, balances, nil
}

// UpdateCommitteeCache gets called at the beginning of every epoch to cache the committee shuffled indices
// list with committee index and epoch number. It caches the shuffled indices for current epoch and next epoch.
func UpdateCommitteeCache(state iface.ReadOnlyBeaconState, epoch types.Epoch) error {
	for _, e := range []types.Epoch{epoch, epoch + 1} {
		seed, err := Seed(state, e, params.BeaconConfig().DomainBeaconAttester)
		if err != nil {
			return err
		}

		if committeeCache.HasEntry(string(seed[:])) {
			return nil
		}

		indices, balance, err := activeIndicesAndBalance(state, e)
		if err != nil {
			return err
		}

		// Get the shuffled indices based on the seed.
		shuffledIndices, err := UnshuffleList(indices, seed)
		if err != nil {
			return err
		}

		count := SlotCommitteeCount(uint64(len(shuffledIndices)))

		// Store the sorted indices as well as shuffled indices. In current spec,
		// sorted indices is required to retrieve proposer index. This is also
		// used for failing verify signature fallback.
		sortedIndices := make([]types.ValidatorIndex, len(shuffledIndices))
		copy(sortedIndices, shuffledIndices)
		sort.Slice(sortedIndices, func(i, j int) bool {
			return sortedIndices[i] < sortedIndices[j]
		})

		// Only update active balance field in cache if it's current epoch.
		// Using current epoch state to update next epoch field will cause insert an invalid
		// active balance value.
		b := &cache.Balance{}
		if e == epoch {
			b = &cache.Balance{
				Exist: true,
				Total: balance,
			}
		}

		if err := committeeCache.AddCommitteeShuffledList(&cache.Committees{
			ShuffledIndices: shuffledIndices,
			CommitteeCount:  uint64(params.BeaconConfig().SlotsPerEpoch.Mul(count)),
			Seed:            seed,
			SortedIndices:   sortedIndices,
			ActiveBalance:   b,
		}); err != nil {
			return err
		}
	}

	return nil
}

// UpdateProposerIndicesInCache updates proposer indices entry of the committee cache.
func UpdateProposerIndicesInCache(state iface.ReadOnlyBeaconState) error {
	// The cache uses the state root at the (current epoch - 2)'s slot as key. (e.g. for epoch 2, the key is root at slot 31)
	// Which is the reason why we skip genesis epoch.
	if CurrentEpoch(state) <= params.BeaconConfig().GenesisEpoch+params.BeaconConfig().MinSeedLookahead {
		return nil
	}

	// Use state root from (current_epoch - 1 - lookahead))
	wantedEpoch := CurrentEpoch(state) - 1
	if wantedEpoch >= params.BeaconConfig().MinSeedLookahead {
		wantedEpoch -= params.BeaconConfig().MinSeedLookahead
	}
	s, err := EndSlot(wantedEpoch)
	if err != nil {
		return err
	}
	r, err := StateRootAtSlot(state, s)
	if err != nil {
		return err
	}
	// Skip cache update if we have an invalid key
	if r == nil || bytes.Equal(r, params.BeaconConfig().ZeroHash[:]) {
		return nil
	}
	// Skip cache update if the key already exists
	exists, err := proposerIndicesCache.HasProposerIndices(bytesutil.ToBytes32(r))
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	indices, err := ActiveValidatorIndices(state, CurrentEpoch(state))
	if err != nil {
		return err
	}
	proposerIndices, err := precomputeProposerIndices(state, indices)
	if err != nil {
		return err
	}
	return proposerIndicesCache.AddProposerIndices(&cache.ProposerIndices{
		BlockRoot:       bytesutil.ToBytes32(r),
		ProposerIndices: proposerIndices,
	})
}

// ClearCache clears the beacon committee cache and sync committee cache.
func ClearCache() {
	committeeCache = cache.NewCommitteesCache()
	proposerIndicesCache = cache.NewProposerIndicesCache()
	syncCommitteeCache = cache.NewSyncCommittee()
}

// IsCurrentEpochSyncCommittee returns true if the input validator index belongs in the current epoch sync committee
// along with the sync committee root.
// 1.) Checks if the public key exists in the sync committee cache
// 2.) If 1 fails, checks if the public key exists in the input current sync committee object
func IsCurrentEpochSyncCommittee(
	st iface.BeaconStateAltair, valIdx types.ValidatorIndex,
) (bool, error) {
	root, err := syncPeriodBoundaryRoot(st)
	if err != nil {
		return false, err
	}
	indices, err := syncCommitteeCache.CurrentEpochIndexPosition(bytesutil.ToBytes32(root), valIdx)
	if err == cache.ErrNonExistingSyncCommitteeKey {
		val, err := st.ValidatorAtIndex(valIdx)
		if err != nil {
			return false, nil
		}
		committee, err := st.CurrentSyncCommittee()
		if err != nil {
			return false, err
		}

		// Fill in the cache on miss.
		go func() {
			if err := syncCommitteeCache.UpdatePositionsInCommittee(bytesutil.ToBytes32(root), st); err != nil {
				log.Errorf("Could not fill sync committee cache on miss: %v", err)
			}
		}()

		return len(findSubCommitteeIndices(val.PublicKey, committee.Pubkeys)) > 0, nil
	}
	if err != nil {
		return false, err
	}
	return len(indices) > 0, nil
}

// IsNextEpochSyncCommittee returns true if the input validator index belongs in the next epoch sync committee
// along with the sync committee root.
// 1.) Checks if the public key exists in the sync committee cache
// 2.) If 1 fails, checks if the public key exists in the input next sync committee object
func IsNextEpochSyncCommittee(
	st iface.BeaconStateAltair, valIdx types.ValidatorIndex,
) (bool, error) {
	root, err := syncPeriodBoundaryRoot(st)
	if err != nil {
		return false, err
	}
	indices, err := syncCommitteeCache.NextEpochIndexPosition(bytesutil.ToBytes32(root), valIdx)
	if err == cache.ErrNonExistingSyncCommitteeKey {
		val, err := st.ValidatorAtIndex(valIdx)
		if err != nil {
			return false, nil
		}
		committee, err := st.NextSyncCommittee()
		if err != nil {
			return false, err
		}
		return len(findSubCommitteeIndices(val.PublicKey, committee.Pubkeys)) > 0, nil
	}
	if err != nil {
		return false, err
	}
	return len(indices) > 0, nil
}

// CurrentEpochSyncSubcommitteeIndices returns the subcommittee indices of the
// current epoch sync committee for input validator.
func CurrentEpochSyncSubcommitteeIndices(
	st iface.BeaconStateAltair, valIdx types.ValidatorIndex,
) ([]uint64, error) {
	root, err := syncPeriodBoundaryRoot(st)
	if err != nil {
		return nil, err
	}
	indices, err := syncCommitteeCache.CurrentEpochIndexPosition(bytesutil.ToBytes32(root), valIdx)
	if err == cache.ErrNonExistingSyncCommitteeKey {
		val, err := st.ValidatorAtIndex(valIdx)
		if err != nil {
			return nil, nil
		}
		committee, err := st.CurrentSyncCommittee()
		if err != nil {
			return nil, err
		}

		// Fill in the cache on miss.
		go func() {
			if err := syncCommitteeCache.UpdatePositionsInCommittee(bytesutil.ToBytes32(root), st); err != nil {
				log.Errorf("Could not fill sync committee cache on miss: %v", err)
			}
		}()

		return findSubCommitteeIndices(val.PublicKey, committee.Pubkeys), nil
	}
	if err != nil {
		return nil, err
	}
	return indices, nil
}

// NextEpochSyncSubcommitteeIndices returns the subcommittee indices of the next epoch sync committee for input validator.
func NextEpochSyncSubcommitteeIndices(
	st iface.BeaconStateAltair, valIdx types.ValidatorIndex,
) ([]uint64, error) {
	root, err := syncPeriodBoundaryRoot(st)
	if err != nil {
		return nil, err
	}
	indices, err := syncCommitteeCache.NextEpochIndexPosition(bytesutil.ToBytes32(root), valIdx)
	if err == cache.ErrNonExistingSyncCommitteeKey {
		val, err := st.ValidatorAtIndex(valIdx)
		if err != nil {
			return nil, nil
		}
		committee, err := st.NextSyncCommittee()
		if err != nil {
			return nil, err
		}
		return findSubCommitteeIndices(val.PublicKey, committee.Pubkeys), nil
	}
	if err != nil {
		return nil, err
	}
	return indices, nil
}

// UpdateSyncCommitteeCache updates sync committee cache.
// It uses `state`'s latest block header root as key. To avoid miss usage, it disallows
// block header with state root zeroed out.
func UpdateSyncCommitteeCache(state iface.BeaconStateAltair) error {
	nextSlot := state.Slot() + 1
	if nextSlot%params.BeaconConfig().SlotsPerEpoch != 0 {
		return errors.New("not at the end of the epoch to update cache")
	}
	if SlotToEpoch(nextSlot)%params.BeaconConfig().EpochsPerSyncCommitteePeriod != 0 {
		return errors.New("not at sync committee period boundary to update cache")
	}

	header := state.LatestBlockHeader()
	if bytes.Equal(header.StateRoot, params.BeaconConfig().ZeroHash[:]) {
		return errors.New("zero hash state root can't be used to update cache")
	}

	prevBlockRoot, err := header.HashTreeRoot()
	if err != nil {
		return err
	}

	return syncCommitteeCache.UpdatePositionsInCommittee(prevBlockRoot, state)
}

// Loop through `pubKeys` for matching `pubKey` and get the indices where it matches.
func findSubCommitteeIndices(pubKey []byte, pubKeys [][]byte) []uint64 {
	var indices []uint64
	for i, k := range pubKeys {
		if bytes.Equal(k, pubKey) {
			indices = append(indices, uint64(i))
		}
	}
	return indices
}

// Retrieve the current sync period boundary root by calculating sync period start epoch
// and calling `BlockRoot`.
// It uses the boundary slot - 1 for block root. (Ex: SlotsPerEpoch * EpochsPerSyncCommitteePeriod - 1)
func syncPeriodBoundaryRoot(state iface.ReadOnlyBeaconState) ([]byte, error) {
	// Can't call `BlockRoot` until the first slot.
	if state.Slot() == params.BeaconConfig().GenesisSlot {
		return params.BeaconConfig().ZeroHash[:], nil
	}

	startEpoch, err := SyncCommitteePeriodStartEpoch(CurrentEpoch(state))
	if err != nil {
		return nil, err
	}
	startEpochSlot, err := StartSlot(startEpoch)
	if err != nil {
		return nil, err
	}

	// Prevent underflow
	if startEpochSlot >= 1 {
		startEpochSlot--
	}

	return BlockRootAtSlot(state, startEpochSlot)
}

// This computes proposer indices of the current epoch and returns a list of proposer indices,
// the index of the list represents the slot number.
func precomputeProposerIndices(state iface.ReadOnlyBeaconState, activeIndices []types.ValidatorIndex) ([]types.ValidatorIndex, error) {
	hashFunc := hashutil.CustomSHA256Hasher()
	proposerIndices := make([]types.ValidatorIndex, params.BeaconConfig().SlotsPerEpoch)

	e := CurrentEpoch(state)
	seed, err := Seed(state, e, params.BeaconConfig().DomainBeaconProposer)
	if err != nil {
		return nil, errors.Wrap(err, "could not generate seed")
	}
	slot, err := StartSlot(e)
	if err != nil {
		return nil, err
	}
	for i := uint64(0); i < uint64(params.BeaconConfig().SlotsPerEpoch); i++ {
		seedWithSlot := append(seed[:], bytesutil.Bytes8(uint64(slot)+i)...)
		seedWithSlotHash := hashFunc(seedWithSlot)
		index, err := ComputeProposerIndex(state, activeIndices, seedWithSlotHash)
		if err != nil {
			return nil, err
		}
		proposerIndices[i] = index
	}

	return proposerIndices, nil
}
