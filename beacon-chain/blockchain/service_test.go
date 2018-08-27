package blockchain

import (
	"context"
	"io/ioutil"
	"testing"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	gethTypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
	"github.com/prysmaticlabs/prysm/beacon-chain/params"
	"github.com/prysmaticlabs/prysm/beacon-chain/powchain"
	"github.com/prysmaticlabs/prysm/beacon-chain/types"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	"github.com/prysmaticlabs/prysm/shared/database"
	"github.com/prysmaticlabs/prysm/shared/testutil"
	"github.com/sirupsen/logrus"
	logTest "github.com/sirupsen/logrus/hooks/test"
)

func init() {
	logrus.SetLevel(logrus.DebugLevel)
	logrus.SetOutput(ioutil.Discard)
}

type mockClient struct{}

func (f *mockClient) SubscribeNewHead(ctx context.Context, ch chan<- *gethTypes.Header) (ethereum.Subscription, error) {
	return new(event.Feed).Subscribe(ch), nil
}

func (f *mockClient) BlockByHash(ctx context.Context, hash common.Hash) (*gethTypes.Block, error) {
	return nil, nil
}

func (f *mockClient) SubscribeFilterLogs(ctx context.Context, q ethereum.FilterQuery, ch chan<- gethTypes.Log) (ethereum.Subscription, error) {
	return new(event.Feed).Subscribe(ch), nil
}

func (f *mockClient) LatestBlockHash() common.Hash {
	return common.BytesToHash([]byte{'A'})
}

func TestStartStop(t *testing.T) {
	ctx := context.Background()

	config := &database.DBConfig{DataDir: "", Name: "", InMemory: true}
	db, err := database.NewDB(config)
	if err != nil {
		t.Fatalf("could not setup beaconDB: %v", err)
	}

	endpoint := "ws://127.0.0.1"
	client := &mockClient{}
	web3Service, err := powchain.NewWeb3Service(ctx, &powchain.Web3ServiceConfig{Endpoint: endpoint, Pubkey: "", VrcAddr: common.Address{}}, client, client, client)
	if err != nil {
		t.Fatalf("unable to set up web3 service: %v", err)
	}
	beaconChain, err := NewBeaconChain(db.DB())
	cfg := &Config{
		BeaconBlockBuf: 0,
		BeaconDB:       db.DB(),
		Chain:          beaconChain,
	}
	if err != nil {
		t.Fatalf("could not register blockchain service: %v", err)
	}
	chainService, err := NewChainService(ctx, cfg)
	if err != nil {
		t.Fatalf("unable to setup chain service: %v", err)
	}
	chainService.Start()

	cfg = &Config{
		BeaconBlockBuf: 0,
		BeaconDB:       db.DB(),
		Chain:          beaconChain,
		Web3Service:    web3Service,
	}
	chainService, err = NewChainService(ctx, cfg)
	if err != nil {
		t.Fatalf("unable to setup chain service: %v", err)
	}
	chainService.Start()

	if len(chainService.CurrentActiveState().RecentBlockHashes()) != 0 {
		t.Errorf("incorrect recent block hashes")
	}
	if len(chainService.CurrentCrystallizedState().Validators()) != params.BootstrappedValidatorsCount {
		t.Errorf("incorrect default validator size")
	}
	if chainService.ContainsBlock([32]byte{}) {
		t.Errorf("chain is not empty")
	}
	hasState, err := chainService.HasStoredState()
	if err != nil {
		t.Fatalf("calling HasStoredState failed")
	}
	if hasState {
		t.Errorf("has stored state should return false")
	}
	chainService.CanonicalBlockFeed()
	chainService.CanonicalCrystallizedStateFeed()

	chainService, _ = NewChainService(ctx, cfg)

	active := types.NewActiveState(&pb.ActiveState{RecentBlockHashes: [][]byte{{'A'}}})
	activeStateHash, err := active.Hash()
	if err != nil {
		t.Fatalf("Cannot hash active state: %v", err)
	}
	chainService.chain.SetActiveState(active)

	crystallized := types.NewCrystallizedState(&pb.CrystallizedState{LastStateRecalc: 10000})
	crystallizedStateHash, err := crystallized.Hash()
	if err != nil {
		t.Fatalf("Cannot hash crystallized state: %v", err)
	}
	chainService.chain.SetCrystallizedState(crystallized)

	parentBlock := NewBlock(t, nil)
	parentHash, _ := parentBlock.Hash()

	block := NewBlock(t, &pb.BeaconBlock{
		SlotNumber:            2,
		ActiveStateHash:       activeStateHash[:],
		CrystallizedStateHash: crystallizedStateHash[:],
		ParentHash:            parentHash[:],
		PowChainRef:           []byte("a"),
	})
	if err := chainService.SaveBlock(block); err != nil {
		t.Errorf("save block should have failed")
	}

	// Save states so HasStoredState state should return true.
	chainService.chain.SetActiveState(types.NewActiveState(&pb.ActiveState{}))
	chainService.chain.SetCrystallizedState(types.NewCrystallizedState(&pb.CrystallizedState{}))
	hasState, _ = chainService.HasStoredState()
	if !hasState {
		t.Errorf("has stored state should return false")
	}

	if err := chainService.Stop(); err != nil {
		t.Fatalf("unable to stop chain service: %v", err)
	}

	// The context should have been canceled.
	if chainService.ctx.Err() == nil {
		t.Error("context was not canceled")
	}
}

func TestFaultyStop(t *testing.T) {
	ctx := context.Background()
	config := &database.DBConfig{DataDir: "", Name: "", InMemory: true}
	db, err := database.NewDB(config)
	if err != nil {
		t.Fatalf("could not setup beaconDB: %v", err)

	}
	endpoint := "ws://127.0.0.1"
	client := &mockClient{}
	web3Service, err := powchain.NewWeb3Service(ctx, &powchain.Web3ServiceConfig{Endpoint: endpoint, Pubkey: "", VrcAddr: common.Address{}}, client, client, client)
	if err != nil {
		t.Fatalf("unable to set up web3 service: %v", err)
	}
	beaconChain, err := NewBeaconChain(db.DB())
	if err != nil {
		t.Fatalf("could not register blockchain service: %v", err)
	}
	cfg := &Config{
		BeaconBlockBuf: 0,
		BeaconDB:       db.DB(),
		Chain:          beaconChain,
		Web3Service:    web3Service,
	}

	chainService, err := NewChainService(ctx, cfg)
	if err != nil {
		t.Fatalf("unable to setup chain service: %v", err)
	}

	chainService.Start()

	chainService.chain.SetActiveState(types.NewActiveState(nil))
	err = chainService.Stop()
	if err == nil {
		t.Errorf("chain stop should have failed with persist active state")
	}

	chainService.chain.SetActiveState(types.NewActiveState(&pb.ActiveState{}))
	chainService.chain.SetCrystallizedState(types.NewCrystallizedState(nil))
	err = chainService.Stop()
	if err == nil {
		t.Errorf("chain stop should have failed with persist crystallized state")
	}
}

func TestProcessingBadBlock(t *testing.T) {
	hook := logTest.NewGlobal()
	ctx := context.Background()
	config := &database.DBConfig{DataDir: "", Name: "", InMemory: true}
	db, err := database.NewDB(config)
	if err != nil {
		t.Fatalf("could not setup beaconDB: %v", err)

	}
	endpoint := "ws://127.0.0.1"
	client := &mockClient{}
	web3Service, err := powchain.NewWeb3Service(ctx, &powchain.Web3ServiceConfig{Endpoint: endpoint, Pubkey: "", VrcAddr: common.Address{}}, client, client, client)
	if err != nil {
		t.Fatalf("unable to set up web3 service: %v", err)
	}
	beaconChain, err := NewBeaconChain(db.DB())
	if err != nil {
		t.Fatalf("could not register blockchain service: %v", err)
	}

	var validators []*pb.ValidatorRecord
	for i := 0; i < 40; i++ {
		validator := &pb.ValidatorRecord{Balance: 32, StartDynasty: 1, EndDynasty: 10}
		validators = append(validators, validator)
	}

	crystallized := types.NewCrystallizedState(&pb.CrystallizedState{Validators: validators, CurrentDynasty: 5})
	crystallizedStateHash, err := crystallized.Hash()
	if err != nil {
		t.Fatalf("Cannot hash crystallized state: %v", err)
	}

	testAttesterBitfield := []byte{200, 148, 146, 179, 49}
	active := types.NewActiveState(&pb.ActiveState{PendingAttestations: []*pb.AttestationRecord{{AttesterBitfield: testAttesterBitfield}}})
	if err := beaconChain.SetActiveState(active); err != nil {
		t.Fatalf("unable to Mutate Active state: %v", err)
	}

	cfg := &Config{
		BeaconBlockBuf: 0,
		BeaconDB:       db.DB(),
		Chain:          beaconChain,
		Web3Service:    web3Service,
	}
	chainService, _ := NewChainService(ctx, cfg)
	chainService.chain.SetCrystallizedState(crystallized)

	parentBlock := NewBlock(t, nil)
	if err := chainService.SaveBlock(parentBlock); err != nil {
		t.Fatal(err)
	}
	parentHash, _ := parentBlock.Hash()

	activeStateHash, err := active.Hash()
	if err != nil {
		t.Fatalf("Cannot hash active state: %v", err)
	}

	block := NewBlock(t, &pb.BeaconBlock{
		SlotNumber:            1,
		ActiveStateHash:       activeStateHash[:],
		CrystallizedStateHash: crystallizedStateHash[:],
		ParentHash:            parentHash[:],
		PowChainRef:           []byte("a"),
	})

	exitRoutine := make(chan bool)
	go func() {
		chainService.blockProcessing(chainService.ctx.Done())
		<-exitRoutine
	}()
	if err := chainService.SaveBlock(block); err != nil {
		t.Fatal(err)
	}

	chainService.incomingBlockChan <- block
	chainService.cancel()
	exitRoutine <- true

	testutil.AssertLogsContain(t, hook, "Finished processing received block and states into DAG")
}

func TestUpdateHead(t *testing.T) {
	hook := logTest.NewGlobal()
	ctx := context.Background()
	config := &database.DBConfig{DataDir: "", Name: "", InMemory: true}
	db, err := database.NewDB(config)
	if err != nil {
		t.Fatalf("could not setup beaconDB: %v", err)

	}
	endpoint := "ws://127.0.0.1"
	client := &mockClient{}
	web3Service, err := powchain.NewWeb3Service(ctx, &powchain.Web3ServiceConfig{Endpoint: endpoint, Pubkey: "", VrcAddr: common.Address{}}, client, client, client)
	if err != nil {
		t.Fatalf("unable to set up web3 service: %v", err)
	}
	beaconChain, err := NewBeaconChain(db.DB())
	if err != nil {
		t.Fatalf("could not register blockchain service: %v", err)
	}
	cfg := &Config{
		BeaconBlockBuf:   0,
		IncomingBlockBuf: 0,
		BeaconDB:         db.DB(),
		Chain:            beaconChain,
		Web3Service:      web3Service,
	}
	chainService, _ := NewChainService(ctx, cfg)

	active, crystallized, err := types.NewGenesisStates()
	if err != nil {
		t.Fatalf("Can't generate genesis state: %v", err)
	}
	activeStateHash, _ := active.Hash()
	crystallizedStateHash, _ := crystallized.Hash()

	parentHash := []byte{}
	chainService.processedBlockHashesBySlot[4] = append(
		chainService.processedBlockHashesBySlot[4],
		parentHash,
	)

	block := NewBlock(t, &pb.BeaconBlock{
		SlotNumber:            64,
		ActiveStateHash:       activeStateHash[:],
		CrystallizedStateHash: crystallizedStateHash[:],
		ParentHash:            parentHash,
		PowChainRef:           []byte("a"),
	})

	h, err := block.Hash()
	if err != nil {
		t.Fatal(err)
	}

	chainService.processedBlockHashesBySlot[64] = append(
		chainService.processedBlockHashesBySlot[64],
		h[:],
	)
	chainService.processedBlocksBySlot[64] = append(
		chainService.processedBlocksBySlot[64],
		block,
	)
	chainService.processedActiveStatesBySlot[64] = append(
		chainService.processedActiveStatesBySlot[64],
		active,
	)
	chainService.processedCrystallizedStatesBySlot[64] = append(
		chainService.processedCrystallizedStatesBySlot[64],
		crystallized,
	)

	chainService.lastSlot = 64
	chainService.updateHead(65)
	testutil.AssertLogsContain(t, hook, "Canonical block determined")

	chainService.lastSlot = 100
	chainService.updateHead(101)
}

func TestProcessingBlockWithAttestations(t *testing.T) {
	ctx := context.Background()
	config := &database.DBConfig{DataDir: "", Name: "", InMemory: true}
	db, err := database.NewDB(config)
	if err != nil {
		t.Fatalf("could not setup beaconDB: %v", err)

	}
	endpoint := "ws://127.0.0.1"
	client := &mockClient{}
	web3Service, err := powchain.NewWeb3Service(ctx, &powchain.Web3ServiceConfig{Endpoint: endpoint, Pubkey: "", VrcAddr: common.Address{}}, client, client, client)
	if err != nil {
		t.Fatalf("unable to set up web3 service: %v", err)
	}
	beaconChain, err := NewBeaconChain(db.DB())
	if err != nil {
		t.Fatalf("could not register blockchain service: %v", err)
	}

	var validators []*pb.ValidatorRecord
	for i := 0; i < 40; i++ {
		validator := &pb.ValidatorRecord{Balance: 32, StartDynasty: 1, EndDynasty: 10}
		validators = append(validators, validator)
	}

	crystallized := types.NewCrystallizedState(&pb.CrystallizedState{
		LastStateRecalc: 64,
		Validators:      validators,
		CurrentDynasty:  5,
		IndicesForSlots: []*pb.ShardAndCommitteeArray{
			{
				ArrayShardAndCommittee: []*pb.ShardAndCommittee{
					{ShardId: 0, Committee: []uint32{0, 1, 2, 3, 4, 5}},
				},
			},
		},
	})
	crystallizedStateHash, err := crystallized.Hash()
	if err != nil {
		t.Fatalf("Cannot hash crystallized state: %v", err)
	}
	if err := beaconChain.SetCrystallizedState(crystallized); err != nil {
		t.Fatalf("unable to mutate crystallized state: %v", err)
	}

	var recentBlockHashes [][]byte
	for i := 0; i < params.CycleLength+1; i++ {
		recentBlockHashes = append(recentBlockHashes, []byte{'X'})
	}
	active := types.NewActiveState(&pb.ActiveState{RecentBlockHashes: recentBlockHashes})
	if err := beaconChain.SetActiveState(active); err != nil {
		t.Fatalf("unable to mutate active state: %v", err)
	}
	cfg := &Config{
		BeaconBlockBuf: 0,
		BeaconDB:       db.DB(),
		Chain:          beaconChain,
		Web3Service:    web3Service,
	}

	chainService, _ := NewChainService(ctx, cfg)

	exitRoutine := make(chan bool)
	go func() {
		chainService.blockProcessing(chainService.ctx.Done())
		<-exitRoutine
	}()

	parentBlock := NewBlock(t, nil)
	if err := chainService.SaveBlock(parentBlock); err != nil {
		t.Fatal(err)
	}
	parentHash, _ := parentBlock.Hash()

	activeStateHash, err := active.Hash()
	if err != nil {
		t.Fatalf("Cannot hash active state: %v", err)
	}

	block := NewBlock(t, &pb.BeaconBlock{
		SlotNumber:            1,
		ActiveStateHash:       activeStateHash[:],
		CrystallizedStateHash: crystallizedStateHash[:],
		ParentHash:            parentHash[:],
		PowChainRef:           []byte("a"),
		Attestations: []*pb.AttestationRecord{
			{Slot: 0, ShardId: 0, AttesterBitfield: []byte{'0'}},
		},
	})

	chainService.incomingBlockChan <- block
	chainService.cancel()
	exitRoutine <- true
}
