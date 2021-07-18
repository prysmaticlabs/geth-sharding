package beacon

import (
	"context"

	beaconv1 "github.com/prysmaticlabs/prysm/beacon-chain/rpc/prysm/v1alpha1/beacon"
	ethpb "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/proto/interfaces"
	prysmv2 "github.com/prysmaticlabs/prysm/proto/prysm/v2"
	"github.com/prysmaticlabs/prysm/shared/cmd"
	"github.com/prysmaticlabs/prysm/shared/version"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ListBlocks retrieves blocks by root, slot, or epoch.
//
// The server may return multiple blocks in the case that a slot or epoch is
// provided as the filter criteria. The server may return an empty list when
// no blocks in their database match the filter criteria. This RPC should
// not return NOT_FOUND. Only one filter criteria should be used.
func (bs *Server) ListBlocks(
	ctx context.Context, req *ethpb.ListBlocksRequest,
) (*prysmv2.ListBlocksResponseAltair, error) {
	if int(req.PageSize) > cmd.Get().MaxRPCPageSize {
		return nil, status.Errorf(codes.InvalidArgument, "Requested page size %d can not be greater than max size %d",
			req.PageSize, cmd.Get().MaxRPCPageSize)
	}

	switch q := req.QueryFilter.(type) {
	case *ethpb.ListBlocksRequest_Epoch:
		ctrs, numBlks, nextPageToken, err := bs.V1Server.ListBlocksForEpoch(ctx, req, q)
		if err != nil {
			return nil, err
		}
		altCtrs, err := convertFromV1Containers(ctrs)
		if err != nil {
			return nil, err
		}
		return &prysmv2.ListBlocksResponseAltair{
			BlockContainers: altCtrs,
			TotalSize:       int32(numBlks),
			NextPageToken:   nextPageToken,
		}, nil
	case *ethpb.ListBlocksRequest_Root:
		ctrs, numBlks, nextPageToken, err := bs.V1Server.ListBlocksForRoot(ctx, req, q)
		if err != nil {
			return nil, err
		}
		altCtrs, err := convertFromV1Containers(ctrs)
		if err != nil {
			return nil, err
		}
		return &prysmv2.ListBlocksResponseAltair{
			BlockContainers: altCtrs,
			TotalSize:       int32(numBlks),
			NextPageToken:   nextPageToken,
		}, nil

	case *ethpb.ListBlocksRequest_Slot:
		ctrs, numBlks, nextPageToken, err := bs.V1Server.ListBlocksForSlot(ctx, req, q)
		if err != nil {
			return nil, err
		}
		altCtrs, err := convertFromV1Containers(ctrs)
		if err != nil {
			return nil, err
		}
		return &prysmv2.ListBlocksResponseAltair{
			BlockContainers: altCtrs,
			TotalSize:       int32(numBlks),
			NextPageToken:   nextPageToken,
		}, nil
	case *ethpb.ListBlocksRequest_Genesis:
		ctrs, numBlks, nextPageToken, err := bs.V1Server.ListBlocksForGenesis(ctx, req, q)
		if err != nil {
			return nil, err
		}
		altCtrs, err := convertFromV1Containers(ctrs)
		if err != nil {
			return nil, err
		}
		return &prysmv2.ListBlocksResponseAltair{
			BlockContainers: altCtrs,
			TotalSize:       int32(numBlks),
			NextPageToken:   nextPageToken,
		}, nil
	}

	return nil, status.Error(codes.InvalidArgument, "Must specify a filter criteria for fetching blocks")
}

func convertFromV1Containers(ctrs []beaconv1.BlockContainer) ([]*prysmv2.BeaconBlockContainerAltair, error) {
	protoCtrs := make([]*prysmv2.BeaconBlockContainerAltair, len(ctrs))
	var err error
	for i, c := range ctrs {
		protoCtrs[i], err = convertToBlockContainer(c.Blk, c.Root, c.IsCanonical)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "Could not get block container: %v", err)
		}
	}
	return protoCtrs, nil
}

func convertToBlockContainer(blk interfaces.SignedBeaconBlock, root [32]byte, isCanonical bool) (*prysmv2.BeaconBlockContainerAltair, error) {
	ctr := &prysmv2.BeaconBlockContainerAltair{
		BlockRoot: root[:],
		Canonical: isCanonical,
	}

	switch blk.Version() {
	case version.Phase0:
		rBlk, err := blk.PbPhase0Block()
		if err != nil {
			return nil, err
		}
		ctr.Block = &prysmv2.BeaconBlockContainerAltair_Phase0Block{Phase0Block: rBlk}
	case version.Altair:
		rBlk, err := blk.PbAltairBlock()
		if err != nil {
			return nil, err
		}
		ctr.Block = &prysmv2.BeaconBlockContainerAltair_AltairBlock{AltairBlock: rBlk}
	}
	return ctr, nil
}
