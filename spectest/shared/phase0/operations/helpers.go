package operations

import (
	"context"
	"io/ioutil"
	"path"
	"strings"
	"testing"

	"github.com/bazelbuild/rules_go/go/tools/bazel"
	"github.com/golang/snappy"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	iface "github.com/prysmaticlabs/prysm/beacon-chain/state/interface"
	v1 "github.com/prysmaticlabs/prysm/beacon-chain/state/v1"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	ethpb "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/proto/eth/v1alpha1/wrapper"
	"github.com/prysmaticlabs/prysm/proto/interfaces"
	"github.com/prysmaticlabs/prysm/shared/testutil"
	"github.com/prysmaticlabs/prysm/shared/testutil/require"
	"google.golang.org/protobuf/proto"
	"gopkg.in/d4l3k/messagediff.v1"
)

type blockOperation func(context.Context, iface.BeaconState, interfaces.SignedBeaconBlock) (iface.BeaconState, error)

// RunBlockOperationTest takes in the prestate and the beacon block body, processes it through the
// passed in block operation function and checks the post state with the expected post state.
func RunBlockOperationTest(
	t *testing.T,
	folderPath string,
	body *ethpb.BeaconBlockBody,
	operationFn blockOperation,
) {
	preBeaconStateFile, err := testutil.BazelFileBytes(path.Join(folderPath, "pre.ssz_snappy"))
	require.NoError(t, err)
	preBeaconStateSSZ, err := snappy.Decode(nil /* dst */, preBeaconStateFile)
	require.NoError(t, err, "Failed to decompress")
	preStateBase := &pb.BeaconState{}
	if err := preStateBase.UnmarshalSSZ(preBeaconStateSSZ); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}
	preState, err := v1.InitializeFromProto(preStateBase)
	require.NoError(t, err)

	// If the post.ssz is not present, it means the test should fail on our end.
	postSSZFilepath, err := bazel.Runfile(path.Join(folderPath, "post.ssz_snappy"))
	postSSZExists := true
	if err != nil && strings.Contains(err.Error(), "could not locate file") {
		postSSZExists = false
	} else if err != nil {
		t.Fatal(err)
	}

	helpers.ClearCache()
	b := testutil.NewBeaconBlock()
	b.Block.Body = body
	beaconState, err := operationFn(context.Background(), preState, wrapper.WrappedPhase0SignedBeaconBlock(b))
	if postSSZExists {
		require.NoError(t, err)

		postBeaconStateFile, err := ioutil.ReadFile(postSSZFilepath)
		require.NoError(t, err)
		postBeaconStateSSZ, err := snappy.Decode(nil /* dst */, postBeaconStateFile)
		require.NoError(t, err, "Failed to decompress")

		postBeaconState := &pb.BeaconState{}
		if err := postBeaconState.UnmarshalSSZ(postBeaconStateSSZ); err != nil {
			t.Fatalf("Failed to unmarshal: %v", err)
		}
		pbState, err := v1.ProtobufBeaconState(beaconState.InnerStateUnsafe())
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
