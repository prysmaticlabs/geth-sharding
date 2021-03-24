// Package components defines utilities to spin up actual
// beacon node and validator processes as needed by end to end tests.
package components

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"os/exec"
	"strings"
	"testing"

	"github.com/bazelbuild/rules_go/go/tools/bazel"
	"github.com/prysmaticlabs/prysm/endtoend/helpers"
	e2e "github.com/prysmaticlabs/prysm/endtoend/params"
	e2etypes "github.com/prysmaticlabs/prysm/endtoend/types"
)

var _ e2etypes.ComponentRunner = (*BootNode)(nil)

// BootNode represents boot node.
type BootNode struct {
	e2etypes.ComponentRunner
	started chan struct{}
	enr     string
}

// NewBootNode creates and returns boot node.
func NewBootNode(_ *e2etypes.E2EConfig) *BootNode {
	return &BootNode{
		started: make(chan struct{}, 1),
	}
}

// ENR exposes node's ENR.
func (node *BootNode) ENR() string {
	return node.enr
}

// StartBootnode starts a bootnode and returns its ENR.
func (node *BootNode) Start(ctx context.Context) error {
	binaryPath, found := bazel.FindBinary("tools/bootnode", "bootnode")
	if !found {
		log.Info(binaryPath)
		return errors.New("boot node binary not found")
	}

	stdOutFile, err := helpers.DeleteAndCreateFile(e2e.TestParams.LogPath, e2e.BootNodeLogFileName)
	if err != nil {
		return err
	}

	args := []string{
		fmt.Sprintf("--log-file=%s", stdOutFile.Name()),
		fmt.Sprintf("--discv5-port=%d", e2e.TestParams.BootNodePort),
		fmt.Sprintf("--metrics-port=%d", e2e.TestParams.BootNodePort+20),
		"--debug",
	}

	cmd := exec.CommandContext(ctx, binaryPath, args...)
	cmd.Stdout = stdOutFile
	cmd.Stderr = stdOutFile
	log.Infof("Starting boot node with flags: %s", strings.Join(args[1:], " "))
	if err = cmd.Start(); err != nil {
		return fmt.Errorf("failed to start beacon node: %w", err)
	}

	if err = helpers.WaitForTextInFile(stdOutFile, "Running bootnode"); err != nil {
		return fmt.Errorf("could not find enr for bootnode, this means the bootnode had issues starting: %w", err)
	}

	node.enr, err = enrFromLogFile(stdOutFile.Name())
	if err != nil {
		return fmt.Errorf("could not get enr for bootnode: %w", err)
	}

	// Mark node as ready.
	close(node.started)

	return cmd.Wait()
}

// Started checks whether boot node is started and ready to be queried.
func (node *BootNode) Started() <-chan struct{} {
	return node.started
}

// StartBootnode starts a bootnode and returns its ENR.
func StartBootnode(t *testing.T) string {
	binaryPath, found := bazel.FindBinary("tools/bootnode", "bootnode")
	if !found {
		t.Log(binaryPath)
		t.Fatal("boot node binary not found")
	}

	stdOutFile, err := helpers.DeleteAndCreateFile(e2e.TestParams.LogPath, e2e.BootNodeLogFileName)
	if err != nil {
		t.Fatal(err)
	}

	args := []string{
		fmt.Sprintf("--log-file=%s", stdOutFile.Name()),
		fmt.Sprintf("--discv5-port=%d", e2e.TestParams.BootNodePort),
		fmt.Sprintf("--metrics-port=%d", e2e.TestParams.BootNodePort+20),
		"--debug",
	}

	cmd := exec.Command(binaryPath, args...)
	cmd.Stdout = stdOutFile
	cmd.Stderr = stdOutFile
	t.Logf("Starting boot node with flags: %s", strings.Join(args[1:], " "))
	if err = cmd.Start(); err != nil {
		t.Fatalf("Failed to start beacon node: %v", err)
	}

	if err = helpers.WaitForTextInFile(stdOutFile, "Running bootnode"); err != nil {
		t.Fatalf("could not find enr for bootnode, this means the bootnode had issues starting: %v", err)
	}

	enr, err := enrFromLogFile(stdOutFile.Name())
	if err != nil {
		t.Fatalf("could not get enr for bootnode: %v", err)
	}

	return enr
}

func enrFromLogFile(name string) (string, error) {
	byteContent, err := ioutil.ReadFile(name)
	if err != nil {
		return "", err
	}
	contents := string(byteContent)

	searchText := "Running bootnode: "
	startIdx := strings.Index(contents, searchText)
	if startIdx == -1 {
		return "", fmt.Errorf("did not find ENR text in %s", contents)
	}
	startIdx += len(searchText)
	endIdx := strings.Index(contents[startIdx:], " prefix=bootnode")
	if endIdx == -1 {
		return "", fmt.Errorf("did not find ENR text in %s", contents)
	}
	return contents[startIdx : startIdx+endIdx-1], nil
}
