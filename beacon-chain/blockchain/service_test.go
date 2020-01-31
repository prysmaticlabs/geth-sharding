package blockchain

import (
	"bytes"
	"context"
	"encoding/hex"
	"io/ioutil"
	"reflect"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/gogo/protobuf/proto"
	ethpb "github.com/prysmaticlabs/ethereumapis/eth/v1alpha1"
	ssz "github.com/prysmaticlabs/go-ssz"
	"github.com/prysmaticlabs/prysm/beacon-chain/cache/depositcache"
	b "github.com/prysmaticlabs/prysm/beacon-chain/core/blocks"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/feed"
	statefeed "github.com/prysmaticlabs/prysm/beacon-chain/core/feed/state"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/state"
	"github.com/prysmaticlabs/prysm/beacon-chain/db"
	testDB "github.com/prysmaticlabs/prysm/beacon-chain/db/testing"
	"github.com/prysmaticlabs/prysm/beacon-chain/operations/attestations"
	"github.com/prysmaticlabs/prysm/beacon-chain/p2p"
	"github.com/prysmaticlabs/prysm/beacon-chain/powchain"
	beaconstate "github.com/prysmaticlabs/prysm/beacon-chain/state"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/prysmaticlabs/prysm/shared/event"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/testutil"
	"github.com/sirupsen/logrus"
	logTest "github.com/sirupsen/logrus/hooks/test"
)

func init() {
	logrus.SetLevel(logrus.DebugLevel)
	logrus.SetOutput(ioutil.Discard)
}

type store struct {
	headRoot []byte
}

func (s *store) OnBlock(ctx context.Context, b *ethpb.SignedBeaconBlock) (*beaconstate.BeaconState, error) {
	return nil, nil
}

func (s *store) OnBlockCacheFilteredTree(ctx context.Context, b *ethpb.SignedBeaconBlock) (*beaconstate.BeaconState, error) {
	return nil, nil
}

func (s *store) OnBlockInitialSyncStateTransition(ctx context.Context, b *ethpb.SignedBeaconBlock) (*beaconstate.BeaconState, error) {
	return nil, nil
}

func (s *store) OnAttestation(ctx context.Context, a *ethpb.Attestation) ([]uint64, error) {
	return nil, nil
}

func (s *store) GenesisStore(ctx context.Context, justifiedCheckpoint *ethpb.Checkpoint, finalizedCheckpoint *ethpb.Checkpoint) error {
	return nil
}

func (s *store) FinalizedCheckpt() *ethpb.Checkpoint {
	return nil
}

func (s *store) JustifiedCheckpt() *ethpb.Checkpoint {
	return nil
}

func (s *store) Head(ctx context.Context) ([]byte, error) {
	return s.headRoot, nil
}

type mockBeaconNode struct {
	stateFeed *event.Feed
}

// StateFeed mocks the same method in the beacon node.
func (mbn *mockBeaconNode) StateFeed() *event.Feed {
	if mbn.stateFeed == nil {
		mbn.stateFeed = new(event.Feed)
	}
	return mbn.stateFeed
}

type mockBroadcaster struct {
	broadcastCalled bool
}

func (mb *mockBroadcaster) Broadcast(_ context.Context, _ proto.Message) error {
	mb.broadcastCalled = true
	return nil
}

var _ = p2p.Broadcaster(&mockBroadcaster{})

func setupBeaconChain(t *testing.T, beaconDB db.Database) *Service {
	endpoint := "ws://127.0.0.1"
	ctx := context.Background()
	var web3Service *powchain.Service
	var err error
	web3Service, err = powchain.NewService(ctx, &powchain.Web3ServiceConfig{
		BeaconDB:        beaconDB,
		ETH1Endpoint:    endpoint,
		DepositContract: common.Address{},
	})
	if err != nil {
		t.Fatalf("unable to set up web3 service: %v", err)
	}

	cfg := &Config{
		BeaconBlockBuf:    0,
		BeaconDB:          beaconDB,
		DepositCache:      depositcache.NewDepositCache(),
		ChainStartFetcher: web3Service,
		P2p:               &mockBroadcaster{},
		StateNotifier:     &mockBeaconNode{},
		AttPool:           attestations.NewPool(),
	}
	if err != nil {
		t.Fatalf("could not register blockchain service: %v", err)
	}
	chainService, err := NewService(ctx, cfg)
	if err != nil {
		t.Fatalf("unable to setup chain service: %v", err)
	}
	chainService.genesisTime = time.Unix(1, 0) // non-zero time

	return chainService
}

func TestChainStartStop_Uninitialized(t *testing.T) {
	hook := logTest.NewGlobal()
	db := testDB.SetupDB(t)
	defer testDB.TeardownDB(t, db)
	chainService := setupBeaconChain(t, db)

	// Listen for state events.
	stateSubChannel := make(chan *feed.Event, 1)
	stateSub := chainService.stateNotifier.StateFeed().Subscribe(stateSubChannel)

	// Test the chain start state notifier.
	genesisTime := time.Unix(1, 0)
	chainService.Start()
	event := &feed.Event{
		Type: statefeed.ChainStarted,
		Data: &statefeed.ChainStartedData{
			StartTime: genesisTime,
		},
	}
	// Send in a loop to ensure it is delivered (busy wait for the service to subscribe to the state feed).
	for sent := 1; sent == 1; {
		sent = chainService.stateNotifier.StateFeed().Send(event)
		if sent == 1 {
			// Flush our local subscriber.
			<-stateSubChannel
		}
	}

	// Now wait for notification the state is ready.
	for stateInitialized := false; stateInitialized == false; {
		recv := <-stateSubChannel
		if recv.Type == statefeed.Initialized {
			stateInitialized = true
		}
	}
	stateSub.Unsubscribe()

	beaconState, err := db.HeadState(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if beaconState == nil || beaconState.Slot() != 0 {
		t.Error("Expected canonical state feed to send a state with genesis block")
	}
	if err := chainService.Stop(); err != nil {
		t.Fatalf("Unable to stop chain service: %v", err)
	}
	// The context should have been canceled.
	if chainService.ctx.Err() != context.Canceled {
		t.Error("Context was not canceled")
	}
	testutil.AssertLogsContain(t, hook, "Waiting")
	testutil.AssertLogsContain(t, hook, "Initialized beacon chain genesis state")
}

func TestChainStartStop_Initialized(t *testing.T) {
	hook := logTest.NewGlobal()
	ctx := context.Background()
	db := testDB.SetupDB(t)
	defer testDB.TeardownDB(t, db)

	chainService := setupBeaconChain(t, db)

	genesisBlk := b.NewGenesisBlock([]byte{})
	blkRoot, err := ssz.HashTreeRoot(genesisBlk.Block)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.SaveBlock(ctx, genesisBlk); err != nil {
		t.Fatal(err)
	}
	s, err := beaconstate.InitializeFromProto(&pb.BeaconState{Slot: 1})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.SaveState(ctx, s, blkRoot); err != nil {
		t.Fatal(err)
	}
	if err := db.SaveHeadBlockRoot(ctx, blkRoot); err != nil {
		t.Fatal(err)
	}
	if err := db.SaveGenesisBlockRoot(ctx, blkRoot); err != nil {
		t.Fatal(err)
	}
	if err := db.SaveJustifiedCheckpoint(ctx, &ethpb.Checkpoint{Root: blkRoot[:]}); err != nil {
		t.Fatal(err)
	}

	// Test the start function.
	chainService.Start()

	if err := chainService.Stop(); err != nil {
		t.Fatalf("unable to stop chain service: %v", err)
	}

	// The context should have been canceled.
	if chainService.ctx.Err() != context.Canceled {
		t.Error("context was not canceled")
	}
	testutil.AssertLogsContain(t, hook, "data already exists")
}

func TestChainService_InitializeBeaconChain(t *testing.T) {
	db := testDB.SetupDB(t)
	defer testDB.TeardownDB(t, db)
	ctx := context.Background()

	bc := setupBeaconChain(t, db)
	var err error

	// Set up 10 deposits pre chain start for validators to register
	count := uint64(10)
	deposits, _, _ := testutil.DeterministicDepositsAndKeys(count)
	trie, _, err := testutil.DepositTrieFromDeposits(deposits)
	if err != nil {
		t.Fatal(err)
	}
	hashTreeRoot := trie.HashTreeRoot()
	genState, err := state.EmptyGenesisState()
	if err != nil {
		t.Fatal(err)
	}
	genState.SetEth1Data(&ethpb.Eth1Data{
		DepositRoot:  hashTreeRoot[:],
		DepositCount: uint64(len(deposits)),
	})
	genState, err = b.ProcessDeposits(ctx, genState, &ethpb.BeaconBlockBody{Deposits: deposits})
	if err != nil {
		t.Fatal(err)
	}
	if err := bc.initializeBeaconChain(ctx, time.Unix(0, 0), genState, &ethpb.Eth1Data{
		DepositRoot: hashTreeRoot[:],
	}); err != nil {
		t.Fatal(err)
	}

	s, err := bc.beaconDB.State(ctx, bytesutil.ToBytes32(bc.canonicalRoots[0]))
	if err != nil {
		t.Fatal(err)
	}

	for _, v := range s.Validators() {
		if !db.HasValidatorIndex(ctx, v.PublicKey) {
			t.Errorf("Validator %s missing from db", hex.EncodeToString(v.PublicKey))
		}
	}

	if _, err := bc.HeadState(ctx); err != nil {
		t.Error(err)
	}
	if bc.HeadBlock() == nil {
		t.Error("Head state can't be nil after initialize beacon chain")
	}
	if bc.canonicalRoots[0] == nil {
		t.Error("Canonical root for slot 0 can't be nil after initialize beacon chain")
	}
}

func TestChainService_InitializeChainInfo(t *testing.T) {
	db := testDB.SetupDB(t)
	defer testDB.TeardownDB(t, db)
	ctx := context.Background()

	genesis := b.NewGenesisBlock([]byte{})
	genesisRoot, err := ssz.HashTreeRoot(genesis.Block)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.SaveGenesisBlockRoot(ctx, genesisRoot); err != nil {
		t.Fatal(err)
	}
	if err := db.SaveBlock(ctx, genesis); err != nil {
		t.Fatal(err)
	}

	finalizedSlot := params.BeaconConfig().SlotsPerEpoch*2 + 1
	headBlock := &ethpb.SignedBeaconBlock{Block: &ethpb.BeaconBlock{Slot: finalizedSlot, ParentRoot: genesisRoot[:]}}
	headState, err := beaconstate.InitializeFromProto(&pb.BeaconState{Slot: finalizedSlot})
	if err != nil {
		t.Fatal(err)
	}
	headRoot, _ := ssz.HashTreeRoot(headBlock.Block)
	if err := db.SaveState(ctx, headState, headRoot); err != nil {
		t.Fatal(err)
	}
	if err := db.SaveBlock(ctx, headBlock); err != nil {
		t.Fatal(err)
	}
	if err := db.SaveFinalizedCheckpoint(ctx, &ethpb.Checkpoint{
		Epoch: helpers.SlotToEpoch(finalizedSlot),
		Root:  headRoot[:],
	}); err != nil {
		t.Fatal(err)
	}
	if err := db.SaveBlock(ctx, headBlock); err != nil {
		t.Fatal(err)
	}
	c := &Service{beaconDB: db, canonicalRoots: make(map[uint64][]byte)}
	if err := c.initializeChainInfo(ctx); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(c.HeadBlock(), headBlock) {
		t.Error("head block incorrect")
	}
	s, err := c.HeadState(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(s, headState) {
		t.Error("head state incorrect")
	}
	if headBlock.Block.Slot != c.HeadSlot() {
		t.Error("head slot incorrect")
	}
	r, err := c.HeadRoot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(headRoot[:], r) {
		t.Error("head slot incorrect")
	}
	if c.genesisRoot != genesisRoot {
		t.Error("genesis block root incorrect")
	}
}

func TestChainService_SaveHeadNoDB(t *testing.T) {
	db := testDB.SetupDB(t)
	defer testDB.TeardownDB(t, db)
	ctx := context.Background()
	s := &Service{
		beaconDB:       db,
		canonicalRoots: make(map[uint64][]byte),
	}
	b := &ethpb.SignedBeaconBlock{Block: &ethpb.BeaconBlock{Slot: 1}}
	r, _ := ssz.HashTreeRoot(b)
	if err := s.saveHeadNoDB(ctx, b, r); err != nil {
		t.Fatal(err)
	}

	newB, err := s.beaconDB.HeadBlock(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if reflect.DeepEqual(newB, b) {
		t.Error("head block should not be equal")
	}
}
