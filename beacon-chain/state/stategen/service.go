package stategen

import (
	"context"
	"sync"

	"github.com/prysmaticlabs/prysm/beacon-chain/cache"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	"github.com/prysmaticlabs/prysm/beacon-chain/db"
	"github.com/prysmaticlabs/prysm/beacon-chain/state"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	"github.com/prysmaticlabs/prysm/shared/params"
	"go.opencensus.io/trace"
)

const archivedInterval = 256

// State represents a management object that handles the internal
// logic of maintaining both hot and cold states in DB.
type State struct {
	beaconDB                db.NoHeadAccessDatabase
	slotsPerArchivedPoint   uint64
	epochBoundarySlotToRoot map[uint64][32]byte
	epochBoundaryLock       sync.RWMutex
	hotStateCache           *cache.HotStateCache
	splitInfo               *splitSlotAndRoot
}

// This tracks the split point. The point where slot and the block root of
// cold and hot sections of the DB splits.
type splitSlotAndRoot struct {
	slot uint64
	root [32]byte
}

// New returns a new state management object.
func New(db db.NoHeadAccessDatabase) *State {
	return &State{
		beaconDB:                db,
		epochBoundarySlotToRoot: make(map[uint64][32]byte),
		hotStateCache:           cache.NewHotStateCache(),
		splitInfo:               &splitSlotAndRoot{slot: 0, root: params.BeaconConfig().ZeroHash},
		slotsPerArchivedPoint:   archivedInterval,
	}
}

// Resume resumes a new state management object from previously saved finalized check point in DB.
func (s *State) Resume(ctx context.Context, lastArchivedRoot [32]byte) (*state.BeaconState, error) {
	ctx, span := trace.StartSpan(ctx, "stateGen.Resume")
	defer span.End()

	lastArchivedState, err := s.beaconDB.LastArchivedIndexState(ctx)
	if err != nil {
		return nil, err
	}
	// Resume as genesis state if there's no last archived state.
	if lastArchivedState == nil {
		return s.beaconDB.GenesisState(ctx)
	}

	s.splitInfo = &splitSlotAndRoot{slot: lastArchivedState.Slot(), root: lastArchivedRoot}

	if err := s.beaconDB.SaveStateSummary(ctx,
		&pb.StateSummary{Slot: lastArchivedState.Slot(), Root: lastArchivedRoot[:], BoundaryRoot: lastArchivedRoot[:]}); err != nil {
		return nil, err
	}

	// In case the finalized state slot was skipped.
	slot := lastArchivedState.Slot()
	if !helpers.IsEpochStart(slot) {
		slot = helpers.StartSlot(helpers.SlotToEpoch(slot) + 1)
	}
	s.setEpochBoundaryRoot(slot, lastArchivedRoot)

	return lastArchivedState, nil
}

// This verifies the archive point frequency is valid. It checks the interval
// is a divisor of the number of slots per epoch. This ensures we have at least one
// archive point within range of our state root history when iterating
// backwards. It also ensures the archive points align with hot state summaries
// which makes it quicker to migrate hot to cold.
func verifySlotsPerArchivePoint(slotsPerArchivePoint uint64) bool {
	return slotsPerArchivePoint > 0 &&
		slotsPerArchivePoint%params.BeaconConfig().SlotsPerEpoch == 0
}
