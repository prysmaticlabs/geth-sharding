package balances

import (
	"fmt"

	"github.com/prysmaticlabs/prysm/beacon-chain/core/epoch"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/validators"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	"github.com/prysmaticlabs/prysm/shared/mathutil"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/slices"
)

// baseRewardQuotient takes the total balance and calculates for
// the quotient of the base reward.
//
// Spec pseudocode definition:
//    base_reward_quotient =
//    	BASE_REWARD_QUOTIENT * integer_squareroot(total_balance // GWEI_PER_ETH)
func baseRewardQuotient(totalBalance uint64) uint64 {

	baseRewardQuotient := params.BeaconConfig().BaseRewardQuotient * mathutil.IntegerSquareRoot(
		totalBalance/params.BeaconConfig().Gwei)

	return baseRewardQuotient
}

// baseReward takes state and validator index to calculate for
// individual validator's base reward.
//
// Spec pseudocode definition:
//    base_reward(state, index) =
//    	get_effective_balance(state, index) // base_reward_quotient // 5
func baseReward(
	state *pb.BeaconState,
	validatorIndex uint32,
	baseRewardQuotient uint64) uint64 {

	validatorBalance := validators.EffectiveBalance(state, validatorIndex)
	return validatorBalance / baseRewardQuotient / 5
}

// inactivityPenalty takes state and validator index to calculate for
// individual validator's penalty for being offline.
//
// Spec pseudocode definition:
//    inactivity_penalty(state, index, epochs_since_finality) =
//    	base_reward(state, index) + get_effective_balance(state, index)
//    	* epochs_since_finality // INACTIVITY_PENALTY_QUOTIENT // 2
func inactivityPenalty(
	state *pb.BeaconState,
	validatorIndex uint32,
	baseRewardQuotient uint64,
	epochsSinceFinality uint64) uint64 {

	baseReward := baseReward(state, validatorIndex, baseRewardQuotient)
	validatorBalance := validators.EffectiveBalance(state, validatorIndex)
	return baseReward + validatorBalance*epochsSinceFinality/params.BeaconConfig().InactivityPenaltyQuotient/2
}

// FFGSrcRewardsPenalties applies rewards or penalties
// for an expected FFG source. It uses total justified
// attesting balances, total validator balances and base
// reward quotient to calculate the reward amount.
// Validators who voted for previous justified hash
// will get a reward, everyone else will get a penalty.
//
// Spec pseudocode definition:
//    Any validator index in previous_epoch_justified_attester_indices
//    gains base_reward(state, index) * previous_epoch_justified_attesting_balance // total_balance.
//	  Any active validator v not in previous_epoch_justified_attester_indices
//	  loses base_reward(state, index).
func FFGSrcRewardsPenalties(
	state *pb.BeaconState,
	justifiedAttesterIndices []uint32,
	justifiedAttestingBalance uint64,
	totalBalance uint64) *pb.BeaconState {

	baseRewardQuotient := baseRewardQuotient(totalBalance)

	for _, index := range justifiedAttesterIndices {
		state.ValidatorBalances[index] +=
			baseReward(state, index, baseRewardQuotient) *
				justifiedAttestingBalance /
				totalBalance
	}

	allValidatorIndices := validators.AllActiveValidatorsIndices(state)
	didNotAttestIndices := slices.Not(justifiedAttesterIndices, allValidatorIndices)

	for _, index := range didNotAttestIndices {
		state.ValidatorBalances[index] -=
			baseReward(state, index, baseRewardQuotient)
	}
	return state
}

// FFGTargetRewardsPenalties applies rewards or penalties
// for an expected FFG target. It uses total boundary
// attesting balances, total validator balances and base
// reward quotient to calculate the reward amount.
// Validators who voted for epoch boundary block
// will get a reward, everyone else will get a penalty.
//
// Spec pseudocode definition:
//    Any validator index in previous_epoch_boundary_attester_indices gains
//    base_reward(state, index) * previous_epoch_boundary_attesting_balance // total_balance.
//	  Any active validator index not in previous_epoch_boundary_attester_indices loses
//	  base_reward(state, index).
func FFGTargetRewardsPenalties(
	state *pb.BeaconState,
	boundaryAttesterIndices []uint32,
	boundaryAttestingBalance uint64,
	totalBalance uint64) *pb.BeaconState {

	baseRewardQuotient := baseRewardQuotient(totalBalance)

	for _, index := range boundaryAttesterIndices {
		state.ValidatorBalances[index] +=
			baseReward(state, index, baseRewardQuotient) *
				boundaryAttestingBalance /
				totalBalance
	}

	allValidatorIndices := validators.AllActiveValidatorsIndices(state)
	didNotAttestIndices := slices.Not(boundaryAttesterIndices, allValidatorIndices)

	for _, index := range didNotAttestIndices {
		state.ValidatorBalances[index] -=
			baseReward(state, index, baseRewardQuotient)
	}
	return state
}

// ChainHeadRewardsPenalties applies rewards or penalties
// for an expected beacon chain head. It uses total head
// attesting balances, total validator balances and base
// reward quotient to calculate the reward amount.
// Validators who voted for the canonical head block
// will get a reward, everyone else will get a penalty.
//
// Spec pseudocode definition:
//    Any validator index in previous_epoch_head_attester_indices gains
//    base_reward(state, index) * previous_epoch_head_attesting_balance // total_balance).
//    Any active validator index not in previous_epoch_head_attester_indices loses
//    base_reward(state, index).
func ChainHeadRewardsPenalties(
	state *pb.BeaconState,
	headAttesterIndices []uint32,
	headAttestingBalance uint64,
	totalBalance uint64) *pb.BeaconState {

	baseRewardQuotient := baseRewardQuotient(totalBalance)

	for _, index := range headAttesterIndices {
		state.ValidatorBalances[index] +=
			baseReward(state, index, baseRewardQuotient) *
				headAttestingBalance /
				totalBalance
	}

	allValidatorIndices := validators.AllActiveValidatorsIndices(state)
	didNotAttestIndices := slices.Not(headAttesterIndices, allValidatorIndices)

	for _, index := range didNotAttestIndices {
		state.ValidatorBalances[index] -=
			baseReward(state, index, baseRewardQuotient)
	}
	return state
}

// InclusionDistRewards applies rewards based on
// inclusion distance. It uses calculated inclusion distance
// and base reward quotient to calculate the reward amount.
//
// Spec pseudocode definition:
//    Any validator index in previous_epoch_attester_indices gains
//    base_reward(state, index) * MIN_ATTESTATION_INCLUSION_DELAY //
//    inclusion_distance(state, index)
func InclusionDistRewards(
	state *pb.BeaconState,
	attesterIndices []uint32,
	totalBalance uint64) (*pb.BeaconState, error) {

	baseRewardQuotient := baseRewardQuotient(totalBalance)

	for _, index := range attesterIndices {
		inclusionDistance, err := epoch.InclusionDistance(state, index)
		if err != nil {
			return nil, fmt.Errorf("could not get inclusion distance: %v", err)
		}
		state.ValidatorBalances[index] +=
			baseReward(state, index, baseRewardQuotient) *
				params.BeaconConfig().MinAttestationInclusionDelay /
				inclusionDistance
	}
	return state, nil
}
