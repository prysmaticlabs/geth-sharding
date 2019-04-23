package attestation

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/prysmaticlabs/prysm/beacon-chain/cache"
	"github.com/prysmaticlabs/prysm/beacon-chain/internal"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/prysmaticlabs/prysm/shared/hashutil"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/sirupsen/logrus"
)

func init() {
	logrus.SetLevel(logrus.DebugLevel)
}

func TestUpdateLatestAttestation_UpdatesLatest(t *testing.T) {
	beaconDB := internal.SetupDB(t)
	defer internal.TeardownDB(t, beaconDB)
	ctx := context.Background()

	var validators []*pb.Validator
	for i := 0; i < 64; i++ {
		validators = append(validators, &pb.Validator{
			Pubkey:          []byte{byte(i)},
			ActivationEpoch: params.BeaconConfig().GenesisEpoch,
			ExitEpoch:       params.BeaconConfig().GenesisEpoch + 10,
		})
	}

	beaconState := &pb.BeaconState{
		Slot:              params.BeaconConfig().GenesisSlot + 1,
		ValidatorRegistry: validators,
	}
	block := &pb.BeaconBlock{
		Slot: params.BeaconConfig().GenesisSlot + 1,
	}
	if err := beaconDB.SaveBlock(block); err != nil {
		t.Fatal(err)
	}
	if err := beaconDB.UpdateChainHead(ctx, block, beaconState); err != nil {
		t.Fatal(err)
	}
	service := NewAttestationService(context.Background(), &Config{BeaconDB: beaconDB})

	attestation := &pb.Attestation{
		AggregationBitfield: []byte{0x80},
		Data: &pb.AttestationData{
			Slot:  params.BeaconConfig().GenesisSlot + 1,
			Shard: 1,
		},
	}

	if err := service.UpdateLatestAttestation(ctx, attestation); err != nil {
		t.Fatalf("could not update latest attestation: %v", err)
	}
	pubkey := bytesutil.ToBytes48([]byte{byte(3)})
	if service.store.m[pubkey].Data.Slot !=
		attestation.Data.Slot {
		t.Errorf("Incorrect slot stored, wanted: %d, got: %d",
			attestation.Data.Slot, service.store.m[pubkey].Data.Slot)
	}

	beaconState = &pb.BeaconState{
		Slot:              params.BeaconConfig().GenesisSlot + 36,
		ValidatorRegistry: validators,
	}
	if err := beaconDB.UpdateChainHead(ctx, block, beaconState); err != nil {
		t.Fatalf("could not save state: %v", err)
	}

	attestation.Data.Slot = params.BeaconConfig().GenesisSlot + 36
	attestation.Data.Shard = 36
	if err := service.UpdateLatestAttestation(ctx, attestation); err != nil {
		t.Fatalf("could not update latest attestation: %v", err)
	}
	if service.store.m[pubkey].Data.Slot !=
		attestation.Data.Slot {
		t.Errorf("Incorrect slot stored, wanted: %d, got: %d",
			attestation.Data.Slot, service.store.m[pubkey].Data.Slot)
	}
}

func TestAttestationPool_UpdatesAttestationPool(t *testing.T) {
	beaconDB := internal.SetupDB(t)
	defer internal.TeardownDB(t, beaconDB)
	ctx := context.Background()

	var validators []*pb.Validator
	for i := 0; i < 64; i++ {
		validators = append(validators, &pb.Validator{
			Pubkey:          []byte{byte(i)},
			ActivationEpoch: params.BeaconConfig().GenesisEpoch,
			ExitEpoch:       params.BeaconConfig().GenesisEpoch + 10,
		})
	}
	beaconState := &pb.BeaconState{
		Slot:              params.BeaconConfig().GenesisSlot + 1,
		ValidatorRegistry: validators,
	}
	block := &pb.BeaconBlock{
		Slot: params.BeaconConfig().GenesisSlot + 1,
	}
	if err := beaconDB.SaveBlock(block); err != nil {
		t.Fatal(err)
	}
	if err := beaconDB.UpdateChainHead(ctx, block, beaconState); err != nil {
		t.Fatal(err)
	}

	service := NewAttestationService(context.Background(), &Config{BeaconDB: beaconDB})
	attestation := &pb.Attestation{
		AggregationBitfield: []byte{0x80},
		Data: &pb.AttestationData{
			Slot:  params.BeaconConfig().GenesisSlot + 1,
			Shard: 1,
		},
	}

	if err := service.handleAttestation(context.Background(), attestation); err != nil {
		t.Error(err)
	}
}

func TestLatestAttestation_ReturnsLatestAttestation(t *testing.T) {
	beaconDB := internal.SetupDB(t)
	defer internal.TeardownDB(t, beaconDB)
	ctx := context.Background()

	pubKey := []byte{'A'}
	if err := beaconDB.SaveState(ctx, &pb.BeaconState{
		ValidatorRegistry: []*pb.Validator{{Pubkey: pubKey}},
	}); err != nil {
		t.Fatalf("could not save state: %v", err)
	}

	service := NewAttestationService(context.Background(), &Config{BeaconDB: beaconDB})
	pubKey48 := bytesutil.ToBytes48(pubKey)
	attestation := &pb.Attestation{AggregationBitfield: []byte{'B'}}
	service.store.m[pubKey48] = attestation

	latestAttestation, err := service.LatestAttestation(ctx, 0)
	if err != nil {
		t.Fatalf("Could not get latest attestation: %v", err)
	}
	if !reflect.DeepEqual(attestation, latestAttestation) {
		t.Errorf("Wanted: %v, got: %v", attestation, latestAttestation)
	}
}

func TestLatestAttestation_InvalidIndex(t *testing.T) {
	beaconDB := internal.SetupDB(t)
	defer internal.TeardownDB(t, beaconDB)
	ctx := context.Background()

	if err := beaconDB.SaveState(ctx, &pb.BeaconState{
		ValidatorRegistry: []*pb.Validator{},
	}); err != nil {
		t.Fatalf("could not save state: %v", err)
	}
	service := NewAttestationService(context.Background(), &Config{BeaconDB: beaconDB})

	index := uint64(0)
	want := fmt.Sprintf("invalid validator index %d", index)
	if _, err := service.LatestAttestation(ctx, index); !strings.Contains(err.Error(), want) {
		t.Errorf("Wanted error to contain %s, received %v", want, err)
	}
}

func TestLatestAttestation_NoAttestation(t *testing.T) {
	beaconDB := internal.SetupDB(t)
	defer internal.TeardownDB(t, beaconDB)
	ctx := context.Background()

	if err := beaconDB.SaveState(ctx, &pb.BeaconState{
		ValidatorRegistry: []*pb.Validator{{}},
	}); err != nil {
		t.Fatalf("could not save state: %v", err)
	}
	service := NewAttestationService(context.Background(), &Config{BeaconDB: beaconDB})

	index := 0
	a, err := service.LatestAttestation(ctx, uint64(index))
	if err != nil {
		t.Fatalf("could not run latest attestation: %v", err)
	}
	if a != nil {
		t.Errorf("Wanted attesstation %v, received %v", nil, a)
	}
}

func TestLatestAttestationTarget_CantGetAttestation(t *testing.T) {
	beaconDB := internal.SetupDB(t)
	defer internal.TeardownDB(t, beaconDB)
	ctx := context.Background()

	if err := beaconDB.SaveState(ctx, &pb.BeaconState{
		ValidatorRegistry: []*pb.Validator{{}},
	}); err != nil {
		t.Fatalf("could not save state: %v", err)
	}
	service := NewAttestationService(context.Background(), &Config{BeaconDB: beaconDB})

	index := uint64(100)
	want := fmt.Sprintf("could not get attestation: invalid validator index %d", index)
	if _, err := service.LatestAttestationTarget(ctx, index); !strings.Contains(err.Error(), want) {
		t.Errorf("Wanted error to contain %s, received %v", want, err)
	}
}

func TestLatestAttestationTarget_ReturnsLatestAttestedBlock(t *testing.T) {
	beaconDB := internal.SetupDB(t)
	defer internal.TeardownDB(t, beaconDB)
	ctx := context.Background()

	pubKey := []byte{'A'}
	if err := beaconDB.SaveState(ctx, &pb.BeaconState{
		ValidatorRegistry: []*pb.Validator{{Pubkey: pubKey}},
	}); err != nil {
		t.Fatalf("could not save state: %v", err)
	}

	block := &pb.BeaconBlock{Slot: 999}
	if err := beaconDB.SaveBlock(block); err != nil {
		t.Fatalf("could not save block: %v", err)
	}
	blockRoot, err := hashutil.HashBeaconBlock(block)
	if err != nil {
		log.Fatalf("could not hash block: %v", err)
	}

	service := NewAttestationService(context.Background(), &Config{BeaconDB: beaconDB})

	attestation := &pb.Attestation{
		Data: &pb.AttestationData{
			BeaconBlockRootHash32: blockRoot[:],
		}}
	pubKey48 := bytesutil.ToBytes48(pubKey)
	service.store.m[pubKey48] = attestation

	latestAttestedBlock, err := service.LatestAttestationTarget(ctx, 0)
	if err != nil {
		t.Fatalf("Could not get latest attestation: %v", err)
	}
	if !reflect.DeepEqual(block, latestAttestedBlock) {
		t.Errorf("Wanted: %v, got: %v", block, latestAttestedBlock)
	}
}

func TestUpdateLatestAttestation_CacheEnabledAndMiss(t *testing.T) {

	beaconDB := internal.SetupDB(t)
	defer internal.TeardownDB(t, beaconDB)
	ctx := context.Background()

	var validators []*pb.Validator
	for i := 0; i < 64; i++ {
		validators = append(validators, &pb.Validator{
			Pubkey:          []byte{byte(i)},
			ActivationEpoch: params.BeaconConfig().GenesisEpoch,
			ExitEpoch:       params.BeaconConfig().GenesisEpoch + 10,
		})
	}

	beaconState := &pb.BeaconState{
		Slot:              params.BeaconConfig().GenesisSlot + 1,
		ValidatorRegistry: validators,
	}
	block := &pb.BeaconBlock{
		Slot: params.BeaconConfig().GenesisSlot + 1,
	}
	if err := beaconDB.SaveBlock(block); err != nil {
		t.Fatal(err)
	}
	if err := beaconDB.UpdateChainHead(ctx, block, beaconState); err != nil {
		t.Fatal(err)
	}
	service := NewAttestationService(context.Background(), &Config{BeaconDB: beaconDB})

	attestation := &pb.Attestation{
		AggregationBitfield: []byte{0x80},
		Data: &pb.AttestationData{
			Slot:  params.BeaconConfig().GenesisSlot + 1,
			Shard: 1,
		},
	}

	if err := service.UpdateLatestAttestation(ctx, attestation); err != nil {
		t.Fatalf("could not update latest attestation: %v", err)
	}
	pubkey := bytesutil.ToBytes48([]byte{byte(3)})
	if service.store.m[pubkey].Data.Slot !=
		attestation.Data.Slot {
		t.Errorf("Incorrect slot stored, wanted: %d, got: %d",
			attestation.Data.Slot, service.store.m[pubkey].Data.Slot)
	}

	attestation.Data.Slot = params.BeaconConfig().GenesisSlot + 36
	attestation.Data.Shard = 36

	beaconState = &pb.BeaconState{
		Slot:              params.BeaconConfig().GenesisSlot + 36,
		ValidatorRegistry: validators,
	}
	if err := beaconDB.UpdateChainHead(ctx, block, beaconState); err != nil {
		t.Fatalf("could not save state: %v", err)
	}

	if err := service.UpdateLatestAttestation(ctx, attestation); err != nil {
		t.Fatalf("could not update latest attestation: %v", err)
	}
	if service.store.m[pubkey].Data.Slot !=
		attestation.Data.Slot {
		t.Errorf("Incorrect slot stored, wanted: %d, got: %d",
			attestation.Data.Slot, service.store.m[pubkey].Data.Slot)
	}

	// Verify the committee for attestation's data slot was cached.
	fetchedCommittees, err := committeeCache.CommitteesInfoBySlot(attestation.Data.Slot)
	if err != nil {
		t.Fatal(err)
	}
	wantedCommittee := []uint64{38}
	if !reflect.DeepEqual(wantedCommittee, fetchedCommittees.Committees[0].Committee) {
		t.Errorf(
			"Result indices was an unexpected value. Wanted %d, got %d",
			wantedCommittee,
			fetchedCommittees.Committees[0].Committee,
		)
	}
}

func TestUpdateLatestAttestation_CacheEnabledAndHit(t *testing.T) {

	var validators []*pb.Validator
	for i := 0; i < 64; i++ {
		validators = append(validators, &pb.Validator{
			Pubkey:          []byte{byte(i)},
			ActivationEpoch: params.BeaconConfig().GenesisEpoch,
			ExitEpoch:       params.BeaconConfig().GenesisEpoch + 10,
		})
	}

	beaconDB := internal.SetupDB(t)
	defer internal.TeardownDB(t, beaconDB)
	ctx := context.Background()

	beaconState := &pb.BeaconState{
		Slot:              params.BeaconConfig().GenesisSlot + 2,
		ValidatorRegistry: validators,
	}
	block := &pb.BeaconBlock{
		Slot: params.BeaconConfig().GenesisSlot + 2,
	}
	if err := beaconDB.SaveBlock(block); err != nil {
		t.Fatal(err)
	}
	if err := beaconDB.UpdateChainHead(ctx, block, beaconState); err != nil {
		t.Fatal(err)
	}

	service := NewAttestationService(context.Background(), &Config{BeaconDB: beaconDB})

	slot := params.BeaconConfig().GenesisSlot + 2
	shard := uint64(3)
	index := uint64(4)
	attestation := &pb.Attestation{
		AggregationBitfield: []byte{0x80},
		Data: &pb.AttestationData{
			Slot:  slot,
			Shard: shard,
		},
	}

	csInSlot := &cache.CommitteesInSlot{
		Slot: slot,
		Committees: []*cache.CommitteeInfo{
			{Shard: shard, Committee: []uint64{index, 999}},
		}}

	if err := committeeCache.AddCommittees(csInSlot); err != nil {
		t.Fatal(err)
	}

	if err := service.UpdateLatestAttestation(ctx, attestation); err != nil {
		t.Fatalf("could not update latest attestation: %v", err)
	}
	pubkey := bytesutil.ToBytes48([]byte{byte(index)})
	if err := service.UpdateLatestAttestation(ctx, attestation); err != nil {
		t.Fatalf("could not update latest attestation: %v", err)
	}

	if service.store.m[pubkey].Data.Slot !=
		attestation.Data.Slot {
		t.Errorf("Incorrect slot stored, wanted: %d, got: %d",
			attestation.Data.Slot, service.store.m[pubkey].Data.Slot)
	}
}
