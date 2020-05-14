package stategen

import (
	"context"
	"testing"

	ethpb "github.com/prysmaticlabs/ethereumapis/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/beacon-chain/cache"
	testDB "github.com/prysmaticlabs/prysm/beacon-chain/db/testing"
	"github.com/prysmaticlabs/prysm/beacon-chain/state/stateutil"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/testutil"
	logTest "github.com/sirupsen/logrus/hooks/test"
)

func TestMigrateToCold_NoBlock(t *testing.T) {
	hook := logTest.NewGlobal()
	ctx := context.Background()
	db := testDB.SetupDB(t)

	service := New(db, cache.NewStateSummaryCache())
	service.splitInfo.slot = 1
	if err := service.MigrateToCold(ctx, params.BeaconConfig().SlotsPerEpoch, [32]byte{}); err != nil {
		t.Fatal(err)
	}

	testutil.AssertLogsContain(t, hook, "Set hot and cold state split point")
}

func TestMigrateToCold_HigherSplitSlot(t *testing.T) {
	hook := logTest.NewGlobal()
	ctx := context.Background()
	db := testDB.SetupDB(t)

	service := New(db, cache.NewStateSummaryCache())
	service.splitInfo.slot = 2
	if err := service.MigrateToCold(ctx, 1, [32]byte{}); err != nil {
		t.Fatal(err)
	}

	testutil.AssertLogsDoNotContain(t, hook, "Set hot and cold state split point")
}

func TestMigrateToCold_MigrationCompletes(t *testing.T) {
	hook := logTest.NewGlobal()
	ctx := context.Background()
	db := testDB.SetupDB(t)

	service := New(db, cache.NewStateSummaryCache())
	service.splitInfo.slot = 1
	service.slotsPerArchivedPoint = 2

	beaconState, _ := testutil.DeterministicGenesisState(t, 32)
	if err := beaconState.SetSlot(params.BeaconConfig().SlotsPerEpoch); err != nil {
		t.Fatal(err)
	}
	b := &ethpb.SignedBeaconBlock{
		Block: &ethpb.BeaconBlock{Slot: 2},
	}
	if err := service.beaconDB.SaveBlock(ctx, b); err != nil {
		t.Fatal(err)
	}
	bRoot, err := stateutil.BlockRoot(b.Block)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.beaconDB.SaveStateSummary(ctx, &pb.StateSummary{Root: bRoot[:], Slot: 2}); err != nil {
		t.Fatal(err)
	}
	if err := service.beaconDB.SaveState(ctx, beaconState, bRoot); err != nil {
		t.Fatal(err)
	}

	newBeaconState, _ := testutil.DeterministicGenesisState(t, 32)
	if err := newBeaconState.SetSlot(3); err != nil {
		t.Fatal(err)
	}
	b = &ethpb.SignedBeaconBlock{
		Block: &ethpb.BeaconBlock{Slot: 3},
	}
	if err := service.beaconDB.SaveBlock(ctx, b); err != nil {
		t.Fatal(err)
	}
	bRoot, err = stateutil.BlockRoot(b.Block)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.beaconDB.SaveStateSummary(ctx, &pb.StateSummary{Root: bRoot[:], Slot: 3}); err != nil {
		t.Fatal(err)
	}
	if err := service.beaconDB.SaveState(ctx, newBeaconState, bRoot); err != nil {
		t.Fatal(err)
	}

	if err := service.MigrateToCold(ctx, beaconState.Slot(), [32]byte{}); err != nil {
		t.Fatal(err)
	}

	if !service.beaconDB.HasArchivedPoint(ctx, 1) {
		t.Error("Did not preserve archived point")
	}

	testutil.AssertLogsContain(t, hook, "Saved archived point during state migration")
	testutil.AssertLogsContain(t, hook, "Deleted state during migration")
	testutil.AssertLogsContain(t, hook, "Set hot and cold state split point")
}

func TestMigrateToCold_CantDeleteCurrentArchivedIndex(t *testing.T) {
	ctx := context.Background()
	db := testDB.SetupDB(t)

	service := New(db, cache.NewStateSummaryCache())
	service.splitInfo.slot = 1
	service.slotsPerArchivedPoint = 2

	beaconState, _ := testutil.DeterministicGenesisState(t, 32)
	if err := beaconState.SetSlot(params.BeaconConfig().SlotsPerEpoch); err != nil {
		t.Fatal(err)
	}
	b := &ethpb.SignedBeaconBlock{
		Block: &ethpb.BeaconBlock{Slot: 2},
	}
	if err := service.beaconDB.SaveBlock(ctx, b); err != nil {
		t.Fatal(err)
	}
	bRoot, err := stateutil.BlockRoot(b.Block)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.beaconDB.SaveStateSummary(ctx, &pb.StateSummary{Root: bRoot[:], Slot: 2}); err != nil {
		t.Fatal(err)
	}
	if err := service.beaconDB.SaveState(ctx, beaconState, bRoot); err != nil {
		t.Fatal(err)
	}

	newBeaconState, _ := testutil.DeterministicGenesisState(t, 32)
	if err := newBeaconState.SetSlot(3); err != nil {
		t.Fatal(err)
	}
	b = &ethpb.SignedBeaconBlock{
		Block: &ethpb.BeaconBlock{Slot: 3},
	}
	if err := service.beaconDB.SaveBlock(ctx, b); err != nil {
		t.Fatal(err)
	}
	bRoot, err = stateutil.BlockRoot(b.Block)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.beaconDB.SaveStateSummary(ctx, &pb.StateSummary{Root: bRoot[:], Slot: 3}); err != nil {
		t.Fatal(err)
	}
	if err := service.beaconDB.SaveState(ctx, newBeaconState, bRoot); err != nil {
		t.Fatal(err)
	}
	if err := service.beaconDB.SaveArchivedPointRoot(ctx, bRoot, 1); err != nil {
		t.Fatal(err)
	}
	if err := service.beaconDB.SaveLastArchivedIndex(ctx, 1); err != nil {
		t.Fatal(err)
	}

	if err := service.MigrateToCold(ctx, beaconState.Slot(), [32]byte{}); err != nil {
		t.Fatal(err)
	}

	if !service.beaconDB.HasArchivedPoint(ctx, 1) {
		t.Error("Did not preserve archived point")
	}
	if !service.beaconDB.HasState(ctx, bRoot) {
		t.Error("State should not be deleted")
	}
}
