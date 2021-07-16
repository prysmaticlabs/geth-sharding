// Package slasher implements slashing detection for eth2, able to catch slashable attestations
// and proposals that it receives via two event feeds, respectively. Any found slashings
// are then submitted to the beacon node's slashing operations pool. See the design document
// here https://hackmd.io/@prysmaticlabs/slasher.
package slasher

import (
	"context"
	"time"

	types "github.com/prysmaticlabs/eth2-types"
	"github.com/prysmaticlabs/prysm/beacon-chain/blockchain"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/feed"
	statefeed "github.com/prysmaticlabs/prysm/beacon-chain/core/feed/state"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	"github.com/prysmaticlabs/prysm/beacon-chain/db"
	"github.com/prysmaticlabs/prysm/beacon-chain/db/filters"
	"github.com/prysmaticlabs/prysm/beacon-chain/operations/slashings"
	slashertypes "github.com/prysmaticlabs/prysm/beacon-chain/slasher/types"
	"github.com/prysmaticlabs/prysm/beacon-chain/state/stategen"
	"github.com/prysmaticlabs/prysm/beacon-chain/sync"
	ethpb "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/shared/blockutil"
	"github.com/prysmaticlabs/prysm/shared/event"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/slotutil"
)

// ServiceConfig for the slasher service in the beacon node.
// This struct allows us to specify required dependencies and
// parameters for slasher to function as needed.
type ServiceConfig struct {
	IndexedAttestationsFeed *event.Feed
	BeaconBlockHeadersFeed  *event.Feed
	Database                db.SlasherDatabase
	BeaconDatabase          db.Database
	StateNotifier           statefeed.Notifier
	AttestationStateFetcher blockchain.AttestationStateFetcher
	StateGen                stategen.StateManager
	SlashingPoolInserter    slashings.PoolInserter
	HeadStateFetcher        blockchain.HeadFetcher
	SyncChecker             sync.Checker
}

// SlashingChecker is an interface for defining services that the beacon node may interact with to provide slashing data.
type SlashingChecker interface {
	IsSlashableBlock(ctx context.Context, proposal *ethpb.SignedBeaconBlockHeader) (*ethpb.ProposerSlashing, error)
	IsSlashableAttestation(ctx context.Context, attestation *ethpb.IndexedAttestation) ([]*ethpb.AttesterSlashing, error)
}

// Service defining a slasher implementation as part of
// the beacon node, able to detect eth2 slashable offenses.
type Service struct {
	params                 *Parameters
	serviceCfg             *ServiceConfig
	indexedAttsChan        chan *ethpb.IndexedAttestation
	beaconBlockHeadersChan chan *ethpb.SignedBeaconBlockHeader
	attsQueue              *attestationsQueue
	blksQueue              *blocksQueue
	ctx                    context.Context
	cancel                 context.CancelFunc
	slotTicker             *slotutil.SlotTicker
	genesisTime            time.Time
}

// New instantiates a new slasher from configuration values.
func New(ctx context.Context, srvCfg *ServiceConfig) (*Service, error) {
	ctx, cancel := context.WithCancel(ctx)
	return &Service{
		params:                 DefaultParams(),
		serviceCfg:             srvCfg,
		indexedAttsChan:        make(chan *ethpb.IndexedAttestation, 1),
		beaconBlockHeadersChan: make(chan *ethpb.SignedBeaconBlockHeader, 1),
		attsQueue:              newAttestationsQueue(),
		blksQueue:              newBlocksQueue(),
		ctx:                    ctx,
		cancel:                 cancel,
	}, nil
}

// Start listening for received indexed attestations and blocks
// and perform slashing detection on them.
func (s *Service) Start() {
	go s.run()
}

func (s *Service) run() {
	stateChannel := make(chan *feed.Event, 1)
	stateSub := s.serviceCfg.StateNotifier.StateFeed().Subscribe(stateChannel)
	stateEvent := <-stateChannel

	// Wait for us to receive the genesis time via a chain started notification.
	if stateEvent.Type == statefeed.ChainStarted {
		data, ok := stateEvent.Data.(*statefeed.ChainStartedData)
		if !ok {
			log.Error("Could not receive chain start notification, want *statefeed.ChainStartedData")
			return
		}
		s.genesisTime = data.StartTime
		log.WithField("genesisTime", s.genesisTime).Info("Starting slasher, received chain start event")
	} else if stateEvent.Type == statefeed.Initialized {
		// Alternatively, if the chain has already started, we then read the genesis
		// time value from this data.
		data, ok := stateEvent.Data.(*statefeed.InitializedData)
		if !ok {
			log.Error("Could not receive chain start notification, want *statefeed.ChainStartedData")
			return
		}
		s.genesisTime = data.StartTime
		log.WithField("genesisTime", s.genesisTime).Info("Starting slasher, chain already initialized")
	} else {
		// This should not happen.
		log.Error("Could start slasher, could not receive chain start event")
		return
	}

	stateSub.Unsubscribe()
	secondsPerSlot := params.BeaconConfig().SecondsPerSlot
	s.slotTicker = slotutil.NewSlotTicker(s.genesisTime, secondsPerSlot)

	log.Info("Waiting for chain to complete initial sync")
	s.waitForSync(s.genesisTime)

	log.Info("Backfilling any missing data before starting slasher listener")
	s.waitForDataBackfill(s.params.historyLength)

	log.Info("Completed chain sync, starting slashing detection")
	go s.processQueuedAttestations(s.ctx, s.slotTicker.C())
	go s.processQueuedBlocks(s.ctx, s.slotTicker.C())
	go s.receiveAttestations(s.ctx)
	go s.receiveBlocks(s.ctx)
	go s.pruneSlasherData(s.ctx, s.slotTicker.C())
}

func (s *Service) waitForDataBackfill(wssPeriod tyes.Epoch) {
	// The lowest epoch we need to backfill for slasher is based on the
	// head epoch minus the weak subjectivity period.
	headSlot := s.serviceCfg.HeadStateFetcher.HeadSlot()
	headEpoch := helpers.SlotToEpoch(headSlot)
	lowestEpoch := headEpoch
	if lowestEpoch > wssPeriod {
		lowestEpoch = lowestEpoch - wssPeriod
	}

	log.Infof("Beginning slasher data backfill from epoch %d to %d", lowestEpoch, headEpoch)
	start := time.Now()
	s.backfill(lowestEpoch, headEpoch)
	log.Infof("Finished backfilling range with time elapsed %v", time.Since(start))
	lowestEpoch = headEpoch

	for {
		// If we have no difference between the max epoch we have detected for
		// slasher and the current epoch on the clock, then we can exit the loop.
		currentEpoch := slotutil.EpochsSinceGenesis(s.genesisTime)
		diff := currentEpoch
		if diff >= lowestEpoch {
			diff = diff - lowestEpoch
		}
		if diff == 0 {
			break
		}

		// We set the max epoch for slasher to the current epoch on the clock for backfilling.
		maxEpoch := currentEpoch

		log.Infof("Beginning slasher data backfill from epoch %d to %d", lowestEpoch, maxEpoch)
		start := time.Now()
		s.backfill(lowestEpoch, maxEpoch)
		log.Infof("Finished backfilling range with time elapsed %v", time.Since(start))

		// After backfilling, we set the lowest epoch for backfilling to be the
		// max epoch we have completed backfill to.
		lowestEpoch = maxEpoch
	}
}

// Backfill slasher's historical database from blocks in a range of epochs.
// The max range between start and end is approximately 4096 epochs,
// so we perform backfilling in chunks of a set size to reduce impact
// on disk reads and writes during the procedure.
func (s *Service) backfill(start, end types.Epoch) error {
	f := filters.NewFilter().SetStartEpoch(start).SetEndEpoch(end)
	blocks, roots, err := s.serviceCfg.BeaconDatabase.Blocks(s.ctx, f)
	if err != nil {
		return err
	}
	headers := make([]*slashertypes.SignedBlockHeaderWrapper, 0, len(blocks))
	for i, block := range blocks {
		header, err := blockutil.SignedBeaconBlockHeaderFromBlock(block)
		if err != nil {
			return err
		}
		headers = append(headers, &slashertypes.SignedBlockHeaderWrapper{
			SignedBeaconBlockHeader: header,
			SigningRoot:             roots[i],
		})
	}
	slashings, err := s.detectProposerSlashings(s.ctx, headers)
	if err != nil {
		return err
	}
	_ = slashings
	return nil
}

// Stop the slasher service.
func (s *Service) Stop() error {
	s.cancel()
	if s.slotTicker != nil {
		s.slotTicker.Done()
	}
	return nil
}

// Status of the slasher service.
func (s *Service) Status() error {
	return nil
}

func (s *Service) waitForSync(genesisTime time.Time) {
	if slotutil.SlotsSinceGenesis(genesisTime) == 0 || !s.serviceCfg.SyncChecker.Syncing() {
		return
	}
	for {
		select {
		case <-s.slotTicker.C():
			// If node is still syncing, do not operate slasher.
			if s.serviceCfg.SyncChecker.Syncing() {
				continue
			}
			return
		case <-s.ctx.Done():
			return
		}
	}
}
