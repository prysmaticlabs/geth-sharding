package blockchain

import (
	"context"
	"encoding/hex"
	"fmt"

	"github.com/pkg/errors"
	"github.com/prysmaticlabs/go-ssz"
	b "github.com/prysmaticlabs/prysm/beacon-chain/core/blocks"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/state"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/validators"
	"github.com/prysmaticlabs/prysm/beacon-chain/db"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	ethpb "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/prysmaticlabs/prysm/shared/event"
	"github.com/sirupsen/logrus"
	"go.opencensus.io/trace"
)

// BlockReceiver interface defines the methods in the blockchain service which
// directly receives a new block from other services and applies the full processing pipeline.
type BlockReceiver interface {
	CanonicalBlockFeed() *event.Feed
	ReceiveBlockDeprecated(ctx context.Context, block *ethpb.BeaconBlock) (*pb.BeaconState, error)
	IsCanonical(slot uint64, hash []byte) bool
	UpdateCanonicalRoots(block *ethpb.BeaconBlock, root [32]byte)
}

// BlockProcessor defines a common interface for methods useful for directly applying state transitions
// to beacon blocks and generating a new beacon state from the Ethereum 2.0 core primitives.
type BlockProcessor interface {
	VerifyBlockValidity(ctx context.Context, block *ethpb.BeaconBlock, beaconState *pb.BeaconState) error
	AdvanceStateDeprecated(ctx context.Context, beaconState *pb.BeaconState, block *ethpb.BeaconBlock) (*pb.BeaconState, error)
	CleanupBlockOperations(ctx context.Context, block *ethpb.BeaconBlock) error
}

// BlockFailedProcessingErr represents a block failing a state transition function.
type BlockFailedProcessingErr struct {
	err error
}

func (b *BlockFailedProcessingErr) Error() string {
	return fmt.Sprintf("block failed processing: %v", b.err)
}

// ReceiveBlockDeprecated is a function that defines the operations that are preformed on
// any block that is received from p2p layer or rpc. It performs the following actions: It checks the block to see
// 1. Verify a block passes pre-processing conditions
// 2. Save and broadcast the block via p2p to other peers
// 3. Apply the block state transition function and account for skip slots.
// 4. Process and cleanup any block operations, such as attestations and deposits, which would need to be
//    either included or flushed from the beacon node's runtime.
func (c *ChainService) ReceiveBlockDeprecated(ctx context.Context, block *ethpb.BeaconBlock) (*pb.BeaconState, error) {
	c.receiveBlockLock.Lock()
	defer c.receiveBlockLock.Unlock()
	ctx, span := trace.StartSpan(ctx, "beacon-chain.blockchain.ReceiveBlock")
	defer span.End()
	// TODO(3219): Fix with new fork choice service.
	db, isLegacyDB := c.beaconDB.(*db.BeaconDB)
	if !isLegacyDB {
		panic("Deprecated receive block only works with deprecated database impl.")
	}

	parentRoot := bytesutil.ToBytes32(block.ParentRoot)
	parent, err := db.BlockDeprecated(parentRoot)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get parent block")
	}
	if parent == nil {
		return nil, errors.New("parent does not exist in DB")
	}
	beaconState, err := db.HistoricalStateFromSlot(ctx, parent.Slot, parentRoot)
	if err != nil {
		return nil, errors.Wrap(err, "could not retrieve beacon state")
	}

	blockRoot, err := ssz.SigningRoot(block)
	if err != nil {
		return nil, fmt.Errorf("could not hash beacon block")
	}
	// We first verify the block's basic validity conditions.
	if err := c.VerifyBlockValidity(ctx, block, beaconState); err != nil {
		return beaconState, errors.Wrapf(err, "block with slot %d is not ready for processing", block.Slot)
	}

	// We save the block to the DB and broadcast it to our peers.
	if err := c.SaveAndBroadcastBlock(ctx, block); err != nil {
		return beaconState, fmt.Errorf(
			"could not save and broadcast beacon block with slot %d: %v",
			block.Slot, err,
		)
	}

	log.WithField("slot", block.Slot).Info("Executing state transition")

	// We then apply the block state transition accordingly to obtain the resulting beacon state.
	beaconState, err = c.AdvanceStateDeprecated(ctx, beaconState, block)
	if err != nil {
		switch err.(type) {
		case *BlockFailedProcessingErr:
			// If the block fails processing, we mark it as blacklisted and delete it from our DB.
			db.MarkEvilBlockHash(blockRoot)
			if err := db.DeleteBlockDeprecated(block); err != nil {
				return nil, errors.Wrap(err, "could not delete bad block from db")
			}
			return beaconState, err
		default:
			return beaconState, errors.Wrap(err, "could not apply block state transition")
		}
	}

	log.WithFields(logrus.Fields{
		"slot":  block.Slot,
		"epoch": helpers.SlotToEpoch(block.Slot),
	}).Info("State transition complete")

	// We process the block's contained deposits, attestations, and other operations
	// and that may need to be stored or deleted from the beacon node's persistent storage.
	if err := c.CleanupBlockOperations(ctx, block); err != nil {
		return beaconState, errors.Wrap(err, "could not process block deposits, attestations, and other operations")
	}

	log.WithFields(logrus.Fields{
		"slot":         block.Slot,
		"attestations": len(block.Body.Attestations),
		"deposits":     len(block.Body.Deposits),
	}).Info("Finished processing beacon block")

	return beaconState, nil
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

	// Broadcast the new block to the network.
	if err := c.p2p.Broadcast(ctx, block); err != nil {
		return errors.Wrap(err, "could not broadcast block")
	}

	// Apply state transition on the new block.
	if err := c.forkChoiceStore.OnBlock(ctx, block); err != nil {
		return errors.Wrap(err, "could not process block from fork choice service")
	}
	root, err := ssz.SigningRoot(block)
	if err != nil {
		return errors.Wrap(err, "could not get signing root on received block")
	}
	log.WithFields(logrus.Fields{
		"slots": block.Slot,
		"root":  hex.EncodeToString(root[:]),
	}).Info("Finished state transition and updated fork choice store for block")

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
	}).Info("Finished fork choice")

	// Save head info after running fork choice.
	c.canonicalRootsLock.Lock()
	defer c.canonicalRootsLock.Unlock()
	c.headSlot = headBlk.Slot
	c.canonicalRoots[headBlk.Slot] = headRoot
	if err := c.beaconDB.SaveHeadBlockRoot(ctx, bytesutil.ToBytes32(headRoot)); err != nil {
		return errors.Wrap(err, "could not save head root in DB")
	}
	log.WithFields(logrus.Fields{
		"slot": headBlk.Slot,
		"root": hex.EncodeToString(headRoot),
	}).Info("Saved head info")

	// Remove block's contained deposits, attestations, and other operations from persistent storage.
	if err := c.CleanupBlockOperations(ctx, block); err != nil {
		return errors.Wrap(err, "could not clean up block deposits, attestations, and other operations")
	}

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
	log.WithFields(logrus.Fields{
		"slot": block.Slot,
		"root": hex.EncodeToString(root[:]),
	}).Info("Finished state transition and updated fork choice store for block")

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
	}).Info("Finished fork choice")

	// Save head info after running fork choice.
	c.canonicalRootsLock.Lock()
	defer c.canonicalRootsLock.Unlock()
	c.headSlot = headBlk.Slot
	c.canonicalRoots[headBlk.Slot] = headRoot
	if err := c.beaconDB.SaveHeadBlockRoot(ctx, bytesutil.ToBytes32(headRoot)); err != nil {
		return errors.Wrap(err, "could not save head root in DB")
	}
	log.WithFields(logrus.Fields{
		"headSlot": headBlk.Slot,
		"headRoot": hex.EncodeToString(headRoot),
	}).Info("Saved head info")

	// Remove block's contained deposits, attestations, and other operations from persistent storage.
	if err := c.CleanupBlockOperations(ctx, block); err != nil {
		return errors.Wrap(err, "could not clean up block deposits, attestations, and other operations")
	}

	return nil
}

// ReceiveBlockNoPubsubForkchoice is a function that defines the all operations (minus pubsub and forkchoice)
// that are preformed blocks that is received from initial sync service. The operations consists of:
//   1. Validate block, apply state transition and update check points
//   2. Save latest head info
func (c *ChainService) ReceiveBlockNoForkchoice(ctx context.Context, block *ethpb.BeaconBlock) error {
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
	log.WithFields(logrus.Fields{
		"slots": block.Slot,
		"root":  hex.EncodeToString(root[:]),
	}).Info("Finished state transition and updated fork choice store for block")

	// Save new block as head.
	c.canonicalRootsLock.Lock()
	defer c.canonicalRootsLock.Unlock()
	c.headSlot = block.Slot
	c.canonicalRoots[block.Slot] = root[:]
	if err := c.beaconDB.SaveHeadBlockRoot(ctx, root); err != nil {
		return errors.Wrap(err, "could not save head root in DB")
	}
	log.WithFields(logrus.Fields{
		"slots": block.Slot,
		"root":  hex.EncodeToString(root[:]),
	}).Info("Saved head info")

	// Remove block's contained deposits, attestations, and other operations from persistent storage.
	if err := c.CleanupBlockOperations(ctx, block); err != nil {
		return errors.Wrap(err, "could not clean up block deposits, attestations, and other operations")
	}

	return nil
}

// VerifyBlockValidity cross-checks the block against the pre-processing conditions from
// Ethereum 2.0, namely:
//   The parent block with root block.parent_root has been processed and accepted.
//   The node has processed its state up to slot, block.slot - 1.
//   The Ethereum 1.0 block pointed to by the state.processed_pow_receipt_root has been processed and accepted.
//   The node's local clock time is greater than or equal to state.genesis_time + block.slot * SECONDS_PER_SLOT.
func (c *ChainService) VerifyBlockValidity(
	ctx context.Context,
	block *ethpb.BeaconBlock,
	beaconState *pb.BeaconState,
) error {
	if block.Slot == 0 {
		return fmt.Errorf("cannot process a genesis block: received block with slot %d",
			block.Slot)
	}
	powBlockFetcher := c.web3Service.Client().BlockByHash
	if err := b.IsValidBlock(ctx, beaconState, block,
		c.beaconDB.HasBlock, powBlockFetcher, c.genesisTime); err != nil {
		return errors.Wrap(err, "block does not fulfill pre-processing conditions")
	}
	return nil
}

// SaveAndBroadcastBlock stores the block in persistent storage and then broadcasts it to
// peers via p2p. Blocks which have already been saved are not processed again via p2p, which is why
// the order of operations is important in this function to prevent infinite p2p loops.
func (c *ChainService) SaveAndBroadcastBlock(ctx context.Context, block *ethpb.BeaconBlock) error {
	blockRoot, err := ssz.SigningRoot(block)
	if err != nil {
		return errors.Wrap(err, "could not tree hash incoming block")
	}
	if err := c.beaconDB.SaveBlock(ctx, block); err != nil {
		return errors.Wrap(err, "failed to save block")
	}
	// TODO(3219): Update after new fork choice service.
	db, isLegacyDB := c.beaconDB.(*db.BeaconDB)
	if isLegacyDB {
		if err := db.SaveAttestationTarget(ctx, &pb.AttestationTarget{
			Slot:            block.Slot,
			BeaconBlockRoot: blockRoot[:],
			ParentRoot:      block.ParentRoot,
		}); err != nil {
			return errors.Wrap(err, "failed to save attestation target")
		}
	}
	// Announce the new block to the network.
	c.p2p.Broadcast(ctx, &pb.BeaconBlockAnnounce{
		Hash:       blockRoot[:],
		SlotNumber: block.Slot,
	})
	return nil
}

// CleanupBlockOperations processes and cleans up any block operations relevant to the beacon node
// such as attestations, exits, and deposits. We update the latest seen attestation by validator
// in the local node's runtime, cleanup and remove pending deposits which have been included in the block
// from our node's local cache, and process validator exits and more.
func (c *ChainService) CleanupBlockOperations(ctx context.Context, block *ethpb.BeaconBlock) error {
	// Forward processed block to operation pool to remove individual operation from DB.
	if c.opsPoolService.IncomingProcessedBlockFeed().Send(block) == 0 {
		log.Error("Sent processed block to no subscribers")
	}

	if err := c.attsService.BatchUpdateLatestAttestations(ctx, block.Body.Attestations); err != nil {
		return errors.Wrap(err, "failed to update latest attestation for store")
	}

	// Remove pending deposits from the deposit queue.
	for _, dep := range block.Body.Deposits {
		c.depositCache.RemovePendingDeposit(ctx, dep)
	}
	return nil
}

// AdvanceStateDeprecated executes the Ethereum 2.0 core state transition for the beacon chain and
// updates important checkpoints and local persistent data during epoch transitions. It serves as a wrapper
// around the more low-level, core state transition function primitive.
func (c *ChainService) AdvanceStateDeprecated(
	ctx context.Context,
	beaconState *pb.BeaconState,
	block *ethpb.BeaconBlock,
) (*pb.BeaconState, error) {
	finalizedEpoch := beaconState.FinalizedCheckpoint.Epoch
	newState, err := state.ExecuteStateTransition(
		ctx,
		beaconState,
		block,
	)
	if err != nil {
		return beaconState, &BlockFailedProcessingErr{err}
	}
	// Prune the block cache and helper caches on every new finalized epoch.
	if newState.FinalizedCheckpoint.Epoch > finalizedEpoch {
		helpers.ClearAllCaches()
		c.beaconDB.(*db.BeaconDB).ClearBlockCache()
	}

	log.WithField(
		"slotsSinceGenesis", newState.Slot,
	).Info("Slot transition successfully processed")

	if block != nil {
		log.WithField(
			"slotsSinceGenesis", newState.Slot,
		).Info("Block transition successfully processed")

		blockRoot, err := ssz.SigningRoot(block)
		if err != nil {
			return nil, err
		}
		// Save Historical States.
		if err := c.beaconDB.(*db.BeaconDB).SaveHistoricalState(ctx, beaconState, blockRoot); err != nil {
			return nil, errors.Wrap(err, "could not save historical state")
		}
	}

	if helpers.IsEpochStart(newState.Slot) {
		// Save activated validators of this epoch to public key -> index DB.
		if err := c.saveValidatorIdx(ctx, newState); err != nil {
			return newState, errors.Wrap(err, "could not save validator index")
		}
		// Delete exited validators of this epoch to public key -> index DB.
		if err := c.deleteValidatorIdx(ctx, newState); err != nil {
			return newState, errors.Wrap(err, "could not delete validator index")
		}
		// Update FFG checkpoints in DB.
		if err := c.updateFFGCheckPts(ctx, newState); err != nil {
			return newState, errors.Wrap(err, "could not update FFG checkpts")
		}
		logEpochData(newState)
	}
	return newState, nil
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

// logs epoch related data in each epoch transition
func logEpochData(beaconState *pb.BeaconState) {

	log.WithField("currentEpochAttestations", len(beaconState.CurrentEpochAttestations)).Info("Number of current epoch attestations")
	log.WithField("prevEpochAttestations", len(beaconState.PreviousEpochAttestations)).Info("Number of previous epoch attestations")
	log.WithField(
		"previousJustifiedEpoch", beaconState.PreviousJustifiedCheckpoint.Epoch,
	).Info("Previous justified epoch")
	log.WithField(
		"justifiedEpoch", beaconState.CurrentJustifiedCheckpoint.Epoch,
	).Info("Justified epoch")
	log.WithField(
		"finalizedEpoch", beaconState.FinalizedCheckpoint.Epoch,
	).Info("Finalized epoch")
	log.WithField(
		"Deposit Index", beaconState.Eth1DepositIndex,
	).Info("ETH1 Deposit Index")
	log.WithField(
		"numValidators", len(beaconState.Validators),
	).Info("Validator registry length")

	log.WithField(
		"SlotsSinceGenesis", beaconState.Slot,
	).Info("Epoch transition successfully processed")
}
