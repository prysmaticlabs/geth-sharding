package client

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"sync"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	types "github.com/prysmaticlabs/eth2-types"
	ethpb "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	prysmv2 "github.com/prysmaticlabs/prysm/proto/prysm/v2"
	validatorpb "github.com/prysmaticlabs/prysm/proto/validator/accounts/v2"
	"github.com/prysmaticlabs/prysm/shared/bls"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/prysmaticlabs/prysm/shared/event"
	"github.com/prysmaticlabs/prysm/shared/featureconfig"
	"github.com/prysmaticlabs/prysm/shared/mock"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/testutil"
	"github.com/prysmaticlabs/prysm/shared/testutil/assert"
	"github.com/prysmaticlabs/prysm/shared/testutil/require"
	"github.com/prysmaticlabs/prysm/validator/client/iface"
	dbTest "github.com/prysmaticlabs/prysm/validator/db/testing"
	"github.com/sirupsen/logrus"
	logTest "github.com/sirupsen/logrus/hooks/test"
	grpc "google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
)

func init() {
	logrus.SetLevel(logrus.DebugLevel)
	logrus.SetOutput(ioutil.Discard)
}

var _ iface.Validator = (*validator)(nil)

const cancelledCtx = "context has been canceled"

func genMockKeymanager(numKeys int) *mockKeymanager {
	km := make(map[[48]byte]bls.SecretKey, numKeys)
	for i := 0; i < numKeys; i++ {
		k, err := bls.RandKey()
		if err != nil {
			panic(err)
		}
		km[bytesutil.ToBytes48(k.PublicKey().Marshal())] = k
	}

	return &mockKeymanager{keysMap: km}
}

type mockKeymanager struct {
	lock                sync.RWMutex
	keysMap             map[[48]byte]bls.SecretKey
	fetchNoKeys         bool
	accountsChangedFeed *event.Feed
}

func (m *mockKeymanager) FetchValidatingPublicKeys(ctx context.Context) ([][48]byte, error) {
	m.lock.RLock()
	defer m.lock.RUnlock()
	keys := make([][48]byte, 0)
	if m.fetchNoKeys {
		m.fetchNoKeys = false
		return keys, nil
	}
	for pubKey := range m.keysMap {
		keys = append(keys, pubKey)
	}
	return keys, nil
}

func (m *mockKeymanager) Sign(ctx context.Context, req *validatorpb.SignRequest) (bls.Signature, error) {
	pubKey := [48]byte{}
	copy(pubKey[:], req.PublicKey)
	privKey, ok := m.keysMap[pubKey]
	if !ok {
		return nil, errors.New("not found")
	}
	sig := privKey.Sign(req.SigningRoot)
	return sig, nil
}

func (m *mockKeymanager) SubscribeAccountChanges(pubKeysChan chan [][48]byte) event.Subscription {
	if m.accountsChangedFeed == nil {
		m.accountsChangedFeed = &event.Feed{}
	}
	return m.accountsChangedFeed.Subscribe(pubKeysChan)
}

func (m *mockKeymanager) SimulateAccountChanges(newKeys [][48]byte) {
	m.accountsChangedFeed.Send(newKeys)
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

func TestWaitForChainStart_SetsGenesisInfo(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := mock.NewMockBeaconNodeValidatorClient(ctrl)

	db := dbTest.SetupDB(t, [][48]byte{})
	v := validator{
		validatorClient: client,
		db:              db,
	}

	// Make sure its clean at the start.
	savedGenValRoot, err := db.GenesisValidatorsRoot(context.Background())
	require.NoError(t, err)
	assert.DeepEqual(t, []byte(nil), savedGenValRoot, "Unexpected saved genesis validator root")

	genesis := uint64(time.Unix(1, 0).Unix())
	genesisValidatorsRoot := bytesutil.ToBytes32([]byte("validators"))
	clientStream := mock.NewMockBeaconNodeValidator_WaitForChainStartClient(ctrl)
	client.EXPECT().WaitForChainStart(
		gomock.Any(),
		&emptypb.Empty{},
	).Return(clientStream, nil)
	clientStream.EXPECT().Recv().Return(
		&ethpb.ChainStartResponse{
			Started:               true,
			GenesisTime:           genesis,
			GenesisValidatorsRoot: genesisValidatorsRoot[:],
		},
		nil,
	)
	require.NoError(t, v.WaitForChainStart(context.Background()))
	savedGenValRoot, err = db.GenesisValidatorsRoot(context.Background())
	require.NoError(t, err)

	assert.DeepEqual(t, genesisValidatorsRoot[:], savedGenValRoot, "Unexpected saved genesis validator root")
	assert.Equal(t, genesis, v.genesisTime, "Unexpected chain start time")
	assert.NotNil(t, v.ticker, "Expected ticker to be set, received nil")

	// Make sure theres no errors running if its the same data.
	client.EXPECT().WaitForChainStart(
		gomock.Any(),
		&emptypb.Empty{},
	).Return(clientStream, nil)
	clientStream.EXPECT().Recv().Return(
		&ethpb.ChainStartResponse{
			Started:               true,
			GenesisTime:           genesis,
			GenesisValidatorsRoot: genesisValidatorsRoot[:],
		},
		nil,
	)
	require.NoError(t, v.WaitForChainStart(context.Background()))
}

func TestWaitForChainStart_SetsGenesisInfo_IncorrectSecondTry(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := mock.NewMockBeaconNodeValidatorClient(ctrl)

	db := dbTest.SetupDB(t, [][48]byte{})
	v := validator{
		validatorClient: client,
		db:              db,
	}
	genesis := uint64(time.Unix(1, 0).Unix())
	genesisValidatorsRoot := bytesutil.ToBytes32([]byte("validators"))
	clientStream := mock.NewMockBeaconNodeValidator_WaitForChainStartClient(ctrl)
	client.EXPECT().WaitForChainStart(
		gomock.Any(),
		&emptypb.Empty{},
	).Return(clientStream, nil)
	clientStream.EXPECT().Recv().Return(
		&ethpb.ChainStartResponse{
			Started:               true,
			GenesisTime:           genesis,
			GenesisValidatorsRoot: genesisValidatorsRoot[:],
		},
		nil,
	)
	require.NoError(t, v.WaitForChainStart(context.Background()))
	savedGenValRoot, err := db.GenesisValidatorsRoot(context.Background())
	require.NoError(t, err)

	assert.DeepEqual(t, genesisValidatorsRoot[:], savedGenValRoot, "Unexpected saved genesis validator root")
	assert.Equal(t, genesis, v.genesisTime, "Unexpected chain start time")
	assert.NotNil(t, v.ticker, "Expected ticker to be set, received nil")

	genesisValidatorsRoot = bytesutil.ToBytes32([]byte("badvalidators"))

	// Make sure theres no errors running if its the same data.
	client.EXPECT().WaitForChainStart(
		gomock.Any(),
		&emptypb.Empty{},
	).Return(clientStream, nil)
	clientStream.EXPECT().Recv().Return(
		&ethpb.ChainStartResponse{
			Started:               true,
			GenesisTime:           genesis,
			GenesisValidatorsRoot: genesisValidatorsRoot[:],
		},
		nil,
	)
	err = v.WaitForChainStart(context.Background())
	require.ErrorContains(t, "does not match root saved", err)
}

func TestWaitForChainStart_ContextCanceled(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := mock.NewMockBeaconNodeValidatorClient(ctrl)

	v := validator{
		//keyManager:      testKeyManager,
		validatorClient: client,
	}
	genesis := uint64(time.Unix(0, 0).Unix())
	genesisValidatorsRoot := bytesutil.PadTo([]byte("validators"), 32)
	clientStream := mock.NewMockBeaconNodeValidator_WaitForChainStartClient(ctrl)
	client.EXPECT().WaitForChainStart(
		gomock.Any(),
		&emptypb.Empty{},
	).Return(clientStream, nil)
	clientStream.EXPECT().Recv().Return(
		&ethpb.ChainStartResponse{
			Started:               true,
			GenesisTime:           genesis,
			GenesisValidatorsRoot: genesisValidatorsRoot,
		},
		nil,
	)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	assert.ErrorContains(t, cancelledCtx, v.WaitForChainStart(ctx))
}

func TestWaitForChainStart_StreamSetupFails(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := mock.NewMockBeaconNodeValidatorClient(ctrl)

	privKey, err := bls.RandKey()
	require.NoError(t, err)
	pubKey := [48]byte{}
	copy(pubKey[:], privKey.PublicKey().Marshal())
	km := &mockKeymanager{
		keysMap: make(map[[48]byte]bls.SecretKey),
	}
	v := validator{
		validatorClient: client,
		keyManager:      km,
	}
	clientStream := mock.NewMockBeaconNodeValidator_WaitForChainStartClient(ctrl)
	client.EXPECT().WaitForChainStart(
		gomock.Any(),
		&emptypb.Empty{},
	).Return(clientStream, errors.New("failed stream"))
	err = v.WaitForChainStart(context.Background())
	want := "could not setup beacon chain ChainStart streaming client"
	assert.ErrorContains(t, want, err)
}

func TestWaitForChainStart_ReceiveErrorFromStream(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := mock.NewMockBeaconNodeValidatorClient(ctrl)

	v := validator{
		validatorClient: client,
	}
	clientStream := mock.NewMockBeaconNodeValidator_WaitForChainStartClient(ctrl)
	client.EXPECT().WaitForChainStart(
		gomock.Any(),
		&emptypb.Empty{},
	).Return(clientStream, nil)
	clientStream.EXPECT().Recv().Return(
		nil,
		errors.New("fails"),
	)
	err := v.WaitForChainStart(context.Background())
	want := "could not receive ChainStart from stream"
	assert.ErrorContains(t, want, err)
}

func TestCanonicalHeadSlot_FailedRPC(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := mock.NewMockBeaconChainClient(ctrl)
	v := validator{
		beaconClient: client,
		genesisTime:  1,
	}
	client.EXPECT().GetChainHead(
		gomock.Any(),
		gomock.Any(),
	).Return(nil, errors.New("failed"))
	_, err := v.CanonicalHeadSlot(context.Background())
	assert.ErrorContains(t, "failed", err)
}

func TestCanonicalHeadSlot_OK(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := mock.NewMockBeaconChainClient(ctrl)
	v := validator{
		beaconClient: client,
	}
	client.EXPECT().GetChainHead(
		gomock.Any(),
		gomock.Any(),
	).Return(&ethpb.ChainHead{HeadSlot: 0}, nil)
	headSlot, err := v.CanonicalHeadSlot(context.Background())
	require.NoError(t, err)
	assert.Equal(t, types.Slot(0), headSlot, "Mismatch slots")
}

func TestWaitMultipleActivation_LogsActivationEpochOK(t *testing.T) {
	hook := logTest.NewGlobal()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := mock.NewMockBeaconNodeValidatorClient(ctrl)
	privKey, err := bls.RandKey()
	require.NoError(t, err)
	pubKey := [48]byte{}
	copy(pubKey[:], privKey.PublicKey().Marshal())
	km := &mockKeymanager{
		keysMap: map[[48]byte]bls.SecretKey{
			pubKey: privKey,
		},
	}
	v := validator{
		validatorClient: client,
		keyManager:      km,
		genesisTime:     1,
	}

	resp := generateMockStatusResponse([][]byte{pubKey[:]})
	resp.Statuses[0].Status.Status = ethpb.ValidatorStatus_ACTIVE
	clientStream := mock.NewMockBeaconNodeValidator_WaitForActivationClient(ctrl)
	client.EXPECT().WaitForActivation(
		gomock.Any(),
		&ethpb.ValidatorActivationRequest{
			PublicKeys: [][]byte{pubKey[:]},
		},
	).Return(clientStream, nil)
	clientStream.EXPECT().Recv().Return(
		resp,
		nil,
	)
	require.NoError(t, v.WaitForActivation(context.Background(), nil), "Could not wait for activation")
	require.LogsContain(t, hook, "Validator activated")
}

func TestWaitActivation_NotAllValidatorsActivatedOK(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := mock.NewMockBeaconNodeValidatorClient(ctrl)

	privKey, err := bls.RandKey()
	require.NoError(t, err)
	pubKey := [48]byte{}
	copy(pubKey[:], privKey.PublicKey().Marshal())
	km := &mockKeymanager{
		keysMap: map[[48]byte]bls.SecretKey{
			pubKey: privKey,
		},
	}
	v := validator{
		validatorClient: client,
		keyManager:      km,
		genesisTime:     1,
	}
	resp := generateMockStatusResponse([][]byte{pubKey[:]})
	resp.Statuses[0].Status.Status = ethpb.ValidatorStatus_ACTIVE
	clientStream := mock.NewMockBeaconNodeValidator_WaitForActivationClient(ctrl)
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
	assert.NoError(t, v.WaitForActivation(context.Background(), nil), "Could not wait for activation")
}

func TestWaitSync_ContextCanceled(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	n := mock.NewMockNodeClient(ctrl)

	v := validator{
		node: n,
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	n.EXPECT().GetSyncStatus(
		gomock.Any(),
		gomock.Any(),
	).Return(&ethpb.SyncStatus{Syncing: true}, nil)

	assert.ErrorContains(t, cancelledCtx, v.WaitForSync(ctx))
}

func TestWaitSync_NotSyncing(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	n := mock.NewMockNodeClient(ctrl)

	v := validator{
		node: n,
	}

	n.EXPECT().GetSyncStatus(
		gomock.Any(),
		gomock.Any(),
	).Return(&ethpb.SyncStatus{Syncing: false}, nil)

	require.NoError(t, v.WaitForSync(context.Background()))
}

func TestWaitSync_Syncing(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	n := mock.NewMockNodeClient(ctrl)

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

	require.NoError(t, v.WaitForSync(context.Background()))
}

func TestUpdateDuties_DoesNothingWhenNotEpochStart_AlreadyExistingAssignments(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := mock.NewMockBeaconNodeValidatorClient(ctrl)

	slot := types.Slot(1)
	v := validator{
		validatorClient: client,
		duties: &ethpb.DutiesResponse{
			Duties: []*ethpb.DutiesResponse_Duty{
				{
					Committee:      []types.ValidatorIndex{},
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

	assert.NoError(t, v.UpdateDuties(context.Background(), slot), "Could not update assignments")
}

func TestUpdateDuties_ReturnsError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := mock.NewMockBeaconNodeValidatorClient(ctrl)

	privKey, err := bls.RandKey()
	require.NoError(t, err)
	pubKey := [48]byte{}
	copy(pubKey[:], privKey.PublicKey().Marshal())
	km := &mockKeymanager{
		keysMap: map[[48]byte]bls.SecretKey{
			pubKey: privKey,
		},
	}
	v := validator{
		validatorClient: client,
		keyManager:      km,
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

	assert.ErrorContains(t, expected.Error(), v.UpdateDuties(context.Background(), params.BeaconConfig().SlotsPerEpoch))
	assert.Equal(t, (*ethpb.DutiesResponse)(nil), v.duties, "Assignments should have been cleared on failure")
}

func TestUpdateDuties_OK(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := mock.NewMockBeaconNodeValidatorClient(ctrl)

	slot := params.BeaconConfig().SlotsPerEpoch
	privKey, err := bls.RandKey()
	require.NoError(t, err)
	pubKey := [48]byte{}
	copy(pubKey[:], privKey.PublicKey().Marshal())
	km := &mockKeymanager{
		keysMap: map[[48]byte]bls.SecretKey{
			pubKey: privKey,
		},
	}
	resp := &ethpb.DutiesResponse{
		Duties: []*ethpb.DutiesResponse_Duty{
			{
				AttesterSlot:   params.BeaconConfig().SlotsPerEpoch,
				ValidatorIndex: 200,
				CommitteeIndex: 100,
				Committee:      []types.ValidatorIndex{0, 1, 2, 3},
				PublicKey:      []byte("testPubKey_1"),
				ProposerSlots:  []types.Slot{params.BeaconConfig().SlotsPerEpoch + 1},
			},
		},
	}
	v := validator{
		keyManager:      km,
		validatorClient: client,
	}
	client.EXPECT().GetDuties(
		gomock.Any(),
		gomock.Any(),
	).Return(resp, nil)

	var wg sync.WaitGroup
	wg.Add(1)

	client.EXPECT().SubscribeCommitteeSubnets(
		gomock.Any(),
		gomock.Any(),
	).DoAndReturn(func(_ context.Context, _ *ethpb.CommitteeSubnetsSubscribeRequest, arg2 ...grpc.CallOption) (*emptypb.Empty, error) {
		wg.Done()
		return nil, nil
	})

	require.NoError(t, v.UpdateDuties(context.Background(), slot), "Could not update assignments")

	testutil.WaitTimeout(&wg, 3*time.Second)

	assert.Equal(t, params.BeaconConfig().SlotsPerEpoch+1, v.duties.Duties[0].ProposerSlots[0], "Unexpected validator assignments")
	assert.Equal(t, params.BeaconConfig().SlotsPerEpoch, v.duties.Duties[0].AttesterSlot, "Unexpected validator assignments")
	assert.Equal(t, resp.Duties[0].CommitteeIndex, v.duties.Duties[0].CommitteeIndex, "Unexpected validator assignments")
	assert.Equal(t, resp.Duties[0].ValidatorIndex, v.duties.Duties[0].ValidatorIndex, "Unexpected validator assignments")
}

func TestUpdateDuties_OK_FilterBlacklistedPublicKeys(t *testing.T) {
	hook := logTest.NewGlobal()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := mock.NewMockBeaconNodeValidatorClient(ctrl)
	slot := params.BeaconConfig().SlotsPerEpoch

	numValidators := 10
	keysMap := make(map[[48]byte]bls.SecretKey)
	blacklistedPublicKeys := make(map[[48]byte]bool)
	for i := 0; i < numValidators; i++ {
		priv, err := bls.RandKey()
		require.NoError(t, err)
		pubKey := [48]byte{}
		copy(pubKey[:], priv.PublicKey().Marshal())
		keysMap[pubKey] = priv
		blacklistedPublicKeys[pubKey] = true
	}

	km := &mockKeymanager{
		keysMap: keysMap,
	}
	resp := &ethpb.DutiesResponse{
		Duties: []*ethpb.DutiesResponse_Duty{},
	}
	v := validator{
		keyManager:                     km,
		validatorClient:                client,
		eipImportBlacklistedPublicKeys: blacklistedPublicKeys,
	}
	client.EXPECT().GetDuties(
		gomock.Any(),
		gomock.Any(),
	).Return(resp, nil)

	var wg sync.WaitGroup
	wg.Add(1)
	client.EXPECT().SubscribeCommitteeSubnets(
		gomock.Any(),
		gomock.Any(),
	).DoAndReturn(func(_ context.Context, _ *ethpb.CommitteeSubnetsSubscribeRequest, arg2 ...grpc.CallOption) (*emptypb.Empty, error) {
		wg.Done()
		return nil, nil
	})

	require.NoError(t, v.UpdateDuties(context.Background(), slot), "Could not update assignments")

	testutil.WaitTimeout(&wg, 3*time.Second)

	for range blacklistedPublicKeys {
		assert.LogsContain(t, hook, "Not including slashable public key")
	}
}

func TestRolesAt_OK(t *testing.T) {
	v, m, validatorKey, finish := setup(t)
	defer finish()

	v.duties = &ethpb.DutiesResponse{
		Duties: []*ethpb.DutiesResponse_Duty{
			{
				CommitteeIndex:  1,
				AttesterSlot:    1,
				PublicKey:       validatorKey.PublicKey().Marshal(),
				IsSyncCommittee: true,
			},
		},
		NextEpochDuties: []*ethpb.DutiesResponse_Duty{
			{
				CommitteeIndex:  1,
				AttesterSlot:    1,
				PublicKey:       validatorKey.PublicKey().Marshal(),
				IsSyncCommittee: true,
			},
		},
	}

	m.validatorClient.EXPECT().DomainData(
		gomock.Any(), // ctx
		gomock.Any(), // epoch
	).Return(&ethpb.DomainResponse{SignatureDomain: make([]byte, 32)}, nil /*err*/)

	m.validatorClientV2.EXPECT().GetSyncSubcommitteeIndex(
		gomock.Any(), // ctx
		&prysmv2.SyncSubcommitteeIndexRequest{
			PublicKey: validatorKey.PublicKey().Marshal(),
			Slot:      1,
		},
	).Return(&prysmv2.SyncSubcommitteeIndexResponse{}, nil /*err*/)

	roleMap, err := v.RolesAt(context.Background(), 1)
	require.NoError(t, err)

	assert.Equal(t, iface.RoleAttester, roleMap[bytesutil.ToBytes48(validatorKey.PublicKey().Marshal())][0])
	assert.Equal(t, iface.RoleAggregator, roleMap[bytesutil.ToBytes48(validatorKey.PublicKey().Marshal())][1])
	assert.Equal(t, iface.RoleSyncCommittee, roleMap[bytesutil.ToBytes48(validatorKey.PublicKey().Marshal())][2])

	// Test sync committee role at epoch boundary.
	v.duties = &ethpb.DutiesResponse{
		Duties: []*ethpb.DutiesResponse_Duty{
			{
				CommitteeIndex:  1,
				AttesterSlot:    1,
				PublicKey:       validatorKey.PublicKey().Marshal(),
				IsSyncCommittee: false,
			},
		},
		NextEpochDuties: []*ethpb.DutiesResponse_Duty{
			{
				CommitteeIndex:  1,
				AttesterSlot:    1,
				PublicKey:       validatorKey.PublicKey().Marshal(),
				IsSyncCommittee: true,
			},
		},
	}

	m.validatorClientV2.EXPECT().GetSyncSubcommitteeIndex(
		gomock.Any(), // ctx
		&prysmv2.SyncSubcommitteeIndexRequest{
			PublicKey: validatorKey.PublicKey().Marshal(),
			Slot:      31,
		},
	).Return(&prysmv2.SyncSubcommitteeIndexResponse{}, nil /*err*/)

	roleMap, err = v.RolesAt(context.Background(), params.BeaconConfig().SlotsPerEpoch-1)
	require.NoError(t, err)
	assert.Equal(t, iface.RoleSyncCommittee, roleMap[bytesutil.ToBytes48(validatorKey.PublicKey().Marshal())][0])
}

func TestRolesAt_DoesNotAssignProposer_Slot0(t *testing.T) {
	v, m, validatorKey, finish := setup(t)
	defer finish()

	v.duties = &ethpb.DutiesResponse{
		Duties: []*ethpb.DutiesResponse_Duty{
			{
				CommitteeIndex: 1,
				AttesterSlot:   0,
				ProposerSlots:  []types.Slot{0},
				PublicKey:      validatorKey.PublicKey().Marshal(),
			},
		},
	}

	m.validatorClient.EXPECT().DomainData(
		gomock.Any(), // ctx
		gomock.Any(), // epoch
	).Return(&ethpb.DomainResponse{SignatureDomain: make([]byte, 32)}, nil /*err*/)

	roleMap, err := v.RolesAt(context.Background(), 0)
	require.NoError(t, err)

	assert.Equal(t, iface.RoleAttester, roleMap[bytesutil.ToBytes48(validatorKey.PublicKey().Marshal())][0])
}

func TestCheckAndLogValidatorStatus_OK(t *testing.T) {
	nonexistentIndex := types.ValidatorIndex(^uint64(0))
	type statusTest struct {
		name   string
		status *validatorStatus
		log    string
		active bool
	}
	pubKeys := [][]byte{bytesutil.Uint64ToBytesLittleEndian(0)}
	tests := []statusTest{
		{
			name: "UNKNOWN_STATUS, no deposit found yet",
			status: &validatorStatus{
				publicKey: pubKeys[0],
				index:     nonexistentIndex,
				status: &ethpb.ValidatorStatusResponse{
					Status: ethpb.ValidatorStatus_UNKNOWN_STATUS,
				},
			},
			log:    "Waiting for deposit to be observed by beacon node",
			active: false,
		},
		{
			name: "DEPOSITED into state",
			status: &validatorStatus{
				publicKey: pubKeys[0],
				index:     30,
				status: &ethpb.ValidatorStatusResponse{
					Status:                    ethpb.ValidatorStatus_DEPOSITED,
					PositionInActivationQueue: 30,
				},
			},
			log:    "Deposit processed, entering activation queue after finalization\" index=30 positionInActivationQueue=30",
			active: false,
		},
		{
			name: "PENDING",
			status: &validatorStatus{
				publicKey: pubKeys[0],
				index:     50,
				status: &ethpb.ValidatorStatusResponse{
					Status:                    ethpb.ValidatorStatus_PENDING,
					ActivationEpoch:           params.BeaconConfig().FarFutureEpoch,
					PositionInActivationQueue: 6,
				},
			},
			log:    "Waiting to be assigned activation epoch\" index=50 positionInActivationQueue=6",
			active: false,
		},
		{
			name: "PENDING",
			status: &validatorStatus{
				publicKey: pubKeys[0],
				index:     89,
				status: &ethpb.ValidatorStatusResponse{
					Status:                    ethpb.ValidatorStatus_PENDING,
					ActivationEpoch:           60,
					PositionInActivationQueue: 5,
				},
			},
			log:    "Waiting for activation\" activationEpoch=60 index=89",
			active: false,
		},
		{
			name: "ACTIVE",
			status: &validatorStatus{
				publicKey: pubKeys[0],
				index:     89,
				status: &ethpb.ValidatorStatusResponse{
					Status: ethpb.ValidatorStatus_ACTIVE,
				},
			},
			active: true,
		},
		{
			name: "EXITING",
			status: &validatorStatus{
				publicKey: pubKeys[0],
				index:     89,
				status: &ethpb.ValidatorStatusResponse{
					Status: ethpb.ValidatorStatus_EXITING,
				},
			},
			active: true,
		},
		{
			name: "EXITED",
			status: &validatorStatus{
				publicKey: pubKeys[0],
				status: &ethpb.ValidatorStatusResponse{
					Status: ethpb.ValidatorStatus_EXITED,
				},
			},
			log:    "Validator exited",
			active: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			hook := logTest.NewGlobal()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			client := mock.NewMockBeaconNodeValidatorClient(ctrl)
			v := validator{
				validatorClient: client,
				duties: &ethpb.DutiesResponse{
					Duties: []*ethpb.DutiesResponse_Duty{
						{
							CommitteeIndex: 1,
						},
					},
				},
			}

			active := v.checkAndLogValidatorStatus([]*validatorStatus{test.status})
			require.Equal(t, test.active, active)
			if test.log != "" {
				require.LogsContain(t, hook, test.log)
			}
		})
	}
}

func TestAllValidatorsAreExited_AllExited(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := mock.NewMockBeaconNodeValidatorClient(ctrl)

	statuses := []*ethpb.ValidatorStatusResponse{
		{Status: ethpb.ValidatorStatus_EXITED},
		{Status: ethpb.ValidatorStatus_EXITED},
	}

	client.EXPECT().MultipleValidatorStatus(
		gomock.Any(), // ctx
		gomock.Any(), // request
	).Return(&ethpb.MultipleValidatorStatusResponse{Statuses: statuses}, nil /*err*/)

	v := validator{keyManager: genMockKeymanager(2), validatorClient: client}
	exited, err := v.AllValidatorsAreExited(context.Background())
	require.NoError(t, err)
	assert.Equal(t, true, exited)
}

func TestAllValidatorsAreExited_NotAllExited(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := mock.NewMockBeaconNodeValidatorClient(ctrl)

	statuses := []*ethpb.ValidatorStatusResponse{
		{Status: ethpb.ValidatorStatus_ACTIVE},
		{Status: ethpb.ValidatorStatus_EXITED},
	}

	client.EXPECT().MultipleValidatorStatus(
		gomock.Any(), // ctx
		gomock.Any(), // request
	).Return(&ethpb.MultipleValidatorStatusResponse{Statuses: statuses}, nil /*err*/)

	v := validator{keyManager: genMockKeymanager(2), validatorClient: client}
	exited, err := v.AllValidatorsAreExited(context.Background())
	require.NoError(t, err)
	assert.Equal(t, false, exited)
}

func TestAllValidatorsAreExited_PartialResult(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := mock.NewMockBeaconNodeValidatorClient(ctrl)

	statuses := []*ethpb.ValidatorStatusResponse{
		{Status: ethpb.ValidatorStatus_EXITED},
	}

	client.EXPECT().MultipleValidatorStatus(
		gomock.Any(), // ctx
		gomock.Any(), // request
	).Return(&ethpb.MultipleValidatorStatusResponse{Statuses: statuses}, nil /*err*/)

	v := validator{keyManager: genMockKeymanager(2), validatorClient: client}
	exited, err := v.AllValidatorsAreExited(context.Background())
	require.ErrorContains(t, "number of status responses did not match number of requested keys", err)
	assert.Equal(t, false, exited)
}

func TestAllValidatorsAreExited_NoKeys(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := mock.NewMockBeaconNodeValidatorClient(ctrl)
	v := validator{keyManager: genMockKeymanager(0), validatorClient: client}
	exited, err := v.AllValidatorsAreExited(context.Background())
	require.NoError(t, err)
	assert.Equal(t, false, exited)
}

// TestAllValidatorsAreExited_CorrectRequest is a regression test that checks if the request contains the correct keys
func TestAllValidatorsAreExited_CorrectRequest(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := mock.NewMockBeaconNodeValidatorClient(ctrl)

	// Create two different public keys
	pubKey0 := [48]byte{1, 2, 3, 4}
	pubKey1 := [48]byte{6, 7, 8, 9}
	// This is the request expected from AllValidatorsAreExited()
	request := &ethpb.MultipleValidatorStatusRequest{
		PublicKeys: [][]byte{
			pubKey0[:],
			pubKey1[:],
		},
	}
	statuses := []*ethpb.ValidatorStatusResponse{
		{Status: ethpb.ValidatorStatus_ACTIVE},
		{Status: ethpb.ValidatorStatus_EXITED},
	}

	client.EXPECT().MultipleValidatorStatus(
		gomock.Any(), // ctx
		request,      // request
	).Return(&ethpb.MultipleValidatorStatusResponse{Statuses: statuses}, nil /*err*/)

	keysMap := make(map[[48]byte]bls.SecretKey)
	// secretKey below is just filler and is used multiple times
	secretKeyBytes := [32]byte{1}
	secretKey, err := bls.SecretKeyFromBytes(secretKeyBytes[:])
	require.NoError(t, err)
	keysMap[pubKey0] = secretKey
	keysMap[pubKey1] = secretKey

	// If AllValidatorsAreExited does not create the expected request, this test will fail
	v := validator{keyManager: &mockKeymanager{keysMap: keysMap}, validatorClient: client}
	exited, err := v.AllValidatorsAreExited(context.Background())
	require.NoError(t, err)
	assert.Equal(t, false, exited)
}

func TestService_ReceiveBlocks_NilBlock(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	valClient := mock.NewMockBeaconNodeValidatorAltairClient(ctrl)
	v := validator{
		blockFeed:         new(event.Feed),
		validatorClientV2: valClient,
	}
	stream := mock.NewMockBeaconNodeValidatorAltair_StreamBlocksClient(ctrl)
	ctx, cancel := context.WithCancel(context.Background())
	valClient.EXPECT().StreamBlocks(
		gomock.Any(),
		&ethpb.StreamBlocksRequest{VerifiedOnly: true},
	).Return(stream, nil)
	stream.EXPECT().Context().Return(ctx).AnyTimes()
	stream.EXPECT().Recv().Return(
		&prysmv2.StreamBlocksResponse{Block: &prysmv2.StreamBlocksResponse_Phase0Block{
			Phase0Block: &ethpb.SignedBeaconBlock{}}},
		nil,
	).Do(func() {
		cancel()
	})
	connectionErrorChannel := make(chan error)
	v.ReceiveBlocks(ctx, connectionErrorChannel)
	require.Equal(t, types.Slot(0), v.highestValidSlot)
}

func TestService_ReceiveBlocks_SetHighest(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := mock.NewMockBeaconNodeValidatorAltairClient(ctrl)

	v := validator{
		validatorClientV2: client,
		blockFeed:         new(event.Feed),
	}
	stream := mock.NewMockBeaconNodeValidatorAltair_StreamBlocksClient(ctrl)
	ctx, cancel := context.WithCancel(context.Background())
	client.EXPECT().StreamBlocks(
		gomock.Any(),
		&ethpb.StreamBlocksRequest{VerifiedOnly: true},
	).Return(stream, nil)
	stream.EXPECT().Context().Return(ctx).AnyTimes()
	slot := types.Slot(100)
	stream.EXPECT().Recv().Return(
		&prysmv2.StreamBlocksResponse{
			Block: &prysmv2.StreamBlocksResponse_Phase0Block{
				Phase0Block: &ethpb.SignedBeaconBlock{Block: &ethpb.BeaconBlock{Slot: slot, Body: &ethpb.BeaconBlockBody{}}}},
		},
		nil,
	).Do(func() {
		cancel()
	})
	connectionErrorChannel := make(chan error)
	v.ReceiveBlocks(ctx, connectionErrorChannel)
	require.Equal(t, slot, v.highestValidSlot)
}

type doppelGangerRequestMatcher struct {
	req *ethpb.DoppelGangerRequest
}

var _ gomock.Matcher = (*doppelGangerRequestMatcher)(nil)

func (m *doppelGangerRequestMatcher) Matches(x interface{}) bool {
	r, ok := x.(*ethpb.DoppelGangerRequest)
	if !ok {
		panic("Invalid match type")
	}
	return gomock.InAnyOrder(m.req.ValidatorRequests).Matches(r.ValidatorRequests)
}

func (m *doppelGangerRequestMatcher) String() string {
	return fmt.Sprintf("%#v", m.req.ValidatorRequests)
}

func TestValidator_CheckDoppelGanger(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	flgs := featureconfig.Get()
	flgs.EnableDoppelGanger = true
	reset := featureconfig.InitWithReset(flgs)
	defer reset()
	tests := []struct {
		name            string
		validatorSetter func(t *testing.T) *validator
		err             string
	}{
		{
			name: "no doppelganger",
			validatorSetter: func(t *testing.T) *validator {
				client := mock.NewMockBeaconNodeValidatorClient(ctrl)
				km := genMockKeymanager(10)
				keys, err := km.FetchValidatingPublicKeys(context.Background())
				assert.NoError(t, err)
				db := dbTest.SetupDB(t, keys)
				req := &ethpb.DoppelGangerRequest{ValidatorRequests: []*ethpb.DoppelGangerRequest_ValidatorRequest{}}
				resp := &ethpb.DoppelGangerRequest{ValidatorRequests: []*ethpb.DoppelGangerRequest_ValidatorRequest{}}
				for _, k := range keys {
					pkey := k
					att := createAttestation(10, 12)
					rt, err := att.Data.HashTreeRoot()
					assert.NoError(t, err)
					assert.NoError(t, db.SaveAttestationForPubKey(context.Background(), pkey, rt, att))
					resp.ValidatorRequests = append(resp.ValidatorRequests, &ethpb.DoppelGangerRequest_ValidatorRequest{PublicKey: pkey[:], Epoch: att.Data.Target.Epoch, SignedRoot: rt[:]})
					req.ValidatorRequests = append(req.ValidatorRequests, &ethpb.DoppelGangerRequest_ValidatorRequest{PublicKey: pkey[:], Epoch: att.Data.Target.Epoch, SignedRoot: rt[:]})
				}
				v := &validator{
					validatorClient: client,
					keyManager:      km,
					db:              db,
				}
				client.EXPECT().CheckDoppelGanger(
					gomock.Any(),                     // ctx
					&doppelGangerRequestMatcher{req}, // request
				).Return(nil, nil /*err*/)

				return v
			},
		},
		{
			name: "multiple doppelganger exists",
			validatorSetter: func(t *testing.T) *validator {
				client := mock.NewMockBeaconNodeValidatorClient(ctrl)
				km := genMockKeymanager(10)
				keys, err := km.FetchValidatingPublicKeys(context.Background())
				assert.NoError(t, err)
				db := dbTest.SetupDB(t, keys)
				req := &ethpb.DoppelGangerRequest{ValidatorRequests: []*ethpb.DoppelGangerRequest_ValidatorRequest{}}
				resp := &ethpb.DoppelGangerResponse{Responses: []*ethpb.DoppelGangerResponse_ValidatorResponse{}}
				for i, k := range keys {
					pkey := k
					att := createAttestation(10, 12)
					rt, err := att.Data.HashTreeRoot()
					assert.NoError(t, err)
					assert.NoError(t, db.SaveAttestationForPubKey(context.Background(), pkey, rt, att))
					if i%3 == 0 {
						resp.Responses = append(resp.Responses, &ethpb.DoppelGangerResponse_ValidatorResponse{PublicKey: pkey[:], DuplicateExists: true})
					}
					req.ValidatorRequests = append(req.ValidatorRequests, &ethpb.DoppelGangerRequest_ValidatorRequest{PublicKey: pkey[:], Epoch: att.Data.Target.Epoch, SignedRoot: rt[:]})
				}
				v := &validator{
					validatorClient: client,
					keyManager:      km,
					db:              db,
				}
				client.EXPECT().CheckDoppelGanger(
					gomock.Any(),                     // ctx
					&doppelGangerRequestMatcher{req}, // request
				).Return(resp, nil /*err*/)
				return v
			},
			err: "Duplicate instances exists in the network for validator keys",
		},
		{
			name: "single doppelganger exists",
			validatorSetter: func(t *testing.T) *validator {
				client := mock.NewMockBeaconNodeValidatorClient(ctrl)
				km := genMockKeymanager(10)
				keys, err := km.FetchValidatingPublicKeys(context.Background())
				assert.NoError(t, err)
				db := dbTest.SetupDB(t, keys)
				req := &ethpb.DoppelGangerRequest{ValidatorRequests: []*ethpb.DoppelGangerRequest_ValidatorRequest{}}
				resp := &ethpb.DoppelGangerResponse{Responses: []*ethpb.DoppelGangerResponse_ValidatorResponse{}}
				for i, k := range keys {
					pkey := k
					att := createAttestation(10, 12)
					rt, err := att.Data.HashTreeRoot()
					assert.NoError(t, err)
					assert.NoError(t, db.SaveAttestationForPubKey(context.Background(), pkey, rt, att))
					if i%9 == 0 {
						resp.Responses = append(resp.Responses, &ethpb.DoppelGangerResponse_ValidatorResponse{PublicKey: pkey[:], DuplicateExists: true})
					}
					req.ValidatorRequests = append(req.ValidatorRequests, &ethpb.DoppelGangerRequest_ValidatorRequest{PublicKey: pkey[:], Epoch: att.Data.Target.Epoch, SignedRoot: rt[:]})
				}
				v := &validator{
					validatorClient: client,
					keyManager:      km,
					db:              db,
				}
				client.EXPECT().CheckDoppelGanger(
					gomock.Any(),                     // ctx
					&doppelGangerRequestMatcher{req}, // request
				).Return(resp, nil /*err*/)
				return v
			},
			err: "Duplicate instances exists in the network for validator keys",
		},
		{
			name: "multiple attestations saved",
			validatorSetter: func(t *testing.T) *validator {
				client := mock.NewMockBeaconNodeValidatorClient(ctrl)
				km := genMockKeymanager(10)
				keys, err := km.FetchValidatingPublicKeys(context.Background())
				assert.NoError(t, err)
				db := dbTest.SetupDB(t, keys)
				req := &ethpb.DoppelGangerRequest{ValidatorRequests: []*ethpb.DoppelGangerRequest_ValidatorRequest{}}
				resp := &ethpb.DoppelGangerResponse{Responses: []*ethpb.DoppelGangerResponse_ValidatorResponse{}}
				attLimit := 5
				for i, k := range keys {
					pkey := k
					for j := 0; j < attLimit; j++ {
						att := createAttestation(10+types.Epoch(j), 12+types.Epoch(j))
						rt, err := att.Data.HashTreeRoot()
						assert.NoError(t, err)
						assert.NoError(t, db.SaveAttestationForPubKey(context.Background(), pkey, rt, att))
						if j == attLimit-1 {
							req.ValidatorRequests = append(req.ValidatorRequests, &ethpb.DoppelGangerRequest_ValidatorRequest{PublicKey: pkey[:], Epoch: att.Data.Target.Epoch, SignedRoot: rt[:]})
						}
					}
					if i%3 == 0 {
						resp.Responses = append(resp.Responses, &ethpb.DoppelGangerResponse_ValidatorResponse{PublicKey: pkey[:], DuplicateExists: true})
					}
				}
				v := &validator{
					validatorClient: client,
					keyManager:      km,
					db:              db,
				}
				client.EXPECT().CheckDoppelGanger(
					gomock.Any(),                     // ctx
					&doppelGangerRequestMatcher{req}, // request
				).Return(resp, nil /*err*/)
				return v
			},
			err: "Duplicate instances exists in the network for validator keys",
		},
		{
			name: "no history exists",
			validatorSetter: func(t *testing.T) *validator {
				client := mock.NewMockBeaconNodeValidatorClient(ctrl)
				// Use only 1 key for deterministic order.
				km := genMockKeymanager(1)
				keys, err := km.FetchValidatingPublicKeys(context.Background())
				assert.NoError(t, err)
				db := dbTest.SetupDB(t, keys)
				resp := &ethpb.DoppelGangerResponse{Responses: []*ethpb.DoppelGangerResponse_ValidatorResponse{}}
				req := &ethpb.DoppelGangerRequest{ValidatorRequests: []*ethpb.DoppelGangerRequest_ValidatorRequest{}}
				for _, k := range keys {
					resp.Responses = append(resp.Responses, &ethpb.DoppelGangerResponse_ValidatorResponse{PublicKey: k[:], DuplicateExists: false})
					req.ValidatorRequests = append(req.ValidatorRequests, &ethpb.DoppelGangerRequest_ValidatorRequest{PublicKey: k[:], SignedRoot: make([]byte, 32), Epoch: 0})
				}
				v := &validator{
					validatorClient: client,
					keyManager:      km,
					db:              db,
				}
				client.EXPECT().CheckDoppelGanger(
					gomock.Any(), // ctx
					req,          // request
				).Return(resp, nil /*err*/)
				return v
			},
			err: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := tt.validatorSetter(t)
			if err := v.CheckDoppelGanger(context.Background()); tt.err != "" {
				assert.ErrorContains(t, tt.err, err)
			}
		})
	}
}

func TestValidatorAttestationsAreOrdered(t *testing.T) {
	km := genMockKeymanager(10)
	keys, err := km.FetchValidatingPublicKeys(context.Background())
	assert.NoError(t, err)
	db := dbTest.SetupDB(t, keys)

	k := keys[0]
	att := createAttestation(10, 14)
	rt, err := att.Data.HashTreeRoot()
	assert.NoError(t, err)
	assert.NoError(t, db.SaveAttestationForPubKey(context.Background(), k, rt, att))

	att = createAttestation(6, 8)
	rt, err = att.Data.HashTreeRoot()
	assert.NoError(t, err)
	assert.NoError(t, db.SaveAttestationForPubKey(context.Background(), k, rt, att))

	att = createAttestation(10, 12)
	rt, err = att.Data.HashTreeRoot()
	assert.NoError(t, err)
	assert.NoError(t, db.SaveAttestationForPubKey(context.Background(), k, rt, att))

	att = createAttestation(2, 3)
	rt, err = att.Data.HashTreeRoot()
	assert.NoError(t, err)
	assert.NoError(t, db.SaveAttestationForPubKey(context.Background(), k, rt, att))

	histories, err := db.AttestationHistoryForPubKey(context.Background(), k)
	assert.NoError(t, err)
	r := retrieveLatestRecord(histories)
	assert.Equal(t, r.Target, types.Epoch(14))
}

func createAttestation(source, target types.Epoch) *ethpb.IndexedAttestation {
	return &ethpb.IndexedAttestation{
		Data: &ethpb.AttestationData{
			Source: &ethpb.Checkpoint{
				Epoch: source,
				Root:  make([]byte, 32),
			},
			Target: &ethpb.Checkpoint{
				Epoch: target,
				Root:  make([]byte, 32),
			},
			BeaconBlockRoot: make([]byte, 32),
		},
		Signature: make([]byte, 96),
	}
}
