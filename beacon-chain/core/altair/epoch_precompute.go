package altair

import (
	"context"

	"github.com/pkg/errors"
	types "github.com/prysmaticlabs/eth2-types"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/epoch/precompute"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	iface "github.com/prysmaticlabs/prysm/beacon-chain/state/interface"
	"github.com/prysmaticlabs/prysm/shared/mathutil"
	"github.com/prysmaticlabs/prysm/shared/params"
	"go.opencensus.io/trace"
)

// InitializeEpochValidators gets called at the beginning of process epoch cycle to return
// pre computed instances of validators attesting records and total
// balances attested in an epoch.
func InitializeEpochValidators(ctx context.Context, state iface.BeaconStateAltair) ([]*precompute.Validator, *precompute.Balance, error) {
	ctx, span := trace.StartSpan(ctx, "altair.InitializeEpochValidators")
	defer span.End()
	pValidators := make([]*precompute.Validator, state.NumValidators())
	bal := &precompute.Balance{}
	prevEpoch := helpers.PrevEpoch(state)

	inactivityScores, err := state.InactivityScores()
	if err != nil {
		return nil, nil, err
	}

	// This shouldn't happen with a correct beacon state,
	// but rather be safe to defend against index out of bound panics.
	if state.NumValidators() > len(inactivityScores) {
		return nil, nil, errors.New("num of validators can't be greater than length of inactivity scores")
	}
	if err := state.ReadFromEveryValidator(func(idx int, val iface.ReadOnlyValidator) error {
		// Was validator withdrawable or slashed
		withdrawable := prevEpoch+1 >= val.WithdrawableEpoch()
		pVal := &precompute.Validator{
			IsSlashed:                    val.Slashed(),
			IsWithdrawableCurrentEpoch:   withdrawable,
			CurrentEpochEffectiveBalance: val.EffectiveBalance(),
			InactivityScore:              inactivityScores[idx],
		}
		// Validator active current epoch
		if helpers.IsActiveValidatorUsingTrie(val, helpers.CurrentEpoch(state)) {
			pVal.IsActiveCurrentEpoch = true
			bal.ActiveCurrentEpoch += val.EffectiveBalance()
		}
		// Validator active previous epoch
		if helpers.IsActiveValidatorUsingTrie(val, prevEpoch) {
			pVal.IsActivePrevEpoch = true
			bal.ActivePrevEpoch += val.EffectiveBalance()
		}

		pValidators[idx] = pVal
		return nil
	}); err != nil {
		return nil, nil, errors.Wrap(err, "could not initialize epoch validator")
	}
	return pValidators, bal, nil
}

// ProcessInactivityScores of beacon chain. This updates inactivity scores of beacon chain and
// updates the precompute validator struct for later processing.
func ProcessInactivityScores(
	ctx context.Context,
	state iface.BeaconState,
	vals []*precompute.Validator,
) (iface.BeaconState, []*precompute.Validator, error) {
	if helpers.CurrentEpoch(state) == params.BeaconConfig().GenesisEpoch {
		return state, vals, nil
	}

	inactivityScores, err := state.InactivityScores()
	if err != nil {
		return nil, nil, err
	}

	for i, v := range vals {
		if v.IsPrevEpochTargetAttester && !v.IsSlashed {
			// Decrease inactivity score when validator gets target correct.
			if v.InactivityScore > 0 {
				score := uint64(1)
				if score > v.InactivityScore {
					score = v.InactivityScore
				}
				v.InactivityScore -= score
			}
		} else {
			v.InactivityScore += params.BeaconConfig().InactivityScoreBias
		}
		if !helpers.IsInInactivityLeak(helpers.PrevEpoch(state), state.FinalizedCheckpointEpoch()) {
			score := params.BeaconConfig().InactivityScoreRecoveryRate
			if score > v.InactivityScore {
				score = v.InactivityScore
			}
			v.InactivityScore -= score

		}
		inactivityScores[i] = v.InactivityScore
	}

	if err := state.SetInactivityScores(inactivityScores); err != nil {
		return nil, nil, err
	}

	return state, vals, nil
}

// ProcessEpochParticipation processes the epoch participation in state and updates individual validator's pre computes,
// it also tracks and updates epoch attesting balances.
func ProcessEpochParticipation(
	ctx context.Context,
	state iface.BeaconState,
	bal *precompute.Balance,
	vals []*precompute.Validator,
) ([]*precompute.Validator, *precompute.Balance, error) {
	ctx, span := trace.StartSpan(ctx, "altair.ProcessEpochParticipation")
	defer span.End()

	cp, err := state.CurrentEpochParticipation()
	if err != nil {
		return nil, nil, err
	}
	for i, b := range cp {
		if HasValidatorFlag(b, params.BeaconConfig().TimelyTargetFlagIndex) {
			vals[i].IsCurrentEpochTargetAttester = true
		}
	}
	pp, err := state.PreviousEpochParticipation()
	if err != nil {
		return nil, nil, err
	}
	for i, b := range pp {
		if HasValidatorFlag(b, params.BeaconConfig().TimelySourceFlagIndex) {
			vals[i].IsPrevEpochAttester = true
		}
		if HasValidatorFlag(b, params.BeaconConfig().TimelyTargetFlagIndex) {
			vals[i].IsPrevEpochTargetAttester = true
		}
		if HasValidatorFlag(b, params.BeaconConfig().TimelyHeadFlagIndex) {
			vals[i].IsPrevEpochHeadAttester = true
		}
	}
	bal = precompute.UpdateBalance(vals, bal)
	return vals, bal, nil
}

// ProcessRewardsAndPenaltiesPrecompute processes the rewards and penalties of individual validator.
// This is an optimized version by passing in precomputed validator attesting records and and total epoch balances.
func ProcessRewardsAndPenaltiesPrecompute(
	state iface.BeaconStateAltair,
	bal *precompute.Balance,
	vals []*precompute.Validator,
) (iface.BeaconStateAltair, error) {
	// Don't process rewards and penalties in genesis epoch.
	if helpers.CurrentEpoch(state) == 0 {
		return state, nil
	}

	numOfVals := state.NumValidators()
	// Guard against an out-of-bounds using validator balance precompute.
	if len(vals) != numOfVals || len(vals) != state.BalancesLength() {
		return state, errors.New("validator registries not the same length as state's validator registries")
	}

	attsRewards, attsPenalties, err := AttestationsDelta(state, bal, vals)
	if err != nil {
		return nil, errors.Wrap(err, "could not get attestation delta")
	}

	balances := state.Balances()
	for i := 0; i < numOfVals; i++ {
		vals[i].BeforeEpochTransitionBalance = balances[i]

		// Compute the post balance of the validator after accounting for the
		// attester and proposer rewards and penalties.
		balances[i] = helpers.IncreaseBalanceWithVal(balances[i], attsRewards[i])
		balances[i] = helpers.DecreaseBalanceWithVal(balances[i], attsPenalties[i])

		vals[i].AfterEpochTransitionBalance = balances[i]
	}

	if err := state.SetBalances(balances); err != nil {
		return nil, errors.Wrap(err, "could not set validator balances")
	}

	return state, nil
}

// AttestationsDelta computes and returns the rewards and penalties differences for individual validators based on the
// voting records.
func AttestationsDelta(state iface.BeaconStateAltair, bal *precompute.Balance, vals []*precompute.Validator) (rewards, penalties []uint64, err error) {
	numOfVals := state.NumValidators()
	rewards = make([]uint64, numOfVals)
	penalties = make([]uint64, numOfVals)
	prevEpoch := helpers.PrevEpoch(state)
	finalizedEpoch := state.FinalizedCheckpointEpoch()

	for i, v := range vals {
		rewards[i], penalties[i] = attestationDelta(bal, v, prevEpoch, finalizedEpoch)
	}

	return rewards, penalties, nil
}

func attestationDelta(bal *precompute.Balance, v *precompute.Validator, prevEpoch, finalizedEpoch types.Epoch) (r, p uint64) {
	eligible := v.IsActivePrevEpoch || (v.IsSlashed && !v.IsWithdrawableCurrentEpoch)
	if !eligible || bal.ActiveCurrentEpoch == 0 {
		return 0, 0
	}

	ebi := params.BeaconConfig().EffectiveBalanceIncrement
	eb := v.CurrentEpochEffectiveBalance
	br := (eb / ebi) * (ebi * params.BeaconConfig().BaseRewardFactor / mathutil.IntegerSquareRoot(bal.ActiveCurrentEpoch))
	activeCurrentEpochIncrements := bal.ActiveCurrentEpoch / ebi

	r, p = uint64(0), uint64(0)
	// Process source reward / penalty
	if v.IsPrevEpochAttester && !v.IsSlashed {
		if !helpers.IsInInactivityLeak(prevEpoch, finalizedEpoch) {
			rewardNumerator := br * params.BeaconConfig().TimelySourceWeight * (bal.PrevEpochAttested / ebi)
			r += rewardNumerator / (activeCurrentEpochIncrements * params.BeaconConfig().WeightDenominator)
		}
	} else {
		p += br * params.BeaconConfig().TimelySourceWeight / params.BeaconConfig().WeightDenominator
	}

	// Process target reward / penalty
	if v.IsPrevEpochTargetAttester && !v.IsSlashed {
		if !helpers.IsInInactivityLeak(prevEpoch, finalizedEpoch) {
			rewardNumerator := br * params.BeaconConfig().TimelyTargetWeight * (bal.PrevEpochTargetAttested / ebi)
			r += rewardNumerator / (activeCurrentEpochIncrements * params.BeaconConfig().WeightDenominator)
		}
	} else {
		p += br * params.BeaconConfig().TimelyTargetWeight / params.BeaconConfig().WeightDenominator
	}

	// Process head reward / penalty
	if v.IsPrevEpochHeadAttester && !v.IsSlashed {
		if !helpers.IsInInactivityLeak(prevEpoch, finalizedEpoch) {
			rewardNumerator := br * params.BeaconConfig().TimelyHeadWeight * (bal.PrevEpochHeadAttested / ebi)
			r += rewardNumerator / (activeCurrentEpochIncrements * params.BeaconConfig().WeightDenominator)
		}
	}

	// Process finality delay penalty
	// Apply an additional penalty to validators that did not vote on the correct target or slashed.
	if !v.IsPrevEpochTargetAttester || v.IsSlashed {
		penaltyNumerator := eb * v.InactivityScore
		penaltyDenominator := params.BeaconConfig().InactivityScoreBias * params.BeaconConfig().InactivityPenaltyQuotientAltair
		p += penaltyNumerator / penaltyDenominator
	}

	return r, p
}
