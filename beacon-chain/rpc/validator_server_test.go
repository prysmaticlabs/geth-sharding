package rpc

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	b "github.com/prysmaticlabs/prysm/beacon-chain/core/blocks"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/state"
	"github.com/prysmaticlabs/prysm/beacon-chain/internal"
	pbp2p "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/rpc/v1"
	"github.com/prysmaticlabs/prysm/shared/params"
)

func genesisState(validators uint64) (*pbp2p.BeaconState, error) {
	genesisTime := time.Unix(0, 0).Unix()
	deposits := make([]*pbp2p.Deposit, validators)
	for i := 0; i < len(deposits); i++ {
		var pubKey [96]byte
		copy(pubKey[:], []byte(strconv.Itoa(i)))
		depositInput := &pbp2p.DepositInput{
			Pubkey: pubKey[:],
		}
		depositData, err := helpers.EncodeDepositData(
			depositInput,
			params.BeaconConfig().MaxDepositAmount,
			genesisTime,
		)
		if err != nil {
			return nil, err
		}
		deposits[i] = &pbp2p.Deposit{DepositData: depositData}
	}
	return state.GenesisBeaconState(deposits, uint64(genesisTime), nil)
}

func TestValidatorIndex_OK(t *testing.T) {
	db := internal.SetupDB(t)
	defer internal.TeardownDB(t, db)

	pubKey := []byte{'A'}
	if err := db.SaveValidatorIndex(pubKey, 0); err != nil {
		t.Fatalf("Could not save validator index: %v", err)
	}

	validatorServer := &ValidatorServer{
		beaconDB: db,
	}

	req := &pb.ValidatorIndexRequest{
		PublicKey: pubKey,
	}
	if _, err := validatorServer.ValidatorIndex(context.Background(), req); err != nil {
		t.Errorf("Could not get validator index: %v", err)
	}
}

func TestValidatorIndex_InStateNotInDB(t *testing.T) {
	db := internal.SetupDB(t)
	defer internal.TeardownDB(t, db)

	pubKey := []byte{'A'}

	// Wanted validator with public key 'A' is in index '1'.
	s := &pbp2p.BeaconState{
		ValidatorRegistry: []*pbp2p.Validator{{Pubkey: []byte{0}}, {Pubkey: []byte{'A'}}, {Pubkey: []byte{'B'}}},
	}

	if err := db.SaveState(context.Background(), s); err != nil {
		t.Fatal(err)
	}

	validatorServer := &ValidatorServer{
		beaconDB: db,
	}

	req := &pb.ValidatorIndexRequest{
		PublicKey: pubKey,
	}

	// Verify index can be retrieved from state when it's not saved in DB.
	res, err := validatorServer.ValidatorIndex(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if res.Index != 1 {
		t.Errorf("Wanted index 1 got %d", res.Index)
	}

	// Verify index is also saved in DB.
	idx, err := validatorServer.beaconDB.ValidatorIndex(pubKey)
	if err != nil {
		t.Fatal(err)
	}
	if idx != 1 {
		t.Errorf("Wanted index 1 in DB got %d", res.Index)
	}
}

func TestNextEpochCommitteeAssignment_WrongPubkeyLength(t *testing.T) {
	db := internal.SetupDB(t)
	ctx := context.Background()
	defer internal.TeardownDB(t, db)
	beaconState, err := genesisState(8)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.SaveState(context.Background(), beaconState); err != nil {
		t.Fatal(err)
	}
	block := b.NewGenesisBlock([]byte{})
	if err := db.SaveBlock(block); err != nil {
		t.Fatalf("Could not save block: %v", err)
	}
	if err := db.UpdateChainHead(ctx, block, beaconState); err != nil {
		t.Fatalf("Could not update head: %v", err)
	}
	validatorServer := &ValidatorServer{
		beaconDB: db,
	}
	req := &pb.CommitteeAssignmentsRequest{
		PublicKeys: [][]byte{{1}},
		EpochStart: params.BeaconConfig().GenesisEpoch,
	}
	want := fmt.Sprintf("expected public key to have length %d", params.BeaconConfig().BLSPubkeyLength)
	if _, err := validatorServer.CommitteeAssignment(context.Background(), req); err != nil && !strings.Contains(err.Error(), want) {
		t.Errorf("Expected %v, received %v", want, err)
	}
}

func TestNextEpochCommitteeAssignment_CantFindValidatorIdx(t *testing.T) {
	db := internal.SetupDB(t)
	defer internal.TeardownDB(t, db)
	ctx := context.Background()
	genesis := b.NewGenesisBlock([]byte{})
	if err := db.SaveBlock(genesis); err != nil {
		t.Fatalf("Could not save genesis block: %v", err)
	}
	state, err := genesisState(params.BeaconConfig().DepositsForChainStart)
	if err != nil {
		t.Fatalf("Could not setup genesis state: %v", err)
	}
	db.UpdateChainHead(ctx, genesis, state)
	vs := &ValidatorServer{
		beaconDB: db,
	}

	pubKey := make([]byte, 96)
	req := &pb.CommitteeAssignmentsRequest{
		PublicKeys: [][]byte{pubKey},
		EpochStart: params.BeaconConfig().GenesisEpoch,
	}
	want := fmt.Sprintf("validator %#x does not exist", req.PublicKeys[0])
	if _, err := vs.CommitteeAssignment(ctx, req); err != nil && !strings.Contains(err.Error(), want) {
		t.Errorf("Expected %v, received %v", want, err)
	}
}

func TestCommitteeAssignment_OK(t *testing.T) {
	db := internal.SetupDB(t)
	defer internal.TeardownDB(t, db)
	ctx := context.Background()

	genesis := b.NewGenesisBlock([]byte{})
	if err := db.SaveBlock(genesis); err != nil {
		t.Fatalf("Could not save genesis block: %v", err)
	}
	state, err := genesisState(params.BeaconConfig().DepositsForChainStart)
	if err != nil {
		t.Fatalf("Could not setup genesis state: %v", err)
	}
	if err := db.UpdateChainHead(ctx, genesis, state); err != nil {
		t.Fatalf("Could not save genesis state: %v", err)
	}
	var wg sync.WaitGroup
	numOfValidators := int(params.BeaconConfig().DepositsForChainStart)
	errs := make(chan error, numOfValidators)
	for i := 0; i < numOfValidators; i++ {
		pubKeyBuf := make([]byte, params.BeaconConfig().BLSPubkeyLength)
		copy(pubKeyBuf[:], []byte(strconv.Itoa(i)))
		wg.Add(1)
		go func(index int) {
			errs <- db.SaveValidatorIndexBatch(pubKeyBuf, index)
			wg.Done()
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("Could not save validator index: %v", err)
		}
	}

	vs := &ValidatorServer{
		beaconDB: db,
	}

	pubKeyBuf := make([]byte, params.BeaconConfig().BLSPubkeyLength)
	copy(pubKeyBuf[:], []byte(strconv.FormatUint(0, 10)))
	// Test the first validator in registry.
	req := &pb.CommitteeAssignmentsRequest{
		PublicKeys: [][]byte{pubKeyBuf},
		EpochStart: params.BeaconConfig().GenesisSlot,
	}
	res, err := vs.CommitteeAssignment(context.Background(), req)
	if err != nil {
		t.Fatalf("Could not call epoch committee assignment %v", err)
	}
	if res.Assignment[0].Shard >= params.BeaconConfig().ShardCount {
		t.Errorf("Assigned shard %d can't be higher than %d",
			res.Assignment[0].Shard, params.BeaconConfig().ShardCount)
	}
	if res.Assignment[0].Slot > state.Slot+params.BeaconConfig().SlotsPerEpoch {
		t.Errorf("Assigned slot %d can't be higher than %d",
			res.Assignment[0].Slot, state.Slot+params.BeaconConfig().SlotsPerEpoch)
	}

	// Test the last validator in registry.
	lastValidatorIndex := params.BeaconConfig().DepositsForChainStart - 1
	pubKeyBuf = make([]byte, params.BeaconConfig().BLSPubkeyLength)
	copy(pubKeyBuf[:], []byte(strconv.FormatUint(lastValidatorIndex, 10)))
	req = &pb.CommitteeAssignmentsRequest{
		PublicKeys: [][]byte{pubKeyBuf},
		EpochStart: params.BeaconConfig().GenesisSlot,
	}
	res, err = vs.CommitteeAssignment(context.Background(), req)
	if err != nil {
		t.Fatalf("Could not call epoch committee assignment %v", err)
	}
	if res.Assignment[0].Shard >= params.BeaconConfig().ShardCount {
		t.Errorf("Assigned shard %d can't be higher than %d",
			res.Assignment[0].Shard, params.BeaconConfig().ShardCount)
	}
	if res.Assignment[0].Slot > state.Slot+params.BeaconConfig().SlotsPerEpoch {
		t.Errorf("Assigned slot %d can't be higher than %d",
			res.Assignment[0].Slot, state.Slot+params.BeaconConfig().SlotsPerEpoch)
	}
}

func TestCommitteeAssignment_multipleKeys_OK(t *testing.T) {
	db := internal.SetupDB(t)
	defer internal.TeardownDB(t, db)
	ctx := context.Background()

	genesis := b.NewGenesisBlock([]byte{})
	if err := db.SaveBlock(genesis); err != nil {
		t.Fatalf("Could not save genesis block: %v", err)
	}
	state, err := genesisState(params.BeaconConfig().DepositsForChainStart)
	if err != nil {
		t.Fatalf("Could not setup genesis state: %v", err)
	}
	if err := db.UpdateChainHead(ctx, genesis, state); err != nil {
		t.Fatalf("Could not save genesis state: %v", err)
	}
	var wg sync.WaitGroup
	numOfValidators := int(params.BeaconConfig().DepositsForChainStart)
	errs := make(chan error, numOfValidators)
	for i := 0; i < numOfValidators; i++ {
		pubKeyBuf := make([]byte, params.BeaconConfig().BLSPubkeyLength)
		copy(pubKeyBuf[:], []byte(strconv.Itoa(i)))
		wg.Add(1)
		go func(index int) {
			errs <- db.SaveValidatorIndexBatch(pubKeyBuf, index)
			wg.Done()
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("Could not save validator index: %v", err)
		}
	}

	vs := &ValidatorServer{
		beaconDB: db,
	}

	pubKeyBuf0 := make([]byte, params.BeaconConfig().BLSPubkeyLength)
	copy(pubKeyBuf0[:], []byte(strconv.Itoa(0)))
	pubKeyBuf1 := make([]byte, params.BeaconConfig().BLSPubkeyLength)
	copy(pubKeyBuf1[:], []byte(strconv.Itoa(1)))
	// Test the first validator in registry.
	req := &pb.CommitteeAssignmentsRequest{
		PublicKeys: [][]byte{pubKeyBuf0, pubKeyBuf1},
		EpochStart: params.BeaconConfig().GenesisSlot,
	}
	res, err := vs.CommitteeAssignment(context.Background(), req)
	if err != nil {
		t.Fatalf("Could not call epoch committee assignment %v", err)
	}

	if len(res.Assignment) != 2 {
		t.Fatalf("expected 2 assignments but got %d", len(res.Assignment))
	}
}

func TestValidatorStatus_CantFindValidatorIdx(t *testing.T) {
	db := internal.SetupDB(t)
	defer internal.TeardownDB(t, db)
	ctx := context.Background()

	if err := db.SaveState(ctx, &pbp2p.BeaconState{ValidatorRegistry: []*pbp2p.Validator{}}); err != nil {
		t.Fatalf("could not save state: %v", err)
	}
	vs := &ValidatorServer{
		beaconDB: db,
	}
	req := &pb.ValidatorIndexRequest{
		PublicKey: []byte{'B'},
	}
	want := fmt.Sprintf("validator %#x does not exist", req.PublicKey)
	if _, err := vs.ValidatorStatus(context.Background(), req); !strings.Contains(err.Error(), want) {
		t.Errorf("Expected %v, received %v", want, err)
	}
}

func TestValidatorStatus_PendingActive(t *testing.T) {
	db := internal.SetupDB(t)
	defer internal.TeardownDB(t, db)
	ctx := context.Background()

	pubKey := []byte{'A'}
	if err := db.SaveValidatorIndex(pubKey, 0); err != nil {
		t.Fatalf("Could not save validator index: %v", err)
	}

	// Pending active because activation epoch is still defaulted at far future slot.
	if err := db.SaveState(ctx, &pbp2p.BeaconState{ValidatorRegistry: []*pbp2p.Validator{
		{ActivationEpoch: params.BeaconConfig().FarFutureEpoch, Pubkey: pubKey},
	}}); err != nil {
		t.Fatalf("could not save state: %v", err)
	}

	vs := &ValidatorServer{
		beaconDB: db,
	}
	req := &pb.ValidatorIndexRequest{
		PublicKey: pubKey,
	}
	resp, err := vs.ValidatorStatus(context.Background(), req)
	if err != nil {
		t.Fatalf("Could not get validator status %v", err)
	}
	if resp.Status != pb.ValidatorStatus_PENDING_ACTIVE {
		t.Errorf("Wanted %v, got %v", pb.ValidatorStatus_PENDING_ACTIVE, resp.Status)
	}
}

func TestValidatorStatus_Active(t *testing.T) {
	db := internal.SetupDB(t)
	defer internal.TeardownDB(t, db)
	ctx := context.Background()

	pubKey := []byte{'A'}
	if err := db.SaveValidatorIndex(pubKey, 0); err != nil {
		t.Fatalf("Could not save validator index: %v", err)
	}

	// Active because activation epoch <= current epoch < exit epoch.
	if err := db.SaveState(ctx, &pbp2p.BeaconState{
		Slot: params.BeaconConfig().GenesisSlot,
		ValidatorRegistry: []*pbp2p.Validator{{
			ActivationEpoch: params.BeaconConfig().GenesisEpoch,
			ExitEpoch:       params.BeaconConfig().FarFutureEpoch,
			Pubkey:          pubKey},
		}}); err != nil {
		t.Fatalf("could not save state: %v", err)
	}

	vs := &ValidatorServer{
		beaconDB: db,
	}
	req := &pb.ValidatorIndexRequest{
		PublicKey: pubKey,
	}
	resp, err := vs.ValidatorStatus(context.Background(), req)
	if err != nil {
		t.Fatalf("Could not get validator status %v", err)
	}
	if resp.Status != pb.ValidatorStatus_ACTIVE {
		t.Errorf("Wanted %v, got %v", pb.ValidatorStatus_ACTIVE, resp.Status)
	}
}

func TestValidatorStatus_InitiatedExit(t *testing.T) {
	db := internal.SetupDB(t)
	defer internal.TeardownDB(t, db)
	ctx := context.Background()

	pubKey := []byte{'A'}
	if err := db.SaveValidatorIndex(pubKey, 0); err != nil {
		t.Fatalf("Could not save validator index: %v", err)
	}

	// Initiated exit because validator status flag = Validator_INITIATED_EXIT.
	if err := db.SaveState(ctx, &pbp2p.BeaconState{
		Slot: params.BeaconConfig().GenesisSlot,
		ValidatorRegistry: []*pbp2p.Validator{{
			StatusFlags: pbp2p.Validator_INITIATED_EXIT,
			Pubkey:      pubKey},
		}}); err != nil {
		t.Fatalf("could not save state: %v", err)
	}

	vs := &ValidatorServer{
		beaconDB: db,
	}
	req := &pb.ValidatorIndexRequest{
		PublicKey: pubKey,
	}
	resp, err := vs.ValidatorStatus(context.Background(), req)
	if err != nil {
		t.Fatalf("Could not get validator status %v", err)
	}
	if resp.Status != pb.ValidatorStatus_INITIATED_EXIT {
		t.Errorf("Wanted %v, got %v", pb.ValidatorStatus_INITIATED_EXIT, resp.Status)
	}
}

func TestValidatorStatus_Withdrawable(t *testing.T) {
	db := internal.SetupDB(t)
	defer internal.TeardownDB(t, db)
	ctx := context.Background()

	pubKey := []byte{'A'}
	if err := db.SaveValidatorIndex(pubKey, 0); err != nil {
		t.Fatalf("Could not save validator index: %v", err)
	}

	// Withdrawable exit because validator status flag = Validator_WITHDRAWABLE.
	if err := db.SaveState(ctx, &pbp2p.BeaconState{
		Slot: params.BeaconConfig().GenesisSlot,
		ValidatorRegistry: []*pbp2p.Validator{{
			StatusFlags: pbp2p.Validator_WITHDRAWABLE,
			Pubkey:      pubKey},
		}}); err != nil {
		t.Fatalf("could not save state: %v", err)
	}

	vs := &ValidatorServer{
		beaconDB: db,
	}
	req := &pb.ValidatorIndexRequest{
		PublicKey: pubKey,
	}
	resp, err := vs.ValidatorStatus(context.Background(), req)
	if err != nil {
		t.Fatalf("Could not get validator status %v", err)
	}
	if resp.Status != pb.ValidatorStatus_WITHDRAWABLE {
		t.Errorf("Wanted %v, got %v", pb.ValidatorStatus_WITHDRAWABLE, resp.Status)
	}
}

func TestValidatorStatus_ExitedSlashed(t *testing.T) {
	db := internal.SetupDB(t)
	defer internal.TeardownDB(t, db)
	ctx := context.Background()

	pubKey := []byte{'A'}
	if err := db.SaveValidatorIndex(pubKey, 0); err != nil {
		t.Fatalf("Could not save validator index: %v", err)
	}

	// Exit slashed because exit epoch and slashed epoch are =< current epoch.
	if err := db.SaveState(ctx, &pbp2p.BeaconState{
		Slot: params.BeaconConfig().GenesisSlot,
		ValidatorRegistry: []*pbp2p.Validator{{
			Pubkey: pubKey},
		}}); err != nil {
		t.Fatalf("could not save state: %v", err)
	}

	vs := &ValidatorServer{
		beaconDB: db,
	}
	req := &pb.ValidatorIndexRequest{
		PublicKey: pubKey,
	}
	resp, err := vs.ValidatorStatus(context.Background(), req)
	if err != nil {
		t.Fatalf("Could not get validator status %v", err)
	}
	if resp.Status != pb.ValidatorStatus_EXITED_SLASHED {
		t.Errorf("Wanted %v, got %v", pb.ValidatorStatus_EXITED_SLASHED, resp.Status)
	}
}

func TestValidatorStatus_Exited(t *testing.T) {
	db := internal.SetupDB(t)
	defer internal.TeardownDB(t, db)
	ctx := context.Background()

	pubKey := []byte{'A'}
	if err := db.SaveValidatorIndex(pubKey, 0); err != nil {
		t.Fatalf("Could not save validator index: %v", err)
	}

	// Exit because only exit epoch is =< current epoch.
	if err := db.SaveState(ctx, &pbp2p.BeaconState{
		Slot: params.BeaconConfig().GenesisSlot + 64,
		ValidatorRegistry: []*pbp2p.Validator{{
			Pubkey:       pubKey,
			SlashedEpoch: params.BeaconConfig().FarFutureEpoch},
		}}); err != nil {
		t.Fatalf("could not save state: %v", err)
	}

	vs := &ValidatorServer{
		beaconDB: db,
	}
	req := &pb.ValidatorIndexRequest{
		PublicKey: pubKey,
	}
	resp, err := vs.ValidatorStatus(context.Background(), req)
	if err != nil {
		t.Fatalf("Could not get validator status %v", err)
	}
	if resp.Status != pb.ValidatorStatus_EXITED {
		t.Errorf("Wanted %v, got %v", pb.ValidatorStatus_EXITED, resp.Status)
	}
}

func TestValidatorStatus_UnknownStatus(t *testing.T) {
	db := internal.SetupDB(t)
	defer internal.TeardownDB(t, db)
	ctx := context.Background()

	pubKey := []byte{'A'}
	if err := db.SaveValidatorIndex(pubKey, 0); err != nil {
		t.Fatalf("Could not save validator index: %v", err)
	}

	if err := db.SaveState(ctx, &pbp2p.BeaconState{
		Slot: params.BeaconConfig().GenesisSlot,
		ValidatorRegistry: []*pbp2p.Validator{{
			ActivationEpoch: params.BeaconConfig().GenesisSlot,
			ExitEpoch:       params.BeaconConfig().FarFutureEpoch,
			Pubkey:          pubKey},
		}}); err != nil {
		t.Fatalf("could not save state: %v", err)
	}

	vs := &ValidatorServer{
		beaconDB: db,
	}
	req := &pb.ValidatorIndexRequest{
		PublicKey: pubKey,
	}
	resp, err := vs.ValidatorStatus(context.Background(), req)
	if err != nil {
		t.Fatalf("Could not get validator status %v", err)
	}
	if resp.Status != pb.ValidatorStatus_UNKNOWN_STATUS {
		t.Errorf("Wanted %v, got %v", pb.ValidatorStatus_UNKNOWN_STATUS, resp.Status)
	}
}

func TestWaitForActivation_ContextClosed(t *testing.T) {
	db := internal.SetupDB(t)
	defer internal.TeardownDB(t, db)
	ctx := context.Background()

	beaconState := &pbp2p.BeaconState{
		Slot: params.BeaconConfig().GenesisSlot,
	}
	if err := db.SaveState(ctx, beaconState); err != nil {
		t.Fatalf("could not save state: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	vs := &ValidatorServer{
		beaconDB:           db,
		ctx:                ctx,
		chainService:       newMockChainService(),
		canonicalStateChan: make(chan *pbp2p.BeaconState, 1),
	}
	req := &pb.ValidatorActivationRequest{
		PublicKeys: [][]byte{[]byte("A")},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockStream := internal.NewMockValidatorService_WaitForActivationServer(ctrl)
	exitRoutine := make(chan bool)
	go func(tt *testing.T) {
		want := "context closed"
		if err := vs.WaitForActivation(req, mockStream); !strings.Contains(err.Error(), want) {
			tt.Errorf("Could not call RPC method: %v", err)
		}
		<-exitRoutine
	}(t)
	cancel()
	exitRoutine <- true
}

func TestWaitForActivation_ValidatorOriginallyExists(t *testing.T) {
	db := internal.SetupDB(t)
	defer internal.TeardownDB(t, db)
	ctx := context.Background()

	pubKeys := [][]byte{{'A'}, {'B'}}
	if err := db.SaveValidatorIndex(pubKeys[0], 0); err != nil {
		t.Fatalf("Could not save validator index: %v", err)
	}
	if err := db.SaveValidatorIndex(pubKeys[1], 0); err != nil {
		t.Fatalf("Could not save validator index: %v", err)
	}

	beaconState := &pbp2p.BeaconState{
		Slot: params.BeaconConfig().GenesisSlot,
		ValidatorRegistry: []*pbp2p.Validator{{
			ActivationEpoch: params.BeaconConfig().GenesisEpoch,
			ExitEpoch:       params.BeaconConfig().FarFutureEpoch,
			Pubkey:          pubKeys[0]},
			{
				ActivationEpoch: params.BeaconConfig().GenesisEpoch,
				ExitEpoch:       params.BeaconConfig().FarFutureEpoch,
				Pubkey:          pubKeys[1]},
		},
	}
	if err := db.SaveState(ctx, beaconState); err != nil {
		t.Fatalf("could not save state: %v", err)
	}
	if err := db.SaveValidatorIndex(pubKeys[0], 0); err != nil {
		t.Fatalf("could not save validator index: %v", err)
	}
	if err := db.SaveValidatorIndex(pubKeys[1], 1); err != nil {
		t.Fatalf("could not save validator index: %v", err)
	}
	vs := &ValidatorServer{
		beaconDB:           db,
		ctx:                context.Background(),
		chainService:       newMockChainService(),
		canonicalStateChan: make(chan *pbp2p.BeaconState, 1),
	}
	req := &pb.ValidatorActivationRequest{
		PublicKeys: pubKeys,
	}
	ctrl := gomock.NewController(t)

	defer ctrl.Finish()
	mockStream := internal.NewMockValidatorService_WaitForActivationServer(ctrl)
	mockStream.EXPECT().Context().Return(context.Background())
	mockStream.EXPECT().Send(
		&pb.ValidatorActivationResponse{
			ActivatedPublicKeys: pubKeys,
		},
	).Return(nil)

	if err := vs.WaitForActivation(req, mockStream); err != nil {
		t.Fatalf("Could not setup wait for activation stream: %v", err)
	}
}

func TestFilterActivePublicKeys(t *testing.T) {
	currentEpoch := uint64(15)
	beaconState := &pbp2p.BeaconState{
		Slot: helpers.StartSlot(currentEpoch),
		ValidatorRegistry: []*pbp2p.Validator{
			// Active validiators in our request
			{
				Pubkey:          []byte("pk1"),
				ActivationEpoch: currentEpoch - 1,
				ExitEpoch:       math.MaxUint64,
			},
			// Inactive validators in our request
			{
				Pubkey:          []byte("pk2"),
				ActivationEpoch: currentEpoch - 2,
				ExitEpoch:       currentEpoch - 1,
			},
			// Other active validators in the registry
			{
				Pubkey:          []byte("pk3"),
				ActivationEpoch: 0,
				ExitEpoch:       math.MaxUint64,
			},
		},
	}

	vs := &ValidatorServer{}

	activeKeys := vs.filterActivePublicKeys(
		beaconState,
		[][]byte{
			[]byte("pk1"),
			[]byte("pk2"),
		},
	)

	if len(activeKeys) != 1 || !bytes.Equal(activeKeys[0], []byte("pk1")) {
		t.Error("Wrong active keys returned")
	}
}

func TestAddNonActivePublicKeysAssignmentStatus(t *testing.T) {
	db := internal.SetupDB(t)
	defer internal.TeardownDB(t, db)
	currentEpoch := uint64(15)
	beaconState := &pbp2p.BeaconState{
		Slot: helpers.StartSlot(currentEpoch),
		ValidatorRegistry: []*pbp2p.Validator{
			// Active validiators in our request
			{
				Pubkey:          []byte("pk1"),
				ActivationEpoch: currentEpoch - 1,
				ExitEpoch:       math.MaxUint64,
			},
			// Inactive validators in our request
			{
				Pubkey:          []byte("pk2"),
				ActivationEpoch: currentEpoch - 2,
				ExitEpoch:       currentEpoch - 1,
			},
			// Other active validators in the registry
			{
				Pubkey:          []byte("pk3"),
				ActivationEpoch: 0,
				ExitEpoch:       math.MaxUint64,
			},
		},
	}
	if err := db.SaveState(context.Background(), beaconState); err != nil {
		t.Fatal(err)
	}
	vs := &ValidatorServer{
		beaconDB: db,
	}
	var assignments []*pb.CommitteeAssignmentResponse_CommitteeAssignment
	assignments = vs.addNonActivePublicKeysAssignmentStatus(beaconState,
		[][]byte{
			[]byte("pk1"),
			[]byte("pk4"),
		}, assignments)
	if len(assignments) != 1 || assignments[0].Status != pb.ValidatorStatus_UNKNOWN_STATUS || !bytes.Equal(assignments[0].PublicKey, []byte("pk4")) {
		t.Errorf("Unknown public key status wasn't returned: %v", assignments)
	}
}
