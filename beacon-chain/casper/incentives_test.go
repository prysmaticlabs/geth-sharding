package casper

import (
	"math"
	"testing"

	"github.com/prysmaticlabs/prysm/beacon-chain/params"

	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
)

func NewValidators() []*pb.ValidatorRecord {
	var validators []*pb.ValidatorRecord

	for i := 0; i < 10; i++ {
		validator := &pb.ValidatorRecord{Balance: 1e18, StartDynasty: 1, EndDynasty: 10}
		validators = append(validators, validator)
	}
	return validators
}

func TestComputeValidatorRewardsAndPenalties(t *testing.T) {
	validators := NewValidators()
	defaultBalance := uint64(1e18)

	rewQuotient := RewardQuotient(1, validators)
	participatedDeposit := 4 * defaultBalance
	totalDeposit := 10 * defaultBalance
	depositFactor := (2*participatedDeposit - totalDeposit) / totalDeposit
	penaltyQuotient := quadraticPenaltyQuotient()
	timeSinceFinality := uint64(5)

	data := &pb.CrystallizedState{
		Validators:        validators,
		CurrentDynasty:    1,
		LastJustifiedSlot: 4,
		LastFinalizedSlot: 3,
	}

	rewardedValidators := CalculateRewards(
		5,
		[]uint32{2, 3, 6, 9},
		data.Validators,
		data.CurrentDynasty,
		participatedDeposit,
		timeSinceFinality)

	expectedBalance := defaultBalance - defaultBalance/uint64(rewQuotient)

	if rewardedValidators[0].Balance != expectedBalance {
		t.Fatalf("validator balance not updated correctly: %d, %d", rewardedValidators[0].Balance, expectedBalance)
	}

	expectedBalance = uint64(defaultBalance + (defaultBalance/rewQuotient)*depositFactor)

	if rewardedValidators[6].Balance != expectedBalance {
		t.Fatalf("validator balance not updated correctly: %d, %d", rewardedValidators[6].Balance, expectedBalance)
	}

	if rewardedValidators[9].Balance != expectedBalance {
		t.Fatalf("validator balance not updated correctly: %d, %d", rewardedValidators[9].Balance, expectedBalance)
	}

	validators = NewValidators()
	timeSinceFinality = 200

	rewardedValidators = CalculateRewards(
		5,
		[]uint32{1, 2, 7, 8},
		validators,
		data.CurrentDynasty,
		participatedDeposit,
		timeSinceFinality)

	if rewardedValidators[1].Balance != defaultBalance {
		t.Fatalf("validator balance not updated correctly: %d, %d", rewardedValidators[1].Balance, defaultBalance)
	}

	if rewardedValidators[7].Balance != defaultBalance {
		t.Fatalf("validator balance not updated correctly: %d, %d", rewardedValidators[7].Balance, defaultBalance)
	}

	expectedBalance = defaultBalance - (defaultBalance/rewQuotient + defaultBalance*timeSinceFinality/penaltyQuotient)

	if rewardedValidators[0].Balance != expectedBalance {
		t.Fatalf("validator balance not updated correctly: %d, %d", rewardedValidators[0].Balance, expectedBalance)
	}

	if rewardedValidators[9].Balance != expectedBalance {
		t.Fatalf("validator balance not updated correctly: %d, %d", rewardedValidators[9].Balance, expectedBalance)
	}

}

func TestRewardQuotient(t *testing.T) {
	validators := []*pb.ValidatorRecord{
		{Balance: 1e18,
			StartDynasty: 0,
			EndDynasty:   2},
	}
	rewQuotient := RewardQuotient(0, validators)

	if rewQuotient != params.BaseRewardQuotient {
		t.Errorf("incorrect reward quotient: %d", rewQuotient)
	}
}

func TestSlotMaxInterestRate(t *testing.T) {
	validators := []*pb.ValidatorRecord{
		{Balance: 1e18,
			StartDynasty: 0,
			EndDynasty:   2},
	}

	interestRate := SlotMaxInterestRate(0, validators)

	if interestRate != 1/float64(params.BaseRewardQuotient) {
		t.Errorf("incorrect interest rate generated %f", interestRate)
	}

}

func TestQuadraticPenaltyQuotient(t *testing.T) {
	penaltyQuotient := quadraticPenaltyQuotient()

	if penaltyQuotient != uint64(math.Pow(math.Pow(2, 17), 2)) {
		t.Errorf("incorrect penalty quotient %d", penaltyQuotient)
	}
}

func TestQuadraticPenalty(t *testing.T) {
	numOfSlots := uint64(4)
	penalty := QuadraticPenalty(numOfSlots)
	penaltyQuotient := uint64(math.Pow(math.Pow(2, 17), 0.5))

	expectedPenalty := (numOfSlots * numOfSlots / 2) / penaltyQuotient

	if expectedPenalty != penalty {
		t.Errorf("quadric penalty is not the expected amount for %d slots %d", numOfSlots, penalty)
	}
}

func TestRewardCrosslink(t *testing.T) {
	totalDeposit := uint64(6e18)
	participatedDeposit := uint64(3e18)
	rewardQuotient := params.BaseRewardQuotient * uint64(math.Pow(float64(totalDeposit), 0.5))
	validator := &pb.ValidatorRecord{
		Balance: 1e18,
	}

	RewardValidatorCrosslink(totalDeposit, participatedDeposit, rewardQuotient, validator)

	if validator.Balance != 1e18 {
		t.Errorf("validator balances have changed when they were not supposed to %d", validator.Balance)
	}
	participatedDeposit = uint64(4e18)
	RewardValidatorCrosslink(totalDeposit, participatedDeposit, rewardQuotient, validator)

	if validator.Balance == 1e18 {
		t.Errorf("validator balances have not been updated %d ", validator.Balance)
	}

}

func TestPenaltyCrosslink(t *testing.T) {
	totalDeposit := uint64(6e18)
	rewardQuotient := params.BaseRewardQuotient * uint64(math.Pow(float64(totalDeposit), 0.5))
	validator := &pb.ValidatorRecord{
		Balance: 1e18,
	}
	timeSinceConfirmation := uint64(100)
	quadraticQuotient := quadraticPenaltyQuotient()

	PenaliseValidatorCrosslink(timeSinceConfirmation, rewardQuotient, validator)
	expectedBalance := 1e18 - 1e18/rewardQuotient + 100/quadraticQuotient

	if validator.Balance != expectedBalance {
		t.Fatalf("balances not updated correctly %d, %d", validator.Balance, expectedBalance)
	}

}
