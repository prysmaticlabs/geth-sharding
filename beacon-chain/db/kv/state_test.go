package kv

import (
	"context"
	"reflect"
	"testing"

	types "github.com/prysmaticlabs/eth2-types"
	iface "github.com/prysmaticlabs/prysm/beacon-chain/state/interface"
	ethpb "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/proto/eth/v1alpha1/wrapper"
	"github.com/prysmaticlabs/prysm/proto/interfaces"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/testutil"
	"github.com/prysmaticlabs/prysm/shared/testutil/assert"
	"github.com/prysmaticlabs/prysm/shared/testutil/require"
	"gopkg.in/d4l3k/messagediff.v1"
)

func TestState_CanSaveRetrieve(t *testing.T) {
	db := setupDB(t)

	r := [32]byte{'A'}

	require.Equal(t, false, db.HasState(context.Background(), r))

	st, err := testutil.NewBeaconState()
	require.NoError(t, err)
	require.NoError(t, st.SetSlot(100))

	require.NoError(t, db.SaveState(context.Background(), st, r))
	assert.Equal(t, true, db.HasState(context.Background(), r))

	savedS, err := db.State(context.Background(), r)
	require.NoError(t, err)

	if !reflect.DeepEqual(st.InnerStateUnsafe(), savedS.InnerStateUnsafe()) {
		diff, _ := messagediff.PrettyDiff(st.InnerStateUnsafe(), savedS.InnerStateUnsafe())
		t.Errorf("Did not retrieve saved state: %v", diff)
	}

	savedS, err = db.State(context.Background(), [32]byte{'B'})
	require.NoError(t, err)
	assert.Equal(t, iface.ReadOnlyBeaconState(nil), savedS, "Unsaved state should've been nil")
}

func TestGenesisState_CanSaveRetrieve(t *testing.T) {
	db := setupDB(t)

	headRoot := [32]byte{'B'}

	st, err := testutil.NewBeaconState()
	require.NoError(t, err)
	require.NoError(t, st.SetSlot(1))
	require.NoError(t, db.SaveGenesisBlockRoot(context.Background(), headRoot))
	require.NoError(t, db.SaveState(context.Background(), st, headRoot))

	savedGenesisS, err := db.GenesisState(context.Background())
	require.NoError(t, err)
	assert.DeepSSZEqual(t, st.InnerStateUnsafe(), savedGenesisS.InnerStateUnsafe(), "Did not retrieve saved state")
	require.NoError(t, db.SaveGenesisBlockRoot(context.Background(), [32]byte{'C'}))
}

func TestStore_StatesBatchDelete(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()
	numBlocks := 100
	totalBlocks := make([]interfaces.SignedBeaconBlock, numBlocks)
	blockRoots := make([][32]byte, 0)
	evenBlockRoots := make([][32]byte, 0)
	for i := 0; i < len(totalBlocks); i++ {
		b := testutil.NewBeaconBlock()
		b.Block.Slot = types.Slot(i)
		totalBlocks[i] = wrapper.WrappedPhase0SignedBeaconBlock(b)
		r, err := totalBlocks[i].Block().HashTreeRoot()
		require.NoError(t, err)
		st, err := testutil.NewBeaconState()
		require.NoError(t, err)
		require.NoError(t, st.SetSlot(types.Slot(i)))
		require.NoError(t, db.SaveState(context.Background(), st, r))
		blockRoots = append(blockRoots, r)
		if i%2 == 0 {
			evenBlockRoots = append(evenBlockRoots, r)
		}
	}
	require.NoError(t, db.SaveBlocks(ctx, totalBlocks))
	// We delete all even indexed states.
	require.NoError(t, db.DeleteStates(ctx, evenBlockRoots))
	// When we retrieve the data, only the odd indexed state should remain.
	for _, r := range blockRoots {
		s, err := db.State(context.Background(), r)
		require.NoError(t, err)
		if s == nil {
			continue
		}
		assert.Equal(t, types.Slot(1), s.Slot()%2, "State with slot %d should have been deleted", s.Slot())
	}
}

func TestStore_DeleteGenesisState(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	genesisBlockRoot := [32]byte{'A'}
	require.NoError(t, db.SaveGenesisBlockRoot(ctx, genesisBlockRoot))
	st, err := testutil.NewBeaconState()
	require.NoError(t, err)
	require.NoError(t, st.SetSlot(100))
	require.NoError(t, db.SaveState(ctx, st, genesisBlockRoot))
	wantedErr := "cannot delete genesis, finalized, or head state"
	assert.ErrorContains(t, wantedErr, db.DeleteState(ctx, genesisBlockRoot))
}

func TestStore_DeleteFinalizedState(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	genesis := bytesutil.ToBytes32([]byte{'G', 'E', 'N', 'E', 'S', 'I', 'S'})
	require.NoError(t, db.SaveGenesisBlockRoot(ctx, genesis))

	blk := testutil.NewBeaconBlock()
	blk.Block.ParentRoot = genesis[:]
	blk.Block.Slot = 100

	require.NoError(t, db.SaveBlock(ctx, wrapper.WrappedPhase0SignedBeaconBlock(blk)))

	finalizedBlockRoot, err := blk.Block.HashTreeRoot()
	require.NoError(t, err)

	finalizedState, err := testutil.NewBeaconState()
	require.NoError(t, err)
	require.NoError(t, finalizedState.SetSlot(100))
	require.NoError(t, db.SaveState(ctx, finalizedState, finalizedBlockRoot))
	finalizedCheckpoint := &ethpb.Checkpoint{Root: finalizedBlockRoot[:]}
	require.NoError(t, db.SaveFinalizedCheckpoint(ctx, finalizedCheckpoint))
	wantedErr := "cannot delete genesis, finalized, or head state"
	assert.ErrorContains(t, wantedErr, db.DeleteState(ctx, finalizedBlockRoot))
}

func TestStore_DeleteHeadState(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	genesis := bytesutil.ToBytes32([]byte{'G', 'E', 'N', 'E', 'S', 'I', 'S'})
	require.NoError(t, db.SaveGenesisBlockRoot(ctx, genesis))

	blk := testutil.NewBeaconBlock()
	blk.Block.ParentRoot = genesis[:]
	blk.Block.Slot = 100
	require.NoError(t, db.SaveBlock(ctx, wrapper.WrappedPhase0SignedBeaconBlock(blk)))

	headBlockRoot, err := blk.Block.HashTreeRoot()
	require.NoError(t, err)
	st, err := testutil.NewBeaconState()
	require.NoError(t, err)
	require.NoError(t, st.SetSlot(100))
	require.NoError(t, db.SaveState(ctx, st, headBlockRoot))
	require.NoError(t, db.SaveHeadBlockRoot(ctx, headBlockRoot))
	wantedErr := "cannot delete genesis, finalized, or head state"
	assert.ErrorContains(t, wantedErr, db.DeleteState(ctx, headBlockRoot))
}

func TestStore_SaveDeleteState_CanGetHighestBelow(t *testing.T) {
	db := setupDB(t)

	b := testutil.NewBeaconBlock()
	b.Block.Slot = 1
	r, err := b.Block.HashTreeRoot()
	require.NoError(t, err)
	require.NoError(t, db.SaveBlock(context.Background(), wrapper.WrappedPhase0SignedBeaconBlock(b)))
	st, err := testutil.NewBeaconState()
	require.NoError(t, err)
	require.NoError(t, st.SetSlot(1))
	s0 := st.InnerStateUnsafe()
	require.NoError(t, db.SaveState(context.Background(), st, r))

	b.Block.Slot = 100
	r1, err := b.Block.HashTreeRoot()
	require.NoError(t, err)
	require.NoError(t, db.SaveBlock(context.Background(), wrapper.WrappedPhase0SignedBeaconBlock(b)))
	st, err = testutil.NewBeaconState()
	require.NoError(t, err)
	require.NoError(t, st.SetSlot(100))
	s1 := st.InnerStateUnsafe()
	require.NoError(t, db.SaveState(context.Background(), st, r1))

	b.Block.Slot = 1000
	r2, err := b.Block.HashTreeRoot()
	require.NoError(t, err)
	require.NoError(t, db.SaveBlock(context.Background(), wrapper.WrappedPhase0SignedBeaconBlock(b)))
	st, err = testutil.NewBeaconState()
	require.NoError(t, err)
	require.NoError(t, st.SetSlot(1000))
	s2 := st.InnerStateUnsafe()

	require.NoError(t, db.SaveState(context.Background(), st, r2))

	highest, err := db.HighestSlotStatesBelow(context.Background(), 2)
	require.NoError(t, err)
	assert.DeepSSZEqual(t, highest[0].InnerStateUnsafe(), s0)

	highest, err = db.HighestSlotStatesBelow(context.Background(), 101)
	require.NoError(t, err)
	assert.DeepSSZEqual(t, highest[0].InnerStateUnsafe(), s1)

	highest, err = db.HighestSlotStatesBelow(context.Background(), 1001)
	require.NoError(t, err)
	assert.DeepSSZEqual(t, highest[0].InnerStateUnsafe(), s2)
}

func TestStore_GenesisState_CanGetHighestBelow(t *testing.T) {
	db := setupDB(t)

	genesisState, err := testutil.NewBeaconState()
	require.NoError(t, err)
	genesisRoot := [32]byte{'a'}
	require.NoError(t, db.SaveGenesisBlockRoot(context.Background(), genesisRoot))
	require.NoError(t, db.SaveState(context.Background(), genesisState, genesisRoot))

	b := testutil.NewBeaconBlock()
	b.Block.Slot = 1
	r, err := b.Block.HashTreeRoot()
	require.NoError(t, err)
	require.NoError(t, db.SaveBlock(context.Background(), wrapper.WrappedPhase0SignedBeaconBlock(b)))

	st, err := testutil.NewBeaconState()
	require.NoError(t, err)
	require.NoError(t, st.SetSlot(1))
	require.NoError(t, db.SaveState(context.Background(), st, r))

	highest, err := db.HighestSlotStatesBelow(context.Background(), 2)
	require.NoError(t, err)
	assert.DeepSSZEqual(t, highest[0].InnerStateUnsafe(), st.InnerStateUnsafe())

	highest, err = db.HighestSlotStatesBelow(context.Background(), 1)
	require.NoError(t, err)
	assert.DeepSSZEqual(t, highest[0].InnerStateUnsafe(), genesisState.InnerStateUnsafe())
	highest, err = db.HighestSlotStatesBelow(context.Background(), 0)
	require.NoError(t, err)
	assert.DeepSSZEqual(t, highest[0].InnerStateUnsafe(), genesisState.InnerStateUnsafe())
}

func TestStore_CleanUpDirtyStates_AboveThreshold(t *testing.T) {
	db := setupDB(t)

	genesisState, err := testutil.NewBeaconState()
	require.NoError(t, err)
	genesisRoot := [32]byte{'a'}
	require.NoError(t, db.SaveGenesisBlockRoot(context.Background(), genesisRoot))
	require.NoError(t, db.SaveState(context.Background(), genesisState, genesisRoot))

	bRoots := make([][32]byte, 0)
	slotsPerArchivedPoint := types.Slot(128)
	prevRoot := genesisRoot
	for i := types.Slot(1); i <= slotsPerArchivedPoint; i++ {
		b := testutil.NewBeaconBlock()
		b.Block.Slot = i
		b.Block.ParentRoot = prevRoot[:]
		r, err := b.Block.HashTreeRoot()
		require.NoError(t, err)
		require.NoError(t, db.SaveBlock(context.Background(), wrapper.WrappedPhase0SignedBeaconBlock(b)))
		bRoots = append(bRoots, r)
		prevRoot = r

		st, err := testutil.NewBeaconState()
		require.NoError(t, err)
		require.NoError(t, st.SetSlot(i))
		require.NoError(t, db.SaveState(context.Background(), st, r))
	}

	require.NoError(t, db.SaveFinalizedCheckpoint(context.Background(), &ethpb.Checkpoint{
		Root:  bRoots[len(bRoots)-1][:],
		Epoch: types.Epoch(slotsPerArchivedPoint / params.BeaconConfig().SlotsPerEpoch),
	}))
	require.NoError(t, db.CleanUpDirtyStates(context.Background(), slotsPerArchivedPoint))

	for i, root := range bRoots {
		if types.Slot(i) >= slotsPerArchivedPoint.SubSlot(slotsPerArchivedPoint.Div(3)) {
			require.Equal(t, true, db.HasState(context.Background(), root))
		} else {
			require.Equal(t, false, db.HasState(context.Background(), root))
		}
	}
}

func TestStore_CleanUpDirtyStates_Finalized(t *testing.T) {
	db := setupDB(t)

	genesisState, err := testutil.NewBeaconState()
	require.NoError(t, err)
	genesisRoot := [32]byte{'a'}
	require.NoError(t, db.SaveGenesisBlockRoot(context.Background(), genesisRoot))
	require.NoError(t, db.SaveState(context.Background(), genesisState, genesisRoot))

	for i := types.Slot(1); i <= params.BeaconConfig().SlotsPerEpoch; i++ {
		b := testutil.NewBeaconBlock()
		b.Block.Slot = i
		r, err := b.Block.HashTreeRoot()
		require.NoError(t, err)
		require.NoError(t, db.SaveBlock(context.Background(), wrapper.WrappedPhase0SignedBeaconBlock(b)))

		st, err := testutil.NewBeaconState()
		require.NoError(t, err)
		require.NoError(t, st.SetSlot(i))
		require.NoError(t, db.SaveState(context.Background(), st, r))
	}

	require.NoError(t, db.SaveFinalizedCheckpoint(context.Background(), &ethpb.Checkpoint{Root: genesisRoot[:]}))
	require.NoError(t, db.CleanUpDirtyStates(context.Background(), params.BeaconConfig().SlotsPerEpoch))
	require.Equal(t, true, db.HasState(context.Background(), genesisRoot))
}

func TestStore_CleanUpDirtyStates_DontDeleteNonFinalized(t *testing.T) {
	db := setupDB(t)

	genesisState, err := testutil.NewBeaconState()
	require.NoError(t, err)
	genesisRoot := [32]byte{'a'}
	require.NoError(t, db.SaveGenesisBlockRoot(context.Background(), genesisRoot))
	require.NoError(t, db.SaveState(context.Background(), genesisState, genesisRoot))

	var unfinalizedRoots [][32]byte
	for i := types.Slot(1); i <= params.BeaconConfig().SlotsPerEpoch; i++ {
		b := testutil.NewBeaconBlock()
		b.Block.Slot = i
		r, err := b.Block.HashTreeRoot()
		require.NoError(t, err)
		require.NoError(t, db.SaveBlock(context.Background(), wrapper.WrappedPhase0SignedBeaconBlock(b)))
		unfinalizedRoots = append(unfinalizedRoots, r)

		st, err := testutil.NewBeaconState()
		require.NoError(t, err)
		require.NoError(t, st.SetSlot(i))
		require.NoError(t, db.SaveState(context.Background(), st, r))
	}

	require.NoError(t, db.SaveFinalizedCheckpoint(context.Background(), &ethpb.Checkpoint{Root: genesisRoot[:]}))
	require.NoError(t, db.CleanUpDirtyStates(context.Background(), params.BeaconConfig().SlotsPerEpoch))

	for _, rt := range unfinalizedRoots {
		require.Equal(t, true, db.HasState(context.Background(), rt))
	}
}

func TestAltairState_CanSaveRetrieve(t *testing.T) {
	db := setupDB(t)

	r := [32]byte{'A'}

	require.Equal(t, false, db.HasState(context.Background(), r))

	st, _ := testutil.DeterministicGenesisStateAltair(t, 1)
	require.NoError(t, st.SetSlot(100))

	require.NoError(t, db.SaveState(context.Background(), st, r))
	require.Equal(t, true, db.HasState(context.Background(), r))

	savedS, err := db.State(context.Background(), r)
	require.NoError(t, err)

	require.DeepSSZEqual(t, st.InnerStateUnsafe(), savedS.InnerStateUnsafe())

	savedS, err = db.State(context.Background(), [32]byte{'B'})
	require.NoError(t, err)
	require.Equal(t, iface.ReadOnlyBeaconState(nil), savedS, "Unsaved state should've been nil")
}

func TestAltairState_CanDelete(t *testing.T) {
	db := setupDB(t)

	r := [32]byte{'A'}

	require.Equal(t, false, db.HasState(context.Background(), r))

	st, _ := testutil.DeterministicGenesisStateAltair(t, 1)
	require.NoError(t, st.SetSlot(100))

	require.NoError(t, db.SaveState(context.Background(), st, r))
	require.Equal(t, true, db.HasState(context.Background(), r))

	require.NoError(t, db.DeleteState(context.Background(), r))
	savedS, err := db.State(context.Background(), r)
	require.NoError(t, err)
	require.Equal(t, iface.ReadOnlyBeaconState(nil), savedS, "Unsaved state should've been nil")
}
