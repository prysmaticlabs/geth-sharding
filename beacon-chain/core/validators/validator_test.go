package validators

import (
	"reflect"
	"testing"

	ethpb "github.com/prysmaticlabs/ethereumapis/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	"github.com/prysmaticlabs/prysm/shared/params"
)

func TestHasVoted_OK(t *testing.T) {
	// Setting bitlist to 11111111.
	pendingAttestation := &ethpb.Attestation{
		AggregationBits: []byte{0xFF, 0x01},
	}

	for i := uint64(0); i < pendingAttestation.AggregationBits.Len(); i++ {
		if !pendingAttestation.AggregationBits.BitAt(i) {
			t.Error("validator voted but received didn't vote")
		}
	}

	// Setting bit field to 10101010.
	pendingAttestation = &ethpb.Attestation{
		AggregationBits: []byte{0xAA, 0x1},
	}

	for i := uint64(0); i < pendingAttestation.AggregationBits.Len(); i++ {
		voted := pendingAttestation.AggregationBits.BitAt(i)
		if i%2 == 0 && voted {
			t.Error("validator didn't vote but received voted")
		}
		if i%2 == 1 && !voted {
			t.Error("validator voted but received didn't vote")
		}
	}
}

func TestInitiateValidatorExit_AlreadyExited(t *testing.T) {
	exitEpoch := uint64(199)
	state := &pb.BeaconState{Validators: []*ethpb.Validator{{
		ExitEpoch: exitEpoch},
	}}
	newState, err := InitiateValidatorExit(state, 0)
	if err != nil {
		t.Fatal(err)
	}
	if newState.Validators[0].ExitEpoch != exitEpoch {
		t.Errorf("Already exited, wanted exit epoch %d, got %d",
			exitEpoch, newState.Validators[0].ExitEpoch)
	}
}

func TestInitiateValidatorExit_ProperExit(t *testing.T) {
	exitedEpoch := uint64(100)
	idx := uint64(3)
	state := &pb.BeaconState{Validators: []*ethpb.Validator{
		{ExitEpoch: exitedEpoch},
		{ExitEpoch: exitedEpoch + 1},
		{ExitEpoch: exitedEpoch + 2},
		{ExitEpoch: params.BeaconConfig().FarFutureEpoch},
	}}
	newState, err := InitiateValidatorExit(state, idx)
	if err != nil {
		t.Fatal(err)
	}
	if newState.Validators[idx].ExitEpoch != exitedEpoch+2 {
		t.Errorf("Exit epoch was not the highest, wanted exit epoch %d, got %d",
			exitedEpoch+2, newState.Validators[idx].ExitEpoch)
	}
}

func TestInitiateValidatorExit_ChurnOverflow(t *testing.T) {
	exitedEpoch := uint64(100)
	idx := uint64(4)
	state := &pb.BeaconState{Validators: []*ethpb.Validator{
		{ExitEpoch: exitedEpoch + 2},
		{ExitEpoch: exitedEpoch + 2},
		{ExitEpoch: exitedEpoch + 2},
		{ExitEpoch: exitedEpoch + 2}, //over flow here
		{ExitEpoch: params.BeaconConfig().FarFutureEpoch},
	}}
	newState, err := InitiateValidatorExit(state, idx)
	if err != nil {
		t.Fatal(err)
	}

	// Because of exit queue overflow,
	// validator who init exited has to wait one more epoch.
	wantedEpoch := state.Validators[0].ExitEpoch + 1

	if newState.Validators[idx].ExitEpoch != wantedEpoch {
		t.Errorf("Exit epoch did not cover overflow case, wanted exit epoch %d, got %d",
			wantedEpoch, newState.Validators[idx].ExitEpoch)
	}
}

func TestSlashValidator_OK(t *testing.T) {
	validatorCount := 100
	registry := make([]*ethpb.Validator, 0, validatorCount)
	balances := make([]uint64, 0, validatorCount)
	for i := 0; i < validatorCount; i++ {
		registry = append(registry, &ethpb.Validator{
			ActivationEpoch:  0,
			ExitEpoch:        params.BeaconConfig().FarFutureEpoch,
			EffectiveBalance: params.BeaconConfig().MaxEffectiveBalance,
		})
		balances = append(balances, params.BeaconConfig().MaxEffectiveBalance)
	}

	beaconState := &pb.BeaconState{
		Validators:  registry,
		Slashings:   make([]uint64, params.BeaconConfig().EpochsPerSlashingsVector),
		RandaoMixes: make([][]byte, params.BeaconConfig().EpochsPerHistoricalVector),
		Balances:    balances,
	}

	slashedIdx := uint64(2)
	whistleIdx := uint64(10)

	state, err := SlashValidator(beaconState, slashedIdx, whistleIdx)
	if err != nil {
		t.Fatalf("Could not slash validator %v", err)
	}

	if !state.Validators[slashedIdx].Slashed {
		t.Errorf("Validator not slashed despite supposed to being slashed")
	}

	if state.Validators[slashedIdx].WithdrawableEpoch != helpers.CurrentEpoch(state)+params.BeaconConfig().EpochsPerSlashingsVector {
		t.Errorf("Withdrawable epoch not the expected value %d", state.Validators[slashedIdx].WithdrawableEpoch)
	}

	maxBalance := params.BeaconConfig().MaxEffectiveBalance
	slashedBalance := state.Slashings[state.Slot%params.BeaconConfig().EpochsPerSlashingsVector]
	if slashedBalance != maxBalance {
		t.Errorf("Slashed balance isnt the expected amount: got %d but expected %d", slashedBalance, maxBalance)
	}

	proposer, err := helpers.BeaconProposerIndex(state)
	if err != nil {
		t.Errorf("Could not get proposer %v", err)
	}

	whistleblowerReward := slashedBalance / params.BeaconConfig().WhistleBlowerRewardQuotient
	proposerReward := whistleblowerReward / params.BeaconConfig().ProposerRewardQuotient

	if state.Balances[proposer] != maxBalance+proposerReward {
		t.Errorf("Did not get expected balance for proposer %d", state.Balances[proposer])
	}
	if state.Balances[whistleIdx] != maxBalance+whistleblowerReward-proposerReward {
		t.Errorf("Did not get expected balance for whistleblower %d", state.Balances[whistleIdx])
	}
	if state.Balances[slashedIdx] != maxBalance-(state.Validators[slashedIdx].EffectiveBalance/params.BeaconConfig().MinSlashingPenaltyQuotient) {
		t.Errorf("Did not get expected balance for slashed validator, wanted %d but got %d",
			state.Validators[slashedIdx].EffectiveBalance/params.BeaconConfig().MinSlashingPenaltyQuotient, state.Balances[slashedIdx])
	}
}

func TestActivatedValidatorIndices(t *testing.T) {
	tests := []struct {
		state  *pb.BeaconState
		wanted []uint64
	}{
		{
			state: &pb.BeaconState{
				Slot: 0,
				Validators: []*ethpb.Validator{
					{
						ActivationEpoch: helpers.DelayedActivationExitEpoch(0),
					},
					{
						ActivationEpoch: helpers.DelayedActivationExitEpoch(0),
					},
					{
						ActivationEpoch: helpers.DelayedActivationExitEpoch(5),
					},
					{
						ActivationEpoch: helpers.DelayedActivationExitEpoch(0),
					},
				},
			},
			wanted: []uint64{0, 1, 3},
		},
		{
			state: &pb.BeaconState{
				Slot: 0,
				Validators: []*ethpb.Validator{
					{
						ActivationEpoch: helpers.DelayedActivationExitEpoch(10),
					},
				},
			},
			wanted: []uint64{},
		},
		{
			state: &pb.BeaconState{
				Slot: 0,
				Validators: []*ethpb.Validator{
					{
						ActivationEpoch: helpers.DelayedActivationExitEpoch(0),
					},
				},
			},
			wanted: []uint64{0},
		},
	}
	for _, tt := range tests {
		activatedIndices := ActivatedValidatorIndices(helpers.CurrentEpoch(tt.state), tt.state.Validators)
		if !reflect.DeepEqual(tt.wanted, activatedIndices) {
			t.Errorf("Wanted %v, received %v", tt.wanted, activatedIndices)
		}
	}
}

func TestSlashedValidatorIndices(t *testing.T) {
	tests := []struct {
		state  *pb.BeaconState
		wanted []uint64
	}{
		{
			state: &pb.BeaconState{
				Slot: 0,
				Validators: []*ethpb.Validator{
					{
						WithdrawableEpoch: params.BeaconConfig().EpochsPerSlashingsVector,
						Slashed:           true,
					},
					{
						WithdrawableEpoch: params.BeaconConfig().EpochsPerSlashingsVector,
						Slashed:           false,
					},
					{
						WithdrawableEpoch: params.BeaconConfig().EpochsPerSlashingsVector,
						Slashed:           true,
					},
				},
			},
			wanted: []uint64{0, 2},
		},
		{
			state: &pb.BeaconState{
				Slot: 0,
				Validators: []*ethpb.Validator{
					{
						WithdrawableEpoch: params.BeaconConfig().EpochsPerSlashingsVector,
					},
				},
			},
			wanted: []uint64{},
		},
		{
			state: &pb.BeaconState{
				Slot: 0,
				Validators: []*ethpb.Validator{
					{
						WithdrawableEpoch: params.BeaconConfig().EpochsPerSlashingsVector,
						Slashed:           true,
					},
				},
			},
			wanted: []uint64{0},
		},
	}
	for _, tt := range tests {
		slashedIndices := SlashedValidatorIndices(helpers.CurrentEpoch(tt.state), tt.state.Validators)
		if !reflect.DeepEqual(tt.wanted, slashedIndices) {
			t.Errorf("Wanted %v, received %v", tt.wanted, slashedIndices)
		}
	}
}

func TestExitedValidatorIndices(t *testing.T) {
	tests := []struct {
		state  *pb.BeaconState
		wanted []uint64
	}{
		{
			state: &pb.BeaconState{
				Slot: helpers.SlotToEpoch(1),
				Validators: []*ethpb.Validator{
					{
						ExitEpoch:         0,
						WithdrawableEpoch: params.BeaconConfig().MinValidatorWithdrawabilityDelay,
					},
					{
						ExitEpoch:         0,
						WithdrawableEpoch: 10,
					},
					{
						ExitEpoch:         0,
						WithdrawableEpoch: params.BeaconConfig().MinValidatorWithdrawabilityDelay,
					},
				},
			},
			wanted: []uint64{0, 2},
		},
		{
			state: &pb.BeaconState{
				Slot: helpers.SlotToEpoch(1),
				Validators: []*ethpb.Validator{
					{
						ExitEpoch:         params.BeaconConfig().FarFutureEpoch,
						WithdrawableEpoch: params.BeaconConfig().MinValidatorWithdrawabilityDelay,
					},
				},
			},
			wanted: []uint64{},
		},
		{
			state: &pb.BeaconState{
				Slot: helpers.SlotToEpoch(1),
				Validators: []*ethpb.Validator{
					{
						ExitEpoch:         0,
						WithdrawableEpoch: params.BeaconConfig().MinValidatorWithdrawabilityDelay,
					},
				},
			},
			wanted: []uint64{0},
		},
	}
	for _, tt := range tests {
		activeCount, err := helpers.ActiveValidatorCount(tt.state, helpers.PrevEpoch(tt.state))
		if err != nil {
			t.Fatal(err)
		}
		exitedIndices, err := ExitedValidatorIndices(0, tt.state.Validators, activeCount)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(tt.wanted, exitedIndices) {
			t.Errorf("Wanted %v, received %v", tt.wanted, exitedIndices)
		}
	}
}
