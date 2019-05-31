package testutils

import (
	"crypto/ecdsa"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/accounts/abi/bind/backends"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/crypto"
	contract "github.com/prysmaticlabs/prysm/contracts/deposit-contract"
)

var (
	amount33Eth        = "33000000000000000000"
	amount32Eth        = "32000000000000000000"
	amountLessThan1Eth = "500000000000000000"
)

type testAccount struct {
	Addr         common.Address
	Contract     *contract.DepositContract
	ContractAddr common.Address
	Backend      *backends.SimulatedBackend
	TxOpts       *bind.TransactOpts
}

// Setup creates the simulated backedn with the deposit contract deployed
func Setup() (*testAccount, error) {
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

	depositsRequired := big.NewInt(8)
	minDeposit := big.NewInt(1e9)
	maxDeposit := big.NewInt(32e9)
	contractAddr, _, contract, err := contract.DeployDepositContract(txOpts, backend, depositsRequired, minDeposit, maxDeposit, big.NewInt(1), addr)
	if err != nil {
		return nil, err
	}
	backend.Commit()

	return &testAccount{addr, contract, contractAddr, backend, txOpts}, nil
}

// Amount32Eth returns 32Eth(in wei) in terms of the big.Int type.
func Amount32Eth() *big.Int {
	amount, _ := new(big.Int).SetString(amount32Eth, 10)
	return amount
}

// Amount33Eth returns 33Eth(in wei) in terms of the big.Int type.
func Amount33Eth() *big.Int {
	amount, _ := new(big.Int).SetString(amount33Eth, 10)
	return amount
}

// LessThan1Eth returns less than 1 Eth(in wei) in terms of the big.Int type.
func LessThan1Eth() *big.Int {
	amount, _ := new(big.Int).SetString(amountLessThan1Eth, 10)
	return amount
}
