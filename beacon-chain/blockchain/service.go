// Package blockchain defines the life-cycle and status of the beacon chain.
package blockchain

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/event"
	"github.com/prysmaticlabs/prysm/beacon-chain/powchain"
	"github.com/prysmaticlabs/prysm/beacon-chain/db"
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
	beaconDB						   *db.DB
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
	BeaconBlockBuf   int
	IncomingBlockBuf int
	IncomingAttestationBuf int
	Chain            *BeaconChain
	Web3Service      *powchain.Web3Service
	BeaconDB         *db.DB
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

// updateHead applies the fork choice rule to the last received slot.
func (c *ChainService) updateHead() {
	// Super naive fork choice rule: pick the first element at each slot
	// level as canonical.
	//
	// TODO: Implement real fork choice rule here.
	log.WithField("slotNumber", c.candidateBlock.SlotNumber()).Info("Applying fork choice rule")
	c.chain.SetActiveState(c.candidateActiveState)
	c.chain.SetCrystallizedState(c.candidateCrystallizedState)

	h, err := c.candidateBlock.Hash()
	if err != nil {
		log.Errorf("Unable to hash canonical block: %v", err)
		return
	}

	if err := c.beaconDB.RecordChainTip(c.candidateBlock, c.candidateActiveState, c.candidateCrystallizedState); err != nil {
		log.Errorf("Unable to record new head: %v", err)
		return
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
		// Receive new attestations from the sync service.
		case attestation := <-c.incomingAttestationChan:
			h, err := attestation.Hash()
			if err != nil {
				log.Debugf("Could not hash incoming attestation: %v", err)
			}
			if err := c.beaconDB.SaveAttestation(attestation); err != nil {
				log.Errorf("Could not save attestation: %v", err)
				continue
			}

			c.processedAttestationFeed.Send(attestation.Proto)
			log.Infof("Relaying attestation 0x%v to proposers through grpc", h)

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
			parentBlock, err := c.beaconDB.GetBlock(block.ParentHash())
			if err != nil {
				log.Errorf("Could not get parent block: %v", err)
			}
			if parentBlock == nil || !c.doesPoWBlockExist(block) || !block.IsValid(aState, cState, parentBlock.SlotNumber()) {
				continue
			}

			// If a candidate block exists and it is a lower slot, run the fork choice rule.
			if c.candidateBlock != nilBlock && block.SlotNumber() > c.candidateBlock.SlotNumber() {
				c.updateHead()
			}

			if err := c.beaconDB.SaveBlockAndAttestations(block); err != nil {
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
