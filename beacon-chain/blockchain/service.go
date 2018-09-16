// Package blockchain defines the life-cycle and status of the beacon chain.
package blockchain

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/event"
	"github.com/prysmaticlabs/prysm/beacon-chain/powchain"
	"github.com/prysmaticlabs/prysm/beacon-chain/types"
	"github.com/sirupsen/logrus"
)

var log = logrus.WithField("prefix", "blockchain")
var nilBlock = &types.Block{}
var nilActiveState = &types.ActiveState{}
var nilCrystallizedState = &types.CrystallizedState{}

// ChainService represents a service that handles the internal
// logic of managing the full PoS beacon chain.
type ChainService struct {
	ctx                            context.Context
	cancel                         context.CancelFunc
	beaconDB                       ethdb.Database
	chain                          *BeaconChain
	web3Service                    *powchain.Web3Service
	incomingBlockFeed              *event.Feed
	incomingBlockChan              chan *types.Block
	incomingAttestationFeed        *event.Feed
	incomingAttestationChan        chan *types.Attestation
	processedAttestationFeed       *event.Feed
	canonicalBlockFeed             *event.Feed
	canonicalCrystallizedStateFeed *event.Feed
	latestProcessedBlock           chan *types.Block
	candidateBlock                 *types.Block
	candidateActiveState           *types.ActiveState
	candidateCrystallizedState     *types.CrystallizedState
}

// Config options for the service.
type Config struct {
	BeaconBlockBuf         int
	IncomingBlockBuf       int
	Chain                  *BeaconChain
	Web3Service            *powchain.Web3Service
	BeaconDB               ethdb.Database
	IncomingAttestationBuf int
}

// NewChainService instantiates a new service instance that will
// be registered into a running beacon node.
func NewChainService(ctx context.Context, cfg *Config) (*ChainService, error) {
	ctx, cancel := context.WithCancel(ctx)
	return &ChainService{
		ctx:                            ctx,
		chain:                          cfg.Chain,
		cancel:                         cancel,
		beaconDB:                       cfg.BeaconDB,
		web3Service:                    cfg.Web3Service,
		latestProcessedBlock:           make(chan *types.Block, cfg.BeaconBlockBuf),
		incomingBlockChan:              make(chan *types.Block, cfg.IncomingBlockBuf),
		incomingBlockFeed:              new(event.Feed),
		incomingAttestationChan:        make(chan *types.Attestation, cfg.IncomingAttestationBuf),
		incomingAttestationFeed:        new(event.Feed),
		processedAttestationFeed:       new(event.Feed),
		canonicalBlockFeed:             new(event.Feed),
		canonicalCrystallizedStateFeed: new(event.Feed),
		candidateBlock:                 nilBlock,
		candidateActiveState:           nilActiveState,
		candidateCrystallizedState:     nilCrystallizedState,
	}, nil
}

// Start a blockchain service's main event loop.
func (c *ChainService) Start() {
	// TODO(#474): Fetch the slot: (block, state) DAGs from persistent storage
	// to truly continue across sessions.
	log.Infof("Starting service")
	go c.blockProcessing(c.ctx.Done())
}

// Stop the blockchain service's main event loop and associated goroutines.
func (c *ChainService) Stop() error {
	defer c.cancel()
	log.Info("Stopping service")
	log.Infof("Persisting current active and crystallized states before closing")
	if err := c.chain.PersistActiveState(); err != nil {
		return fmt.Errorf("Error persisting active state: %v", err)
	}
	if err := c.chain.PersistCrystallizedState(); err != nil {
		return fmt.Errorf("Error persisting crystallized state: %v", err)
	}
	return nil
}

// IncomingBlockFeed returns a feed that any service can send incoming p2p blocks into.
// The chain service will subscribe to this feed in order to process incoming blocks.
func (c *ChainService) IncomingBlockFeed() *event.Feed {
	return c.incomingBlockFeed
}

// IncomingAttestationFeed returns a feed that any service can send incoming p2p attestations into.
// The chain service will subscribe to this feed in order to relay incoming attestations.
func (c *ChainService) IncomingAttestationFeed() *event.Feed {
	return c.incomingAttestationFeed
}

// ProcessedAttestationFeed returns a feed that will be used to stream attestations that have been
// processed by the beacon node to its rpc clients.
func (c *ChainService) ProcessedAttestationFeed() *event.Feed {
	return c.processedAttestationFeed
}

// HasStoredState checks if there is any Crystallized/Active State or blocks(not implemented) are
// persisted to the db.
func (c *ChainService) HasStoredState() (bool, error) {
	hasCrystallized, err := c.beaconDB.Has(crystallizedStateLookupKey)
	if err != nil {
		return false, err
	}

	return hasCrystallized, nil
}

// SaveBlock is a mock which saves a block to the local db using the
// blockhash as the key.
func (c *ChainService) SaveBlock(block *types.Block) error {
	return c.chain.saveBlock(block)
}

// ContainsBlock checks if a block for the hash exists in the chain.
// This method must be safe to call from a goroutine.
func (c *ChainService) ContainsBlock(h [32]byte) (bool, error) {
	return c.chain.hasBlock(h)
}

// GetBlockSlotNumber returns the slot number of a block.
func (c *ChainService) GetBlockSlotNumber(h [32]byte) (uint64, error) {
	block, err := c.chain.getBlock(h)
	if err != nil {
		return 0, fmt.Errorf("could not get block from DB: %v", err)
	}
	return block.SlotNumber(), nil
}

// CurrentCrystallizedState of the canonical chain.
func (c *ChainService) CurrentCrystallizedState() *types.CrystallizedState {
	return c.chain.CrystallizedState()
}

// CurrentActiveState of the canonical chain.
func (c *ChainService) CurrentActiveState() *types.ActiveState {
	return c.chain.ActiveState()
}

// CanonicalBlockFeed returns a channel that is written to
// whenever a new block is determined to be canonical in the chain.
func (c *ChainService) CanonicalBlockFeed() *event.Feed {
	return c.canonicalBlockFeed
}

// CanonicalCrystallizedStateFeed returns a feed that is written to
// whenever a new crystallized state is determined to be canonical in the chain.
func (c *ChainService) CanonicalCrystallizedStateFeed() *event.Feed {
	return c.canonicalCrystallizedStateFeed
}

// CheckForCanonicalBlockBySlot checks if the canonical block for that slot exists
// in the db.
func (c *ChainService) CheckForCanonicalBlockBySlot(slotnumber uint64) (bool, error) {
	return c.chain.hasCanonicalBlockForSlot(slotnumber)
}

// GetCanonicalBlockBySlotNumber retrieves the canonical block for that slot which
// has been saved in the db.
func (c *ChainService) GetCanonicalBlockBySlotNumber(slotnumber uint64) (*types.Block, error) {
	return c.chain.getCanonicalBlockForSlot(slotnumber)
}

// updateHead applies the fork choice rule to the last received slot.
func (c *ChainService) updateHead() {
	// Super naive fork choice rule: pick the first element at each slot
	// level as canonical.
	//
	// TODO: Implement real fork choice rule here.
	log.WithField("slotNumber", c.candidateBlock.SlotNumber()).Info("Applying fork choice rule")
	if err := c.chain.SetActiveState(c.candidateActiveState); err != nil {
		log.Errorf("Write active state to disk failed: %v", err)
	}

	if err := c.chain.SetCrystallizedState(c.candidateCrystallizedState); err != nil {
		log.Errorf("Write crystallized state to disk failed: %v", err)
	}

	h, err := c.candidateBlock.Hash()
	if err != nil {
		log.Errorf("Unable to hash canonical block: %v", err)
		return
	}

	// Save canonical slotnumber to DB.
	if err := c.chain.saveCanonicalSlotNumber(c.candidateBlock.SlotNumber(), h); err != nil {
		log.Errorf("Unable to save slot number to db: %v", err)
	}

	// Save canonical block to DB.
	if err := c.chain.saveCanonicalBlock(c.candidateBlock); err != nil {
		log.Errorf("Unable to save block to db: %v", err)
	}
	log.WithField("blockHash", fmt.Sprintf("0x%x", h)).Info("Canonical block determined")

	// We fire events that notify listeners of a new block (or crystallized state in
	// the case of a state transition). This is useful for the beacon node's gRPC
	// server to stream these events to beacon clients.
	cState := c.chain.CrystallizedState()
	if cState.IsCycleTransition(c.candidateBlock.SlotNumber()) {
		c.canonicalCrystallizedStateFeed.Send(c.candidateCrystallizedState)
	}
	c.canonicalBlockFeed.Send(c.candidateBlock)

	c.candidateBlock = nilBlock
	c.candidateActiveState = nilActiveState
	c.candidateCrystallizedState = nilCrystallizedState
}

// doesPoWBlockExist checks if the referenced PoW block exists.
func (c *ChainService) doesPoWBlockExist(block *types.Block) bool {
	powBlock, err := c.web3Service.Client().BlockByHash(context.Background(), block.PowChainRef())
	if err != nil {
		log.Debugf("fetching PoW block corresponding to mainchain reference failed: %v", err)
		return false
	}

	return powBlock != nil
}

func (c *ChainService) blockProcessing(done <-chan struct{}) {
	subBlock := c.incomingBlockFeed.Subscribe(c.incomingBlockChan)
	subAttestation := c.incomingAttestationFeed.Subscribe(c.incomingAttestationChan)
	defer subBlock.Unsubscribe()
	defer subAttestation.Unsubscribe()
	for {
		select {
		case <-done:
			log.Debug("Chain service context closed, exiting goroutine")
			return
		// Listen for a newly received incoming attestation from the sync service.
		case attestation := <-c.incomingAttestationChan:
			h, err := attestation.Hash()
			if err != nil {
				log.Debugf("Could not hash incoming attestation: %v", err)
			}
			if err := c.chain.saveAttestation(attestation); err != nil {
				log.Errorf("Could not save attestation: %v", err)
				continue
			}

			c.processedAttestationFeed.Send(attestation.Proto)
			log.Info("Relaying attestation 0x%v to proposers through grpc", h)

		// Listen for a newly received incoming block from the sync service.
		case block := <-c.incomingBlockChan:
			// 1. Validate the block
			// 2. If a candidate block with a lower slot exists, run the fork choice rule
			// 3. Save the block
			// 4. If a candidate block exists, exit
			// 4. Calculate the active and crystallized state for the block
			// 5. Set the block as the new candidate block
			aState := c.chain.ActiveState()
			cState := c.chain.CrystallizedState()
			blockHash, err := block.Hash()
			if err != nil {
				log.Errorf("Failed to get hash of block: %v", err)
				continue
			}

			// Process block as a validator if beacon node has registered, else process block as an observer.
			parentExists, err := c.chain.hasBlock(block.ParentHash())
			if err != nil {
				log.Errorf("Could not check existence of parent: %v", err)
				continue
			}

			parentBlock, err := c.chain.getBlock(block.ParentHash())
			if err != nil {
				log.Errorf("Could not get parent block: %v", err)
				continue
			}

			if !parentExists || !c.doesPoWBlockExist(block) || !block.IsValid(aState, cState, parentBlock.SlotNumber()) {
				continue
			}

			// If a candidate block exists and it is a lower slot, run the fork choice rule.
			if c.candidateBlock != nilBlock && block.SlotNumber() > c.candidateBlock.SlotNumber() {
				c.updateHead()
			}

			if err := c.chain.saveBlockAndAttestations(block); err != nil {
				log.Errorf("Failed to save block: %v", err)
				continue
			}

			log.Infof("Finished processing received block: %x", blockHash)

			// Do not proceed further, because a candidate has already been chosen.
			if c.candidateBlock != nilBlock {
				continue
			}

			// Refetch active and crystallized state, in case `updateHead` was called.
			aState = c.chain.ActiveState()
			cState = c.chain.CrystallizedState()

			// Entering cycle transitions.
			if cState.IsCycleTransition(block.SlotNumber()) {
				log.Info("Entering cycle transition")
				cState, err = cState.NewStateRecalculations(aState, block)
			}
			if err != nil {
				log.Errorf("Failed to calculate the new crystallized state: %v", err)
				continue
			}

			aState, err = aState.CalculateNewActiveState(block, cState, parentBlock.SlotNumber())
			if err != nil {
				log.Errorf("Compute active state failed: %v", err)
				continue
			}

			c.candidateBlock = block
			c.candidateActiveState = aState
			c.candidateCrystallizedState = cState

			log.Infof("Finished processing state for candidate block: %x", blockHash)
		}
	}
}
