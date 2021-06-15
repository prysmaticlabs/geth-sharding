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
	mockp2p "github.com/prysmaticlabs/prysm/beacon-chain/p2p/testing"
	"github.com/prysmaticlabs/prysm/beacon-chain/state/stategen"
	mockSync "github.com/prysmaticlabs/prysm/beacon-chain/sync/initial-sync/testing"
	p2ppb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	ethpb "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/shared/bls"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/prysmaticlabs/prysm/shared/interfaces"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/testutil"
	"github.com/prysmaticlabs/prysm/shared/testutil/assert"
	"github.com/prysmaticlabs/prysm/shared/testutil/require"
)

func TestService_ValidateSyncContributionAndProof(t *testing.T) {
	chainService := &mockChain.ChainService{
		Genesis:        time.Now(),
		ValidatorsRoot: [32]byte{'A'},
	}
	emptySig := [96]byte{}
	type args struct {
		ctx context.Context
		pid peer.ID
		msg *ethpb.SignedContributionAndProof
	}
	tests := []struct {
		name     string
		svc      *Service
		setupSvc func(s *Service, msg *ethpb.SignedContributionAndProof) *Service
		args     args
		want     pubsub.ValidationResult
	}{
		{
			name: "Valid Signed Sync Contribution And Proof",
			svc: NewService(context.Background(), &Config{
				P2P:               mockp2p.NewTestP2P(t),
				InitialSync:       &mockSync.Sync{IsSyncing: false},
				Chain:             chainService,
				StateNotifier:     chainService.StateNotifier(),
				OperationNotifier: chainService.OperationNotifier(),
			}),
			setupSvc: func(s *Service, msg *ethpb.SignedContributionAndProof) *Service {
				db := testingDB.SetupDB(t)
				s.cfg.StateGen = stategen.New(db)
				headRoot, keys := fillUpBlocksAndState(context.Background(), t, db)
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
						rt, err := altair.SyncCommitteeSigningRoot(hState, helpers.PrevSlot(hState.Slot()), types.CommitteeIndex(i))
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
			args: args{ctx: context.Background(), pid: "random", msg: &ethpb.SignedContributionAndProof{
				Message: &ethpb.ContributionAndProof{
					AggregatorIndex: 1,
					Contribution: &ethpb.SyncCommitteeContribution{
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
			topic := p2p.SyncContributionAndProofSubnetTopicFormat
			topic = fmt.Sprintf(topic, []byte{0xAB, 0x00, 0xCC, 0x9E})
			topic = topic + tt.svc.cfg.P2P.Encoding().ProtocolSuffix()
			msg := &pubsub.Message{
				Message: &pubsub_pb.Message{
					Data:  marshalledObj,
					Topic: &topic,
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
		_, testState, err = state.ExecuteStateTransitionNoVerifyAnySig(ctx, testState, interfaces.WrappedAltairSignedBeaconBlock(blk))
		assert.NoError(t, err)
		assert.NoError(t, beaconDB.SaveBlock(ctx, interfaces.WrappedAltairSignedBeaconBlock(blk)))
		assert.NoError(t, beaconDB.SaveStateSummary(ctx, &p2ppb.StateSummary{Slot: i, Root: r[:]}))
		assert.NoError(t, beaconDB.SaveState(ctx, testState, r))
		require.NoError(t, beaconDB.SaveHeadBlockRoot(ctx, r))
		hRoot = r
	}
	return hRoot, keys
}
