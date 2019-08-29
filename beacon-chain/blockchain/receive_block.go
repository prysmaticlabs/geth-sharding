package blockchain

import (
	"bytes"
	"context"
	"encoding/hex"

	"github.com/pkg/errors"
	"github.com/prysmaticlabs/go-ssz"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/validators"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	ethpb "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/sirupsen/logrus"
	"go.opencensus.io/trace"
)

// BlockReceiver interface defines the methods of chain service receive and processing new blocks.
type BlockReceiver interface {
	ReceiveBlock(ctx context.Context, block *ethpb.BeaconBlock) error
	ReceiveBlockNoPubsub(ctx context.Context, block *ethpb.BeaconBlock) error
	ReceiveBlockNoPubsubForkchoice(ctx context.Context, block *ethpb.BeaconBlock) error
}

// ReceiveBlock is a function that defines the operations that are preformed on
// blocks that is received from rpc service. The operations consists of:
//   1. Gossip block to other peers
//   2. Validate block, apply state transition and update check points
//   3. Apply fork choice to the processed block
//   4. Save latest head info
func (c *ChainService) ReceiveBlock(ctx context.Context, block *ethpb.BeaconBlock) error {
	ctx, span := trace.StartSpan(ctx, "beacon-chain.blockchain.ReceiveBlock")
	defer span.End()

	root, err := ssz.SigningRoot(block)
	if err != nil {
		return errors.Wrap(err, "could not get signing root on received block")
	}

	// Broadcast the new block to the network.
	if err := c.p2p.Broadcast(ctx, block); err != nil {
		return errors.Wrap(err, "could not broadcast block")
	}
	log.WithFields(logrus.Fields{
		"blockRoot": hex.EncodeToString(root[:]),
	}).Info("Broadcasting block")

	if err := c.ReceiveBlockNoPubsub(ctx, block); err != nil {
		return err
	}

	processedBlk.Inc()
	return nil
}

// ReceiveBlockNoPubsub is a function that defines the the operations (minus pubsub)
// that are preformed on blocks that is received from regular sync service. The operations consists of:
//   1. Validate block, apply state transition and update check points
//   2. Apply fork choice to the processed block
//   3. Save latest head info
func (c *ChainService) ReceiveBlockNoPubsub(ctx context.Context, block *ethpb.BeaconBlock) error {
	ctx, span := trace.StartSpan(ctx, "beacon-chain.blockchain.ReceiveBlockNoPubsub")
	defer span.End()

	// Apply state transition on the new block.
	if err := c.forkChoiceStore.OnBlock(ctx, block); err != nil {
		return errors.Wrap(err, "could not process block from fork choice service")
	}
	root, err := ssz.SigningRoot(block)
	if err != nil {
		return errors.Wrap(err, "could not get signing root on received block")
	}
	logStateTransitionData(block, root[:])

	// Run fork choice after applying state transition on the new block.
	headRoot, err := c.forkChoiceStore.Head(ctx)
	if err != nil {
		return errors.Wrap(err, "could not get head from fork choice service")
	}
	headBlk, err := c.beaconDB.Block(ctx, bytesutil.ToBytes32(headRoot))
	if err != nil {
		return errors.Wrap(err, "could not compute state from block head")
	}
	log.WithFields(logrus.Fields{
		"headSlot": headBlk.Slot,
		"headRoot": hex.EncodeToString(headRoot),
	}).Info("Finished applying fork choice for block")

	isCompetingBlock(root[:], block.Slot, headRoot, headBlk.Slot)

	// Save head info after running fork choice.
	if err := c.saveHead(ctx, headBlk, bytesutil.ToBytes32(headRoot)); err != nil {
		return errors.Wrap(err, "could not save head")
	}

	// Remove block's contained deposits, attestations, and other operations from persistent storage.
	if err := c.cleanupBlockOperations(ctx, block); err != nil {
		return errors.Wrap(err, "could not clean up block deposits, attestations, and other operations")
	}

	// Update validator's public key to indices mapping in DB.
	if helpers.IsEpochStart(block.Slot) {
		if err := c.updateValidatorsDB(ctx, root); err != nil {
			return errors.Wrap(err, "could not update validators db")
		}
	}

	processedBlkNoPubsub.Inc()
	return nil
}

// ReceiveBlockNoPubsubForkchoice is a function that defines the all operations (minus pubsub and forkchoice)
// that are preformed blocks that is received from initial sync service. The operations consists of:
//   1. Validate block, apply state transition and update check points
//   2. Save latest head info
func (c *ChainService) ReceiveBlockNoPubsubForkchoice(ctx context.Context, block *ethpb.BeaconBlock) error {
	ctx, span := trace.StartSpan(ctx, "beacon-chain.blockchain.ReceiveBlockNoForkchoice")
	defer span.End()

	// Apply state transition on the incoming newly received block.
	if err := c.forkChoiceStore.OnBlock(ctx, block); err != nil {
		return errors.Wrap(err, "could not process block from fork choice service")
	}
	root, err := ssz.SigningRoot(block)
	if err != nil {
		return errors.Wrap(err, "could not get signing root on received block")
	}
	logStateTransitionData(block, root[:])

	// Save new block as head.
	if err := c.saveHead(ctx, block, root); err != nil {
		return errors.Wrap(err, "could not save head")
	}

	// Remove block's contained deposits, attestations, and other operations from persistent storage.
	if err := c.cleanupBlockOperations(ctx, block); err != nil {
		return errors.Wrap(err, "could not clean up block deposits, attestations, and other operations")
	}

	// Update validator's public key to indices mapping in DB.
	if helpers.IsEpochStart(block.Slot) {
		if err := c.updateValidatorsDB(ctx, root); err != nil {
			return errors.Wrap(err, "could not update validators db")
		}
	}

	processedBlkNoPubsubForkchoice.Inc()
	return nil
}

// cleanupBlockOperations processes and cleans up any block operations relevant to the beacon node
// such as attestations, exits, and deposits. We update the latest seen attestation by validator
// in the local node's runtime, cleanup and remove pending deposits which have been included in the block
// from our node's local cache, and process validator exits and more.
func (c *ChainService) cleanupBlockOperations(ctx context.Context, block *ethpb.BeaconBlock) error {
	// Forward processed block to operation pool to remove individual operation from DB.
	if c.opsPoolService.IncomingProcessedBlockFeed().Send(block) == 0 {
		log.Error("Sent processed block to no subscribers")
	}

	// Remove pending deposits from the deposit queue.
	for _, dep := range block.Body.Deposits {
		c.depositCache.RemovePendingDeposit(ctx, dep)
	}
	return nil
}

// this updates validator's public key to indices mapping stored in DB, due to the frequent
// validator activation and exit, we should check this every epoch.
func (c *ChainService) updateValidatorsDB(ctx context.Context, r [32]byte) error {
	s, err := c.beaconDB.State(ctx, r)
	if err != nil {
		return errors.Wrap(err, "could not retrieve latest processed state in DB")
	}
	if err := c.saveValidatorIdx(ctx, s); err != nil {
		return errors.Wrap(err, "could not save validator index")
	}
	if err := c.deleteValidatorIdx(ctx, s); err != nil {
		return errors.Wrap(err, "could not delete validator index")
	}
	return nil
}

// saveValidatorIdx saves the validators public key to index mapping in DB, these
// validators were activated from current epoch. After it saves, current epoch key
// is deleted from ActivatedValidators mapping.
func (c *ChainService) saveValidatorIdx(ctx context.Context, state *pb.BeaconState) error {
	nextEpoch := helpers.CurrentEpoch(state) + 1
	activatedValidators := validators.ActivatedValFromEpoch(nextEpoch)
	var idxNotInState []uint64
	for _, idx := range activatedValidators {
		// If for some reason the activated validator indices is not in state,
		// we skip them and save them to process for next epoch.
		if int(idx) >= len(state.Validators) {
			idxNotInState = append(idxNotInState, idx)
			continue
		}
		pubKey := state.Validators[idx].PublicKey
		if err := c.beaconDB.SaveValidatorIndex(ctx, bytesutil.ToBytes48(pubKey), idx); err != nil {
			return errors.Wrap(err, "could not save validator index")
		}
	}
	// Since we are processing next epoch, save the can't processed validator indices
	// to the epoch after that.
	validators.InsertActivatedIndices(nextEpoch+1, idxNotInState)
	validators.DeleteActivatedVal(helpers.CurrentEpoch(state))
	return nil
}

// deleteValidatorIdx deletes the validators public key to index mapping in DB, the
// validators were exited from current epoch. After it deletes, current epoch key
// is deleted from ExitedValidators mapping.
func (c *ChainService) deleteValidatorIdx(ctx context.Context, state *pb.BeaconState) error {
	exitedValidators := validators.ExitedValFromEpoch(helpers.CurrentEpoch(state) + 1)
	for _, idx := range exitedValidators {
		pubKey := state.Validators[idx].PublicKey
		if err := c.beaconDB.DeleteValidatorIndex(ctx, bytesutil.ToBytes48(pubKey)); err != nil {
			return errors.Wrap(err, "could not delete validator index")
		}
	}
	validators.DeleteExitedVal(helpers.CurrentEpoch(state))
	return nil
}

// This checks if the block is from a competing chain, emits warning and updates metrics.
func isCompetingBlock(root []byte, slot uint64, headRoot []byte, headSlot uint64) {
	if !bytes.Equal(root[:], headRoot) {
		log.WithFields(logrus.Fields{
			"blkSlot":  slot,
			"blkRoot":  hex.EncodeToString(root[:]),
			"headSlot": headSlot,
			"headRoot": hex.EncodeToString(headRoot),
		}).Warn("Calculated head diffs from new block")
		competingBlks.Inc()
	}
}
