package client

import (
	"context"
	"errors"
	"io/ioutil"
	"strings"
	"testing"
	"time"

	ptypes "github.com/gogo/protobuf/types"
	"github.com/golang/mock/gomock"
	ethpb "github.com/prysmaticlabs/ethereumapis/eth/v1alpha1"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/rpc/v1"
	"github.com/prysmaticlabs/prysm/shared/bls"
	"github.com/prysmaticlabs/prysm/shared/keystore"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/testutil"
	"github.com/prysmaticlabs/prysm/validator/internal"
	"github.com/sirupsen/logrus"
	logTest "github.com/sirupsen/logrus/hooks/test"
)

func init() {
	logrus.SetLevel(logrus.DebugLevel)
	logrus.SetOutput(ioutil.Discard)
}

var _ = Validator(&validator{})

const cancelledCtx = "context has been canceled"

func publicKeys(keys map[[48]byte]*keystore.Key) [][]byte {
	pks := make([][]byte, 0, len(keys))
	for key := range keys {
		pks = append(pks, key[:])
	}
	return pks
}

func generateMockStatusResponse(pubkeys [][]byte) *ethpb.ValidatorActivationResponse {
	multipleStatus := make([]*ethpb.ValidatorActivationResponse_Status, len(pubkeys))
	for i, key := range pubkeys {
		multipleStatus[i] = &ethpb.ValidatorActivationResponse_Status{
			PublicKey: key,
			Status: &ethpb.ValidatorStatusResponse{
				Status: ethpb.ValidatorStatus_UNKNOWN_STATUS,
			},
		}
	}
	return &ethpb.ValidatorActivationResponse{Statuses: multipleStatus}
}

func TestWaitForChainStart_SetsChainStartGenesisTime(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := internal.NewMockBeaconNodeValidatorClient(ctrl)

	v := validator{
		keys:            keyMap,
		validatorClient: client,
	}
	genesis := uint64(time.Unix(0, 0).Unix())
	clientStream := internal.NewMockBeaconNodeValidator_WaitForChainStartClient(ctrl)
	client.EXPECT().WaitForChainStart(
		gomock.Any(),
		&ptypes.Empty{},
	).Return(clientStream, nil)
	clientStream.EXPECT().Recv().Return(
		&ethpb.ChainStartResponse{
			Started:     true,
			GenesisTime: genesis,
		},
		nil,
	)
	if err := v.WaitForChainStart(context.Background()); err != nil {
		t.Fatal(err)
	}
	if v.genesisTime != genesis {
		t.Errorf("Expected chain start time to equal %d, received %d", genesis, v.genesisTime)
	}
	if v.ticker == nil {
		t.Error("Expected ticker to be set, received nil")
	}
}

func TestWaitForChainStart_ContextCanceled(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := internal.NewMockBeaconNodeValidatorClient(ctrl)

	v := validator{
		keys:            keyMap,
		validatorClient: client,
	}
	genesis := uint64(time.Unix(0, 0).Unix())
	clientStream := internal.NewMockBeaconNodeValidator_WaitForChainStartClient(ctrl)
	client.EXPECT().WaitForChainStart(
		gomock.Any(),
		&ptypes.Empty{},
	).Return(clientStream, nil)
	clientStream.EXPECT().Recv().Return(
		&ethpb.ChainStartResponse{
			Started:     true,
			GenesisTime: genesis,
		},
		nil,
	)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := v.WaitForChainStart(ctx)
	want := cancelledCtx
	if !strings.Contains(err.Error(), want) {
		t.Errorf("Expected %v, received %v", want, err)
	}
}

func TestWaitForChainStart_StreamSetupFails(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := internal.NewMockBeaconNodeValidatorClient(ctrl)

	v := validator{
		keys:            keyMap,
		validatorClient: client,
	}
	clientStream := internal.NewMockBeaconNodeValidator_WaitForChainStartClient(ctrl)
	client.EXPECT().WaitForChainStart(
		gomock.Any(),
		&ptypes.Empty{},
	).Return(clientStream, errors.New("failed stream"))
	err := v.WaitForChainStart(context.Background())
	want := "could not setup beacon chain ChainStart streaming client"
	if !strings.Contains(err.Error(), want) {
		t.Errorf("Expected %v, received %v", want, err)
	}
}

func TestWaitForChainStart_ReceiveErrorFromStream(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := internal.NewMockBeaconNodeValidatorClient(ctrl)

	v := validator{
		keys:            keyMap,
		validatorClient: client,
	}
	clientStream := internal.NewMockBeaconNodeValidator_WaitForChainStartClient(ctrl)
	client.EXPECT().WaitForChainStart(
		gomock.Any(),
		&ptypes.Empty{},
	).Return(clientStream, nil)
	clientStream.EXPECT().Recv().Return(
		nil,
		errors.New("fails"),
	)
	err := v.WaitForChainStart(context.Background())
	want := "could not receive ChainStart from stream"
	if !strings.Contains(err.Error(), want) {
		t.Errorf("Expected %v, received %v", want, err)
	}
}

func TestWaitActivation_ContextCanceled(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := internal.NewMockBeaconNodeValidatorClient(ctrl)

	v := validator{
		keys:            keyMap,
		pubkeys:         make([][]byte, 0),
		validatorClient: client,
	}
	v.pubkeys = publicKeys(v.keys)
	clientStream := internal.NewMockBeaconNodeValidator_WaitForActivationClient(ctrl)

	client.EXPECT().WaitForActivation(
		gomock.Any(),
		&ethpb.ValidatorActivationRequest{
			PublicKeys: publicKeys(v.keys),
		},
	).Return(clientStream, nil)
	clientStream.EXPECT().Recv().Return(
		&ethpb.ValidatorActivationResponse{},
		nil,
	)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := v.WaitForActivation(ctx)
	want := cancelledCtx
	if !strings.Contains(err.Error(), want) {
		t.Errorf("Expected %v, received %v", want, err)
	}
}

func TestWaitActivation_StreamSetupFails(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := internal.NewMockBeaconNodeValidatorClient(ctrl)

	v := validator{
		keys:            keyMap,
		pubkeys:         make([][]byte, 0),
		validatorClient: client,
	}
	v.pubkeys = publicKeys(v.keys)
	clientStream := internal.NewMockBeaconNodeValidator_WaitForActivationClient(ctrl)
	client.EXPECT().WaitForActivation(
		gomock.Any(),
		&ethpb.ValidatorActivationRequest{
			PublicKeys: publicKeys(v.keys),
		},
	).Return(clientStream, errors.New("failed stream"))
	err := v.WaitForActivation(context.Background())
	want := "could not setup validator WaitForActivation streaming client"
	if !strings.Contains(err.Error(), want) {
		t.Errorf("Expected %v, received %v", want, err)
	}
}

func TestWaitActivation_ReceiveErrorFromStream(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := internal.NewMockBeaconNodeValidatorClient(ctrl)

	v := validator{
		keys:            keyMap,
		pubkeys:         make([][]byte, 0),
		validatorClient: client,
	}
	v.pubkeys = publicKeys(v.keys)
	clientStream := internal.NewMockBeaconNodeValidator_WaitForActivationClient(ctrl)
	client.EXPECT().WaitForActivation(
		gomock.Any(),
		&ethpb.ValidatorActivationRequest{
			PublicKeys: publicKeys(v.keys),
		},
	).Return(clientStream, nil)
	clientStream.EXPECT().Recv().Return(
		nil,
		errors.New("fails"),
	)
	err := v.WaitForActivation(context.Background())
	want := "could not receive validator activation from stream"
	if !strings.Contains(err.Error(), want) {
		t.Errorf("Expected %v, received %v", want, err)
	}
}

func TestWaitActivation_LogsActivationEpochOK(t *testing.T) {
	hook := logTest.NewGlobal()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := internal.NewMockBeaconNodeValidatorClient(ctrl)

	v := validator{
		keys:            keyMap,
		pubkeys:         make([][]byte, 0),
		validatorClient: client,
	}
	v.pubkeys = publicKeys(v.keys)
	resp := generateMockStatusResponse(v.pubkeys)
	resp.Statuses[0].Status.Status = ethpb.ValidatorStatus_ACTIVE
	clientStream := internal.NewMockBeaconNodeValidator_WaitForActivationClient(ctrl)
	client.EXPECT().WaitForActivation(
		gomock.Any(),
		&ethpb.ValidatorActivationRequest{
			PublicKeys: publicKeys(v.keys),
		},
	).Return(clientStream, nil)
	clientStream.EXPECT().Recv().Return(
		resp,
		nil,
	)
	if err := v.WaitForActivation(context.Background()); err != nil {
		t.Errorf("Could not wait for activation: %v", err)
	}
	testutil.AssertLogsContain(t, hook, "Validator activated")
}

func TestCanonicalHeadSlot_FailedRPC(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := internal.NewMockBeaconChainClient(ctrl)
	v := validator{
		keys:         keyMap,
		beaconClient: client,
	}
	client.EXPECT().GetChainHead(
		gomock.Any(),
		gomock.Any(),
	).Return(nil, errors.New("failed"))
	if _, err := v.CanonicalHeadSlot(context.Background()); !strings.Contains(err.Error(), "failed") {
		t.Errorf("Wanted: %v, received: %v", "failed", err)
	}
}

func TestCanonicalHeadSlot_OK(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := internal.NewMockBeaconChainClient(ctrl)
	v := validator{
		keys:         keyMap,
		beaconClient: client,
	}
	client.EXPECT().GetChainHead(
		gomock.Any(),
		gomock.Any(),
	).Return(&ethpb.ChainHead{HeadSlot: 0}, nil)
	headSlot, err := v.CanonicalHeadSlot(context.Background())
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if headSlot != 0 {
		t.Errorf("Mismatch slots, wanted: %v, received: %v", 0, headSlot)
	}
}

func TestWaitMultipleActivation_LogsActivationEpochOK(t *testing.T) {
	hook := logTest.NewGlobal()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := internal.NewMockBeaconNodeValidatorClient(ctrl)

	v := validator{
		keys:            keyMapThreeValidators,
		pubkeys:         make([][]byte, 0),
		validatorClient: client,
	}
	v.pubkeys = publicKeys(v.keys)
	resp := generateMockStatusResponse(v.pubkeys)
	resp.Statuses[0].Status.Status = ethpb.ValidatorStatus_ACTIVE
	resp.Statuses[1].Status.Status = ethpb.ValidatorStatus_ACTIVE
	clientStream := internal.NewMockBeaconNodeValidator_WaitForActivationClient(ctrl)
	client.EXPECT().WaitForActivation(
		gomock.Any(),
		&ethpb.ValidatorActivationRequest{
			PublicKeys: v.pubkeys,
		},
	).Return(clientStream, nil)
	clientStream.EXPECT().Recv().Return(
		resp,
		nil,
	)
	if err := v.WaitForActivation(context.Background()); err != nil {
		t.Errorf("Could not wait for activation: %v", err)
	}
	testutil.AssertLogsContain(t, hook, "Validator activated")
}
func TestWaitActivation_NotAllValidatorsActivatedOK(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := internal.NewMockBeaconNodeValidatorClient(ctrl)

	v := validator{
		keys:            keyMapThreeValidators,
		validatorClient: client,
		pubkeys:         publicKeys(keyMapThreeValidators),
	}
	resp := generateMockStatusResponse(v.pubkeys)
	resp.Statuses[0].Status.Status = ethpb.ValidatorStatus_ACTIVE
	clientStream := internal.NewMockBeaconNodeValidator_WaitForActivationClient(ctrl)
	client.EXPECT().WaitForActivation(
		gomock.Any(),
		gomock.Any(),
	).Return(clientStream, nil)
	clientStream.EXPECT().Recv().Return(
		&ethpb.ValidatorActivationResponse{},
		nil,
	)
	clientStream.EXPECT().Recv().Return(
		resp,
		nil,
	)
	if err := v.WaitForActivation(context.Background()); err != nil {
		t.Errorf("Could not wait for activation: %v", err)
	}
}

func TestWaitSync_ContextCanceled(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	n := internal.NewMockNodeClient(ctrl)

	v := validator{
		node: n,
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	n.EXPECT().GetSyncStatus(
		gomock.Any(),
		gomock.Any(),
	).Return(&ethpb.SyncStatus{Syncing: true}, nil)

	err := v.WaitForSync(ctx)
	want := cancelledCtx
	if !strings.Contains(err.Error(), want) {
		t.Errorf("Expected %v, received %v", want, err)
	}
}

func TestWaitSync_NotSyncing(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	n := internal.NewMockNodeClient(ctrl)

	v := validator{
		node: n,
	}

	n.EXPECT().GetSyncStatus(
		gomock.Any(),
		gomock.Any(),
	).Return(&ethpb.SyncStatus{Syncing: false}, nil)

	err := v.WaitForSync(context.Background())
	if err != nil {
		t.Fatal(err)
	}
}

func TestWaitSync_Syncing(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	n := internal.NewMockNodeClient(ctrl)

	v := validator{
		node: n,
	}

	n.EXPECT().GetSyncStatus(
		gomock.Any(),
		gomock.Any(),
	).Return(&ethpb.SyncStatus{Syncing: true}, nil)

	n.EXPECT().GetSyncStatus(
		gomock.Any(),
		gomock.Any(),
	).Return(&ethpb.SyncStatus{Syncing: false}, nil)

	err := v.WaitForSync(context.Background())
	if err != nil {
		t.Fatal(err)
	}
}

func TestUpdateAssignments_DoesNothingWhenNotEpochStartAndAlreadyExistingAssignments(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := internal.NewMockBeaconNodeValidatorClient(ctrl)

	slot := uint64(1)
	v := validator{
		keys:            keyMap,
		validatorClient: client,
		duties: &ethpb.DutiesResponse{
			Duties: []*ethpb.DutiesResponse_Duty{
				{
					Committee:      []uint64{},
					AttesterSlot:   10,
					CommitteeIndex: 20,
				},
			},
		},
	}
	client.EXPECT().GetDuties(
		gomock.Any(),
		gomock.Any(),
	).Times(0)

	if err := v.UpdateDuties(context.Background(), slot); err != nil {
		t.Errorf("Could not update assignments: %v", err)
	}
}

func TestUpdateAssignments_ReturnsError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := internal.NewMockBeaconNodeValidatorClient(ctrl)

	v := validator{
		keys:            keyMap,
		validatorClient: client,
		duties: &ethpb.DutiesResponse{
			Duties: []*ethpb.DutiesResponse_Duty{
				{
					CommitteeIndex: 1,
				},
			},
		},
	}

	expected := errors.New("bad")

	client.EXPECT().GetDuties(
		gomock.Any(),
		gomock.Any(),
	).Return(nil, expected)

	if err := v.UpdateDuties(context.Background(), params.BeaconConfig().SlotsPerEpoch); err != expected {
		t.Errorf("Bad error; want=%v got=%v", expected, err)
	}
	if v.duties != nil {
		t.Error("Assignments should have been cleared on failure")
	}
}

func TestUpdateAssignments_OK(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := internal.NewMockBeaconNodeValidatorClient(ctrl)

	slot := params.BeaconConfig().SlotsPerEpoch
	resp := &ethpb.DutiesResponse{
		Duties: []*ethpb.DutiesResponse_Duty{
			{
				AttesterSlot:   params.BeaconConfig().SlotsPerEpoch,
				CommitteeIndex: 100,
				Committee:      []uint64{0, 1, 2, 3},
				PublicKey:      []byte("testPubKey_1"),
				ProposerSlot:   params.BeaconConfig().SlotsPerEpoch + 1,
			},
		},
	}
	v := validator{
		keys:            keyMap,
		validatorClient: client,
	}
	client.EXPECT().GetDuties(
		gomock.Any(),
		gomock.Any(),
	).Return(resp, nil)

	if err := v.UpdateDuties(context.Background(), slot); err != nil {
		t.Fatalf("Could not update assignments: %v", err)
	}
	if v.duties.Duties[0].ProposerSlot != params.BeaconConfig().SlotsPerEpoch+1 {
		t.Errorf(
			"Unexpected validator assignments. want=%v got=%v",
			params.BeaconConfig().SlotsPerEpoch+1,
			v.duties.Duties[0].ProposerSlot,
		)
	}
	if v.duties.Duties[0].AttesterSlot != params.BeaconConfig().SlotsPerEpoch {
		t.Errorf(
			"Unexpected validator assignments. want=%v got=%v",
			params.BeaconConfig().SlotsPerEpoch,
			v.duties.Duties[0].AttesterSlot,
		)
	}
	if v.duties.Duties[0].CommitteeIndex != resp.Duties[0].CommitteeIndex {
		t.Errorf(
			"Unexpected validator assignments. want=%v got=%v",
			resp.Duties[0].CommitteeIndex,
			v.duties.Duties[0].CommitteeIndex,
		)
	}
}

func TestRolesAt_OK(t *testing.T) {
	v, m, finish := setup(t)
	defer finish()

	v.duties = &ethpb.DutiesResponse{
		Duties: []*ethpb.DutiesResponse_Duty{
			{
				CommitteeIndex: 1,
				AttesterSlot:   1,
				PublicKey:      []byte{0x01},
			},
			{
				CommitteeIndex: 2,
				ProposerSlot:   1,
				PublicKey:      []byte{0x02},
			},
			{
				CommitteeIndex: 1,
				AttesterSlot:   2,
				PublicKey:      []byte{0x03},
			},
			{
				CommitteeIndex: 2,
				AttesterSlot:   1,
				ProposerSlot:   1,
				PublicKey:      []byte{0x04},
			},
		},
	}

	priv := bls.RandKey()
	keyStore, _ := keystore.NewKeyFromBLS(priv)
	v.keys[[48]byte{0x01}] = keyStore
	v.keys[[48]byte{0x04}] = keyStore

	m.validatorClient.EXPECT().DomainData(
		gomock.Any(), // ctx
		gomock.Any(), // epoch
	).Return(&ethpb.DomainResponse{}, nil /*err*/)
	m.validatorClient.EXPECT().DomainData(
		gomock.Any(), // ctx
		gomock.Any(), // epoch
	).Return(&ethpb.DomainResponse{}, nil /*err*/)

	roleMap, err := v.RolesAt(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}

	if roleMap[[48]byte{0x01}][0] != pb.ValidatorRole_ATTESTER {
		t.Errorf("Unexpected validator role. want: ValidatorRole_PROPOSER")
	}
	if roleMap[[48]byte{0x02}][0] != pb.ValidatorRole_PROPOSER {
		t.Errorf("Unexpected validator role. want: ValidatorRole_ATTESTER")
	}
	if roleMap[[48]byte{0x03}][0] != pb.ValidatorRole_UNKNOWN {
		t.Errorf("Unexpected validator role. want: UNKNOWN")
	}
	if roleMap[[48]byte{0x04}][0] != pb.ValidatorRole_PROPOSER {
		t.Errorf("Unexpected validator role. want: ValidatorRole_PROPOSER")
	}
	if roleMap[[48]byte{0x04}][1] != pb.ValidatorRole_ATTESTER {
		t.Errorf("Unexpected validator role. want: ValidatorRole_ATTESTER")
	}
	if roleMap[[48]byte{0x04}][2] != pb.ValidatorRole_AGGREGATOR {
		t.Errorf("Unexpected validator role. want: ValidatorRole_AGGREGATOR")
	}
}
