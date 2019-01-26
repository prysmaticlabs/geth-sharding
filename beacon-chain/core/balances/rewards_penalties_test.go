package balances

import (
	"reflect"
	"testing"

	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	"github.com/prysmaticlabs/prysm/shared/params"
)

func TestBaseRewardQuotient(t *testing.T) {
	if params.BeaconConfig().BaseRewardQuotient != 1<<10 {
		t.Errorf("BaseRewardQuotient should be 1024 for these tests to pass")
	}

	tests := []struct {
		a uint64
		b uint64
	}{
		{0, 0},
		{1e6 * 1e9, 30881},   //1M ETH staked, 9.76% interest.
		{2e6 * 1e9, 43673},   //2M ETH staked, 6.91% interest.
		{5e6 * 1e9, 69053},   //5M ETH staked, 4.36% interest.
		{10e6 * 1e9, 97656},  // 10M ETH staked, 3.08% interest.
		{20e6 * 1e9, 138106}, // 20M ETH staked, 2.18% interest.
	}
	for _, tt := range tests {
		b := baseRewardQuotient(tt.a)
		if b != tt.b {
			t.Errorf("BaseRewardQuotient(%d) = %d, want = %d",
				tt.a, b, tt.b)
		}
	}
}

func TestBaseReward(t *testing.T) {
	tests := []struct {
		a uint64
		b uint64
	}{
		{0, 0},
		{params.BeaconConfig().MinDepositInGwei, 61},
		{30 * 1e9, 1853},
		{params.BeaconConfig().MaxDepositInGwei, 1976},
		{40 * 1e9, 1976},
	}
	for _, tt := range tests {
		state := &pb.BeaconState{
			ValidatorBalances: []uint64{tt.a},
		}
		// Assume 10M Eth staked (base reward quotient: 3237888).
		b := baseReward(state, 0, 3237888)
		if b != tt.b {
			t.Errorf("BaseReward(%d) = %d, want = %d",
				tt.a, b, tt.b)
		}
	}
}

func TestInactivityPenalty(t *testing.T) {
	tests := []struct {
		a uint64
		b uint64
	}{
		{1, 2929},
		{2, 3883},
		{5, 6744},
		{10, 11512},
		{50, 49659},
	}
	for _, tt := range tests {
		state := &pb.BeaconState{
			ValidatorBalances: []uint64{params.BeaconConfig().MaxDepositInGwei},
		}
		// Assume 10 ETH staked (base reward quotient: 3237888).
		b := inactivityPenalty(state, 0, 3237888, tt.a)
		if b != tt.b {
			t.Errorf("InactivityPenalty(%d) = %d, want = %d",
				tt.a, b, tt.b)
		}
	}
}

func TestFFGSrcRewardsPenalties(t *testing.T) {
	tests := []struct {
		voted                          []uint32
		balanceAfterSrcRewardPenalties []uint64
	}{
		// voted represents the validator indices that voted for FFG source,
		// balanceAfterSrcRewardPenalties represents their final balances,
		// validators who voted should get an increase, who didn't should get a decrease.
		{[]uint32{}, []uint64{31981661892, 31981661892, 31981661892, 31981661892}},
		{[]uint32{0, 1}, []uint64{32009169054, 32009169054, 31981661892, 31981661892}},
		{[]uint32{0, 1, 2, 3}, []uint64{32018338108, 32018338108, 32018338108, 32018338108}},
	}
	for _, tt := range tests {
		validatorBalances := make([]uint64, 4)
		for i := 0; i < len(validatorBalances); i++ {
			validatorBalances[i] = params.BeaconConfig().MaxDepositInGwei
		}
		state := &pb.BeaconState{
			ValidatorRegistry: []*pb.ValidatorRecord{
				{ExitSlot: params.BeaconConfig().FarFutureSlot},
				{ExitSlot: params.BeaconConfig().FarFutureSlot},
				{ExitSlot: params.BeaconConfig().FarFutureSlot},
				{ExitSlot: params.BeaconConfig().FarFutureSlot},
			},
			ValidatorBalances: validatorBalances,
		}
		state = ExpectedFFGSource(
			state,
			tt.voted,
			uint64(len(tt.voted))*params.BeaconConfig().MaxDepositInGwei,
			uint64(len(validatorBalances))*params.BeaconConfig().MaxDepositInGwei)

		if !reflect.DeepEqual(state.ValidatorBalances, tt.balanceAfterSrcRewardPenalties) {
			t.Errorf("FFGSrcRewardsPenalties(%v) = %v, wanted: %v",
				tt.voted, state.ValidatorBalances, tt.balanceAfterSrcRewardPenalties)
		}
	}
}

func TestFFGTargetRewardsPenalties(t *testing.T) {
	tests := []struct {
		voted                          []uint32
		balanceAfterTgtRewardPenalties []uint64
	}{
		// voted represents the validator indices that voted for FFG target,
		// balanceAfterTgtRewardPenalties represents their final balances,
		// validators who voted should get an increase, who didn't should get a decrease.
		{[]uint32{}, []uint64{31981661892, 31981661892, 31981661892, 31981661892}},
		{[]uint32{0, 1}, []uint64{32009169054, 32009169054, 31981661892, 31981661892}},
		{[]uint32{0, 1, 2, 3}, []uint64{32018338108, 32018338108, 32018338108, 32018338108}},
	}
	for _, tt := range tests {
		validatorBalances := make([]uint64, 4)
		for i := 0; i < len(validatorBalances); i++ {
			validatorBalances[i] = params.BeaconConfig().MaxDepositInGwei
		}
		state := &pb.BeaconState{
			ValidatorRegistry: []*pb.ValidatorRecord{
				{ExitSlot: params.BeaconConfig().FarFutureSlot},
				{ExitSlot: params.BeaconConfig().FarFutureSlot},
				{ExitSlot: params.BeaconConfig().FarFutureSlot},
				{ExitSlot: params.BeaconConfig().FarFutureSlot},
			},
			ValidatorBalances: validatorBalances,
		}
		state = ExpectedFFGTarget(
			state,
			tt.voted,
			uint64(len(tt.voted))*params.BeaconConfig().MaxDepositInGwei,
			uint64(len(validatorBalances))*params.BeaconConfig().MaxDepositInGwei)

		if !reflect.DeepEqual(state.ValidatorBalances, tt.balanceAfterTgtRewardPenalties) {
			t.Errorf("FFGTargetRewardsPenalties(%v) = %v, wanted: %v",
				tt.voted, state.ValidatorBalances, tt.balanceAfterTgtRewardPenalties)
		}
	}
}

func TestChainHeadRewardsPenalties(t *testing.T) {
	tests := []struct {
		voted                           []uint32
		balanceAfterHeadRewardPenalties []uint64
	}{
		// voted represents the validator indices that voted for canonical chain,
		// balanceAfterHeadRewardPenalties represents their final balances,
		// validators who voted should get an increase, who didn't should get a decrease.
		{[]uint32{}, []uint64{31981661892, 31981661892, 31981661892, 31981661892}},
		{[]uint32{0, 1}, []uint64{32009169054, 32009169054, 31981661892, 31981661892}},
		{[]uint32{0, 1, 2, 3}, []uint64{32018338108, 32018338108, 32018338108, 32018338108}},
	}
	for _, tt := range tests {
		validatorBalances := make([]uint64, 4)
		for i := 0; i < len(validatorBalances); i++ {
			validatorBalances[i] = params.BeaconConfig().MaxDepositInGwei
		}
		state := &pb.BeaconState{
			ValidatorRegistry: []*pb.ValidatorRecord{
				{ExitSlot: params.BeaconConfig().FarFutureSlot},
				{ExitSlot: params.BeaconConfig().FarFutureSlot},
				{ExitSlot: params.BeaconConfig().FarFutureSlot},
				{ExitSlot: params.BeaconConfig().FarFutureSlot},
			},
			ValidatorBalances: validatorBalances,
		}
		state = ExpectedBeaconChainHead(
			state,
			tt.voted,
			uint64(len(tt.voted))*params.BeaconConfig().MaxDepositInGwei,
			uint64(len(validatorBalances))*params.BeaconConfig().MaxDepositInGwei)

		if !reflect.DeepEqual(state.ValidatorBalances, tt.balanceAfterHeadRewardPenalties) {
			t.Errorf("ChainHeadRewardsPenalties(%v) = %v, wanted: %v",
				tt.voted, state.ValidatorBalances, tt.balanceAfterHeadRewardPenalties)
		}
	}
}

func TestInclusionDistRewards_Ok(t *testing.T) {
	validators := make([]*pb.ValidatorRecord, config.EpochLength*4)
	for i := 0; i < len(validators); i++ {
		validators[i] = &pb.ValidatorRecord{
			ExitSlot: config.FarFutureSlot,
		}
	}

	attestation := []*pb.PendingAttestationRecord{
		{Data: &pb.AttestationData{Slot: 0},
			ParticipationBitfield: []byte{0xff},
			SlotIncluded:          5},
	}

	tests := []struct {
		voted []uint32
	}{
		{[]uint32{}},
		{[]uint32{237, 224}},
		{[]uint32{237, 224, 2, 242}},
	}
	for _, tt := range tests {
		validatorBalances := make([]uint64, config.EpochLength*4)
		for i := 0; i < len(validatorBalances); i++ {
			validatorBalances[i] = params.BeaconConfig().MaxDepositInGwei
		}
		state := &pb.BeaconState{
			ValidatorRegistry:  validators,
			ValidatorBalances:  validatorBalances,
			LatestAttestations: attestation,
		}
		state, err := InclusionDistance(
			state,
			tt.voted,
			uint64(len(validatorBalances))*params.BeaconConfig().MaxDepositInGwei)
		if err != nil {
			t.Fatalf("could not execute InclusionDistRewards:%v", err)
		}

		for _, i := range tt.voted {
			validatorBalances[i] = 32000055555
		}

		if !reflect.DeepEqual(state.ValidatorBalances, validatorBalances) {
			t.Errorf("InclusionDistRewards(%v) = %v, wanted: %v",
				tt.voted, state.ValidatorBalances, validatorBalances)
		}
	}
}

func TestInclusionDistRewards_NotOk(t *testing.T) {
	validators := make([]*pb.ValidatorRecord, config.EpochLength*2)
	for i := 0; i < len(validators); i++ {
		validators[i] = &pb.ValidatorRecord{
			ExitSlot: config.FarFutureSlot,
		}
	}

	attestation := []*pb.PendingAttestationRecord{
		{Data: &pb.AttestationData{Shard: 1, Slot: 0},
			ParticipationBitfield: []byte{0xff}},
	}

	tests := []struct {
		voted                        []uint32
		balanceAfterInclusionRewards []uint64
	}{
		{[]uint32{0, 1, 2, 3}, []uint64{}},
	}
	for _, tt := range tests {
		state := &pb.BeaconState{
			ValidatorRegistry:  validators,
			LatestAttestations: attestation,
		}
		_, err := InclusionDistance(state, tt.voted, 0)
		if err == nil {
			t.Fatal("InclusionDistRewards should have failed")
		}
	}
}

func TestInactivityFFGSrcPenalty(t *testing.T) {
	tests := []struct {
		voted                     []uint32
		balanceAfterFFGSrcPenalty []uint64
		epochsSinceFinality       uint64
	}{
		// The higher the epochs since finality, the more penalties applied.
		{[]uint32{0, 1}, []uint64{32000000000, 32000000000, 31981657124, 31981657124}, 5},
		{[]uint32{}, []uint64{31981657124, 31981657124, 31981657124, 31981657124}, 5},
		{[]uint32{}, []uint64{31981652356, 31981652356, 31981652356, 31981652356}, 10},
		{[]uint32{}, []uint64{31981642819, 31981642819, 31981642819, 31981642819}, 20},
	}
	for _, tt := range tests {
		validatorBalances := make([]uint64, 4)
		for i := 0; i < len(validatorBalances); i++ {
			validatorBalances[i] = params.BeaconConfig().MaxDepositInGwei
		}
		state := &pb.BeaconState{
			ValidatorRegistry: []*pb.ValidatorRecord{
				{ExitSlot: params.BeaconConfig().FarFutureSlot},
				{ExitSlot: params.BeaconConfig().FarFutureSlot},
				{ExitSlot: params.BeaconConfig().FarFutureSlot},
				{ExitSlot: params.BeaconConfig().FarFutureSlot},
			},
			ValidatorBalances: validatorBalances,
		}
		state = InactivityFFGSource(
			state,
			tt.voted,
			uint64(len(validatorBalances))*params.BeaconConfig().MaxDepositInGwei,
			tt.epochsSinceFinality)

		if !reflect.DeepEqual(state.ValidatorBalances, tt.balanceAfterFFGSrcPenalty) {
			t.Errorf("InactivityFFGSrcPenalty(%v) = %v, wanted: %v",
				tt.voted, state.ValidatorBalances, tt.balanceAfterFFGSrcPenalty)
		}
	}
}

func TestInactivityFFGTargetPenalty(t *testing.T) {
	tests := []struct {
		voted                        []uint32
		balanceAfterFFGTargetPenalty []uint64
		epochsSinceFinality          uint64
	}{
		// The higher the epochs since finality, the more penalties applied.
		{[]uint32{0, 1}, []uint64{32000000000, 32000000000, 31981657124, 31981657124}, 5},
		{[]uint32{}, []uint64{31981657124, 31981657124, 31981657124, 31981657124}, 5},
		{[]uint32{}, []uint64{31981652356, 31981652356, 31981652356, 31981652356}, 10},
		{[]uint32{}, []uint64{31981642819, 31981642819, 31981642819, 31981642819}, 20},
	}
	for _, tt := range tests {
		validatorBalances := make([]uint64, 4)
		for i := 0; i < len(validatorBalances); i++ {
			validatorBalances[i] = params.BeaconConfig().MaxDepositInGwei
		}
		state := &pb.BeaconState{
			ValidatorRegistry: []*pb.ValidatorRecord{
				{ExitSlot: params.BeaconConfig().FarFutureSlot},
				{ExitSlot: params.BeaconConfig().FarFutureSlot},
				{ExitSlot: params.BeaconConfig().FarFutureSlot},
				{ExitSlot: params.BeaconConfig().FarFutureSlot},
			},
			ValidatorBalances: validatorBalances,
		}
		state = InactivityFFGTarget(
			state,
			tt.voted,
			uint64(len(validatorBalances))*params.BeaconConfig().MaxDepositInGwei,
			tt.epochsSinceFinality)

		if !reflect.DeepEqual(state.ValidatorBalances, tt.balanceAfterFFGTargetPenalty) {
			t.Errorf("InactivityFFGTargetPenalty(%v) = %v, wanted: %v",
				tt.voted, state.ValidatorBalances, tt.balanceAfterFFGTargetPenalty)
		}
	}
}

func TestInactivityHeadPenalty(t *testing.T) {
	tests := []struct {
		voted                             []uint32
		balanceAfterInactivityHeadPenalty []uint64
	}{
		{[]uint32{}, []uint64{31981661892, 31981661892, 31981661892, 31981661892}},
		{[]uint32{0, 1}, []uint64{32000000000, 32000000000, 31981661892, 31981661892}},
		{[]uint32{0, 1, 2, 3}, []uint64{32000000000, 32000000000, 32000000000, 32000000000}},
	}
	for _, tt := range tests {
		validatorBalances := make([]uint64, 4)
		for i := 0; i < len(validatorBalances); i++ {
			validatorBalances[i] = params.BeaconConfig().MaxDepositInGwei
		}
		state := &pb.BeaconState{
			ValidatorRegistry: []*pb.ValidatorRecord{
				{ExitSlot: params.BeaconConfig().FarFutureSlot},
				{ExitSlot: params.BeaconConfig().FarFutureSlot},
				{ExitSlot: params.BeaconConfig().FarFutureSlot},
				{ExitSlot: params.BeaconConfig().FarFutureSlot},
			},
			ValidatorBalances: validatorBalances,
		}
		state = InactivityChainHead(
			state,
			tt.voted,
			uint64(len(validatorBalances))*params.BeaconConfig().MaxDepositInGwei)

		if !reflect.DeepEqual(state.ValidatorBalances, tt.balanceAfterInactivityHeadPenalty) {
			t.Errorf("InactivityHeadPenalty(%v) = %v, wanted: %v",
				tt.voted, state.ValidatorBalances, tt.balanceAfterInactivityHeadPenalty)
		}
	}
}

func TestInactivityExitedPenality(t *testing.T) {
	tests := []struct {
		balanceAfterExitedPenalty []uint64
		epochsSinceFinality       uint64
	}{
		{[]uint64{31944976140, 31944976140, 31944976140, 31944976140}, 5},
		{[]uint64{31944966604, 31944966604, 31944966604, 31944966604}, 10},
		{[]uint64{31944032002, 31944032002, 31944032002, 31944032002}, 500},
	}
	for _, tt := range tests {
		validatorBalances := make([]uint64, 4)
		for i := 0; i < len(validatorBalances); i++ {
			validatorBalances[i] = params.BeaconConfig().MaxDepositInGwei
		}
		state := &pb.BeaconState{
			ValidatorRegistry: []*pb.ValidatorRecord{
				{ExitSlot: params.BeaconConfig().FarFutureSlot},
				{ExitSlot: params.BeaconConfig().FarFutureSlot},
				{ExitSlot: params.BeaconConfig().FarFutureSlot},
				{ExitSlot: params.BeaconConfig().FarFutureSlot}},
			ValidatorBalances: validatorBalances,
		}
		state = InactivityExitedPenalties(
			state,
			uint64(len(validatorBalances))*params.BeaconConfig().MaxDepositInGwei,
			tt.epochsSinceFinality,
		)

		if !reflect.DeepEqual(state.ValidatorBalances, tt.balanceAfterExitedPenalty) {
			t.Errorf("InactivityExitedPenalty(epochSinceFinality=%v) = %v, wanted: %v",
				tt.epochsSinceFinality, state.ValidatorBalances, tt.balanceAfterExitedPenalty)
		}
	}
}

func TestInactivityInclusionPenalty_Ok(t *testing.T) {
	validators := make([]*pb.ValidatorRecord, config.EpochLength*4)
	for i := 0; i < len(validators); i++ {
		validators[i] = &pb.ValidatorRecord{
			ExitSlot: config.FarFutureSlot,
		}
	}
	attestation := []*pb.PendingAttestationRecord{
		{Data: &pb.AttestationData{Slot: 0},
			ParticipationBitfield: []byte{0xff},
			SlotIncluded:          5},
	}

	tests := []struct {
		voted []uint32
	}{
		{[]uint32{}},
		{[]uint32{237, 224}},
		{[]uint32{237, 224, 2, 242}},
	}
	for _, tt := range tests {
		validatorBalances := make([]uint64, config.EpochLength*4)
		for i := 0; i < len(validatorBalances); i++ {
			validatorBalances[i] = params.BeaconConfig().MaxDepositInGwei
		}
		state := &pb.BeaconState{
			ValidatorRegistry:  validators,
			ValidatorBalances:  validatorBalances,
			LatestAttestations: attestation,
		}
		state, err := InactivityInclusionDistance(
			state,
			tt.voted,
			uint64(len(validatorBalances))*params.BeaconConfig().MaxDepositInGwei)

		for _, i := range tt.voted {
			validatorBalances[i] = 32000055555
		}

		if err != nil {
			t.Fatalf("could not execute InactivityInclusionPenalty:%v", err)
		}
		if !reflect.DeepEqual(state.ValidatorBalances, validatorBalances) {
			t.Errorf("InactivityInclusionPenalty(%v) = %v, wanted: %v",
				tt.voted, state.ValidatorBalances, validatorBalances)
		}
	}
}

func TestInactivityInclusionPenalty_NotOk(t *testing.T) {
	validators := make([]*pb.ValidatorRecord, config.EpochLength*2)
	for i := 0; i < len(validators); i++ {
		validators[i] = &pb.ValidatorRecord{
			ExitSlot: config.FarFutureSlot,
		}
	}
	attestation := []*pb.PendingAttestationRecord{
		{Data: &pb.AttestationData{Shard: 1, Slot: 0},
			ParticipationBitfield: []byte{0xff}},
	}

	tests := []struct {
		voted                        []uint32
		balanceAfterInclusionRewards []uint64
	}{
		{[]uint32{0, 1, 2, 3}, []uint64{}},
	}
	for _, tt := range tests {
		state := &pb.BeaconState{
			ValidatorRegistry:  validators,
			LatestAttestations: attestation,
		}
		_, err := InactivityInclusionDistance(state, tt.voted, 0)
		if err == nil {
			t.Fatal("InclusionDistRewards should have failed")
		}
	}
}

func TestAttestationInclusionRewards(t *testing.T) {
	validators := make([]*pb.ValidatorRecord, config.EpochLength*4)
	for i := 0; i < len(validators); i++ {
		validators[i] = &pb.ValidatorRecord{
			ExitSlot: config.FarFutureSlot,
		}
	}

	attestation := []*pb.PendingAttestationRecord{
		{Data: &pb.AttestationData{Slot: 0},
			ParticipationBitfield: []byte{0xff},
			SlotIncluded:          0},
	}

	tests := []struct {
		voted []uint32
	}{
		{[]uint32{}},
		{[]uint32{237}},
	}
	for _, tt := range tests {
		validatorBalances := make([]uint64, config.EpochLength*4)
		for i := 0; i < len(validatorBalances); i++ {
			validatorBalances[i] = params.BeaconConfig().MaxDepositInGwei
		}
		state := &pb.BeaconState{
			ValidatorRegistry:  validators,
			ValidatorBalances:  validatorBalances,
			LatestAttestations: attestation,
		}
		state, err := AttestationInclusion(
			state,
			uint64(len(validatorBalances))*params.BeaconConfig().MaxDepositInGwei,
			tt.voted)

		for _, i := range tt.voted {
			validatorBalances[i] = 32000008680
		}

		if err != nil {
			t.Fatalf("could not execute InactivityInclusionPenalty:%v", err)
		}
		if !reflect.DeepEqual(state.ValidatorBalances, validatorBalances) {
			t.Errorf("AttestationInclusionRewards(%v) = %v, wanted: %v",
				tt.voted, state.ValidatorBalances, validatorBalances)
		}
	}
}

func TestAttestationInclusionRewards_NoInclusionSlot(t *testing.T) {
	validators := make([]*pb.ValidatorRecord, config.EpochLength*2)
	for i := 0; i < len(validators); i++ {
		validators[i] = &pb.ValidatorRecord{
			ExitSlot: config.FarFutureSlot,
		}
	}

	tests := []struct {
		voted                            []uint32
		balanceAfterAttestationInclusion []uint64
	}{
		{[]uint32{0, 1, 2, 3}, []uint64{32000000000, 32000000000, 32000000000, 32000000000}},
	}
	for _, tt := range tests {
		validatorBalances := make([]uint64, 4)
		for i := 0; i < len(validatorBalances); i++ {
			validatorBalances[i] = params.BeaconConfig().MaxDepositInGwei
		}
		state := &pb.BeaconState{
			ValidatorRegistry: validators,
			ValidatorBalances: validatorBalances,
		}
		if _, err := AttestationInclusion(state, 0, tt.voted); err == nil {
			t.Fatal("AttestationInclusionRewards should have failed with no inclusion slot")
		}
	}
}

func TestAttestationInclusionRewards_NoProposerIndex(t *testing.T) {
	validators := make([]*pb.ValidatorRecord, config.EpochLength*2)
	for i := 0; i < len(validators); i++ {
		validators[i] = &pb.ValidatorRecord{
			ExitSlot: config.FarFutureSlot,
		}
	}
	attestation := []*pb.PendingAttestationRecord{
		{Data: &pb.AttestationData{Shard: 1, Slot: 0},
			ParticipationBitfield: []byte{0xff},
			SlotIncluded:          0},
	}

	tests := []struct {
		voted                            []uint32
		balanceAfterAttestationInclusion []uint64
	}{
		{[]uint32{0}, []uint64{32000071022, 32000000000, 32000000000, 32000000000}},
	}
	for _, tt := range tests {
		validatorBalances := make([]uint64, 4)
		for i := 0; i < len(validatorBalances); i++ {
			validatorBalances[i] = params.BeaconConfig().MaxDepositInGwei
		}
		state := &pb.BeaconState{
			Slot:               1000,
			ValidatorRegistry:  validators,
			ValidatorBalances:  validatorBalances,
			LatestAttestations: attestation,
		}
		if _, err := AttestationInclusion(state, 0, tt.voted); err == nil {
			t.Fatal("AttestationInclusionRewards should have failed with no proposer index")
		}
	}
}

func TestCrosslinksRewardsPenalties(t *testing.T) {
	validators := make([]*pb.ValidatorRecord, config.EpochLength*4)
	for i := 0; i < len(validators); i++ {
		validators[i] = &pb.ValidatorRecord{
			ExitSlot: config.FarFutureSlot,
		}
	}

	tests := []struct {
		voted                        []byte
		balanceAfterCrosslinkRewards []uint64
	}{
		{[]byte{0x0}, []uint64{
			32 * 1e9, 32 * 1e9, 32 * 1e9, 32 * 1e9, 32 * 1e9, 32 * 1e9, 32 * 1e9, 32 * 1e9}},
		{[]byte{0xF}, []uint64{
			31585730498, 31585730498, 31585730498, 31585730498,
			32416931985, 32416931985, 32416931985, 32416931985}},
		{[]byte{0xFF}, []uint64{
			32829149760, 32829149760, 32829149760, 32829149760,
			32829149760, 32829149760, 32829149760, 32829149760}},
	}
	for _, tt := range tests {
		validatorBalances := make([]uint64, config.EpochLength*4)
		for i := 0; i < len(validatorBalances); i++ {
			validatorBalances[i] = params.BeaconConfig().MaxDepositInGwei
		}
		attestation := []*pb.PendingAttestationRecord{
			{Data: &pb.AttestationData{Shard: 1, Slot: 0},
				ParticipationBitfield: tt.voted,
				SlotIncluded:          0},
		}
		state := &pb.BeaconState{
			ValidatorRegistry:  validators,
			ValidatorBalances:  validatorBalances,
			LatestAttestations: attestation,
		}
		state, err := Crosslinks(
			state,
			attestation,
			nil)
		if err != nil {
			t.Fatalf("Could not apply Crosslinks rewards: %v", err)
		}
		if !reflect.DeepEqual(state.ValidatorBalances, validatorBalances) {
			t.Errorf("CrosslinksRewardsPenalties(%v) = %v, wanted: %v",
				tt.voted, state.ValidatorBalances, validatorBalances)
		}
	}
}
