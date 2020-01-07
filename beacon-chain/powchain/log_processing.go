package powchain

import (
	"context"
	"encoding/binary"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	gethTypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/pkg/errors"
	ethpb "github.com/prysmaticlabs/ethereumapis/eth/v1alpha1"
	"github.com/prysmaticlabs/go-ssz"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/feed"
	statefeed "github.com/prysmaticlabs/prysm/beacon-chain/core/feed/state"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/state"
	contracts "github.com/prysmaticlabs/prysm/contracts/deposit-contract"
	protodb "github.com/prysmaticlabs/prysm/proto/beacon/db"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/prysmaticlabs/prysm/shared/featureconfig"
	"github.com/prysmaticlabs/prysm/shared/hashutil"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/sirupsen/logrus"
)

var (
	depositEventSignature = hashutil.HashKeccak256([]byte("DepositEvent(bytes,bytes,bytes,bytes,bytes)"))
)

const eth1LookBackPeriod = 100
const eth1DataSavingInterval = 100

// Eth2GenesisPowchainInfo retrieves the genesis time and eth1 block number of the beacon chain
// from the deposit contract.
func (s *Service) Eth2GenesisPowchainInfo() (uint64, *big.Int) {
	return s.chainStartData.GenesisTime, big.NewInt(int64(s.chainStartData.GenesisBlock))
}

// ProcessETH1Block processes the logs from the provided eth1Block.
func (s *Service) ProcessETH1Block(ctx context.Context, blkNum *big.Int) error {
	query := ethereum.FilterQuery{
		Addresses: []common.Address{
			s.depositContractAddress,
		},
		FromBlock: blkNum,
		ToBlock:   blkNum,
	}
	logs, err := s.httpLogger.FilterLogs(ctx, query)
	if err != nil {
		return err
	}
	for _, log := range logs {
		// ignore logs that are not of the required block number
		if log.BlockNumber != blkNum.Uint64() {
			continue
		}
		if err := s.ProcessLog(ctx, log); err != nil {
			return errors.Wrap(err, "could not process log")
		}
	}
	if !s.chainStartData.Chainstarted {
		if err := s.checkForChainStart(ctx, blkNum); err != nil {
			return err
		}
	}
	return nil
}

// ProcessLog is the main method which handles the processing of all
// logs from the deposit contract on the ETH1.0 chain.
func (s *Service) ProcessLog(ctx context.Context, depositLog gethTypes.Log) error {
	s.processingLock.RLock()
	defer s.processingLock.RUnlock()
	// Process logs according to their event signature.
	if depositLog.Topics[0] == depositEventSignature {
		if err := s.ProcessDepositLog(ctx, depositLog); err != nil {
			return errors.Wrap(err, "Could not process deposit log")
		}
		if featureconfig.Get().EnableSavingOfDepositData && s.lastReceivedMerkleIndex%eth1DataSavingInterval == 0 {
			eth1Data := &protodb.ETH1ChainData{
				CurrentEth1Data:   s.latestEth1Data,
				ChainstartData:    s.chainStartData,
				BeaconState:       s.preGenesisState,
				Trie:              s.depositTrie.ToProto(),
				DepositContainers: s.depositCache.AllDepositContainers(ctx),
			}
			return s.beaconDB.SavePowchainData(ctx, eth1Data)
		}
		return nil
	}
	log.WithField("signature", fmt.Sprintf("%#x", depositLog.Topics[0])).Debug("Not a valid event signature")
	return nil
}

// ProcessDepositLog processes the log which had been received from
// the ETH1.0 chain by trying to ascertain which participant deposited
// in the contract.
func (s *Service) ProcessDepositLog(ctx context.Context, depositLog gethTypes.Log) error {
	pubkey, withdrawalCredentials, amount, signature, merkleTreeIndex, err := contracts.UnpackDepositLogData(depositLog.Data)
	if err != nil {
		return errors.Wrap(err, "Could not unpack log")
	}
	// If we have already seen this Merkle index, skip processing the log.
	// This can happen sometimes when we receive the same log twice from the
	// ETH1.0 network, and prevents us from updating our trie
	// with the same log twice, causing an inconsistent state root.
	index := binary.LittleEndian.Uint64(merkleTreeIndex)
	if int64(index) <= s.lastReceivedMerkleIndex {
		return nil
	}

	if int64(index) != s.lastReceivedMerkleIndex+1 {
		missedDepositLogsCount.Inc()
		if s.requestingOldLogs {
			return errors.New("received incorrect merkle index")
		}
		if err := s.requestMissingLogs(ctx, depositLog.BlockNumber, int64(index-1)); err != nil {
			return errors.Wrap(err, "could not get correct merkle index")
		}

	}
	s.lastReceivedMerkleIndex = int64(index)

	// We then decode the deposit input in order to create a deposit object
	// we can store in our persistent DB.
	depositData := &ethpb.Deposit_Data{
		Amount:                bytesutil.FromBytes8(amount),
		PublicKey:             pubkey,
		Signature:             signature,
		WithdrawalCredentials: withdrawalCredentials,
	}

	depositHash, err := ssz.HashTreeRoot(depositData)
	if err != nil {
		return errors.Wrap(err, "Unable to determine hashed value of deposit")
	}

	s.depositTrie.Insert(depositHash[:], int(index))

	proof, err := s.depositTrie.MerkleProof(int(index))
	if err != nil {
		return errors.Wrap(err, "Unable to generate merkle proof for deposit")
	}

	deposit := &ethpb.Deposit{
		Data:  depositData,
		Proof: proof,
	}

	// Make sure duplicates are rejected pre-chainstart.
	if !s.chainStartData.Chainstarted {
		var pubkey = fmt.Sprintf("#%x", depositData.PublicKey)
		if s.depositCache.PubkeyInChainstart(ctx, pubkey) {
			log.Warnf("Pubkey %#x has already been submitted for chainstart", pubkey)
		} else {
			s.depositCache.MarkPubkeyForChainstart(ctx, pubkey)
		}

	}

	// We always store all historical deposits in the DB.
	s.depositCache.InsertDeposit(ctx, deposit, depositLog.BlockNumber, int64(index), s.depositTrie.Root())
	validData := true
	if !s.chainStartData.Chainstarted {
		s.chainStartData.ChainstartDeposits = append(s.chainStartData.ChainstartDeposits, deposit)
		root := s.depositTrie.Root()
		eth1Data := &ethpb.Eth1Data{
			DepositRoot:  root[:],
			DepositCount: uint64(len(s.chainStartData.ChainstartDeposits)),
		}
		if err := s.processDeposit(eth1Data, deposit); err != nil {
			log.Errorf("Invalid deposit processed: %v", err)
			validData = false
		}
	} else {
		s.depositCache.InsertPendingDeposit(ctx, deposit, depositLog.BlockNumber, int64(index), s.depositTrie.Root())
	}
	if validData {
		log.WithFields(logrus.Fields{
			"publicKey":       fmt.Sprintf("%#x", depositData.PublicKey),
			"merkleTreeIndex": index,
		}).Debug("Deposit registered from deposit contract")
		validDepositsCount.Inc()
	} else {
		log.WithFields(logrus.Fields{
			"eth1Block":       depositLog.BlockHash.Hex(),
			"eth1Tx":          depositLog.TxHash.Hex(),
			"merkleTreeIndex": index,
		}).Info("Invalid deposit registered in deposit contract")
	}
	return nil
}

// ProcessChainStart processes the log which had been received from
// the ETH1.0 chain by trying to determine when to start the beacon chain.
func (s *Service) ProcessChainStart(genesisTime uint64, eth1BlockHash [32]byte, blockNumber *big.Int) {
	s.chainStartData.Chainstarted = true
	s.chainStartData.GenesisBlock = blockNumber.Uint64()

	chainStartTime := time.Unix(int64(genesisTime), 0)

	for i := range s.chainStartData.ChainstartDeposits {
		proof, err := s.depositTrie.MerkleProof(i)
		if err != nil {
			log.Errorf("Unable to generate deposit proof %v", err)
		}
		s.chainStartData.ChainstartDeposits[i].Proof = proof
	}

	root := s.depositTrie.Root()
	s.chainStartData.Eth1Data = &ethpb.Eth1Data{
		DepositCount: uint64(len(s.chainStartData.ChainstartDeposits)),
		DepositRoot:  root[:],
		BlockHash:    eth1BlockHash[:],
	}

	log.WithFields(logrus.Fields{
		"ChainStartTime": chainStartTime,
	}).Info("Minimum number of validators reached for beacon-chain to start")
	s.stateNotifier.StateFeed().Send(&feed.Event{
		Type: statefeed.ChainStarted,
		Data: &statefeed.ChainStartedData{
			StartTime: chainStartTime,
		},
	})
}

func (s *Service) createGenesisTime(timeStamp uint64) uint64 {
	if featureconfig.Get().NoGenesisDelay {
		return timeStamp
	}
	timeStampRdDown := timeStamp - timeStamp%params.BeaconConfig().MinGenesisDelay
	// genesisTime will be set to the first second of the day, two days after it was triggered.
	return timeStampRdDown + 2*params.BeaconConfig().MinGenesisDelay
}

// processPastLogs processes all the past logs from the deposit contract and
// updates the deposit trie with the data from each individual log.
func (s *Service) processPastLogs(ctx context.Context) error {
	currentBlockNum := s.latestEth1Data.LastRequestedBlock
	query := ethereum.FilterQuery{
		Addresses: []common.Address{
			s.depositContractAddress,
		},
	}

	// if we are not starting from the first deposit log, we use
	// the current saved last requested block number.
	if s.lastReceivedMerkleIndex != -1 {
		query = ethereum.FilterQuery{
			Addresses: []common.Address{
				s.depositContractAddress,
			},
			FromBlock: big.NewInt(int64(currentBlockNum)),
		}
	}

	logs, err := s.httpLogger.FilterLogs(ctx, query)
	if err != nil {
		return err
	}

	for _, log := range logs {
		if log.BlockNumber > currentBlockNum {
			if !s.chainStartData.Chainstarted {
				if err := s.checkForChainStart(ctx, big.NewInt(int64(currentBlockNum))); err != nil {
					return err
				}
			}
			// set new block number after checking for chainstart for previous block.
			s.latestEth1Data.LastRequestedBlock = currentBlockNum
			currentBlockNum = log.BlockNumber
		}
		if err := s.ProcessLog(ctx, log); err != nil {
			return err
		}
	}

	s.latestEth1Data.LastRequestedBlock = currentBlockNum

	currentState, err := s.beaconDB.HeadState(ctx)
	if err != nil {
		return errors.Wrap(err, "could not get head state")
	}

	if currentState != nil && currentState.Eth1DepositIndex > 0 {
		s.depositCache.PrunePendingDeposits(ctx, int(currentState.Eth1DepositIndex))
	}

	return nil
}

// requestBatchedLogs requests and processes all the logs from the period
// last polled to now.
func (s *Service) requestBatchedLogs(ctx context.Context) error {
	// We request for the nth block behind the current head, in order to have
	// stabilized logs when we retrieve it from the 1.0 chain.

	requestedBlock := s.latestEth1Data.BlockHeight
	for i := s.latestEth1Data.LastRequestedBlock + 1; i <= requestedBlock; i++ {
		err := s.ProcessETH1Block(ctx, big.NewInt(int64(i)))
		if err != nil {
			return err
		}
		s.latestEth1Data.LastRequestedBlock = i
	}

	return nil
}

// requestMissingLogs requests any logs that were missed by requesting from previous blocks
// until the current block(exclusive).
func (s *Service) requestMissingLogs(ctx context.Context, blkNumber uint64, wantedIndex int64) error {
	// Prevent this method from being called recursively
	s.requestingOldLogs = true
	defer func() {
		s.requestingOldLogs = false
	}()
	// We request from the last requested block till the current block(exclusive)
	beforeCurrentBlk := big.NewInt(int64(blkNumber) - 1)
	startBlock := s.latestEth1Data.LastRequestedBlock + 1
	for {
		err := s.processBlksInRange(ctx, startBlock, beforeCurrentBlk.Uint64())
		if err != nil {
			return err
		}

		if s.lastReceivedMerkleIndex == wantedIndex {
			break
		}

		// If the required logs still do not exist after the lookback period, then we return an error.
		if startBlock < s.latestEth1Data.LastRequestedBlock-eth1LookBackPeriod {
			return fmt.Errorf(
				"latest index observed is not accurate, wanted %d, but received  %d",
				wantedIndex,
				s.lastReceivedMerkleIndex,
			)
		}
		startBlock--
	}
	return nil
}

func (s *Service) processBlksInRange(ctx context.Context, startBlk uint64, endBlk uint64) error {
	for i := startBlk; i <= endBlk; i++ {
		err := s.ProcessETH1Block(ctx, big.NewInt(int64(i)))
		if err != nil {
			return err
		}
	}
	return nil
}

// checkForChainStart checks the given  block number for if chainstart has occurred.
func (s *Service) checkForChainStart(ctx context.Context, blkNum *big.Int) error {
	blk, err := s.blockFetcher.BlockByNumber(ctx, blkNum)
	if err != nil {
		return errors.Wrap(err, "could not get eth1 block")
	}
	if blk == nil {
		return errors.Wrap(err, "got empty block from powchain service")
	}
	if blk.Hash() == [32]byte{} {
		return errors.New("got empty blockhash from powchain service")
	}
	timeStamp := blk.Time()
	valCount, _ := helpers.ActiveValidatorCount(s.preGenesisState, 0)
	triggered := state.IsValidGenesisState(valCount, s.createGenesisTime(timeStamp))
	if triggered {
		s.chainStartData.GenesisTime = s.createGenesisTime(timeStamp)
		s.ProcessChainStart(s.chainStartData.GenesisTime, blk.Hash(), blk.Number())
	}
	return nil
}
