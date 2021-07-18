package operations

import (
	"context"
	"path"
	"testing"

	"github.com/golang/snappy"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/altair"
	iface "github.com/prysmaticlabs/prysm/beacon-chain/state/interface"
	"github.com/prysmaticlabs/prysm/proto/interfaces"
	prysmv2 "github.com/prysmaticlabs/prysm/proto/prysm/v2"
	"github.com/prysmaticlabs/prysm/shared/testutil"
	"github.com/prysmaticlabs/prysm/shared/testutil/require"
	"github.com/prysmaticlabs/prysm/spectest/utils"
)

func RunSyncCommitteeTest(t *testing.T, config string) {
	require.NoError(t, utils.SetConfig(t, config))
	testFolders, testsFolderPath := utils.TestFolders(t, config, "altair", "operations/sync_aggregate/pyspec_tests")
	for _, folder := range testFolders {
		t.Run(folder.Name(), func(t *testing.T) {
			folderPath := path.Join(testsFolderPath, folder.Name())
			syncCommitteeFile, err := testutil.BazelFileBytes(folderPath, "sync_aggregate.ssz_snappy")
			require.NoError(t, err)
			syncCommitteeSSZ, err := snappy.Decode(nil /* dst */, syncCommitteeFile)
			require.NoError(t, err, "Failed to decompress")
			sc := &prysmv2.SyncAggregate{}
			require.NoError(t, sc.UnmarshalSSZ(syncCommitteeSSZ), "Failed to unmarshal")

			body := &prysmv2.BeaconBlockBody{SyncAggregate: sc}
			RunBlockOperationTest(t, folderPath, body, func(ctx context.Context, s iface.BeaconState, b interfaces.SignedBeaconBlock) (iface.BeaconState, error) {
				return altair.ProcessSyncAggregate(s, body.SyncAggregate)
			})
		})
	}
}
