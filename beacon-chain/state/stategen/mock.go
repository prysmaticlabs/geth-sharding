package stategen

import (
	"context"

	eth "github.com/prysmaticlabs/ethereumapis/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/beacon-chain/state"
	ethereum_beacon_p2p_v1 "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
)

// MockStateManager is a fake implementation of StateManager.
type MockStateManager struct {
	StatesByRoot map[[32]byte]*state.BeaconState
	StatesBySlot map[uint64]*state.BeaconState
}

// NewMockService --
func NewMockService() *MockStateManager {
	return &MockStateManager{
		StatesByRoot: make(map[[32]byte]*state.BeaconState),
		StatesBySlot: make(map[uint64]*state.BeaconState),
	}
}

// Resume --
func (m *MockStateManager) Resume(ctx context.Context) (*state.BeaconState, error) {
	panic("implement me")
}

// SaveFinalizedState --
func (m *MockStateManager) SaveFinalizedState(fSlot uint64, fRoot [32]byte, fState *state.BeaconState) {
	panic("implement me")
}

// MigrateToCold --
func (m *MockStateManager) MigrateToCold(ctx context.Context, fRoot [32]byte) error {
	panic("implement me")
}

// ReplayBlocks --
func (m *MockStateManager) ReplayBlocks(
	ctx context.Context,
	state *state.BeaconState,
	signed []*eth.SignedBeaconBlock,
	targetSlot uint64,
) (*state.BeaconState, error) {
	panic("implement me")
}

// LoadBlocks --
func (m *MockStateManager) LoadBlocks(
	ctx context.Context,
	startSlot, endSlot uint64,
	endBlockRoot [32]byte,
) ([]*eth.SignedBeaconBlock, error) {
	panic("implement me")
}

// HasState --
func (m *MockStateManager) HasState(ctx context.Context, blockRoot [32]byte) (bool, error) {
	panic("implement me")
}

// HasStateInCache --
func (m *MockStateManager) HasStateInCache(ctx context.Context, blockRoot [32]byte) (bool, error) {
	panic("implement me")
}

// StateByRoot --
func (m *MockStateManager) StateByRoot(ctx context.Context, blockRoot [32]byte) (*state.BeaconState, error) {
	return m.StatesByRoot[blockRoot], nil
}

// StateByRootInitialSync --
func (m *MockStateManager) StateByRootInitialSync(ctx context.Context, blockRoot [32]byte) (*state.BeaconState, error) {
	panic("implement me")
}

func (m *MockStateManager) StateByStateRoot(
	ctx context.Context,
	stateRoot [32]byte,
	headState *state.BeaconState,
) (*state.BeaconState, error) {
	panic("implement me")
}

// StateBySlot --
func (m *MockStateManager) StateBySlot(ctx context.Context, slot uint64) (*state.BeaconState, error) {
	return m.StatesBySlot[slot], nil
}

// RecoverStateSummary --
func (m *MockStateManager) RecoverStateSummary(
	ctx context.Context,
	blockRoot [32]byte,
) (*ethereum_beacon_p2p_v1.StateSummary, error) {
	panic("implement me")
}

// SaveState --
func (m *MockStateManager) SaveState(ctx context.Context, root [32]byte, st *state.BeaconState) error {
	panic("implement me")
}

// ForceCheckpoint --
func (m *MockStateManager) ForceCheckpoint(ctx context.Context, root []byte) error {
	panic("implement me")
}

// EnableSaveHotStateToDB --
func (m *MockStateManager) EnableSaveHotStateToDB(_ context.Context) {
	panic("implement me")
}

// DisableSaveHotStateToDB --
func (m *MockStateManager) DisableSaveHotStateToDB(ctx context.Context) error {
	panic("implement me")
}

// AddStateForRoot --
func (m *MockStateManager) AddStateForRoot(state *state.BeaconState, blockRoot [32]byte) {
	m.StatesByRoot[blockRoot] = state
}

// AddStateForSlot --
func (m *MockStateManager) AddStateForSlot(state *state.BeaconState, slot uint64) {
	m.StatesBySlot[slot] = state
}
