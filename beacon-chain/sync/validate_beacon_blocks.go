package sync

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/libp2p/go-libp2p-core/peer"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	types "github.com/prysmaticlabs/eth2-types"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/blocks"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/feed"
	blockfeed "github.com/prysmaticlabs/prysm/beacon-chain/core/feed/block"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/state"
	ethpb "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/proto/eth/v1alpha1/wrapper"
	"github.com/prysmaticlabs/prysm/proto/interfaces"
	"github.com/prysmaticlabs/prysm/shared/blockutil"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/prysmaticlabs/prysm/shared/featureconfig"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/timeutils"
	"github.com/prysmaticlabs/prysm/shared/traceutil"
	"github.com/sirupsen/logrus"
	"go.opencensus.io/trace"
)

// validateBeaconBlockPubSub checks that the incoming block has a valid BLS signature.
// Blocks that have already been seen are ignored. If the BLS signature is any valid signature,
// this method rebroadcasts the message.
func (s *Service) validateBeaconBlockPubSub(ctx context.Context, pid peer.ID, msg *pubsub.Message) pubsub.ValidationResult {
	receivedTime := timeutils.Now()
	// Validation runs on publish (not just subscriptions), so we should approve any message from
	// ourselves.
	if pid == s.cfg.P2P.PeerID() {
		return pubsub.ValidationAccept
	}

	// We should not attempt to process blocks until fully synced, but propagation is OK.
	if s.cfg.InitialSync.Syncing() {
		return pubsub.ValidationIgnore
	}

	ctx, span := trace.StartSpan(ctx, "sync.validateBeaconBlockPubSub")
	defer span.End()

	m, err := s.decodePubsubMessage(msg)
	if err != nil {
		log.WithError(err).Debug("Could not decode message")
		traceutil.AnnotateError(span, err)
		return pubsub.ValidationReject
	}

	s.validateBlockLock.Lock()
	defer s.validateBlockLock.Unlock()

	rblk, ok := m.(*ethpb.SignedBeaconBlock)
	if !ok {
		log.WithError(errors.New("msg is not ethpb.SignedBeaconBlock")).Debug("Rejected block")
		return pubsub.ValidationReject
	}
	blk := wrapper.WrappedPhase0SignedBeaconBlock(rblk)

	if blk.IsNil() || blk.Block().IsNil() {
		log.WithError(errors.New("block.Block is nil")).Debug("Rejected block")
		return pubsub.ValidationReject
	}

	// Broadcast the block on a feed to notify other services in the beacon node
	// of a received block (even if it does not process correctly through a state transition).
	s.cfg.BlockNotifier.BlockFeed().Send(&feed.Event{
		Type: blockfeed.ReceivedBlock,
		Data: &blockfeed.ReceivedBlockData{
			SignedBlock: blk,
		},
	})

	if featureconfig.Get().EnableSlasher {
		// Feed the block header to slasher if enabled. This action
		// is done in the background to avoid adding more load to this critical code path.
		go func() {
			wrap := wrapper.WrappedPhase0SignedBeaconBlock(rblk)
			blockHeader, err := blockutil.SignedBeaconBlockHeaderFromBlock(wrap)
			if err != nil {
				log.WithError(err).WithField("blockSlot", blk.Block().Slot()).Warn("Could not extract block header")
			}
			s.cfg.SlasherBlockHeadersFeed.Send(blockHeader)
		}()
	}

	// Verify the block is the first block received for the proposer for the slot.
	if s.hasSeenBlockIndexSlot(blk.Block().Slot(), blk.Block().ProposerIndex()) {
		return pubsub.ValidationIgnore
	}

	blockRoot, err := blk.Block().HashTreeRoot()
	if err != nil {
		log.WithError(err).WithField("blockSlot", blk.Block().Slot()).Debug("Ignored block")
		return pubsub.ValidationIgnore
	}
	if s.cfg.DB.HasBlock(ctx, blockRoot) {
		return pubsub.ValidationIgnore
	}
	// Check if parent is a bad block and then reject the block.
	if s.hasBadBlock(bytesutil.ToBytes32(blk.Block().ParentRoot())) {
		s.setBadBlock(ctx, blockRoot)
		e := fmt.Errorf("received block with root %#x that has an invalid parent %#x", blockRoot, blk.Block().ParentRoot())
		log.WithError(e).WithField("blockSlot", blk.Block().Slot()).Debug("Rejected block")
		return pubsub.ValidationReject
	}

	s.pendingQueueLock.RLock()
	if s.seenPendingBlocks[blockRoot] {
		s.pendingQueueLock.RUnlock()
		return pubsub.ValidationIgnore
	}
	s.pendingQueueLock.RUnlock()

	// Be lenient in handling early blocks. Instead of discarding blocks arriving later than
	// MAXIMUM_GOSSIP_CLOCK_DISPARITY in future, we tolerate blocks arriving at max two slots
	// earlier (SECONDS_PER_SLOT * 2 seconds). Queue such blocks and process them at the right slot.
	genesisTime := uint64(s.cfg.Chain.GenesisTime().Unix())
	if err := helpers.VerifySlotTime(genesisTime, blk.Block().Slot(), earlyBlockProcessingTolerance); err != nil {
		log.WithError(err).WithField("blockSlot", blk.Block().Slot()).Debug("Ignored block")
		return pubsub.ValidationIgnore
	}

	// Add metrics for block arrival time subtracts slot start time.
	if err := captureArrivalTimeMetric(genesisTime, blk.Block().Slot()); err != nil {
		log.WithError(err).WithField("blockSlot", blk.Block().Slot()).Debug("Ignored block")
		return pubsub.ValidationIgnore
	}

	startSlot, err := helpers.StartSlot(s.cfg.Chain.FinalizedCheckpt().Epoch)
	if err != nil {
		log.WithError(err).WithField("blockSlot", blk.Block().Slot()).Debug("Ignored block")
		return pubsub.ValidationIgnore
	}
	if startSlot >= blk.Block().Slot() {
		e := fmt.Errorf("finalized slot %d greater or equal to block slot %d", startSlot, blk.Block().Slot())
		log.WithError(e).WithField("blockSlot", blk.Block().Slot()).Debug("Ignored block")
		return pubsub.ValidationIgnore
	}

	// Process the block if the clock jitter is less than MAXIMUM_GOSSIP_CLOCK_DISPARITY.
	// Otherwise queue it for processing in the right slot.
	if isBlockQueueable(genesisTime, blk.Block().Slot(), receivedTime) {
		s.pendingQueueLock.Lock()
		if err := s.insertBlockToPendingQueue(blk.Block().Slot(), blk, blockRoot); err != nil {
			s.pendingQueueLock.Unlock()
			log.WithError(err).WithField("blockSlot", blk.Block().Slot()).Debug("Ignored block")
			return pubsub.ValidationIgnore
		}
		s.pendingQueueLock.Unlock()
		log.WithError(errors.New("early block")).WithField("currentSlot", s.cfg.Chain.CurrentSlot()).WithField("blockSlot", blk.Block().Slot()).Debug("Ignored block")
		return pubsub.ValidationIgnore
	}

	// Handle block when the parent is unknown.
	if !s.cfg.DB.HasBlock(ctx, bytesutil.ToBytes32(blk.Block().ParentRoot())) {
		s.pendingQueueLock.Lock()
		if err := s.insertBlockToPendingQueue(blk.Block().Slot(), blk, blockRoot); err != nil {
			s.pendingQueueLock.Unlock()
			log.WithError(err).WithField("blockSlot", blk.Block().Slot()).Debug("Ignored block")
			return pubsub.ValidationIgnore
		}
		s.pendingQueueLock.Unlock()
		log.WithError(errors.New("unknown parent")).WithField("blockSlot", blk.Block().Slot()).Debug("Ignored block")
		return pubsub.ValidationIgnore
	}

	if err := s.validateBeaconBlock(ctx, blk, blockRoot); err != nil {
		log.WithError(err).WithField("blockSlot", blk.Block().Slot()).Warn("Rejected block")
		return pubsub.ValidationReject
	}

	// Record attribute of valid block.
	span.AddAttributes(trace.Int64Attribute("slotInEpoch", int64(blk.Block().Slot()%params.BeaconConfig().SlotsPerEpoch)))
	msg.ValidatorData = rblk // Used in downstream subscriber

	// Log the arrival time of the accepted block
	startTime, err := helpers.SlotToTime(genesisTime, blk.Block().Slot())
	if err != nil {
		log.WithError(err).WithField("blockSlot", blk.Block().Slot()).Debug("Couldn't get slot start time")
		return pubsub.ValidationIgnore
	}
	log.WithFields(logrus.Fields{
		"blockSlot":          blk.Block().Slot(),
		"sinceSlotStartTime": receivedTime.Sub(startTime),
	}).Debug("Received block")
	return pubsub.ValidationAccept
}

func (s *Service) validateBeaconBlock(ctx context.Context, blk interfaces.SignedBeaconBlock, blockRoot [32]byte) error {
	ctx, span := trace.StartSpan(ctx, "sync.validateBeaconBlock")
	defer span.End()

	if err := s.cfg.Chain.VerifyBlkDescendant(ctx, bytesutil.ToBytes32(blk.Block().ParentRoot())); err != nil {
		s.setBadBlock(ctx, blockRoot)
		return err
	}

	hasStateSummaryDB := s.cfg.DB.HasStateSummary(ctx, bytesutil.ToBytes32(blk.Block().ParentRoot()))
	if !hasStateSummaryDB {
		_, err := s.cfg.StateGen.RecoverStateSummary(ctx, bytesutil.ToBytes32(blk.Block().ParentRoot()))
		if err != nil {
			return err
		}
	}
	parentState, err := s.cfg.StateGen.StateByRoot(ctx, bytesutil.ToBytes32(blk.Block().ParentRoot()))
	if err != nil {
		return err
	}

	if err := blocks.VerifyBlockSignature(parentState, blk.Block().ProposerIndex(), blk.Signature(), blk.Block().HashTreeRoot); err != nil {
		s.setBadBlock(ctx, blockRoot)
		return err
	}
	// There is an epoch lookahead for validator proposals
	// for the next epoch from the start of our current epoch. We
	// use the randao mix at the end of the previous epoch as the seed
	// to determine proposals.
	// Seed for Next Epoch => Derived From Randao Mix at the end of the Previous Epoch.
	// Which is why we simply set the slot over here.
	nextEpoch := helpers.NextEpoch(parentState)
	expectedEpoch := helpers.SlotToEpoch(blk.Block().Slot())
	if expectedEpoch <= nextEpoch {
		err = parentState.SetSlot(blk.Block().Slot())
		if err != nil {
			return err
		}
	} else {
		// In the event the block is more than an epoch ahead from its
		// parent state, we have to advance the state forward.
		parentState, err = state.ProcessSlots(ctx, parentState, blk.Block().Slot())
		if err != nil {
			return err
		}
	}
	idx, err := helpers.BeaconProposerIndex(parentState)
	if err != nil {
		return err
	}
	if blk.Block().ProposerIndex() != idx {
		s.setBadBlock(ctx, blockRoot)
		return errors.New("incorrect proposer index")
	}

	return nil
}

// Returns true if the block is not the first block proposed for the proposer for the slot.
func (s *Service) hasSeenBlockIndexSlot(slot types.Slot, proposerIdx types.ValidatorIndex) bool {
	s.seenBlockLock.RLock()
	defer s.seenBlockLock.RUnlock()
	b := append(bytesutil.Bytes32(uint64(slot)), bytesutil.Bytes32(uint64(proposerIdx))...)
	_, seen := s.seenBlockCache.Get(string(b))
	return seen
}

// Set block proposer index and slot as seen for incoming blocks.
func (s *Service) setSeenBlockIndexSlot(slot types.Slot, proposerIdx types.ValidatorIndex) {
	s.seenBlockLock.Lock()
	defer s.seenBlockLock.Unlock()
	b := append(bytesutil.Bytes32(uint64(slot)), bytesutil.Bytes32(uint64(proposerIdx))...)
	s.seenBlockCache.Add(string(b), true)
}

// Returns true if the block is marked as a bad block.
func (s *Service) hasBadBlock(root [32]byte) bool {
	s.badBlockLock.RLock()
	defer s.badBlockLock.RUnlock()
	_, seen := s.badBlockCache.Get(string(root[:]))
	return seen
}

// Set bad block in the cache.
func (s *Service) setBadBlock(ctx context.Context, root [32]byte) {
	s.badBlockLock.Lock()
	defer s.badBlockLock.Unlock()
	if ctx.Err() != nil { // Do not mark block as bad if it was due to context error.
		return
	}
	s.badBlockCache.Add(string(root[:]), true)
}

// This captures metrics for block arrival time by subtracts slot start time.
func captureArrivalTimeMetric(genesisTime uint64, currentSlot types.Slot) error {
	startTime, err := helpers.SlotToTime(genesisTime, currentSlot)
	if err != nil {
		return err
	}
	ms := timeutils.Now().Sub(startTime) / time.Millisecond
	arrivalBlockPropagationHistogram.Observe(float64(ms))

	return nil
}

// isBlockQueueable checks if the slot_time in the block is greater than
// current_time +  MAXIMUM_GOSSIP_CLOCK_DISPARITY. in short, this function
// returns true if the corresponding block should be queued and false if
// the block should be processed immediately.
func isBlockQueueable(genesisTime uint64, slot types.Slot, receivedTime time.Time) bool {
	slotTime, err := helpers.SlotToTime(genesisTime, slot)
	if err != nil {
		return false
	}

	currentTimeWithDisparity := receivedTime.Add(params.BeaconNetworkConfig().MaximumGossipClockDisparity)
	return currentTimeWithDisparity.Unix() < slotTime.Unix()
}
