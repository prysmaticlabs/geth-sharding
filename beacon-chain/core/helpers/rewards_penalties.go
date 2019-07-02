package helpers

import (
	"fmt"

	"github.com/prysmaticlabs/prysm/beacon-chain/cache"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	"github.com/prysmaticlabs/prysm/shared/params"
)

var totalBalanceCache = cache.NewTotalBalanceCache()
var totalActiveBalanceCache = cache.NewActiveBalanceCache()

// TotalBalance returns the total amount at stake in Gwei
// of input validators.
//
// Spec pseudocode definition:
//   def get_total_balance(state: BeaconState, indices: List[ValidatorIndex]) -> Gwei:
//    """
//    Return the combined effective balance of the ``indices``. (1 Gwei minimum to avoid divisions by zero.)
//    """
//    return max(sum([state.validator_registry[index].effective_balance for index in indices]), 1)
func TotalBalance(state *pb.BeaconState, indices []uint64) uint64 {
	total := uint64(0)
	for _, idx := range indices {
		total += state.Validators[idx].EffectiveBalance
	}

	// Return 1 Gwei minimum to avoid divisions by zero
	if total == 0 {
		return 1
	}

	return total
}

// TotalActiveBalance returns the total amount at stake in Gwei
// of active validators.
func TotalActiveBalance(state *pb.BeaconState) (uint64, error) {
	epoch := CurrentEpoch(state)
	total, err := totalActiveBalanceCache.ActiveBalanceInEpoch(epoch)
	if err != nil {
		return 0, fmt.Errorf("could not retrieve total balance from cache: %v", err)
	}
	if total != params.BeaconConfig().FarFutureEpoch {
		return total, nil
	}

	total = 0
	for i, v := range state.Validators {
		if IsActiveValidator(v, epoch) {
			total += state.Validators[i].EffectiveBalance
		}
	}

	if err := totalActiveBalanceCache.AddActiveBalance(&cache.ActiveBalanceByEpoch{
		Epoch:         epoch,
		ActiveBalance: total,
	}); err != nil {
		return 0, fmt.Errorf("could not save active balance for cache: %v", err)
	}
	return total, nil
}

// IncreaseBalance increases validator with the given 'index' balance by 'delta' in Gwei.
//
// Spec pseudocode definition:
//  def increase_balance(state: BeaconState, index: ValidatorIndex, delta: Gwei) -> None:
//    """
//    Increase validator balance by ``delta``.
//    """
//    state.balances[index] += delta
func IncreaseBalance(state *pb.BeaconState, idx uint64, delta uint64) *pb.BeaconState {
	state.Balances[idx] += delta
	return state
}

// DecreaseBalance decreases validator with the given 'index' balance by 'delta' in Gwei.
//
// Spec pseudocode definition:
//  def decrease_balance(state: BeaconState, index: ValidatorIndex, delta: Gwei) -> None:
//    """
//    Decrease validator balance by ``delta`` with underflow protection.
//    """
//    state.balances[index] = 0 if delta > state.balances[index] else state.balances[index] - delta
func DecreaseBalance(state *pb.BeaconState, idx uint64, delta uint64) *pb.BeaconState {
	if delta > state.Balances[idx] {
		state.Balances[idx] = 0
		return state
	}
	state.Balances[idx] -= delta
	return state
}
