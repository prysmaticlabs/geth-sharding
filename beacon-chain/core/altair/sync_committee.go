package altair

import (
	"bytes"
	"fmt"

	"github.com/pkg/errors"
	types "github.com/prysmaticlabs/eth2-types"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	iface "github.com/prysmaticlabs/prysm/beacon-chain/state/interface"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	prysmv2 "github.com/prysmaticlabs/prysm/proto/prysm/v2"
	"github.com/prysmaticlabs/prysm/shared/bls"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/prysmaticlabs/prysm/shared/hashutil"
	"github.com/prysmaticlabs/prysm/shared/mathutil"
	"github.com/prysmaticlabs/prysm/shared/params"
)

// ValidateNilSyncContribution validates that the signed contribution
// is not nil.
func ValidateNilSyncContribution(s *prysmv2.SignedContributionAndProof) error {
	if s == nil {
		return errors.New("signed message can't be nil")
	}
	if s.Message == nil {
		return errors.New("signed contribution's message can't be nil")
	}
	if s.Message.Contribution == nil {
		return errors.New("inner contribution can't be nil")
	}
	if s.Message.Contribution.AggregationBits == nil {
		return errors.New("contribution's bitfield can't be nil")
	}
	return nil
}

// NextSyncCommittee returns the next sync committee for a given state.
//
// Spec code:
// def get_next_sync_committee(state: BeaconState) -> SyncCommittee:
//    """
//    Return the next sync committee, with possible pubkey duplicates.
//    """
//    indices = get_next_sync_committee_indices(state)
//    pubkeys = [state.validators[index].pubkey for index in indices]
//    aggregate_pubkey = bls.AggregatePKs(pubkeys)
//    return SyncCommittee(pubkeys=pubkeys, aggregate_pubkey=aggregate_pubkey)
func NextSyncCommittee(state iface.BeaconStateAltair) (*pb.SyncCommittee, error) {
	indices, err := NextSyncCommitteeIndices(state)
	if err != nil {
		return nil, err
	}
	pubkeys := make([][]byte, params.BeaconConfig().SyncCommitteeSize)
	for i, index := range indices {
		p := state.PubkeyAtIndex(index) // Using ReadOnlyValidators interface. No copy here.
		pubkeys[i] = p[:]
	}
	aggregated, err := bls.AggregatePublicKeys(pubkeys)
	if err != nil {
		return nil, err
	}
	return &pb.SyncCommittee{
		Pubkeys:         pubkeys,
		AggregatePubkey: aggregated.Marshal(),
	}, nil
}

// NextSyncCommitteeIndices returns the next sync committee indices for a given state.
//
// Spec code:
// def get_next_sync_committee_indices(state: BeaconState) -> Sequence[ValidatorIndex]:
//    """
//    Return the sync committee indices, with possible duplicates, for the next sync committee.
//    """
//    epoch = Epoch(get_current_epoch(state) + 1)
//
//    MAX_RANDOM_BYTE = 2**8 - 1
//    active_validator_indices = get_active_validator_indices(state, epoch)
//    active_validator_count = uint64(len(active_validator_indices))
//    seed = get_seed(state, epoch, DOMAIN_SYNC_COMMITTEE)
//    i = 0
//    sync_committee_indices: List[ValidatorIndex] = []
//    while len(sync_committee_indices) < SYNC_COMMITTEE_SIZE:
//        shuffled_index = compute_shuffled_index(uint64(i % active_validator_count), active_validator_count, seed)
//        candidate_index = active_validator_indices[shuffled_index]
//        random_byte = hash(seed + uint_to_bytes(uint64(i // 32)))[i % 32]
//        effective_balance = state.validators[candidate_index].effective_balance
//        if effective_balance * MAX_RANDOM_BYTE >= MAX_EFFECTIVE_BALANCE * random_byte:
//            sync_committee_indices.append(candidate_index)
//        i += 1
//    return sync_committee_indices
func NextSyncCommitteeIndices(state iface.BeaconStateAltair) ([]types.ValidatorIndex, error) {
	epoch := helpers.NextEpoch(state)
	indices, err := helpers.ActiveValidatorIndices(state, epoch)
	if err != nil {
		return nil, err
	}
	count := uint64(len(indices))
	seed, err := helpers.Seed(state, epoch, params.BeaconConfig().DomainSyncCommittee)
	if err != nil {
		return nil, err
	}
	i := types.ValidatorIndex(0)
	cIndices := make([]types.ValidatorIndex, 0, params.BeaconConfig().SyncCommitteeSize)
	hashFunc := hashutil.CustomSHA256Hasher()
	maxRandomByte := uint64(1<<8 - 1)
	for uint64(len(cIndices)) < params.BeaconConfig().SyncCommitteeSize {
		sIndex, err := helpers.ComputeShuffledIndex(i.Mod(count), count, seed, true)
		if err != nil {
			return nil, err
		}

		b := append(seed[:], bytesutil.Bytes8(uint64(i.Div(32)))...)
		randomByte := hashFunc(b)[i%32]
		cIndex := indices[sIndex]
		v, err := state.ValidatorAtIndexReadOnly(cIndex)
		if err != nil {
			return nil, err
		}

		effectiveBal := v.EffectiveBalance()
		if effectiveBal*maxRandomByte >= params.BeaconConfig().MaxEffectiveBalance*uint64(randomByte) {
			cIndices = append(cIndices, cIndex)
		}
		i++
	}
	return cIndices, nil
}

// AssignedToSyncCommittee returns true if input validator `i` is assigned to a sync committee at epoch `e`.
//
// Spec code:
// def is_assigned_to_sync_committee(state: BeaconState,
//                                  epoch: Epoch,
//                                  validator_index: ValidatorIndex) -> bool:
//    sync_committee_period = compute_sync_committee_period(epoch)
//    current_epoch = get_current_epoch(state)
//    current_sync_committee_period = compute_sync_committee_period(current_epoch)
//    next_sync_committee_period = current_sync_committee_period + 1
//    assert sync_committee_period in (current_sync_committee_period, next_sync_committee_period)
//
//    pubkey = state.validators[validator_index].pubkey
//    if sync_committee_period == current_sync_committee_period:
//        return pubkey in state.current_sync_committee.pubkeys
//    else:  # sync_committee_period == next_sync_committee_period
//        return pubkey in state.next_sync_committee.pubkeys
func AssignedToSyncCommittee(
	state iface.BeaconStateAltair,
	epoch types.Epoch,
	i types.ValidatorIndex,
) (bool, error) {
	p := helpers.SyncCommitteePeriod(epoch)
	currentEpoch := helpers.CurrentEpoch(state)
	currentPeriod := helpers.SyncCommitteePeriod(currentEpoch)
	nextEpoch := currentPeriod + 1

	if p != currentPeriod && p != nextEpoch {
		return false, fmt.Errorf("epoch period %d is not current period %d or next period %d in state", p, currentEpoch, nextEpoch)
	}

	v, err := state.ValidatorAtIndexReadOnly(i)
	if err != nil {
		return false, err
	}
	vPubKey := v.PublicKey()

	hasKey := func(c *pb.SyncCommittee, k [48]byte) (bool, error) {
		for _, p := range c.Pubkeys {
			if bytes.Equal(vPubKey[:], p) {
				return true, nil
			}
		}
		return false, nil
	}

	if p == currentPeriod {
		c, err := state.CurrentSyncCommittee()
		if err != nil {
			return false, err
		}
		return hasKey(c, vPubKey)
	}

	c, err := state.NextSyncCommittee()
	if err != nil {
		return false, err
	}
	return hasKey(c, vPubKey)
}

// SubnetsForSyncCommittee returns subnet number of what validator `i` belongs to.
//
// Spec code:
// def compute_subnets_for_sync_committee(state: BeaconState, validator_index: ValidatorIndex) -> Sequence[uint64]:
//    next_slot_epoch = compute_epoch_at_slot(Slot(state.slot + 1))
//    if compute_sync_committee_period(get_current_epoch(state)) == compute_sync_committee_period(next_slot_epoch):
//        sync_committee = state.current_sync_committee
//    else:
//        sync_committee = state.next_sync_committee
//
//    target_pubkey = state.validators[validator_index].pubkey
//    sync_committee_indices = [index for index, pubkey in enumerate(sync_committee.pubkeys) if pubkey == target_pubkey]
//    return [
//        uint64(index // (SYNC_COMMITTEE_SIZE // SYNC_COMMITTEE_SUBNET_COUNT))
//        for index in sync_committee_indices
//    ]
func SubnetsForSyncCommittee(state iface.BeaconStateAltair, i types.ValidatorIndex) ([]uint64, error) {
	committee, err := state.NextSyncCommittee()
	if err != nil {
		return nil, err
	}

	nextSlotEpoch := helpers.SlotToEpoch(state.Slot() + 1)
	if helpers.SyncCommitteePeriod(nextSlotEpoch) == helpers.SyncCommitteePeriod(helpers.CurrentEpoch(state)) {
		committee, err = state.CurrentSyncCommittee()
		if err != nil {
			return nil, err
		}
	}

	v, err := state.ValidatorAtIndexReadOnly(i)
	if err != nil {
		return nil, err
	}
	vPubKey := v.PublicKey()
	return SubnetsFromCommittee(vPubKey[:], committee), nil
}

// SubnetsFromCommittee retrieves the relevant subnets for the chosen validator.
func SubnetsFromCommittee(pubkey []byte, comm *pb.SyncCommittee) []uint64 {
	positions := make([]uint64, 0)
	for i, pkey := range comm.Pubkeys {
		if bytes.Equal(pubkey, pkey) {
			positions = append(positions, uint64(i)/(params.BeaconConfig().SyncCommitteeSize/params.BeaconConfig().SyncCommitteeSubnetCount))
		}
	}
	return positions
}

// SyncSubCommitteePubkeys returns the pubkeys participating in a sync subcommittee.
//
// def get_sync_subcommittee_pubkeys(state: BeaconState, subcommittee_index: uint64) -> Sequence[BLSPubkey]:
//    # Committees assigned to `slot` sign for `slot - 1`
//    # This creates the exceptional logic below when transitioning between sync committee periods
//    next_slot_epoch = compute_epoch_at_slot(Slot(state.slot + 1))
//    if compute_sync_committee_period(get_current_epoch(state)) == compute_sync_committee_period(next_slot_epoch):
//        sync_committee = state.current_sync_committee
//    else:
//        sync_committee = state.next_sync_committee
//
//    # Return pubkeys for the subcommittee index
//    sync_subcommittee_size = SYNC_COMMITTEE_SIZE // SYNC_COMMITTEE_SUBNET_COUNT
//    i = subcommittee_index * sync_subcommittee_size
//    return sync_committee.pubkeys[i:i + sync_subcommittee_size]
func SyncSubCommitteePubkeys(st iface.BeaconStateAltair, subComIdx types.CommitteeIndex) ([][]byte, error) {
	nextSlotEpoch := helpers.SlotToEpoch(st.Slot() + 1)
	currEpoch := helpers.SlotToEpoch(st.Slot())

	var syncCommittee *pb.SyncCommittee
	var err error
	if helpers.SyncCommitteePeriod(currEpoch) == helpers.SyncCommitteePeriod(nextSlotEpoch) {
		syncCommittee, err = st.CurrentSyncCommittee()
		if err != nil {
			return nil, err
		}
	} else {
		syncCommittee, err = st.NextSyncCommittee()
		if err != nil {
			return nil, err
		}
	}
	subCommSize := params.BeaconConfig().SyncCommitteeSize / params.BeaconConfig().SyncCommitteeSubnetCount
	i := uint64(subComIdx) * subCommSize
	endOfSubCom := i + subCommSize
	pubkeyLen := uint64(len(syncCommittee.Pubkeys))
	if endOfSubCom > pubkeyLen {
		return nil, errors.Errorf("end index is larger than array length: %d > %d", endOfSubCom, pubkeyLen)
	}
	return syncCommittee.Pubkeys[i:endOfSubCom], nil
}

// IsSyncCommitteeAggregator checks whether the provided signature is for a valid
// aggregator.
// def is_sync_committee_aggregator(signature: BLSSignature) -> bool:
//    modulo = max(1, SYNC_COMMITTEE_SIZE // SYNC_COMMITTEE_SUBNET_COUNT // TARGET_AGGREGATORS_PER_SYNC_SUBCOMMITTEE)
//    return bytes_to_uint64(hash(signature)[0:8]) % modulo == 0
func IsSyncCommitteeAggregator(sig []byte) bool {
	modulo := mathutil.Max(1, params.BeaconConfig().SyncCommitteeSize/params.BeaconConfig().SyncCommitteeSubnetCount/params.BeaconConfig().TargetAggregatorsPerSyncSubcommittee)
	hashedSig := hashutil.Hash(sig)
	return bytesutil.FromBytes8(hashedSig[:8])%modulo == 0
}

// SyncSelectionProofSigningRoot returns the signing root from the relevant provided data.
//
// def get_sync_committee_selection_proof(state: BeaconState,
//                                       slot: Slot,
//                                       subcommittee_index: uint64,
//                                       privkey: int) -> BLSSignature:
//    domain = get_domain(state, DOMAIN_SYNC_COMMITTEE_SELECTION_PROOF, compute_epoch_at_slot(slot))
//    signing_data = SyncAggregatorSelectionData(
//        slot=slot,
//        subcommittee_index=subcommittee_index,
//    )
//    signing_root = compute_signing_root(signing_data, domain)
//    return bls.Sign(privkey, signing_root)
func SyncSelectionProofSigningRoot(st iface.BeaconState, slot types.Slot, comIdx types.CommitteeIndex) ([32]byte, error) {
	dom, err := helpers.Domain(st.Fork(), helpers.SlotToEpoch(slot), params.BeaconConfig().DomainSyncCommitteeSelectionProof, st.GenesisValidatorRoot())
	if err != nil {
		return [32]byte{}, err
	}
	selectionData := &pb.SyncAggregatorSelectionData{Slot: slot, SubcommitteeIndex: uint64(comIdx)}
	return helpers.ComputeSigningRoot(selectionData, dom)
}

// VerifySyncSelectionData verifies that the provided sync contribution has a valid
// selection proof.
func VerifySyncSelectionData(st iface.BeaconState, m *prysmv2.ContributionAndProof) error {
	selectionData := &pb.SyncAggregatorSelectionData{Slot: m.Contribution.Slot, SubcommitteeIndex: uint64(m.Contribution.SubcommitteeIndex)}
	return helpers.ComputeDomainVerifySigningRoot(st, m.AggregatorIndex, helpers.SlotToEpoch(m.Contribution.Slot), selectionData, params.BeaconConfig().DomainSyncCommitteeSelectionProof, m.SelectionProof)
}
