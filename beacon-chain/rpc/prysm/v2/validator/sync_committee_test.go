package validator

import (
	"context"
	"testing"

	types "github.com/prysmaticlabs/eth2-types"
	mock "github.com/prysmaticlabs/prysm/beacon-chain/blockchain/testing"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/state"
	"github.com/prysmaticlabs/prysm/beacon-chain/operations/synccommittee"
	mockp2p "github.com/prysmaticlabs/prysm/beacon-chain/p2p/testing"
	stateAltair "github.com/prysmaticlabs/prysm/beacon-chain/state/v2"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	prysmv2 "github.com/prysmaticlabs/prysm/proto/prysm/v2"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/testutil"
	"github.com/prysmaticlabs/prysm/shared/testutil/require"
	"google.golang.org/protobuf/types/known/emptypb"
)

func TestGetSyncMessageBlockRoot_OK(t *testing.T) {
	r := []byte{'a'}
	server := &Server{
		HeadFetcher: &mock.ChainService{Root: r},
	}
	res, err := server.GetSyncMessageBlockRoot(context.Background(), &emptypb.Empty{})
	require.NoError(t, err)
	require.DeepEqual(t, r, res.Root)
}

func TestSubmitSyncMessage_OK(t *testing.T) {
	st, _ := testutil.DeterministicGenesisStateAltair(t, 10)
	server := &Server{
		SyncCommitteePool: synccommittee.NewStore(),
		P2P:               &mockp2p.MockBroadcaster{},
		HeadFetcher: &mock.ChainService{
			State: st,
		},
	}
	msg := &prysmv2.SyncCommitteeMessage{
		Slot:           1,
		ValidatorIndex: 2,
	}
	_, err := server.SubmitSyncMessage(context.Background(), msg)
	require.NoError(t, err)
	savedMsgs, err := server.SyncCommitteePool.SyncCommitteeMessages(1)
	require.NoError(t, err)
	require.DeepEqual(t, []*prysmv2.SyncCommitteeMessage{msg}, savedMsgs)
}

func TestGetSyncSubcommitteeIndex_Ok(t *testing.T) {
	params.SetupTestConfigCleanup(t)
	params.OverrideBeaconConfig(params.MainnetConfig())
	state.SkipSlotCache.Disable()
	defer state.SkipSlotCache.Enable()

	headState, _ := testutil.DeterministicGenesisStateAltair(t, params.BeaconConfig().MaxValidatorsPerCommittee)
	require.NoError(t, headState.SetSlot(100))

	// Set sync committee for current period.
	syncCommittee := &pb.SyncCommittee{
		AggregatePubkey: bytesutil.PadTo([]byte{}, params.BeaconConfig().BLSPubkeyLength),
	}
	for i := 0; i < 512; i++ {
		v, err := headState.ValidatorAtIndex(types.ValidatorIndex(i))
		require.NoError(t, err)
		syncCommittee.Pubkeys = append(syncCommittee.Pubkeys, bytesutil.PadTo(v.PublicKey, 48))
	}
	require.NoError(t, headState.SetCurrentSyncCommittee(syncCommittee))

	// Set sync committee for next period. Shuffle index 0 and index 1.
	syncCommittee = stateAltair.CopySyncCommittee(syncCommittee)
	syncCommittee.Pubkeys[0], syncCommittee.Pubkeys[1] = syncCommittee.Pubkeys[1], syncCommittee.Pubkeys[0]
	require.NoError(t, headState.SetNextSyncCommittee(syncCommittee))

	server := &Server{
		HeadFetcher: &mock.ChainService{
			State: headState,
		},
	}
	v, err := headState.ValidatorAtIndex(0)
	require.NoError(t, err)

	// Request slot 0, should get the index 0 for validator 0.
	res, err := server.GetSyncSubcommitteeIndex(context.Background(), &prysmv2.SyncSubcommitteeIndexRequest{
		PublicKey: v.PublicKey, Slot: types.Slot(0),
	})
	require.NoError(t, err)
	require.DeepEqual(t, []uint64{0}, res.Indices)

	// Request at period boundary, should get index 1 for validator 0 due to the shuffle.
	periodBoundary := types.Slot(params.BeaconConfig().EpochsPerSyncCommitteePeriod)*params.BeaconConfig().SlotsPerEpoch - 1
	res, err = server.GetSyncSubcommitteeIndex(context.Background(), &prysmv2.SyncSubcommitteeIndexRequest{
		PublicKey: v.PublicKey, Slot: periodBoundary,
	})
	require.NoError(t, err)
	require.DeepEqual(t, []uint64{1}, res.Indices)
}

func TestSubmitSignedContributionAndProof_OK(t *testing.T) {
	server := &Server{
		SyncCommitteePool: synccommittee.NewStore(),
		P2P:               &mockp2p.MockBroadcaster{},
	}
	contribution := &prysmv2.SignedContributionAndProof{
		Message: &prysmv2.ContributionAndProof{
			Contribution: &prysmv2.SyncCommitteeContribution{
				Slot:              1,
				SubcommitteeIndex: 2,
			},
		},
	}
	_, err := server.SubmitSignedContributionAndProof(context.Background(), contribution)
	require.NoError(t, err)
	savedMsgs, err := server.SyncCommitteePool.SyncCommitteeContributions(1)
	require.NoError(t, err)
	require.DeepEqual(t, []*prysmv2.SyncCommitteeContribution{contribution.Message.Contribution}, savedMsgs)
}

func TestSyncCommitteeHeadStateCache_RoundTrip(t *testing.T) {
	c := newSyncCommitteeHeadState()
	beaconState, _ := testutil.DeterministicGenesisStateAltair(t, 100)
	require.NoError(t, beaconState.SetSlot(100))
	cachedState := c.get(101)
	require.Equal(t, nil, cachedState)
	c.add(101, beaconState)
	cachedState = c.get(101)
	require.DeepEqual(t, beaconState, cachedState)
}
