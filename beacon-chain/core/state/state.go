// Package state implements the whole state transition
// function which consists of per slot, per-epoch transitions.
// It also bootstraps the genesis beacon state for slot 0.
package state

import (
	"bytes"
	"encoding/binary"
	"fmt"

	b "github.com/prysmaticlabs/prysm/beacon-chain/core/blocks"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	"github.com/prysmaticlabs/prysm/shared/hashutil"
	"github.com/prysmaticlabs/prysm/shared/params"
)

// GenesisBeaconState gets called when DepositsForChainStart count of
// full deposits were made to the deposit contract and the ChainStart log gets emitted.
//
// Spec pseudocode definition:
//  def get_genesis_beacon_state(deposits: List[Deposit],
//                             genesis_time: int,
//                             genesis_eth1_data: Eth1Data) -> BeaconState:
//    """
//    Get the genesis ``BeaconState``.
//    """
//    state = BeaconState(genesis_time=genesis_time, latest_eth1_data=genesis_eth1_data)
//
//    # Process genesis deposits
//    for deposit in genesis_validator_deposits:
//        process_deposit(state, deposit)
//
//    # Process genesis activations
//    for validator in state.validator_registry:
//        if validator.effective_balance >= MAX_EFFECTIVE_BALANCE:
//            validator.activation_eligibility_epoch = GENESIS_EPOCH
//            validator.activation_epoch = GENESIS_EPOCH
//
//    genesis_active_index_root = hash_tree_root(get_active_validator_indices(state, GENESIS_EPOCH))
//    for index in range(LATEST_ACTIVE_INDEX_ROOTS_LENGTH):
//        state.latest_active_index_roots[index] = genesis_active_index_root
//
//    return state
func GenesisBeaconState(deposits []*pb.Deposit, genesisTime uint64, eth1Data *pb.Eth1Data) (*pb.BeaconState, error) {
	latestRandaoMixes := make([][]byte, params.BeaconConfig().EpochsPerHistoricalVector)
	for i := 0; i < len(latestRandaoMixes); i++ {
		latestRandaoMixes[i] = make([]byte, 32)
	}

	zeroHash := params.BeaconConfig().ZeroHash[:]

	latestActiveIndexRoots := make([][]byte, params.BeaconConfig().EpochsPerHistoricalVector)
	for i := 0; i < len(latestActiveIndexRoots); i++ {
		latestActiveIndexRoots[i] = zeroHash
	}

	crosslinks := make([]*pb.Crosslink, params.BeaconConfig().ShardCount)
	for i := 0; i < len(crosslinks); i++ {
		crosslinks[i] = &pb.Crosslink{
			Shard: uint64(i),
		}
	}

	latestBlockRoots := make([][]byte, params.BeaconConfig().HistoricalRootsLimit)
	for i := 0; i < len(latestBlockRoots); i++ {
		latestBlockRoots[i] = zeroHash
	}

	latestSlashedExitBalances := make([]uint64, params.BeaconConfig().EpochsPerSlashingsVector)

	if eth1Data == nil {
		eth1Data = &pb.Eth1Data{}
	}

	state := &pb.BeaconState{
		// Misc fields.
		Slot:        0,
		GenesisTime: genesisTime,

		Fork: &pb.Fork{
			PreviousVersion: params.BeaconConfig().GenesisForkVersion,
			CurrentVersion:  params.BeaconConfig().GenesisForkVersion,
			Epoch:           0,
		},

		// Validator registry fields.
		Validators: []*pb.Validator{},
		Balances:   []uint64{},

		// Randomness and committees.
		RandaoMixes: latestRandaoMixes,

		// Finality.
		PreviousJustifiedCheckpoint: &pb.Checkpoint{
			Epoch: 0,
			Root:  params.BeaconConfig().ZeroHash[:],
		},
		CurrentJustifiedCheckpoint: &pb.Checkpoint{
			Epoch: 0,
			Root:  params.BeaconConfig().ZeroHash[:],
		},
		JustificationBits: []byte{0},
		FinalizedCheckpoint: &pb.Checkpoint{
			Epoch: 0,
			Root:  params.BeaconConfig().ZeroHash[:],
		},

		// Recent state.
		CurrentCrosslinks:         crosslinks,
		PreviousCrosslinks:        crosslinks,
		ActiveIndexRoots:          latestActiveIndexRoots,
		BlockRoots:                latestBlockRoots,
		Slashings:                 latestSlashedExitBalances,
		CurrentEpochAttestations:  []*pb.PendingAttestation{},
		PreviousEpochAttestations: []*pb.PendingAttestation{},

		// Eth1 data.
		Eth1Data:         eth1Data,
		Eth1DataVotes:    []*pb.Eth1Data{},
		Eth1DepositIndex: 0,
	}

	// Process initial deposits.
	var err error
	validatorMap := make(map[[32]byte]int)
	for _, deposit := range deposits {
		eth1DataExists := !bytes.Equal(eth1Data.DepositRoot, []byte{})
		state, err = b.ProcessDeposit(
			state,
			deposit,
			validatorMap,
			false,
			eth1DataExists,
		)
		if err != nil {
			return nil, fmt.Errorf("could not process validator deposit: %v", err)
		}
	}
	for i := 0; i < len(state.Validators); i++ {
		if state.Validators[i].EffectiveBalance >=
			params.BeaconConfig().MaxEffectiveBalance {
			state.Validators[i].ActivationEligibilityEpoch = 0
			state.Validators[i].ActivationEpoch = 0
		}
	}
	activeValidators, err := helpers.ActiveValidatorIndices(state, 0)
	if err != nil {
		return nil, fmt.Errorf("could not get active validator indices: %v", err)
	}

	indicesBytes := []byte{}
	for _, val := range activeValidators {
		buf := make([]byte, 8)
		binary.LittleEndian.PutUint64(buf, val)
		indicesBytes = append(indicesBytes, buf...)
	}
	genesisActiveIndexRoot := hashutil.Hash(indicesBytes)
	for i := uint64(0); i < params.BeaconConfig().EpochsPerHistoricalVector; i++ {
		state.ActiveIndexRoots[i] = genesisActiveIndexRoot[:]
	}
	return state, nil
}
