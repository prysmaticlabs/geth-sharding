package depositcontract

import (
	"context"
	"crypto/ecdsa"
	"encoding/binary"
	"fmt"
	"log"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/accounts/abi/bind/backends"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/prysmaticlabs/prysm/shared/hashutil"
	"github.com/prysmaticlabs/prysm/shared/mathutil"
	"github.com/prysmaticlabs/prysm/shared/params"
)

var (
	amount33Eth, _        = new(big.Int).SetString("33000000000000000000", 10)
	amount32Eth, _        = new(big.Int).SetString("32000000000000000000", 10)
	amountLessThan1Eth, _ = new(big.Int).SetString("500000000000000000", 10)
)

type testAccount struct {
	addr         common.Address
	contract     *DepositContract
	contractAddr common.Address
	backend      *backends.SimulatedBackend
	txOpts       *bind.TransactOpts
}

func setup() (*testAccount, error) {
	genesis := make(core.GenesisAlloc)
	privKey, _ := crypto.GenerateKey()
	pubKeyECDSA, ok := privKey.Public().(*ecdsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("error casting public key to ECDSA")
	}

	// strip off the 0x and the first 2 characters 04 which is always the EC prefix and is not required.
	publicKeyBytes := crypto.FromECDSAPub(pubKeyECDSA)[4:]
	var pubKey = make([]byte, 48)
	copy(pubKey[:], []byte(publicKeyBytes))

	addr := crypto.PubkeyToAddress(privKey.PublicKey)
	txOpts := bind.NewKeyedTransactor(privKey)
	startingBalance, _ := new(big.Int).SetString("100000000000000000000000000000000000000", 10)
	genesis[addr] = core.GenesisAccount{Balance: startingBalance}
	backend := backends.NewSimulatedBackend(genesis, 210000000000)

	contractAddr, _, contract, err := DeployDepositContract(txOpts, backend)
	if err != nil {
		return nil, err
	}

	return &testAccount{addr, contract, contractAddr, backend, txOpts}, nil
}

func TestSetupAndContractRegistration(t *testing.T) {
	_, err := setup()
	if err != nil {
		log.Fatalf("Can not deploy validator registration contract: %v", err)
	}
}

// negative test case, deposit with less than 1 ETH which is less than the top off amount.
func TestRegisterWithLessThan1Eth(t *testing.T) {
	testAccount, err := setup()
	if err != nil {
		t.Fatal(err)
	}

	testAccount.txOpts.Value = amountLessThan1Eth
	_, err = testAccount.contract.Deposit(testAccount.txOpts, []byte{})
	if err == nil {
		t.Error("Validator registration should have failed with insufficient deposit")
	}
}

// negative test case, deposit with more than 32 ETH which is more than the asked amount.
func TestRegisterWithMoreThan32Eth(t *testing.T) {
	testAccount, err := setup()
	if err != nil {
		t.Fatal(err)
	}

	testAccount.txOpts.Value = amount33Eth
	_, err = testAccount.contract.Deposit(testAccount.txOpts, []byte{})
	if err == nil {
		t.Error("Validator registration should have failed with more than asked deposit amount")
	}
}

// normal test case, test depositing 32 ETH and verify HashChainValue event is correctly emitted.
func TestValidatorRegisters(t *testing.T) {
	testAccount, err := setup()
	if err != nil {
		t.Fatal(err)
	}
	testAccount.txOpts.Value = amount32Eth

	_, err = testAccount.contract.Deposit(testAccount.txOpts, []byte{'A'})
	testAccount.backend.Commit()
	if err != nil {
		t.Errorf("Validator registration failed: %v", err)
	}
	_, err = testAccount.contract.Deposit(testAccount.txOpts, []byte{'B'})
	testAccount.backend.Commit()
	if err != nil {
		t.Errorf("Validator registration failed: %v", err)
	}
	_, err = testAccount.contract.Deposit(testAccount.txOpts, []byte{'C'})
	testAccount.backend.Commit()
	if err != nil {
		t.Errorf("Validator registration failed: %v", err)
	}

	query := ethereum.FilterQuery{
		Addresses: []common.Address{
			testAccount.contractAddr,
		},
	}

	logs, err := testAccount.backend.FilterLogs(context.Background(), query)
	if err != nil {
		t.Fatalf("Unable to get logs of deposit contract: %v", err)
	}

	merkleTreeIndex := make([]uint64, 5)
	depositData := make([][]byte, 5)

	for i, log := range logs {
		_, data, idx, err := UnpackDepositLogData(log.Data)
		if err != nil {
			t.Fatalf("Unable to unpack log data: %v", err)
		}
		merkleTreeIndex[i] = binary.BigEndian.Uint64(idx)
		depositData[i] = data
	}

	twoTothePowerOfTreeDepth := mathutil.PowerOf2(params.BeaconConfig().DepositContractTreeDepth)

	if merkleTreeIndex[0] != twoTothePowerOfTreeDepth {
		t.Errorf("HashChainValue event total desposit count miss matched. Want: %d, Got: %d", twoTothePowerOfTreeDepth+1, merkleTreeIndex[0])
	}

	if merkleTreeIndex[1] != twoTothePowerOfTreeDepth+1 {
		t.Errorf("HashChainValue event total desposit count miss matched. Want: %d, Got: %d", twoTothePowerOfTreeDepth+2, merkleTreeIndex[1])
	}

	if merkleTreeIndex[2] != twoTothePowerOfTreeDepth+2 {
		t.Errorf("HashChainValue event total desposit count miss matched. Want: %v, Got: %v", twoTothePowerOfTreeDepth+3, merkleTreeIndex[2])
	}
}

// normal test case, test beacon chain start log event.
func TestChainStarts(t *testing.T) {
	testAccount, err := setup()
	if err != nil {
		t.Fatal(err)
	}
	testAccount.txOpts.Value = amount32Eth

	for i := 0; i < 8; i++ {
		_, err = testAccount.contract.Deposit(testAccount.txOpts, []byte{'A'})
		if err != nil {
			t.Errorf("Validator registration failed: %v", err)
		}
	}

	testAccount.backend.Commit()

	query := ethereum.FilterQuery{
		Addresses: []common.Address{
			testAccount.contractAddr,
		},
	}

	logs, err := testAccount.backend.FilterLogs(context.Background(), query)
	if err != nil {
		t.Fatalf("Unable to get logs %v", err)
	}

	if logs[8].Topics[0] != hashutil.Hash([]byte("ChainStart(bytes32,bytes)")) {
		t.Error("Chain start even did not get emitted")
	}
}
