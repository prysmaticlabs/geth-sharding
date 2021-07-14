package helpers

import (
	"bytes"

	"github.com/pkg/errors"
	types "github.com/prysmaticlabs/eth2-types"
	iface "github.com/prysmaticlabs/prysm/beacon-chain/state/interface"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	ethpb "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/shared/bls"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/prysmaticlabs/prysm/shared/hashutil"
	"github.com/prysmaticlabs/prysm/shared/params"
)

// IsActiveValidator returns the boolean value on whether the validator
// is active or not.
//
// Spec pseudocode definition:
//  def is_active_validator(validator: Validator, epoch: Epoch) -> bool:
//    """
//    Check if ``validator`` is active.
//    """
//    return validator.activation_epoch <= epoch < validator.exit_epoch
func IsActiveValidator(validator *ethpb.Validator, epoch types.Epoch) bool {
	return checkValidatorActiveStatus(validator.ActivationEpoch, validator.ExitEpoch, epoch)
}

// IsActiveValidatorUsingTrie checks if a read only validator is active.
func IsActiveValidatorUsingTrie(validator iface.ReadOnlyValidator, epoch types.Epoch) bool {
	return checkValidatorActiveStatus(validator.ActivationEpoch(), validator.ExitEpoch(), epoch)
}

func checkValidatorActiveStatus(activationEpoch, exitEpoch, epoch types.Epoch) bool {
	return activationEpoch <= epoch && epoch < exitEpoch
}

// IsSlashableValidator returns the boolean value on whether the validator
// is slashable or not.
//
// Spec pseudocode definition:
//  def is_slashable_validator(validator: Validator, epoch: Epoch) -> bool:
//  """
//  Check if ``validator`` is slashable.
//  """
//  return (not validator.slashed) and (validator.activation_epoch <= epoch < validator.withdrawable_epoch)
func IsSlashableValidator(activationEpoch, withdrawableEpoch types.Epoch, slashed bool, epoch types.Epoch) bool {
	return checkValidatorSlashable(activationEpoch, withdrawableEpoch, slashed, epoch)
}

// IsSlashableValidatorUsingTrie checks if a read only validator is slashable.
func IsSlashableValidatorUsingTrie(val iface.ReadOnlyValidator, epoch types.Epoch) bool {
	return checkValidatorSlashable(val.ActivationEpoch(), val.WithdrawableEpoch(), val.Slashed(), epoch)
}

func checkValidatorSlashable(activationEpoch, withdrawableEpoch types.Epoch, slashed bool, epoch types.Epoch) bool {
	active := activationEpoch <= epoch
	beforeWithdrawable := epoch < withdrawableEpoch
	return beforeWithdrawable && active && !slashed
}

// ActiveValidatorIndices filters out active validators based on validator status
// and returns their indices in a list.
//
// WARNING: This method allocates a new copy of the validator index set and is
// considered to be very memory expensive. Avoid using this unless you really
// need the active validator indices for some specific reason.
//
// Spec pseudocode definition:
//  def get_active_validator_indices(state: BeaconState, epoch: Epoch) -> Sequence[ValidatorIndex]:
//    """
//    Return the sequence of active validator indices at ``epoch``.
//    """
//    return [ValidatorIndex(i) for i, v in enumerate(state.validators) if is_active_validator(v, epoch)]
func ActiveValidatorIndices(state iface.ReadOnlyBeaconState, epoch types.Epoch) ([]types.ValidatorIndex, error) {
	seed, err := Seed(state, epoch, params.BeaconConfig().DomainBeaconAttester)
	if err != nil {
		return nil, errors.Wrap(err, "could not get seed")
	}
	activeIndices, err := committeeCache.ActiveIndices(seed)
	if err != nil {
		return nil, errors.Wrap(err, "could not interface with committee cache")
	}
	if activeIndices != nil {
		return activeIndices, nil
	}
	var indices []types.ValidatorIndex
	if err := state.ReadFromEveryValidator(func(idx int, val iface.ReadOnlyValidator) error {
		if IsActiveValidatorUsingTrie(val, epoch) {
			indices = append(indices, types.ValidatorIndex(idx))
		}
		return nil
	}); err != nil {
		return nil, err
	}

	if err := UpdateCommitteeCache(state, epoch); err != nil {
		return nil, errors.Wrap(err, "could not update committee cache")
	}

	return indices, nil
}

// ActiveValidatorCount returns the number of active validators in the state
// at the given epoch.
func ActiveValidatorCount(state iface.ReadOnlyBeaconState, epoch types.Epoch) (uint64, error) {
	seed, err := Seed(state, epoch, params.BeaconConfig().DomainBeaconAttester)
	if err != nil {
		return 0, errors.Wrap(err, "could not get seed")
	}
	activeCount, err := committeeCache.ActiveIndicesCount(seed)
	if err != nil {
		return 0, errors.Wrap(err, "could not interface with committee cache")
	}
	if activeCount != 0 && state.Slot() != 0 {
		return uint64(activeCount), nil
	}

	count := uint64(0)
	if err := state.ReadFromEveryValidator(func(idx int, val iface.ReadOnlyValidator) error {
		if IsActiveValidatorUsingTrie(val, epoch) {
			count++
		}
		return nil
	}); err != nil {
		return 0, err
	}

	if err := UpdateCommitteeCache(state, epoch); err != nil {
		return 0, errors.Wrap(err, "could not update committee cache")
	}

	return count, nil
}

// ActivationExitEpoch takes in epoch number and returns when
// the validator is eligible for activation and exit.
//
// Spec pseudocode definition:
//  def compute_activation_exit_epoch(epoch: Epoch) -> Epoch:
//    """
//    Return the epoch during which validator activations and exits initiated in ``epoch`` take effect.
//    """
//    return Epoch(epoch + 1 + MAX_SEED_LOOKAHEAD)
func ActivationExitEpoch(epoch types.Epoch) types.Epoch {
	return epoch + 1 + params.BeaconConfig().MaxSeedLookahead
}

// ValidatorChurnLimit returns the number of validators that are allowed to
// enter and exit validator pool for an epoch.
//
// Spec pseudocode definition:
//   def get_validator_churn_limit(state: BeaconState) -> uint64:
//    """
//    Return the validator churn limit for the current epoch.
//    """
//    active_validator_indices = get_active_validator_indices(state, get_current_epoch(state))
//    return max(MIN_PER_EPOCH_CHURN_LIMIT, uint64(len(active_validator_indices)) // CHURN_LIMIT_QUOTIENT)
func ValidatorChurnLimit(activeValidatorCount uint64) (uint64, error) {
	churnLimit := activeValidatorCount / params.BeaconConfig().ChurnLimitQuotient
	if churnLimit < params.BeaconConfig().MinPerEpochChurnLimit {
		churnLimit = params.BeaconConfig().MinPerEpochChurnLimit
	}
	return churnLimit, nil
}

// BeaconProposerIndex returns proposer index of a current slot.
//
// Spec pseudocode definition:
//  def get_beacon_proposer_index(state: BeaconState) -> ValidatorIndex:
//    """
//    Return the beacon proposer index at the current slot.
//    """
//    epoch = get_current_epoch(state)
//    seed = hash(get_seed(state, epoch, DOMAIN_BEACON_PROPOSER) + uint_to_bytes(state.slot))
//    indices = get_active_validator_indices(state, epoch)
//    return compute_proposer_index(state, indices, seed)
func BeaconProposerIndex(state iface.ReadOnlyBeaconState) (types.ValidatorIndex, error) {
	e := CurrentEpoch(state)
	// The cache uses the state root of the previous epoch - minimum_seed_lookahead last slot as key. (e.g. Starting epoch 1, slot 32, the key would be block root at slot 31)
	// For simplicity, the node will skip caching of genesis epoch.
	if e > params.BeaconConfig().GenesisEpoch+params.BeaconConfig().MinSeedLookahead {
		wantedEpoch := PrevEpoch(state)
		if wantedEpoch >= params.BeaconConfig().MinSeedLookahead {
			wantedEpoch -= params.BeaconConfig().MinSeedLookahead
		}
		s, err := EndSlot(wantedEpoch)
		if err != nil {
			return 0, err
		}
		r, err := StateRootAtSlot(state, s)
		if err != nil {
			return 0, err
		}
		if r != nil && !bytes.Equal(r, params.BeaconConfig().ZeroHash[:]) {
			proposerIndices, err := proposerIndicesCache.ProposerIndices(bytesutil.ToBytes32(r))
			if err != nil {
				return 0, errors.Wrap(err, "could not interface with committee cache")
			}
			if proposerIndices != nil {
				if len(proposerIndices) != int(params.BeaconConfig().SlotsPerEpoch) {
					return 0, errors.Errorf("length of proposer indices is not equal %d to slots per epoch", len(proposerIndices))
				}
				return proposerIndices[state.Slot()%params.BeaconConfig().SlotsPerEpoch], nil
			}
			if err := UpdateProposerIndicesInCache(state); err != nil {
				return 0, errors.Wrap(err, "could not update committee cache")
			}
		}
	}

	seed, err := Seed(state, e, params.BeaconConfig().DomainBeaconProposer)
	if err != nil {
		return 0, errors.Wrap(err, "could not generate seed")
	}

	seedWithSlot := append(seed[:], bytesutil.Bytes8(uint64(state.Slot()))...)
	seedWithSlotHash := hashutil.Hash(seedWithSlot)

	indices, err := ActiveValidatorIndices(state, e)
	if err != nil {
		return 0, errors.Wrap(err, "could not get active indices")
	}

	return ComputeProposerIndex(state, indices, seedWithSlotHash)
}

// ComputeProposerIndex returns the index sampled by effective balance, which is used to calculate proposer.
//
// Spec pseudocode definition:
//  def compute_proposer_index(state: BeaconState, indices: Sequence[ValidatorIndex], seed: Bytes32) -> ValidatorIndex:
//    """
//    Return from ``indices`` a random index sampled by effective balance.
//    """
//    assert len(indices) > 0
//    MAX_RANDOM_BYTE = 2**8 - 1
//    i = uint64(0)
//    total = uint64(len(indices))
//    while True:
//        candidate_index = indices[compute_shuffled_index(i % total, total, seed)]
//        random_byte = hash(seed + uint_to_bytes(uint64(i // 32)))[i % 32]
//        effective_balance = state.validators[candidate_index].effective_balance
//        if effective_balance * MAX_RANDOM_BYTE >= MAX_EFFECTIVE_BALANCE * random_byte:
//            return candidate_index
//        i += 1
func ComputeProposerIndex(bState iface.ReadOnlyValidators, activeIndices []types.ValidatorIndex, seed [32]byte) (types.ValidatorIndex, error) {
	length := uint64(len(activeIndices))
	if length == 0 {
		return 0, errors.New("empty active indices list")
	}
	maxRandomByte := uint64(1<<8 - 1)
	hashFunc := hashutil.CustomSHA256Hasher()

	for i := uint64(0); ; i++ {
		candidateIndex, err := ComputeShuffledIndex(types.ValidatorIndex(i%length), length, seed, true /* shuffle */)
		if err != nil {
			return 0, err
		}
		candidateIndex = activeIndices[candidateIndex]
		if uint64(candidateIndex) >= uint64(bState.NumValidators()) {
			return 0, errors.New("active index out of range")
		}
		b := append(seed[:], bytesutil.Bytes8(i/32)...)
		randomByte := hashFunc(b)[i%32]
		v, err := bState.ValidatorAtIndexReadOnly(candidateIndex)
		if err != nil {
			return 0, err
		}
		effectiveBal := v.EffectiveBalance()

		if effectiveBal*maxRandomByte >= params.BeaconConfig().MaxEffectiveBalance*uint64(randomByte) {
			return candidateIndex, nil
		}
	}
}

// Domain returns the domain version for BLS private key to sign and verify.
//
// Spec pseudocode definition:
//  def get_domain(state: BeaconState, domain_type: DomainType, epoch: Epoch=None) -> Domain:
//    """
//    Return the signature domain (fork version concatenated with domain type) of a message.
//    """
//    epoch = get_current_epoch(state) if epoch is None else epoch
//    fork_version = state.fork.previous_version if epoch < state.fork.epoch else state.fork.current_version
//    return compute_domain(domain_type, fork_version, state.genesis_validators_root)
func Domain(fork *pb.Fork, epoch types.Epoch, domainType [bls.DomainByteLength]byte, genesisRoot []byte) (types.Domain, error) {
	if fork == nil {
		return types.Domain{}, errors.New("nil fork or domain type")
	}
	var forkVersion []byte
	if epoch < fork.Epoch {
		forkVersion = fork.PreviousVersion
	} else {
		forkVersion = fork.CurrentVersion
	}
	if len(forkVersion) != 4 {
		return types.Domain{}, errors.New("fork version length is not 4 byte")
	}
	var forkVersionArray [4]byte
	copy(forkVersionArray[:], forkVersion[:4])
	return ComputeDomain(domainType, forkVersionArray[:], genesisRoot)
}

// IsEligibleForActivationQueue checks if the validator is eligible to
// be placed into the activation queue.
//
// Spec pseudocode definition:
//  def is_eligible_for_activation_queue(validator: Validator) -> bool:
//    """
//    Check if ``validator`` is eligible to be placed into the activation queue.
//    """
//    return (
//        validator.activation_eligibility_epoch == FAR_FUTURE_EPOCH
//        and validator.effective_balance == MAX_EFFECTIVE_BALANCE
//    )
func IsEligibleForActivationQueue(validator *ethpb.Validator) bool {
	return isEligibileForActivationQueue(validator.ActivationEligibilityEpoch, validator.EffectiveBalance)
}

// IsEligibleForActivationQueueUsingTrie checks if the read-only validator is eligible to
// be placed into the activation queue.
func IsEligibleForActivationQueueUsingTrie(validator iface.ReadOnlyValidator) bool {
	return isEligibileForActivationQueue(validator.ActivationEligibilityEpoch(), validator.EffectiveBalance())
}

// isEligibleForActivationQueue carries out the logic for IsEligibleForActivationQueue*
func isEligibileForActivationQueue(activationEligibilityEpoch types.Epoch, effectiveBalance uint64) bool {
	return activationEligibilityEpoch == params.BeaconConfig().FarFutureEpoch &&
		effectiveBalance == params.BeaconConfig().MaxEffectiveBalance
}

// IsEligibleForActivation checks if the validator is eligible for activation.
//
// Spec pseudocode definition:
//  def is_eligible_for_activation(state: BeaconState, validator: Validator) -> bool:
//    """
//    Check if ``validator`` is eligible for activation.
//    """
//    return (
//        # Placement in queue is finalized
//        validator.activation_eligibility_epoch <= state.finalized_checkpoint.epoch
//        # Has not yet been activated
//        and validator.activation_epoch == FAR_FUTURE_EPOCH
//    )
func IsEligibleForActivation(state iface.ReadOnlyCheckpoint, validator *ethpb.Validator) bool {
	finalizedEpoch := state.FinalizedCheckpointEpoch()
	return isEligibleForActivation(validator.ActivationEligibilityEpoch, validator.ActivationEpoch, finalizedEpoch)
}

// IsEligibleForActivationUsingTrie checks if the validator is eligible for activation.
func IsEligibleForActivationUsingTrie(state iface.ReadOnlyCheckpoint, validator iface.ReadOnlyValidator) bool {
	cpt := state.FinalizedCheckpoint()
	if cpt == nil {
		return false
	}
	return isEligibleForActivation(validator.ActivationEligibilityEpoch(), validator.ActivationEpoch(), cpt.Epoch)
}

// isEligibleForActivation carries out the logic for IsEligibleForActivation*
func isEligibleForActivation(activationEligibilityEpoch, activationEpoch, finalizedEpoch types.Epoch) bool {
	return activationEligibilityEpoch <= finalizedEpoch &&
		activationEpoch == params.BeaconConfig().FarFutureEpoch
}
