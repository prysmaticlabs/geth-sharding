package evaluators

import (
	"context"
	"fmt"

	ptypes "github.com/gogo/protobuf/types"
	"github.com/pkg/errors"
	eth "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/shared/params"
)

// FinalizationOccurs is an evaluator to make sure finalization is performing as it should.
// Requires to be run after at least 4 epochs have passed.
var FinalizationOccurs = Evaluator{
	Name:       "finalizes_at_epoch_%d",
	Policy:     afterNthEpoch(3),
	Evaluation: finalizationOccurs,
}

func finalizationOccurs(client eth.BeaconChainClient) error {
	chainHead, err := client.GetChainHead(context.Background(), &ptypes.Empty{})
	if err != nil {
		return errors.Wrap(err, "failed to get chain head")
	}
	currentEpoch := chainHead.BlockSlot / params.BeaconConfig().SlotsPerEpoch
	finalizedEpoch := chainHead.FinalizedSlot / params.BeaconConfig().SlotsPerEpoch

	expectedFinalizedEpoch := currentEpoch - 2
	if expectedFinalizedEpoch != finalizedEpoch {
		return fmt.Errorf(
			"expected finalized epoch to be %d, received: %d",
			expectedFinalizedEpoch,
			finalizedEpoch,
		)
	}
	previousJustifiedEpoch := chainHead.PreviousJustifiedSlot / params.BeaconConfig().SlotsPerEpoch
	currentJustifiedEpoch := chainHead.JustifiedSlot / params.BeaconConfig().SlotsPerEpoch
	if previousJustifiedEpoch+1 != currentJustifiedEpoch {
		return fmt.Errorf(
			"there should be no gaps between current and previous justified epochs, received current %d and previous %d",
			currentJustifiedEpoch,
			previousJustifiedEpoch,
		)
	}
	if currentJustifiedEpoch+1 != currentEpoch {
		return fmt.Errorf(
			"there should be no gaps between current epoch and current justified epoch, received current %d and justified %d",
			currentEpoch,
			currentJustifiedEpoch,
		)
	}
	return nil
}
