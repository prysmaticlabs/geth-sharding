package sync

import (
	"context"
	"sync"
	"testing"
	"time"

	gcache "github.com/patrickmn/go-cache"
	mockChain "github.com/prysmaticlabs/prysm/beacon-chain/blockchain/testing"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/feed"
	statefeed "github.com/prysmaticlabs/prysm/beacon-chain/core/feed/state"
	p2ptest "github.com/prysmaticlabs/prysm/beacon-chain/p2p/testing"
	v1 "github.com/prysmaticlabs/prysm/beacon-chain/state/v1"
	mockSync "github.com/prysmaticlabs/prysm/beacon-chain/sync/initial-sync/testing"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	"github.com/prysmaticlabs/prysm/shared/abool"
	"github.com/prysmaticlabs/prysm/shared/bls"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/prysmaticlabs/prysm/shared/testutil"
	"github.com/prysmaticlabs/prysm/shared/testutil/assert"
	"github.com/prysmaticlabs/prysm/shared/testutil/require"
)

func TestService_StatusZeroEpoch(t *testing.T) {
	bState, err := v1.InitializeFromProto(&pb.BeaconState{Slot: 0})
	require.NoError(t, err)
	r := &Service{
		cfg: &Config{
			P2P:         p2ptest.NewTestP2P(t),
			InitialSync: new(mockSync.Sync),
			Chain: &mockChain.ChainService{
				Genesis: time.Now(),
				State:   bState,
			},
		},
		chainStarted: abool.New(),
	}
	r.chainStarted.Set()

	assert.NoError(t, r.Status(), "Wanted non failing status")
}

func TestSyncHandlers_WaitToSync(t *testing.T) {
	p2p := p2ptest.NewTestP2P(t)
	chainService := &mockChain.ChainService{
		Genesis:        time.Now(),
		ValidatorsRoot: [32]byte{'A'},
	}
	r := Service{
		ctx: context.Background(),
		cfg: &Config{
			P2P:           p2p,
			Chain:         chainService,
			StateNotifier: chainService.StateNotifier(),
			InitialSync:   &mockSync.Sync{IsSyncing: false},
		},
		chainStarted: abool.New(),
	}

	topic := "/eth2/%x/beacon_block"
	go r.registerHandlers()
	time.Sleep(100 * time.Millisecond)
	i := r.cfg.StateNotifier.StateFeed().Send(&feed.Event{
		Type: statefeed.Initialized,
		Data: &statefeed.InitializedData{
			StartTime: time.Now(),
		},
	})
	if i == 0 {
		t.Fatal("didn't send genesis time to subscribers")
	}
	b := []byte("sk")
	b32 := bytesutil.ToBytes32(b)
	sk, err := bls.SecretKeyFromBytes(b32[:])
	require.NoError(t, err)

	msg := testutil.NewBeaconBlock()
	msg.Block.ParentRoot = testutil.Random32Bytes(t)
	msg.Signature = sk.Sign([]byte("data")).Marshal()
	p2p.ReceivePubSub(topic, msg)
	// wait for chainstart to be sent
	time.Sleep(400 * time.Millisecond)
	require.Equal(t, true, r.chainStarted.IsSet(), "Did not receive chain start event.")
}

func TestSyncHandlers_WaitForChainStart(t *testing.T) {
	p2p := p2ptest.NewTestP2P(t)
	chainService := &mockChain.ChainService{
		Genesis:        time.Now(),
		ValidatorsRoot: [32]byte{'A'},
	}
	r := Service{
		ctx: context.Background(),
		cfg: &Config{
			P2P:           p2p,
			Chain:         chainService,
			StateNotifier: chainService.StateNotifier(),
			InitialSync:   &mockSync.Sync{IsSyncing: false},
		},
		chainStarted:        abool.New(),
		slotToPendingBlocks: gcache.New(time.Second, 2*time.Second),
	}

	go r.registerHandlers()
	time.Sleep(100 * time.Millisecond)
	i := r.cfg.StateNotifier.StateFeed().Send(&feed.Event{
		Type: statefeed.Initialized,
		Data: &statefeed.InitializedData{
			StartTime: time.Now().Add(2 * time.Second),
		},
	})
	if i == 0 {
		t.Fatal("didn't send genesis time to subscribers")
	}
	require.Equal(t, false, r.chainStarted.IsSet(), "Chainstart was marked prematurely")

	// wait for chainstart to be sent
	time.Sleep(3 * time.Second)
	require.Equal(t, true, r.chainStarted.IsSet(), "Did not receive chain start event.")
}

func TestSyncHandlers_WaitTillSynced(t *testing.T) {
	p2p := p2ptest.NewTestP2P(t)
	chainService := &mockChain.ChainService{
		Genesis:        time.Now(),
		ValidatorsRoot: [32]byte{'A'},
	}
	r := Service{
		ctx: context.Background(),
		cfg: &Config{
			P2P:           p2p,
			Chain:         chainService,
			StateNotifier: chainService.StateNotifier(),
			InitialSync:   &mockSync.Sync{IsSyncing: false},
		},
		chainStarted: abool.New(),
		subHandler:   newSubTopicHandler(),
	}

	topic := "/eth2/%x/beacon_block"
	go r.registerHandlers()
	time.Sleep(100 * time.Millisecond)
	i := r.cfg.StateNotifier.StateFeed().Send(&feed.Event{
		Type: statefeed.Initialized,
		Data: &statefeed.InitializedData{
			StartTime: time.Now(),
		},
	})
	if i == 0 {
		t.Fatal("didn't send genesis time to subscribers")
	}
	b := []byte("sk")
	b32 := bytesutil.ToBytes32(b)
	sk, err := bls.SecretKeyFromBytes(b32[:])
	require.NoError(t, err)

	msg := testutil.NewBeaconBlock()
	msg.Block.ParentRoot = testutil.Random32Bytes(t)
	msg.Signature = sk.Sign([]byte("data")).Marshal()
	p2p.Digest, err = r.currentForkDigest()
	r.cfg.BlockNotifier = chainService.BlockNotifier()
	blockChan := make(chan feed.Event, 1)
	sub := r.cfg.BlockNotifier.BlockFeed().Subscribe(blockChan)

	require.NoError(t, err)
	p2p.ReceivePubSub(topic, msg)

	// wait for chainstart to be sent
	time.Sleep(2 * time.Second)
	require.Equal(t, true, r.chainStarted.IsSet(), "Did not receive chain start event.")

	assert.Equal(t, 0, len(blockChan), "block was received by sync service despite not being fully synced")

	i = r.cfg.StateNotifier.StateFeed().Send(&feed.Event{
		Type: statefeed.Synced,
		Data: &statefeed.SyncedData{
			StartTime: time.Now(),
		},
	})

	if i == 0 {
		t.Fatal("didn't send genesis time to sync event subscribers")
	}

	wg := new(sync.WaitGroup)
	wg.Add(1)
	go func() {
		// Wait for block to be received by service.
		<-blockChan
		wg.Done()
		sub.Unsubscribe()
	}()

	p2p.ReceivePubSub(topic, msg)
	// wait for message to be sent
	testutil.WaitTimeout(wg, 2*time.Second)
}

func TestSyncService_StopCleanly(t *testing.T) {
	p2p := p2ptest.NewTestP2P(t)
	chainService := &mockChain.ChainService{
		Genesis:        time.Now(),
		ValidatorsRoot: [32]byte{'A'},
	}
	ctx, cancel := context.WithCancel(context.Background())
	r := Service{
		ctx:    ctx,
		cancel: cancel,
		cfg: &Config{
			P2P:           p2p,
			Chain:         chainService,
			StateNotifier: chainService.StateNotifier(),
			InitialSync:   &mockSync.Sync{IsSyncing: false},
		},
		chainStarted: abool.New(),
		subHandler:   newSubTopicHandler(),
	}

	go r.registerHandlers()
	time.Sleep(100 * time.Millisecond)
	i := r.cfg.StateNotifier.StateFeed().Send(&feed.Event{
		Type: statefeed.Initialized,
		Data: &statefeed.InitializedData{
			StartTime: time.Now(),
		},
	})
	if i == 0 {
		t.Fatal("didn't send genesis time to subscribers")
	}

	var err error
	p2p.Digest, err = r.currentForkDigest()
	require.NoError(t, err)

	// wait for chainstart to be sent
	time.Sleep(2 * time.Second)
	require.Equal(t, true, r.chainStarted.IsSet(), "Did not receive chain start event.")

	i = r.cfg.StateNotifier.StateFeed().Send(&feed.Event{
		Type: statefeed.Synced,
		Data: &statefeed.SyncedData{
			StartTime: time.Now(),
		},
	})
	if i == 0 {
		t.Fatal("didn't send genesis time to sync event subscribers")
	}

	time.Sleep(1 * time.Second)

	require.NotEqual(t, 0, len(r.cfg.P2P.PubSub().GetTopics()))
	require.NotEqual(t, 0, len(r.cfg.P2P.Host().Mux().Protocols()))

	// Both pubsub and rpc topcis should be unsubscribed.
	require.NoError(t, r.Stop())

	// Sleep to allow pubsub topics to be deregistered.
	time.Sleep(1 * time.Second)
	require.Equal(t, 0, len(r.cfg.P2P.PubSub().GetTopics()))
	require.Equal(t, 0, len(r.cfg.P2P.Host().Mux().Protocols()))
}
