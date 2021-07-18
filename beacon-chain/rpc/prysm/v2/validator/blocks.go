package validator

import (
	"github.com/prysmaticlabs/prysm/beacon-chain/core/blocks"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/feed"
	blockfeed "github.com/prysmaticlabs/prysm/beacon-chain/core/feed/block"
	statefeed "github.com/prysmaticlabs/prysm/beacon-chain/core/feed/state"
	ethpb "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	v2 "github.com/prysmaticlabs/prysm/proto/prysm/v2"
	"github.com/prysmaticlabs/prysm/shared/event"
	"github.com/prysmaticlabs/prysm/shared/version"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var log = logrus.WithField("prefix", "rpc")

// StreamBlocks to clients every single time a block is received by the beacon node.
func (bs *Server) StreamBlocks(req *ethpb.StreamBlocksRequest, stream v2.BeaconNodeValidatorAltair_StreamBlocksServer) error {
	blocksChannel := make(chan *feed.Event, 1)
	var blockSub event.Subscription
	if req.VerifiedOnly {
		blockSub = bs.V1Server.StateNotifier.StateFeed().Subscribe(blocksChannel)
	} else {
		blockSub = bs.V1Server.BlockNotifier.BlockFeed().Subscribe(blocksChannel)
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
					b := &v2.StreamBlocksResponse{}
					switch data.SignedBlock.Version() {
					case version.Phase0:
						phBlk, ok := data.SignedBlock.Proto().(*ethpb.SignedBeaconBlock)
						if !ok {
							log.Warn("Mismatch between version and block type, was expecting *ethpb.SignedBeaconBlock")
							continue
						}
						b.Block = &v2.StreamBlocksResponse_Phase0Block{Phase0Block: phBlk}
					case version.Altair:
						phBlk, ok := data.SignedBlock.Proto().(*v2.SignedBeaconBlockAltair)
						if !ok {
							log.Warn("Mismatch between version and block type, was expecting *v2.SignedBeaconBlockAltair")
							continue
						}
						b.Block = &v2.StreamBlocksResponse_AltairBlock{AltairBlock: phBlk}
					}

					if err := stream.Send(b); err != nil {
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
					b := &v2.StreamBlocksResponse{}
					switch data.SignedBlock.Version() {
					case version.Phase0:
						phBlk, ok := data.SignedBlock.Proto().(*ethpb.SignedBeaconBlock)
						if !ok {
							log.Warn("Mismatch between version and block type, was expecting *ethpb.SignedBeaconBlock")
							continue
						}
						b.Block = &v2.StreamBlocksResponse_Phase0Block{Phase0Block: phBlk}
					case version.Altair:
						phBlk, ok := data.SignedBlock.Proto().(*v2.SignedBeaconBlockAltair)
						if !ok {
							log.Warn("Mismatch between version and block type, was expecting *v2.SignedBeaconBlockAltair")
							continue
						}
						b.Block = &v2.StreamBlocksResponse_AltairBlock{AltairBlock: phBlk}
					}
					if err := stream.Send(b); err != nil {
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
