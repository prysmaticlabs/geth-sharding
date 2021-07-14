package helpers

import (
	"math"
	"testing"
	"time"

	types "github.com/prysmaticlabs/eth2-types"
	v1 "github.com/prysmaticlabs/prysm/beacon-chain/state/v1"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/testutil/assert"
	"github.com/prysmaticlabs/prysm/shared/testutil/require"
	"github.com/prysmaticlabs/prysm/shared/timeutils"
)

func TestSlotToEpoch_OK(t *testing.T) {
	tests := []struct {
		slot  types.Slot
		epoch types.Epoch
	}{
		{slot: 0, epoch: 0},
		{slot: 50, epoch: 1},
		{slot: 64, epoch: 2},
		{slot: 128, epoch: 4},
		{slot: 200, epoch: 6},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.epoch, SlotToEpoch(tt.slot), "SlotToEpoch(%d)", tt.slot)
	}
}

func TestCurrentEpoch_OK(t *testing.T) {
	tests := []struct {
		slot  types.Slot
		epoch types.Epoch
	}{
		{slot: 0, epoch: 0},
		{slot: 50, epoch: 1},
		{slot: 64, epoch: 2},
		{slot: 128, epoch: 4},
		{slot: 200, epoch: 6},
	}
	for _, tt := range tests {
		state, err := v1.InitializeFromProto(&pb.BeaconState{Slot: tt.slot})
		require.NoError(t, err)
		assert.Equal(t, tt.epoch, CurrentEpoch(state), "ActiveCurrentEpoch(%d)", state.Slot())
	}
}

func TestPrevEpoch_OK(t *testing.T) {
	tests := []struct {
		slot  types.Slot
		epoch types.Epoch
	}{
		{slot: 0, epoch: 0},
		{slot: 0 + params.BeaconConfig().SlotsPerEpoch + 1, epoch: 0},
		{slot: 2 * params.BeaconConfig().SlotsPerEpoch, epoch: 1},
	}
	for _, tt := range tests {
		state, err := v1.InitializeFromProto(&pb.BeaconState{Slot: tt.slot})
		require.NoError(t, err)
		assert.Equal(t, tt.epoch, PrevEpoch(state), "ActivePrevEpoch(%d)", state.Slot())
	}
}

func TestNextEpoch_OK(t *testing.T) {
	tests := []struct {
		slot  types.Slot
		epoch types.Epoch
	}{
		{slot: 0, epoch: types.Epoch(0/params.BeaconConfig().SlotsPerEpoch + 1)},
		{slot: 50, epoch: types.Epoch(0/params.BeaconConfig().SlotsPerEpoch + 2)},
		{slot: 64, epoch: types.Epoch(64/params.BeaconConfig().SlotsPerEpoch + 1)},
		{slot: 128, epoch: types.Epoch(128/params.BeaconConfig().SlotsPerEpoch + 1)},
		{slot: 200, epoch: types.Epoch(200/params.BeaconConfig().SlotsPerEpoch + 1)},
	}
	for _, tt := range tests {
		state, err := v1.InitializeFromProto(&pb.BeaconState{Slot: tt.slot})
		require.NoError(t, err)
		assert.Equal(t, tt.epoch, NextEpoch(state), "NextEpoch(%d)", state.Slot())
	}
}

func TestEpochStartSlot_OK(t *testing.T) {
	tests := []struct {
		epoch     types.Epoch
		startSlot types.Slot
		error     bool
	}{
		{epoch: 0, startSlot: 0 * params.BeaconConfig().SlotsPerEpoch, error: false},
		{epoch: 1, startSlot: 1 * params.BeaconConfig().SlotsPerEpoch, error: false},
		{epoch: 10, startSlot: 10 * params.BeaconConfig().SlotsPerEpoch, error: false},
		{epoch: 1 << 58, startSlot: 1 << 63, error: false},
		{epoch: 1 << 59, startSlot: 1 << 63, error: true},
		{epoch: 1 << 60, startSlot: 1 << 63, error: true},
	}
	for _, tt := range tests {
		ss, err := StartSlot(tt.epoch)
		if !tt.error {
			require.NoError(t, err)
			assert.Equal(t, tt.startSlot, ss, "StartSlot(%d)", tt.epoch)
		} else {
			require.ErrorContains(t, "start slot calculation overflow", err)
		}
	}
}

func TestEpochEndSlot_OK(t *testing.T) {
	tests := []struct {
		epoch     types.Epoch
		startSlot types.Slot
		error     bool
	}{
		{epoch: 0, startSlot: 1*params.BeaconConfig().SlotsPerEpoch - 1, error: false},
		{epoch: 1, startSlot: 2*params.BeaconConfig().SlotsPerEpoch - 1, error: false},
		{epoch: 10, startSlot: 11*params.BeaconConfig().SlotsPerEpoch - 1, error: false},
		{epoch: 1 << 59, startSlot: 1 << 63, error: true},
		{epoch: 1 << 60, startSlot: 1 << 63, error: true},
		{epoch: math.MaxUint64, startSlot: 0, error: true},
	}
	for _, tt := range tests {
		ss, err := EndSlot(tt.epoch)
		if !tt.error {
			require.NoError(t, err)
			assert.Equal(t, tt.startSlot, ss, "StartSlot(%d)", tt.epoch)
		} else {
			require.ErrorContains(t, "start slot calculation overflow", err)
		}
	}
}

func TestIsEpochStart(t *testing.T) {
	epochLength := params.BeaconConfig().SlotsPerEpoch

	tests := []struct {
		slot   types.Slot
		result bool
	}{
		{
			slot:   epochLength + 1,
			result: false,
		},
		{
			slot:   epochLength - 1,
			result: false,
		},
		{
			slot:   epochLength,
			result: true,
		},
		{
			slot:   epochLength * 2,
			result: true,
		},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.result, IsEpochStart(tt.slot), "IsEpochStart(%d)", tt.slot)
	}
}

func TestIsEpochEnd(t *testing.T) {
	epochLength := params.BeaconConfig().SlotsPerEpoch

	tests := []struct {
		slot   types.Slot
		result bool
	}{
		{
			slot:   epochLength + 1,
			result: false,
		},
		{
			slot:   epochLength,
			result: false,
		},
		{
			slot:   epochLength - 1,
			result: true,
		},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.result, IsEpochEnd(tt.slot), "IsEpochEnd(%d)", tt.slot)
	}
}

func TestSlotsSinceEpochStarts(t *testing.T) {
	tests := []struct {
		slots       types.Slot
		wantedSlots types.Slot
	}{
		{slots: 0, wantedSlots: 0},
		{slots: 1, wantedSlots: 1},
		{slots: params.BeaconConfig().SlotsPerEpoch - 1, wantedSlots: params.BeaconConfig().SlotsPerEpoch - 1},
		{slots: params.BeaconConfig().SlotsPerEpoch + 1, wantedSlots: 1},
		{slots: 10*params.BeaconConfig().SlotsPerEpoch + 2, wantedSlots: 2},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.wantedSlots, SlotsSinceEpochStarts(tt.slots))
	}
}

func TestRoundUpToNearestEpoch_OK(t *testing.T) {
	tests := []struct {
		startSlot     types.Slot
		roundedUpSlot types.Slot
	}{
		{startSlot: 0 * params.BeaconConfig().SlotsPerEpoch, roundedUpSlot: 0},
		{startSlot: 1*params.BeaconConfig().SlotsPerEpoch - 10, roundedUpSlot: 1 * params.BeaconConfig().SlotsPerEpoch},
		{startSlot: 10*params.BeaconConfig().SlotsPerEpoch - (params.BeaconConfig().SlotsPerEpoch - 1), roundedUpSlot: 10 * params.BeaconConfig().SlotsPerEpoch},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.roundedUpSlot, RoundUpToNearestEpoch(tt.startSlot), "RoundUpToNearestEpoch(%d)", tt.startSlot)
	}
}

func TestSlotToTime(t *testing.T) {
	type args struct {
		genesisTimeSec uint64
		slot           types.Slot
	}
	tests := []struct {
		name      string
		args      args
		want      time.Time
		wantedErr string
	}{
		{
			name: "slot_0",
			args: args{
				genesisTimeSec: 0,
				slot:           0,
			},
			want: time.Unix(0, 0),
		},
		{
			name: "slot_1",
			args: args{
				genesisTimeSec: 0,
				slot:           1,
			},
			want: time.Unix(int64(1*params.BeaconConfig().SecondsPerSlot), 0),
		},
		{
			name: "slot_12",
			args: args{
				genesisTimeSec: 500,
				slot:           12,
			},
			want: time.Unix(500+int64(12*params.BeaconConfig().SecondsPerSlot), 0),
		},
		{
			name: "overflow",
			args: args{
				genesisTimeSec: 500,
				slot:           math.MaxUint64,
			},
			wantedErr: "is in the far distant future",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SlotToTime(tt.args.genesisTimeSec, tt.args.slot)
			if tt.wantedErr != "" {
				assert.ErrorContains(t, tt.wantedErr, err)
			} else {
				assert.NoError(t, err)
				assert.DeepEqual(t, tt.want, got)
			}
		})
	}
}

func TestVerifySlotTime(t *testing.T) {
	type args struct {
		genesisTime   int64
		slot          types.Slot
		timeTolerance time.Duration
	}
	tests := []struct {
		name      string
		args      args
		wantedErr string
	}{
		{
			name: "Past slot",
			args: args{
				genesisTime: timeutils.Now().Add(-1 * 5 * time.Duration(params.BeaconConfig().SecondsPerSlot) * time.Second).Unix(),
				slot:        3,
			},
		},
		{
			name: "within tolerance",
			args: args{
				genesisTime: timeutils.Now().Add(-1 * 5 * time.Duration(params.BeaconConfig().SecondsPerSlot) * time.Second).Add(20 * time.Millisecond).Unix(),
				slot:        5,
			},
		},
		{
			name: "future slot",
			args: args{
				genesisTime: timeutils.Now().Add(-1 * 5 * time.Duration(params.BeaconConfig().SecondsPerSlot) * time.Second).Unix(),
				slot:        6,
			},
			wantedErr: "could not process slot from the future",
		},
		{
			name: "max future slot",
			args: args{
				genesisTime: timeutils.Now().Add(-1 * 5 * time.Duration(params.BeaconConfig().SecondsPerSlot) * time.Second).Unix(),
				slot:        types.Slot(MaxSlotBuffer + 6),
			},
			wantedErr: "exceeds max allowed value relative to the local clock",
		},
		{
			name: "evil future slot",
			args: args{
				genesisTime: timeutils.Now().Add(-1 * 24 * time.Duration(params.BeaconConfig().SecondsPerSlot) * time.Second).Unix(), // 24 slots in the past
				// Gets multiplied with slot duration, and results in an overflow. Wraps around to a valid time.
				// Lower than max signed int. And chosen specifically to wrap to a valid slot 24
				slot: types.Slot((^uint64(0))/params.BeaconConfig().SecondsPerSlot) + 24,
			},
			wantedErr: "is in the far distant future",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := VerifySlotTime(uint64(tt.args.genesisTime), tt.args.slot, tt.args.timeTolerance)
			if tt.wantedErr != "" {
				assert.ErrorContains(t, tt.wantedErr, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateSlotClock_HandlesBadSlot(t *testing.T) {
	genTime := timeutils.Now().Add(-1 * time.Duration(MaxSlotBuffer) * time.Duration(params.BeaconConfig().SecondsPerSlot) * time.Second).Unix()

	assert.NoError(t, ValidateSlotClock(types.Slot(MaxSlotBuffer), uint64(genTime)), "unexpected error validating slot")
	assert.NoError(t, ValidateSlotClock(types.Slot(2*MaxSlotBuffer), uint64(genTime)), "unexpected error validating slot")
	assert.ErrorContains(t, "which exceeds max allowed value relative to the local clock", ValidateSlotClock(types.Slot(2*MaxSlotBuffer+1), uint64(genTime)), "no error from bad slot")
	assert.ErrorContains(t, "which exceeds max allowed value relative to the local clock", ValidateSlotClock(1<<63, uint64(genTime)), "no error from bad slot")
}

func TestPrevSlot(t *testing.T) {
	tests := []struct {
		name string
		slot types.Slot
		want types.Slot
	}{
		{
			name: "no underflow",
			slot: 0,
			want: 0,
		},
		{
			name: "slot 1",
			slot: 1,
			want: 0,
		},
		{
			name: "slot 2",
			slot: 2,
			want: 1,
		},
		{
			name: "max",
			slot: 1<<64 - 1,
			want: 1<<64 - 1 - 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := PrevSlot(tt.slot); got != tt.want {
				t.Errorf("PrevSlot() = %v, want %v", got, tt.want)
			}
		})
	}
}
