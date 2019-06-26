package blockchain

import (
	"context"
	"encoding/binary"
	"math/big"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"

	"github.com/prysmaticlabs/go-ssz"
	"github.com/prysmaticlabs/prysm/beacon-chain/attestation"
	b "github.com/prysmaticlabs/prysm/beacon-chain/core/blocks"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/state"
	v "github.com/prysmaticlabs/prysm/beacon-chain/core/validators"
	"github.com/prysmaticlabs/prysm/beacon-chain/internal"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	"github.com/prysmaticlabs/prysm/shared/blockutil"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/prysmaticlabs/prysm/shared/featureconfig"
	"github.com/prysmaticlabs/prysm/shared/hashutil"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/testutil"
	"github.com/prysmaticlabs/prysm/shared/trieutil"
	logTest "github.com/sirupsen/logrus/hooks/test"
)

// Ensure ChainService implements interfaces.
var _ = BlockProcessor(&ChainService{})

func initBlockStateRoot(t *testing.T, block *pb.BeaconBlock, chainService *ChainService) {
	parentRoot := bytesutil.ToBytes32(block.ParentRoot)
	parent, err := chainService.beaconDB.Block(parentRoot)
	if err != nil {
		t.Fatal(err)
	}
	beaconState, err := chainService.beaconDB.HistoricalStateFromSlot(context.Background(), parent.Slot, parentRoot)
	if err != nil {
		t.Fatalf("Unable to retrieve state %v", err)
	}

	computedState, err := chainService.AdvanceState(context.Background(), beaconState, block)
	if err != nil {
		t.Fatalf("could not apply block state transition: %v", err)
	}

	stateRoot, err := hashutil.HashProto(computedState)
	if err != nil {
		t.Fatalf("could not tree hash state: %v", err)
	}
	block.StateRoot = stateRoot[:]
	t.Logf("state root after block: %#x", stateRoot)
}

func TestReceiveBlock_FaultyPOWChain(t *testing.T) {
	db := internal.SetupDB(t)
	defer internal.TeardownDB(t, db)
	chainService := setupBeaconChain(t, db, nil)
	unixTime := uint64(time.Now().Unix())
	deposits, _ := testutil.GenerateDeposits(t, 100, false)
	if err := db.InitializeState(context.Background(), unixTime, deposits, nil); err != nil {
		t.Fatalf("Could not initialize beacon state to disk: %v", err)
	}

	if err := SetSlotInState(chainService, 1); err != nil {
		t.Fatal(err)
	}

	parentBlock := &pb.BeaconBlock{
		Slot: 1,
	}

	parentRoot, err := blockutil.BlockSigningRoot(parentBlock)
	if err != nil {
		t.Fatalf("Unable to tree hash block %v", err)
	}

	if err := chainService.beaconDB.SaveBlock(parentBlock); err != nil {
		t.Fatalf("Unable to save block %v", err)
	}

	block := &pb.BeaconBlock{
		Slot:       2,
		ParentRoot: parentRoot[:],
		Body: &pb.BeaconBlockBody{
			Eth1Data: &pb.Eth1Data{
				DepositRoot: []byte("a"),
				BlockHash:   []byte("b"),
			},
		},
	}

	if err := chainService.beaconDB.SaveBlock(block); err != nil {
		t.Fatal(err)
	}
	if _, err := chainService.ReceiveBlock(context.Background(), block); err == nil {
		t.Errorf("Expected receive block to fail, received nil: %v", err)
	}
}

func TestReceiveBlock_ProcessCorrectly(t *testing.T) {
	featureconfig.InitFeatureConfig(&featureconfig.FeatureFlagConfig{
		EnableCheckBlockStateRoot: false,
	})
	hook := logTest.NewGlobal()
	db := internal.SetupDB(t)
	defer internal.TeardownDB(t, db)
	ctx := context.Background()

	chainService := setupBeaconChain(t, db, nil)
	deposits, privKeys := testutil.GenerateDeposits(t, 100, true)
	beaconState, err := state.GenesisBeaconState(deposits, 0, nil)
	if err != nil {
		t.Fatalf("Can't generate genesis state: %v", err)
	}
	beaconState.LatestStateRoots = make([][]byte, params.BeaconConfig().SlotsPerHistoricalRoot)
	genesis := b.NewGenesisBlock([]byte{})
	bodyRoot, err := ssz.HashTreeRoot(genesis.Body)
	if err != nil {
		t.Fatal(err)
	}
	beaconState.LatestBlockHeader = &pb.BeaconBlockHeader{
		Slot:       genesis.Slot,
		ParentRoot: genesis.ParentRoot,
		BodyRoot:   bodyRoot[:],
	}
	if err := chainService.beaconDB.SaveBlock(genesis); err != nil {
		t.Fatalf("Could not save block to db: %v", err)
	}
	parentRoot, err := blockutil.BlockSigningRoot(genesis)
	if err != nil {
		t.Fatal(err)
	}

	if err := db.SaveHistoricalState(ctx, beaconState, parentRoot); err != nil {
		t.Fatal(err)
	}

	if err := chainService.beaconDB.UpdateChainHead(ctx, genesis, beaconState); err != nil {
		t.Fatal(err)
	}

	beaconState.Slot++
	epoch := helpers.CurrentEpoch(beaconState)
	randaoReveal, err := helpers.CreateRandaoReveal(beaconState, epoch, privKeys)
	if err != nil {
		t.Fatal(err)
	}

	block := &pb.BeaconBlock{
		Slot:       beaconState.Slot,
		ParentRoot: parentRoot[:],
		Body: &pb.BeaconBlockBody{
			Eth1Data: &pb.Eth1Data{
				DepositRoot: []byte("a"),
				BlockHash:   []byte("b"),
			},
			RandaoReveal: randaoReveal,
			Attestations: nil,
		},
	}

	if err := chainService.beaconDB.SaveJustifiedBlock(block); err != nil {
		t.Fatal(err)
	}
	if err := chainService.beaconDB.SaveFinalizedBlock(block); err != nil {
		t.Fatal(err)
	}
	if err := chainService.beaconDB.SaveBlock(block); err != nil {
		t.Fatal(err)
	}
	if _, err := chainService.ReceiveBlock(context.Background(), block); err != nil {
		t.Errorf("Block failed processing: %v", err)
	}
	testutil.AssertLogsContain(t, hook, "Finished processing beacon block")
}

func TestReceiveBlock_UsesParentBlockState(t *testing.T) {
	featureconfig.InitFeatureConfig(&featureconfig.FeatureFlagConfig{
		EnableCheckBlockStateRoot: false,
	})
	hook := logTest.NewGlobal()
	db := internal.SetupDB(t)
	defer internal.TeardownDB(t, db)
	ctx := context.Background()

	chainService := setupBeaconChain(t, db, nil)
	deposits, _ := testutil.GenerateDeposits(t, 100, false)
	beaconState, err := state.GenesisBeaconState(deposits, 0, nil)
	if err != nil {
		t.Fatalf("Can't generate genesis state: %v", err)
	}
	beaconState.LatestStateRoots = make([][]byte, params.BeaconConfig().SlotsPerHistoricalRoot)
	genesis := b.NewGenesisBlock([]byte{})
	bodyRoot, err := ssz.HashTreeRoot(genesis.Body)
	if err != nil {
		t.Fatal(err)
	}
	beaconState.LatestBlockHeader = &pb.BeaconBlockHeader{
		Slot:       genesis.Slot,
		ParentRoot: genesis.ParentRoot,
		BodyRoot:   bodyRoot[:],
	}

	parentHash, genesisBlock := setupGenesisBlock(t, chainService)
	if err := chainService.beaconDB.UpdateChainHead(ctx, genesisBlock, beaconState); err != nil {
		t.Fatal(err)
	}
	if err := chainService.beaconDB.SaveHistoricalState(ctx, beaconState, parentHash); err != nil {
		t.Fatal(err)
	}
	parentRoot, err := ssz.SigningRoot(beaconState.LatestBlockHeader)
	if err != nil {
		t.Fatal(err)
	}
	// We ensure the block uses the right state parent if its ancestor is not block.Slot-1.
	block := &pb.BeaconBlock{
		Slot:       beaconState.Slot + 4,
		StateRoot:  []byte{},
		ParentRoot: parentRoot[:],
		Body: &pb.BeaconBlockBody{
			Eth1Data: &pb.Eth1Data{
				DepositRoot: []byte("a"),
				BlockHash:   []byte("b"),
			},
			RandaoReveal: []byte{},
			Attestations: nil,
		},
	}
	if err := chainService.beaconDB.SaveBlock(block); err != nil {
		t.Fatal(err)
	}
	if _, err := chainService.ReceiveBlock(context.Background(), block); err != nil {
		t.Errorf("Block failed processing: %v", err)
	}
	testutil.AssertLogsContain(t, hook, "Finished processing beacon block")
}

func TestReceiveBlock_DeletesBadBlock(t *testing.T) {
	featureconfig.InitFeatureConfig(&featureconfig.FeatureFlagConfig{
		EnableCheckBlockStateRoot: false,
	})
	db := internal.SetupDB(t)
	defer internal.TeardownDB(t, db)
	ctx := context.Background()

	attsService := attestation.NewAttestationService(
		context.Background(),
		&attestation.Config{BeaconDB: db})
	chainService := setupBeaconChain(t, db, attsService)
	deposits, _ := testutil.GenerateDeposits(t, 100, false)
	beaconState, err := state.GenesisBeaconState(deposits, 0, nil)
	if err != nil {
		t.Fatalf("Can't generate genesis state: %v", err)
	}
	beaconState.LatestStateRoots = make([][]byte, params.BeaconConfig().SlotsPerHistoricalRoot)
	genesis := b.NewGenesisBlock([]byte{})
	bodyRoot, err := ssz.HashTreeRoot(genesis.Body)
	if err != nil {
		t.Fatal(err)
	}
	beaconState.LatestBlockHeader = &pb.BeaconBlockHeader{
		Slot:       genesis.Slot,
		ParentRoot: genesis.ParentRoot,
		BodyRoot:   bodyRoot[:],
	}

	parentHash, genesisBlock := setupGenesisBlock(t, chainService)
	if err := chainService.beaconDB.UpdateChainHead(ctx, genesisBlock, beaconState); err != nil {
		t.Fatal(err)
	}
	if err := chainService.beaconDB.SaveHistoricalState(ctx, beaconState, parentHash); err != nil {
		t.Fatal(err)
	}

	beaconState.Slot++

	parentRoot, err := ssz.SigningRoot(beaconState.LatestBlockHeader)
	if err != nil {
		t.Fatal(err)
	}
	block := &pb.BeaconBlock{
		Slot:       beaconState.Slot,
		StateRoot:  []byte{},
		ParentRoot: parentRoot[:],
		Body: &pb.BeaconBlockBody{
			Eth1Data: &pb.Eth1Data{
				DepositRoot: []byte("a"),
				BlockHash:   []byte("b"),
			},
			RandaoReveal: []byte{},
			Attestations: []*pb.Attestation{{
				Data: &pb.AttestationData{
					TargetEpoch: 5,
				},
			}},
		},
	}

	blockRoot, err := blockutil.BlockSigningRoot(block)
	if err != nil {
		t.Fatal(err)
	}

	_, err = chainService.ReceiveBlock(context.Background(), block)
	switch err.(type) {
	case *BlockFailedProcessingErr:
		t.Log("Block failed processing as expected")
	default:
		t.Errorf("Expected block processing to fail, received: %v", err)
	}

	savedBlock, err := db.Block(blockRoot)
	if err != nil {
		t.Fatal(err)
	}
	if savedBlock != nil {
		t.Errorf("Expected bad block to have been deleted, received: %v", savedBlock)
	}
	// We also verify the block has been blacklisted.
	if !db.IsEvilBlockHash(blockRoot) {
		t.Error("Expected block root to have been blacklisted")
	}
	featureconfig.InitFeatureConfig(&featureconfig.FeatureFlagConfig{
		EnableCheckBlockStateRoot: true,
	})
}

func TestReceiveBlock_CheckBlockStateRoot_GoodState(t *testing.T) {
	hook := logTest.NewGlobal()
	db := internal.SetupDB(t)
	defer internal.TeardownDB(t, db)
	ctx := context.Background()

	attsService := attestation.NewAttestationService(
		context.Background(),
		&attestation.Config{BeaconDB: db})
	chainService := setupBeaconChain(t, db, attsService)
	deposits, privKeys := testutil.GenerateDeposits(t, 100, true)
	beaconState, err := state.GenesisBeaconState(deposits, 0, nil)
	if err != nil {
		t.Fatalf("Can't generate genesis state: %v", err)
	}
	genesis := b.NewGenesisBlock([]byte{})
	bodyRoot, err := ssz.HashTreeRoot(genesis.Body)
	if err != nil {
		t.Fatal(err)
	}
	beaconState.LatestStateRoots = make([][]byte, params.BeaconConfig().SlotsPerHistoricalRoot)
	beaconState.LatestBlockHeader = &pb.BeaconBlockHeader{
		Slot:       genesis.Slot,
		ParentRoot: genesis.ParentRoot,
		BodyRoot:   bodyRoot[:],
	}
	parentHash, genesisBlock := setupGenesisBlock(t, chainService)
	if err := chainService.beaconDB.SaveHistoricalState(ctx, beaconState, parentHash); err != nil {
		t.Fatal(err)
	}
	beaconState.Slot++
	if err := chainService.beaconDB.UpdateChainHead(ctx, genesisBlock, beaconState); err != nil {
		t.Fatal(err)
	}

	beaconState.Slot++
	parentRoot, err := blockutil.BlockSigningRoot(genesis)
	if err != nil {
		t.Fatal(err)
	}
	epoch := helpers.CurrentEpoch(beaconState)
	randaoReveal, err := helpers.CreateRandaoReveal(beaconState, epoch, privKeys)
	if err != nil {
		t.Fatal(err)
	}

	goodStateBlock := &pb.BeaconBlock{
		Slot:       beaconState.Slot,
		ParentRoot: parentRoot[:],
		Body: &pb.BeaconBlockBody{
			Eth1Data:     &pb.Eth1Data{},
			RandaoReveal: randaoReveal,
		},
	}
	beaconState.Slot--
	initBlockStateRoot(t, goodStateBlock, chainService)

	if err := chainService.beaconDB.SaveBlock(goodStateBlock); err != nil {
		t.Fatal(err)
	}

	_, err = chainService.ReceiveBlock(context.Background(), goodStateBlock)
	if err != nil {
		t.Fatalf("error exists for good block %v", err)
	}
	testutil.AssertLogsContain(t, hook, "Executing state transition")
}

func TestReceiveBlock_CheckBlockStateRoot_BadState(t *testing.T) {
	db := internal.SetupDB(t)
	defer internal.TeardownDB(t, db)
	ctx := context.Background()
	chainService := setupBeaconChain(t, db, nil)
	deposits, privKeys := testutil.GenerateDeposits(t, 100, true)
	beaconState, err := state.GenesisBeaconState(deposits, 0, nil)
	if err != nil {
		t.Fatalf("Can't generate genesis state: %v", err)
	}
	genesis := b.NewGenesisBlock([]byte{})
	bodyRoot, err := ssz.HashTreeRoot(genesis.Body)
	if err != nil {
		t.Fatal(err)
	}
	beaconState.LatestStateRoots = make([][]byte, params.BeaconConfig().SlotsPerHistoricalRoot)
	beaconState.LatestBlockHeader = &pb.BeaconBlockHeader{
		Slot:       genesis.Slot,
		ParentRoot: genesis.ParentRoot,
		BodyRoot:   bodyRoot[:],
	}
	parentHash, genesisBlock := setupGenesisBlock(t, chainService)
	if err := chainService.beaconDB.SaveHistoricalState(ctx, beaconState, parentHash); err != nil {
		t.Fatal(err)
	}
	beaconState.Slot++
	if err := chainService.beaconDB.UpdateChainHead(ctx, genesisBlock, beaconState); err != nil {
		t.Fatal(err)
	}

	beaconState.Slot++
	parentRoot, err := blockutil.BlockSigningRoot(genesis)
	if err != nil {
		t.Fatal(err)
	}
	epoch := helpers.CurrentEpoch(beaconState)
	randaoReveal, err := helpers.CreateRandaoReveal(beaconState, epoch, privKeys)
	if err != nil {
		t.Fatal(err)
	}

	invalidStateBlock := &pb.BeaconBlock{
		Slot:       beaconState.Slot,
		StateRoot:  []byte{'b', 'a', 'd', ' ', 'h', 'a', 's', 'h'},
		ParentRoot: parentRoot[:],
		Body: &pb.BeaconBlockBody{
			Eth1Data:     &pb.Eth1Data{},
			RandaoReveal: randaoReveal,
		},
	}
	beaconState.Slot--

	_, err = chainService.ReceiveBlock(context.Background(), invalidStateBlock)
	if err == nil {
		t.Fatal("no error for wrong block state root")
	}
	if !strings.Contains(err.Error(), "beacon state root is not equal to block state root: ") {
		t.Fatal(err)
	}
}

func TestReceiveBlock_RemovesPendingDeposits(t *testing.T) {
	hook := logTest.NewGlobal()
	db := internal.SetupDB(t)
	defer internal.TeardownDB(t, db)
	ctx := context.Background()

	attsService := attestation.NewAttestationService(
		context.Background(),
		&attestation.Config{BeaconDB: db})
	chainService := setupBeaconChain(t, db, attsService)
	deposits, privKeys := testutil.GenerateDeposits(t, 100, true)
	beaconState, err := state.GenesisBeaconState(deposits, 0, nil)
	if err != nil {
		t.Fatalf("Can't generate genesis state: %v", err)
	}
	genesis := b.NewGenesisBlock([]byte{})
	bodyRoot, err := ssz.HashTreeRoot(genesis.Body)
	if err != nil {
		t.Fatal(err)
	}
	beaconState.LatestStateRoots = make([][]byte, params.BeaconConfig().SlotsPerHistoricalRoot)
	beaconState.LatestBlockHeader = &pb.BeaconBlockHeader{
		Slot:       genesis.Slot,
		ParentRoot: genesis.ParentRoot,
		BodyRoot:   bodyRoot[:],
	}
	if err := chainService.beaconDB.SaveJustifiedState(beaconState); err != nil {
		t.Fatal(err)
	}
	if err := db.SaveFinalizedState(beaconState); err != nil {
		t.Fatal(err)
	}

	stateRoot, err := hashutil.HashProto(beaconState)
	if err != nil {
		t.Fatalf("Could not tree hash state: %v", err)
	}
	parentHash, genesisBlock := setupGenesisBlock(t, chainService)
	beaconState.Slot++
	if err := chainService.beaconDB.UpdateChainHead(ctx, genesisBlock, beaconState); err != nil {
		t.Fatal(err)
	}

	currentSlot := uint64(0)

	epoch := helpers.CurrentEpoch(beaconState)
	randaoReveal, err := helpers.CreateRandaoReveal(beaconState, epoch, privKeys)
	if err != nil {
		t.Fatal(err)
	}

	pendingDeposits := []*pb.Deposit{
		createPreChainStartDeposit([]byte{'F'}),
	}
	pendingDepositsData := make([][]byte, len(pendingDeposits))
	for i, pd := range pendingDeposits {
		h, err := ssz.HashTreeRoot(pd.Data)
		if err != nil {
			t.Fatal(err)
		}
		pendingDepositsData[i] = h[:]
	}
	depositTrie, err := trieutil.GenerateTrieFromItems(pendingDepositsData, int(params.BeaconConfig().DepositContractTreeDepth))
	if err != nil {
		t.Fatalf("Could not generate deposit trie: %v", err)
	}
	for i := range pendingDeposits {
		proof, err := depositTrie.MerkleProof(0)
		if err != nil {
			t.Fatalf("Could not generate proof: %v", err)
		}
		pendingDeposits[i].Proof = proof
	}
	depositRoot := depositTrie.Root()
	beaconState.LatestEth1Data.DepositRoot = depositRoot[:]
	if err := db.SaveHistoricalState(context.Background(), beaconState, parentHash); err != nil {
		t.Fatal(err)
	}

	parentRoot, err := blockutil.BlockSigningRoot(genesis)
	if err != nil {
		t.Fatal(err)
	}
	block := &pb.BeaconBlock{
		Slot:       currentSlot + 1,
		StateRoot:  stateRoot[:],
		ParentRoot: parentRoot[:],
		Body: &pb.BeaconBlockBody{
			Eth1Data: &pb.Eth1Data{
				DepositRoot: []byte("a"),
				BlockHash:   []byte("b"),
			},
			RandaoReveal: randaoReveal,
			Deposits:     pendingDeposits,
		},
	}

	beaconState.Slot--
	beaconState.DepositIndex = 0
	if err := chainService.beaconDB.SaveState(ctx, beaconState); err != nil {
		t.Fatal(err)
	}
	initBlockStateRoot(t, block, chainService)

	blockRoot, err := blockutil.BlockSigningRoot(block)
	if err != nil {
		log.Fatalf("could not hash block: %v", err)
	}

	if err := chainService.beaconDB.SaveJustifiedBlock(block); err != nil {
		t.Fatal(err)
	}
	if err := chainService.beaconDB.SaveFinalizedBlock(block); err != nil {
		t.Fatal(err)
	}

	for _, dep := range pendingDeposits {
		db.InsertPendingDeposit(chainService.ctx, dep, big.NewInt(0), 0, [32]byte{})
	}

	if len(db.PendingDeposits(chainService.ctx, nil)) != len(pendingDeposits) || len(pendingDeposits) == 0 {
		t.Fatalf("Expected %d pending deposits", len(pendingDeposits))
	}

	beaconState.Slot--
	if err := chainService.beaconDB.SaveState(ctx, beaconState); err != nil {
		t.Fatal(err)
	}
	if err := db.SaveHistoricalState(context.Background(), beaconState, blockRoot); err != nil {
		t.Fatal(err)
	}
	computedState, err := chainService.ReceiveBlock(context.Background(), block)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < len(beaconState.ValidatorRegistry); i++ {
		pubKey := bytesutil.ToBytes48(beaconState.ValidatorRegistry[i].Pubkey)
		attsService.InsertAttestationIntoStore(pubKey, &pb.Attestation{
			Data: &pb.AttestationData{
				BeaconBlockRoot: blockRoot[:],
			}},
		)
	}
	if err := chainService.ApplyForkChoiceRule(context.Background(), block, computedState); err != nil {
		t.Fatal(err)
	}

	if len(db.PendingDeposits(chainService.ctx, nil)) != 0 {
		t.Fatalf("Expected 0 pending deposits, but there are %+v", db.PendingDeposits(chainService.ctx, nil))
	}
	testutil.AssertLogsContain(t, hook, "Executing state transition")
}

// Scenario graph: http://bit.ly/2K1k2KZ
//
//digraph G {
//    rankdir=LR;
//    node [shape="none"];
//
//    subgraph blocks {
//        rankdir=LR;
//        node [shape="box"];
//        a->b;
//        b->c;
//        c->e;
//        c->f;
//        f->g;
//        e->h;
//    }
//
//    { rank=same; 1; a;}
//    { rank=same; 2; b;}
//    { rank=same; 3; c;}
//    { rank=same; 5; e;}
//    { rank=same; 6; f;}
//    { rank=same; 7; g;}
//    { rank=same; 8; h;}
//
//    1->2->3->4->5->6->7->8->9[arrowhead=none];
//}
func TestReceiveBlock_OnChainSplit(t *testing.T) {
	// The scenario to test is that we think that the canonical head is block H
	// and then we receive block G. We don't have block F, so we request it. Then
	// we process F, the G. The expected behavior is that we load the historical
	// state from slot 3 where the common ancestor block C is present.

	db := internal.SetupDB(t)
	defer internal.TeardownDB(t, db)
	ctx := context.Background()

	chainService := setupBeaconChain(t, db, nil)
	deposits, privKeys := testutil.GenerateDeposits(t, 100, true)
	beaconState, err := state.GenesisBeaconState(deposits, 0, nil)
	if err != nil {
		t.Fatalf("Can't generate genesis state: %v", err)
	}
	genesis := b.NewGenesisBlock([]byte{})
	bodyRoot, err := ssz.HashTreeRoot(genesis.Body)
	if err != nil {
		t.Fatal(err)
	}
	beaconState.LatestStateRoots = make([][]byte, params.BeaconConfig().SlotsPerHistoricalRoot)
	beaconState.LatestBlockHeader = &pb.BeaconBlockHeader{
		Slot:       genesis.Slot,
		ParentRoot: genesis.ParentRoot,
		BodyRoot:   bodyRoot[:],
	}
	stateRoot, err := hashutil.HashProto(beaconState)
	if err != nil {
		t.Fatalf("Could not tree hash state: %v", err)
	}
	_, genesisBlock := setupGenesisBlock(t, chainService)
	if err := db.UpdateChainHead(ctx, genesisBlock, beaconState); err != nil {
		t.Fatal(err)
	}
	if err := db.SaveFinalizedState(beaconState); err != nil {
		t.Fatal(err)
	}
	genesisSlot := uint64(0)

	parentRoot, err := blockutil.BlockSigningRoot(genesisBlock)
	if err != nil {
		t.Fatal(err)
	}
	epoch := helpers.CurrentEpoch(beaconState)
	randaoReveal, err := helpers.CreateRandaoReveal(beaconState, epoch, privKeys)
	if err != nil {
		t.Fatal(err)
	}

	// Top chain slots (see graph)
	blockSlots := []uint64{1, 2, 3, 5, 8}
	for _, slot := range blockSlots {
		block := &pb.BeaconBlock{
			Slot:       genesisSlot + slot,
			StateRoot:  stateRoot[:],
			ParentRoot: parentRoot[:],
			Body: &pb.BeaconBlockBody{
				Eth1Data:     &pb.Eth1Data{},
				RandaoReveal: randaoReveal,
			},
		}
		initBlockStateRoot(t, block, chainService)
		computedState, err := chainService.ReceiveBlock(ctx, block)
		if err != nil {
			t.Fatal(err)
		}
		stateRoot, err = hashutil.HashProto(computedState)
		if err != nil {
			t.Fatal(err)
		}
		if err = db.SaveBlock(block); err != nil {
			t.Fatal(err)
		}
		if err = db.UpdateChainHead(ctx, block, computedState); err != nil {
			t.Fatal(err)
		}
		parentRoot, err = blockutil.BlockSigningRoot(block)
		if err != nil {
			t.Fatal(err)
		}
	}

	// Common ancestor is block at slot 3
	commonAncestor, err := db.CanonicalBlockBySlot(ctx, genesisSlot+3)
	if err != nil {
		t.Fatal(err)
	}

	parentRoot, err = blockutil.BlockSigningRoot(commonAncestor)
	if err != nil {
		t.Fatal(err)
	}

	beaconState, err = db.HistoricalStateFromSlot(ctx, commonAncestor.Slot, parentRoot)
	if err != nil {
		t.Fatal(err)
	}
	stateRoot, err = hashutil.HashProto(beaconState)
	if err != nil {
		t.Fatal(err)
	}

	epoch = helpers.CurrentEpoch(beaconState)
	randaoReveal, err = helpers.CreateRandaoReveal(beaconState, epoch, privKeys)
	if err != nil {
		t.Fatal(err)
	}

	// Then we receive the block `f` from slot 6
	blockF := &pb.BeaconBlock{
		Slot:       genesisSlot + 6,
		ParentRoot: parentRoot[:],
		StateRoot:  stateRoot[:],
		Body: &pb.BeaconBlockBody{
			Eth1Data:     &pb.Eth1Data{},
			RandaoReveal: randaoReveal,
		},
	}
	rootF, _ := blockutil.BlockSigningRoot(blockF)
	if err := db.SaveHistoricalState(ctx, beaconState, rootF); err != nil {
		t.Fatal(err)
	}

	initBlockStateRoot(t, blockF, chainService)
	computedState, err := chainService.ReceiveBlock(ctx, blockF)
	if err != nil {
		t.Fatal(err)
	}

	stateRoot, err = hashutil.HashProto(computedState)
	if err != nil {
		t.Fatal(err)
	}

	if err := db.SaveBlock(blockF); err != nil {
		t.Fatal(err)
	}

	parentRoot, err = blockutil.BlockSigningRoot(blockF)
	if err != nil {
		t.Fatal(err)
	}

	epoch = helpers.CurrentEpoch(beaconState)
	randaoReveal, err = helpers.CreateRandaoReveal(beaconState, epoch, privKeys)
	if err != nil {
		t.Fatal(err)
	}

	// Then we apply block `g` from slot 7
	blockG := &pb.BeaconBlock{
		Slot:       genesisSlot + 7,
		ParentRoot: parentRoot[:],
		StateRoot:  stateRoot[:],
		Body: &pb.BeaconBlockBody{
			Eth1Data:     &pb.Eth1Data{},
			RandaoReveal: randaoReveal,
		},
	}
	initBlockStateRoot(t, blockG, chainService)

	computedState, err = chainService.ReceiveBlock(ctx, blockG)
	if err != nil {
		t.Fatal(err)
	}

	if computedState.Slot != blockG.Slot {
		t.Errorf("Unexpect state slot %d, wanted %d", computedState.Slot, blockG.Slot)
	}
}

func TestIsBlockReadyForProcessing_ValidBlock(t *testing.T) {
	db := internal.SetupDB(t)
	defer internal.TeardownDB(t, db)
	ctx := context.Background()

	chainService := setupBeaconChain(t, db, nil)
	unixTime := uint64(time.Now().Unix())
	deposits, privKeys := testutil.GenerateDeposits(t, 100, true)
	if err := db.InitializeState(context.Background(), unixTime, deposits, nil); err != nil {
		t.Fatalf("Could not initialize beacon state to disk: %v", err)
	}
	beaconState, err := db.HeadState(ctx)
	if err != nil {
		t.Fatalf("Can't get genesis state: %v", err)
	}
	genesis := b.NewGenesisBlock([]byte{})
	bodyRoot, err := ssz.HashTreeRoot(genesis.Body)
	if err != nil {
		t.Fatal(err)
	}
	beaconState.LatestStateRoots = make([][]byte, params.BeaconConfig().SlotsPerHistoricalRoot)
	beaconState.LatestBlockHeader = &pb.BeaconBlockHeader{
		Slot:       genesis.Slot,
		ParentRoot: genesis.ParentRoot,
		BodyRoot:   bodyRoot[:],
	}
	block := &pb.BeaconBlock{
		ParentRoot: []byte{'a'},
	}

	if err := chainService.VerifyBlockValidity(ctx, block, beaconState); err == nil {
		t.Fatal("block processing succeeded despite block having no parent saved")
	}

	beaconState.Slot = 10

	stateRoot, err := hashutil.HashProto(beaconState)
	if err != nil {
		t.Fatalf("Could not tree hash state: %v", err)
	}
	if err := chainService.beaconDB.SaveBlock(genesis); err != nil {
		t.Fatalf("cannot save block: %v", err)
	}
	parentRoot, err := blockutil.BlockSigningRoot(genesis)
	if err != nil {
		t.Fatalf("unable to get root of canonical head: %v", err)
	}

	beaconState.LatestEth1Data = &pb.Eth1Data{
		DepositRoot: []byte{2},
		BlockHash:   []byte{3},
	}
	beaconState.Slot = 0

	currentSlot := uint64(1)

	epoch := helpers.CurrentEpoch(beaconState)
	randaoReveal, err := helpers.CreateRandaoReveal(beaconState, epoch, privKeys)
	if err != nil {
		t.Fatal(err)
	}

	block2 := &pb.BeaconBlock{
		Slot:       currentSlot,
		StateRoot:  stateRoot[:],
		ParentRoot: parentRoot[:],
		Body: &pb.BeaconBlockBody{
			Eth1Data: &pb.Eth1Data{
				DepositRoot: []byte("a"),
				BlockHash:   []byte("b"),
			},
			RandaoReveal: randaoReveal,
			Attestations: []*pb.Attestation{{
				AggregationBitfield: []byte{128, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
					0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
				Data: &pb.AttestationData{
					SourceRoot: parentRoot[:],
					Crosslink: &pb.Crosslink{
						Shard: 960,
					},
				},
			}},
		},
	}

	if err := chainService.VerifyBlockValidity(ctx, block2, beaconState); err != nil {
		t.Fatalf("block processing failed despite being a valid block: %v", err)
	}
}

func TestDeleteValidatorIdx_DeleteWorks(t *testing.T) {
	db := internal.SetupDB(t)
	defer internal.TeardownDB(t, db)
	epoch := uint64(2)
	v.InsertActivatedIndices(epoch+1, []uint64{0, 1, 2})
	v.InsertExitedVal(epoch+1, []uint64{0, 2})
	var validators []*pb.Validator
	for i := 0; i < 3; i++ {
		pubKeyBuf := make([]byte, params.BeaconConfig().BLSPubkeyLength)
		binary.PutUvarint(pubKeyBuf, uint64(i))
		validators = append(validators, &pb.Validator{
			Pubkey: pubKeyBuf,
		})
	}
	state := &pb.BeaconState{
		ValidatorRegistry: validators,
		Slot:              epoch * params.BeaconConfig().SlotsPerEpoch,
	}
	chainService := setupBeaconChain(t, db, nil)
	if err := chainService.saveValidatorIdx(state); err != nil {
		t.Fatalf("Could not save validator idx: %v", err)
	}
	if err := chainService.deleteValidatorIdx(state); err != nil {
		t.Fatalf("Could not delete validator idx: %v", err)
	}
	wantedIdx := uint64(1)
	idx, err := chainService.beaconDB.ValidatorIndex(validators[wantedIdx].Pubkey)
	if err != nil {
		t.Fatalf("Could not get validator index: %v", err)
	}
	if wantedIdx != idx {
		t.Errorf("Wanted: %d, got: %d", wantedIdx, idx)
	}

	wantedIdx = uint64(2)
	if chainService.beaconDB.HasValidator(validators[wantedIdx].Pubkey) {
		t.Errorf("Validator index %d should have been deleted", wantedIdx)
	}
	if v.ExitedValFromEpoch(epoch) != nil {
		t.Errorf("Activated validators mapping for epoch %d still there", epoch)
	}
}

func TestSaveValidatorIdx_SaveRetrieveWorks(t *testing.T) {
	db := internal.SetupDB(t)
	defer internal.TeardownDB(t, db)
	epoch := uint64(1)
	v.InsertActivatedIndices(epoch+1, []uint64{0, 1, 2})
	var validators []*pb.Validator
	for i := 0; i < 3; i++ {
		pubKeyBuf := make([]byte, params.BeaconConfig().BLSPubkeyLength)
		binary.PutUvarint(pubKeyBuf, uint64(i))
		validators = append(validators, &pb.Validator{
			Pubkey: pubKeyBuf,
		})
	}
	state := &pb.BeaconState{
		ValidatorRegistry: validators,
		Slot:              epoch * params.BeaconConfig().SlotsPerEpoch,
	}
	chainService := setupBeaconChain(t, db, nil)
	if err := chainService.saveValidatorIdx(state); err != nil {
		t.Fatalf("Could not save validator idx: %v", err)
	}

	wantedIdx := uint64(2)
	idx, err := chainService.beaconDB.ValidatorIndex(validators[wantedIdx].Pubkey)
	if err != nil {
		t.Fatalf("Could not get validator index: %v", err)
	}
	if wantedIdx != idx {
		t.Errorf("Wanted: %d, got: %d", wantedIdx, idx)
	}

	if v.ActivatedValFromEpoch(epoch) != nil {
		t.Errorf("Activated validators mapping for epoch %d still there", epoch)
	}
}

func TestSaveValidatorIdx_IdxNotInState(t *testing.T) {
	db := internal.SetupDB(t)
	defer internal.TeardownDB(t, db)
	epoch := uint64(100)

	// Tried to insert 5 active indices to DB with only 3 validators in state
	v.InsertActivatedIndices(epoch+1, []uint64{0, 1, 2, 3, 4})
	var validators []*pb.Validator
	for i := 0; i < 3; i++ {
		pubKeyBuf := make([]byte, params.BeaconConfig().BLSPubkeyLength)
		binary.PutUvarint(pubKeyBuf, uint64(i))
		validators = append(validators, &pb.Validator{
			Pubkey: pubKeyBuf,
		})
	}
	state := &pb.BeaconState{
		ValidatorRegistry: validators,
		Slot:              epoch * params.BeaconConfig().SlotsPerEpoch,
	}
	chainService := setupBeaconChain(t, db, nil)
	if err := chainService.saveValidatorIdx(state); err != nil {
		t.Fatalf("Could not save validator idx: %v", err)
	}

	wantedIdx := uint64(2)
	idx, err := chainService.beaconDB.ValidatorIndex(validators[wantedIdx].Pubkey)
	if err != nil {
		t.Fatalf("Could not get validator index: %v", err)
	}
	if wantedIdx != idx {
		t.Errorf("Wanted: %d, got: %d", wantedIdx, idx)
	}

	if v.ActivatedValFromEpoch(epoch) != nil {
		t.Errorf("Activated validators mapping for epoch %d still there", epoch)
	}

	// Verify the skipped validators are included in the next epoch
	if !reflect.DeepEqual(v.ActivatedValFromEpoch(epoch+2), []uint64{3, 4}) {
		t.Error("Did not get wanted validator from activation queue")
	}
}
