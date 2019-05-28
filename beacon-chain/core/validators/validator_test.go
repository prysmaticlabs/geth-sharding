package validators

import (
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/state/stateutils"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	"github.com/prysmaticlabs/prysm/shared/bitutil"
	"github.com/prysmaticlabs/prysm/shared/params"
)

func TestHasVoted_OK(t *testing.T) {
	// Setting bit field to 11111111.
	pendingAttestation := &pb.Attestation{
		AggregationBitfield: []byte{255},
	}

	for i := 0; i < len(pendingAttestation.AggregationBitfield); i++ {
		voted, err := bitutil.CheckBit(pendingAttestation.AggregationBitfield, i)
		if err != nil {
			t.Errorf("checking bit failed at index: %d with : %v", i, err)
		}
		if !voted {
			t.Error("validator voted but received didn't vote")
		}
	}

	// Setting bit field to 10101000.
	pendingAttestation = &pb.Attestation{
		AggregationBitfield: []byte{84},
	}

	for i := 0; i < len(pendingAttestation.AggregationBitfield); i++ {
		voted, err := bitutil.CheckBit(pendingAttestation.AggregationBitfield, i)
		if err != nil {
			t.Errorf("checking bit failed at index: %d : %v", i, err)
		}
		if i%2 == 0 && voted {
			t.Error("validator didn't vote but received voted")
		}
		if i%2 == 1 && !voted {
			t.Error("validator voted but received didn't vote")
		}
	}
}

func TestAllValidatorIndices_OK(t *testing.T) {
	tests := []struct {
		registries []*pb.Validator
		indices    []uint64
	}{
		{registries: []*pb.Validator{}, indices: []uint64{}},
		{registries: []*pb.Validator{{}}, indices: []uint64{0}},
		{registries: []*pb.Validator{{}, {}, {}, {}}, indices: []uint64{0, 1, 2, 3}},
	}
	for _, tt := range tests {
		state := &pb.BeaconState{ValidatorRegistry: tt.registries}
		if !reflect.DeepEqual(allValidatorsIndices(state), tt.indices) {
			t.Errorf("AllValidatorsIndices(%v) = %v, wanted:%v",
				tt.registries, allValidatorsIndices(state), tt.indices)
		}
	}
}

func TestProcessDeposit_BadWithdrawalCredentials(t *testing.T) {
	registry := []*pb.Validator{
		{
			Pubkey: []byte{1, 2, 3},
		},
		{
			Pubkey:                []byte{4, 5, 6},
			WithdrawalCredentials: []byte{0},
		},
	}
	beaconState := &pb.BeaconState{
		ValidatorRegistry: registry,
	}
	pubkey := []byte{4, 5, 6}
	deposit := uint64(1000)
	proofOfPossession := []byte{}
	withdrawalCredentials := []byte{1}

	want := "expected withdrawal credentials to match"
	if _, err := ProcessDeposit(
		beaconState,
		stateutils.ValidatorIndexMap(beaconState),
		pubkey,
		deposit,
		proofOfPossession,
		withdrawalCredentials,
	); !strings.Contains(err.Error(), want) {
		t.Errorf("Wanted error to contain %s, received %v", want, err)
	}
}

func TestProcessDeposit_GoodWithdrawalCredentials(t *testing.T) {
	registry := []*pb.Validator{
		{
			Pubkey: []byte{1, 2, 3},
		},
		{
			Pubkey:                []byte{4, 5, 6},
			WithdrawalCredentials: []byte{1},
		},
	}
	balances := []uint64{0, 0}
	beaconState := &pb.BeaconState{
		Balances:          balances,
		ValidatorRegistry: registry,
	}
	pubkey := []byte{7, 8, 9}
	deposit := uint64(1000)
	proofOfPossession := []byte{}
	withdrawalCredentials := []byte{2}

	newState, err := ProcessDeposit(
		beaconState,
		stateutils.ValidatorIndexMap(beaconState),
		pubkey,
		deposit,
		proofOfPossession,
		withdrawalCredentials,
	)
	if err != nil {
		t.Fatalf("Process deposit failed: %v", err)
	}
	if newState.Balances[2] != 1000 {
		t.Errorf("Expected balance at index 1 to be 1000, received %d", newState.Balances[2])
	}
}

func TestProcessDeposit_RepeatedDeposit(t *testing.T) {
	registry := []*pb.Validator{
		{
			Pubkey: []byte{1, 2, 3},
		},
		{
			Pubkey:                []byte{4, 5, 6},
			WithdrawalCredentials: []byte{1},
		},
	}
	balances := []uint64{0, 50}
	beaconState := &pb.BeaconState{
		Balances:          balances,
		ValidatorRegistry: registry,
	}
	pubkey := []byte{4, 5, 6}
	deposit := uint64(1000)
	proofOfPossession := []byte{}
	withdrawalCredentials := []byte{1}

	newState, err := ProcessDeposit(
		beaconState,
		stateutils.ValidatorIndexMap(beaconState),
		pubkey,
		deposit,
		proofOfPossession,
		withdrawalCredentials,
	)
	if err != nil {
		t.Fatalf("Process deposit failed: %v", err)
	}
	if newState.Balances[1] != 1050 {
		t.Errorf("Expected balance at index 1 to be 1050, received %d", newState.Balances[1])
	}
}

func TestProcessDeposit_PublicKeyDoesNotExist(t *testing.T) {
	registry := []*pb.Validator{
		{
			Pubkey:                []byte{1, 2, 3},
			WithdrawalCredentials: []byte{2},
		},
		{
			Pubkey:                []byte{4, 5, 6},
			WithdrawalCredentials: []byte{1},
		},
	}
	balances := []uint64{1000, 1000}
	beaconState := &pb.BeaconState{
		Balances:          balances,
		ValidatorRegistry: registry,
	}
	pubkey := []byte{7, 8, 9}
	deposit := uint64(2000)
	proofOfPossession := []byte{}
	withdrawalCredentials := []byte{1}

	newState, err := ProcessDeposit(
		beaconState,
		stateutils.ValidatorIndexMap(beaconState),
		pubkey,
		deposit,
		proofOfPossession,
		withdrawalCredentials,
	)
	if err != nil {
		t.Fatalf("Process deposit failed: %v", err)
	}
	if len(newState.Balances) != 3 {
		t.Errorf("Expected validator balances list to increase by 1, received len %d", len(newState.Balances))
	}
	if newState.Balances[2] != 2000 {
		t.Errorf("Expected new validator have balance of %d, received %d", 2000, newState.Balances[2])
	}
}

func TestProcessDeposit_PublicKeyDoesNotExistAndEmptyValidator(t *testing.T) {
	registry := []*pb.Validator{
		{
			Pubkey:                []byte{1, 2, 3},
			WithdrawalCredentials: []byte{2},
		},
		{
			Pubkey:                []byte{4, 5, 6},
			WithdrawalCredentials: []byte{1},
		},
	}
	balances := []uint64{0, 1000}
	beaconState := &pb.BeaconState{
		Slot:              params.BeaconConfig().SlotsPerEpoch,
		Balances:          balances,
		ValidatorRegistry: registry,
	}
	pubkey := []byte{7, 8, 9}
	deposit := uint64(2000)
	proofOfPossession := []byte{}
	withdrawalCredentials := []byte{1}

	newState, err := ProcessDeposit(
		beaconState,
		stateutils.ValidatorIndexMap(beaconState),
		pubkey,
		deposit,
		proofOfPossession,
		withdrawalCredentials,
	)
	if err != nil {
		t.Fatalf("Process deposit failed: %v", err)
	}
	if len(newState.Balances) != 3 {
		t.Errorf("Expected validator balances list to be 3, received len %d", len(newState.Balances))
	}
	if newState.Balances[len(newState.Balances)-1] != 2000 {
		t.Errorf("Expected validator at last index to have balance of %d, received %d", 2000, newState.Balances[0])
	}
}

func TestActivateValidatorGenesis_OK(t *testing.T) {
	state := &pb.BeaconState{
		ValidatorRegistry: []*pb.Validator{
			{Pubkey: []byte{'A'}},
		},
	}
	newState, err := ActivateValidator(state, 0, true)
	if err != nil {
		t.Fatalf("could not execute activateValidator:%v", err)
	}
	if newState.ValidatorRegistry[0].ActivationEpoch != 0 {
		t.Errorf("Wanted activation epoch = genesis epoch, got %d",
			newState.ValidatorRegistry[0].ActivationEpoch)
	}
	if newState.ValidatorRegistry[0].ActivationEligibilityEpoch != 0 {
		t.Errorf("Wanted activation eligibility epoch = genesis epoch, got %d",
			newState.ValidatorRegistry[0].ActivationEligibilityEpoch)
	}
}

func TestActivateValidator_OK(t *testing.T) {
	state := &pb.BeaconState{
		Slot: 100, // epoch 2
		ValidatorRegistry: []*pb.Validator{
			{Pubkey: []byte{'A'}},
		},
	}
	newState, err := ActivateValidator(state, 0, false)
	if err != nil {
		t.Fatalf("could not execute activateValidator:%v", err)
	}
	currentEpoch := helpers.CurrentEpoch(state)
	wantedEpoch := helpers.DelayedActivationExitEpoch(currentEpoch)
	if newState.ValidatorRegistry[0].ActivationEpoch != wantedEpoch {
		t.Errorf("Wanted activation slot = %d, got %d",
			wantedEpoch,
			newState.ValidatorRegistry[0].ActivationEpoch)
	}
}

func TestInitiateValidatorExit_AlreadyExited(t *testing.T) {
	exitEpoch := uint64(199)
	state := &pb.BeaconState{ValidatorRegistry: []*pb.Validator{{
		ExitEpoch: exitEpoch},
	}}
	newState := InitiateValidatorExit(state, 0)
	if newState.ValidatorRegistry[0].ExitEpoch != exitEpoch {
		t.Errorf("Already exited, wanted exit epoch %d, got %d",
			exitEpoch, newState.ValidatorRegistry[0].ExitEpoch)
	}
}

func TestInitiateValidatorExit_ProperExit(t *testing.T) {
	exitedEpoch := uint64(100)
	idx := uint64(3)
	state := &pb.BeaconState{ValidatorRegistry: []*pb.Validator{
		{ExitEpoch: exitedEpoch},
		{ExitEpoch: exitedEpoch + 1},
		{ExitEpoch: exitedEpoch + 2},
		{ExitEpoch: params.BeaconConfig().FarFutureEpoch},
	}}
	newState := InitiateValidatorExit(state, idx)
	if newState.ValidatorRegistry[idx].ExitEpoch != exitedEpoch+2 {
		t.Errorf("Exit epoch was not the highest, wanted exit epoch %d, got %d",
			exitedEpoch+2, newState.ValidatorRegistry[idx].ExitEpoch)
	}
}

func TestInitiateValidatorExit_ChurnOverflow(t *testing.T) {
	exitedEpoch := uint64(100)
	idx := uint64(4)
	state := &pb.BeaconState{ValidatorRegistry: []*pb.Validator{
		{ExitEpoch: exitedEpoch + 2},
		{ExitEpoch: exitedEpoch + 2},
		{ExitEpoch: exitedEpoch + 2},
		{ExitEpoch: exitedEpoch + 2}, //over flow here
		{ExitEpoch: params.BeaconConfig().FarFutureEpoch},
	}}
	newState := InitiateValidatorExit(state, idx)

	// Because of exit queue overflow,
	// validator who init exited has to wait one more epoch.
	wantedEpoch := state.ValidatorRegistry[0].ExitEpoch + 1

	if newState.ValidatorRegistry[idx].ExitEpoch != wantedEpoch {
		t.Errorf("Exit epoch did not cover overflow case, wanted exit epoch %d, got %d",
			wantedEpoch, newState.ValidatorRegistry[idx].ExitEpoch)
	}
}

func TestExitValidator_OK(t *testing.T) {
	state := &pb.BeaconState{
		Slot:                  100, // epoch 2
		LatestSlashedBalances: []uint64{0},
		ValidatorRegistry: []*pb.Validator{
			{ExitEpoch: params.BeaconConfig().FarFutureEpoch, Pubkey: []byte{'B'}},
		},
	}
	newState := ExitValidator(state, 0)

	currentEpoch := helpers.CurrentEpoch(state)
	wantedEpoch := helpers.DelayedActivationExitEpoch(currentEpoch)
	if newState.ValidatorRegistry[0].ExitEpoch != wantedEpoch {
		t.Errorf("Wanted exit slot %d, got %d",
			wantedEpoch,
			newState.ValidatorRegistry[0].ExitEpoch)
	}
}

func TestExitValidator_AlreadyExited(t *testing.T) {
	state := &pb.BeaconState{
		Slot: 1000,
		ValidatorRegistry: []*pb.Validator{
			{ExitEpoch: params.BeaconConfig().ActivationExitDelay},
		},
	}
	state = ExitValidator(state, 0)
	if state.ValidatorRegistry[0].ExitEpoch != params.BeaconConfig().ActivationExitDelay {
		t.Error("Expected exited validator to stay exited")
	}
}

func TestEligibleToExit_OK(t *testing.T) {
	state := &pb.BeaconState{
		Slot: 1,
		ValidatorRegistry: []*pb.Validator{
			{ExitEpoch: params.BeaconConfig().ActivationExitDelay},
		},
	}
	if eligibleToExit(state, 0) {
		t.Error("eligible to exit should be true but got false")
	}

	state = &pb.BeaconState{
		Slot: params.BeaconConfig().MinValidatorWithdrawalDelay,
		ValidatorRegistry: []*pb.Validator{
			{ExitEpoch: params.BeaconConfig().ActivationExitDelay,
				SlashedEpoch: 1},
		},
	}
	if eligibleToExit(state, 0) {
		t.Error("eligible to exit should be true but got false")
	}
}

func TestMaxBalanceChurn_OK(t *testing.T) {
	maxDepositAmount := params.BeaconConfig().MaxDepositAmount
	tests := []struct {
		totalBalance    uint64
		maxBalanceChurn uint64
	}{
		{totalBalance: 1e9, maxBalanceChurn: maxDepositAmount},
		{totalBalance: maxDepositAmount, maxBalanceChurn: maxDepositAmount},
		{totalBalance: maxDepositAmount * 10, maxBalanceChurn: maxDepositAmount},
		{totalBalance: params.BeaconConfig().MaxDepositAmount * 1000, maxBalanceChurn: 5 * 1e11},
	}

	for _, tt := range tests {
		churn := maxBalanceChurn(tt.totalBalance)
		if tt.maxBalanceChurn != churn {
			t.Errorf("MaxBalanceChurn was not an expected value. Wanted: %d, got: %d",
				tt.maxBalanceChurn, churn)
		}
	}
}

func TestInitializeValidatoreStore(t *testing.T) {
	registry := make([]*pb.Validator, 0)
	indices := make([]uint64, 0)
	validatorsLimit := 100
	for i := 0; i < validatorsLimit; i++ {
		registry = append(registry, &pb.Validator{
			Pubkey:          []byte(strconv.Itoa(i)),
			ActivationEpoch: 0,
			ExitEpoch:       params.BeaconConfig().FarFutureEpoch,
		})
		indices = append(indices, uint64(i))
	}

	bState := &pb.BeaconState{
		ValidatorRegistry: registry,
		Slot:              0,
	}

	if _, ok := VStore.activatedValidators[helpers.CurrentEpoch(bState)]; ok {
		t.Fatalf("Validator store already has indices saved in this epoch")
	}

	InitializeValidatorStore(bState)
	retrievedIndices := VStore.activatedValidators[helpers.CurrentEpoch(bState)]

	if !reflect.DeepEqual(retrievedIndices, indices) {
		t.Errorf("Saved active indices are not the same as the one in the validator store, got %v but expected %v", retrievedIndices, indices)
	}
}
