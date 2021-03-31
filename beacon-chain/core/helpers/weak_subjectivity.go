package helpers

import (
	"fmt"

	types "github.com/prysmaticlabs/eth2-types"
	iface "github.com/prysmaticlabs/prysm/beacon-chain/state/interface"
	"github.com/prysmaticlabs/prysm/shared/mathutil"
	"github.com/prysmaticlabs/prysm/shared/params"
)

// ComputeWeakSubjectivityCheckptEpoch returns weak subjectivity period for the active validator count and finalized epoch.
//
// Reference spec implementation:
// https://github.com/ethereum/eth2.0-specs/blob/master/specs/phase0/weak-subjectivity.md#calculating-the-weak-subjectivity-period
//
// def compute_weak_subjectivity_period(state: BeaconState) -> uint64:
//    """
//    Returns the weak subjectivity period for the current ``state``.
//    This computation takes into account the effect of:
//        - validator set churn (bounded by ``get_validator_churn_limit()`` per epoch), and
//        - validator balance top-ups (bounded by ``MAX_DEPOSITS * SLOTS_PER_EPOCH`` per epoch).
//    A detailed calculation can be found at:
//    https://github.com/runtimeverification/beacon-chain-verification/blob/master/weak-subjectivity/weak-subjectivity-analysis.pdf
//    """
//    ws_period = MIN_VALIDATOR_WITHDRAWABILITY_DELAY
//    N = len(get_active_validator_indices(state, get_current_epoch(state)))
//    t = get_total_active_balance(state) // N // ETH_TO_GWEI
//    T = MAX_EFFECTIVE_BALANCE // ETH_TO_GWEI
//    delta = get_validator_churn_limit(state)
//    Delta = MAX_DEPOSITS * SLOTS_PER_EPOCH
//    D = SAFETY_DECAY
//
//    if T * (200 + 3 * D) < t * (200 + 12 * D):
//        epochs_for_validator_set_churn = (
//            N * (t * (200 + 12 * D) - T * (200 + 3 * D)) // (600 * delta * (2 * t + T))
//        )
//        epochs_for_balance_top_ups = (
//            N * (200 + 3 * D) // (600 * Delta)
//        )
//        ws_period += max(epochs_for_validator_set_churn, epochs_for_balance_top_ups)
//    else:
//        ws_period += (
//            3 * N * D * t // (200 * Delta * (T - t))
//        )
//
//    return ws_period
func ComputeWeakSubjectivityCheckptEpoch(st iface.ReadOnlyBeaconState) (types.Epoch, error) {
	// Weak subjectivity period cannot be smaller than withdrawal delay.
	wsp := uint64(params.BeaconConfig().MinValidatorWithdrawabilityDelay)

	// Cardinality of active validator set.
	N, err := ActiveValidatorCount(st, CurrentEpoch(st))
	if err != nil {
		return 0, fmt.Errorf("cannot obtain active valiadtor count: %w", err)
	}

	// Average effective balance in the given validator set, in Ether.
	t, err := TotalActiveBalance(st)
	if err != nil {
		return 0, fmt.Errorf("cannot find total active balance of validators: %w", err)
	}
	t = t / N / params.BeaconConfig().GweiPerEth

	// Maximum effective balance per validator.
	T := params.BeaconConfig().MaxEffectiveBalance / params.BeaconConfig().GweiPerEth

	// Validator churn limit.
	delta, err := ValidatorChurnLimit(N)
	if err != nil {
		return 0, fmt.Errorf("cannot obtain active validator churn limit: %w", err)
	}

	// Balance top-ups.
	Delta := uint64(params.BeaconConfig().SlotsPerEpoch.Mul(params.BeaconConfig().MaxDeposits))

	// Safety decay, maximum tolerable loss of safety margin of FFG finality.
	D := params.BeaconConfig().SafetyDecay

	if T*(200+3*D) < t*(200+12*D) {
		epochsForValidatorSetChurn := N * (t*(200+12*D) - T*(200+3*D)) / (600 * delta * (2*t + T))
		epochsForBalanceTopUps := N * (200 + 3*D) / (600 * Delta)
		wsp += mathutil.Max(epochsForValidatorSetChurn, epochsForBalanceTopUps)
	} else {
		wsp += 3 * N * D * t / (200 * Delta * (T - t))
	}

	return types.Epoch(wsp), nil
}
