package client

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/gogo/protobuf/proto"
	ptypes "github.com/gogo/protobuf/types"
	"github.com/golang/mock/gomock"
	pbp2p "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/rpc/v1"
	"github.com/prysmaticlabs/prysm/shared/bitutil"
	"github.com/prysmaticlabs/prysm/shared/forkutil"
	"github.com/prysmaticlabs/prysm/shared/hashutil"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/testutil"
	logTest "github.com/sirupsen/logrus/hooks/test"
)

func TestAttestToBlockHead_ValidatorIndexRequestFailure(t *testing.T) {
	hook := logTest.NewGlobal()

	validator, m, finish := setup(t)
	defer finish()
	m.validatorClient.EXPECT().ValidatorIndex(
		gomock.Any(), // ctx
		gomock.AssignableToTypeOf(&pb.ValidatorIndexRequest{}),
	).Return(nil /* Validator Index Response*/, errors.New("something bad happened"))

	validator.AttestToBlockHead(context.Background(), 30)
	testutil.AssertLogsContain(t, hook, "Could not fetch validator index")
}

func TestAttestToBlockHead_ValidatorCommitteeAtSlotFailure(t *testing.T) {
	hook := logTest.NewGlobal()

	validator, m, finish := setup(t)
	defer finish()
	m.validatorClient.EXPECT().ValidatorIndex(
		gomock.Any(), // ctx
		gomock.AssignableToTypeOf(&pb.ValidatorIndexRequest{}),
	).Return(&pb.ValidatorIndexResponse{Index: 5}, nil)
	m.validatorClient.EXPECT().CommitteeAssignment(
		gomock.Any(), // ctx
		gomock.Any(),
	).Return(nil, errors.New("something went wrong"))

	validator.AttestToBlockHead(context.Background(), 30+params.BeaconConfig().GenesisSlot)
	testutil.AssertLogsContain(t, hook, "Could not fetch crosslink committees at slot 30")
}

func TestAttestToBlockHead_AttestationDataAtSlotFailure(t *testing.T) {
	hook := logTest.NewGlobal()

	validator, m, finish := setup(t)
	defer finish()
	m.validatorClient.EXPECT().ValidatorIndex(
		gomock.Any(), // ctx
		gomock.AssignableToTypeOf(&pb.ValidatorIndexRequest{}),
	).Return(&pb.ValidatorIndexResponse{Index: 5}, nil)
	m.validatorClient.EXPECT().CommitteeAssignment(
		gomock.Any(), // ctx
		gomock.AssignableToTypeOf(&pb.ValidatorEpochAssignmentsRequest{}),
	).Return(&pb.CommitteeAssignmentResponse{
		Shard: 5,
	}, nil)
	m.attesterClient.EXPECT().AttestationDataAtSlot(
		gomock.Any(), // ctx
		gomock.AssignableToTypeOf(&pb.AttestationDataRequest{}),
	).Return(nil, errors.New("something went wrong"))

	validator.AttestToBlockHead(context.Background(), 30)
	testutil.AssertLogsContain(t, hook, "Could not fetch necessary info to produce attestation")
}

func TestAttestToBlockHead_AttestHeadRequestFailure(t *testing.T) {
	hook := logTest.NewGlobal()

	validator, m, finish := setup(t)
	defer finish()
	m.validatorClient.EXPECT().ValidatorIndex(
		gomock.Any(), // ctx
		gomock.AssignableToTypeOf(&pb.ValidatorIndexRequest{}),
	).Return(&pb.ValidatorIndexResponse{
		Index: 0,
	}, nil)
	m.validatorClient.EXPECT().CommitteeAssignment(
		gomock.Any(), // ctx
		gomock.AssignableToTypeOf(&pb.ValidatorEpochAssignmentsRequest{}),
	).Return(&pb.CommitteeAssignmentResponse{
		Shard:     5,
		Committee: make([]uint64, 111),
	}, nil)
	m.attesterClient.EXPECT().AttestationDataAtSlot(
		gomock.Any(), // ctx
		gomock.AssignableToTypeOf(&pb.AttestationDataRequest{}),
	).Return(&pb.AttestationDataResponse{
		BeaconBlockRootHash32:    []byte{},
		EpochBoundaryRootHash32:  []byte{},
		JustifiedBlockRootHash32: []byte{},
		LatestCrosslink:          &pbp2p.Crosslink{},
		JustifiedEpoch:           0,
	}, nil)
	m.beaconClient.EXPECT().ForkData(
		gomock.Any(), // ctx
		gomock.Eq(&ptypes.Empty{}),
	).Return(&pbp2p.Fork{
		Epoch:           params.BeaconConfig().GenesisEpoch,
		CurrentVersion:  0,
		PreviousVersion: 0,
	}, nil /*err*/)
	m.attesterClient.EXPECT().AttestHead(
		gomock.Any(), // ctx
		gomock.AssignableToTypeOf(&pbp2p.Attestation{}),
	).Return(nil, errors.New("something went wrong"))

	validator.AttestToBlockHead(context.Background(), 30)
	testutil.AssertLogsContain(t, hook, "Could not submit attestation to beacon node")
}

func TestAttestToBlockHead_AttestsCorrectly(t *testing.T) {
	hook := logTest.NewGlobal()

	validator, m, finish := setup(t)
	defer finish()
	validatorIndex := uint64(4)
	committee := []uint64{0, 3, 4, 2, validatorIndex, 6, 8, 9, 10}
	m.validatorClient.EXPECT().ValidatorIndex(
		gomock.Any(), // ctx
		gomock.AssignableToTypeOf(&pb.ValidatorIndexRequest{}),
	).Return(&pb.ValidatorIndexResponse{
		Index: uint64(validatorIndex),
	}, nil)
	m.validatorClient.EXPECT().CommitteeAssignment(
		gomock.Any(), // ctx
		gomock.AssignableToTypeOf(&pb.ValidatorEpochAssignmentsRequest{}),
	).Return(&pb.CommitteeAssignmentResponse{
		Shard:     5,
		Committee: committee,
	}, nil)
	m.beaconClient.EXPECT().ForkData(
		gomock.Any(), // ctx
		gomock.Eq(&ptypes.Empty{}),
	).Return(&pbp2p.Fork{
		Epoch:           params.BeaconConfig().GenesisEpoch,
		CurrentVersion:  0,
		PreviousVersion: 0,
	}, nil /*err*/)
	m.attesterClient.EXPECT().AttestationDataAtSlot(
		gomock.Any(), // ctx
		gomock.AssignableToTypeOf(&pb.AttestationDataRequest{}),
	).Return(&pb.AttestationDataResponse{
		BeaconBlockRootHash32:    []byte("A"),
		EpochBoundaryRootHash32:  []byte("B"),
		JustifiedBlockRootHash32: []byte("C"),
		LatestCrosslink:          &pbp2p.Crosslink{CrosslinkDataRootHash32: []byte{'D'}},
		JustifiedEpoch:           3,
	}, nil)

	var generatedAttestation *pbp2p.Attestation
	m.attesterClient.EXPECT().AttestHead(
		gomock.Any(), // ctx
		gomock.AssignableToTypeOf(&pbp2p.Attestation{}),
	).Do(func(_ context.Context, att *pbp2p.Attestation) {
		generatedAttestation = att
	}).Return(&pb.AttestResponse{}, nil /* error */)

	validator.AttestToBlockHead(context.Background(), 30)

	// Validator index is at index 4 in the mocked committee defined in this test.
	expectedAttestation := &pbp2p.Attestation{
		Data: &pbp2p.AttestationData{
			Slot:                     30,
			Shard:                    5,
			BeaconBlockRootHash32:    []byte("A"),
			EpochBoundaryRootHash32:  []byte("B"),
			JustifiedBlockRootHash32: []byte("C"),
			LatestCrosslink:          &pbp2p.Crosslink{CrosslinkDataRootHash32: []byte{'D'}},
			CrosslinkDataRootHash32:  params.BeaconConfig().ZeroHash[:],
			JustifiedEpoch:           3,
		},
		CustodyBitfield: make([]byte, (len(committee)+7)/8),
	}

	aggregationBitfield := bitutil.SetBitfield(int(validatorIndex))
	expectedAttestation.AggregationBitfield = aggregationBitfield

	// Retrieve the current fork data from the beacon node.
	fork := &pbp2p.Fork{
		Epoch:           params.BeaconConfig().GenesisEpoch,
		CurrentVersion:  0,
		PreviousVersion: 0,
	}

	epoch := 30 / params.BeaconConfig().SlotsPerEpoch
	attestationHash, err := hashutil.HashProto(&pbp2p.AttestationDataAndCustodyBit{
		Data:       expectedAttestation.Data,
		CustodyBit: true,
	})
	if err != nil {
		log.Fatalf("Could not hash attestation data: %v", err)
		return
	}
	domain := forkutil.DomainVersion(fork, epoch, params.BeaconConfig().DomainAttestation)

	expectedAttestation.AggregateSignature = validator.key.SecretKey.Sign(attestationHash[:], domain).Marshal()

	if !proto.Equal(generatedAttestation, expectedAttestation) {
		t.Errorf("Incorrectly attested head, wanted %v, received %v", expectedAttestation, generatedAttestation)
	}
	testutil.AssertLogsContain(t, hook, "Submitted attestation successfully")
}

func TestAttestToBlockHead_DoesNotAttestBeforeDelay(t *testing.T) {
	validator, m, finish := setup(t)
	defer finish()

	var wg sync.WaitGroup
	wg.Add(3)
	defer wg.Wait()

	validator.genesisTime = uint64(time.Now().Unix())
	validatorIndex := uint64(5)
	committee := []uint64{0, 3, 4, 2, validatorIndex, 6, 8, 9, 10}
	m.validatorClient.EXPECT().CommitteeAssignment(
		gomock.Any(), // ctx
		gomock.AssignableToTypeOf(&pb.ValidatorEpochAssignmentsRequest{}),
		gomock.Any(), // ctx
	).Return(&pb.CommitteeAssignmentResponse{
		Shard:     5,
		Committee: committee,
	}, nil).Do(func(arg0, arg1 interface{}) {
		wg.Done()
	})

	m.attesterClient.EXPECT().AttestationDataAtSlot(
		gomock.Any(), // ctx
		gomock.AssignableToTypeOf(&pb.AttestationDataRequest{}),
	).Return(&pb.AttestationDataResponse{
		BeaconBlockRootHash32:    []byte("A"),
		EpochBoundaryRootHash32:  []byte("B"),
		JustifiedBlockRootHash32: []byte("C"),
		LatestCrosslink:          &pbp2p.Crosslink{CrosslinkDataRootHash32: []byte{'D'}},
		JustifiedEpoch:           3,
	}, nil).Do(func(arg0, arg1 interface{}) {
		wg.Done()
	})

	m.validatorClient.EXPECT().ValidatorIndex(
		gomock.Any(), // ctx
		gomock.AssignableToTypeOf(&pb.ValidatorIndexRequest{}),
	).Return(&pb.ValidatorIndexResponse{
		Index: uint64(validatorIndex),
	}, nil).Do(func(arg0, arg1 interface{}) {
		wg.Done()
	})

	m.beaconClient.EXPECT().ForkData(
		gomock.Any(), // ctx
		gomock.Eq(&ptypes.Empty{}),
	).Return(&pbp2p.Fork{
		Epoch:           params.BeaconConfig().GenesisEpoch,
		CurrentVersion:  0,
		PreviousVersion: 0,
	}, nil /*err*/).Times(1)

	m.attesterClient.EXPECT().AttestHead(
		gomock.Any(), // ctx
		gomock.AssignableToTypeOf(&pbp2p.Attestation{}),
	).Return(&pb.AttestResponse{}, nil /* error */).Times(0)

	delay = 2
	timer := time.NewTimer(time.Duration(1 * time.Second))
	go validator.AttestToBlockHead(context.Background(), 0)
	<-timer.C
}

func TestAttestToBlockHead_DoesAttestAfterDelay(t *testing.T) {
	validator, m, finish := setup(t)
	defer finish()

	var wg sync.WaitGroup
	wg.Add(3)
	defer wg.Wait()

	validator.genesisTime = uint64(time.Now().Unix())
	validatorIndex := uint64(5)
	committee := []uint64{0, 3, 4, 2, validatorIndex, 6, 8, 9, 10}
	m.validatorClient.EXPECT().CommitteeAssignment(
		gomock.Any(), // ctx
		gomock.AssignableToTypeOf(&pb.ValidatorEpochAssignmentsRequest{}),
	).Return(&pb.CommitteeAssignmentResponse{
		Shard:     5,
		Committee: committee,
	}, nil).Do(func(arg0, arg1 interface{}) {
		wg.Done()
	})

	m.attesterClient.EXPECT().AttestationDataAtSlot(
		gomock.Any(), // ctx
		gomock.AssignableToTypeOf(&pb.AttestationDataRequest{}),
	).Return(&pb.AttestationDataResponse{
		BeaconBlockRootHash32:    []byte("A"),
		EpochBoundaryRootHash32:  []byte("B"),
		JustifiedBlockRootHash32: []byte("C"),
		LatestCrosslink:          &pbp2p.Crosslink{CrosslinkDataRootHash32: []byte{'D'}},
		JustifiedEpoch:           3,
	}, nil).Do(func(arg0, arg1 interface{}) {
		wg.Done()
	})

	m.validatorClient.EXPECT().ValidatorIndex(
		gomock.Any(), // ctx
		gomock.AssignableToTypeOf(&pb.ValidatorIndexRequest{}),
	).Return(&pb.ValidatorIndexResponse{
		Index: uint64(validatorIndex),
	}, nil).Do(func(arg0, arg1 interface{}) {
		wg.Done()
	})

	m.beaconClient.EXPECT().ForkData(
		gomock.Any(), // ctx
		gomock.Eq(&ptypes.Empty{}),
	).Return(&pbp2p.Fork{
		Epoch:           params.BeaconConfig().GenesisEpoch,
		CurrentVersion:  0,
		PreviousVersion: 0,
	}, nil /*err*/).Times(1)

	m.attesterClient.EXPECT().AttestHead(
		gomock.Any(), // ctx
		gomock.Any(),
	).Return(&pb.AttestResponse{}, nil).Times(1)

	delay = 0
	validator.AttestToBlockHead(context.Background(), 0)
}
