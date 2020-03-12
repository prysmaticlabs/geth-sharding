package stategen

import (
	"context"

	"github.com/pkg/errors"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	"github.com/prysmaticlabs/prysm/beacon-chain/state"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/prysmaticlabs/prysm/shared/params"
	"go.opencensus.io/trace"
)

// This loads a post finalized beacon state from the hot section of the DB. If necessary it will
// replay blocks starting from the nearest epoch boundary. It returns the beacon state that
// corresponds to the input block root.
func (s *State) loadHotStateByRoot(ctx context.Context, blockRoot [32]byte) (*state.BeaconState, error) {
	ctx, span := trace.StartSpan(ctx, "stateGen.loadHotStateByRoot")
	defer span.End()

	// Load the hot state cache.
	cachedState := s.hotStateCache.Get(blockRoot)
	if cachedState != nil {
		return cachedState, nil
	}

	summary, err := s.beaconDB.StateSummary(ctx, blockRoot)
	if err != nil {
		return nil, err
	}
	if summary == nil {
		return nil, errUnknownStateSummary
	}
	boundaryState, err := s.beaconDB.State(ctx, bytesutil.ToBytes32(summary.BoundaryRoot))
	if err != nil {
		return nil, err
	}
	if boundaryState == nil {
		return nil, errUnknownBoundaryState
	}

	// Don't need to replay the blocks if we're already on an epoch boundary,
	// the target slot is the same as the state slot.
	var hotState *state.BeaconState
	targetSlot := summary.Slot
	if targetSlot == boundaryState.Slot() {
		hotState = boundaryState
	} else {
		blks, err := s.LoadBlocks(ctx, boundaryState.Slot()+1, targetSlot, bytesutil.ToBytes32(summary.Root))
		if err != nil {
			return nil, errors.Wrap(err, "could not load blocks for hot state using root")
		}
		hotState, err = s.ReplayBlocks(ctx, boundaryState, blks, targetSlot)
		if err != nil {
			return nil, errors.Wrap(err, "could not replay blocks for hot state using root")
		}
	}

	// Save the copied state because the reference also returned in the end.
	s.hotStateCache.Put(blockRoot, hotState.Copy())

	return hotState, nil
}

// This loads a hot state by slot where the slot lies between the epoch boundary points.
// This is a slower implementation (versus ByRoot) as slot is the only argument. It require fetching
// all the blocks between the epoch boundary points for playback.
// Use `loadHotStateByRoot` unless you really don't know the root.
func (s *State) loadHotStateBySlot(ctx context.Context, slot uint64) (*state.BeaconState, error) {
	ctx, span := trace.StartSpan(ctx, "stateGen.loadHotStateBySlot")
	defer span.End()

	// Gather epoch boundary information, that is where node starts to replay the blocks.
	boundarySlot := helpers.StartSlot(helpers.SlotToEpoch(slot))
	boundaryRoot, ok := s.epochBoundaryRoot(boundarySlot)
	if !ok {
		return nil, errUnknownBoundaryRoot
	}
	// Try the cache first then try the DB.
	boundaryState := s.hotStateCache.Get(boundaryRoot)
	var err error
	if boundaryState == nil {
		boundaryState, err = s.beaconDB.State(ctx, boundaryRoot)
		if err != nil {
			return nil, err
		}
		if boundaryState == nil {
			return nil, errUnknownBoundaryState
		}
	}

	// Gather the last saved block root and the slot number.
	lastValidRoot, lastValidSlot, err := s.lastSavedBlock(ctx, slot)
	if err != nil {
		return nil, errors.Wrap(err, "could not get last valid block for hot state using slot")
	}

	// Load and replay blocks to get the intermediate state.
	replayBlks, err := s.LoadBlocks(ctx, boundaryState.Slot()+1, lastValidSlot, lastValidRoot)
	if err != nil {
		return nil, err
	}

	return s.ReplayBlocks(ctx, boundaryState, replayBlks, slot)
}

// This loads the epoch boundary root of a given state based on the state slot.
// If the epoch boundary does not have a valid root, it then recovers by going
// back to find the last slot before boundary which has a valid block.
func (s *State) loadEpochBoundaryRoot(ctx context.Context, blockRoot [32]byte, state *state.BeaconState) ([32]byte, error) {
	ctx, span := trace.StartSpan(ctx, "stateGen.loadEpochBoundaryRoot")
	defer span.End()

	boundarySlot := helpers.CurrentEpoch(state) * params.BeaconConfig().SlotsPerEpoch

	// First checks if epoch boundary root already exists in cache.
	r, ok := s.epochBoundarySlotToRoot[boundarySlot]
	if ok {
		return r, nil
	}

	// At epoch boundary, return the root which is just itself.
	if state.Slot() == boundarySlot {
		return blockRoot, nil
	}

	// Node uses genesis getters if the epoch boundary slot is genesis slot.
	if boundarySlot == 0 {
		r, err := s.genesisRoot(ctx)
		if err != nil {
			return [32]byte{}, nil
		}
		s.setEpochBoundaryRoot(boundarySlot, r)
		return r, nil
	}

	// Now to find the epoch boundary root via DB.
	r, _, err := s.lastSavedBlock(ctx, boundarySlot)
	if err != nil {
		return [32]byte{}, errors.Wrap(err, "could not get last saved block for epoch boundary root")
	}

	// Set the epoch boundary root cache.
	s.setEpochBoundaryRoot(boundarySlot, r)

	return r, nil
}
