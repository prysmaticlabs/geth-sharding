package cache

import (
	"testing"

	types "github.com/prysmaticlabs/eth2-types"
	iface "github.com/prysmaticlabs/prysm/beacon-chain/state/interface"
	v1 "github.com/prysmaticlabs/prysm/beacon-chain/state/v1"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	ethpb "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/testutil/assert"
	"github.com/prysmaticlabs/prysm/shared/testutil/require"
	"google.golang.org/protobuf/proto"
)

func TestCheckpointStateCache_StateByCheckpoint(t *testing.T) {
	cache := NewCheckpointStateCache()

	cp1 := &ethpb.Checkpoint{Epoch: 1, Root: bytesutil.PadTo([]byte{'A'}, 32)}
	st, err := v1.InitializeFromProto(&pb.BeaconState{
		GenesisValidatorsRoot: params.BeaconConfig().ZeroHash[:],
		Slot:                  64,
	})
	require.NoError(t, err)

	state, err := cache.StateByCheckpoint(cp1)
	require.NoError(t, err)
	assert.Equal(t, iface.BeaconState(nil), state, "Expected state not to exist in empty cache")

	require.NoError(t, cache.AddCheckpointState(cp1, st))

	state, err = cache.StateByCheckpoint(cp1)
	require.NoError(t, err)

	pbState1, err := v1.ProtobufBeaconState(state.InnerStateUnsafe())
	require.NoError(t, err)
	pbState2, err := v1.ProtobufBeaconState(st.InnerStateUnsafe())
	require.NoError(t, err)
	if !proto.Equal(pbState1, pbState2) {
		t.Error("incorrectly cached state")
	}

	cp2 := &ethpb.Checkpoint{Epoch: 2, Root: bytesutil.PadTo([]byte{'B'}, 32)}
	st2, err := v1.InitializeFromProto(&pb.BeaconState{
		Slot: 128,
	})
	require.NoError(t, err)
	require.NoError(t, cache.AddCheckpointState(cp2, st2))

	state, err = cache.StateByCheckpoint(cp2)
	require.NoError(t, err)
	assert.DeepEqual(t, st2.CloneInnerState(), state.CloneInnerState(), "incorrectly cached state")

	state, err = cache.StateByCheckpoint(cp1)
	require.NoError(t, err)
	assert.DeepEqual(t, st.CloneInnerState(), state.CloneInnerState(), "incorrectly cached state")
}

func TestCheckpointStateCache_MaxSize(t *testing.T) {
	c := NewCheckpointStateCache()
	st, err := v1.InitializeFromProto(&pb.BeaconState{
		Slot: 0,
	})
	require.NoError(t, err)

	for i := uint64(0); i < uint64(maxCheckpointStateSize+100); i++ {
		require.NoError(t, st.SetSlot(types.Slot(i)))
		require.NoError(t, c.AddCheckpointState(&ethpb.Checkpoint{Epoch: types.Epoch(i), Root: make([]byte, 32)}, st))
	}

	assert.Equal(t, maxCheckpointStateSize, len(c.cache.Keys()))
}
