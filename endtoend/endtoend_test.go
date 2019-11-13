package endtoend

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"math/big"
	"net/http"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/bazelbuild/rules_go/go/tools/bazel"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	ptypes "github.com/gogo/protobuf/types"
	"github.com/pborman/uuid"
	contracts "github.com/prysmaticlabs/prysm/contracts/deposit-contract"
	ev "github.com/prysmaticlabs/prysm/endtoend/evaluators"
	"github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/testutil"
	"google.golang.org/grpc"
)

type end2EndConfig struct {
	minimalConfig  bool
	tmpPath        string
	epochsToRun    uint64
	numValidators  uint64
	numBeaconNodes uint64
	contractAddr   common.Address
	evaluators     []ev.Evaluator
}

type beaconNodeInfo struct {
	processID   int
	datadir     string
	rpcPort     uint64
	monitorPort uint64
	grpcPort    uint64
	multiAddr   string
}

type validatorClientInfo struct {
	processID   int
	monitorPort uint64
}

func runEndToEndTest(t *testing.T, config *end2EndConfig) {
	tmpPath := path.Join("/tmp/e2e/", uuid.NewRandom().String()[:18])
	if err := os.MkdirAll(tmpPath, os.ModePerm); err != nil {
		t.Fatal(err)
	}
	config.tmpPath = tmpPath
	t.Logf("Test Path: %s\n", tmpPath)

	contractAddr, keystorePath, eth1PID := startEth1(t, tmpPath)
	config.contractAddr = contractAddr
	beaconNodes := startBeaconNodes(t, config)
	valClients := initializeValidators(t, config, keystorePath, beaconNodes)
	processIDs := []int{eth1PID}
	for _, vv := range valClients {
		processIDs = append(processIDs, vv.processID)
	}
	for _, bb := range beaconNodes {
		processIDs = append(processIDs, bb.processID)
	}
	defer logOutput(t, tmpPath)
	defer killProcesses(t, processIDs)

	if config.numBeaconNodes > 1 {
		t.Run("all_peers_connect", func(t *testing.T) {
			for _, bNode := range beaconNodes {
				if err := peersConnect(bNode.monitorPort, config.numBeaconNodes-1); err != nil {
					t.Fatalf("failed to connect to peers: %v", err)
				}
			}
		})
	}

	beaconLogFile, err := os.Open(path.Join(tmpPath, "beacon-0.log"))
	if err != nil {
		t.Fatal(err)
	}
	if err := waitForTextInFile(beaconLogFile, "Sending genesis time notification"); err != nil {
		t.Fatal(err)
	}
	conn, err := grpc.Dial("127.0.0.1:4000", grpc.WithInsecure())
	if err != nil {
		t.Fatalf("fail to dial: %v", err)
	}
	beaconClient := eth.NewBeaconChainClient(conn)
	nodeClient := eth.NewNodeClient(conn)

	genesis, err := nodeClient.GetGenesis(context.Background(), &ptypes.Empty{})
	if err != nil {
		t.Fatal(err)
	}
	// Small offset so evaluators perform in the middle of an epoch.
	epochSeconds := params.BeaconConfig().SecondsPerSlot * params.BeaconConfig().SlotsPerEpoch
	genesisTime := time.Unix(genesis.GenesisTime.Seconds+int64(epochSeconds/2), 0)
	currentEpoch := uint64(0)
	ticker := GetEpochTicker(genesisTime, epochSeconds)
	for c := range ticker.C() {
		if c < config.epochsToRun {
			for _, evaluator := range config.evaluators {
				// Only run if the policy says so.
				if !evaluator.Policy(currentEpoch) {
					continue
				}
				t.Run(fmt.Sprintf(evaluator.Name, currentEpoch), func(t *testing.T) {
					if err := evaluator.Evaluation(beaconClient); err != nil {
						t.Fatal(err)
					}
				})
			}
			currentEpoch++
		} else {
			ticker.Done()
			break
		}
	}

	if currentEpoch < config.epochsToRun {
		t.Fatalf("test ended prematurely, only reached epoch %d", currentEpoch)
	}
}

// startEth1 starts an eth1 local dev chain and deploys a deposit contract.
func startEth1(t *testing.T, tmpPath string) (common.Address, string, int) {
	binaryPath, found := bazel.FindBinary("cmd/geth", "geth")
	if !found {
		t.Fatal("go-ethereum binary not found")
	}

	args := []string{
		fmt.Sprintf("--datadir=%s", path.Join(tmpPath, "eth1data/")),
		"--rpc",
		"--rpcaddr=0.0.0.0",
		"--rpccorsdomain=\"*\"",
		"--rpcvhosts=\"*\"",
		"--ws",
		"--wsaddr=0.0.0.0",
		"--wsorigins=\"*\"",
		"--dev",
		"--dev.period=0",
	}
	cmd := exec.Command(binaryPath, args...)
	file, err := os.Create(path.Join(tmpPath, "eth1.log"))
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stdout = file
	cmd.Stderr = file
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start eth1 chain: %v", err)
	}

	if err = waitForTextInFile(file, "IPC endpoint opened"); err != nil {
		t.Fatal(err)
	}

	// Connect to the started geth dev chain.
	client, err := rpc.Dial(path.Join(tmpPath, "eth1data/geth.ipc"))
	if err != nil {
		t.Fatalf("failed to connect to ipc: %v", err)
	}
	web3 := ethclient.NewClient(client)

	// Access the dev account keystore to deploy the contract.
	fileName, err := exec.Command("ls", path.Join(tmpPath, "eth1data/keystore")).Output()
	if err != nil {
		t.Fatal(err)
	}
	keystorePath := path.Join(tmpPath, fmt.Sprintf("eth1data/keystore/%s", strings.TrimSpace(string(fileName))))
	jsonBytes, err := ioutil.ReadFile(keystorePath)
	if err != nil {
		t.Fatal(err)
	}
	keystore, err := keystore.DecryptKey(jsonBytes, "" /*password*/)
	if err != nil {
		t.Fatal(err)
	}
	// Advancing the blocks eth1follow distance to prevent issues reading the chain.
	if err := mineBlocks(web3, keystore, params.BeaconConfig().Eth1FollowDistance); err != nil {
		t.Fatalf("unable to advance chain: %v", err)
	}

	txOpts, err := bind.NewTransactor(bytes.NewReader(jsonBytes), "" /*password*/)
	if err != nil {
		t.Fatal(err)
	}
	nonce, err := web3.PendingNonceAt(context.Background(), keystore.Address)
	if err != nil {
		t.Fatal(err)
	}
	txOpts.Nonce = big.NewInt(int64(nonce))
	contractAddr, tx, _, err := contracts.DeployDepositContract(txOpts, web3, txOpts.From)
	if err != nil {
		t.Fatalf("failed to deploy deposit contract: %v", err)
	}

	// Wait for contract to mine.
	for pending := true; pending; _, pending, err = web3.TransactionByHash(context.Background(), tx.Hash()) {
		if err != nil {
			t.Fatal(err)
		}
		time.Sleep(100 * time.Millisecond)
	}

	return contractAddr, keystorePath, cmd.Process.Pid
}

// startBeaconNodes starts the requested amount of beacon nodes, passing in the deposit contract given.
func startBeaconNodes(t *testing.T, config *end2EndConfig) []*beaconNodeInfo {
	numNodes := config.numBeaconNodes

	nodeInfo := []*beaconNodeInfo{}
	for i := uint64(0); i < numNodes; i++ {
		newNode := startNewBeaconNode(t, config, nodeInfo)
		nodeInfo = append(nodeInfo, newNode)
	}

	return nodeInfo
}

func startNewBeaconNode(t *testing.T, config *end2EndConfig, beaconNodes []*beaconNodeInfo) *beaconNodeInfo {
	tmpPath := config.tmpPath
	index := len(beaconNodes)
	binaryPath, found := bazel.FindBinary("beacon-chain", "beacon-chain")
	if !found {
		t.Log(binaryPath)
		t.Fatal("beacon chain binary not found")
	}
	file, err := os.Create(path.Join(tmpPath, fmt.Sprintf("beacon-%d.log", index)))
	if err != nil {
		t.Fatal(err)
	}

	args := []string{
		"--no-discovery",
		"--http-web3provider=http://127.0.0.1:8545",
		"--web3provider=ws://127.0.0.1:8546",
		fmt.Sprintf("--datadir=%s/eth2-beacon-node-%d", tmpPath, index),
		fmt.Sprintf("--deposit-contract=%s", config.contractAddr.Hex()),
		fmt.Sprintf("--rpc-port=%d", 4000+index),
		fmt.Sprintf("--p2p-udp-port=%d", 12000+index),
		fmt.Sprintf("--p2p-tcp-port=%d", 13000+index),
		fmt.Sprintf("--monitoring-port=%d", 8080+index),
		fmt.Sprintf("--grpc-gateway-port=%d", 3200+index),
	}

	if config.minimalConfig {
		args = append(args, "--minimal-config")
	}
	// After the first node is made, have all following nodes connect to all previously made nodes.
	if index >= 1 {
		for p := 0; p < index; p++ {
			args = append(args, fmt.Sprintf("--peer=%s", beaconNodes[p].multiAddr))
		}
	}

	t.Logf("Starting beacon chain with flags %s", strings.Join(args, " "))
	cmd := exec.Command(binaryPath, args...)
	cmd.Stderr = file
	cmd.Stdout = file
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start beacon node: %v", err)
	}

	if err = waitForTextInFile(file, "Node started p2p server"); err != nil {
		t.Fatal(err)
	}

	byteContent, err := ioutil.ReadFile(file.Name())
	if err != nil {
		t.Fatal(err)
	}
	contents := string(byteContent)

	searchText := "\"Node started p2p server\" multiAddr=\""
	startIdx := strings.Index(contents, searchText)
	if startIdx == -1 {
		t.Fatalf("did not find peer text in %s", contents)
	}
	startIdx += len(searchText)
	endIdx := strings.Index(contents[startIdx:], "\"")
	if endIdx == -1 {
		t.Fatalf("did not find peer text in %s", contents)
	}
	multiAddr := contents[startIdx : startIdx+endIdx]

	return &beaconNodeInfo{
		processID:   cmd.Process.Pid,
		datadir:     fmt.Sprintf("%s/eth2-beacon-node-%d", tmpPath, index),
		rpcPort:     (4000) + uint64(index),
		monitorPort: 8080 + uint64(index),
		grpcPort:    3200 + uint64(index),
		multiAddr:   multiAddr,
	}
}

// initializeValidators sends the deposits to the eth1 chain and starts the validator clients.
func initializeValidators(
	t *testing.T,
	config *end2EndConfig,
	keystorePath string,
	beaconNodes []*beaconNodeInfo,
) []*validatorClientInfo {
	binaryPath, found := bazel.FindBinary("validator", "validator")
	if !found {
		t.Fatal("validator binary not found")
	}

	tmpPath := config.tmpPath
	contractAddress := config.contractAddr
	validatorNum := config.numValidators
	beaconNodeNum := config.numBeaconNodes
	if validatorNum%beaconNodeNum != 0 {
		t.Fatal("Validator count is not easily divisible by beacon node count.")
	}

	valClients := make([]*validatorClientInfo, beaconNodeNum)
	validatorsPerNode := validatorNum / beaconNodeNum
	for n := uint64(0); n < beaconNodeNum; n++ {
		file, err := os.Create(path.Join(tmpPath, fmt.Sprintf("vals-%d.log", n)))
		if err != nil {
			t.Fatal(err)
		}
		args := []string{
			fmt.Sprintf("--interop-num-validators=%d", validatorsPerNode),
			fmt.Sprintf("--interop-start-index=%d", validatorsPerNode*n),
			fmt.Sprintf("--monitoring-port=%d", 9080+n),
			fmt.Sprintf("--beacon-rpc-provider=localhost:%d", 4000+n),
		}
		cmd := exec.Command(binaryPath, args...)
		cmd.Stdout = file
		cmd.Stderr = file
		t.Logf("Starting validator client with flags %s", strings.Join(args, " "))
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		valClients[n] = &validatorClientInfo{
			processID:   cmd.Process.Pid,
			monitorPort: 9080 + n,
		}
	}

	client, err := rpc.Dial(path.Join(tmpPath, "eth1data/geth.ipc"))
	if err != nil {
		t.Fatal(err)
	}
	web3 := ethclient.NewClient(client)

	jsonBytes, err := ioutil.ReadFile(keystorePath)
	if err != nil {
		t.Fatal(err)
	}
	txOps, err := bind.NewTransactor(bytes.NewReader(jsonBytes), "" /*password*/)
	if err != nil {
		t.Fatal(err)
	}
	depositInGwei := big.NewInt(int64(params.BeaconConfig().MaxEffectiveBalance))
	txOps.Value = depositInGwei.Mul(depositInGwei, big.NewInt(int64(params.BeaconConfig().GweiPerEth)))
	txOps.GasLimit = 4000000

	contract, err := contracts.NewDepositContract(contractAddress, web3)
	if err != nil {
		t.Fatal(err)
	}

	deposits, roots, _ := testutil.SetupInitialDeposits(t, validatorNum)
	for index, dd := range deposits {
		_, err = contract.Deposit(txOps, dd.Data.PublicKey, dd.Data.WithdrawalCredentials, dd.Data.Signature, roots[index])
		if err != nil {
			t.Error("unable to send transaction to contract")
		}
	}

	keystore, err := keystore.DecryptKey(jsonBytes, "" /*password*/)
	if err != nil {
		t.Fatal(err)
	}
	// Picked 20 for this as a "safe" number of blocks to mine so the deposits
	// are detected.
	if err := mineBlocks(web3, keystore, 20); err != nil {
		t.Fatal(err)
	}

	return valClients
}

func peersConnect(port uint64, expectedPeers uint64) error {
	response, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/p2p", port))
	if err != nil {
		return err
	}
	dataInBytes, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return err
	}
	pageContent := string(dataInBytes)
	if err := response.Body.Close(); err != nil {
		return err
	}
	// Subtracting by 2 here since the libp2p page has "3 peers" as text.
	// With a starting index before the "p", going two characters back should give us
	// the number we need.
	startIdx := strings.Index(pageContent, "peers") - 2
	if startIdx == -3 {
		return fmt.Errorf("could not find needed text in %s", pageContent)
	}
	peerCount, err := strconv.Atoi(pageContent[startIdx : startIdx+1])
	if err != nil {
		return err
	}
	if expectedPeers != uint64(peerCount) {
		return fmt.Errorf("unexpected amount of peers, expected %d, received %d", expectedPeers, peerCount)
	}
	return nil
}

func mineBlocks(web3 *ethclient.Client, keystore *keystore.Key, blocksToMake uint64) error {
	nonce, err := web3.PendingNonceAt(context.Background(), keystore.Address)
	if err != nil {
		return err
	}
	chainID, err := web3.NetworkID(context.Background())
	if err != nil {
		return err
	}
	block, err := web3.BlockByNumber(context.Background(), nil)
	if err != nil {
		return err
	}
	finishBlock := block.NumberU64() + blocksToMake

	for block.NumberU64() <= finishBlock {
		spamTX := types.NewTransaction(nonce, keystore.Address, big.NewInt(0), 21000, big.NewInt(1e6), []byte{})
		signed, err := types.SignTx(spamTX, types.NewEIP155Signer(chainID), keystore.PrivateKey)
		if err != nil {
			return err
		}
		if err := web3.SendTransaction(context.Background(), signed); err != nil {
			return err
		}
		nonce++
		time.Sleep(250 * time.Microsecond)
		block, err = web3.BlockByNumber(context.Background(), nil)
		if err != nil {
			return err
		}
	}
	return nil
}

func killProcesses(t *testing.T, pIDs []int) {
	for _, id := range pIDs {
		process, err := os.FindProcess(id)
		if err != nil {
			t.Fatalf("could not find process %d: %v", id, err)
		}
		if err := process.Kill(); err != nil {
			t.Fatal(err)
		}
	}
}

func logOutput(t *testing.T, tmpPath string) {
	if t.Failed() {
		t.Log("beacon-1.log")
		beacon1LogFile, err := os.Open(path.Join(tmpPath, "beacon-1.log"))
		if err != nil {
			t.Fatal(err)
		}
		scanner := bufio.NewScanner(beacon1LogFile)
		for scanner.Scan() {
			currentLine := scanner.Text()
			t.Log(currentLine)
		}
	}
}

func waitForTextInFile(file *os.File, text string) error {
	wait := 1
	// Putting the wait cap at 16 since at this point its already been waiting
	// for 15 seconds. Using exponential backoff.
	maxWait := 16
	for wait <= maxWait {
		time.Sleep(time.Duration(wait) * time.Second)
		// Rewind the file pointer to the start of the file so we can read it again.
		_, err := file.Seek(0, io.SeekStart)
		if err != nil {
			return fmt.Errorf("could not rewind file to start: %v", err)
		}

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			if strings.Contains(scanner.Text(), text) {
				return nil
			}
		}
		if err := scanner.Err(); err != nil {
			return err
		}
		if wait == 1 {
			wait++
		}
		wait *= wait
	}
	return fmt.Errorf("could not find requested text %s in logs", text)
}
