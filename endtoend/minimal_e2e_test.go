package endtoend

import (
	"fmt"
	"os"
	"strconv"
	"testing"

	ev "github.com/prysmaticlabs/prysm/endtoend/evaluators"
	e2eParams "github.com/prysmaticlabs/prysm/endtoend/params"
	"github.com/prysmaticlabs/prysm/endtoend/types"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/testutil/require"
)

func TestEndToEnd_MinimalConfig(t *testing.T) {
	e2eMinimal(t, false /*usePrysmSh*/)
}

// Run minimal e2e config with the current release validator against latest beacon node.
func TestEndToEnd_MinimalConfig_ValidatorAtCurrentRelease(t *testing.T) {
	t.Skip("Skipping for Altair hard fork as there are breaking changes")
	e2eMinimal(t, true /*usePrysmSh*/)
}

func e2eMinimal(t *testing.T, usePrysmSh bool) {
	params.UseE2EConfig()
	require.NoError(t, e2eParams.Init(e2eParams.StandardBeaconCount))

	// Run for 10 epochs if not in long-running to confirm long-running has no issues.
	epochsToRun := 10
	var err error
	epochStr, longRunning := os.LookupEnv("E2E_EPOCHS")
	if longRunning {
		epochsToRun, err = strconv.Atoi(epochStr)
		require.NoError(t, err)
	}

	testConfig := &types.E2EConfig{
		BeaconFlags: []string{
			fmt.Sprintf("--slots-per-archive-point=%d", params.BeaconConfig().SlotsPerEpoch*16),
		},
		ValidatorFlags:      []string{},
		EpochsToRun:         uint64(epochsToRun),
		TestSync:            true,
		TestDeposits:        true,
		TestSlasher:         false,
		UsePrysmShValidator: usePrysmSh,
		UsePprof:            !longRunning,
		Evaluators: []types.Evaluator{
			ev.PeersConnect,
			ev.HealthzCheck,
			ev.MetricsCheck,
			ev.ValidatorsAreActive,
			ev.ValidatorsParticipating,
			ev.FinalizationOccurs,
			ev.ProcessesDepositsInBlocks,
			ev.VerifyBlockGraffiti,
			ev.ActivatesDepositedValidators,
			ev.DepositedValidatorsAreActive,
			ev.ProposeVoluntaryExit,
			ev.ValidatorHasExited,
			ev.ValidatorsVoteWithTheMajority,
			ev.ColdStateCheckpoint,
			ev.ApiVerifyValidators,
			ev.ForkTransition,
		},
	}

	newTestRunner(t, testConfig).run()
}
