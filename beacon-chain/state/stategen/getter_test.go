package stategen

import (
	"context"
	"testing"

	"github.com/gogo/protobuf/proto"
	ethpb "github.com/prysmaticlabs/ethereumapis/eth/v1alpha1"
	"github.com/prysmaticlabs/go-ssz"
	"github.com/prysmaticlabs/prysm/beacon-chain/cache"
	testDB "github.com/prysmaticlabs/prysm/beacon-chain/db/testing"
	"github.com/prysmaticlabs/prysm/beacon-chain/state/stateutil"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/testutil"
	"github.com/prysmaticlabs/prysm/shared/testutil/assert"
	"github.com/prysmaticlabs/prysm/shared/testutil/require"
)

func TestStateByRoot_ColdState(t *testing.T) {
	ctx := context.Background()
	db, _ := testDB.SetupDB(t)

	service := New(db, cache.NewStateSummaryCache())
	service.finalizedInfo.slot = 2
	service.slotsPerArchivedPoint = 1

	b := &ethpb.SignedBeaconBlock{Block: &ethpb.BeaconBlock{Slot: 1}}
	require.NoError(t, db.SaveBlock(ctx, b))
	bRoot, err := stateutil.BlockRoot(b.Block)
	require.NoError(t, err)
	beaconState, _ := testutil.DeterministicGenesisState(t, 32)
	require.NoError(t, beaconState.SetSlot(1))
	require.NoError(t, service.beaconDB.SaveState(ctx, beaconState, bRoot))
	require.NoError(t, service.beaconDB.SaveBlock(ctx, b))
	require.NoError(t, service.beaconDB.SaveGenesisBlockRoot(ctx, bRoot))
	loadedState, err := service.StateByRoot(ctx, bRoot)
	require.NoError(t, err)
	if !proto.Equal(loadedState.InnerStateUnsafe(), beaconState.InnerStateUnsafe()) {
		t.Error("Did not correctly save state")
	}
}

func TestStateByRoot_HotStateUsingEpochBoundaryCacheNoReplay(t *testing.T) {
	ctx := context.Background()
	db, _ := testDB.SetupDB(t)

	service := New(db, cache.NewStateSummaryCache())

	beaconState, _ := testutil.DeterministicGenesisState(t, 32)
	require.NoError(t, beaconState.SetSlot(10))
	blk := &ethpb.SignedBeaconBlock{Block: &ethpb.BeaconBlock{}}
	blkRoot, err := stateutil.BlockRoot(blk.Block)
	require.NoError(t, err)
	require.NoError(t, service.beaconDB.SaveStateSummary(ctx, &pb.StateSummary{Root: blkRoot[:]}))
	require.NoError(t, service.epochBoundaryStateCache.put(blkRoot, beaconState))
	loadedState, err := service.StateByRoot(ctx, blkRoot)
	require.NoError(t, err)
	assert.Equal(t, uint64(10), loadedState.Slot(), "Did not correctly load state")
}

func TestStateByRoot_HotStateUsingEpochBoundaryCacheWithReplay(t *testing.T) {
	ctx := context.Background()
	db, ssc := testDB.SetupDB(t)

	service := New(db, ssc)

	beaconState, _ := testutil.DeterministicGenesisState(t, 32)
	blk := &ethpb.SignedBeaconBlock{Block: &ethpb.BeaconBlock{}}
	blkRoot, err := stateutil.BlockRoot(blk.Block)
	require.NoError(t, err)
	require.NoError(t, service.epochBoundaryStateCache.put(blkRoot, beaconState))
	targetSlot := uint64(10)
	targetBlock := &ethpb.SignedBeaconBlock{Block: &ethpb.BeaconBlock{Slot: 11, ParentRoot: blkRoot[:], ProposerIndex: 8}}
	require.NoError(t, service.beaconDB.SaveBlock(ctx, targetBlock))
	targetRoot, err := stateutil.BlockRoot(targetBlock.Block)
	require.NoError(t, err)
	require.NoError(t, service.beaconDB.SaveStateSummary(ctx, &pb.StateSummary{Slot: targetSlot, Root: targetRoot[:]}))
	loadedState, err := service.StateByRoot(ctx, targetRoot)
	require.NoError(t, err)
	assert.Equal(t, targetSlot, loadedState.Slot(), "Did not correctly load state")
}

func TestStateByRoot_HotStateCached(t *testing.T) {
	ctx := context.Background()
	db, _ := testDB.SetupDB(t)

	service := New(db, cache.NewStateSummaryCache())

	beaconState, _ := testutil.DeterministicGenesisState(t, 32)
	r := [32]byte{'A'}
	require.NoError(t, service.beaconDB.SaveStateSummary(ctx, &pb.StateSummary{Root: r[:]}))
	service.hotStateCache.Put(r, beaconState)

	loadedState, err := service.StateByRoot(ctx, r)
	require.NoError(t, err)
	if !proto.Equal(loadedState.InnerStateUnsafe(), beaconState.InnerStateUnsafe()) {
		t.Error("Did not correctly cache state")
	}
}

func TestStateByRootInitialSync_UseEpochStateCache(t *testing.T) {
	ctx := context.Background()
	db, ssc := testDB.SetupDB(t)

	service := New(db, ssc)

	beaconState, _ := testutil.DeterministicGenesisState(t, 32)
	targetSlot := uint64(10)
	require.NoError(t, beaconState.SetSlot(targetSlot))
	blk := &ethpb.SignedBeaconBlock{Block: &ethpb.BeaconBlock{}}
	blkRoot, err := stateutil.BlockRoot(blk.Block)
	require.NoError(t, err)
	require.NoError(t, service.epochBoundaryStateCache.put(blkRoot, beaconState))
	loadedState, err := service.StateByRootInitialSync(ctx, blkRoot)
	require.NoError(t, err)
	assert.Equal(t, targetSlot, loadedState.Slot(), "Did not correctly load state")
}

func TestStateByRootInitialSync_UseCache(t *testing.T) {
	ctx := context.Background()
	db, _ := testDB.SetupDB(t)

	service := New(db, cache.NewStateSummaryCache())

	beaconState, _ := testutil.DeterministicGenesisState(t, 32)
	r := [32]byte{'A'}
	require.NoError(t, service.beaconDB.SaveStateSummary(ctx, &pb.StateSummary{Root: r[:]}))
	service.hotStateCache.Put(r, beaconState)

	loadedState, err := service.StateByRootInitialSync(ctx, r)
	require.NoError(t, err)
	if !proto.Equal(loadedState.InnerStateUnsafe(), beaconState.InnerStateUnsafe()) {
		t.Error("Did not correctly cache state")
	}
	if service.hotStateCache.Has(r) {
		t.Error("Hot state cache was not invalidated")
	}
}

func TestStateByRootInitialSync_CanProcessUpTo(t *testing.T) {
	ctx := context.Background()
	db, _ := testDB.SetupDB(t)
	service := New(db, cache.NewStateSummaryCache())

	beaconState, _ := testutil.DeterministicGenesisState(t, 32)
	blk := &ethpb.SignedBeaconBlock{Block: &ethpb.BeaconBlock{}}
	blkRoot, err := stateutil.BlockRoot(blk.Block)
	require.NoError(t, err)
	require.NoError(t, service.epochBoundaryStateCache.put(blkRoot, beaconState))
	targetSlot := uint64(10)
	targetBlk := &ethpb.SignedBeaconBlock{Block: &ethpb.BeaconBlock{Slot: 11, ParentRoot: blkRoot[:]}}
	targetRoot, err := stateutil.BlockRoot(targetBlk.Block)
	require.NoError(t, err)
	require.NoError(t, service.beaconDB.SaveBlock(ctx, targetBlk))
	require.NoError(t, service.beaconDB.SaveStateSummary(ctx, &pb.StateSummary{Slot: targetSlot, Root: targetRoot[:]}))

	loadedState, err := service.StateByRootInitialSync(ctx, targetRoot)
	require.NoError(t, err)
	assert.Equal(t, targetSlot, loadedState.Slot(), "Did not correctly load state")
}

func TestStateBySlot_ColdState(t *testing.T) {
	ctx := context.Background()
	db, _ := testDB.SetupDB(t)

	service := New(db, cache.NewStateSummaryCache())
	service.slotsPerArchivedPoint = params.BeaconConfig().SlotsPerEpoch * 2
	service.finalizedInfo.slot = service.slotsPerArchivedPoint + 1

	beaconState, _ := testutil.DeterministicGenesisState(t, 32)
	require.NoError(t, beaconState.SetSlot(1))
	b := &ethpb.SignedBeaconBlock{Block: &ethpb.BeaconBlock{Slot: 1}}
	require.NoError(t, db.SaveBlock(ctx, b))
	bRoot, err := stateutil.BlockRoot(b.Block)
	require.NoError(t, err)
	require.NoError(t, db.SaveState(ctx, beaconState, bRoot))
	require.NoError(t, db.SaveGenesisBlockRoot(ctx, bRoot))

	r := [32]byte{}
	require.NoError(t, service.beaconDB.SaveStateSummary(ctx, &pb.StateSummary{Slot: service.slotsPerArchivedPoint, Root: r[:]}))

	slot := uint64(20)
	loadedState, err := service.StateBySlot(ctx, slot)
	require.NoError(t, err)
	assert.Equal(t, slot, loadedState.Slot(), "Did not correctly save state")
}

func TestStateBySlot_HotStateDB(t *testing.T) {
	ctx := context.Background()
	db, _ := testDB.SetupDB(t)

	service := New(db, cache.NewStateSummaryCache())

	beaconState, _ := testutil.DeterministicGenesisState(t, 32)
	b := &ethpb.SignedBeaconBlock{Block: &ethpb.BeaconBlock{}}
	require.NoError(t, db.SaveBlock(ctx, b))
	bRoot, err := stateutil.BlockRoot(b.Block)
	require.NoError(t, err)
	require.NoError(t, db.SaveState(ctx, beaconState, bRoot))
	require.NoError(t, db.SaveGenesisBlockRoot(ctx, bRoot))

	slot := uint64(10)
	loadedState, err := service.StateBySlot(ctx, slot)
	require.NoError(t, err)
	assert.Equal(t, slot, loadedState.Slot(), "Did not correctly load state")
}

func TestStateSummary_CanGetFromCacheOrDB(t *testing.T) {
	ctx := context.Background()
	db, _ := testDB.SetupDB(t)

	service := New(db, cache.NewStateSummaryCache())

	r := [32]byte{'a'}
	summary := &pb.StateSummary{Slot: 100}
	_, err := service.stateSummary(ctx, r)
	require.ErrorContains(t, errUnknownStateSummary.Error(), err)

	service.stateSummaryCache.Put(r, summary)
	got, err := service.stateSummary(ctx, r)
	require.NoError(t, err)
	if !proto.Equal(got, summary) {
		t.Error("Did not get wanted summary")
	}

	r = [32]byte{'b'}
	summary = &pb.StateSummary{Root: r[:], Slot: 101}
	_, err = service.stateSummary(ctx, r)
	require.ErrorContains(t, errUnknownStateSummary.Error(), err)

	require.NoError(t, service.beaconDB.SaveStateSummary(ctx, summary))
	got, err = service.stateSummary(ctx, r)
	require.NoError(t, err)
	if !proto.Equal(got, summary) {
		t.Error("Did not get wanted summary")
	}
}

func TestLoadeStateByRoot_Cached(t *testing.T) {
	ctx := context.Background()
	db, _ := testDB.SetupDB(t)
	service := New(db, cache.NewStateSummaryCache())

	beaconState, _ := testutil.DeterministicGenesisState(t, 32)
	r := [32]byte{'A'}
	service.hotStateCache.Put(r, beaconState)

	// This tests where hot state was already cached.
	loadedState, err := service.loadStateByRoot(ctx, r)
	require.NoError(t, err)

	if !proto.Equal(loadedState.InnerStateUnsafe(), beaconState.InnerStateUnsafe()) {
		t.Error("Did not correctly cache state")
	}
}

func TestLoadeStateByRoot_EpochBoundaryStateCanProcess(t *testing.T) {
	ctx := context.Background()
	db, ssc := testDB.SetupDB(t)
	service := New(db, ssc)

	beaconState, _ := testutil.DeterministicGenesisState(t, 32)
	gBlk := &ethpb.SignedBeaconBlock{Block: &ethpb.BeaconBlock{}}
	gBlkRoot, err := stateutil.BlockRoot(gBlk.Block)
	require.NoError(t, err)
	require.NoError(t, service.epochBoundaryStateCache.put(gBlkRoot, beaconState))

	blk := &ethpb.SignedBeaconBlock{Block: &ethpb.BeaconBlock{Slot: 11, ParentRoot: gBlkRoot[:], ProposerIndex: 8}}
	require.NoError(t, service.beaconDB.SaveBlock(ctx, blk))
	blkRoot, err := stateutil.BlockRoot(blk.Block)
	require.NoError(t, err)
	require.NoError(t, service.beaconDB.SaveStateSummary(ctx, &pb.StateSummary{Slot: 10, Root: blkRoot[:]}))

	// This tests where hot state was not cached and needs processing.
	loadedState, err := service.loadStateByRoot(ctx, blkRoot)
	require.NoError(t, err)
	assert.Equal(t, uint64(10), loadedState.Slot(), "Did not correctly load state")
}

func TestLoadeStateByRoot_FromDBBoundaryCase(t *testing.T) {
	ctx := context.Background()
	db, _ := testDB.SetupDB(t)
	service := New(db, cache.NewStateSummaryCache())

	beaconState, _ := testutil.DeterministicGenesisState(t, 32)
	blk := &ethpb.SignedBeaconBlock{Block: &ethpb.BeaconBlock{}}
	blkRoot, err := stateutil.BlockRoot(blk.Block)
	require.NoError(t, err)
	require.NoError(t, service.epochBoundaryStateCache.put(blkRoot, beaconState))
	targetSlot := uint64(0)
	require.NoError(t, service.beaconDB.SaveStateSummary(ctx, &pb.StateSummary{Slot: targetSlot, Root: blkRoot[:]}))

	// This tests where hot state was not cached but doesn't need processing
	// because it on the epoch boundary slot.
	loadedState, err := service.loadStateByRoot(ctx, blkRoot)
	require.NoError(t, err)
	assert.Equal(t, targetSlot, loadedState.Slot(), "Did not correctly load state")
}

func TestLoadeStateBySlot_CanAdvanceSlotUsingDB(t *testing.T) {
	ctx := context.Background()
	db, _ := testDB.SetupDB(t)
	service := New(db, cache.NewStateSummaryCache())
	beaconState, _ := testutil.DeterministicGenesisState(t, 32)
	b := &ethpb.SignedBeaconBlock{Block: &ethpb.BeaconBlock{}}
	require.NoError(t, service.beaconDB.SaveBlock(ctx, b))
	gRoot, err := stateutil.BlockRoot(b.Block)
	require.NoError(t, err)
	require.NoError(t, service.beaconDB.SaveGenesisBlockRoot(ctx, gRoot))
	require.NoError(t, service.beaconDB.SaveState(ctx, beaconState, gRoot))

	slot := uint64(10)
	loadedState, err := service.loadStateBySlot(ctx, slot)
	require.NoError(t, err)
	assert.Equal(t, slot, loadedState.Slot(), "Did not correctly load state")
}

func TestLastAncestorState_CanGetUsingDB(t *testing.T) {
	ctx := context.Background()
	db, _ := testDB.SetupDB(t)
	service := New(db, cache.NewStateSummaryCache())

	b0 := testutil.NewBeaconBlock()
	b0.Block.ParentRoot = bytesutil.PadTo([]byte{'a'}, 32)
	r0, err := ssz.HashTreeRoot(b0.Block)
	require.NoError(t, err)
	b1 := testutil.NewBeaconBlock()
	b1.Block.Slot = 1
	b1.Block.ParentRoot = bytesutil.PadTo(r0[:], 32)
	r1, err := ssz.HashTreeRoot(b1.Block)
	require.NoError(t, err)
	b2 := testutil.NewBeaconBlock()
	b2.Block.Slot = 2
	b2.Block.ParentRoot = bytesutil.PadTo(r1[:], 32)
	r2, err := ssz.HashTreeRoot(b2.Block)
	require.NoError(t, err)
	b3 := testutil.NewBeaconBlock()
	b3.Block.Slot = 3
	b3.Block.ParentRoot = bytesutil.PadTo(r2[:], 32)
	r3, err := ssz.HashTreeRoot(b3.Block)
	require.NoError(t, err)

	b1State := testutil.NewBeaconState()
	require.NoError(t, b1State.SetSlot(1))

	require.NoError(t, service.beaconDB.SaveBlock(ctx, b0))
	require.NoError(t, service.beaconDB.SaveBlock(ctx, b1))
	require.NoError(t, service.beaconDB.SaveBlock(ctx, b2))
	require.NoError(t, service.beaconDB.SaveBlock(ctx, b3))
	require.NoError(t, service.beaconDB.SaveState(ctx, b1State, r1))

	lastState, err := service.lastAncestorState(ctx, r3)
	require.NoError(t, err)
	assert.Equal(t, b1State.Slot(), lastState.Slot(), "Did not get wanted state")
}

func TestLastAncestorState_CanGetUsingCache(t *testing.T) {
	ctx := context.Background()
	db, _ := testDB.SetupDB(t)
	service := New(db, cache.NewStateSummaryCache())

	b0 := testutil.NewBeaconBlock()
	b0.Block.ParentRoot = bytesutil.PadTo([]byte{'a'}, 32)
	r0, err := ssz.HashTreeRoot(b0.Block)
	require.NoError(t, err)
	b1 := testutil.NewBeaconBlock()
	b1.Block.Slot = 1
	b1.Block.ParentRoot = bytesutil.PadTo(r0[:], 32)
	r1, err := ssz.HashTreeRoot(b1.Block)
	require.NoError(t, err)
	b2 := testutil.NewBeaconBlock()
	b2.Block.Slot = 2
	b2.Block.ParentRoot = bytesutil.PadTo(r1[:], 32)
	r2, err := ssz.HashTreeRoot(b2.Block)
	require.NoError(t, err)
	b3 := testutil.NewBeaconBlock()
	b3.Block.Slot = 3
	b3.Block.ParentRoot = bytesutil.PadTo(r2[:], 32)
	r3, err := ssz.HashTreeRoot(b3.Block)
	require.NoError(t, err)

	b1State := testutil.NewBeaconState()
	require.NoError(t, b1State.SetSlot(1))

	require.NoError(t, service.beaconDB.SaveBlock(ctx, b0))
	require.NoError(t, service.beaconDB.SaveBlock(ctx, b1))
	require.NoError(t, service.beaconDB.SaveBlock(ctx, b2))
	require.NoError(t, service.beaconDB.SaveBlock(ctx, b3))
	service.hotStateCache.Put(r1, b1State)

	lastState, err := service.lastAncestorState(ctx, r3)
	require.NoError(t, err)
	assert.Equal(t, b1State.Slot(), lastState.Slot(), "Did not get wanted state")
}
