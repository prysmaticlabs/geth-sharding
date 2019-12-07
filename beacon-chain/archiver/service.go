package archiver

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"github.com/prysmaticlabs/prysm/beacon-chain/blockchain"
	epochProcessing "github.com/prysmaticlabs/prysm/beacon-chain/core/epoch"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/feed"
	statefeed "github.com/prysmaticlabs/prysm/beacon-chain/core/feed/state"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/validators"
	"github.com/prysmaticlabs/prysm/beacon-chain/db"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/sirupsen/logrus"
)

var log = logrus.WithField("prefix", "archiver")

// Service defining archiver functionality for persisting checkpointed
// beacon chain information to a database backend for historical purposes.
type Service struct {
	ctx               context.Context
	cancel            context.CancelFunc
	beaconDB          db.Database
	headFetcher       blockchain.HeadFetcher
	stateNotifier     statefeed.Notifier
	lastArchivedEpoch uint64
}

// Config options for the archiver service.
type Config struct {
	BeaconDB      db.Database
	HeadFetcher   blockchain.HeadFetcher
	StateNotifier statefeed.Notifier
}

// NewArchiverService initializes the service from configuration options.
func NewArchiverService(ctx context.Context, cfg *Config) *Service {
	ctx, cancel := context.WithCancel(ctx)
	return &Service{
		ctx:           ctx,
		cancel:        cancel,
		beaconDB:      cfg.BeaconDB,
		headFetcher:   cfg.HeadFetcher,
		stateNotifier: cfg.StateNotifier,
	}
}

// Start the archiver service event loop.
func (s *Service) Start() {
	go s.run(s.ctx)
}

// Stop the archiver service event loop.
func (s *Service) Stop() error {
	defer s.cancel()
	return nil
}

// Status reports the healthy status of the archiver. Returning nil means service
// is correctly running without error.
func (s *Service) Status() error {
	return nil
}

// We archive committee information pertaining to the head state's epoch.
func (s *Service) archiveCommitteeInfo(ctx context.Context, headState *pb.BeaconState, epoch uint64) error {
	proposerSeed, err := helpers.Seed(headState, epoch, params.BeaconConfig().DomainBeaconProposer)
	if err != nil {
		return errors.Wrap(err, "could not generate seed")
	}
	attesterSeed, err := helpers.Seed(headState, epoch, params.BeaconConfig().DomainBeaconAttester)
	if err != nil {
		return errors.Wrap(err, "could not generate seed")
	}

	info := &pb.ArchivedCommitteeInfo{
		ProposerSeed: proposerSeed[:],
		AttesterSeed: attesterSeed[:],
	}
	if err := s.beaconDB.SaveArchivedCommitteeInfo(ctx, epoch, info); err != nil {
		return errors.Wrap(err, "could not archive committee info")
	}
	return nil
}

// We archive active validator set changes that happened during the previous epoch.
func (s *Service) archiveActiveSetChanges(ctx context.Context, headState *pb.BeaconState, epoch uint64) error {
	prevEpoch := epoch - 1
	activations := validators.ActivatedValidatorIndices(prevEpoch, headState.Validators)
	slashings := validators.SlashedValidatorIndices(prevEpoch, headState.Validators)
	activeValidatorCount, err := helpers.ActiveValidatorCount(headState, prevEpoch)
	if err != nil {
		return errors.Wrap(err, "could not get active validator count")
	}
	exited, err := validators.ExitedValidatorIndices(prevEpoch, headState.Validators, activeValidatorCount)
	if err != nil {
		return errors.Wrap(err, "could not determine exited validator indices")
	}
	activeSetChanges := &pb.ArchivedActiveSetChanges{
		Activated: activations,
		Exited:    exited,
		Slashed:   slashings,
	}
	if err := s.beaconDB.SaveArchivedActiveValidatorChanges(ctx, prevEpoch, activeSetChanges); err != nil {
		return errors.Wrap(err, "could not archive active validator set changes")
	}
	return nil
}

// We compute participation metrics by first retrieving the head state and
// matching validator attestations during the epoch.
func (s *Service) archiveParticipation(ctx context.Context, headState *pb.BeaconState, epoch uint64) error {
	participation, err := epochProcessing.ComputeValidatorParticipation(headState, epoch)
	if err != nil {
		return errors.Wrap(err, "could not compute participation")
	}
	return s.beaconDB.SaveArchivedValidatorParticipation(ctx, epoch, participation)
}

// We archive validator balances and active indices.
func (s *Service) archiveBalances(ctx context.Context, headState *pb.BeaconState, epoch uint64) error {
	balances := headState.Balances
	if err := s.beaconDB.SaveArchivedBalances(ctx, epoch, balances); err != nil {
		return errors.Wrap(err, "could not archive balances")
	}
	return nil
}

func (s *Service) run(ctx context.Context) {
	stateChannel := make(chan *feed.Event, 1)
	stateSub := s.stateNotifier.StateFeed().Subscribe(stateChannel)
	defer stateSub.Unsubscribe()
	for {
		select {
		case event := <-stateChannel:
			if event.Type == statefeed.BlockProcessed {
				data := event.Data.(*statefeed.BlockProcessedData)
				log.WithField("headRoot", fmt.Sprintf("%#x", data.BlockRoot)).Debug("Received block processed event")
				headState, err := s.headFetcher.HeadState(ctx)
				if err != nil {
					log.WithError(err).Error("Head state is not available")
					continue
				}
				currentEpoch := helpers.CurrentEpoch(headState)
				if !helpers.IsEpochEnd(headState.Slot) && currentEpoch <= s.lastArchivedEpoch {
					continue
				}
				epochToArchive := currentEpoch
				if !helpers.IsEpochEnd(headState.Slot) {
					epochToArchive--
				}
				if err := s.archiveCommitteeInfo(ctx, headState, epochToArchive); err != nil {
					log.WithError(err).Error("Could not archive committee info")
					continue
				}
				if err := s.archiveActiveSetChanges(ctx, headState, epochToArchive); err != nil {
					log.WithError(err).Error("Could not archive active validator set changes")
					continue
				}
				if err := s.archiveParticipation(ctx, headState, epochToArchive); err != nil {
					log.WithError(err).Error("Could not archive validator participation")
					continue
				}
				if err := s.archiveBalances(ctx, headState, epochToArchive); err != nil {
					log.WithError(err).Error("Could not archive validator balances and active indices")
					continue
				}
				log.WithField(
					"epoch",
					epochToArchive,
				).Debug("Successfully archived beacon chain data during epoch")
				s.lastArchivedEpoch = epochToArchive
			}
		case <-s.ctx.Done():
			log.Debug("Context closed, exiting goroutine")
			return
		case err := <-stateSub.Err():
			log.WithError(err).Error("Subscription to state feed notifier failed")
			return
		}
	}
}
