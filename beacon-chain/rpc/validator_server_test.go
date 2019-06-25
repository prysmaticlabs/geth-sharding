package rpc

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/mock/gomock"
	blk "github.com/prysmaticlabs/prysm/beacon-chain/core/blocks"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/state"
	"github.com/prysmaticlabs/prysm/beacon-chain/db"
	"github.com/prysmaticlabs/prysm/beacon-chain/internal"
	pbp2p "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/rpc/v1"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/testutil"
)

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

	deposits, _ := testutil.SetupInitialDeposits(t, 8, false)
	beaconState, err := state.GenesisBeaconState(deposits, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.SaveState(context.Background(), beaconState); err != nil {
		t.Fatal(err)
	}
	block := blk.NewGenesisBlock([]byte{})
	if err := db.SaveBlock(block); err != nil {
		t.Fatalf("Could not save block: %v", err)
	}
	if err := db.UpdateChainHead(ctx, block, beaconState); err != nil {
		t.Fatalf("Could not update head: %v", err)
	}
	validatorServer := &ValidatorServer{
		beaconDB: db,
	}
	req := &pb.AssignmentRequest{
		PublicKeys: [][]byte{{1}},
		EpochStart: 0,
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
	genesis := blk.NewGenesisBlock([]byte{})
	if err := db.SaveBlock(genesis); err != nil {
		t.Fatalf("Could not save genesis block: %v", err)
	}
	deposits, _ := testutil.SetupInitialDeposits(t, params.BeaconConfig().DepositsForChainStart, false)
	state, err := state.GenesisBeaconState(deposits, 0, nil)
	if err != nil {
		t.Fatalf("Could not setup genesis state: %v", err)
	}
	db.UpdateChainHead(ctx, genesis, state)
	vs := &ValidatorServer{
		beaconDB: db,
	}

	pubKey := make([]byte, 96)
	req := &pb.AssignmentRequest{
		PublicKeys: [][]byte{pubKey},
		EpochStart: 0,
	}
	want := fmt.Sprintf("validator %#x does not exist", req.PublicKeys[0])
	if _, err := vs.CommitteeAssignment(ctx, req); err != nil && !strings.Contains(err.Error(), want) {
		t.Errorf("Expected %v, received %v", want, err)
	}
}

func TestCommitteeAssignment_OK(t *testing.T) {
	helpers.ClearAllCaches()

	db := internal.SetupDB(t)
	defer internal.TeardownDB(t, db)
	ctx := context.Background()

	genesis := blk.NewGenesisBlock([]byte{})
	if err := db.SaveBlock(genesis); err != nil {
		t.Fatalf("Could not save genesis block: %v", err)
	}
	depChainStart := params.BeaconConfig().DepositsForChainStart / 16

	deposits, _ := testutil.SetupInitialDeposits(t, depChainStart, false)
	state, err := state.GenesisBeaconState(deposits, 0, nil)
	if err != nil {
		t.Fatalf("Could not setup genesis state: %v", err)
	}
	if err := db.UpdateChainHead(ctx, genesis, state); err != nil {
		t.Fatalf("Could not save genesis state: %v", err)
	}
	var wg sync.WaitGroup
	numOfValidators := int(depChainStart)
	errs := make(chan error, numOfValidators)
	for i := 0; i < len(deposits); i++ {
		wg.Add(1)
		go func(index int) {
			errs <- db.SaveValidatorIndexBatch(deposits[index].Data.Pubkey, index)
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

	// Test the first validator in registry.
	req := &pb.AssignmentRequest{
		PublicKeys: [][]byte{deposits[0].Data.Pubkey},
		EpochStart: 0,
	}
	res, err := vs.CommitteeAssignment(context.Background(), req)
	if err != nil {
		t.Fatalf("Could not call epoch committee assignment %v", err)
	}
	if res.ValidatorAssignment[0].Shard >= params.BeaconConfig().ShardCount {
		t.Errorf("Assigned shard %d can't be higher than %d",
			res.ValidatorAssignment[0].Shard, params.BeaconConfig().ShardCount)
	}
	if res.ValidatorAssignment[0].Slot > state.Slot+params.BeaconConfig().SlotsPerEpoch {
		t.Errorf("Assigned slot %d can't be higher than %d",
			res.ValidatorAssignment[0].Slot, state.Slot+params.BeaconConfig().SlotsPerEpoch)
	}

	// Test the last validator in registry.
	lastValidatorIndex := depChainStart - 1
	req = &pb.AssignmentRequest{
		PublicKeys: [][]byte{deposits[lastValidatorIndex].Data.Pubkey},
		EpochStart: 0,
	}
	res, err = vs.CommitteeAssignment(context.Background(), req)
	if err != nil {
		t.Fatalf("Could not call epoch committee assignment %v", err)
	}
	if res.ValidatorAssignment[0].Shard >= params.BeaconConfig().ShardCount {
		t.Errorf("Assigned shard %d can't be higher than %d",
			res.ValidatorAssignment[0].Shard, params.BeaconConfig().ShardCount)
	}
	if res.ValidatorAssignment[0].Slot > state.Slot+params.BeaconConfig().SlotsPerEpoch {
		t.Errorf("Assigned slot %d can't be higher than %d",
			res.ValidatorAssignment[0].Slot, state.Slot+params.BeaconConfig().SlotsPerEpoch)
	}
}

func TestCommitteeAssignment_multipleKeys_OK(t *testing.T) {
	db := internal.SetupDB(t)
	defer internal.TeardownDB(t, db)
	ctx := context.Background()

	genesis := blk.NewGenesisBlock([]byte{})
	if err := db.SaveBlock(genesis); err != nil {
		t.Fatalf("Could not save genesis block: %v", err)
	}
	depChainStart := params.BeaconConfig().DepositsForChainStart / 16
	deposits, _ := testutil.SetupInitialDeposits(t, depChainStart, false)
	state, err := state.GenesisBeaconState(deposits, 0, nil)
	if err != nil {
		t.Fatalf("Could not setup genesis state: %v", err)
	}
	if err := db.UpdateChainHead(ctx, genesis, state); err != nil {
		t.Fatalf("Could not save genesis state: %v", err)
	}
	var wg sync.WaitGroup
	numOfValidators := int(depChainStart)
	errs := make(chan error, numOfValidators)
	for i := 0; i < numOfValidators; i++ {
		wg.Add(1)
		go func(index int) {
			errs <- db.SaveValidatorIndexBatch(deposits[index].Data.Pubkey, index)
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

	pubkey0 := deposits[0].Data.Pubkey
	pubkey1 := deposits[1].Data.Pubkey

	// Test the first validator in registry.
	req := &pb.AssignmentRequest{
		PublicKeys: [][]byte{pubkey0, pubkey1},
		EpochStart: 0,
	}
	res, err := vs.CommitteeAssignment(context.Background(), req)
	if err != nil {
		t.Fatalf("Could not call epoch committee assignment %v", err)
	}

	if len(res.ValidatorAssignment) != 2 {
		t.Fatalf("expected 2 assignments but got %d", len(res.ValidatorAssignment))
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
	},
		Slot: 5000,
	}); err != nil {
		t.Fatalf("could not save state: %v", err)
	}
	depData := &pbp2p.DepositData{
		Pubkey:                pubKey,
		Signature:             []byte("hi"),
		WithdrawalCredentials: []byte("hey"),
	}

	deposit := &pbp2p.Deposit{
		Data: depData,
	}
	db.InsertDeposit(ctx, deposit, big.NewInt(0) /*blockNum*/, 0)

	height := time.Unix(int64(params.BeaconConfig().Eth1FollowDistance), 0).Unix()
	vs := &ValidatorServer{
		beaconDB: db,
		powChainService: &mockPOWChainService{
			blockTimeByHeight: map[int]uint64{
				0: uint64(height),
			},
		},
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

	depData := &pbp2p.DepositData{
		Pubkey:                pubKey,
		Signature:             []byte("hi"),
		WithdrawalCredentials: []byte("hey"),
	}

	deposit := &pbp2p.Deposit{
		Data: depData,
	}
	db.InsertDeposit(ctx, deposit, big.NewInt(0), 0)

	// Active because activation epoch <= current epoch < exit epoch.
	activeEpoch := helpers.DelayedActivationExitEpoch(0)
	if err := db.SaveState(ctx, &pbp2p.BeaconState{
		GenesisTime: uint64(time.Unix(0, 0).Unix()),
		Slot:        10000,
		ValidatorRegistry: []*pbp2p.Validator{{
			ActivationEpoch: activeEpoch,
			ExitEpoch:       params.BeaconConfig().FarFutureEpoch,
			Pubkey:          pubKey},
		}}); err != nil {
		t.Fatalf("could not save state: %v", err)
	}

	timestamp := time.Unix(int64(params.BeaconConfig().Eth1FollowDistance), 0).Unix()
	vs := &ValidatorServer{
		beaconDB: db,
		powChainService: &mockPOWChainService{
			blockTimeByHeight: map[int]uint64{
				int(params.BeaconConfig().Eth1FollowDistance): uint64(timestamp),
			},
		},
	}
	req := &pb.ValidatorIndexRequest{
		PublicKey: pubKey,
	}
	resp, err := vs.ValidatorStatus(context.Background(), req)
	if err != nil {
		t.Fatalf("Could not get validator status %v", err)
	}

	expected := &pb.ValidatorStatusResponse{
		Status:               pb.ValidatorStatus_ACTIVE,
		ActivationEpoch:      5,
		DepositInclusionSlot: 3413,
	}
	if !proto.Equal(resp, expected) {
		t.Errorf("Wanted %v, got %v", expected, resp)
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

	// Initiated exit because validator exit epoch and withdrawable epoch are not FAR_FUTURE_EPOCH
	slot := uint64(10000)
	epoch := helpers.SlotToEpoch(slot)
	exitEpoch := helpers.DelayedActivationExitEpoch(epoch)
	withdrawableEpoch := exitEpoch + params.BeaconConfig().MinValidatorWithdrawalDelay
	if err := db.SaveState(ctx, &pbp2p.BeaconState{
		Slot: slot,
		ValidatorRegistry: []*pbp2p.Validator{{
			Pubkey:            pubKey,
			ActivationEpoch:   0,
			ExitEpoch:         exitEpoch,
			WithdrawableEpoch: withdrawableEpoch},
		}}); err != nil {
		t.Fatalf("could not save state: %v", err)
	}
	depData := &pbp2p.DepositData{
		Pubkey:                pubKey,
		Signature:             []byte("hi"),
		WithdrawalCredentials: []byte("hey"),
	}

	deposit := &pbp2p.Deposit{
		Data: depData,
	}
	db.InsertDeposit(ctx, deposit, big.NewInt(0), 0)
	height := time.Unix(int64(params.BeaconConfig().Eth1FollowDistance), 0).Unix()
	vs := &ValidatorServer{
		beaconDB: db,
		powChainService: &mockPOWChainService{
			blockTimeByHeight: map[int]uint64{
				0: uint64(height),
			},
		},
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

	// Withdrawable exit because current epoch is after validator withdrawable epoch.
	slot := uint64(10000)
	epoch := helpers.SlotToEpoch(slot)
	if err := db.SaveState(ctx, &pbp2p.BeaconState{
		Slot: 10000,
		ValidatorRegistry: []*pbp2p.Validator{{
			WithdrawableEpoch: epoch - 1,
			ExitEpoch:         epoch - 2,
			Pubkey:            pubKey},
		}}); err != nil {
		t.Fatalf("could not save state: %v", err)
	}
	depData := &pbp2p.DepositData{
		Pubkey:                pubKey,
		Signature:             []byte("hi"),
		WithdrawalCredentials: []byte("hey"),
	}

	deposit := &pbp2p.Deposit{
		Data: depData,
	}
	db.InsertDeposit(ctx, deposit, big.NewInt(0), 0)
	height := time.Unix(int64(params.BeaconConfig().Eth1FollowDistance), 0).Unix()
	vs := &ValidatorServer{
		beaconDB: db,
		powChainService: &mockPOWChainService{
			blockTimeByHeight: map[int]uint64{
				0: uint64(height),
			},
		},
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

	// Exit slashed because slashed is true, exit epoch is =< current epoch and withdrawable epoch > epoch .
	slot := uint64(10000)
	epoch := helpers.SlotToEpoch(slot)
	if err := db.SaveState(ctx, &pbp2p.BeaconState{
		Slot: slot,
		ValidatorRegistry: []*pbp2p.Validator{{
			Slashed:           true,
			Pubkey:            pubKey,
			WithdrawableEpoch: epoch + 1},
		}}); err != nil {
		t.Fatalf("could not save state: %v", err)
	}
	depData := &pbp2p.DepositData{
		Pubkey:                pubKey,
		Signature:             []byte("hi"),
		WithdrawalCredentials: []byte("hey"),
	}

	deposit := &pbp2p.Deposit{
		Data: depData,
	}
	db.InsertDeposit(ctx, deposit, big.NewInt(0), 0)
	height := time.Unix(int64(params.BeaconConfig().Eth1FollowDistance), 0).Unix()
	vs := &ValidatorServer{
		beaconDB: db,
		powChainService: &mockPOWChainService{
			blockTimeByHeight: map[int]uint64{
				0: uint64(height),
			},
		},
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
	slot := uint64(10000)
	epoch := helpers.SlotToEpoch(slot)
	if err := db.SaveState(ctx, &pbp2p.BeaconState{
		Slot: slot,
		ValidatorRegistry: []*pbp2p.Validator{{
			Pubkey:            pubKey,
			WithdrawableEpoch: epoch + 1},
		}}); err != nil {
		t.Fatalf("could not save state: %v", err)
	}
	depData := &pbp2p.DepositData{
		Pubkey:                pubKey,
		Signature:             []byte("hi"),
		WithdrawalCredentials: []byte("hey"),
	}

	deposit := &pbp2p.Deposit{
		Data: depData,
	}
	db.InsertDeposit(ctx, deposit, big.NewInt(0), 0)
	height := time.Unix(int64(params.BeaconConfig().Eth1FollowDistance), 0).Unix()
	vs := &ValidatorServer{
		beaconDB: db,
		powChainService: &mockPOWChainService{
			blockTimeByHeight: map[int]uint64{
				0: uint64(height),
			},
		},
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
		Slot: 0,
		ValidatorRegistry: []*pbp2p.Validator{{
			ActivationEpoch: 0,
			ExitEpoch:       params.BeaconConfig().FarFutureEpoch,
			Pubkey:          pubKey},
		}}); err != nil {
		t.Fatalf("could not save state: %v", err)
	}
	depData := &pbp2p.DepositData{
		Pubkey:                pubKey,
		Signature:             []byte("hi"),
		WithdrawalCredentials: []byte("hey"),
	}

	deposit := &pbp2p.Deposit{
		Data: depData,
	}
	db.InsertDeposit(ctx, deposit, big.NewInt(0), 0)
	height := time.Unix(int64(params.BeaconConfig().Eth1FollowDistance), 0).Unix()
	vs := &ValidatorServer{
		beaconDB: db,
		powChainService: &mockPOWChainService{
			blockTimeByHeight: map[int]uint64{
				0: uint64(height),
			},
		},
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
		Slot:              0,
		ValidatorRegistry: []*pbp2p.Validator{},
	}
	if err := db.SaveState(ctx, beaconState); err != nil {
		t.Fatalf("could not save state: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	vs := &ValidatorServer{
		beaconDB:           db,
		ctx:                ctx,
		chainService:       newMockChainService(),
		powChainService:    &mockPOWChainService{},
		canonicalStateChan: make(chan *pbp2p.BeaconState, 1),
	}
	req := &pb.ValidatorActivationRequest{
		PublicKeys: [][]byte{[]byte("A")},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockStream := internal.NewMockValidatorService_WaitForActivationServer(ctrl)
	mockStream.EXPECT().Context().Return(context.Background())
	mockStream.EXPECT().Send(gomock.Any()).Return(nil)
	mockStream.EXPECT().Context().Return(context.Background())
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
		Slot: 4000,
		ValidatorRegistry: []*pbp2p.Validator{
			{
				ActivationEpoch: 0,
				ExitEpoch:       params.BeaconConfig().FarFutureEpoch,
				Pubkey:          pubKeys[0],
			},
			{
				ActivationEpoch: 0,
				ExitEpoch:       params.BeaconConfig().FarFutureEpoch,
				Pubkey:          pubKeys[1],
			},
		},
	}
	if err := db.SaveState(ctx, beaconState); err != nil {
		t.Fatalf("could not save state: %v", err)
	}
	depData := &pbp2p.DepositData{
		Pubkey:                []byte{'A'},
		Signature:             []byte("hi"),
		WithdrawalCredentials: []byte("hey"),
	}

	deposit := &pbp2p.Deposit{
		Data: depData,
	}
	db.InsertDeposit(context.Background(), deposit, big.NewInt(10), 0)

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
		powChainService:    &mockPOWChainService{},
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
			Statuses: []*pb.ValidatorActivationResponse_Status{
				{PublicKey: []byte{'A'},
					Status: &pb.ValidatorStatusResponse{
						Status:                    pb.ValidatorStatus_ACTIVE,
						Eth1DepositBlockNumber:    10,
						DepositInclusionSlot:      3413,
						PositionInActivationQueue: params.BeaconConfig().FarFutureEpoch,
					},
				},
				{PublicKey: []byte{'B'},
					Status: &pb.ValidatorStatusResponse{
						ActivationEpoch: params.BeaconConfig().FarFutureEpoch,
					},
				},
			},
		},
	).Return(nil)

	if err := vs.WaitForActivation(req, mockStream); err != nil {
		t.Fatalf("Could not setup wait for activation stream: %v", err)
	}
}

func TestMultipleValidatorStatus_OK(t *testing.T) {
	db := internal.SetupDB(t)
	defer internal.TeardownDB(t, db)
	ctx := context.Background()

	pubKeys := [][]byte{{'A'}, {'B'}, {'C'}}
	if err := db.SaveValidatorIndex(pubKeys[0], 0); err != nil {
		t.Fatalf("Could not save validator index: %v", err)
	}
	if err := db.SaveValidatorIndex(pubKeys[1], 0); err != nil {
		t.Fatalf("Could not save validator index: %v", err)
	}

	beaconState := &pbp2p.BeaconState{
		Slot: 4000,
		ValidatorRegistry: []*pbp2p.Validator{{
			ActivationEpoch: 0,
			ExitEpoch:       params.BeaconConfig().FarFutureEpoch,
			Pubkey:          pubKeys[0]},
			{
				ActivationEpoch: 0,
				ExitEpoch:       params.BeaconConfig().FarFutureEpoch,
				Pubkey:          pubKeys[1]},
			{
				ActivationEpoch: 0,
				ExitEpoch:       params.BeaconConfig().FarFutureEpoch,
				Pubkey:          pubKeys[2]},
		},
	}
	if err := db.SaveState(ctx, beaconState); err != nil {
		t.Fatalf("could not save state: %v", err)
	}
	depData := &pbp2p.DepositData{
		Pubkey:                []byte{'A'},
		Signature:             []byte("hi"),
		WithdrawalCredentials: []byte("hey"),
		Amount:                10,
	}

	dep := &pbp2p.Deposit{
		Data: depData,
	}
	db.InsertDeposit(context.Background(), dep, big.NewInt(10), 0)
	depData = &pbp2p.DepositData{
		Pubkey:                []byte{'C'},
		Signature:             []byte("hi"),
		WithdrawalCredentials: []byte("hey"),
		Amount:                10,
	}

	dep = &pbp2p.Deposit{
		Data: depData,
	}
	db.InsertDeposit(context.Background(), dep, big.NewInt(15), 0)

	if err := db.SaveValidatorIndex(pubKeys[0], 0); err != nil {
		t.Fatalf("could not save validator index: %v", err)
	}
	if err := db.SaveValidatorIndex(pubKeys[1], 1); err != nil {
		t.Fatalf("could not save validator index: %v", err)
	}
	if err := db.SaveValidatorIndex(pubKeys[2], 2); err != nil {
		t.Fatalf("could not save validator index: %v", err)
	}
	vs := &ValidatorServer{
		beaconDB:           db,
		ctx:                context.Background(),
		chainService:       newMockChainService(),
		canonicalStateChan: make(chan *pbp2p.BeaconState, 1),
		powChainService:    &mockPOWChainService{},
	}
	activeExists, response, err := vs.MultipleValidatorStatus(context.Background(), pubKeys)
	if err != nil {
		t.Fatal(err)
	}
	if !activeExists {
		t.Fatal("No activated validator exists when there was supposed to be 2")
	}
	if response[0].Status.Status != pb.ValidatorStatus_ACTIVE {
		t.Errorf("Validator with pubkey %#x is not activated and instead has this status: %s",
			response[0].PublicKey, response[0].Status.Status.String())
	}

	if response[1].Status.Status == pb.ValidatorStatus_ACTIVE {
		t.Errorf("Validator with pubkey %#x was activated when not supposed to",
			response[1].PublicKey)
	}

	if response[2].Status.Status != pb.ValidatorStatus_ACTIVE {
		t.Errorf("Validator with pubkey %#x is not activated and instead has this status: %s",
			response[2].PublicKey, response[2].Status.Status.String())
	}
}

func BenchmarkAssignment(b *testing.B) {
	b.StopTimer()
	randPath, _ := rand.Int(rand.Reader, big.NewInt(1000000))
	path := path.Join(testutil.TempDir(), fmt.Sprintf("/%d", randPath))
	db, _ := db.NewDB(path)
	defer db.Close()
	os.RemoveAll(db.DatabasePath)

	genesis := blk.NewGenesisBlock([]byte{})
	if err := db.SaveBlock(genesis); err != nil {
		b.Fatalf("Could not save genesis block: %v", err)
	}
	validatorCount := params.BeaconConfig().DepositsForChainStart * 4
	state, err := genesisState(validatorCount)
	if err != nil {
		b.Fatalf("Could not setup genesis state: %v", err)
	}
	if err := db.UpdateChainHead(context.Background(), genesis, state); err != nil {
		b.Fatalf("Could not save genesis state: %v", err)
	}
	var wg sync.WaitGroup
	errs := make(chan error, validatorCount)
	for i := 0; i < int(validatorCount); i++ {
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
			b.Fatal(err)
		}
	}

	vs := &ValidatorServer{
		beaconDB: db,
	}

	// Set up request for 100 public keys at a time
	pubKeys := make([][]byte, 100)
	for i := 0; i < len(pubKeys); i++ {
		buf := make([]byte, params.BeaconConfig().BLSPubkeyLength)
		copy(buf, []byte(strconv.Itoa(i)))
		pubKeys[i] = buf
	}

	req := &pb.AssignmentRequest{
		PublicKeys: pubKeys,
		EpochStart: 0,
	}

	// Precache the shuffled indices
	for i := uint64(0); i < validatorCount/params.BeaconConfig().TargetCommitteeSize; i++ {
		if _, err := helpers.CrosslinkCommitteeAtEpoch(state, 0, i); err != nil {
			b.Fatal(err)
		}
	}

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		if _, err := vs.CommitteeAssignment(context.Background(), req); err != nil {
			b.Fatal(err)
		}
	}
}

func genesisState(validators uint64) (*pbp2p.BeaconState, error) {
	genesisTime := time.Unix(0, 0).Unix()
	deposits := make([]*pbp2p.Deposit, validators)
	for i := 0; i < len(deposits); i++ {
		var pubKey [96]byte
		copy(pubKey[:], []byte(strconv.Itoa(i)))
		depositData := &pbp2p.DepositData{
			Pubkey: pubKey[:],
			Amount: params.BeaconConfig().MaxDepositAmount,
		}

		deposits[i] = &pbp2p.Deposit{Data: depositData}
	}
	return state.GenesisBeaconState(deposits, uint64(genesisTime), nil)
}
