package blockchain

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"

	ethpb "github.com/prysmaticlabs/ethereumapis/eth/v1alpha1"
	"github.com/prysmaticlabs/go-ssz"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/blocks"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	"github.com/prysmaticlabs/prysm/beacon-chain/db"
	testDB "github.com/prysmaticlabs/prysm/beacon-chain/db/testing"
	stateTrie "github.com/prysmaticlabs/prysm/beacon-chain/state"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/prysmaticlabs/prysm/shared/featureconfig"
	"github.com/prysmaticlabs/prysm/shared/params"
)

func TestStore_OnBlock(t *testing.T) {
	ctx := context.Background()
	db := testDB.SetupDB(t)
	defer testDB.TeardownDB(t, db)

	cfg := &Config{BeaconDB: db}
	service, err := NewService(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}

	genesisStateRoot := [32]byte{}
	genesis := blocks.NewGenesisBlock(genesisStateRoot[:])
	if err := db.SaveBlock(ctx, genesis); err != nil {
		t.Error(err)
	}
	validGenesisRoot, err := ssz.HashTreeRoot(genesis.Block)
	if err != nil {
		t.Error(err)
	}
	if err := service.beaconDB.SaveState(ctx, &stateTrie.BeaconState{}, validGenesisRoot); err != nil {
		t.Fatal(err)
	}
	roots, err := blockTree1(db, validGenesisRoot[:])
	if err != nil {
		t.Fatal(err)
	}
	random := &ethpb.SignedBeaconBlock{Block: &ethpb.BeaconBlock{Slot: 1, ParentRoot: validGenesisRoot[:]}}
	if err := db.SaveBlock(ctx, random); err != nil {
		t.Error(err)
	}
	randomParentRoot, err := ssz.HashTreeRoot(random.Block)
	if err != nil {
		t.Error(err)
	}
	if err := service.beaconDB.SaveState(ctx, &stateTrie.BeaconState{}, randomParentRoot); err != nil {
		t.Fatal(err)
	}
	randomParentRoot2 := roots[1]
	if err := service.beaconDB.SaveState(ctx, &stateTrie.BeaconState{}, bytesutil.ToBytes32(randomParentRoot2)); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name          string
		blk           *ethpb.BeaconBlock
		s             *stateTrie.BeaconState
		time          uint64
		wantErrString string
	}{
		{
			name:          "parent block root does not have a state",
			blk:           &ethpb.BeaconBlock{},
			s:             &stateTrie.BeaconState{},
			wantErrString: "pre state of slot 0 does not exist",
		},
		{
			name:          "block is from the feature",
			blk:           &ethpb.BeaconBlock{ParentRoot: randomParentRoot[:], Slot: params.BeaconConfig().FarFutureEpoch},
			s:             &stateTrie.BeaconState{},
			wantErrString: "could not process slot from the future",
		},
		{
			name:          "could not get finalized block",
			blk:           &ethpb.BeaconBlock{ParentRoot: randomParentRoot[:]},
			s:             &stateTrie.BeaconState{},
			wantErrString: "block from slot 0 is not a descendent of the current finalized block",
		},
		{
			name:          "same slot as finalized block",
			blk:           &ethpb.BeaconBlock{Slot: 0, ParentRoot: randomParentRoot2},
			s:             &stateTrie.BeaconState{},
			wantErrString: "block is equal or earlier than finalized block, slot 0 < slot 0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service.justifiedCheckpt = &ethpb.Checkpoint{Root: validGenesisRoot[:]}
			service.bestJustifiedCheckpt = &ethpb.Checkpoint{Root: validGenesisRoot[:]}
			service.finalizedCheckpt = &ethpb.Checkpoint{Root: validGenesisRoot[:]}
			service.prevFinalizedCheckpt = &ethpb.Checkpoint{Root: validGenesisRoot[:]}
			service.finalizedCheckpt.Root = roots[0]

			_, err := service.onBlock(ctx, &ethpb.SignedBeaconBlock{Block: tt.blk})
			if !strings.Contains(err.Error(), tt.wantErrString) {
				t.Errorf("Store.OnBlock() error = %v, wantErr = %v", err, tt.wantErrString)
			}
		})
	}
}

func TestStore_SaveNewValidators(t *testing.T) {
	ctx := context.Background()
	db := testDB.SetupDB(t)
	defer testDB.TeardownDB(t, db)

	cfg := &Config{BeaconDB: db}
	service, err := NewService(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	preCount := 2 // validators 0 and validators 1
	s, _ := stateTrie.InitializeFromProto(&pb.BeaconState{Validators: []*ethpb.Validator{
		{PublicKey: []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}},
		{PublicKey: []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}},
		{PublicKey: []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 2}},
		{PublicKey: []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 3}},
	}})
	if err := service.saveNewValidators(ctx, preCount, s); err != nil {
		t.Fatal(err)
	}

	if !db.HasValidatorIndex(ctx, []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 2}) {
		t.Error("Wanted validator saved in db")
	}
	if !db.HasValidatorIndex(ctx, []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 3}) {
		t.Error("Wanted validator saved in db")
	}
	if db.HasValidatorIndex(ctx, []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}) {
		t.Error("validator not suppose to be saved in db")
	}
}

func TestRemoveStateSinceLastFinalized(t *testing.T) {
	ctx := context.Background()
	db := testDB.SetupDB(t)
	defer testDB.TeardownDB(t, db)
	params.UseMinimalConfig()
	defer params.UseMainnetConfig()

	cfg := &Config{BeaconDB: db}
	service, err := NewService(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}

	// Save 100 blocks in DB, each has a state.
	numBlocks := 100
	totalBlocks := make([]*ethpb.SignedBeaconBlock, numBlocks)
	blockRoots := make([][32]byte, 0)
	for i := 0; i < len(totalBlocks); i++ {
		totalBlocks[i] = &ethpb.SignedBeaconBlock{
			Block: &ethpb.BeaconBlock{
				Slot: uint64(i),
			},
		}
		r, err := ssz.HashTreeRoot(totalBlocks[i].Block)
		if err != nil {
			t.Fatal(err)
		}
		s, _ := stateTrie.InitializeFromProto(&pb.BeaconState{Slot: uint64(i)})
		if err := service.beaconDB.SaveState(ctx, s, r); err != nil {
			t.Fatal(err)
		}
		if err := service.beaconDB.SaveBlock(ctx, totalBlocks[i]); err != nil {
			t.Fatal(err)
		}
		blockRoots = append(blockRoots, r)
		if err := service.beaconDB.SaveHeadBlockRoot(ctx, r); err != nil {
			t.Fatal(err)
		}
	}

	// New finalized epoch: 1
	finalizedEpoch := uint64(1)
	finalizedSlot := finalizedEpoch * params.BeaconConfig().SlotsPerEpoch
	endSlot := helpers.StartSlot(finalizedEpoch+1) - 1 // Inclusive
	if err := service.rmStatesOlderThanLastFinalized(ctx, 0, endSlot); err != nil {
		t.Fatal(err)
	}
	for _, r := range blockRoots {
		s, err := service.beaconDB.State(ctx, r)
		if err != nil {
			t.Fatal(err)
		}
		// Also verifies genesis state didnt get deleted
		if s != nil && s.Slot() != finalizedSlot && s.Slot() != 0 && s.Slot() < endSlot {
			t.Errorf("State with slot %d should not be in DB", s.Slot())
		}
	}

	// New finalized epoch: 5
	newFinalizedEpoch := uint64(5)
	newFinalizedSlot := newFinalizedEpoch * params.BeaconConfig().SlotsPerEpoch
	endSlot = helpers.StartSlot(newFinalizedEpoch+1) - 1 // Inclusive
	if err := service.rmStatesOlderThanLastFinalized(ctx, helpers.StartSlot(finalizedEpoch+1)-1, endSlot); err != nil {
		t.Fatal(err)
	}
	for _, r := range blockRoots {
		s, err := service.beaconDB.State(ctx, r)
		if err != nil {
			t.Fatal(err)
		}
		// Also verifies genesis state didnt get deleted
		if s != nil && s.Slot() != newFinalizedSlot && s.Slot() != finalizedSlot && s.Slot() != 0 && s.Slot() < endSlot {
			t.Errorf("State with slot %d should not be in DB", s.Slot())
		}
	}
}

func TestRemoveStateSinceLastFinalized_EmptyStartSlot(t *testing.T) {
	ctx := context.Background()
	db := testDB.SetupDB(t)
	defer testDB.TeardownDB(t, db)
	params.UseMinimalConfig()
	defer params.UseMainnetConfig()

	cfg := &Config{BeaconDB: db}
	service, err := NewService(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	service.genesisTime = time.Now()

	update, err := service.shouldUpdateCurrentJustified(ctx, &ethpb.Checkpoint{})
	if err != nil {
		t.Fatal(err)
	}
	if !update {
		t.Error("Should be able to update justified, received false")
	}

	lastJustifiedBlk := &ethpb.SignedBeaconBlock{Block: &ethpb.BeaconBlock{ParentRoot: []byte{'G'}}}
	lastJustifiedRoot, _ := ssz.HashTreeRoot(lastJustifiedBlk.Block)
	newJustifiedBlk := &ethpb.SignedBeaconBlock{Block: &ethpb.BeaconBlock{Slot: 1, ParentRoot: lastJustifiedRoot[:]}}
	newJustifiedRoot, _ := ssz.HashTreeRoot(newJustifiedBlk.Block)
	if err := service.beaconDB.SaveBlock(ctx, newJustifiedBlk); err != nil {
		t.Fatal(err)
	}
	if err := service.beaconDB.SaveBlock(ctx, lastJustifiedBlk); err != nil {
		t.Fatal(err)
	}

	diff := (params.BeaconConfig().SlotsPerEpoch - 1) * params.BeaconConfig().SecondsPerSlot
	service.genesisTime = time.Unix(time.Now().Unix()-int64(diff), 0)
	service.justifiedCheckpt = &ethpb.Checkpoint{Root: lastJustifiedRoot[:]}
	update, err = service.shouldUpdateCurrentJustified(ctx, &ethpb.Checkpoint{Root: newJustifiedRoot[:]})
	if err != nil {
		t.Fatal(err)
	}
	if !update {
		t.Error("Should be able to update justified, received false")
	}
}

func TestShouldUpdateJustified_ReturnFalse(t *testing.T) {
	ctx := context.Background()
	db := testDB.SetupDB(t)
	defer testDB.TeardownDB(t, db)
	params.UseMinimalConfig()
	defer params.UseMainnetConfig()

	cfg := &Config{BeaconDB: db}
	service, err := NewService(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	lastJustifiedBlk := &ethpb.SignedBeaconBlock{Block: &ethpb.BeaconBlock{ParentRoot: []byte{'G'}}}
	lastJustifiedRoot, _ := ssz.HashTreeRoot(lastJustifiedBlk.Block)
	newJustifiedBlk := &ethpb.SignedBeaconBlock{Block: &ethpb.BeaconBlock{ParentRoot: lastJustifiedRoot[:]}}
	newJustifiedRoot, _ := ssz.HashTreeRoot(newJustifiedBlk.Block)
	if err := service.beaconDB.SaveBlock(ctx, newJustifiedBlk); err != nil {
		t.Fatal(err)
	}
	if err := service.beaconDB.SaveBlock(ctx, lastJustifiedBlk); err != nil {
		t.Fatal(err)
	}

	diff := (params.BeaconConfig().SlotsPerEpoch - 1) * params.BeaconConfig().SecondsPerSlot
	service.genesisTime = time.Unix(time.Now().Unix()-int64(diff), 0)
	service.justifiedCheckpt = &ethpb.Checkpoint{Root: lastJustifiedRoot[:]}

	update, err := service.shouldUpdateCurrentJustified(ctx, &ethpb.Checkpoint{Root: newJustifiedRoot[:]})
	if err != nil {
		t.Fatal(err)
	}
	if update {
		t.Error("Should not be able to update justified, received true")
	}
}

func TestCachedPreState_CanGetFromCache(t *testing.T) {
	ctx := context.Background()
	db := testDB.SetupDB(t)
	defer testDB.TeardownDB(t, db)

	cfg := &Config{BeaconDB: db}
	service, err := NewService(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}

	s, _ := stateTrie.InitializeFromProto(&pb.BeaconState{Slot: 1})
	r := [32]byte{'A'}
	b := &ethpb.BeaconBlock{Slot: 1, ParentRoot: r[:]}
	service.initSyncState[r] = s

	wanted := "pre state of slot 1 does not exist"
	if _, err := service.verifyBlkPreState(ctx, b); !strings.Contains(err.Error(), wanted) {
		t.Fatal("Not expected error")
	}
}

func TestCachedPreState_CanGetFromCacheWithFeature(t *testing.T) {
	ctx := context.Background()
	db := testDB.SetupDB(t)
	defer testDB.TeardownDB(t, db)
	config := &featureconfig.Flags{
		InitSyncCacheState: true,
	}
	featureconfig.Init(config)

	cfg := &Config{BeaconDB: db}
	service, err := NewService(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}

	s, _ := stateTrie.InitializeFromProto(&pb.BeaconState{Slot: 1})
	r := [32]byte{'A'}
	b := &ethpb.BeaconBlock{Slot: 1, ParentRoot: r[:]}
	service.initSyncState[r] = s

	received, err := service.verifyBlkPreState(ctx, b)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(s.InnerStateUnsafe(), received.InnerStateUnsafe()) {
		t.Error("cached state not the same")
	}
}

func TestCachedPreState_CanGetFromDB(t *testing.T) {
	ctx := context.Background()
	db := testDB.SetupDB(t)
	defer testDB.TeardownDB(t, db)

	cfg := &Config{BeaconDB: db}
	service, err := NewService(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}

	r := [32]byte{'A'}
	b := &ethpb.BeaconBlock{Slot: 1, ParentRoot: r[:]}

	_, err = service.verifyBlkPreState(ctx, b)
	wanted := "pre state of slot 1 does not exist"
	if err.Error() != wanted {
		t.Error("Did not get wanted error")
	}

	s, _ := stateTrie.InitializeFromProto(&pb.BeaconState{Slot: 1})
	service.beaconDB.SaveState(ctx, s, r)

	received, err := service.verifyBlkPreState(ctx, b)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(s, received) {
		t.Error("cached state not the same")
	}
}

func TestSaveInitState_CanSaveDelete(t *testing.T) {
	ctx := context.Background()
	db := testDB.SetupDB(t)
	defer testDB.TeardownDB(t, db)

	cfg := &Config{BeaconDB: db}
	service, err := NewService(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}

	config := &featureconfig.Flags{
		InitSyncCacheState: true,
	}
	featureconfig.Init(config)

	for i := uint64(0); i < 64; i++ {
		b := &ethpb.BeaconBlock{Slot: i}
		s, _ := stateTrie.InitializeFromProto(&pb.BeaconState{Slot: i})
		r, _ := ssz.HashTreeRoot(b)
		service.initSyncState[r] = s
	}

	// Set finalized root as slot 32
	finalizedRoot, _ := ssz.HashTreeRoot(&ethpb.BeaconBlock{Slot: 32})

	s, _ := stateTrie.InitializeFromProto(&pb.BeaconState{FinalizedCheckpoint: &ethpb.Checkpoint{
		Epoch: 1, Root: finalizedRoot[:]}})
	if err := service.saveInitState(ctx, s); err != nil {
		t.Fatal(err)
	}

	// Verify finalized state is saved in DB
	finalizedState, err := service.beaconDB.State(ctx, finalizedRoot)
	if err != nil {
		t.Fatal(err)
	}
	if finalizedState == nil {
		t.Error("finalized state can't be nil")
	}

	// Verify cached state is properly pruned
	if len(service.initSyncState) != int(params.BeaconConfig().SlotsPerEpoch) {
		t.Errorf("wanted: %d, got: %d", len(service.initSyncState), params.BeaconConfig().SlotsPerEpoch)
	}
}

func TestUpdateJustified_CouldUpdateBest(t *testing.T) {
	ctx := context.Background()
	db := testDB.SetupDB(t)
	defer testDB.TeardownDB(t, db)

	cfg := &Config{BeaconDB: db}
	service, err := NewService(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}

	signedBlock := &ethpb.SignedBeaconBlock{Block: &ethpb.BeaconBlock{}}
	if err := db.SaveBlock(ctx, signedBlock); err != nil {
		t.Fatal(err)
	}
	r, err := ssz.HashTreeRoot(signedBlock.Block)
	if err != nil {
		t.Fatal(err)
	}
	service.justifiedCheckpt = &ethpb.Checkpoint{Root: []byte{'A'}}
	service.bestJustifiedCheckpt = &ethpb.Checkpoint{Root: []byte{'A'}}
	service.initSyncState[r] = &stateTrie.BeaconState{}
	if err := db.SaveState(ctx, &stateTrie.BeaconState{}, r); err != nil {
		t.Fatal(err)
	}

	// Could update
	s, _ := stateTrie.InitializeFromProto(&pb.BeaconState{CurrentJustifiedCheckpoint: &ethpb.Checkpoint{Epoch: 1, Root: r[:]}})
	if err := service.updateJustified(context.Background(), s); err != nil {
		t.Fatal(err)
	}

	if service.bestJustifiedCheckpt.Epoch != s.CurrentJustifiedCheckpoint().Epoch {
		t.Error("Incorrect justified epoch in service")
	}

	// Could not update
	service.bestJustifiedCheckpt.Epoch = 2
	if err := service.updateJustified(context.Background(), s); err != nil {
		t.Fatal(err)
	}

	if service.bestJustifiedCheckpt.Epoch != 2 {
		t.Error("Incorrect justified epoch in service")
	}
}

func TestFilterBlockRoots_CanFilter(t *testing.T) {
	ctx := context.Background()
	db := testDB.SetupDB(t)
	defer testDB.TeardownDB(t, db)

	cfg := &Config{BeaconDB: db}
	service, err := NewService(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}

	fBlock := &ethpb.BeaconBlock{}
	fRoot, _ := ssz.HashTreeRoot(fBlock)
	hBlock := &ethpb.BeaconBlock{Slot: 1}
	headRoot, _ := ssz.HashTreeRoot(hBlock)
	if err := service.beaconDB.SaveBlock(ctx, &ethpb.SignedBeaconBlock{Block: fBlock}); err != nil {
		t.Fatal(err)
	}
	if err := service.beaconDB.SaveState(ctx, &stateTrie.BeaconState{}, fRoot); err != nil {
		t.Fatal(err)
	}
	if err := service.beaconDB.SaveFinalizedCheckpoint(ctx, &ethpb.Checkpoint{Root: fRoot[:]}); err != nil {
		t.Fatal(err)
	}
	if err := service.beaconDB.SaveBlock(ctx, &ethpb.SignedBeaconBlock{Block: hBlock}); err != nil {
		t.Fatal(err)
	}
	if err := service.beaconDB.SaveState(ctx, &stateTrie.BeaconState{}, headRoot); err != nil {
		t.Fatal(err)
	}
	if err := service.beaconDB.SaveHeadBlockRoot(ctx, headRoot); err != nil {
		t.Fatal(err)
	}

	roots := [][32]byte{{'C'}, {'D'}, headRoot, {'E'}, fRoot, {'F'}}
	wanted := [][32]byte{{'C'}, {'D'}, {'E'}, {'F'}}

	received, err := service.filterBlockRoots(ctx, roots)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(wanted, received) {
		t.Error("Did not filter correctly")
	}
}

// blockTree1 constructs the following tree:
//    /- B1
// B0           /- B5 - B7
//    \- B3 - B4 - B6 - B8
// (B1, and B3 are all from the same slots)
func blockTree1(db db.Database, genesisRoot []byte) ([][]byte, error) {
	b0 := &ethpb.BeaconBlock{Slot: 0, ParentRoot: genesisRoot}
	r0, _ := ssz.HashTreeRoot(b0)
	b1 := &ethpb.BeaconBlock{Slot: 1, ParentRoot: r0[:]}
	r1, _ := ssz.HashTreeRoot(b1)
	b3 := &ethpb.BeaconBlock{Slot: 3, ParentRoot: r0[:]}
	r3, _ := ssz.HashTreeRoot(b3)
	b4 := &ethpb.BeaconBlock{Slot: 4, ParentRoot: r3[:]}
	r4, _ := ssz.HashTreeRoot(b4)
	b5 := &ethpb.BeaconBlock{Slot: 5, ParentRoot: r4[:]}
	r5, _ := ssz.HashTreeRoot(b5)
	b6 := &ethpb.BeaconBlock{Slot: 6, ParentRoot: r4[:]}
	r6, _ := ssz.HashTreeRoot(b6)
	b7 := &ethpb.BeaconBlock{Slot: 7, ParentRoot: r5[:]}
	r7, _ := ssz.HashTreeRoot(b7)
	b8 := &ethpb.BeaconBlock{Slot: 8, ParentRoot: r6[:]}
	r8, _ := ssz.HashTreeRoot(b8)
	for _, b := range []*ethpb.BeaconBlock{b0, b1, b3, b4, b5, b6, b7, b8} {
		if err := db.SaveBlock(context.Background(), &ethpb.SignedBeaconBlock{Block: b}); err != nil {
			return nil, err
		}
		if err := db.SaveState(context.Background(), &stateTrie.BeaconState{}, bytesutil.ToBytes32(b.ParentRoot)); err != nil {
			return nil, err
		}
	}
	if err := db.SaveState(context.Background(), &stateTrie.BeaconState{}, r1); err != nil {
		return nil, err
	}
	if err := db.SaveState(context.Background(), &stateTrie.BeaconState{}, r7); err != nil {
		return nil, err
	}
	if err := db.SaveState(context.Background(), &stateTrie.BeaconState{}, r8); err != nil {
		return nil, err
	}
	return [][]byte{r0[:], r1[:], nil, r3[:], r4[:], r5[:], r6[:], r7[:], r8[:]}, nil
}
