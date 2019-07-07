package spectest

import (
	"io/ioutil"
	"testing"

	"github.com/bazelbuild/rules_go/go/tools/bazel"
	"github.com/ghodss/yaml"
	"github.com/gogo/protobuf/proto"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/blocks"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	"github.com/prysmaticlabs/prysm/shared/params/spectest"
	"gopkg.in/d4l3k/messagediff.v1"
)

func runTransferTest(t *testing.T, filename string) {
	file, err := ioutil.ReadFile(filename)
	if err != nil {
		t.Fatalf("Could not load file %v", err)
	}

	test := &BlockOperationTest{}
	if err := yaml.Unmarshal(file, test); err != nil {
		t.Fatalf("Failed to Unmarshal: %v", err)
	}

	if err := spectest.SetConfig(test.Config); err != nil {
		t.Fatal(err)
	}

	for _, tt := range test.TestCases {
		t.Run(tt.Description, func(t *testing.T) {
			helpers.ClearAllCaches()

			body := &pb.BeaconBlockBody{Transfers: []*pb.Transfer{tt.Transfer}}

			postState, err := blocks.ProcessTransfers(tt.Pre, body, true)
			// Note: This doesn't test anything worthwhile. It essentially tests
			// that *any* error has occurred, not any specific error.
			if len(tt.Post.Validators) == 0 {
				if err == nil {
					t.Fatal("Did not fail when expected")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}

			if !proto.Equal(postState, tt.Post) {
				diff, _ := messagediff.PrettyDiff(postState, tt.Post)
				t.Log(diff)
				t.Fatal("Post state does not match expected")
			}
		})
	}
}

var transferPrefix = "eth2_spec_tests/tests/operations/transfer/"

func TestTransferMinimal(t *testing.T) {
	t.Skip("Transfer tests are disabled. See https://github.com/ethereum/eth2.0-specs/pull/1238#issuecomment-507054595")
	filepath, err := bazel.Runfile(transferPrefix + "transfer_minimal.yaml")
	if err != nil {
		t.Fatal(err)
	}
	runTransferTest(t, filepath)
}

func TestTransferMainnet(t *testing.T) {
	t.Skip("Transfer tests are disabled. See https://github.com/ethereum/eth2.0-specs/pull/1238#issuecomment-507054595")
	filepath, err := bazel.Runfile(transferPrefix + "transfer_mainnet.yaml")
	if err != nil {
		t.Fatal(err)
	}
	runTransferTest(t, filepath)
}
