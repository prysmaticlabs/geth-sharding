package altair_test

import (
	"testing"

	types "github.com/prysmaticlabs/eth2-types"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/altair"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	stateAltair "github.com/prysmaticlabs/prysm/beacon-chain/state/v2"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	ethpb "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/testutil/assert"
	"github.com/prysmaticlabs/prysm/shared/testutil/require"
)

func TestSlashValidator_OK(t *testing.T) {
	validatorCount := params.BeaconConfig().MinGenesisActiveValidatorCount
	registry := make([]*ethpb.Validator, 0, validatorCount)
	balances := make([]uint64, 0, validatorCount)
	for i := uint64(0); i < validatorCount; i++ {
		registry = append(registry, &ethpb.Validator{
			ActivationEpoch:  0,
			ExitEpoch:        params.BeaconConfig().FarFutureEpoch,
			EffectiveBalance: params.BeaconConfig().MaxEffectiveBalance,
		})
		balances = append(balances, params.BeaconConfig().MaxEffectiveBalance)
	}

	base := &pb.BeaconStateAltair{
		Validators:  registry,
		Slashings:   make([]uint64, params.BeaconConfig().EpochsPerSlashingsVector),
		RandaoMixes: make([][]byte, params.BeaconConfig().EpochsPerHistoricalVector),
		Balances:    balances,
	}
	state, err := stateAltair.InitializeFromProto(base)
	require.NoError(t, err)

	slashedIdx := types.ValidatorIndex(2)

	proposer, err := helpers.BeaconProposerIndex(state)
	require.NoError(t, err, "Could not get proposer")
	proposerBal, err := state.BalanceAtIndex(proposer)
	require.NoError(t, err)
	slashedState, err := altair.SlashValidator(state, slashedIdx)
	require.NoError(t, err, "Could not slash validator")
	state, ok := slashedState.(*stateAltair.BeaconState)
	require.Equal(t, true, ok)

	v, err := state.ValidatorAtIndex(slashedIdx)
	require.NoError(t, err)
	assert.Equal(t, true, v.Slashed, "Validator not slashed despite supposed to being slashed")
	assert.Equal(t, helpers.CurrentEpoch(state)+params.BeaconConfig().EpochsPerSlashingsVector, v.WithdrawableEpoch, "Withdrawable epoch not the expected value")

	maxBalance := params.BeaconConfig().MaxEffectiveBalance
	slashedBalance := state.Slashings()[state.Slot().Mod(uint64(params.BeaconConfig().EpochsPerSlashingsVector))]
	assert.Equal(t, maxBalance, slashedBalance, "Slashed balance isnt the expected amount")

	whistleblowerReward := slashedBalance / params.BeaconConfig().WhistleBlowerRewardQuotient
	bal, err := state.BalanceAtIndex(proposer)
	require.NoError(t, err)
	// The proposer is the whistleblower in phase 0.
	assert.Equal(t, proposerBal+whistleblowerReward, bal, "Did not get expected balance for proposer")
	bal, err = state.BalanceAtIndex(slashedIdx)
	require.NoError(t, err)
	v, err = state.ValidatorAtIndex(slashedIdx)
	require.NoError(t, err)
	assert.Equal(t, maxBalance-(v.EffectiveBalance/params.BeaconConfig().MinSlashingPenaltyQuotientAltair), bal, "Did not get expected balance for slashed validator")
}
