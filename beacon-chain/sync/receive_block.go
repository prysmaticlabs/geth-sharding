package sync

import (
	"context"
	"fmt"

	"github.com/prysmaticlabs/go-ssz"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	ethpb "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/prysmaticlabs/prysm/shared/p2p"
	"go.opencensus.io/trace"
)

// receiveBlockAnnounce accepts a block hash, determines if we do not contain
// the block in our local DB, and then request the full block data.
func (rs *RegularSync) receiveBlockAnnounce(msg p2p.Message) error {
	log.WithField("peer", msg.Peer).
		Warn("Received block announcement")

	ctx, span := trace.StartSpan(msg.Ctx, "beacon-chain.sync.receiveBlockAnnounce")
	defer span.End()
	recBlockAnnounce.Inc()

	data := msg.Data.(*pb.BeaconBlockAnnounce)
	h := bytesutil.ToBytes32(data.Hash[:32])

	isEvilBlock := rs.db.IsEvilBlockHash(h)
	span.AddAttributes(trace.BoolAttribute("isEvilBlock", isEvilBlock))

	if isEvilBlock {
		log.WithField("blockRoot", fmt.Sprintf("%#x", bytesutil.Trunc(h[:]))).
			Debug("Received blacklisted block")
		return nil
	}

	// This prevents us from processing a block announcement we have already received.
	// TODO(#2072): If the peer failed to give the block, broadcast request to the whole network.
	rs.blockAnnouncementsLock.Lock()
	defer rs.blockAnnouncementsLock.Unlock()
	if _, ok := rs.blockAnnouncements[data.SlotNumber]; ok {
		return nil
	}

	hasBlock := rs.db.HasBlock(h)
	span.AddAttributes(trace.BoolAttribute("hasBlock", hasBlock))

	if hasBlock {
		log.WithField("blockRoot", fmt.Sprintf("%#x", bytesutil.Trunc(h[:]))).Debug("Already processed")
		return nil
	}

	log.WithField("blockRoot", fmt.Sprintf("%#x", bytesutil.Trunc(h[:]))).Debug("Received incoming block root, requesting full block data from sender")
	// Request the full block data from peer that sent the block hash.
	if err := rs.p2p.Send(ctx, &pb.BeaconBlockRequest{Hash: h[:]}, msg.Peer); err != nil {
		log.Error(err)
		return err
	}
	rs.blockAnnouncements[data.SlotNumber] = data.Hash
	sentBlockReq.Inc()
	return nil
}

// receiveBlock processes a block message from the p2p layer and requests for its
// parents recursively if they are not yet contained in the local node's persistent storage.
func (rs *RegularSync) receiveBlock(msg p2p.Message) error {
	ctx, span := trace.StartSpan(msg.Ctx, "beacon-chain.sync.receiveBlock")
	defer span.End()
	recBlock.Inc()
	rs.blockProcessingLock.Lock()
	defer rs.blockProcessingLock.Unlock()
	return rs.processBlockAndFetchAncestors(ctx, msg)
}

// processBlockAndFetchAncestors verifies if a block has a child in the pending blocks map - if so, then
// we recursively call processBlock which applies block state transitions and updates the chain service.
// At the end of the recursive call, we'll have a block which has no children in the map, and at that point
// we can apply the fork choice rule for ETH 2.0.
func (rs *RegularSync) processBlockAndFetchAncestors(ctx context.Context, msg p2p.Message) error {
	block, isValid, err := rs.validateAndProcessBlock(ctx, msg)
	if err != nil {
		return err
	}

	if !isValid {
		return nil
	}

	blockRoot, err := ssz.SigningRoot(block)
	if err != nil {
		return err
	}

	if rs.db.IsEvilBlockHash(blockRoot) {
		log.WithField("blockRoot", bytesutil.Trunc(blockRoot[:])).Debug("Skipping blacklisted block")
		return nil
	}

	// If the block has a child, we then clear it from the blocks pending processing
	// and call receiveBlock recursively. The recursive function call will stop once
	// the block we process no longer has children.
	if child, ok := rs.hasChild(blockRoot); ok {
		// We clear the block root from the pending processing map.
		rs.clearPendingBlock(blockRoot)
		return rs.processBlockAndFetchAncestors(ctx, child)
	}
	return nil
}

func (rs *RegularSync) validateAndProcessBlock(
	ctx context.Context, blockMsg p2p.Message,
) (*ethpb.BeaconBlock, bool, error) {
	ctx, span := trace.StartSpan(ctx, "beacon-chain.sync.validateAndProcessBlock")
	defer span.End()

	response := blockMsg.Data.(*pb.BeaconBlockResponse)
	block := response.Block
	blockRoot, err := ssz.SigningRoot(block)
	if err != nil {
		log.Errorf("Could not hash received block: %v", err)
		span.AddAttributes(trace.BoolAttribute("invalidBlock", true))
		return nil, false, err
	}

	log.WithField("blockRoot", fmt.Sprintf("%#x", bytesutil.Trunc(blockRoot[:]))).
		Debug("Processing response to block request")

	hasBlock := rs.db.HasBlock(blockRoot)
	if hasBlock {
		log.Debug("Received a block that already exists. Exiting...")
		span.AddAttributes(trace.BoolAttribute("invalidBlock", true))
		return nil, false, err
	}

	// We check if we have the block's parents saved locally.
	parentRoot := bytesutil.ToBytes32(block.ParentRoot)
	hasParent := rs.db.HasBlock(parentRoot)
	span.AddAttributes(trace.BoolAttribute("hasParent", hasParent))

	if !hasParent {
		// If we do not have the parent, we insert it into a pending block's map.
		rs.insertPendingBlock(ctx, parentRoot, blockMsg)
		// We update the last observed slot to the received canonical block's slot.
		if block.Slot > rs.highestObservedSlot {
			rs.highestObservedSlot = block.Slot
		}
		return nil, false, nil
	}

	log.WithField("blockRoot", fmt.Sprintf("%#x", bytesutil.Trunc(blockRoot[:]))).Debug(
		"Sending newly received block to chain service")
	// We then process the block by passing it through the ChainService and running
	// a fork choice rule.
	if err = rs.chainService.ReceiveBlock(ctx, block); err != nil {
		log.Errorf("Could not process beacon block: %v", err)
		rs.p2p.Reputation(blockMsg.Peer, p2p.RepPenalityInvalidBlock)
		span.AddAttributes(trace.BoolAttribute("invalidBlock", true))
		return nil, false, err
	}

	rs.p2p.Reputation(blockMsg.Peer, p2p.RepRewardValidBlock)
	sentBlocks.Inc()
	// We update the last observed slot to the received canonical block's slot.
	if block.Slot > rs.highestObservedSlot {
		rs.highestObservedSlot = block.Slot
	}

	span.AddAttributes(trace.Int64Attribute("highestObservedSlot", int64(rs.highestObservedSlot)))
	return block, true, nil
}

func (rs *RegularSync) insertPendingBlock(ctx context.Context, blockRoot [32]byte, blockMsg p2p.Message) {
	rs.blocksAwaitingProcessingLock.Lock()
	defer rs.blocksAwaitingProcessingLock.Unlock()
	// Do not reinsert into the map if block root was previously added.
	if _, ok := rs.blocksAwaitingProcessing[blockRoot]; ok {
		return
	}
	rs.blocksAwaitingProcessing[blockRoot] = blockMsg
	blocksAwaitingProcessingGauge.Inc()
	rs.p2p.Broadcast(ctx, &pb.BeaconBlockRequest{Hash: blockRoot[:]})
}

func (rs *RegularSync) clearPendingBlock(blockRoot [32]byte) {
	rs.blocksAwaitingProcessingLock.Lock()
	defer rs.blocksAwaitingProcessingLock.Unlock()
	delete(rs.blocksAwaitingProcessing, blockRoot)
	blocksAwaitingProcessingGauge.Dec()
}

func (rs *RegularSync) hasChild(blockRoot [32]byte) (p2p.Message, bool) {
	rs.blocksAwaitingProcessingLock.Lock()
	defer rs.blocksAwaitingProcessingLock.Unlock()
	child, ok := rs.blocksAwaitingProcessing[blockRoot]
	return child, ok
}
