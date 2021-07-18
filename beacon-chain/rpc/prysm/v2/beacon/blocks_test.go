package beacon

import (
	"context"
	"fmt"
	"strconv"
	"testing"

	types "github.com/prysmaticlabs/eth2-types"
	chainMock "github.com/prysmaticlabs/prysm/beacon-chain/blockchain/testing"
	dbTest "github.com/prysmaticlabs/prysm/beacon-chain/db/testing"
	beaconv1 "github.com/prysmaticlabs/prysm/beacon-chain/rpc/prysm/v1alpha1/beacon"
	ethpb "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/proto/eth/v1alpha1/wrapper"
	"github.com/prysmaticlabs/prysm/proto/interfaces"
	prysmv2 "github.com/prysmaticlabs/prysm/proto/prysm/v2"
	"github.com/prysmaticlabs/prysm/shared/cmd"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/testutil"
	"github.com/prysmaticlabs/prysm/shared/testutil/assert"
	"github.com/prysmaticlabs/prysm/shared/testutil/require"
	"google.golang.org/protobuf/proto"
)

func TestServer_ListBlocks_NoResults(t *testing.T) {
	db := dbTest.SetupDB(t)
	ctx := context.Background()

	bs := &Server{
		V1Server: &beaconv1.Server{BeaconDB: db},
	}
	wanted := &prysmv2.ListBlocksResponseAltair{
		BlockContainers: make([]*prysmv2.BeaconBlockContainerAltair, 0),
		TotalSize:       int32(0),
		NextPageToken:   strconv.Itoa(0),
	}
	res, err := bs.ListBlocks(ctx, &ethpb.ListBlocksRequest{
		QueryFilter: &ethpb.ListBlocksRequest_Slot{
			Slot: 0,
		},
	})
	require.NoError(t, err)
	if !proto.Equal(wanted, res) {
		t.Errorf("Wanted %v, received %v", wanted, res)
	}
	res, err = bs.ListBlocks(ctx, &ethpb.ListBlocksRequest{
		QueryFilter: &ethpb.ListBlocksRequest_Slot{
			Slot: 0,
		},
	})
	require.NoError(t, err)
	if !proto.Equal(wanted, res) {
		t.Errorf("Wanted %v, received %v", wanted, res)
	}
	res, err = bs.ListBlocks(ctx, &ethpb.ListBlocksRequest{
		QueryFilter: &ethpb.ListBlocksRequest_Root{
			Root: make([]byte, 32),
		},
	})
	require.NoError(t, err)
	if !proto.Equal(wanted, res) {
		t.Errorf("Wanted %v, received %v", wanted, res)
	}
}

func TestServer_ListBlocks_Genesis(t *testing.T) {
	db := dbTest.SetupDB(t)
	ctx := context.Background()

	bs := &Server{
		V1Server: &beaconv1.Server{BeaconDB: db},
	}

	// Should throw an error if no genesis block is found.
	_, err := bs.ListBlocks(ctx, &ethpb.ListBlocksRequest{
		QueryFilter: &ethpb.ListBlocksRequest_Genesis{
			Genesis: true,
		},
	})
	require.ErrorContains(t, "Could not find genesis", err)

	// Should return the proper genesis block if it exists.
	parentRoot := [32]byte{'a'}
	blk := testutil.NewBeaconBlock()
	blk.Block.ParentRoot = parentRoot[:]
	root, err := blk.Block.HashTreeRoot()
	require.NoError(t, err)
	require.NoError(t, db.SaveBlock(ctx, wrapper.WrappedPhase0SignedBeaconBlock(blk)))
	require.NoError(t, db.SaveGenesisBlockRoot(ctx, root))
	ctr, err := convertToBlockContainer(wrapper.WrappedPhase0SignedBeaconBlock(blk), root, true)
	assert.NoError(t, err)
	wanted := &prysmv2.ListBlocksResponseAltair{
		BlockContainers: []*prysmv2.BeaconBlockContainerAltair{ctr},
		NextPageToken:   "0",
		TotalSize:       1,
	}
	res, err := bs.ListBlocks(ctx, &ethpb.ListBlocksRequest{
		QueryFilter: &ethpb.ListBlocksRequest_Genesis{
			Genesis: true,
		},
	})
	require.NoError(t, err)
	if !proto.Equal(wanted, res) {
		t.Errorf("Wanted %v, received %v", wanted, res)
	}
}

func TestServer_ListBlocks_Genesis_MultiBlocks(t *testing.T) {
	db := dbTest.SetupDB(t)
	ctx := context.Background()

	bs := &Server{
		V1Server: &beaconv1.Server{BeaconDB: db},
	}
	// Should return the proper genesis block if it exists.
	parentRoot := [32]byte{1, 2, 3}
	blk := testutil.NewBeaconBlock()
	blk.Block.ParentRoot = parentRoot[:]
	root, err := blk.Block.HashTreeRoot()
	require.NoError(t, err)
	require.NoError(t, db.SaveBlock(ctx, wrapper.WrappedPhase0SignedBeaconBlock(blk)))
	require.NoError(t, db.SaveGenesisBlockRoot(ctx, root))

	count := types.Slot(100)
	blks := make([]interfaces.SignedBeaconBlock, count)
	for i := types.Slot(0); i < count; i++ {
		b := testutil.NewBeaconBlock()
		b.Block.Slot = i
		require.NoError(t, err)
		blks[i] = wrapper.WrappedPhase0SignedBeaconBlock(b)
	}
	require.NoError(t, db.SaveBlocks(ctx, blks))

	// Should throw an error if more than one blk returned.
	_, err = bs.ListBlocks(ctx, &ethpb.ListBlocksRequest{
		QueryFilter: &ethpb.ListBlocksRequest_Genesis{
			Genesis: true,
		},
	})
	require.NoError(t, err)
}

func TestServer_ListBlocks_Pagination(t *testing.T) {
	params.UseMinimalConfig()
	defer params.UseMainnetConfig()

	db := dbTest.SetupDB(t)
	chain := &chainMock.ChainService{
		CanonicalRoots: map[[32]byte]bool{},
	}
	ctx := context.Background()

	count := types.Slot(100)
	blks := make([]interfaces.SignedBeaconBlock, count)
	blkContainers := make([]*prysmv2.BeaconBlockContainerAltair, count)
	for i := types.Slot(0); i < count; i++ {
		b := testutil.NewBeaconBlock()
		b.Block.Slot = i
		root, err := b.Block.HashTreeRoot()
		require.NoError(t, err)
		chain.CanonicalRoots[root] = true
		blks[i] = wrapper.WrappedPhase0SignedBeaconBlock(b)
		ctr, err := convertToBlockContainer(blks[i], root, true)
		require.NoError(t, err)
		blkContainers[i] = ctr
	}
	require.NoError(t, db.SaveBlocks(ctx, blks))

	orphanedBlk := testutil.NewBeaconBlock()
	orphanedBlk.Block.Slot = 300
	orphanedBlkRoot, err := orphanedBlk.Block.HashTreeRoot()
	require.NoError(t, err)
	require.NoError(t, db.SaveBlock(ctx, wrapper.WrappedPhase0SignedBeaconBlock(orphanedBlk)))

	bs := &Server{
		V1Server: &beaconv1.Server{BeaconDB: db, CanonicalFetcher: chain},
	}

	root6, err := blks[6].Block().HashTreeRoot()
	require.NoError(t, err)

	tests := []struct {
		req *ethpb.ListBlocksRequest
		res *prysmv2.ListBlocksResponseAltair
	}{
		{req: &ethpb.ListBlocksRequest{
			PageToken:   strconv.Itoa(0),
			QueryFilter: &ethpb.ListBlocksRequest_Slot{Slot: 5},
			PageSize:    3},
			res: &prysmv2.ListBlocksResponseAltair{
				BlockContainers: []*prysmv2.BeaconBlockContainerAltair{{Block: &prysmv2.BeaconBlockContainerAltair_Phase0Block{Phase0Block: testutil.HydrateSignedBeaconBlock(&ethpb.SignedBeaconBlock{
					Block: &ethpb.BeaconBlock{
						Slot: 5}})},
					BlockRoot: blkContainers[5].BlockRoot,
					Canonical: blkContainers[5].Canonical}},
				NextPageToken: "",
				TotalSize:     1,
			},
		},
		{req: &ethpb.ListBlocksRequest{
			PageToken:   strconv.Itoa(0),
			QueryFilter: &ethpb.ListBlocksRequest_Root{Root: root6[:]},
			PageSize:    3},
			res: &prysmv2.ListBlocksResponseAltair{
				BlockContainers: []*prysmv2.BeaconBlockContainerAltair{{Block: &prysmv2.BeaconBlockContainerAltair_Phase0Block{
					Phase0Block: testutil.HydrateSignedBeaconBlock(&ethpb.SignedBeaconBlock{
						Block: &ethpb.BeaconBlock{
							Slot: 6}})},
					BlockRoot: blkContainers[6].BlockRoot,
					Canonical: blkContainers[6].Canonical}},
				TotalSize: 1, NextPageToken: strconv.Itoa(0)}},
		{req: &ethpb.ListBlocksRequest{QueryFilter: &ethpb.ListBlocksRequest_Root{Root: root6[:]}},
			res: &prysmv2.ListBlocksResponseAltair{
				BlockContainers: []*prysmv2.BeaconBlockContainerAltair{{Block: &prysmv2.BeaconBlockContainerAltair_Phase0Block{
					Phase0Block: testutil.HydrateSignedBeaconBlock(&ethpb.SignedBeaconBlock{
						Block: &ethpb.BeaconBlock{
							Slot: 6}})},
					BlockRoot: blkContainers[6].BlockRoot,
					Canonical: blkContainers[6].Canonical}},
				TotalSize: 1, NextPageToken: strconv.Itoa(0)}},
		{req: &ethpb.ListBlocksRequest{
			PageToken:   strconv.Itoa(0),
			QueryFilter: &ethpb.ListBlocksRequest_Epoch{Epoch: 0},
			PageSize:    100},
			res: &prysmv2.ListBlocksResponseAltair{
				BlockContainers: blkContainers[0:params.BeaconConfig().SlotsPerEpoch],
				NextPageToken:   "",
				TotalSize:       int32(params.BeaconConfig().SlotsPerEpoch)}},
		{req: &ethpb.ListBlocksRequest{
			PageToken:   strconv.Itoa(1),
			QueryFilter: &ethpb.ListBlocksRequest_Epoch{Epoch: 5},
			PageSize:    3},
			res: &prysmv2.ListBlocksResponseAltair{
				BlockContainers: blkContainers[43:46],
				NextPageToken:   "2",
				TotalSize:       int32(params.BeaconConfig().SlotsPerEpoch)}},
		{req: &ethpb.ListBlocksRequest{
			PageToken:   strconv.Itoa(1),
			QueryFilter: &ethpb.ListBlocksRequest_Epoch{Epoch: 11},
			PageSize:    7},
			res: &prysmv2.ListBlocksResponseAltair{
				BlockContainers: blkContainers[95:96],
				NextPageToken:   "",
				TotalSize:       int32(params.BeaconConfig().SlotsPerEpoch)}},
		{req: &ethpb.ListBlocksRequest{
			PageToken:   strconv.Itoa(0),
			QueryFilter: &ethpb.ListBlocksRequest_Epoch{Epoch: 12},
			PageSize:    4},
			res: &prysmv2.ListBlocksResponseAltair{
				BlockContainers: blkContainers[96:100],
				NextPageToken:   "",
				TotalSize:       int32(params.BeaconConfig().SlotsPerEpoch / 2)}},
		{req: &ethpb.ListBlocksRequest{
			PageToken:   strconv.Itoa(0),
			QueryFilter: &ethpb.ListBlocksRequest_Slot{Slot: 300},
			PageSize:    3},
			res: &prysmv2.ListBlocksResponseAltair{
				BlockContainers: []*prysmv2.BeaconBlockContainerAltair{{Block: &prysmv2.BeaconBlockContainerAltair_Phase0Block{
					Phase0Block: testutil.HydrateSignedBeaconBlock(&ethpb.SignedBeaconBlock{
						Block: &ethpb.BeaconBlock{
							Slot: 300}})},
					BlockRoot: orphanedBlkRoot[:],
					Canonical: false}},
				NextPageToken: "",
				TotalSize:     1}},
	}

	for i, test := range tests {
		t.Run(fmt.Sprintf("test_%d", i), func(t *testing.T) {
			res, err := bs.ListBlocks(ctx, test.req)
			require.NoError(t, err)
			require.DeepSSZEqual(t, res, test.res)
		})
	}
}

func TestServer_ListBlocks_Errors(t *testing.T) {
	db := dbTest.SetupDB(t)
	ctx := context.Background()

	bs := &Server{
		V1Server: &beaconv1.Server{BeaconDB: db},
	}
	exceedsMax := int32(cmd.Get().MaxRPCPageSize + 1)

	wanted := fmt.Sprintf("Requested page size %d can not be greater than max size %d", exceedsMax, cmd.Get().MaxRPCPageSize)
	req := &ethpb.ListBlocksRequest{PageToken: strconv.Itoa(0), PageSize: exceedsMax}
	_, err := bs.ListBlocks(ctx, req)
	assert.ErrorContains(t, wanted, err)

	wanted = "Must specify a filter criteria for fetching"
	req = &ethpb.ListBlocksRequest{}
	_, err = bs.ListBlocks(ctx, req)
	assert.ErrorContains(t, wanted, err)

	req = &ethpb.ListBlocksRequest{QueryFilter: &ethpb.ListBlocksRequest_Slot{Slot: 0}}
	res, err := bs.ListBlocks(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, 0, len(res.BlockContainers), "Wanted empty list")
	assert.Equal(t, int32(0), res.TotalSize, "Wanted total size 0")

	req = &ethpb.ListBlocksRequest{QueryFilter: &ethpb.ListBlocksRequest_Slot{}}
	res, err = bs.ListBlocks(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, 0, len(res.BlockContainers), "Wanted empty list")
	assert.Equal(t, int32(0), res.TotalSize, "Wanted total size 0")

	req = &ethpb.ListBlocksRequest{QueryFilter: &ethpb.ListBlocksRequest_Root{Root: []byte{'A'}}}
	res, err = bs.ListBlocks(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, 0, len(res.BlockContainers), "Wanted empty list")
	assert.Equal(t, int32(0), res.TotalSize, "Wanted total size 0")

	req = &ethpb.ListBlocksRequest{QueryFilter: &ethpb.ListBlocksRequest_Root{Root: []byte{'A'}}}
	res, err = bs.ListBlocks(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, 0, len(res.BlockContainers), "Wanted empty list")
	assert.Equal(t, int32(0), res.TotalSize, "Wanted total size 0")
}
