package altair

import (
	"github.com/pkg/errors"
	types "github.com/prysmaticlabs/eth2-types"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	iface "github.com/prysmaticlabs/prysm/beacon-chain/state/interface"
	"github.com/prysmaticlabs/prysm/shared/mathutil"
	"github.com/prysmaticlabs/prysm/shared/params"
)

// BaseReward takes state and validator index and calculate
// individual validator's base reward quotient.
//
// Spec code:
//  def get_base_reward(state: BeaconState, index: ValidatorIndex) -> Gwei:
//    """
//    Return the base reward for the validator defined by ``index`` with respect to the current ``state``.
//
//    Note: An optimally performing validator can earn one base reward per epoch over a long time horizon.
//    This takes into account both per-epoch (e.g. attestation) and intermittent duties (e.g. block proposal
//    and sync committees).
//    """
//    increments = state.validators[index].effective_balance // EFFECTIVE_BALANCE_INCREMENT
//    return Gwei(increments * get_base_reward_per_increment(state))
func BaseReward(state iface.ReadOnlyBeaconState, index types.ValidatorIndex) (uint64, error) {
	totalBalance, err := helpers.TotalActiveBalance(state)
	if err != nil {
		return 0, errors.Wrap(err, "could not calculate active balance")
	}
	return BaseRewardWithTotalBalance(state, index, totalBalance)
}

// BaseRewardWithTotalBalance calculates the base reward with the provided total balance.
func BaseRewardWithTotalBalance(state iface.ReadOnlyBeaconState, index types.ValidatorIndex, totalBalance uint64) (uint64, error) {
	val, err := state.ValidatorAtIndexReadOnly(index)
	if err != nil {
		return 0, err
	}
	increments := val.EffectiveBalance() / params.BeaconConfig().EffectiveBalanceIncrement
	return increments * baseRewardPerIncrement(totalBalance), nil
}

// baseRewardPerIncrement of the beacon state
//
// Spec code:
// def get_base_reward_per_increment(state: BeaconState) -> Gwei:
//    return Gwei(EFFECTIVE_BALANCE_INCREMENT * BASE_REWARD_FACTOR // integer_squareroot(get_total_active_balance(state)))
func baseRewardPerIncrement(activeBalance uint64) uint64 {
	return params.BeaconConfig().EffectiveBalanceIncrement * params.BeaconConfig().BaseRewardFactor / mathutil.IntegerSquareRoot(activeBalance)
}
