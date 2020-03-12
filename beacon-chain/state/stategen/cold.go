package stategen

import (
	"context"

	"github.com/pkg/errors"
	"github.com/prysmaticlabs/prysm/beacon-chain/state"
	"go.opencensus.io/trace"
)

// Given the archive index, this returns the archived cold state in the DB.
// If the archived state does not exist in the state, it'll compute it and save it.
func (s *State) archivedPointByIndex(ctx context.Context, archiveIndex uint64) (*state.BeaconState, error) {
	ctx, span := trace.StartSpan(ctx, "stateGen.loadArchivedPointByIndex")
	defer span.End()
	if s.beaconDB.HasArchivedPoint(ctx, archiveIndex) {
		return s.beaconDB.ArchivedPointState(ctx, archiveIndex)
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
	if err := s.beaconDB.SaveArchivedPointState(ctx, archivedState, archiveIndex); err != nil {
		return nil, err
	}

	return archivedState, nil
}
