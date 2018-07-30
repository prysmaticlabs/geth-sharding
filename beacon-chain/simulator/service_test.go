package simulator

import (
	"context"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/event"
	"github.com/prysmaticlabs/prysm/shared/p2p"
	"github.com/prysmaticlabs/prysm/shared/testutil"
	logTest "github.com/sirupsen/logrus/hooks/test"
)

type mockP2P struct{}

func (mp *mockP2P) Feed(msg interface{}) *event.Feed {
	return new(event.Feed)
}

func (mp *mockP2P) Broadcast(msg interface{}) {}

func (mp *mockP2P) Send(msg interface{}, peer p2p.Peer) {}

type mockPOWChainService struct{}

func (mpow *mockPOWChainService) LatestBlockHash() common.Hash {
	return common.BytesToHash([]byte{})
}

func TestLifecycle(t *testing.T) {
	hook := logTest.NewGlobal()
	cfg := &Config{Delay: time.Second, BlockRequestBuf: 0}
	sim := NewSimulator(context.Background(), cfg, &mockP2P{}, &mockPOWChainService{})

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
	sim := NewSimulator(context.Background(), cfg, &mockP2P{}, &mockPOWChainService{})

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
