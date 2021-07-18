package altair_test

import (
	"bytes"
	"testing"

	types "github.com/prysmaticlabs/eth2-types"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/altair"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	stateAltair "github.com/prysmaticlabs/prysm/beacon-chain/state/v2"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	ethpb "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	prysmv2 "github.com/prysmaticlabs/prysm/proto/prysm/v2"
	"github.com/prysmaticlabs/prysm/shared/bls"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/testutil"
	"github.com/prysmaticlabs/prysm/shared/testutil/require"
)

func TestSyncCommitteeIndices_CanGet(t *testing.T) {
	getState := func(t *testing.T, count uint64) *stateAltair.BeaconState {
		validators := make([]*ethpb.Validator, count)
		for i := 0; i < len(validators); i++ {
			validators[i] = &ethpb.Validator{
				ExitEpoch:        params.BeaconConfig().FarFutureEpoch,
				EffectiveBalance: params.BeaconConfig().MinDepositAmount,
			}
		}
		state, err := stateAltair.InitializeFromProto(&pb.BeaconStateAltair{
			Validators:  validators,
			RandaoMixes: make([][]byte, params.BeaconConfig().EpochsPerHistoricalVector),
		})
		require.NoError(t, err)
		return state
	}

	type args struct {
		state *stateAltair.BeaconState
		epoch types.Epoch
	}
	tests := []struct {
		name      string
		args      args
		wantErr   bool
		errString string
	}{
		{
			name: "nil state",
			args: args{
				state: nil,
			},
			wantErr:   true,
			errString: "nil inner state",
		},
		{
			name: "genesis validator count, epoch 0",
			args: args{
				state: getState(t, params.BeaconConfig().MinGenesisActiveValidatorCount),
				epoch: 0,
			},
			wantErr: false,
		},
		{
			name: "genesis validator count, epoch 100",
			args: args{
				state: getState(t, params.BeaconConfig().MinGenesisActiveValidatorCount),
				epoch: 100,
			},
			wantErr: false,
		},
		{
			name: "less than optimal validator count, epoch 100",
			args: args{
				state: getState(t, params.BeaconConfig().MaxValidatorsPerCommittee),
				epoch: 100,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			helpers.ClearCache()
			got, err := altair.NextSyncCommitteeIndices(tt.args.state)
			if tt.wantErr {
				require.ErrorContains(t, tt.errString, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, int(params.BeaconConfig().SyncCommitteeSize), len(got))
			}
		})
	}
}

func TestSyncCommitteeIndices_DifferentPeriods(t *testing.T) {
	helpers.ClearCache()
	getState := func(t *testing.T, count uint64) *stateAltair.BeaconState {
		validators := make([]*ethpb.Validator, count)
		for i := 0; i < len(validators); i++ {
			validators[i] = &ethpb.Validator{
				ExitEpoch:        params.BeaconConfig().FarFutureEpoch,
				EffectiveBalance: params.BeaconConfig().MinDepositAmount,
			}
		}
		state, err := stateAltair.InitializeFromProto(&pb.BeaconStateAltair{
			Validators:  validators,
			RandaoMixes: make([][]byte, params.BeaconConfig().EpochsPerHistoricalVector),
		})
		require.NoError(t, err)
		return state
	}

	state := getState(t, params.BeaconConfig().MaxValidatorsPerCommittee)
	got1, err := altair.NextSyncCommitteeIndices(state)
	require.NoError(t, err)
	require.NoError(t, state.SetSlot(params.BeaconConfig().SlotsPerEpoch))
	got2, err := altair.NextSyncCommitteeIndices(state)
	require.NoError(t, err)
	require.DeepNotEqual(t, got1, got2)
	require.NoError(t, state.SetSlot(params.BeaconConfig().SlotsPerEpoch*types.Slot(params.BeaconConfig().EpochsPerSyncCommitteePeriod)))
	got2, err = altair.NextSyncCommitteeIndices(state)
	require.NoError(t, err)
	require.DeepNotEqual(t, got1, got2)
	require.NoError(t, state.SetSlot(params.BeaconConfig().SlotsPerEpoch*types.Slot(2*params.BeaconConfig().EpochsPerSyncCommitteePeriod)))
	got2, err = altair.NextSyncCommitteeIndices(state)
	require.NoError(t, err)
	require.DeepNotEqual(t, got1, got2)
}

func TestSyncCommittee_CanGet(t *testing.T) {
	getState := func(t *testing.T, count uint64) *stateAltair.BeaconState {
		validators := make([]*ethpb.Validator, count)
		for i := 0; i < len(validators); i++ {
			blsKey, err := bls.RandKey()
			require.NoError(t, err)
			validators[i] = &ethpb.Validator{
				ExitEpoch:        params.BeaconConfig().FarFutureEpoch,
				EffectiveBalance: params.BeaconConfig().MinDepositAmount,
				PublicKey:        blsKey.PublicKey().Marshal(),
			}
		}
		state, err := stateAltair.InitializeFromProto(&pb.BeaconStateAltair{
			Validators:  validators,
			RandaoMixes: make([][]byte, params.BeaconConfig().EpochsPerHistoricalVector),
		})
		require.NoError(t, err)
		return state
	}

	type args struct {
		state *stateAltair.BeaconState
		epoch types.Epoch
	}
	tests := []struct {
		name      string
		args      args
		wantErr   bool
		errString string
	}{
		{
			name: "nil state",
			args: args{
				state: nil,
			},
			wantErr:   true,
			errString: "nil inner state",
		},
		{
			name: "genesis validator count, epoch 0",
			args: args{
				state: getState(t, params.BeaconConfig().MinGenesisActiveValidatorCount),
				epoch: 0,
			},
			wantErr: false,
		},
		{
			name: "genesis validator count, epoch 100",
			args: args{
				state: getState(t, params.BeaconConfig().MinGenesisActiveValidatorCount),
				epoch: 100,
			},
			wantErr: false,
		},
		{
			name: "less than optimal validator count, epoch 100",
			args: args{
				state: getState(t, params.BeaconConfig().MaxValidatorsPerCommittee),
				epoch: 100,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			helpers.ClearCache()
			if !tt.wantErr {
				require.NoError(t, tt.args.state.SetSlot(types.Slot(tt.args.epoch)*params.BeaconConfig().SlotsPerEpoch))
			}
			got, err := altair.NextSyncCommittee(tt.args.state)
			if tt.wantErr {
				require.ErrorContains(t, tt.errString, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, int(params.BeaconConfig().SyncCommitteeSize), len(got.Pubkeys))
				require.Equal(t, params.BeaconConfig().BLSPubkeyLength, len(got.AggregatePubkey))
			}
		})
	}
}

func TestAssignedToSyncCommittee(t *testing.T) {
	s, _ := testutil.DeterministicGenesisStateAltair(t, 5*params.BeaconConfig().SyncCommitteeSize)
	syncCommittee, err := altair.NextSyncCommittee(s)
	require.NoError(t, err)
	require.NoError(t, s.SetCurrentSyncCommittee(syncCommittee))
	slot := helpers.CurrentEpoch(s) + 2*params.BeaconConfig().EpochsPerSyncCommitteePeriod
	require.NoError(t, s.SetSlot(types.Slot(slot)))
	syncCommittee, err = altair.NextSyncCommittee(s)
	require.NoError(t, err)
	require.NoError(t, s.SetNextSyncCommittee(syncCommittee))
	require.NoError(t, s.SetSlot(0))

	csc, err := s.CurrentSyncCommittee()
	require.NoError(t, err)
	nsc, err := s.NextSyncCommittee()
	require.NoError(t, err)

	currentSyncCommitteeIndex := 0
	vals := s.Validators()
	for i, val := range vals {
		if bytes.Equal(val.PublicKey, csc.Pubkeys[0]) {
			currentSyncCommitteeIndex = i
		}
	}
	nextSyncCommitteeIndex := 0
	for i, val := range vals {
		if bytes.Equal(val.PublicKey, nsc.Pubkeys[0]) {
			nextSyncCommitteeIndex = i
		}
	}

	tests := []struct {
		name   string
		epoch  types.Epoch
		check  types.ValidatorIndex
		exists bool
	}{
		{
			name:   "does not exist while asking current sync committee",
			epoch:  0,
			check:  0,
			exists: false,
		},
		{
			name:   "exists in current sync committee",
			epoch:  0,
			check:  types.ValidatorIndex(currentSyncCommitteeIndex),
			exists: true,
		},
		{
			name:   "exists in next sync committee",
			epoch:  256,
			check:  types.ValidatorIndex(nextSyncCommitteeIndex),
			exists: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exists, err := altair.AssignedToSyncCommittee(s, tt.epoch, tt.check)
			require.NoError(t, err)
			require.Equal(t, tt.exists, exists)
		})
	}
}

func TestAssignedToSyncCommittee_IncorrectEpoch(t *testing.T) {
	s, _ := testutil.DeterministicGenesisStateAltair(t, 64)
	_, err := altair.AssignedToSyncCommittee(s, params.BeaconConfig().EpochsPerSyncCommitteePeriod*2, 0)
	require.ErrorContains(t, "epoch period 2 is not current period 0 or next period 1 in state", err)
}

func TestSubnetsForSyncCommittee(t *testing.T) {
	s, _ := testutil.DeterministicGenesisStateAltair(t, params.BeaconConfig().SyncCommitteeSize)
	syncCommittee, err := altair.NextSyncCommittee(s)
	require.NoError(t, err)
	require.NoError(t, s.SetCurrentSyncCommittee(syncCommittee))

	positions, err := altair.SubnetsForSyncCommittee(s, 0)
	require.NoError(t, err)
	require.DeepEqual(t, []uint64{3}, positions)
	positions, err = altair.SubnetsForSyncCommittee(s, 1)
	require.NoError(t, err)
	require.DeepEqual(t, []uint64{1}, positions)
	positions, err = altair.SubnetsForSyncCommittee(s, 2)
	require.NoError(t, err)
	require.DeepEqual(t, []uint64{2}, positions)
}

func TestSyncCommitteePeriod(t *testing.T) {
	tests := []struct {
		epoch  types.Epoch
		wanted uint64
	}{
		{epoch: 0, wanted: 0},
		{epoch: 0, wanted: 0 / uint64(params.BeaconConfig().EpochsPerSyncCommitteePeriod)},
		{epoch: 1, wanted: 1 / uint64(params.BeaconConfig().EpochsPerSyncCommitteePeriod)},
		{epoch: 1000, wanted: 1000 / uint64(params.BeaconConfig().EpochsPerSyncCommitteePeriod)},
	}
	for _, test := range tests {
		require.Equal(t, test.wanted, helpers.SyncCommitteePeriod(test.epoch))
	}
}

func TestValidateNilSyncContribution(t *testing.T) {
	tests := []struct {
		name    string
		s       *prysmv2.SignedContributionAndProof
		wantErr bool
	}{
		{
			name:    "nil object",
			s:       nil,
			wantErr: true,
		},
		{
			name:    "nil message",
			s:       &prysmv2.SignedContributionAndProof{},
			wantErr: true,
		},
		{
			name:    "nil contribution",
			s:       &prysmv2.SignedContributionAndProof{Message: &prysmv2.ContributionAndProof{}},
			wantErr: true,
		},
		{
			name: "nil bitfield",
			s: &prysmv2.SignedContributionAndProof{
				Message: &prysmv2.ContributionAndProof{
					Contribution: &prysmv2.SyncCommitteeContribution{},
				}},
			wantErr: true,
		},
		{
			name: "non nil sync contribution",
			s: &prysmv2.SignedContributionAndProof{
				Message: &prysmv2.ContributionAndProof{
					Contribution: &prysmv2.SyncCommitteeContribution{
						AggregationBits: []byte{},
					},
				}},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := altair.ValidateNilSyncContribution(tt.s); (err != nil) != tt.wantErr {
				t.Errorf("ValidateNilSyncContribution() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
