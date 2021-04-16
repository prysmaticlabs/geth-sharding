package debugv1

import (
	"context"
	"testing"

	protoTypes "github.com/gogo/protobuf/types"
	types "github.com/prysmaticlabs/eth2-types"
	ethpb "github.com/prysmaticlabs/ethereumapis/eth/v1"
	blockchainmock "github.com/prysmaticlabs/prysm/beacon-chain/blockchain/testing"
	"github.com/prysmaticlabs/prysm/beacon-chain/rpc/testutil"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	sharedtestutil "github.com/prysmaticlabs/prysm/shared/testutil"
	"github.com/prysmaticlabs/prysm/shared/testutil/assert"
	"github.com/prysmaticlabs/prysm/shared/testutil/require"
)

func TestGetBeaconState(t *testing.T) {
	fakeState, err := sharedtestutil.NewBeaconState()
	require.NoError(t, err)
	server := &Server{
		StateFetcher: &testutil.MockFetcher{
			BeaconState: fakeState,
		},
	}
	resp, err := server.GetBeaconState(context.Background(), &ethpb.StateRequest{
		StateId: make([]byte, 0),
	})
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestListForkChoiceHeads(t *testing.T) {
	ctx := context.Background()

	expectedSlotsAndRoots := []struct {
		Slot types.Slot
		Root [32]byte
	}{{
		Slot: 0,
		Root: bytesutil.ToBytes32(bytesutil.PadTo([]byte("foo"), 32)),
	}, {
		Slot: 1,
		Root: bytesutil.ToBytes32(bytesutil.PadTo([]byte("bar"), 32)),
	}}

	server := &Server{
		HeadFetcher: &blockchainmock.ChainService{},
	}
	resp, err := server.ListForkChoiceHeads(ctx, &protoTypes.Empty{})
	require.NoError(t, err)
	assert.Equal(t, 2, len(resp.Data))
	for _, sr := range expectedSlotsAndRoots {
		found := false
		for _, h := range resp.Data {
			if h.Slot == sr.Slot {
				found = true
				assert.DeepEqual(t, sr.Root[:], h.Root)
			}
		}
		assert.Equal(t, true, found, "Expected head not found")
	}
}
