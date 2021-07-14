// Package components defines utilities to spin up actual
// beacon node and validator processes as needed by end to end tests.
package components

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/bazelbuild/rules_go/go/tools/bazel"
	"github.com/prysmaticlabs/prysm/cmd/beacon-chain/flags"
	"github.com/prysmaticlabs/prysm/endtoend/helpers"
	e2e "github.com/prysmaticlabs/prysm/endtoend/params"
	e2etypes "github.com/prysmaticlabs/prysm/endtoend/types"
	cmdshared "github.com/prysmaticlabs/prysm/shared/cmd"
	"github.com/prysmaticlabs/prysm/shared/featureconfig"
	"github.com/prysmaticlabs/prysm/shared/params"
)

var _ e2etypes.ComponentRunner = (*BeaconNode)(nil)
var _ e2etypes.ComponentRunner = (*BeaconNodeSet)(nil)

// BeaconNodeSet represents set of beacon nodes.
type BeaconNodeSet struct {
	e2etypes.ComponentRunner
	config  *e2etypes.E2EConfig
	enr     string
	started chan struct{}
}

// SetENR assigns ENR to the set of beacon nodes.
func (s *BeaconNodeSet) SetENR(enr string) {
	s.enr = enr
}

// NewBeaconNodes creates and returns a set of beacon nodes.
func NewBeaconNodes(config *e2etypes.E2EConfig) *BeaconNodeSet {
	return &BeaconNodeSet{
		config:  config,
		started: make(chan struct{}, 1),
	}
}

// Start starts all the beacon nodes in set.
func (s *BeaconNodeSet) Start(ctx context.Context) error {
	if s.enr == "" {
		return errors.New("empty ENR")
	}

	// Create beacon nodes.
	nodes := make([]e2etypes.ComponentRunner, e2e.TestParams.BeaconNodeCount)
	for i := 0; i < e2e.TestParams.BeaconNodeCount; i++ {
		nodes[i] = NewBeaconNode(s.config, i, s.enr, false)
	}

	// Wait for all nodes to finish their job (blocking).
	// Once nodes are ready passed in handler function will be called.
	return helpers.WaitOnNodes(ctx, nodes, func() {
		// All nodes stated, close channel, so that all services waiting on a set, can proceed.
		close(s.started)
	})
}

// Started checks whether beacon node set is started and all nodes are ready to be queried.
func (s *BeaconNodeSet) Started() <-chan struct{} {
	return s.started
}

// BeaconNode represents beacon node.
type BeaconNode struct {
	e2etypes.ComponentRunner
	config     *e2etypes.E2EConfig
	started    chan struct{}
	index      int
	enr        string
	isSyncNode bool
}

// NewBeaconNode creates and returns a beacon node.
func NewBeaconNode(config *e2etypes.E2EConfig, index int, enr string, isSyncNode bool) *BeaconNode {
	return &BeaconNode{
		config:     config,
		index:      index,
		enr:        enr,
		started:    make(chan struct{}, 1),
		isSyncNode: isSyncNode,
	}
}

// Start starts a fresh beacon node, connecting to all passed in beacon nodes.
func (node *BeaconNode) Start(ctx context.Context) error {
	binaryPath, found := bazel.FindBinary("cmd/beacon-chain", "beacon-chain")
	if !found {
		log.Info(binaryPath)
		return errors.New("beacon chain binary not found")
	}

	config, index, enr := node.config, node.index, node.enr
	stdOutFile, err := helpers.DeleteAndCreateFile(e2e.TestParams.LogPath, fmt.Sprintf(e2e.BeaconNodeLogFileName, index))
	if err != nil {
		return err
	}

	args := []string{
		fmt.Sprintf("--%s=%s/eth2-beacon-node-%d", cmdshared.DataDirFlag.Name, e2e.TestParams.TestPath, index),
		fmt.Sprintf("--%s=%s", cmdshared.LogFileName.Name, stdOutFile.Name()),
		fmt.Sprintf("--%s=%s", flags.DepositContractFlag.Name, e2e.TestParams.ContractAddress.Hex()),
		fmt.Sprintf("--%s=%d", flags.RPCPort.Name, e2e.TestParams.BeaconNodeRPCPort+index),
		fmt.Sprintf("--%s=http://127.0.0.1:%d", flags.HTTPWeb3ProviderFlag.Name, e2e.TestParams.Eth1RPCPort),
		fmt.Sprintf("--%s=%d", flags.MinSyncPeers.Name, e2e.TestParams.BeaconNodeCount-1),
		fmt.Sprintf("--%s=%d", cmdshared.P2PUDPPort.Name, e2e.TestParams.BeaconNodeRPCPort+index+10),
		fmt.Sprintf("--%s=%d", cmdshared.P2PTCPPort.Name, e2e.TestParams.BeaconNodeRPCPort+index+20),
		fmt.Sprintf("--%s=%d", flags.MonitoringPortFlag.Name, e2e.TestParams.BeaconNodeMetricsPort+index),
		fmt.Sprintf("--%s=%d", flags.GRPCGatewayPort.Name, e2e.TestParams.BeaconNodeRPCPort+index+40),
		fmt.Sprintf("--%s=%d", flags.EthApiPort.Name, e2e.TestParams.BeaconNodeRPCPort+index+30),
		fmt.Sprintf("--%s=%d", flags.ContractDeploymentBlock.Name, 0),
		fmt.Sprintf("--%s=%d", cmdshared.RPCMaxPageSizeFlag.Name, params.BeaconConfig().MinGenesisActiveValidatorCount),
		fmt.Sprintf("--%s=%s", cmdshared.BootstrapNode.Name, enr),
		fmt.Sprintf("--%s=%s", cmdshared.VerbosityFlag.Name, "debug"),
		"--" + cmdshared.ForceClearDB.Name,
		"--" + cmdshared.E2EConfigFlag.Name,
		"--" + cmdshared.AcceptTosFlag.Name,
	}
	if config.UsePprof {
		args = append(args, "--pprof", fmt.Sprintf("--pprofport=%d", e2e.TestParams.BeaconNodeRPCPort+index+50))
	}
	args = append(args, featureconfig.E2EBeaconChainFlags...)
	args = append(args, config.BeaconFlags...)

	if node.isSyncNode {
		args = append(args, config.SyncingBeaconFlags...)
	}

	cmd := exec.CommandContext(ctx, binaryPath, args...)
	// Write stdout and stderr to log files.
	stdout, err := os.Create(path.Join(e2e.TestParams.LogPath, fmt.Sprintf("beacon_node_%d_stdout.log", index)))
	if err != nil {
		return err
	}
	stderr, err := os.Create(path.Join(e2e.TestParams.LogPath, fmt.Sprintf("beacon_node_%d_stderr.log", index)))
	if err != nil {
		return err
	}
	defer func() {
		if err := stdout.Close(); err != nil {
			log.WithError(err).Error("Failed to close stdout file")
		}
		if err := stderr.Close(); err != nil {
			log.WithError(err).Error("Failed to close stderr file")
		}
	}()
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	log.Infof("Starting beacon chain %d with flags: %s", index, strings.Join(args[2:], " "))
	if err = cmd.Start(); err != nil {
		return fmt.Errorf("failed to start beacon node: %w", err)
	}

	if err = helpers.WaitForTextInFile(stdOutFile, "gRPC server listening on port"); err != nil {
		return fmt.Errorf("could not find multiaddr for node %d, this means the node had issues starting: %w", index, err)
	}

	// Mark node as ready.
	close(node.started)

	return cmd.Wait()
}

// Started checks whether beacon node is started and ready to be queried.
func (node *BeaconNode) Started() <-chan struct{} {
	return node.started
}
