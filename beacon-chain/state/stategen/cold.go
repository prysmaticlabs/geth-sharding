package stategen

import (
	"context"
	"encoding/hex"

	"github.com/pkg/errors"
	"github.com/prysmaticlabs/prysm/beacon-chain/state"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/sirupsen/logrus"
	"go.opencensus.io/trace"
)

// This saves a pre finalized beacon state in the cold section of the DB. The returns an error
// and not store anything if the state does not lie on an archive point boundary.
func (s *State) saveColdState(ctx context.Context, blockRoot [32]byte, state *state.BeaconState) error {
	ctx, span := trace.StartSpan(ctx, "stateGen.saveColdState")
	defer span.End()

	if state.Slot()%s.slotsPerArchivedPoint != 0 {
		return errSlotNonArchivedPoint
	}

	if err := s.beaconDB.SaveState(ctx, state, blockRoot); err != nil {
		return err
	}
	archivedIndex := state.Slot() / s.slotsPerArchivedPoint
	if err := s.beaconDB.SaveArchivedPointRoot(ctx, blockRoot, archivedIndex); err != nil {
		return err
	}

	log.WithFields(logrus.Fields{
		"slot":      state.Slot(),
		"blockRoot": hex.EncodeToString(bytesutil.Trunc(blockRoot[:]))}).Info("Saved full state on archived point")

	return nil
}

// This loads the cold state by block root, it decides whether to load from archived point (faster) or
// somewhere between archived points (slower) because it requires replaying blocks.
// This method is more efficient than load cold state by slot.
func (s *State) loadColdStateByRoot(ctx context.Context, blockRoot [32]byte) (*state.BeaconState, error) {
	ctx, span := trace.StartSpan(ctx, "stateGen.loadColdStateByRoot")
	defer span.End()

	summary, err := s.stateSummary(ctx, blockRoot)
	if err != nil {
		return nil, errors.Wrap(err, "could not get state summary")
	}

	// Use the archived point state if the summary slot lies on top of the archived point.
	if summary.Slot%s.slotsPerArchivedPoint == 0 {
		archivedPoint := summary.Slot / s.slotsPerArchivedPoint
		s, err := s.loadColdStateByArchivedPoint(ctx, archivedPoint)
		if err != nil {
			return nil, errors.Wrap(err, "could not get cold state using archived index")
		}
		if s == nil {
			return nil, errUnknownArchivedState
		}
		return s, nil
	}

	return s.loadColdIntermediateStateByRoot(ctx, summary.Slot, blockRoot)
}

// This loads the cold state for the input archived point.
func (s *State) loadColdStateByArchivedPoint(ctx context.Context, archivedPoint uint64) (*state.BeaconState, error) {
	states, err := s.beaconDB.HighestSlotStatesBelow(ctx, archivedPoint*s.slotsPerArchivedPoint+1)
	if err != nil {
		return nil, err
	}
	return states[0], nil
}

// This loads a cold state by slot and block root combinations.
// This is a faster implementation than by slot given the input block root is provided.
func (s *State) loadColdIntermediateStateByRoot(ctx context.Context, slot uint64, blockRoot [32]byte) (*state.BeaconState, error) {
	ctx, span := trace.StartSpan(ctx, "stateGen.loadColdIntermediateStateByRoot")
	defer span.End()

	// Load the archive point for lower side of the intermediate state.
	lowArchivedPointIdx := slot / s.slotsPerArchivedPoint
	lowArchivedPointState, err := s.archivedPointByIndex(ctx, lowArchivedPointIdx)
	if err != nil {
		return nil, errors.Wrap(err, "could not get lower archived state using index")
	}
	if lowArchivedPointState == nil {
		return nil, errUnknownArchivedState
	}

	replayBlks, err := s.LoadBlocks(ctx, lowArchivedPointState.Slot()+1, slot, blockRoot)
	if err != nil {
		return nil, errors.Wrap(err, "could not get load blocks for cold state using slot")
	}

	return s.ReplayBlocks(ctx, lowArchivedPointState, replayBlks, slot)
}

// This loads a cold state by slot where the slot lies between the archived point.
// This is a slower implementation given there's no root and slot is the only argument. It requires fetching
// all the blocks between the archival points.
func (s *State) loadColdIntermediateStateBySlot(ctx context.Context, slot uint64) (*state.BeaconState, error) {
	ctx, span := trace.StartSpan(ctx, "stateGen.loadColdIntermediateStateBySlot")
	defer span.End()

	// Load the archive point for lower and high side of the intermediate state.
	lowArchivedPointIdx := slot / s.slotsPerArchivedPoint
	highArchivedPointIdx := lowArchivedPointIdx + 1

	lowArchivedPointState, err := s.archivedPointByIndex(ctx, lowArchivedPointIdx)
	if err != nil {
		return nil, errors.Wrap(err, "could not get lower bound archived state using index")
	}
	if lowArchivedPointState == nil {
		return nil, errUnknownArchivedState
	}

	// If the slot of the high archived point lies outside of the split slot, use the split slot and root
	// for the upper archived point.
	var highArchivedPointRoot [32]byte
	highArchivedPointSlot := highArchivedPointIdx * s.slotsPerArchivedPoint
	if highArchivedPointSlot >= s.splitInfo.slot {
		highArchivedPointRoot = s.splitInfo.root
		highArchivedPointSlot = s.splitInfo.slot
	} else {
		if _, err := s.archivedPointByIndex(ctx, highArchivedPointIdx); err != nil {
			return nil, errors.Wrap(err, "could not get upper bound archived state using index")
		}
		highArchivedPointRoot = s.beaconDB.ArchivedPointRoot(ctx, highArchivedPointIdx)
		slot, err := s.blockRootSlot(ctx, highArchivedPointRoot)
		if err != nil {
			return nil, errors.Wrap(err, "could not get high archived point slot")
		}
		highArchivedPointSlot = slot
	}

	replayBlks, err := s.LoadBlocks(ctx, lowArchivedPointState.Slot()+1, highArchivedPointSlot, highArchivedPointRoot)
	if err != nil {
		return nil, errors.Wrap(err, "could not load block for cold state using slot")
	}

	return s.ReplayBlocks(ctx, lowArchivedPointState, replayBlks, slot)
}

// Given the archive index, this returns the archived cold state in the DB.
// If the archived state does not exist in the state, it'll compute it and save it.
func (s *State) archivedPointByIndex(ctx context.Context, archiveIndex uint64) (*state.BeaconState, error) {
	ctx, span := trace.StartSpan(ctx, "stateGen.loadArchivedPointByIndex")
	defer span.End()
	if s.beaconDB.HasArchivedPoint(ctx, archiveIndex) {
		return s.loadColdStateByArchivedPoint(ctx, archiveIndex)
	}

	// If for certain reasons, archived point does not exist in DB,
	// a node should regenerate it and save it.
	return s.recoverArchivedPointByIndex(ctx, archiveIndex)
}

// This recovers an archived point by index. For certain reasons (ex. user toggles feature flag),
// an archived point may not be present in the DB. This regenerates the archived point state via
// playback and saves the archived root/state to the DB.
func (s *State) recoverArchivedPointByIndex(ctx context.Context, archiveIndex uint64) (*state.BeaconState, error) {
	ctx, span := trace.StartSpan(ctx, "stateGen.recoverArchivedPointByIndex")
	defer span.End()

	archivedSlot := archiveIndex * s.slotsPerArchivedPoint
	archivedState, err := s.ComputeStateUpToSlot(ctx, archivedSlot)
	if err != nil {
		return nil, errors.Wrap(err, "could not compute state up to archived index slot")
	}
	if archivedState == nil {
		return nil, errUnknownArchivedState
	}
	lastRoot, _, err := s.lastSavedBlock(ctx, archivedSlot)
	if err != nil {
		return nil, errors.Wrap(err, "could not get last valid block up to archived index slot")
	}

	if err := s.beaconDB.SaveArchivedPointRoot(ctx, lastRoot, archiveIndex); err != nil {
		return nil, err
	}

	return archivedState, nil
}

// Given a block root, this returns the slot of the block root using state summary look up in DB.
// If state summary does not exist in DB, it will recover the state summary and save it to the DB.
// This is used to cover corner cases where users toggle new state service's feature flag.
func (s *State) blockRootSlot(ctx context.Context, blockRoot [32]byte) (uint64, error) {
	ctx, span := trace.StartSpan(ctx, "stateGen.blockRootSlot")
	defer span.End()

	if s.StateSummaryExists(ctx, blockRoot) {
		summary, err := s.stateSummary(ctx, blockRoot)
		if err != nil {
			return 0, err
		}
		return summary.Slot, nil
	}

	// Couldn't find state summary in DB. Retry with block bucket to get block slot.
	b, err := s.beaconDB.Block(ctx, blockRoot)
	if err != nil {
		return 0, err
	}
	if b == nil || b.Block == nil {
		return 0, errUnknownBlock
	}
	if err := s.beaconDB.SaveStateSummary(ctx, &pb.StateSummary{Root: blockRoot[:], Slot: b.Block.Slot}); err != nil {
		return 0, errors.Wrap(err, "could not save state summary")
	}

	return b.Block.Slot, nil
}
