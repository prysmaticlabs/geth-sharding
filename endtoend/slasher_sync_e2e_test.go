package endtoend

import (
	"testing"

	ev "github.com/prysmaticlabs/prysm/endtoend/evaluators"
	e2eParams "github.com/prysmaticlabs/prysm/endtoend/params"
	"github.com/prysmaticlabs/prysm/endtoend/types"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/testutil/require"
)

func TestEndToEnd_Slasher_Sync_MinimalConfig(t *testing.T) {
	params.UseE2EConfig()

	// We start some beacon nodes to test with and attach validators to.
	// Then, we test chain sync by running a single beacon node which
	// will have the --slasher flag set to true.
	require.NoError(t, e2eParams.Init(2))

	testConfig := &types.E2EConfig{
		// Only the syncing beacon node should have the --slasher flag enabled.
		SyncingBeaconFlags: []string{
			"--slasher",
		},
		ValidatorFlags:      []string{},
		EpochsToRun:         4,    // Normally run a node for 4 epochs.
		EpochsToRunPostSync: 3,    // Sync a node, then run that node for 3 more epochs.
		TestSync:            true, // We want to test a second beacon node with --slasher syncing the chain.
		TestDeposits:        false,
		Evaluators: []types.Evaluator{
			ev.HealthzCheck,
			ev.ValidatorsAreActive,
			ev.ValidatorsParticipatingAtEpoch(2),
			ev.FinalizationOccurs(3),
			//ev.InjectDoubleVoteOnEpoch(3),
			ev.InjectDoubleBlockOnEpoch(3),
		},
		PostSyncEvaluators: []types.Evaluator{
			ev.FinishedSyncing,
			ev.AllNodesHaveSameHead,
			ev.ValidatorsParticipatingAtEpoch(6),         // Validators should still be participating.
			ev.ValidatorsSlashedAfterEpoch(5),            // We expect validators are slashed.
			ev.SlashedValidatorsLoseBalanceAfterEpoch(5), // We expect slashed validator will lose balance.
		},
	}

	newTestRunner(t, testConfig).run()
}
