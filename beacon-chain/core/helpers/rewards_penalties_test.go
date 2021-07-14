package helpers

import (
	"testing"

	types "github.com/prysmaticlabs/eth2-types"
	v1 "github.com/prysmaticlabs/prysm/beacon-chain/state/v1"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	ethpb "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/testutil/assert"
	"github.com/prysmaticlabs/prysm/shared/testutil/require"
)

func TestTotalBalance_OK(t *testing.T) {
	state, err := v1.InitializeFromProto(&pb.BeaconState{Validators: []*ethpb.Validator{
		{EffectiveBalance: 27 * 1e9}, {EffectiveBalance: 28 * 1e9},
		{EffectiveBalance: 32 * 1e9}, {EffectiveBalance: 40 * 1e9},
	}})
	require.NoError(t, err)

	balance := TotalBalance(state, []types.ValidatorIndex{0, 1, 2, 3})
	wanted := state.Validators()[0].EffectiveBalance + state.Validators()[1].EffectiveBalance +
		state.Validators()[2].EffectiveBalance + state.Validators()[3].EffectiveBalance
	assert.Equal(t, wanted, balance, "Incorrect TotalBalance")
}

func TestTotalBalance_ReturnsEffectiveBalanceIncrement(t *testing.T) {
	state, err := v1.InitializeFromProto(&pb.BeaconState{Validators: []*ethpb.Validator{}})
	require.NoError(t, err)

	balance := TotalBalance(state, []types.ValidatorIndex{})
	wanted := params.BeaconConfig().EffectiveBalanceIncrement
	assert.Equal(t, wanted, balance, "Incorrect TotalBalance")
}

func TestTotalActiveBalance_OK(t *testing.T) {
	state, err := v1.InitializeFromProto(&pb.BeaconState{Validators: []*ethpb.Validator{
		{
			EffectiveBalance: 32 * 1e9,
			ExitEpoch:        params.BeaconConfig().FarFutureEpoch,
		},
		{
			EffectiveBalance: 30 * 1e9,
			ExitEpoch:        params.BeaconConfig().FarFutureEpoch,
		},
		{
			EffectiveBalance: 30 * 1e9,
			ExitEpoch:        params.BeaconConfig().FarFutureEpoch,
		},
		{
			EffectiveBalance: 32 * 1e9,
			ExitEpoch:        params.BeaconConfig().FarFutureEpoch,
		},
	}})
	require.NoError(t, err)

	balance, err := TotalActiveBalance(state)
	assert.NoError(t, err)
	wanted := state.Validators()[0].EffectiveBalance + state.Validators()[1].EffectiveBalance +
		state.Validators()[2].EffectiveBalance + state.Validators()[3].EffectiveBalance
	assert.Equal(t, wanted, balance, "Incorrect TotalActiveBalance")
}

func TestGetBalance_OK(t *testing.T) {
	tests := []struct {
		i uint64
		b []uint64
	}{
		{i: 0, b: []uint64{27 * 1e9, 28 * 1e9, 32 * 1e9}},
		{i: 1, b: []uint64{27 * 1e9, 28 * 1e9, 32 * 1e9}},
		{i: 2, b: []uint64{27 * 1e9, 28 * 1e9, 32 * 1e9}},
		{i: 0, b: []uint64{0, 0, 0}},
		{i: 2, b: []uint64{0, 0, 0}},
	}
	for _, test := range tests {
		state, err := v1.InitializeFromProto(&pb.BeaconState{Balances: test.b})
		require.NoError(t, err)
		assert.Equal(t, test.b[test.i], state.Balances()[test.i], "Incorrect Validator balance")
	}
}

func TestIncreaseBalance_OK(t *testing.T) {
	tests := []struct {
		i  types.ValidatorIndex
		b  []uint64
		nb uint64
		eb uint64
	}{
		{i: 0, b: []uint64{27 * 1e9, 28 * 1e9, 32 * 1e9}, nb: 1, eb: 27*1e9 + 1},
		{i: 1, b: []uint64{27 * 1e9, 28 * 1e9, 32 * 1e9}, nb: 0, eb: 28 * 1e9},
		{i: 2, b: []uint64{27 * 1e9, 28 * 1e9, 32 * 1e9}, nb: 33 * 1e9, eb: 65 * 1e9},
	}
	for _, test := range tests {
		state, err := v1.InitializeFromProto(&pb.BeaconState{
			Validators: []*ethpb.Validator{
				{EffectiveBalance: 4}, {EffectiveBalance: 4}, {EffectiveBalance: 4}},
			Balances: test.b,
		})
		require.NoError(t, err)
		require.NoError(t, IncreaseBalance(state, test.i, test.nb))
		assert.Equal(t, test.eb, state.Balances()[test.i], "Incorrect Validator balance")
	}
}

func TestDecreaseBalance_OK(t *testing.T) {
	tests := []struct {
		i  types.ValidatorIndex
		b  []uint64
		nb uint64
		eb uint64
	}{
		{i: 0, b: []uint64{2, 28 * 1e9, 32 * 1e9}, nb: 1, eb: 1},
		{i: 1, b: []uint64{27 * 1e9, 28 * 1e9, 32 * 1e9}, nb: 0, eb: 28 * 1e9},
		{i: 2, b: []uint64{27 * 1e9, 28 * 1e9, 1}, nb: 2, eb: 0},
		{i: 3, b: []uint64{27 * 1e9, 28 * 1e9, 1, 28 * 1e9}, nb: 28 * 1e9, eb: 0},
	}
	for _, test := range tests {
		state, err := v1.InitializeFromProto(&pb.BeaconState{
			Validators: []*ethpb.Validator{
				{EffectiveBalance: 4}, {EffectiveBalance: 4}, {EffectiveBalance: 4}, {EffectiveBalance: 3}},
			Balances: test.b,
		})
		require.NoError(t, err)
		require.NoError(t, DecreaseBalance(state, test.i, test.nb))
		assert.Equal(t, test.eb, state.Balances()[test.i], "Incorrect Validator balance")
	}
}

func TestFinalityDelay(t *testing.T) {
	base := buildState(params.BeaconConfig().SlotsPerEpoch*10, 1)
	base.FinalizedCheckpoint = &ethpb.Checkpoint{Epoch: 3}
	beaconState, err := v1.InitializeFromProto(base)
	require.NoError(t, err)
	prevEpoch := types.Epoch(0)
	finalizedEpoch := types.Epoch(0)
	// Set values for each test case
	setVal := func() {
		prevEpoch = PrevEpoch(beaconState)
		finalizedEpoch = beaconState.FinalizedCheckpointEpoch()
	}
	setVal()
	d := FinalityDelay(prevEpoch, finalizedEpoch)
	w := PrevEpoch(beaconState) - beaconState.FinalizedCheckpointEpoch()
	assert.Equal(t, w, d, "Did not get wanted finality delay")

	require.NoError(t, beaconState.SetFinalizedCheckpoint(&ethpb.Checkpoint{Epoch: 4}))
	setVal()
	d = FinalityDelay(prevEpoch, finalizedEpoch)
	w = PrevEpoch(beaconState) - beaconState.FinalizedCheckpointEpoch()
	assert.Equal(t, w, d, "Did not get wanted finality delay")

	require.NoError(t, beaconState.SetFinalizedCheckpoint(&ethpb.Checkpoint{Epoch: 5}))
	setVal()
	d = FinalityDelay(prevEpoch, finalizedEpoch)
	w = PrevEpoch(beaconState) - beaconState.FinalizedCheckpointEpoch()
	assert.Equal(t, w, d, "Did not get wanted finality delay")
}

func TestIsInInactivityLeak(t *testing.T) {
	base := buildState(params.BeaconConfig().SlotsPerEpoch*10, 1)
	base.FinalizedCheckpoint = &ethpb.Checkpoint{Epoch: 3}
	beaconState, err := v1.InitializeFromProto(base)
	require.NoError(t, err)
	prevEpoch := types.Epoch(0)
	finalizedEpoch := types.Epoch(0)
	// Set values for each test case
	setVal := func() {
		prevEpoch = PrevEpoch(beaconState)
		finalizedEpoch = beaconState.FinalizedCheckpointEpoch()
	}
	setVal()
	assert.Equal(t, true, IsInInactivityLeak(prevEpoch, finalizedEpoch), "Wanted inactivity leak true")
	require.NoError(t, beaconState.SetFinalizedCheckpoint(&ethpb.Checkpoint{Epoch: 4}))
	setVal()
	assert.Equal(t, true, IsInInactivityLeak(prevEpoch, finalizedEpoch), "Wanted inactivity leak true")
	require.NoError(t, beaconState.SetFinalizedCheckpoint(&ethpb.Checkpoint{Epoch: 5}))
	setVal()
	assert.Equal(t, false, IsInInactivityLeak(prevEpoch, finalizedEpoch), "Wanted inactivity leak false")
}

func buildState(slot types.Slot, validatorCount uint64) *pb.BeaconState {
	validators := make([]*ethpb.Validator, validatorCount)
	for i := 0; i < len(validators); i++ {
		validators[i] = &ethpb.Validator{
			ExitEpoch:        params.BeaconConfig().FarFutureEpoch,
			EffectiveBalance: params.BeaconConfig().MaxEffectiveBalance,
		}
	}
	validatorBalances := make([]uint64, len(validators))
	for i := 0; i < len(validatorBalances); i++ {
		validatorBalances[i] = params.BeaconConfig().MaxEffectiveBalance
	}
	latestActiveIndexRoots := make(
		[][]byte,
		params.BeaconConfig().EpochsPerHistoricalVector,
	)
	for i := 0; i < len(latestActiveIndexRoots); i++ {
		latestActiveIndexRoots[i] = params.BeaconConfig().ZeroHash[:]
	}
	latestRandaoMixes := make(
		[][]byte,
		params.BeaconConfig().EpochsPerHistoricalVector,
	)
	for i := 0; i < len(latestRandaoMixes); i++ {
		latestRandaoMixes[i] = params.BeaconConfig().ZeroHash[:]
	}
	return &pb.BeaconState{
		Slot:                        slot,
		Balances:                    validatorBalances,
		Validators:                  validators,
		RandaoMixes:                 make([][]byte, params.BeaconConfig().EpochsPerHistoricalVector),
		Slashings:                   make([]uint64, params.BeaconConfig().EpochsPerSlashingsVector),
		BlockRoots:                  make([][]byte, params.BeaconConfig().SlotsPerEpoch*10),
		FinalizedCheckpoint:         &ethpb.Checkpoint{Root: make([]byte, 32)},
		PreviousJustifiedCheckpoint: &ethpb.Checkpoint{Root: make([]byte, 32)},
		CurrentJustifiedCheckpoint:  &ethpb.Checkpoint{Root: make([]byte, 32)},
	}
}
