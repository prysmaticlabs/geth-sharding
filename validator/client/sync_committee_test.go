package client

import (
	"context"
	"encoding/hex"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/pkg/errors"
	types "github.com/prysmaticlabs/eth2-types"
	"github.com/prysmaticlabs/go-bitfield"
	eth "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	prysmv2 "github.com/prysmaticlabs/prysm/proto/prysm/v2"
	"github.com/prysmaticlabs/prysm/shared/bls"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/prysmaticlabs/prysm/shared/testutil/assert"
	"github.com/prysmaticlabs/prysm/shared/testutil/require"
	logTest "github.com/sirupsen/logrus/hooks/test"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
)

func TestSubmitSyncCommitteeMessage_ValidatorDutiesRequestFailure(t *testing.T) {
	hook := logTest.NewGlobal()
	validator, m, validatorKey, finish := setup(t)
	validator.duties = &eth.DutiesResponse{Duties: []*eth.DutiesResponse_Duty{}}
	defer finish()

	m.validatorClientV2.EXPECT().GetSyncMessageBlockRoot(
		gomock.Any(), // ctx
		&emptypb.Empty{},
	).Return(&prysmv2.SyncMessageBlockRootResponse{
		Root: bytesutil.PadTo([]byte{}, 32),
	}, nil)

	pubKey := [48]byte{}
	copy(pubKey[:], validatorKey.PublicKey().Marshal())
	validator.SubmitSyncCommitteeMessage(context.Background(), 1, pubKey)
	require.LogsContain(t, hook, "Could not fetch validator assignment")
}

func TestSubmitSyncCommitteeMessage_BadDomainData(t *testing.T) {
	validator, m, validatorKey, finish := setup(t)
	defer finish()
	hook := logTest.NewGlobal()
	validatorIndex := types.ValidatorIndex(7)
	committee := []types.ValidatorIndex{0, 3, 4, 2, validatorIndex, 6, 8, 9, 10}
	validator.duties = &eth.DutiesResponse{Duties: []*eth.DutiesResponse_Duty{
		{
			PublicKey:      validatorKey.PublicKey().Marshal(),
			Committee:      committee,
			ValidatorIndex: validatorIndex,
		},
	}}

	r := []byte{'a'}
	m.validatorClientV2.EXPECT().GetSyncMessageBlockRoot(
		gomock.Any(), // ctx
		&emptypb.Empty{},
	).Return(&prysmv2.SyncMessageBlockRootResponse{
		Root: bytesutil.PadTo(r, 32),
	}, nil)

	m.validatorClient.EXPECT().
		DomainData(gomock.Any(), gomock.Any()).
		Return(nil, errors.New("uh oh"))

	pubKey := [48]byte{}
	copy(pubKey[:], validatorKey.PublicKey().Marshal())
	validator.SubmitSyncCommitteeMessage(context.Background(), 1, pubKey)
	require.LogsContain(t, hook, "Could not get sync committee domain data")
}

func TestSubmitSyncCommitteeMessage_CouldNotSubmit(t *testing.T) {
	validator, m, validatorKey, finish := setup(t)
	defer finish()
	hook := logTest.NewGlobal()
	validatorIndex := types.ValidatorIndex(7)
	committee := []types.ValidatorIndex{0, 3, 4, 2, validatorIndex, 6, 8, 9, 10}
	validator.duties = &eth.DutiesResponse{Duties: []*eth.DutiesResponse_Duty{
		{
			PublicKey:      validatorKey.PublicKey().Marshal(),
			Committee:      committee,
			ValidatorIndex: validatorIndex,
		},
	}}

	r := []byte{'a'}
	m.validatorClientV2.EXPECT().GetSyncMessageBlockRoot(
		gomock.Any(), // ctx
		&emptypb.Empty{},
	).Return(&prysmv2.SyncMessageBlockRootResponse{
		Root: bytesutil.PadTo(r, 32),
	}, nil)

	m.validatorClient.EXPECT().
		DomainData(gomock.Any(), // ctx
			gomock.Any()). // epoch
		Return(&eth.DomainResponse{
			SignatureDomain: make([]byte, 32),
		}, nil)

	m.validatorClientV2.EXPECT().SubmitSyncMessage(
		gomock.Any(), // ctx
		gomock.AssignableToTypeOf(&prysmv2.SyncCommitteeMessage{}),
	).Return(&emptypb.Empty{}, errors.New("uh oh") /* error */)

	pubKey := [48]byte{}
	copy(pubKey[:], validatorKey.PublicKey().Marshal())
	validator.SubmitSyncCommitteeMessage(context.Background(), 1, pubKey)

	require.LogsContain(t, hook, "Could not submit sync committee message")
}

func TestSubmitSyncCommitteeMessage_OK(t *testing.T) {
	validator, m, validatorKey, finish := setup(t)
	defer finish()
	hook := logTest.NewGlobal()
	validatorIndex := types.ValidatorIndex(7)
	committee := []types.ValidatorIndex{0, 3, 4, 2, validatorIndex, 6, 8, 9, 10}
	validator.duties = &eth.DutiesResponse{Duties: []*eth.DutiesResponse_Duty{
		{
			PublicKey:      validatorKey.PublicKey().Marshal(),
			Committee:      committee,
			ValidatorIndex: validatorIndex,
		},
	}}

	r := []byte{'a'}
	m.validatorClientV2.EXPECT().GetSyncMessageBlockRoot(
		gomock.Any(), // ctx
		&emptypb.Empty{},
	).Return(&prysmv2.SyncMessageBlockRootResponse{
		Root: bytesutil.PadTo(r, 32),
	}, nil)

	m.validatorClient.EXPECT().
		DomainData(gomock.Any(), // ctx
			gomock.Any()). // epoch
		Return(&eth.DomainResponse{
			SignatureDomain: make([]byte, 32),
		}, nil)

	var generatedMsg *prysmv2.SyncCommitteeMessage
	m.validatorClientV2.EXPECT().SubmitSyncMessage(
		gomock.Any(), // ctx
		gomock.AssignableToTypeOf(&prysmv2.SyncCommitteeMessage{}),
	).Do(func(_ context.Context, msg *prysmv2.SyncCommitteeMessage, opts ...grpc.CallOption) {
		generatedMsg = msg
	}).Return(&emptypb.Empty{}, nil /* error */)

	pubKey := [48]byte{}
	copy(pubKey[:], validatorKey.PublicKey().Marshal())
	validator.SubmitSyncCommitteeMessage(context.Background(), 1, pubKey)

	require.LogsDoNotContain(t, hook, "Could not")
	require.Equal(t, types.Slot(1), generatedMsg.Slot)
	require.Equal(t, validatorIndex, generatedMsg.ValidatorIndex)
	require.DeepEqual(t, bytesutil.PadTo(r, 32), generatedMsg.BlockRoot)
}

func TestSubmitSignedContributionAndProof_ValidatorDutiesRequestFailure(t *testing.T) {
	hook := logTest.NewGlobal()
	validator, _, validatorKey, finish := setup(t)
	validator.duties = &eth.DutiesResponse{Duties: []*eth.DutiesResponse_Duty{}}
	defer finish()

	pubKey := [48]byte{}
	copy(pubKey[:], validatorKey.PublicKey().Marshal())
	validator.SubmitSignedContributionAndProof(context.Background(), 1, pubKey)
	require.LogsContain(t, hook, "Could not fetch validator assignment")
}

func TestSubmitSignedContributionAndProof_GetSyncSubcommitteeIndexFailure(t *testing.T) {
	hook := logTest.NewGlobal()
	validator, m, validatorKey, finish := setup(t)
	validatorIndex := types.ValidatorIndex(7)
	committee := []types.ValidatorIndex{0, 3, 4, 2, validatorIndex, 6, 8, 9, 10}
	validator.duties = &eth.DutiesResponse{Duties: []*eth.DutiesResponse_Duty{
		{
			PublicKey:      validatorKey.PublicKey().Marshal(),
			Committee:      committee,
			ValidatorIndex: validatorIndex,
		},
	}}
	defer finish()

	pubKey := [48]byte{}
	copy(pubKey[:], validatorKey.PublicKey().Marshal())
	m.validatorClientV2.EXPECT().GetSyncSubcommitteeIndex(
		gomock.Any(), // ctx
		&prysmv2.SyncSubcommitteeIndexRequest{
			Slot:      1,
			PublicKey: pubKey[:],
		},
	).Return(&prysmv2.SyncSubcommitteeIndexResponse{}, errors.New("Bad index"))

	validator.SubmitSignedContributionAndProof(context.Background(), 1, pubKey)
	require.LogsContain(t, hook, "Could not get sync subcommittee index")
}

func TestSubmitSignedContributionAndProof_NothingToDo(t *testing.T) {
	hook := logTest.NewGlobal()
	validator, m, validatorKey, finish := setup(t)
	validatorIndex := types.ValidatorIndex(7)
	committee := []types.ValidatorIndex{0, 3, 4, 2, validatorIndex, 6, 8, 9, 10}
	validator.duties = &eth.DutiesResponse{Duties: []*eth.DutiesResponse_Duty{
		{
			PublicKey:      validatorKey.PublicKey().Marshal(),
			Committee:      committee,
			ValidatorIndex: validatorIndex,
		},
	}}
	defer finish()

	pubKey := [48]byte{}
	copy(pubKey[:], validatorKey.PublicKey().Marshal())
	m.validatorClientV2.EXPECT().GetSyncSubcommitteeIndex(
		gomock.Any(), // ctx
		&prysmv2.SyncSubcommitteeIndexRequest{
			Slot:      1,
			PublicKey: pubKey[:],
		},
	).Return(&prysmv2.SyncSubcommitteeIndexResponse{Indices: []types.CommitteeIndex{}}, nil)

	validator.SubmitSignedContributionAndProof(context.Background(), 1, pubKey)
	require.LogsContain(t, hook, "Empty subcommittee index list, do nothing")
}

func TestSubmitSignedContributionAndProof_BadDomain(t *testing.T) {
	hook := logTest.NewGlobal()
	validator, m, validatorKey, finish := setup(t)
	validatorIndex := types.ValidatorIndex(7)
	committee := []types.ValidatorIndex{0, 3, 4, 2, validatorIndex, 6, 8, 9, 10}
	validator.duties = &eth.DutiesResponse{Duties: []*eth.DutiesResponse_Duty{
		{
			PublicKey:      validatorKey.PublicKey().Marshal(),
			Committee:      committee,
			ValidatorIndex: validatorIndex,
		},
	}}
	defer finish()

	pubKey := [48]byte{}
	copy(pubKey[:], validatorKey.PublicKey().Marshal())
	m.validatorClientV2.EXPECT().GetSyncSubcommitteeIndex(
		gomock.Any(), // ctx
		&prysmv2.SyncSubcommitteeIndexRequest{
			Slot:      1,
			PublicKey: pubKey[:],
		},
	).Return(&prysmv2.SyncSubcommitteeIndexResponse{Indices: []types.CommitteeIndex{1}}, nil)

	m.validatorClient.EXPECT().
		DomainData(gomock.Any(), // ctx
			gomock.Any()). // epoch
		Return(&eth.DomainResponse{
			SignatureDomain: make([]byte, 32),
		}, errors.New("bad domain response"))

	validator.SubmitSignedContributionAndProof(context.Background(), 1, pubKey)
	require.LogsContain(t, hook, "Could not sign selection data: bad domain response")
}

func TestSubmitSignedContributionAndProof_CouldNotGetContribution(t *testing.T) {
	hook := logTest.NewGlobal()
	// Hardcode secret key in order to have a valid aggregator signature.
	rawKey, err := hex.DecodeString("659e875e1b062c03f2f2a57332974d475b97df6cfc581d322e79642d39aca8fd")
	assert.NoError(t, err)
	validatorKey, err := bls.SecretKeyFromBytes(rawKey)
	assert.NoError(t, err)

	validator, m, validatorKey, finish := setupWithKey(t, validatorKey)
	validatorIndex := types.ValidatorIndex(7)
	committee := []types.ValidatorIndex{0, 3, 4, 2, validatorIndex, 6, 8, 9, 10}
	validator.duties = &eth.DutiesResponse{Duties: []*eth.DutiesResponse_Duty{
		{
			PublicKey:      validatorKey.PublicKey().Marshal(),
			Committee:      committee,
			ValidatorIndex: validatorIndex,
		},
	}}
	defer finish()

	pubKey := [48]byte{}
	copy(pubKey[:], validatorKey.PublicKey().Marshal())
	m.validatorClientV2.EXPECT().GetSyncSubcommitteeIndex(
		gomock.Any(), // ctx
		&prysmv2.SyncSubcommitteeIndexRequest{
			Slot:      1,
			PublicKey: pubKey[:],
		},
	).Return(&prysmv2.SyncSubcommitteeIndexResponse{Indices: []types.CommitteeIndex{1}}, nil)

	m.validatorClient.EXPECT().
		DomainData(gomock.Any(), // ctx
			gomock.Any()). // epoch
		Return(&eth.DomainResponse{
			SignatureDomain: make([]byte, 32),
		}, nil)

	m.validatorClientV2.EXPECT().GetSyncCommitteeContribution(
		gomock.Any(), // ctx
		&prysmv2.SyncCommitteeContributionRequest{
			Slot:      1,
			PublicKey: pubKey[:],
			SubnetId:  0,
		},
	).Return(nil, errors.New("Bad contribution"))

	validator.SubmitSignedContributionAndProof(context.Background(), 1, pubKey)
	require.LogsContain(t, hook, "Could not get sync committee contribution")
}

func TestSubmitSignedContributionAndProof_CouldNotSubmitContribution(t *testing.T) {
	hook := logTest.NewGlobal()
	// Hardcode secret key in order to have a valid aggregator signature.
	rawKey, err := hex.DecodeString("659e875e1b062c03f2f2a57332974d475b97df6cfc581d322e79642d39aca8fd")
	assert.NoError(t, err)
	validatorKey, err := bls.SecretKeyFromBytes(rawKey)
	assert.NoError(t, err)

	validator, m, validatorKey, finish := setupWithKey(t, validatorKey)
	validatorIndex := types.ValidatorIndex(7)
	committee := []types.ValidatorIndex{0, 3, 4, 2, validatorIndex, 6, 8, 9, 10}
	validator.duties = &eth.DutiesResponse{Duties: []*eth.DutiesResponse_Duty{
		{
			PublicKey:      validatorKey.PublicKey().Marshal(),
			Committee:      committee,
			ValidatorIndex: validatorIndex,
		},
	}}
	defer finish()

	pubKey := [48]byte{}
	copy(pubKey[:], validatorKey.PublicKey().Marshal())
	m.validatorClientV2.EXPECT().GetSyncSubcommitteeIndex(
		gomock.Any(), // ctx
		&prysmv2.SyncSubcommitteeIndexRequest{
			Slot:      1,
			PublicKey: pubKey[:],
		},
	).Return(&prysmv2.SyncSubcommitteeIndexResponse{Indices: []types.CommitteeIndex{1}}, nil)

	m.validatorClient.EXPECT().
		DomainData(gomock.Any(), // ctx
			gomock.Any()). // epoch
		Return(&eth.DomainResponse{
			SignatureDomain: make([]byte, 32),
		}, nil)

	m.validatorClientV2.EXPECT().GetSyncCommitteeContribution(
		gomock.Any(), // ctx
		&prysmv2.SyncCommitteeContributionRequest{
			Slot:      1,
			PublicKey: pubKey[:],
			SubnetId:  0,
		},
	).Return(&prysmv2.SyncCommitteeContribution{
		BlockRoot:       make([]byte, 32),
		Signature:       make([]byte, 96),
		AggregationBits: bitfield.NewBitvector128(),
	}, nil)

	m.validatorClient.EXPECT().
		DomainData(gomock.Any(), // ctx
			gomock.Any()). // epoch
		Return(&eth.DomainResponse{
			SignatureDomain: make([]byte, 32),
		}, nil)

	m.validatorClientV2.EXPECT().SubmitSignedContributionAndProof(
		gomock.Any(), // ctx
		gomock.AssignableToTypeOf(&prysmv2.SignedContributionAndProof{
			Message: &prysmv2.ContributionAndProof{
				AggregatorIndex: 7,
				Contribution: &prysmv2.SyncCommitteeContribution{
					BlockRoot:         make([]byte, 32),
					Signature:         make([]byte, 96),
					AggregationBits:   bitfield.NewBitvector128(),
					Slot:              1,
					SubcommitteeIndex: 1,
				},
			},
		}),
	).Return(&emptypb.Empty{}, errors.New("Could not submit contribution"))

	validator.SubmitSignedContributionAndProof(context.Background(), 1, pubKey)
	require.LogsContain(t, hook, "Could not submit signed contribution and proof")
}

func TestSubmitSignedContributionAndProof_Ok(t *testing.T) {
	// Hardcode secret key in order to have a valid aggregator signature.
	rawKey, err := hex.DecodeString("659e875e1b062c03f2f2a57332974d475b97df6cfc581d322e79642d39aca8fd")
	assert.NoError(t, err)
	validatorKey, err := bls.SecretKeyFromBytes(rawKey)
	assert.NoError(t, err)

	validator, m, validatorKey, finish := setupWithKey(t, validatorKey)
	validatorIndex := types.ValidatorIndex(7)
	committee := []types.ValidatorIndex{0, 3, 4, 2, validatorIndex, 6, 8, 9, 10}
	validator.duties = &eth.DutiesResponse{Duties: []*eth.DutiesResponse_Duty{
		{
			PublicKey:      validatorKey.PublicKey().Marshal(),
			Committee:      committee,
			ValidatorIndex: validatorIndex,
		},
	}}
	defer finish()

	pubKey := [48]byte{}
	copy(pubKey[:], validatorKey.PublicKey().Marshal())
	m.validatorClientV2.EXPECT().GetSyncSubcommitteeIndex(
		gomock.Any(), // ctx
		&prysmv2.SyncSubcommitteeIndexRequest{
			Slot:      1,
			PublicKey: pubKey[:],
		},
	).Return(&prysmv2.SyncSubcommitteeIndexResponse{Indices: []types.CommitteeIndex{1}}, nil)

	m.validatorClient.EXPECT().
		DomainData(gomock.Any(), // ctx
			gomock.Any()). // epoch
		Return(&eth.DomainResponse{
			SignatureDomain: make([]byte, 32),
		}, nil)

	m.validatorClientV2.EXPECT().GetSyncCommitteeContribution(
		gomock.Any(), // ctx
		&prysmv2.SyncCommitteeContributionRequest{
			Slot:      1,
			PublicKey: pubKey[:],
			SubnetId:  0,
		},
	).Return(&prysmv2.SyncCommitteeContribution{
		BlockRoot:       make([]byte, 32),
		Signature:       make([]byte, 96),
		AggregationBits: bitfield.NewBitvector128(),
	}, nil)

	m.validatorClient.EXPECT().
		DomainData(gomock.Any(), // ctx
			gomock.Any()). // epoch
		Return(&eth.DomainResponse{
			SignatureDomain: make([]byte, 32),
		}, nil)

	m.validatorClientV2.EXPECT().SubmitSignedContributionAndProof(
		gomock.Any(), // ctx
		gomock.AssignableToTypeOf(&prysmv2.SignedContributionAndProof{
			Message: &prysmv2.ContributionAndProof{
				AggregatorIndex: 7,
				Contribution: &prysmv2.SyncCommitteeContribution{
					BlockRoot:         make([]byte, 32),
					Signature:         make([]byte, 96),
					AggregationBits:   bitfield.NewBitvector128(),
					Slot:              1,
					SubcommitteeIndex: 1,
				},
			},
		}),
	).Return(&emptypb.Empty{}, nil)

	validator.SubmitSignedContributionAndProof(context.Background(), 1, pubKey)
}
