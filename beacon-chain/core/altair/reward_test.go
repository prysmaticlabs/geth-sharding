package altair_test

import (
	"testing"

	altair "github.com/prysmaticlabs/prysm/beacon-chain/core/altair"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/testutil"
	"github.com/prysmaticlabs/prysm/shared/testutil/require"
)

func TestBaseReward(t *testing.T) {
	s, _ := testutil.DeterministicGenesisStateAltair(t, params.BeaconConfig().MaxValidatorsPerCommittee)
	r0, err := altair.BaseReward(s, 0)
	require.NoError(t, err)
	r1, err := altair.BaseReward(s, 1)
	require.NoError(t, err)
	require.Equal(t, r0, r1)

	v, err := s.ValidatorAtIndex(0)
	require.NoError(t, err)
	v.EffectiveBalance = v.EffectiveBalance + params.BeaconConfig().EffectiveBalanceIncrement
	require.NoError(t, s.UpdateValidatorAtIndex(0, v))

	r0, err = altair.BaseReward(s, 0)
	require.NoError(t, err)
	require.Equal(t, true, r0 > r1)
}
