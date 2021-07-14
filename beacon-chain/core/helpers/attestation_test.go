package helpers_test

import (
	"strconv"
	"testing"
	"time"

	types "github.com/prysmaticlabs/eth2-types"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	v1 "github.com/prysmaticlabs/prysm/beacon-chain/state/v1"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	ethpb "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/shared/bls"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/testutil"
	"github.com/prysmaticlabs/prysm/shared/testutil/assert"
	"github.com/prysmaticlabs/prysm/shared/testutil/require"
	"github.com/prysmaticlabs/prysm/shared/timeutils"
)

func TestAttestation_IsAggregator(t *testing.T) {
	t.Run("aggregator", func(t *testing.T) {
		beaconState, privKeys := testutil.DeterministicGenesisState(t, 100)
		committee, err := helpers.BeaconCommitteeFromState(beaconState, 0, 0)
		require.NoError(t, err)
		sig := privKeys[0].Sign([]byte{'A'})
		agg, err := helpers.IsAggregator(uint64(len(committee)), sig.Marshal())
		require.NoError(t, err)
		assert.Equal(t, true, agg, "Wanted aggregator true")
	})

	t.Run("not aggregator", func(t *testing.T) {
		params.UseMinimalConfig()
		defer params.UseMainnetConfig()
		beaconState, privKeys := testutil.DeterministicGenesisState(t, 2048)

		committee, err := helpers.BeaconCommitteeFromState(beaconState, 0, 0)
		require.NoError(t, err)
		sig := privKeys[0].Sign([]byte{'A'})
		agg, err := helpers.IsAggregator(uint64(len(committee)), sig.Marshal())
		require.NoError(t, err)
		assert.Equal(t, false, agg, "Wanted aggregator false")
	})
}

func TestAttestation_AggregateSignature(t *testing.T) {
	t.Run("verified", func(t *testing.T) {
		pubkeys := make([]bls.PublicKey, 0, 100)
		atts := make([]*ethpb.Attestation, 0, 100)
		msg := bytesutil.ToBytes32([]byte("hello"))
		for i := 0; i < 100; i++ {
			priv, err := bls.RandKey()
			require.NoError(t, err)
			pub := priv.PublicKey()
			sig := priv.Sign(msg[:])
			pubkeys = append(pubkeys, pub)
			att := &ethpb.Attestation{Signature: sig.Marshal()}
			atts = append(atts, att)
		}
		aggSig, err := helpers.AggregateSignature(atts)
		require.NoError(t, err)
		assert.Equal(t, true, aggSig.FastAggregateVerify(pubkeys, msg), "Signature did not verify")
	})

	t.Run("not verified", func(t *testing.T) {
		pubkeys := make([]bls.PublicKey, 0, 100)
		atts := make([]*ethpb.Attestation, 0, 100)
		msg := []byte("hello")
		for i := 0; i < 100; i++ {
			priv, err := bls.RandKey()
			require.NoError(t, err)
			pub := priv.PublicKey()
			sig := priv.Sign(msg)
			pubkeys = append(pubkeys, pub)
			att := &ethpb.Attestation{Signature: sig.Marshal()}
			atts = append(atts, att)
		}
		aggSig, err := helpers.AggregateSignature(atts[0 : len(atts)-2])
		require.NoError(t, err)
		assert.Equal(t, false, aggSig.FastAggregateVerify(pubkeys, bytesutil.ToBytes32(msg)), "Signature not suppose to verify")
	})
}

func TestAttestation_ComputeSubnetForAttestation(t *testing.T) {
	// Create 10 committees
	committeeCount := uint64(10)
	validatorCount := committeeCount * params.BeaconConfig().TargetCommitteeSize
	validators := make([]*ethpb.Validator, validatorCount)

	for i := 0; i < len(validators); i++ {
		k := make([]byte, 48)
		copy(k, strconv.Itoa(i))
		validators[i] = &ethpb.Validator{
			PublicKey:             k,
			WithdrawalCredentials: make([]byte, 32),
			ExitEpoch:             params.BeaconConfig().FarFutureEpoch,
		}
	}

	state, err := v1.InitializeFromProto(&pb.BeaconState{
		Validators:  validators,
		Slot:        200,
		BlockRoots:  make([][]byte, params.BeaconConfig().SlotsPerHistoricalRoot),
		StateRoots:  make([][]byte, params.BeaconConfig().SlotsPerHistoricalRoot),
		RandaoMixes: make([][]byte, params.BeaconConfig().EpochsPerHistoricalVector),
	})
	require.NoError(t, err)
	att := &ethpb.Attestation{
		AggregationBits: []byte{'A'},
		Data: &ethpb.AttestationData{
			Slot:            34,
			CommitteeIndex:  4,
			BeaconBlockRoot: []byte{'C'},
			Source:          nil,
			Target:          nil,
		},
		Signature: []byte{'B'},
	}
	valCount, err := helpers.ActiveValidatorCount(state, helpers.SlotToEpoch(att.Data.Slot))
	require.NoError(t, err)
	sub := helpers.ComputeSubnetForAttestation(valCount, att)
	assert.Equal(t, uint64(6), sub, "Did not get correct subnet for attestation")
}

func Test_ValidateAttestationTime(t *testing.T) {
	if params.BeaconNetworkConfig().MaximumGossipClockDisparity < 200*time.Millisecond {
		t.Fatal("This test expects the maximum clock disparity to be at least 200ms")
	}

	type args struct {
		attSlot     types.Slot
		genesisTime time.Time
	}
	tests := []struct {
		name      string
		args      args
		wantedErr string
	}{
		{
			name: "attestation.slot == current_slot",
			args: args{
				attSlot:     15,
				genesisTime: timeutils.Now().Add(-15 * time.Duration(params.BeaconConfig().SecondsPerSlot) * time.Second),
			},
		},
		{
			name: "attestation.slot == current_slot, received in middle of slot",
			args: args{
				attSlot: 15,
				genesisTime: timeutils.Now().Add(
					-15 * time.Duration(params.BeaconConfig().SecondsPerSlot) * time.Second,
				).Add(-(time.Duration(params.BeaconConfig().SecondsPerSlot/2) * time.Second)),
			},
		},
		{
			name: "attestation.slot == current_slot, received 200ms early",
			args: args{
				attSlot: 16,
				genesisTime: timeutils.Now().Add(
					-16 * time.Duration(params.BeaconConfig().SecondsPerSlot) * time.Second,
				).Add(-200 * time.Millisecond),
			},
		},
		{
			name: "attestation.slot > current_slot",
			args: args{
				attSlot:     16,
				genesisTime: timeutils.Now().Add(-15 * time.Duration(params.BeaconConfig().SecondsPerSlot) * time.Second),
			},
			wantedErr: "not within attestation propagation range",
		},
		{
			name: "attestation.slot < current_slot-ATTESTATION_PROPAGATION_SLOT_RANGE",
			args: args{
				attSlot:     100 - params.BeaconNetworkConfig().AttestationPropagationSlotRange - 1,
				genesisTime: timeutils.Now().Add(-100 * time.Duration(params.BeaconConfig().SecondsPerSlot) * time.Second),
			},
			wantedErr: "not within attestation propagation range",
		},
		{
			name: "attestation.slot = current_slot-ATTESTATION_PROPAGATION_SLOT_RANGE",
			args: args{
				attSlot:     100 - params.BeaconNetworkConfig().AttestationPropagationSlotRange,
				genesisTime: timeutils.Now().Add(-100 * time.Duration(params.BeaconConfig().SecondsPerSlot) * time.Second),
			},
		},
		{
			name: "attestation.slot = current_slot-ATTESTATION_PROPAGATION_SLOT_RANGE, received 200ms late",
			args: args{
				attSlot: 100 - params.BeaconNetworkConfig().AttestationPropagationSlotRange,
				genesisTime: timeutils.Now().Add(
					-100 * time.Duration(params.BeaconConfig().SecondsPerSlot) * time.Second,
				).Add(200 * time.Millisecond),
			},
		},
		{
			name: "attestation.slot is well beyond current slot",
			args: args{
				attSlot:     1 << 32,
				genesisTime: timeutils.Now().Add(-15 * time.Duration(params.BeaconConfig().SecondsPerSlot) * time.Second),
			},
			wantedErr: "which exceeds max allowed value relative to the local clock",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := helpers.ValidateAttestationTime(tt.args.attSlot, tt.args.genesisTime,
				params.BeaconNetworkConfig().MaximumGossipClockDisparity)
			if tt.wantedErr != "" {
				assert.ErrorContains(t, tt.wantedErr, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestVerifyCheckpointEpoch_Ok(t *testing.T) {
	// Genesis was 6 epochs ago exactly.
	offset := params.BeaconConfig().SlotsPerEpoch.Mul(params.BeaconConfig().SecondsPerSlot * 6)
	genesis := time.Now().Add(-1 * time.Second * time.Duration(offset))
	assert.Equal(t, true, helpers.VerifyCheckpointEpoch(&ethpb.Checkpoint{Epoch: 6}, genesis))
	assert.Equal(t, true, helpers.VerifyCheckpointEpoch(&ethpb.Checkpoint{Epoch: 5}, genesis))
	assert.Equal(t, false, helpers.VerifyCheckpointEpoch(&ethpb.Checkpoint{Epoch: 4}, genesis))
	assert.Equal(t, false, helpers.VerifyCheckpointEpoch(&ethpb.Checkpoint{Epoch: 2}, genesis))
}

func TestValidateNilAttestation(t *testing.T) {
	tests := []struct {
		name        string
		attestation *ethpb.Attestation
		errString   string
	}{
		{
			name:        "nil attestation",
			attestation: nil,
			errString:   "attestation can't be nil",
		},
		{
			name:        "nil attestation data",
			attestation: &ethpb.Attestation{},
			errString:   "attestation's data can't be nil",
		},
		{
			name: "nil attestation source",
			attestation: &ethpb.Attestation{
				Data: &ethpb.AttestationData{
					Source: nil,
					Target: &ethpb.Checkpoint{},
				},
			},
			errString: "attestation's source can't be nil",
		},
		{
			name: "nil attestation target",
			attestation: &ethpb.Attestation{
				Data: &ethpb.AttestationData{
					Target: nil,
					Source: &ethpb.Checkpoint{},
				},
			},
			errString: "attestation's target can't be nil",
		},
		{
			name: "nil attestation bitfield",
			attestation: &ethpb.Attestation{
				Data: &ethpb.AttestationData{
					Target: &ethpb.Checkpoint{},
					Source: &ethpb.Checkpoint{},
				},
			},
			errString: "attestation's bitfield can't be nil",
		},
		{
			name: "good attestation",
			attestation: &ethpb.Attestation{
				Data: &ethpb.AttestationData{
					Target: &ethpb.Checkpoint{},
					Source: &ethpb.Checkpoint{},
				},
				AggregationBits: []byte{},
			},
			errString: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.errString != "" {
				require.ErrorContains(t, tt.errString, helpers.ValidateNilAttestation(tt.attestation))
			} else {
				require.NoError(t, helpers.ValidateNilAttestation(tt.attestation))
			}
		})
	}
}

func TestValidateSlotTargetEpoch(t *testing.T) {
	tests := []struct {
		name        string
		attestation *ethpb.Attestation
		errString   string
	}{
		{
			name: "incorrect slot",
			attestation: &ethpb.Attestation{
				Data: &ethpb.AttestationData{
					Target: &ethpb.Checkpoint{Epoch: 1},
					Source: &ethpb.Checkpoint{},
				},
				AggregationBits: []byte{},
			},
			errString: "slot 0 does not match target epoch 1",
		},
		{
			name: "good attestation",
			attestation: &ethpb.Attestation{
				Data: &ethpb.AttestationData{
					Slot:   2 * params.BeaconConfig().SlotsPerEpoch,
					Target: &ethpb.Checkpoint{Epoch: 2},
					Source: &ethpb.Checkpoint{},
				},
				AggregationBits: []byte{},
			},
			errString: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.errString != "" {
				require.ErrorContains(t, tt.errString, helpers.ValidateSlotTargetEpoch(tt.attestation.Data))
			} else {
				require.NoError(t, helpers.ValidateSlotTargetEpoch(tt.attestation.Data))
			}
		})
	}
}
