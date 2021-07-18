package helpers

import (
	"github.com/pkg/errors"
	types "github.com/prysmaticlabs/eth2-types"
	"github.com/prysmaticlabs/prysm/beacon-chain/cache"
	iface "github.com/prysmaticlabs/prysm/beacon-chain/state/interface"
	"github.com/prysmaticlabs/prysm/shared/params"
)

// TotalBalance returns the total amount at stake in Gwei
// of input validators.
//
// Spec pseudocode definition:
//   def get_total_balance(state: BeaconState, indices: Set[ValidatorIndex]) -> Gwei:
//    """
//    Return the combined effective balance of the ``indices``.
//    ``EFFECTIVE_BALANCE_INCREMENT`` Gwei minimum to avoid divisions by zero.
//    Math safe up to ~10B ETH, afterwhich this overflows uint64.
//    """
//    return Gwei(max(EFFECTIVE_BALANCE_INCREMENT, sum([state.validators[index].effective_balance for index in indices])))
func TotalBalance(state iface.ReadOnlyValidators, indices []types.ValidatorIndex) uint64 {
	total := uint64(0)

	for _, idx := range indices {
		val, err := state.ValidatorAtIndexReadOnly(idx)
		if err != nil {
			continue
		}
		total += val.EffectiveBalance()
	}

	// EFFECTIVE_BALANCE_INCREMENT is the lower bound for total balance.
	if total < params.BeaconConfig().EffectiveBalanceIncrement {
		return params.BeaconConfig().EffectiveBalanceIncrement
	}

	return total
}

// TotalActiveBalance returns the total amount at stake in Gwei
// of active validators.
//
// Spec pseudocode definition:
//   def get_total_active_balance(state: BeaconState) -> Gwei:
//    """
//    Return the combined effective balance of the active validators.
//    Note: ``get_total_balance`` returns ``EFFECTIVE_BALANCE_INCREMENT`` Gwei minimum to avoid divisions by zero.
//    """
//    return get_total_balance(state, set(get_active_validator_indices(state, get_current_epoch(state))))
func TotalActiveBalance(state iface.ReadOnlyBeaconState) (uint64, error) {
	// Check if the active balance exists in cache.
	epoch := SlotToEpoch(state.Slot())

	seed, err := Seed(state, epoch, params.BeaconConfig().DomainBeaconAttester)
	if err != nil {
		return 0, errors.Wrap(err, "could not get seed")
	}
	activeBalance, err := committeeCache.ActiveBalance(seed)
	if err == nil {
		return activeBalance, nil
	}
	if err != cache.ErrNonCommitteeKey {
		return 0, errors.Wrap(err, "could not interface with committee cache")
	}

	// Cache miss. Manually compute the active balance and fill the cache.
	total := uint64(0)
	if err := state.ReadFromEveryValidator(func(idx int, val iface.ReadOnlyValidator) error {
		if IsActiveValidatorUsingTrie(val, epoch) {
			total += val.EffectiveBalance()
		}
		return nil
	}); err != nil {
		return 0, err
	}

	return total, nil
}

// IncreaseBalance increases validator with the given 'index' balance by 'delta' in Gwei.
//
// Spec pseudocode definition:
//  def increase_balance(state: BeaconState, index: ValidatorIndex, delta: Gwei) -> None:
//    """
//    Increase the validator balance at index ``index`` by ``delta``.
//    """
//    state.balances[index] += delta
func IncreaseBalance(state iface.BeaconState, idx types.ValidatorIndex, delta uint64) error {
	balAtIdx, err := state.BalanceAtIndex(idx)
	if err != nil {
		return err
	}
	return state.UpdateBalancesAtIndex(idx, IncreaseBalanceWithVal(balAtIdx, delta))
}

// IncreaseBalanceWithVal increases validator with the given 'index' balance by 'delta' in Gwei.
// This method is flattened version of the spec method, taking in the raw balance and returning
// the post balance.
//
// Spec pseudocode definition:
//  def increase_balance(state: BeaconState, index: ValidatorIndex, delta: Gwei) -> None:
//    """
//    Increase the validator balance at index ``index`` by ``delta``.
//    """
//    state.balances[index] += delta
func IncreaseBalanceWithVal(currBalance, delta uint64) uint64 {
	return currBalance + delta
}

// DecreaseBalance decreases validator with the given 'index' balance by 'delta' in Gwei.
//
// Spec pseudocode definition:
//  def decrease_balance(state: BeaconState, index: ValidatorIndex, delta: Gwei) -> None:
//    """
//    Decrease the validator balance at index ``index`` by ``delta``, with underflow protection.
//    """
//    state.balances[index] = 0 if delta > state.balances[index] else state.balances[index] - delta
func DecreaseBalance(state iface.BeaconState, idx types.ValidatorIndex, delta uint64) error {
	balAtIdx, err := state.BalanceAtIndex(idx)
	if err != nil {
		return err
	}
	return state.UpdateBalancesAtIndex(idx, DecreaseBalanceWithVal(balAtIdx, delta))
}

// DecreaseBalanceWithVal decreases validator with the given 'index' balance by 'delta' in Gwei.
// This method is flattened version of the spec method, taking in the raw balance and returning
// the post balance.
//
// Spec pseudocode definition:
//  def decrease_balance(state: BeaconState, index: ValidatorIndex, delta: Gwei) -> None:
//    """
//    Decrease the validator balance at index ``index`` by ``delta``, with underflow protection.
//    """
//    state.balances[index] = 0 if delta > state.balances[index] else state.balances[index] - delta
func DecreaseBalanceWithVal(currBalance, delta uint64) uint64 {
	if delta > currBalance {
		return 0
	}
	return currBalance - delta
}

// IsInInactivityLeak returns true if the state is experiencing inactivity leak.
//
// Spec code:
// def is_in_inactivity_leak(state: BeaconState) -> bool:
//    return get_finality_delay(state) > MIN_EPOCHS_TO_INACTIVITY_PENALTY
func IsInInactivityLeak(prevEpoch, finalizedEpoch types.Epoch) bool {
	return FinalityDelay(prevEpoch, finalizedEpoch) > params.BeaconConfig().MinEpochsToInactivityPenalty
}

// FinalityDelay returns the finality delay using the beacon state.
//
// Spec code:
// def get_finality_delay(state: BeaconState) -> uint64:
//    return get_previous_epoch(state) - state.finalized_checkpoint.epoch
func FinalityDelay(prevEpoch, finalizedEpoch types.Epoch) types.Epoch {
	return prevEpoch - finalizedEpoch
}
