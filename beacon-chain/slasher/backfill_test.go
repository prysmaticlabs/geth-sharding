package slasher

import (
	"context"
	"fmt"
	"testing"
	"time"

	types "github.com/prysmaticlabs/eth2-types"
	mock "github.com/prysmaticlabs/prysm/beacon-chain/blockchain/testing"
	dbtest "github.com/prysmaticlabs/prysm/beacon-chain/db/testing"
	"github.com/prysmaticlabs/prysm/beacon-chain/state/stategen"
	mockSync "github.com/prysmaticlabs/prysm/beacon-chain/sync/initial-sync/testing"
	ethpb "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/proto/eth/v1alpha1/wrapper"
	"github.com/prysmaticlabs/prysm/proto/interfaces"
	"github.com/prysmaticlabs/prysm/shared/event"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/testutil"
	"github.com/prysmaticlabs/prysm/shared/testutil/require"
	logTest "github.com/sirupsen/logrus/hooks/test"
)

func TestService_waitForBackfill_OK(t *testing.T) {
	hook := logTest.NewGlobal()
	srv := setupBackfillTest(t, types.Epoch(8))
	srv.waitForDataBackfill(types.Epoch(8))
	require.LogsContain(t, hook, "Beginning slasher data backfill from epoch 0 to 8")
}

func TestService_waitForBackfill_DetectsSlashableBlock(t *testing.T) {
	hook := logTest.NewGlobal()
	srv := setupBackfillTest(t, types.Epoch(8))
	srv.waitForDataBackfill(types.Epoch(8))
	require.LogsContain(t, hook, "Beginning slasher data backfill from epoch 0 to 8")
}

func BenchmarkService_backfill(b *testing.B) {
	b.StopTimer()
	srv := setupBackfillTest(b, types.Epoch(8))
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		srv.waitForDataBackfill(8)
	}
}

func setupBackfillTest(tb testing.TB, numEpochs types.Epoch) *Service {
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
	secondsPerSlot := params.BeaconConfig().SecondsPerSlot
	secondsPerEpoch := secondsPerSlot * uint64(params.BeaconConfig().SlotsPerEpoch)
	totalEpochTimeElapsed := uint64(numEpochs) * secondsPerEpoch
	srv.genesisTime = time.Now().Add(-time.Duration(totalEpochTimeElapsed) * time.Second)

	// Write blocks for every slot from epoch 0 to numEpochs.
	numSlots := uint64(numEpochs) * uint64(params.BeaconConfig().SlotsPerEpoch)
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
	return srv
}
