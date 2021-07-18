package operations

import (
	"context"
	"path"
	"testing"

	"github.com/golang/snappy"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/blocks"
	iface "github.com/prysmaticlabs/prysm/beacon-chain/state/interface"
	ethpb "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/proto/interfaces"
	prysmv2 "github.com/prysmaticlabs/prysm/proto/prysm/v2"
	"github.com/prysmaticlabs/prysm/shared/testutil"
	"github.com/prysmaticlabs/prysm/shared/testutil/require"
	"github.com/prysmaticlabs/prysm/spectest/utils"
)

func RunVoluntaryExitTest(t *testing.T, config string) {
	require.NoError(t, utils.SetConfig(t, config))
	testFolders, testsFolderPath := utils.TestFolders(t, config, "altair", "operations/voluntary_exit/pyspec_tests")
	for _, folder := range testFolders {
		t.Run(folder.Name(), func(t *testing.T) {
			folderPath := path.Join(testsFolderPath, folder.Name())
			exitFile, err := testutil.BazelFileBytes(folderPath, "voluntary_exit.ssz_snappy")
			require.NoError(t, err)
			exitSSZ, err := snappy.Decode(nil /* dst */, exitFile)
			require.NoError(t, err, "Failed to decompress")
			voluntaryExit := &ethpb.SignedVoluntaryExit{}
			require.NoError(t, voluntaryExit.UnmarshalSSZ(exitSSZ), "Failed to unmarshal")

			body := &prysmv2.BeaconBlockBody{VoluntaryExits: []*ethpb.SignedVoluntaryExit{voluntaryExit}}
			RunBlockOperationTest(t, folderPath, body, func(ctx context.Context, s iface.BeaconState, b interfaces.SignedBeaconBlock) (iface.BeaconState, error) {
				return blocks.ProcessVoluntaryExits(ctx, s, b.Block().Body().VoluntaryExits())
			})
		})
	}
}
