package epoch_processing

import (
	"io/ioutil"
	"path"
	"strings"
	"testing"

	"github.com/bazelbuild/rules_go/go/tools/bazel"
	"github.com/golang/snappy"
	iface "github.com/prysmaticlabs/prysm/beacon-chain/state/interface"
	stateAltair "github.com/prysmaticlabs/prysm/beacon-chain/state/v2"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	"github.com/prysmaticlabs/prysm/shared/testutil"
	"github.com/prysmaticlabs/prysm/shared/testutil/require"
	"google.golang.org/protobuf/proto"
	"gopkg.in/d4l3k/messagediff.v1"
)

type epochOperation func(*testing.T, iface.BeaconState) (iface.BeaconState, error)

// RunEpochOperationTest takes in the prestate and processes it through the
// passed in epoch operation function and checks the post state with the expected post state.
func RunEpochOperationTest(
	t *testing.T,
	testFolderPath string,
	operationFn epochOperation,
) {
	preBeaconStateFile, err := testutil.BazelFileBytes(path.Join(testFolderPath, "pre.ssz_snappy"))
	require.NoError(t, err)
	preBeaconStateSSZ, err := snappy.Decode(nil /* dst */, preBeaconStateFile)
	require.NoError(t, err, "Failed to decompress")
	preBeaconStateBase := &pb.BeaconStateAltair{}
	if err := preBeaconStateBase.UnmarshalSSZ(preBeaconStateSSZ); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}
	preBeaconState, err := stateAltair.InitializeFromProto(preBeaconStateBase)
	require.NoError(t, err)

	// If the post.ssz is not present, it means the test should fail on our end.
	postSSZFilepath, err := bazel.Runfile(path.Join(testFolderPath, "post.ssz_snappy"))
	postSSZExists := true
	if err != nil && strings.Contains(err.Error(), "could not locate file") {
		postSSZExists = false
	} else if err != nil {
		t.Fatal(err)
	}

	beaconState, err := operationFn(t, preBeaconState)
	if postSSZExists {
		require.NoError(t, err)

		postBeaconStateFile, err := ioutil.ReadFile(postSSZFilepath)
		require.NoError(t, err)
		postBeaconStateSSZ, err := snappy.Decode(nil /* dst */, postBeaconStateFile)
		require.NoError(t, err, "Failed to decompress")
		postBeaconState := &pb.BeaconStateAltair{}
		if err := postBeaconState.UnmarshalSSZ(postBeaconStateSSZ); err != nil {
			t.Fatalf("Failed to unmarshal: %v", err)
		}

		pbState, err := stateAltair.ProtobufBeaconState(beaconState.InnerStateUnsafe())
		require.NoError(t, err)
		if !proto.Equal(pbState, postBeaconState) {
			diff, _ := messagediff.PrettyDiff(beaconState.InnerStateUnsafe(), postBeaconState)
			t.Log(diff)
			t.Fatal("Post state does not match expected")
		}
	} else {
		// Note: This doesn't test anything worthwhile. It essentially tests
		// that *any* error has occurred, not any specific error.
		if err == nil {
			t.Fatal("Did not fail when expected")
		}
		t.Logf("Expected failure; failure reason = %v", err)
		return
	}
}
