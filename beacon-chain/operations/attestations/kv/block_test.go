package kv

import (
	"sort"
	"testing"

	ethpb "github.com/prysmaticlabs/ethereumapis/eth/v1alpha1"
	"github.com/prysmaticlabs/go-bitfield"
	"github.com/prysmaticlabs/prysm/shared/testutil"
	"github.com/prysmaticlabs/prysm/shared/testutil/assert"
	"github.com/prysmaticlabs/prysm/shared/testutil/require"
)

func TestKV_BlockAttestation_CanSaveRetrieve(t *testing.T) {
	cache := NewAttCaches()

	att1 := testutil.HydrateAttestation(&ethpb.Attestation{Data: &ethpb.AttestationData{Slot: 1}, AggregationBits: bitfield.Bitlist{0b1101}})
	att2 := testutil.HydrateAttestation(&ethpb.Attestation{Data: &ethpb.AttestationData{Slot: 2}, AggregationBits: bitfield.Bitlist{0b1101}})
	att3 := testutil.HydrateAttestation(&ethpb.Attestation{Data: &ethpb.AttestationData{Slot: 3}, AggregationBits: bitfield.Bitlist{0b1101}})
	att4 := testutil.HydrateAttestation(&ethpb.Attestation{Data: &ethpb.AttestationData{Slot: 3}, AggregationBits: bitfield.Bitlist{0b11011}}) // Diff bit length should not panic.
	atts := []*ethpb.Attestation{att1, att2, att3, att4}

	for _, att := range atts {
		require.NoError(t, cache.SaveBlockAttestation(att))
	}

	returned := cache.BlockAttestations()

	sort.Slice(returned, func(i, j int) bool {
		return returned[i].Data.Slot < returned[j].Data.Slot
	})

	assert.DeepEqual(t, atts, returned)
}

func TestKV_BlockAttestation_CanDelete(t *testing.T) {
	cache := NewAttCaches()

	att1 := testutil.HydrateAttestation(&ethpb.Attestation{Data: &ethpb.AttestationData{Slot: 1}, AggregationBits: bitfield.Bitlist{0b1101}})
	att2 := testutil.HydrateAttestation(&ethpb.Attestation{Data: &ethpb.AttestationData{Slot: 2}, AggregationBits: bitfield.Bitlist{0b1101}})
	att3 := testutil.HydrateAttestation(&ethpb.Attestation{Data: &ethpb.AttestationData{Slot: 3}, AggregationBits: bitfield.Bitlist{0b1101}})
	atts := []*ethpb.Attestation{att1, att2, att3}

	for _, att := range atts {
		require.NoError(t, cache.SaveBlockAttestation(att))
	}

	require.NoError(t, cache.DeleteBlockAttestation(att1))
	require.NoError(t, cache.DeleteBlockAttestation(att3))

	returned := cache.BlockAttestations()
	wanted := []*ethpb.Attestation{att2}
	assert.DeepEqual(t, wanted, returned)
}
