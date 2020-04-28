package endtoend

import (
	"os"
	"strconv"
	"testing"

	ev "github.com/prysmaticlabs/prysm/endtoend/evaluators"
	e2eParams "github.com/prysmaticlabs/prysm/endtoend/params"
	"github.com/prysmaticlabs/prysm/endtoend/types"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/testutil"
)

func TestEndToEnd_MinimalConfig(t *testing.T) {
	testutil.ResetCache()
	params.UseMinimalConfig()

	epochsToRun := 6
	var err error
	if epochs, ok := os.LookupEnv("E2E_EPOCHS"); ok {
		epochsToRun, err = strconv.Atoi(epochs)
		if err != nil {
			t.Fatal(err)
		}
	}

	minimalConfig := &types.E2EConfig{
		BeaconFlags:    []string{"--minimal-config", "--custom-genesis-delay=10"},
		ValidatorFlags: []string{"--minimal-config"},
		EpochsToRun:    uint64(epochsToRun),
		TestSync:       true,
		TestSlasher:    true,
		Evaluators: []types.Evaluator{
			ev.PeersConnect,
			ev.HealthzCheck,
			ev.ValidatorsAreActive,
			ev.ValidatorsParticipating,
			ev.FinalizationOccurs,
		},
	}
	if err := e2eParams.Init(4); err != nil {
		t.Fatal(err)
	}

	runEndToEndTest(t, minimalConfig)
}
