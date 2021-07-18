package initialsync

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/paulbellamy/ratecounter"
	types "github.com/prysmaticlabs/eth2-types"
	mock "github.com/prysmaticlabs/prysm/beacon-chain/blockchain/testing"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/feed"
	statefeed "github.com/prysmaticlabs/prysm/beacon-chain/core/feed/state"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	dbtest "github.com/prysmaticlabs/prysm/beacon-chain/db/testing"
	p2pt "github.com/prysmaticlabs/prysm/beacon-chain/p2p/testing"
	"github.com/prysmaticlabs/prysm/cmd/beacon-chain/flags"
	eth "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/proto/eth/v1alpha1/wrapper"
	"github.com/prysmaticlabs/prysm/shared/abool"
	"github.com/prysmaticlabs/prysm/shared/event"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/testutil"
	"github.com/prysmaticlabs/prysm/shared/testutil/assert"
	"github.com/prysmaticlabs/prysm/shared/testutil/require"
	logTest "github.com/sirupsen/logrus/hooks/test"
)

func TestService_Constants(t *testing.T) {
	if params.BeaconConfig().MaxPeersToSync*flags.Get().BlockBatchLimit > 1000 {
		t.Fatal("rpc rejects requests over 1000 range slots")
	}
}

func TestService_InitStartStop(t *testing.T) {
	hook := logTest.NewGlobal()
	tests := []struct {
		name         string
		assert       func()
		methodRuns   func(fd *event.Feed)
		chainService func() *mock.ChainService
	}{
		{
			name: "head is not ready",
			assert: func() {
				assert.LogsContain(t, hook, "Waiting for state to be initialized")
			},
		},
		{
			name: "future genesis",
			chainService: func() *mock.ChainService {
				// Set to future time (genesis time hasn't arrived yet).
				st, err := testutil.NewBeaconState()
				require.NoError(t, err)

				return &mock.ChainService{
					State: st,
					FinalizedCheckPoint: &eth.Checkpoint{
						Epoch: 0,
					},
					Genesis:        time.Unix(4113849600, 0),
					ValidatorsRoot: [32]byte{},
				}
			},
			methodRuns: func(fd *event.Feed) {
				// Send valid event.
				fd.Send(&feed.Event{
					Type: statefeed.Initialized,
					Data: &statefeed.InitializedData{
						StartTime:             time.Unix(4113849600, 0),
						GenesisValidatorsRoot: make([]byte, 32),
					},
				})
			},
			assert: func() {
				assert.LogsContain(t, hook, "Genesis time has not arrived - not syncing")
				assert.LogsContain(t, hook, "Waiting for state to be initialized")
			},
		},
		{
			name: "zeroth epoch",
			chainService: func() *mock.ChainService {
				// Set to nearby slot.
				st, err := testutil.NewBeaconState()
				require.NoError(t, err)
				return &mock.ChainService{
					State: st,
					FinalizedCheckPoint: &eth.Checkpoint{
						Epoch: 0,
					},
					Genesis:        time.Now().Add(-5 * time.Minute),
					ValidatorsRoot: [32]byte{},
				}
			},
			methodRuns: func(fd *event.Feed) {
				// Send valid event.
				fd.Send(&feed.Event{
					Type: statefeed.Initialized,
					Data: &statefeed.InitializedData{
						StartTime:             time.Now().Add(-5 * time.Minute),
						GenesisValidatorsRoot: make([]byte, 32),
					},
				})
			},
			assert: func() {
				assert.LogsContain(t, hook, "Chain started within the last epoch - not syncing")
				assert.LogsDoNotContain(t, hook, "Genesis time has not arrived - not syncing")
				assert.LogsContain(t, hook, "Waiting for state to be initialized")
			},
		},
		{
			name: "already synced",
			chainService: func() *mock.ChainService {
				// Set to some future slot, and then make sure that current head matches it.
				st, err := testutil.NewBeaconState()
				require.NoError(t, err)
				futureSlot := types.Slot(27354)
				require.NoError(t, st.SetSlot(futureSlot))
				return &mock.ChainService{
					State: st,
					FinalizedCheckPoint: &eth.Checkpoint{
						Epoch: helpers.SlotToEpoch(futureSlot),
					},
					Genesis:        makeGenesisTime(futureSlot),
					ValidatorsRoot: [32]byte{},
				}
			},
			methodRuns: func(fd *event.Feed) {
				futureSlot := types.Slot(27354)
				// Send valid event.
				fd.Send(&feed.Event{
					Type: statefeed.Initialized,
					Data: &statefeed.InitializedData{
						StartTime:             makeGenesisTime(futureSlot),
						GenesisValidatorsRoot: make([]byte, 32),
					},
				})
			},
			assert: func() {
				assert.LogsContain(t, hook, "Starting initial chain sync...")
				assert.LogsContain(t, hook, "Already synced to the current chain head")
				assert.LogsDoNotContain(t, hook, "Chain started within the last epoch - not syncing")
				assert.LogsDoNotContain(t, hook, "Genesis time has not arrived - not syncing")
				assert.LogsContain(t, hook, "Waiting for state to be initialized")
			},
		},
	}

	p := p2pt.NewTestP2P(t)
	connectPeers(t, p, []*peerData{}, p.Peers())
	for i, tt := range tests {
		if i == 0 {
			continue
		}
		t.Run(tt.name, func(t *testing.T) {
			defer hook.Reset()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			mc := &mock.ChainService{Genesis: time.Now(), ValidatorsRoot: [32]byte{}}
			// Allow overriding with customized chain service.
			if tt.chainService != nil {
				mc = tt.chainService()
			}
			// Initialize feed
			notifier := &mock.MockStateNotifier{}
			s := NewService(ctx, &Config{
				P2P:           p,
				Chain:         mc,
				StateNotifier: notifier,
			})
			time.Sleep(500 * time.Millisecond)
			assert.NotNil(t, s)
			if tt.methodRuns != nil {
				tt.methodRuns(notifier.StateFeed())
			}

			wg := &sync.WaitGroup{}
			wg.Add(1)
			go func() {
				s.Start()
				wg.Done()
			}()

			go func() {
				// Allow to exit from test (on no head loop waiting for head is started).
				// In most tests, this is redundant, as Start() already exited.
				time.AfterFunc(3*time.Second, func() {
					cancel()
				})
			}()
			if testutil.WaitTimeout(wg, time.Second*4) {
				t.Fatalf("Test should have exited by now, timed out")
			}
			tt.assert()
		})
	}
}

func TestService_waitForStateInitialization(t *testing.T) {
	hook := logTest.NewGlobal()
	newService := func(ctx context.Context, mc *mock.ChainService) *Service {
		ctx, cancel := context.WithCancel(ctx)
		s := &Service{
			cfg:          &Config{Chain: mc, StateNotifier: mc.StateNotifier()},
			ctx:          ctx,
			cancel:       cancel,
			synced:       abool.New(),
			chainStarted: abool.New(),
			counter:      ratecounter.NewRateCounter(counterSeconds * time.Second),
			genesisChan:  make(chan time.Time),
		}
		return s
	}

	t.Run("no state and context close", func(t *testing.T) {
		defer hook.Reset()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		s := newService(ctx, &mock.ChainService{Genesis: time.Now(), ValidatorsRoot: [32]byte{}})
		wg := &sync.WaitGroup{}
		wg.Add(1)
		go func() {
			go s.waitForStateInitialization()
			currTime := <-s.genesisChan
			assert.Equal(t, true, currTime.IsZero())
			wg.Done()
		}()
		go func() {
			time.AfterFunc(500*time.Millisecond, func() {
				cancel()
			})
		}()

		if testutil.WaitTimeout(wg, time.Second*2) {
			t.Fatalf("Test should have exited by now, timed out")
		}
		assert.LogsContain(t, hook, "Waiting for state to be initialized")
		assert.LogsContain(t, hook, "Context closed, exiting goroutine")
		assert.LogsDoNotContain(t, hook, "Subscription to state notifier failed")
	})

	t.Run("no state and state init event received", func(t *testing.T) {
		defer hook.Reset()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		s := newService(ctx, &mock.ChainService{Genesis: time.Now(), ValidatorsRoot: [32]byte{}})

		expectedGenesisTime := time.Unix(358544700, 0)
		var receivedGenesisTime time.Time
		wg := &sync.WaitGroup{}
		wg.Add(1)
		go func() {
			go s.waitForStateInitialization()
			receivedGenesisTime = <-s.genesisChan
			assert.Equal(t, false, receivedGenesisTime.IsZero())
			wg.Done()
		}()
		go func() {
			time.AfterFunc(500*time.Millisecond, func() {
				// Send invalid event at first.
				s.cfg.StateNotifier.StateFeed().Send(&feed.Event{
					Type: statefeed.Initialized,
					Data: &statefeed.BlockProcessedData{},
				})
				// Send valid event.
				s.cfg.StateNotifier.StateFeed().Send(&feed.Event{
					Type: statefeed.Initialized,
					Data: &statefeed.InitializedData{
						StartTime:             expectedGenesisTime,
						GenesisValidatorsRoot: make([]byte, 32),
					},
				})
			})
		}()

		if testutil.WaitTimeout(wg, time.Second*2) {
			t.Fatalf("Test should have exited by now, timed out")
		}
		assert.Equal(t, expectedGenesisTime, receivedGenesisTime)
		assert.LogsContain(t, hook, "Event feed data is not type *statefeed.InitializedData")
		assert.LogsContain(t, hook, "Waiting for state to be initialized")
		assert.LogsContain(t, hook, "Received state initialized event")
		assert.LogsDoNotContain(t, hook, "Context closed, exiting goroutine")
	})

	t.Run("no state and state init event received and service start", func(t *testing.T) {
		defer hook.Reset()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		s := newService(ctx, &mock.ChainService{Genesis: time.Now(), ValidatorsRoot: [32]byte{}})
		// Initialize mock feed
		_ = s.cfg.StateNotifier.StateFeed()

		expectedGenesisTime := time.Now().Add(60 * time.Second)
		wg := &sync.WaitGroup{}
		wg.Add(1)
		go func() {
			s.waitForStateInitialization()
			wg.Done()
		}()

		wg.Add(1)
		go func() {
			time.AfterFunc(500*time.Millisecond, func() {
				// Send valid event.
				s.cfg.StateNotifier.StateFeed().Send(&feed.Event{
					Type: statefeed.Initialized,
					Data: &statefeed.InitializedData{
						StartTime:             expectedGenesisTime,
						GenesisValidatorsRoot: make([]byte, 32),
					},
				})
			})
			s.Start()
			wg.Done()
		}()

		if testutil.WaitTimeout(wg, time.Second*5) {
			t.Fatalf("Test should have exited by now, timed out")
		}
		assert.LogsContain(t, hook, "Waiting for state to be initialized")
		assert.LogsContain(t, hook, "Received state initialized event")
		assert.LogsDoNotContain(t, hook, "Context closed, exiting goroutine")
	})
}

func TestService_markSynced(t *testing.T) {
	mc := &mock.ChainService{Genesis: time.Now(), ValidatorsRoot: [32]byte{}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := NewService(ctx, &Config{
		Chain:         mc,
		StateNotifier: mc.StateNotifier(),
	})
	require.NotNil(t, s)
	assert.Equal(t, false, s.chainStarted.IsSet())
	assert.Equal(t, false, s.synced.IsSet())
	assert.Equal(t, true, s.Syncing())
	assert.NoError(t, s.Status())
	s.chainStarted.Set()
	assert.ErrorContains(t, "syncing", s.Status())

	expectedGenesisTime := time.Unix(358544700, 0)
	var receivedGenesisTime time.Time

	stateChannel := make(chan *feed.Event, 1)
	stateSub := s.cfg.StateNotifier.StateFeed().Subscribe(stateChannel)
	defer stateSub.Unsubscribe()

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		select {
		case stateEvent := <-stateChannel:
			if stateEvent.Type == statefeed.Synced {
				data, ok := stateEvent.Data.(*statefeed.SyncedData)
				require.Equal(t, true, ok, "Event feed data is not type *statefeed.SyncedData")
				receivedGenesisTime = data.StartTime
			}
		case <-s.ctx.Done():
		}
		wg.Done()
	}()
	s.markSynced(expectedGenesisTime)

	if testutil.WaitTimeout(wg, time.Second*2) {
		t.Fatalf("Test should have exited by now, timed out")
	}
	assert.Equal(t, expectedGenesisTime, receivedGenesisTime)
	assert.Equal(t, false, s.Syncing())
}

func TestService_Resync(t *testing.T) {
	p := p2pt.NewTestP2P(t)
	connectPeers(t, p, []*peerData{
		{blocks: makeSequence(1, 160), finalizedEpoch: 5, headSlot: 160},
	}, p.Peers())
	cache.initializeRootCache(makeSequence(1, 160), t)
	beaconDB := dbtest.SetupDB(t)
	err := beaconDB.SaveBlock(context.Background(), wrapper.WrappedPhase0SignedBeaconBlock(testutil.NewBeaconBlock()))
	require.NoError(t, err)
	cache.RLock()
	genesisRoot := cache.rootCache[0]
	cache.RUnlock()

	hook := logTest.NewGlobal()
	tests := []struct {
		name         string
		assert       func(s *Service)
		chainService func() *mock.ChainService
		wantedErr    string
	}{
		{
			name:      "no head state",
			wantedErr: "could not retrieve head state",
		},
		{
			name: "resync ok",
			chainService: func() *mock.ChainService {
				st, err := testutil.NewBeaconState()
				require.NoError(t, err)
				futureSlot := types.Slot(160)
				require.NoError(t, st.SetGenesisTime(uint64(makeGenesisTime(futureSlot).Unix())))
				return &mock.ChainService{
					State: st,
					Root:  genesisRoot[:],
					DB:    beaconDB,
					FinalizedCheckPoint: &eth.Checkpoint{
						Epoch: helpers.SlotToEpoch(futureSlot),
					},
					Genesis:        time.Now(),
					ValidatorsRoot: [32]byte{},
				}
			},
			assert: func(s *Service) {
				assert.LogsContain(t, hook, "Resync attempt complete")
				assert.Equal(t, types.Slot(160), s.cfg.Chain.HeadSlot())
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer hook.Reset()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			mc := &mock.ChainService{}
			// Allow overriding with customized chain service.
			if tt.chainService != nil {
				mc = tt.chainService()
			}
			s := NewService(ctx, &Config{
				DB:            beaconDB,
				P2P:           p,
				Chain:         mc,
				StateNotifier: mc.StateNotifier(),
			})
			assert.NotNil(t, s)
			assert.Equal(t, types.Slot(0), s.cfg.Chain.HeadSlot())
			err := s.Resync()
			if tt.wantedErr != "" {
				assert.ErrorContains(t, tt.wantedErr, err)
			} else {
				assert.NoError(t, err)
			}
			if tt.assert != nil {
				tt.assert(s)
			}
		})
	}
}

func TestService_Initialized(t *testing.T) {
	s := NewService(context.Background(), &Config{
		StateNotifier: &mock.MockStateNotifier{},
	})
	s.chainStarted.Set()
	assert.Equal(t, true, s.Initialized())
	s.chainStarted.UnSet()
	assert.Equal(t, false, s.Initialized())
}

func TestService_Synced(t *testing.T) {
	s := NewService(context.Background(), &Config{
		StateNotifier: &mock.MockStateNotifier{},
	})
	s.synced.UnSet()
	assert.Equal(t, false, s.Synced())
	s.synced.Set()
	assert.Equal(t, true, s.Synced())
}
