package sync

import (
	"context"
	"sync"

	"github.com/kevinms/leakybucket-go"
	"github.com/pkg/errors"
	ethpb "github.com/prysmaticlabs/ethereumapis/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/beacon-chain/blockchain"
	statefeed "github.com/prysmaticlabs/prysm/beacon-chain/core/feed/state"
	"github.com/prysmaticlabs/prysm/beacon-chain/db"
	"github.com/prysmaticlabs/prysm/beacon-chain/operations/attestations"
	"github.com/prysmaticlabs/prysm/beacon-chain/operations/slashings"
	"github.com/prysmaticlabs/prysm/beacon-chain/operations/voluntaryexits"
	"github.com/prysmaticlabs/prysm/beacon-chain/p2p"
	"github.com/prysmaticlabs/prysm/shared"
)

var _ = shared.Service(&Service{})

const allowedBlocksPerSecond = 32.0
const allowedBlocksBurst = 10 * allowedBlocksPerSecond

// Config to set up the regular sync service.
type Config struct {
	P2P           p2p.P2P
	DB            db.NoHeadAccessDatabase
	AttPool       attestations.Pool
	ExitPool      *voluntaryexits.Pool
	SlashingPool  *slashings.Pool
	Chain         blockchainService
	InitialSync   Checker
	StateNotifier statefeed.Notifier
}

// This defines the interface for interacting with block chain service
type blockchainService interface {
	blockchain.BlockReceiver
	blockchain.HeadFetcher
	blockchain.FinalizationFetcher
	blockchain.ForkFetcher
	blockchain.AttestationReceiver
	blockchain.TimeFetcher
}

// NewRegularSync service.
func NewRegularSync(cfg *Config) *Service {
	ctx, cancel := context.WithCancel(context.Background())
	r := &Service{
		ctx:                  ctx,
		cancel:               cancel,
		db:                   cfg.DB,
		p2p:                  cfg.P2P,
		attPool:              cfg.AttPool,
		exitPool:             cfg.ExitPool,
		slashingPool:         cfg.SlashingPool,
		chain:                cfg.Chain,
		initialSync:          cfg.InitialSync,
		slotToPendingBlocks:  make(map[uint64]*ethpb.SignedBeaconBlock),
		seenPendingBlocks:    make(map[[32]byte]bool),
		blkRootToPendingAtts: make(map[[32]byte][]*ethpb.AggregateAttestationAndProof),
		stateNotifier:        cfg.StateNotifier,
		blocksRateLimiter:    leakybucket.NewCollector(allowedBlocksPerSecond, allowedBlocksBurst, false /* deleteEmptyBuckets */),
	}

	r.registerRPCHandlers()
	r.registerSubscribers()

	return r
}

// Service is responsible for handling all run time p2p related operations as the
// main entry point for network messages.
type Service struct {
	ctx                  context.Context
	cancel               context.CancelFunc
	p2p                  p2p.P2P
	db                   db.NoHeadAccessDatabase
	attPool              attestations.Pool
	exitPool             *voluntaryexits.Pool
	slashingPool         *slashings.Pool
	chain                blockchainService
	slotToPendingBlocks  map[uint64]*ethpb.SignedBeaconBlock
	seenPendingBlocks    map[[32]byte]bool
	blkRootToPendingAtts map[[32]byte][]*ethpb.AggregateAttestationAndProof
	pendingAttsLock      sync.RWMutex
	pendingQueueLock     sync.RWMutex
	chainStarted         bool
	initialSync          Checker
	validateBlockLock    sync.RWMutex
	stateNotifier        statefeed.Notifier
	blocksRateLimiter    *leakybucket.Collector
}

// Start the regular sync service.
func (r *Service) Start() {
	r.p2p.AddConnectionHandler(r.sendRPCStatusRequest)
	r.p2p.AddDisconnectionHandler(r.removeDisconnectedPeerStatus)
	r.processPendingBlocksQueue()
	r.processPendingAttsQueue()
	r.maintainPeerStatuses()
	r.resyncIfBehind()
}

// Stop the regular sync service.
func (r *Service) Stop() error {
	defer r.cancel()
	return nil
}

// Status of the currently running regular sync service.
func (r *Service) Status() error {
	if r.chainStarted && r.initialSync.Syncing() {
		return errors.New("waiting for initial sync")
	}
	return nil
}

// Checker defines a struct which can verify whether a node is currently
// synchronizing a chain with the rest of peers in the network.
type Checker interface {
	Syncing() bool
	Status() error
	Resync() error
}
