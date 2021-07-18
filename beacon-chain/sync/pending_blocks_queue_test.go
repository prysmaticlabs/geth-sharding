package sync

import (
	"context"
	"math"
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/p2p/enr"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/protocol"
	gcache "github.com/patrickmn/go-cache"
	types "github.com/prysmaticlabs/eth2-types"
	mock "github.com/prysmaticlabs/prysm/beacon-chain/blockchain/testing"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	dbtest "github.com/prysmaticlabs/prysm/beacon-chain/db/testing"
	"github.com/prysmaticlabs/prysm/beacon-chain/p2p/peers"
	p2ptest "github.com/prysmaticlabs/prysm/beacon-chain/p2p/testing"
	p2ptypes "github.com/prysmaticlabs/prysm/beacon-chain/p2p/types"
	"github.com/prysmaticlabs/prysm/beacon-chain/state/stategen"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	ethpb "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/proto/eth/v1alpha1/wrapper"
	"github.com/prysmaticlabs/prysm/shared/copyutil"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/rand"
	"github.com/prysmaticlabs/prysm/shared/testutil"
	"github.com/prysmaticlabs/prysm/shared/testutil/assert"
	"github.com/prysmaticlabs/prysm/shared/testutil/require"
)

//    /- b1 - b2
// b0
//    \- b3
// Test b1 was missing then received and we can process b0 -> b1 -> b2
func TestRegularSyncBeaconBlockSubscriber_ProcessPendingBlocks1(t *testing.T) {
	db := dbtest.SetupDB(t)

	p1 := p2ptest.NewTestP2P(t)
	r := &Service{
		cfg: &Config{
			P2P: p1,
			DB:  db,
			Chain: &mock.ChainService{
				FinalizedCheckPoint: &ethpb.Checkpoint{
					Epoch: 0,
				},
			},
			StateGen: stategen.New(db),
		},
		slotToPendingBlocks: gcache.New(time.Second, 2*time.Second),
		seenPendingBlocks:   make(map[[32]byte]bool),
	}
	err := r.initCaches()
	require.NoError(t, err)

	b0 := testutil.NewBeaconBlock()
	require.NoError(t, r.cfg.DB.SaveBlock(context.Background(), wrapper.WrappedPhase0SignedBeaconBlock(b0)))
	b0Root, err := b0.Block.HashTreeRoot()
	require.NoError(t, err)
	b3 := testutil.NewBeaconBlock()
	b3.Block.Slot = 3
	b3.Block.ParentRoot = b0Root[:]
	require.NoError(t, r.cfg.DB.SaveBlock(context.Background(), wrapper.WrappedPhase0SignedBeaconBlock(b3)))
	// Incomplete block link
	b1 := testutil.NewBeaconBlock()
	b1.Block.Slot = 1
	b1.Block.ParentRoot = b0Root[:]
	b1Root, err := b1.Block.HashTreeRoot()
	require.NoError(t, err)
	b2 := testutil.NewBeaconBlock()
	b2.Block.Slot = 2
	b2.Block.ParentRoot = b1Root[:]
	b2Root, err := b1.Block.HashTreeRoot()
	require.NoError(t, err)

	// Add b2 to the cache
	require.NoError(t, r.insertBlockToPendingQueue(b2.Block.Slot, wrapper.WrappedPhase0SignedBeaconBlock(b2), b2Root))

	require.NoError(t, r.processPendingBlocks(context.Background()))
	assert.Equal(t, 1, len(r.slotToPendingBlocks.Items()), "Incorrect size for slot to pending blocks cache")
	assert.Equal(t, 1, len(r.seenPendingBlocks), "Incorrect size for seen pending block")

	// Add b1 to the cache
	require.NoError(t, r.insertBlockToPendingQueue(b1.Block.Slot, wrapper.WrappedPhase0SignedBeaconBlock(b1), b1Root))
	require.NoError(t, r.cfg.DB.SaveBlock(context.Background(), wrapper.WrappedPhase0SignedBeaconBlock(b1)))

	// Insert bad b1 in the cache to verify the good one doesn't get replaced.
	require.NoError(t, r.insertBlockToPendingQueue(b1.Block.Slot, wrapper.WrappedPhase0SignedBeaconBlock(testutil.NewBeaconBlock()), [32]byte{}))

	require.NoError(t, r.processPendingBlocks(context.Background())) // Marks a block as bad
	require.NoError(t, r.processPendingBlocks(context.Background())) // Bad block removed on second run

	assert.Equal(t, 1, len(r.slotToPendingBlocks.Items()), "Incorrect size for slot to pending blocks cache")
	assert.Equal(t, 2, len(r.seenPendingBlocks), "Incorrect size for seen pending block")
}

func TestRegularSync_InsertDuplicateBlocks(t *testing.T) {
	db := dbtest.SetupDB(t)

	p1 := p2ptest.NewTestP2P(t)
	r := &Service{
		cfg: &Config{
			P2P: p1,
			DB:  db,
			Chain: &mock.ChainService{
				FinalizedCheckPoint: &ethpb.Checkpoint{
					Epoch: 0,
					Root:  make([]byte, 32),
				},
			},
		},
		slotToPendingBlocks: gcache.New(time.Second, 2*time.Second),
		seenPendingBlocks:   make(map[[32]byte]bool),
	}
	err := r.initCaches()
	require.NoError(t, err)

	b0 := testutil.NewBeaconBlock()
	b0r := [32]byte{'a'}
	require.NoError(t, r.cfg.DB.SaveBlock(context.Background(), wrapper.WrappedPhase0SignedBeaconBlock(b0)))
	b0Root, err := b0.Block.HashTreeRoot()
	require.NoError(t, err)
	b1 := testutil.NewBeaconBlock()
	b1.Block.Slot = 1
	b1.Block.ParentRoot = b0Root[:]
	b1r := [32]byte{'b'}

	require.NoError(t, r.insertBlockToPendingQueue(b0.Block.Slot, wrapper.WrappedPhase0SignedBeaconBlock(b0), b0r))
	require.Equal(t, 1, len(r.pendingBlocksInCache(b0.Block.Slot)), "Block was not added to map")

	require.NoError(t, r.insertBlockToPendingQueue(b1.Block.Slot, wrapper.WrappedPhase0SignedBeaconBlock(b1), b1r))
	require.Equal(t, 1, len(r.pendingBlocksInCache(b1.Block.Slot)), "Block was not added to map")

	// Add duplicate block which should not be saved.
	require.NoError(t, r.insertBlockToPendingQueue(b0.Block.Slot, wrapper.WrappedPhase0SignedBeaconBlock(b0), b0r))
	require.Equal(t, 1, len(r.pendingBlocksInCache(b0.Block.Slot)), "Block was added to map")

	// Add duplicate block which should not be saved.
	require.NoError(t, r.insertBlockToPendingQueue(b1.Block.Slot, wrapper.WrappedPhase0SignedBeaconBlock(b1), b1r))
	require.Equal(t, 1, len(r.pendingBlocksInCache(b1.Block.Slot)), "Block was added to map")

}

//    /- b1 - b2 - b5
// b0
//    \- b3 - b4
// Test b2 and b3 were missed, after receiving them we can process 2 chains.
func TestRegularSyncBeaconBlockSubscriber_ProcessPendingBlocks_2Chains(t *testing.T) {
	db := dbtest.SetupDB(t)
	p1 := p2ptest.NewTestP2P(t)
	p2 := p2ptest.NewTestP2P(t)
	p1.Connect(p2)
	assert.Equal(t, 1, len(p1.BHost.Network().Peers()), "Expected peers to be connected")
	pcl := protocol.ID("/eth2/beacon_chain/req/hello/1/ssz_snappy")
	var wg sync.WaitGroup
	wg.Add(1)
	p2.BHost.SetStreamHandler(pcl, func(stream network.Stream) {
		defer wg.Done()
		code, errMsg, err := ReadStatusCode(stream, p1.Encoding())
		assert.NoError(t, err)
		if code == 0 {
			t.Error("Expected a non-zero code")
		}
		if errMsg != p2ptypes.ErrWrongForkDigestVersion.Error() {
			t.Logf("Received error string len %d, wanted error string len %d", len(errMsg), len(p2ptypes.ErrWrongForkDigestVersion.Error()))
			t.Errorf("Received unexpected message response in the stream: %s. Wanted %s.", errMsg, p2ptypes.ErrWrongForkDigestVersion.Error())
		}
	})

	r := &Service{
		cfg: &Config{
			P2P: p1,
			DB:  db,
			Chain: &mock.ChainService{
				FinalizedCheckPoint: &ethpb.Checkpoint{
					Epoch: 0,
					Root:  make([]byte, 32),
				},
			},
			StateGen: stategen.New(db),
		},
		slotToPendingBlocks: gcache.New(time.Second, 2*time.Second),
		seenPendingBlocks:   make(map[[32]byte]bool),
	}
	err := r.initCaches()
	require.NoError(t, err)
	p1.Peers().Add(new(enr.Record), p2.PeerID(), nil, network.DirOutbound)
	p1.Peers().SetConnectionState(p2.PeerID(), peers.PeerConnected)
	p1.Peers().SetChainState(p2.PeerID(), &pb.Status{})

	b0 := testutil.NewBeaconBlock()
	require.NoError(t, r.cfg.DB.SaveBlock(context.Background(), wrapper.WrappedPhase0SignedBeaconBlock(b0)))
	b0Root, err := b0.Block.HashTreeRoot()
	require.NoError(t, err)
	b1 := testutil.NewBeaconBlock()
	b1.Block.Slot = 1
	b1.Block.ParentRoot = b0Root[:]
	require.NoError(t, r.cfg.DB.SaveBlock(context.Background(), wrapper.WrappedPhase0SignedBeaconBlock(b1)))
	b1Root, err := b1.Block.HashTreeRoot()
	require.NoError(t, err)

	// Incomplete block links
	b2 := testutil.NewBeaconBlock()
	b2.Block.Slot = 2
	b2.Block.ParentRoot = b1Root[:]
	b2Root, err := b2.Block.HashTreeRoot()
	require.NoError(t, err)
	b5 := testutil.NewBeaconBlock()
	b5.Block.Slot = 5
	b5.Block.ParentRoot = b2Root[:]
	b5Root, err := b5.Block.HashTreeRoot()
	require.NoError(t, err)
	b3 := testutil.NewBeaconBlock()
	b3.Block.Slot = 3
	b3.Block.ParentRoot = b0Root[:]
	b3Root, err := b3.Block.HashTreeRoot()
	require.NoError(t, err)
	b4 := testutil.NewBeaconBlock()
	b4.Block.Slot = 4
	b4.Block.ParentRoot = b3Root[:]
	b4Root, err := b4.Block.HashTreeRoot()
	require.NoError(t, err)

	require.NoError(t, r.insertBlockToPendingQueue(b4.Block.Slot, wrapper.WrappedPhase0SignedBeaconBlock(b4), b4Root))
	require.NoError(t, r.insertBlockToPendingQueue(b5.Block.Slot, wrapper.WrappedPhase0SignedBeaconBlock(b5), b5Root))

	require.NoError(t, r.processPendingBlocks(context.Background())) // Marks a block as bad
	require.NoError(t, r.processPendingBlocks(context.Background())) // Bad block removed on second run

	assert.Equal(t, 2, len(r.slotToPendingBlocks.Items()), "Incorrect size for slot to pending blocks cache")
	assert.Equal(t, 2, len(r.seenPendingBlocks), "Incorrect size for seen pending block")

	// Add b3 to the cache
	require.NoError(t, r.insertBlockToPendingQueue(b3.Block.Slot, wrapper.WrappedPhase0SignedBeaconBlock(b3), b3Root))
	require.NoError(t, r.cfg.DB.SaveBlock(context.Background(), wrapper.WrappedPhase0SignedBeaconBlock(b3)))

	require.NoError(t, r.processPendingBlocks(context.Background())) // Marks a block as bad
	require.NoError(t, r.processPendingBlocks(context.Background())) // Bad block removed on second run

	assert.Equal(t, 1, len(r.slotToPendingBlocks.Items()), "Incorrect size for slot to pending blocks cache")
	assert.Equal(t, 3, len(r.seenPendingBlocks), "Incorrect size for seen pending block")

	// Add b2 to the cache
	require.NoError(t, r.insertBlockToPendingQueue(b2.Block.Slot, wrapper.WrappedPhase0SignedBeaconBlock(b2), b2Root))

	require.NoError(t, r.cfg.DB.SaveBlock(context.Background(), wrapper.WrappedPhase0SignedBeaconBlock(b2)))

	require.NoError(t, r.processPendingBlocks(context.Background())) // Marks a block as bad
	require.NoError(t, r.processPendingBlocks(context.Background())) // Bad block removed on second run

	assert.Equal(t, 0, len(r.slotToPendingBlocks.Items()), "Incorrect size for slot to pending blocks cache")
	assert.Equal(t, 4, len(r.seenPendingBlocks), "Incorrect size for seen pending block")
}

func TestRegularSyncBeaconBlockSubscriber_PruneOldPendingBlocks(t *testing.T) {
	db := dbtest.SetupDB(t)
	p1 := p2ptest.NewTestP2P(t)
	p2 := p2ptest.NewTestP2P(t)
	p1.Connect(p2)
	assert.Equal(t, 1, len(p1.BHost.Network().Peers()), "Expected peers to be connected")

	r := &Service{
		cfg: &Config{
			P2P: p1,
			DB:  db,
			Chain: &mock.ChainService{
				FinalizedCheckPoint: &ethpb.Checkpoint{
					Epoch: 1,
					Root:  make([]byte, 32),
				},
			},
		},
		slotToPendingBlocks: gcache.New(time.Second, 2*time.Second),
		seenPendingBlocks:   make(map[[32]byte]bool),
	}
	err := r.initCaches()
	require.NoError(t, err)
	p1.Peers().Add(new(enr.Record), p1.PeerID(), nil, network.DirOutbound)
	p1.Peers().SetConnectionState(p1.PeerID(), peers.PeerConnected)
	p1.Peers().SetChainState(p1.PeerID(), &pb.Status{})

	b0 := testutil.NewBeaconBlock()
	require.NoError(t, r.cfg.DB.SaveBlock(context.Background(), wrapper.WrappedPhase0SignedBeaconBlock(b0)))
	b0Root, err := b0.Block.HashTreeRoot()
	require.NoError(t, err)
	b1 := testutil.NewBeaconBlock()
	b1.Block.Slot = 1
	b1.Block.ParentRoot = b0Root[:]
	require.NoError(t, r.cfg.DB.SaveBlock(context.Background(), wrapper.WrappedPhase0SignedBeaconBlock(b1)))
	b1Root, err := b1.Block.HashTreeRoot()
	require.NoError(t, err)

	// Incomplete block links
	b2 := testutil.NewBeaconBlock()
	b2.Block.Slot = 2
	b2.Block.ParentRoot = b1Root[:]
	b2Root, err := b2.Block.HashTreeRoot()
	require.NoError(t, err)
	b5 := testutil.NewBeaconBlock()
	b5.Block.Slot = 5
	b5.Block.ParentRoot = b2Root[:]
	b5Root, err := b5.Block.HashTreeRoot()
	require.NoError(t, err)
	b3 := testutil.NewBeaconBlock()
	b3.Block.Slot = 3
	b3.Block.ParentRoot = b0Root[:]
	b3Root, err := b3.Block.HashTreeRoot()
	require.NoError(t, err)
	b4 := testutil.NewBeaconBlock()
	b4.Block.Slot = 4
	b4.Block.ParentRoot = b3Root[:]
	b4Root, err := b4.Block.HashTreeRoot()
	require.NoError(t, err)

	require.NoError(t, r.insertBlockToPendingQueue(b2.Block.Slot, wrapper.WrappedPhase0SignedBeaconBlock(b2), b2Root))
	require.NoError(t, r.insertBlockToPendingQueue(b3.Block.Slot, wrapper.WrappedPhase0SignedBeaconBlock(b3), b3Root))
	require.NoError(t, r.insertBlockToPendingQueue(b4.Block.Slot, wrapper.WrappedPhase0SignedBeaconBlock(b4), b4Root))
	require.NoError(t, r.insertBlockToPendingQueue(b5.Block.Slot, wrapper.WrappedPhase0SignedBeaconBlock(b5), b5Root))

	require.NoError(t, r.processPendingBlocks(context.Background()))
	assert.Equal(t, 0, len(r.slotToPendingBlocks.Items()), "Incorrect size for slot to pending blocks cache")
	assert.Equal(t, 4, len(r.seenPendingBlocks), "Incorrect size for seen pending block")
}

func TestService_sortedPendingSlots(t *testing.T) {
	r := &Service{
		slotToPendingBlocks: gcache.New(time.Second, 2*time.Second),
		seenPendingBlocks:   make(map[[32]byte]bool),
	}

	var lastSlot types.Slot = math.MaxUint64
	require.NoError(t, r.insertBlockToPendingQueue(lastSlot, wrapper.WrappedPhase0SignedBeaconBlock(testutil.HydrateSignedBeaconBlock(&ethpb.SignedBeaconBlock{Block: &ethpb.BeaconBlock{Slot: lastSlot}})), [32]byte{1}))
	require.NoError(t, r.insertBlockToPendingQueue(lastSlot-3, wrapper.WrappedPhase0SignedBeaconBlock(testutil.HydrateSignedBeaconBlock(&ethpb.SignedBeaconBlock{Block: &ethpb.BeaconBlock{Slot: lastSlot - 3}})), [32]byte{2}))
	require.NoError(t, r.insertBlockToPendingQueue(lastSlot-5, wrapper.WrappedPhase0SignedBeaconBlock(testutil.HydrateSignedBeaconBlock(&ethpb.SignedBeaconBlock{Block: &ethpb.BeaconBlock{Slot: lastSlot - 5}})), [32]byte{3}))
	require.NoError(t, r.insertBlockToPendingQueue(lastSlot-2, wrapper.WrappedPhase0SignedBeaconBlock(testutil.HydrateSignedBeaconBlock(&ethpb.SignedBeaconBlock{Block: &ethpb.BeaconBlock{Slot: lastSlot - 2}})), [32]byte{4}))

	want := []types.Slot{lastSlot - 5, lastSlot - 3, lastSlot - 2, lastSlot}
	assert.DeepEqual(t, want, r.sortedPendingSlots(), "Unexpected pending slots list")
}

func TestService_BatchRootRequest(t *testing.T) {
	db := dbtest.SetupDB(t)
	p1 := p2ptest.NewTestP2P(t)
	p2 := p2ptest.NewTestP2P(t)
	p1.Connect(p2)
	assert.Equal(t, 1, len(p1.BHost.Network().Peers()), "Expected peers to be connected")

	r := &Service{
		cfg: &Config{
			P2P: p1,
			DB:  db,
			Chain: &mock.ChainService{
				FinalizedCheckPoint: &ethpb.Checkpoint{
					Epoch: 1,
					Root:  make([]byte, 32),
				},
				ValidatorsRoot: [32]byte{},
				Genesis:        time.Now(),
			},
		},
		slotToPendingBlocks: gcache.New(time.Second, 2*time.Second),
		seenPendingBlocks:   make(map[[32]byte]bool),
	}

	err := r.initCaches()
	require.NoError(t, err)
	p1.Peers().Add(new(enr.Record), p2.PeerID(), nil, network.DirOutbound)
	p1.Peers().SetConnectionState(p2.PeerID(), peers.PeerConnected)
	p1.Peers().SetChainState(p2.PeerID(), &pb.Status{FinalizedEpoch: 2})

	b0 := testutil.NewBeaconBlock()
	require.NoError(t, r.cfg.DB.SaveBlock(context.Background(), wrapper.WrappedPhase0SignedBeaconBlock(b0)))
	b0Root, err := b0.Block.HashTreeRoot()
	require.NoError(t, err)
	b1 := testutil.NewBeaconBlock()
	b1.Block.Slot = 1
	b1.Block.ParentRoot = b0Root[:]
	require.NoError(t, r.cfg.DB.SaveBlock(context.Background(), wrapper.WrappedPhase0SignedBeaconBlock(b1)))
	b1Root, err := b1.Block.HashTreeRoot()
	require.NoError(t, err)

	b2 := testutil.NewBeaconBlock()
	b2.Block.Slot = 2
	b2.Block.ParentRoot = b1Root[:]
	b2Root, err := b2.Block.HashTreeRoot()
	require.NoError(t, err)
	b5 := testutil.NewBeaconBlock()
	b5.Block.Slot = 5
	b5.Block.ParentRoot = b2Root[:]
	b5Root, err := b5.Block.HashTreeRoot()
	require.NoError(t, err)
	b3 := testutil.NewBeaconBlock()
	b3.Block.Slot = 3
	b3.Block.ParentRoot = b0Root[:]
	b3Root, err := b3.Block.HashTreeRoot()
	require.NoError(t, err)
	b4 := testutil.NewBeaconBlock()
	b4.Block.Slot = 4
	b4.Block.ParentRoot = b3Root[:]
	b4Root, err := b4.Block.HashTreeRoot()
	require.NoError(t, err)

	// Send in duplicated roots to also test deduplicaton.
	sentRoots := p2ptypes.BeaconBlockByRootsReq{b2Root, b2Root, b3Root, b3Root, b4Root, b5Root}
	expectedRoots := p2ptypes.BeaconBlockByRootsReq{b2Root, b3Root, b4Root, b5Root}

	pcl := protocol.ID("/eth2/beacon_chain/req/beacon_blocks_by_root/1/ssz_snappy")
	var wg sync.WaitGroup
	wg.Add(1)
	p2.BHost.SetStreamHandler(pcl, func(stream network.Stream) {
		defer wg.Done()
		var out p2ptypes.BeaconBlockByRootsReq
		assert.NoError(t, p2.Encoding().DecodeWithMaxLength(stream, &out))
		assert.DeepEqual(t, expectedRoots, out, "Did not receive expected message")
		response := []*ethpb.SignedBeaconBlock{b2, b3, b4, b5}
		for _, blk := range response {
			_, err := stream.Write([]byte{responseCodeSuccess})
			assert.NoError(t, err, "Could not write to stream")
			_, err = p2.Encoding().EncodeWithMaxLength(stream, blk)
			assert.NoError(t, err, "Could not send response back")
		}
		assert.NoError(t, stream.Close())
	})

	require.NoError(t, r.sendBatchRootRequest(context.Background(), sentRoots, rand.NewGenerator()))

	if testutil.WaitTimeout(&wg, 1*time.Second) {
		t.Fatal("Did not receive stream within 1 sec")
	}
	assert.Equal(t, 4, len(r.slotToPendingBlocks.Items()), "Incorrect size for slot to pending blocks cache")
	assert.Equal(t, 4, len(r.seenPendingBlocks), "Incorrect size for seen pending block")
}

func TestService_AddPeningBlockToQueueOverMax(t *testing.T) {
	r := &Service{
		slotToPendingBlocks: gcache.New(time.Second, 2*time.Second),
		seenPendingBlocks:   make(map[[32]byte]bool),
	}

	b := testutil.NewBeaconBlock()
	b1 := copyutil.CopySignedBeaconBlock(b)
	b1.Block.StateRoot = []byte{'a'}
	b2 := copyutil.CopySignedBeaconBlock(b)
	b2.Block.StateRoot = []byte{'b'}
	require.NoError(t, r.insertBlockToPendingQueue(0, wrapper.WrappedPhase0SignedBeaconBlock(b), [32]byte{}))
	require.NoError(t, r.insertBlockToPendingQueue(0, wrapper.WrappedPhase0SignedBeaconBlock(b1), [32]byte{1}))
	require.NoError(t, r.insertBlockToPendingQueue(0, wrapper.WrappedPhase0SignedBeaconBlock(b2), [32]byte{2}))

	b3 := copyutil.CopySignedBeaconBlock(b)
	b3.Block.StateRoot = []byte{'c'}
	require.NoError(t, r.insertBlockToPendingQueue(0, wrapper.WrappedPhase0SignedBeaconBlock(b2), [32]byte{3}))
	require.Equal(t, maxBlocksPerSlot, len(r.pendingBlocksInCache(0)))
}

func TestService_ProcessPendingBlockOnCorrectSlot(t *testing.T) {
	ctx := context.Background()
	db := dbtest.SetupDB(t)

	p1 := p2ptest.NewTestP2P(t)
	mockChain := mock.ChainService{Genesis: time.Unix(time.Now().Unix()-int64(params.BeaconConfig().SecondsPerSlot), 0),
		FinalizedCheckPoint: &ethpb.Checkpoint{
			Epoch: 0,
		}}
	r := &Service{
		cfg: &Config{
			P2P:      p1,
			DB:       db,
			Chain:    &mockChain,
			StateGen: stategen.New(db),
		},
		slotToPendingBlocks: gcache.New(time.Second, 2*time.Second),
		seenPendingBlocks:   make(map[[32]byte]bool),
	}
	err := r.initCaches()
	require.NoError(t, err)

	beaconState, privKeys := testutil.DeterministicGenesisState(t, 100)
	parentBlock := testutil.NewBeaconBlock()
	require.NoError(t, db.SaveBlock(ctx, wrapper.WrappedPhase0SignedBeaconBlock(parentBlock)))
	bRoot, err := parentBlock.Block.HashTreeRoot()
	require.NoError(t, err)
	require.NoError(t, db.SaveState(ctx, beaconState, bRoot))
	require.NoError(t, db.SaveStateSummary(ctx, &pb.StateSummary{Root: bRoot[:]}))
	copied := beaconState.Copy()
	require.NoError(t, copied.SetSlot(1))
	proposerIdx, err := helpers.BeaconProposerIndex(copied)
	require.NoError(t, err)

	st, err := testutil.NewBeaconState()
	require.NoError(t, err)
	mockChain.Root = bRoot[:]
	mockChain.State = st

	b1 := testutil.NewBeaconBlock()
	b1.Block.ParentRoot = bRoot[:]
	b1.Block.Slot = 1
	b1Root, err := b1.Block.HashTreeRoot()
	require.NoError(t, err)
	b1.Block.ProposerIndex = proposerIdx
	b1.Signature, err = helpers.ComputeDomainAndSign(beaconState, 0, b1.Block, params.BeaconConfig().DomainBeaconProposer, privKeys[proposerIdx])
	require.NoError(t, err)

	b2 := testutil.NewBeaconBlock()
	b2.Block.Slot = 2
	b2.Block.ParentRoot = bRoot[:]
	b2Root, err := b2.Block.HashTreeRoot()
	require.NoError(t, err)

	b3 := testutil.NewBeaconBlock()
	b3.Block.Slot = 3
	b3.Block.ParentRoot = b2Root[:]
	b3Root, err := b3.Block.HashTreeRoot()
	require.NoError(t, err)

	// Add block1 for slot1
	require.NoError(t, r.insertBlockToPendingQueue(b1.Block.Slot, wrapper.WrappedPhase0SignedBeaconBlock(b1), b1Root))
	// Add block2 for slot2
	require.NoError(t, r.insertBlockToPendingQueue(b2.Block.Slot, wrapper.WrappedPhase0SignedBeaconBlock(b2), b2Root))
	// Add block3 for slot3
	require.NoError(t, r.insertBlockToPendingQueue(b3.Block.Slot, wrapper.WrappedPhase0SignedBeaconBlock(b3), b3Root))

	// processPendingBlocks should process only blocks of the current slot. i.e. slot 1.
	// Then check if the other two blocks are still in the pendingQueue.
	require.NoError(t, r.processPendingBlocks(context.Background()))
	assert.Equal(t, 2, len(r.slotToPendingBlocks.Items()), "Incorrect size for slot to pending blocks cache")
}
