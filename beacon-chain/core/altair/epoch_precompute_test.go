package altair

import (
	"context"
	"testing"

	types "github.com/prysmaticlabs/eth2-types"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/epoch/precompute"
	iface "github.com/prysmaticlabs/prysm/beacon-chain/state/interface"
	stateAltair "github.com/prysmaticlabs/prysm/beacon-chain/state/v2"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	ethpb "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/testutil/assert"
	"github.com/prysmaticlabs/prysm/shared/testutil/require"
)

func TestInitializeEpochValidators_Ok(t *testing.T) {
	ffe := params.BeaconConfig().FarFutureEpoch
	s, err := stateAltair.InitializeFromProto(&pb.BeaconStateAltair{
		Slot: params.BeaconConfig().SlotsPerEpoch,
		// Validator 0 is slashed
		// Validator 1 is withdrawable
		// Validator 2 is active prev epoch and current epoch
		// Validator 3 is active prev epoch
		Validators: []*ethpb.Validator{
			{Slashed: true, WithdrawableEpoch: ffe, EffectiveBalance: 100},
			{EffectiveBalance: 100},
			{WithdrawableEpoch: ffe, ExitEpoch: ffe, EffectiveBalance: 100},
			{WithdrawableEpoch: ffe, ExitEpoch: 1, EffectiveBalance: 100},
		},
		InactivityScores: []uint64{0, 1, 2, 3},
	})
	require.NoError(t, err)
	v, b, err := InitializeEpochValidators(context.Background(), s)
	require.NoError(t, err)
	assert.DeepEqual(t, &precompute.Validator{
		IsSlashed:                    true,
		CurrentEpochEffectiveBalance: 100,
		InactivityScore:              0,
	}, v[0], "Incorrect validator 0 status")
	assert.DeepEqual(t, &precompute.Validator{
		IsWithdrawableCurrentEpoch:   true,
		CurrentEpochEffectiveBalance: 100,
		InactivityScore:              1,
	}, v[1], "Incorrect validator 1 status")
	assert.DeepEqual(t, &precompute.Validator{
		IsActivePrevEpoch:            true,
		IsActiveCurrentEpoch:         true,
		CurrentEpochEffectiveBalance: 100,
		InactivityScore:              2,
	}, v[2], "Incorrect validator 2 status")
	assert.DeepEqual(t, &precompute.Validator{
		IsActivePrevEpoch:            true,
		CurrentEpochEffectiveBalance: 100,
		InactivityScore:              3,
	}, v[3], "Incorrect validator 3 status")

	wantedBalances := &precompute.Balance{
		ActiveCurrentEpoch: 100,
		ActivePrevEpoch:    200,
	}
	assert.DeepEqual(t, wantedBalances, b, "Incorrect wanted balance")
}

func TestInitializeEpochValidators_BadState(t *testing.T) {
	s, err := stateAltair.InitializeFromProto(&pb.BeaconStateAltair{
		Validators:       []*ethpb.Validator{{}},
		InactivityScores: []uint64{},
	})
	require.NoError(t, err)
	_, _, err = InitializeEpochValidators(context.Background(), s)
	require.ErrorContains(t, "num of validators can't be greater than length of inactivity scores", err)
}

func TestProcessEpochParticipation(t *testing.T) {
	s, err := testState()
	require.NoError(t, err)
	validators, balance, err := InitializeEpochValidators(context.Background(), s)
	require.NoError(t, err)
	validators, balance, err = ProcessEpochParticipation(context.Background(), s, balance, validators)
	require.NoError(t, err)
	require.DeepEqual(t, &precompute.Validator{
		IsActiveCurrentEpoch:         true,
		IsActivePrevEpoch:            true,
		IsWithdrawableCurrentEpoch:   true,
		CurrentEpochEffectiveBalance: params.BeaconConfig().MaxEffectiveBalance,
	}, validators[0])
	require.DeepEqual(t, &precompute.Validator{
		IsActiveCurrentEpoch:         true,
		IsActivePrevEpoch:            true,
		IsWithdrawableCurrentEpoch:   true,
		CurrentEpochEffectiveBalance: params.BeaconConfig().MaxEffectiveBalance,
		IsPrevEpochAttester:          true,
	}, validators[1])
	require.DeepEqual(t, &precompute.Validator{
		IsActiveCurrentEpoch:         true,
		IsActivePrevEpoch:            true,
		IsWithdrawableCurrentEpoch:   true,
		CurrentEpochEffectiveBalance: params.BeaconConfig().MaxEffectiveBalance,
		IsPrevEpochAttester:          true,
		IsCurrentEpochTargetAttester: true,
		IsPrevEpochTargetAttester:    true,
	}, validators[2])
	require.DeepEqual(t, &precompute.Validator{
		IsActiveCurrentEpoch:         true,
		IsActivePrevEpoch:            true,
		IsWithdrawableCurrentEpoch:   true,
		CurrentEpochEffectiveBalance: params.BeaconConfig().MaxEffectiveBalance,
		IsPrevEpochAttester:          true,
		IsCurrentEpochTargetAttester: true,
		IsPrevEpochTargetAttester:    true,
		IsPrevEpochHeadAttester:      true,
	}, validators[3])
	require.Equal(t, params.BeaconConfig().MaxEffectiveBalance*3, balance.PrevEpochAttested)
	require.Equal(t, balance.CurrentEpochTargetAttested, params.BeaconConfig().MaxEffectiveBalance*2)
	require.Equal(t, balance.PrevEpochTargetAttested, params.BeaconConfig().MaxEffectiveBalance*2)
	require.Equal(t, balance.PrevEpochHeadAttested, params.BeaconConfig().MaxEffectiveBalance*1)
}

func TestAttestationsDelta(t *testing.T) {
	s, err := testState()
	require.NoError(t, err)
	validators, balance, err := InitializeEpochValidators(context.Background(), s)
	require.NoError(t, err)
	validators, balance, err = ProcessEpochParticipation(context.Background(), s, balance, validators)
	require.NoError(t, err)
	rewards, penalties, err := AttestationsDelta(s, balance, validators)
	require.NoError(t, err)

	// Reward amount should increase as validator index increases due to setup.
	for i := 1; i < len(rewards); i++ {
		require.Equal(t, true, rewards[i] > rewards[i-1])
	}

	// Penalty amount should decrease as validator index increases due to setup.
	for i := 1; i < len(penalties); i++ {
		require.Equal(t, true, penalties[i] <= penalties[i-1])
	}

	// First index should have 0 reward.
	require.Equal(t, uint64(0), rewards[0])
	// Last index should have 0 penalty.
	require.Equal(t, uint64(0), penalties[len(penalties)-1])
}

func TestProcessRewardsAndPenaltiesPrecompute_Ok(t *testing.T) {
	s, err := testState()
	require.NoError(t, err)
	validators, balance, err := InitializeEpochValidators(context.Background(), s)
	require.NoError(t, err)
	validators, balance, err = ProcessEpochParticipation(context.Background(), s, balance, validators)
	require.NoError(t, err)
	s, err = ProcessRewardsAndPenaltiesPrecompute(s, balance, validators)
	require.NoError(t, err)

	balances := s.Balances()
	// Reward amount should increase as validator index increases due to setup.
	for i := 1; i < len(balances); i++ {
		require.Equal(t, true, balances[i] >= balances[i-1])
	}

	wanted := make([]uint64, s.NumValidators())
	rewards, penalties, err := AttestationsDelta(s, balance, validators)
	require.NoError(t, err)
	for i := range rewards {
		wanted[i] += rewards[i]
	}
	for i := range penalties {
		if wanted[i] > penalties[i] {
			wanted[i] -= penalties[i]
		} else {
			wanted[i] = 0
		}
	}
	require.DeepEqual(t, wanted, balances)
}

func TestProcessRewardsAndPenaltiesPrecompute_InactivityLeak(t *testing.T) {
	s, err := testState()
	require.NoError(t, err)
	validators, balance, err := InitializeEpochValidators(context.Background(), s)
	require.NoError(t, err)
	validators, balance, err = ProcessEpochParticipation(context.Background(), s, balance, validators)
	require.NoError(t, err)
	sCopy := s.Copy()
	s, err = ProcessRewardsAndPenaltiesPrecompute(s, balance, validators)
	require.NoError(t, err)

	// Copied state where finality happened long ago
	require.NoError(t, sCopy.SetSlot(params.BeaconConfig().SlotsPerEpoch*1000))
	sCopy, err = ProcessRewardsAndPenaltiesPrecompute(sCopy, balance, validators)
	require.NoError(t, err)

	balances := s.Balances()
	inactivityBalances := sCopy.Balances()
	// Balances should be much less in inactivity leak cases.
	for i := 0; i < len(balances); i++ {
		require.Equal(t, true, balances[i] >= inactivityBalances[i])
	}
}

func TestProcessInactivityScores_CanProcess(t *testing.T) {
	s, err := testState()
	require.NoError(t, err)
	defaultScore := uint64(5)
	require.NoError(t, s.SetInactivityScores([]uint64{defaultScore, defaultScore, defaultScore, defaultScore}))
	require.NoError(t, s.SetSlot(params.BeaconConfig().SlotsPerEpoch*types.Slot(params.BeaconConfig().MinEpochsToInactivityPenalty+2)))
	validators, balance, err := InitializeEpochValidators(context.Background(), s)
	require.NoError(t, err)
	validators, _, err = ProcessEpochParticipation(context.Background(), s, balance, validators)
	require.NoError(t, err)
	s, _, err = ProcessInactivityScores(context.Background(), s, validators)
	require.NoError(t, err)
	inactivityScores, err := s.InactivityScores()
	require.NoError(t, err)
	// V0 and V1 didn't vote head. V2 and V3 did.
	require.Equal(t, defaultScore+params.BeaconConfig().InactivityScoreBias, inactivityScores[0])
	require.Equal(t, defaultScore+params.BeaconConfig().InactivityScoreBias, inactivityScores[1])
	require.Equal(t, defaultScore-1, inactivityScores[2])
	require.Equal(t, defaultScore-1, inactivityScores[3])
}

func TestProcessRewardsAndPenaltiesPrecompute_GenesisEpoch(t *testing.T) {
	s, err := testState()
	require.NoError(t, err)
	validators, balance, err := InitializeEpochValidators(context.Background(), s)
	require.NoError(t, err)
	validators, balance, err = ProcessEpochParticipation(context.Background(), s, balance, validators)
	require.NoError(t, err)
	require.NoError(t, s.SetSlot(0))
	s, err = ProcessRewardsAndPenaltiesPrecompute(s, balance, validators)
	require.NoError(t, err)

	balances := s.Balances()
	// Nothing should happen at genesis epoch
	for i := 1; i < len(balances); i++ {
		require.Equal(t, true, balances[i] == balances[i-1])
	}
}

func TestProcessRewardsAndPenaltiesPrecompute_BadState(t *testing.T) {
	s, err := testState()
	require.NoError(t, err)
	validators, balance, err := InitializeEpochValidators(context.Background(), s)
	require.NoError(t, err)
	_, balance, err = ProcessEpochParticipation(context.Background(), s, balance, validators)
	require.NoError(t, err)
	_, err = ProcessRewardsAndPenaltiesPrecompute(s, balance, []*precompute.Validator{})
	require.ErrorContains(t, "validator registries not the same length as state's validator registries", err)
}

func testState() (iface.BeaconState, error) {
	generateParticipation := func(flags ...uint8) byte {
		b := byte(0)
		for _, flag := range flags {
			b = AddValidatorFlag(b, flag)
		}
		return b
	}
	return stateAltair.InitializeFromProto(&pb.BeaconStateAltair{
		Slot: 2 * params.BeaconConfig().SlotsPerEpoch,
		Validators: []*ethpb.Validator{
			{EffectiveBalance: params.BeaconConfig().MaxEffectiveBalance, ExitEpoch: params.BeaconConfig().FarFutureEpoch},
			{EffectiveBalance: params.BeaconConfig().MaxEffectiveBalance, ExitEpoch: params.BeaconConfig().FarFutureEpoch},
			{EffectiveBalance: params.BeaconConfig().MaxEffectiveBalance, ExitEpoch: params.BeaconConfig().FarFutureEpoch},
			{EffectiveBalance: params.BeaconConfig().MaxEffectiveBalance, ExitEpoch: params.BeaconConfig().FarFutureEpoch},
		},
		CurrentEpochParticipation: []byte{
			0,
			generateParticipation(params.BeaconConfig().TimelySourceFlagIndex),
			generateParticipation(params.BeaconConfig().TimelySourceFlagIndex, params.BeaconConfig().TimelyTargetFlagIndex),
			generateParticipation(params.BeaconConfig().TimelySourceFlagIndex, params.BeaconConfig().TimelyTargetFlagIndex, params.BeaconConfig().TimelyHeadFlagIndex),
		},
		PreviousEpochParticipation: []byte{
			0,
			generateParticipation(params.BeaconConfig().TimelySourceFlagIndex),
			generateParticipation(params.BeaconConfig().TimelySourceFlagIndex, params.BeaconConfig().TimelyTargetFlagIndex),
			generateParticipation(params.BeaconConfig().TimelySourceFlagIndex, params.BeaconConfig().TimelyTargetFlagIndex, params.BeaconConfig().TimelyHeadFlagIndex),
		},
		InactivityScores: []uint64{0, 0, 0, 0},
		Balances:         []uint64{0, 0, 0, 0},
	})
}
