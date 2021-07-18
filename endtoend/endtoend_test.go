// Package endtoend performs full a end-to-end test for Prysm,
// including spinning up an ETH1 dev chain, sending deposits to the deposit
// contract, and making sure the beacon node and validators are running and
// performing properly for a few epochs.
package endtoend

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"strings"
	"sync"
	"testing"
	"time"

	types "github.com/prysmaticlabs/eth2-types"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/state"
	"github.com/prysmaticlabs/prysm/endtoend/components"
	ev "github.com/prysmaticlabs/prysm/endtoend/evaluators"
	"github.com/prysmaticlabs/prysm/endtoend/helpers"
	e2e "github.com/prysmaticlabs/prysm/endtoend/params"
	e2etypes "github.com/prysmaticlabs/prysm/endtoend/types"
	eth "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/testutil/assert"
	"github.com/prysmaticlabs/prysm/shared/testutil/require"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
)

const (
	// allNodesStartTimeout defines period after which nodes are considered
	// stalled (safety measure for nodes stuck at startup, shouldn't normally happen).
	allNodesStartTimeout = 5 * time.Minute
)

func init() {
	state.SkipSlotCache.Disable()
}

// testRunner abstracts E2E test configuration and running.
type testRunner struct {
	t      *testing.T
	config *e2etypes.E2EConfig
}

// newTestRunner creates E2E test runner.
func newTestRunner(t *testing.T, config *e2etypes.E2EConfig) *testRunner {
	return &testRunner{
		t:      t,
		config: config,
	}
}

// run executes configured E2E test.
func (r *testRunner) run() {
	t, config := r.t, r.config
	t.Logf("Shard index: %d\n", e2e.TestParams.TestShardIndex)
	t.Logf("Starting time: %s\n", time.Now().String())
	t.Logf("Log Path: %s\n", e2e.TestParams.LogPath)

	minGenesisActiveCount := int(params.BeaconConfig().MinGenesisActiveValidatorCount)

	ctx, done := context.WithCancel(context.Background())
	g, ctx := errgroup.WithContext(ctx)

	// ETH1 node.
	eth1Node := components.NewEth1Node()
	g.Go(func() error {
		return eth1Node.Start(ctx)
	})
	g.Go(func() error {
		if err := helpers.ComponentsStarted(ctx, []e2etypes.ComponentRunner{eth1Node}); err != nil {
			return fmt.Errorf("sending and mining deposits require ETH1 node to run: %w", err)
		}
		return components.SendAndMineDeposits(eth1Node.KeystorePath(), minGenesisActiveCount, 0, true /* partial */)
	})

	// Boot node.
	bootNode := components.NewBootNode()
	g.Go(func() error {
		return bootNode.Start(ctx)
	})

	// Beacon nodes.
	beaconNodes := components.NewBeaconNodes(config)
	g.Go(func() error {
		if err := helpers.ComponentsStarted(ctx, []e2etypes.ComponentRunner{eth1Node, bootNode}); err != nil {
			return fmt.Errorf("beacon nodes require ETH1 and boot node to run: %w", err)
		}
		beaconNodes.SetENR(bootNode.ENR())
		return beaconNodes.Start(ctx)
	})

	// Validator nodes.
	validatorNodes := components.NewValidatorNodeSet(config)
	g.Go(func() error {
		if err := helpers.ComponentsStarted(ctx, []e2etypes.ComponentRunner{beaconNodes}); err != nil {
			return fmt.Errorf("validator nodes require beacon nodes to run: %w", err)
		}
		return validatorNodes.Start(ctx)
	})

	// Slasher nodes.
	var slasherNodes e2etypes.ComponentRunner
	if config.TestSlasher {
		slasherNodes := components.NewSlasherNodeSet(config)
		g.Go(func() error {
			if err := helpers.ComponentsStarted(ctx, []e2etypes.ComponentRunner{beaconNodes}); err != nil {
				return fmt.Errorf("slasher nodes require beacon nodes to run: %w", err)
			}
			return slasherNodes.Start(ctx)
		})
	}

	// Run E2E evaluators and tests.
	g.Go(func() error {
		// When everything is done, cancel parent context (will stop all spawned nodes).
		defer func() {
			log.Info("All E2E evaluations are finished, cleaning up")
			done()
		}()

		// Wait for all required nodes to start.
		requiredComponents := []e2etypes.ComponentRunner{
			eth1Node, bootNode, beaconNodes, validatorNodes,
		}
		if config.TestSlasher && slasherNodes != nil {
			requiredComponents = append(requiredComponents, slasherNodes)
		}
		ctxAllNodesReady, cancel := context.WithTimeout(ctx, allNodesStartTimeout)
		defer cancel()
		if err := helpers.ComponentsStarted(ctxAllNodesReady, requiredComponents); err != nil {
			return fmt.Errorf("components take too long to start: %w", err)
		}

		// Since defer unwraps in LIFO order, parent context will be closed only after logs are written.
		defer helpers.LogOutput(t, config)
		if config.UsePprof {
			defer func() {
				log.Info("Writing output pprof files")
				for i := 0; i < e2e.TestParams.BeaconNodeCount; i++ {
					assert.NoError(t, helpers.WritePprofFiles(e2e.TestParams.LogPath, i))
				}
			}()
		}

		// Blocking, wait period varies depending on number of validators.
		r.waitForChainStart()

		// Failing early in case chain doesn't start.
		if t.Failed() {
			return errors.New("chain cannot start")
		}

		if config.TestDeposits {
			log.Info("Running deposit tests")
			r.testDeposits(ctx, g, eth1Node, []e2etypes.ComponentRunner{beaconNodes})
		}

		// Create GRPC connection to beacon nodes.
		conns, closeConns, err := helpers.NewLocalConnections(ctx, e2e.TestParams.BeaconNodeCount)
		require.NoError(t, err, "Cannot create local connections")
		defer closeConns()

		// Calculate genesis time.
		nodeClient := eth.NewNodeClient(conns[0])
		genesis, err := nodeClient.GetGenesis(context.Background(), &emptypb.Empty{})
		require.NoError(t, err)
		tickingStartTime := helpers.EpochTickerStartTime(genesis)

		// Run assigned evaluators.
		if err := r.runEvaluators(conns, tickingStartTime); err != nil {
			return err
		}

		// If requested, run sync test.
		if !config.TestSync {
			return nil
		}
		return r.testBeaconChainSync(ctx, g, conns, tickingStartTime, bootNode.ENR())
	})

	if err := g.Wait(); err != nil && !errors.Is(err, context.Canceled) {
		// At the end of the main evaluator goroutine all nodes are killed, no need to fail the test.
		if strings.Contains(err.Error(), "signal: killed") {
			return
		}
		t.Fatalf("E2E test ended in error: %v", err)
	}
}

// waitForChainStart allows to wait up until beacon nodes are started.
func (r *testRunner) waitForChainStart() {
	// Sleep depending on the count of validators, as generating the genesis state could take some time.
	time.Sleep(time.Duration(params.BeaconConfig().GenesisDelay) * time.Second)
	beaconLogFile, err := os.Open(path.Join(e2e.TestParams.LogPath, fmt.Sprintf(e2e.BeaconNodeLogFileName, 0)))
	require.NoError(r.t, err)

	r.t.Run("chain started", func(t *testing.T) {
		require.NoError(t, helpers.WaitForTextInFile(beaconLogFile, "Chain started in sync service"), "Chain did not start")
	})
}

// runEvaluators executes assigned evaluators.
func (r *testRunner) runEvaluators(conns []*grpc.ClientConn, tickingStartTime time.Time) error {
	t, config := r.t, r.config
	secondsPerEpoch := uint64(params.BeaconConfig().SlotsPerEpoch.Mul(params.BeaconConfig().SecondsPerSlot))
	ticker := helpers.NewEpochTicker(tickingStartTime, secondsPerEpoch)
	for currentEpoch := range ticker.C() {
		wg := new(sync.WaitGroup)
		for _, ev := range config.Evaluators {
			// Fix reference to evaluator as it will be running
			// in a separate goroutine.
			evaluator := ev
			// Only run if the policy says so.
			if !evaluator.Policy(types.Epoch(currentEpoch)) {
				continue
			}

			// Add evaluator to our waitgroup.
			wg.Add(1)

			go t.Run(fmt.Sprintf(evaluator.Name, currentEpoch), func(t *testing.T) {
				err := evaluator.Evaluation(conns...)
				assert.NoError(t, err, "Evaluation failed for epoch %d: %v", currentEpoch, err)
				wg.Done()
			})
		}
		// Wait for all evaluators to finish their evaluation for the epoch.
		wg.Wait()
		if t.Failed() || currentEpoch >= config.EpochsToRun-1 {
			ticker.Done()
			if t.Failed() {
				return errors.New("test failed")
			}
			break
		}
	}
	return nil
}

// testDeposits runs tests when config.TestDeposits is enabled.
func (r *testRunner) testDeposits(ctx context.Context, g *errgroup.Group,
	eth1Node *components.Eth1Node, requiredNodes []e2etypes.ComponentRunner) {
	minGenesisActiveCount := int(params.BeaconConfig().MinGenesisActiveValidatorCount)

	depositCheckValidator := components.NewValidatorNode(r.config, int(e2e.DepositCount), e2e.TestParams.BeaconNodeCount, minGenesisActiveCount)
	g.Go(func() error {
		if err := helpers.ComponentsStarted(ctx, requiredNodes); err != nil {
			return fmt.Errorf("deposit check validator node requires beacon nodes to run: %w", err)
		}
		go func() {
			err := components.SendAndMineDeposits(eth1Node.KeystorePath(), int(e2e.DepositCount), minGenesisActiveCount, false /* partial */)
			if err != nil {
				r.t.Fatal(err)
			}
		}()
		return depositCheckValidator.Start(ctx)
	})
}

// testBeaconChainSync creates another beacon node, and tests whether it can sync to head using previous nodes.
func (r *testRunner) testBeaconChainSync(ctx context.Context, g *errgroup.Group,
	conns []*grpc.ClientConn, tickingStartTime time.Time, enr string) error {
	t, config := r.t, r.config
	index := e2e.TestParams.BeaconNodeCount
	syncBeaconNode := components.NewBeaconNode(config, index, enr)
	g.Go(func() error {
		return syncBeaconNode.Start(ctx)
	})
	if err := helpers.ComponentsStarted(ctx, []e2etypes.ComponentRunner{syncBeaconNode}); err != nil {
		return fmt.Errorf("sync beacon node not ready: %w", err)
	}
	syncConn, err := grpc.Dial(fmt.Sprintf("127.0.0.1:%d", e2e.TestParams.BeaconNodeRPCPort+index), grpc.WithInsecure())
	require.NoError(t, err, "Failed to dial")
	conns = append(conns, syncConn)

	// Sleep a second for every 4 blocks that need to be synced for the newly started node.
	secondsPerEpoch := uint64(params.BeaconConfig().SlotsPerEpoch.Mul(params.BeaconConfig().SecondsPerSlot))
	extraSecondsToSync := (config.EpochsToRun)*secondsPerEpoch + uint64(params.BeaconConfig().SlotsPerEpoch.Div(4).Mul(config.EpochsToRun))
	waitForSync := tickingStartTime.Add(time.Duration(extraSecondsToSync) * time.Second)
	time.Sleep(time.Until(waitForSync))

	syncLogFile, err := os.Open(path.Join(e2e.TestParams.LogPath, fmt.Sprintf(e2e.BeaconNodeLogFileName, index)))
	require.NoError(t, err)
	defer helpers.LogErrorOutput(t, syncLogFile, "beacon chain node", index)
	t.Run("sync completed", func(t *testing.T) {
		assert.NoError(t, helpers.WaitForTextInFile(syncLogFile, "Synced up to"), "Failed to sync")
	})
	if t.Failed() {
		return errors.New("cannot sync beacon node")
	}

	// Sleep a slot to make sure the synced state is made.
	time.Sleep(time.Duration(params.BeaconConfig().SecondsPerSlot) * time.Second)
	syncEvaluators := []e2etypes.Evaluator{ev.FinishedSyncing, ev.AllNodesHaveSameHead}
	for _, evaluator := range syncEvaluators {
		t.Run(evaluator.Name, func(t *testing.T) {
			assert.NoError(t, evaluator.Evaluation(conns...), "Evaluation failed for sync node")
		})
	}

	return nil
}
