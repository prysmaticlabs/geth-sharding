// Package blockchain defines the life-cycle and status of the beacon chain.
package blockchain

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/event"
	"github.com/prysmaticlabs/prysm/beacon-chain/params"
	"github.com/prysmaticlabs/prysm/beacon-chain/powchain"
	"github.com/prysmaticlabs/prysm/beacon-chain/types"
	"github.com/prysmaticlabs/prysm/beacon-chain/utils"
	"github.com/sirupsen/logrus"
)

var log = logrus.WithField("prefix", "blockchain")

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
	canonicalBlockFeed             *event.Feed
	canonicalCrystallizedStateFeed *event.Feed
	blocksPendingProcessing        [][32]byte
	lock                           sync.Mutex
	genesisTimestamp               time.Time
	slotAlignmentDuration          uint64
	enableCrossLinks               bool
	enableRewardChecking           bool
	enableAttestationValidity      bool
	enablePOWChain                 bool
}

// Config options for the service.
type Config struct {
	BeaconBlockBuf            int
	IncomingBlockBuf          int
	Chain                     *BeaconChain
	Web3Service               *powchain.Web3Service
	BeaconDB                  ethdb.Database
	DevMode                   bool
	EnableCrossLinks          bool
	EnableRewardChecking      bool
	EnableAttestationValidity bool
	EnablePOWChain            bool
}

// NewChainService instantiates a new service instance that will
// be registered into a running beacon node.
func NewChainService(ctx context.Context, cfg *Config) (*ChainService, error) {
	ctx, cancel := context.WithCancel(ctx)
	return &ChainService{
		ctx:                            ctx,
		genesisTimestamp:               params.GetConfig().GenesisTime,
		chain:                          cfg.Chain,
		cancel:                         cancel,
		beaconDB:                       cfg.BeaconDB,
		web3Service:                    cfg.Web3Service,
		incomingBlockChan:              make(chan *types.Block, cfg.IncomingBlockBuf),
		incomingBlockFeed:              new(event.Feed),
		canonicalBlockFeed:             new(event.Feed),
		canonicalCrystallizedStateFeed: new(event.Feed),
		blocksPendingProcessing:        [][32]byte{},
		enablePOWChain:                 cfg.EnablePOWChain,
		enableCrossLinks:               cfg.EnableCrossLinks,
		enableRewardChecking:           cfg.EnableRewardChecking,
		enableAttestationValidity:      cfg.EnableAttestationValidity,
		slotAlignmentDuration:          params.GetConfig().SlotDuration,
	}, nil
}

// Start a blockchain service's main event loop.
func (c *ChainService) Start() {
	// TODO(#474): Fetch the slot: (block, state) DAGs from persistent storage
	// to truly continue across sessions.
	log.Info("Starting service")

	// If the genesis time was at 12:00:00PM and the current time is 12:00:03PM,
	// the next slot should tick at 12:00:08PM. We can accomplish this
	// using utils.BlockingWait and passing in the desired
	// slot duration.
	//
	// Instead of utilizing SlotDuration from config, we utilize a property of
	// RPC service struct so this value can be set to 0 seconds
	// as a parameter in tests. Otherwise, tests would sleep.
	utils.BlockingWait(time.Duration(c.slotAlignmentDuration) * time.Second)

	go c.updateHead(time.NewTicker(time.Second * time.Duration(params.GetConfig().SlotDuration)).C)
	go c.blockProcessing()
}

// Stop the blockchain service's main event loop and associated goroutines.
func (c *ChainService) Stop() error {
	defer c.cancel()
	log.Info("Stopping service")
	log.Info("Persisting current active and crystallized states before closing")
	if err := c.chain.PersistActiveState(); err != nil {
		return fmt.Errorf("Error persisting active state: %v", err)
	}
	if err := c.chain.PersistCrystallizedState(); err != nil {
		return fmt.Errorf("Error persisting crystallized state: %v", err)
	}
	return nil
}

// CurrentBeaconSlot based on the seconds since genesis.
func (c *ChainService) CurrentBeaconSlot() uint64 {
	secondsSinceGenesis := time.Since(c.genesisTimestamp).Seconds()
	if math.Floor(secondsSinceGenesis/8.0)-1 < 0 {
		return 0
	}
	return uint64(math.Floor(secondsSinceGenesis/8.0)) - 1
}

// CanonicalHead of the current beacon chain.
func (c *ChainService) CanonicalHead() (*types.Block, error) {
	return c.chain.CanonicalHead()
}

// CanonicalCrystallizedState of the current beacon chain's head.
func (c *ChainService) CanonicalCrystallizedState() *types.CrystallizedState {
	return c.chain.CrystallizedState()
}

// IncomingBlockFeed returns a feed that any service can send incoming p2p blocks into.
// The chain service will subscribe to this feed in order to process incoming blocks.
func (c *ChainService) IncomingBlockFeed() *event.Feed {
	return c.incomingBlockFeed
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

// BlockSlotNumberByHash returns the slot number of a block.
func (c *ChainService) BlockSlotNumberByHash(h [32]byte) (uint64, error) {
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
func (c *ChainService) CheckForCanonicalBlockBySlot(slotNumber uint64) (bool, error) {
	return c.chain.hasCanonicalBlockForSlot(slotNumber)
}

// CanonicalBlockBySlotNumber retrieves the canonical block for that slot which
// has been saved in the db.
func (c *ChainService) CanonicalBlockBySlotNumber(slotNumber uint64) (*types.Block, error) {
	return c.chain.canonicalBlockForSlot(slotNumber)
}

// GenesisBlock returns the contents of the genesis block.
func (c *ChainService) GenesisBlock() (*types.Block, error) {
	return c.chain.genesisBlock()
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

// updateHead applies the fork choice rule to the beacon chain
// at the start of each new slot interval. The function looks
// at an in-memory slice of block hashes pending processing and
// selects the best block according to the in-protocol fork choice
// rule as canonical. This block is then persisted to storage.
func (c *ChainService) updateHead(slotInterval <-chan time.Time) {
	for {
		select {
		case <-c.ctx.Done():
			return
		case <-slotInterval:
			log.WithField("slotNumber", c.CurrentBeaconSlot()).Info("New beacon slot")

			// First, we check if there were any blocks processed in the previous slot.
			// If there is, we fetch the first one from the DB.
			if len(c.blocksPendingProcessing) == 0 {
				continue
			}

			// Naive fork choice rule: we pick the first block we processed for the previous slot
			// as canonical.
			block, err := c.chain.getBlock(c.blocksPendingProcessing[0])
			if err != nil {
				log.Errorf("Could not get block: %v", err)
				continue
			}

			h, err := block.Hash()
			if err != nil {
				log.Errorf("Could not hash incoming block: %v", err)
				continue
			}

			log.Info("Applying fork choice rule")

			parentBlock, err := c.chain.getBlock(block.ParentHash())
			if err != nil {
				log.Errorf("Failed to get parent of block 0x%x", h)
				continue
			}

			cState := c.chain.CrystallizedState()
			aState := c.chain.ActiveState()
			var stateTransitioned bool

			for cState.IsCycleTransition(parentBlock.SlotNumber()) {
				cState, aState, err = cState.NewStateRecalculations(
					aState,
					block,
					c.enableCrossLinks,
					c.enableRewardChecking,
				)
				if err != nil {
					log.Errorf("Initialize new cycle transition failed: %v", err)
					continue
				}
				stateTransitioned = true
			}

			aState, err = aState.CalculateNewActiveState(
				block,
				cState,
				parentBlock.SlotNumber(),
				c.enableAttestationValidity,
			)
			if err != nil {
				log.Errorf("Compute active state failed: %v", err)
				continue
			}

			if err := c.chain.SetActiveState(aState); err != nil {
				log.Errorf("Write active state to disk failed: %v", err)
				continue
			}

			if stateTransitioned {
				if err := c.chain.SetCrystallizedState(cState); err != nil {
					log.Errorf("Write crystallized state to disk failed: %v", err)
					continue
				}
				log.WithField("slotNumber", block.SlotNumber()).Info("Cycle transition occurred, crystallized state updated")
			}

			// Save canonical block hash with slot number to DB.
			if err := c.chain.saveCanonicalSlotNumber(block.SlotNumber(), h); err != nil {
				log.Errorf("Unable to save slot number to db: %v", err)
				continue
			}

			// Save canonical block to DB.
			if err := c.chain.saveCanonicalBlock(block); err != nil {
				log.Errorf("Unable to save block to db: %v", err)
				continue
			}

			log.WithField("blockHash", fmt.Sprintf("0x%x", h)).Info("Canonical block determined")

			// We fire events that notify listeners of a new block (or crystallized state in
			// the case of a state transition). This is useful for the beacon node's gRPC
			// server to stream these events to beacon clients.
			if stateTransitioned {
				c.canonicalCrystallizedStateFeed.Send(cState)
			}
			c.canonicalBlockFeed.Send(block)

			// Clear the blocks pending processing, mutex lock for thread safety
			// in updating this slice.
			c.lock.Lock()
			c.blocksPendingProcessing = [][32]byte{}
			c.lock.Unlock()
		}
	}
}

func (c *ChainService) blockProcessing() {
	subBlock := c.incomingBlockFeed.Subscribe(c.incomingBlockChan)
	defer subBlock.Unsubscribe()
	for {
		select {
		case <-c.ctx.Done():
			log.Debug("Chain service context closed, exiting goroutine")
			return

		// Listen for a newly received incoming block from the sync service.
		case block := <-c.incomingBlockChan:
			log.WithField("slotNumber", block.SlotNumber()).Info("Received an incoming block for processing")
			blockHash, err := block.Hash()
			if err != nil {
				log.Errorf("Failed to get hash of block: %v", err)
				continue
			}

			if c.enablePOWChain && !c.doesPoWBlockExist(block) {
				log.Debugf("Proof-of-Work chain reference in block does not exist")
				continue
			}

			// Check if we have received the parent block.
			parentExists, err := c.chain.hasBlock(block.ParentHash())
			if err != nil {
				log.Errorf("Could not check existence of parent: %v", err)
				continue
			}
			if !parentExists {
				log.Debugf("Block points to nil parent: %v", err)
				continue
			}
			parent, err := c.chain.getBlock(block.ParentHash())
			if err != nil {
				log.Debugf("Could not get parent block: %v", err)
				continue
			}

			aState := c.chain.ActiveState()
			cState := c.chain.CrystallizedState()

			if valid := block.IsValid(c, aState, cState, parent.SlotNumber(), c.enableAttestationValidity); !valid {
				log.Debugf("Block failed validity conditions: %v", err)
				continue
			}

			if err := c.chain.saveBlock(block); err != nil {
				log.Errorf("Failed to save block: %v", err)
				continue
			}

			log.Infof("Finished processing received block: %x", blockHash)

			// We push the hash of the block we just stored to a pending processing
			// slice the fork choice rule will utilize.
			c.lock.Lock()
			c.blocksPendingProcessing = append(c.blocksPendingProcessing, blockHash)
			c.lock.Unlock()
		}
	}
}
