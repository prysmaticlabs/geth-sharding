package spectest

import (
	"context"
	"path"
	"testing"

	"github.com/prysmaticlabs/prysm/beacon-chain/core/epoch/precompute"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	"github.com/prysmaticlabs/prysm/shared/params/spectest"
	"github.com/prysmaticlabs/prysm/shared/testutil"
)

func runJustificationAndFinalizationTests(t *testing.T, config string) {
	if err := spectest.SetConfig(config); err != nil {
		t.Fatal(err)
	}

	testPath := "epoch_processing/justification_and_finalization/pyspec_tests"
	testFolders, testsFolderPath := testutil.TestFolders(t, config, testPath)
	for _, folder := range testFolders {
		t.Run(folder.Name(), func(t *testing.T) {
			folderPath := path.Join(testsFolderPath, folder.Name())
			testutil.RunEpochOperationTest(t, folderPath, processJustificationAndFinalizationPrecomputeWrapper)
		})
	}
}

func processJustificationAndFinalizationPrecomputeWrapper(t *testing.T, state *pb.BeaconState) (*pb.BeaconState, error) {
	ctx := context.Background()
	vp, bp := precompute.New(ctx, state)
	_, bp, err := precompute.ProcessAttestations(ctx, state, vp, bp)
	if err != nil {
		t.Fatal(err)
	}

	state, err = precompute.ProcessJustificationAndFinalizationPreCompute(state, bp)
	if err != nil {
		t.Fatalf("could not process justification: %v", err)
	}

	return state, nil
}
