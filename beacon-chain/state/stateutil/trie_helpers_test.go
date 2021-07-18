package stateutil_test

import (
	"testing"

	types "github.com/prysmaticlabs/eth2-types"
	"github.com/prysmaticlabs/prysm/beacon-chain/state/stateutil"
	v1 "github.com/prysmaticlabs/prysm/beacon-chain/state/v1"
	ethpb "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/prysmaticlabs/prysm/shared/hashutil"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/testutil"
	"github.com/prysmaticlabs/prysm/shared/testutil/assert"
	"github.com/prysmaticlabs/prysm/shared/testutil/require"
)

func TestReturnTrieLayer_OK(t *testing.T) {
	newState, _ := testutil.DeterministicGenesisState(t, 32)
	root, err := v1.RootsArrayHashTreeRoot(newState.BlockRoots(), uint64(params.BeaconConfig().SlotsPerHistoricalRoot), "BlockRoots")
	require.NoError(t, err)
	blockRts := newState.BlockRoots()
	roots := make([][32]byte, 0, len(blockRts))
	for _, rt := range blockRts {
		roots = append(roots, bytesutil.ToBytes32(rt))
	}
	layers := stateutil.ReturnTrieLayer(roots, uint64(len(roots)))
	newRoot := *layers[len(layers)-1][0]
	assert.Equal(t, root, newRoot)
}

func TestReturnTrieLayerVariable_OK(t *testing.T) {
	newState, _ := testutil.DeterministicGenesisState(t, 32)
	root, err := v1.ValidatorRegistryRoot(newState.Validators())
	require.NoError(t, err)
	hasher := hashutil.CustomSHA256Hasher()
	validators := newState.Validators()
	roots := make([][32]byte, 0, len(validators))
	for _, val := range validators {
		rt, err := stateutil.ValidatorRootWithHasher(hasher, val)
		require.NoError(t, err)
		roots = append(roots, rt)
	}
	layers := stateutil.ReturnTrieLayerVariable(roots, params.BeaconConfig().ValidatorRegistryLimit)
	newRoot := *layers[len(layers)-1][0]
	newRoot, err = stateutil.AddInMixin(newRoot, uint64(len(validators)))
	require.NoError(t, err)
	assert.Equal(t, root, newRoot)
}

func TestRecomputeFromLayer_FixedSizedArray(t *testing.T) {
	newState, _ := testutil.DeterministicGenesisState(t, 32)
	blockRts := newState.BlockRoots()
	roots := make([][32]byte, 0, len(blockRts))
	for _, rt := range blockRts {
		roots = append(roots, bytesutil.ToBytes32(rt))
	}
	layers := stateutil.ReturnTrieLayer(roots, uint64(len(roots)))

	changedIdx := []uint64{24, 41}
	changedRoots := [][32]byte{{'A', 'B', 'C'}, {'D', 'E', 'F'}}
	require.NoError(t, newState.UpdateBlockRootAtIndex(changedIdx[0], changedRoots[0]))
	require.NoError(t, newState.UpdateBlockRootAtIndex(changedIdx[1], changedRoots[1]))

	expectedRoot, err := v1.RootsArrayHashTreeRoot(newState.BlockRoots(), uint64(params.BeaconConfig().SlotsPerHistoricalRoot), "BlockRoots")
	require.NoError(t, err)
	root, _, err := stateutil.RecomputeFromLayer(changedRoots, changedIdx, layers)
	require.NoError(t, err)
	assert.Equal(t, expectedRoot, root)
}

func TestRecomputeFromLayer_VariableSizedArray(t *testing.T) {
	newState, _ := testutil.DeterministicGenesisState(t, 32)
	validators := newState.Validators()
	hasher := hashutil.CustomSHA256Hasher()
	roots := make([][32]byte, 0, len(validators))
	for _, val := range validators {
		rt, err := stateutil.ValidatorRootWithHasher(hasher, val)
		require.NoError(t, err)
		roots = append(roots, rt)
	}
	layers := stateutil.ReturnTrieLayerVariable(roots, params.BeaconConfig().ValidatorRegistryLimit)

	changedIdx := []uint64{2, 29}
	val1, err := newState.ValidatorAtIndex(10)
	require.NoError(t, err)
	val2, err := newState.ValidatorAtIndex(11)
	require.NoError(t, err)
	val1.Slashed = true
	val1.ExitEpoch = 20

	val2.Slashed = true
	val2.ExitEpoch = 40

	changedVals := []*ethpb.Validator{val1, val2}
	require.NoError(t, newState.UpdateValidatorAtIndex(types.ValidatorIndex(changedIdx[0]), changedVals[0]))
	require.NoError(t, newState.UpdateValidatorAtIndex(types.ValidatorIndex(changedIdx[1]), changedVals[1]))

	expectedRoot, err := v1.ValidatorRegistryRoot(newState.Validators())
	require.NoError(t, err)
	roots = make([][32]byte, 0, len(changedVals))
	for _, val := range changedVals {
		rt, err := stateutil.ValidatorRootWithHasher(hasher, val)
		require.NoError(t, err)
		roots = append(roots, rt)
	}
	root, _, err := stateutil.RecomputeFromLayerVariable(roots, changedIdx, layers)
	require.NoError(t, err)
	root, err = stateutil.AddInMixin(root, uint64(len(validators)))
	require.NoError(t, err)
	assert.Equal(t, expectedRoot, root)
}
