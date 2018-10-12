// Package blockchain defines the life-cycle and status of the beacon chain.
package blockchain

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/prysmaticlabs/prysm/beacon-chain/casper"
	"github.com/prysmaticlabs/prysm/beacon-chain/db"
	"github.com/prysmaticlabs/prysm/beacon-chain/powchain"
	"github.com/prysmaticlabs/prysm/beacon-chain/types"
	"github.com/prysmaticlabs/prysm/beacon-chain/utils"
	"github.com/prysmaticlabs/prysm/shared/event"
	"github.com/sirupsen/logrus"
)

var log = logrus.WithField("prefix", "blockchain")

// ChainService represents a service that handles the internal
// logic of managing the full PoS beacon chain.
type ChainService struct {
	ctx                            context.Context
	cancel                         context.CancelFunc
	beaconDB                       *db.BeaconDB
	web3Service                    *powchain.Web3Service
	incomingBlockFeed              *event.Feed
	incomingBlockChan              chan *types.Block
	canonicalBlockFeed             *event.Feed
	canonicalCrystallizedStateFeed *event.Feed
	blocksPendingProcessing        [][32]byte
	lock                           sync.Mutex
	genesisTime                    time.Time
	slotTicker                     utils.SlotTicker
	enableCrossLinks               bool
	enableRewardChecking           bool
	enableAttestationValidity      bool
	enablePOWChain                 bool
}

// Config options for the service.
type Config struct {
	BeaconBlockBuf            int
	IncomingBlockBuf          int
	Web3Service               *powchain.Web3Service
	BeaconDB                  *db.BeaconDB
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
	}, nil
}

// Start a blockchain service's main event loop.
func (c *ChainService) Start() {
	// TODO(#474): Fetch the slot: (block, state) DAGs from persistent storage
	// to truly continue across sessions.
	log.Info("Starting service")

	var err error
	c.genesisTime, err = c.beaconDB.GetGenesisTime()
	if err != nil {
		log.Fatal(err)
		return
	}

	c.slotTicker = utils.GetSlotTicker(c.genesisTime)
	go c.updateHead(c.slotTicker.C())
	go c.blockProcessing()
}

// Stop the blockchain service's main event loop and associated goroutines.
func (c *ChainService) Stop() error {
	defer c.cancel()
	c.slotTicker.Done()

	log.Info("Stopping service")
	return nil
}

// IncomingBlockFeed returns a feed that any service can send incoming p2p blocks into.
// The chain service will subscribe to this feed in order to process incoming blocks.
func (c *ChainService) IncomingBlockFeed() *event.Feed {
	return c.incomingBlockFeed
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
func (c *ChainService) updateHead(slotInterval <-chan uint64) {
	for {
		select {
		case <-c.ctx.Done():
			return
		case slot := <-slotInterval:
			log.WithField("slotNumber", slot).Info("New beacon slot")

			// First, we check if there were any blocks processed in the previous slot.
			if len(c.blocksPendingProcessing) == 0 {
				continue
			}

			log.Info("Applying fork choice rule")

			canonicalDecision, err := casper.GHOSTForkChoice(
				c.beaconDB,
				c.beaconDB.GetActiveState(),
				c.beaconDB.GetCrystallizedState(),
				c.blocksPendingProcessing,
				c.enableCrossLinks,
				c.enableRewardChecking,
				c.enableAttestationValidity,
			)

			if err != nil {
				log.Errorf("Could not apply fork choice rule: %v", err)
				continue
			}

			// If no highest scoring block was determined, we do not update the head of the chain.
			if canonicalDecision.CanonicalBlock == nil {
				continue
			}

			h, err := canonicalResponse.CanonicalBlock.Hash()
			if err != nil {
				log.Errorf("Could not hash highest scoring block: %v", err)
				continue
			}

			log.WithField("blockHash", fmt.Sprintf("0x%x", h)).Info("Canonical block determined")

			// Save canonical block hash with slot number to DB.
			if err := c.beaconDB.SaveCanonicalSlotNumber(canonicalResponse.CanonicalBlock.SlotNumber(), h); err != nil {
				log.Errorf("Unable to save slot number to db: %v", err)
				continue
			}

			// Save canonical block to DB.
			if err := c.beaconDB.SaveCanonicalBlock(canonicalResponse.CanonicalBlock); err != nil {
				log.Errorf("Unable to save block to db: %v", err)
				continue
			}

			if err := c.beaconDB.SaveActiveState(canonicalDecision.CanonicalActiveState); err != nil {
				log.Errorf("Write active state to disk failed: %v", err)
				continue
			}

			// We fire events that notify listeners of a new block (or crystallized state in
			// the case of a state transition). This is useful for the beacon node's gRPC
			// server to stream these events to beacon clients.
			if cycleTransitioned {
				if err := c.beaconDB.SaveCrystallizedState(canonicalDecision.CanonicalCrystallizedState); err != nil {
					log.Errorf("Write crystallized state to disk failed: %v", err)
					continue
				}
				c.canonicalCrystallizedStateFeed.Send(canonicalResponse.CanonicalCrystallizedState)
			}
			c.canonicalBlockFeed.Send(canonicalResponse.CanonicalBlock)

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
			parentExists, err := c.beaconDB.HasBlock(block.ParentHash())
			if err != nil {
				log.Errorf("Could not check existence of parent: %v", err)
				continue
			}
			if !parentExists {
				log.Debugf("Block points to nil parent: %v", err)
				continue
			}
			parent, err := c.beaconDB.GetBlock(block.ParentHash())
			if err != nil {
				log.Debugf("Could not get parent block: %v", err)
				continue
			}

			aState := c.beaconDB.GetActiveState()
			cState := c.beaconDB.GetCrystallizedState()

			if valid := block.IsValid(
				c.beaconDB,
				aState,
				cState,
				parent.SlotNumber(),
				c.enableAttestationValidity,
				c.genesisTime,
			); !valid {
				log.Debugf("Block failed validity conditions: %v", err)
				continue
			}

			if err := c.beaconDB.SaveBlock(block); err != nil {
				log.Errorf("Failed to save block: %v", err)
				continue
			}

			log.Infof("Finished processing received block: 0x%x", blockHash)

			// We push the hash of the block we just stored to a pending processing
			// slice the fork choice rule will utilize.
			c.lock.Lock()
			c.blocksPendingProcessing = append(c.blocksPendingProcessing, blockHash)
			c.lock.Unlock()
		}
	}
}
