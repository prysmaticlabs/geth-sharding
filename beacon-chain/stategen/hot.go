package stategen

import (
	"context"
	"encoding/hex"
	"errors"

	"github.com/prysmaticlabs/go-ssz"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	"github.com/prysmaticlabs/prysm/beacon-chain/db/filters"
	"github.com/prysmaticlabs/prysm/beacon-chain/state"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/sirupsen/logrus"
)

// This saves a post finalized beacon state in the hot section of the DB. On the epoch boundary,
// it saves a full state. On an intermediate slot, it saves a back pointer to the
// nearest epoch boundary state.
func (s *State) saveHotState(ctx context.Context, blockRoot [32]byte, state *state.BeaconState) error {
	// On an epoch boundary, saves the whole state.
	if helpers.IsEpochStart(state.Slot()) {
		if err := s.beaconDB.SaveState(ctx, state, blockRoot); err != nil {
			return err
		}
		log.WithFields(logrus.Fields{
			"slot":      state.Slot(),
			"blockRoot": hex.EncodeToString(bytesutil.Trunc(blockRoot[:]))}).Info("Saved full state on epoch boundary")
		hotStateSaved.Inc()
	}

	// On an intermediate slot, save the state summary.
	epochRoot, err := s.loadEpochBoundaryRoot(ctx, blockRoot, state)
	if err != nil {
		return err
	}
	if err := s.beaconDB.SaveHotStateSummary(ctx, &pb.HotStateSummary{
		Slot:         state.Slot(),
		LatestRoot:   blockRoot[:],
		BoundaryRoot: epochRoot[:],
	}); err != nil {
		return err
	}

	// Store the state in the cache.

	return nil
}

// This loads a post finalized beacon state from the hot section of the DB. If necessary it will
// replay blocks from the nearest epoch boundary.
func (s *State) loadHotState(ctx context.Context, blockRoot [32]byte) (*state.BeaconState, error) {
	// Load the cache

	summary, err := s.beaconDB.HotStateSummary(ctx, blockRoot)
	if err != nil {
		return nil, err
	}
	targetSlot := summary.Slot

	boundaryState, err := s.beaconDB.State(ctx, bytesutil.ToBytes32(summary.BoundaryRoot))
	if err != nil {
		return nil, err
	}
	if boundaryState == nil {
		return nil, errors.New("boundary state can't be nil")
	}

	// Don't need to replay the blocks if we're already on an epoch boundary.
	var hotState *state.BeaconState
	if helpers.IsEpochStart(targetSlot) {
		hotState = boundaryState
	} else {
		blks, err := s.LoadBlocks(ctx, boundaryState.Slot()+1, targetSlot, bytesutil.ToBytes32(summary.LatestRoot))
		if err != nil {
			return nil, err
		}
		hotState, err = s.ReplayBlocks(ctx, boundaryState, blks, targetSlot)
		if err != nil {
			return nil, err
		}
	}

	// Save the cache

	log.WithFields(logrus.Fields{
		"slot":      hotState.Slot(),
		"blockRoot": hex.EncodeToString(bytesutil.Trunc(blockRoot[:]))}).Info("Loaded hot state")

	return hotState, nil
}

// This loads the epoch boundary root of a given state based on the state slot.
func (s *State) loadEpochBoundaryRoot(ctx context.Context, blockRoot [32]byte, state *state.BeaconState) ([32]byte, error) {
	epochBoundarySlot := helpers.CurrentEpoch(state) * params.BeaconConfig().SlotsPerEpoch

	// First check if epoch boundary root already exists in cache.
	r, ok := s.epochBoundarySlotToRoot[epochBoundarySlot]
	if ok {
		return r, nil
	}

	// At epoch boundary, the root is just itself.
	if state.Slot() == epochBoundarySlot {
		return blockRoot, nil
	}

	// Use genesis getters if the epoch boundary slot is on genesis slot.
	if epochBoundarySlot == 0 {
		b, err := s.beaconDB.GenesisBlock(ctx)
		if err != nil {
			return [32]byte{}, err
		}

		r, err = ssz.HashTreeRoot(b.Block)
		if err != nil {
			return [32]byte{}, err
		}

		s.setEpochBoundaryRoot(epochBoundarySlot, r)

		return r, nil
	}

	filter := filters.NewFilter().SetStartSlot(epochBoundarySlot).SetEndSlot(epochBoundarySlot)
	rs, err := s.beaconDB.BlockRoots(ctx, filter)
	if err != nil {
		return [32]byte{}, err
	}
	// If the epoch boundary is a skip slot, traverse back to find the last valid state.
	if len(rs) == 0 {
		r, err = s.handleLastValidState(ctx, epochBoundarySlot)
		if err != nil {
			return [32]byte{}, err
		}
	} else {
		r = rs[0]
	}

	s.setEpochBoundaryRoot(epochBoundarySlot, r)

	return r, nil
}

// This finds the last valid state from searching backwards starting at epoch boundary slot
// and returns the root of the state.
func (s *State) handleLastValidState(ctx context.Context, epochBoundarySlot uint64) ([32]byte, error) {
	filter := filters.NewFilter().SetStartSlot(0).SetEndSlot(epochBoundarySlot)
	rs, err := s.beaconDB.BlockRoots(ctx, filter)
	if err != nil {
		return [32]byte{}, err
	}
	lastRoot := rs[len(rs)-1]

	r, _ := s.epochBoundaryRoot(epochBoundarySlot - params.BeaconConfig().SlotsPerEpoch)
	startState, err := s.beaconDB.State(ctx, r)
	if err != nil {
		return [32]byte{}, err
	}

	blks, err := s.LoadBlocks(ctx, startState.Slot()+1, epochBoundarySlot, lastRoot)
	if err != nil {
		return [32]byte{}, err
	}
	startState, err = s.ReplayBlocks(ctx, startState, blks, epochBoundarySlot)
	if err != nil {
		return [32]byte{}, err
	}

	if err := s.beaconDB.SaveState(ctx, startState, lastRoot); err != nil {
		return [32]byte{}, err
	}

	return lastRoot, nil
}
