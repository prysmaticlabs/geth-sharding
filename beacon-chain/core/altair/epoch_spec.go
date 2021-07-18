package altair

import (
	"github.com/pkg/errors"
	types "github.com/prysmaticlabs/eth2-types"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	iface "github.com/prysmaticlabs/prysm/beacon-chain/state/interface"
	ethpb "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/shared/mathutil"
	"github.com/prysmaticlabs/prysm/shared/params"
)

// ProcessSyncCommitteeUpdates  processes sync client committee updates for the beacon state.
//
// Spec code:
// def process_sync_committee_updates(state: BeaconState) -> None:
//    next_epoch = get_current_epoch(state) + Epoch(1)
//    if next_epoch % EPOCHS_PER_SYNC_COMMITTEE_PERIOD == 0:
//        state.current_sync_committee = state.next_sync_committee
//        state.next_sync_committee = get_next_sync_committee(state)
func ProcessSyncCommitteeUpdates(beaconState iface.BeaconStateAltair) (iface.BeaconStateAltair, error) {
	nextEpoch := helpers.NextEpoch(beaconState)
	if nextEpoch%params.BeaconConfig().EpochsPerSyncCommitteePeriod == 0 {
		currentSyncCommittee, err := beaconState.NextSyncCommittee()
		if err != nil {
			return nil, err
		}
		if err := beaconState.SetCurrentSyncCommittee(currentSyncCommittee); err != nil {
			return nil, err
		}
		nextCommittee, err := NextSyncCommittee(beaconState)
		if err != nil {
			return nil, err
		}
		if err := beaconState.SetNextSyncCommittee(nextCommittee); err != nil {
			return nil, err
		}
		if err := helpers.UpdateSyncCommitteeCache(beaconState); err != nil {
			return nil, err
		}
	}
	return beaconState, nil
}

// ProcessParticipationFlagUpdates processes participation flag updates by rotating current to previous.
//
// Spec code:
// def process_participation_flag_updates(state: BeaconState) -> None:
//    state.previous_epoch_participation = state.current_epoch_participation
//    state.current_epoch_participation = [ParticipationFlags(0b0000_0000) for _ in range(len(state.validators))]
func ProcessParticipationFlagUpdates(beaconState iface.BeaconStateAltair) (iface.BeaconStateAltair, error) {
	c, err := beaconState.CurrentEpochParticipation()
	if err != nil {
		return nil, err
	}
	if err := beaconState.SetPreviousParticipationBits(c); err != nil {
		return nil, err
	}
	if err := beaconState.SetCurrentParticipationBits(make([]byte, beaconState.NumValidators())); err != nil {
		return nil, err
	}
	return beaconState, nil
}

// ProcessSlashings processes the slashed validators during epoch processing,
// The function is modified to use PROPORTIONAL_SLASHING_MULTIPLIER_ALTAIR.
//
// Spec code:
//  def process_slashings(state: BeaconState) -> None:
//    epoch = get_current_epoch(state)
//    total_balance = get_total_active_balance(state)
//    adjusted_total_slashing_balance = min(sum(state.slashings) * PROPORTIONAL_SLASHING_MULTIPLIER_ALTAIR, total_balance)
//    for index, validator in enumerate(state.validators):
//        if validator.slashed and epoch + EPOCHS_PER_SLASHINGS_VECTOR // 2 == validator.withdrawable_epoch:
//            increment = EFFECTIVE_BALANCE_INCREMENT  # Factored out from penalty numerator to avoid uint64 overflow
//            penalty_numerator = validator.effective_balance // increment * adjusted_total_slashing_balance
//            penalty = penalty_numerator // total_balance * increment
//            decrease_balance(state, ValidatorIndex(index), penalty)
//            decrease_balance(state, ValidatorIndex(index), penalty)
func ProcessSlashings(state iface.BeaconState) (iface.BeaconState, error) {
	currentEpoch := helpers.CurrentEpoch(state)
	totalBalance, err := helpers.TotalActiveBalance(state)
	if err != nil {
		return nil, errors.Wrap(err, "could not get total active balance")
	}

	// Compute slashed balances in the current epoch
	exitLength := params.BeaconConfig().EpochsPerSlashingsVector

	// Compute the sum of state slashings
	slashings := state.Slashings()
	totalSlashing := uint64(0)
	for _, slashing := range slashings {
		totalSlashing += slashing
	}

	// a callback is used here to apply the following actions  to all validators
	// below equally.
	increment := params.BeaconConfig().EffectiveBalanceIncrement
	minSlashing := mathutil.Min(totalSlashing*params.BeaconConfig().ProportionalSlashingMultiplierAltair, totalBalance)
	err = state.ApplyToEveryValidator(func(idx int, val *ethpb.Validator) (bool, *ethpb.Validator, error) {
		correctEpoch := (currentEpoch + exitLength/2) == val.WithdrawableEpoch
		if val.Slashed && correctEpoch {
			penaltyNumerator := val.EffectiveBalance / increment * minSlashing
			penalty := penaltyNumerator / totalBalance * increment
			if err := helpers.DecreaseBalance(state, types.ValidatorIndex(idx), penalty); err != nil {
				return false, val, err
			}
			return true, val, nil
		}
		return false, val, nil
	})
	return state, err
}
