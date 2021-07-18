package altair

import (
	"github.com/pkg/errors"
	types "github.com/prysmaticlabs/eth2-types"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/validators"
	iface "github.com/prysmaticlabs/prysm/beacon-chain/state/interface"
	"github.com/prysmaticlabs/prysm/shared/params"
)

// SlashValidator with slashed index.
// The function  is modified to use MIN_SLASHING_PENALTY_QUOTIENT_ALTAIR and use PROPOSER_WEIGHT when calculating the proposer reward.
//
// def slash_validator(state: BeaconState,
//                    slashed_index: ValidatorIndex,
//                    whistleblower_index: ValidatorIndex=None) -> None:
//    """
//    Slash the validator with index ``slashed_index``.
//    """
//    epoch = get_current_epoch(state)
//    initiate_validator_exit(state, slashed_index)
//    validator = state.validators[slashed_index]
//    validator.slashed = True
//    validator.withdrawable_epoch = max(validator.withdrawable_epoch, Epoch(epoch + EPOCHS_PER_SLASHINGS_VECTOR))
//    state.slashings[epoch % EPOCHS_PER_SLASHINGS_VECTOR] += validator.effective_balance
//    decrease_balance(state, slashed_index, validator.effective_balance // MIN_SLASHING_PENALTY_QUOTIENT_ALTAIR)
//
//    # Apply proposer and whistleblower rewards
//    proposer_index = get_beacon_proposer_index(state)
//    if whistleblower_index is None:
//        whistleblower_index = proposer_index
//    whistleblower_reward = Gwei(validator.effective_balance // WHISTLEBLOWER_REWARD_QUOTIENT)
//    proposer_reward = Gwei(whistleblower_reward * PROPOSER_WEIGHT // WEIGHT_DENOMINATOR)
//    increase_balance(state, proposer_index, proposer_reward)
//    increase_balance(state, whistleblower_index, Gwei(whistleblower_reward - proposer_reward))
func SlashValidator(state iface.BeaconState, slashedIdx types.ValidatorIndex) (iface.BeaconState, error) {
	state, err := validators.InitiateValidatorExit(state, slashedIdx)
	if err != nil {
		return nil, errors.Wrapf(err, "could not initiate validator %d exit", slashedIdx)
	}
	currentEpoch := helpers.SlotToEpoch(state.Slot())
	validator, err := state.ValidatorAtIndex(slashedIdx)
	if err != nil {
		return nil, err
	}
	validator.Slashed = true
	maxWithdrawableEpoch := types.MaxEpoch(validator.WithdrawableEpoch, currentEpoch+params.BeaconConfig().EpochsPerSlashingsVector)
	validator.WithdrawableEpoch = maxWithdrawableEpoch

	if err := state.UpdateValidatorAtIndex(slashedIdx, validator); err != nil {
		return nil, err
	}

	// The slashing amount is represented by epochs per slashing vector. The validator's effective balance is then applied to that amount.
	slashings := state.Slashings()
	currentSlashing := slashings[currentEpoch%params.BeaconConfig().EpochsPerSlashingsVector]
	if err := state.UpdateSlashingsAtIndex(
		uint64(currentEpoch%params.BeaconConfig().EpochsPerSlashingsVector),
		currentSlashing+validator.EffectiveBalance,
	); err != nil {
		return nil, err
	}
	if err := helpers.DecreaseBalance(state, slashedIdx, validator.EffectiveBalance/params.BeaconConfig().MinSlashingPenaltyQuotientAltair); err != nil {
		return nil, err
	}

	proposerIdx, err := helpers.BeaconProposerIndex(state)
	if err != nil {
		return nil, errors.Wrap(err, "could not get proposer idx")
	}

	// In this implementation, proposer is the whistleblower.
	whistleBlowerIdx := proposerIdx
	whistleblowerReward := validator.EffectiveBalance / params.BeaconConfig().WhistleBlowerRewardQuotient
	proposerReward := whistleblowerReward * params.BeaconConfig().ProposerWeight / params.BeaconConfig().WeightDenominator
	err = helpers.IncreaseBalance(state, proposerIdx, proposerReward)
	if err != nil {
		return nil, err
	}
	err = helpers.IncreaseBalance(state, whistleBlowerIdx, whistleblowerReward-proposerReward)
	if err != nil {
		return nil, err
	}
	return state, nil
}
