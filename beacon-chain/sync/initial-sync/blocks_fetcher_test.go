package initialsync

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/kevinms/leakybucket-go"
	core "github.com/libp2p/go-libp2p-core"
	"github.com/libp2p/go-libp2p-core/network"
	types "github.com/prysmaticlabs/eth2-types"
	mock "github.com/prysmaticlabs/prysm/beacon-chain/blockchain/testing"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	dbtest "github.com/prysmaticlabs/prysm/beacon-chain/db/testing"
	p2pm "github.com/prysmaticlabs/prysm/beacon-chain/p2p"
	p2pt "github.com/prysmaticlabs/prysm/beacon-chain/p2p/testing"
	beaconsync "github.com/prysmaticlabs/prysm/beacon-chain/sync"
	"github.com/prysmaticlabs/prysm/cmd/beacon-chain/flags"
	p2ppb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	eth "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/proto/eth/v1alpha1/wrapper"
	"github.com/prysmaticlabs/prysm/proto/interfaces"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/sliceutil"
	"github.com/prysmaticlabs/prysm/shared/testutil"
	"github.com/prysmaticlabs/prysm/shared/testutil/assert"
	"github.com/prysmaticlabs/prysm/shared/testutil/require"
	"github.com/sirupsen/logrus"
	logTest "github.com/sirupsen/logrus/hooks/test"
)

func TestBlocksFetcher_InitStartStop(t *testing.T) {
	mc, p2p, _ := initializeTestServices(t, []types.Slot{}, []*peerData{})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	fetcher := newBlocksFetcher(
		ctx,
		&blocksFetcherConfig{
			chain: mc,
			p2p:   p2p,
		},
	)

	t.Run("check for leaked goroutines", func(t *testing.T) {
		err := fetcher.start()
		require.NoError(t, err)
		fetcher.stop() // should block up until all resources are reclaimed
		select {
		case <-fetcher.requestResponses():
		default:
			t.Error("fetchResponses channel is leaked")
		}
	})

	t.Run("re-starting of stopped fetcher", func(t *testing.T) {
		assert.ErrorContains(t, errFetcherCtxIsDone.Error(), fetcher.start())
	})

	t.Run("multiple stopping attempts", func(t *testing.T) {
		fetcher := newBlocksFetcher(
			context.Background(),
			&blocksFetcherConfig{
				chain: mc,
				p2p:   p2p,
			})
		require.NoError(t, fetcher.start())
		fetcher.stop()
		fetcher.stop()
	})

	t.Run("cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		fetcher := newBlocksFetcher(
			ctx,
			&blocksFetcherConfig{
				chain: mc,
				p2p:   p2p,
			})
		require.NoError(t, fetcher.start())
		cancel()
		fetcher.stop()
	})

	t.Run("peer filter capacity weight", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		fetcher := newBlocksFetcher(
			ctx,
			&blocksFetcherConfig{
				chain:                    mc,
				p2p:                      p2p,
				peerFilterCapacityWeight: 2,
			})
		require.NoError(t, fetcher.start())
		assert.Equal(t, peerFilterCapacityWeight, fetcher.capacityWeight)
	})
}

func TestBlocksFetcher_RoundRobin(t *testing.T) {
	slotsInBatch := types.Slot(flags.Get().BlockBatchLimit)
	requestsGenerator := func(start, end, batchSize types.Slot) []*fetchRequestParams {
		var requests []*fetchRequestParams
		for i := start; i <= end; i += batchSize {
			requests = append(requests, &fetchRequestParams{
				start: i,
				count: uint64(batchSize),
			})
		}
		return requests
	}
	tests := []struct {
		name               string
		expectedBlockSlots []types.Slot
		peers              []*peerData
		requests           []*fetchRequestParams
	}{
		{
			name:               "Single peer with all blocks",
			expectedBlockSlots: makeSequence(1, 3*slotsInBatch),
			peers: []*peerData{
				{
					blocks:         makeSequence(1, 3*slotsInBatch),
					finalizedEpoch: helpers.SlotToEpoch(3 * slotsInBatch),
					headSlot:       3 * slotsInBatch,
				},
			},
			requests: requestsGenerator(1, 3*slotsInBatch, slotsInBatch),
		},
		{
			name:               "Single peer with all blocks (many small requests)",
			expectedBlockSlots: makeSequence(1, 3*slotsInBatch),
			peers: []*peerData{
				{
					blocks:         makeSequence(1, 3*slotsInBatch),
					finalizedEpoch: helpers.SlotToEpoch(3 * slotsInBatch),
					headSlot:       3 * slotsInBatch,
				},
			},
			requests: requestsGenerator(1, 3*slotsInBatch, slotsInBatch/4),
		},
		{
			name:               "Multiple peers with all blocks",
			expectedBlockSlots: makeSequence(1, 3*slotsInBatch),
			peers: []*peerData{
				{
					blocks:         makeSequence(1, 3*slotsInBatch),
					finalizedEpoch: helpers.SlotToEpoch(3 * slotsInBatch),
					headSlot:       3 * slotsInBatch,
				},
				{
					blocks:         makeSequence(1, 3*slotsInBatch),
					finalizedEpoch: helpers.SlotToEpoch(3 * slotsInBatch),
					headSlot:       3 * slotsInBatch,
				},
				{
					blocks:         makeSequence(1, 3*slotsInBatch),
					finalizedEpoch: helpers.SlotToEpoch(3 * slotsInBatch),
					headSlot:       3 * slotsInBatch,
				},
			},
			requests: requestsGenerator(1, 3*slotsInBatch, slotsInBatch),
		},
		{
			name:               "Multiple peers with skipped slots",
			expectedBlockSlots: append(makeSequence(1, 64), makeSequence(500, 640)...), // up to 18th epoch
			peers: []*peerData{
				{
					blocks:         append(makeSequence(1, 64), makeSequence(500, 640)...),
					finalizedEpoch: 18,
					headSlot:       640,
				},
				{
					blocks:         append(makeSequence(1, 64), makeSequence(500, 640)...),
					finalizedEpoch: 18,
					headSlot:       640,
				},
				{
					blocks:         append(makeSequence(1, 64), makeSequence(500, 640)...),
					finalizedEpoch: 18,
					headSlot:       640,
				},
				{
					blocks:         append(makeSequence(1, 64), makeSequence(500, 640)...),
					finalizedEpoch: 18,
					headSlot:       640,
				},
				{
					blocks:         append(makeSequence(1, 64), makeSequence(500, 640)...),
					finalizedEpoch: 18,
					headSlot:       640,
				},
			},
			requests: []*fetchRequestParams{
				{
					start: 1,
					count: uint64(slotsInBatch),
				},
				{
					start: slotsInBatch + 1,
					count: uint64(slotsInBatch),
				},
				{
					start: 2*slotsInBatch + 1,
					count: uint64(slotsInBatch),
				},
				{
					start: 500,
					count: 53,
				},
				{
					start: 553,
					count: 200,
				},
			},
		},
		{
			name:               "Multiple peers with failures",
			expectedBlockSlots: makeSequence(1, 2*slotsInBatch),
			peers: []*peerData{
				{
					blocks:         makeSequence(1, 320),
					finalizedEpoch: 8,
					headSlot:       320,
				},
				{
					blocks:         makeSequence(1, 320),
					finalizedEpoch: 8,
					headSlot:       320,
					failureSlots:   makeSequence(1, 32), // first epoch
				},
				{
					blocks:         makeSequence(1, 320),
					finalizedEpoch: 8,
					headSlot:       320,
				},
				{
					blocks:         makeSequence(1, 320),
					finalizedEpoch: 8,
					headSlot:       320,
				},
			},
			requests: []*fetchRequestParams{
				{
					start: 1,
					count: uint64(slotsInBatch),
				},
				{
					start: slotsInBatch + 1,
					count: uint64(slotsInBatch),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache.initializeRootCache(tt.expectedBlockSlots, t)

			beaconDB := dbtest.SetupDB(t)

			p := p2pt.NewTestP2P(t)
			connectPeers(t, p, tt.peers, p.Peers())
			cache.RLock()
			genesisRoot := cache.rootCache[0]
			cache.RUnlock()

			err := beaconDB.SaveBlock(context.Background(), wrapper.WrappedPhase0SignedBeaconBlock(testutil.NewBeaconBlock()))
			require.NoError(t, err)

			st, err := testutil.NewBeaconState()
			require.NoError(t, err)

			mc := &mock.ChainService{
				State: st,
				Root:  genesisRoot[:],
				DB:    beaconDB,
				FinalizedCheckPoint: &eth.Checkpoint{
					Epoch: 0,
				},
				Genesis:        time.Now(),
				ValidatorsRoot: [32]byte{},
			}

			ctx, cancel := context.WithCancel(context.Background())
			fetcher := newBlocksFetcher(ctx, &blocksFetcherConfig{
				chain: mc,
				p2p:   p,
			})
			require.NoError(t, fetcher.start())

			var wg sync.WaitGroup
			wg.Add(len(tt.requests)) // how many block requests we are going to make
			go func() {
				wg.Wait()
				log.Debug("Stopping fetcher")
				fetcher.stop()
			}()

			processFetchedBlocks := func() ([]interfaces.SignedBeaconBlock, error) {
				defer cancel()
				var unionRespBlocks []interfaces.SignedBeaconBlock

				for {
					select {
					case resp, ok := <-fetcher.requestResponses():
						if !ok { // channel closed, aggregate
							return unionRespBlocks, nil
						}

						if resp.err != nil {
							log.WithError(resp.err).Debug("Block fetcher returned error")
						} else {
							unionRespBlocks = append(unionRespBlocks, resp.blocks...)
							if len(resp.blocks) == 0 {
								log.WithFields(logrus.Fields{
									"start": resp.start,
									"count": resp.count,
								}).Debug("Received empty slot")
							}
						}

						wg.Done()
					case <-ctx.Done():
						log.Debug("Context closed, exiting goroutine")
						return unionRespBlocks, nil
					}
				}
			}

			maxExpectedBlocks := uint64(0)
			for _, requestParams := range tt.requests {
				err = fetcher.scheduleRequest(context.Background(), requestParams.start, requestParams.count)
				assert.NoError(t, err)
				maxExpectedBlocks += requestParams.count
			}

			blocks, err := processFetchedBlocks()
			assert.NoError(t, err)

			sort.Slice(blocks, func(i, j int) bool {
				return blocks[i].Block().Slot() < blocks[j].Block().Slot()
			})

			slots := make([]types.Slot, len(blocks))
			for i, block := range blocks {
				slots[i] = block.Block().Slot()
			}

			log.WithFields(logrus.Fields{
				"blocksLen": len(blocks),
				"slots":     slots,
			}).Debug("Finished block fetching")

			if len(blocks) > int(maxExpectedBlocks) {
				t.Errorf("Too many blocks returned. Wanted %d got %d", maxExpectedBlocks, len(blocks))
			}
			assert.Equal(t, len(tt.expectedBlockSlots), len(blocks), "Processes wrong number of blocks")
			var receivedBlockSlots []types.Slot
			for _, blk := range blocks {
				receivedBlockSlots = append(receivedBlockSlots, blk.Block().Slot())
			}
			missing := sliceutil.NotSlot(sliceutil.IntersectionSlot(tt.expectedBlockSlots, receivedBlockSlots), tt.expectedBlockSlots)
			if len(missing) > 0 {
				t.Errorf("Missing blocks at slots %v", missing)
			}
		})
	}
}

func TestBlocksFetcher_scheduleRequest(t *testing.T) {
	blockBatchLimit := uint64(flags.Get().BlockBatchLimit)
	t.Run("context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		fetcher := newBlocksFetcher(ctx, &blocksFetcherConfig{})
		cancel()
		assert.ErrorContains(t, "context canceled", fetcher.scheduleRequest(ctx, 1, blockBatchLimit))
	})

	t.Run("unblock on context cancellation", func(t *testing.T) {
		fetcher := newBlocksFetcher(context.Background(), &blocksFetcherConfig{})
		for i := 0; i < maxPendingRequests; i++ {
			assert.NoError(t, fetcher.scheduleRequest(context.Background(), 1, blockBatchLimit))
		}

		// Will block on next request (and wait until requests are either processed or context is closed).
		go func() {
			fetcher.cancel()
		}()
		assert.ErrorContains(t, errFetcherCtxIsDone.Error(),
			fetcher.scheduleRequest(context.Background(), 1, blockBatchLimit))
	})
}
func TestBlocksFetcher_handleRequest(t *testing.T) {
	blockBatchLimit := flags.Get().BlockBatchLimit
	chainConfig := struct {
		expectedBlockSlots []types.Slot
		peers              []*peerData
	}{
		expectedBlockSlots: makeSequence(1, types.Slot(blockBatchLimit)),
		peers: []*peerData{
			{
				blocks:         makeSequence(1, 320),
				finalizedEpoch: 8,
				headSlot:       320,
			},
			{
				blocks:         makeSequence(1, 320),
				finalizedEpoch: 8,
				headSlot:       320,
			},
		},
	}

	mc, p2p, _ := initializeTestServices(t, chainConfig.expectedBlockSlots, chainConfig.peers)
	mc.ValidatorsRoot = [32]byte{}
	mc.Genesis = time.Now()
	t.Run("context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		fetcher := newBlocksFetcher(ctx, &blocksFetcherConfig{
			chain: mc,
			p2p:   p2p,
		})

		cancel()
		response := fetcher.handleRequest(ctx, 1, uint64(blockBatchLimit))
		assert.ErrorContains(t, "context canceled", response.err)
	})

	t.Run("receive blocks", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		fetcher := newBlocksFetcher(ctx, &blocksFetcherConfig{
			chain: mc,
			p2p:   p2p,
		})

		requestCtx, reqCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer reqCancel()
		go func() {
			response := fetcher.handleRequest(requestCtx, 1 /* start */, uint64(blockBatchLimit) /* count */)
			select {
			case <-ctx.Done():
			case fetcher.fetchResponses <- response:
			}
		}()

		var blocks []interfaces.SignedBeaconBlock
		select {
		case <-ctx.Done():
			t.Error(ctx.Err())
		case resp := <-fetcher.requestResponses():
			if resp.err != nil {
				t.Error(resp.err)
			} else {
				blocks = resp.blocks
			}
		}
		if uint64(len(blocks)) != uint64(blockBatchLimit) {
			t.Errorf("incorrect number of blocks returned, expected: %v, got: %v", blockBatchLimit, len(blocks))
		}

		var receivedBlockSlots []types.Slot
		for _, blk := range blocks {
			receivedBlockSlots = append(receivedBlockSlots, blk.Block().Slot())
		}
		missing := sliceutil.NotSlot(sliceutil.IntersectionSlot(chainConfig.expectedBlockSlots, receivedBlockSlots), chainConfig.expectedBlockSlots)
		if len(missing) > 0 {
			t.Errorf("Missing blocks at slots %v", missing)
		}
	})
}

func TestBlocksFetcher_requestBeaconBlocksByRange(t *testing.T) {
	blockBatchLimit := flags.Get().BlockBatchLimit
	chainConfig := struct {
		expectedBlockSlots []types.Slot
		peers              []*peerData
	}{
		expectedBlockSlots: makeSequence(1, 320),
		peers: []*peerData{
			{
				blocks:         makeSequence(1, 320),
				finalizedEpoch: 8,
				headSlot:       320,
			},
			{
				blocks:         makeSequence(1, 320),
				finalizedEpoch: 8,
				headSlot:       320,
			},
		},
	}

	mc, p2p, _ := initializeTestServices(t, chainConfig.expectedBlockSlots, chainConfig.peers)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fetcher := newBlocksFetcher(
		ctx,
		&blocksFetcherConfig{
			chain: mc,
			p2p:   p2p,
		})

	_, peerIDs := p2p.Peers().BestFinalized(params.BeaconConfig().MaxPeersToSync, helpers.SlotToEpoch(mc.HeadSlot()))
	req := &p2ppb.BeaconBlocksByRangeRequest{
		StartSlot: 1,
		Step:      1,
		Count:     uint64(blockBatchLimit),
	}
	blocks, err := fetcher.requestBlocks(ctx, req, peerIDs[0])
	assert.NoError(t, err)
	assert.Equal(t, uint64(blockBatchLimit), uint64(len(blocks)), "Incorrect number of blocks returned")

	// Test context cancellation.
	ctx, cancel = context.WithCancel(context.Background())
	cancel()
	_, err = fetcher.requestBlocks(ctx, req, peerIDs[0])
	assert.ErrorContains(t, "context canceled", err)
}

func TestBlocksFetcher_RequestBlocksRateLimitingLocks(t *testing.T) {
	p1 := p2pt.NewTestP2P(t)
	p2 := p2pt.NewTestP2P(t)
	p3 := p2pt.NewTestP2P(t)
	p1.Connect(p2)
	p1.Connect(p3)
	require.Equal(t, 2, len(p1.BHost.Network().Peers()), "Expected peers to be connected")
	req := &p2ppb.BeaconBlocksByRangeRequest{
		StartSlot: 100,
		Step:      1,
		Count:     64,
	}

	topic := p2pm.RPCBlocksByRangeTopicV1
	protocol := core.ProtocolID(topic + p2.Encoding().ProtocolSuffix())
	streamHandlerFn := func(stream network.Stream) {
		assert.NoError(t, stream.Close())
	}
	p2.BHost.SetStreamHandler(protocol, streamHandlerFn)
	p3.BHost.SetStreamHandler(protocol, streamHandlerFn)

	burstFactor := uint64(flags.Get().BlockBatchLimitBurstFactor)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	fetcher := newBlocksFetcher(ctx, &blocksFetcherConfig{p2p: p1})
	fetcher.rateLimiter = leakybucket.NewCollector(float64(req.Count), int64(req.Count*burstFactor), false)
	fetcher.chain = &mock.ChainService{Genesis: time.Now(), ValidatorsRoot: [32]byte{}}
	hook := logTest.NewGlobal()
	wg := new(sync.WaitGroup)
	wg.Add(1)
	go func() {
		// Exhaust available rate for p2, so that rate limiting is triggered.
		for i := uint64(0); i <= burstFactor; i++ {
			if i == burstFactor {
				// The next request will trigger rate limiting for p2. Now, allow concurrent
				// p3 data request (p3 shouldn't be rate limited).
				time.AfterFunc(1*time.Second, func() {
					wg.Done()
				})
			}
			_, err := fetcher.requestBlocks(ctx, req, p2.PeerID())
			if err != nil {
				assert.ErrorContains(t, errFetcherCtxIsDone.Error(), err)
			}
		}
	}()

	// Wait until p2 exhausts its rate and is spinning on rate limiting timer.
	wg.Wait()

	// The next request should NOT trigger rate limiting as rate is exhausted for p2, not p3.
	ch := make(chan struct{}, 1)
	go func() {
		_, err := fetcher.requestBlocks(ctx, req, p3.PeerID())
		assert.NoError(t, err)
		ch <- struct{}{}
	}()
	timer := time.NewTimer(2 * time.Second)
	select {
	case <-timer.C:
		t.Error("p3 takes too long to respond: lock contention")
	case <-ch:
		// p3 responded w/o waiting for rate limiter's lock (on which p2 spins).
	}
	// Make sure that p2 has been rate limited.
	require.LogsContain(t, hook, fmt.Sprintf("msg=\"Slowing down for rate limit\" peer=%s", p2.PeerID()))
}

func TestBlocksFetcher_requestBlocksFromPeerReturningInvalidBlocks(t *testing.T) {
	p1 := p2pt.NewTestP2P(t)
	tests := []struct {
		name         string
		req          *p2ppb.BeaconBlocksByRangeRequest
		handlerGenFn func(req *p2ppb.BeaconBlocksByRangeRequest) func(stream network.Stream)
		wantedErr    string
		validate     func(req *p2ppb.BeaconBlocksByRangeRequest, blocks []interfaces.SignedBeaconBlock)
	}{
		{
			name: "no error",
			req: &p2ppb.BeaconBlocksByRangeRequest{
				StartSlot: 100,
				Step:      4,
				Count:     64,
			},
			handlerGenFn: func(req *p2ppb.BeaconBlocksByRangeRequest) func(stream network.Stream) {
				return func(stream network.Stream) {
					for i := req.StartSlot; i < req.StartSlot.Add(req.Count*req.Step); i += types.Slot(req.Step) {
						blk := testutil.NewBeaconBlock()
						blk.Block.Slot = i
						mchain := &mock.ChainService{Genesis: time.Now(), ValidatorsRoot: [32]byte{}}
						assert.NoError(t, beaconsync.WriteBlockChunk(stream, mchain, p1.Encoding(), wrapper.WrappedPhase0SignedBeaconBlock(blk)))
					}
					assert.NoError(t, stream.Close())
				}
			},
			validate: func(req *p2ppb.BeaconBlocksByRangeRequest, blocks []interfaces.SignedBeaconBlock) {
				assert.Equal(t, req.Count, uint64(len(blocks)))
			},
		},
		{
			name: "too many blocks",
			req: &p2ppb.BeaconBlocksByRangeRequest{
				StartSlot: 100,
				Step:      1,
				Count:     64,
			},
			handlerGenFn: func(req *p2ppb.BeaconBlocksByRangeRequest) func(stream network.Stream) {
				return func(stream network.Stream) {
					for i := req.StartSlot; i < req.StartSlot.Add(req.Count*req.Step+1); i += types.Slot(req.Step) {
						blk := testutil.NewBeaconBlock()
						blk.Block.Slot = i
						chain := &mock.ChainService{Genesis: time.Now(), ValidatorsRoot: [32]byte{}}
						assert.NoError(t, beaconsync.WriteBlockChunk(stream, chain, p1.Encoding(), wrapper.WrappedPhase0SignedBeaconBlock(blk)))
					}
					assert.NoError(t, stream.Close())
				}
			},
			validate: func(req *p2ppb.BeaconBlocksByRangeRequest, blocks []interfaces.SignedBeaconBlock) {
				assert.Equal(t, 0, len(blocks))
			},
			wantedErr: beaconsync.ErrInvalidFetchedData.Error(),
		},
		{
			name: "not in a consecutive order",
			req: &p2ppb.BeaconBlocksByRangeRequest{
				StartSlot: 100,
				Step:      1,
				Count:     64,
			},
			handlerGenFn: func(req *p2ppb.BeaconBlocksByRangeRequest) func(stream network.Stream) {
				return func(stream network.Stream) {
					blk := testutil.NewBeaconBlock()
					blk.Block.Slot = 163
					chain := &mock.ChainService{Genesis: time.Now(), ValidatorsRoot: [32]byte{}}
					assert.NoError(t, beaconsync.WriteBlockChunk(stream, chain, p1.Encoding(), wrapper.WrappedPhase0SignedBeaconBlock(blk)))

					blk = testutil.NewBeaconBlock()
					blk.Block.Slot = 162
					assert.NoError(t, beaconsync.WriteBlockChunk(stream, chain, p1.Encoding(), wrapper.WrappedPhase0SignedBeaconBlock(blk)))
					assert.NoError(t, stream.Close())
				}
			},
			validate: func(req *p2ppb.BeaconBlocksByRangeRequest, blocks []interfaces.SignedBeaconBlock) {
				assert.Equal(t, 0, len(blocks))
			},
			wantedErr: beaconsync.ErrInvalidFetchedData.Error(),
		},
		{
			name: "same slot number",
			req: &p2ppb.BeaconBlocksByRangeRequest{
				StartSlot: 100,
				Step:      1,
				Count:     64,
			},
			handlerGenFn: func(req *p2ppb.BeaconBlocksByRangeRequest) func(stream network.Stream) {
				return func(stream network.Stream) {
					blk := testutil.NewBeaconBlock()
					blk.Block.Slot = 160
					chain := &mock.ChainService{Genesis: time.Now(), ValidatorsRoot: [32]byte{}}

					assert.NoError(t, beaconsync.WriteBlockChunk(stream, chain, p1.Encoding(), wrapper.WrappedPhase0SignedBeaconBlock(blk)))

					blk = testutil.NewBeaconBlock()
					blk.Block.Slot = 160
					assert.NoError(t, beaconsync.WriteBlockChunk(stream, chain, p1.Encoding(), wrapper.WrappedPhase0SignedBeaconBlock(blk)))
					assert.NoError(t, stream.Close())
				}
			},
			validate: func(req *p2ppb.BeaconBlocksByRangeRequest, blocks []interfaces.SignedBeaconBlock) {
				assert.Equal(t, 0, len(blocks))
			},
			wantedErr: beaconsync.ErrInvalidFetchedData.Error(),
		},
		{
			name: "slot is too low",
			req: &p2ppb.BeaconBlocksByRangeRequest{
				StartSlot: 100,
				Step:      1,
				Count:     64,
			},
			handlerGenFn: func(req *p2ppb.BeaconBlocksByRangeRequest) func(stream network.Stream) {
				return func(stream network.Stream) {
					defer func() {
						assert.NoError(t, stream.Close())
					}()
					for i := req.StartSlot; i < req.StartSlot.Add(req.Count*req.Step); i += types.Slot(req.Step) {
						blk := testutil.NewBeaconBlock()
						chain := &mock.ChainService{Genesis: time.Now(), ValidatorsRoot: [32]byte{}}
						// Patch mid block, with invalid slot number.
						if i == req.StartSlot.Add(req.Count*req.Step/2) {
							blk.Block.Slot = req.StartSlot - 1
							assert.NoError(t, beaconsync.WriteBlockChunk(stream, chain, p1.Encoding(), wrapper.WrappedPhase0SignedBeaconBlock(blk)))
							break
						} else {
							blk.Block.Slot = i
							assert.NoError(t, beaconsync.WriteBlockChunk(stream, chain, p1.Encoding(), wrapper.WrappedPhase0SignedBeaconBlock(blk)))
						}
					}
				}
			},
			wantedErr: beaconsync.ErrInvalidFetchedData.Error(),
			validate: func(req *p2ppb.BeaconBlocksByRangeRequest, blocks []interfaces.SignedBeaconBlock) {
				assert.Equal(t, 0, len(blocks))
			},
		},
		{
			name: "slot is too high",
			req: &p2ppb.BeaconBlocksByRangeRequest{
				StartSlot: 100,
				Step:      1,
				Count:     64,
			},
			handlerGenFn: func(req *p2ppb.BeaconBlocksByRangeRequest) func(stream network.Stream) {
				return func(stream network.Stream) {
					defer func() {
						assert.NoError(t, stream.Close())
					}()
					for i := req.StartSlot; i < req.StartSlot.Add(req.Count*req.Step); i += types.Slot(req.Step) {
						blk := testutil.NewBeaconBlock()
						chain := &mock.ChainService{Genesis: time.Now(), ValidatorsRoot: [32]byte{}}
						// Patch mid block, with invalid slot number.
						if i == req.StartSlot.Add(req.Count*req.Step/2) {
							blk.Block.Slot = req.StartSlot.Add(req.Count * req.Step)
							assert.NoError(t, beaconsync.WriteBlockChunk(stream, chain, p1.Encoding(), wrapper.WrappedPhase0SignedBeaconBlock(blk)))
							break
						} else {
							blk.Block.Slot = i
							assert.NoError(t, beaconsync.WriteBlockChunk(stream, chain, p1.Encoding(), wrapper.WrappedPhase0SignedBeaconBlock(blk)))
						}
					}
				}
			},
			wantedErr: beaconsync.ErrInvalidFetchedData.Error(),
			validate: func(req *p2ppb.BeaconBlocksByRangeRequest, blocks []interfaces.SignedBeaconBlock) {
				assert.Equal(t, 0, len(blocks))
			},
		},
		{
			name: "valid step increment",
			req: &p2ppb.BeaconBlocksByRangeRequest{
				StartSlot: 100,
				Step:      5,
				Count:     64,
			},
			handlerGenFn: func(req *p2ppb.BeaconBlocksByRangeRequest) func(stream network.Stream) {
				return func(stream network.Stream) {
					blk := testutil.NewBeaconBlock()
					blk.Block.Slot = 100
					chain := &mock.ChainService{Genesis: time.Now(), ValidatorsRoot: [32]byte{}}
					assert.NoError(t, beaconsync.WriteBlockChunk(stream, chain, p1.Encoding(), wrapper.WrappedPhase0SignedBeaconBlock(blk)))

					blk = testutil.NewBeaconBlock()
					blk.Block.Slot = 105
					assert.NoError(t, beaconsync.WriteBlockChunk(stream, chain, p1.Encoding(), wrapper.WrappedPhase0SignedBeaconBlock(blk)))
					assert.NoError(t, stream.Close())
				}
			},
			validate: func(req *p2ppb.BeaconBlocksByRangeRequest, blocks []interfaces.SignedBeaconBlock) {
				assert.Equal(t, 2, len(blocks))
			},
		},
		{
			name: "invalid step increment",
			req: &p2ppb.BeaconBlocksByRangeRequest{
				StartSlot: 100,
				Step:      5,
				Count:     64,
			},
			handlerGenFn: func(req *p2ppb.BeaconBlocksByRangeRequest) func(stream network.Stream) {
				return func(stream network.Stream) {
					blk := testutil.NewBeaconBlock()
					blk.Block.Slot = 100
					chain := &mock.ChainService{Genesis: time.Now(), ValidatorsRoot: [32]byte{}}
					assert.NoError(t, beaconsync.WriteBlockChunk(stream, chain, p1.Encoding(), wrapper.WrappedPhase0SignedBeaconBlock(blk)))

					blk = testutil.NewBeaconBlock()
					blk.Block.Slot = 103
					assert.NoError(t, beaconsync.WriteBlockChunk(stream, chain, p1.Encoding(), wrapper.WrappedPhase0SignedBeaconBlock(blk)))
					assert.NoError(t, stream.Close())
				}
			},
			validate: func(req *p2ppb.BeaconBlocksByRangeRequest, blocks []interfaces.SignedBeaconBlock) {
				assert.Equal(t, 0, len(blocks))
			},
			wantedErr: beaconsync.ErrInvalidFetchedData.Error(),
		},
	}

	topic := p2pm.RPCBlocksByRangeTopicV1
	protocol := core.ProtocolID(topic + p1.Encoding().ProtocolSuffix())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	fetcher := newBlocksFetcher(ctx, &blocksFetcherConfig{p2p: p1, chain: &mock.ChainService{Genesis: time.Now(), ValidatorsRoot: [32]byte{}}})
	fetcher.rateLimiter = leakybucket.NewCollector(0.000001, 640, false)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p2 := p2pt.NewTestP2P(t)
			p1.Connect(p2)

			p2.BHost.SetStreamHandler(protocol, tt.handlerGenFn(tt.req))
			blocks, err := fetcher.requestBlocks(ctx, tt.req, p2.PeerID())
			if tt.wantedErr != "" {
				assert.ErrorContains(t, tt.wantedErr, err)
			} else {
				assert.NoError(t, err)
				tt.validate(tt.req, blocks)
			}
		})
	}
}
