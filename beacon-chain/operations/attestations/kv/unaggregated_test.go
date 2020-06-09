package kv

import (
	"reflect"
	"testing"

	ethpb "github.com/prysmaticlabs/ethereumapis/eth/v1alpha1"
	"github.com/prysmaticlabs/go-bitfield"
)

func TestKV_SaveUnaggregatedAttestation(t *testing.T) {
	tests := []struct {
		name          string
		att           *ethpb.Attestation
		count         int
		wantErrString string
	}{
		{
			name: "nil attestation",
			att:  nil,
		},
		{
			name:          "already aggregated",
			att:           &ethpb.Attestation{AggregationBits: bitfield.Bitlist{0b10101}},
			wantErrString: "attestation is aggregated",
		},
		{
			name: "invalid hash",
			att: &ethpb.Attestation{
				Data: &ethpb.AttestationData{
					BeaconBlockRoot: []byte{0b0},
				},
			},
			wantErrString: "could not tree hash attestation: incorrect fixed bytes marshalling",
		},
		{
			name:  "normal save",
			att:   &ethpb.Attestation{AggregationBits: bitfield.Bitlist{0b0001}},
			count: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache := NewAttCaches()
			if len(cache.unAggregatedAtt) != 0 {
				t.Errorf("Invalid start pool, atts: %d", len(cache.unAggregatedAtt))
			}

			err := cache.SaveUnaggregatedAttestation(tt.att)
			if tt.wantErrString != "" && (err == nil || err.Error() != tt.wantErrString) {
				t.Errorf("Did not receive wanted error, want: %q, got: %v", tt.wantErrString, err)
				return
			}
			if tt.wantErrString == "" && err != nil {
				t.Error(err)
				return
			}
			if len(cache.unAggregatedAtt) != tt.count {
				t.Errorf("Incorrect attestation count, want: %d, got: %d", tt.count, len(cache.unAggregatedAtt))
			}
		})
	}
}

func TestKV_SaveUnaggregatedAttestations(t *testing.T) {
	tests := []struct {
		name          string
		atts          []*ethpb.Attestation
		count         int
		wantErrString string
	}{
		{
			name: "unaggregated only",
			atts: []*ethpb.Attestation{
				{Data: &ethpb.AttestationData{Slot: 1}},
				{Data: &ethpb.AttestationData{Slot: 2}},
				{Data: &ethpb.AttestationData{Slot: 3}},
			},
			count: 3,
		},
		{
			name: "has aggregated",
			atts: []*ethpb.Attestation{
				{Data: &ethpb.AttestationData{Slot: 1}},
				{AggregationBits: bitfield.Bitlist{0b1111}, Data: &ethpb.AttestationData{Slot: 2}},
				{Data: &ethpb.AttestationData{Slot: 3}},
			},
			wantErrString: "attestation is aggregated",
			count:         1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache := NewAttCaches()
			if len(cache.unAggregatedAtt) != 0 {
				t.Errorf("Invalid start pool, atts: %d", len(cache.unAggregatedAtt))
			}

			err := cache.SaveUnaggregatedAttestations(tt.atts)
			if tt.wantErrString != "" && (err == nil || err.Error() != tt.wantErrString) {
				t.Errorf("Did not receive wanted error, want: %q, got: %v", tt.wantErrString, err)
			}
			if tt.wantErrString == "" && err != nil {
				t.Error(err)
			}
			if len(cache.unAggregatedAtt) != tt.count {
				t.Errorf("Incorrect attestation count, want: %d, got: %d", tt.count, len(cache.unAggregatedAtt))
			}
		})
	}
}

func TestKV_Unaggregated_CanDelete(t *testing.T) {
	cache := NewAttCaches()

	att1 := &ethpb.Attestation{Data: &ethpb.AttestationData{Slot: 1}, AggregationBits: bitfield.Bitlist{0b101}}
	att2 := &ethpb.Attestation{Data: &ethpb.AttestationData{Slot: 2}, AggregationBits: bitfield.Bitlist{0b110}}
	att3 := &ethpb.Attestation{Data: &ethpb.AttestationData{Slot: 3}, AggregationBits: bitfield.Bitlist{0b110}}
	atts := []*ethpb.Attestation{att1, att2, att3}

	for _, att := range atts {
		if err := cache.SaveUnaggregatedAttestation(att); err != nil {
			t.Fatal(err)
		}
	}

	if err := cache.DeleteUnaggregatedAttestation(att1); err != nil {
		t.Fatal(err)
	}
	if err := cache.DeleteUnaggregatedAttestation(att2); err != nil {
		t.Fatal(err)
	}

	if err := cache.DeleteUnaggregatedAttestation(att3); err != nil {
		t.Fatal(err)
	}

	returned := cache.UnaggregatedAttestations()

	if !reflect.DeepEqual([]*ethpb.Attestation{}, returned) {
		t.Error("Did not receive correct aggregated atts")
	}
}

func TestKV_Unaggregated_CanGetByCommitteeAndSlot(t *testing.T) {
	cache := NewAttCaches()

	att1 := &ethpb.Attestation{Data: &ethpb.AttestationData{Slot: 1, CommitteeIndex: 1}, AggregationBits: bitfield.Bitlist{0b101}}
	att2 := &ethpb.Attestation{Data: &ethpb.AttestationData{Slot: 1, CommitteeIndex: 2}, AggregationBits: bitfield.Bitlist{0b110}}
	att3 := &ethpb.Attestation{Data: &ethpb.AttestationData{Slot: 2, CommitteeIndex: 1}, AggregationBits: bitfield.Bitlist{0b110}}
	atts := []*ethpb.Attestation{att1, att2, att3}

	for _, att := range atts {
		if err := cache.SaveUnaggregatedAttestation(att); err != nil {
			t.Fatal(err)
		}
	}

	returned := cache.UnaggregatedAttestationsBySlotIndex(1, 1)
	if !reflect.DeepEqual([]*ethpb.Attestation{att1}, returned) {
		t.Error("Did not receive correct aggregated atts")
	}
	returned = cache.UnaggregatedAttestationsBySlotIndex(1, 2)
	if !reflect.DeepEqual([]*ethpb.Attestation{att2}, returned) {
		t.Error("Did not receive correct aggregated atts")
	}
	returned = cache.UnaggregatedAttestationsBySlotIndex(2, 1)
	if !reflect.DeepEqual([]*ethpb.Attestation{att3}, returned) {
		t.Error("Did not receive correct aggregated atts")
	}
}
