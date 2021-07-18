package beacon

import (
	"context"
	"strconv"

	"github.com/prysmaticlabs/prysm/beacon-chain/core/blocks"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/feed"
	blockfeed "github.com/prysmaticlabs/prysm/beacon-chain/core/feed/block"
	statefeed "github.com/prysmaticlabs/prysm/beacon-chain/core/feed/state"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	"github.com/prysmaticlabs/prysm/beacon-chain/db/filters"
	ethpb "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/proto/interfaces"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/prysmaticlabs/prysm/shared/cmd"
	"github.com/prysmaticlabs/prysm/shared/event"
	"github.com/prysmaticlabs/prysm/shared/pagination"
	"github.com/prysmaticlabs/prysm/shared/params"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

// BlockContainer represents an instance of
// block along with its relevant metadata.
type BlockContainer struct {
	Blk         interfaces.SignedBeaconBlock
	Root        [32]byte
	IsCanonical bool
}

// ListBlocks retrieves blocks by root, slot, or epoch.
//
// The server may return multiple blocks in the case that a slot or epoch is
// provided as the filter criteria. The server may return an empty list when
// no blocks in their database match the filter criteria. This RPC should
// not return NOT_FOUND. Only one filter criteria should be used.
func (bs *Server) ListBlocks(
	ctx context.Context, req *ethpb.ListBlocksRequest,
) (*ethpb.ListBlocksResponse, error) {
	if int(req.PageSize) > cmd.Get().MaxRPCPageSize {
		return nil, status.Errorf(codes.InvalidArgument, "Requested page size %d can not be greater than max size %d",
			req.PageSize, cmd.Get().MaxRPCPageSize)
	}

	switch q := req.QueryFilter.(type) {
	case *ethpb.ListBlocksRequest_Epoch:
		ctrs, numBlks, nextPageToken, err := bs.ListBlocksForEpoch(ctx, req, q)
		if err != nil {
			return nil, err
		}
		blkContainers, err := convertToProto(ctrs)
		if err != nil {
			return nil, err
		}

		return &ethpb.ListBlocksResponse{
			BlockContainers: blkContainers,
			TotalSize:       int32(numBlks),
			NextPageToken:   nextPageToken,
		}, nil
	case *ethpb.ListBlocksRequest_Root:
		ctrs, numBlks, nextPageToken, err := bs.ListBlocksForRoot(ctx, req, q)
		if err != nil {
			return nil, err
		}
		blkContainers, err := convertToProto(ctrs)
		if err != nil {
			return nil, err
		}

		return &ethpb.ListBlocksResponse{
			BlockContainers: blkContainers,
			TotalSize:       int32(numBlks),
			NextPageToken:   nextPageToken,
		}, nil

	case *ethpb.ListBlocksRequest_Slot:
		ctrs, numBlks, nextPageToken, err := bs.ListBlocksForSlot(ctx, req, q)
		if err != nil {
			return nil, err
		}
		blkContainers, err := convertToProto(ctrs)
		if err != nil {
			return nil, err
		}

		return &ethpb.ListBlocksResponse{
			BlockContainers: blkContainers,
			TotalSize:       int32(numBlks),
			NextPageToken:   nextPageToken,
		}, nil
	case *ethpb.ListBlocksRequest_Genesis:
		ctrs, numBlks, nextPageToken, err := bs.ListBlocksForGenesis(ctx, req, q)
		if err != nil {
			return nil, err
		}
		blkContainers, err := convertToProto(ctrs)
		if err != nil {
			return nil, err
		}
		return &ethpb.ListBlocksResponse{
			BlockContainers: blkContainers,
			TotalSize:       int32(numBlks),
			NextPageToken:   nextPageToken,
		}, nil
	}

	return nil, status.Error(codes.InvalidArgument, "Must specify a filter criteria for fetching blocks")
}

// ListBlocksForEpoch retrieves all blocks for the provided epoch.
func (bs *Server) ListBlocksForEpoch(ctx context.Context, req *ethpb.ListBlocksRequest, q *ethpb.ListBlocksRequest_Epoch) ([]BlockContainer, int, string, error) {
	blks, _, err := bs.BeaconDB.Blocks(ctx, filters.NewFilter().SetStartEpoch(q.Epoch).SetEndEpoch(q.Epoch))
	if err != nil {
		return nil, 0, strconv.Itoa(0), status.Errorf(codes.Internal, "Could not get blocks: %v", err)
	}

	numBlks := len(blks)
	if len(blks) == 0 {
		return []BlockContainer{}, numBlks, strconv.Itoa(0), nil
	}

	start, end, nextPageToken, err := pagination.StartAndEndPage(req.PageToken, int(req.PageSize), numBlks)
	if err != nil {
		return nil, 0, strconv.Itoa(0), status.Errorf(codes.Internal, "Could not paginate blocks: %v", err)
	}

	returnedBlks := blks[start:end]
	containers := make([]BlockContainer, len(returnedBlks))
	for i, b := range returnedBlks {
		root, err := b.Block().HashTreeRoot()
		if err != nil {
			return nil, 0, strconv.Itoa(0), err
		}
		canonical, err := bs.CanonicalFetcher.IsCanonical(ctx, root)
		if err != nil {
			return nil, 0, strconv.Itoa(0), status.Errorf(codes.Internal, "Could not determine if block is canonical: %v", err)
		}
		containers[i] = BlockContainer{
			Blk:         b,
			Root:        root,
			IsCanonical: canonical,
		}
	}

	return containers, numBlks, nextPageToken, nil
}

// ListBlocksForRoot retrieves the block for the provided root.
func (bs *Server) ListBlocksForRoot(ctx context.Context, req *ethpb.ListBlocksRequest, q *ethpb.ListBlocksRequest_Root) ([]BlockContainer, int, string, error) {
	blk, err := bs.BeaconDB.Block(ctx, bytesutil.ToBytes32(q.Root))
	if err != nil {
		return nil, 0, strconv.Itoa(0), status.Errorf(codes.Internal, "Could not retrieve block: %v", err)
	}
	if blk == nil || blk.IsNil() {
		return []BlockContainer{}, 0, strconv.Itoa(0), nil

	}
	root, err := blk.Block().HashTreeRoot()
	if err != nil {
		return nil, 0, strconv.Itoa(0), status.Errorf(codes.Internal, "Could not determine block root: %v", err)
	}
	canonical, err := bs.CanonicalFetcher.IsCanonical(ctx, root)
	if err != nil {
		return nil, 0, strconv.Itoa(0), status.Errorf(codes.Internal, "Could not determine if block is canonical: %v", err)
	}
	return []BlockContainer{{
		Blk:         blk,
		Root:        root,
		IsCanonical: canonical,
	}}, 1, strconv.Itoa(0), nil
}

// ListBlocksForSlot retrieves all blocks for the provided slot.
func (bs *Server) ListBlocksForSlot(ctx context.Context, req *ethpb.ListBlocksRequest, q *ethpb.ListBlocksRequest_Slot) ([]BlockContainer, int, string, error) {
	hasBlocks, blks, err := bs.BeaconDB.BlocksBySlot(ctx, q.Slot)
	if err != nil {
		return nil, 0, strconv.Itoa(0), status.Errorf(codes.Internal, "Could not retrieve blocks for slot %d: %v", q.Slot, err)
	}
	if !hasBlocks {
		return []BlockContainer{}, 0, strconv.Itoa(0), nil
	}

	numBlks := len(blks)

	start, end, nextPageToken, err := pagination.StartAndEndPage(req.PageToken, int(req.PageSize), numBlks)
	if err != nil {
		return nil, 0, strconv.Itoa(0), status.Errorf(codes.Internal, "Could not paginate blocks: %v", err)
	}

	returnedBlks := blks[start:end]
	containers := make([]BlockContainer, len(returnedBlks))
	for i, b := range returnedBlks {
		root, err := b.Block().HashTreeRoot()
		if err != nil {
			return nil, 0, strconv.Itoa(0), status.Errorf(codes.Internal, "Could not determine block root: %v", err)
		}
		canonical, err := bs.CanonicalFetcher.IsCanonical(ctx, root)
		if err != nil {
			return nil, 0, strconv.Itoa(0), status.Errorf(codes.Internal, "Could not determine if block is canonical: %v", err)
		}
		containers[i] = BlockContainer{
			Blk:         b,
			Root:        root,
			IsCanonical: canonical,
		}
	}
	return containers, numBlks, nextPageToken, nil
}

// ListBlocksForGenesis retrieves the genesis block.
func (bs *Server) ListBlocksForGenesis(ctx context.Context, req *ethpb.ListBlocksRequest, q *ethpb.ListBlocksRequest_Genesis) ([]BlockContainer, int, string, error) {
	genBlk, err := bs.BeaconDB.GenesisBlock(ctx)
	if err != nil {
		return nil, 0, strconv.Itoa(0), status.Errorf(codes.Internal, "Could not retrieve blocks for genesis slot: %v", err)
	}
	if genBlk == nil || genBlk.IsNil() {
		return []BlockContainer{}, 0, strconv.Itoa(0), status.Error(codes.Internal, "Could not find genesis block")
	}
	root, err := genBlk.Block().HashTreeRoot()
	if err != nil {
		return nil, 0, strconv.Itoa(0), status.Errorf(codes.Internal, "Could not determine block root: %v", err)
	}
	return []BlockContainer{{
		Blk:         genBlk,
		Root:        root,
		IsCanonical: true,
	}}, 1, strconv.Itoa(0), nil
}

func convertToProto(ctrs []BlockContainer) ([]*ethpb.BeaconBlockContainer, error) {
	protoCtrs := make([]*ethpb.BeaconBlockContainer, len(ctrs))
	for i, c := range ctrs {
		phBlk, err := c.Blk.PbPhase0Block()
		if err != nil {
			return nil, status.Errorf(codes.Internal, "Could not get phase 0 block: %v", err)
		}
		copiedRoot := c.Root
		protoCtrs[i] = &ethpb.BeaconBlockContainer{
			Block:     phBlk,
			BlockRoot: copiedRoot[:],
			Canonical: c.IsCanonical,
		}
	}
	return protoCtrs, nil
}

// GetChainHead retrieves information about the head of the beacon chain from
// the view of the beacon chain node.
//
// This includes the head block slot and root as well as information about
// the most recent finalized and justified slots.
func (bs *Server) GetChainHead(ctx context.Context, _ *emptypb.Empty) (*ethpb.ChainHead, error) {
	return bs.chainHeadRetrieval(ctx)
}

// StreamBlocks to clients every single time a block is received by the beacon node.
func (bs *Server) StreamBlocks(req *ethpb.StreamBlocksRequest, stream ethpb.BeaconChain_StreamBlocksServer) error {
	blocksChannel := make(chan *feed.Event, 1)
	var blockSub event.Subscription
	if req.VerifiedOnly {
		blockSub = bs.StateNotifier.StateFeed().Subscribe(blocksChannel)
	} else {
		blockSub = bs.BlockNotifier.BlockFeed().Subscribe(blocksChannel)
	}
	defer blockSub.Unsubscribe()

	for {
		select {
		case blockEvent := <-blocksChannel:
			if req.VerifiedOnly {
				if blockEvent.Type == statefeed.BlockProcessed {
					data, ok := blockEvent.Data.(*statefeed.BlockProcessedData)
					if !ok || data == nil {
						continue
					}
					phBlk, err := data.SignedBlock.PbPhase0Block()
					if err != nil {
						log.Error(err)
						continue
					}
					if err := stream.Send(phBlk); err != nil {
						return status.Errorf(codes.Unavailable, "Could not send over stream: %v", err)
					}
				}
			} else {
				if blockEvent.Type == blockfeed.ReceivedBlock {
					data, ok := blockEvent.Data.(*blockfeed.ReceivedBlockData)
					if !ok {
						// Got bad data over the stream.
						continue
					}
					if data.SignedBlock == nil {
						// One nil block shouldn't stop the stream.
						continue
					}
					headState, err := bs.HeadFetcher.HeadState(bs.Ctx)
					if err != nil {
						log.WithError(err).WithField("blockSlot", data.SignedBlock.Block().Slot()).Error("Could not get head state")
						continue
					}
					signed := data.SignedBlock
					if err := blocks.VerifyBlockSignature(headState, signed.Block().ProposerIndex(), signed.Signature(), signed.Block().HashTreeRoot); err != nil {
						log.WithError(err).WithField("blockSlot", data.SignedBlock.Block().Slot()).Error("Could not verify block signature")
						continue
					}
					phBlk, err := signed.PbPhase0Block()
					if err != nil {
						log.Error(err)
						continue
					}
					if err := stream.Send(phBlk); err != nil {
						return status.Errorf(codes.Unavailable, "Could not send over stream: %v", err)
					}
				}
			}
		case <-blockSub.Err():
			return status.Error(codes.Aborted, "Subscriber closed, exiting goroutine")
		case <-bs.Ctx.Done():
			return status.Error(codes.Canceled, "Context canceled")
		case <-stream.Context().Done():
			return status.Error(codes.Canceled, "Context canceled")
		}
	}
}

// StreamChainHead to clients every single time the head block and state of the chain change.
func (bs *Server) StreamChainHead(_ *emptypb.Empty, stream ethpb.BeaconChain_StreamChainHeadServer) error {
	stateChannel := make(chan *feed.Event, 1)
	stateSub := bs.StateNotifier.StateFeed().Subscribe(stateChannel)
	defer stateSub.Unsubscribe()
	for {
		select {
		case stateEvent := <-stateChannel:
			if stateEvent.Type == statefeed.BlockProcessed {
				res, err := bs.chainHeadRetrieval(stream.Context())
				if err != nil {
					return status.Errorf(codes.Internal, "Could not retrieve chain head: %v", err)
				}
				if err := stream.Send(res); err != nil {
					return status.Errorf(codes.Unavailable, "Could not send over stream: %v", err)
				}
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
	headBlock, err := bs.HeadFetcher.HeadBlock(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, "Could not get head block")
	}
	if headBlock == nil || headBlock.IsNil() || headBlock.Block().IsNil() {
		return nil, status.Error(codes.Internal, "Head block of chain was nil")
	}
	headBlockRoot, err := headBlock.Block().HashTreeRoot()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Could not get head block root: %v", err)
	}

	isGenesis := func(cp *ethpb.Checkpoint) bool {
		return bytesutil.ToBytes32(cp.Root) == params.BeaconConfig().ZeroHash && cp.Epoch == 0
	}
	// Retrieve genesis block in the event we have genesis checkpoints.
	genBlock, err := bs.BeaconDB.GenesisBlock(ctx)
	if err != nil || genBlock == nil || genBlock.IsNil() || genBlock.Block().IsNil() {
		return nil, status.Error(codes.Internal, "Could not get genesis block")
	}

	finalizedCheckpoint := bs.FinalizationFetcher.FinalizedCheckpt()
	if !isGenesis(finalizedCheckpoint) {
		b, err := bs.BeaconDB.Block(ctx, bytesutil.ToBytes32(finalizedCheckpoint.Root))
		if err != nil {
			return nil, status.Error(codes.Internal, "Could not get finalized block")
		}
		if err := helpers.VerifyNilBeaconBlock(b); err != nil {
			return nil, status.Errorf(codes.Internal, "Could not get finalized block: %v", err)
		}
	}

	justifiedCheckpoint := bs.FinalizationFetcher.CurrentJustifiedCheckpt()
	if !isGenesis(justifiedCheckpoint) {
		b, err := bs.BeaconDB.Block(ctx, bytesutil.ToBytes32(justifiedCheckpoint.Root))
		if err != nil {
			return nil, status.Error(codes.Internal, "Could not get justified block")
		}
		if err := helpers.VerifyNilBeaconBlock(b); err != nil {
			return nil, status.Errorf(codes.Internal, "Could not get justified block: %v", err)
		}
	}

	prevJustifiedCheckpoint := bs.FinalizationFetcher.PreviousJustifiedCheckpt()
	if !isGenesis(prevJustifiedCheckpoint) {
		b, err := bs.BeaconDB.Block(ctx, bytesutil.ToBytes32(prevJustifiedCheckpoint.Root))
		if err != nil {
			return nil, status.Error(codes.Internal, "Could not get prev justified block")
		}
		if err := helpers.VerifyNilBeaconBlock(b); err != nil {
			return nil, status.Errorf(codes.Internal, "Could not get prev justified block: %v", err)
		}
	}

	fSlot, err := helpers.StartSlot(finalizedCheckpoint.Epoch)
	if err != nil {
		return nil, err
	}
	jSlot, err := helpers.StartSlot(justifiedCheckpoint.Epoch)
	if err != nil {
		return nil, err
	}
	pjSlot, err := helpers.StartSlot(prevJustifiedCheckpoint.Epoch)
	if err != nil {
		return nil, err
	}
	return &ethpb.ChainHead{
		HeadSlot:                   headBlock.Block().Slot(),
		HeadEpoch:                  helpers.SlotToEpoch(headBlock.Block().Slot()),
		HeadBlockRoot:              headBlockRoot[:],
		FinalizedSlot:              fSlot,
		FinalizedEpoch:             finalizedCheckpoint.Epoch,
		FinalizedBlockRoot:         finalizedCheckpoint.Root,
		JustifiedSlot:              jSlot,
		JustifiedEpoch:             justifiedCheckpoint.Epoch,
		JustifiedBlockRoot:         justifiedCheckpoint.Root,
		PreviousJustifiedSlot:      pjSlot,
		PreviousJustifiedEpoch:     prevJustifiedCheckpoint.Epoch,
		PreviousJustifiedBlockRoot: prevJustifiedCheckpoint.Root,
	}, nil
}

// GetWeakSubjectivityCheckpoint retrieves weak subjectivity state root, block root, and epoch.
func (bs *Server) GetWeakSubjectivityCheckpoint(ctx context.Context, _ *emptypb.Empty) (*ethpb.WeakSubjectivityCheckpoint, error) {
	hs, err := bs.HeadFetcher.HeadState(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, "Could not get head state")
	}
	wsEpoch, err := helpers.LatestWeakSubjectivityEpoch(hs)
	if err != nil {
		return nil, status.Error(codes.Internal, "Could not get weak subjectivity epoch")
	}
	wsSlot, err := helpers.StartSlot(wsEpoch)
	if err != nil {
		return nil, status.Error(codes.Internal, "Could not get weak subjectivity slot")
	}

	wsState, err := bs.StateGen.StateBySlot(ctx, wsSlot)
	if err != nil {
		return nil, status.Error(codes.Internal, "Could not get weak subjectivity state")
	}
	stateRoot, err := wsState.HashTreeRoot(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, "Could not get weak subjectivity state root")
	}
	blkRoot, err := wsState.LatestBlockHeader().HashTreeRoot()
	if err != nil {
		return nil, status.Error(codes.Internal, "Could not get weak subjectivity block root")
	}

	return &ethpb.WeakSubjectivityCheckpoint{
		BlockRoot: blkRoot[:],
		StateRoot: stateRoot[:],
		Epoch:     wsEpoch,
	}, nil
}
