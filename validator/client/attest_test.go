package client

import (
	"context"
	"encoding/hex"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	types "github.com/prysmaticlabs/eth2-types"
	"github.com/prysmaticlabs/go-bitfield"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	ethpb "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/proto/eth/v1alpha1/wrapper"
	validatorpb "github.com/prysmaticlabs/prysm/proto/validator/accounts/v2"
	"github.com/prysmaticlabs/prysm/shared/bls"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/prysmaticlabs/prysm/shared/event"
	"github.com/prysmaticlabs/prysm/shared/featureconfig"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/testutil"
	"github.com/prysmaticlabs/prysm/shared/testutil/assert"
	"github.com/prysmaticlabs/prysm/shared/testutil/require"
	"github.com/prysmaticlabs/prysm/shared/timeutils"
	logTest "github.com/sirupsen/logrus/hooks/test"
	grpc "google.golang.org/grpc"
	"gopkg.in/d4l3k/messagediff.v1"
)

func TestRequestAttestation_ValidatorDutiesRequestFailure(t *testing.T) {
	hook := logTest.NewGlobal()
	validator, _, validatorKey, finish := setup(t)
	validator.duties = &ethpb.DutiesResponse{Duties: []*ethpb.DutiesResponse_Duty{}}
	defer finish()

	pubKey := [48]byte{}
	copy(pubKey[:], validatorKey.PublicKey().Marshal())
	validator.SubmitAttestation(context.Background(), 30, pubKey)
	require.LogsContain(t, hook, "Could not fetch validator assignment")
}

func TestAttestToBlockHead_SubmitAttestation_EmptyCommittee(t *testing.T) {
	hook := logTest.NewGlobal()

	validator, _, validatorKey, finish := setup(t)
	defer finish()
	pubKey := [48]byte{}
	copy(pubKey[:], validatorKey.PublicKey().Marshal())
	validator.duties = &ethpb.DutiesResponse{Duties: []*ethpb.DutiesResponse_Duty{
		{
			PublicKey:      validatorKey.PublicKey().Marshal(),
			CommitteeIndex: 0,
			Committee:      make([]types.ValidatorIndex, 0),
			ValidatorIndex: 0,
		}}}
	validator.SubmitAttestation(context.Background(), 0, pubKey)
	require.LogsContain(t, hook, "Empty committee")
}

func TestAttestToBlockHead_SubmitAttestation_RequestFailure(t *testing.T) {
	hook := logTest.NewGlobal()

	validator, m, validatorKey, finish := setup(t)
	defer finish()
	validator.duties = &ethpb.DutiesResponse{Duties: []*ethpb.DutiesResponse_Duty{
		{
			PublicKey:      validatorKey.PublicKey().Marshal(),
			CommitteeIndex: 5,
			Committee:      make([]types.ValidatorIndex, 111),
			ValidatorIndex: 0,
		}}}
	m.validatorClient.EXPECT().GetAttestationData(
		gomock.Any(), // ctx
		gomock.AssignableToTypeOf(&ethpb.AttestationDataRequest{}),
	).Return(&ethpb.AttestationData{
		BeaconBlockRoot: make([]byte, 32),
		Target:          &ethpb.Checkpoint{Root: make([]byte, 32)},
		Source:          &ethpb.Checkpoint{Root: make([]byte, 32)},
	}, nil)
	m.validatorClient.EXPECT().DomainData(
		gomock.Any(), // ctx
		gomock.Any(), // epoch2
	).Times(2).Return(&ethpb.DomainResponse{SignatureDomain: make([]byte, 32)}, nil /*err*/)
	m.validatorClient.EXPECT().ProposeAttestation(
		gomock.Any(), // ctx
		gomock.AssignableToTypeOf(&ethpb.Attestation{}),
	).Return(nil, errors.New("something went wrong"))

	pubKey := [48]byte{}
	copy(pubKey[:], validatorKey.PublicKey().Marshal())
	validator.SubmitAttestation(context.Background(), 30, pubKey)
	require.LogsContain(t, hook, "Could not submit attestation to beacon node")
}

func TestAttestToBlockHead_AttestsCorrectly(t *testing.T) {
	validator, m, validatorKey, finish := setup(t)
	defer finish()
	hook := logTest.NewGlobal()
	validatorIndex := types.ValidatorIndex(7)
	committee := []types.ValidatorIndex{0, 3, 4, 2, validatorIndex, 6, 8, 9, 10}
	pubKey := [48]byte{}
	copy(pubKey[:], validatorKey.PublicKey().Marshal())
	validator.duties = &ethpb.DutiesResponse{Duties: []*ethpb.DutiesResponse_Duty{
		{
			PublicKey:      validatorKey.PublicKey().Marshal(),
			CommitteeIndex: 5,
			Committee:      committee,
			ValidatorIndex: validatorIndex,
		},
	}}

	beaconBlockRoot := bytesutil.ToBytes32([]byte("A"))
	targetRoot := bytesutil.ToBytes32([]byte("B"))
	sourceRoot := bytesutil.ToBytes32([]byte("C"))
	m.validatorClient.EXPECT().GetAttestationData(
		gomock.Any(), // ctx
		gomock.AssignableToTypeOf(&ethpb.AttestationDataRequest{}),
	).Return(&ethpb.AttestationData{
		BeaconBlockRoot: beaconBlockRoot[:],
		Target:          &ethpb.Checkpoint{Root: targetRoot[:]},
		Source:          &ethpb.Checkpoint{Root: sourceRoot[:], Epoch: 3},
	}, nil)

	m.validatorClient.EXPECT().DomainData(
		gomock.Any(), // ctx
		gomock.Any(), // epoch
	).Times(2).Return(&ethpb.DomainResponse{SignatureDomain: make([]byte, 32)}, nil /*err*/)

	var generatedAttestation *ethpb.Attestation
	m.validatorClient.EXPECT().ProposeAttestation(
		gomock.Any(), // ctx
		gomock.AssignableToTypeOf(&ethpb.Attestation{}),
	).Do(func(_ context.Context, att *ethpb.Attestation, opts ...grpc.CallOption) {
		generatedAttestation = att
	}).Return(&ethpb.AttestResponse{}, nil /* error */)

	validator.SubmitAttestation(context.Background(), 30, pubKey)

	aggregationBitfield := bitfield.NewBitlist(uint64(len(committee)))
	aggregationBitfield.SetBitAt(4, true)
	expectedAttestation := &ethpb.Attestation{
		Data: &ethpb.AttestationData{
			BeaconBlockRoot: beaconBlockRoot[:],
			Target:          &ethpb.Checkpoint{Root: targetRoot[:]},
			Source:          &ethpb.Checkpoint{Root: sourceRoot[:], Epoch: 3},
		},
		AggregationBits: aggregationBitfield,
		Signature:       make([]byte, 96),
	}

	root, err := helpers.ComputeSigningRoot(expectedAttestation.Data, make([]byte, 32))
	require.NoError(t, err)

	sig, err := validator.keyManager.Sign(context.Background(), &validatorpb.SignRequest{
		PublicKey:   validatorKey.PublicKey().Marshal(),
		SigningRoot: root[:],
	})
	require.NoError(t, err)
	expectedAttestation.Signature = sig.Marshal()
	if !reflect.DeepEqual(generatedAttestation, expectedAttestation) {
		t.Errorf("Incorrectly attested head, wanted %v, received %v", expectedAttestation, generatedAttestation)
		diff, _ := messagediff.PrettyDiff(expectedAttestation, generatedAttestation)
		t.Log(diff)
	}
	require.LogsDoNotContain(t, hook, "Could not")
}

func TestAttestToBlockHead_BlocksDoubleAtt(t *testing.T) {
	hook := logTest.NewGlobal()
	validator, m, validatorKey, finish := setup(t)
	defer finish()
	validatorIndex := types.ValidatorIndex(7)
	committee := []types.ValidatorIndex{0, 3, 4, 2, validatorIndex, 6, 8, 9, 10}
	pubKey := [48]byte{}
	copy(pubKey[:], validatorKey.PublicKey().Marshal())
	validator.duties = &ethpb.DutiesResponse{Duties: []*ethpb.DutiesResponse_Duty{
		{
			PublicKey:      validatorKey.PublicKey().Marshal(),
			CommitteeIndex: 5,
			Committee:      committee,
			ValidatorIndex: validatorIndex,
		},
	}}
	beaconBlockRoot := bytesutil.ToBytes32([]byte("A"))
	targetRoot := bytesutil.ToBytes32([]byte("B"))
	sourceRoot := bytesutil.ToBytes32([]byte("C"))
	beaconBlockRoot2 := bytesutil.ToBytes32([]byte("D"))

	m.validatorClient.EXPECT().GetAttestationData(
		gomock.Any(), // ctx
		gomock.AssignableToTypeOf(&ethpb.AttestationDataRequest{}),
	).Return(&ethpb.AttestationData{
		BeaconBlockRoot: beaconBlockRoot[:],
		Target:          &ethpb.Checkpoint{Root: targetRoot[:], Epoch: 4},
		Source:          &ethpb.Checkpoint{Root: sourceRoot[:], Epoch: 3},
	}, nil)
	m.validatorClient.EXPECT().GetAttestationData(
		gomock.Any(), // ctx
		gomock.AssignableToTypeOf(&ethpb.AttestationDataRequest{}),
	).Return(&ethpb.AttestationData{
		BeaconBlockRoot: beaconBlockRoot2[:],
		Target:          &ethpb.Checkpoint{Root: targetRoot[:], Epoch: 4},
		Source:          &ethpb.Checkpoint{Root: sourceRoot[:], Epoch: 3},
	}, nil)
	m.validatorClient.EXPECT().DomainData(
		gomock.Any(), // ctx
		gomock.Any(), // epoch
	).Times(4).Return(&ethpb.DomainResponse{SignatureDomain: make([]byte, 32)}, nil /*err*/)

	m.validatorClient.EXPECT().ProposeAttestation(
		gomock.Any(), // ctx
		gomock.AssignableToTypeOf(&ethpb.Attestation{}),
	).Return(&ethpb.AttestResponse{AttestationDataRoot: make([]byte, 32)}, nil /* error */)

	validator.SubmitAttestation(context.Background(), 30, pubKey)
	validator.SubmitAttestation(context.Background(), 30, pubKey)
	require.LogsContain(t, hook, "Failed attestation slashing protection")
}

func TestAttestToBlockHead_BlocksSurroundAtt(t *testing.T) {
	hook := logTest.NewGlobal()
	validator, m, validatorKey, finish := setup(t)
	defer finish()
	validatorIndex := types.ValidatorIndex(7)
	committee := []types.ValidatorIndex{0, 3, 4, 2, validatorIndex, 6, 8, 9, 10}
	pubKey := [48]byte{}
	copy(pubKey[:], validatorKey.PublicKey().Marshal())
	validator.duties = &ethpb.DutiesResponse{Duties: []*ethpb.DutiesResponse_Duty{
		{
			PublicKey:      validatorKey.PublicKey().Marshal(),
			CommitteeIndex: 5,
			Committee:      committee,
			ValidatorIndex: validatorIndex,
		},
	}}
	beaconBlockRoot := bytesutil.ToBytes32([]byte("A"))
	targetRoot := bytesutil.ToBytes32([]byte("B"))
	sourceRoot := bytesutil.ToBytes32([]byte("C"))

	m.validatorClient.EXPECT().GetAttestationData(
		gomock.Any(), // ctx
		gomock.AssignableToTypeOf(&ethpb.AttestationDataRequest{}),
	).Return(&ethpb.AttestationData{
		BeaconBlockRoot: beaconBlockRoot[:],
		Target:          &ethpb.Checkpoint{Root: targetRoot[:], Epoch: 2},
		Source:          &ethpb.Checkpoint{Root: sourceRoot[:], Epoch: 1},
	}, nil)
	m.validatorClient.EXPECT().GetAttestationData(
		gomock.Any(), // ctx
		gomock.AssignableToTypeOf(&ethpb.AttestationDataRequest{}),
	).Return(&ethpb.AttestationData{
		BeaconBlockRoot: beaconBlockRoot[:],
		Target:          &ethpb.Checkpoint{Root: targetRoot[:], Epoch: 3},
		Source:          &ethpb.Checkpoint{Root: sourceRoot[:], Epoch: 0},
	}, nil)

	m.validatorClient.EXPECT().DomainData(
		gomock.Any(), // ctx
		gomock.Any(), // epoch
	).Times(4).Return(&ethpb.DomainResponse{SignatureDomain: make([]byte, 32)}, nil /*err*/)

	m.validatorClient.EXPECT().ProposeAttestation(
		gomock.Any(), // ctx
		gomock.AssignableToTypeOf(&ethpb.Attestation{}),
	).Return(&ethpb.AttestResponse{}, nil /* error */)

	validator.SubmitAttestation(context.Background(), 30, pubKey)
	validator.SubmitAttestation(context.Background(), 30, pubKey)
	require.LogsContain(t, hook, "Failed attestation slashing protection")
}

func TestAttestToBlockHead_BlocksSurroundedAtt(t *testing.T) {
	hook := logTest.NewGlobal()
	validator, m, validatorKey, finish := setup(t)
	defer finish()
	validatorIndex := types.ValidatorIndex(7)
	pubKey := [48]byte{}
	copy(pubKey[:], validatorKey.PublicKey().Marshal())
	committee := []types.ValidatorIndex{0, 3, 4, 2, validatorIndex, 6, 8, 9, 10}
	validator.duties = &ethpb.DutiesResponse{Duties: []*ethpb.DutiesResponse_Duty{
		{
			PublicKey:      validatorKey.PublicKey().Marshal(),
			CommitteeIndex: 5,
			Committee:      committee,
			ValidatorIndex: validatorIndex,
		},
	}}
	beaconBlockRoot := bytesutil.ToBytes32([]byte("A"))
	targetRoot := bytesutil.ToBytes32([]byte("B"))
	sourceRoot := bytesutil.ToBytes32([]byte("C"))

	m.validatorClient.EXPECT().GetAttestationData(
		gomock.Any(), // ctx
		gomock.AssignableToTypeOf(&ethpb.AttestationDataRequest{}),
	).Return(&ethpb.AttestationData{
		BeaconBlockRoot: beaconBlockRoot[:],
		Target:          &ethpb.Checkpoint{Root: targetRoot[:], Epoch: 3},
		Source:          &ethpb.Checkpoint{Root: sourceRoot[:], Epoch: 0},
	}, nil)

	m.validatorClient.EXPECT().DomainData(
		gomock.Any(), // ctx
		gomock.Any(), // epoch
	).Times(4).Return(&ethpb.DomainResponse{SignatureDomain: make([]byte, 32)}, nil /*err*/)

	m.validatorClient.EXPECT().ProposeAttestation(
		gomock.Any(), // ctx
		gomock.AssignableToTypeOf(&ethpb.Attestation{}),
	).Return(&ethpb.AttestResponse{}, nil /* error */)

	validator.SubmitAttestation(context.Background(), 30, pubKey)
	require.LogsDoNotContain(t, hook, failedAttLocalProtectionErr)

	m.validatorClient.EXPECT().GetAttestationData(
		gomock.Any(), // ctx
		gomock.AssignableToTypeOf(&ethpb.AttestationDataRequest{}),
	).Return(&ethpb.AttestationData{
		BeaconBlockRoot: bytesutil.PadTo([]byte("A"), 32),
		Target:          &ethpb.Checkpoint{Root: bytesutil.PadTo([]byte("B"), 32), Epoch: 2},
		Source:          &ethpb.Checkpoint{Root: bytesutil.PadTo([]byte("C"), 32), Epoch: 1},
	}, nil)

	validator.SubmitAttestation(context.Background(), 30, pubKey)
	require.LogsContain(t, hook, "Failed attestation slashing protection")
}

func TestAttestToBlockHead_DoesNotAttestBeforeDelay(t *testing.T) {
	validator, m, validatorKey, finish := setup(t)
	defer finish()

	pubKey := [48]byte{}
	copy(pubKey[:], validatorKey.PublicKey().Marshal())
	validator.genesisTime = uint64(timeutils.Now().Unix())
	m.validatorClient.EXPECT().GetDuties(
		gomock.Any(), // ctx
		gomock.AssignableToTypeOf(&ethpb.DutiesRequest{}),
		gomock.Any(),
	).Times(0)

	m.validatorClient.EXPECT().GetAttestationData(
		gomock.Any(), // ctx
		gomock.AssignableToTypeOf(&ethpb.AttestationDataRequest{}),
	).Times(0)

	m.validatorClient.EXPECT().ProposeAttestation(
		gomock.Any(), // ctx
		gomock.AssignableToTypeOf(&ethpb.Attestation{}),
	).Return(&ethpb.AttestResponse{}, nil /* error */).Times(0)

	timer := time.NewTimer(1 * time.Second)
	go validator.SubmitAttestation(context.Background(), 0, pubKey)
	<-timer.C
}

func TestAttestToBlockHead_DoesAttestAfterDelay(t *testing.T) {
	validator, m, validatorKey, finish := setup(t)
	defer finish()

	var wg sync.WaitGroup
	wg.Add(1)
	defer wg.Wait()

	validator.genesisTime = uint64(timeutils.Now().Unix())
	validatorIndex := types.ValidatorIndex(5)
	committee := []types.ValidatorIndex{0, 3, 4, 2, validatorIndex, 6, 8, 9, 10}
	pubKey := [48]byte{}
	copy(pubKey[:], validatorKey.PublicKey().Marshal())
	validator.duties = &ethpb.DutiesResponse{Duties: []*ethpb.DutiesResponse_Duty{
		{
			PublicKey:      validatorKey.PublicKey().Marshal(),
			CommitteeIndex: 5,
			Committee:      committee,
			ValidatorIndex: validatorIndex,
		}}}

	m.validatorClient.EXPECT().GetAttestationData(
		gomock.Any(), // ctx
		gomock.AssignableToTypeOf(&ethpb.AttestationDataRequest{}),
	).Return(&ethpb.AttestationData{
		BeaconBlockRoot: bytesutil.PadTo([]byte("A"), 32),
		Target:          &ethpb.Checkpoint{Root: bytesutil.PadTo([]byte("B"), 32)},
		Source:          &ethpb.Checkpoint{Root: bytesutil.PadTo([]byte("C"), 32), Epoch: 3},
	}, nil).Do(func(arg0, arg1 interface{}, arg2 ...grpc.CallOption) {
		wg.Done()
	})

	m.validatorClient.EXPECT().DomainData(
		gomock.Any(), // ctx
		gomock.Any(), // epoch
	).Times(2).Return(&ethpb.DomainResponse{SignatureDomain: make([]byte, 32)}, nil /*err*/)

	m.validatorClient.EXPECT().ProposeAttestation(
		gomock.Any(), // ctx
		gomock.Any(),
	).Return(&ethpb.AttestResponse{}, nil).Times(1)

	validator.SubmitAttestation(context.Background(), 0, pubKey)
}

func TestAttestToBlockHead_CorrectBitfieldLength(t *testing.T) {
	validator, m, validatorKey, finish := setup(t)
	defer finish()
	validatorIndex := types.ValidatorIndex(2)
	committee := []types.ValidatorIndex{0, 3, 4, 2, validatorIndex, 6, 8, 9, 10}
	pubKey := [48]byte{}
	copy(pubKey[:], validatorKey.PublicKey().Marshal())
	validator.duties = &ethpb.DutiesResponse{Duties: []*ethpb.DutiesResponse_Duty{
		{
			PublicKey:      validatorKey.PublicKey().Marshal(),
			CommitteeIndex: 5,
			Committee:      committee,
			ValidatorIndex: validatorIndex,
		}}}
	m.validatorClient.EXPECT().GetAttestationData(
		gomock.Any(), // ctx
		gomock.AssignableToTypeOf(&ethpb.AttestationDataRequest{}),
	).Return(&ethpb.AttestationData{
		Target:          &ethpb.Checkpoint{Root: bytesutil.PadTo([]byte("B"), 32)},
		Source:          &ethpb.Checkpoint{Root: bytesutil.PadTo([]byte("C"), 32), Epoch: 3},
		BeaconBlockRoot: make([]byte, 32),
	}, nil)

	m.validatorClient.EXPECT().DomainData(
		gomock.Any(), // ctx
		gomock.Any(), // epoch
	).Times(2).Return(&ethpb.DomainResponse{SignatureDomain: make([]byte, 32)}, nil /*err*/)

	var generatedAttestation *ethpb.Attestation
	m.validatorClient.EXPECT().ProposeAttestation(
		gomock.Any(), // ctx
		gomock.AssignableToTypeOf(&ethpb.Attestation{}),
	).Do(func(_ context.Context, att *ethpb.Attestation, arg2 ...grpc.CallOption) {
		generatedAttestation = att
	}).Return(&ethpb.AttestResponse{}, nil /* error */)

	validator.SubmitAttestation(context.Background(), 30, pubKey)

	assert.Equal(t, 2, len(generatedAttestation.AggregationBits))
}

func TestSignAttestation(t *testing.T) {
	validator, m, _, finish := setup(t)
	defer finish()

	secretKey, err := bls.SecretKeyFromBytes(bytesutil.PadTo([]byte{1}, 32))
	require.NoError(t, err, "Failed to generate key from bytes")
	publicKey := secretKey.PublicKey()
	wantedFork := &pb.Fork{
		PreviousVersion: []byte{'a', 'b', 'c', 'd'},
		CurrentVersion:  []byte{'d', 'e', 'f', 'f'},
		Epoch:           0,
	}
	genesisValidatorRoot := [32]byte{0x01, 0x02}
	attesterDomain, err := helpers.Domain(wantedFork, 0, params.BeaconConfig().DomainBeaconAttester, genesisValidatorRoot[:])
	require.NoError(t, err)
	m.validatorClient.EXPECT().
		DomainData(gomock.Any(), gomock.Any()).
		Return(&ethpb.DomainResponse{SignatureDomain: attesterDomain}, nil)
	ctx := context.Background()
	att := testutil.NewAttestation()
	att.Data.Source.Epoch = 100
	att.Data.Target.Epoch = 200
	att.Data.Slot = 999
	att.Data.BeaconBlockRoot = bytesutil.PadTo([]byte("blockRoot"), 32)
	var pubKey [48]byte
	copy(pubKey[:], publicKey.Marshal())
	km := &mockKeymanager{
		keysMap: map[[48]byte]bls.SecretKey{
			pubKey: secretKey,
		},
	}
	validator.keyManager = km
	sig, sr, err := validator.signAtt(ctx, pubKey, att.Data)
	require.NoError(t, err, "%x,%x,%v", sig, sr, err)
	require.Equal(t, "b6a60f8497bd328908be83634d045"+
		"dd7a32f5e246b2c4031fc2f316983f362e36fc27fd3d6d5a2b15"+
		"b4dbff38804ffb10b1719b7ebc54e9cbf3293fd37082bc0fc91f"+
		"79d70ce5b04ff13de3c8e10bb41305bfdbe921a43792c12624f2"+
		"25ee865", hex.EncodeToString(sig))
	// proposer domain
	require.DeepEqual(t, "02bbdb88056d6cbafd6e94575540"+
		"e74b8cf2c0f2c1b79b8e17e7b21ed1694305", hex.EncodeToString(sr[:]))
}

func TestServer_WaitToSlotOneThird_CanWait(t *testing.T) {
	currentTime := uint64(time.Now().Unix())
	currentSlot := types.Slot(4)
	genesisTime := currentTime - uint64(currentSlot.Mul(params.BeaconConfig().SecondsPerSlot))

	v := &validator{
		genesisTime: genesisTime,
		blockFeed:   new(event.Feed),
	}

	timeToSleep := params.BeaconConfig().SecondsPerSlot / 3
	oneThird := currentTime + timeToSleep
	v.waitOneThirdOrValidBlock(context.Background(), currentSlot)

	if oneThird != uint64(time.Now().Unix()) {
		t.Errorf("Wanted %d time for slot one third but got %d", oneThird, currentTime)
	}
}

func TestServer_WaitToSlotOneThird_SameReqSlot(t *testing.T) {
	currentTime := uint64(time.Now().Unix())
	currentSlot := types.Slot(4)
	genesisTime := currentTime - uint64(currentSlot.Mul(params.BeaconConfig().SecondsPerSlot))

	v := &validator{
		genesisTime:      genesisTime,
		blockFeed:        new(event.Feed),
		highestValidSlot: currentSlot,
	}

	v.waitOneThirdOrValidBlock(context.Background(), currentSlot)

	if currentTime != uint64(time.Now().Unix()) {
		t.Errorf("Wanted %d time for slot one third but got %d", uint64(time.Now().Unix()), currentTime)
	}
}

func TestServer_WaitToSlotOneThird_ReceiveBlockSlot(t *testing.T) {
	resetCfg := featureconfig.InitWithReset(&featureconfig.Flags{AttestTimely: true})
	defer resetCfg()

	currentTime := uint64(time.Now().Unix())
	currentSlot := types.Slot(4)
	genesisTime := currentTime - uint64(currentSlot.Mul(params.BeaconConfig().SecondsPerSlot))

	v := &validator{
		genesisTime: genesisTime,
		blockFeed:   new(event.Feed),
	}

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		time.Sleep(100 * time.Millisecond)
		v.blockFeed.Send(wrapper.WrappedPhase0SignedBeaconBlock(
			&ethpb.SignedBeaconBlock{
				Block: &ethpb.BeaconBlock{Slot: currentSlot},
			}))
		wg.Done()
	}()

	v.waitOneThirdOrValidBlock(context.Background(), currentSlot)

	if currentTime != uint64(time.Now().Unix()) {
		t.Errorf("Wanted %d time for slot one third but got %d", uint64(time.Now().Unix()), currentTime)
	}
}
