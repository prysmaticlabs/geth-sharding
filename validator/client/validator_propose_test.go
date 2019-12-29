package client

import (
	"context"
	"errors"
	"testing"

	"github.com/golang/mock/gomock"
	ethpb "github.com/prysmaticlabs/ethereumapis/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/shared/testutil"
	"github.com/prysmaticlabs/prysm/validator/internal"
	logTest "github.com/sirupsen/logrus/hooks/test"
)

type mocks struct {
	validatorClient  *internal.MockBeaconNodeValidatorClient
	aggregatorClient *internal.MockAggregatorServiceClient
}

func setup(t *testing.T) (*validator, *mocks, func()) {
	ctrl := gomock.NewController(t)
	m := &mocks{
		validatorClient:  internal.NewMockBeaconNodeValidatorClient(ctrl),
		aggregatorClient: internal.NewMockAggregatorServiceClient(ctrl),
	}
	validator := &validator{
		validatorClient:  m.validatorClient,
		aggregatorClient: m.aggregatorClient,
		keys:             keyMap,
		graffiti:         []byte{},
	}

	return validator, m, ctrl.Finish
}

func TestProposeBlock_DoesNotProposeGenesisBlock(t *testing.T) {
	hook := logTest.NewGlobal()
	validator, _, finish := setup(t)
	defer finish()
	validator.ProposeBlock(context.Background(), 0, validatorPubKey)

	testutil.AssertLogsContain(t, hook, "Assigned to genesis slot, skipping proposal")
}

func TestProposeBlock_DomainDataFailed(t *testing.T) {
	hook := logTest.NewGlobal()
	validator, m, finish := setup(t)
	defer finish()

	m.validatorClient.EXPECT().DomainData(
		gomock.Any(), // ctx
		gomock.Any(), // epoch
	).Return(nil /*response*/, errors.New("uh oh"))

	validator.ProposeBlock(context.Background(), 1, validatorPubKey)
	testutil.AssertLogsContain(t, hook, "Failed to sign randao reveal")
}

func TestProposeBlock_RequestBlockFailed(t *testing.T) {
	hook := logTest.NewGlobal()
	validator, m, finish := setup(t)
	defer finish()

	m.validatorClient.EXPECT().DomainData(
		gomock.Any(), // ctx
		gomock.Any(), // epoch
	).Return(&ethpb.DomainResponse{}, nil /*err*/)

	m.validatorClient.EXPECT().GetBlock(
		gomock.Any(), // ctx
		gomock.Any(), // block request
	).Return(nil /*response*/, errors.New("uh oh"))

	validator.ProposeBlock(context.Background(), 1, validatorPubKey)
	testutil.AssertLogsContain(t, hook, "Failed to request block from beacon node")
}

func TestProposeBlock_ProposeBlockFailed(t *testing.T) {
	hook := logTest.NewGlobal()
	validator, m, finish := setup(t)
	defer finish()

	m.validatorClient.EXPECT().DomainData(
		gomock.Any(), // ctx
		gomock.Any(), //epoch
	).Return(&ethpb.DomainResponse{}, nil /*err*/)

	m.validatorClient.EXPECT().GetBlock(
		gomock.Any(), // ctx
		gomock.Any(),
	).Return(&ethpb.BeaconBlock{Body: &ethpb.BeaconBlockBody{}}, nil /*err*/)

	m.validatorClient.EXPECT().DomainData(
		gomock.Any(), // ctx
		gomock.Any(), //epoch
	).Return(&ethpb.DomainResponse{}, nil /*err*/)

	m.validatorClient.EXPECT().ProposeBlock(
		gomock.Any(), // ctx
		gomock.AssignableToTypeOf(&ethpb.BeaconBlock{}),
	).Return(nil /*response*/, errors.New("uh oh"))

	validator.ProposeBlock(context.Background(), 1, validatorPubKey)
	testutil.AssertLogsContain(t, hook, "Failed to propose block")
}

func TestProposeBlock_BroadcastsBlock(t *testing.T) {
	validator, m, finish := setup(t)
	defer finish()

	m.validatorClient.EXPECT().DomainData(
		gomock.Any(), // ctx
		gomock.Any(), //epoch
	).Return(&ethpb.DomainResponse{}, nil /*err*/)

	m.validatorClient.EXPECT().GetBlock(
		gomock.Any(), // ctx
		gomock.Any(),
	).Return(&ethpb.BeaconBlock{Body: &ethpb.BeaconBlockBody{}}, nil /*err*/)

	m.validatorClient.EXPECT().DomainData(
		gomock.Any(), // ctx
		gomock.Any(), //epoch
	).Return(&ethpb.DomainResponse{}, nil /*err*/)

	m.validatorClient.EXPECT().ProposeBlock(
		gomock.Any(), // ctx
		gomock.AssignableToTypeOf(&ethpb.BeaconBlock{}),
	).Return(&ethpb.ProposeResponse{}, nil /*error*/)

	validator.ProposeBlock(context.Background(), 1, validatorPubKey)
}

func TestProposeBlock_BroadcastsBlock_WithGraffiti(t *testing.T) {
	validator, m, finish := setup(t)
	defer finish()

	validator.graffiti = []byte("12345678901234567890123456789012")

	m.validatorClient.EXPECT().DomainData(
		gomock.Any(), // ctx
		gomock.Any(), //epoch
	).Return(&ethpb.DomainResponse{}, nil /*err*/)

	m.validatorClient.EXPECT().GetBlock(
		gomock.Any(), // ctx
		gomock.Any(),
	).Return(&ethpb.BeaconBlock{Body: &ethpb.BeaconBlockBody{Graffiti: validator.graffiti}}, nil /*err*/)

	m.validatorClient.EXPECT().DomainData(
		gomock.Any(), // ctx
		gomock.Any(), //epoch
	).Return(&ethpb.DomainResponse{}, nil /*err*/)

	var sentBlock *ethpb.BeaconBlock

	m.validatorClient.EXPECT().ProposeBlock(
		gomock.Any(), // ctx
		gomock.AssignableToTypeOf(&ethpb.BeaconBlock{}),
	).DoAndReturn(func(ctx context.Context, block *ethpb.BeaconBlock) (*ethpb.ProposeResponse, error) {
		sentBlock = block
		return &ethpb.ProposeResponse{}, nil
	})

	validator.ProposeBlock(context.Background(), 1, validatorPubKey)

	if string(sentBlock.Body.Graffiti) != string(validator.graffiti) {
		t.Errorf("Block was broadcast with the wrong graffiti field, wanted \"%v\", got \"%v\"", string(validator.graffiti), string(sentBlock.Body.Graffiti))
	}
}
