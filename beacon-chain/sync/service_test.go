package sync

import (
	"context"
	"testing"

	"github.com/ethereum/go-ethereum/event"
	"github.com/prysmaticlabs/prysm/beacon-chain/types"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	"github.com/prysmaticlabs/prysm/shared/p2p"
	"github.com/prysmaticlabs/prysm/shared/testutil"
	logTest "github.com/sirupsen/logrus/hooks/test"
	"golang.org/x/crypto/blake2b"
)

type mockP2P struct{}

func (mp *mockP2P) Subscribe(msg interface{}, channel interface{}) event.Subscription {
	return new(event.Feed).Subscribe(channel)
}

func (mp *mockP2P) Broadcast(msg interface{}) {}

func (mp *mockP2P) Send(msg interface{}, peer p2p.Peer) {}

type mockChainService struct {
	processedBlockHashes        [][32]byte
	processedCrystallizedHashes [][32]byte
	processedActiveHashes       [][32]byte
}

func (ms *mockChainService) ProcessBlock(b *types.Block) error {
	h, err := b.Hash()
	if err != nil {
		return err
	}

	if ms.processedBlockHashes == nil {
		ms.processedBlockHashes = [][32]byte{}
	}
	ms.processedBlockHashes = append(ms.processedBlockHashes, h)
	return nil
}

func (ms *mockChainService) ContainsBlock(h [32]byte) bool {
	for _, h1 := range ms.processedBlockHashes {
		if h == h1 {
			return true
		}
	}
	return false
}

func (ms *mockChainService) ProcessedBlockHashes() [][32]byte {
	return ms.processedBlockHashes
}

func (ms *mockChainService) ProcessActiveState(a *types.ActiveState) error {
	h, err := a.Hash()
	if err != nil {
		return err
	}

	if ms.processedActiveHashes == nil {
		ms.processedActiveHashes = [][32]byte{}
	}
	ms.processedActiveHashes = append(ms.processedActiveHashes, h)
	return nil
}

func (ms *mockChainService) ContainsActiveState(h [32]byte) bool {
	for _, h1 := range ms.processedActiveHashes {
		if h == h1 {
			return true
		}
	}
	return false
}

func (ms *mockChainService) ProcessedActiveStateHashes() [][32]byte {
	return ms.processedActiveHashes
}

func (ms *mockChainService) ProcessCrystallizedState(c *types.CrystallizedState) error {
	h, err := c.Hash()
	if err != nil {
		return err
	}

	if ms.processedCrystallizedHashes == nil {
		ms.processedCrystallizedHashes = [][32]byte{}
	}
	ms.processedCrystallizedHashes = append(ms.processedCrystallizedHashes, h)
	return nil
}

func (ms *mockChainService) ContainsCrystallizedState(h [32]byte) bool {
	for _, h1 := range ms.processedCrystallizedHashes {
		if h == h1 {
			return true
		}
	}
	return false
}

func (ms *mockChainService) ProcessedCrystallizedStateHashes() [][32]byte {
	return ms.processedCrystallizedHashes
}

func TestProcessBlockHash(t *testing.T) {
	hook := logTest.NewGlobal()

	// set the channel's buffer to 0 to make channel interactions blocking
	cfg := Config{BlockHashBufferSize: 0, BlockBufferSize: 0}
	ss := NewSyncService(context.Background(), cfg, &mockP2P{}, &mockChainService{})

	exitRoutine := make(chan bool)

	go func() {
		ss.run(ss.ctx.Done())
		exitRoutine <- true
	}()

	announceHash := blake2b.Sum256([]byte{})
	hashAnnounce := &pb.BeaconBlockHashAnnounce{
		Hash: announceHash[:],
	}

	msg := p2p.Message{
		Peer: p2p.Peer{},
		Data: hashAnnounce,
	}

	// if a new hash is processed
	ss.announceBlockHashBuf <- msg

	ss.cancel()
	<-exitRoutine

	testutil.AssertLogsContain(t, hook, "requesting full block data from sender")
	hook.Reset()
}

func TestProcessBlock(t *testing.T) {
	hook := logTest.NewGlobal()

	cfg := Config{BlockHashBufferSize: 0, BlockBufferSize: 0}
	ms := &mockChainService{}
	ss := NewSyncService(context.Background(), cfg, &mockP2P{}, ms)

	exitRoutine := make(chan bool)

	go func() {
		ss.run(ss.ctx.Done())
		exitRoutine <- true
	}()

	data := &pb.BeaconBlock{
		MainChainRef: []byte{1, 2, 3, 4, 5},
		ParentHash:   make([]byte, 32),
	}

	responseBlock := &pb.BeaconBlockResponse{
		Block: data,
	}

	msg := p2p.Message{
		Peer: p2p.Peer{},
		Data: responseBlock,
	}

	ss.blockBuf <- msg
	ss.cancel()
	<-exitRoutine

	block, err := types.NewBlock(data)
	if err != nil {
		t.Fatalf("Could not instantiate new block from proto: %v", err)
	}
	h, err := block.Hash()
	if err != nil {
		t.Fatal(err)
	}

	if ms.processedBlockHashes[0] != h {
		t.Errorf("Expected processed hash to be equal to block hash. wanted=%x, got=%x", h, ms.processedBlockHashes[0])
	}
	hook.Reset()
}

func TestProcessMultipleBlocks(t *testing.T) {
	hook := logTest.NewGlobal()

	cfg := Config{BlockHashBufferSize: 0, BlockBufferSize: 0}
	ms := &mockChainService{}
	ss := NewSyncService(context.Background(), cfg, &mockP2P{}, ms)

	exitRoutine := make(chan bool)

	go func() {
		ss.run(ss.ctx.Done())
		exitRoutine <- true
	}()

	data1 := &pb.BeaconBlock{
		MainChainRef: []byte{1, 2, 3, 4, 5},
		ParentHash:   make([]byte, 32),
	}

	responseBlock1 := &pb.BeaconBlockResponse{
		Block: data1,
	}

	msg1 := p2p.Message{
		Peer: p2p.Peer{},
		Data: responseBlock1,
	}

	data2 := &pb.BeaconBlock{
		MainChainRef: []byte{6, 7, 8, 9, 10},
		ParentHash:   make([]byte, 32),
	}

	responseBlock2 := &pb.BeaconBlockResponse{
		Block: data2,
	}

	msg2 := p2p.Message{
		Peer: p2p.Peer{},
		Data: responseBlock2,
	}

	ss.blockBuf <- msg1
	ss.blockBuf <- msg2
	ss.cancel()
	<-exitRoutine

	block1, err := types.NewBlock(data1)
	if err != nil {
		t.Fatalf("Could not instantiate new block from proto: %v", err)
	}
	h1, err := block1.Hash()
	if err != nil {
		t.Fatal(err)
	}

	block2, err := types.NewBlock(data2)
	if err != nil {
		t.Fatalf("Could not instantiate new block from proto: %v", err)
	}
	h2, err := block2.Hash()
	if err != nil {
		t.Fatal(err)
	}

	// Sync service broadcasts the two separate blocks
	// and forwards them to to the local chain.
	if ms.processedBlockHashes[0] != h1 {
		t.Errorf("Expected processed hash to be equal to block hash. wanted=%x, got=%x", h1, ms.processedBlockHashes[0])
	}
	if ms.processedBlockHashes[1] != h2 {
		t.Errorf("Expected processed hash to be equal to block hash. wanted=%x, got=%x", h2, ms.processedBlockHashes[1])
	}
	hook.Reset()
}

func TestProcessSameBlock(t *testing.T) {
	hook := logTest.NewGlobal()

	cfg := Config{BlockHashBufferSize: 0, BlockBufferSize: 0}
	ms := &mockChainService{}
	ss := NewSyncService(context.Background(), cfg, &mockP2P{}, ms)

	exitRoutine := make(chan bool)

	go func() {
		ss.run(ss.ctx.Done())
		exitRoutine <- true
	}()

	data := &pb.BeaconBlock{
		MainChainRef: []byte{1, 2, 3},
		ParentHash:   make([]byte, 32),
	}

	responseBlock := &pb.BeaconBlockResponse{
		Block: data,
	}

	msg := p2p.Message{
		Peer: p2p.Peer{},
		Data: responseBlock,
	}
	ss.blockBuf <- msg
	ss.blockBuf <- msg
	ss.cancel()
	<-exitRoutine

	block, err := types.NewBlock(data)
	if err != nil {
		t.Fatalf("Could not instantiate new block from proto: %v", err)
	}
	h, err := block.Hash()
	if err != nil {
		t.Fatal(err)
	}

	// Sync service broadcasts the two separate blocks
	// and forwards them to to the local chain.
	if len(ms.processedBlockHashes) > 1 {
		t.Error("Should have only processed one block, processed both instead")
	}
	if ms.processedBlockHashes[0] != h {
		t.Errorf("Expected processed hash to be equal to block hash. wanted=%x, got=%x", h, ms.processedBlockHashes[0])
	}
	hook.Reset()
}

func TestProcessCrystallizedHash(t *testing.T) {
	hook := logTest.NewGlobal()

	// set the channel's buffer to 0 to make channel interactions blocking
	cfg := Config{CrystallizedStateHashBufferSize: 0, CrystallizedStateBufferSize: 0}
	ss := NewSyncService(context.Background(), cfg, &mockP2P{}, &mockChainService{})

	exitRoutine := make(chan bool)

	go func() {
		ss.run(ss.ctx.Done())
		exitRoutine <- true
	}()

	announceHash := blake2b.Sum256([]byte{})
	hashAnnounce := &pb.CrystallizedStateHashAnnounce{
		Hash: announceHash[:],
	}

	msg := p2p.Message{
		Peer: p2p.Peer{},
		Data: hashAnnounce,
	}

	ss.announceCrystallizedHashBuf <- msg

	ss.cancel()
	<-exitRoutine
	testutil.AssertLogsContain(t, hook, "Received crystallized state hash, requesting state data from sender")

	hook.Reset()
}

func TestProcessActiveHash(t *testing.T) {
	hook := logTest.NewGlobal()

	// set the channel's buffer to 0 to make channel interactions blocking
	cfg := Config{ActiveStateHashBufferSize: 0, ActiveStateBufferSize: 0}
	ss := NewSyncService(context.Background(), cfg, &mockP2P{}, &mockChainService{})

	exitRoutine := make(chan bool)

	go func() {
		ss.run(ss.ctx.Done())
		exitRoutine <- true
	}()

	announceHash := blake2b.Sum256([]byte{})
	hashAnnounce := &pb.ActiveStateHashAnnounce{
		Hash: announceHash[:],
	}

	msg := p2p.Message{
		Peer: p2p.Peer{},
		Data: hashAnnounce,
	}

	ss.announceActiveHashBuf <- msg

	ss.cancel()
	<-exitRoutine
	testutil.AssertLogsContain(t, hook, "Received active state hash, requesting state data from sender")

	hook.Reset()
}

func TestProcessBadCrystallizedHash(t *testing.T) {
	hook := logTest.NewGlobal()

	// set the channel's buffer to 0 to make channel interactions blocking
	cfg := Config{CrystallizedStateHashBufferSize: 0, CrystallizedStateBufferSize: 0}
	ss := NewSyncService(context.Background(), cfg, &mockP2P{}, &mockChainService{})

	exitRoutine := make(chan bool)

	go func() {
		ss.run(ss.ctx.Done())
		exitRoutine <- true
	}()

	// Send blockHashAnnounce msg format to crystallized state channel. Should fail
	announceHash := blake2b.Sum256([]byte{})
	hashAnnounce := &pb.BeaconBlockHashAnnounce{
		Hash: announceHash[:],
	}

	msg := p2p.Message{
		Peer: p2p.Peer{},
		Data: hashAnnounce,
	}

	ss.announceCrystallizedHashBuf <- msg

	ss.cancel()
	<-exitRoutine
	testutil.AssertLogsContain(t, hook, "Received malformed crystallized state hash announcement p2p message")

	hook.Reset()
}

func TestProcessBadActiveHash(t *testing.T) {
	hook := logTest.NewGlobal()

	// set the channel's buffer to 0 to make channel interactions blocking
	cfg := Config{ActiveStateHashBufferSize: 0, ActiveStateBufferSize: 0}
	ss := NewSyncService(context.Background(), cfg, &mockP2P{}, &mockChainService{})

	exitRoutine := make(chan bool)

	go func() {
		ss.run(ss.ctx.Done())
		exitRoutine <- true
	}()

	// Send blockHashAnnounce msg format to active state channel. Should fail
	announceHash := blake2b.Sum256([]byte{})
	hashAnnounce := &pb.BeaconBlockHashAnnounce{
		Hash: announceHash[:],
	}

	msg := p2p.Message{
		Peer: p2p.Peer{},
		Data: hashAnnounce,
	}

	ss.announceActiveHashBuf <- msg

	ss.cancel()
	<-exitRoutine
	testutil.AssertLogsContain(t, hook, "Received malformed active state hash announcement p2p message")

	hook.Reset()
}

func TestProcessCrystallizedStates(t *testing.T) {
	hook := logTest.NewGlobal()

	cfg := Config{CrystallizedStateHashBufferSize: 0, CrystallizedStateBufferSize: 0}
	ms := &mockChainService{}
	ss := NewSyncService(context.Background(), cfg, &mockP2P{}, ms)

	exitRoutine := make(chan bool)

	go func() {
		ss.run(ss.ctx.Done())
		exitRoutine <- true
	}()

	data1 := &pb.CrystallizedState{
		LastJustifiedEpoch: 100,
		LastFinalizedEpoch: 99,
	}
	data2 := &pb.CrystallizedState{
		LastJustifiedEpoch: 100,
		LastFinalizedEpoch: 98,
	}

	responseState1 := &pb.CrystallizedStateResponse{
		CrystallizedState: data1,
	}
	responseState2 := &pb.CrystallizedStateResponse{
		CrystallizedState: data2,
	}

	msg1 := p2p.Message{
		Peer: p2p.Peer{},
		Data: responseState1,
	}
	msg2 := p2p.Message{
		Peer: p2p.Peer{},
		Data: responseState2,
	}

	ss.crystallizedStateBuf <- msg1
	ss.crystallizedStateBuf <- msg2
	ss.cancel()
	<-exitRoutine

	state1 := types.NewCrystallizedState(data1)
	state2 := types.NewCrystallizedState(data2)

	h, err := state1.Hash()
	if err != nil {
		t.Fatal(err)
	}
	if ms.processedCrystallizedHashes[0] != h {
		t.Errorf("Expected processed hash to be equal to state hash. wanted=%x, got=%x", h, ms.processedCrystallizedHashes[0])
	}

	h, err = state2.Hash()
	if err != nil {
		t.Fatal(err)
	}
	if ms.processedCrystallizedHashes[1] != h {
		t.Errorf("Expected processed hash to be equal to state hash. wanted=%x, got=%x", h, ms.processedCrystallizedHashes[1])
	}

	hook.Reset()
}

func TestProcessActiveStates(t *testing.T) {
	hook := logTest.NewGlobal()

	cfg := Config{ActiveStateHashBufferSize: 0, ActiveStateBufferSize: 0}
	ms := &mockChainService{}
	ss := NewSyncService(context.Background(), cfg, &mockP2P{}, ms)

	exitRoutine := make(chan bool)

	go func() {
		ss.run(ss.ctx.Done())
		exitRoutine <- true
	}()

	state1 := &pb.ActiveState{
		TotalAttesterDeposits: 10000,
	}
	state2 := &pb.ActiveState{
		TotalAttesterDeposits: 10001,
	}

	responseState1 := &pb.ActiveStateResponse{
		ActiveState: state1,
	}
	responseState2 := &pb.ActiveStateResponse{
		ActiveState: state2,
	}

	msg1 := p2p.Message{
		Peer: p2p.Peer{},
		Data: responseState1,
	}
	msg2 := p2p.Message{
		Peer: p2p.Peer{},
		Data: responseState2,
	}

	ss.activeStateBuf <- msg1
	ss.activeStateBuf <- msg2
	ss.cancel()
	<-exitRoutine

	state := types.NewActiveState(state1)
	h, err := state.Hash()
	if err != nil {
		t.Fatal(err)
	}

	if ms.processedActiveHashes[0] != h {
		t.Errorf("Expected processed hash to be equal to state hash. wanted=%x, got=%x", h, ms.processedActiveHashes[0])
	}

	state = types.NewActiveState(state2)
	h, err = state.Hash()
	if err != nil {
		t.Fatal(err)
	}

	if ms.processedActiveHashes[1] != h {
		t.Errorf("Expected processed hash to be equal to state hash. wanted=%x, got=%x", h, ms.processedActiveHashes[1])
	}

	hook.Reset()
}

func TestProcessSameCrystallizedState(t *testing.T) {
	hook := logTest.NewGlobal()

	cfg := Config{CrystallizedStateHashBufferSize: 0, CrystallizedStateBufferSize: 0}
	ms := &mockChainService{}
	ss := NewSyncService(context.Background(), cfg, &mockP2P{}, ms)

	exitRoutine := make(chan bool)

	go func() {
		ss.run(ss.ctx.Done())
		exitRoutine <- true
	}()

	data := &pb.CrystallizedState{
		LastJustifiedEpoch: 100,
		LastFinalizedEpoch: 99,
	}

	responseState := &pb.CrystallizedStateResponse{
		CrystallizedState: data,
	}

	msg1 := p2p.Message{
		Peer: p2p.Peer{},
		Data: responseState,
	}
	msg2 := p2p.Message{
		Peer: p2p.Peer{},
		Data: responseState,
	}

	ss.crystallizedStateBuf <- msg1
	ss.crystallizedStateBuf <- msg2
	ss.cancel()
	<-exitRoutine

	state := types.NewCrystallizedState(data)

	h, err := state.Hash()
	if err != nil {
		t.Fatal(err)
	}
	if len(ms.processedCrystallizedHashes) > 1 {
		t.Errorf("Processed more hash than it was received. Got=%x", len(ms.processedCrystallizedHashes))
	}
	if ms.processedCrystallizedHashes[0] != h {
		t.Errorf("Expected processed hash to be equal to state hash. wanted=%x, got=%x", h, ms.processedCrystallizedHashes[0])
	}

	hook.Reset()
}

func TestProcessSameActiveState(t *testing.T) {
	hook := logTest.NewGlobal()

	cfg := Config{ActiveStateHashBufferSize: 0, ActiveStateBufferSize: 0}
	ms := &mockChainService{}
	ss := NewSyncService(context.Background(), cfg, &mockP2P{}, ms)

	exitRoutine := make(chan bool)

	go func() {
		ss.run(ss.ctx.Done())
		exitRoutine <- true
	}()

	data := &pb.ActiveState{
		TotalAttesterDeposits: 100,
	}

	responseState1 := &pb.ActiveStateResponse{
		ActiveState: data,
	}

	msg1 := p2p.Message{
		Peer: p2p.Peer{},
		Data: responseState1,
	}
	msg2 := p2p.Message{
		Peer: p2p.Peer{},
		Data: responseState1,
	}

	ss.activeStateBuf <- msg1
	ss.activeStateBuf <- msg2
	ss.cancel()
	<-exitRoutine

	state := types.NewActiveState(data)

	h, err := state.Hash()
	if err != nil {
		t.Fatal(err)
	}
	if len(ms.processedActiveHashes) > 1 {
		t.Errorf("Processed more hash than it was received. Got=%x", len(ms.processedActiveHashes))
	}
	if ms.processedActiveHashes[0] != h {
		t.Errorf("Expected processed hash to be equal to state hash. wanted=%x, got=%x", h, ms.processedActiveHashes[0])
	}

	hook.Reset()
}
