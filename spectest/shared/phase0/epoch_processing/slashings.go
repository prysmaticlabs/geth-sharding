package epoch_processing

import (
	"context"
	"path"
	"testing"

	"github.com/prysmaticlabs/prysm/beacon-chain/core/epoch"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/epoch/precompute"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	iface "github.com/prysmaticlabs/prysm/beacon-chain/state/interface"
	"github.com/prysmaticlabs/prysm/shared/testutil/require"
	"github.com/prysmaticlabs/prysm/spectest/utils"
)

// RunSlashingsTests executes "epoch_processing/slashings" tests.
func RunSlashingsTests(t *testing.T, config string) {
	require.NoError(t, utils.SetConfig(t, config))

	testFolders, testsFolderPath := utils.TestFolders(t, config, "phase0", "epoch_processing/slashings/pyspec_tests")
	for _, folder := range testFolders {
		helpers.ClearCache()
		t.Run(folder.Name(), func(t *testing.T) {
			folderPath := path.Join(testsFolderPath, folder.Name())
			RunEpochOperationTest(t, folderPath, processSlashingsWrapper)
			RunEpochOperationTest(t, folderPath, processSlashingsPrecomputeWrapper)
		})
	}
}

func processSlashingsWrapper(t *testing.T, state iface.BeaconState) (iface.BeaconState, error) {
	state, err := epoch.ProcessSlashings(state)
	require.NoError(t, err, "Could not process slashings")
	return state, nil
}

func processSlashingsPrecomputeWrapper(t *testing.T, state iface.BeaconState) (iface.BeaconState, error) {
	ctx := context.Background()
	vp, bp, err := precompute.New(ctx, state)
	require.NoError(t, err)
	_, bp, err = precompute.ProcessAttestations(ctx, state, vp, bp)
	require.NoError(t, err)

	return state, precompute.ProcessSlashingsPrecompute(state, bp)
}
