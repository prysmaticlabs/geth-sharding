package kv

import (
	"context"
	"reflect"
	"testing"

	"github.com/gogo/protobuf/proto"
	ethpb "github.com/prysmaticlabs/ethereumapis/eth/v1alpha1"
	pbp2p "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
)

func TestStore_ArchivedActiveValidatorChanges(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()
	activated := []uint64{3, 4, 5}
	exited := []uint64{6, 7, 8}
	slashed := []uint64{1212}
	someRoot := [32]byte{1, 2, 3}
	changes := &pbp2p.ArchivedActiveSetChanges{
		Activated: activated,
		Exited:    exited,
		Slashed:   slashed,
		VoluntaryExits: []*ethpb.VoluntaryExit{
			{
				Epoch:          5,
				ValidatorIndex: 6,
			},
			{
				Epoch:          5,
				ValidatorIndex: 7,
			},
			{
				Epoch:          5,
				ValidatorIndex: 8,
			},
		},
		ProposerSlashings: []*ethpb.ProposerSlashing{
			{
				Header_1: &ethpb.SignedBeaconBlockHeader{
					Header: &ethpb.BeaconBlockHeader{
						ProposerIndex: 1212,
						Slot:          10,
						ParentRoot:    someRoot[:],
						StateRoot:     someRoot[:],
						BodyRoot:      someRoot[:],
					},
					Signature: make([]byte, 96),
				},
				Header_2: &ethpb.SignedBeaconBlockHeader{
					Header: &ethpb.BeaconBlockHeader{
						ProposerIndex: 1212,
						Slot:          10,
						ParentRoot:    someRoot[:],
						StateRoot:     someRoot[:],
						BodyRoot:      someRoot[:],
					},
					Signature: make([]byte, 96),
				},
			},
		},
		AttesterSlashings: []*ethpb.AttesterSlashing{
			{
				Attestation_1: &ethpb.IndexedAttestation{
					Data: &ethpb.AttestationData{
						BeaconBlockRoot: someRoot[:],
						Source: &ethpb.Checkpoint{
							Epoch: 5,
							Root:  someRoot[:],
						},
						Target: &ethpb.Checkpoint{
							Epoch: 5,
							Root:  someRoot[:],
						},
					},
				},
				Attestation_2: &ethpb.IndexedAttestation{
					Data: &ethpb.AttestationData{
						BeaconBlockRoot: someRoot[:],
						Source: &ethpb.Checkpoint{
							Epoch: 5,
							Root:  someRoot[:],
						},
						Target: &ethpb.Checkpoint{
							Epoch: 5,
							Root:  someRoot[:],
						},
					},
				},
			},
		},
	}
	epoch := uint64(10)
	if err := db.SaveArchivedActiveValidatorChanges(ctx, epoch, changes); err != nil {
		t.Fatal(err)
	}
	retrieved, err := db.ArchivedActiveValidatorChanges(ctx, epoch)
	if err != nil {
		t.Fatal(err)
	}
	if !proto.Equal(changes, retrieved) {
		t.Errorf("Wanted %v, received %v", changes, retrieved)
	}
}

func TestStore_ArchivedCommitteeInfo(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()
	someSeed := [32]byte{1, 2, 3}
	info := &pbp2p.ArchivedCommitteeInfo{
		ProposerSeed: someSeed[:],
		AttesterSeed: someSeed[:],
	}
	epoch := uint64(10)
	if err := db.SaveArchivedCommitteeInfo(ctx, epoch, info); err != nil {
		t.Fatal(err)
	}
	retrieved, err := db.ArchivedCommitteeInfo(ctx, epoch)
	if err != nil {
		t.Fatal(err)
	}
	if !proto.Equal(info, retrieved) {
		t.Errorf("Wanted %v, received %v", info, retrieved)
	}
}

func TestStore_ArchivedBalances(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()
	balances := []uint64{2, 3, 4, 5, 6, 7}
	epoch := uint64(10)
	if err := db.SaveArchivedBalances(ctx, epoch, balances); err != nil {
		t.Fatal(err)
	}
	retrieved, err := db.ArchivedBalances(ctx, epoch)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(balances, retrieved) {
		t.Errorf("Wanted %v, received %v", balances, retrieved)
	}
}

func TestStore_ArchivedValidatorParticipation(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()
	epoch := uint64(10)
	part := &ethpb.ValidatorParticipation{
		GlobalParticipationRate: 0.99,
		EligibleEther:           12202000,
		VotedEther:              12079998,
	}
	if err := db.SaveArchivedValidatorParticipation(ctx, epoch, part); err != nil {
		t.Fatal(err)
	}
	retrieved, err := db.ArchivedValidatorParticipation(ctx, epoch)
	if err != nil {
		t.Fatal(err)
	}
	if !proto.Equal(part, retrieved) {
		t.Errorf("Wanted %v, received %v", part, retrieved)
	}
}
