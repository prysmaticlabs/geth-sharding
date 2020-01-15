package forkchoice

import (
	"bytes"
	"context"
	"reflect"
	"strings"
	"testing"
	"time"

	ethpb "github.com/prysmaticlabs/ethereumapis/eth/v1alpha1"
	"github.com/prysmaticlabs/go-bitfield"
	"github.com/prysmaticlabs/go-ssz"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/blocks"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	testDB "github.com/prysmaticlabs/prysm/beacon-chain/db/testing"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/prysmaticlabs/prysm/shared/featureconfig"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/stateutil"
)

func TestStore_OnBlock(t *testing.T) {
	ctx := context.Background()
	db := testDB.SetupDB(t)
	defer testDB.TeardownDB(t, db)

	store := NewForkChoiceService(ctx, db)

	genesisStateRoot, err := stateutil.HashTreeRootState(&pb.BeaconState{})
	if err != nil {
		t.Error(err)
	}
	genesis := blocks.NewGenesisBlock(genesisStateRoot[:])
	if err := db.SaveBlock(ctx, genesis); err != nil {
		t.Error(err)
	}
	validGenesisRoot, err := ssz.HashTreeRoot(genesis.Block)
	if err != nil {
		t.Error(err)
	}
	if err := store.db.SaveState(ctx, &pb.BeaconState{}, validGenesisRoot); err != nil {
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
	if err := store.db.SaveState(ctx, &pb.BeaconState{}, randomParentRoot); err != nil {
		t.Fatal(err)
	}
	randomParentRoot2 := roots[1]
	if err := store.db.SaveState(ctx, &pb.BeaconState{}, bytesutil.ToBytes32(randomParentRoot2)); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name          string
		blk           *ethpb.BeaconBlock
		s             *pb.BeaconState
		time          uint64
		wantErrString string
	}{
		{
			name:          "parent block root does not have a state",
			blk:           &ethpb.BeaconBlock{},
			s:             &pb.BeaconState{},
			wantErrString: "pre state of slot 0 does not exist",
		},
		{
			name:          "block is from the feature",
			blk:           &ethpb.BeaconBlock{ParentRoot: randomParentRoot[:], Slot: params.BeaconConfig().FarFutureEpoch},
			s:             &pb.BeaconState{},
			wantErrString: "could not process slot from the future",
		},
		{
			name:          "could not get finalized block",
			blk:           &ethpb.BeaconBlock{ParentRoot: randomParentRoot[:]},
			s:             &pb.BeaconState{},
			wantErrString: "block from slot 0 is not a descendent of the current finalized block",
		},
		{
			name:          "same slot as finalized block",
			blk:           &ethpb.BeaconBlock{Slot: 0, ParentRoot: randomParentRoot2},
			s:             &pb.BeaconState{},
			wantErrString: "block is equal or earlier than finalized block, slot 0 < slot 0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := store.GenesisStore(ctx, &ethpb.Checkpoint{Root: validGenesisRoot[:]}, &ethpb.Checkpoint{Root: validGenesisRoot[:]}); err != nil {
				t.Fatal(err)
			}
			store.finalizedCheckpt.Root = roots[0]

			err := store.OnBlock(ctx, &ethpb.SignedBeaconBlock{Block: tt.blk})
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

	store := NewForkChoiceService(ctx, db)
	preCount := 2 // validators 0 and validators 1
	s := &pb.BeaconState{Validators: []*ethpb.Validator{
		{PublicKey: []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}},
		{PublicKey: []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}},
		{PublicKey: []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 2}},
		{PublicKey: []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 3}},
	}}
	if err := store.saveNewValidators(ctx, preCount, s); err != nil {
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

func TestStore_SavesNewBlockAttestations(t *testing.T) {
	ctx := context.Background()
	db := testDB.SetupDB(t)
	defer testDB.TeardownDB(t, db)

	store := NewForkChoiceService(ctx, db)
	a1 := &ethpb.Attestation{Data: &ethpb.AttestationData{}, AggregationBits: bitfield.Bitlist{0b101}}
	a2 := &ethpb.Attestation{Data: &ethpb.AttestationData{BeaconBlockRoot: []byte{'A'}}, AggregationBits: bitfield.Bitlist{0b110}}
	r1, _ := ssz.HashTreeRoot(a1.Data)
	r2, _ := ssz.HashTreeRoot(a2.Data)

	if err := store.saveNewBlockAttestations(ctx, []*ethpb.Attestation{a1, a2}); err != nil {
		t.Fatal(err)
	}

	saved, err := store.db.AttestationsByDataRoot(ctx, r1)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual([]*ethpb.Attestation{a1}, saved) {
		t.Error("did not retrieve saved attestation")
	}

	saved, err = store.db.AttestationsByDataRoot(ctx, r2)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual([]*ethpb.Attestation{a2}, saved) {
		t.Error("did not retrieve saved attestation")
	}

	a1 = &ethpb.Attestation{Data: &ethpb.AttestationData{}, AggregationBits: bitfield.Bitlist{0b111}}
	a2 = &ethpb.Attestation{Data: &ethpb.AttestationData{BeaconBlockRoot: []byte{'A'}}, AggregationBits: bitfield.Bitlist{0b111}}

	if err := store.saveNewBlockAttestations(ctx, []*ethpb.Attestation{a1, a2}); err != nil {
		t.Fatal(err)
	}

	saved, err = store.db.AttestationsByDataRoot(ctx, r1)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual([]*ethpb.Attestation{a1}, saved) {
		t.Error("did not retrieve saved attestation")
	}

	saved, err = store.db.AttestationsByDataRoot(ctx, r2)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual([]*ethpb.Attestation{a2}, saved) {
		t.Error("did not retrieve saved attestation")
	}
}

func TestRemoveStateSinceLastFinalized(t *testing.T) {
	ctx := context.Background()
	db := testDB.SetupDB(t)
	defer testDB.TeardownDB(t, db)
	params.UseMinimalConfig()
	defer params.UseMainnetConfig()

	store := NewForkChoiceService(ctx, db)

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
		if err := store.db.SaveState(ctx, &pb.BeaconState{Slot: uint64(i)}, r); err != nil {
			t.Fatal(err)
		}
		if err := store.db.SaveBlock(ctx, totalBlocks[i]); err != nil {
			t.Fatal(err)
		}
		blockRoots = append(blockRoots, r)
		if err := store.db.SaveHeadBlockRoot(ctx, r); err != nil {
			t.Fatal(err)
		}
	}

	// New finalized epoch: 1
	finalizedEpoch := uint64(1)
	finalizedSlot := finalizedEpoch * params.BeaconConfig().SlotsPerEpoch
	endSlot := helpers.StartSlot(finalizedEpoch+1) - 1 // Inclusive
	if err := store.rmStatesOlderThanLastFinalized(ctx, 0, endSlot); err != nil {
		t.Fatal(err)
	}
	for _, r := range blockRoots {
		s, err := store.db.State(ctx, r)
		if err != nil {
			t.Fatal(err)
		}
		// Also verifies genesis state didnt get deleted
		if s != nil && s.Slot != finalizedSlot && s.Slot != 0 && s.Slot < endSlot {
			t.Errorf("State with slot %d should not be in DB", s.Slot)
		}
	}

	// New finalized epoch: 5
	newFinalizedEpoch := uint64(5)
	newFinalizedSlot := newFinalizedEpoch * params.BeaconConfig().SlotsPerEpoch
	endSlot = helpers.StartSlot(newFinalizedEpoch+1) - 1 // Inclusive
	if err := store.rmStatesOlderThanLastFinalized(ctx, helpers.StartSlot(finalizedEpoch+1)-1, endSlot); err != nil {
		t.Fatal(err)
	}
	for _, r := range blockRoots {
		s, err := store.db.State(ctx, r)
		if err != nil {
			t.Fatal(err)
		}
		// Also verifies genesis state didnt get deleted
		if s != nil && s.Slot != newFinalizedSlot && s.Slot != finalizedSlot && s.Slot != 0 && s.Slot < endSlot {
			t.Errorf("State with slot %d should not be in DB", s.Slot)
		}
	}
}

func TestRemoveStateSinceLastFinalized_EmptyStartSlot(t *testing.T) {
	ctx := context.Background()
	db := testDB.SetupDB(t)
	defer testDB.TeardownDB(t, db)
	params.UseMinimalConfig()
	defer params.UseMainnetConfig()

	store := NewForkChoiceService(ctx, db)
	store.genesisTime = uint64(time.Now().Unix())

	update, err := store.shouldUpdateCurrentJustified(ctx, &ethpb.Checkpoint{})
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
	if err := store.db.SaveBlock(ctx, newJustifiedBlk); err != nil {
		t.Fatal(err)
	}
	if err := store.db.SaveBlock(ctx, lastJustifiedBlk); err != nil {
		t.Fatal(err)
	}

	diff := (params.BeaconConfig().SlotsPerEpoch - 1) * params.BeaconConfig().SecondsPerSlot
	store.genesisTime = uint64(time.Now().Unix()) - diff
	store.justifiedCheckpt = &ethpb.Checkpoint{Root: lastJustifiedRoot[:]}
	update, err = store.shouldUpdateCurrentJustified(ctx, &ethpb.Checkpoint{Root: newJustifiedRoot[:]})
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

	store := NewForkChoiceService(ctx, db)

	lastJustifiedBlk := &ethpb.SignedBeaconBlock{Block: &ethpb.BeaconBlock{ParentRoot: []byte{'G'}}}
	lastJustifiedRoot, _ := ssz.HashTreeRoot(lastJustifiedBlk.Block)
	newJustifiedBlk := &ethpb.SignedBeaconBlock{Block: &ethpb.BeaconBlock{ParentRoot: lastJustifiedRoot[:]}}
	newJustifiedRoot, _ := ssz.HashTreeRoot(newJustifiedBlk.Block)
	if err := store.db.SaveBlock(ctx, newJustifiedBlk); err != nil {
		t.Fatal(err)
	}
	if err := store.db.SaveBlock(ctx, lastJustifiedBlk); err != nil {
		t.Fatal(err)
	}

	diff := (params.BeaconConfig().SlotsPerEpoch - 1) * params.BeaconConfig().SecondsPerSlot
	store.genesisTime = uint64(time.Now().Unix()) - diff
	store.justifiedCheckpt = &ethpb.Checkpoint{Root: lastJustifiedRoot[:]}

	update, err := store.shouldUpdateCurrentJustified(ctx, &ethpb.Checkpoint{Root: newJustifiedRoot[:]})
	if err != nil {
		t.Fatal(err)
	}
	if update {
		t.Error("Should not be able to update justified, received true")
	}
}

func TestUpdateJustifiedCheckpoint_Update(t *testing.T) {
	ctx := context.Background()
	db := testDB.SetupDB(t)
	defer testDB.TeardownDB(t, db)
	params.UseMinimalConfig()
	defer params.UseMainnetConfig()

	store := NewForkChoiceService(ctx, db)
	store.genesisTime = uint64(time.Now().Unix())

	store.justifiedCheckpt = &ethpb.Checkpoint{Root: []byte{'A'}}
	store.bestJustifiedCheckpt = &ethpb.Checkpoint{Epoch: 1, Root: []byte{'B'}}
	store.updateJustifiedCheckpoint()

	if !bytes.Equal(store.justifiedCheckpt.Root, []byte{'B'}) {
		t.Error("Justified check point root did not update")
	}
}

func TestUpdateJustifiedCheckpoint_NoUpdate(t *testing.T) {
	ctx := context.Background()
	db := testDB.SetupDB(t)
	defer testDB.TeardownDB(t, db)
	params.UseMinimalConfig()
	defer params.UseMainnetConfig()

	store := NewForkChoiceService(ctx, db)
	store.genesisTime = uint64(time.Now().Unix()) - params.BeaconConfig().SecondsPerSlot

	store.justifiedCheckpt = &ethpb.Checkpoint{Root: []byte{'A'}}
	store.bestJustifiedCheckpt = &ethpb.Checkpoint{Epoch: 1, Root: []byte{'B'}}
	store.updateJustifiedCheckpoint()

	if bytes.Equal(store.justifiedCheckpt.Root, []byte{'B'}) {
		t.Error("Justified check point root was not suppose to update")

		store := NewForkChoiceService(ctx, db)

		// Save 5 blocks in DB, each has a state.
		numBlocks := 5
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
			if err := store.db.SaveState(ctx, &pb.BeaconState{Slot: uint64(i)}, r); err != nil {
				t.Fatal(err)
			}
			if err := store.db.SaveBlock(ctx, totalBlocks[i]); err != nil {
				t.Fatal(err)
			}
			blockRoots = append(blockRoots, r)
		}
		if err := store.db.SaveHeadBlockRoot(ctx, blockRoots[0]); err != nil {
			t.Fatal(err)
		}
		if err := store.rmStatesOlderThanLastFinalized(ctx, 10, 11); err != nil {
			t.Fatal(err)
		}
		// Since 5-10 are skip slots, block with slot 4 should be deleted
		s, err := store.db.State(ctx, blockRoots[4])
		if err != nil {
			t.Fatal(err)
		}
		if s != nil {
			t.Error("Did not delete state for start slot")
		}
	}
}

func TestCachedPreState_CanGetFromCache(t *testing.T) {
	ctx := context.Background()
	db := testDB.SetupDB(t)
	defer testDB.TeardownDB(t, db)

	store := NewForkChoiceService(ctx, db)
	s := &pb.BeaconState{Slot: 1}
	r := [32]byte{'A'}
	b := &ethpb.BeaconBlock{Slot: 1, ParentRoot: r[:]}
	store.initSyncState[r] = s

	wanted := "pre state of slot 1 does not exist"
	if _, err := store.cachedPreState(ctx, b); !strings.Contains(err.Error(), wanted) {
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

	store := NewForkChoiceService(ctx, db)
	s := &pb.BeaconState{Slot: 1}
	r := [32]byte{'A'}
	b := &ethpb.BeaconBlock{Slot: 1, ParentRoot: r[:]}
	store.initSyncState[r] = s

	received, err := store.cachedPreState(ctx, b)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(s, received) {
		t.Error("cached state not the same")
	}
}

func TestCachedPreState_CanGetFromDB(t *testing.T) {
	ctx := context.Background()
	db := testDB.SetupDB(t)
	defer testDB.TeardownDB(t, db)

	store := NewForkChoiceService(ctx, db)
	r := [32]byte{'A'}
	b := &ethpb.BeaconBlock{Slot: 1, ParentRoot: r[:]}

	_, err := store.cachedPreState(ctx, b)
	wanted := "pre state of slot 1 does not exist"
	if err.Error() != wanted {
		t.Error("Did not get wanted error")
	}

	s := &pb.BeaconState{Slot: 1}
	store.db.SaveState(ctx, s, r)

	received, err := store.cachedPreState(ctx, b)
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

	store := NewForkChoiceService(ctx, db)

	config := &featureconfig.Flags{
		InitSyncCacheState: true,
	}
	featureconfig.Init(config)

	for i := uint64(0); i < 64; i++ {
		b := &ethpb.BeaconBlock{Slot: i}
		s := &pb.BeaconState{Slot: i}
		r, _ := ssz.HashTreeRoot(b)
		store.initSyncState[r] = s
	}

	// Set finalized root as slot 32
	finalizedRoot, _ := ssz.HashTreeRoot(&ethpb.BeaconBlock{Slot: 32})

	if err := store.saveInitState(ctx, &pb.BeaconState{FinalizedCheckpoint: &ethpb.Checkpoint{
		Epoch: 1, Root: finalizedRoot[:]}}); err != nil {
		t.Fatal(err)
	}

	// Verify finalized state is saved in DB
	finalizedState, err := store.db.State(ctx, finalizedRoot)
	if err != nil {
		t.Fatal(err)
	}
	if finalizedState == nil {
		t.Error("finalized state can't be nil")
	}

	// Verify cached state is properly pruned
	if len(store.initSyncState) != int(params.BeaconConfig().SlotsPerEpoch) {
		t.Errorf("wanted: %d, got: %d", len(store.initSyncState), params.BeaconConfig().SlotsPerEpoch)
	}
}

func TestUpdateJustified_CouldUpdateBest(t *testing.T) {
	ctx := context.Background()
	db := testDB.SetupDB(t)
	defer testDB.TeardownDB(t, db)

	store := NewForkChoiceService(ctx, db)
	signedBlock := &ethpb.SignedBeaconBlock{Block: &ethpb.BeaconBlock{}}
	if err := db.SaveBlock(ctx, signedBlock); err != nil {
		t.Fatal(err)
	}
	r, err := ssz.HashTreeRoot(signedBlock.Block)
	if err != nil {
		t.Fatal(err)
	}
	store.justifiedCheckpt = &ethpb.Checkpoint{Root: []byte{'A'}}
	store.bestJustifiedCheckpt = &ethpb.Checkpoint{Root: []byte{'A'}}
	store.initSyncState[r] = &pb.BeaconState{}
	if err := db.SaveState(ctx, &pb.BeaconState{}, r); err != nil {
		t.Fatal(err)
	}

	// Could update
	s := &pb.BeaconState{CurrentJustifiedCheckpoint: &ethpb.Checkpoint{Epoch: 1, Root: r[:]}}
	if err := store.updateJustified(context.Background(), s); err != nil {
		t.Fatal(err)
	}

	if store.bestJustifiedCheckpt.Epoch != s.CurrentJustifiedCheckpoint.Epoch {
		t.Error("Incorrect justified epoch in store")
	}

	// Could not update
	store.bestJustifiedCheckpt.Epoch = 2
	if err := store.updateJustified(context.Background(), s); err != nil {
		t.Fatal(err)
	}

	if store.bestJustifiedCheckpt.Epoch != 2 {
		t.Error("Incorrect justified epoch in store")
	}
}

func TestFilterBlockRoots_CanFilter(t *testing.T) {
	ctx := context.Background()
	db := testDB.SetupDB(t)
	defer testDB.TeardownDB(t, db)

	store := NewForkChoiceService(ctx, db)
	fBlock := &ethpb.BeaconBlock{}
	fRoot, _ := ssz.HashTreeRoot(fBlock)
	hBlock := &ethpb.BeaconBlock{Slot: 1}
	headRoot, _ := ssz.HashTreeRoot(hBlock)
	if err := store.db.SaveBlock(ctx, &ethpb.SignedBeaconBlock{Block: fBlock}); err != nil {
		t.Fatal(err)
	}
	if err := store.db.SaveState(ctx, &pb.BeaconState{}, fRoot); err != nil {
		t.Fatal(err)
	}
	if err := store.db.SaveFinalizedCheckpoint(ctx, &ethpb.Checkpoint{Root: fRoot[:]}); err != nil {
		t.Fatal(err)
	}
	if err := store.db.SaveBlock(ctx, &ethpb.SignedBeaconBlock{Block: hBlock}); err != nil {
		t.Fatal(err)
	}
	if err := store.db.SaveState(ctx, &pb.BeaconState{}, headRoot); err != nil {
		t.Fatal(err)
	}
	if err := store.db.SaveHeadBlockRoot(ctx, headRoot); err != nil {
		t.Fatal(err)
	}

	roots := [][32]byte{{'C'}, {'D'}, headRoot, {'E'}, fRoot, {'F'}}
	wanted := [][32]byte{{'C'}, {'D'}, {'E'}, {'F'}}

	received, err := store.filterBlockRoots(ctx, roots)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(wanted, received) {
		t.Error("Did not filter correctly")
	}
}
