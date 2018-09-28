package casper

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/prysmaticlabs/prysm/beacon-chain/params"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
)

func TestGetShardAndCommitteesForSlots(t *testing.T) {
	state := &pb.CrystallizedState{
		LastStateRecalc: 65,
		ShardAndCommitteesForSlots: []*pb.ShardAndCommitteeArray{
			{ArrayShardAndCommittee: []*pb.ShardAndCommittee{
				{ShardId: 1, Committee: []uint32{0, 1, 2, 3, 4}},
				{ShardId: 2, Committee: []uint32{5, 6, 7, 8, 9}},
			}},
			{ArrayShardAndCommittee: []*pb.ShardAndCommittee{
				{ShardId: 3, Committee: []uint32{0, 1, 2, 3, 4}},
				{ShardId: 4, Committee: []uint32{5, 6, 7, 8, 9}},
			}},
		}}
	if _, err := GetShardAndCommitteesForSlot(state.ShardAndCommitteesForSlots, state.LastStateRecalc, 1000); err == nil {
		t.Error("getShardAndCommitteesForSlot should have failed with invalid slot")
	}
	committee, err := GetShardAndCommitteesForSlot(state.ShardAndCommitteesForSlots, state.LastStateRecalc, 1)
	if err != nil {
		t.Errorf("getShardAndCommitteesForSlot failed: %v", err)
	}
	if committee.ArrayShardAndCommittee[0].ShardId != 1 {
		t.Errorf("getShardAndCommitteesForSlot returns shardID should be 1, got: %v", committee.ArrayShardAndCommittee[0].ShardId)
	}
	committee, _ = GetShardAndCommitteesForSlot(state.ShardAndCommitteesForSlots, state.LastStateRecalc, 2)
	if committee.ArrayShardAndCommittee[0].ShardId != 3 {
		t.Errorf("getShardAndCommitteesForSlot returns shardID should be 3, got: %v", committee.ArrayShardAndCommittee[0].ShardId)
	}
}

func TestMaxValidators(t *testing.T) {
	// Create more validators than MaxValidators defined in config, this should fail.
	var validators []*pb.ValidatorRecord
	for i := 0; i < params.GetConfig().MaxValidators+1; i++ {
		validator := &pb.ValidatorRecord{StartDynasty: 1, EndDynasty: 100}
		validators = append(validators, validator)
	}

	// ValidatorsBySlotShard should fail the same.
	if _, err := ShuffleValidatorsToCommittees(common.Hash{'A'}, validators, 1, 0); err == nil {
		t.Errorf("ValidatorsBySlotShard should have failed")
	}
}

func TestShuffleActiveValidators(t *testing.T) {
	// Create 1000 validators in ActiveValidators.
	var validators []*pb.ValidatorRecord
	for i := 0; i < 1000; i++ {
		validator := &pb.ValidatorRecord{StartDynasty: 1, EndDynasty: 100}
		validators = append(validators, validator)
	}

	indices, err := ShuffleValidatorsToCommittees(common.Hash{'A'}, validators, 1, 0)
	if err != nil {
		t.Errorf("validatorsBySlotShard failed with %v:", err)
	}
	if len(indices) != int(params.GetConfig().CycleLength) {
		t.Errorf("incorret length for validator indices. Want: %d. Got: %v", params.GetConfig().CycleLength, len(indices))
	}
}

func TestSmallSampleValidators(t *testing.T) {
	// Create a small number of validators validators in ActiveValidators.
	var validators []*pb.ValidatorRecord
	for i := 0; i < 20; i++ {
		validator := &pb.ValidatorRecord{StartDynasty: 1, EndDynasty: 100}
		validators = append(validators, validator)
	}

	indices, err := ShuffleValidatorsToCommittees(common.Hash{'A'}, validators, 1, 0)
	if err != nil {
		t.Errorf("validatorsBySlotShard failed with %v:", err)
	}
	if len(indices) != int(params.GetConfig().CycleLength) {
		t.Errorf("incorret length for validator indices. Want: %d. Got: %d", params.GetConfig().CycleLength, len(indices))
	}
}

func TestGetCommitteeParamsSmallValidatorSet(t *testing.T) {
	numValidators := int(params.GetConfig().CycleLength * params.GetConfig().MinCommiteeSize / 4)

	committesPerSlot, slotsPerCommittee := getCommitteeParams(numValidators)
	if committesPerSlot != 1 {
		t.Fatalf("Expected committeesPerSlot to equal %d: got %d", 1, committesPerSlot)
	}

	if slotsPerCommittee != 4 {
		t.Fatalf("Expected slotsPerCommittee to equal %d: got %d", 4, slotsPerCommittee)
	}
}

func TestGetCommitteeParamsRegularValidatorSet(t *testing.T) {
	numValidators := int(params.GetConfig().CycleLength * params.GetConfig().MinCommiteeSize)

	committesPerSlot, slotsPerCommittee := getCommitteeParams(numValidators)
	if committesPerSlot != 1 {
		t.Fatalf("Expected committeesPerSlot to equal %d: got %d", 1, committesPerSlot)
	}

	if slotsPerCommittee != 1 {
		t.Fatalf("Expected slotsPerCommittee to equal %d: got %d", 1, slotsPerCommittee)
	}
}

func TestGetCommitteeParamsLargeValidatorSet(t *testing.T) {
	numValidators := int(params.GetConfig().CycleLength*params.GetConfig().MinCommiteeSize) * 8

	committesPerSlot, slotsPerCommittee := getCommitteeParams(numValidators)
	if committesPerSlot != 5 {
		t.Fatalf("Expected committeesPerSlot to equal %d: got %d", 5, committesPerSlot)
	}

	if slotsPerCommittee != 1 {
		t.Fatalf("Expected slotsPerCommittee to equal %d: got %d", 1, slotsPerCommittee)
	}
}

func TestValidatorsBySlotShardRegularValidatorSet(t *testing.T) {
	validatorIndices := []uint32{}
	numValidators := int(params.GetConfig().CycleLength * params.GetConfig().MinCommiteeSize)
	for i := 0; i < numValidators; i++ {
		validatorIndices = append(validatorIndices, uint32(i))
	}

	shardAndCommitteeArray := splitBySlotShard(validatorIndices, 0)

	if len(shardAndCommitteeArray) != int(params.GetConfig().CycleLength) {
		t.Fatalf("Expected length %d: got %d", params.GetConfig().CycleLength, len(shardAndCommitteeArray))
	}

	for i := 0; i < len(shardAndCommitteeArray); i++ {
		shardAndCommittees := shardAndCommitteeArray[i].ArrayShardAndCommittee
		if len(shardAndCommittees) != 1 {
			t.Fatalf("Expected %d committee per slot: got %d", params.GetConfig().MinCommiteeSize, 1)
		}

		committeeSize := len(shardAndCommittees[0].Committee)
		if committeeSize != int(params.GetConfig().MinCommiteeSize) {
			t.Fatalf("Expected committee size %d: got %d", params.GetConfig().MinCommiteeSize, committeeSize)
		}
	}
}

func TestValidatorsBySlotShardLargeValidatorSet(t *testing.T) {
	validatorIndices := []uint32{}
	numValidators := int(params.GetConfig().CycleLength*params.GetConfig().MinCommiteeSize) * 2
	for i := 0; i < numValidators; i++ {
		validatorIndices = append(validatorIndices, uint32(i))
	}

	shardAndCommitteeArray := splitBySlotShard(validatorIndices, 0)

	if len(shardAndCommitteeArray) != int(params.GetConfig().CycleLength) {
		t.Fatalf("Expected length %d: got %d", params.GetConfig().CycleLength, len(shardAndCommitteeArray))
	}

	for i := 0; i < len(shardAndCommitteeArray); i++ {
		shardAndCommittees := shardAndCommitteeArray[i].ArrayShardAndCommittee
		if len(shardAndCommittees) != 2 {
			t.Fatalf("Expected %d committee per slot: got %d", params.GetConfig().MinCommiteeSize, 2)
		}

		t.Logf("slot %d", i)
		for j := 0; j < len(shardAndCommittees); j++ {
			shardCommittee := shardAndCommittees[j]
			t.Logf("shard %d", shardCommittee.ShardId)
			t.Logf("committee: %v", shardCommittee.Committee)
			if len(shardCommittee.Committee) != int(params.GetConfig().MinCommiteeSize) {
				t.Fatalf("Expected committee size %d: got %d", params.GetConfig().MinCommiteeSize, len(shardCommittee.Committee))
			}
		}

	}
}

func TestValidatorsBySlotShardSmallValidatorSet(t *testing.T) {
	validatorIndices := []uint32{}
	numValidators := int(params.GetConfig().CycleLength*params.GetConfig().MinCommiteeSize) / 2
	for i := 0; i < numValidators; i++ {
		validatorIndices = append(validatorIndices, uint32(i))
	}

	shardAndCommitteeArray := splitBySlotShard(validatorIndices, 0)

	if len(shardAndCommitteeArray) != int(params.GetConfig().CycleLength) {
		t.Fatalf("Expected length %d: got %d", params.GetConfig().CycleLength, len(shardAndCommitteeArray))
	}

	for i := 0; i < len(shardAndCommitteeArray); i++ {
		shardAndCommittees := shardAndCommitteeArray[i].ArrayShardAndCommittee
		if len(shardAndCommittees) != 1 {
			t.Fatalf("Expected %d committee per slot: got %d", params.GetConfig().MinCommiteeSize, 1)
		}

		for j := 0; j < len(shardAndCommittees); j++ {
			shardCommittee := shardAndCommittees[j]
			if len(shardCommittee.Committee) != int(params.GetConfig().MinCommiteeSize/2) {
				t.Fatalf("Expected committee size %d: got %d", params.GetConfig().MinCommiteeSize/2, len(shardCommittee.Committee))
			}
		}
	}
}
