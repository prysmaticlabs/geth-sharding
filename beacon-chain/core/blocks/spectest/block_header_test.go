package spectest

import (
	"io/ioutil"
	"testing"

	"github.com/bazelbuild/rules_go/go/tools/bazel"
	"github.com/ghodss/yaml"
	"github.com/gogo/protobuf/proto"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/blocks"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	"github.com/prysmaticlabs/prysm/shared/params/spectest"
	"gopkg.in/d4l3k/messagediff.v1"
)

// Block header test is actually a full block processing test. Not sure why it
// was named "block_header". The note in the test format readme says "Note that
// block_header is not strictly an operation (and is a full Block), but
// processed in the same manner, and hence included here."
func runBlockHeaderTest(t *testing.T, filename string) {
	file, err := ioutil.ReadFile(filename)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	test := &BlockOperationTest{}
	if err := yaml.Unmarshal(file, test); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if err := spectest.SetConfig(test.Config); err != nil {
		t.Fatal(err)
	}

	for _, tt := range test.TestCases {
		t.Run(tt.Description, func(t *testing.T) {
			helpers.ClearAllCaches()

			post, err := blocks.ProcessBlockHeader(tt.Pre, tt.Block, true)

			if tt.Post == nil {
				// Note: This doesn't test anything worthwhile. It essentially tests
				// that *any* error has occurred, not any specific error.
				if err == nil {
					t.Fatal("did not fail when expected")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}

			if !proto.Equal(post, tt.Post) {
				diff, _ := messagediff.PrettyDiff(post, tt.Post)
				t.Log(diff)
				t.Fatal("Post state does not match expected")
			}
		})
	}
}

var blkHeaderPrefix = "tests/operations/block_header/"

func TestBlockHeaderMinimal(t *testing.T) {
	filepath, err := bazel.Runfile(blkHeaderPrefix + "block_header_minimal.yaml")
	if err != nil {
		t.Fatal(err)
	}
	runBlockHeaderTest(t, filepath)
}

func TestBlockHeaderMainnet(t *testing.T) {
	filepath, err := bazel.Runfile(blkHeaderPrefix + "block_header_mainnet.yaml")
	if err != nil {
		t.Fatal(err)
	}
	runBlockHeaderTest(t, filepath)
}
