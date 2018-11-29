package initialsync

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/prysmaticlabs/prysm/beacon-chain/types"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	"github.com/prysmaticlabs/prysm/shared/event"
	"github.com/prysmaticlabs/prysm/shared/p2p"
	"github.com/prysmaticlabs/prysm/shared/testutil"
	logTest "github.com/sirupsen/logrus/hooks/test"
)

type mockP2P struct {
}

func (mp *mockP2P) Subscribe(msg proto.Message, channel chan p2p.Message) event.Subscription {
	return new(event.Feed).Subscribe(channel)
}

func (mp *mockP2P) Broadcast(msg proto.Message) {}

func (mp *mockP2P) Send(msg proto.Message, peer p2p.Peer) {
}

type mockSyncService struct {
	hasStarted bool
	isSynced   bool
}

func (ms *mockSyncService) Start() {
	ms.hasStarted = true
}

func (ms *mockSyncService) IsSyncedWithNetwork() bool {
	return ms.isSynced
}

func (ms *mockSyncService) ResumeSync() {

}

type mockQueryService struct {
	isSynced bool
}

func (ms *mockQueryService) IsSynced() (bool, error) {
	return ms.isSynced, nil
}

type mockDB struct{}

func (m *mockDB) SaveBlock(*types.Block) error {
	return nil
}

func (m *mockDB) SaveCrystallizedState(*types.CrystallizedState) error {
	return nil
}

func blockResponse(slot uint64, t *testing.T) (p2p.Message, [32]byte) {
	genericHash := make([]byte, 32)
	genericHash[0] = 'a'

	block := &pb.BeaconBlock{
		PowChainRef:           []byte{1, 2, 3},
		AncestorHashes:        [][]byte{genericHash},
		Slot:                  slot,
		CrystallizedStateRoot: nil,
	}

	blockResponse := &pb.BeaconBlockResponse{
		Block: block,
	}

	hash, err := types.NewBlock(block).Hash()
	if err != nil {
		t.Fatalf("unable to hash block %v", err)
	}

	return p2p.Message{
		Peer: p2p.Peer{},
		Data: blockResponse,
	}, hash
}

func TestSetBlockForInitialSync(t *testing.T) {
	hook := logTest.NewGlobal()

	cfg := Config{
		P2P:         &mockP2P{},
		SyncService: &mockSyncService{},
		BeaconDB:    &mockDB{},
	}

	ss := NewInitialSyncService(context.Background(), cfg)

	exitRoutine := make(chan bool)
	delayChan := make(chan time.Time)
	defer func() {
		close(exitRoutine)
		close(delayChan)
	}()

	go func() {
		ss.run(delayChan)
		exitRoutine <- true
	}()

	msg1, _ := blockResponse(1, t)
	block := msg1.Data.(*pb.BeaconBlockResponse).GetBlock()

	ss.blockBuf <- msg1

	ss.cancel()
	<-exitRoutine

	var hash [32]byte
	copy(hash[:], block.CrystallizedStateRoot)

	if hash != ss.initialCrystallizedStateRoot {
		t.Fatalf("Crystallized state hash not updated: %#x", block.CrystallizedStateRoot)
	}

	hook.Reset()
}

func TestSavingBlocksInSync(t *testing.T) {
	hook := logTest.NewGlobal()

	cfg := Config{
		P2P:                         &mockP2P{},
		SyncService:                 &mockSyncService{},
		BeaconDB:                    &mockDB{},
		CrystallizedStateBufferSize: 100,
	}
	ss := NewInitialSyncService(context.Background(), cfg)

	exitRoutine := make(chan bool)
	delayChan := make(chan time.Time)

	defer func() {
		close(exitRoutine)
		close(delayChan)
	}()

	go func() {
		ss.run(delayChan)
		exitRoutine <- true
	}()

	genericHash := make([]byte, 32)
	genericHash[0] = 'a'

	crystallizedState := &pb.CrystallizedState{
		LastFinalizedSlot: 99,
	}

	stateResponse := &pb.CrystallizedStateResponse{
		CrystallizedState: crystallizedState,
	}

	incorrectState := &pb.CrystallizedState{
		LastFinalizedSlot: 9,
		LastJustifiedSlot: 20,
	}

	incorrectStateResponse := &pb.CrystallizedStateResponse{
		CrystallizedState: incorrectState,
	}

	crystallizedStateRoot, err := types.NewCrystallizedState(crystallizedState).Hash()
	if err != nil {
		t.Fatalf("unable to get hash of crystallized state: %v", err)
	}

	getBlockResponseMsg := func(Slot uint64) p2p.Message {
		block := &pb.BeaconBlock{
			PowChainRef:           []byte{1, 2, 3},
			AncestorHashes:        [][]byte{genericHash},
			Slot:                  Slot,
			CrystallizedStateRoot: crystallizedStateRoot[:],
		}

		blockResponse := &pb.BeaconBlockResponse{
			Block: block,
		}

		return p2p.Message{
			Peer: p2p.Peer{},
			Data: blockResponse,
		}
	}

	msg1 := getBlockResponseMsg(1)

	msg2 := p2p.Message{
		Peer: p2p.Peer{},
		Data: incorrectStateResponse,
	}

	ss.blockBuf <- msg1
	ss.crystallizedStateBuf <- msg2

	if ss.currentSlot == incorrectStateResponse.CrystallizedState.LastFinalizedSlot {
		t.Fatalf("Crystallized state updated incorrectly: %d", ss.currentSlot)
	}

	msg2.Data = stateResponse

	ss.crystallizedStateBuf <- msg2

	if crystallizedStateRoot != ss.initialCrystallizedStateRoot {
		br := msg1.Data.(*pb.BeaconBlockResponse)
		t.Fatalf("Crystallized state hash not updated to: %#x instead it is %#x", br.Block.CrystallizedStateRoot,
			ss.initialCrystallizedStateRoot)
	}

	msg1 = getBlockResponseMsg(30)
	ss.blockBuf <- msg1

	if stateResponse.CrystallizedState.GetLastFinalizedSlot() != ss.currentSlot {
		t.Fatalf("Slot saved when it was not supposed too: %v", stateResponse.CrystallizedState.GetLastFinalizedSlot())
	}

	msg1 = getBlockResponseMsg(100)
	ss.blockBuf <- msg1

	ss.cancel()
	<-exitRoutine

	br := msg1.Data.(*pb.BeaconBlockResponse)
	if br.Block.GetSlot() != ss.currentSlot {
		t.Fatalf("Slot not updated despite receiving a valid block: %v", ss.currentSlot)
	}

	hook.Reset()
}

func TestDelayChan(t *testing.T) {
	hook := logTest.NewGlobal()
	cfg := Config{
		P2P:         &mockP2P{},
		SyncService: &mockSyncService{},
		BeaconDB:    &mockDB{},
	}
	ss := NewInitialSyncService(context.Background(), cfg)

	exitRoutine := make(chan bool)
	delayChan := make(chan time.Time)

	defer func() {
		close(exitRoutine)
		close(delayChan)
	}()

	go func() {
		ss.run(delayChan)
		exitRoutine <- true
	}()

	genericHash := make([]byte, 32)
	genericHash[0] = 'a'

	crystallizedstate := &pb.CrystallizedState{
		LastFinalizedSlot: 99,
	}

	stateResponse := &pb.CrystallizedStateResponse{
		CrystallizedState: crystallizedstate,
	}

	crystallizedStateRoot, err := types.NewCrystallizedState(stateResponse.CrystallizedState).Hash()
	if err != nil {
		t.Fatalf("unable to get hash of crystallized state: %v", err)
	}

	block := &pb.BeaconBlock{
		PowChainRef:           []byte{1, 2, 3},
		AncestorHashes:        [][]byte{genericHash},
		Slot:                  uint64(1),
		CrystallizedStateRoot: crystallizedStateRoot[:],
	}

	blockResponse := &pb.BeaconBlockResponse{
		Block: block,
	}

	msg1 := p2p.Message{
		Peer: p2p.Peer{},
		Data: blockResponse,
	}

	msg2 := p2p.Message{
		Peer: p2p.Peer{},
		Data: stateResponse,
	}

	ss.blockBuf <- msg1

	ss.crystallizedStateBuf <- msg2

	blockResponse.Block.Slot = 100
	msg1.Data = blockResponse

	ss.blockBuf <- msg1

	delayChan <- time.Time{}

	ss.cancel()
	<-exitRoutine

	testutil.AssertLogsContain(t, hook, "Exiting initial sync and starting normal sync")

	hook.Reset()
}

func TestIsSyncedWithNetwork(t *testing.T) {
	hook := logTest.NewGlobal()
	mockSync := &mockSyncService{}
	cfg := Config{
		P2P:         &mockP2P{},
		SyncService: mockSync,
		BeaconDB:    &mockDB{},
		QueryService: &mockQueryService{
			isSynced: true,
		},
		SyncPollingInterval: 1,
	}
	ss := NewInitialSyncService(context.Background(), cfg)

	ss.Start()
	ss.Stop()

	testutil.AssertLogsContain(t, hook, "Chain fully synced, exiting initial sync")
	testutil.AssertLogsContain(t, hook, "Stopping service")

	hook.Reset()
}

func TestIsNotSyncedWithNetwork(t *testing.T) {
	hook := logTest.NewGlobal()
	mockSync := &mockSyncService{}
	cfg := Config{
		P2P:         &mockP2P{},
		SyncService: mockSync,
		BeaconDB:    &mockDB{},
		QueryService: &mockQueryService{
			isSynced: false,
		},
		SyncPollingInterval: 1,
	}
	ss := NewInitialSyncService(context.Background(), cfg)

	ss.Start()
	ss.Stop()

	testutil.AssertLogsDoNotContain(t, hook, "Chain fully synced, exiting initial sync")
	testutil.AssertLogsContain(t, hook, "Stopping service")

	hook.Reset()
}

func TestRequestBlocksBySlot(t *testing.T) {
	hook := logTest.NewGlobal()
	cfg := Config{
		P2P:             &mockP2P{},
		SyncService:     &mockSyncService{},
		BeaconDB:        &mockDB{},
		BlockBufferSize: 100,
	}
	ss := NewInitialSyncService(context.Background(), cfg)

	exitRoutine := make(chan bool)
	delayChan := make(chan time.Time)

	defer func() {
		close(exitRoutine)
		close(delayChan)
	}()

	go func() {
		ss.run(delayChan)
		exitRoutine <- true
	}()

	genericHash := make([]byte, 32)
	genericHash[0] = 'a'

	getBlockResponseMsg := func(Slot uint64) (p2p.Message, [32]byte) {

		block := &pb.BeaconBlock{
			PowChainRef:           []byte{1, 2, 3},
			AncestorHashes:        [][]byte{genericHash},
			Slot:                  Slot,
			CrystallizedStateRoot: nil,
		}

		blockResponse := &pb.BeaconBlockResponse{
			Block: block,
		}

		hash, err := types.NewBlock(block).Hash()
		if err != nil {
			t.Fatalf("unable to hash block %v", err)
		}

		return p2p.Message{
			Peer: p2p.Peer{},
			Data: blockResponse,
		}, hash
	}

	// sending all blocks except for the initial block
	for i := uint64(2); i < 10; i++ {
		response, _ := getBlockResponseMsg(i)
		ss.blockBuf <- response
	}

	initialResponse, _ := getBlockResponseMsg(1)

	//sending initial block
	ss.blockBuf <- initialResponse

	_, hash := getBlockResponseMsg(9)

	expString := fmt.Sprintf("Saved block with hash %#x and slot %d for initial sync", hash, 9)

	// waiting for the current slot to come up to the
	// expected one.
	testutil.WaitForLog(t, hook, expString)

	delayChan <- time.Time{}

	ss.cancel()
	<-exitRoutine

	testutil.AssertLogsContain(t, hook, "Exiting initial sync and starting normal sync")

	hook.Reset()
}

func TestRequestBatchedBlocks(t *testing.T) {
	hook := logTest.NewGlobal()
	cfg := Config{
		P2P:             &mockP2P{},
		SyncService:     &mockSyncService{},
		BeaconDB:        &mockDB{},
		BlockBufferSize: 100,
	}
	ss := NewInitialSyncService(context.Background(), cfg)

	exitRoutine := make(chan bool)
	delayChan := make(chan time.Time)

	defer func() {
		close(exitRoutine)
		close(delayChan)
	}()

	go func() {
		ss.run(delayChan)
		exitRoutine <- true
	}()

	genericHash := make([]byte, 32)
	genericHash[0] = 'a'

	getBlockResponse := func(Slot uint64) (*pb.BeaconBlockResponse, [32]byte) {

		block := &pb.BeaconBlock{
			PowChainRef:           []byte{1, 2, 3},
			AncestorHashes:        [][]byte{genericHash},
			Slot:                  Slot,
			CrystallizedStateRoot: nil,
		}

		blockResponse := &pb.BeaconBlockResponse{
			Block: block,
		}

		hash, err := types.NewBlock(block).Hash()
		if err != nil {
			t.Fatalf("unable to hash block %v", err)
		}

		return blockResponse, hash
	}

	for i := ss.currentSlot + 1; i <= 10; i++ {
		response, _ := getBlockResponse(i)
		ss.inMemoryBlocks[i] = response.Block
	}

	ss.requestBatchedBlocks(10)

	_, hash := getBlockResponse(10)
	expString := fmt.Sprintf("Saved block with hash %#x and slot %d for initial sync", hash, 10)

	// waiting for the current slot to come up to the
	// expected one.

	testutil.WaitForLog(t, hook, expString)

	delayChan <- time.Time{}

	ss.cancel()
	<-exitRoutine

	testutil.AssertLogsContain(t, hook, "Exiting initial sync and starting normal sync")

	hook.Reset()
}
