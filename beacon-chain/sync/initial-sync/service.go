package initialsync

import (
	"context"
	"time"

	"github.com/pkg/errors"
	"github.com/prysmaticlabs/prysm/beacon-chain/blockchain"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/feed"
	statefeed "github.com/prysmaticlabs/prysm/beacon-chain/core/feed/state"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	"github.com/prysmaticlabs/prysm/beacon-chain/db"
	"github.com/prysmaticlabs/prysm/beacon-chain/flags"
	"github.com/prysmaticlabs/prysm/beacon-chain/p2p"
	"github.com/prysmaticlabs/prysm/shared"
	"github.com/prysmaticlabs/prysm/shared/roughtime"
	"github.com/prysmaticlabs/prysm/shared/slotutil"
	"github.com/sirupsen/logrus"
)

var _ = shared.Service(&Service{})

type blockchainService interface {
	blockchain.BlockReceiver
	blockchain.HeadFetcher
}

const (
	handshakePollingInterval = 5 * time.Second // Polling interval for checking the number of received handshakes.
)

// Config to set up the initial sync service.
type Config struct {
	P2P           p2p.P2P
	DB            db.Database
	Chain         blockchainService
	StateNotifier statefeed.Notifier
}

// Service service.
type Service struct {
	ctx           context.Context
	chain         blockchainService
	p2p           p2p.P2P
	db            db.Database
	synced        bool
	chainStarted  bool
	stateNotifier statefeed.Notifier
}

// NewInitialSync configures the initial sync service responsible for bringing the node up to the
// latest head of the blockchain.
func NewInitialSync(cfg *Config) *Service {
	return &Service{
		ctx:           context.Background(),
		chain:         cfg.Chain,
		p2p:           cfg.P2P,
		db:            cfg.DB,
		stateNotifier: cfg.StateNotifier,
	}
}

// Start the initial sync service.
func (s *Service) Start() {
	var genesis time.Time

	headState, err := s.chain.HeadState(s.ctx)
	if headState == nil || err != nil {
		// Wait for state to be initialized.
		stateChannel := make(chan *feed.Event, 1)
		stateSub := s.stateNotifier.StateFeed().Subscribe(stateChannel)
		defer stateSub.Unsubscribe()
		genesisSet := false
		for !genesisSet {
			select {
			case event := <-stateChannel:
				if event.Type == statefeed.Initialized {
					data := event.Data.(*statefeed.InitializedData)
					log.WithField("starttime", data.StartTime).Debug("Received state initialized event")
					genesis = data.StartTime
					genesisSet = true
				}
			case <-s.ctx.Done():
				log.Debug("Context closed, exiting goroutine")
				return
			case err := <-stateSub.Err():
				log.WithError(err).Error("Subscription to state notifier failed")
				return
			}
		}
		stateSub.Unsubscribe()
	} else {
		genesis = time.Unix(int64(headState.GenesisTime), 0)
	}

	if genesis.After(roughtime.Now()) {
		log.WithField(
			"genesis time",
			genesis,
		).Warn("Genesis time is in the future - waiting to start sync...")
		time.Sleep(roughtime.Until(genesis))
	}
	s.chainStarted = true
	currentSlot := slotutil.SlotsSinceGenesis(genesis)
	if helpers.SlotToEpoch(currentSlot) == 0 {
		log.Info("Chain started within the last epoch - not syncing")
		s.synced = true
		return
	}
	log.Info("Starting initial chain sync...")
	// Are we already in sync, or close to it?
	if helpers.SlotToEpoch(s.chain.HeadSlot()) == helpers.SlotToEpoch(currentSlot) {
		log.Info("Already synced to the current chain head")
		s.synced = true
		return
	}
	s.waitForMinimumPeers()
	if err := s.roundRobinSync(genesis); err != nil {
		panic(err)
	}

	log.Infof("Synced up to slot %d", s.chain.HeadSlot())
	s.synced = true
}

// Stop initial sync.
func (s *Service) Stop() error {
	return nil
}

// Status of initial sync.
func (s *Service) Status() error {
	if !s.synced && s.chainStarted {
		return errors.New("syncing")
	}
	return nil
}

// Syncing returns true if initial sync is still running.
func (s *Service) Syncing() bool {
	return !s.synced
}

// Resync allows a node to start syncing again if it has fallen
// behind the current network head.
func (s *Service) Resync() error {
	// set it to false since we are syncing again
	s.synced = false
	headState, err := s.chain.HeadState(context.Background())
	if err != nil {
		return errors.Wrap(err, "could not retrieve head state")
	}
	genesis := time.Unix(int64(headState.GenesisTime), 0)

	s.waitForMinimumPeers()
	if err := s.roundRobinSync(genesis); err != nil {
		return errors.Wrap(err, "could not retrieve head state")
	}
	log.Infof("Synced up to slot %d", s.chain.HeadSlot())

	s.synced = true
	return nil
}

func (s *Service) waitForMinimumPeers() {
	// Every 5 sec, report handshake count.
	for {
		count := len(s.p2p.Peers().Connected())
		if count >= flags.Get().MinimumSyncPeers {
			break
		}
		log.WithFields(logrus.Fields{
			"valid handshakes":    count,
			"required handshakes": flags.Get().MinimumSyncPeers}).Info("Waiting for enough peer handshakes before syncing")
		time.Sleep(handshakePollingInterval)
	}
}
