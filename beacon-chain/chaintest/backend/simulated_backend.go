// Package backend contains utilities for simulating an entire
// ETH 2.0 beacon chain in-memory for e2e tests and benchmarking
// purposes.
package backend

import (
	"context"

	"github.com/prysmaticlabs/prysm/beacon-chain/blockchain"
	"github.com/prysmaticlabs/prysm/beacon-chain/types"
)

// SimulatedBackend allowing for a programmatic advancement
// of an in-memory beacon chain for client test runs
// and other e2e use cases.
type SimulatedBackend struct {
	chainService   *blockchain.ChainService
	db             *simulatedDB
	cState         *types.CrystallizedState
	aState         *types.ActiveState
	testParameters string
}

type simulatedDB struct{}

// NewSimulatedBackend creates an instance by initializing a chain service
// utilizing a mockDB which will act according to test run parameters specified
// in the common ETH 2.0 client test YAML format.
func NewSimulatedBackend() (*SimulatedBackend, error) {
	cs, err := blockchain.NewChainService(context.Background(), &blockchain.Config{
		IncomingBlockBuf:          0,
		EnablePOWChain:            false,
		EnableCrossLinks:          false,
		EnableRewardChecking:      false,
		EnableAttestationValidity: false,
	})
	if err != nil {
		return nil, err
	}
	return &SimulatedBackend{
		chainService: cs,
	}, nil
}

// Commit a new block to the chain directly
// and sets it as the head of the simulated backend.
func (s *SimulatedBackend) Commit() {
	return
}
