package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"math/big"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/pkg/errors"
	contracts "github.com/prysmaticlabs/prysm/contracts/deposit-contract"
	pb "github.com/prysmaticlabs/prysm/proto/cluster"
	"github.com/prysmaticlabs/prysm/shared/keystore"
)

var gasLimit = uint64(4000000)
var blockTime = time.Duration(14)

type server struct {
	contract      *contracts.DepositContract
	db            *db
	depositAmount *big.Int
	txPk          *ecdsa.PrivateKey
	client        *ethclient.Client

	clientLock sync.Mutex
}

func newServer(
	db *db,
	rpcAddr string,
	depositContractAddr string,
	funderPK string,
	validatorDepositAmount int64,
) *server {
	rpcClient, err := rpc.Dial(rpcAddr)
	if err != nil {
		panic(err)
	}
	client := ethclient.NewClient(rpcClient)

	contract, err := contracts.NewDepositContract(common.HexToAddress(depositContractAddr), client)
	if err != nil {
		panic(err)
	}

	txPk, err := crypto.HexToECDSA(funderPK)
	if err != nil {
		panic(err)
	}

	depositAmount := big.NewInt(validatorDepositAmount)

	return &server{
		contract:      contract,
		client:        client,
		db:            db,
		depositAmount: depositAmount,
		txPk:          txPk,
	}
}

func (s *server) makeDeposit(pubkey []byte, withdrawalCredentials []byte, signature []byte, depositRoot [32]byte) (*types.Transaction, error) {
	txOps := bind.NewKeyedTransactor(s.txPk)
	txOps.Value = s.depositAmount
	txOps.GasLimit = gasLimit
	tx, err := s.contract.Deposit(txOps, pubkey, withdrawalCredentials, signature, depositRoot)
	if err != nil {
		return nil, errors.Wrap(err, "deposit failed")
	}
	log.WithField("tx", tx.Hash().Hex()).Info("Deposit transaction sent")

	return tx, nil
}

func (s *server) Request(ctx context.Context, req *pb.PrivateKeyRequest) (*pb.PrivateKeyResponse, error) {
	s.clientLock.Lock()
	defer s.clientLock.Unlock()

	if req.NumberOfKeys == 0 {
		req.NumberOfKeys = 1
	}

	// build the list of PKs in the following order, until the requested
	// amount is ready to return.
	// - PKs already assigned to the pod
	// - PKs that have not yet been allocated
	// - PKs that are newly initialized with deposits

	pks, err := s.db.PodPKs(ctx, req.PodName)
	if err != nil {
		return nil, err
	}
	if pks != nil && len(pks.PrivateKeys) > 0 {
		log.WithField("pod", req.PodName).Debug("Returning existing assignment(s)")
		return &pb.PrivateKeyResponse{
			PrivateKeys: pks,
		}, nil
	}

	unallocated, err := s.db.UnallocatedPKs(ctx, req.NumberOfKeys)
	if err != nil {
		return nil, err
	}
	log.WithField(
		"pod", req.PodName,
	).WithField(
		"keys", len(unallocated.PrivateKeys),
	).Debug("Recycling existing private key(s)")

	pks.PrivateKeys = append(pks.PrivateKeys, unallocated.PrivateKeys...)

	if len(pks.PrivateKeys) < int(req.NumberOfKeys) {
		c := int(req.NumberOfKeys) - len(pks.PrivateKeys)
		newKeys, err := s.allocateNewKeys(ctx, req.PodName, c)
		if err != nil {
			return nil, err
		}
		pks.PrivateKeys = append(pks.PrivateKeys, newKeys.PrivateKeys...)
	}

	if err := s.db.AssignExistingPKs(ctx, pks, req.PodName); err != nil {
		return nil, err
	}

	return &pb.PrivateKeyResponse{PrivateKeys: pks}, nil
}

func (s *server) allocateNewKeys(ctx context.Context, podName string, numKeys int) (*pb.PrivateKeys, error) {
	pks := make([][]byte, 0, numKeys)
	txMap := make(map[*keystore.Key]*types.Transaction)

	for i := 0; i < numKeys; i++ {
		key, err := keystore.NewKey(rand.Reader)
		if err != nil {
			return nil, err
		}

		// Make the validator deposit
		// NOTE: This uses the validator key as the withdrawal key
		di, dr, err := keystore.DepositInput(key /*depositKey*/, key /*withdrawalKey*/, new(big.Int).Div(s.depositAmount, big.NewInt(1e9)).Uint64())
		if err != nil {
			return nil, err
		}

		// Do the actual deposit
		tx, err := s.makeDeposit(di.PublicKey, di.WithdrawalCredentials, di.Signature, dr)
		if err != nil {
			return nil, err
		}
		txMap[key] = tx
		// Store in database
		if err := s.db.AllocateNewPkToPod(ctx, key, podName); err != nil {
			return nil, err
		}
	}

	for {
		time.Sleep(time.Second * blockTime)
		receivedKeys, err := s.checkDepositTxs(ctx, txMap)
		if err != nil {
			return nil, err
		}
		pks = append(pks, receivedKeys...)
		if len(txMap) == 0 {
			break
		}
	}

	return &pb.PrivateKeys{PrivateKeys: pks}, nil
}

func (s *server) checkDepositTxs(ctx context.Context, txMap map[*keystore.Key]*types.Transaction) ([][]byte,
	error) {
	pks := make([][]byte, 0, len(txMap))
	for k, tx := range txMap {
		receipt, err := s.client.TransactionReceipt(ctx, tx.Hash())
		if err == ethereum.NotFound {
			// tx still not processed yet.
			continue
		}
		if err != nil {
			return nil, err
		}
		if receipt.Status == types.ReceiptStatusFailed {
			delete(txMap, k)
			continue
		}
		// append key if tx succeeded.
		pks = append(pks, k.SecretKey.Marshal())
		delete(txMap, k)
	}
	return pks, nil
}
