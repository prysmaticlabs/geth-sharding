// Package slasher --
package slasher

import (
	"context"
	"sync"
	"time"

	ethpb "github.com/prysmaticlabs/ethereumapis/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/beacon-chain/db"
	slashertypes "github.com/prysmaticlabs/prysm/beacon-chain/slasher/types"
	"github.com/prysmaticlabs/prysm/shared/event"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/slotutil"
)

// ServiceConfig for the slasher service in the beacon node.
// This struct allows us to specify required dependencies and
// parameters for slasher to function as needed.
type ServiceConfig struct {
	IndexedAttsFeed    *event.Feed
	BeaconBlocksFeed   *event.Feed
	AttSlashingsFeed   *event.Feed
	BlockSlashingsFeed *event.Feed
	Database           db.Database
	GenesisTime        time.Time
}

// Service defining a slasher implementation as part of
// the beacon node, able to detect eth2 slashable offenses.
type Service struct {
	params                *Parameters
	serviceCfg            *ServiceConfig
	indexedAttsChan       chan *ethpb.IndexedAttestation
	beaconBlocksChan      chan *ethpb.SignedBeaconBlockHeader
	proposerSlashingsFeed *event.Feed
	attesterSlashingsFeed *event.Feed
	attestationQueueLock  sync.Mutex
	blockQueueLock        sync.Mutex
	attestationQueue      []*slashertypes.IndexedAttestationWrapper
	beaconBlocksQueue     []*slashertypes.SignedBlockHeaderWrapper
	ctx                   context.Context
	cancel                context.CancelFunc
	genesisTime           time.Time
}

// New instantiates a new slasher from configuration values.
func New(ctx context.Context, srvCfg *ServiceConfig) (*Service, error) {
	ctx, cancel := context.WithCancel(ctx)
	return &Service{
		params:                DefaultParams(),
		serviceCfg:            srvCfg,
		indexedAttsChan:       make(chan *ethpb.IndexedAttestation, 1),
		beaconBlocksChan:      make(chan *ethpb.SignedBeaconBlockHeader, 1),
		proposerSlashingsFeed: new(event.Feed),
		attesterSlashingsFeed: new(event.Feed),
		attestationQueue:      make([]*slashertypes.IndexedAttestationWrapper, 0),
		beaconBlocksQueue:     make([]*slashertypes.SignedBlockHeaderWrapper, 0),
		ctx:                   ctx,
		cancel:                cancel,
		genesisTime:           time.Now(),
	}, nil
}

// Start listening for received indexed attestations and blocks
// and perform slashing detection on them.
func (s *Service) Start() {
	secondsPerEpoch := params.BeaconConfig().SecondsPerSlot * uint64(params.BeaconConfig().SlotsPerEpoch)
	ticker := slotutil.NewEpochTicker(s.genesisTime, secondsPerEpoch)
	defer ticker.Done()
	go s.processQueuedAttestations(s.ctx, ticker.C())
	go s.processQueuedBlocks(s.ctx, ticker.C())
	go s.receiveAttestations(s.ctx)
	go s.receiveBlocks(s.ctx)
	go s.pruneSlasherData(s.ctx, ticker.C())
}

// Stop the slasher service.
func (s *Service) Stop() error {
	s.cancel()
	return nil
}

// Status of the slasher service.
func (s *Service) Status() error {
	return nil
}
