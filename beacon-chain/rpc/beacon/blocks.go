package beacon

import (
	"context"
	"strconv"

	ptypes "github.com/gogo/protobuf/types"
	ethpb "github.com/prysmaticlabs/ethereumapis/eth/v1alpha1"
	"github.com/prysmaticlabs/go-ssz"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/feed"
	statefeed "github.com/prysmaticlabs/prysm/beacon-chain/core/feed/state"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	"github.com/prysmaticlabs/prysm/beacon-chain/db/filters"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/prysmaticlabs/prysm/shared/pagination"
	"github.com/prysmaticlabs/prysm/shared/params"
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
) (*ethpb.ListBlocksResponse, error) {
	if int(req.PageSize) > params.BeaconConfig().MaxPageSize {
		return nil, status.Errorf(codes.InvalidArgument, "Requested page size %d can not be greater than max size %d",
			req.PageSize, params.BeaconConfig().MaxPageSize)
	}

	switch q := req.QueryFilter.(type) {
	case *ethpb.ListBlocksRequest_Root:
		blk, err := bs.BeaconDB.Block(ctx, bytesutil.ToBytes32(q.Root))
		if err != nil {
			return nil, status.Errorf(codes.Internal, "Could not retrieve block: %v", err)
		}
		if blk == nil {
			return &ethpb.ListBlocksResponse{
				BlockContainers: make([]*ethpb.BeaconBlockContainer, 0),
				TotalSize:       0,
				NextPageToken:   strconv.Itoa(0),
			}, nil
		}
		root, err := ssz.SigningRoot(blk)
		if err != nil {
			return nil, err
		}

		return &ethpb.ListBlocksResponse{
			BlockContainers: []*ethpb.BeaconBlockContainer{{
				Block:     blk,
				BlockRoot: root[:]},
			},
			TotalSize: 1,
		}, nil

	case *ethpb.ListBlocksRequest_Slot:
		blks, err := bs.BeaconDB.Blocks(ctx, filters.NewFilter().SetStartSlot(q.Slot).SetEndSlot(q.Slot))
		if err != nil {
			return nil, status.Errorf(codes.Internal, "Could not retrieve blocks for slot %d: %v", q.Slot, err)
		}

		numBlks := len(blks)
		if numBlks == 0 {
			return &ethpb.ListBlocksResponse{
				BlockContainers: make([]*ethpb.BeaconBlockContainer, 0),
				TotalSize:       0,
				NextPageToken:   strconv.Itoa(0),
			}, nil
		}

		start, end, nextPageToken, err := pagination.StartAndEndPage(req.PageToken, int(req.PageSize), numBlks)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "Could not paginate blocks: %v", err)
		}

		returnedBlks := blks[start:end]
		containers := make([]*ethpb.BeaconBlockContainer, len(returnedBlks))
		for i, b := range returnedBlks {
			root, err := ssz.SigningRoot(b)
			if err != nil {
				return nil, err
			}
			containers[i] = &ethpb.BeaconBlockContainer{
				Block:     b,
				BlockRoot: root[:],
			}
		}

		return &ethpb.ListBlocksResponse{
			BlockContainers: containers,
			TotalSize:       int32(numBlks),
			NextPageToken:   nextPageToken,
		}, nil
	case *ethpb.ListBlocksRequest_Genesis:
		blks, err := bs.BeaconDB.Blocks(ctx, filters.NewFilter().SetStartSlot(0).SetEndSlot(0))
		if err != nil {
			return nil, status.Errorf(codes.Internal, "Could not retrieve blocks for genesis slot: %v", err)
		}
		numBlks := len(blks)
		if numBlks == 0 {
			return nil, status.Error(codes.Internal, "Could not find genesis block")
		}
		if numBlks != 1 {
			return nil, status.Error(codes.Internal, "Found more than 1 genesis block")
		}
		root, err := ssz.SigningRoot(blks[0])
		if err != nil {
			return nil, err
		}
		containers := []*ethpb.BeaconBlockContainer{
			{
				Block:     blks[0],
				BlockRoot: root[:],
			},
		}

		return &ethpb.ListBlocksResponse{
			BlockContainers: containers,
			TotalSize:       int32(1),
			NextPageToken:   strconv.Itoa(0),
		}, nil
	}

	return nil, status.Error(codes.InvalidArgument, "Must specify a filter criteria for fetching blocks")
}

// GetChainHead retrieves information about the head of the beacon chain from
// the view of the beacon chain node.
//
// This includes the head block slot and root as well as information about
// the most recent finalized and justified slots.
func (bs *Server) GetChainHead(ctx context.Context, _ *ptypes.Empty) (*ethpb.ChainHead, error) {
	return bs.chainHeadRetrieval(ctx)
}

// StreamChainHead to clients every single time the head block and state of the chain change.
func (bs *Server) StreamChainHead(_ *ptypes.Empty, stream ethpb.BeaconChain_StreamChainHeadServer) error {
	stateChannel := make(chan *feed.Event, 1)
	stateSub := bs.StateNotifier.StateFeed().Subscribe(stateChannel)
	defer stateSub.Unsubscribe()
	for {
		select {
		case event := <-stateChannel:
			if event.Type == statefeed.BlockProcessed {
				res, err := bs.chainHeadRetrieval(bs.Ctx)
				if err != nil {
					return status.Errorf(codes.Internal, "Could not retrieve chain head: %v", err)
				}
				return stream.Send(res)
			}
		case <-stateSub.Err():
			return status.Error(codes.Aborted, "Subscriber closed, exiting goroutine")
		case <-bs.Ctx.Done():
			return status.Error(codes.Canceled, "Context canceled")
		case <-stream.Context().Done():
			return status.Error(codes.Canceled, "Context canceled")
		}
	}
}

// Retrieve chain head information from the DB and the current beacon state.
func (bs *Server) chainHeadRetrieval(ctx context.Context) (*ethpb.ChainHead, error) {
	headState, err := bs.HeadFetcher.HeadState(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Could not get head state: %v", err)
	}

	headBlock := bs.HeadFetcher.HeadBlock()
	headBlockRoot, err := ssz.SigningRoot(headBlock)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Could not get head block root: %v", err)
	}
	finalizedCheckpoint := headState.FinalizedCheckpoint
	justifiedCheckpoint := headState.CurrentJustifiedCheckpoint

	if headState.Slot == 0 {
		return &ethpb.ChainHead{
			HeadSlot:                   0,
			HeadEpoch:                  0,
			HeadBlockRoot:              headBlockRoot[:],
			FinalizedSlot:              0,
			FinalizedEpoch:             0,
			FinalizedBlockRoot:         finalizedCheckpoint.Root,
			JustifiedSlot:              0,
			JustifiedEpoch:             0,
			JustifiedBlockRoot:         justifiedCheckpoint.Root,
			PreviousJustifiedSlot:      0,
			PreviousJustifiedEpoch:     0,
			PreviousJustifiedBlockRoot: justifiedCheckpoint.Root,
		}, nil
	}

	b, err := bs.BeaconDB.Block(ctx, bytesutil.ToBytes32(finalizedCheckpoint.Root))
	if err != nil || b == nil {
		return nil, status.Error(codes.Internal, "Could not get finalized block")
	}
	finalizedSlot := b.Slot

	b, err = bs.BeaconDB.Block(ctx, bytesutil.ToBytes32(justifiedCheckpoint.Root))
	if err != nil || b == nil {
		return nil, status.Error(codes.Internal, "Could not get justified block")
	}
	justifiedSlot := b.Slot

	prevJustifiedCheckpoint := headState.PreviousJustifiedCheckpoint
	b, err = bs.BeaconDB.Block(ctx, bytesutil.ToBytes32(prevJustifiedCheckpoint.Root))
	if err != nil || b == nil {
		return nil, status.Error(codes.Internal, "Could not get prev justified block")
	}
	prevJustifiedSlot := b.Slot

	return &ethpb.ChainHead{
		HeadSlot:                   headBlock.Slot,
		HeadEpoch:                  helpers.SlotToEpoch(headBlock.Slot),
		HeadBlockRoot:              headBlockRoot[:],
		FinalizedSlot:              finalizedSlot,
		FinalizedEpoch:             finalizedCheckpoint.Epoch,
		FinalizedBlockRoot:         finalizedCheckpoint.Root,
		JustifiedSlot:              justifiedSlot,
		JustifiedEpoch:             justifiedCheckpoint.Epoch,
		JustifiedBlockRoot:         justifiedCheckpoint.Root,
		PreviousJustifiedSlot:      prevJustifiedSlot,
		PreviousJustifiedEpoch:     prevJustifiedCheckpoint.Epoch,
		PreviousJustifiedBlockRoot: prevJustifiedCheckpoint.Root,
	}, nil
}
