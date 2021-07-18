package state_test

import (
	"context"
	"testing"

	types "github.com/prysmaticlabs/eth2-types"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/state"
	v1 "github.com/prysmaticlabs/prysm/beacon-chain/state/v1"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	ethpb "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/shared/hashutil"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/testutil"
	"github.com/prysmaticlabs/prysm/shared/testutil/assert"
	"github.com/prysmaticlabs/prysm/shared/testutil/require"
	"google.golang.org/protobuf/proto"
)

func TestGenesisBeaconState_OK(t *testing.T) {
	genesisEpoch := types.Epoch(0)

	assert.DeepEqual(t, []byte{0, 0, 0, 0}, params.BeaconConfig().GenesisForkVersion, "GenesisSlot( should be {0,0,0,0} for these tests to pass")
	genesisForkVersion := params.BeaconConfig().GenesisForkVersion

	assert.Equal(t, [32]byte{}, params.BeaconConfig().ZeroHash, "ZeroHash should be all 0s for these tests to pass")
	assert.Equal(t, types.Epoch(65536), params.BeaconConfig().EpochsPerHistoricalVector, "EpochsPerHistoricalVector should be 8192 for these tests to pass")

	latestRandaoMixesLength := params.BeaconConfig().EpochsPerHistoricalVector
	assert.Equal(t, uint64(16777216), params.BeaconConfig().HistoricalRootsLimit, "HistoricalRootsLimit should be 16777216 for these tests to pass")

	depositsForChainStart := 100
	assert.Equal(t, types.Epoch(8192), params.BeaconConfig().EpochsPerSlashingsVector, "EpochsPerSlashingsVector should be 8192 for these tests to pass")

	genesisTime := uint64(99999)
	deposits, _, err := testutil.DeterministicDepositsAndKeys(uint64(depositsForChainStart))
	require.NoError(t, err)
	eth1Data, err := testutil.DeterministicEth1Data(len(deposits))
	require.NoError(t, err)
	newState, err := state.GenesisBeaconState(context.Background(), deposits, genesisTime, eth1Data)
	require.NoError(t, err, "Could not execute GenesisBeaconState")

	// Misc fields checks.
	assert.Equal(t, types.Slot(0), newState.Slot(), "Slot was not correctly initialized")
	if !proto.Equal(newState.Fork(), &pb.Fork{
		PreviousVersion: genesisForkVersion,
		CurrentVersion:  genesisForkVersion,
		Epoch:           genesisEpoch,
	}) {
		t.Error("Fork was not correctly initialized")
	}

	// Validator registry fields checks.
	assert.Equal(t, depositsForChainStart, len(newState.Validators()), "Validators was not correctly initialized")
	v, err := newState.ValidatorAtIndex(0)
	require.NoError(t, err)
	assert.Equal(t, types.Epoch(0), v.ActivationEpoch, "Validators was not correctly initialized")
	v, err = newState.ValidatorAtIndex(0)
	require.NoError(t, err)
	assert.Equal(t, types.Epoch(0), v.ActivationEligibilityEpoch, "Validators was not correctly initialized")
	assert.Equal(t, depositsForChainStart, len(newState.Balances()), "Balances was not correctly initialized")

	// Randomness and committees fields checks.
	assert.Equal(t, latestRandaoMixesLength, types.Epoch(len(newState.RandaoMixes())), "Length of RandaoMixes was not correctly initialized")
	mix, err := newState.RandaoMixAtIndex(0)
	require.NoError(t, err)
	assert.DeepEqual(t, eth1Data.BlockHash, mix, "RandaoMixes was not correctly initialized")

	// Finality fields checks.
	assert.Equal(t, genesisEpoch, newState.PreviousJustifiedCheckpoint().Epoch, "PreviousJustifiedCheckpoint.Epoch was not correctly initialized")
	assert.Equal(t, genesisEpoch, newState.CurrentJustifiedCheckpoint().Epoch, "JustifiedEpoch was not correctly initialized")
	assert.Equal(t, genesisEpoch, newState.FinalizedCheckpointEpoch(), "FinalizedSlot was not correctly initialized")
	assert.Equal(t, uint8(0x00), newState.JustificationBits()[0], "JustificationBits was not correctly initialized")

	// Recent state checks.
	assert.DeepEqual(t, make([]uint64, params.BeaconConfig().EpochsPerSlashingsVector), newState.Slashings(), "Slashings was not correctly initialized")
	currAtt, err := newState.CurrentEpochAttestations()
	require.NoError(t, err)
	assert.DeepSSZEqual(t, []*pb.PendingAttestation{}, currAtt, "CurrentEpochAttestations was not correctly initialized")
	prevAtt, err := newState.CurrentEpochAttestations()
	require.NoError(t, err)
	assert.DeepSSZEqual(t, []*pb.PendingAttestation{}, prevAtt, "PreviousEpochAttestations was not correctly initialized")

	zeroHash := params.BeaconConfig().ZeroHash[:]
	// History root checks.
	assert.DeepEqual(t, zeroHash, newState.StateRoots()[0], "StateRoots was not correctly initialized")
	assert.DeepEqual(t, zeroHash, newState.BlockRoots()[0], "BlockRoots was not correctly initialized")

	// Deposit root checks.
	assert.DeepEqual(t, eth1Data.DepositRoot, newState.Eth1Data().DepositRoot, "Eth1Data DepositRoot was not correctly initialized")
	assert.DeepSSZEqual(t, []*ethpb.Eth1Data{}, newState.Eth1DataVotes(), "Eth1DataVotes was not correctly initialized")
}

func TestGenesisState_HashEquality(t *testing.T) {
	deposits, _, err := testutil.DeterministicDepositsAndKeys(100)
	require.NoError(t, err)
	state1, err := state.GenesisBeaconState(context.Background(), deposits, 0, &ethpb.Eth1Data{BlockHash: make([]byte, 32)})
	require.NoError(t, err)
	state2, err := state.GenesisBeaconState(context.Background(), deposits, 0, &ethpb.Eth1Data{BlockHash: make([]byte, 32)})
	require.NoError(t, err)

	pbState1, err := v1.ProtobufBeaconState(state1.CloneInnerState())
	require.NoError(t, err)
	pbState2, err := v1.ProtobufBeaconState(state2.CloneInnerState())
	require.NoError(t, err)

	root1, err1 := hashutil.HashProto(pbState1)
	root2, err2 := hashutil.HashProto(pbState2)

	if err1 != nil || err2 != nil {
		t.Fatalf("Failed to marshal state to bytes: %v %v", err1, err2)
	}
	require.DeepEqual(t, root1, root2, "Tree hash of two genesis states should be equal, received %#x == %#x", root1, root2)
}

func TestGenesisState_InitializesLatestBlockHashes(t *testing.T) {
	s, err := state.GenesisBeaconState(context.Background(), nil, 0, &ethpb.Eth1Data{})
	require.NoError(t, err)
	got, want := uint64(len(s.BlockRoots())), uint64(params.BeaconConfig().SlotsPerHistoricalRoot)
	assert.Equal(t, want, got, "Wrong number of recent block hashes")

	got = uint64(cap(s.BlockRoots()))
	assert.Equal(t, want, got, "The slice underlying array capacity is wrong")

	for _, h := range s.BlockRoots() {
		assert.DeepEqual(t, params.BeaconConfig().ZeroHash[:], h, "Unexpected non-zero hash data")
	}
}

func TestGenesisState_FailsWithoutEth1data(t *testing.T) {
	_, err := state.GenesisBeaconState(context.Background(), nil, 0, nil)
	assert.ErrorContains(t, "no eth1data provided for genesis state", err)
}
