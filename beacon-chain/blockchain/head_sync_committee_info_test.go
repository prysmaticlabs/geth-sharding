package blockchain

import (
	"context"
	"testing"

	types "github.com/prysmaticlabs/eth2-types"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/testutil"
	"github.com/prysmaticlabs/prysm/shared/testutil/require"
)

func TestService_HeadCurrentSyncCommitteeIndices(t *testing.T) {
	s, _ := testutil.DeterministicGenesisStateAltair(t, params.BeaconConfig().TargetCommitteeSize)
	c := &Service{}
	c.head = &head{state: s}

	// Process slot up to `EpochsPerSyncCommitteePeriod` so it can `ProcessSyncCommitteeUpdates`.
	slot := uint64(params.BeaconConfig().EpochsPerSyncCommitteePeriod)*uint64(params.BeaconConfig().SlotsPerEpoch) + 1
	indices, err := c.HeadCurrentSyncCommitteeIndices(context.Background(), 0, types.Slot(slot))
	require.NoError(t, err)

	// NextSyncCommittee becomes CurrentSyncCommittee so it should be empty by default.
	require.Equal(t, 0, len(indices))
}

func TestService_HeadNextSyncCommitteeIndices(t *testing.T) {
	s, _ := testutil.DeterministicGenesisStateAltair(t, params.BeaconConfig().TargetCommitteeSize)
	c := &Service{}
	c.head = &head{state: s}

	// Process slot up to `EpochsPerSyncCommitteePeriod` so it can `ProcessSyncCommitteeUpdates`.
	slot := uint64(params.BeaconConfig().EpochsPerSyncCommitteePeriod)*uint64(params.BeaconConfig().SlotsPerEpoch) + 1
	indices, err := c.HeadNextSyncCommitteeIndices(context.Background(), 0, types.Slot(slot))
	require.NoError(t, err)

	// NextSyncCommittee should be be empty after `ProcessSyncCommitteeUpdates`. Validator should get indices.
	require.NotEqual(t, 0, len(indices))
}

func TestService_HeadSyncCommitteePubKeys(t *testing.T) {
	s, _ := testutil.DeterministicGenesisStateAltair(t, params.BeaconConfig().TargetCommitteeSize)
	c := &Service{}
	c.head = &head{state: s}

	// Process slot up to 2 * `EpochsPerSyncCommitteePeriod` so it can run `ProcessSyncCommitteeUpdates` twice.
	slot := uint64(2*params.BeaconConfig().EpochsPerSyncCommitteePeriod)*uint64(params.BeaconConfig().SlotsPerEpoch) + 1
	pubkeys, err := c.HeadSyncCommitteePubKeys(context.Background(), types.Slot(slot), 0)
	require.NoError(t, err)

	// Any subcommittee should match the subcommittee size.
	subCommitteeSize := params.BeaconConfig().SyncCommitteeSize / params.BeaconConfig().SyncCommitteeSubnetCount
	require.Equal(t, int(subCommitteeSize), len(pubkeys))
}

func TestService_HeadSyncCommitteeDomain(t *testing.T) {
	s, _ := testutil.DeterministicGenesisStateAltair(t, params.BeaconConfig().TargetCommitteeSize)
	c := &Service{}
	c.head = &head{state: s}

	wanted, err := helpers.Domain(s.Fork(), helpers.SlotToEpoch(s.Slot()), params.BeaconConfig().DomainSyncCommittee, s.GenesisValidatorRoot())
	require.NoError(t, err)

	d, err := c.HeadSyncCommitteeDomain(context.Background(), 0)
	require.NoError(t, err)

	require.DeepEqual(t, wanted, d)
}

func TestService_HeadSyncContributionProofDomain(t *testing.T) {
	s, _ := testutil.DeterministicGenesisStateAltair(t, params.BeaconConfig().TargetCommitteeSize)
	c := &Service{}
	c.head = &head{state: s}

	wanted, err := helpers.Domain(s.Fork(), helpers.SlotToEpoch(s.Slot()), params.BeaconConfig().DomainContributionAndProof, s.GenesisValidatorRoot())
	require.NoError(t, err)

	d, err := c.HeadSyncContributionProofDomain(context.Background(), 0)
	require.NoError(t, err)

	require.DeepEqual(t, wanted, d)
}

func TestService_HeadSyncSelectionProofDomain(t *testing.T) {
	s, _ := testutil.DeterministicGenesisStateAltair(t, params.BeaconConfig().TargetCommitteeSize)
	c := &Service{}
	c.head = &head{state: s}

	wanted, err := helpers.Domain(s.Fork(), helpers.SlotToEpoch(s.Slot()), params.BeaconConfig().DomainSyncCommitteeSelectionProof, s.GenesisValidatorRoot())
	require.NoError(t, err)

	d, err := c.HeadSyncSelectionProofDomain(context.Background(), 0)
	require.NoError(t, err)

	require.DeepEqual(t, wanted, d)
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
