package validator

import (
	"context"
	"testing"

	types "github.com/prysmaticlabs/eth2-types"
	mock "github.com/prysmaticlabs/prysm/beacon-chain/blockchain/testing"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/state"
	"github.com/prysmaticlabs/prysm/beacon-chain/operations/synccommittee"
	mockp2p "github.com/prysmaticlabs/prysm/beacon-chain/p2p/testing"
	prysmv2 "github.com/prysmaticlabs/prysm/proto/prysm/v2"
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

	server := &Server{
		HeadFetcher: &mock.ChainService{
			CurrentSyncCommitteeIndices: []types.CommitteeIndex{0},
			NextSyncCommitteeIndices:    []types.CommitteeIndex{1},
		},
	}
	pubKey := [48]byte{}
	// Request slot 0, should get the index 0 for validator 0.
	res, err := server.GetSyncSubcommitteeIndex(context.Background(), &prysmv2.SyncSubcommitteeIndexRequest{
		PublicKey: pubKey[:], Slot: types.Slot(0),
	})
	require.NoError(t, err)
	require.DeepEqual(t, []types.CommitteeIndex{0}, res.Indices)

	// Request at period boundary, should get index 1 for validator 0.
	periodBoundary := types.Slot(params.BeaconConfig().EpochsPerSyncCommitteePeriod)*params.BeaconConfig().SlotsPerEpoch - 1
	res, err = server.GetSyncSubcommitteeIndex(context.Background(), &prysmv2.SyncSubcommitteeIndexRequest{
		PublicKey: pubKey[:], Slot: periodBoundary,
	})
	require.NoError(t, err)
	require.DeepEqual(t, []types.CommitteeIndex{1}, res.Indices)
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
