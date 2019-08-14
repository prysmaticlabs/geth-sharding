package kv

import (
	"context"
	"testing"

	"github.com/gogo/protobuf/proto"
	"github.com/prysmaticlabs/go-ssz"
	"github.com/prysmaticlabs/prysm/beacon-chain/db/filters"
	ethpb "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
)

func TestStore_AttestationCRUD(t *testing.T) {
	db := setupDB(t)
	defer teardownDB(t, db)
	att := &ethpb.Attestation{
		Data: &ethpb.AttestationData{
			Crosslink: &ethpb.Crosslink{
				Shard:      5,
				ParentRoot: []byte("parent"),
				StartEpoch: 1,
				EndEpoch:   2,
			},
		},
	}
	ctx := context.Background()
	if err := db.SaveAttestation(ctx, att); err != nil {
		t.Fatal(err)
	}
	attDataRoot, err := ssz.HashTreeRoot(att.Data)
	if err != nil {
		t.Fatal(err)
	}
	if !db.HasAttestation(ctx, attDataRoot) {
		t.Error("Expected attestation to exist in the db")
	}
	retrievedAtt, err := db.Attestation(ctx, attDataRoot)
	if err != nil {
		t.Fatal(err)
	}
	if !proto.Equal(att, retrievedAtt) {
		t.Errorf("Wanted %v, received %v", att, retrievedAtt)
	}
	if err := db.DeleteAttestation(ctx, attDataRoot); err != nil {
		t.Fatal(err)
	}
	if db.HasAttestation(ctx, attDataRoot) {
		t.Error("Expected attestation to have been deleted from the db")
	}
}

func TestStore_Attestations_FiltersCorrectly(t *testing.T) {
	db := setupDB(t)
	defer teardownDB(t, db)
	sameParentRoot := [32]byte{1, 2, 3}
	otherParentRoot := [32]byte{4, 5, 6}
	atts := []*ethpb.Attestation{
		{
			Data: &ethpb.AttestationData{
				Crosslink: &ethpb.Crosslink{
					Shard:      5,
					ParentRoot: sameParentRoot[:],
					StartEpoch: 1,
					EndEpoch:   2,
				},
			},
		},
		{
			Data: &ethpb.AttestationData{
				Crosslink: &ethpb.Crosslink{
					Shard:      5,
					ParentRoot: sameParentRoot[:],
					StartEpoch: 10,
					EndEpoch:   11,
				},
			},
		},
		{
			Data: &ethpb.AttestationData{
				Crosslink: &ethpb.Crosslink{
					Shard:      5,
					ParentRoot: otherParentRoot[:],
					StartEpoch: 1,
					EndEpoch:   20,
				},
			},
		},
	}
	ctx := context.Background()
	if err := db.SaveAttestations(ctx, atts); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		filter         *filters.QueryFilter
		expectedNumAtt int
	}{
		{
			filter:         filters.NewFilter().SetShard(5),
			expectedNumAtt: 3,
		},
		{
			filter:         filters.NewFilter().SetShard(5).SetParentRoot(otherParentRoot[:]),
			expectedNumAtt: 1,
		},
		{
			filter:         filters.NewFilter().SetShard(5).SetParentRoot(sameParentRoot[:]),
			expectedNumAtt: 2,
		},
		//{
		//	filter:         filters.NewFilter().SetStartEpoch(1),
		//	expectedNumAtt: 2,
		//},
		//{
		//	filter:         filters.NewFilter().SetParentRoot([]byte("parent3")),
		//	expectedNumAtt: 1,
		//},
		//{
		//	// Only a single attestation in the list meets the composite filter criteria above.
		//	filter:         filters.NewFilter().SetShard(5).SetStartEpoch(1),
		//	expectedNumAtt: 1,
		//},
		//{
		//	// No specified filter should return all attestations.
		//	filter:         nil,
		//	expectedNumAtt: 3,
		//},
		//{
		//	// No attestation meets the criteria below.
		//	filter:         filters.NewFilter().SetShard(1000),
		//	expectedNumAtt: 0,
		//},
	}
	for _, tt := range tests {
		retrievedAtts, err := db.Attestations(ctx, tt.filter)
		if err != nil {
			t.Fatal(err)
		}
		if len(retrievedAtts) != tt.expectedNumAtt {
			t.Errorf("Expected %d attestations, received %d", tt.expectedNumAtt, len(retrievedAtts))
		}
	}
}
