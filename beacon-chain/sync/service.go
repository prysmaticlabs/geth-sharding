// Package sync includes all chain-synchronization logic for the beacon node,
// including gossip-sub validators for blocks, attestations, and other p2p
// messages, as well as ability to process and respond to block requests
// by peers.
package sync

import (
	"context"
	"sync"
	"time"

	lru "github.com/hashicorp/golang-lru"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
	gcache "github.com/patrickmn/go-cache"
	"github.com/pkg/errors"
	"github.com/prysmaticlabs/prysm/beacon-chain/blockchain"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/feed"
	blockfeed "github.com/prysmaticlabs/prysm/beacon-chain/core/feed/block"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/feed/operation"
	statefeed "github.com/prysmaticlabs/prysm/beacon-chain/core/feed/state"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	"github.com/prysmaticlabs/prysm/beacon-chain/db"
	"github.com/prysmaticlabs/prysm/beacon-chain/operations/attestations"
	"github.com/prysmaticlabs/prysm/beacon-chain/operations/slashings"
	"github.com/prysmaticlabs/prysm/beacon-chain/operations/synccommittee"
	"github.com/prysmaticlabs/prysm/beacon-chain/operations/voluntaryexits"
	"github.com/prysmaticlabs/prysm/beacon-chain/p2p"
	"github.com/prysmaticlabs/prysm/beacon-chain/state/stategen"
	"github.com/prysmaticlabs/prysm/cmd/beacon-chain/flags"
	ethpb "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/shared"
	"github.com/prysmaticlabs/prysm/shared/abool"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/runutil"
	"github.com/prysmaticlabs/prysm/shared/slotutil"
	"github.com/prysmaticlabs/prysm/shared/timeutils"
)

var _ shared.Service = (*Service)(nil)

const rangeLimit = 1024
const seenBlockSize = 1000
const seenAttSize = 10000
const seenSyncMsgSize = 1000
const seenSyncSize = 300
const seenExitSize = 100
const seenProposerSlashingSize = 100
const badBlockSize = 1000
const syncMetricsInterval = 10 * time.Second

var (
	// Seconds in one epoch.
	pendingBlockExpTime = time.Duration(params.BeaconConfig().SlotsPerEpoch.Mul(params.BeaconConfig().SecondsPerSlot)) * time.Second
	// time to allow processing early blocks.
	earlyBlockProcessingTolerance = slotutil.MultiplySlotBy(2)
	// time to allow processing early attestations.
	earlyAttestationProcessingTolerance = params.BeaconNetworkConfig().MaximumGossipClockDisparity
)

// Config to set up the regular sync service.
type Config struct {
	P2P               p2p.P2P
	DB                db.NoHeadAccessDatabase
	AttPool           attestations.Pool
	ExitPool          voluntaryexits.PoolManager
	SlashingPool      slashings.PoolManager
	SyncCommsPool     synccommittee.Pool
	Chain             blockchainService
	InitialSync       Checker
	StateNotifier     statefeed.Notifier
	BlockNotifier     blockfeed.Notifier
	OperationNotifier operation.Notifier
	StateGen          *stategen.State
}

// This defines the interface for interacting with block chain service
type blockchainService interface {
	blockchain.BlockReceiver
	blockchain.HeadFetcher
	blockchain.FinalizationFetcher
	blockchain.ForkFetcher
	blockchain.AttestationReceiver
	blockchain.TimeFetcher
	blockchain.GenesisFetcher
	blockchain.CanonicalFetcher
}

// Service is responsible for handling all run time p2p related operations as the
// main entry point for network messages.
type Service struct {
	cfg                       *Config
	ctx                       context.Context
	cancel                    context.CancelFunc
	slotToPendingBlocks       *gcache.Cache
	seenPendingBlocks         map[[32]byte]bool
	blkRootToPendingAtts      map[[32]byte][]*ethpb.SignedAggregateAttestationAndProof
	subHandler                *subTopicHandler
	pendingAttsLock           sync.RWMutex
	pendingQueueLock          sync.RWMutex
	chainStarted              *abool.AtomicBool
	validateBlockLock         sync.RWMutex
	rateLimiter               *limiter
	seenBlockLock             sync.RWMutex
	seenBlockCache            *lru.Cache
	seenAttestationLock       sync.RWMutex
	seenAttestationCache      *lru.Cache
	seenExitLock              sync.RWMutex
	seenExitCache             *lru.Cache
	seenProposerSlashingLock  sync.RWMutex
	seenProposerSlashingCache *lru.Cache
	seenAttesterSlashingLock  sync.RWMutex
	seenAttesterSlashingCache map[uint64]bool
	seenSyncContributionLock  sync.RWMutex
	seenSyncContributionCache *lru.Cache
	seenSyncMessageLock       sync.RWMutex
	seenSyncMessageCache      *lru.Cache
	badBlockCache             *lru.Cache
	badBlockLock              sync.RWMutex
}

// NewService initializes new regular sync service.
func NewService(ctx context.Context, cfg *Config) *Service {
	c := gcache.New(pendingBlockExpTime /* exp time */, 2*pendingBlockExpTime /* prune time */)

	rLimiter := newRateLimiter(cfg.P2P)
	ctx, cancel := context.WithCancel(ctx)
	r := &Service{
		cfg:                  cfg,
		ctx:                  ctx,
		cancel:               cancel,
		chainStarted:         abool.New(),
		slotToPendingBlocks:  c,
		seenPendingBlocks:    make(map[[32]byte]bool),
		blkRootToPendingAtts: make(map[[32]byte][]*ethpb.SignedAggregateAttestationAndProof),
		subHandler:           newSubTopicHandler(),
		rateLimiter:          rLimiter,
	}

	go r.registerHandlers()

	return r
}

// Start the regular sync service.
func (s *Service) Start() {
	if err := s.initCaches(); err != nil {
		panic(err)
	}

	s.cfg.P2P.AddConnectionHandler(s.reValidatePeer, s.sendGoodbye)
	s.cfg.P2P.AddDisconnectionHandler(func(_ context.Context, _ peer.ID) error {
		// no-op
		return nil
	})
	s.cfg.P2P.AddPingMethod(s.sendPingRequest)
	s.processPendingBlocksQueue()
	s.processPendingAttsQueue()
	s.maintainPeerStatuses()
	if !flags.Get().DisableSync {
		s.resyncIfBehind()
	}

	// Update sync metrics.
	runutil.RunEvery(s.ctx, syncMetricsInterval, s.updateMetrics)
}

// Stop the regular sync service.
func (s *Service) Stop() error {
	defer func() {
		if s.rateLimiter != nil {
			s.rateLimiter.free()
		}
	}()
	// Removing RPC Stream handlers.
	for _, p := range s.cfg.P2P.Host().Mux().Protocols() {
		s.cfg.P2P.Host().RemoveStreamHandler(protocol.ID(p))
	}
	// Deregister Topic Subscribers.
	for _, t := range s.cfg.P2P.PubSub().GetTopics() {
		if err := s.cfg.P2P.PubSub().UnregisterTopicValidator(t); err != nil {
			log.Errorf("Could not successfully unregister for topic %s: %v", t, err)
		}
		s.subHandler.removeTopic(t)
	}
	defer s.cancel()
	return nil
}

// Status of the currently running regular sync service.
func (s *Service) Status() error {
	// If our head slot is on a previous epoch and our peers are reporting their head block are
	// in the most recent epoch, then we might be out of sync.
	if headEpoch := helpers.SlotToEpoch(s.cfg.Chain.HeadSlot()); headEpoch+1 < helpers.SlotToEpoch(s.cfg.Chain.CurrentSlot()) &&
		headEpoch+1 < s.cfg.P2P.Peers().HighestEpoch() {
		return errors.New("out of sync")
	}
	return nil
}

// This initializes the caches to update seen beacon objects coming in from the wire
// and prevent DoS.
func (s *Service) initCaches() error {
	blkCache, err := lru.New(seenBlockSize)
	if err != nil {
		return err
	}
	attCache, err := lru.New(seenAttSize)
	if err != nil {
		return err
	}
	syncMsgCache, err := lru.New(seenSyncMsgSize)
	if err != nil {
		return err
	}
	syncContrCache, err := lru.New(seenSyncSize)
	if err != nil {
		return err
	}
	exitCache, err := lru.New(seenExitSize)
	if err != nil {
		return err
	}
	proposerSlashingCache, err := lru.New(seenProposerSlashingSize)
	if err != nil {
		return err
	}
	badBlockCache, err := lru.New(badBlockSize)
	if err != nil {
		return err
	}
	s.seenBlockCache = blkCache
	s.seenAttestationCache = attCache
	s.seenSyncContributionCache = syncContrCache
	s.seenSyncMessageCache = syncMsgCache
	s.seenExitCache = exitCache
	s.seenAttesterSlashingCache = make(map[uint64]bool)
	s.seenProposerSlashingCache = proposerSlashingCache
	s.badBlockCache = badBlockCache

	return nil
}

func (s *Service) registerHandlers() {
	// Wait until chain start.
	stateChannel := make(chan *feed.Event, 1)
	stateSub := s.cfg.StateNotifier.StateFeed().Subscribe(stateChannel)
	defer stateSub.Unsubscribe()
	for {
		select {
		case event := <-stateChannel:
			switch event.Type {
			case statefeed.Initialized:
				data, ok := event.Data.(*statefeed.InitializedData)
				if !ok {
					log.Error("Event feed data is not type *statefeed.InitializedData")
					return
				}
				startTime := data.StartTime
				log.WithField("starttime", startTime).Debug("Received state initialized event")

				// Register respective rpc handlers at state initialized event.
				s.registerRPCHandlers()
				// Wait for chainstart in separate routine.
				go func() {
					if startTime.After(timeutils.Now()) {
						time.Sleep(timeutils.Until(startTime))
					}
					log.WithField("starttime", startTime).Debug("Chain started in sync service")
					s.markForChainStart()
				}()
			case statefeed.Synced:
				_, ok := event.Data.(*statefeed.SyncedData)
				if !ok {
					log.Error("Event feed data is not type *statefeed.SyncedData")
					return
				}
				// Register respective pubsub handlers at state synced event.
				digest, err := s.currentForkDigest()
				if err != nil {
					log.WithError(err).Error("Could not retrieve current fork digest")
				}
				currentEpoch := helpers.SlotToEpoch(helpers.CurrentSlot(uint64(s.cfg.Chain.GenesisTime().Unix())))
				s.registerSubscribers(currentEpoch, digest)
				go s.forkWatcher()
				return
			}
		case <-s.ctx.Done():
			log.Debug("Context closed, exiting goroutine")
			return
		case err := <-stateSub.Err():
			log.WithError(err).Error("Could not subscribe to state notifier")
			return
		}
	}
}

// marks the chain as having started.
func (s *Service) markForChainStart() {
	s.chainStarted.Set()
}

// Checker defines a struct which can verify whether a node is currently
// synchronizing a chain with the rest of peers in the network.
type Checker interface {
	Initialized() bool
	Syncing() bool
	Synced() bool
	Status() error
	Resync() error
}
