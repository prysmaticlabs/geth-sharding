package helpers_test

import (
	"fmt"
	"testing"

	types "github.com/prysmaticlabs/eth2-types"
	ethpb "github.com/prysmaticlabs/ethereumapis/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	iface "github.com/prysmaticlabs/prysm/beacon-chain/state/interface"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/testutil"
	"github.com/prysmaticlabs/prysm/shared/testutil/assert"
	"github.com/prysmaticlabs/prysm/shared/testutil/require"
)

func TestWeakSubjectivity_ComputeWeakSubjectivityPeriod(t *testing.T) {
	tests := []struct {
		valCount   uint64
		avgBalance uint64
		want       types.Epoch
	}{
		// Asserting that we get the same numbers as defined in the reference table:
		// https://github.com/ethereum/eth2.0-specs/blob/master/specs/phase0/weak-subjectivity.md#calculating-the-weak-subjectivity-period
		{valCount: 32768, avgBalance: 28, want: 504},
		{valCount: 65536, avgBalance: 28, want: 752},
		{valCount: 131072, avgBalance: 28, want: 1248},
		{valCount: 262144, avgBalance: 28, want: 2241},
		{valCount: 524288, avgBalance: 28, want: 2241},
		{valCount: 1048576, avgBalance: 28, want: 2241},
		{valCount: 32768, avgBalance: 32, want: 665},
		{valCount: 65536, avgBalance: 32, want: 1075},
		{valCount: 131072, avgBalance: 32, want: 1894},
		{valCount: 262144, avgBalance: 32, want: 3532},
		{valCount: 524288, avgBalance: 32, want: 3532},
		{valCount: 1048576, avgBalance: 32, want: 3532},
		// Additional test vectors, to check case when T*(200+3*D) >= t*(200+12*D)
		{valCount: 32768, avgBalance: 22, want: 277},
		{valCount: 65536, avgBalance: 22, want: 298},
		{valCount: 131072, avgBalance: 22, want: 340},
		{valCount: 262144, avgBalance: 22, want: 424},
		{valCount: 524288, avgBalance: 22, want: 593},
		{valCount: 1048576, avgBalance: 22, want: 931},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("valCount: %d, avgBalance: %d", tt.valCount, tt.avgBalance), func(t *testing.T) {
			// Reset committee cache - as we need to recalculate active validator set for each test.
			helpers.ResetCommitteeCache()
			got, err := helpers.ComputeWeakSubjectivityPeriod(genState(t, tt.valCount, tt.avgBalance))
			require.NoError(t, err)
			assert.Equal(t, tt.want, got, "valCount: %v, avgBalance: %v", tt.valCount, tt.avgBalance)
		})
	}
}
func TestWeakSubjectivity_IsWithinWeakSubjectivityPeriod(t *testing.T) {
	tests := []struct {
		name            string
		epoch           types.Epoch
		genWsState      func() iface.ReadOnlyBeaconState
		genWsCheckpoint func() *ethpb.WeakSubjectivityCheckpoint
		want            bool
		wantedErr       string
	}{
		{
			name: "nil weak subjectivity state",
			genWsState: func() iface.ReadOnlyBeaconState {
				return nil
			},
			genWsCheckpoint: func() *ethpb.WeakSubjectivityCheckpoint {
				return &ethpb.WeakSubjectivityCheckpoint{
					BlockRoot: make([]byte, 32),
					StateRoot: make([]byte, 32),
					Epoch:     42,
				}
			},
			wantedErr: "invalid weak subjectivity state or checkpoint",
		},
		{
			name: "nil weak subjectivity checkpoint",
			genWsState: func() iface.ReadOnlyBeaconState {
				return genState(t, 128, 32)
			},
			genWsCheckpoint: func() *ethpb.WeakSubjectivityCheckpoint {
				return nil
			},
			wantedErr: "invalid weak subjectivity state or checkpoint",
		},
		{
			name: "state and checkpoint roots do not match",
			genWsState: func() iface.ReadOnlyBeaconState {
				beaconState := genState(t, 128, 32)
				require.NoError(t, beaconState.SetSlot(42*params.BeaconConfig().SlotsPerEpoch))
				err := beaconState.SetLatestBlockHeader(&ethpb.BeaconBlockHeader{
					Slot:      42 * params.BeaconConfig().SlotsPerEpoch,
					StateRoot: bytesutil.PadTo([]byte("stateroot1"), 32),
				})
				require.NoError(t, err)
				return beaconState
			},
			genWsCheckpoint: func() *ethpb.WeakSubjectivityCheckpoint {
				return &ethpb.WeakSubjectivityCheckpoint{
					StateRoot: bytesutil.PadTo([]byte("stateroot2"), 32),
					Epoch:     42,
				}
			},
			wantedErr: fmt.Sprintf("state (%#x) and checkpoint (%#x) roots do not match",
				bytesutil.PadTo([]byte("stateroot1"), 32), bytesutil.PadTo([]byte("stateroot2"), 32)),
		},
		{
			name: "state and checkpoint epochs do not match",
			genWsState: func() iface.ReadOnlyBeaconState {
				beaconState := genState(t, 128, 32)
				require.NoError(t, beaconState.SetSlot(42*params.BeaconConfig().SlotsPerEpoch))
				err := beaconState.SetLatestBlockHeader(&ethpb.BeaconBlockHeader{
					Slot:      42 * params.BeaconConfig().SlotsPerEpoch,
					StateRoot: bytesutil.PadTo([]byte("stateroot"), 32),
				})
				require.NoError(t, err)
				return beaconState
			},
			genWsCheckpoint: func() *ethpb.WeakSubjectivityCheckpoint {
				return &ethpb.WeakSubjectivityCheckpoint{
					StateRoot: bytesutil.PadTo([]byte("stateroot"), 32),
					Epoch:     43,
				}
			},
			wantedErr: "state (42) and checkpoint (43) epochs do not match",
		},
		{
			name: "no active validators",
			genWsState: func() iface.ReadOnlyBeaconState {
				beaconState := genState(t, 0, 32)
				require.NoError(t, beaconState.SetSlot(42*params.BeaconConfig().SlotsPerEpoch))
				err := beaconState.SetLatestBlockHeader(&ethpb.BeaconBlockHeader{
					Slot:      42 * params.BeaconConfig().SlotsPerEpoch,
					StateRoot: bytesutil.PadTo([]byte("stateroot"), 32),
				})
				require.NoError(t, err)
				return beaconState
			},
			genWsCheckpoint: func() *ethpb.WeakSubjectivityCheckpoint {
				return &ethpb.WeakSubjectivityCheckpoint{
					StateRoot: bytesutil.PadTo([]byte("stateroot"), 32),
					Epoch:     42,
				}
			},
			wantedErr: "cannot compute weak subjectivity period: no active validators found",
		},
		{
			name:  "outside weak subjectivity period",
			epoch: 300,
			genWsState: func() iface.ReadOnlyBeaconState {
				beaconState := genState(t, 128, 32)
				require.NoError(t, beaconState.SetSlot(42*params.BeaconConfig().SlotsPerEpoch))
				err := beaconState.SetLatestBlockHeader(&ethpb.BeaconBlockHeader{
					Slot:      42 * params.BeaconConfig().SlotsPerEpoch,
					StateRoot: bytesutil.PadTo([]byte("stateroot"), 32),
				})
				require.NoError(t, err)
				return beaconState
			},
			genWsCheckpoint: func() *ethpb.WeakSubjectivityCheckpoint {
				return &ethpb.WeakSubjectivityCheckpoint{
					StateRoot: bytesutil.PadTo([]byte("stateroot"), 32),
					Epoch:     42,
				}
			},
			want: false,
		},
		{
			name:  "within weak subjectivity period",
			epoch: 299,
			genWsState: func() iface.ReadOnlyBeaconState {
				beaconState := genState(t, 128, 32)
				require.NoError(t, beaconState.SetSlot(42*params.BeaconConfig().SlotsPerEpoch))
				err := beaconState.SetLatestBlockHeader(&ethpb.BeaconBlockHeader{
					Slot:      42 * params.BeaconConfig().SlotsPerEpoch,
					StateRoot: bytesutil.PadTo([]byte("stateroot"), 32),
				})
				require.NoError(t, err)
				return beaconState
			},
			genWsCheckpoint: func() *ethpb.WeakSubjectivityCheckpoint {
				return &ethpb.WeakSubjectivityCheckpoint{
					StateRoot: bytesutil.PadTo([]byte("stateroot"), 32),
					Epoch:     42,
				}
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := helpers.IsWithinWeakSubjectivityPeriod(tt.epoch, tt.genWsState(), tt.genWsCheckpoint())
			if tt.wantedErr != "" {
				assert.Equal(t, false, got)
				assert.ErrorContains(t, tt.wantedErr, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func genState(t *testing.T, valCount uint64, avgBalance uint64) iface.BeaconState {
	beaconState, err := testutil.NewBeaconState()
	require.NoError(t, err)

	validators := make([]*ethpb.Validator, valCount)
	balances := make([]uint64, len(validators))
	for i := uint64(0); i < valCount; i++ {
		validators[i] = &ethpb.Validator{
			PublicKey:             make([]byte, params.BeaconConfig().BLSPubkeyLength),
			WithdrawalCredentials: make([]byte, 32),
			EffectiveBalance:      avgBalance * 1e9,
			ExitEpoch:             params.BeaconConfig().FarFutureEpoch,
		}
		balances[i] = validators[i].EffectiveBalance
	}

	require.NoError(t, beaconState.SetValidators(validators))
	require.NoError(t, beaconState.SetBalances(balances))

	return beaconState
}
