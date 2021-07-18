package kv

import (
	"context"
	"sort"
	"testing"

	types "github.com/prysmaticlabs/eth2-types"
	"github.com/prysmaticlabs/prysm/beacon-chain/db/filters"
	"github.com/prysmaticlabs/prysm/proto/interfaces"
	"github.com/prysmaticlabs/prysm/proto/prysm/v2/wrapper"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/testutil"
	"github.com/prysmaticlabs/prysm/shared/testutil/assert"
	"github.com/prysmaticlabs/prysm/shared/testutil/require"
	"google.golang.org/protobuf/proto"
)

func TestStore_SaveAltairBlock_NoDuplicates(t *testing.T) {
	BlockCacheSize = 1
	db := setupDB(t)
	slot := types.Slot(20)
	ctx := context.Background()
	// First we save a previous block to ensure the cache max size is reached.
	prevBlock := testutil.NewBeaconBlockAltair()
	prevBlock.Block.Slot = slot - 1
	prevBlock.Block.ParentRoot = bytesutil.PadTo([]byte{1, 2, 3}, 32)
	require.NoError(t, db.SaveBlock(ctx, wrapper.WrappedAltairSignedBeaconBlock(prevBlock)))

	block := testutil.NewBeaconBlockAltair()
	block.Block.Slot = slot
	block.Block.ParentRoot = bytesutil.PadTo([]byte{1, 2, 3}, 32)
	// Even with a full cache, saving new blocks should not cause
	// duplicated blocks in the DB.
	for i := 0; i < 100; i++ {
		require.NoError(t, db.SaveBlock(ctx, wrapper.WrappedAltairSignedBeaconBlock(block)))
	}
	f := filters.NewFilter().SetStartSlot(slot).SetEndSlot(slot)
	retrieved, _, err := db.Blocks(ctx, f)
	require.NoError(t, err)
	assert.Equal(t, 1, len(retrieved))
	// We reset the block cache size.
	BlockCacheSize = 256
}

func TestStore_AltairBlocksCRUD(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	block := testutil.NewBeaconBlockAltair()
	block.Block.Slot = 20
	block.Block.ParentRoot = bytesutil.PadTo([]byte{1, 2, 3}, 32)

	blockRoot, err := block.Block.HashTreeRoot()
	require.NoError(t, err)
	retrievedBlock, err := db.Block(ctx, blockRoot)
	require.NoError(t, err)
	assert.DeepEqual(t, nil, retrievedBlock, "Expected nil block")
	require.NoError(t, db.SaveBlock(ctx, wrapper.WrappedAltairSignedBeaconBlock(block)))
	assert.Equal(t, true, db.HasBlock(ctx, blockRoot), "Expected block to exist in the db")
	retrievedBlock, err = db.Block(ctx, blockRoot)
	require.NoError(t, err)
	assert.Equal(t, true, proto.Equal(block, retrievedBlock.Proto()), "Wanted: %v, received: %v", block, retrievedBlock)
	require.NoError(t, db.deleteBlock(ctx, blockRoot))
	assert.Equal(t, false, db.HasBlock(ctx, blockRoot), "Expected block to have been deleted from the db")
}

func TestStore_AltairBlocksBatchDelete(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()
	numBlocks := 10
	totalBlocks := make([]interfaces.SignedBeaconBlock, numBlocks)
	blockRoots := make([][32]byte, 0)
	oddBlocks := make([]interfaces.SignedBeaconBlock, 0)
	for i := 0; i < len(totalBlocks); i++ {
		b := testutil.NewBeaconBlockAltair()
		b.Block.Slot = types.Slot(i)
		b.Block.ParentRoot = bytesutil.PadTo([]byte("parent"), 32)
		totalBlocks[i] = wrapper.WrappedAltairSignedBeaconBlock(b)
		if i%2 == 0 {
			r, err := totalBlocks[i].Block().HashTreeRoot()
			require.NoError(t, err)
			blockRoots = append(blockRoots, r)
		} else {
			oddBlocks = append(oddBlocks, totalBlocks[i])
		}
	}
	require.NoError(t, db.SaveBlocks(ctx, totalBlocks))
	retrieved, _, err := db.Blocks(ctx, filters.NewFilter().SetParentRoot(bytesutil.PadTo([]byte("parent"), 32)))
	require.NoError(t, err)
	assert.Equal(t, numBlocks, len(retrieved), "Unexpected number of blocks received")
	// We delete all even indexed blocks.
	require.NoError(t, db.deleteBlocks(ctx, blockRoots))
	// When we retrieve the data, only the odd indexed blocks should remain.
	retrieved, _, err = db.Blocks(ctx, filters.NewFilter().SetParentRoot(bytesutil.PadTo([]byte("parent"), 32)))
	require.NoError(t, err)
	sort.Slice(retrieved, func(i, j int) bool {
		return retrieved[i].Block().Slot() < retrieved[j].Block().Slot()
	})
	for i, block := range retrieved {
		assert.Equal(t, true, proto.Equal(block.Proto(), oddBlocks[i].Proto()), "Wanted: %v, received: %v", block, oddBlocks[i])
	}
}

func TestStore_AltairBlocksHandleZeroCase(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()
	numBlocks := 10
	totalBlocks := make([]interfaces.SignedBeaconBlock, numBlocks)
	for i := 0; i < len(totalBlocks); i++ {
		b := testutil.NewBeaconBlockAltair()
		b.Block.Slot = types.Slot(i)
		b.Block.ParentRoot = bytesutil.PadTo([]byte("parent"), 32)
		totalBlocks[i] = wrapper.WrappedAltairSignedBeaconBlock(b)
		_, err := totalBlocks[i].Block().HashTreeRoot()
		require.NoError(t, err)
	}
	require.NoError(t, db.SaveBlocks(ctx, totalBlocks))
	zeroFilter := filters.NewFilter().SetStartSlot(0).SetEndSlot(0)
	retrieved, _, err := db.Blocks(ctx, zeroFilter)
	require.NoError(t, err)
	assert.Equal(t, 1, len(retrieved), "Unexpected number of blocks received, expected one")
}

func TestStore_AltairBlocksHandleInvalidEndSlot(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()
	numBlocks := 10
	totalBlocks := make([]interfaces.SignedBeaconBlock, numBlocks)
	// Save blocks from slot 1 onwards.
	for i := 0; i < len(totalBlocks); i++ {
		b := testutil.NewBeaconBlockAltair()
		b.Block.Slot = types.Slot(i) + 1
		b.Block.ParentRoot = bytesutil.PadTo([]byte("parent"), 32)
		totalBlocks[i] = wrapper.WrappedAltairSignedBeaconBlock(b)
		_, err := totalBlocks[i].Block().HashTreeRoot()
		require.NoError(t, err)
	}
	require.NoError(t, db.SaveBlocks(ctx, totalBlocks))
	badFilter := filters.NewFilter().SetStartSlot(5).SetEndSlot(1)
	_, _, err := db.Blocks(ctx, badFilter)
	require.ErrorContains(t, errInvalidSlotRange.Error(), err)

	goodFilter := filters.NewFilter().SetStartSlot(0).SetEndSlot(1)
	requested, _, err := db.Blocks(ctx, goodFilter)
	require.NoError(t, err)
	assert.Equal(t, 1, len(requested), "Unexpected number of blocks received, only expected two")
}

func TestStore_AltairBlocksCRUD_NoCache(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()
	block := testutil.NewBeaconBlockAltair()
	block.Block.Slot = 20
	block.Block.ParentRoot = bytesutil.PadTo([]byte{1, 2, 3}, 32)
	blockRoot, err := block.Block.HashTreeRoot()
	require.NoError(t, err)
	retrievedBlock, err := db.Block(ctx, blockRoot)
	require.NoError(t, err)
	require.DeepEqual(t, nil, retrievedBlock, "Expected nil block")
	require.NoError(t, db.SaveBlock(ctx, wrapper.WrappedAltairSignedBeaconBlock(block)))
	db.blockCache.Del(string(blockRoot[:]))
	assert.Equal(t, true, db.HasBlock(ctx, blockRoot), "Expected block to exist in the db")
	retrievedBlock, err = db.Block(ctx, blockRoot)
	require.NoError(t, err)
	assert.Equal(t, true, proto.Equal(block, retrievedBlock.Proto()), "Wanted: %v, received: %v", block, retrievedBlock)
	require.NoError(t, db.deleteBlock(ctx, blockRoot))
	assert.Equal(t, false, db.HasBlock(ctx, blockRoot), "Expected block to have been deleted from the db")
}

func TestStore_AltairBlocks_FiltersCorrectly(t *testing.T) {
	db := setupDB(t)
	b4 := testutil.NewBeaconBlockAltair()
	b4.Block.Slot = 4
	b4.Block.ParentRoot = bytesutil.PadTo([]byte("parent"), 32)
	b5 := testutil.NewBeaconBlockAltair()
	b5.Block.Slot = 5
	b5.Block.ParentRoot = bytesutil.PadTo([]byte("parent2"), 32)
	b6 := testutil.NewBeaconBlockAltair()
	b6.Block.Slot = 6
	b6.Block.ParentRoot = bytesutil.PadTo([]byte("parent2"), 32)
	b7 := testutil.NewBeaconBlockAltair()
	b7.Block.Slot = 7
	b7.Block.ParentRoot = bytesutil.PadTo([]byte("parent3"), 32)
	b8 := testutil.NewBeaconBlockAltair()
	b8.Block.Slot = 8
	b8.Block.ParentRoot = bytesutil.PadTo([]byte("parent4"), 32)
	blocks := []interfaces.SignedBeaconBlock{
		wrapper.WrappedAltairSignedBeaconBlock(b4),
		wrapper.WrappedAltairSignedBeaconBlock(b5),
		wrapper.WrappedAltairSignedBeaconBlock(b6),
		wrapper.WrappedAltairSignedBeaconBlock(b7),
		wrapper.WrappedAltairSignedBeaconBlock(b8),
	}
	ctx := context.Background()
	require.NoError(t, db.SaveBlocks(ctx, blocks))

	tests := []struct {
		filter            *filters.QueryFilter
		expectedNumBlocks int
	}{
		{
			filter:            filters.NewFilter().SetParentRoot(bytesutil.PadTo([]byte("parent2"), 32)),
			expectedNumBlocks: 2,
		},
		{
			// No block meets the criteria below.
			filter:            filters.NewFilter().SetParentRoot(bytesutil.PadTo([]byte{3, 4, 5}, 32)),
			expectedNumBlocks: 0,
		},
		{
			// Block slot range filter criteria.
			filter:            filters.NewFilter().SetStartSlot(5).SetEndSlot(7),
			expectedNumBlocks: 3,
		},
		{
			filter:            filters.NewFilter().SetStartSlot(7).SetEndSlot(7),
			expectedNumBlocks: 1,
		},
		{
			filter:            filters.NewFilter().SetStartSlot(4).SetEndSlot(8),
			expectedNumBlocks: 5,
		},
		{
			filter:            filters.NewFilter().SetStartSlot(4).SetEndSlot(5),
			expectedNumBlocks: 2,
		},
		{
			filter:            filters.NewFilter().SetStartSlot(5).SetEndSlot(9),
			expectedNumBlocks: 4,
		},
		{
			filter:            filters.NewFilter().SetEndSlot(7),
			expectedNumBlocks: 4,
		},
		{
			filter:            filters.NewFilter().SetEndSlot(8),
			expectedNumBlocks: 5,
		},
		{
			filter:            filters.NewFilter().SetStartSlot(5).SetEndSlot(10),
			expectedNumBlocks: 4,
		},
		{
			// Composite filter criteria.
			filter: filters.NewFilter().
				SetParentRoot(bytesutil.PadTo([]byte("parent2"), 32)).
				SetStartSlot(6).
				SetEndSlot(8),
			expectedNumBlocks: 1,
		},
	}
	for _, tt := range tests {
		retrievedBlocks, _, err := db.Blocks(ctx, tt.filter)
		require.NoError(t, err)
		assert.Equal(t, tt.expectedNumBlocks, len(retrievedBlocks), "Unexpected number of blocks")
	}
}

func TestStore_AltairBlocks_VerifyBlockRoots(t *testing.T) {
	ctx := context.Background()
	db := setupDB(t)
	b1 := testutil.NewBeaconBlockAltair()
	b1.Block.Slot = 1
	r1, err := b1.Block.HashTreeRoot()
	require.NoError(t, err)
	b2 := testutil.NewBeaconBlockAltair()
	b2.Block.Slot = 2
	r2, err := b2.Block.HashTreeRoot()
	require.NoError(t, err)

	require.NoError(t, db.SaveBlock(ctx, wrapper.WrappedAltairSignedBeaconBlock(b1)))
	require.NoError(t, db.SaveBlock(ctx, wrapper.WrappedAltairSignedBeaconBlock(b2)))

	filter := filters.NewFilter().SetStartSlot(b1.Block.Slot).SetEndSlot(b2.Block.Slot)
	roots, err := db.BlockRoots(ctx, filter)
	require.NoError(t, err)

	assert.DeepEqual(t, [][32]byte{r1, r2}, roots)
}

func TestStore_AltairBlocks_Retrieve_SlotRange(t *testing.T) {
	db := setupDB(t)
	totalBlocks := make([]interfaces.SignedBeaconBlock, 500)
	for i := 0; i < 500; i++ {
		b := testutil.NewBeaconBlockAltair()
		b.Block.Slot = types.Slot(i)
		b.Block.ParentRoot = bytesutil.PadTo([]byte("parent"), 32)
		totalBlocks[i] = wrapper.WrappedAltairSignedBeaconBlock(b)
	}
	ctx := context.Background()
	require.NoError(t, db.SaveBlocks(ctx, totalBlocks))
	retrieved, _, err := db.Blocks(ctx, filters.NewFilter().SetStartSlot(100).SetEndSlot(399))
	require.NoError(t, err)
	assert.Equal(t, 300, len(retrieved))
}

func TestStore_AltairBlocks_Retrieve_Epoch(t *testing.T) {
	db := setupDB(t)
	slots := params.BeaconConfig().SlotsPerEpoch.Mul(7)
	totalBlocks := make([]interfaces.SignedBeaconBlock, slots)
	for i := types.Slot(0); i < slots; i++ {
		b := testutil.NewBeaconBlockAltair()
		b.Block.Slot = i
		b.Block.ParentRoot = bytesutil.PadTo([]byte("parent"), 32)
		totalBlocks[i] = wrapper.WrappedAltairSignedBeaconBlock(b)
	}
	ctx := context.Background()
	require.NoError(t, db.SaveBlocks(ctx, totalBlocks))
	retrieved, _, err := db.Blocks(ctx, filters.NewFilter().SetStartEpoch(5).SetEndEpoch(6))
	require.NoError(t, err)
	want := params.BeaconConfig().SlotsPerEpoch.Mul(2)
	assert.Equal(t, uint64(want), uint64(len(retrieved)))
	retrieved, _, err = db.Blocks(ctx, filters.NewFilter().SetStartEpoch(0).SetEndEpoch(0))
	require.NoError(t, err)
	want = params.BeaconConfig().SlotsPerEpoch
	assert.Equal(t, uint64(want), uint64(len(retrieved)))
}

func TestStore_AltairBlocks_Retrieve_SlotRangeWithStep(t *testing.T) {
	db := setupDB(t)
	totalBlocks := make([]interfaces.SignedBeaconBlock, 500)
	for i := 0; i < 500; i++ {
		b := testutil.NewBeaconBlockAltair()
		b.Block.Slot = types.Slot(i)
		b.Block.ParentRoot = bytesutil.PadTo([]byte("parent"), 32)
		totalBlocks[i] = wrapper.WrappedAltairSignedBeaconBlock(b)
	}
	const step = 2
	ctx := context.Background()
	require.NoError(t, db.SaveBlocks(ctx, totalBlocks))
	retrieved, _, err := db.Blocks(ctx, filters.NewFilter().SetStartSlot(100).SetEndSlot(399).SetSlotStep(step))
	require.NoError(t, err)
	assert.Equal(t, 150, len(retrieved))
	for _, b := range retrieved {
		assert.Equal(t, types.Slot(0), (b.Block().Slot()-100)%step, "Unexpect block slot %d", b.Block().Slot())
	}
}

func TestStore_SaveAltairBlock_CanGetHighestAt(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	block1 := testutil.NewBeaconBlockAltair()
	block1.Block.Slot = 1
	require.NoError(t, db.SaveBlock(ctx, wrapper.WrappedAltairSignedBeaconBlock(block1)))
	block2 := testutil.NewBeaconBlockAltair()
	block2.Block.Slot = 10
	require.NoError(t, db.SaveBlock(ctx, wrapper.WrappedAltairSignedBeaconBlock(block2)))
	block3 := testutil.NewBeaconBlockAltair()
	block3.Block.Slot = 100
	require.NoError(t, db.SaveBlock(ctx, wrapper.WrappedAltairSignedBeaconBlock(block3)))

	highestAt, err := db.HighestSlotBlocksBelow(ctx, 2)
	require.NoError(t, err)
	assert.Equal(t, false, len(highestAt) <= 0, "Got empty highest at slice")
	assert.Equal(t, true, proto.Equal(block1, highestAt[0].Proto()), "Wanted: %v, received: %v", block1, highestAt[0])
	highestAt, err = db.HighestSlotBlocksBelow(ctx, 11)
	require.NoError(t, err)
	assert.Equal(t, false, len(highestAt) <= 0, "Got empty highest at slice")
	assert.Equal(t, true, proto.Equal(block2, highestAt[0].Proto()), "Wanted: %v, received: %v", block2, highestAt[0])
	highestAt, err = db.HighestSlotBlocksBelow(ctx, 101)
	require.NoError(t, err)
	assert.Equal(t, false, len(highestAt) <= 0, "Got empty highest at slice")
	assert.Equal(t, true, proto.Equal(block3, highestAt[0].Proto()), "Wanted: %v, received: %v", block3, highestAt[0])

	r3, err := block3.Block.HashTreeRoot()
	require.NoError(t, err)
	require.NoError(t, db.deleteBlock(ctx, r3))

	highestAt, err = db.HighestSlotBlocksBelow(ctx, 101)
	require.NoError(t, err)
	assert.Equal(t, true, proto.Equal(block2, highestAt[0].Proto()), "Wanted: %v, received: %v", block2, highestAt[0])
}

func TestStore_GenesisAltairBlock_CanGetHighestAt(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	genesisBlock := testutil.NewBeaconBlockAltair()
	genesisRoot, err := genesisBlock.Block.HashTreeRoot()
	require.NoError(t, err)
	require.NoError(t, db.SaveGenesisBlockRoot(ctx, genesisRoot))
	require.NoError(t, db.SaveBlock(ctx, wrapper.WrappedAltairSignedBeaconBlock(genesisBlock)))
	block1 := testutil.NewBeaconBlockAltair()
	block1.Block.Slot = 1
	require.NoError(t, db.SaveBlock(ctx, wrapper.WrappedAltairSignedBeaconBlock(block1)))

	highestAt, err := db.HighestSlotBlocksBelow(ctx, 2)
	require.NoError(t, err)
	assert.Equal(t, true, proto.Equal(block1, highestAt[0].Proto()), "Wanted: %v, received: %v", block1, highestAt[0])
	highestAt, err = db.HighestSlotBlocksBelow(ctx, 1)
	require.NoError(t, err)
	assert.Equal(t, true, proto.Equal(genesisBlock, highestAt[0].Proto()), "Wanted: %v, received: %v", genesisBlock, highestAt[0])
	highestAt, err = db.HighestSlotBlocksBelow(ctx, 0)
	require.NoError(t, err)
	assert.Equal(t, true, proto.Equal(genesisBlock, highestAt[0].Proto()), "Wanted: %v, received: %v", genesisBlock, highestAt[0])
}

func TestStore_SaveAltairBlocks_HasCachedBlocks(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	b := make([]interfaces.SignedBeaconBlock, 500)
	for i := 0; i < 500; i++ {
		blk := testutil.NewBeaconBlockAltair()
		blk.Block.ParentRoot = bytesutil.PadTo([]byte("parent"), 32)
		blk.Block.Slot = types.Slot(i)
		b[i] = wrapper.WrappedAltairSignedBeaconBlock(blk)
	}

	require.NoError(t, db.SaveBlock(ctx, b[0]))
	require.NoError(t, db.SaveBlocks(ctx, b))
	f := filters.NewFilter().SetStartSlot(0).SetEndSlot(500)

	blks, _, err := db.Blocks(ctx, f)
	require.NoError(t, err)
	assert.Equal(t, 500, len(blks), "Did not get wanted blocks")
}

func TestStore_SaveAltairBlocks_HasRootsMatched(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	b := make([]interfaces.SignedBeaconBlock, 500)
	for i := 0; i < 500; i++ {
		blk := testutil.NewBeaconBlockAltair()
		blk.Block.ParentRoot = bytesutil.PadTo([]byte("parent"), 32)
		blk.Block.Slot = types.Slot(i)
		b[i] = wrapper.WrappedAltairSignedBeaconBlock(blk)
	}

	require.NoError(t, db.SaveBlocks(ctx, b))
	f := filters.NewFilter().SetStartSlot(0).SetEndSlot(500)

	blks, roots, err := db.Blocks(ctx, f)
	require.NoError(t, err)
	assert.Equal(t, 500, len(blks), "Did not get wanted blocks")

	for i, blk := range blks {
		rt, err := blk.Block().HashTreeRoot()
		require.NoError(t, err)
		assert.Equal(t, roots[i], rt, "mismatch of block roots")
	}
}

func TestStore_AltairBlocksBySlot_BlockRootsBySlot(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	b1 := testutil.NewBeaconBlockAltair()
	b1.Block.Slot = 20
	require.NoError(t, db.SaveBlock(ctx, wrapper.WrappedAltairSignedBeaconBlock(b1)))
	b2 := testutil.NewBeaconBlockAltair()
	b2.Block.Slot = 100
	b2.Block.ParentRoot = bytesutil.PadTo([]byte("parent1"), 32)
	require.NoError(t, db.SaveBlock(ctx, wrapper.WrappedAltairSignedBeaconBlock(b2)))
	b3 := testutil.NewBeaconBlockAltair()
	b3.Block.Slot = 100
	b3.Block.ParentRoot = bytesutil.PadTo([]byte("parent2"), 32)
	require.NoError(t, db.SaveBlock(ctx, wrapper.WrappedAltairSignedBeaconBlock(b3)))

	r1, err := b1.Block.HashTreeRoot()
	require.NoError(t, err)
	r2, err := b2.Block.HashTreeRoot()
	require.NoError(t, err)
	r3, err := b3.Block.HashTreeRoot()
	require.NoError(t, err)

	hasBlocks, retrievedBlocks, err := db.BlocksBySlot(ctx, 1)
	require.NoError(t, err)
	assert.Equal(t, 0, len(retrievedBlocks), "Unexpected number of blocks received, expected none")
	assert.Equal(t, false, hasBlocks, "Expected no blocks")
	hasBlocks, retrievedBlocks, err = db.BlocksBySlot(ctx, 20)
	require.NoError(t, err)
	assert.Equal(t, true, proto.Equal(b1, retrievedBlocks[0].Proto()), "Wanted: %v, received: %v", b1, retrievedBlocks[0])
	assert.Equal(t, true, hasBlocks, "Expected to have blocks")
	hasBlocks, retrievedBlocks, err = db.BlocksBySlot(ctx, 100)
	require.NoError(t, err)
	assert.Equal(t, true, proto.Equal(b2, retrievedBlocks[0].Proto()), "Wanted: %v, received: %v", b2, retrievedBlocks[0])
	assert.Equal(t, true, proto.Equal(b3, retrievedBlocks[1].Proto()), "Wanted: %v, received: %v", b3, retrievedBlocks[1])
	assert.Equal(t, true, hasBlocks, "Expected to have blocks")

	hasBlockRoots, retrievedBlockRoots, err := db.BlockRootsBySlot(ctx, 1)
	require.NoError(t, err)
	assert.DeepEqual(t, [][32]byte{}, retrievedBlockRoots)
	assert.Equal(t, false, hasBlockRoots, "Expected no block roots")
	hasBlockRoots, retrievedBlockRoots, err = db.BlockRootsBySlot(ctx, 20)
	require.NoError(t, err)
	assert.DeepEqual(t, [][32]byte{r1}, retrievedBlockRoots)
	assert.Equal(t, true, hasBlockRoots, "Expected no block roots")
	hasBlockRoots, retrievedBlockRoots, err = db.BlockRootsBySlot(ctx, 100)
	require.NoError(t, err)
	assert.DeepEqual(t, [][32]byte{r2, r3}, retrievedBlockRoots)
	assert.Equal(t, true, hasBlockRoots, "Expected no block roots")
}
