package endtoend

import (
	"testing"

	ev "github.com/prysmaticlabs/prysm/endtoend/evaluators"
	e2eParams "github.com/prysmaticlabs/prysm/endtoend/params"
	"github.com/prysmaticlabs/prysm/endtoend/types"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/testutil"
)

func TestEndToEnd_MinimalConfig(t *testing.T) {
	testutil.ResetCache()
	params.UseE2EConfig()

	minimalConfig := &types.E2EConfig{
		BeaconFlags:    []string{"--custom-genesis-delay=25"},
		ValidatorFlags: []string{},
		EpochsToRun:    8,
		TestSync:       true,
		TestSlasher:    true,
		Evaluators: []types.Evaluator{
			ev.PeersConnect,
			ev.HealthzCheck,
			ev.ValidatorsAreActive,
			ev.ValidatorsParticipating,
			ev.FinalizationOccurs,
			ev.MetricsCheck,
		},
	}
	if err := e2eParams.Init(2); err != nil {
		t.Fatal(err)
	}

	runEndToEndTest(t, minimalConfig)
}
