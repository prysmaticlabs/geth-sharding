package sync

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/kevinms/leakybucket-go"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/protocol"
	gcache "github.com/patrickmn/go-cache"
	types "github.com/prysmaticlabs/eth2-types"
	mock "github.com/prysmaticlabs/prysm/beacon-chain/blockchain/testing"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/state"
	db "github.com/prysmaticlabs/prysm/beacon-chain/db/testing"
	"github.com/prysmaticlabs/prysm/beacon-chain/p2p"
	p2ptest "github.com/prysmaticlabs/prysm/beacon-chain/p2p/testing"
	p2pTypes "github.com/prysmaticlabs/prysm/beacon-chain/p2p/types"
	ethpb "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/proto/eth/v1alpha1/wrapper"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/testutil"
	"github.com/prysmaticlabs/prysm/shared/testutil/assert"
	"github.com/prysmaticlabs/prysm/shared/testutil/require"
)

func TestRecentBeaconBlocksRPCHandler_ReturnsBlocks(t *testing.T) {
	p1 := p2ptest.NewTestP2P(t)
	p2 := p2ptest.NewTestP2P(t)
	p1.Connect(p2)
	assert.Equal(t, 1, len(p1.BHost.Network().Peers()), "Expected peers to be connected")
	d := db.SetupDB(t)

	var blkRoots p2pTypes.BeaconBlockByRootsReq
	// Populate the database with blocks that would match the request.
	for i := types.Slot(1); i < 11; i++ {
		blk := testutil.NewBeaconBlock()
		blk.Block.Slot = i
		root, err := blk.Block.HashTreeRoot()
		require.NoError(t, err)
		require.NoError(t, d.SaveBlock(context.Background(), wrapper.WrappedPhase0SignedBeaconBlock(blk)))
		blkRoots = append(blkRoots, root)
	}

	r := &Service{cfg: &Config{P2P: p1, DB: d}, rateLimiter: newRateLimiter(p1)}
	r.cfg.Chain = &mock.ChainService{ValidatorsRoot: [32]byte{}}
	pcl := protocol.ID(p2p.RPCBlocksByRootTopicV1)
	topic := string(pcl)
	r.rateLimiter.limiterMap[topic] = leakybucket.NewCollector(10000, 10000, false)

	var wg sync.WaitGroup
	wg.Add(1)
	p2.BHost.SetStreamHandler(pcl, func(stream network.Stream) {
		defer wg.Done()
		for i := range blkRoots {
			expectSuccess(t, stream)
			res := testutil.NewBeaconBlock()
			assert.NoError(t, r.cfg.P2P.Encoding().DecodeWithMaxLength(stream, res))
			if uint64(res.Block.Slot) != uint64(i+1) {
				t.Errorf("Received unexpected block slot %d but wanted %d", res.Block.Slot, i+1)
			}
		}
	})

	stream1, err := p1.BHost.NewStream(context.Background(), p2.BHost.ID(), pcl)
	require.NoError(t, err)
	err = r.beaconBlocksRootRPCHandler(context.Background(), &blkRoots, stream1)
	assert.NoError(t, err)

	if testutil.WaitTimeout(&wg, 1*time.Second) {
		t.Fatal("Did not receive stream within 1 sec")
	}
}

func TestRecentBeaconBlocks_RPCRequestSent(t *testing.T) {
	p1 := p2ptest.NewTestP2P(t)
	p2 := p2ptest.NewTestP2P(t)
	p1.DelaySend = true

	blockA := testutil.NewBeaconBlock()
	blockA.Block.Slot = 111
	blockB := testutil.NewBeaconBlock()
	blockB.Block.Slot = 40
	// Set up a head state with data we expect.
	blockARoot, err := blockA.Block.HashTreeRoot()
	require.NoError(t, err)
	blockBRoot, err := blockB.Block.HashTreeRoot()
	require.NoError(t, err)
	genesisState, err := state.GenesisBeaconState(context.Background(), nil, 0, &ethpb.Eth1Data{})
	require.NoError(t, err)
	require.NoError(t, genesisState.SetSlot(111))
	require.NoError(t, genesisState.UpdateBlockRootAtIndex(111%uint64(params.BeaconConfig().SlotsPerHistoricalRoot), blockARoot))
	finalizedCheckpt := &ethpb.Checkpoint{
		Epoch: 5,
		Root:  blockBRoot[:],
	}

	expectedRoots := p2pTypes.BeaconBlockByRootsReq{blockBRoot, blockARoot}

	r := &Service{
		cfg: &Config{
			P2P: p1,
			Chain: &mock.ChainService{
				State:               genesisState,
				FinalizedCheckPoint: finalizedCheckpt,
				Root:                blockARoot[:],
				Genesis:             time.Now(),
				ValidatorsRoot:      [32]byte{},
			},
		},
		slotToPendingBlocks: gcache.New(time.Second, 2*time.Second),
		seenPendingBlocks:   make(map[[32]byte]bool),
		ctx:                 context.Background(),
		rateLimiter:         newRateLimiter(p1),
	}

	// Setup streams
	pcl := protocol.ID("/eth2/beacon_chain/req/beacon_blocks_by_root/1/ssz_snappy")
	topic := string(pcl)
	r.rateLimiter.limiterMap[topic] = leakybucket.NewCollector(10000, 10000, false)

	var wg sync.WaitGroup
	wg.Add(1)
	p2.BHost.SetStreamHandler(pcl, func(stream network.Stream) {
		defer wg.Done()
		out := new(p2pTypes.BeaconBlockByRootsReq)
		assert.NoError(t, p2.Encoding().DecodeWithMaxLength(stream, out))
		assert.DeepEqual(t, &expectedRoots, out, "Did not receive expected message")
		response := []*ethpb.SignedBeaconBlock{blockB, blockA}
		for _, blk := range response {
			_, err := stream.Write([]byte{responseCodeSuccess})
			assert.NoError(t, err, "Could not write to stream")
			_, err = p2.Encoding().EncodeWithMaxLength(stream, blk)
			assert.NoError(t, err, "Could not send response back")
		}
		assert.NoError(t, stream.Close())
	})

	p1.Connect(p2)
	require.NoError(t, r.sendRecentBeaconBlocksRequest(context.Background(), &expectedRoots, p2.PeerID()))

	if testutil.WaitTimeout(&wg, 1*time.Second) {
		t.Fatal("Did not receive stream within 1 sec")
	}
}

func TestRecentBeaconBlocksRPCHandler_HandleZeroBlocks(t *testing.T) {
	p1 := p2ptest.NewTestP2P(t)
	p2 := p2ptest.NewTestP2P(t)
	p1.Connect(p2)
	assert.Equal(t, 1, len(p1.BHost.Network().Peers()), "Expected peers to be connected")
	d := db.SetupDB(t)

	r := &Service{cfg: &Config{P2P: p1, DB: d}, rateLimiter: newRateLimiter(p1)}
	pcl := protocol.ID(p2p.RPCBlocksByRootTopicV1)
	topic := string(pcl)
	r.rateLimiter.limiterMap[topic] = leakybucket.NewCollector(1, 1, false)

	var wg sync.WaitGroup
	wg.Add(1)
	p2.BHost.SetStreamHandler(pcl, func(stream network.Stream) {
		defer wg.Done()
		expectFailure(t, 1, "no block roots provided in request", stream)
	})

	stream1, err := p1.BHost.NewStream(context.Background(), p2.BHost.ID(), pcl)
	require.NoError(t, err)
	err = r.beaconBlocksRootRPCHandler(context.Background(), &p2pTypes.BeaconBlockByRootsReq{}, stream1)
	assert.ErrorContains(t, "no block roots provided", err)
	if testutil.WaitTimeout(&wg, 1*time.Second) {
		t.Fatal("Did not receive stream within 1 sec")
	}

	r.rateLimiter.RLock() // retrieveCollector requires a lock to be held.
	defer r.rateLimiter.RUnlock()
	lter, err := r.rateLimiter.retrieveCollector(topic)
	require.NoError(t, err)
	assert.Equal(t, 1, int(lter.Count(stream1.Conn().RemotePeer().String())))
}
