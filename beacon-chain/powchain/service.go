// Package powchain defines a runtime service which is tasked with
// communicating with an eth1 endpoint, processing logs from a deposit
// contract, and the latest eth1 data headers for usage in the beacon node.
package powchain

import (
	"context"
	"fmt"
	"math/big"
	"reflect"
	"runtime/debug"
	"sort"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	gethTypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	gethRPC "github.com/ethereum/go-ethereum/rpc"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prysmaticlabs/prysm/beacon-chain/cache/depositcache"
	statefeed "github.com/prysmaticlabs/prysm/beacon-chain/core/feed/state"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/state"
	"github.com/prysmaticlabs/prysm/beacon-chain/db"
	"github.com/prysmaticlabs/prysm/beacon-chain/powchain/types"
	iface "github.com/prysmaticlabs/prysm/beacon-chain/state/interface"
	"github.com/prysmaticlabs/prysm/beacon-chain/state/stategen"
	"github.com/prysmaticlabs/prysm/beacon-chain/state/v1"
	contracts "github.com/prysmaticlabs/prysm/contracts/deposit-contract"
	protodb "github.com/prysmaticlabs/prysm/proto/beacon/db"
	ethpb "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/prysmaticlabs/prysm/shared/clientstats"
	"github.com/prysmaticlabs/prysm/shared/httputils"
	"github.com/prysmaticlabs/prysm/shared/httputils/authorizationmethod"
	"github.com/prysmaticlabs/prysm/shared/logutil"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/timeutils"
	"github.com/prysmaticlabs/prysm/shared/trieutil"
	"github.com/sirupsen/logrus"
)

var (
	validDepositsCount = promauto.NewCounter(prometheus.CounterOpts{
		Name: "powchain_valid_deposits_received",
		Help: "The number of valid deposits received in the deposit contract",
	})
	blockNumberGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "powchain_block_number",
		Help: "The current block number in the proof-of-work chain",
	})
	missedDepositLogsCount = promauto.NewCounter(prometheus.CounterOpts{
		Name: "powchain_missed_deposit_logs",
		Help: "The number of times a missed deposit log is detected",
	})
)

var (
	// time to wait before trying to reconnect with the eth1 node.
	backOffPeriod = 15 * time.Second
	// amount of times before we log the status of the eth1 dial attempt.
	logThreshold = 8
	// period to log chainstart related information
	logPeriod = 1 * time.Minute
	// threshold of how old we will accept an eth1 node's head to be.
	eth1Threshold = 20 * time.Minute
	// error when eth1 node is not synced.
	errNotSynced = errors.New("eth1 node is still syncing")
	// error when eth1 node is not synched and genesis state does not exist
	errNotSyncedNoGenesisState = errors.New("genesis state missing, eth1 node out of sync")
	// error when eth1 node is too far behind.
	errFarBehind = errors.Errorf("eth1 head is more than %s behind from current wall clock time", eth1Threshold.String())
)

// ChainStartFetcher retrieves information pertaining to the chain start event
// of the beacon chain for usage across various services.
type ChainStartFetcher interface {
	ChainStartDeposits() []*ethpb.Deposit
	ChainStartEth1Data() *ethpb.Eth1Data
	PreGenesisState() iface.BeaconState
	ClearPreGenesisData()
}

// ChainInfoFetcher retrieves information about eth1 metadata at the Ethereum consensus genesis time.
type ChainInfoFetcher interface {
	Eth2GenesisPowchainInfo() (uint64, *big.Int)
	IsConnectedToETH1() bool
}

// POWBlockFetcher defines a struct that can retrieve mainchain blocks.
type POWBlockFetcher interface {
	BlockTimeByHeight(ctx context.Context, height *big.Int) (uint64, error)
	BlockByTimestamp(ctx context.Context, time uint64) (*types.HeaderInfo, error)
	BlockHashByHeight(ctx context.Context, height *big.Int) (common.Hash, error)
	BlockExists(ctx context.Context, hash common.Hash) (bool, *big.Int, error)
	BlockExistsWithCache(ctx context.Context, hash common.Hash) (bool, *big.Int, error)
}

// Chain defines a standard interface for the powchain service in Prysm.
type Chain interface {
	ChainStartFetcher
	ChainInfoFetcher
	POWBlockFetcher
}

// RPCDataFetcher defines a subset of methods conformed to by ETH1.0 RPC clients for
// fetching eth1 data from the clients.
type RPCDataFetcher interface {
	HeaderByNumber(ctx context.Context, number *big.Int) (*gethTypes.Header, error)
	HeaderByHash(ctx context.Context, hash common.Hash) (*gethTypes.Header, error)
	SyncProgress(ctx context.Context) (*ethereum.SyncProgress, error)
}

// RPCClient defines the rpc methods required to interact with the eth1 node.
type RPCClient interface {
	BatchCall(b []gethRPC.BatchElem) error
}

// Service fetches important information about the canonical
// Ethereum ETH1.0 chain via a web3 endpoint using an ethclient. The Random
// Beacon Chain requires synchronization with the ETH1.0 chain's current
// blockhash, block number, and access to logs within the
// Validator Registration Contract on the ETH1.0 chain to kick off the beacon
// chain's validator registration process.
type Service struct {
	connectedETH1           bool
	isRunning               bool
	processingLock          sync.RWMutex
	cfg                     *Web3ServiceConfig
	ctx                     context.Context
	cancel                  context.CancelFunc
	headTicker              *time.Ticker
	httpEndpoints           []httputils.Endpoint
	currHttpEndpoint        httputils.Endpoint
	httpLogger              bind.ContractFilterer
	eth1DataFetcher         RPCDataFetcher
	rpcClient               RPCClient
	headerCache             *headerCache // cache to store block hash/block height.
	latestEth1Data          *protodb.LatestETH1Data
	depositContractCaller   *contracts.DepositContractCaller
	depositTrie             *trieutil.SparseMerkleTrie
	chainStartData          *protodb.ChainStartData
	lastReceivedMerkleIndex int64 // Keeps track of the last received index to prevent log spam.
	runError                error
	preGenesisState         iface.BeaconState
	bsUpdater               BeaconNodeStatsUpdater
}

// Web3ServiceConfig defines a config struct for web3 service to use through its life cycle.
type Web3ServiceConfig struct {
	HttpEndpoints          []string
	DepositContract        common.Address
	BeaconDB               db.HeadAccessDatabase
	DepositCache           *depositcache.DepositCache
	StateNotifier          statefeed.Notifier
	StateGen               *stategen.State
	Eth1HeaderReqLimit     uint64
	BeaconNodeStatsUpdater BeaconNodeStatsUpdater
}

// NewService sets up a new instance with an ethclient when
// given a web3 endpoint as a string in the config.
func NewService(ctx context.Context, config *Web3ServiceConfig) (*Service, error) {
	ctx, cancel := context.WithCancel(ctx)
	_ = cancel // govet fix for lost cancel. Cancel is handled in service.Stop()
	depositTrie, err := trieutil.NewTrie(params.BeaconConfig().DepositContractTreeDepth)
	if err != nil {
		cancel()
		return nil, errors.Wrap(err, "could not setup deposit trie")
	}
	genState, err := state.EmptyGenesisState()
	if err != nil {
		return nil, errors.Wrap(err, "could not setup genesis state")
	}

	if config.Eth1HeaderReqLimit == 0 {
		config.Eth1HeaderReqLimit = defaultEth1HeaderReqLimit
	}

	stringEndpoints := dedupEndpoints(config.HttpEndpoints)
	endpoints := make([]httputils.Endpoint, len(stringEndpoints))
	for i, e := range stringEndpoints {
		endpoints[i] = HttpEndpoint(e)
	}

	// Select first http endpoint in the provided list.
	var currEndpoint httputils.Endpoint
	if len(config.HttpEndpoints) > 0 {
		currEndpoint = endpoints[0]
	}
	s := &Service{
		ctx:              ctx,
		cancel:           cancel,
		cfg:              config,
		httpEndpoints:    endpoints,
		currHttpEndpoint: currEndpoint,
		latestEth1Data: &protodb.LatestETH1Data{
			BlockHeight:        0,
			BlockTime:          0,
			BlockHash:          []byte{},
			LastRequestedBlock: 0,
		},
		headerCache: newHeaderCache(),
		depositTrie: depositTrie,
		chainStartData: &protodb.ChainStartData{
			Eth1Data:           &ethpb.Eth1Data{},
			ChainstartDeposits: make([]*ethpb.Deposit, 0),
		},
		lastReceivedMerkleIndex: -1,
		preGenesisState:         genState,
		headTicker:              time.NewTicker(time.Duration(params.BeaconConfig().SecondsPerETH1Block) * time.Second),
		// use the nop updater by default, rely on upstream set up to pass in an appropriate impl
		bsUpdater: config.BeaconNodeStatsUpdater,
	}

	if config.BeaconNodeStatsUpdater == nil {
		s.bsUpdater = &NopBeaconNodeStatsUpdater{}
	}

	if err := s.ensureValidPowchainData(ctx); err != nil {
		return nil, errors.Wrap(err, "unable to validate powchain data")
	}

	eth1Data, err := config.BeaconDB.PowchainData(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "unable to retrieve eth1 data")
	}
	if err := s.initializeEth1Data(ctx, eth1Data); err != nil {
		return nil, err
	}

	return s, nil
}

// Start a web3 service's main event loop.
func (s *Service) Start() {
	// If the chain has not started already and we don't have access to eth1 nodes, we will not be
	// able to generate the genesis state.
	if !s.chainStartData.Chainstarted && s.currHttpEndpoint.Url == "" {
		// check for genesis state before shutting down the node,
		// if a genesis state exists, we can continue on.
		genState, err := s.cfg.BeaconDB.GenesisState(s.ctx)
		if err != nil {
			log.Fatal(err)
		}
		if genState == nil || genState.IsNil() {
			log.Fatal("cannot create genesis state: no eth1 http endpoint defined")
		}
	}

	// Exit early if eth1 endpoint is not set.
	if s.currHttpEndpoint.Url == "" {
		return
	}
	go func() {
		s.isRunning = true
		s.waitForConnection()
		if s.ctx.Err() != nil {
			log.Info("Context closed, exiting pow goroutine")
			return
		}
		s.run(s.ctx.Done())
	}()
}

// Stop the web3 service's main event loop and associated goroutines.
func (s *Service) Stop() error {
	if s.cancel != nil {
		defer s.cancel()
	}
	s.closeClients()
	return nil
}

// ChainStartDeposits returns a slice of validator deposit data processed
// by the deposit contract and cached in the powchain service.
func (s *Service) ChainStartDeposits() []*ethpb.Deposit {
	return s.chainStartData.ChainstartDeposits
}

// ClearPreGenesisData clears out the stored chainstart deposits and beacon state.
func (s *Service) ClearPreGenesisData() {
	s.chainStartData.ChainstartDeposits = []*ethpb.Deposit{}
	s.preGenesisState = &v1.BeaconState{}
}

// ChainStartEth1Data returns the eth1 data at chainstart.
func (s *Service) ChainStartEth1Data() *ethpb.Eth1Data {
	return s.chainStartData.Eth1Data
}

// PreGenesisState returns a state that contains
// pre-chainstart deposits.
func (s *Service) PreGenesisState() iface.BeaconState {
	return s.preGenesisState
}

// Status is service health checks. Return nil or error.
func (s *Service) Status() error {
	// Service don't start
	if !s.isRunning {
		return nil
	}
	// get error from run function
	if s.runError != nil {
		return s.runError
	}
	return nil
}

func (s *Service) updateBeaconNodeStats() {
	bs := clientstats.BeaconNodeStats{}
	if len(s.httpEndpoints) > 1 {
		bs.SyncEth1FallbackConfigured = true
	}
	if s.IsConnectedToETH1() {
		if s.primaryConnected() {
			bs.SyncEth1Connected = true
		} else {
			bs.SyncEth1FallbackConnected = true
		}
	}
	s.bsUpdater.Update(bs)
}

func (s *Service) updateCurrHttpEndpoint(endpoint httputils.Endpoint) {
	s.currHttpEndpoint = endpoint
	s.updateBeaconNodeStats()
}

func (s *Service) updateConnectedETH1(state bool) {
	s.connectedETH1 = state
	s.updateBeaconNodeStats()
}

// IsConnectedToETH1 checks if the beacon node is connected to a ETH1 Node.
func (s *Service) IsConnectedToETH1() bool {
	return s.connectedETH1
}

// DepositRoot returns the Merkle root of the latest deposit trie
// from the ETH1.0 deposit contract.
func (s *Service) DepositRoot() [32]byte {
	return s.depositTrie.Root()
}

// DepositTrie returns the sparse Merkle trie used for storing
// deposits from the ETH1.0 deposit contract.
func (s *Service) DepositTrie() *trieutil.SparseMerkleTrie {
	return s.depositTrie
}

// LatestBlockHeight in the ETH1.0 chain.
func (s *Service) LatestBlockHeight() *big.Int {
	return big.NewInt(int64(s.latestEth1Data.BlockHeight))
}

// LatestBlockHash in the ETH1.0 chain.
func (s *Service) LatestBlockHash() common.Hash {
	return bytesutil.ToBytes32(s.latestEth1Data.BlockHash)
}

// AreAllDepositsProcessed determines if all the logs from the deposit contract
// are processed.
func (s *Service) AreAllDepositsProcessed() (bool, error) {
	s.processingLock.RLock()
	defer s.processingLock.RUnlock()
	countByte, err := s.depositContractCaller.GetDepositCount(&bind.CallOpts{})
	if err != nil {
		return false, errors.Wrap(err, "could not get deposit count")
	}
	count := bytesutil.FromBytes8(countByte)
	deposits := s.cfg.DepositCache.AllDeposits(s.ctx, nil)
	if count != uint64(len(deposits)) {
		return false, nil
	}
	return true, nil
}

// refers to the latest eth1 block which follows the condition: eth1_timestamp +
// SECONDS_PER_ETH1_BLOCK * ETH1_FOLLOW_DISTANCE <= current_unix_time
func (s *Service) followBlockHeight(ctx context.Context) (uint64, error) {
	latestValidBlock := uint64(0)
	if s.latestEth1Data.BlockHeight > params.BeaconConfig().Eth1FollowDistance {
		latestValidBlock = s.latestEth1Data.BlockHeight - params.BeaconConfig().Eth1FollowDistance
	}
	return latestValidBlock, nil
}

func (s *Service) connectToPowChain() error {
	httpClient, rpcClient, err := s.dialETH1Nodes(s.currHttpEndpoint)
	if err != nil {
		return errors.Wrap(err, "could not dial eth1 nodes")
	}

	depositContractCaller, err := contracts.NewDepositContractCaller(s.cfg.DepositContract, httpClient)
	if err != nil {
		return errors.Wrap(err, "could not create deposit contract caller")
	}

	if httpClient == nil || rpcClient == nil || depositContractCaller == nil {
		return errors.New("eth1 client is nil")
	}

	s.initializeConnection(httpClient, rpcClient, depositContractCaller)
	return nil
}

func (s *Service) dialETH1Nodes(endpoint httputils.Endpoint) (*ethclient.Client, *gethRPC.Client, error) {
	httpRPCClient, err := gethRPC.Dial(endpoint.Url)
	if err != nil {
		return nil, nil, err
	}
	if endpoint.Auth.Method != authorizationmethod.None {
		header, err := endpoint.Auth.ToHeaderValue()
		if err != nil {
			return nil, nil, err
		}
		httpRPCClient.SetHeader("Authorization", header)
	}
	httpClient := ethclient.NewClient(httpRPCClient)
	// Add a method to clean-up and close clients in the event
	// of any connection failure.
	closeClients := func() {
		httpRPCClient.Close()
		httpClient.Close()
	}
	syncProg, err := httpClient.SyncProgress(s.ctx)
	if err != nil {
		closeClients()
		return nil, nil, err
	}
	if syncProg != nil {
		closeClients()
		return nil, nil, errors.New("eth1 node has not finished syncing yet")
	}
	// Make a simple call to ensure we are actually connected to a working node.
	cID, err := httpClient.ChainID(s.ctx)
	if err != nil {
		closeClients()
		return nil, nil, err
	}
	nID, err := httpClient.NetworkID(s.ctx)
	if err != nil {
		closeClients()
		return nil, nil, err
	}
	if cID.Uint64() != params.BeaconConfig().DepositChainID {
		closeClients()
		return nil, nil, fmt.Errorf("eth1 node using incorrect chain id, %d != %d", cID.Uint64(), params.BeaconConfig().DepositChainID)
	}
	if nID.Uint64() != params.BeaconConfig().DepositNetworkID {
		closeClients()
		return nil, nil, fmt.Errorf("eth1 node using incorrect network id, %d != %d", nID.Uint64(), params.BeaconConfig().DepositNetworkID)
	}

	return httpClient, httpRPCClient, nil
}

func (s *Service) initializeConnection(
	httpClient *ethclient.Client,
	rpcClient *gethRPC.Client,
	contractCaller *contracts.DepositContractCaller,
) {
	s.httpLogger = httpClient
	s.eth1DataFetcher = httpClient
	s.depositContractCaller = contractCaller
	s.rpcClient = rpcClient
}

// closes down our active eth1 clients.
func (s *Service) closeClients() {
	gethClient, ok := s.rpcClient.(*gethRPC.Client)
	if ok {
		gethClient.Close()
	}
	httpClient, ok := s.eth1DataFetcher.(*ethclient.Client)
	if ok {
		httpClient.Close()
	}
}

func (s *Service) waitForConnection() {
	errConnect := s.connectToPowChain()
	if errConnect == nil {
		synced, errSynced := s.isEth1NodeSynced()
		// Resume if eth1 node is synced.
		if synced {
			s.updateConnectedETH1(true)
			s.runError = nil
			log.WithFields(logrus.Fields{
				"endpoint": logutil.MaskCredentialsLogging(s.currHttpEndpoint.Url),
			}).Info("Connected to eth1 proof-of-work chain")
			return
		}
		if errSynced != nil {
			s.runError = errSynced
			log.WithError(errSynced).Error("Could not check sync status of eth1 chain")
		}
	}
	if errConnect != nil {
		s.runError = errConnect
		log.WithError(errConnect).Error("Could not connect to powchain endpoint")
	}
	// Use a custom logger to only log errors
	// once in  a while.
	logCounter := 0
	errorLogger := func(err error, msg string) {
		if logCounter > logThreshold {
			log.Errorf("%s: %v", msg, err)
			logCounter = 0
		}
		logCounter++
	}

	ticker := time.NewTicker(backOffPeriod)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			log.Debugf("Trying to dial endpoint: %s", logutil.MaskCredentialsLogging(s.currHttpEndpoint.Url))
			errConnect := s.connectToPowChain()
			if errConnect != nil {
				errorLogger(errConnect, "Could not connect to powchain endpoint")
				s.runError = errConnect
				s.fallbackToNextEndpoint()
				continue
			}
			synced, errSynced := s.isEth1NodeSynced()
			if errSynced != nil {
				errorLogger(errSynced, "Could not check sync status of eth1 chain")
				s.runError = errSynced
				s.fallbackToNextEndpoint()
				continue
			}
			if synced {
				s.updateConnectedETH1(true)
				s.runError = nil
				log.WithFields(logrus.Fields{
					"endpoint": logutil.MaskCredentialsLogging(s.currHttpEndpoint.Url),
				}).Info("Connected to eth1 proof-of-work chain")
				return
			}
			genState, genErr := s.cfg.BeaconDB.GenesisState(s.ctx)
			if genErr != nil {
				s.runError = genErr
				log.WithError(genErr).Error("Could not retrieve genesis state")
				return
			}
			if genState.IsNil() {
				s.runError = errNotSyncedNoGenesisState
				log.Warning("Eth1 node is out of sync and genesis state is missing")
				return
			}
			s.runError = errNotSynced
			log.Debug("Eth1 node is currently syncing")
		case <-s.ctx.Done():
			log.Debug("Received cancelled context,closing existing powchain service")
			return
		}
	}
}

// checks if the eth1 node is healthy and ready to serve before
// fetching data from  it.
func (s *Service) isEth1NodeSynced() (bool, error) {
	syncProg, err := s.eth1DataFetcher.SyncProgress(s.ctx)
	if err != nil {
		return false, err
	}
	if syncProg != nil {
		return false, nil
	}
	head, err := s.eth1DataFetcher.HeaderByNumber(s.ctx, nil)
	if err != nil {
		return false, err
	}
	return !eth1HeadIsBehind(head.Time), nil
}

// Reconnect to eth1 node in case of any failure.
func (s *Service) retryETH1Node(err error) {
	s.runError = err
	s.updateConnectedETH1(false)
	// Back off for a while before
	// resuming dialing the eth1 node.
	time.Sleep(backOffPeriod)
	s.waitForConnection()
	// Reset run error in the event of a successful connection.
	s.runError = nil
}

func (s *Service) initDepositCaches(ctx context.Context, ctrs []*protodb.DepositContainer) error {
	if len(ctrs) == 0 {
		return nil
	}
	s.cfg.DepositCache.InsertDepositContainers(ctx, ctrs)
	if !s.chainStartData.Chainstarted {
		// do not add to pending cache
		// if no genesis state exists.
		validDepositsCount.Add(float64(s.preGenesisState.Eth1DepositIndex()))
		return nil
	}
	genesisState, err := s.cfg.BeaconDB.GenesisState(ctx)
	if err != nil {
		return err
	}
	// Default to all deposits post-genesis deposits in
	// the event we cannot find a finalized state.
	currIndex := genesisState.Eth1DepositIndex()
	chkPt, err := s.cfg.BeaconDB.FinalizedCheckpoint(ctx)
	if err != nil {
		return err
	}
	rt := bytesutil.ToBytes32(chkPt.Root)
	if rt != [32]byte{} {
		fState, err := s.cfg.StateGen.StateByRoot(ctx, rt)
		if err != nil {
			return errors.Wrap(err, "could not get finalized state")
		}
		if fState == nil || fState.IsNil() {
			return errors.Errorf("finalized state with root %#x does not exist in the db", rt)
		}
		// Set deposit index to the one in the current archived state.
		currIndex = fState.Eth1DepositIndex()
	}
	validDepositsCount.Add(float64(currIndex))
	// Only add pending deposits if the container slice length
	// is more than the current index in state.
	if uint64(len(ctrs)) > currIndex {
		for _, c := range ctrs[currIndex:] {
			s.cfg.DepositCache.InsertPendingDeposit(ctx, c.Deposit, c.Eth1BlockHeight, c.Index, bytesutil.ToBytes32(c.DepositRoot))
		}
	}
	return nil
}

// processBlockHeader adds a newly observed eth1 block to the block cache and
// updates the latest blockHeight, blockHash, and blockTime properties of the service.
func (s *Service) processBlockHeader(header *gethTypes.Header) {
	defer safelyHandlePanic()
	blockNumberGauge.Set(float64(header.Number.Int64()))
	s.latestEth1Data.BlockHeight = header.Number.Uint64()
	s.latestEth1Data.BlockHash = header.Hash().Bytes()
	s.latestEth1Data.BlockTime = header.Time
	log.WithFields(logrus.Fields{
		"blockNumber": s.latestEth1Data.BlockHeight,
		"blockHash":   hexutil.Encode(s.latestEth1Data.BlockHash),
	}).Debug("Latest eth1 chain event")
}

// batchRequestHeaders requests the block range specified in the arguments. Instead of requesting
// each block in one call, it batches all requests into a single rpc call.
func (s *Service) batchRequestHeaders(startBlock, endBlock uint64) ([]*gethTypes.Header, error) {
	if startBlock > endBlock {
		return nil, fmt.Errorf("start block height %d cannot be > end block height %d", startBlock, endBlock)
	}
	requestRange := (endBlock - startBlock) + 1
	elems := make([]gethRPC.BatchElem, 0, requestRange)
	headers := make([]*gethTypes.Header, 0, requestRange)
	errs := make([]error, 0, requestRange)
	if requestRange == 0 {
		return headers, nil
	}
	for i := startBlock; i <= endBlock; i++ {
		header := &gethTypes.Header{}
		err := error(nil)
		elems = append(elems, gethRPC.BatchElem{
			Method: "eth_getBlockByNumber",
			Args:   []interface{}{hexutil.EncodeBig(big.NewInt(int64(i))), false},
			Result: header,
			Error:  err,
		})
		headers = append(headers, header)
		errs = append(errs, err)
	}
	ioErr := s.rpcClient.BatchCall(elems)
	if ioErr != nil {
		return nil, ioErr
	}
	for _, e := range errs {
		if e != nil {
			return nil, e
		}
	}
	for _, h := range headers {
		if h != nil {
			if err := s.headerCache.AddHeader(h); err != nil {
				return nil, err
			}
		}
	}
	return headers, nil
}

// safelyHandleHeader will recover and log any panic that occurs from the
// block
func safelyHandlePanic() {
	if r := recover(); r != nil {
		log.WithFields(logrus.Fields{
			"r": r,
		}).Error("Panicked when handling data from ETH 1.0 Chain! Recovering...")

		debug.PrintStack()
	}
}

func (s *Service) handleETH1FollowDistance() {
	defer safelyHandlePanic()
	ctx := s.ctx

	// use a 5 minutes timeout for block time, because the max mining time is 278 sec (block 7208027)
	// (analyzed the time of the block from 2018-09-01 to 2019-02-13)
	fiveMinutesTimeout := timeutils.Now().Add(-5 * time.Minute)
	// check that web3 client is syncing
	if time.Unix(int64(s.latestEth1Data.BlockTime), 0).Before(fiveMinutesTimeout) {
		log.Warn("eth1 client is not syncing")
	}
	if !s.chainStartData.Chainstarted {
		if err := s.checkBlockNumberForChainStart(ctx, big.NewInt(int64(s.latestEth1Data.LastRequestedBlock))); err != nil {
			s.runError = err
			log.Error(err)
			return
		}
	}
	// If the last requested block has not changed,
	// we do not request batched logs as this means there are no new
	// logs for the powchain service to process. Also is a potential
	// failure condition as would mean we have not respected the protocol
	// threshold.
	if s.latestEth1Data.LastRequestedBlock == s.latestEth1Data.BlockHeight {
		log.Error("Beacon node is not respecting the follow distance")
		return
	}
	if err := s.requestBatchedHeadersAndLogs(ctx); err != nil {
		s.runError = err
		log.Error(err)
		return
	}
	// Reset the Status.
	if s.runError != nil {
		s.runError = nil
	}
}

func (s *Service) initPOWService() {

	// Run in a select loop to retry in the event of any failures.
	for {
		select {
		case <-s.ctx.Done():
			return
		default:
			ctx := s.ctx
			header, err := s.eth1DataFetcher.HeaderByNumber(ctx, nil)
			if err != nil {
				log.Errorf("Unable to retrieve latest ETH1.0 chain header: %v", err)
				s.retryETH1Node(err)
				continue
			}

			s.latestEth1Data.BlockHeight = header.Number.Uint64()
			s.latestEth1Data.BlockHash = header.Hash().Bytes()
			s.latestEth1Data.BlockTime = header.Time

			if err := s.processPastLogs(ctx); err != nil {
				log.Errorf("Unable to process past logs %v", err)
				s.retryETH1Node(err)
				continue
			}
			// Cache eth1 headers from our voting period.
			if err := s.cacheHeadersForEth1DataVote(ctx); err != nil {
				log.Errorf("Unable to process past headers %v", err)
				s.retryETH1Node(err)
				continue
			}
			// Handle edge case with embedded genesis state by fetching genesis header to determine
			// its height.
			if s.chainStartData.Chainstarted && s.chainStartData.GenesisBlock == 0 {
				genHeader, err := s.eth1DataFetcher.HeaderByHash(ctx, common.BytesToHash(s.chainStartData.Eth1Data.BlockHash))
				if err != nil {
					log.Errorf("Unable to retrieve genesis ETH1.0 chain header: %v", err)
					s.retryETH1Node(err)
					continue
				}
				s.chainStartData.GenesisBlock = genHeader.Number.Uint64()
				if err := s.savePowchainData(ctx); err != nil {
					log.Errorf("Unable to save powchain data: %v", err)
				}
			}
			return
		}
	}
}

// run subscribes to all the services for the ETH1.0 chain.
func (s *Service) run(done <-chan struct{}) {
	s.runError = nil

	s.initPOWService()

	chainstartTicker := time.NewTicker(logPeriod)
	defer chainstartTicker.Stop()

	for {
		select {
		case <-done:
			s.isRunning = false
			s.runError = nil
			s.updateConnectedETH1(false)
			log.Debug("Context closed, exiting goroutine")
			return
		case <-s.headTicker.C:
			head, err := s.eth1DataFetcher.HeaderByNumber(s.ctx, nil)
			if err != nil {
				log.WithError(err).Debug("Could not fetch latest eth1 header")
				s.retryETH1Node(err)
				continue
			}
			if eth1HeadIsBehind(head.Time) {
				log.WithError(errFarBehind).Debug("Could not get an up to date eth1 header")
				s.retryETH1Node(errFarBehind)
				continue
			}
			s.processBlockHeader(head)
			s.handleETH1FollowDistance()
			s.checkDefaultEndpoint()
		case <-chainstartTicker.C:
			if s.chainStartData.Chainstarted {
				chainstartTicker.Stop()
				continue
			}
			s.logTillChainStart()
		}
	}
}

// logs the current thresholds required to hit chainstart every minute.
func (s *Service) logTillChainStart() {
	if s.chainStartData.Chainstarted {
		return
	}
	_, blockTime, err := s.retrieveBlockHashAndTime(s.ctx, big.NewInt(int64(s.latestEth1Data.LastRequestedBlock)))
	if err != nil {
		log.Error(err)
		return
	}
	valCount, genesisTime := s.currentCountAndTime(blockTime)
	valNeeded := uint64(0)
	if valCount < params.BeaconConfig().MinGenesisActiveValidatorCount {
		valNeeded = params.BeaconConfig().MinGenesisActiveValidatorCount - valCount
	}
	secondsLeft := uint64(0)
	if genesisTime < params.BeaconConfig().MinGenesisTime {
		secondsLeft = params.BeaconConfig().MinGenesisTime - genesisTime
	}

	fields := logrus.Fields{
		"Additional validators needed": valNeeded,
	}
	if secondsLeft > 0 {
		fields["Generating genesis state in"] = time.Duration(secondsLeft) * time.Second
	}

	log.WithFields(fields).Info("Currently waiting for chainstart")
}

// cacheHeadersForEth1DataVote makes sure that voting for eth1data after startup utilizes cached headers
// instead of making multiple RPC requests to the ETH1 endpoint.
func (s *Service) cacheHeadersForEth1DataVote(ctx context.Context) error {
	// Find the end block to request from.
	end, err := s.followBlockHeight(ctx)
	if err != nil {
		return err
	}
	start, err := s.determineEarliestVotingBlock(ctx, end)
	if err != nil {
		return err
	}
	// We call batchRequestHeaders for its header caching side-effect, so we don't need the return value.
	_, err = s.batchRequestHeaders(start, end)
	if err != nil {
		return err
	}
	return nil
}

// determines the earliest voting block from which to start caching all our previous headers from.
func (s *Service) determineEarliestVotingBlock(ctx context.Context, followBlock uint64) (uint64, error) {
	genesisTime := s.chainStartData.GenesisTime
	currSlot := helpers.CurrentSlot(genesisTime)

	// In the event genesis has not occurred yet, we just request go back follow_distance blocks.
	if genesisTime == 0 || currSlot == 0 {
		earliestBlk := uint64(0)
		if followBlock > params.BeaconConfig().Eth1FollowDistance {
			earliestBlk = followBlock - params.BeaconConfig().Eth1FollowDistance
		}
		return earliestBlk, nil
	}
	votingTime := helpers.VotingPeriodStartTime(genesisTime, currSlot)
	followBackDist := 2 * params.BeaconConfig().SecondsPerETH1Block * params.BeaconConfig().Eth1FollowDistance
	if followBackDist > votingTime {
		return 0, errors.Errorf("invalid genesis time provided. %d > %d", followBackDist, votingTime)
	}
	earliestValidTime := votingTime - followBackDist
	hdr, err := s.BlockByTimestamp(ctx, earliestValidTime)
	if err != nil {
		return 0, err
	}
	return hdr.Number.Uint64(), nil
}

// This performs a health check on our primary endpoint, and if it
// is ready to serve we connect to it again. This method is only
// relevant if we are on our backup endpoint.
func (s *Service) checkDefaultEndpoint() {
	primaryEndpoint := s.httpEndpoints[0]
	// Return early if we are running on our primary
	// endpoint.
	if s.currHttpEndpoint.Equals(primaryEndpoint) {
		return
	}

	httpClient, rpcClient, err := s.dialETH1Nodes(primaryEndpoint)
	if err != nil {
		log.Debugf("Primary endpoint not ready: %v", err)
		return
	}
	log.Info("Primary endpoint ready again, switching back to it")
	// Close the clients and let our main connection routine
	// properly connect with it.
	httpClient.Close()
	rpcClient.Close()
	// Close current active clients.
	s.closeClients()

	// Switch back to primary endpoint and try connecting
	// to it again.
	s.updateCurrHttpEndpoint(primaryEndpoint)
	s.retryETH1Node(nil)
}

// This is an inefficient way to search for the next endpoint, but given N is expected to be
// small ( < 25), it is fine to search this way.
func (s *Service) fallbackToNextEndpoint() {
	currEndpoint := s.currHttpEndpoint
	currIndex := 0
	totalEndpoints := len(s.httpEndpoints)

	for i, endpoint := range s.httpEndpoints {
		if endpoint.Equals(currEndpoint) {
			currIndex = i
			break
		}
	}
	nextIndex := currIndex + 1
	if nextIndex >= totalEndpoints {
		nextIndex = 0
	}
	if nextIndex != currIndex {
		log.Infof("Falling back to alternative endpoint: %s", logutil.MaskCredentialsLogging(s.currHttpEndpoint.Url))
	}
	s.updateCurrHttpEndpoint(s.httpEndpoints[nextIndex])
}

// initializes our service from the provided eth1data object by initializing all the relevant
// fields and data.
func (s *Service) initializeEth1Data(ctx context.Context, eth1DataInDB *protodb.ETH1ChainData) error {
	// The node has no eth1data persisted on disk, so we exit and instead
	// request from contract logs.
	if eth1DataInDB == nil {
		return nil
	}
	s.depositTrie = trieutil.CreateTrieFromProto(eth1DataInDB.Trie)
	s.chainStartData = eth1DataInDB.ChainstartData
	var err error
	if !reflect.ValueOf(eth1DataInDB.BeaconState).IsZero() {
		s.preGenesisState, err = v1.InitializeFromProto(eth1DataInDB.BeaconState)
		if err != nil {
			return errors.Wrap(err, "Could not initialize state trie")
		}
	}
	s.latestEth1Data = eth1DataInDB.CurrentEth1Data
	numOfItems := s.depositTrie.NumOfItems()
	s.lastReceivedMerkleIndex = int64(numOfItems - 1)
	if err := s.initDepositCaches(ctx, eth1DataInDB.DepositContainers); err != nil {
		return errors.Wrap(err, "could not initialize caches")
	}
	return nil
}

// validates that all deposit containers are valid and have their relevant indices
// in order.
func (s *Service) validateDepositContainers(ctrs []*protodb.DepositContainer) bool {
	ctrLen := len(ctrs)
	// Exit for empty containers.
	if ctrLen == 0 {
		return true
	}
	// Sort deposits in ascending order.
	sort.Slice(ctrs, func(i, j int) bool {
		return ctrs[i].Index < ctrs[j].Index
	})
	startIndex := int64(0)
	for _, c := range ctrs {
		if c.Index != startIndex {
			log.Info("Recovering missing deposit containers, node is re-requesting missing deposit data")
			return false
		}
		startIndex++
	}
	return true
}

// validates the current powchain data saved and makes sure that any
// embedded genesis state is correctly accounted for.
func (s *Service) ensureValidPowchainData(ctx context.Context) error {
	genState, err := s.cfg.BeaconDB.GenesisState(ctx)
	if err != nil {
		return err
	}
	// Exit early if no genesis state is saved.
	if genState == nil || genState.IsNil() {
		return nil
	}
	eth1Data, err := s.cfg.BeaconDB.PowchainData(ctx)
	if err != nil {
		return errors.Wrap(err, "unable to retrieve eth1 data")
	}
	if eth1Data == nil || !eth1Data.ChainstartData.Chainstarted || !s.validateDepositContainers(eth1Data.DepositContainers) {
		pbState, err := v1.ProtobufBeaconState(s.preGenesisState.InnerStateUnsafe())
		if err != nil {
			return err
		}
		s.chainStartData = &protodb.ChainStartData{
			Chainstarted:       true,
			GenesisTime:        genState.GenesisTime(),
			GenesisBlock:       0,
			Eth1Data:           genState.Eth1Data(),
			ChainstartDeposits: make([]*ethpb.Deposit, 0),
		}
		eth1Data = &protodb.ETH1ChainData{
			CurrentEth1Data:   s.latestEth1Data,
			ChainstartData:    s.chainStartData,
			BeaconState:       pbState,
			Trie:              s.depositTrie.ToProto(),
			DepositContainers: s.cfg.DepositCache.AllDepositContainers(ctx),
		}
		return s.cfg.BeaconDB.SavePowchainData(ctx, eth1Data)
	}
	return nil
}

func dedupEndpoints(endpoints []string) []string {
	selectionMap := make(map[string]bool)
	newEndpoints := make([]string, 0, len(endpoints))
	for _, point := range endpoints {
		if selectionMap[point] {
			continue
		}
		newEndpoints = append(newEndpoints, point)
		selectionMap[point] = true
	}
	return newEndpoints
}

// Checks if the provided timestamp is beyond the prescribed bound from
// the current wall clock time.
func eth1HeadIsBehind(timestamp uint64) bool {
	timeout := timeutils.Now().Add(-eth1Threshold)
	// check that web3 client is syncing
	return time.Unix(int64(timestamp), 0).Before(timeout)
}

func (s *Service) primaryConnected() bool {
	return s.currHttpEndpoint.Equals(s.httpEndpoints[0])
}
