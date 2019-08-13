package sync

import (
	"context"
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/prysmaticlabs/prysm/beacon-chain/internal"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	ethpb "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/shared/deprecated-p2p"
	"github.com/prysmaticlabs/prysm/shared/event"
	"github.com/prysmaticlabs/prysm/shared/testutil"
	logTest "github.com/sirupsen/logrus/hooks/test"
)

type genesisPowChain struct {
	feed              *event.Feed
	depositsProcessed bool
}

func (mp *genesisPowChain) HasChainStarted() bool {
	return false
}

func (mp *genesisPowChain) BlockExists(ctx context.Context, hash common.Hash) (bool, *big.Int, error) {
	return true, big.NewInt(0), nil
}

func (mp *genesisPowChain) ChainStartFeed() *event.Feed {
	return mp.feed
}

func (mp *genesisPowChain) AreAllDepositsProcessed() (bool, error) {
	return mp.depositsProcessed, nil
}

type afterGenesisPowChain struct {
	feed *event.Feed
}

func (mp *afterGenesisPowChain) HasChainStarted() bool {
	return true
}

func (mp *afterGenesisPowChain) BlockExists(ctx context.Context, hash common.Hash) (bool, *big.Int, error) {
	return true, big.NewInt(0), nil
}

func (mp *afterGenesisPowChain) ChainStartFeed() *event.Feed {
	return mp.feed
}

func (mp *afterGenesisPowChain) AreAllDepositsProcessed() (bool, error) {
	return true, nil
}

func TestQuerier_StartStop(t *testing.T) {
	hook := logTest.NewGlobal()
	db := internal.SetupDB(t)
	defer internal.TeardownDB(t, db)
	cfg := &QuerierConfig{
		P2P:                &mockP2P{},
		ResponseBufferSize: 100,
		PowChain:           &afterGenesisPowChain{},
		BeaconDB:           db,
		ChainService:       &mockChainService{},
	}
	sq := NewQuerierService(context.Background(), cfg)

	exitRoutine := make(chan bool)

	defer func() {
		close(exitRoutine)
	}()

	go func() {
		sq.Start()
		exitRoutine <- true
	}()

	sq.Stop()
	<-exitRoutine

	testutil.AssertLogsContain(t, hook, "Stopping service")

	hook.Reset()
}

func TestListenForStateInitialization_ContextCancelled(t *testing.T) {
	db := internal.SetupDB(t)
	defer internal.TeardownDB(t, db)
	cfg := &QuerierConfig{
		P2P:                &mockP2P{},
		ResponseBufferSize: 100,
		ChainService:       &mockChainService{},
		BeaconDB:           db,
	}
	sq := NewQuerierService(context.Background(), cfg)
	exitRoutine := make(chan bool)

	defer func() {
		close(exitRoutine)
	}()

	go func() {
		sq.listenForStateInitialization()
		exitRoutine <- true
	}()

	sq.cancel()
	<-exitRoutine

	if sq.ctx.Done() == nil {
		t.Error("Despite context being canceled, the done channel is nil")
	}
}

func TestListenForStateInitialization(t *testing.T) {
	db := internal.SetupDB(t)
	defer internal.TeardownDB(t, db)
	cfg := &QuerierConfig{
		P2P:                &mockP2P{},
		ResponseBufferSize: 100,
		ChainService:       &mockChainService{},
		BeaconDB:           db,
	}
	sq := NewQuerierService(context.Background(), cfg)

	sq.chainStartBuf <- time.Now()
	sq.listenForStateInitialization()

	if !sq.chainStarted {
		t.Fatal("ChainStart in the querier service is not true despite the log being fired")
	}
	sq.cancel()
}

func TestQuerier_ChainReqResponse(t *testing.T) {
	hook := logTest.NewGlobal()
	cfg := &QuerierConfig{
		P2P:                &mockP2P{},
		ResponseBufferSize: 100,
		PowChain:           &afterGenesisPowChain{},
	}
	sq := NewQuerierService(context.Background(), cfg)

	exitRoutine := make(chan bool)
	go func() {
		sq.run()
		exitRoutine <- true
	}()

	response := &pb.ChainHeadResponse{
		CanonicalSlot:            1,
		CanonicalStateRootHash32: []byte{'a', 'b'},
	}

	msg := deprecated_p2p.Message{
		Data: response,
	}

	sq.responseBuf <- msg

	expMsg := fmt.Sprintf(
		"Latest chain head is at slot: %d and state root: %#x",
		response.CanonicalSlot, response.CanonicalStateRootHash32,
	)

	<-exitRoutine
	testutil.AssertLogsContain(t, hook, expMsg)
	close(exitRoutine)
	hook.Reset()
}

func TestQuerier_BestPeerAssignment(t *testing.T) {
	hook := logTest.NewGlobal()
	cfg := &QuerierConfig{
		P2P:                &mockP2P{},
		ResponseBufferSize: 100,
		PowChain:           &afterGenesisPowChain{},
	}
	sq := NewQuerierService(context.Background(), cfg)

	exitRoutine := make(chan bool)
	go func() {
		sq.run()
		exitRoutine <- true
	}()

	response := &pb.ChainHeadResponse{
		CanonicalSlot:            1,
		CanonicalStateRootHash32: []byte{'a', 'b'},
	}

	msg := deprecated_p2p.Message{
		Data: response,
		Peer: "TestQuerier_BestPeerAssignment",
	}

	sq.responseBuf <- msg

	<-exitRoutine
	testutil.AssertLogsContain(t, hook, "level=info msg=\"Peer with highest canonical head\" peerID=HupjP1BPtXeX766WHAeYyATx9MJ3RFe5MZCwC3UEw")

	close(exitRoutine)
	hook.Reset()
}

func TestSyncedInGenesis(t *testing.T) {
	db := internal.SetupDB(t)
	defer internal.TeardownDB(t, db)
	cfg := &QuerierConfig{
		P2P:                &mockP2P{},
		ResponseBufferSize: 100,
		ChainService:       &mockChainService{},
		BeaconDB:           db,
		PowChain:           &genesisPowChain{depositsProcessed: true},
	}
	sq := NewQuerierService(context.Background(), cfg)

	sq.chainStartBuf <- time.Now()
	sq.Start()

	synced, err := sq.IsSynced()
	if err != nil {
		t.Fatalf("Unable to check if the node is synced")
	}
	if !synced {
		t.Errorf("node is not synced when it is supposed to be")
	}
	sq.cancel()
}

func TestSyncedInRestarts(t *testing.T) {
	db := internal.SetupDB(t)
	defer internal.TeardownDB(t, db)
	cfg := &QuerierConfig{
		P2P:                &mockP2P{},
		ResponseBufferSize: 100,
		ChainService:       &mockChainService{},
		BeaconDB:           db,
		PowChain:           &afterGenesisPowChain{},
	}
	sq := NewQuerierService(context.Background(), cfg)

	bState := &pb.BeaconState{Slot: 0}
	blk := &ethpb.BeaconBlock{Slot: 0}
	if err := db.SaveState(context.Background(), bState); err != nil {
		t.Fatalf("Could not save state: %v", err)
	}
	if err := db.SaveBlock(blk); err != nil {
		t.Fatalf("Could not save state: %v", err)
	}
	if err := db.UpdateChainHead(context.Background(), blk, bState); err != nil {
		t.Fatalf("Could not update chainhead: %v", err)
	}

	exitRoutine := make(chan bool)
	go func() {
		sq.Start()
		exitRoutine <- true
	}()

	response := &pb.ChainHeadResponse{
		CanonicalSlot:            10,
		CanonicalStateRootHash32: []byte{'a', 'b'},
	}

	msg := deprecated_p2p.Message{
		Data: response,
	}

	sq.responseBuf <- msg

	<-exitRoutine

	synced, err := sq.IsSynced()
	if err != nil {
		t.Fatalf("Unable to check if the node is synced; %v", err)
	}
	if synced {
		t.Errorf("node is synced when it is not supposed to be in a restart")
	}
	sq.cancel()
}

func TestWaitForDepositsProcessed_OK(t *testing.T) {
	db := internal.SetupDB(t)
	defer internal.TeardownDB(t, db)
	powchain := &genesisPowChain{depositsProcessed: false}
	cfg := &QuerierConfig{
		P2P:                &mockP2P{},
		ResponseBufferSize: 100,
		ChainService:       &mockChainService{},
		BeaconDB:           db,
		PowChain:           powchain,
	}
	sq := NewQuerierService(context.Background(), cfg)

	sq.chainStartBuf <- time.Now()
	exitRoutine := make(chan bool)
	go func() {
		sq.waitForAllDepositsToBeProcessed()
		exitRoutine <- true
	}()
	if len(exitRoutine) == 1 {
		t.Fatal("Deposits processed despite not being ready")
	}

	powchain.depositsProcessed = true
	<-exitRoutine

	sq.cancel()
	close(exitRoutine)
}
