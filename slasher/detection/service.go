package detection

import (
	"context"

	ethpb "github.com/prysmaticlabs/ethereumapis/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/shared/event"
	"github.com/prysmaticlabs/prysm/slasher/beaconclient"
	"github.com/prysmaticlabs/prysm/slasher/db"
	"github.com/prysmaticlabs/prysm/slasher/detection/attestations"
	"github.com/prysmaticlabs/prysm/slasher/detection/attestations/iface"
	"github.com/sirupsen/logrus"
	"go.opencensus.io/trace"
)

var log = logrus.WithField("prefix", "detection")

// Service struct for the detection service of the slasher.
type Service struct {
	ctx                   context.Context
	cancel                context.CancelFunc
	slasherDB             db.Database
	blocksChan            chan *ethpb.SignedBeaconBlock
	attsChan              chan *ethpb.IndexedAttestation
	notifier              beaconclient.Notifier
	chainFetcher          beaconclient.ChainFetcher
	beaconClient          *beaconclient.Service
	attesterSlashingsFeed *event.Feed
	proposerSlashingsFeed *event.Feed
	minMaxSpanDetector    iface.SpanDetector
}

// Config options for the detection service.
type Config struct {
	Notifier              beaconclient.Notifier
	SlasherDB             db.Database
	ChainFetcher          beaconclient.ChainFetcher
	BeaconClient          *beaconclient.Service
	AttesterSlashingsFeed *event.Feed
	ProposerSlashingsFeed *event.Feed
}

// NewDetectionService instantiation.
func NewDetectionService(ctx context.Context, cfg *Config) *Service {
	ctx, cancel := context.WithCancel(ctx)
	return &Service{
		ctx:                   ctx,
		cancel:                cancel,
		notifier:              cfg.Notifier,
		chainFetcher:          cfg.ChainFetcher,
		slasherDB:             cfg.SlasherDB,
		beaconClient:          cfg.BeaconClient,
		blocksChan:            make(chan *ethpb.SignedBeaconBlock, 1),
		attsChan:              make(chan *ethpb.IndexedAttestation, 1),
		attesterSlashingsFeed: cfg.AttesterSlashingsFeed,
		proposerSlashingsFeed: cfg.ProposerSlashingsFeed,
		minMaxSpanDetector:    attestations.NewSpanDetector(),
	}
}

// Stop the notifier service.
func (ds *Service) Stop() error {
	ds.cancel()
	log.Info("Stopping service")
	return nil
}

// Status returns an error if there exists an error in
// the notifier service.
func (ds *Service) Status() error {
	return nil
}

// Start the detection service runtime.
func (ds *Service) Start() {

	// We wait for the gRPC beacon client to be ready and the beacon node
	// to be fully synced before proceeding.
	ch := make(chan bool)
	sub := ds.notifier.ClientReadyFeed().Subscribe(ch)
	<-ch
	sub.Unsubscribe()

	// The detection service runs detection on all historical
	// chain data since genesis.
	go ds.detectHistoricalChainData(ds.ctx)

	// We subscribe to incoming blocks from the beacon node via
	// our gRPC client to keep detecting slashable offenses.
	go ds.detectIncomingBlocks(ds.ctx, ds.blocksChan)
	go ds.detectIncomingAttestations(ds.ctx, ds.attsChan)
}

func (ds *Service) detectHistoricalChainData(ctx context.Context) {
	ctx, span := trace.StartSpan(ctx, "detection.detectHistoricalChainData")
	defer span.End()
	// We fetch both the latest persisted chain head in our DB as well
	// as the current chain head from the beacon node via gRPC.
	latestStoredHead, err := ds.slasherDB.ChainHead(ctx)
	if err != nil {
		log.WithError(err).Fatal("Could not retrieve chain head from DB")
	}
	currentChainHead, err := ds.chainFetcher.ChainHead(ctx)
	if err != nil {
		log.WithError(err).Fatal("Cannot retrieve chain head from beacon node")
	}
	var latestStoredEpoch uint64
	if latestStoredHead != nil {
		latestStoredEpoch = latestStoredHead.HeadEpoch
	}
	// We retrieve historical chain data from the last persisted chain head in the
	// slasher DB up to the current beacon node's head epoch we retrieved via gRPC.
	// If no data was persisted from previous sessions, we request data starting from
	// the genesis epoch.
	for epoch := latestStoredEpoch; epoch < currentChainHead.HeadEpoch; epoch++ {
		indexedAtts, err := ds.beaconClient.RequestHistoricalAttestations(ctx, epoch)
		if err != nil {
			log.WithError(err).Errorf("Could not fetch attestations for epoch: %d", epoch)
		}
		log.Debugf(
			"Running slashing detection on %d attestations in epoch %d...",
			len(indexedAtts),
			epoch,
		)
		for _, att := range indexedAtts {
			slashings, err := ds.detectAttesterSlashings(ctx, att)
			if err != nil {
				log.WithError(err).Error("Could not detect attester slashings")
				continue
			}
			ds.submitAttesterSlashings(ctx, slashings)
		}
	}
	if err := ds.slasherDB.SaveChainHead(ctx, currentChainHead); err != nil {
		log.WithError(err).Error("Could not persist chain head to disk")
	}
	log.Infof("Completed slashing detection on historical chain data up to epoch %d", currentChainHead.HeadEpoch)
}

func (ds *Service) submitAttesterSlashings(ctx context.Context, slashings []*ethpb.AttesterSlashing) {
	if len(slashings) > 0 {
		log.Infof("Found %d attester slashings, submitting to beacon node...", len(slashings))
	}
	for i := 0; i < len(slashings); i++ {
		ds.attesterSlashingsFeed.Send(slashings[i])
	}
}
