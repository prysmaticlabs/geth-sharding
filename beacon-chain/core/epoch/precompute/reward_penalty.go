package precompute

import (
	"github.com/pkg/errors"
	types "github.com/prysmaticlabs/eth2-types"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	iface "github.com/prysmaticlabs/prysm/beacon-chain/state/interface"
	"github.com/prysmaticlabs/prysm/shared/mathutil"
	"github.com/prysmaticlabs/prysm/shared/params"
)

// ProcessRewardsAndPenaltiesPrecompute processes the rewards and penalties of individual validator.
// This is an optimized version by passing in precomputed validator attesting records and and total epoch balances.
func ProcessRewardsAndPenaltiesPrecompute(
	state iface.BeaconState,
	pBal *Balance,
	vp []*Validator,
) (iface.BeaconState, error) {
	// Can't process rewards and penalties in genesis epoch.
	if helpers.CurrentEpoch(state) == 0 {
		return state, nil
	}

	numOfVals := state.NumValidators()
	// Guard against an out-of-bounds using validator balance precompute.
	if len(vp) != numOfVals || len(vp) != state.BalancesLength() {
		return state, errors.New("precomputed registries not the same length as state registries")
	}

	attsRewards, attsPenalties, err := AttestationsDelta(state, pBal, vp)
	if err != nil {
		return nil, errors.Wrap(err, "could not get attestation delta")
	}
	proposerRewards, err := ProposersDelta(state, pBal, vp)
	if err != nil {
		return nil, errors.Wrap(err, "could not get attestation delta")
	}
	validatorBals := state.Balances()
	for i := 0; i < numOfVals; i++ {
		vp[i].BeforeEpochTransitionBalance = validatorBals[i]

		// Compute the post balance of the validator after accounting for the
		// attester and proposer rewards and penalties.
		validatorBals[i] = helpers.IncreaseBalanceWithVal(validatorBals[i], attsRewards[i]+proposerRewards[i])
		validatorBals[i] = helpers.DecreaseBalanceWithVal(validatorBals[i], attsPenalties[i])

		vp[i].AfterEpochTransitionBalance = validatorBals[i]
	}

	if err := state.SetBalances(validatorBals); err != nil {
		return nil, errors.Wrap(err, "could not set validator balances")
	}

	return state, nil
}

// AttestationsDelta computes and returns the rewards and penalties differences for individual validators based on the
// voting records.
func AttestationsDelta(state iface.ReadOnlyBeaconState, pBal *Balance, vp []*Validator) ([]uint64, []uint64, error) {
	numOfVals := state.NumValidators()
	rewards := make([]uint64, numOfVals)
	penalties := make([]uint64, numOfVals)
	prevEpoch := helpers.PrevEpoch(state)
	finalizedEpoch := state.FinalizedCheckpointEpoch()

	for i, v := range vp {
		rewards[i], penalties[i] = attestationDelta(pBal, v, prevEpoch, finalizedEpoch)
	}
	return rewards, penalties, nil
}

func attestationDelta(pBal *Balance, v *Validator, prevEpoch, finalizedEpoch types.Epoch) (uint64, uint64) {
	if !EligibleForRewards(v) || pBal.ActiveCurrentEpoch == 0 {
		return 0, 0
	}

	baseRewardsPerEpoch := params.BeaconConfig().BaseRewardsPerEpoch
	effectiveBalanceIncrement := params.BeaconConfig().EffectiveBalanceIncrement
	vb := v.CurrentEpochEffectiveBalance
	br := vb * params.BeaconConfig().BaseRewardFactor / mathutil.IntegerSquareRoot(pBal.ActiveCurrentEpoch) / baseRewardsPerEpoch
	r, p := uint64(0), uint64(0)
	currentEpochBalance := pBal.ActiveCurrentEpoch / effectiveBalanceIncrement

	// Process source reward / penalty
	if v.IsPrevEpochAttester && !v.IsSlashed {
		proposerReward := br / params.BeaconConfig().ProposerRewardQuotient
		maxAttesterReward := br - proposerReward
		r += maxAttesterReward / uint64(v.InclusionDistance)

		if helpers.IsInInactivityLeak(prevEpoch, finalizedEpoch) {
			// Since full base reward will be canceled out by inactivity penalty deltas,
			// optimal participation receives full base reward compensation here.
			r += br
		} else {
			rewardNumerator := br * (pBal.PrevEpochAttested / effectiveBalanceIncrement)
			r += rewardNumerator / currentEpochBalance

		}
	} else {
		p += br
	}

	// Process target reward / penalty
	if v.IsPrevEpochTargetAttester && !v.IsSlashed {
		if helpers.IsInInactivityLeak(prevEpoch, finalizedEpoch) {
			// Since full base reward will be canceled out by inactivity penalty deltas,
			// optimal participation receives full base reward compensation here.
			r += br
		} else {
			rewardNumerator := br * (pBal.PrevEpochTargetAttested / effectiveBalanceIncrement)
			r += rewardNumerator / currentEpochBalance
		}
	} else {
		p += br
	}

	// Process head reward / penalty
	if v.IsPrevEpochHeadAttester && !v.IsSlashed {
		if helpers.IsInInactivityLeak(prevEpoch, finalizedEpoch) {
			// Since full base reward will be canceled out by inactivity penalty deltas,
			// optimal participation receives full base reward compensation here.
			r += br
		} else {
			rewardNumerator := br * (pBal.PrevEpochHeadAttested / effectiveBalanceIncrement)
			r += rewardNumerator / currentEpochBalance
		}
	} else {
		p += br
	}

	// Process finality delay penalty
	finalityDelay := helpers.FinalityDelay(prevEpoch, finalizedEpoch)

	if helpers.IsInInactivityLeak(prevEpoch, finalizedEpoch) {
		// If validator is performing optimally, this cancels all rewards for a neutral balance.
		proposerReward := br / params.BeaconConfig().ProposerRewardQuotient
		p += baseRewardsPerEpoch*br - proposerReward
		// Apply an additional penalty to validators that did not vote on the correct target or has been slashed.
		// Equivalent to the following condition from the spec:
		// `index not in get_unslashed_attesting_indices(state, matching_target_attestations)`
		if !v.IsPrevEpochTargetAttester || v.IsSlashed {
			p += vb * uint64(finalityDelay) / params.BeaconConfig().InactivityPenaltyQuotient
		}
	}
	return r, p
}

// ProposersDelta computes and returns the rewards and penalties differences for individual validators based on the
// proposer inclusion records.
func ProposersDelta(state iface.ReadOnlyBeaconState, pBal *Balance, vp []*Validator) ([]uint64, error) {
	numofVals := state.NumValidators()
	rewards := make([]uint64, numofVals)

	totalBalance := pBal.ActiveCurrentEpoch
	balanceSqrt := mathutil.IntegerSquareRoot(totalBalance)
	// Balance square root cannot be 0, this prevents division by 0.
	if balanceSqrt == 0 {
		balanceSqrt = 1
	}

	baseRewardFactor := params.BeaconConfig().BaseRewardFactor
	baseRewardsPerEpoch := params.BeaconConfig().BaseRewardsPerEpoch
	proposerRewardQuotient := params.BeaconConfig().ProposerRewardQuotient
	for _, v := range vp {
		if uint64(v.ProposerIndex) >= uint64(len(rewards)) {
			// This should never happen with a valid state / validator.
			return nil, errors.New("proposer index out of range")
		}
		// Only apply inclusion rewards to proposer only if the attested hasn't been slashed.
		if v.IsPrevEpochAttester && !v.IsSlashed {
			vBalance := v.CurrentEpochEffectiveBalance
			baseReward := vBalance * baseRewardFactor / balanceSqrt / baseRewardsPerEpoch
			proposerReward := baseReward / proposerRewardQuotient
			rewards[v.ProposerIndex] += proposerReward
		}
	}
	return rewards, nil
}

// EligibleForRewards for validator.
//
// Spec code:
// if is_active_validator(v, previous_epoch) or (v.slashed and previous_epoch + 1 < v.withdrawable_epoch)
func EligibleForRewards(v *Validator) bool {
	return v.IsActivePrevEpoch || (v.IsSlashed && !v.IsWithdrawableCurrentEpoch)
}
