package helpers

import (
	"errors"
	"testing"

	types "github.com/prysmaticlabs/eth2-types"
	"github.com/prysmaticlabs/prysm/beacon-chain/cache"
	v1 "github.com/prysmaticlabs/prysm/beacon-chain/state/v1"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	ethpb "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/prysmaticlabs/prysm/shared/hashutil"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/testutil/assert"
	"github.com/prysmaticlabs/prysm/shared/testutil/require"
)

func TestIsActiveValidator_OK(t *testing.T) {
	tests := []struct {
		a types.Epoch
		b bool
	}{
		{a: 0, b: false},
		{a: 10, b: true},
		{a: 100, b: false},
		{a: 1000, b: false},
		{a: 64, b: true},
	}
	for _, test := range tests {
		validator := &ethpb.Validator{ActivationEpoch: 10, ExitEpoch: 100}
		assert.Equal(t, test.b, IsActiveValidator(validator, test.a), "IsActiveValidator(%d)", test.a)
	}
}

func TestIsActiveValidatorUsingTrie_OK(t *testing.T) {
	tests := []struct {
		a types.Epoch
		b bool
	}{
		{a: 0, b: false},
		{a: 10, b: true},
		{a: 100, b: false},
		{a: 1000, b: false},
		{a: 64, b: true},
	}
	val := &ethpb.Validator{ActivationEpoch: 10, ExitEpoch: 100}
	beaconState, err := v1.InitializeFromProto(&pb.BeaconState{Validators: []*ethpb.Validator{val}})
	require.NoError(t, err)
	for _, test := range tests {
		readOnlyVal, err := beaconState.ValidatorAtIndexReadOnly(0)
		require.NoError(t, err)
		assert.Equal(t, test.b, IsActiveValidatorUsingTrie(readOnlyVal, test.a), "IsActiveValidatorUsingTrie(%d)", test.a)
	}
}

func TestIsSlashableValidator_OK(t *testing.T) {
	tests := []struct {
		name      string
		validator *ethpb.Validator
		epoch     types.Epoch
		slashable bool
	}{
		{
			name: "Unset withdrawable, slashable",
			validator: &ethpb.Validator{
				WithdrawableEpoch: params.BeaconConfig().FarFutureEpoch,
			},
			epoch:     0,
			slashable: true,
		},
		{
			name: "before withdrawable, slashable",
			validator: &ethpb.Validator{
				WithdrawableEpoch: 5,
			},
			epoch:     3,
			slashable: true,
		},
		{
			name: "inactive, not slashable",
			validator: &ethpb.Validator{
				ActivationEpoch:   5,
				WithdrawableEpoch: params.BeaconConfig().FarFutureEpoch,
			},
			epoch:     2,
			slashable: false,
		},
		{
			name: "after withdrawable, not slashable",
			validator: &ethpb.Validator{
				WithdrawableEpoch: 3,
			},
			epoch:     3,
			slashable: false,
		},
		{
			name: "slashed and withdrawable, not slashable",
			validator: &ethpb.Validator{
				Slashed:           true,
				ExitEpoch:         params.BeaconConfig().FarFutureEpoch,
				WithdrawableEpoch: 1,
			},
			epoch:     2,
			slashable: false,
		},
		{
			name: "slashed, not slashable",
			validator: &ethpb.Validator{
				Slashed:           true,
				ExitEpoch:         params.BeaconConfig().FarFutureEpoch,
				WithdrawableEpoch: params.BeaconConfig().FarFutureEpoch,
			},
			epoch:     2,
			slashable: false,
		},
		{
			name: "inactive and slashed, not slashable",
			validator: &ethpb.Validator{
				Slashed:           true,
				ActivationEpoch:   4,
				ExitEpoch:         params.BeaconConfig().FarFutureEpoch,
				WithdrawableEpoch: params.BeaconConfig().FarFutureEpoch,
			},
			epoch:     2,
			slashable: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			slashableValidator := IsSlashableValidator(test.validator.ActivationEpoch,
				test.validator.WithdrawableEpoch, test.validator.Slashed, test.epoch)
			assert.Equal(t, test.slashable, slashableValidator, "Expected active validator slashable to be %t", test.slashable)
		})
	}
}

func TestIsSlashableValidatorUsingTrie_OK(t *testing.T) {
	tests := []struct {
		name      string
		validator *ethpb.Validator
		epoch     types.Epoch
		slashable bool
	}{
		{
			name: "Unset withdrawable, slashable",
			validator: &ethpb.Validator{
				WithdrawableEpoch: params.BeaconConfig().FarFutureEpoch,
			},
			epoch:     0,
			slashable: true,
		},
		{
			name: "before withdrawable, slashable",
			validator: &ethpb.Validator{
				WithdrawableEpoch: 5,
			},
			epoch:     3,
			slashable: true,
		},
		{
			name: "inactive, not slashable",
			validator: &ethpb.Validator{
				ActivationEpoch:   5,
				WithdrawableEpoch: params.BeaconConfig().FarFutureEpoch,
			},
			epoch:     2,
			slashable: false,
		},
		{
			name: "after withdrawable, not slashable",
			validator: &ethpb.Validator{
				WithdrawableEpoch: 3,
			},
			epoch:     3,
			slashable: false,
		},
		{
			name: "slashed and withdrawable, not slashable",
			validator: &ethpb.Validator{
				Slashed:           true,
				ExitEpoch:         params.BeaconConfig().FarFutureEpoch,
				WithdrawableEpoch: 1,
			},
			epoch:     2,
			slashable: false,
		},
		{
			name: "slashed, not slashable",
			validator: &ethpb.Validator{
				Slashed:           true,
				ExitEpoch:         params.BeaconConfig().FarFutureEpoch,
				WithdrawableEpoch: params.BeaconConfig().FarFutureEpoch,
			},
			epoch:     2,
			slashable: false,
		},
		{
			name: "inactive and slashed, not slashable",
			validator: &ethpb.Validator{
				Slashed:           true,
				ActivationEpoch:   4,
				ExitEpoch:         params.BeaconConfig().FarFutureEpoch,
				WithdrawableEpoch: params.BeaconConfig().FarFutureEpoch,
			},
			epoch:     2,
			slashable: false,
		},
	}

	for _, test := range tests {
		beaconState, err := v1.InitializeFromProto(&pb.BeaconState{Validators: []*ethpb.Validator{test.validator}})
		require.NoError(t, err)
		readOnlyVal, err := beaconState.ValidatorAtIndexReadOnly(0)
		require.NoError(t, err)
		t.Run(test.name, func(t *testing.T) {
			slashableValidator := IsSlashableValidatorUsingTrie(readOnlyVal, test.epoch)
			assert.Equal(t, test.slashable, slashableValidator, "Expected active validator slashable to be %t", test.slashable)
		})
	}
}

func TestBeaconProposerIndex_OK(t *testing.T) {
	params.SetupTestConfigCleanup(t)
	ClearCache()
	c := params.BeaconConfig()
	c.MinGenesisActiveValidatorCount = 16384
	params.OverrideBeaconConfig(c)
	validators := make([]*ethpb.Validator, params.BeaconConfig().MinGenesisActiveValidatorCount/8)
	for i := 0; i < len(validators); i++ {
		validators[i] = &ethpb.Validator{
			ExitEpoch: params.BeaconConfig().FarFutureEpoch,
		}
	}

	state, err := v1.InitializeFromProto(&pb.BeaconState{
		Validators:  validators,
		Slot:        0,
		RandaoMixes: make([][]byte, params.BeaconConfig().EpochsPerHistoricalVector),
	})
	require.NoError(t, err)

	tests := []struct {
		slot  types.Slot
		index types.ValidatorIndex
	}{
		{
			slot:  1,
			index: 2039,
		},
		{
			slot:  5,
			index: 1895,
		},
		{
			slot:  19,
			index: 1947,
		},
		{
			slot:  30,
			index: 369,
		},
		{
			slot:  43,
			index: 464,
		},
	}

	for _, tt := range tests {
		ClearCache()
		require.NoError(t, state.SetSlot(tt.slot))
		result, err := BeaconProposerIndex(state)
		require.NoError(t, err, "Failed to get shard and committees at slot")
		assert.Equal(t, tt.index, result, "Result index was an unexpected value")
	}
}

func TestBeaconProposerIndex_BadState(t *testing.T) {
	params.SetupTestConfigCleanup(t)
	ClearCache()
	c := params.BeaconConfig()
	c.MinGenesisActiveValidatorCount = 16384
	params.OverrideBeaconConfig(c)
	validators := make([]*ethpb.Validator, params.BeaconConfig().MinGenesisActiveValidatorCount/8)
	for i := 0; i < len(validators); i++ {
		validators[i] = &ethpb.Validator{
			ExitEpoch: params.BeaconConfig().FarFutureEpoch,
		}
	}
	roots := make([][]byte, params.BeaconConfig().SlotsPerHistoricalRoot)
	for i := uint64(0); i < uint64(params.BeaconConfig().SlotsPerHistoricalRoot); i++ {
		roots[i] = make([]byte, 32)
	}

	state, err := v1.InitializeFromProto(&pb.BeaconState{
		Validators:  validators,
		Slot:        0,
		RandaoMixes: make([][]byte, params.BeaconConfig().EpochsPerHistoricalVector),
		BlockRoots:  roots,
		StateRoots:  roots,
	})
	require.NoError(t, err)
	// Set a very high slot, so that retrieved block root will be
	// non existent for the proposer cache.
	require.NoError(t, state.SetSlot(100))
	_, err = BeaconProposerIndex(state)
	require.NoError(t, err)
	assert.Equal(t, 0, len(proposerIndicesCache.ProposerIndicesCache.ListKeys()))
}

func TestComputeProposerIndex_Compatibility(t *testing.T) {
	validators := make([]*ethpb.Validator, params.BeaconConfig().MinGenesisActiveValidatorCount)
	for i := 0; i < len(validators); i++ {
		validators[i] = &ethpb.Validator{
			ExitEpoch: params.BeaconConfig().FarFutureEpoch,
		}
	}

	state, err := v1.InitializeFromProto(&pb.BeaconState{
		Validators:  validators,
		RandaoMixes: make([][]byte, params.BeaconConfig().EpochsPerHistoricalVector),
	})
	require.NoError(t, err)

	indices, err := ActiveValidatorIndices(state, 0)
	require.NoError(t, err)

	var proposerIndices []types.ValidatorIndex
	seed, err := Seed(state, 0, params.BeaconConfig().DomainBeaconProposer)
	require.NoError(t, err)
	for i := uint64(0); i < uint64(params.BeaconConfig().SlotsPerEpoch); i++ {
		seedWithSlot := append(seed[:], bytesutil.Bytes8(i)...)
		seedWithSlotHash := hashutil.Hash(seedWithSlot)
		index, err := ComputeProposerIndex(state, indices, seedWithSlotHash)
		require.NoError(t, err)
		proposerIndices = append(proposerIndices, index)
	}

	var wantedProposerIndices []types.ValidatorIndex
	seed, err = Seed(state, 0, params.BeaconConfig().DomainBeaconProposer)
	require.NoError(t, err)
	for i := uint64(0); i < uint64(params.BeaconConfig().SlotsPerEpoch); i++ {
		seedWithSlot := append(seed[:], bytesutil.Bytes8(i)...)
		seedWithSlotHash := hashutil.Hash(seedWithSlot)
		index, err := computeProposerIndexWithValidators(state.Validators(), indices, seedWithSlotHash)
		require.NoError(t, err)
		wantedProposerIndices = append(wantedProposerIndices, index)
	}
	assert.DeepEqual(t, wantedProposerIndices, proposerIndices, "Wanted proposer indices from ComputeProposerIndexWithValidators does not match")
}

func TestDelayedActivationExitEpoch_OK(t *testing.T) {
	epoch := types.Epoch(9999)
	wanted := epoch + 1 + params.BeaconConfig().MaxSeedLookahead
	assert.Equal(t, wanted, ActivationExitEpoch(epoch))
}

func TestActiveValidatorCount_Genesis(t *testing.T) {
	c := 1000
	validators := make([]*ethpb.Validator, c)
	for i := 0; i < len(validators); i++ {
		validators[i] = &ethpb.Validator{
			ExitEpoch: params.BeaconConfig().FarFutureEpoch,
		}
	}
	beaconState, err := v1.InitializeFromProto(&pb.BeaconState{
		Slot:        0,
		Validators:  validators,
		RandaoMixes: make([][]byte, params.BeaconConfig().EpochsPerHistoricalVector),
	})
	require.NoError(t, err)

	// Preset cache to a bad count.
	seed, err := Seed(beaconState, 0, params.BeaconConfig().DomainBeaconAttester)
	require.NoError(t, err)
	require.NoError(t, committeeCache.AddCommitteeShuffledList(&cache.Committees{Seed: seed, ShuffledIndices: []types.ValidatorIndex{1, 2, 3}}))
	validatorCount, err := ActiveValidatorCount(beaconState, CurrentEpoch(beaconState))
	require.NoError(t, err)
	assert.Equal(t, uint64(c), validatorCount, "Did not get the correct validator count")
}

func TestChurnLimit_OK(t *testing.T) {
	tests := []struct {
		validatorCount int
		wantedChurn    uint64
	}{
		{validatorCount: 1000, wantedChurn: 4},
		{validatorCount: 100000, wantedChurn: 4},
		{validatorCount: 1000000, wantedChurn: 15 /* validatorCount/churnLimitQuotient */},
		{validatorCount: 2000000, wantedChurn: 30 /* validatorCount/churnLimitQuotient */},
	}
	for _, test := range tests {
		ClearCache()

		validators := make([]*ethpb.Validator, test.validatorCount)
		for i := 0; i < len(validators); i++ {
			validators[i] = &ethpb.Validator{
				ExitEpoch: params.BeaconConfig().FarFutureEpoch,
			}
		}

		beaconState, err := v1.InitializeFromProto(&pb.BeaconState{
			Slot:        1,
			Validators:  validators,
			RandaoMixes: make([][]byte, params.BeaconConfig().EpochsPerHistoricalVector),
		})
		require.NoError(t, err)
		validatorCount, err := ActiveValidatorCount(beaconState, CurrentEpoch(beaconState))
		require.NoError(t, err)
		resultChurn, err := ValidatorChurnLimit(validatorCount)
		require.NoError(t, err)
		assert.Equal(t, test.wantedChurn, resultChurn, "ValidatorChurnLimit(%d)", test.validatorCount)
	}
}

func TestDomain_OK(t *testing.T) {
	state := &pb.BeaconState{
		Fork: &pb.Fork{
			Epoch:           3,
			PreviousVersion: []byte{0, 0, 0, 2},
			CurrentVersion:  []byte{0, 0, 0, 3},
		},
	}
	tests := []struct {
		epoch      types.Epoch
		domainType [4]byte
		result     []byte
	}{
		{epoch: 1, domainType: bytesutil.ToBytes4(bytesutil.Bytes4(4)), result: bytesutil.ToBytes(947067381421703172, 32)},
		{epoch: 2, domainType: bytesutil.ToBytes4(bytesutil.Bytes4(4)), result: bytesutil.ToBytes(947067381421703172, 32)},
		{epoch: 2, domainType: bytesutil.ToBytes4(bytesutil.Bytes4(5)), result: bytesutil.ToBytes(947067381421703173, 32)},
		{epoch: 3, domainType: bytesutil.ToBytes4(bytesutil.Bytes4(4)), result: bytesutil.ToBytes(9369798235163459588, 32)},
		{epoch: 3, domainType: bytesutil.ToBytes4(bytesutil.Bytes4(5)), result: bytesutil.ToBytes(9369798235163459589, 32)},
	}
	for _, tt := range tests {
		domain, err := Domain(state.Fork, tt.epoch, tt.domainType, nil)
		require.NoError(t, err)
		assert.DeepEqual(t, types.Domain(tt.result[:8]), domain[:8], "Unexpected domain version")
	}
}

// Test basic functionality of ActiveValidatorIndices without caching. This test will need to be
// rewritten when releasing some cache flag.
func TestActiveValidatorIndices(t *testing.T) {
	farFutureEpoch := params.BeaconConfig().FarFutureEpoch
	type args struct {
		state *pb.BeaconState
		epoch types.Epoch
	}
	tests := []struct {
		name      string
		args      args
		want      []types.ValidatorIndex
		wantedErr string
	}{
		{
			name: "all_active_epoch_10",
			args: args{
				state: &pb.BeaconState{
					RandaoMixes: make([][]byte, params.BeaconConfig().EpochsPerHistoricalVector),
					Validators: []*ethpb.Validator{
						{
							ActivationEpoch: 0,
							ExitEpoch:       farFutureEpoch,
						},
						{
							ActivationEpoch: 0,
							ExitEpoch:       farFutureEpoch,
						},
						{
							ActivationEpoch: 0,
							ExitEpoch:       farFutureEpoch,
						},
					},
				},
				epoch: 10,
			},
			want: []types.ValidatorIndex{0, 1, 2},
		},
		{
			name: "some_active_epoch_10",
			args: args{
				state: &pb.BeaconState{
					RandaoMixes: make([][]byte, params.BeaconConfig().EpochsPerHistoricalVector),
					Validators: []*ethpb.Validator{
						{
							ActivationEpoch: 0,
							ExitEpoch:       farFutureEpoch,
						},
						{
							ActivationEpoch: 0,
							ExitEpoch:       farFutureEpoch,
						},
						{
							ActivationEpoch: 0,
							ExitEpoch:       1,
						},
					},
				},
				epoch: 10,
			},
			want: []types.ValidatorIndex{0, 1},
		},
		{
			name: "some_active_with_recent_new_epoch_10",
			args: args{
				state: &pb.BeaconState{
					RandaoMixes: make([][]byte, params.BeaconConfig().EpochsPerHistoricalVector),
					Validators: []*ethpb.Validator{
						{
							ActivationEpoch: 0,
							ExitEpoch:       farFutureEpoch,
						},
						{
							ActivationEpoch: 0,
							ExitEpoch:       farFutureEpoch,
						},
						{
							ActivationEpoch: 0,
							ExitEpoch:       1,
						},
						{
							ActivationEpoch: 0,
							ExitEpoch:       farFutureEpoch,
						},
					},
				},
				epoch: 10,
			},
			want: []types.ValidatorIndex{0, 1, 3},
		},
		{
			name: "some_active_with_recent_new_epoch_10",
			args: args{
				state: &pb.BeaconState{
					RandaoMixes: make([][]byte, params.BeaconConfig().EpochsPerHistoricalVector),
					Validators: []*ethpb.Validator{
						{
							ActivationEpoch: 0,
							ExitEpoch:       farFutureEpoch,
						},
						{
							ActivationEpoch: 0,
							ExitEpoch:       farFutureEpoch,
						},
						{
							ActivationEpoch: 0,
							ExitEpoch:       1,
						},
						{
							ActivationEpoch: 0,
							ExitEpoch:       farFutureEpoch,
						},
					},
				},
				epoch: 10,
			},
			want: []types.ValidatorIndex{0, 1, 3},
		},
		{
			name: "some_active_with_recent_new_epoch_10",
			args: args{
				state: &pb.BeaconState{
					RandaoMixes: make([][]byte, params.BeaconConfig().EpochsPerHistoricalVector),
					Validators: []*ethpb.Validator{
						{
							ActivationEpoch: 0,
							ExitEpoch:       farFutureEpoch,
						},
						{
							ActivationEpoch: 0,
							ExitEpoch:       1,
						},
						{
							ActivationEpoch: 0,
							ExitEpoch:       farFutureEpoch,
						},
						{
							ActivationEpoch: 0,
							ExitEpoch:       farFutureEpoch,
						},
					},
				},
				epoch: 10,
			},
			want: []types.ValidatorIndex{0, 2, 3},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := v1.InitializeFromProto(tt.args.state)
			require.NoError(t, err)
			got, err := ActiveValidatorIndices(s, tt.args.epoch)
			if tt.wantedErr != "" {
				assert.ErrorContains(t, tt.wantedErr, err)
				return
			}
			assert.DeepEqual(t, tt.want, got, "ActiveValidatorIndices()")
			ClearCache()
		})
	}
}

func TestComputeProposerIndex(t *testing.T) {
	seed := bytesutil.ToBytes32([]byte("seed"))
	type args struct {
		validators []*ethpb.Validator
		indices    []types.ValidatorIndex
		seed       [32]byte
	}
	tests := []struct {
		name      string
		args      args
		want      types.ValidatorIndex
		wantedErr string
	}{
		{
			name: "all_active_indices",
			args: args{
				validators: []*ethpb.Validator{
					{EffectiveBalance: params.BeaconConfig().MaxEffectiveBalance},
					{EffectiveBalance: params.BeaconConfig().MaxEffectiveBalance},
					{EffectiveBalance: params.BeaconConfig().MaxEffectiveBalance},
					{EffectiveBalance: params.BeaconConfig().MaxEffectiveBalance},
					{EffectiveBalance: params.BeaconConfig().MaxEffectiveBalance},
				},
				indices: []types.ValidatorIndex{0, 1, 2, 3, 4},
				seed:    seed,
			},
			want: 2,
		},
		{ // Regression test for https://github.com/prysmaticlabs/prysm/issues/4259.
			name: "1_active_index",
			args: args{
				validators: []*ethpb.Validator{
					{EffectiveBalance: params.BeaconConfig().MaxEffectiveBalance},
					{EffectiveBalance: params.BeaconConfig().MaxEffectiveBalance},
					{EffectiveBalance: params.BeaconConfig().MaxEffectiveBalance},
					{EffectiveBalance: params.BeaconConfig().MaxEffectiveBalance},
					{EffectiveBalance: params.BeaconConfig().MaxEffectiveBalance},
				},
				indices: []types.ValidatorIndex{3},
				seed:    seed,
			},
			want: 3,
		},
		{
			name: "empty_active_indices",
			args: args{
				validators: []*ethpb.Validator{
					{EffectiveBalance: params.BeaconConfig().MaxEffectiveBalance},
					{EffectiveBalance: params.BeaconConfig().MaxEffectiveBalance},
					{EffectiveBalance: params.BeaconConfig().MaxEffectiveBalance},
					{EffectiveBalance: params.BeaconConfig().MaxEffectiveBalance},
					{EffectiveBalance: params.BeaconConfig().MaxEffectiveBalance},
				},
				indices: []types.ValidatorIndex{},
				seed:    seed,
			},
			wantedErr: "empty active indices list",
		},
		{
			name: "active_indices_out_of_range",
			args: args{
				validators: []*ethpb.Validator{
					{EffectiveBalance: params.BeaconConfig().MaxEffectiveBalance},
					{EffectiveBalance: params.BeaconConfig().MaxEffectiveBalance},
					{EffectiveBalance: params.BeaconConfig().MaxEffectiveBalance},
					{EffectiveBalance: params.BeaconConfig().MaxEffectiveBalance},
					{EffectiveBalance: params.BeaconConfig().MaxEffectiveBalance},
				},
				indices: []types.ValidatorIndex{100},
				seed:    seed,
			},
			wantedErr: "active index out of range",
		},
		{
			name: "second_half_active",
			args: args{
				validators: []*ethpb.Validator{
					{EffectiveBalance: params.BeaconConfig().MaxEffectiveBalance},
					{EffectiveBalance: params.BeaconConfig().MaxEffectiveBalance},
					{EffectiveBalance: params.BeaconConfig().MaxEffectiveBalance},
					{EffectiveBalance: params.BeaconConfig().MaxEffectiveBalance},
					{EffectiveBalance: params.BeaconConfig().MaxEffectiveBalance},
					{EffectiveBalance: params.BeaconConfig().MaxEffectiveBalance},
					{EffectiveBalance: params.BeaconConfig().MaxEffectiveBalance},
					{EffectiveBalance: params.BeaconConfig().MaxEffectiveBalance},
					{EffectiveBalance: params.BeaconConfig().MaxEffectiveBalance},
					{EffectiveBalance: params.BeaconConfig().MaxEffectiveBalance},
				},
				indices: []types.ValidatorIndex{5, 6, 7, 8, 9},
				seed:    seed,
			},
			want: 7,
		},
		{
			name: "nil_validator",
			args: args{
				validators: []*ethpb.Validator{
					{EffectiveBalance: params.BeaconConfig().MaxEffectiveBalance},
					{EffectiveBalance: params.BeaconConfig().MaxEffectiveBalance},
					nil, // Should never happen, but would cause a panic when it does happen.
					{EffectiveBalance: params.BeaconConfig().MaxEffectiveBalance},
					{EffectiveBalance: params.BeaconConfig().MaxEffectiveBalance},
				},
				indices: []types.ValidatorIndex{0, 1, 2, 3, 4},
				seed:    seed,
			},
			want: 4,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bState := &pb.BeaconState{Validators: tt.args.validators}
			stTrie, err := v1.InitializeFromProtoUnsafe(bState)
			require.NoError(t, err)
			got, err := ComputeProposerIndex(stTrie, tt.args.indices, tt.args.seed)
			if tt.wantedErr != "" {
				assert.ErrorContains(t, tt.wantedErr, err)
				return
			}
			assert.Equal(t, tt.want, got, "ComputeProposerIndex()")
		})
	}
}

func TestIsEligibleForActivationQueue(t *testing.T) {
	tests := []struct {
		name      string
		validator *ethpb.Validator
		want      bool
	}{
		{"Eligible",
			&ethpb.Validator{ActivationEligibilityEpoch: params.BeaconConfig().FarFutureEpoch, EffectiveBalance: params.BeaconConfig().MaxEffectiveBalance},
			true},
		{"Incorrect activation eligibility epoch",
			&ethpb.Validator{ActivationEligibilityEpoch: 1, EffectiveBalance: params.BeaconConfig().MaxEffectiveBalance},
			false},
		{"Not enough balance",
			&ethpb.Validator{ActivationEligibilityEpoch: params.BeaconConfig().FarFutureEpoch, EffectiveBalance: 1},
			false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, IsEligibleForActivationQueue(tt.validator), "IsEligibleForActivationQueue()")
		})
	}
}

func TestIsIsEligibleForActivation(t *testing.T) {
	tests := []struct {
		name      string
		validator *ethpb.Validator
		state     *pb.BeaconState
		want      bool
	}{
		{"Eligible",
			&ethpb.Validator{ActivationEligibilityEpoch: 1, ActivationEpoch: params.BeaconConfig().FarFutureEpoch},
			&pb.BeaconState{FinalizedCheckpoint: &ethpb.Checkpoint{Epoch: 2}},
			true},
		{"Not yet finalized",
			&ethpb.Validator{ActivationEligibilityEpoch: 1, ActivationEpoch: params.BeaconConfig().FarFutureEpoch},
			&pb.BeaconState{FinalizedCheckpoint: &ethpb.Checkpoint{Root: make([]byte, 32)}},
			false},
		{"Incorrect activation epoch",
			&ethpb.Validator{ActivationEligibilityEpoch: 1},
			&pb.BeaconState{FinalizedCheckpoint: &ethpb.Checkpoint{Epoch: 2}},
			false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := v1.InitializeFromProto(tt.state)
			require.NoError(t, err)
			assert.Equal(t, tt.want, IsEligibleForActivation(s, tt.validator), "IsEligibleForActivation()")
		})
	}
}

func computeProposerIndexWithValidators(validators []*ethpb.Validator, activeIndices []types.ValidatorIndex, seed [32]byte) (types.ValidatorIndex, error) {
	length := uint64(len(activeIndices))
	if length == 0 {
		return 0, errors.New("empty active indices list")
	}
	maxRandomByte := uint64(1<<8 - 1)
	hashFunc := hashutil.CustomSHA256Hasher()

	for i := uint64(0); ; i++ {
		candidateIndex, err := ComputeShuffledIndex(types.ValidatorIndex(i%length), length, seed, true /* shuffle */)
		if err != nil {
			return 0, err
		}
		candidateIndex = activeIndices[candidateIndex]
		if uint64(candidateIndex) >= uint64(len(validators)) {
			return 0, errors.New("active index out of range")
		}
		b := append(seed[:], bytesutil.Bytes8(i/32)...)
		randomByte := hashFunc(b)[i%32]
		v := validators[candidateIndex]
		var effectiveBal uint64
		if v != nil {
			effectiveBal = v.EffectiveBalance
		}
		if effectiveBal*maxRandomByte >= params.BeaconConfig().MaxEffectiveBalance*uint64(randomByte) {
			return candidateIndex, nil
		}
	}
}
