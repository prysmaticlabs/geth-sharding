package spectest

import (
	"bytes"
	"context"
	"fmt"
	"path"
	"testing"

	"github.com/prysmaticlabs/prysm/beacon-chain/core/epoch"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/epoch/precompute"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
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
			testutil.RunEpochOperationTest(t, folderPath, processJustificationAndFinalizationWrapper)
			testutil.RunEpochOperationTest(t, folderPath, processJustificationAndFinalizationPrecomputeWrapper)
		})
	}
}

// This is a subset of state.ProcessEpoch. The spec test defines input data for
// `justification_and_finalization` only.
func processJustificationAndFinalizationWrapper(t *testing.T, state *pb.BeaconState) (*pb.BeaconState, error) {
	prevEpochAtts, err := targetAtts(state, helpers.PrevEpoch(state))
	if err != nil {
		t.Fatalf("could not get target atts prev epoch %d: %v", helpers.PrevEpoch(state), err)
	}
	currentEpochAtts, err := targetAtts(state, helpers.CurrentEpoch(state))
	if err != nil {
		t.Fatalf("could not get target atts current epoch %d: %v", helpers.CurrentEpoch(state), err)
	}
	prevEpochAttestedBalance, err := epoch.AttestingBalance(state, prevEpochAtts)
	if err != nil {
		t.Fatalf("could not get attesting balance prev epoch: %v", err)
	}
	currentEpochAttestedBalance, err := epoch.AttestingBalance(state, currentEpochAtts)
	if err != nil {
		t.Fatalf("could not get attesting balance current epoch: %v", err)
	}

	state, err = epoch.ProcessJustificationAndFinalization(state, prevEpochAttestedBalance, currentEpochAttestedBalance)
	if err != nil {
		t.Fatalf("could not process justification: %v", err)
	}

	return state, nil
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

func targetAtts(state *pb.BeaconState, epoch uint64) ([]*pb.PendingAttestation, error) {
	currentEpoch := helpers.CurrentEpoch(state)
	previousEpoch := helpers.PrevEpoch(state)

	// Input epoch for matching the source attestations has to be within range
	// of current epoch & previous epoch.
	if epoch != currentEpoch && epoch != previousEpoch {
		return nil, fmt.Errorf("input epoch: %d != current epoch: %d or previous epoch: %d",
			epoch, currentEpoch, previousEpoch)
	}

	// Decide if the source attestations are coming from current or previous epoch.
	var srcAtts []*pb.PendingAttestation
	if epoch == currentEpoch {
		srcAtts = state.CurrentEpochAttestations
	} else {
		srcAtts = state.PreviousEpochAttestations
	}
	targetRoot, err := helpers.BlockRoot(state, epoch)
	if err != nil {
		return nil, err
	}

	tgtAtts := make([]*pb.PendingAttestation, 0, len(srcAtts))
	for _, srcAtt := range srcAtts {
		if bytes.Equal(srcAtt.Data.Target.Root, targetRoot) {
			tgtAtts = append(tgtAtts, srcAtt)
		}
	}

	return tgtAtts, nil
}
