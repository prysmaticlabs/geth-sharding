package sync

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/golang/snappy"
	"github.com/libp2p/go-libp2p-core/peer"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	pubsub_pb "github.com/libp2p/go-libp2p-pubsub/pb"
	types "github.com/prysmaticlabs/eth2-types"
	"github.com/prysmaticlabs/go-bitfield"
	mockChain "github.com/prysmaticlabs/prysm/beacon-chain/blockchain/testing"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/altair"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/state"
	"github.com/prysmaticlabs/prysm/beacon-chain/db"
	testingDB "github.com/prysmaticlabs/prysm/beacon-chain/db/testing"
	"github.com/prysmaticlabs/prysm/beacon-chain/p2p"
	"github.com/prysmaticlabs/prysm/beacon-chain/p2p/encoder"
	mockp2p "github.com/prysmaticlabs/prysm/beacon-chain/p2p/testing"
	p2ptypes "github.com/prysmaticlabs/prysm/beacon-chain/p2p/types"
	"github.com/prysmaticlabs/prysm/beacon-chain/state/stategen"
	mockSync "github.com/prysmaticlabs/prysm/beacon-chain/sync/initial-sync/testing"
	p2ppb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	prysmv2 "github.com/prysmaticlabs/prysm/proto/prysm/v2"
	"github.com/prysmaticlabs/prysm/proto/prysm/v2/wrapper"
	"github.com/prysmaticlabs/prysm/shared/bls"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/testutil"
	"github.com/prysmaticlabs/prysm/shared/testutil/assert"
	"github.com/prysmaticlabs/prysm/shared/testutil/require"
)

func TestService_ValidateSyncContributionAndProof(t *testing.T) {
	db := testingDB.SetupDB(t)
	headRoot, keys := fillUpBlocksAndState(context.Background(), t, db)
	defaultTopic := p2p.SyncContributionAndProofSubnetTopicFormat
	defaultTopic = fmt.Sprintf(defaultTopic, []byte{0xAB, 0x00, 0xCC, 0x9E})
	defaultTopic = defaultTopic + "/" + encoder.ProtocolSuffixSSZSnappy
	chainService := &mockChain.ChainService{
		Genesis:        time.Now(),
		ValidatorsRoot: [32]byte{'A'},
	}
	emptySig := [96]byte{}
	type args struct {
		ctx   context.Context
		pid   peer.ID
		msg   *prysmv2.SignedContributionAndProof
		topic string
	}
	tests := []struct {
		name     string
		svc      *Service
		setupSvc func(s *Service, msg *prysmv2.SignedContributionAndProof) *Service
		args     args
		want     pubsub.ValidationResult
	}{
		{
			name: "Is syncing",
			svc: NewService(context.Background(), &Config{
				P2P:               mockp2p.NewTestP2P(t),
				InitialSync:       &mockSync.Sync{IsSyncing: true},
				Chain:             chainService,
				StateNotifier:     chainService.StateNotifier(),
				OperationNotifier: chainService.OperationNotifier(),
			}),
			setupSvc: func(s *Service, msg *prysmv2.SignedContributionAndProof) *Service {
				s.cfg.StateGen = stategen.New(db)
				msg.Message.Contribution.BlockRoot = headRoot[:]
				s.cfg.DB = db
				assert.NoError(t, s.initCaches())
				return s
			},
			args: args{
				ctx:   context.Background(),
				pid:   "random",
				topic: "junk",
				msg: &prysmv2.SignedContributionAndProof{
					Message: &prysmv2.ContributionAndProof{
						AggregatorIndex: 1,
						Contribution: &prysmv2.SyncCommitteeContribution{
							Slot:              1,
							SubcommitteeIndex: 1,
							BlockRoot:         params.BeaconConfig().ZeroHash[:],
							AggregationBits:   bitfield.NewBitvector128(),
							Signature:         emptySig[:],
						},
						SelectionProof: emptySig[:],
					},
					Signature: emptySig[:],
				}},
			want: pubsub.ValidationIgnore,
		},
		{
			name: "Bad Topic",
			svc: NewService(context.Background(), &Config{
				P2P:               mockp2p.NewTestP2P(t),
				InitialSync:       &mockSync.Sync{IsSyncing: false},
				Chain:             chainService,
				StateNotifier:     chainService.StateNotifier(),
				OperationNotifier: chainService.OperationNotifier(),
			}),
			setupSvc: func(s *Service, msg *prysmv2.SignedContributionAndProof) *Service {
				s.cfg.StateGen = stategen.New(db)
				msg.Message.Contribution.BlockRoot = headRoot[:]
				s.cfg.DB = db
				assert.NoError(t, s.initCaches())
				return s
			},
			args: args{
				ctx:   context.Background(),
				pid:   "random",
				topic: "junk",
				msg: &prysmv2.SignedContributionAndProof{
					Message: &prysmv2.ContributionAndProof{
						AggregatorIndex: 1,
						Contribution: &prysmv2.SyncCommitteeContribution{
							Slot:              1,
							SubcommitteeIndex: 1,
							BlockRoot:         params.BeaconConfig().ZeroHash[:],
							AggregationBits:   bitfield.NewBitvector128(),
							Signature:         emptySig[:],
						},
						SelectionProof: emptySig[:],
					},
					Signature: emptySig[:],
				}},
			want: pubsub.ValidationReject,
		},
		{
			name: "Future Slot Message",
			svc: NewService(context.Background(), &Config{
				P2P:               mockp2p.NewTestP2P(t),
				InitialSync:       &mockSync.Sync{IsSyncing: false},
				Chain:             chainService,
				StateNotifier:     chainService.StateNotifier(),
				OperationNotifier: chainService.OperationNotifier(),
			}),
			setupSvc: func(s *Service, msg *prysmv2.SignedContributionAndProof) *Service {
				s.cfg.StateGen = stategen.New(db)
				s.cfg.DB = db
				assert.NoError(t, s.initCaches())
				return s
			},
			args: args{
				ctx:   context.Background(),
				pid:   "random",
				topic: defaultTopic,
				msg: &prysmv2.SignedContributionAndProof{
					Message: &prysmv2.ContributionAndProof{
						AggregatorIndex: 1,
						Contribution: &prysmv2.SyncCommitteeContribution{
							Slot:              30,
							SubcommitteeIndex: 1,
							BlockRoot:         params.BeaconConfig().ZeroHash[:],
							AggregationBits:   bitfield.NewBitvector128(),
							Signature:         emptySig[:],
						},
						SelectionProof: emptySig[:],
					},
					Signature: emptySig[:],
				}},
			want: pubsub.ValidationIgnore,
		},
		{
			name: "Already Seen Message",
			svc: NewService(context.Background(), &Config{
				P2P:               mockp2p.NewTestP2P(t),
				InitialSync:       &mockSync.Sync{IsSyncing: false},
				Chain:             chainService,
				StateNotifier:     chainService.StateNotifier(),
				OperationNotifier: chainService.OperationNotifier(),
			}),
			setupSvc: func(s *Service, msg *prysmv2.SignedContributionAndProof) *Service {
				s.cfg.StateGen = stategen.New(db)
				s.cfg.DB = db
				assert.NoError(t, s.initCaches())
				msg.Message.Contribution.BlockRoot = headRoot[:]

				s.setSyncContributionIndexSlotSeen(1, 1, 1)
				return s
			},
			args: args{
				ctx:   context.Background(),
				pid:   "random",
				topic: defaultTopic,
				msg: &prysmv2.SignedContributionAndProof{
					Message: &prysmv2.ContributionAndProof{
						AggregatorIndex: 1,
						Contribution: &prysmv2.SyncCommitteeContribution{
							Slot:              1,
							SubcommitteeIndex: 1,
							BlockRoot:         params.BeaconConfig().ZeroHash[:],
							AggregationBits:   bitfield.NewBitvector128(),
							Signature:         emptySig[:],
						},
						SelectionProof: emptySig[:],
					},
					Signature: emptySig[:],
				}},
			want: pubsub.ValidationIgnore,
		},
		{
			name: "Invalid Selection Proof",
			svc: NewService(context.Background(), &Config{
				P2P:               mockp2p.NewTestP2P(t),
				InitialSync:       &mockSync.Sync{IsSyncing: false},
				Chain:             chainService,
				StateNotifier:     chainService.StateNotifier(),
				OperationNotifier: chainService.OperationNotifier(),
			}),
			setupSvc: func(s *Service, msg *prysmv2.SignedContributionAndProof) *Service {
				s.cfg.StateGen = stategen.New(db)
				s.cfg.DB = db
				assert.NoError(t, s.initCaches())
				s.cfg.Chain = &mockChain.ChainService{
					ValidatorsRoot: [32]byte{'A'},
					Genesis:        time.Now().Add(-time.Second * time.Duration(params.BeaconConfig().SecondsPerSlot) * time.Duration(10)),
				}
				msg.Message.Contribution.BlockRoot = headRoot[:]
				incorrectProof := [96]byte{0xBB}
				msg.Message.SelectionProof = incorrectProof[:]

				return s
			},
			args: args{
				ctx:   context.Background(),
				pid:   "random",
				topic: defaultTopic,
				msg: &prysmv2.SignedContributionAndProof{
					Message: &prysmv2.ContributionAndProof{
						AggregatorIndex: 1,
						Contribution: &prysmv2.SyncCommitteeContribution{
							Slot:              1,
							SubcommitteeIndex: 1,
							BlockRoot:         params.BeaconConfig().ZeroHash[:],
							AggregationBits:   bitfield.NewBitvector128(),
							Signature:         emptySig[:],
						},
						SelectionProof: emptySig[:],
					},
					Signature: emptySig[:],
				}},
			want: pubsub.ValidationReject,
		},
		{
			name: "Invalid Aggregator",
			svc: NewService(context.Background(), &Config{
				P2P:               mockp2p.NewTestP2P(t),
				InitialSync:       &mockSync.Sync{IsSyncing: false},
				Chain:             chainService,
				StateNotifier:     chainService.StateNotifier(),
				OperationNotifier: chainService.OperationNotifier(),
			}),
			setupSvc: func(s *Service, msg *prysmv2.SignedContributionAndProof) *Service {
				s.cfg.StateGen = stategen.New(db)
				s.cfg.DB = db
				assert.NoError(t, s.initCaches())
				s.cfg.Chain = &mockChain.ChainService{
					ValidatorsRoot: [32]byte{'A'},
					Genesis:        time.Now().Add(-time.Second * time.Duration(params.BeaconConfig().SecondsPerSlot) * time.Duration(10)),
				}
				msg.Message.Contribution.BlockRoot = headRoot[:]
				hState, err := db.State(context.Background(), headRoot)
				assert.NoError(t, err)
				for i := uint64(0); i < params.BeaconConfig().SyncCommitteeSubnetCount; i++ {
					coms, err := altair.SyncSubCommitteePubkeys(hState, types.CommitteeIndex(i))
					assert.NoError(t, err)
					for _, p := range coms {
						idx, ok := hState.ValidatorIndexByPubkey(bytesutil.ToBytes48(p))
						assert.Equal(t, true, ok)
						rt, err := altair.SyncSelectionProofSigningRoot(hState, helpers.PrevSlot(hState.Slot()), types.CommitteeIndex(i))
						assert.NoError(t, err)
						sig := keys[idx].Sign(rt[:])
						if !altair.IsSyncCommitteeAggregator(sig.Marshal()) {
							msg.Message.AggregatorIndex = idx
							break
						}
					}
				}
				return s
			},
			args: args{
				ctx:   context.Background(),
				pid:   "random",
				topic: defaultTopic,
				msg: &prysmv2.SignedContributionAndProof{
					Message: &prysmv2.ContributionAndProof{
						AggregatorIndex: 1,
						Contribution: &prysmv2.SyncCommitteeContribution{
							Slot:              1,
							SubcommitteeIndex: 1,
							BlockRoot:         params.BeaconConfig().ZeroHash[:],
							AggregationBits:   bitfield.NewBitvector128(),
							Signature:         emptySig[:],
						},
						SelectionProof: emptySig[:],
					},
					Signature: emptySig[:],
				}},
			want: pubsub.ValidationReject,
		},
		{
			name: "Failed Selection Proof Verification ",
			svc: NewService(context.Background(), &Config{
				P2P:               mockp2p.NewTestP2P(t),
				InitialSync:       &mockSync.Sync{IsSyncing: false},
				Chain:             chainService,
				StateNotifier:     chainService.StateNotifier(),
				OperationNotifier: chainService.OperationNotifier(),
			}),
			setupSvc: func(s *Service, msg *prysmv2.SignedContributionAndProof) *Service {
				s.cfg.StateGen = stategen.New(db)
				s.cfg.DB = db
				msg.Message.Contribution.BlockRoot = headRoot[:]
				hState, err := db.State(context.Background(), headRoot)
				assert.NoError(t, err)
				for i := uint64(0); i < params.BeaconConfig().SyncCommitteeSubnetCount; i++ {
					coms, err := altair.SyncSubCommitteePubkeys(hState, types.CommitteeIndex(i))
					assert.NoError(t, err)
					for _, p := range coms {
						idx, ok := hState.ValidatorIndexByPubkey(bytesutil.ToBytes48(p))
						assert.Equal(t, true, ok)
						rt, err := altair.SyncSelectionProofSigningRoot(hState, helpers.PrevSlot(hState.Slot()), types.CommitteeIndex(i))
						assert.NoError(t, err)
						sig := keys[idx].Sign(rt[:])
						if altair.IsSyncCommitteeAggregator(sig.Marshal()) {
							msg.Message.AggregatorIndex = idx
							break
						}
					}
				}
				s.cfg.Chain = &mockChain.ChainService{
					ValidatorsRoot: [32]byte{'A'},
					Genesis:        time.Now().Add(-time.Second * time.Duration(params.BeaconConfig().SecondsPerSlot) * time.Duration(hState.Slot()-1)),
				}

				assert.NoError(t, s.initCaches())
				return s
			},
			args: args{
				ctx:   context.Background(),
				pid:   "random",
				topic: defaultTopic,
				msg: &prysmv2.SignedContributionAndProof{
					Message: &prysmv2.ContributionAndProof{
						AggregatorIndex: 1,
						Contribution: &prysmv2.SyncCommitteeContribution{
							Slot:              1,
							SubcommitteeIndex: 1,
							BlockRoot:         params.BeaconConfig().ZeroHash[:],
							AggregationBits:   bitfield.NewBitvector128(),
							Signature:         emptySig[:],
						},
						SelectionProof: emptySig[:],
					},
					Signature: emptySig[:],
				}},
			want: pubsub.ValidationReject,
		},
		{
			name: "Invalid Proof Signature",
			svc: NewService(context.Background(), &Config{
				P2P:               mockp2p.NewTestP2P(t),
				InitialSync:       &mockSync.Sync{IsSyncing: false},
				Chain:             chainService,
				StateNotifier:     chainService.StateNotifier(),
				OperationNotifier: chainService.OperationNotifier(),
			}),
			setupSvc: func(s *Service, msg *prysmv2.SignedContributionAndProof) *Service {
				s.cfg.StateGen = stategen.New(db)
				s.cfg.DB = db
				msg.Message.Contribution.BlockRoot = headRoot[:]
				hState, err := db.State(context.Background(), headRoot)
				assert.NoError(t, err)
				for i := uint64(0); i < params.BeaconConfig().SyncCommitteeSubnetCount; i++ {
					coms, err := altair.SyncSubCommitteePubkeys(hState, types.CommitteeIndex(i))
					assert.NoError(t, err)
					for _, p := range coms {
						idx, ok := hState.ValidatorIndexByPubkey(bytesutil.ToBytes48(p))
						assert.Equal(t, true, ok)
						rt, err := altair.SyncSelectionProofSigningRoot(hState, helpers.PrevSlot(hState.Slot()), types.CommitteeIndex(i))
						assert.NoError(t, err)
						sig := keys[idx].Sign(rt[:])
						if altair.IsSyncCommitteeAggregator(sig.Marshal()) {
							infiniteSig := [96]byte{0xC0}
							msg.Message.AggregatorIndex = idx
							msg.Message.SelectionProof = sig.Marshal()
							msg.Message.Contribution.Slot = helpers.PrevSlot(hState.Slot())
							msg.Message.Contribution.SubcommitteeIndex = i
							msg.Message.Contribution.Signature = infiniteSig[:]
							msg.Message.Contribution.BlockRoot = headRoot[:]
							msg.Message.Contribution.AggregationBits = bitfield.NewBitvector128()
							msg.Signature = infiniteSig[:]
							break
						}
					}
				}
				s.cfg.Chain = &mockChain.ChainService{
					ValidatorsRoot: [32]byte{'A'},
					Genesis:        time.Now().Add(-time.Second * time.Duration(params.BeaconConfig().SecondsPerSlot) * time.Duration(hState.Slot()-1)),
				}

				assert.NoError(t, s.initCaches())
				return s
			},
			args: args{
				ctx:   context.Background(),
				pid:   "random",
				topic: defaultTopic,
				msg: &prysmv2.SignedContributionAndProof{
					Message: &prysmv2.ContributionAndProof{
						AggregatorIndex: 1,
						Contribution: &prysmv2.SyncCommitteeContribution{
							Slot:              1,
							SubcommitteeIndex: 1,
							BlockRoot:         params.BeaconConfig().ZeroHash[:],
							AggregationBits:   bitfield.NewBitvector128(),
							Signature:         emptySig[:],
						},
						SelectionProof: emptySig[:],
					},
					Signature: emptySig[:],
				}},
			want: pubsub.ValidationReject,
		},
		{
			name: "Invalid Sync Aggregate",
			svc: NewService(context.Background(), &Config{
				P2P:               mockp2p.NewTestP2P(t),
				InitialSync:       &mockSync.Sync{IsSyncing: false},
				Chain:             chainService,
				StateNotifier:     chainService.StateNotifier(),
				OperationNotifier: chainService.OperationNotifier(),
			}),
			setupSvc: func(s *Service, msg *prysmv2.SignedContributionAndProof) *Service {
				s.cfg.StateGen = stategen.New(db)
				s.cfg.DB = db
				msg.Message.Contribution.BlockRoot = headRoot[:]
				hState, err := db.State(context.Background(), headRoot)
				assert.NoError(t, err)
				for i := uint64(0); i < params.BeaconConfig().SyncCommitteeSubnetCount; i++ {
					coms, err := altair.SyncSubCommitteePubkeys(hState, types.CommitteeIndex(i))
					assert.NoError(t, err)
					for _, p := range coms {
						idx, ok := hState.ValidatorIndexByPubkey(bytesutil.ToBytes48(p))
						assert.Equal(t, true, ok)
						rt, err := altair.SyncSelectionProofSigningRoot(hState, helpers.PrevSlot(hState.Slot()), types.CommitteeIndex(i))
						assert.NoError(t, err)
						sig := keys[idx].Sign(rt[:])
						if altair.IsSyncCommitteeAggregator(sig.Marshal()) {
							infiniteSig := [96]byte{0xC0}
							junkRoot := [32]byte{'A'}
							badSig := keys[idx].Sign(junkRoot[:])
							msg.Message.AggregatorIndex = idx
							msg.Message.SelectionProof = sig.Marshal()
							msg.Message.Contribution.Slot = helpers.PrevSlot(hState.Slot())
							msg.Message.Contribution.SubcommitteeIndex = i
							msg.Message.Contribution.Signature = badSig.Marshal()
							msg.Message.Contribution.BlockRoot = headRoot[:]
							msg.Message.Contribution.AggregationBits = bitfield.NewBitvector128()
							msg.Signature = infiniteSig[:]

							d, err := helpers.Domain(hState.Fork(), helpers.SlotToEpoch(helpers.PrevSlot(hState.Slot())), params.BeaconConfig().DomainContributionAndProof, hState.GenesisValidatorRoot())
							assert.NoError(t, err)
							sigRoot, err := helpers.ComputeSigningRoot(msg.Message, d)
							assert.NoError(t, err)
							contrSig := keys[idx].Sign(sigRoot[:])

							msg.Signature = contrSig.Marshal()
							break
						}
					}
				}
				s.cfg.Chain = &mockChain.ChainService{
					ValidatorsRoot: [32]byte{'A'},
					Genesis:        time.Now().Add(-time.Second * time.Duration(params.BeaconConfig().SecondsPerSlot) * time.Duration(hState.Slot()-1)),
				}

				assert.NoError(t, s.initCaches())
				return s
			},
			args: args{
				ctx:   context.Background(),
				pid:   "random",
				topic: defaultTopic,
				msg: &prysmv2.SignedContributionAndProof{
					Message: &prysmv2.ContributionAndProof{
						AggregatorIndex: 1,
						Contribution: &prysmv2.SyncCommitteeContribution{
							Slot:              1,
							SubcommitteeIndex: 1,
							BlockRoot:         params.BeaconConfig().ZeroHash[:],
							AggregationBits:   bitfield.NewBitvector128(),
							Signature:         emptySig[:],
						},
						SelectionProof: emptySig[:],
					},
					Signature: emptySig[:],
				}},
			want: pubsub.ValidationReject,
		},
		{
			name: "Valid Signed Sync Contribution And Proof",
			svc: NewService(context.Background(), &Config{
				P2P:               mockp2p.NewTestP2P(t),
				InitialSync:       &mockSync.Sync{IsSyncing: false},
				Chain:             chainService,
				StateNotifier:     chainService.StateNotifier(),
				OperationNotifier: chainService.OperationNotifier(),
			}),
			setupSvc: func(s *Service, msg *prysmv2.SignedContributionAndProof) *Service {
				s.cfg.StateGen = stategen.New(db)
				msg.Message.Contribution.BlockRoot = headRoot[:]
				s.cfg.DB = db
				hState, err := db.State(context.Background(), headRoot)
				assert.NoError(t, err)
				for i := uint64(0); i < params.BeaconConfig().SyncCommitteeSubnetCount; i++ {
					coms, err := altair.SyncSubCommitteePubkeys(hState, types.CommitteeIndex(i))
					assert.NoError(t, err)
					for _, p := range coms {
						idx, ok := hState.ValidatorIndexByPubkey(bytesutil.ToBytes48(p))
						assert.Equal(t, true, ok)
						rt, err := altair.SyncSelectionProofSigningRoot(hState, helpers.PrevSlot(hState.Slot()), types.CommitteeIndex(i))
						assert.NoError(t, err)
						sig := keys[idx].Sign(rt[:])
						if altair.IsSyncCommitteeAggregator(sig.Marshal()) {
							infiniteSig := [96]byte{0xC0}
							msg.Message.AggregatorIndex = idx
							msg.Message.SelectionProof = sig.Marshal()
							msg.Message.Contribution.Slot = helpers.PrevSlot(hState.Slot())
							msg.Message.Contribution.SubcommitteeIndex = i
							msg.Message.Contribution.Signature = infiniteSig[:]
							msg.Message.Contribution.BlockRoot = headRoot[:]
							msg.Message.Contribution.AggregationBits = bitfield.NewBitvector128()
							d, err := helpers.Domain(hState.Fork(), helpers.SlotToEpoch(helpers.PrevSlot(hState.Slot())), params.BeaconConfig().DomainContributionAndProof, hState.GenesisValidatorRoot())
							assert.NoError(t, err)
							sigRoot, err := helpers.ComputeSigningRoot(msg.Message, d)
							assert.NoError(t, err)
							contrSig := keys[idx].Sign(sigRoot[:])

							msg.Signature = contrSig.Marshal()
							break
						}
					}
				}
				s.cfg.Chain = &mockChain.ChainService{
					ValidatorsRoot: [32]byte{'A'},
					Genesis:        time.Now().Add(-time.Second * time.Duration(params.BeaconConfig().SecondsPerSlot) * time.Duration(hState.Slot()-1)),
				}

				assert.NoError(t, s.initCaches())
				return s
			},
			args: args{
				ctx:   context.Background(),
				pid:   "random",
				topic: defaultTopic,
				msg: &prysmv2.SignedContributionAndProof{
					Message: &prysmv2.ContributionAndProof{
						AggregatorIndex: 1,
						Contribution: &prysmv2.SyncCommitteeContribution{
							Slot:              1,
							SubcommitteeIndex: 1,
							BlockRoot:         params.BeaconConfig().ZeroHash[:],
							AggregationBits:   bitfield.NewBitvector128(),
							Signature:         emptySig[:],
						},
						SelectionProof: emptySig[:],
					},
					Signature: emptySig[:],
				}},
			want: pubsub.ValidationAccept,
		},
		{
			name: "Valid Signed Sync Contribution And Proof with Multiple Signatures",
			svc: NewService(context.Background(), &Config{
				P2P:               mockp2p.NewTestP2P(t),
				InitialSync:       &mockSync.Sync{IsSyncing: false},
				Chain:             chainService,
				StateNotifier:     chainService.StateNotifier(),
				OperationNotifier: chainService.OperationNotifier(),
			}),
			setupSvc: func(s *Service, msg *prysmv2.SignedContributionAndProof) *Service {
				s.cfg.StateGen = stategen.New(db)
				msg.Message.Contribution.BlockRoot = headRoot[:]
				s.cfg.DB = db
				hState, err := db.State(context.Background(), headRoot)
				assert.NoError(t, err)
				for i := uint64(0); i < params.BeaconConfig().SyncCommitteeSubnetCount; i++ {
					coms, err := altair.SyncSubCommitteePubkeys(hState, types.CommitteeIndex(i))
					assert.NoError(t, err)
					for _, p := range coms {
						idx, ok := hState.ValidatorIndexByPubkey(bytesutil.ToBytes48(p))
						assert.Equal(t, true, ok)
						rt, err := altair.SyncSelectionProofSigningRoot(hState, helpers.PrevSlot(hState.Slot()), types.CommitteeIndex(i))
						assert.NoError(t, err)
						sig := keys[idx].Sign(rt[:])
						if altair.IsSyncCommitteeAggregator(sig.Marshal()) {
							infiniteSig := [96]byte{0xC0}
							msg.Message.AggregatorIndex = idx
							msg.Message.SelectionProof = sig.Marshal()
							msg.Message.Contribution.Slot = helpers.PrevSlot(hState.Slot())
							msg.Message.Contribution.SubcommitteeIndex = i
							msg.Message.Contribution.Signature = infiniteSig[:]
							msg.Message.Contribution.BlockRoot = headRoot[:]
							msg.Message.Contribution.AggregationBits = bitfield.NewBitvector128()
							d, err := helpers.Domain(hState.Fork(), helpers.SlotToEpoch(hState.Slot()), params.BeaconConfig().DomainSyncCommittee, hState.GenesisValidatorRoot())
							assert.NoError(t, err)
							rawBytes := p2ptypes.SSZBytes(headRoot[:])
							sigRoot, err := helpers.ComputeSigningRoot(&rawBytes, d)
							assert.NoError(t, err)
							sigs := []bls.Signature{}
							for i, p2 := range coms {
								idx, ok := hState.ValidatorIndexByPubkey(bytesutil.ToBytes48(p2))
								assert.Equal(t, true, ok)
								sig := keys[idx].Sign(sigRoot[:])
								sigs = append(sigs, sig)
								msg.Message.Contribution.AggregationBits.SetBitAt(uint64(i), true)
							}
							msg.Message.Contribution.Signature = bls.AggregateSignatures(sigs).Marshal()
							d, err = helpers.Domain(hState.Fork(), helpers.SlotToEpoch(helpers.PrevSlot(hState.Slot())), params.BeaconConfig().DomainContributionAndProof, hState.GenesisValidatorRoot())
							assert.NoError(t, err)
							sigRoot, err = helpers.ComputeSigningRoot(msg.Message, d)
							assert.NoError(t, err)
							contrSig := keys[idx].Sign(sigRoot[:])
							msg.Signature = contrSig.Marshal()
							break
						}
					}
				}
				s.cfg.Chain = &mockChain.ChainService{
					ValidatorsRoot: [32]byte{'A'},
					Genesis:        time.Now().Add(-time.Second * time.Duration(params.BeaconConfig().SecondsPerSlot) * time.Duration(hState.Slot()-1)),
				}

				assert.NoError(t, s.initCaches())
				return s
			},
			args: args{
				ctx:   context.Background(),
				pid:   "random",
				topic: defaultTopic,
				msg: &prysmv2.SignedContributionAndProof{
					Message: &prysmv2.ContributionAndProof{
						AggregatorIndex: 1,
						Contribution: &prysmv2.SyncCommitteeContribution{
							Slot:              1,
							SubcommitteeIndex: 1,
							BlockRoot:         params.BeaconConfig().ZeroHash[:],
							AggregationBits:   bitfield.NewBitvector128(),
							Signature:         emptySig[:],
						},
						SelectionProof: emptySig[:],
					},
					Signature: emptySig[:],
				}},
			want: pubsub.ValidationAccept,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.svc = tt.setupSvc(tt.svc, tt.args.msg)
			marshalledObj, err := tt.args.msg.MarshalSSZ()
			assert.NoError(t, err)
			marshalledObj = snappy.Encode(nil, marshalledObj)
			msg := &pubsub.Message{
				Message: &pubsub_pb.Message{
					Data:  marshalledObj,
					Topic: &tt.args.topic,
				},
				ReceivedFrom:  "",
				ValidatorData: nil,
			}
			if got := tt.svc.validateSyncContributionAndProof(tt.args.ctx, tt.args.pid, msg); got != tt.want {
				t.Errorf("validateSyncContributionAndProof() = %v, want %v", got, tt.want)
			}
		})
	}
}

func fillUpBlocksAndState(ctx context.Context, t *testing.T, beaconDB db.Database) ([32]byte, []bls.SecretKey) {
	gs, keys := testutil.DeterministicGenesisStateAltair(t, 64)
	sCom, err := altair.NextSyncCommittee(gs)
	assert.NoError(t, err)
	assert.NoError(t, gs.SetCurrentSyncCommittee(sCom))
	assert.NoError(t, beaconDB.SaveGenesisData(context.Background(), gs))

	testState := gs.Copy()
	hRoot := [32]byte{}
	for i := types.Slot(1); i <= params.BeaconConfig().SlotsPerEpoch; i++ {
		blk, err := testutil.GenerateFullBlockAltair(testState, keys, testutil.DefaultBlockGenConfig(), i)
		require.NoError(t, err)
		r, err := blk.Block.HashTreeRoot()
		require.NoError(t, err)
		_, testState, err = state.ExecuteStateTransitionNoVerifyAnySig(ctx, testState, wrapper.WrappedAltairSignedBeaconBlock(blk))
		assert.NoError(t, err)
		assert.NoError(t, beaconDB.SaveBlock(ctx, wrapper.WrappedAltairSignedBeaconBlock(blk)))
		assert.NoError(t, beaconDB.SaveStateSummary(ctx, &p2ppb.StateSummary{Slot: i, Root: r[:]}))
		assert.NoError(t, beaconDB.SaveState(ctx, testState, r))
		require.NoError(t, beaconDB.SaveHeadBlockRoot(ctx, r))
		hRoot = r
	}
	return hRoot, keys
}
