package rpc

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/mock/gomock"
	"github.com/prysmaticlabs/go-ssz"
	mock "github.com/prysmaticlabs/prysm/beacon-chain/blockchain/testing"
	"github.com/prysmaticlabs/prysm/beacon-chain/cache/depositcache"
	blk "github.com/prysmaticlabs/prysm/beacon-chain/core/blocks"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/state"
	dbutil "github.com/prysmaticlabs/prysm/beacon-chain/db/testing"
	"github.com/prysmaticlabs/prysm/beacon-chain/internal"
	pbp2p "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/rpc/v1"
	ethpb "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/shared/bls"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/testutil"
	"github.com/prysmaticlabs/prysm/shared/trieutil"
)

func TestValidatorIndex_OK(t *testing.T) {
	db := dbutil.SetupDB(t)
	defer dbutil.TeardownDB(t, db)
	ctx := context.Background()
	if err := db.SaveState(ctx, &pbp2p.BeaconState{}, [32]byte{}); err != nil {
		t.Fatal(err)
	}

	pubKey := []byte{'A'}
	if err := db.SaveValidatorIndex(ctx, bytesutil.ToBytes48(pubKey), 0); err != nil {
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

func TestNextEpochCommitteeAssignment_WrongPubkeyLength(t *testing.T) {
	db := dbutil.SetupDB(t)
	defer dbutil.TeardownDB(t, db)
	ctx := context.Background()
	helpers.ClearAllCaches()

	deposits, _ := testutil.SetupInitialDeposits(t, 8)
	beaconState, err := state.GenesisBeaconState(deposits, 0, &ethpb.Eth1Data{})
	if err != nil {
		t.Fatal(err)
	}
	block := blk.NewGenesisBlock([]byte{})
	if err := db.SaveBlock(ctx, block); err != nil {
		t.Fatalf("Could not save genesis block: %v", err)
	}
	genesisRoot, err := ssz.SigningRoot(block)
	if err != nil {
		t.Fatalf("Could not get signing root %v", err)
	}

	validatorServer := &ValidatorServer{
		beaconDB:     db,
		chainService: &mock.ChainService{State: beaconState, Root: genesisRoot[:]},
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
	db := dbutil.SetupDB(t)
	defer dbutil.TeardownDB(t, db)
	ctx := context.Background()
	deposits, _ := testutil.SetupInitialDeposits(t, params.BeaconConfig().MinGenesisActiveValidatorCount)
	beaconState, err := state.GenesisBeaconState(deposits, 0, &ethpb.Eth1Data{})
	if err != nil {
		t.Fatalf("Could not setup genesis state: %v", err)
	}
	genesis := blk.NewGenesisBlock([]byte{})
	genesisRoot, err := ssz.SigningRoot(genesis)
	if err != nil {
		t.Fatalf("Could not get signing root %v", err)
	}

	vs := &ValidatorServer{
		chainService: &mock.ChainService{State: beaconState, Root: genesisRoot[:]},
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
	db := dbutil.SetupDB(t)
	defer dbutil.TeardownDB(t, db)
	ctx := context.Background()

	genesis := blk.NewGenesisBlock([]byte{})
	depChainStart := params.BeaconConfig().MinGenesisActiveValidatorCount / 16

	deposits, _ := testutil.SetupInitialDeposits(t, depChainStart)
	state, err := state.GenesisBeaconState(deposits, 0, &ethpb.Eth1Data{})
	if err != nil {
		t.Fatalf("Could not setup genesis state: %v", err)
	}
	genesisRoot, err := ssz.SigningRoot(genesis)
	if err != nil {
		t.Fatalf("Could not get signing root %v", err)
	}

	var wg sync.WaitGroup
	numOfValidators := int(depChainStart)
	errs := make(chan error, numOfValidators)
	for i := 0; i < len(deposits); i++ {
		wg.Add(1)
		go func(index int) {
			errs <- db.SaveValidatorIndex(ctx, bytesutil.ToBytes48(deposits[index].Data.PublicKey), uint64(index))
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
		beaconDB:     db,
		chainService: &mock.ChainService{State: state, Root: genesisRoot[:]},
	}

	// Test the first validator in registry.
	req := &pb.AssignmentRequest{
		PublicKeys: [][]byte{deposits[0].Data.PublicKey},
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
		PublicKeys: [][]byte{deposits[lastValidatorIndex].Data.PublicKey},
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

func TestCommitteeAssignment_CurrentEpoch_ShouldNotFail(t *testing.T) {
	helpers.ClearAllCaches()
	db := dbutil.SetupDB(t)
	defer dbutil.TeardownDB(t, db)
	ctx := context.Background()

	genesis := blk.NewGenesisBlock([]byte{})
	depChainStart := params.BeaconConfig().MinGenesisActiveValidatorCount / 16

	deposits, _ := testutil.SetupInitialDeposits(t, depChainStart)
	state, err := state.GenesisBeaconState(deposits, 0, &ethpb.Eth1Data{})
	if err != nil {
		t.Fatalf("Could not setup genesis state: %v", err)
	}
	state.Slot = 5 // Set state to non-epoch start slot.

	genesisRoot, err := ssz.SigningRoot(genesis)
	if err != nil {
		t.Fatalf("Could not get signing root %v", err)
	}

	var wg sync.WaitGroup
	numOfValidators := int(depChainStart)
	errs := make(chan error, numOfValidators)
	for i := 0; i < len(deposits); i++ {
		wg.Add(1)
		go func(index int) {
			errs <- db.SaveValidatorIndex(ctx, bytesutil.ToBytes48(deposits[index].Data.PublicKey), uint64(index))
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
		beaconDB:     db,
		chainService: &mock.ChainService{State: state, Root: genesisRoot[:]},
	}

	// Test the first validator in registry.
	req := &pb.AssignmentRequest{
		PublicKeys: [][]byte{deposits[0].Data.PublicKey},
		EpochStart: 0,
	}
	res, err := vs.CommitteeAssignment(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.ValidatorAssignment) != 1 {
		t.Error("Expected 1 assignment")
	}
}

func TestCommitteeAssignment_MultipleKeys_OK(t *testing.T) {
	db := dbutil.SetupDB(t)
	defer dbutil.TeardownDB(t, db)
	ctx := context.Background()

	genesis := blk.NewGenesisBlock([]byte{})
	depChainStart := params.BeaconConfig().MinGenesisActiveValidatorCount / 16
	deposits, _ := testutil.SetupInitialDeposits(t, depChainStart)
	state, err := state.GenesisBeaconState(deposits, 0, &ethpb.Eth1Data{})
	if err != nil {
		t.Fatalf("Could not setup genesis state: %v", err)
	}
	genesisRoot, err := ssz.SigningRoot(genesis)
	if err != nil {
		t.Fatalf("Could not get signing root %v", err)
	}

	var wg sync.WaitGroup
	numOfValidators := int(depChainStart)
	errs := make(chan error, numOfValidators)
	for i := 0; i < numOfValidators; i++ {
		wg.Add(1)
		go func(index int) {
			errs <- db.SaveValidatorIndex(ctx, bytesutil.ToBytes48(deposits[index].Data.PublicKey), uint64(index))
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
		beaconDB:     db,
		chainService: &mock.ChainService{State: state, Root: genesisRoot[:]},
	}

	pubkey0 := deposits[0].Data.PublicKey
	pubkey1 := deposits[1].Data.PublicKey

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
	db := dbutil.SetupDB(t)
	defer dbutil.TeardownDB(t, db)
	ctx := context.Background()

	pubKey := []byte{'A'}
	if err := db.SaveValidatorIndex(ctx, bytesutil.ToBytes48(pubKey), 0); err != nil {
		t.Fatalf("Could not save validator index: %v", err)
	}
	block := blk.NewGenesisBlock([]byte{})
	if err := db.SaveBlock(ctx, block); err != nil {
		t.Fatalf("Could not save genesis block: %v", err)
	}
	genesisRoot, err := ssz.SigningRoot(block)
	if err != nil {
		t.Fatalf("Could not get signing root %v", err)
	}
	if err := db.SaveHeadBlockRoot(ctx, genesisRoot); err != nil {
		t.Fatalf("Could not save genesis state: %v", err)
	}
	// Pending active because activation epoch is still defaulted at far future slot.
	state := &pbp2p.BeaconState{
		Validators: []*ethpb.Validator{
			{
				ActivationEpoch: params.BeaconConfig().FarFutureEpoch,
				PublicKey:       pubKey,
			},
		},
		Slot: 5000,
	}
	if err := db.SaveState(ctx, state, genesisRoot); err != nil {
		t.Fatalf("could not save state: %v", err)
	}
	depData := &ethpb.Deposit_Data{
		PublicKey:             pubKey,
		Signature:             []byte("hi"),
		WithdrawalCredentials: []byte("hey"),
	}

	deposit := &ethpb.Deposit{
		Data: depData,
	}
	depositTrie, err := trieutil.NewTrie(int(params.BeaconConfig().DepositContractTreeDepth))
	if err != nil {
		t.Fatal(fmt.Errorf("could not setup deposit trie: %v", err))
	}
	depositCache := depositcache.NewDepositCache()
	depositCache.InsertDeposit(ctx, deposit, big.NewInt(0) /*blockNum*/, 0, depositTrie.Root())

	height := time.Unix(int64(params.BeaconConfig().Eth1FollowDistance), 0).Unix()
	vs := &ValidatorServer{
		beaconDB: db,
		powChainService: &mockPOWChainService{
			blockTimeByHeight: map[int]uint64{
				0: uint64(height),
			},
		},
		depositCache: depositCache,
		chainService: &mock.ChainService{State: state, Root: genesisRoot[:]},
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
	db := dbutil.SetupDB(t)
	defer dbutil.TeardownDB(t, db)
	// This test breaks if it doesnt use mainnet config
	params.OverrideBeaconConfig(params.MainnetConfig())
	defer params.OverrideBeaconConfig(params.MinimalSpecConfig())
	ctx := context.Background()

	pubKey := []byte{'A'}
	if err := db.SaveValidatorIndex(ctx, bytesutil.ToBytes48(pubKey), 0); err != nil {
		t.Fatalf("Could not save validator index: %v", err)
	}

	depData := &ethpb.Deposit_Data{
		PublicKey:             pubKey,
		Signature:             []byte("hi"),
		WithdrawalCredentials: []byte("hey"),
	}

	deposit := &ethpb.Deposit{
		Data: depData,
	}
	depositTrie, err := trieutil.NewTrie(int(params.BeaconConfig().DepositContractTreeDepth))
	if err != nil {
		t.Fatal(fmt.Errorf("could not setup deposit trie: %v", err))
	}
	depositCache := depositcache.NewDepositCache()
	depositCache.InsertDeposit(ctx, deposit, big.NewInt(0) /*blockNum*/, 0, depositTrie.Root())

	// Active because activation epoch <= current epoch < exit epoch.
	activeEpoch := helpers.DelayedActivationExitEpoch(0)

	block := blk.NewGenesisBlock([]byte{})
	if err := db.SaveBlock(ctx, block); err != nil {
		t.Fatalf("Could not save genesis block: %v", err)
	}
	genesisRoot, err := ssz.SigningRoot(block)
	if err != nil {
		t.Fatalf("Could not get signing root %v", err)
	}

	state := &pbp2p.BeaconState{
		GenesisTime: uint64(time.Unix(0, 0).Unix()),
		Slot:        10000,
		Validators: []*ethpb.Validator{{
			ActivationEpoch: activeEpoch,
			ExitEpoch:       params.BeaconConfig().FarFutureEpoch,
			PublicKey:       pubKey},
		}}

	timestamp := time.Unix(int64(params.BeaconConfig().Eth1FollowDistance), 0).Unix()
	vs := &ValidatorServer{
		beaconDB: db,
		powChainService: &mockPOWChainService{
			blockTimeByHeight: map[int]uint64{
				int(params.BeaconConfig().Eth1FollowDistance): uint64(timestamp),
			},
		},
		depositCache: depositCache,
		chainService: &mock.ChainService{State: state, Root: genesisRoot[:]},
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
	db := dbutil.SetupDB(t)
	defer dbutil.TeardownDB(t, db)
	ctx := context.Background()

	pubKey := []byte{'A'}
	if err := db.SaveValidatorIndex(ctx, bytesutil.ToBytes48(pubKey), 0); err != nil {
		t.Fatalf("Could not save validator index: %v", err)
	}

	// Initiated exit because validator exit epoch and withdrawable epoch are not FAR_FUTURE_EPOCH
	slot := uint64(10000)
	epoch := helpers.SlotToEpoch(slot)
	exitEpoch := helpers.DelayedActivationExitEpoch(epoch)
	withdrawableEpoch := exitEpoch + params.BeaconConfig().MinValidatorWithdrawabilityDelay
	block := blk.NewGenesisBlock([]byte{})
	if err := db.SaveBlock(ctx, block); err != nil {
		t.Fatalf("Could not save genesis block: %v", err)
	}
	genesisRoot, err := ssz.SigningRoot(block)
	if err != nil {
		t.Fatalf("Could not get signing root %v", err)
	}

	state := &pbp2p.BeaconState{
		Slot: slot,
		Validators: []*ethpb.Validator{{
			PublicKey:         pubKey,
			ActivationEpoch:   0,
			ExitEpoch:         exitEpoch,
			WithdrawableEpoch: withdrawableEpoch},
		}}

	depData := &ethpb.Deposit_Data{
		PublicKey:             pubKey,
		Signature:             []byte("hi"),
		WithdrawalCredentials: []byte("hey"),
	}

	deposit := &ethpb.Deposit{
		Data: depData,
	}
	depositTrie, err := trieutil.NewTrie(int(params.BeaconConfig().DepositContractTreeDepth))
	if err != nil {
		t.Fatal(fmt.Errorf("could not setup deposit trie: %v", err))
	}
	depositCache := depositcache.NewDepositCache()
	depositCache.InsertDeposit(ctx, deposit, big.NewInt(0) /*blockNum*/, 0, depositTrie.Root())
	height := time.Unix(int64(params.BeaconConfig().Eth1FollowDistance), 0).Unix()
	vs := &ValidatorServer{
		beaconDB: db,
		powChainService: &mockPOWChainService{
			blockTimeByHeight: map[int]uint64{
				0: uint64(height),
			},
		},
		depositCache: depositCache,
		chainService: &mock.ChainService{State: state, Root: genesisRoot[:]},
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
	db := dbutil.SetupDB(t)
	defer dbutil.TeardownDB(t, db)
	ctx := context.Background()

	pubKey := []byte{'A'}
	if err := db.SaveValidatorIndex(ctx, bytesutil.ToBytes48(pubKey), 0); err != nil {
		t.Fatalf("Could not save validator index: %v", err)
	}

	// Withdrawable exit because current epoch is after validator withdrawable epoch.
	slot := uint64(10000)
	epoch := helpers.SlotToEpoch(slot)
	block := blk.NewGenesisBlock([]byte{})
	if err := db.SaveBlock(ctx, block); err != nil {
		t.Fatalf("Could not save genesis block: %v", err)
	}
	genesisRoot, err := ssz.SigningRoot(block)
	if err != nil {
		t.Fatalf("Could not get signing root %v", err)
	}

	state := &pbp2p.BeaconState{
		Slot: 10000,
		Validators: []*ethpb.Validator{{
			WithdrawableEpoch: epoch - 1,
			ExitEpoch:         epoch - 2,
			PublicKey:         pubKey},
		}}
	depData := &ethpb.Deposit_Data{
		PublicKey:             pubKey,
		Signature:             []byte("hi"),
		WithdrawalCredentials: []byte("hey"),
	}

	deposit := &ethpb.Deposit{
		Data: depData,
	}
	depositTrie, err := trieutil.NewTrie(int(params.BeaconConfig().DepositContractTreeDepth))
	if err != nil {
		t.Fatal(fmt.Errorf("could not setup deposit trie: %v", err))
	}
	depositCache := depositcache.NewDepositCache()
	depositCache.InsertDeposit(ctx, deposit, big.NewInt(0) /*blockNum*/, 0, depositTrie.Root())
	height := time.Unix(int64(params.BeaconConfig().Eth1FollowDistance), 0).Unix()
	vs := &ValidatorServer{
		beaconDB: db,
		powChainService: &mockPOWChainService{
			blockTimeByHeight: map[int]uint64{
				0: uint64(height),
			},
		},
		depositCache: depositCache,
		chainService: &mock.ChainService{State: state, Root: genesisRoot[:]},
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
	db := dbutil.SetupDB(t)
	defer dbutil.TeardownDB(t, db)
	ctx := context.Background()

	pubKey := []byte{'A'}
	if err := db.SaveValidatorIndex(ctx, bytesutil.ToBytes48(pubKey), 0); err != nil {
		t.Fatalf("Could not save validator index: %v", err)
	}

	// Exit slashed because slashed is true, exit epoch is =< current epoch and withdrawable epoch > epoch .
	slot := uint64(10000)
	epoch := helpers.SlotToEpoch(slot)
	block := blk.NewGenesisBlock([]byte{})
	if err := db.SaveBlock(ctx, block); err != nil {
		t.Fatalf("Could not save genesis block: %v", err)
	}
	genesisRoot, err := ssz.SigningRoot(block)
	if err != nil {
		t.Fatalf("Could not get signing root %v", err)
	}

	state := &pbp2p.BeaconState{
		Slot: slot,
		Validators: []*ethpb.Validator{{
			Slashed:           true,
			PublicKey:         pubKey,
			WithdrawableEpoch: epoch + 1},
		}}
	depData := &ethpb.Deposit_Data{
		PublicKey:             pubKey,
		Signature:             []byte("hi"),
		WithdrawalCredentials: []byte("hey"),
	}

	deposit := &ethpb.Deposit{
		Data: depData,
	}
	depositTrie, err := trieutil.NewTrie(int(params.BeaconConfig().DepositContractTreeDepth))
	if err != nil {
		t.Fatal(fmt.Errorf("could not setup deposit trie: %v", err))
	}
	depositCache := depositcache.NewDepositCache()
	depositCache.InsertDeposit(ctx, deposit, big.NewInt(0) /*blockNum*/, 0, depositTrie.Root())
	height := time.Unix(int64(params.BeaconConfig().Eth1FollowDistance), 0).Unix()
	vs := &ValidatorServer{
		beaconDB: db,
		powChainService: &mockPOWChainService{
			blockTimeByHeight: map[int]uint64{
				0: uint64(height),
			},
		},
		depositCache: depositCache,
		chainService: &mock.ChainService{State: state, Root: genesisRoot[:]},
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
	db := dbutil.SetupDB(t)
	defer dbutil.TeardownDB(t, db)
	ctx := context.Background()

	pubKey := []byte{'A'}
	if err := db.SaveValidatorIndex(ctx, bytesutil.ToBytes48(pubKey), 0); err != nil {
		t.Fatalf("Could not save validator index: %v", err)
	}

	// Exit because only exit epoch is =< current epoch.
	slot := uint64(10000)
	epoch := helpers.SlotToEpoch(slot)
	block := blk.NewGenesisBlock([]byte{})
	if err := db.SaveBlock(ctx, block); err != nil {
		t.Fatalf("Could not save genesis block: %v", err)
	}
	genesisRoot, err := ssz.SigningRoot(block)
	if err != nil {
		t.Fatalf("Could not get signing root %v", err)
	}
	if err := db.SaveHeadBlockRoot(ctx, genesisRoot); err != nil {
		t.Fatalf("Could not save genesis state: %v", err)
	}
	state := &pbp2p.BeaconState{
		Slot: slot,
		Validators: []*ethpb.Validator{{
			PublicKey:         pubKey,
			WithdrawableEpoch: epoch + 1},
		}}
	depData := &ethpb.Deposit_Data{
		PublicKey:             pubKey,
		Signature:             []byte("hi"),
		WithdrawalCredentials: []byte("hey"),
	}

	deposit := &ethpb.Deposit{
		Data: depData,
	}
	depositTrie, err := trieutil.NewTrie(int(params.BeaconConfig().DepositContractTreeDepth))
	if err != nil {
		t.Fatal(fmt.Errorf("could not setup deposit trie: %v", err))
	}
	depositCache := depositcache.NewDepositCache()
	depositCache.InsertDeposit(ctx, deposit, big.NewInt(0) /*blockNum*/, 0, depositTrie.Root())
	height := time.Unix(int64(params.BeaconConfig().Eth1FollowDistance), 0).Unix()
	vs := &ValidatorServer{
		beaconDB: db,
		powChainService: &mockPOWChainService{
			blockTimeByHeight: map[int]uint64{
				0: uint64(height),
			},
		},
		depositCache: depositCache,
		chainService: &mock.ChainService{State: state, Root: genesisRoot[:]},
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
	db := dbutil.SetupDB(t)
	defer dbutil.TeardownDB(t, db)
	ctx := context.Background()

	pubKey := []byte{'A'}
	if err := db.SaveValidatorIndex(ctx, bytesutil.ToBytes48(pubKey), 0); err != nil {
		t.Fatalf("Could not save validator index: %v", err)
	}

	block := blk.NewGenesisBlock([]byte{})
	if err := db.SaveBlock(ctx, block); err != nil {
		t.Fatalf("Could not save genesis block: %v", err)
	}
	genesisRoot, err := ssz.SigningRoot(block)
	if err != nil {
		t.Fatalf("Could not get signing root %v", err)
	}
	state := &pbp2p.BeaconState{
		Slot: 0,
		Validators: []*ethpb.Validator{{
			ActivationEpoch: 0,
			ExitEpoch:       params.BeaconConfig().FarFutureEpoch,
			PublicKey:       pubKey},
		}}
	depData := &ethpb.Deposit_Data{
		PublicKey:             pubKey,
		Signature:             []byte("hi"),
		WithdrawalCredentials: []byte("hey"),
	}

	deposit := &ethpb.Deposit{
		Data: depData,
	}
	depositTrie, err := trieutil.NewTrie(int(params.BeaconConfig().DepositContractTreeDepth))
	if err != nil {
		t.Fatal(fmt.Errorf("could not setup deposit trie: %v", err))
	}
	depositCache := depositcache.NewDepositCache()
	depositCache.InsertDeposit(ctx, deposit, big.NewInt(0) /*blockNum*/, 0, depositTrie.Root())
	height := time.Unix(int64(params.BeaconConfig().Eth1FollowDistance), 0).Unix()
	vs := &ValidatorServer{
		beaconDB: db,
		powChainService: &mockPOWChainService{
			blockTimeByHeight: map[int]uint64{
				0: uint64(height),
			},
		},
		depositCache: depositCache,
		chainService: &mock.ChainService{State: state, Root: genesisRoot[:]},
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
	db := dbutil.SetupDB(t)
	defer dbutil.TeardownDB(t, db)
	ctx := context.Background()

	beaconState := &pbp2p.BeaconState{
		Slot:       0,
		Validators: []*ethpb.Validator{},
	}
	block := blk.NewGenesisBlock([]byte{})
	if err := db.SaveBlock(ctx, block); err != nil {
		t.Fatalf("Could not save genesis block: %v", err)
	}
	genesisRoot, err := ssz.SigningRoot(block)
	if err != nil {
		t.Fatalf("Could not get signing root %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	vs := &ValidatorServer{
		beaconDB:           db,
		ctx:                ctx,
		powChainService:    &mockPOWChainService{},
		canonicalStateChan: make(chan *pbp2p.BeaconState, 1),
		depositCache:       depositcache.NewDepositCache(),
		chainService:       &mock.ChainService{State: beaconState, Root: genesisRoot[:]},
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
	db := dbutil.SetupDB(t)
	defer dbutil.TeardownDB(t, db)
	// This test breaks if it doesnt use mainnet config
	params.OverrideBeaconConfig(params.MainnetConfig())
	defer params.OverrideBeaconConfig(params.MinimalSpecConfig())
	ctx := context.Background()

	priv1, err := bls.RandKey(rand.Reader)
	if err != nil {
		t.Error(err)
	}
	priv2, err := bls.RandKey(rand.Reader)
	if err != nil {
		t.Error(err)
	}
	pubKey1 := priv1.PublicKey().Marshal()[:]
	pubKey2 := priv2.PublicKey().Marshal()[:]

	if err := db.SaveValidatorIndex(ctx, bytesutil.ToBytes48(pubKey1), 0); err != nil {
		t.Fatalf("Could not save validator index: %v", err)
	}
	if err := db.SaveValidatorIndex(ctx, bytesutil.ToBytes48(pubKey2), 0); err != nil {
		t.Fatalf("Could not save validator index: %v", err)
	}

	beaconState := &pbp2p.BeaconState{
		Slot: 4000,
		Validators: []*ethpb.Validator{
			{
				ActivationEpoch: 0,
				ExitEpoch:       params.BeaconConfig().FarFutureEpoch,
				PublicKey:       pubKey1,
			},
			{
				ActivationEpoch: 0,
				ExitEpoch:       params.BeaconConfig().FarFutureEpoch,
				PublicKey:       pubKey2,
			},
		},
	}
	block := blk.NewGenesisBlock([]byte{})
	genesisRoot, err := ssz.SigningRoot(block)
	if err != nil {
		t.Fatalf("Could not get signing root %v", err)
	}
	depData := &ethpb.Deposit_Data{
		PublicKey:             pubKey1,
		WithdrawalCredentials: []byte("hey"),
	}
	signingRoot, err := ssz.SigningRoot(depData)
	if err != nil {
		t.Error(err)
	}
	domain := bls.Domain(params.BeaconConfig().DomainDeposit, params.BeaconConfig().GenesisForkVersion)
	depData.Signature = priv1.Sign(signingRoot[:], domain).Marshal()[:]

	deposit := &ethpb.Deposit{
		Data: depData,
	}
	depositTrie, err := trieutil.NewTrie(int(params.BeaconConfig().DepositContractTreeDepth))
	if err != nil {
		t.Fatal(fmt.Errorf("could not setup deposit trie: %v", err))
	}
	depositCache := depositcache.NewDepositCache()
	depositCache.InsertDeposit(ctx, deposit, big.NewInt(10) /*blockNum*/, 0, depositTrie.Root())
	if err := db.SaveValidatorIndex(ctx, bytesutil.ToBytes48(pubKey1), 0); err != nil {
		t.Fatalf("could not save validator index: %v", err)
	}
	if err := db.SaveValidatorIndex(ctx, bytesutil.ToBytes48(pubKey2), 1); err != nil {
		t.Fatalf("could not save validator index: %v", err)
	}
	vs := &ValidatorServer{
		beaconDB:           db,
		ctx:                context.Background(),
		canonicalStateChan: make(chan *pbp2p.BeaconState, 1),
		powChainService:    &mockPOWChainService{},
		depositCache:       depositCache,
		chainService:       &mock.ChainService{State: beaconState, Root: genesisRoot[:]},
	}
	req := &pb.ValidatorActivationRequest{
		PublicKeys: [][]byte{pubKey1, pubKey2},
	}
	ctrl := gomock.NewController(t)

	defer ctrl.Finish()
	mockStream := internal.NewMockValidatorService_WaitForActivationServer(ctrl)
	mockStream.EXPECT().Context().Return(context.Background())
	mockStream.EXPECT().Send(
		&pb.ValidatorActivationResponse{
			Statuses: []*pb.ValidatorActivationResponse_Status{
				{PublicKey: pubKey1,
					Status: &pb.ValidatorStatusResponse{
						Status:                    pb.ValidatorStatus_ACTIVE,
						Eth1DepositBlockNumber:    10,
						DepositInclusionSlot:      3413,
						PositionInActivationQueue: params.BeaconConfig().FarFutureEpoch,
					},
				},
				{PublicKey: pubKey2,
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
	db := dbutil.SetupDB(t)
	defer dbutil.TeardownDB(t, db)
	ctx := context.Background()

	pubKeys := [][]byte{{'A'}, {'B'}, {'C'}}
	if err := db.SaveValidatorIndex(ctx, bytesutil.ToBytes48(pubKeys[0]), 0); err != nil {
		t.Fatalf("Could not save validator index: %v", err)
	}
	if err := db.SaveValidatorIndex(ctx, bytesutil.ToBytes48(pubKeys[1]), 0); err != nil {
		t.Fatalf("Could not save validator index: %v", err)
	}

	beaconState := &pbp2p.BeaconState{
		Slot: 4000,
		Validators: []*ethpb.Validator{{
			ActivationEpoch: 0,
			ExitEpoch:       params.BeaconConfig().FarFutureEpoch,
			PublicKey:       pubKeys[0]},
			{
				ActivationEpoch: 0,
				ExitEpoch:       params.BeaconConfig().FarFutureEpoch,
				PublicKey:       pubKeys[1]},
			{
				ActivationEpoch: 0,
				ExitEpoch:       params.BeaconConfig().FarFutureEpoch,
				PublicKey:       pubKeys[2]},
		},
	}
	block := blk.NewGenesisBlock([]byte{})
	genesisRoot, err := ssz.SigningRoot(block)
	if err != nil {
		t.Fatalf("Could not get signing root %v", err)
	}
	depData := &ethpb.Deposit_Data{
		PublicKey:             []byte{'A'},
		Signature:             []byte("hi"),
		WithdrawalCredentials: []byte("hey"),
		Amount:                10,
	}

	dep := &ethpb.Deposit{
		Data: depData,
	}
	depositTrie, err := trieutil.NewTrie(int(params.BeaconConfig().DepositContractTreeDepth))
	if err != nil {
		t.Fatal(fmt.Errorf("could not setup deposit trie: %v", err))
	}
	depositCache := depositcache.NewDepositCache()
	depositCache.InsertDeposit(ctx, dep, big.NewInt(10) /*blockNum*/, 0, depositTrie.Root())
	depData = &ethpb.Deposit_Data{
		PublicKey:             []byte{'C'},
		Signature:             []byte("hi"),
		WithdrawalCredentials: []byte("hey"),
		Amount:                10,
	}

	dep = &ethpb.Deposit{
		Data: depData,
	}
	depositTrie.InsertIntoTrie(dep.Data.Signature, 15)
	depositCache.InsertDeposit(context.Background(), dep, big.NewInt(15), 0, depositTrie.Root())

	if err := db.SaveValidatorIndex(ctx, bytesutil.ToBytes48(pubKeys[0]), 0); err != nil {
		t.Fatalf("could not save validator index: %v", err)
	}
	if err := db.SaveValidatorIndex(ctx, bytesutil.ToBytes48(pubKeys[1]), 1); err != nil {
		t.Fatalf("could not save validator index: %v", err)
	}
	if err := db.SaveValidatorIndex(ctx, bytesutil.ToBytes48(pubKeys[2]), 2); err != nil {
		t.Fatalf("could not save validator index: %v", err)
	}
	vs := &ValidatorServer{
		beaconDB:           db,
		ctx:                context.Background(),
		canonicalStateChan: make(chan *pbp2p.BeaconState, 1),
		powChainService:    &mockPOWChainService{},
		depositCache:       depositCache,
		chainService:       &mock.ChainService{State: beaconState, Root: genesisRoot[:]},
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
	db := dbutil.SetupDB(b)
	defer dbutil.TeardownDB(b, db)
	ctx := context.Background()

	genesis := blk.NewGenesisBlock([]byte{})
	if err := db.SaveBlock(ctx, genesis); err != nil {
		b.Fatalf("Could not save genesis block: %v", err)
	}
	validatorCount := params.BeaconConfig().MinGenesisActiveValidatorCount * 4
	state, err := genesisState(validatorCount)
	if err != nil {
		b.Fatalf("Could not setup genesis state: %v", err)
	}
	genesisRoot, err := ssz.SigningRoot(genesis)
	if err != nil {
		b.Fatalf("Could not get signing root %v", err)
	}
	var wg sync.WaitGroup
	errs := make(chan error, validatorCount)
	for i := 0; i < int(validatorCount); i++ {
		pubKeyBuf := make([]byte, params.BeaconConfig().BLSPubkeyLength)
		copy(pubKeyBuf[:], []byte(strconv.Itoa(i)))
		wg.Add(1)
		go func(index int) {
			errs <- db.SaveValidatorIndex(ctx, bytesutil.ToBytes48(pubKeyBuf), uint64(index))
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
		beaconDB:     db,
		chainService: &mock.ChainService{State: state, Root: genesisRoot[:]},
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
		if _, err := helpers.CrosslinkCommittee(state, 0, i); err != nil {
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
	deposits := make([]*ethpb.Deposit, validators)
	for i := 0; i < len(deposits); i++ {
		var pubKey [96]byte
		copy(pubKey[:], []byte(strconv.Itoa(i)))
		depositData := &ethpb.Deposit_Data{
			PublicKey: pubKey[:],
			Amount:    params.BeaconConfig().MaxEffectiveBalance,
		}

		deposits[i] = &ethpb.Deposit{Data: depositData}
	}
	return state.GenesisBeaconState(deposits, uint64(genesisTime), &ethpb.Eth1Data{})
}
