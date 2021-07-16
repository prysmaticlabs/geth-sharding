package slasher

import (
	"context"
	"fmt"
	"io/ioutil"
	"testing"
	"time"

	types "github.com/prysmaticlabs/eth2-types"
	mock "github.com/prysmaticlabs/prysm/beacon-chain/blockchain/testing"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/feed"
	statefeed "github.com/prysmaticlabs/prysm/beacon-chain/core/feed/state"
	dbtest "github.com/prysmaticlabs/prysm/beacon-chain/db/testing"
	"github.com/prysmaticlabs/prysm/beacon-chain/state/stategen"
	mockSync "github.com/prysmaticlabs/prysm/beacon-chain/sync/initial-sync/testing"
	ethpb "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/proto/eth/v1alpha1/wrapper"
	"github.com/prysmaticlabs/prysm/proto/interfaces"
	"github.com/prysmaticlabs/prysm/shared/event"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/slotutil"
	"github.com/prysmaticlabs/prysm/shared/testutil"
	"github.com/prysmaticlabs/prysm/shared/testutil/require"
	"github.com/sirupsen/logrus"
	logTest "github.com/sirupsen/logrus/hooks/test"
)

var _ = SlashingChecker(&Service{})
var _ = SlashingChecker(&MockSlashingChecker{})

func TestMain(m *testing.M) {
	logrus.SetLevel(logrus.DebugLevel)
	logrus.SetOutput(ioutil.Discard)

	m.Run()
}

func TestService_waitForBackfill_OK(t *testing.T) {
	beaconDB := dbtest.SetupDB(t)
	slasherDB := dbtest.SetupSlasherDB(t)
	hook := logTest.NewGlobal()

	beaconState, err := testutil.NewBeaconState()
	require.NoError(t, err)
	currentSlot := types.Slot(0)
	require.NoError(t, beaconState.SetSlot(currentSlot))

	mockChain := &mock.ChainService{
		State: beaconState,
		Slot:  &currentSlot,
	}

	ctx := context.Background()
	srv, err := New(ctx, &ServiceConfig{
		IndexedAttestationsFeed: new(event.Feed),
		BeaconBlockHeadersFeed:  new(event.Feed),
		StateNotifier:           &mock.MockStateNotifier{},
		Database:                slasherDB,
		BeaconDatabase:          beaconDB,
		HeadStateFetcher:        mockChain,
		SyncChecker:             &mockSync.Sync{IsSyncing: false},
		StateGen:                stategen.New(beaconDB),
	})
	require.NoError(t, err)

	// Set genesis time to a custom number of epochs ago.
	numEpochs := uint64(8)
	secondsPerSlot := params.BeaconConfig().SecondsPerSlot
	secondsPerEpoch := secondsPerSlot * uint64(params.BeaconConfig().SlotsPerEpoch)
	totalEpochTimeElapsed := numEpochs * secondsPerEpoch
	srv.genesisTime = time.Now().Add(-time.Duration(totalEpochTimeElapsed) * time.Second)

	// Write blocks for every slot from epoch 0 to numEpochs.
	numSlots := numEpochs * uint64(params.BeaconConfig().SlotsPerEpoch)
	blocks := make([]interfaces.SignedBeaconBlock, 0, numSlots)

	for i := uint64(0); i < numSlots; i++ {
		// Create a realistic looking block for the slot.
		require.NoError(t, beaconState.SetSlot(types.Slot(i)))
		sig := make([]byte, 96)
		copy(sig[:], fmt.Sprintf("%d", i))
		blk := testutil.HydrateSignedBeaconBlock(&ethpb.SignedBeaconBlock{
			Block: testutil.HydrateBeaconBlock(&ethpb.BeaconBlock{
				Slot:          types.Slot(i),
				ProposerIndex: types.ValidatorIndex(i),
			}),
			Signature: sig,
		})
		wrap := wrapper.WrappedPhase0SignedBeaconBlock(blk)
		blocks = append(blocks, wrap)

		// Save the state.
		blockRoot, err := blk.Block.HashTreeRoot()
		require.NoError(t, err)
		srv.serviceCfg.StateGen.SaveState(ctx, blockRoot, beaconState)
	}
	require.NoError(t, beaconDB.SaveBlocks(ctx, blocks))

	require.NoError(t, beaconState.SetSlot(types.Slot(0)))
	srv.waitForDataBackfill(types.Epoch(numEpochs))
	require.LogsContain(t, hook, "Beginning slasher data backfill from epoch 0 to 8")
}

func TestService_waitForBackfill_DetectsSlashableBlock(t *testing.T) {
	beaconDB := dbtest.SetupDB(t)
	slasherDB := dbtest.SetupSlasherDB(t)
	hook := logTest.NewGlobal()

	beaconState, err := testutil.NewBeaconState()
	require.NoError(t, err)
	currentSlot := types.Slot(0)
	require.NoError(t, beaconState.SetSlot(currentSlot))

	mockChain := &mock.ChainService{
		State: beaconState,
		Slot:  &currentSlot,
	}

	ctx := context.Background()
	srv, err := New(ctx, &ServiceConfig{
		IndexedAttestationsFeed: new(event.Feed),
		BeaconBlockHeadersFeed:  new(event.Feed),
		StateNotifier:           &mock.MockStateNotifier{},
		Database:                slasherDB,
		BeaconDatabase:          beaconDB,
		HeadStateFetcher:        mockChain,
		SyncChecker:             &mockSync.Sync{IsSyncing: false},
		StateGen:                stategen.New(beaconDB),
	})
	require.NoError(t, err)

	// Set genesis time to a custom number of epochs ago.
	numEpochs := uint64(8)
	secondsPerSlot := params.BeaconConfig().SecondsPerSlot
	secondsPerEpoch := secondsPerSlot * uint64(params.BeaconConfig().SlotsPerEpoch)
	totalEpochTimeElapsed := numEpochs * secondsPerEpoch
	srv.genesisTime = time.Now().Add(-time.Duration(totalEpochTimeElapsed) * time.Second)

	// Write blocks for every slot from epoch 0 to numEpochs.
	numSlots := numEpochs * uint64(params.BeaconConfig().SlotsPerEpoch)
	blocks := make([]interfaces.SignedBeaconBlock, 0, numSlots)

	for i := uint64(0); i < numSlots; i++ {
		// Create a realistic looking block for the slot.
		require.NoError(t, beaconState.SetSlot(types.Slot(i)))
		sig := make([]byte, 96)
		copy(sig[:], fmt.Sprintf("%d", i))
		blk := testutil.HydrateSignedBeaconBlock(&ethpb.SignedBeaconBlock{
			Block: testutil.HydrateBeaconBlock(&ethpb.BeaconBlock{
				Slot:          types.Slot(i),
				ProposerIndex: types.ValidatorIndex(i),
			}),
			Signature: sig,
		})
		wrap := wrapper.WrappedPhase0SignedBeaconBlock(blk)
		blocks = append(blocks, wrap)

		// Save the state.
		blockRoot, err := blk.Block.HashTreeRoot()
		require.NoError(t, err)
		srv.serviceCfg.StateGen.SaveState(ctx, blockRoot, beaconState)
	}
	require.NoError(t, beaconDB.SaveBlocks(ctx, blocks))

	require.NoError(t, beaconState.SetSlot(types.Slot(0)))
	srv.waitForDataBackfill(types.Epoch(numEpochs))
	require.LogsContain(t, hook, "Beginning slasher data backfill from epoch 0 to 8")
}

func BenchmarkService_backfill(b *testing.B) {
	b.StopTimer()
	srv := setupBackfillTest(b)
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		srv.waitForDataBackfill(8)
	}
}

func setupBackfillTest(tb testing.TB) *Service {
	beaconDB := dbtest.SetupDB(tb)
	slasherDB := dbtest.SetupSlasherDB(tb)

	beaconState, err := testutil.NewBeaconState()
	require.NoError(tb, err)
	currentSlot := types.Slot(0)
	require.NoError(tb, beaconState.SetSlot(currentSlot))

	mockChain := &mock.ChainService{
		State: beaconState,
		Slot:  &currentSlot,
	}

	ctx := context.Background()
	srv, err := New(ctx, &ServiceConfig{
		IndexedAttestationsFeed: new(event.Feed),
		BeaconBlockHeadersFeed:  new(event.Feed),
		StateNotifier:           &mock.MockStateNotifier{},
		Database:                slasherDB,
		BeaconDatabase:          beaconDB,
		HeadStateFetcher:        mockChain,
		SyncChecker:             &mockSync.Sync{IsSyncing: false},
		StateGen:                stategen.New(beaconDB),
	})
	require.NoError(tb, err)

	// Set genesis time to a custom number of epochs ago.
	numEpochs := uint64(8)
	secondsPerSlot := params.BeaconConfig().SecondsPerSlot
	secondsPerEpoch := secondsPerSlot * uint64(params.BeaconConfig().SlotsPerEpoch)
	totalEpochTimeElapsed := numEpochs * secondsPerEpoch
	srv.genesisTime = time.Now().Add(-time.Duration(totalEpochTimeElapsed) * time.Second)

	// Write blocks for every slot from epoch 0 to numEpochs.
	numSlots := numEpochs * uint64(params.BeaconConfig().SlotsPerEpoch)
	blocks := make([]interfaces.SignedBeaconBlock, 0, numSlots)

	for i := uint64(0); i < numSlots; i++ {
		// Create a realistic looking block for the slot.
		require.NoError(tb, beaconState.SetSlot(types.Slot(i)))
		sig := make([]byte, 96)
		copy(sig[:], fmt.Sprintf("%d", i))
		blk := testutil.HydrateSignedBeaconBlock(&ethpb.SignedBeaconBlock{
			Block: testutil.HydrateBeaconBlock(&ethpb.BeaconBlock{
				Slot:          types.Slot(i),
				ProposerIndex: types.ValidatorIndex(i),
			}),
			Signature: sig,
		})
		wrap := wrapper.WrappedPhase0SignedBeaconBlock(blk)
		blocks = append(blocks, wrap)

		// Save the state.
		blockRoot, err := blk.Block.HashTreeRoot()
		require.NoError(tb, err)
		srv.serviceCfg.StateGen.SaveState(ctx, blockRoot, beaconState)
	}
	require.NoError(tb, beaconDB.SaveBlocks(ctx, blocks))
	require.NoError(tb, beaconState.SetSlot(types.Slot(0)))
}

func TestService_StartStop_ChainStartEvent(t *testing.T) {
	slasherDB := dbtest.SetupSlasherDB(t)
	hook := logTest.NewGlobal()

	beaconState, err := testutil.NewBeaconState()
	require.NoError(t, err)
	currentSlot := types.Slot(4)
	require.NoError(t, beaconState.SetSlot(currentSlot))
	mockChain := &mock.ChainService{
		State: beaconState,
		Slot:  &currentSlot,
	}

	srv, err := New(context.Background(), &ServiceConfig{
		IndexedAttestationsFeed: new(event.Feed),
		BeaconBlockHeadersFeed:  new(event.Feed),
		StateNotifier:           &mock.MockStateNotifier{},
		Database:                slasherDB,
		HeadStateFetcher:        mockChain,
		SyncChecker:             &mockSync.Sync{IsSyncing: false},
	})
	require.NoError(t, err)
	go srv.Start()
	time.Sleep(time.Millisecond * 100)
	srv.serviceCfg.StateNotifier.StateFeed().Send(&feed.Event{
		Type: statefeed.ChainStarted,
		Data: &statefeed.ChainStartedData{StartTime: time.Now()},
	})
	time.Sleep(time.Millisecond * 100)
	srv.slotTicker = &slotutil.SlotTicker{}
	require.NoError(t, srv.Stop())
	require.NoError(t, srv.Status())
	require.LogsContain(t, hook, "received chain start event")
}

func TestService_StartStop_ChainAlreadyInitialized(t *testing.T) {
	slasherDB := dbtest.SetupSlasherDB(t)
	hook := logTest.NewGlobal()
	beaconState, err := testutil.NewBeaconState()
	require.NoError(t, err)
	currentSlot := types.Slot(4)
	require.NoError(t, beaconState.SetSlot(currentSlot))
	mockChain := &mock.ChainService{
		State: beaconState,
		Slot:  &currentSlot,
	}
	srv, err := New(context.Background(), &ServiceConfig{
		IndexedAttestationsFeed: new(event.Feed),
		BeaconBlockHeadersFeed:  new(event.Feed),
		StateNotifier:           &mock.MockStateNotifier{},
		Database:                slasherDB,
		HeadStateFetcher:        mockChain,
		SyncChecker:             &mockSync.Sync{IsSyncing: false},
	})
	require.NoError(t, err)
	go srv.Start()
	time.Sleep(time.Millisecond * 100)
	srv.serviceCfg.StateNotifier.StateFeed().Send(&feed.Event{
		Type: statefeed.Initialized,
		Data: &statefeed.InitializedData{StartTime: time.Now()},
	})
	time.Sleep(time.Millisecond * 100)
	srv.slotTicker = &slotutil.SlotTicker{}
	require.NoError(t, srv.Stop())
	require.NoError(t, srv.Status())
	require.LogsContain(t, hook, "chain already initialized")
}
