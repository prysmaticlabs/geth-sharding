// Package validators contains libraries to shuffle validators
// and retrieve active validator indices from a given slot
// or an attestation. It also provides helper functions to locate
// validator based on pubic key.
package validators

import (
	"bytes"
	"fmt"
	"sync"

	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/sliceutil"
	"github.com/sirupsen/logrus"
)

var log = logrus.WithField("prefix", "validator")

type validatorStore struct {
	sync.RWMutex
	// activatedValidators is a mapping that tracks validator activation epoch to validators index.
	activatedValidators map[uint64][]uint64
	// exitedValidators is a mapping that tracks validator exit epoch to validators index.
	exitedValidators map[uint64][]uint64
}

//VStore validator map for quick
var VStore = validatorStore{
	activatedValidators: make(map[uint64][]uint64),
	exitedValidators:    make(map[uint64][]uint64),
}

// ValidatorIndices returns all the validator indices from the input attestations
// and state.
//
// Spec pseudocode definition:
//   Let attester_indices be the union of the validator
//   index sets given by [get_attestation_participants(state, a.data, a.aggregation_bitfield)
//   for a in attestations]
func ValidatorIndices(
	state *pb.BeaconState,
	attestations []*pb.PendingAttestation,
) ([]uint64, error) {

	var attesterIndicesIntersection []uint64
	for _, attestation := range attestations {
		attesterIndices, err := helpers.AttestingIndices(
			state,
			attestation.Data,
			attestation.AggregationBitfield)
		if err != nil {
			return nil, err
		}

		attesterIndicesIntersection = sliceutil.UnionUint64(attesterIndicesIntersection, attesterIndices)
	}

	return attesterIndicesIntersection, nil
}

// AttestingValidatorIndices returns the crosslink committee validator indices
// if the validators from crosslink committee is part of the input attestations.
//
// Spec pseudocode definition:
// Let attesting_validator_indices(crosslink_committee, shard_block_root)
// 	be the union of the validator index sets given by
// 	[get_attestation_participants(state, a.data, a.participation_bitfield)
// 	for a in current_epoch_attestations + previous_epoch_attestations
// 		if a.shard == shard_committee.shard and a.shard_block_root == shard_block_root]
func AttestingValidatorIndices(
	state *pb.BeaconState,
	shard uint64,
	crosslinkDataRoot []byte,
	thisEpochAttestations []*pb.PendingAttestation,
	prevEpochAttestations []*pb.PendingAttestation) ([]uint64, error) {

	var validatorIndicesCommittees []uint64
	attestations := append(thisEpochAttestations, prevEpochAttestations...)

	for _, attestation := range attestations {
		if attestation.Data.Shard == shard &&
			bytes.Equal(attestation.Data.CrosslinkDataRoot, crosslinkDataRoot) {

			validatorIndicesCommittee, err := helpers.AttestingIndices(state, attestation.Data, attestation.AggregationBitfield)
			if err != nil {
				return nil, fmt.Errorf("could not get attester indices: %v", err)
			}
			validatorIndicesCommittees = sliceutil.UnionUint64(validatorIndicesCommittees, validatorIndicesCommittee)
		}
	}
	return validatorIndicesCommittees, nil
}

// ProcessDeposit mutates a corresponding index in the beacon state for
// a validator depositing ETH into the beacon chain. Specifically, this function
// adds a validator balance or tops up an existing validator's balance
// by some deposit amount. This function returns a mutated beacon state and
// the validator index corresponding to the validator in the processed
// deposit.
func ProcessDeposit(
	state *pb.BeaconState,
	validatorIdxMap map[[32]byte]int,
	pubkey []byte,
	amount uint64,
	_ /*proofOfPossession*/ []byte,
	withdrawalCredentials []byte,
) (*pb.BeaconState, error) {
	// TODO(#258): Validate proof of possession using BLS.
	var publicKeyExists bool
	var existingValidatorIdx int

	existingValidatorIdx, publicKeyExists = validatorIdxMap[bytesutil.ToBytes32(pubkey)]
	if !publicKeyExists {
		// If public key does not exist in the registry, we add a new validator
		// to the beacon state.
		newValidator := &pb.Validator{
			Pubkey:                pubkey,
			ActivationEpoch:       params.BeaconConfig().FarFutureEpoch,
			ExitEpoch:             params.BeaconConfig().FarFutureEpoch,
			WithdrawableEpoch:     params.BeaconConfig().FarFutureEpoch,
			SlashedEpoch:          params.BeaconConfig().FarFutureEpoch,
			StatusFlags:           0,
			WithdrawalCredentials: withdrawalCredentials,
		}
		state.ValidatorRegistry = append(state.ValidatorRegistry, newValidator)
		state.Balances = append(state.Balances, amount)
	} else {
		if !bytes.Equal(
			state.ValidatorRegistry[existingValidatorIdx].WithdrawalCredentials,
			withdrawalCredentials,
		) {
			return state, fmt.Errorf(
				"expected withdrawal credentials to match, received %#x == %#x",
				state.ValidatorRegistry[existingValidatorIdx].WithdrawalCredentials,
				withdrawalCredentials,
			)
		}
		state.Balances[existingValidatorIdx] += amount
	}
	state.DepositIndex++

	return state, nil
}

// ActivateValidator takes in validator index and updates
// validator's activation slot.
//
// Spec pseudocode definition:
// def activate_validator(state: BeaconState, index: ValidatorIndex, is_genesis: bool) -> None:
//    """
//    Activate the validator of the given ``index``.
//    Note that this function mutates ``state``.
//    """
//    validator = state.validator_registry[index]
//
//    validator.activation_epoch = GENESIS_EPOCH if is_genesis else get_entry_exit_effect_epoch(get_current_epoch(state))
func ActivateValidator(state *pb.BeaconState, idx uint64, genesis bool) (*pb.BeaconState, error) {
	validator := state.ValidatorRegistry[idx]
	if genesis {
		validator.ActivationEligibilityEpoch = params.BeaconConfig().GenesisEpoch
		validator.ActivationEpoch = params.BeaconConfig().GenesisEpoch
	} else {
		validator.ActivationEpoch = helpers.DelayedActivationExitEpoch(helpers.CurrentEpoch(state))
	}

	state.ValidatorRegistry[idx] = validator

	log.WithFields(logrus.Fields{
		"index":           idx,
		"activationEpoch": validator.ActivationEpoch - params.BeaconConfig().GenesisEpoch,
	}).Info("Validator activated")

	return state, nil
}

// InitiateValidatorExit takes in validator index and updates
// validator with INITIATED_EXIT status flag.
//
// Spec pseudocode definition:
// def initiate_validator_exit(state: BeaconState, index: ValidatorIndex) -> None:
//    """
//    Initiate the validator of the given ``index``.
//    Note that this function mutates ``state``.
//    """
//    # Return if validator already initiated exit
//    validator = state.validator_registry[index]
//    if validator.exit_epoch != FAR_FUTURE_EPOCH:
//        return
//
//    # Compute exit queue epoch
//    exit_epochs = [v.exit_epoch for v in state.validator_registry if v.exit_epoch != FAR_FUTURE_EPOCH]
//    exit_queue_epoch = max(exit_epochs + [get_delayed_activation_exit_epoch(get_current_epoch(state))])
//    exit_queue_churn = len([v for v in state.validator_registry if v.exit_epoch == exit_queue_epoch])
//    if exit_queue_churn >= get_churn_limit(state):
//        exit_queue_epoch += 1
//
//    # Set validator exit epoch and withdrawable epoch
//    validator.exit_epoch = exit_queue_epoch
//    validator.withdrawable_epoch = validator.exit_epoch + MIN_VALIDATOR_WITHDRAWABILITY_DELAY
func InitiateValidatorExit(state *pb.BeaconState, idx uint64) *pb.BeaconState {
	v := state.ValidatorRegistry[idx]

	// Return if validator already initiated exit.
	// According to the spec, this is not an assert condition and
	// shouldn't fail beacon block state transition.
	if v.ExitEpoch != params.BeaconConfig().FarFutureEpoch {
		return state
	}

	// Find the highest exit epoch among exited validators.
	highestExitEpoch := helpers.DelayedActivationExitEpoch(helpers.CurrentEpoch(state))
	for i := 0; i < len(state.ValidatorRegistry); i++ {
		if state.ValidatorRegistry[i].ExitEpoch != params.BeaconConfig().FarFutureEpoch {
			if highestExitEpoch < state.ValidatorRegistry[i].ExitEpoch {
				highestExitEpoch = state.ValidatorRegistry[i].ExitEpoch
			}
		}
	}

	// Find the total number of validators exiting same epoch as
	// input validator. If the number is greater than churn limit, postpone
	// exit epoch to the next epoch.
	var currentExitQueueLength uint64
	for i := 0; i < len(state.ValidatorRegistry); i++ {
		if state.ValidatorRegistry[i].ExitEpoch == highestExitEpoch {
			currentExitQueueLength++
		}
	}

	if currentExitQueueLength >= helpers.ChurnLimit(state) {
		highestExitEpoch++
	}

	v.ExitEpoch = highestExitEpoch
	v.WithdrawableEpoch = v.ExitEpoch + params.BeaconConfig().MinValidatorWithdrawalDelay

	state.ValidatorRegistry[idx] = v
	return state
}

// ExitValidator takes in validator index and does house
// keeping work to exit validator with entry exit delay.
//
// Spec pseudocode definition:
// def exit_validator(state: BeaconState, index: ValidatorIndex) -> None:
//    """
//    Exit the validator of the given ``index``.
//    Note that this function mutates ``state``.
//    """
//    validator = state.validator_registry[index]
//
//    # The following updates only occur if not previous exited
//    if validator.exit_epoch <= get_entry_exit_effect_epoch(get_current_epoch(state)):
//        return
//
//    validator.exit_epoch = get_entry_exit_effect_epoch(get_current_epoch(state))
func ExitValidator(state *pb.BeaconState, idx uint64) *pb.BeaconState {
	validator := state.ValidatorRegistry[idx]

	if validator.ExitEpoch != params.BeaconConfig().FarFutureEpoch {
		return state
	}
	validator.ExitEpoch = helpers.DelayedActivationExitEpoch(helpers.CurrentEpoch(state))
	return state
}

// SlashValidator slashes the malicious validator's balance and awards
// the whistleblower's balance.
//
// Spec pseudocode definition:
// def slash_validator(state: BeaconState, index: ValidatorIndex) -> None:
//    """
//    Slash the validator of the given ``index``.
//    Note that this function mutates ``state``.
//    """
//    validator = state.validator_registry[index]
//    state.latest_slashed_balances[get_current_epoch(state) % LATEST_PENALIZED_EXIT_LENGTH] += get_effective_balance(state, index)
//
//    whistleblower_index = get_beacon_proposer_index(state, state.slot)
//    whistleblower_reward = get_effective_balance(state, index) // WHISTLEBLOWER_REWARD_QUOTIENT
//    state.validator_balances[whistleblower_index] += whistleblower_reward
//    state.validator_balances[index] -= whistleblower_reward
//    validator.slashed_epoch = get_current_epoch(state)
func SlashValidator(state *pb.BeaconState, idx uint64) (*pb.BeaconState, error) {
	if state.Slot >= helpers.StartSlot(state.ValidatorRegistry[idx].WithdrawableEpoch) {
		return nil, fmt.Errorf("withdrawn validator %d could not get slashed, "+
			"current slot: %d, withdrawn slot %d",
			idx, state.Slot, helpers.StartSlot(state.ValidatorRegistry[idx].WithdrawableEpoch))
	}

	state = ExitValidator(state, idx)

	slashedDuration := helpers.CurrentEpoch(state) % params.BeaconConfig().LatestSlashedExitLength
	state.LatestSlashedBalances[slashedDuration] += helpers.EffectiveBalance(state, idx)

	whistleblowerIdx, err := helpers.BeaconProposerIndex(state)
	if err != nil {
		return nil, fmt.Errorf("could not get proposer idx: %v", err)
	}
	whistleblowerReward := helpers.EffectiveBalance(state, idx) /
		params.BeaconConfig().WhistleBlowingRewardQuotient

	state.Balances[whistleblowerIdx] += whistleblowerReward
	state.Balances[idx] -= whistleblowerReward

	state.ValidatorRegistry[idx].SlashedEpoch = helpers.CurrentEpoch(state) + params.BeaconConfig().LatestSlashedExitLength
	return state, nil
}

// ProcessPenaltiesAndExits prepares the validators and the slashed validators
// for withdrawal.
//
// Spec pseudocode definition:
// def process_penalties_and_exits(state: BeaconState) -> None:
//    """
//    Process the penalties and prepare the validators who are eligible to withdrawal.
//    Note that this function mutates ``state``.
//    """
//    current_epoch = get_current_epoch(state)
//    # The active validators
//    active_validator_indices = get_active_validator_indices(state.validator_registry, current_epoch)
//    # The total effective balance of active validators
//    total_balance = sum(get_effective_balance(state, i) for i in active_validator_indices)
//
//    for index, validator in enumerate(state.validator_registry):
//        if current_epoch == validator.slashed_epoch + LATEST_PENALIZED_EXIT_LENGTH // 2:
//            epoch_index = current_epoch % LATEST_PENALIZED_EXIT_LENGTH
//            total_at_start = state.latest_slashed_balances[(epoch_index + 1) % LATEST_PENALIZED_EXIT_LENGTH]
//            total_at_end = state.latest_slashed_balances[epoch_index]
//            total_penalties = total_at_end - total_at_start
//            penalty = get_effective_balance(state, index) * min(total_penalties * 3, total_balance) // total_balance
//            state.validator_balances[index] -= penalty
//
//    def eligible(index):
//        validator = state.validator_registry[index]
//        if validator.slashed_epoch <= current_epoch:
//            slashed_withdrawal_epochs = LATEST_PENALIZED_EXIT_LENGTH // 2
//            return current_epoch >= validator.slashed_epoch + slashd_withdrawal_epochs
//        else:
//            return current_epoch >= validator.exit_epoch + MIN_VALIDATOR_WITHDRAWAL_DELAY
//
//    all_indices = list(range(len(state.validator_registry)))
//    eligible_indices = filter(eligible, all_indices)
//    # Sort in order of exit epoch, and validators that exit within the same epoch exit in order of validator index
//    sorted_indices = sorted(eligible_indices, key=lambda index: state.validator_registry[index].exit_epoch)
//    withdrawn_so_far = 0
//    for index in sorted_indices:
//        prepare_validator_for_withdrawal(state, index)
//        withdrawn_so_far += 1
//        if withdrawn_so_far >= MAX_EXIT_DEQUEUES_PER_EPOCH:
//            break
func ProcessPenaltiesAndExits(state *pb.BeaconState) *pb.BeaconState {
	currentEpoch := helpers.CurrentEpoch(state)
	activeValidatorIndices := helpers.ActiveValidatorIndices(state, currentEpoch)
	totalBalance := helpers.TotalBalance(state, activeValidatorIndices)

	for idx, validator := range state.ValidatorRegistry {
		slashed := validator.SlashedEpoch +
			params.BeaconConfig().LatestSlashedExitLength/2
		if currentEpoch == slashed {
			slashedEpoch := currentEpoch % params.BeaconConfig().LatestSlashedExitLength
			slashedEpochStart := (slashedEpoch + 1) % params.BeaconConfig().LatestSlashedExitLength
			totalAtStart := state.LatestSlashedBalances[slashedEpochStart]
			totalAtEnd := state.LatestSlashedBalances[slashedEpoch]
			totalPenalties := totalAtStart - totalAtEnd

			penaltyMultiplier := totalPenalties * 3
			if totalBalance < penaltyMultiplier {
				penaltyMultiplier = totalBalance
			}
			penalty := helpers.EffectiveBalance(state, uint64(idx)) *
				penaltyMultiplier / totalBalance
			state.Balances[idx] -= penalty
		}
	}
	allIndices := allValidatorsIndices(state)
	var eligibleIndices []uint64
	for _, idx := range allIndices {
		if eligibleToExit(state, idx) {
			eligibleIndices = append(eligibleIndices, idx)
		}
	}
	var withdrawnSoFar uint64
	for _, idx := range eligibleIndices {
		state = prepareValidatorForWithdrawal(state, idx)
		withdrawnSoFar++
		if withdrawnSoFar >= params.BeaconConfig().MaxExitDequeuesPerEpoch {
			break
		}
	}
	return state
}

// InitializeValidatorStore sets the current active validators from the current
// state.
func InitializeValidatorStore(bState *pb.BeaconState) {
	VStore.Lock()
	defer VStore.Unlock()

	currentEpoch := helpers.CurrentEpoch(bState)
	activeValidatorIndices := helpers.ActiveValidatorIndices(bState, currentEpoch)
	VStore.activatedValidators[currentEpoch] = activeValidatorIndices

}

// InsertActivatedVal locks the validator store, inserts the activated validator
// indices, then unlocks the store again. This method may be used by
// external services in testing to populate the validator store.
func InsertActivatedVal(epoch uint64, validators []uint64) {
	VStore.Lock()
	defer VStore.Unlock()
	VStore.activatedValidators[epoch] = validators
}

// InsertExitedVal locks the validator store, inserts the exited validator
// indices, then unlocks the store again. This method may be used by
// external services in testing to remove the validator store.
func InsertExitedVal(epoch uint64, validators []uint64) {
	VStore.Lock()
	defer VStore.Unlock()
	VStore.exitedValidators[epoch] = validators
}

// ActivatedValFromEpoch locks the validator store, retrieves the activated validator
// indices of a given epoch, then unlocks the store again.
func ActivatedValFromEpoch(epoch uint64) []uint64 {
	VStore.RLock()
	defer VStore.RUnlock()
	if _, exists := VStore.activatedValidators[epoch]; !exists {
		return nil
	}
	return VStore.activatedValidators[epoch]
}

// ExitedValFromEpoch locks the validator store, retrieves the exited validator
// indices of a given epoch, then unlocks the store again.
func ExitedValFromEpoch(epoch uint64) []uint64 {
	VStore.RLock()
	defer VStore.RUnlock()
	if _, exists := VStore.exitedValidators[epoch]; !exists {
		return nil
	}
	return VStore.exitedValidators[epoch]
}

// DeleteActivatedVal locks the validator store, delete the activated validator
// indices of a given epoch, then unlocks the store again.
func DeleteActivatedVal(epoch uint64) {
	VStore.Lock()
	defer VStore.Unlock()
	delete(VStore.activatedValidators, epoch)
}

// DeleteExitedVal locks the validator store, delete the exited validator
// indices of a given epoch, then unlocks the store again.
func DeleteExitedVal(epoch uint64) {
	VStore.Lock()
	defer VStore.Unlock()
	delete(VStore.exitedValidators, epoch)
}

// allValidatorsIndices returns all validator indices from 0 to
// the last validator.
func allValidatorsIndices(state *pb.BeaconState) []uint64 {
	validatorIndices := make([]uint64, len(state.ValidatorRegistry))
	for i := 0; i < len(validatorIndices); i++ {
		validatorIndices[i] = uint64(i)
	}
	return validatorIndices
}

// maxBalanceChurn returns the maximum balance churn in Gwei,
// this determines how many validators can be rotated
// in and out of the validator pool.
// Spec pseudocode definition:
//     max_balance_churn = max(
//        MAX_DEPOSIT_AMOUNT,
//        total_balance // (2 * MAX_BALANCE_CHURN_QUOTIENT))
func maxBalanceChurn(totalBalance uint64) uint64 {
	maxBalanceChurn := totalBalance / (2 * params.BeaconConfig().MaxBalanceChurnQuotient)
	if maxBalanceChurn > params.BeaconConfig().MaxDepositAmount {
		return maxBalanceChurn
	}
	return params.BeaconConfig().MaxDepositAmount
}

// eligibleToExit checks if a validator is eligible to exit whether it was
// slashed or not.
//
// Spec pseudocode definition:
// def eligible(index):
//    validator = state.validator_registry[index]
//    if validator.slashed_epoch <= current_epoch:
//         slashed_withdrawal_epochs = LATEST_PENALIZED_EXIT_LENGTH // 2
//        return current_epoch >= validator.slashed_epoch + slashd_withdrawal_epochs
//    else:
//        return current_epoch >= validator.exit_epoch + MIN_VALIDATOR_WITHDRAWAL_DELAY
func eligibleToExit(state *pb.BeaconState, idx uint64) bool {
	currentEpoch := helpers.CurrentEpoch(state)
	validator := state.ValidatorRegistry[idx]

	if validator.SlashedEpoch <= currentEpoch {
		slashedWithdrawalEpochs := params.BeaconConfig().LatestSlashedExitLength / 2
		return currentEpoch >= validator.SlashedEpoch+slashedWithdrawalEpochs
	}
	return currentEpoch >= validator.ExitEpoch+params.BeaconConfig().MinValidatorWithdrawalDelay
}

// prepareValidatorForWithdrawal sets validator's status flag to
// WITHDRAWABLE.
//
// Spec pseudocode definition:
// def prepare_validator_for_withdrawal(state: BeaconState, index: ValidatorIndex) -> None:
//    """
//    Set the validator with the given ``index`` with ``WITHDRAWABLE`` flag.
//    Note that this function mutates ``state``.
//    """
//    validator = state.validator_registry[index]
//    validator.status_flags |= WITHDRAWABLE
func prepareValidatorForWithdrawal(state *pb.BeaconState, idx uint64) *pb.BeaconState {
	state.ValidatorRegistry[idx].StatusFlags |=
		pb.Validator_WITHDRAWABLE
	return state
}
