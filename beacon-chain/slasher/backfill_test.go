package slasher

import (
	"context"
	"fmt"
	"testing"
	"time"

	types "github.com/prysmaticlabs/eth2-types"
	mock "github.com/prysmaticlabs/prysm/beacon-chain/blockchain/testing"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/blocks"
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

type backfillTestConfig struct {
	numEpochs              types.Epoch
	proposerSlashingAtSlot types.Slot
}

func TestService_waitForBackfill_OK(t *testing.T) {
	hook := logTest.NewGlobal()
	srv := setupBackfillTest(t, &backfillTestConfig{
		numEpochs: types.Epoch(8),
	})
	srv.waitForDataBackfill(types.Epoch(8))
	require.LogsContain(t, hook, "Beginning slasher data backfill from epoch 0 to 8")
}

func TestService_waitForBackfill_DetectsSlashableBlock(t *testing.T) {
	hook := logTest.NewGlobal()
	srv := setupBackfillTest(t, &backfillTestConfig{
		numEpochs:              types.Epoch(8),
		proposerSlashingAtSlot: 20,
	})
	srv.waitForDataBackfill(types.Epoch(8))
	require.LogsContain(t, hook, "Beginning slasher data backfill from epoch 0 to 8")
	require.LogsContain(t, hook, "Found 1 proposer slashing")
}

func BenchmarkService_backfill(b *testing.B) {
	b.StopTimer()
	srv := setupBackfillTest(b, &backfillTestConfig{
		numEpochs: types.Epoch(8),
	})
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		srv.waitForDataBackfill(8)
	}
}

func setupBackfillTest(tb testing.TB, cfg *backfillTestConfig) *Service {
	ctx := context.Background()
	beaconDB := dbtest.SetupDB(tb)
	slasherDB := dbtest.SetupSlasherDB(tb)

	beaconState, err := testutil.NewBeaconState()
	require.NoError(tb, err)
	currentSlot := types.Slot(0)
	require.NoError(tb, beaconState.SetSlot(currentSlot))
	genesisStateRoot, err := beaconState.HashTreeRoot(ctx)
	require.NoError(tb, err)

	mockChain := &mock.ChainService{
		State: beaconState,
		Slot:  &currentSlot,
	}

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

	genesisBlock := blocks.NewGenesisBlock(genesisStateRoot[:])
	genesisRoot, err := genesisBlock.Block.HashTreeRoot()
	require.NoError(tb, err)
	wrapGenesis := wrapper.WrappedPhase0SignedBeaconBlock(genesisBlock)
	require.NoError(tb, srv.serviceCfg.BeaconDatabase.SaveBlock(ctx, wrapGenesis))
	require.NoError(tb, srv.serviceCfg.StateGen.SaveState(ctx, genesisRoot, beaconState))
	require.NoError(tb, srv.serviceCfg.BeaconDatabase.SaveState(ctx, beaconState, genesisRoot))

	// Set genesis time to a custom number of epochs ago.
	secondsPerSlot := params.BeaconConfig().SecondsPerSlot
	secondsPerEpoch := secondsPerSlot * uint64(params.BeaconConfig().SlotsPerEpoch)
	totalEpochTimeElapsed := uint64(cfg.numEpochs) * secondsPerEpoch
	srv.genesisTime = time.Now().Add(-time.Duration(totalEpochTimeElapsed) * time.Second)

	// Write blocks for every slot from epoch 0 to numEpochs.
	numSlots := uint64(cfg.numEpochs) * uint64(params.BeaconConfig().SlotsPerEpoch)
	blocks := make([]interfaces.SignedBeaconBlock, 0, numSlots)

	// Setup validators in the beacon state for a full test setup.

	for i := uint64(0); i < numSlots; i++ {
		// Create a realistic looking block for the slot.
		require.NoError(tb, beaconState.SetSlot(types.Slot(i)))
		sig := make([]byte, 96)
		copy(sig[:], fmt.Sprintf("%d", i))

		parentRoot := make([]byte, 32)
		if i == 0 {
			parentRoot = genesisRoot[:]
		} else {
			parentRoot = blocks[i-1].Block().ParentRoot()
		}

		blk := testutil.HydrateSignedBeaconBlock(&ethpb.SignedBeaconBlock{
			Block: testutil.HydrateBeaconBlock(&ethpb.BeaconBlock{
				Slot:          types.Slot(i),
				ProposerIndex: types.ValidatorIndex(i),
				ParentRoot:    parentRoot,
			}),
			Signature: sig,
		})
		wrap := wrapper.WrappedPhase0SignedBeaconBlock(blk)
		blocks = append(blocks, wrap)

		// If we specify it, create a slashable block at a certain slot.
		if uint64(cfg.proposerSlashingAtSlot) == i && i != 0 {
			slashableGraffiti := make([]byte, 32)
			copy(slashableGraffiti[:], "slashme")
			blk := testutil.HydrateSignedBeaconBlock(&ethpb.SignedBeaconBlock{
				Block: testutil.HydrateBeaconBlock(&ethpb.BeaconBlock{
					Slot:          types.Slot(i),
					ProposerIndex: types.ValidatorIndex(i),
					Body: testutil.HydrateBeaconBlockBody(&ethpb.BeaconBlockBody{
						Graffiti: slashableGraffiti,
					}),
				}),
				Signature: sig,
			})
			wrap := wrapper.WrappedPhase0SignedBeaconBlock(blk)
			blocks = append(blocks, wrap)
		}

		// Save the state.
		blockRoot, err := blk.Block.HashTreeRoot()
		require.NoError(tb, err)
		require.NoError(tb, srv.serviceCfg.StateGen.SaveState(ctx, blockRoot, beaconState))
		require.NoError(tb, srv.serviceCfg.BeaconDatabase.SaveState(ctx, beaconState, blockRoot))
	}
	require.NoError(tb, beaconDB.SaveBlocks(ctx, blocks))
	require.NoError(tb, beaconState.SetSlot(types.Slot(0)))
	return srv
}

func item() {}
