package cache_test

import (
	"context"
	"testing"

	"github.com/prysmaticlabs/prysm/beacon-chain/cache"
	iface "github.com/prysmaticlabs/prysm/beacon-chain/state/interface"
	v1 "github.com/prysmaticlabs/prysm/beacon-chain/state/v1"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	"github.com/prysmaticlabs/prysm/shared/testutil/assert"
	"github.com/prysmaticlabs/prysm/shared/testutil/require"
)

func TestSkipSlotCache_RoundTrip(t *testing.T) {
	ctx := context.Background()
	c := cache.NewSkipSlotCache()

	r := [32]byte{'a'}
	state, err := c.Get(ctx, r)
	require.NoError(t, err)
	assert.Equal(t, iface.BeaconState(nil), state, "Empty cache returned an object")

	require.NoError(t, c.MarkInProgress(r))

	state, err = v1.InitializeFromProto(&pb.BeaconState{
		Slot: 10,
	})
	require.NoError(t, err)

	require.NoError(t, c.Put(ctx, r, state))
	require.NoError(t, c.MarkNotInProgress(r))

	res, err := c.Get(ctx, r)
	require.NoError(t, err)
	assert.DeepEqual(t, res.CloneInnerState(), state.CloneInnerState(), "Expected equal protos to return from cache")
}
