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
	mockChain "github.com/prysmaticlabs/prysm/beacon-chain/blockchain/testing"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	testingDB "github.com/prysmaticlabs/prysm/beacon-chain/db/testing"
	"github.com/prysmaticlabs/prysm/beacon-chain/p2p"
	"github.com/prysmaticlabs/prysm/beacon-chain/p2p/encoder"
	mockp2p "github.com/prysmaticlabs/prysm/beacon-chain/p2p/testing"
	p2ptypes "github.com/prysmaticlabs/prysm/beacon-chain/p2p/types"
	"github.com/prysmaticlabs/prysm/beacon-chain/state/stategen"
	mockSync "github.com/prysmaticlabs/prysm/beacon-chain/sync/initial-sync/testing"
	prysmv2 "github.com/prysmaticlabs/prysm/proto/prysm/v2"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/testutil/assert"
)

func TestService_ValidateSyncCommitteeMessage(t *testing.T) {
	db := testingDB.SetupDB(t)
	headRoot, keys := fillUpBlocksAndState(context.Background(), t, db)
	defaultTopic := p2p.SyncCommitteeSubnetTopicFormat
	fakeDigest := []byte{0xAB, 0x00, 0xCC, 0x9E}
	defaultTopic = defaultTopic + "/" + encoder.ProtocolSuffixSSZSnappy
	chainService := &mockChain.ChainService{
		Genesis:        time.Now(),
		ValidatorsRoot: [32]byte{'A'},
	}
	emptySig := [96]byte{}
	type args struct {
		ctx   context.Context
		pid   peer.ID
		msg   *prysmv2.SyncCommitteeMessage
		topic string
	}
	tests := []struct {
		name     string
		svc      *Service
		setupSvc func(s *Service, msg *prysmv2.SyncCommitteeMessage, topic string) (*Service, string)
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
			setupSvc: func(s *Service, msg *prysmv2.SyncCommitteeMessage, topic string) (*Service, string) {
				s.cfg.StateGen = stategen.New(db)
				msg.BlockRoot = headRoot[:]
				s.cfg.DB = db
				assert.NoError(t, s.initCaches())
				return s, topic
			},
			args: args{
				ctx:   context.Background(),
				pid:   "random",
				topic: "junk",
				msg: &prysmv2.SyncCommitteeMessage{
					Slot:           1,
					ValidatorIndex: 1,
					BlockRoot:      params.BeaconConfig().ZeroHash[:],
					Signature:      emptySig[:],
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
			setupSvc: func(s *Service, msg *prysmv2.SyncCommitteeMessage, topic string) (*Service, string) {
				s.cfg.StateGen = stategen.New(db)
				msg.BlockRoot = headRoot[:]
				s.cfg.DB = db
				assert.NoError(t, s.initCaches())
				return s, topic
			},
			args: args{
				ctx:   context.Background(),
				pid:   "random",
				topic: "junk",
				msg: &prysmv2.SyncCommitteeMessage{
					Slot:           1,
					ValidatorIndex: 1,
					BlockRoot:      params.BeaconConfig().ZeroHash[:],
					Signature:      emptySig[:],
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
			setupSvc: func(s *Service, msg *prysmv2.SyncCommitteeMessage, topic string) (*Service, string) {
				s.cfg.StateGen = stategen.New(db)
				s.cfg.DB = db
				assert.NoError(t, s.initCaches())
				return s, topic
			},
			args: args{
				ctx:   context.Background(),
				pid:   "random",
				topic: fmt.Sprintf(defaultTopic, fakeDigest, 0),
				msg: &prysmv2.SyncCommitteeMessage{
					Slot:           10,
					ValidatorIndex: 1,
					BlockRoot:      params.BeaconConfig().ZeroHash[:],
					Signature:      emptySig[:],
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
			setupSvc: func(s *Service, msg *prysmv2.SyncCommitteeMessage, topic string) (*Service, string) {
				s.cfg.StateGen = stategen.New(db)
				s.cfg.DB = db
				assert.NoError(t, s.initCaches())

				s.setSeenSyncMessageIndexSlot(1, 1, 0)
				return s, topic
			},
			args: args{
				ctx:   context.Background(),
				pid:   "random",
				topic: fmt.Sprintf(defaultTopic, fakeDigest, 0),
				msg: &prysmv2.SyncCommitteeMessage{
					Slot:           1,
					ValidatorIndex: 1,
					BlockRoot:      params.BeaconConfig().ZeroHash[:],
					Signature:      emptySig[:],
				}},
			want: pubsub.ValidationIgnore,
		},
		{
			name: "Non-existent block root",
			svc: NewService(context.Background(), &Config{
				P2P:               mockp2p.NewTestP2P(t),
				InitialSync:       &mockSync.Sync{IsSyncing: false},
				Chain:             chainService,
				StateNotifier:     chainService.StateNotifier(),
				OperationNotifier: chainService.OperationNotifier(),
			}),
			setupSvc: func(s *Service, msg *prysmv2.SyncCommitteeMessage, topic string) (*Service, string) {
				s.cfg.StateGen = stategen.New(db)
				s.cfg.DB = db
				assert.NoError(t, s.initCaches())
				s.cfg.Chain = &mockChain.ChainService{
					ValidatorsRoot: [32]byte{'A'},
					Genesis:        time.Now().Add(-time.Second * time.Duration(params.BeaconConfig().SecondsPerSlot) * time.Duration(10)),
				}
				incorrectRoot := [32]byte{0xBB}
				msg.BlockRoot = incorrectRoot[:]

				return s, topic
			},
			args: args{
				ctx:   context.Background(),
				pid:   "random",
				topic: fmt.Sprintf(defaultTopic, fakeDigest, 0),
				msg: &prysmv2.SyncCommitteeMessage{
					Slot:           1,
					ValidatorIndex: 1,
					BlockRoot:      params.BeaconConfig().ZeroHash[:],
					Signature:      emptySig[:],
				}},
			want: pubsub.ValidationIgnore,
		},
		{
			name: "Subnet is non-existent",
			svc: NewService(context.Background(), &Config{
				P2P:               mockp2p.NewTestP2P(t),
				InitialSync:       &mockSync.Sync{IsSyncing: false},
				Chain:             chainService,
				StateNotifier:     chainService.StateNotifier(),
				OperationNotifier: chainService.OperationNotifier(),
			}),
			setupSvc: func(s *Service, msg *prysmv2.SyncCommitteeMessage, topic string) (*Service, string) {
				s.cfg.StateGen = stategen.New(db)
				s.cfg.DB = db
				assert.NoError(t, s.initCaches())
				msg.BlockRoot = headRoot[:]
				hState, err := db.State(context.Background(), headRoot)
				assert.NoError(t, err)
				s.cfg.Chain = &mockChain.ChainService{
					CurrentSyncCommitteeIndices: []types.CommitteeIndex{0},
					ValidatorsRoot:              [32]byte{'A'},
					Genesis:                     time.Now().Add(-time.Second * time.Duration(params.BeaconConfig().SecondsPerSlot) * time.Duration(hState.Slot()-1)),
				}
				numOfVals := hState.NumValidators()

				chosenVal := numOfVals - 10
				msg.Signature = emptySig[:]
				msg.BlockRoot = headRoot[:]
				msg.ValidatorIndex = types.ValidatorIndex(chosenVal)
				msg.Slot = helpers.PrevSlot(hState.Slot())

				// Set Bad Topic and Subnet
				digest, err := s.currentForkDigest()
				assert.NoError(t, err)
				actualTopic := fmt.Sprintf(defaultTopic, digest, 5)

				return s, actualTopic
			},
			args: args{
				ctx:   context.Background(),
				pid:   "random",
				topic: defaultTopic,
				msg: &prysmv2.SyncCommitteeMessage{
					Slot:           1,
					ValidatorIndex: 1,
					BlockRoot:      params.BeaconConfig().ZeroHash[:],
					Signature:      emptySig[:],
				}},
			want: pubsub.ValidationReject,
		},
		{
			name: "Validator is non-existent",
			svc: NewService(context.Background(), &Config{
				P2P:               mockp2p.NewTestP2P(t),
				InitialSync:       &mockSync.Sync{IsSyncing: false},
				Chain:             chainService,
				StateNotifier:     chainService.StateNotifier(),
				OperationNotifier: chainService.OperationNotifier(),
			}),
			setupSvc: func(s *Service, msg *prysmv2.SyncCommitteeMessage, topic string) (*Service, string) {
				s.cfg.StateGen = stategen.New(db)
				s.cfg.DB = db
				assert.NoError(t, s.initCaches())
				msg.BlockRoot = headRoot[:]
				hState, err := db.State(context.Background(), headRoot)
				assert.NoError(t, err)
				s.cfg.Chain = &mockChain.ChainService{
					ValidatorsRoot: [32]byte{'A'},
					Genesis:        time.Now().Add(-time.Second * time.Duration(params.BeaconConfig().SecondsPerSlot) * time.Duration(hState.Slot()-1)),
				}

				numOfVals := hState.NumValidators()

				chosenVal := numOfVals + 10
				msg.Signature = emptySig[:]
				msg.BlockRoot = headRoot[:]
				msg.ValidatorIndex = types.ValidatorIndex(chosenVal)
				msg.Slot = helpers.PrevSlot(hState.Slot())

				return s, topic
			},
			args: args{
				ctx:   context.Background(),
				pid:   "random",
				topic: defaultTopic,
				msg: &prysmv2.SyncCommitteeMessage{
					Slot:           1,
					ValidatorIndex: 1,
					BlockRoot:      params.BeaconConfig().ZeroHash[:],
					Signature:      emptySig[:],
				}},
			want: pubsub.ValidationReject,
		},
		{
			name: "Invalid Sync Committee Signature",
			svc: NewService(context.Background(), &Config{
				P2P:               mockp2p.NewTestP2P(t),
				InitialSync:       &mockSync.Sync{IsSyncing: false},
				Chain:             chainService,
				StateNotifier:     chainService.StateNotifier(),
				OperationNotifier: chainService.OperationNotifier(),
			}),
			setupSvc: func(s *Service, msg *prysmv2.SyncCommitteeMessage, topic string) (*Service, string) {
				s.cfg.StateGen = stategen.New(db)
				s.cfg.DB = db
				assert.NoError(t, s.initCaches())
				msg.BlockRoot = headRoot[:]
				hState, err := db.State(context.Background(), headRoot)
				assert.NoError(t, err)

				numOfVals := hState.NumValidators()

				chosenVal := numOfVals - 10
				msg.Signature = emptySig[:]
				msg.BlockRoot = headRoot[:]
				msg.ValidatorIndex = types.ValidatorIndex(chosenVal)
				msg.Slot = helpers.PrevSlot(hState.Slot())

				d, err := helpers.Domain(hState.Fork(), helpers.SlotToEpoch(hState.Slot()), params.BeaconConfig().DomainSyncCommittee, hState.GenesisValidatorRoot())
				assert.NoError(t, err)
				subCommitteeSize := params.BeaconConfig().SyncCommitteeSize / params.BeaconConfig().SyncCommitteeSubnetCount
				s.cfg.Chain = &mockChain.ChainService{
					CurrentSyncCommitteeIndices: []types.CommitteeIndex{types.CommitteeIndex(subCommitteeSize)},
					ValidatorsRoot:              [32]byte{'A'},
					Genesis:                     time.Now().Add(-time.Second * time.Duration(params.BeaconConfig().SecondsPerSlot) * time.Duration(hState.Slot()-1)),
					SyncCommitteeDomain:         d,
					PublicKey:                   bytesutil.ToBytes48(keys[chosenVal].PublicKey().Marshal()),
				}

				// Set Topic and Subnet
				digest, err := s.currentForkDigest()
				assert.NoError(t, err)
				actualTopic := fmt.Sprintf(defaultTopic, digest, 1)

				return s, actualTopic
			},
			args: args{
				ctx:   context.Background(),
				pid:   "random",
				topic: defaultTopic,
				msg: &prysmv2.SyncCommitteeMessage{
					Slot:           1,
					ValidatorIndex: 1,
					BlockRoot:      params.BeaconConfig().ZeroHash[:],
					Signature:      emptySig[:],
				}},
			want: pubsub.ValidationReject,
		},
		{
			name: "Valid Sync Committee Signature",
			svc: NewService(context.Background(), &Config{
				P2P:               mockp2p.NewTestP2P(t),
				InitialSync:       &mockSync.Sync{IsSyncing: false},
				Chain:             chainService,
				StateNotifier:     chainService.StateNotifier(),
				OperationNotifier: chainService.OperationNotifier(),
			}),
			setupSvc: func(s *Service, msg *prysmv2.SyncCommitteeMessage, topic string) (*Service, string) {
				s.cfg.StateGen = stategen.New(db)
				s.cfg.DB = db
				assert.NoError(t, s.initCaches())
				msg.BlockRoot = headRoot[:]
				hState, err := db.State(context.Background(), headRoot)
				assert.NoError(t, err)
				subCommitteeSize := params.BeaconConfig().SyncCommitteeSize / params.BeaconConfig().SyncCommitteeSubnetCount

				numOfVals := hState.NumValidators()

				chosenVal := numOfVals - 10
				d, err := helpers.Domain(hState.Fork(), helpers.SlotToEpoch(hState.Slot()), params.BeaconConfig().DomainSyncCommittee, hState.GenesisValidatorRoot())
				assert.NoError(t, err)
				rawBytes := p2ptypes.SSZBytes(headRoot[:])
				sigRoot, err := helpers.ComputeSigningRoot(&rawBytes, d)
				assert.NoError(t, err)

				s.cfg.Chain = &mockChain.ChainService{
					CurrentSyncCommitteeIndices: []types.CommitteeIndex{types.CommitteeIndex(subCommitteeSize)},
					ValidatorsRoot:              [32]byte{'A'},
					Genesis:                     time.Now().Add(-time.Second * time.Duration(params.BeaconConfig().SecondsPerSlot) * time.Duration(hState.Slot()-1)),
					SyncCommitteeDomain:         d,
					PublicKey:                   bytesutil.ToBytes48(keys[chosenVal].PublicKey().Marshal()),
				}

				msg.Signature = keys[chosenVal].Sign(sigRoot[:]).Marshal()
				msg.BlockRoot = headRoot[:]
				msg.ValidatorIndex = types.ValidatorIndex(chosenVal)
				msg.Slot = helpers.PrevSlot(hState.Slot())

				// Set Topic and Subnet
				digest, err := s.currentForkDigest()
				assert.NoError(t, err)
				actualTopic := fmt.Sprintf(defaultTopic, digest, 1)

				return s, actualTopic
			},
			args: args{
				ctx:   context.Background(),
				pid:   "random",
				topic: defaultTopic,
				msg: &prysmv2.SyncCommitteeMessage{
					Slot:           1,
					ValidatorIndex: 1,
					BlockRoot:      params.BeaconConfig().ZeroHash[:],
					Signature:      emptySig[:],
				}},
			want: pubsub.ValidationAccept,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "Bad Topic" {
				t.Skip()
			}
			tt.svc, tt.args.topic = tt.setupSvc(tt.svc, tt.args.msg, tt.args.topic)
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
			if got := tt.svc.validateSyncCommitteeMessage(tt.args.ctx, tt.args.pid, msg); got != tt.want {
				t.Errorf("validateSyncCommitteeMessage() = %v, want %v", got, tt.want)
			}
		})
	}
}
