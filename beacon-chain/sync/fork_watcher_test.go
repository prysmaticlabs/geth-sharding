package sync

import (
	"context"
	"testing"
	"time"

	types "github.com/prysmaticlabs/eth2-types"
	mockChain "github.com/prysmaticlabs/prysm/beacon-chain/blockchain/testing"
	"github.com/prysmaticlabs/prysm/beacon-chain/p2p"
	p2ptest "github.com/prysmaticlabs/prysm/beacon-chain/p2p/testing"
	mockSync "github.com/prysmaticlabs/prysm/beacon-chain/sync/initial-sync/testing"
	"github.com/prysmaticlabs/prysm/shared/abool"
	"github.com/prysmaticlabs/prysm/shared/p2putils"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/testutil/assert"
)

func TestService_CheckForNextEpochFork(t *testing.T) {
	params.SetupTestConfigCleanup(t)
	tests := []struct {
		name         string
		svcCreator   func(t *testing.T) *Service
		currEpoch    types.Epoch
		wantErr      bool
		postSvcCheck func(t *testing.T, s *Service)
	}{
		{
			name: "no fork in the next epoch",
			svcCreator: func(t *testing.T) *Service {
				p2p := p2ptest.NewTestP2P(t)
				chainService := &mockChain.ChainService{
					Genesis:        time.Now().Add(time.Duration(-params.BeaconConfig().SlotsPerEpoch.Mul(uint64(params.BeaconConfig().SlotsPerEpoch))) * time.Second),
					ValidatorsRoot: [32]byte{'A'},
				}
				ctx, cancel := context.WithCancel(context.Background())
				r := &Service{
					ctx:    ctx,
					cancel: cancel,
					cfg: &Config{
						P2P:           p2p,
						Chain:         chainService,
						StateNotifier: chainService.StateNotifier(),
						InitialSync:   &mockSync.Sync{IsSyncing: false},
					},
					chainStarted: abool.New(),
					subHandler:   newSubTopicHandler(),
				}
				return r
			},
			currEpoch: 10,
			wantErr:   false,
			postSvcCheck: func(t *testing.T, s *Service) {

			},
		},
		{
			name: "fork in the next epoch",
			svcCreator: func(t *testing.T) *Service {
				p2p := p2ptest.NewTestP2P(t)
				chainService := &mockChain.ChainService{
					Genesis:        time.Now().Add(-4 * oneEpoch()),
					ValidatorsRoot: [32]byte{'A'},
				}
				bCfg := params.BeaconConfig()
				bCfg.AltairForkEpoch = 5
				params.OverrideBeaconConfig(bCfg)
				params.BeaconConfig().InitializeForkSchedule()
				ctx, cancel := context.WithCancel(context.Background())
				r := &Service{
					ctx:    ctx,
					cancel: cancel,
					cfg: &Config{
						P2P:           p2p,
						Chain:         chainService,
						StateNotifier: chainService.StateNotifier(),
						InitialSync:   &mockSync.Sync{IsSyncing: false},
					},
					chainStarted: abool.New(),
					subHandler:   newSubTopicHandler(),
				}
				return r
			},
			currEpoch: 4,
			wantErr:   false,
			postSvcCheck: func(t *testing.T, s *Service) {
				genRoot := s.cfg.Chain.GenesisValidatorRoot()
				digest, err := p2putils.ForkDigestFromEpoch(5, genRoot[:])
				assert.NoError(t, err)
				assert.Equal(t, true, s.subHandler.digestExists(digest))
				rpcMap := make(map[string]bool)
				for _, p := range s.cfg.P2P.Host().Mux().Protocols() {
					rpcMap[p] = true
				}
				assert.Equal(t, true, rpcMap[p2p.RPCBlocksByRangeTopicV2+s.cfg.P2P.Encoding().ProtocolSuffix()], "topic doesn't exist")
				assert.Equal(t, true, rpcMap[p2p.RPCBlocksByRootTopicV2+s.cfg.P2P.Encoding().ProtocolSuffix()], "topic doesn't exist")
				assert.Equal(t, true, rpcMap[p2p.RPCMetaDataTopicV2+s.cfg.P2P.Encoding().ProtocolSuffix()], "topic doesn't exist")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := tt.svcCreator(t)
			if err := s.checkForNextEpochFork(tt.currEpoch); (err != nil) != tt.wantErr {
				t.Errorf("checkForNextEpochFork() error = %v, wantErr %v", err, tt.wantErr)
			}
			tt.postSvcCheck(t, s)
		})
	}
}

func TestService_CheckForPreviousEpochFork(t *testing.T) {
	params.SetupTestConfigCleanup(t)
	tests := []struct {
		name         string
		svcCreator   func(t *testing.T) *Service
		currEpoch    types.Epoch
		wantErr      bool
		postSvcCheck func(t *testing.T, s *Service)
	}{
		{
			name: "no fork in the previous epoch",
			svcCreator: func(t *testing.T) *Service {
				p2p := p2ptest.NewTestP2P(t)
				chainService := &mockChain.ChainService{
					Genesis:        time.Now().Add(-oneEpoch()),
					ValidatorsRoot: [32]byte{'A'},
				}
				ctx, cancel := context.WithCancel(context.Background())
				r := &Service{
					ctx:    ctx,
					cancel: cancel,
					cfg: &Config{
						P2P:           p2p,
						Chain:         chainService,
						StateNotifier: chainService.StateNotifier(),
						InitialSync:   &mockSync.Sync{IsSyncing: false},
					},
					chainStarted: abool.New(),
					subHandler:   newSubTopicHandler(),
				}
				return r
			},
			currEpoch: 10,
			wantErr:   false,
			postSvcCheck: func(t *testing.T, s *Service) {

			},
		},
		{
			name: "fork in the previous epoch",
			svcCreator: func(t *testing.T) *Service {
				p2p := p2ptest.NewTestP2P(t)
				chainService := &mockChain.ChainService{
					Genesis:        time.Now().Add(-4 * oneEpoch()),
					ValidatorsRoot: [32]byte{'A'},
				}
				bCfg := params.BeaconConfig()
				bCfg.AltairForkEpoch = 3
				params.OverrideBeaconConfig(bCfg)
				params.BeaconConfig().InitializeForkSchedule()
				ctx, cancel := context.WithCancel(context.Background())
				r := &Service{
					ctx:    ctx,
					cancel: cancel,
					cfg: &Config{
						P2P:           p2p,
						Chain:         chainService,
						StateNotifier: chainService.StateNotifier(),
						InitialSync:   &mockSync.Sync{IsSyncing: false},
					},
					chainStarted: abool.New(),
					subHandler:   newSubTopicHandler(),
				}
				genRoot := r.cfg.Chain.GenesisValidatorRoot()
				digest, err := p2putils.ForkDigestFromEpoch(0, genRoot[:])
				assert.NoError(t, err)
				r.registerSubscribers(0, digest)
				assert.Equal(t, true, r.subHandler.digestExists(digest))

				digest, err = p2putils.ForkDigestFromEpoch(3, genRoot[:])
				assert.NoError(t, err)
				r.registerSubscribers(3, digest)
				assert.Equal(t, true, r.subHandler.digestExists(digest))

				return r
			},
			currEpoch: 4,
			wantErr:   false,
			postSvcCheck: func(t *testing.T, s *Service) {
				genRoot := s.cfg.Chain.GenesisValidatorRoot()
				digest, err := p2putils.ForkDigestFromEpoch(0, genRoot[:])
				assert.NoError(t, err)
				assert.Equal(t, false, s.subHandler.digestExists(digest))
				digest, err = p2putils.ForkDigestFromEpoch(3, genRoot[:])
				assert.NoError(t, err)
				assert.Equal(t, true, s.subHandler.digestExists(digest))
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := tt.svcCreator(t)
			if err := s.checkForPreviousEpochFork(tt.currEpoch); (err != nil) != tt.wantErr {
				t.Errorf("checkForNextEpochFork() error = %v, wantErr %v", err, tt.wantErr)
			}
			tt.postSvcCheck(t, s)
		})
	}
}

func oneEpoch() time.Duration {
	return time.Duration(params.BeaconConfig().SlotsPerEpoch.Mul(uint64(params.BeaconConfig().SecondsPerSlot))) * time.Second
}
