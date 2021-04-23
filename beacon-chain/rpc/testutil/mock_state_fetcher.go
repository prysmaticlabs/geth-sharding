package testutil

import (
	"context"

	iface "github.com/prysmaticlabs/prysm/beacon-chain/state/interface"
)

// MockFetcher is a fake implementation of statefetcher.Fetcher.
type MockFetcher struct {
	BeaconState iface.BeaconState
}

// State --
func (m *MockFetcher) State(context.Context, []byte) (iface.BeaconState, error) {
	return m.BeaconState, nil
}
