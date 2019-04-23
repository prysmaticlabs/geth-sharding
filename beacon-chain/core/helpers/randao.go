package helpers

import (
	"encoding/binary"
	"fmt"

	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/prysmaticlabs/prysm/shared/hashutil"
	"github.com/prysmaticlabs/prysm/shared/params"
)

// GenerateSeed generates the randao seed of a given epoch.
//
// Spec pseudocode definition:
//   def generate_seed(state: BeaconState,
//                  epoch: Epoch) -> Bytes32:
//    """
//    Generate a seed for the given ``epoch``.
//    """
//    return hash(
//      get_randao_mix(state, epoch - MIN_SEED_LOOKAHEAD) +
//      get_active_index_root(state, epoch) +
//      int_to_bytes32(epoch)
//    )
func GenerateSeed(state *pb.BeaconState, wantedEpoch uint64) ([32]byte, error) {
	if wantedEpoch > params.BeaconConfig().MinSeedLookahead {
		wantedEpoch -= params.BeaconConfig().MinSeedLookahead
	}
	randaoMix, err := RandaoMix(state, wantedEpoch)
	if err != nil {
		return [32]byte{}, err
	}
	indexRoot, err := ActiveIndexRoot(state, wantedEpoch)
	if err != nil {
		return [32]byte{}, err
	}

	epochInBytes := make([]byte, 32)
	binary.LittleEndian.PutUint64(epochInBytes, wantedEpoch)
	epochBytes32 := bytesutil.ToBytes32(epochInBytes)
	bytes := append(randaoMix, indexRoot...)
	return hashutil.Hash(append(bytes, epochBytes32[:]...)), nil
}

// ActiveIndexRoot returns the index root of a given epoch.
//
// Spec pseudocode definition:
//   def get_active_index_root(state: BeaconState,
//                          epoch: Epoch) -> Bytes32:
//    """
//    Return the index root at a recent ``epoch``.
//    """
//    assert get_current_epoch(state) - LATEST_INDEX_ROOTS_LENGTH + ACTIVATION_EXIT_DELAY < epoch <= get_current_epoch(state) + ACTIVATION_EXIT_DELAY
//    return state.latest_index_roots[epoch % LATEST_INDEX_ROOTS_LENGTH]
func ActiveIndexRoot(state *pb.BeaconState, wantedEpoch uint64) ([]byte, error) {
	var earliestEpoch uint64
	currentEpoch := CurrentEpoch(state)
	if currentEpoch > params.BeaconConfig().LatestActiveIndexRootsLength+params.BeaconConfig().ActivationExitDelay {
		earliestEpoch = currentEpoch - (params.BeaconConfig().LatestActiveIndexRootsLength + params.BeaconConfig().ActivationExitDelay)
	}
	if earliestEpoch > wantedEpoch || wantedEpoch > currentEpoch {
		return nil, fmt.Errorf("input indexRoot epoch %d out of bounds: %d <= epoch < %d",
			wantedEpoch, earliestEpoch, currentEpoch)
	}
	return state.LatestIndexRootHash32S[wantedEpoch%params.BeaconConfig().LatestActiveIndexRootsLength], nil
}

// RandaoMix returns the randao mix (xor'ed seed)
// of a given slot. It is used to shuffle validators.
//
// Spec pseudocode definition:
//   def get_randao_mix(state: BeaconState,
//                   epoch: Epoch) -> Bytes32:
//    """
//    Return the randao mix at a recent ``epoch``.
//    """
//    assert get_current_epoch(state) - LATEST_RANDAO_MIXES_LENGTH < epoch <= get_current_epoch(state)
//    return state.latest_randao_mixes[epoch % LATEST_RANDAO_MIXES_LENGTH]
func RandaoMix(state *pb.BeaconState, wantedEpoch uint64) ([]byte, error) {
	var earliestEpoch uint64
	currentEpoch := CurrentEpoch(state)
	if currentEpoch > params.BeaconConfig().LatestRandaoMixesLength {
		earliestEpoch = currentEpoch - params.BeaconConfig().LatestRandaoMixesLength
	}
	if earliestEpoch > wantedEpoch || wantedEpoch > currentEpoch {
		return nil, fmt.Errorf("input randaoMix epoch %d out of bounds: %d <= epoch < %d",
			wantedEpoch, earliestEpoch, currentEpoch)
	}
	return state.LatestRandaoMixes[wantedEpoch%params.BeaconConfig().LatestRandaoMixesLength], nil
}
