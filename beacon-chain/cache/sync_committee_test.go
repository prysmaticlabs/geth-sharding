package cache_test

import (
	"testing"

	types "github.com/prysmaticlabs/eth2-types"
	"github.com/prysmaticlabs/prysm/beacon-chain/cache"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/testutil"
	"github.com/prysmaticlabs/prysm/shared/testutil/require"
)

func TestSyncCommitteeCache_CanUpdateAndRetrieve(t *testing.T) {
	numValidators := 101
	deterministicState, _ := testutil.DeterministicGenesisStateAltair(t, uint64(numValidators))
	pubKeys := make([][]byte, deterministicState.NumValidators())
	for i, val := range deterministicState.Validators() {
		pubKeys[i] = val.PublicKey
	}
	tests := []struct {
		name                 string
		currentSyncCommittee *pb.SyncCommittee
		nextSyncCommittee    *pb.SyncCommittee
		currentSyncMap       map[types.ValidatorIndex][]types.CommitteeIndex
		nextSyncMap          map[types.ValidatorIndex][]types.CommitteeIndex
	}{
		{
			name: "only current epoch",
			currentSyncCommittee: convertToCommittee([][]byte{
				pubKeys[1], pubKeys[2], pubKeys[3], pubKeys[2], pubKeys[2],
			}),
			nextSyncCommittee: convertToCommittee([][]byte{}),
			currentSyncMap: map[types.ValidatorIndex][]types.CommitteeIndex{
				1: {0},
				2: {1, 3, 4},
				3: {2},
			},
			nextSyncMap: map[types.ValidatorIndex][]types.CommitteeIndex{
				1: {},
				2: {},
				3: {},
			},
		},
		{
			name:                 "only next epoch",
			currentSyncCommittee: convertToCommittee([][]byte{}),
			nextSyncCommittee: convertToCommittee([][]byte{
				pubKeys[1], pubKeys[2], pubKeys[3], pubKeys[2], pubKeys[2],
			}),
			currentSyncMap: map[types.ValidatorIndex][]types.CommitteeIndex{
				1: {},
				2: {},
				3: {},
			},
			nextSyncMap: map[types.ValidatorIndex][]types.CommitteeIndex{
				1: {0},
				2: {1, 3, 4},
				3: {2},
			},
		},
		{
			name: "some current epoch and some next epoch",
			currentSyncCommittee: convertToCommittee([][]byte{
				pubKeys[1],
				pubKeys[2],
				pubKeys[3],
				pubKeys[2],
				pubKeys[2],
			}),
			nextSyncCommittee: convertToCommittee([][]byte{
				pubKeys[7],
				pubKeys[6],
				pubKeys[5],
				pubKeys[4],
				pubKeys[7],
			}),
			currentSyncMap: map[types.ValidatorIndex][]types.CommitteeIndex{
				1: {0},
				2: {1, 3, 4},
				3: {2},
			},
			nextSyncMap: map[types.ValidatorIndex][]types.CommitteeIndex{
				7: {0, 4},
				6: {1},
				5: {2},
				4: {3},
			},
		},
		{
			name: "some current epoch and some next epoch duplicated across",
			currentSyncCommittee: convertToCommittee([][]byte{
				pubKeys[1],
				pubKeys[2],
				pubKeys[3],
				pubKeys[2],
				pubKeys[2],
			}),
			nextSyncCommittee: convertToCommittee([][]byte{
				pubKeys[2],
				pubKeys[1],
				pubKeys[3],
				pubKeys[2],
				pubKeys[1],
			}),
			currentSyncMap: map[types.ValidatorIndex][]types.CommitteeIndex{
				1: {0},
				2: {1, 3, 4},
				3: {2},
			},
			nextSyncMap: map[types.ValidatorIndex][]types.CommitteeIndex{
				1: {1, 4},
				2: {0, 3},
				3: {2},
			},
		},
		{
			name: "all duplicated",
			currentSyncCommittee: convertToCommittee([][]byte{
				pubKeys[100],
				pubKeys[100],
				pubKeys[100],
				pubKeys[100],
			}),
			nextSyncCommittee: convertToCommittee([][]byte{
				pubKeys[100],
				pubKeys[100],
				pubKeys[100],
				pubKeys[100],
			}),
			currentSyncMap: map[types.ValidatorIndex][]types.CommitteeIndex{
				100: {0, 1, 2, 3},
			},
			nextSyncMap: map[types.ValidatorIndex][]types.CommitteeIndex{
				100: {0, 1, 2, 3},
			},
		},
		{
			name: "unknown keys",
			currentSyncCommittee: convertToCommittee([][]byte{
				pubKeys[100],
				pubKeys[100],
				pubKeys[100],
				pubKeys[100],
			}),
			nextSyncCommittee: convertToCommittee([][]byte{
				pubKeys[100],
				pubKeys[100],
				pubKeys[100],
				pubKeys[100],
			}),
			currentSyncMap: map[types.ValidatorIndex][]types.CommitteeIndex{
				1: {},
			},
			nextSyncMap: map[types.ValidatorIndex][]types.CommitteeIndex{
				1: {},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, _ := testutil.DeterministicGenesisStateAltair(t, uint64(numValidators))
			require.NoError(t, s.SetCurrentSyncCommittee(tt.currentSyncCommittee))
			require.NoError(t, s.SetNextSyncCommittee(tt.nextSyncCommittee))
			cache := cache.NewSyncCommittee()
			r := [32]byte{'a'}
			require.NoError(t, cache.UpdatePositionsInCommittee(r, s))
			for key, indices := range tt.currentSyncMap {
				pos, err := cache.CurrentEpochIndexPosition(r, key)
				require.NoError(t, err)
				require.DeepEqual(t, indices, pos)
			}
			for key, indices := range tt.nextSyncMap {
				pos, err := cache.NextEpochIndexPosition(r, key)
				require.NoError(t, err)
				require.DeepEqual(t, indices, pos)
			}
		})
	}
}

func TestSyncCommitteeCache_RootDoesNotExist(t *testing.T) {
	c := cache.NewSyncCommittee()
	_, err := c.CurrentEpochIndexPosition([32]byte{}, 0)
	require.Equal(t, cache.ErrNonExistingSyncCommitteeKey, err)
}

func TestSyncCommitteeCache_CanRotate(t *testing.T) {
	c := cache.NewSyncCommittee()
	s, _ := testutil.DeterministicGenesisStateAltair(t, 64)
	require.NoError(t, s.SetCurrentSyncCommittee(convertToCommittee([][]byte{{1}})))
	require.NoError(t, c.UpdatePositionsInCommittee([32]byte{'a'}, s))
	require.NoError(t, s.SetCurrentSyncCommittee(convertToCommittee([][]byte{{2}})))
	require.NoError(t, c.UpdatePositionsInCommittee([32]byte{'b'}, s))
	require.NoError(t, s.SetCurrentSyncCommittee(convertToCommittee([][]byte{{3}})))
	require.NoError(t, c.UpdatePositionsInCommittee([32]byte{'c'}, s))
	require.NoError(t, s.SetCurrentSyncCommittee(convertToCommittee([][]byte{{4}})))
	require.NoError(t, c.UpdatePositionsInCommittee([32]byte{'d'}, s))

	_, err := c.CurrentEpochIndexPosition([32]byte{'a'}, 0)
	require.Equal(t, cache.ErrNonExistingSyncCommitteeKey, err)

	_, err = c.CurrentEpochIndexPosition([32]byte{'c'}, 0)
	require.NoError(t, err)
}

func convertToCommittee(inputKeys [][]byte) *pb.SyncCommittee {
	var pubKeys [][]byte
	for i := uint64(0); i < params.BeaconConfig().SyncCommitteeSize; i++ {
		if i < uint64(len(inputKeys)) {
			pubKeys = append(pubKeys, bytesutil.PadTo(inputKeys[i], params.BeaconConfig().BLSPubkeyLength))
		} else {
			pubKeys = append(pubKeys, bytesutil.PadTo([]byte{}, params.BeaconConfig().BLSPubkeyLength))
		}
	}

	return &pb.SyncCommittee{
		Pubkeys:         pubKeys,
		AggregatePubkey: bytesutil.PadTo([]byte{}, params.BeaconConfig().BLSPubkeyLength),
	}
}
