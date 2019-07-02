// Package epoch contains epoch processing libraries. These libraries
// process new balance for the validators, justify and finalize new
// check points, shuffle and reassign validators to different slots and
// shards.
package epoch

import (
	"bytes"
	"errors"
	"fmt"
	"sort"

	"github.com/gogo/protobuf/proto"
	"github.com/prysmaticlabs/go-ssz"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/validators"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	"github.com/prysmaticlabs/prysm/shared/mathutil"
	"github.com/prysmaticlabs/prysm/shared/params"
)

// MatchedAttestations is an object that contains the correctly
// voted attestations based on source, target and head criteria.
type MatchedAttestations struct {
	source []*pb.PendingAttestation
	Target []*pb.PendingAttestation
	head   []*pb.PendingAttestation
}

// MatchAttestations matches the attestations gathered in a span of an epoch
// and categorize them whether they correctly voted for source, target and head.
// We combined the individual helpers from spec for efficiency and to achieve O(N) run time.
//
// Spec pseudocode definition:
//  def get_matching_source_attestations(state: BeaconState, epoch: Epoch) -> List[PendingAttestation]:
//    assert epoch in (get_current_epoch(state), get_previous_epoch(state))
//    return state.current_epoch_attestations if epoch == get_current_epoch(state) else state.previous_epoch_attestations
//
//  def get_matching_target_attestations(state: BeaconState, epoch: Epoch) -> List[PendingAttestation]:
//    return [
//        a for a in get_matching_source_attestations(state, epoch)
//        if a.data.target_root == get_block_root(state, epoch)
//    ]
//
//  def get_matching_head_attestations(state: BeaconState, epoch: Epoch) -> List[PendingAttestation]:
//    return [
//        a for a in get_matching_source_attestations(state, epoch)
//        if a.data.beacon_block_root == get_block_root_at_slot(state, a.data.slot)
//    ]
func MatchAttestations(state *pb.BeaconState, epoch uint64) (*MatchedAttestations, error) {
	currentEpoch := helpers.CurrentEpoch(state)
	previousEpoch := helpers.PrevEpoch(state)

	// Input epoch for matching the source attestations has to be within range
	// of current epoch & previous epoch.
	if epoch != currentEpoch && epoch != previousEpoch {
		return nil, fmt.Errorf("input epoch: %d != current epoch: %d or previous epoch: %d",
			epoch, currentEpoch, previousEpoch)
	}

	// Decide if the source attestations are coming from current or previous epoch.
	var srcAtts []*pb.PendingAttestation
	if epoch == currentEpoch {
		srcAtts = state.CurrentEpochAttestations
	} else {
		srcAtts = state.PreviousEpochAttestations
	}
	targetRoot, err := helpers.BlockRoot(state, epoch)
	if err != nil {
		return nil, fmt.Errorf("could not get block root for epoch %d: %v", epoch, err)
	}

	tgtAtts := make([]*pb.PendingAttestation, 0, len(srcAtts))
	headAtts := make([]*pb.PendingAttestation, 0, len(srcAtts))
	for _, srcAtt := range srcAtts {
		// If the target root matches attestation's target root,
		// then we know this attestation has correctly voted for target.
		if bytes.Equal(srcAtt.Data.TargetRoot, targetRoot) {
			tgtAtts = append(tgtAtts, srcAtt)
		}

		// If the block root at slot matches attestation's block root at slot,
		// then we know this attestation has correctly voted for head.
		slot, err := helpers.AttestationDataSlot(state, srcAtt.Data)
		if err != nil {
			return nil, fmt.Errorf("could not get attestation slot: %v", err)
		}
		headRoot, err := helpers.BlockRootAtSlot(state, slot)
		if err != nil {
			return nil, fmt.Errorf("could not get block root for slot %d: %v", slot, err)
		}
		if bytes.Equal(srcAtt.Data.BeaconBlockRoot, headRoot) {
			headAtts = append(headAtts, srcAtt)
		}
	}

	return &MatchedAttestations{
		source: srcAtts,
		Target: tgtAtts,
		head:   headAtts,
	}, nil
}

// AttestingBalance returns the total balance from all the attesting indices.
//
// WARNING: This method allocates a new copy of the attesting validator indices set and is
// considered to be very memory expensive. Avoid using this unless you really
// need to get attesting balance from attestations.
//
// Spec pseudocode definition:
//  def get_attesting_balance(state: BeaconState, attestations: List[PendingAttestation]) -> Gwei:
//    return get_total_balance(state, get_unslashed_attesting_indices(state, attestations))
func AttestingBalance(state *pb.BeaconState, atts []*pb.PendingAttestation) (uint64, error) {
	indices, err := unslashedAttestingIndices(state, atts)
	if err != nil {
		return 0, fmt.Errorf("could not get attesting indices: %v", err)
	}
	return helpers.TotalBalance(state, indices), nil
}

// ProcessJustificationAndFinalization processes justification and finalization during
// epoch processing. This is where a beacon node can justify and finalize a new epoch.
//
// Spec pseudocode definition:
//  def process_justification_and_finalization(state: BeaconState) -> None:
//    if get_current_epoch(state) <= GENESIS_EPOCH + 1:
//        return
//
//    previous_epoch = get_previous_epoch(state)
//    current_epoch = get_current_epoch(state)
//    old_previous_justified_epoch = state.previous_justified_epoch
//    old_current_justified_epoch = state.current_justified_epoch
//
//    # Process justifications
//    state.previous_justified_epoch = state.current_justified_epoch
//    state.previous_justified_root = state.current_justified_root
//    state.justification_bitfield = (state.justification_bitfield << 1) % 2**64
//    previous_epoch_matching_target_balance = get_attesting_balance(state, get_matching_target_attestations(state, previous_epoch))
//    if previous_epoch_matching_target_balance * 3 >= get_total_active_balance(state) * 2:
//        state.current_justified_epoch = previous_epoch
//        state.current_justified_root = get_block_root(state, state.current_justified_epoch)
//        state.justification_bitfield |= (1 << 1)
//    current_epoch_matching_target_balance = get_attesting_balance(state, get_matching_target_attestations(state, current_epoch))
//    if current_epoch_matching_target_balance * 3 >= get_total_active_balance(state) * 2:
//        state.current_justified_epoch = current_epoch
//        state.current_justified_root = get_block_root(state, state.current_justified_epoch)
//        state.justification_bitfield |= (1 << 0)
//
//    # Process finalizations
//    bitfield = state.justification_bitfield
//    # The 2nd/3rd/4th most recent epochs are justified, the 2nd using the 4th as source
//    if (bitfield >> 1) % 8 == 0b111 and old_previous_justified_epoch - 3 == current_epoch:
//        state.finalized_epoch = old_previous_justified_epoch
//        state.finalized_root = get_block_root(state, state.finalized_epoch)
//    # The 2nd/3rd most recent epochs are justified, the 2nd using the 3rd as source
//    if (bitfield >> 1) % 4 == 0b11 and old_previous_justified_epoch - 2 == current_epoch:
//        state.finalized_epoch = old_previous_justified_epoch
//        state.finalized_root = get_block_root(state, state.finalized_epoch)
//    # The 1st/2nd/3rd most recent epochs are justified, the 1st using the 3rd as source
//    if (bitfield >> 0) % 8 == 0b111 and old_current_justified_epoch - 2 == current_epoch:
//        state.finalized_epoch = old_current_justified_epoch
//        state.finalized_root = get_block_root(state, state.finalized_epoch)
//    # The 1st/2nd most recent epochs are justified, the 1st using the 2nd as source
//    if (bitfield >> 0) % 4 == 0b11 and old_current_justified_epoch - 1 == current_epoch:
//        state.finalized_epoch = old_current_justified_epoch
//        state.finalized_root = get_block_root(state, state.finalized_epoch)
func ProcessJustificationAndFinalization(state *pb.BeaconState, prevAttestedBal uint64, currAttestedBal uint64) (
	*pb.BeaconState, error) {
	// There's no reason to process justification until the 3rd epoch.
	currentEpoch := helpers.CurrentEpoch(state)
	if currentEpoch <= 1 {
		return state, nil
	}

	prevEpoch := helpers.PrevEpoch(state)
	totalBal, err := helpers.TotalActiveBalance(state)
	if err != nil {
		return nil, fmt.Errorf("could not get total balance: %v", err)
	}

	oldPrevJustifiedCheckpoint := state.PreviousJustifiedCheckpoint
	oldCurrJustifiedCheckpoint := state.CurrentJustifiedCheckpoint
	state.PreviousJustifiedCheckpoint = state.CurrentJustifiedCheckpoint
	// Process justification.
	if len(state.JustificationBits) != 1 {
		return nil, errors.New("state justification bits is not exactly 1 byte")
	}
	// Note that the justification bits are type [4]BitVector. This means that
	// the maximum value is 0b1111 for a uint8 field.
	state.JustificationBits[0] <<= 1
	state.JustificationBits[0] &= 0x0F // mask with 0b1111. This eliminates the first left most 4 bits.
	if 3*prevAttestedBal >= 2*totalBal {
		state.CurrentJustifiedCheckpoint.Epoch = prevEpoch
		blockRoot, err := helpers.BlockRoot(state, prevEpoch)
		if err != nil {
			return nil, fmt.Errorf("could not get block root for previous epoch %d: %v",
				prevEpoch, err)
		}
		state.CurrentJustifiedCheckpoint.Root = blockRoot
		state.JustificationBits[0] |= 2
	}
	if 3*currAttestedBal >= 2*totalBal {
		state.CurrentJustifiedCheckpoint.Epoch = currentEpoch
		blockRoot, err := helpers.BlockRoot(state, currentEpoch)
		if err != nil {
			return nil, fmt.Errorf("could not get block root for current epoch %d: %v",
				prevEpoch, err)
		}
		state.CurrentJustifiedCheckpoint.Root = blockRoot
		state.JustificationBits[0] |= 1
	}
	// Process finalization.
	bitfield := state.JustificationBits[0]
	// When the 2nd, 3rd and 4th most recent epochs are all justified,
	// 2nd epoch can finalize the 4th epoch as a source.
	if oldPrevJustifiedCheckpoint.Epoch+3 == currentEpoch && (bitfield>>1)%8 == 7 {
		state.FinalizedCheckpoint.Epoch = oldPrevJustifiedCheckpoint.Epoch
		state.FinalizedCheckpoint.Root = oldPrevJustifiedCheckpoint.Root
	}
	// when 2nd and 3rd most recent epochs are all justified,
	// 2nd epoch can finalize 3rd as a source.
	if oldPrevJustifiedCheckpoint.Epoch+2 == currentEpoch && (bitfield>>1)%4 == 3 {
		state.FinalizedCheckpoint.Epoch = oldPrevJustifiedCheckpoint.Epoch
		state.FinalizedCheckpoint.Root = oldPrevJustifiedCheckpoint.Root
	}
	// when 1st, 2nd and 3rd most recent epochs are all justified,
	// 1st epoch can finalize 3rd as a source.
	if oldCurrJustifiedCheckpoint.Epoch+2 == currentEpoch && (bitfield>>0)%8 == 7 {
		state.FinalizedCheckpoint.Epoch = oldCurrJustifiedCheckpoint.Epoch
		state.FinalizedCheckpoint.Root = oldCurrJustifiedCheckpoint.Root
	}
	// when 1st, 2nd most recent epochs are all justified,
	// 1st epoch can finalize 2nd as a source.
	if oldCurrJustifiedCheckpoint.Epoch+1 == currentEpoch && (bitfield>>0)%4 == 3 {
		state.FinalizedCheckpoint.Epoch = oldCurrJustifiedCheckpoint.Epoch
		state.FinalizedCheckpoint.Root = oldCurrJustifiedCheckpoint.Root
	}
	return state, nil
}

// ProcessCrosslinks processes crosslink and finds the crosslink
// with enough state to make it canonical in state.
//
// Spec pseudocode definition:
//  def process_crosslinks(state: BeaconState) -> None:
//    state.previous_crosslinks = [c for c in state.current_crosslinks]
//    for epoch in (get_previous_epoch(state), get_current_epoch(state)):
//        for offset in range(get_epoch_committee_count(state, epoch)):
//            shard = (get_epoch_start_shard(state, epoch) + offset) % SHARD_COUNT
//            crosslink_committee = get_crosslink_committee(state, epoch, shard)
//            winning_crosslink, attesting_indices = get_winning_crosslink_and_attesting_indices(state, epoch, shard)
//            if 3 * get_total_balance(state, attesting_indices) >= 2 * get_total_balance(state, crosslink_committee):
//                state.current_crosslinks[shard] = winning_crosslink
func ProcessCrosslinks(state *pb.BeaconState) (*pb.BeaconState, error) {
	copy(state.PreviousCrosslinks, state.CurrentCrosslinks)
	epochs := []uint64{helpers.PrevEpoch(state), helpers.CurrentEpoch(state)}
	for _, e := range epochs {
		count, err := helpers.EpochCommitteeCount(state, e)
		if err != nil {
			return nil, fmt.Errorf("could not get epoch committee count: %v", err)
		}
		startShard, err := helpers.EpochStartShard(state, e)
		if err != nil {
			return nil, fmt.Errorf("could not get epoch start shards: %v", err)
		}
		for offset := uint64(0); offset < count; offset++ {
			shard := (startShard + offset) % params.BeaconConfig().ShardCount
			committee, err := helpers.CrosslinkCommitteeAtEpoch(state, e, shard)
			if err != nil {
				return nil, fmt.Errorf("could not get crosslink committee: %v", err)
			}
			crosslink, indices, err := winningCrosslink(state, shard, e)
			if err != nil {
				return nil, fmt.Errorf("could not get winning crosslink: %v", err)
			}
			attestedBalance := helpers.TotalBalance(state, indices)
			totalBalance := helpers.TotalBalance(state, committee)

			// In order for a crosslink to get included in state, the attesting balance needs to
			// be greater than 2/3 of the total balance.
			if 3*attestedBalance >= 2*totalBalance {
				state.CurrentCrosslinks[shard] = crosslink
			}
		}
	}
	return state, nil
}

// ProcessRewardsAndPenalties processes the rewards and penalties of individual validator.
//
// Spec pseudocode definition:
//  def process_rewards_and_penalties(state: BeaconState) -> None:
//    if get_current_epoch(state) == GENESIS_EPOCH:
//        return
//
//    rewards1, penalties1 = get_attestation_deltas(state)
//    rewards2, penalties2 = get_crosslink_deltas(state)
//    for i in range(len(state.validator_registry)):
//        increase_balance(state, i, rewards1[i] + rewards2[i])
//        decrease_balance(state, i, penalties1[i] + penalties2[i])
func ProcessRewardsAndPenalties(state *pb.BeaconState) (*pb.BeaconState, error) {
	// Can't process rewards and penalties in genesis epoch.
	if helpers.CurrentEpoch(state) == 0 {
		return state, nil
	}
	attsRewards, attsPenalties, err := attestationDelta(state)
	if err != nil {
		return nil, fmt.Errorf("could not get attestation delta: %v ", err)
	}
	clRewards, clPenalties, err := crosslinkDelta(state)
	if err != nil {
		return nil, fmt.Errorf("could not get crosslink delta: %v ", err)
	}
	for i := 0; i < len(state.Validators); i++ {
		state = helpers.IncreaseBalance(state, uint64(i), attsRewards[i]+clRewards[i])
		state = helpers.DecreaseBalance(state, uint64(i), attsPenalties[i]+clPenalties[i])
	}
	return state, nil
}

// ProcessRegistryUpdates rotates validators in and out of active pool.
// the amount to rotate is determined churn limit.
//
// Spec pseudocode definition:
//   def process_registry_updates(state: BeaconState) -> None:
//    # Process activation eligibility and ejections
//    for index, validator in enumerate(state.validator_registry):
//        if (
//            validator.activation_eligibility_epoch == FAR_FUTURE_EPOCH and
//            validator.effective_balance >= MAX_EFFECTIVE_BALANCE
//        ):
//            validator.activation_eligibility_epoch = get_current_epoch(state)
//
//        if is_active_validator(validator, get_current_epoch(state)) and validator.effective_balance <= EJECTION_BALANCE:
//            initiate_validator_exit(state, index)
//
//    # Queue validators eligible for activation and not dequeued for activation prior to finalized epoch
//    activation_queue = sorted([
//        index for index, validator in enumerate(state.validator_registry) if
//        validator.activation_eligibility_epoch != FAR_FUTURE_EPOCH and
//        validator.activation_epoch >= get_delayed_activation_exit_epoch(state.finalized_epoch)
//    ], key=lambda index: state.validator_registry[index].activation_eligibility_epoch)
//    # Dequeued validators for activation up to churn limit (without resetting activation epoch)
//    for index in activation_queue[:get_churn_limit(state)]:
//        validator = state.validator_registry[index]
//        if validator.activation_epoch == FAR_FUTURE_EPOCH:
//            validator.activation_epoch = get_delayed_activation_exit_epoch(get_current_epoch(state))
func ProcessRegistryUpdates(state *pb.BeaconState) (*pb.BeaconState, error) {
	currentEpoch := helpers.CurrentEpoch(state)

	var err error
	for idx, validator := range state.Validators {
		// Process the validators for activation eligibility.
		eligibleToActivate := validator.ActivationEligibilityEpoch == params.BeaconConfig().FarFutureEpoch
		properBalance := validator.EffectiveBalance >= params.BeaconConfig().MaxEffectiveBalance
		if eligibleToActivate && properBalance {
			validator.ActivationEligibilityEpoch = currentEpoch
		}
		// Process the validators for ejection.
		isActive := helpers.IsActiveValidator(validator, currentEpoch)
		belowEjectionBalance := validator.EffectiveBalance <= params.BeaconConfig().EjectionBalance
		if isActive && belowEjectionBalance {
			state, err = validators.InitiateValidatorExit(state, uint64(idx))
			if err != nil {
				return nil, err
			}
		}
	}

	// Queue the validators whose eligible to activate and sort them by activation eligibility epoch number
	var activationQ []uint64
	for idx, validator := range state.Validators {
		eligibleActivated := validator.ActivationEligibilityEpoch != params.BeaconConfig().FarFutureEpoch
		canBeActive := validator.ActivationEpoch >= helpers.DelayedActivationExitEpoch(state.FinalizedCheckpoint.Epoch)
		if eligibleActivated && canBeActive {
			activationQ = append(activationQ, uint64(idx))
		}
	}
	sort.Slice(activationQ, func(i, j int) bool {
		return state.Validators[i].ActivationEligibilityEpoch < state.Validators[j].ActivationEligibilityEpoch
	})

	// Only activate just enough validators according to the activation churn limit.
	limit := len(activationQ)
	churnLimit, err := helpers.ChurnLimit(state)
	if err != nil {
		return nil, fmt.Errorf("could not get churn limit: %v", err)
	}

	// Prevent churn limit cause index out of bound.
	if int(churnLimit) < limit {
		limit = int(churnLimit)
	}
	for _, index := range activationQ[:limit] {
		validator := state.Validators[index]
		if validator.ActivationEpoch == params.BeaconConfig().FarFutureEpoch {
			validator.ActivationEpoch = helpers.DelayedActivationExitEpoch(currentEpoch)
		}
	}
	return state, nil
}

// ProcessSlashings processes the slashed validators during epoch processing,
//
//  def process_slashings(state: BeaconState) -> None:
//    current_epoch = get_current_epoch(state)
//    total_balance = get_total_active_balance(state)
//
//    # Compute slashed balances in the current epoch
//    total_at_start = state.latest_slashed_balances[(current_epoch + 1) % LATEST_SLASHED_EXIT_LENGTH]
//    total_at_end = state.latest_slashed_balances[current_epoch % LATEST_SLASHED_EXIT_LENGTH]
//    total_penalties = total_at_end - total_at_start
//
//    for index, validator in enumerate(state.validator_registry):
//        if validator.slashed and current_epoch == validator.withdrawable_epoch - LATEST_SLASHED_EXIT_LENGTH // 2:
//            penalty = max(
//                validator.effective_balance * min(total_penalties * 3, total_balance) // total_balance,
//                validator.effective_balance // MIN_SLASHING_PENALTY_QUOTIENT
//            )
//            decrease_balance(state, index, penalty)
func ProcessSlashings(state *pb.BeaconState) (*pb.BeaconState, error) {
	currentEpoch := helpers.CurrentEpoch(state)
	totalBalance, err := helpers.TotalActiveBalance(state)
	if err != nil {
		return nil, fmt.Errorf("could not get total active balance: %v", err)
	}

	// Compute slashed balances in the current epoch
	exitLength := params.BeaconConfig().EpochsPerSlashingsVector
	totalAtStart := state.Slashings[(currentEpoch+1)%exitLength]
	totalAtEnd := state.Slashings[currentEpoch%exitLength]
	totalPenalties := totalAtEnd - totalAtStart

	// Compute slashing for each validator.
	for index, validator := range state.Validators {
		correctEpoch := currentEpoch == validator.WithdrawableEpoch-exitLength/2
		if validator.Slashed && correctEpoch {
			minPenalties := totalPenalties * 3
			if minPenalties > totalBalance {
				minPenalties = totalBalance
			}
			effectiveBal := validator.EffectiveBalance
			penalty := effectiveBal * minPenalties / totalBalance
			if penalty < effectiveBal/params.BeaconConfig().MinSlashingPenaltyQuotient {
				penalty = effectiveBal / params.BeaconConfig().MinSlashingPenaltyQuotient
			}
			state = helpers.DecreaseBalance(state, uint64(index), penalty)
		}
	}
	return state, err
}

// ProcessFinalUpdates processes the final updates during epoch processing.
//
// Spec pseudocode definition:
//  def process_final_updates(state: BeaconState) -> None:
//    current_epoch = get_current_epoch(state)
//    next_epoch = current_epoch + 1
//    # Reset eth1 data votes
//    if (state.slot + 1) % SLOTS_PER_ETH1_VOTING_PERIOD == 0:
//        state.eth1_data_votes = []
//    # Update effective balances with hysteresis
//    for index, validator in enumerate(state.validator_registry):
//        balance = state.balances[index]
//        HALF_INCREMENT = EFFECTIVE_BALANCE_INCREMENT // 2
//        if balance < validator.effective_balance or validator.effective_balance + 3 * HALF_INCREMENT < balance:
//            validator.effective_balance = min(balance - balance % EFFECTIVE_BALANCE_INCREMENT, MAX_EFFECTIVE_BALANCE)
//    # Update start shard
//    state.latest_start_shard = (state.latest_start_shard + get_shard_delta(state, current_epoch)) % SHARD_COUNT
//    # Set active index root
//    index_root_position = (next_epoch + ACTIVATION_EXIT_DELAY) % LATEST_ACTIVE_INDEX_ROOTS_LENGTH
//    state.latest_active_index_roots[index_root_position] = hash_tree_root(
//        get_active_validator_indices(state, next_epoch + ACTIVATION_EXIT_DELAY)
//    )
//    # Set total slashed balances
//    state.latest_slashed_balances[next_epoch % LATEST_SLASHED_EXIT_LENGTH] = (
//        state.latest_slashed_balances[current_epoch % LATEST_SLASHED_EXIT_LENGTH]
//    )
//    # Set randao mix
//    state.latest_randao_mixes[next_epoch % LATEST_RANDAO_MIXES_LENGTH] = get_randao_mix(state, current_epoch)
//    # Set historical root accumulator
//    if next_epoch % (SLOTS_PER_HISTORICAL_ROOT // SLOTS_PER_EPOCH) == 0:
//        historical_batch = HistoricalBatch(
//            block_roots=state.latest_block_roots,
//            state_roots=state.latest_state_roots,
//        )
//        state.historical_roots.append(hash_tree_root(historical_batch))
//    # Rotate current/previous epoch attestations
//    state.previous_epoch_attestations = state.current_epoch_attestations
//    state.current_epoch_attestations = []
func ProcessFinalUpdates(state *pb.BeaconState) (*pb.BeaconState, error) {
	currentEpoch := helpers.CurrentEpoch(state)
	nextEpoch := currentEpoch + 1

	// Reset ETH1 data votes.
	if (state.Slot+1)%params.BeaconConfig().SlotsPerEth1VotingPeriod == 0 {
		state.Eth1DataVotes = nil
	}

	// Update effective balances with hysteresis.
	for i, v := range state.Validators {
		balance := state.Balances[i]
		halfInc := params.BeaconConfig().EffectiveBalanceIncrement / 2
		if balance < v.EffectiveBalance || v.EffectiveBalance+3*halfInc < balance {
			v.EffectiveBalance = params.BeaconConfig().MaxEffectiveBalance
			if v.EffectiveBalance > balance-balance%params.BeaconConfig().EffectiveBalanceIncrement {
				v.EffectiveBalance = balance - balance%params.BeaconConfig().EffectiveBalanceIncrement
			}
		}
	}

	// Update start shard.
	delta, err := helpers.ShardDelta(state, currentEpoch)
	if err != nil {
		return nil, fmt.Errorf("could not get shard delta: %v", err)
	}
	state.StartShard = (state.StartShard + delta) %
		params.BeaconConfig().ShardCount

	// Set active index root.
	activationDelay := params.BeaconConfig().ActivationExitDelay
	idxRootPosition := (nextEpoch + activationDelay) % params.BeaconConfig().EpochsPerHistoricalVector
	activeIndices, err := helpers.ActiveValidatorIndices(state, nextEpoch+activationDelay)
	if err != nil {
		return nil, fmt.Errorf("could not get active indices: %v", err)
	}
	idxRoot, err := ssz.HashTreeRoot(activeIndices)
	if err != nil {
		return nil, fmt.Errorf("could not tree hash active indices: %v", err)
	}
	state.ActiveIndexRoots[idxRootPosition] = idxRoot[:]

	commRootPosition := (nextEpoch + activationDelay) % params.BeaconConfig().EpochsPerHistoricalVector
	comRoot, err := helpers.CompactCommitteesRoot(state, nextEpoch)
	if err != nil {
		return nil, fmt.Errorf("could not get compact committee root %v", err)
	}
	state.CompactCommitteesRoots[commRootPosition] = comRoot[:]

	// Set total slashed balances.
	slashedExitLength := params.BeaconConfig().EpochsPerSlashingsVector
	state.Slashings[nextEpoch%slashedExitLength] =
		state.Slashings[currentEpoch%slashedExitLength]

	// Set RANDAO mix.
	randaoMixLength := params.BeaconConfig().EpochsPerHistoricalVector
	mix := helpers.RandaoMix(state, currentEpoch)
	state.RandaoMixes[nextEpoch%randaoMixLength] = mix

	// Set historical root accumulator.
	epochsPerHistoricalRoot := params.BeaconConfig().HistoricalRootsLimit / params.BeaconConfig().SlotsPerEpoch
	if nextEpoch%epochsPerHistoricalRoot == 0 {
		historicalBatch := &pb.HistoricalBatch{
			BlockRoots: state.BlockRoots,
			StateRoots: state.StateRoots,
		}
		batchRoot, err := ssz.HashTreeRoot(historicalBatch)
		if err != nil {
			return nil, fmt.Errorf("could not hash historical batch: %v", err)
		}
		state.HistoricalRoots = append(state.HistoricalRoots, batchRoot[:])
	}

	// Rotate current and previous epoch attestations.
	state.PreviousEpochAttestations = state.CurrentEpochAttestations
	state.CurrentEpochAttestations = make([]*pb.PendingAttestation, 0, 0)

	return state, nil
}

// unslashedAttestingIndices returns all the attesting indices from a list of attestations,
// it sorts the indices and filters out the slashed ones.
//
// Spec pseudocode definition:
//  def get_unslashed_attesting_indices(state: BeaconState, attestations: List[PendingAttestation]) -> List[ValidatorIndex]:
//    output = set()
//    for a in attestations:
//        output = output.union(get_attesting_indices(state, a.data, a.aggregation_bitfield))
//    return sorted(filter(lambda index: not state.validator_registry[index].slashed, list(output)))
func unslashedAttestingIndices(state *pb.BeaconState, atts []*pb.PendingAttestation) ([]uint64, error) {
	var setIndices []uint64
	for _, att := range atts {
		indices, err := helpers.AttestingIndices(state, att.Data, att.AggregationBits)
		if err != nil {
			return nil, fmt.Errorf("could not get attester indices: %v", err)
		}
		setIndices = append(setIndices, indices...)
	}
	// Sort the attesting set indices by increasing order.
	sort.Slice(setIndices, func(i, j int) bool { return setIndices[i] < setIndices[j] })
	// Remove the slashed validator indices.
	for i := 0; i < len(setIndices); i++ {
		if state.Validators[setIndices[i]].Slashed {
			setIndices = append(setIndices[:i], setIndices[i+1:]...)
		}
	}
	return setIndices, nil
}

// winningCrosslink returns the most staked balance-wise crosslink of a given shard and epoch.
// It also returns the attesting inaidces of the winning cross link.
//
// Spec pseudocode definition:
//  def get_winning_crosslink_and_attesting_indices(state: BeaconState,
//                                                epoch: Epoch,
//                                                shard: Shard) -> Tuple[Crosslink, List[ValidatorIndex]]:
//    attestations = [a for a in get_matching_source_attestations(state, epoch) if a.data.crosslink.shard == shard]
//    crosslinks = list(filter(
//        lambda c: hash_tree_root(state.current_crosslinks[shard]) in (c.parent_root, hash_tree_root(c)),
//        [a.data.crosslink for a in attestations]
//    ))
//    # Winning crosslink has the crosslink data root with the most balance voting for it (ties broken lexicographically)
//    winning_crosslink = max(crosslinks, key=lambda c: (
//        get_attesting_balance(state, [a for a in attestations if a.data.crosslink == c]), c.data_root
//    ), default=Crosslink())
//    winning_attestations = [a for a in attestations if a.data.crosslink == winning_crosslink]
//    return winning_crosslink, get_unslashed_attesting_indices(state, winning_attestations)
func winningCrosslink(state *pb.BeaconState, shard uint64, epoch uint64) (*pb.Crosslink, []uint64, error) {
	var shardAtts []*pb.PendingAttestation
	matchedAtts, err := MatchAttestations(state, epoch)
	if err != nil {
		return nil, nil, fmt.Errorf("could not get matching attestations: %v", err)
	}

	// Filter out source attestations by shard.
	for _, att := range matchedAtts.source {
		if att.Data.Crosslink.Shard == shard {
			shardAtts = append(shardAtts, att)
		}
	}
	var candidateCrosslinks []*pb.Crosslink
	// Filter out shard crosslinks with correct current or previous crosslink data.
	for _, a := range shardAtts {
		stateCrosslink := state.CurrentCrosslinks[shard]
		stateCrosslinkRoot, err := ssz.HashTreeRoot(stateCrosslink)
		if err != nil {
			return nil, nil, fmt.Errorf("could not hash tree root crosslink from state: %v", err)
		}
		attCrosslinkRoot, err := ssz.HashTreeRoot(a.Data.Crosslink)
		if err != nil {
			return nil, nil, fmt.Errorf("could not hash tree root crosslink from attestation: %v", err)
		}
		currCrosslinkMatches := bytes.Equal(stateCrosslinkRoot[:], attCrosslinkRoot[:])
		prevCrosslinkMatches := bytes.Equal(stateCrosslinkRoot[:], a.Data.Crosslink.ParentRoot)
		if currCrosslinkMatches || prevCrosslinkMatches {
			candidateCrosslinks = append(candidateCrosslinks, a.Data.Crosslink)
		}
	}

	if len(candidateCrosslinks) == 0 {
		return &pb.Crosslink{
			DataRoot:   params.BeaconConfig().ZeroHash[:],
			ParentRoot: params.BeaconConfig().ZeroHash[:],
		}, nil, nil
	}
	var crosslinkAtts []*pb.PendingAttestation
	var winnerBalance uint64
	var winnerCrosslink *pb.Crosslink
	// Out of the existing shard crosslinks, pick the one that has the
	// most balance staked.
	crosslinkAtts = attsForCrosslink(candidateCrosslinks[0], shardAtts)
	winnerBalance, err = AttestingBalance(state, crosslinkAtts)
	if err != nil {
		return nil, nil, err
	}

	winnerCrosslink = candidateCrosslinks[0]
	for _, c := range candidateCrosslinks {
		crosslinkAtts := crosslinkAtts[:0]
		crosslinkAtts = attsForCrosslink(c, shardAtts)
		attestingBalance, err := AttestingBalance(state, crosslinkAtts)
		if err != nil {
			return nil, nil, fmt.Errorf("could not get crosslink's attesting balance: %v", err)
		}
		if attestingBalance > winnerBalance {
			winnerCrosslink = c
		}
	}

	crosslinkIndices, err := unslashedAttestingIndices(state, attsForCrosslink(winnerCrosslink, shardAtts))
	if err != nil {
		return nil, nil, errors.New("could not get crosslink indices")
	}

	return winnerCrosslink, crosslinkIndices, nil
}

// baseReward takes state and validator index and calculate
// individual validator's base reward quotient.
//
// Note: Adjusted quotient is calculated of base reward because it's too inefficient
// to repeat the same calculation for every validator versus just doing it once.
//
// Spec pseudocode definition:
//  def get_base_reward(state: BeaconState, index: ValidatorIndex) -> Gwei:
//      total_balance = get_total_active_balance(state)
//	    effective_balance = state.validator_registry[index].effective_balance
//	    return effective_balance * BASE_REWARD_FACTOR // integer_squareroot(total_balance) // BASE_REWARDS_PER_EPOCH
func baseReward(state *pb.BeaconState, index uint64) (uint64, error) {
	totalBalance, err := helpers.TotalActiveBalance(state)
	if err != nil {
		return 0, fmt.Errorf("could not calculate active balance: %v", err)
	}
	effectiveBalance := state.Validators[index].EffectiveBalance
	baseReward := effectiveBalance * params.BeaconConfig().BaseRewardFactor /
		mathutil.IntegerSquareRoot(totalBalance) / params.BeaconConfig().BaseRewardsPerEpoch
	return baseReward, nil
}

// attestationDelta calculates the rewards and penalties of individual
// validator for voting the correct FFG source, FFG target, and head. It
// also calculates proposer delay inclusion and inactivity rewards
// and penalties. Individual rewards and penalties are returned in list.
//
// Note: we calculated adjusted quotient outside of base reward because it's too inefficient
// to repeat the same calculation for every validator versus just doing it once.
//
// Spec pseudocode definition:
//  def get_attestation_deltas(state: BeaconState) -> Tuple[List[Gwei], List[Gwei]]:
//    previous_epoch = get_previous_epoch(state)
//    total_balance = get_total_active_balance(state)
//    rewards = [0 for _ in range(len(state.validator_registry))]
//    penalties = [0 for _ in range(len(state.validator_registry))]
//    eligible_validator_indices = [
//        index for index, v in enumerate(state.validator_registry)
//        if is_active_validator(v, previous_epoch) or (v.slashed and previous_epoch + 1 < v.withdrawable_epoch)
//    ]
//
//    # Micro-incentives for matching FFG source, FFG target, and head
//    matching_source_attestations = get_matching_source_attestations(state, previous_epoch)
//    matching_target_attestations = get_matching_target_attestations(state, previous_epoch)
//    matching_head_attestations = get_matching_head_attestations(state, previous_epoch)
//    for attestations in (matching_source_attestations, matching_target_attestations, matching_head_attestations):
//        unslashed_attesting_indices = get_unslashed_attesting_indices(state, attestations)
//        attesting_balance = get_attesting_balance(state, attestations)
//        for index in eligible_validator_indices:
//            if index in unslashed_attesting_indices:
//                rewards[index] += get_base_reward(state, index) * attesting_balance // total_balance
//            else:
//                penalties[index] += get_base_reward(state, index)
//
//    # Proposer and inclusion delay micro-rewards
//    for index in get_unslashed_attesting_indices(state, matching_source_attestations):
//        attestation = min([
//            a for a in attestations if index in get_attesting_indices(state, a.data, a.aggregation_bitfield)
//        ], key=lambda a: a.inclusion_delay)
//        rewards[attestation.proposer_index] += get_base_reward(state, index) // PROPOSER_REWARD_QUOTIENT
//        rewards[index] += get_base_reward(state, index) * MIN_ATTESTATION_INCLUSION_DELAY // attestation.inclusion_delay
//
//    # Inactivity penalty
//    finality_delay = previous_epoch - state.finalized_epoch
//    if finality_delay > MIN_EPOCHS_TO_INACTIVITY_PENALTY:
//        matching_target_attesting_indices = get_unslashed_attesting_indices(state, matching_target_attestations)
//        for index in eligible_validator_indices:
//            penalties[index] += BASE_REWARDS_PER_EPOCH * get_base_reward(state, index)
//            if index not in matching_target_attesting_indices:
//                penalties[index] += state.validator_registry[index].effective_balance * finality_delay // INACTIVITY_PENALTY_QUOTIENT
//
//    return rewards, penalties
func attestationDelta(state *pb.BeaconState) ([]uint64, []uint64, error) {
	prevEpoch := helpers.PrevEpoch(state)
	totalBalance, err := helpers.TotalActiveBalance(state)
	if err != nil {
		return nil, nil, fmt.Errorf("could not get total active balance: %v", err)
	}

	rewards := make([]uint64, len(state.Validators))
	penalties := make([]uint64, len(state.Validators))

	// Filter out the list of eligible validator indices. The eligible validator
	// has to be active or slashed but before withdrawn.
	var eligible []uint64
	for i, v := range state.Validators {
		isActive := helpers.IsActiveValidator(v, prevEpoch)
		isSlashed := v.Slashed && (prevEpoch+1 < v.WithdrawableEpoch)
		if isActive || isSlashed {
			eligible = append(eligible, uint64(i))
		}
	}

	// Apply rewards and penalties for voting correct source target and head.
	// Construct a attestations list contains source, target and head attestations.
	atts, err := MatchAttestations(state, prevEpoch)
	if err != nil {
		return nil, nil, fmt.Errorf("could not get source, target and head attestations: %v", err)
	}
	var attsPackage [][]*pb.PendingAttestation
	attsPackage = append(attsPackage, atts.source)
	attsPackage = append(attsPackage, atts.Target)
	attsPackage = append(attsPackage, atts.head)

	// Cache the validators who voted correctly for source in a map
	// to calculate earliest attestation rewards later.
	attestersVotedSoruce := make(map[uint64]*pb.PendingAttestation)
	// Compute rewards / penalties for each attestation in the list and update
	// the rewards and penalties lists.
	for i, matchAtt := range attsPackage {
		indices, err := unslashedAttestingIndices(state, matchAtt)
		if err != nil {
			return nil, nil, fmt.Errorf("could not get attestation indices: %v", err)
		}

		attested := make(map[uint64]bool)
		// Construct a map to look up validators that voted for source, target or head.
		for _, index := range indices {
			if i == 0 {
				attestersVotedSoruce[index] = &pb.PendingAttestation{InclusionDelay: params.BeaconConfig().FarFutureEpoch}
			}
			attested[index] = true
		}
		attestedBalance := helpers.TotalBalance(state, indices)

		// Update rewards and penalties to each eligible validator index.
		for _, index := range eligible {
			base, err := baseReward(state, index)
			if err != nil {
				return nil, nil, fmt.Errorf("could not get base reward: %v", err)
			}
			if _, ok := attested[index]; ok {
				rewards[index] += base * attestedBalance / totalBalance
			} else {
				penalties[index] += base
			}
		}
	}

	// For every index, filter the matching source attestation that correspond to the index,
	// sort by inclusion delay and get the one that was included on chain first.
	for _, att := range atts.source {
		indices, err := helpers.AttestingIndices(state, att.Data, att.AggregationBits)
		if err != nil {
			return nil, nil, fmt.Errorf("could not get attester indices: %v", err)
		}
		for _, i := range indices {
			if _, ok := attestersVotedSoruce[i]; ok {
				if attestersVotedSoruce[i].InclusionDelay > att.InclusionDelay {
					attestersVotedSoruce[i] = att
				}
			}
		}
	}

	for i, a := range attestersVotedSoruce {
		base, err := baseReward(state, i)
		if err != nil {
			return nil, nil, fmt.Errorf("could not get base reward: %v", err)
		}
		proposerReward := base / params.BeaconConfig().ProposerRewardQuotient
		maxAttesterReward := base - proposerReward
		slotsPerEpoch := params.BeaconConfig().SlotsPerEpoch
		attesterFactor := slotsPerEpoch + params.BeaconConfig().MinAttestationInclusionDelay - a.InclusionDelay
		rewards[i] += maxAttesterReward * attesterFactor / slotsPerEpoch
	}

	// Apply penalties for quadratic leaks.
	// When epoch since finality exceeds inactivity penalty constant, the penalty gets increased
	// based on the finality delay.
	finalityDelay := prevEpoch - state.FinalizedCheckpoint.Epoch
	if finalityDelay > params.BeaconConfig().MinEpochsToInactivityPenalty {
		targetIndices, err := unslashedAttestingIndices(state, atts.Target)
		if err != nil {
			return nil, nil, fmt.Errorf("could not get attestation indices: %v", err)
		}
		attestedTarget := make(map[uint64]bool)
		for _, index := range targetIndices {
			attestedTarget[index] = true
		}
		for _, index := range eligible {
			base, err := baseReward(state, index)
			if err != nil {
				return nil, nil, fmt.Errorf("could not get base reward: %v", err)
			}
			penalties[index] += params.BeaconConfig().BaseRewardsPerEpoch * base
			if _, ok := attestedTarget[index]; !ok {
				penalties[index] += state.Validators[index].EffectiveBalance * finalityDelay /
					params.BeaconConfig().InactivityPenaltyQuotient
			}
		}
	}
	return rewards, penalties, nil
}

// crosslinkDelta calculates the rewards and penalties of individual
// validator for submitting the correct crosslink.
// Individual rewards and penalties are returned in list.
//
// Note: we calculated adjusted quotient outside of base reward because it's too inefficient
// to repeat the same calculation for every validator versus just doing it once.
//
// Spec pseudocode definition:
//  def get_crosslink_deltas(state: BeaconState) -> Tuple[List[Gwei], List[Gwei]]:
//    rewards = [0 for index in range(len(state.validator_registry))]
//    penalties = [0 for index in range(len(state.validator_registry))]
//    epoch = get_previous_epoch(state)
//    for offset in range(get_epoch_committee_count(state, epoch)):
//        shard = (get_epoch_start_shard(state, epoch) + offset) % SHARD_COUNT
//        crosslink_committee = get_crosslink_committee(state, epoch, shard)
//        winning_crosslink, attesting_indices = get_winning_crosslink_and_attesting_indices(state, epoch, shard)
//        attesting_balance = get_total_balance(state, attesting_indices)
//        committee_balance = get_total_balance(state, crosslink_committee)
//        for index in crosslink_committee:
//            base_reward = get_base_reward(state, index)
//            if index in attesting_indices:
//                rewards[index] += base_reward * attesting_balance // committee_balance
//            else:
//                penalties[index] += base_reward
//    return rewards, penalties
func crosslinkDelta(state *pb.BeaconState) ([]uint64, []uint64, error) {
	rewards := make([]uint64, len(state.Validators))
	penalties := make([]uint64, len(state.Validators))
	epoch := helpers.PrevEpoch(state)
	count, err := helpers.EpochCommitteeCount(state, epoch)
	if err != nil {
		return nil, nil, fmt.Errorf("could not get epoch committee count: %v", err)
	}
	startShard, err := helpers.EpochStartShard(state, epoch)
	if err != nil {
		return nil, nil, fmt.Errorf("could not get epoch start shard: %v", err)
	}
	for i := uint64(0); i < count; i++ {
		shard := (startShard + i) % params.BeaconConfig().ShardCount
		committee, err := helpers.CrosslinkCommitteeAtEpoch(state, epoch, shard)
		if err != nil {
			return nil, nil, fmt.Errorf("could not get crosslink's committee: %v", err)
		}
		_, attestingIndices, err := winningCrosslink(state, shard, epoch)
		if err != nil {
			return nil, nil, fmt.Errorf("could not get winning crosslink: %v", err)
		}

		attested := make(map[uint64]bool)
		// Construct a map to look up validators that voted for crosslink.
		for _, index := range attestingIndices {
			attested[index] = true
		}
		committeeBalance := helpers.TotalBalance(state, committee)
		attestingBalance := helpers.TotalBalance(state, attestingIndices)

		for _, index := range committee {
			base, err := baseReward(state, index)
			if err != nil {
				return nil, nil, fmt.Errorf("could not get base reward: %v", err)
			}
			if _, ok := attested[index]; ok {
				rewards[index] += base * attestingBalance / committeeBalance
			} else {
				penalties[index] += base
			}
		}
	}

	return rewards, penalties, nil
}

// attsForCrosslink returns the attestations of the input crosslink.
func attsForCrosslink(crosslink *pb.Crosslink, atts []*pb.PendingAttestation) []*pb.PendingAttestation {
	var crosslinkAtts []*pb.PendingAttestation
	for _, a := range atts {
		if proto.Equal(a.Data.Crosslink, crosslink) {
			crosslinkAtts = append(crosslinkAtts, a)
		}
	}
	return crosslinkAtts
}
