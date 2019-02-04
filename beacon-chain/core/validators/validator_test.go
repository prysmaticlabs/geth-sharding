package validators

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"github.com/prysmaticlabs/prysm/beacon-chain/core/state/stateutils"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	"github.com/prysmaticlabs/prysm/shared/bitutil"
	"github.com/prysmaticlabs/prysm/shared/params"
)

var size = 1<<(config.RandBytes*8) - 1
var validatorsUpperBound = make([]*pb.ValidatorRecord, size)
var validator = &pb.ValidatorRecord{
	ExitSlot: config.FarFutureSlot,
}

func populateValidatorsMax() {
	for i := 0; i < len(validatorsUpperBound); i++ {
		validatorsUpperBound[i] = validator
	}
}

func TestHasVoted(t *testing.T) {
	// Setting bit field to 11111111.
	pendingAttestation := &pb.Attestation{
		ParticipationBitfield: []byte{255},
	}

	for i := 0; i < len(pendingAttestation.ParticipationBitfield); i++ {
		voted, err := bitutil.CheckBit(pendingAttestation.ParticipationBitfield, i)
		if err != nil {
			t.Errorf("checking bit failed at index: %d with : %v", i, err)
		}

		if !voted {
			t.Error("validator voted but received didn't vote")
		}
	}

	// Setting bit field to 01010101.
	pendingAttestation = &pb.Attestation{
		ParticipationBitfield: []byte{85},
	}

	for i := 0; i < len(pendingAttestation.ParticipationBitfield); i++ {
		voted, err := bitutil.CheckBit(pendingAttestation.ParticipationBitfield, i)
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

func TestInitialValidatorRegistry(t *testing.T) {
	validators := InitialValidatorRegistry()
	for idx, validator := range validators {
		if !isActiveValidator(validator, 1) {
			t.Errorf("validator %d status is not active", idx)
		}
	}
}

func TestValidatorIdx(t *testing.T) {
	var validators []*pb.ValidatorRecord
	for i := 0; i < 10; i++ {
		validators = append(validators, &pb.ValidatorRecord{Pubkey: []byte{}, ExitSlot: params.BeaconConfig().FarFutureSlot})
	}
	if _, err := ValidatorIdx([]byte("100"), validators); err == nil {
		t.Fatalf("ValidatorIdx should have failed,  there's no validator with pubkey 100")
	}
	validators[5].Pubkey = []byte("100")
	idx, err := ValidatorIdx([]byte("100"), validators)
	if err != nil {
		t.Fatalf("call ValidatorIdx failed: %v", err)
	}
	if idx != 5 {
		t.Errorf("Incorrect validator index. Wanted 5, Got %v", idx)
	}
}

func TestEffectiveBalance(t *testing.T) {
	defaultBalance := params.BeaconConfig().MaxDeposit

	tests := []struct {
		a uint64
		b uint64
	}{
		{a: 0, b: 0},
		{a: defaultBalance - 1, b: defaultBalance - 1},
		{a: defaultBalance, b: defaultBalance},
		{a: defaultBalance + 1, b: defaultBalance},
		{a: defaultBalance * 100, b: defaultBalance},
	}
	for _, test := range tests {
		state := &pb.BeaconState{ValidatorBalances: []uint64{test.a}}
		if EffectiveBalance(state, 0) != test.b {
			t.Errorf("EffectiveBalance(%d) = %d, want = %d", test.a, EffectiveBalance(state, 0), test.b)
		}
	}
}

func TestTotalEffectiveBalance(t *testing.T) {
	state := &pb.BeaconState{ValidatorBalances: []uint64{
		27 * 1e9, 28 * 1e9, 32 * 1e9, 40 * 1e9,
	}}

	// 27 + 28 + 32 + 32 = 119
	if TotalEffectiveBalance(state, []uint64{0, 1, 2, 3}) != 119*1e9 {
		t.Errorf("Incorrect TotalEffectiveBalance. Wanted: 119, got: %d",
			TotalEffectiveBalance(state, []uint64{0, 1, 2, 3})/1e9)
	}
}

func TestIsActiveValidator(t *testing.T) {
	tests := []struct {
		a uint64
		b bool
	}{
		{a: 0, b: false},
		{a: 10, b: true},
		{a: 100, b: false},
		{a: 1000, b: false},
		{a: 64, b: true},
	}
	for _, test := range tests {
		validator := &pb.ValidatorRecord{ActivationSlot: 10, ExitSlot: 100}
		if isActiveValidator(validator, test.a) != test.b {
			t.Errorf("isActiveValidator(%d) = %v, want = %v",
				test.a, isActiveValidator(validator, test.a), test.b)
		}
	}
}

func TestGetActiveValidatorRecord(t *testing.T) {
	inputValidators := []*pb.ValidatorRecord{
		{RandaoLayers: 0},
		{RandaoLayers: 1},
		{RandaoLayers: 2},
		{RandaoLayers: 3},
		{RandaoLayers: 4},
	}

	outputValidators := []*pb.ValidatorRecord{
		{RandaoLayers: 1},
		{RandaoLayers: 3},
	}

	state := &pb.BeaconState{
		ValidatorRegistry: inputValidators,
	}

	validators := ActiveValidators(state, []uint32{1, 3})

	if !reflect.DeepEqual(outputValidators, validators) {
		t.Errorf("Active validators don't match. Wanted: %v, Got: %v", outputValidators, validators)
	}
}

func TestBoundaryAttestingBalance(t *testing.T) {
	state := &pb.BeaconState{ValidatorBalances: []uint64{
		25 * 1e9, 26 * 1e9, 32 * 1e9, 33 * 1e9, 100 * 1e9,
	}}

	attestedBalances := AttestingBalance(state, []uint64{0, 1, 2, 3, 4})

	// 25 + 26 + 32 + 32 + 32 = 147
	if attestedBalances != 147*1e9 {
		t.Errorf("Incorrect attested balances. Wanted: %f, got: %d", 147*1e9, attestedBalances)
	}
}

func TestBoundaryAttesters(t *testing.T) {
	var validators []*pb.ValidatorRecord

	for i := 0; i < 100; i++ {
		validators = append(validators, &pb.ValidatorRecord{Pubkey: []byte{byte(i)}})
	}

	state := &pb.BeaconState{ValidatorRegistry: validators}

	boundaryAttesters := Attesters(state, []uint64{5, 2, 87, 42, 99, 0})

	expectedBoundaryAttesters := []*pb.ValidatorRecord{
		{Pubkey: []byte{byte(5)}},
		{Pubkey: []byte{byte(2)}},
		{Pubkey: []byte{byte(87)}},
		{Pubkey: []byte{byte(42)}},
		{Pubkey: []byte{byte(99)}},
		{Pubkey: []byte{byte(0)}},
	}

	if !reflect.DeepEqual(expectedBoundaryAttesters, boundaryAttesters) {
		t.Errorf("Incorrect boundary attesters. Wanted: %v, got: %v", expectedBoundaryAttesters, boundaryAttesters)
	}
}

func TestBoundaryAttesterIndices(t *testing.T) {
	if params.BeaconConfig().EpochLength != 64 {
		t.Errorf("EpochLength should be 64 for these tests to pass")
	}
	validators := make([]*pb.ValidatorRecord, config.EpochLength*4)
	for i := 0; i < len(validators); i++ {
		validators[i] = &pb.ValidatorRecord{
			ExitSlot: config.FarFutureSlot,
		}
	}

	state := &pb.BeaconState{
		ValidatorRegistry: validators,
	}

	boundaryAttestations := []*pb.PendingAttestationRecord{
		{Data: &pb.AttestationData{}, ParticipationBitfield: []byte{0x10}}, // returns indices 242
		{Data: &pb.AttestationData{}, ParticipationBitfield: []byte{0xF0}}, // returns indices 237,224,2
	}

	attesterIndices, err := ValidatorIndices(state, boundaryAttestations)
	if err != nil {
		t.Fatalf("Failed to run BoundaryAttesterIndices: %v", err)
	}

	if !reflect.DeepEqual(attesterIndices, []uint64{242, 237, 224, 2}) {
		t.Errorf("Incorrect boundary attester indices. Wanted: %v, got: %v",
			[]uint64{242, 237, 224, 2}, attesterIndices)
	}
}

func TestBeaconProposerIdx(t *testing.T) {
	if params.BeaconConfig().EpochLength != 64 {
		t.Errorf("EpochLength should be 64 for these tests to pass")
	}

	validators := make([]*pb.ValidatorRecord, config.EpochLength*4)
	for i := 0; i < len(validators); i++ {
		validators[i] = &pb.ValidatorRecord{
			ExitSlot: config.FarFutureSlot,
		}
	}

	state := &pb.BeaconState{
		ValidatorRegistry: validators,
	}

	tests := []struct {
		slot  uint64
		index uint64
	}{
		{
			slot:  1,
			index: 244,
		},
		{
			slot:  10,
			index: 82,
		},
		{
			slot:  19,
			index: 157,
		},
		{
			slot:  30,
			index: 3,
		},
		{
			slot:  39,
			index: 220,
		},
	}

	for _, tt := range tests {
		result, err := BeaconProposerIdx(state, tt.slot)
		if err != nil {
			t.Errorf("Failed to get shard and committees at slot: %v", err)
		}

		if result != tt.index {
			t.Errorf(
				"Result index was an unexpected value. Wanted %d, got %d",
				tt.index,
				result,
			)
		}
	}
}

func TestAttestingValidatorIndices_Ok(t *testing.T) {
	if params.BeaconConfig().EpochLength != 64 {
		t.Errorf("EpochLength should be 64 for these tests to pass")
	}

	var committeeIndices []uint64
	for i := uint64(0); i < 8; i++ {
		committeeIndices = append(committeeIndices, i)
	}

	validators := make([]*pb.ValidatorRecord, config.EpochLength*8)
	for i := 0; i < len(validators); i++ {
		validators[i] = &pb.ValidatorRecord{
			ExitSlot: config.FarFutureSlot,
		}
	}

	state := &pb.BeaconState{
		ValidatorRegistry: validators,
		Slot:              5,
	}

	prevAttestation := &pb.PendingAttestationRecord{
		Data: &pb.AttestationData{
			Slot:                 3,
			Shard:                3,
			ShardBlockRootHash32: []byte{'B'},
		},
		ParticipationBitfield: []byte{0x1}, //
	}

	thisAttestation := &pb.PendingAttestationRecord{
		Data: &pb.AttestationData{
			Slot:                 3,
			Shard:                3,
			ShardBlockRootHash32: []byte{'B'},
		},
		ParticipationBitfield: []byte{0x2},
	}

	indices, err := AttestingValidatorIndices(
		state,
		3,
		[]byte{'B'},
		[]*pb.PendingAttestationRecord{thisAttestation},
		[]*pb.PendingAttestationRecord{prevAttestation})
	if err != nil {
		t.Fatalf("Could not execute AttestingValidatorIndices: %v", err)
	}

	if !reflect.DeepEqual(indices, []uint64{267, 15}) {
		t.Errorf("Could not get incorrect validator indices. Wanted: %v, got: %v",
			[]uint64{267, 15}, indices)
	}
}

func TestAttestingValidatorIndices_OutOfBound(t *testing.T) {
	validators := make([]*pb.ValidatorRecord, config.EpochLength*9)
	for i := 0; i < len(validators); i++ {
		validators[i] = &pb.ValidatorRecord{
			ExitSlot: config.FarFutureSlot,
		}
	}

	state := &pb.BeaconState{
		ValidatorRegistry: validators,
		Slot:              5,
	}

	attestation := &pb.PendingAttestationRecord{
		Data: &pb.AttestationData{
			Slot:                 0,
			Shard:                1,
			ShardBlockRootHash32: []byte{'B'},
		},
		ParticipationBitfield: []byte{'A'}, // 01000001 = 1,7
	}

	_, err := AttestingValidatorIndices(
		state,
		1,
		[]byte{'B'},
		[]*pb.PendingAttestationRecord{attestation},
		nil)

	// This will fail because participation bitfield is length:1, committee bitfield is length 0.
	if err == nil {
		t.Error("AttestingValidatorIndices should have failed with incorrect bitfield")
	}
}

func TestAllValidatorIndices(t *testing.T) {
	tests := []struct {
		registries []*pb.ValidatorRecord
		indices    []uint64
	}{
		{registries: []*pb.ValidatorRecord{}, indices: []uint64{}},
		{registries: []*pb.ValidatorRecord{{}}, indices: []uint64{0}},
		{registries: []*pb.ValidatorRecord{{}, {}, {}, {}}, indices: []uint64{0, 1, 2, 3}},
	}
	for _, tt := range tests {
		state := &pb.BeaconState{ValidatorRegistry: tt.registries}
		if !reflect.DeepEqual(AllValidatorsIndices(state), tt.indices) {
			t.Errorf("AllValidatorsIndices(%v) = %v, wanted:%v",
				tt.registries, AllValidatorsIndices(state), tt.indices)
		}
	}
}

func TestNewRegistryDeltaChainTip(t *testing.T) {
	tests := []struct {
		flag                         uint64
		idx                          uint64
		pubKey                       []byte
		currentRegistryDeltaChainTip []byte
		newRegistryDeltaChainTip     []byte
	}{
		{0, 100, []byte{'A'}, []byte{'B'},
			[]byte{35, 123, 149, 41, 92, 226, 26, 73, 96, 40, 4, 219, 59, 254, 27,
				38, 220, 125, 83, 177, 78, 12, 187, 74, 72, 115, 64, 91, 16, 144, 37, 245}},
		{2, 64, []byte{'Y'}, []byte{'Z'},
			[]byte{69, 192, 214, 2, 37, 19, 40, 60, 179, 83, 79, 158, 211, 247, 151,
				7, 240, 82, 41, 37, 251, 149, 221, 37, 22, 151, 204, 234, 64, 69, 7, 166}},
	}
	for _, tt := range tests {
		newChainTip, err := NewRegistryDeltaChainTip(
			pb.ValidatorRegistryDeltaBlock_ValidatorRegistryDeltaFlags(tt.flag),
			tt.idx,
			0,
			tt.pubKey,
			tt.currentRegistryDeltaChainTip,
		)
		if err != nil {
			t.Fatalf("could not execute NewRegistryDeltaChainTip:%v", err)
		}
		if !bytes.Equal(newChainTip[:], tt.newRegistryDeltaChainTip) {
			t.Errorf("Incorrect new chain tip. Wanted %#x, got %#x",
				tt.newRegistryDeltaChainTip, newChainTip[:])
		}
	}
}

func TestProcessDeposit_PublicKeyExistsBadWithdrawalCredentials(t *testing.T) {
	registry := []*pb.ValidatorRecord{
		{
			Pubkey: []byte{1, 2, 3},
		},
		{
			Pubkey:                      []byte{4, 5, 6},
			WithdrawalCredentialsHash32: []byte{0},
		},
	}
	beaconState := &pb.BeaconState{
		ValidatorRegistry: registry,
	}
	pubkey := []byte{4, 5, 6}
	deposit := uint64(1000)
	proofOfPossession := []byte{}
	withdrawalCredentials := []byte{1}
	randaoCommitment := []byte{}

	want := "expected withdrawal credentials to match"
	if _, err := ProcessDeposit(
		beaconState,
		stateutils.ValidatorIndexMap(beaconState),
		pubkey,
		deposit,
		proofOfPossession,
		withdrawalCredentials,
		randaoCommitment,
	); !strings.Contains(err.Error(), want) {
		t.Errorf("Wanted error to contain %s, received %v", want, err)
	}
}

func TestProcessDeposit_PublicKeyExistsGoodWithdrawalCredentials(t *testing.T) {
	registry := []*pb.ValidatorRecord{
		{
			Pubkey: []byte{1, 2, 3},
		},
		{
			Pubkey:                      []byte{4, 5, 6},
			WithdrawalCredentialsHash32: []byte{1},
		},
	}
	balances := []uint64{0, 0}
	beaconState := &pb.BeaconState{
		ValidatorBalances: balances,
		ValidatorRegistry: registry,
	}
	pubkey := []byte{4, 5, 6}
	deposit := uint64(1000)
	proofOfPossession := []byte{}
	withdrawalCredentials := []byte{1}
	randaoCommitment := []byte{}

	newState, err := ProcessDeposit(
		beaconState,
		stateutils.ValidatorIndexMap(beaconState),
		pubkey,
		deposit,
		proofOfPossession,
		withdrawalCredentials,
		randaoCommitment,
	)
	if err != nil {
		t.Fatalf("Process deposit failed: %v", err)
	}
	if newState.ValidatorBalances[1] != 1000 {
		t.Errorf("Expected balance at index 1 to be 1000, received %d", newState.ValidatorBalances[1])
	}
}

func TestProcessDeposit_PublicKeyDoesNotExistNoEmptyValidator(t *testing.T) {
	registry := []*pb.ValidatorRecord{
		{
			Pubkey:                      []byte{1, 2, 3},
			WithdrawalCredentialsHash32: []byte{2},
		},
		{
			Pubkey:                      []byte{4, 5, 6},
			WithdrawalCredentialsHash32: []byte{1},
		},
	}
	balances := []uint64{1000, 1000}
	beaconState := &pb.BeaconState{
		ValidatorBalances: balances,
		ValidatorRegistry: registry,
	}
	pubkey := []byte{7, 8, 9}
	deposit := uint64(2000)
	proofOfPossession := []byte{}
	withdrawalCredentials := []byte{1}
	randaoCommitment := []byte{}

	newState, err := ProcessDeposit(
		beaconState,
		stateutils.ValidatorIndexMap(beaconState),
		pubkey,
		deposit,
		proofOfPossession,
		withdrawalCredentials,
		randaoCommitment,
	)
	if err != nil {
		t.Fatalf("Process deposit failed: %v", err)
	}
	if len(newState.ValidatorBalances) != 3 {
		t.Errorf("Expected validator balances list to increase by 1, received len %d", len(newState.ValidatorBalances))
	}
	if newState.ValidatorBalances[2] != 2000 {
		t.Errorf("Expected new validator have balance of %d, received %d", 2000, newState.ValidatorBalances[2])
	}
}

func TestProcessDeposit_PublicKeyDoesNotExistEmptyValidatorExists(t *testing.T) {
	registry := []*pb.ValidatorRecord{
		{
			Pubkey:                      []byte{1, 2, 3},
			WithdrawalCredentialsHash32: []byte{2},
		},
		{
			Pubkey:                      []byte{4, 5, 6},
			WithdrawalCredentialsHash32: []byte{1},
		},
	}
	balances := []uint64{0, 1000}
	beaconState := &pb.BeaconState{
		Slot:              params.BeaconConfig().EpochLength,
		ValidatorBalances: balances,
		ValidatorRegistry: registry,
	}
	pubkey := []byte{7, 8, 9}
	deposit := uint64(2000)
	proofOfPossession := []byte{}
	withdrawalCredentials := []byte{1}
	randaoCommitment := []byte{}

	newState, err := ProcessDeposit(
		beaconState,
		stateutils.ValidatorIndexMap(beaconState),
		pubkey,
		deposit,
		proofOfPossession,
		withdrawalCredentials,
		randaoCommitment,
	)
	if err != nil {
		t.Fatalf("Process deposit failed: %v", err)
	}
	if len(newState.ValidatorBalances) != 3 {
		t.Errorf("Expected validator balances list to be 3, received len %d", len(newState.ValidatorBalances))
	}
	if newState.ValidatorBalances[len(newState.ValidatorBalances)-1] != 2000 {
		t.Errorf("Expected validator at last index to have balance of %d, received %d", 2000, newState.ValidatorBalances[0])
	}
}

func TestActivateValidatorGenesis_Ok(t *testing.T) {
	state := &pb.BeaconState{
		ValidatorRegistryDeltaChainTipHash32: []byte{'A'},
		ValidatorRegistry: []*pb.ValidatorRecord{
			{Pubkey: []byte{'A'}},
		},
	}
	newState, err := ActivateValidator(state, 0, true)
	if err != nil {
		t.Fatalf("could not execute activateValidator:%v", err)
	}
	if newState.ValidatorRegistry[0].ActivationSlot != params.BeaconConfig().GenesisSlot {
		t.Errorf("Wanted activation slot = genesis slot, got %d",
			newState.ValidatorRegistry[0].ActivationSlot)
	}
}

func TestActivateValidator_Ok(t *testing.T) {
	state := &pb.BeaconState{
		Slot:                                 100,
		ValidatorRegistryDeltaChainTipHash32: []byte{'A'},
		ValidatorRegistry: []*pb.ValidatorRecord{
			{Pubkey: []byte{'A'}},
		},
	}
	newState, err := ActivateValidator(state, 0, false)
	if err != nil {
		t.Fatalf("could not execute activateValidator:%v", err)
	}
	if newState.ValidatorRegistry[0].ActivationSlot !=
		state.Slot+params.BeaconConfig().EntryExitDelay {
		t.Errorf("Wanted activation slot = %d, got %d",
			state.Slot+params.BeaconConfig().EntryExitDelay,
			newState.ValidatorRegistry[0].ActivationSlot)
	}
}

func TestInitiateValidatorExit_Ok(t *testing.T) {
	state := &pb.BeaconState{ValidatorRegistry: []*pb.ValidatorRecord{{}, {}, {}}}
	newState := InitiateValidatorExit(state, 2)
	if newState.ValidatorRegistry[0].StatusFlags != pb.ValidatorRecord_INITIAL {
		t.Errorf("Wanted flag INITIAL, got %v", newState.ValidatorRegistry[0].StatusFlags)
	}
	if newState.ValidatorRegistry[2].StatusFlags != pb.ValidatorRecord_INITIATED_EXIT {
		t.Errorf("Wanted flag ACTIVE_PENDING_EXIT, got %v", newState.ValidatorRegistry[0].StatusFlags)
	}
}

func TestExitValidator_Ok(t *testing.T) {
	state := &pb.BeaconState{
		Slot:                                 100,
		ValidatorRegistryDeltaChainTipHash32: []byte{'A'},
		LatestPenalizedBalances:              []uint64{0},
		ValidatorRegistry: []*pb.ValidatorRecord{
			{ExitSlot: params.BeaconConfig().FarFutureSlot, Pubkey: []byte{'B'}},
		},
	}
	newState, err := ExitValidator(state, 0)
	if err != nil {
		t.Fatalf("could not execute ExitValidator:%v", err)
	}

	if newState.ValidatorRegistry[0].ExitSlot !=
		state.Slot+params.BeaconConfig().EntryExitDelay {
		t.Errorf("Wanted exit slot %d, got %d",
			state.Slot+params.BeaconConfig().EntryExitDelay,
			newState.ValidatorRegistry[0].ExitSlot)
	}
}

func TestExitValidator_AlreadyExited(t *testing.T) {
	state := &pb.BeaconState{
		Slot: 1,
		ValidatorRegistry: []*pb.ValidatorRecord{
			{ExitSlot: params.BeaconConfig().EntryExitDelay},
		},
	}
	if _, err := ExitValidator(state, 0); err == nil {
		t.Fatal("exitValidator should have failed with exiting again")
	}
}

func TestProcessPenaltiesExits_NothingHappened(t *testing.T) {
	state := &pb.BeaconState{
		ValidatorBalances: []uint64{config.MaxDeposit},
		ValidatorRegistry: []*pb.ValidatorRecord{
			{ExitSlot: params.BeaconConfig().FarFutureSlot},
		},
	}
	if ProcessPenaltiesAndExits(state).ValidatorBalances[0] !=
		config.MaxDeposit {
		t.Errorf("wanted validator balance %d, got %d",
			config.MaxDeposit,
			ProcessPenaltiesAndExits(state).ValidatorBalances[0])
	}
}

func TestProcessPenaltiesExits_ValidatorPenalized(t *testing.T) {

	latestPenalizedExits := make([]uint64, config.LatestPenalizedExitLength)
	for i := 0; i < len(latestPenalizedExits); i++ {
		latestPenalizedExits[i] = uint64(i) * config.MaxDeposit
	}

	state := &pb.BeaconState{
		Slot:                    config.LatestPenalizedExitLength / 2 * config.EpochLength,
		LatestPenalizedBalances: latestPenalizedExits,
		ValidatorBalances:       []uint64{config.MaxDeposit, config.MaxDeposit},
		ValidatorRegistry: []*pb.ValidatorRecord{
			{ExitSlot: params.BeaconConfig().FarFutureSlot, RandaoLayers: 1},
		},
	}

	penalty := EffectiveBalance(state, 0) *
		EffectiveBalance(state, 0) /
		config.MaxDeposit

	newState := ProcessPenaltiesAndExits(state)
	if newState.ValidatorBalances[0] != config.MaxDeposit-penalty {
		t.Errorf("wanted validator balance %d, got %d",
			config.MaxDeposit-penalty,
			newState.ValidatorBalances[0])
	}
}

func TestEligibleToExit(t *testing.T) {
	state := &pb.BeaconState{
		Slot: 1,
		ValidatorRegistry: []*pb.ValidatorRecord{
			{ExitSlot: params.BeaconConfig().EntryExitDelay},
		},
	}
	if eligibleToExit(state, 0) {
		t.Error("eligible to exit should be true but got false")
	}

	state = &pb.BeaconState{
		Slot: config.MinValidatorWithdrawalTime,
		ValidatorRegistry: []*pb.ValidatorRecord{
			{ExitSlot: params.BeaconConfig().EntryExitDelay,
				PenalizedSlot: 1},
		},
	}
	if eligibleToExit(state, 0) {
		t.Error("eligible to exit should be true but got false")
	}
}

func TestUpdateRegistry_NoRotation(t *testing.T) {
	state := &pb.BeaconState{
		Slot: 5,
		ValidatorRegistry: []*pb.ValidatorRecord{
			{ExitSlot: params.BeaconConfig().EntryExitDelay},
			{ExitSlot: params.BeaconConfig().EntryExitDelay},
			{ExitSlot: params.BeaconConfig().EntryExitDelay},
			{ExitSlot: params.BeaconConfig().EntryExitDelay},
			{ExitSlot: params.BeaconConfig().EntryExitDelay},
		},
		ValidatorBalances: []uint64{
			config.MaxDeposit,
			config.MaxDeposit,
			config.MaxDeposit,
			config.MaxDeposit,
			config.MaxDeposit,
		},
	}
	newState, err := UpdateRegistry(state)
	if err != nil {
		t.Fatalf("could not update validator registry:%v", err)
	}
	for i, validator := range newState.ValidatorRegistry {
		if validator.ExitSlot != config.EntryExitDelay {
			t.Errorf("could not update registry %d, wanted exit slot %d got %d",
				i, config.EntryExitDelay, validator.ExitSlot)
		}
	}
	if newState.ValidatorRegistryUpdateSlot != state.Slot {
		t.Errorf("wanted validator registry lastet change %d, got %d",
			state.Slot, newState.ValidatorRegistryUpdateSlot)
	}
}

func TestUpdateRegistry_Activate(t *testing.T) {
	state := &pb.BeaconState{
		Slot: 5,
		ValidatorRegistry: []*pb.ValidatorRecord{
			{ExitSlot: params.BeaconConfig().EntryExitDelay,
				ActivationSlot: 5 + config.EntryExitDelay + 1},
			{ExitSlot: params.BeaconConfig().EntryExitDelay,
				ActivationSlot: 5 + config.EntryExitDelay + 1},
		},
		ValidatorBalances: []uint64{
			config.MaxDeposit,
			config.MaxDeposit,
		},
		ValidatorRegistryDeltaChainTipHash32: []byte{'A'},
	}
	newState, err := UpdateRegistry(state)
	if err != nil {
		t.Fatalf("could not update validator registry:%v", err)
	}
	for i, validator := range newState.ValidatorRegistry {
		if validator.ExitSlot != config.EntryExitDelay {
			t.Errorf("could not update registry %d, wanted exit slot %d got %d",
				i, config.EntryExitDelay, validator.ExitSlot)
		}
	}
	if newState.ValidatorRegistryUpdateSlot != state.Slot {
		t.Errorf("wanted validator registry lastet change %d, got %d",
			state.Slot, newState.ValidatorRegistryUpdateSlot)
	}

	if bytes.Equal(newState.ValidatorRegistryDeltaChainTipHash32, []byte{'A'}) {
		t.Errorf("validator registry delta chain did not change")
	}
}

func TestUpdateRegistry_Exit(t *testing.T) {
	state := &pb.BeaconState{
		Slot: 5,
		ValidatorRegistry: []*pb.ValidatorRecord{
			{
				ExitSlot:    5 + config.EntryExitDelay + 1,
				StatusFlags: pb.ValidatorRecord_INITIATED_EXIT},
			{
				ExitSlot:    5 + config.EntryExitDelay + 1,
				StatusFlags: pb.ValidatorRecord_INITIATED_EXIT},
		},
		ValidatorBalances: []uint64{
			config.MaxDeposit,
			config.MaxDeposit,
		},
		ValidatorRegistryDeltaChainTipHash32: []byte{'A'},
	}
	newState, err := UpdateRegistry(state)
	if err != nil {
		t.Fatalf("could not update validator registry:%v", err)
	}
	for i, validator := range newState.ValidatorRegistry {
		if validator.ExitSlot != config.EntryExitDelay+5 {
			t.Errorf("could not update registry %d, wanted exit slot %d got %d",
				i,
				config.EntryExitDelay+5,
				validator.ExitSlot)
		}
	}
	if newState.ValidatorRegistryUpdateSlot != state.Slot {
		t.Errorf("wanted validator registry lastet change %d, got %d",
			state.Slot, newState.ValidatorRegistryUpdateSlot)
	}

	if bytes.Equal(newState.ValidatorRegistryDeltaChainTipHash32, []byte{'A'}) {
		t.Errorf("validator registry delta chain did not change")
	}
}

func TestMaxBalanceChurn(t *testing.T) {
	tests := []struct {
		totalBalance    uint64
		maxBalanceChurn uint64
	}{
		{totalBalance: 1e9, maxBalanceChurn: config.MaxDeposit},
		{totalBalance: config.MaxDeposit, maxBalanceChurn: 512 * 1e9},
		{totalBalance: config.MaxDeposit * 10, maxBalanceChurn: 512 * 1e10},
		{totalBalance: config.MaxDeposit * 1000, maxBalanceChurn: 512 * 1e12},
	}

	for _, tt := range tests {
		churn := maxBalanceChurn(tt.totalBalance)
		if tt.maxBalanceChurn != churn {
			t.Errorf("MaxBalanceChurn was not an expected value. Wanted: %d, got: %d",
				tt.maxBalanceChurn, churn)
		}
	}
}
