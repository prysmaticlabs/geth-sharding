package forkchoice

import (
	"bytes"
	"context"
	"reflect"
	"testing"
	"time"

	ethpb "github.com/prysmaticlabs/ethereumapis/eth/v1alpha1"
	"github.com/prysmaticlabs/go-ssz"
	"github.com/prysmaticlabs/prysm/beacon-chain/cache"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/blocks"
	"github.com/prysmaticlabs/prysm/beacon-chain/db/filters"
	testDB "github.com/prysmaticlabs/prysm/beacon-chain/db/testing"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/prysmaticlabs/prysm/shared/featureconfig"
)

func TestStore_GenesisStoreOk(t *testing.T) {
	ctx := context.Background()
	db := testDB.SetupDB(t)
	defer testDB.TeardownDB(t, db)

	store := NewForkChoiceService(ctx, db)

	genesisTime := time.Unix(9999, 0)
	genesisState := &pb.BeaconState{GenesisTime: uint64(genesisTime.Unix())}
	genesisStateRoot, err := ssz.HashTreeRoot(genesisState)
	if err != nil {
		t.Fatal(err)
	}
	genesisBlk := blocks.NewGenesisBlock(genesisStateRoot[:])
	genesisBlkRoot, err := ssz.SigningRoot(genesisBlk)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.SaveState(ctx, genesisState, genesisBlkRoot); err != nil {
		t.Fatal(err)
	}

	checkPoint := &ethpb.Checkpoint{Root: genesisBlkRoot[:]}
	if err := store.GenesisStore(ctx, checkPoint, checkPoint); err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(store.justifiedCheckpt, checkPoint) {
		t.Error("Justified check point from genesis store did not match")
	}
	if !reflect.DeepEqual(store.finalizedCheckpt, checkPoint) {
		t.Error("Finalized check point from genesis store did not match")
	}

	cachedState, err := store.checkpointState.StateByCheckpoint(checkPoint)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(cachedState, genesisState) {
		t.Error("Incorrect genesis state cached")
	}
}

func TestStore_AncestorOk(t *testing.T) {
	ctx := context.Background()
	db := testDB.SetupDB(t)
	defer testDB.TeardownDB(t, db)

	store := NewForkChoiceService(ctx, db)

	roots, err := blockTree1(db)
	if err != nil {
		t.Fatal(err)
	}
	type args struct {
		root []byte
		slot uint64
	}

	//    /- B1
	// B0           /- B5 - B7
	//    \- B3 - B4 - B6 - B8
	tests := []struct {
		args *args
		want []byte
	}{
		{args: &args{roots[1], 0}, want: roots[0]},
		{args: &args{roots[8], 0}, want: roots[0]},
		{args: &args{roots[8], 4}, want: roots[4]},
		{args: &args{roots[7], 4}, want: roots[4]},
		{args: &args{roots[7], 0}, want: roots[0]},
	}
	for _, tt := range tests {
		got, err := store.ancestor(ctx, tt.args.root, tt.args.slot)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("Store.ancestor(ctx, ) = %v, want %v", got, tt.want)
		}
	}
}

func TestStore_AncestorNotPartOfTheChain(t *testing.T) {
	ctx := context.Background()
	db := testDB.SetupDB(t)
	defer testDB.TeardownDB(t, db)

	store := NewForkChoiceService(ctx, db)

	roots, err := blockTree1(db)
	if err != nil {
		t.Fatal(err)
	}

	//    /- B1
	// B0           /- B5 - B7
	//    \- B3 - B4 - B6 - B8
	root, err := store.ancestor(ctx, roots[8], 1)
	if err != nil {
		t.Fatal(err)
	}
	if root != nil {
		t.Error("block at slot 1 is not part of the chain")
	}
	root, err = store.ancestor(ctx, roots[8], 2)
	if err != nil {
		t.Fatal(err)
	}
	if root != nil {
		t.Error("block at slot 2 is not part of the chain")
	}
}

func TestStore_LatestAttestingBalance(t *testing.T) {
	ctx := context.Background()
	db := testDB.SetupDB(t)
	defer testDB.TeardownDB(t, db)

	store := NewForkChoiceService(ctx, db)

	roots, err := blockTree1(db)
	if err != nil {
		t.Fatal(err)
	}

	validators := make([]*ethpb.Validator, 100)
	for i := 0; i < len(validators); i++ {
		validators[i] = &ethpb.Validator{ExitEpoch: 2, EffectiveBalance: 1e9}
	}

	s := &pb.BeaconState{Validators: validators}
	stateRoot, err := ssz.HashTreeRoot(s)
	if err != nil {
		t.Fatal(err)
	}
	b := blocks.NewGenesisBlock(stateRoot[:])
	blkRoot, err := ssz.SigningRoot(b)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.SaveState(ctx, s, blkRoot); err != nil {
		t.Fatal(err)
	}

	checkPoint := &ethpb.Checkpoint{Root: blkRoot[:]}
	if err := store.GenesisStore(ctx, checkPoint, checkPoint); err != nil {
		t.Fatal(err)
	}

	//    /- B1 (33 votes)
	// B0           /- B5 - B7 (33 votes)
	//    \- B3 - B4 - B6 - B8 (34 votes)
	for i := 0; i < len(validators); i++ {
		switch {
		case i < 33:
			store.latestVoteMap[uint64(i)] = &pb.ValidatorLatestVote{Root: roots[1]}
		case i > 66:
			store.latestVoteMap[uint64(i)] = &pb.ValidatorLatestVote{Root: roots[7]}
		default:
			store.latestVoteMap[uint64(i)] = &pb.ValidatorLatestVote{Root: roots[8]}
		}
	}

	tests := []struct {
		root []byte
		want uint64
	}{
		{root: roots[0], want: 100 * 1e9},
		{root: roots[1], want: 33 * 1e9},
		{root: roots[3], want: 67 * 1e9},
		{root: roots[4], want: 67 * 1e9},
		{root: roots[7], want: 33 * 1e9},
		{root: roots[8], want: 34 * 1e9},
	}
	for _, tt := range tests {
		got, err := store.latestAttestingBalance(ctx, tt.root)
		if err != nil {
			t.Fatal(err)
		}
		if got != tt.want {
			t.Errorf("Store.latestAttestingBalance(ctx, ) = %v, want %v", got, tt.want)
		}
	}
}

func TestStore_ChildrenBlocksFromParentRoot(t *testing.T) {
	ctx := context.Background()
	db := testDB.SetupDB(t)
	defer testDB.TeardownDB(t, db)

	store := NewForkChoiceService(ctx, db)

	roots, err := blockTree1(db)
	if err != nil {
		t.Fatal(err)
	}

	filter := filters.NewFilter().SetParentRoot(roots[0]).SetStartSlot(0)
	children, err := store.db.BlockRoots(ctx, filter)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(children, [][32]byte{bytesutil.ToBytes32(roots[1]), bytesutil.ToBytes32(roots[3])}) {
		t.Error("Did not receive correct children roots")
	}

	filter = filters.NewFilter().SetParentRoot(roots[0]).SetStartSlot(2)
	children, err = store.db.BlockRoots(ctx, filter)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(children, [][32]byte{bytesutil.ToBytes32(roots[3])}) {
		t.Error("Did not receive correct children roots")
	}
}

func TestStore_GetHead(t *testing.T) {
	ctx := context.Background()
	db := testDB.SetupDB(t)
	defer testDB.TeardownDB(t, db)

	store := NewForkChoiceService(ctx, db)

	roots, err := blockTree1(db)
	if err != nil {
		t.Fatal(err)
	}

	validators := make([]*ethpb.Validator, 100)
	for i := 0; i < len(validators); i++ {
		validators[i] = &ethpb.Validator{ExitEpoch: 2, EffectiveBalance: 1e9}
	}

	s := &pb.BeaconState{Validators: validators}
	stateRoot, err := ssz.HashTreeRoot(s)
	if err != nil {
		t.Fatal(err)
	}
	b := blocks.NewGenesisBlock(stateRoot[:])
	blkRoot, err := ssz.SigningRoot(b)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.db.SaveState(ctx, s, blkRoot); err != nil {
		t.Fatal(err)
	}

	checkPoint := &ethpb.Checkpoint{Root: blkRoot[:]}

	if err := store.GenesisStore(ctx, checkPoint, checkPoint); err != nil {
		t.Fatal(err)
	}
	if err := store.db.SaveState(ctx, s, bytesutil.ToBytes32(roots[0])); err != nil {
		t.Fatal(err)
	}
	store.justifiedCheckpt.Root = roots[0]
	if err := store.checkpointState.AddCheckpointState(&cache.CheckpointState{
		Checkpoint: store.justifiedCheckpt,
		State:      s,
	}); err != nil {
		t.Fatal(err)
	}

	//    /- B1 (33 votes)
	// B0           /- B5 - B7 (33 votes)
	//    \- B3 - B4 - B6 - B8 (34 votes)
	for i := 0; i < len(validators); i++ {
		switch {
		case i < 33:
			store.latestVoteMap[uint64(i)] = &pb.ValidatorLatestVote{Root: roots[1]}
		case i > 66:
			store.latestVoteMap[uint64(i)] = &pb.ValidatorLatestVote{Root: roots[7]}
		default:
			store.latestVoteMap[uint64(i)] = &pb.ValidatorLatestVote{Root: roots[8]}
		}
	}

	// Default head is B8
	head, err := store.Head(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(head, roots[8]) {
		t.Error("Incorrect head")
	}

	// 1 validator switches vote to B7 to gain 34%, enough to switch head
	store.latestVoteMap[uint64(50)] = &pb.ValidatorLatestVote{Root: roots[7]}

	head, err = store.Head(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(head, roots[7]) {
		t.Error("Incorrect head")
	}

	// 18 validators switches vote to B1 to gain 51%, enough to switch head
	for i := 0; i < 18; i++ {
		idx := 50 + uint64(i)
		store.latestVoteMap[uint64(idx)] = &pb.ValidatorLatestVote{Root: roots[1]}
	}
	head, err = store.Head(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(head, roots[1]) {
		t.Log(head)
		t.Error("Incorrect head")
	}
}

func TestCacheGenesisState_Correct(t *testing.T) {
	ctx := context.Background()
	db := testDB.SetupDB(t)
	defer testDB.TeardownDB(t, db)

	store := NewForkChoiceService(ctx, db)
	config := &featureconfig.Flags{
		InitSyncCacheState: true,
	}
	featureconfig.Init(config)

	b := &ethpb.BeaconBlock{Slot: 1}
	r, _ := ssz.SigningRoot(b)
	s := &pb.BeaconState{GenesisTime: 99}

	store.db.SaveState(ctx, s, r)
	store.db.SaveGenesisBlockRoot(ctx, r)

	if err := store.cacheGenesisState(ctx); err != nil {
		t.Fatal(err)
	}

	for _, state := range store.initSyncState {
		if !reflect.DeepEqual(s, state) {
			t.Error("Did not get wanted state")
		}
	}
}
