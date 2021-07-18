package state_test

import (
	"context"
	"sync"
	"testing"

	"github.com/prysmaticlabs/prysm/beacon-chain/core/state"
	iface "github.com/prysmaticlabs/prysm/beacon-chain/state/interface"
	v1 "github.com/prysmaticlabs/prysm/beacon-chain/state/v1"
	"github.com/prysmaticlabs/prysm/proto/eth/v1alpha1/wrapper"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/testutil"
	"github.com/prysmaticlabs/prysm/shared/testutil/assert"
	"github.com/prysmaticlabs/prysm/shared/testutil/require"
)

func TestSkipSlotCache_OK(t *testing.T) {
	state.SkipSlotCache.Enable()
	defer state.SkipSlotCache.Disable()
	bState, privs := testutil.DeterministicGenesisState(t, params.MinimalSpecConfig().MinGenesisActiveValidatorCount)
	pbState, err := v1.ProtobufBeaconState(bState.CloneInnerState())
	require.NoError(t, err)
	originalState, err := v1.InitializeFromProto(pbState)
	require.NoError(t, err)

	blkCfg := testutil.DefaultBlockGenConfig()
	blkCfg.NumAttestations = 1

	// First transition will be with an empty cache, so the cache becomes populated
	// with the state
	blk, err := testutil.GenerateFullBlock(bState, privs, blkCfg, originalState.Slot()+10)
	require.NoError(t, err)
	executedState, err := state.ExecuteStateTransition(context.Background(), originalState, wrapper.WrappedPhase0SignedBeaconBlock(blk))
	require.NoError(t, err, "Could not run state transition")
	originalState, ok := executedState.(*v1.BeaconState)
	require.Equal(t, true, ok)
	bState, err = state.ExecuteStateTransition(context.Background(), bState, wrapper.WrappedPhase0SignedBeaconBlock(blk))
	require.NoError(t, err, "Could not process state transition")

	assert.DeepEqual(t, originalState.CloneInnerState(), bState.CloneInnerState(), "Skipped slots cache leads to different states")
}

func TestSkipSlotCache_ConcurrentMixup(t *testing.T) {
	bState, privs := testutil.DeterministicGenesisState(t, params.MinimalSpecConfig().MinGenesisActiveValidatorCount)
	pbState, err := v1.ProtobufBeaconState(bState.CloneInnerState())
	require.NoError(t, err)
	originalState, err := v1.InitializeFromProto(pbState)
	require.NoError(t, err)

	blkCfg := testutil.DefaultBlockGenConfig()
	blkCfg.NumAttestations = 1

	state.SkipSlotCache.Disable()

	// First transition will be with an empty cache, so the cache becomes populated
	// with the state
	blk, err := testutil.GenerateFullBlock(bState, privs, blkCfg, originalState.Slot()+10)
	require.NoError(t, err)
	executedState, err := state.ExecuteStateTransition(context.Background(), originalState, wrapper.WrappedPhase0SignedBeaconBlock(blk))
	require.NoError(t, err, "Could not run state transition")
	originalState, ok := executedState.(*v1.BeaconState)
	require.Equal(t, true, ok)

	// Create two shallow but different forks
	var state1, state2 iface.BeaconState
	{
		blk, err := testutil.GenerateFullBlock(originalState.Copy(), privs, blkCfg, originalState.Slot()+10)
		require.NoError(t, err)
		copy(blk.Block.Body.Graffiti, "block 1")
		signature, err := testutil.BlockSignature(originalState, blk.Block, privs)
		require.NoError(t, err)
		blk.Signature = signature.Marshal()
		state1, err = state.ExecuteStateTransition(context.Background(), originalState.Copy(), wrapper.WrappedPhase0SignedBeaconBlock(blk))
		require.NoError(t, err, "Could not run state transition")
	}

	{
		blk, err := testutil.GenerateFullBlock(originalState.Copy(), privs, blkCfg, originalState.Slot()+10)
		require.NoError(t, err)
		copy(blk.Block.Body.Graffiti, "block 2")
		signature, err := testutil.BlockSignature(originalState, blk.Block, privs)
		require.NoError(t, err)
		blk.Signature = signature.Marshal()
		state2, err = state.ExecuteStateTransition(context.Background(), originalState.Copy(), wrapper.WrappedPhase0SignedBeaconBlock(blk))
		require.NoError(t, err, "Could not run state transition")
	}

	r1, err := state1.HashTreeRoot(context.Background())
	require.NoError(t, err)
	r2, err := state2.HashTreeRoot(context.Background())
	require.NoError(t, err)
	if r1 == r2 {
		t.Fatalf("need different starting states, got: %x", r1)
	}

	if state1.Slot() != state2.Slot() {
		t.Fatalf("expecting different chains, but states at same slot")
	}

	// prepare copies for both states
	var setups []iface.BeaconState
	for i := uint64(0); i < 300; i++ {
		var st iface.BeaconState
		if i%2 == 0 {
			st = state1
		} else {
			st = state2
		}
		setups = append(setups, st.Copy())
	}

	problemSlot := state1.Slot() + 2
	expected1, err := state.ProcessSlots(context.Background(), state1.Copy(), problemSlot)
	require.NoError(t, err)
	expectedRoot1, err := expected1.HashTreeRoot(context.Background())
	require.NoError(t, err)
	t.Logf("chain 1 (even i) expected root %x at slot %d", expectedRoot1[:], problemSlot)

	tmp1, err := state.ProcessSlots(context.Background(), expected1.Copy(), problemSlot+1)
	require.NoError(t, err)
	gotRoot := tmp1.StateRoots()[problemSlot]
	require.DeepEqual(t, expectedRoot1[:], gotRoot, "State roots for chain 1 are bad, expected root doesn't match")

	expected2, err := state.ProcessSlots(context.Background(), state2.Copy(), problemSlot)
	require.NoError(t, err)
	expectedRoot2, err := expected2.HashTreeRoot(context.Background())
	require.NoError(t, err)
	t.Logf("chain 2 (odd i) expected root %x at slot %d", expectedRoot2[:], problemSlot)

	tmp2, err := state.ProcessSlots(context.Background(), expected2.Copy(), problemSlot+1)
	require.NoError(t, err)
	gotRoot = tmp2.StateRoots()[problemSlot]
	require.DeepEqual(t, expectedRoot2[:], gotRoot, "State roots for chain 2 are bad, expected root doesn't match")

	var wg sync.WaitGroup
	wg.Add(len(setups))

	step := func(i int, setup iface.BeaconState) {
		// go at least 1 past problemSlot, to ensure problem slot state root is available
		outState, err := state.ProcessSlots(context.Background(), setup, problemSlot.Add(1+uint64(i))) // keep increasing, to hit and extend the cache
		require.NoError(t, err, "Could not process state transition")
		roots := outState.StateRoots()
		gotRoot := roots[problemSlot]
		if i%2 == 0 {
			assert.DeepEqual(t, expectedRoot1[:], gotRoot, "Unexpected root on chain 1")
		} else {
			assert.DeepEqual(t, expectedRoot2[:], gotRoot, "Unexpected root on chain 2")
		}
		wg.Done()
	}

	state.SkipSlotCache.Enable()
	// now concurrently apply the blocks (alternating between states, and increasing skip slots)
	for i, setup := range setups {
		go step(i, setup)
	}
	// Wait for all transitions to finish
	wg.Wait()
}
