package v2

import (
	"testing"

	"github.com/prysmaticlabs/prysm/shared/testutil/require"
)

func TestBeaconState_AppendCurrentEpochAttestations(t *testing.T) {
	s := &BeaconState{}
	require.ErrorContains(t, "AppendCurrentEpochAttestations is not supported for hard fork 1 beacon state", s.AppendCurrentEpochAttestations(nil))
}

func TestBeaconState_AppendPreviousEpochAttestations(t *testing.T) {
	s := &BeaconState{}
	require.ErrorContains(t, "AppendPreviousEpochAttestations is not supported for hard fork 1 beacon state", s.AppendPreviousEpochAttestations(nil))
}

func TestBeaconState_SetCurrentEpochAttestations(t *testing.T) {
	s := &BeaconState{}
	require.ErrorContains(t, "SetCurrentEpochAttestations is not supported for hard fork 1 beacon state", s.SetCurrentEpochAttestations(nil))
}

func TestBeaconState_SetPreviousEpochAttestations(t *testing.T) {
	s := &BeaconState{}
	require.ErrorContains(t, "SetPreviousEpochAttestations is not supported for hard fork 1 beacon state", s.SetPreviousEpochAttestations(nil))
}
