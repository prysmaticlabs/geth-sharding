// Package state implements the whole state transition
// function which consists of per slot, per-epoch transitions, and
// bootstrapping the genesis state according to the eth2 spec.
package state

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	ethpb "github.com/prysmaticlabs/ethereumapis/eth/v1alpha1"
	b "github.com/prysmaticlabs/prysm/beacon-chain/core/blocks"
	iface "github.com/prysmaticlabs/prysm/beacon-chain/state/interface"
	"github.com/prysmaticlabs/prysm/beacon-chain/state/stateV0"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/trieutil"
)

// GenesisBeaconState gets called when MinGenesisActiveValidatorCount count of
// full deposits were made to the deposit contract and the ChainStart log gets emitted.
//
// Spec pseudocode definition:
//  def initialize_beacon_state_from_eth1(eth1_block_hash: Bytes32,
//                                      eth1_timestamp: uint64,
//                                      deposits: Sequence[Deposit]) -> BeaconState:
//    fork = Fork(
//        previous_version=GENESIS_FORK_VERSION,
//        current_version=GENESIS_FORK_VERSION,
//        epoch=GENESIS_EPOCH,
//    )
//    state = BeaconState(
//        genesis_time=eth1_timestamp + GENESIS_DELAY,
//        fork=fork,
//        eth1_data=Eth1Data(block_hash=eth1_block_hash, deposit_count=uint64(len(deposits))),
//        latest_block_header=BeaconBlockHeader(body_root=hash_tree_root(BeaconBlockBody())),
//        randao_mixes=[eth1_block_hash] * EPOCHS_PER_HISTORICAL_VECTOR,  # Seed RANDAO with Eth1 entropy
//    )
//
//    # Process deposits
//    leaves = list(map(lambda deposit: deposit.data, deposits))
//    for index, deposit in enumerate(deposits):
//        deposit_data_list = List[DepositData, 2**DEPOSIT_CONTRACT_TREE_DEPTH](*leaves[:index + 1])
//        state.eth1_data.deposit_root = hash_tree_root(deposit_data_list)
//        process_deposit(state, deposit)
//
//    # Process activations
//    for index, validator in enumerate(state.validators):
//        balance = state.balances[index]
//        validator.effective_balance = min(balance - balance % EFFECTIVE_BALANCE_INCREMENT, MAX_EFFECTIVE_BALANCE)
//        if validator.effective_balance == MAX_EFFECTIVE_BALANCE:
//            validator.activation_eligibility_epoch = GENESIS_EPOCH
//            validator.activation_epoch = GENESIS_EPOCH
//
//    # Set genesis validators root for domain separation and chain versioning
//    state.genesis_validators_root = hash_tree_root(state.validators)
//
//    return state
// This method differs from the spec so as to process deposits beforehand instead of the end of the function.
func GenesisBeaconState(ctx context.Context, deposits []*ethpb.Deposit, genesisTime uint64, eth1Data *ethpb.Eth1Data) (iface.BeaconState, error) {
	state, err := EmptyGenesisState()
	if err != nil {
		return nil, err
	}

	// Process initial deposits.
	state, err = updateGenesisEth1Data(state, deposits, eth1Data)
	if err != nil {
		return nil, err
	}

	state, err = b.ProcessPreGenesisDeposits(ctx, state, deposits)
	if err != nil {
		return nil, errors.Wrap(err, "could not process validator deposits")
	}

	return OptimizedGenesisBeaconState(genesisTime, state, state.Eth1Data())
}

// OptimizedGenesisBeaconState is used to create a state that has already processed deposits. This is to efficiently
// create a mainnet state at chainstart.
func OptimizedGenesisBeaconState(genesisTime uint64, preState iface.BeaconState, eth1Data *ethpb.Eth1Data) (iface.BeaconState, error) {
	if eth1Data == nil {
		return nil, errors.New("no eth1data provided for genesis state")
	}

	randaoMixes := make([][]byte, params.BeaconConfig().EpochsPerHistoricalVector)
	for i := 0; i < len(randaoMixes); i++ {
		h := make([]byte, 32)
		copy(h, eth1Data.BlockHash)
		randaoMixes[i] = h
	}

	zeroHash := params.BeaconConfig().ZeroHash[:]

	activeIndexRoots := make([][]byte, params.BeaconConfig().EpochsPerHistoricalVector)
	for i := 0; i < len(activeIndexRoots); i++ {
		activeIndexRoots[i] = zeroHash
	}

	blockRoots := make([][]byte, params.BeaconConfig().SlotsPerHistoricalRoot)
	for i := 0; i < len(blockRoots); i++ {
		blockRoots[i] = zeroHash
	}

	stateRoots := make([][]byte, params.BeaconConfig().SlotsPerHistoricalRoot)
	for i := 0; i < len(stateRoots); i++ {
		stateRoots[i] = zeroHash
	}

	slashings := make([]uint64, params.BeaconConfig().EpochsPerSlashingsVector)

	genesisValidatorsRoot, err := stateV0.ValidatorRegistryRoot(preState.Validators())
	if err != nil {
		return nil, errors.Wrapf(err, "could not hash tree root genesis validators %v", err)
	}

	state := &pb.BeaconState{
		// Misc fields.
		Slot:                  0,
		GenesisTime:           genesisTime,
		GenesisValidatorsRoot: genesisValidatorsRoot[:],

		Fork: &pb.Fork{
			PreviousVersion: params.BeaconConfig().GenesisForkVersion,
			CurrentVersion:  params.BeaconConfig().GenesisForkVersion,
			Epoch:           0,
		},

		// Validator registry fields.
		Validators: preState.Validators(),
		Balances:   preState.Balances(),

		// Randomness and committees.
		RandaoMixes: randaoMixes,

		// Finality.
		PreviousJustifiedCheckpoint: &ethpb.Checkpoint{
			Epoch: 0,
			Root:  params.BeaconConfig().ZeroHash[:],
		},
		CurrentJustifiedCheckpoint: &ethpb.Checkpoint{
			Epoch: 0,
			Root:  params.BeaconConfig().ZeroHash[:],
		},
		JustificationBits: []byte{0},
		FinalizedCheckpoint: &ethpb.Checkpoint{
			Epoch: 0,
			Root:  params.BeaconConfig().ZeroHash[:],
		},

		HistoricalRoots:           [][]byte{},
		BlockRoots:                blockRoots,
		StateRoots:                stateRoots,
		Slashings:                 slashings,
		CurrentEpochAttestations:  []*pb.PendingAttestation{},
		PreviousEpochAttestations: []*pb.PendingAttestation{},

		// Eth1 data.
		Eth1Data:         eth1Data,
		Eth1DataVotes:    []*ethpb.Eth1Data{},
		Eth1DepositIndex: preState.Eth1DepositIndex(),

		LatestExecutionPayloadHeader: &pb.ExecutionPayloadHeader{
			BlockHash:        make([]byte, 32),
			ParentHash:       make([]byte, 32),
			Coinbase:         make([]byte, 20),
			StateRoot:        make([]byte, 32),
			GasLimit:         0,
			GasUsed:          0,
			Timestamp:        0,
			ReceiptRoot:      make([]byte, 32),
			LogsBloom:        make([]byte, 256),
			TransactionsRoot: make([]byte, 32),
		},
	}

	bodyRoot, err := (&ethpb.BeaconBlockBody{
		RandaoReveal: make([]byte, 96),
		Eth1Data: &ethpb.Eth1Data{
			DepositRoot: make([]byte, 32),
			BlockHash:   make([]byte, 32),
		},
		Graffiti: make([]byte, 32),
		ExecutionPayload: &ethpb.ExecutionPayload{
			BlockHash:   make([]byte, 32),
			ParentHash:  make([]byte, 32),
			Coinbase:    make([]byte, 20),
			StateRoot:   make([]byte, 32),
			GasLimit:    0,
			GasUsed:     0,
			Timestamp:   0,
			ReceiptRoot: make([]byte, 32),
			LogsBloom:   make([]byte, 256),
		},
	}).HashTreeRoot()
	if err != nil {
		return nil, errors.Wrap(err, "could not hash tree root empty block body")
	}

	state.LatestBlockHeader = &ethpb.BeaconBlockHeader{
		ParentRoot: zeroHash,
		StateRoot:  zeroHash,
		BodyRoot:   bodyRoot[:],
	}

	return stateV0.InitializeFromProto(state)
}

// EmptyGenesisState returns an empty beacon state object.
func EmptyGenesisState() (iface.BeaconState, error) {
	state := &pb.BeaconState{
		// Misc fields.
		Slot: 0,
		Fork: &pb.Fork{
			PreviousVersion: params.BeaconConfig().GenesisForkVersion,
			CurrentVersion:  params.BeaconConfig().GenesisForkVersion,
			Epoch:           0,
		},
		// Validator registry fields.
		Validators: []*ethpb.Validator{},
		Balances:   []uint64{},

		JustificationBits:         []byte{0},
		HistoricalRoots:           [][]byte{},
		CurrentEpochAttestations:  []*pb.PendingAttestation{},
		PreviousEpochAttestations: []*pb.PendingAttestation{},

		// Eth1 data.
		Eth1Data:         &ethpb.Eth1Data{},
		Eth1DataVotes:    []*ethpb.Eth1Data{},
		Eth1DepositIndex: 0,

		LatestExecutionPayloadHeader: &pb.ExecutionPayloadHeader{
			BlockHash:        make([]byte, 32),
			ParentHash:       make([]byte, 32),
			Coinbase:         make([]byte, 20),
			StateRoot:        make([]byte, 32),
			GasLimit:         0,
			GasUsed:          0,
			Timestamp:        0,
			ReceiptRoot:      make([]byte, 32),
			LogsBloom:        make([]byte, 256),
			TransactionsRoot: make([]byte, 32),
		},
	}

	return stateV0.InitializeFromProto(state)
}

// IsValidGenesisState gets called whenever there's a deposit event,
// it checks whether there's enough effective balance to trigger and
// if the minimum genesis time arrived already.
//
// Spec pseudocode definition:
//  def is_valid_genesis_state(state: BeaconState) -> bool:
//     if state.genesis_time < MIN_GENESIS_TIME:
//         return False
//     if len(get_active_validator_indices(state, GENESIS_EPOCH)) < MIN_GENESIS_ACTIVE_VALIDATOR_COUNT:
//         return False
//     return True
// This method has been modified from the spec to allow whole states not to be saved
// but instead only cache the relevant information.
func IsValidGenesisState(chainStartDepositCount, currentTime uint64) bool {
	if currentTime < params.BeaconConfig().MinGenesisTime {
		return false
	}
	if chainStartDepositCount < params.BeaconConfig().MinGenesisActiveValidatorCount {
		return false
	}
	return true
}

func updateGenesisEth1Data(state iface.BeaconState, deposits []*ethpb.Deposit, eth1Data *ethpb.Eth1Data) (iface.BeaconState, error) {
	if eth1Data == nil {
		return nil, errors.New("no eth1data provided for genesis state")
	}

	var leaves [][]byte
	for _, deposit := range deposits {
		if deposit == nil || deposit.Data == nil {
			return nil, fmt.Errorf("nil deposit or deposit with nil data cannot be processed: %v", deposit)
		}
		hash, err := deposit.Data.HashTreeRoot()
		if err != nil {
			return nil, err
		}
		leaves = append(leaves, hash[:])
	}
	var trie *trieutil.SparseMerkleTrie
	var err error
	if len(leaves) > 0 {
		trie, err = trieutil.GenerateTrieFromItems(leaves, params.BeaconConfig().DepositContractTreeDepth)
		if err != nil {
			return nil, err
		}
	} else {
		trie, err = trieutil.NewTrie(params.BeaconConfig().DepositContractTreeDepth)
		if err != nil {
			return nil, err
		}
	}

	depositRoot := trie.Root()
	eth1Data.DepositRoot = depositRoot[:]
	err = state.SetEth1Data(eth1Data)
	if err != nil {
		return nil, err
	}
	return state, nil
}
