package simulator

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/event"
	"github.com/prysmaticlabs/prysm/beacon-chain/types"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	"github.com/prysmaticlabs/prysm/shared/p2p"
	"github.com/prysmaticlabs/prysm/shared/testutil"
	logTest "github.com/sirupsen/logrus/hooks/test"
)

type mockP2P struct{}

func (mp *mockP2P) Subscribe(msg interface{}, channel interface{}) event.Subscription {
	return new(event.Feed).Subscribe(channel)
}

func (mp *mockP2P) Broadcast(msg interface{}) {}

func (mp *mockP2P) Send(msg interface{}, peer p2p.Peer) {}

type mockPOWChainService struct{}

func (mpow *mockPOWChainService) LatestBlockHash() common.Hash {
	return common.BytesToHash([]byte{})
}

type mockChainService struct{}

func (mc *mockChainService) CurrentActiveState() *types.ActiveState {
	return types.NewActiveState(&pb.ActiveState{})
}

func (mc *mockChainService) CurrentCrystallizedState() *types.CrystallizedState {
	return types.NewCrystallizedState(&pb.CrystallizedState{})
}

func TestLifecycle(t *testing.T) {
	hook := logTest.NewGlobal()
	cfg := &Config{Delay: time.Second, BlockRequestBuf: 0}
	sim := NewSimulator(context.Background(), cfg, &mockP2P{}, &mockPOWChainService{}, &mockChainService{})

	sim.Start()
	testutil.AssertLogsContain(t, hook, "Starting service")
	sim.Stop()
	testutil.AssertLogsContain(t, hook, "Stopping service")

	// The context should have been canceled.
	if sim.ctx.Err() == nil {
		t.Error("context was not canceled")
	}
}

func TestBroadcastBlockHash(t *testing.T) {
	hook := logTest.NewGlobal()
	cfg := &Config{Delay: time.Second, BlockRequestBuf: 0}
	sim := NewSimulator(context.Background(), cfg, &mockP2P{}, &mockPOWChainService{}, &mockChainService{})

	delayChan := make(chan time.Time)
	doneChan := make(chan struct{})
	exitRoutine := make(chan bool)

	go func() {
		sim.run(delayChan, doneChan)
		<-exitRoutine
	}()

	delayChan <- time.Time{}
	doneChan <- struct{}{}

	testutil.AssertLogsContain(t, hook, "Announcing block hash")

	exitRoutine <- true

	if len(sim.broadcastedBlockHashes) != 1 {
		t.Error("Did not store the broadcasted block hash")
	}
	hook.Reset()
}

func TestBlockRequest(t *testing.T) {
	hook := logTest.NewGlobal()
	cfg := &Config{Delay: time.Second, BlockRequestBuf: 0}
	sim := NewSimulator(context.Background(), cfg, &mockP2P{}, &mockPOWChainService{}, &mockChainService{})

	delayChan := make(chan time.Time)
	doneChan := make(chan struct{})
	exitRoutine := make(chan bool)

	go func() {
		sim.run(delayChan, doneChan)
		<-exitRoutine
	}()

	block, err := types.NewBlock(&pb.BeaconBlock{ParentHash: make([]byte, 32)})
	if err != nil {
		t.Fatalf("Could not instantiate new block from proto: %v", err)
	}
	h, err := block.Hash()
	if err != nil {
		t.Fatal(err)
	}

	data := &pb.BeaconBlockRequest{
		Hash: h[:],
	}

	msg := p2p.Message{
		Peer: p2p.Peer{},
		Data: data,
	}

	sim.broadcastedBlockHashes[h] = block

	sim.blockRequestChan <- msg
	doneChan <- struct{}{}
	exitRoutine <- true

	testutil.AssertLogsContain(t, hook, fmt.Sprintf("Responding to full block request for hash: 0x%x", h))
}

func TestBroadcastCrystallizedHash(t *testing.T) {
	hook := logTest.NewGlobal()
	cfg := &Config{Delay: time.Second, BlockRequestBuf: 0}
	sim := NewSimulator(context.Background(), cfg, &mockP2P{}, &mockPOWChainService{}, &mockChainService{})

	delayChan := make(chan time.Time)
	doneChan := make(chan struct{})
	exitRoutine := make(chan bool)

	sim.slotNum = 64

	go func() {
		sim.run(delayChan, doneChan)
		<-exitRoutine
	}()

	delayChan <- time.Time{}
	doneChan <- struct{}{}

	testutil.AssertLogsContain(t, hook, "Announcing crystallized state hash")

	exitRoutine <- true

	if len(sim.broadcastedCrystallizedHashes) != 1 {
		t.Error("Did not store the broadcasted state hash")
	}
	hook.Reset()
}

func TestCrystallizedRequest(t *testing.T) {
	hook := logTest.NewGlobal()
	cfg := &Config{Delay: time.Second, BlockRequestBuf: 0}
	sim := NewSimulator(context.Background(), cfg, &mockP2P{}, &mockPOWChainService{}, &mockChainService{})

	delayChan := make(chan time.Time)
	doneChan := make(chan struct{})
	exitRoutine := make(chan bool)

	go func() {
		sim.run(delayChan, doneChan)
		<-exitRoutine
	}()

	state := types.NewCrystallizedState(&pb.CrystallizedState{CurrentEpoch: 99})

	h, err := state.Hash()
	if err != nil {
		t.Fatal(err)
	}

	data := &pb.CrystallizedStateRequest{
		Hash: h[:],
	}

	msg := p2p.Message{
		Peer: p2p.Peer{},
		Data: data,
	}

	sim.broadcastedCrystallizedHashes[h] = state

	sim.crystallizedStateRequestChan <- msg
	doneChan <- struct{}{}
	exitRoutine <- true

	testutil.AssertLogsContain(t, hook, fmt.Sprintf("Responding to crystallized state request for hash: 0x%x", h))
}
