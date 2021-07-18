package stategen

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	types "github.com/prysmaticlabs/eth2-types"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/altair"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	transition "github.com/prysmaticlabs/prysm/beacon-chain/core/state"
	"github.com/prysmaticlabs/prysm/beacon-chain/db/filters"
	iface "github.com/prysmaticlabs/prysm/beacon-chain/state/interface"
	"github.com/prysmaticlabs/prysm/proto/interfaces"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/version"
	"go.opencensus.io/trace"
)

// ReplayBlocks replays the input blocks on the input state until the target slot is reached.
func (s *State) ReplayBlocks(
	ctx context.Context,
	state iface.BeaconState,
	signed []interfaces.SignedBeaconBlock,
	targetSlot types.Slot,
) (iface.BeaconState, error) {
	ctx, span := trace.StartSpan(ctx, "stateGen.ReplayBlocks")
	defer span.End()
	var err error
	// The input block list is sorted in decreasing slots order.
	if len(signed) > 0 {
		for i := len(signed) - 1; i >= 0; i-- {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			if state.Slot() >= targetSlot {
				break
			}
			// A node shouldn't process the block if the block slot is lower than the state slot.
			if state.Slot() >= signed[i].Block().Slot() {
				continue
			}
			state, err = executeStateTransitionStateGen(ctx, state, signed[i])
			if err != nil {
				return nil, err
			}
		}
	}

	// If there is skip slots at the end.
	if targetSlot > state.Slot() {
		state, err = processSlotsStateGen(ctx, state, targetSlot)
		if err != nil {
			return nil, err
		}
	}

	return state, nil
}

// LoadBlocks loads the blocks between start slot and end slot by recursively fetching from end block root.
// The Blocks are returned in slot-descending order.
func (s *State) LoadBlocks(ctx context.Context, startSlot, endSlot types.Slot, endBlockRoot [32]byte) ([]interfaces.SignedBeaconBlock, error) {
	// Nothing to load for invalid range.
	if endSlot < startSlot {
		return nil, fmt.Errorf("start slot %d >= end slot %d", startSlot, endSlot)
	}
	filter := filters.NewFilter().SetStartSlot(startSlot).SetEndSlot(endSlot)
	blocks, blockRoots, err := s.beaconDB.Blocks(ctx, filter)
	if err != nil {
		return nil, err
	}
	// The retrieved blocks and block roots have to be in the same length given same filter.
	if len(blocks) != len(blockRoots) {
		return nil, errors.New("length of blocks and roots don't match")
	}
	// Return early if there's no block given the input.
	length := len(blocks)
	if length == 0 {
		return nil, nil
	}

	// The last retrieved block root has to match input end block root.
	// Covers the edge case if there's multiple blocks on the same end slot,
	// the end root may not be the last index in `blockRoots`.
	for length >= 3 && blocks[length-1].Block().Slot() == blocks[length-2].Block().Slot() && blockRoots[length-1] != endBlockRoot {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		length--
		if blockRoots[length-2] == endBlockRoot {
			length--
			break
		}
	}

	if blockRoots[length-1] != endBlockRoot {
		return nil, errors.New("end block roots don't match")
	}

	filteredBlocks := []interfaces.SignedBeaconBlock{blocks[length-1]}
	// Starting from second to last index because the last block is already in the filtered block list.
	for i := length - 2; i >= 0; i-- {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		b := filteredBlocks[len(filteredBlocks)-1]
		if bytesutil.ToBytes32(b.Block().ParentRoot()) != blockRoots[i] {
			continue
		}
		filteredBlocks = append(filteredBlocks, blocks[i])
	}

	return filteredBlocks, nil
}

// executeStateTransitionStateGen applies state transition on input historical state and block for state gen usages.
// There's no signature verification involved given state gen only works with stored block and state in DB.
// If the objects are already in stored in DB, one can omit redundant signature checks and ssz hashing calculations.
// WARNING: This method should not be used on an unverified new block.
func executeStateTransitionStateGen(
	ctx context.Context,
	state iface.BeaconState,
	signed interfaces.SignedBeaconBlock,
) (iface.BeaconState, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if signed == nil || signed.IsNil() || signed.Block().IsNil() {
		return nil, errUnknownBlock
	}

	ctx, span := trace.StartSpan(ctx, "stategen.ExecuteStateTransitionStateGen")
	defer span.End()
	var err error

	// Execute per slots transition.
	// Given this is for state gen, a node uses the version process slots without skip slots cache.
	state, err = processSlotsStateGen(ctx, state, signed.Block().Slot())
	if err != nil {
		return nil, errors.Wrap(err, "could not process slot")
	}

	// Execute per block transition.
	// Given this is for state gen, a node only cares about the post state without proposer
	// and randao signature verifications.
	state, err = transition.ProcessBlockForStateRoot(ctx, state, signed)
	if err != nil {
		return nil, errors.Wrap(err, "could not process block")
	}
	if signed.Version() == version.Altair {
		sa, err := signed.Block().Body().SyncAggregate()
		if err != nil {
			return nil, err
		}
		state, err = altair.ProcessSyncAggregate(state, sa)
		if err != nil {
			return nil, err
		}
	}

	return state, nil
}

// processSlotsStateGen to process old slots for state gen usages.
// There's no skip slot cache involved given state gen only works with already stored block and state in DB.
// WARNING: This method should not be used for future slot.
func processSlotsStateGen(ctx context.Context, state iface.BeaconState, slot types.Slot) (iface.BeaconState, error) {
	ctx, span := trace.StartSpan(ctx, "stategen.ProcessSlotsStateGen")
	defer span.End()
	if state == nil || state.IsNil() {
		return nil, errUnknownState
	}

	if state.Slot() > slot {
		err := fmt.Errorf("expected state.slot %d < slot %d", state.Slot(), slot)
		return nil, err
	}

	if state.Slot() == slot {
		return state, nil
	}

	var err error
	for state.Slot() < slot {
		state, err = transition.ProcessSlot(ctx, state)
		if err != nil {
			return nil, errors.Wrap(err, "could not process slot")
		}
		if transition.CanProcessEpoch(state) {
			switch state.Version() {
			case version.Phase0:
				state, err = transition.ProcessEpochPrecompute(ctx, state)
				if err != nil {
					return nil, errors.Wrap(err, "could not process epoch with optimizations")
				}
			case version.Altair:
				state, err = altair.ProcessEpoch(ctx, state)
				if err != nil {
					return nil, errors.Wrap(err, "could not process epoch with optimization")
				}
			default:
				return nil, errors.New("beacon state should have a version")
			}
		}
		if err := state.SetSlot(state.Slot() + 1); err != nil {
			return nil, err
		}
		if helpers.IsEpochStart(state.Slot()) && helpers.SlotToEpoch(state.Slot()) == params.BeaconConfig().AltairForkEpoch {
			state, err = altair.UpgradeToAltair(state)
			if err != nil {
				return nil, err
			}
		}
	}

	return state, nil
}

// This finds the last saved block in DB from searching backwards from input slot,
// it returns the block root and the slot of the block.
// This is used by both hot and cold state management.
func (s *State) lastSavedBlock(ctx context.Context, slot types.Slot) ([32]byte, types.Slot, error) {
	ctx, span := trace.StartSpan(ctx, "stateGen.lastSavedBlock")
	defer span.End()

	// Handle the genesis case where the input slot is 0.
	if slot == 0 {
		gRoot, err := s.genesisRoot(ctx)
		if err != nil {
			return [32]byte{}, 0, err
		}
		return gRoot, 0, nil
	}

	lastSaved, err := s.beaconDB.HighestSlotBlocksBelow(ctx, slot+1)
	if err != nil {
		return [32]byte{}, 0, err
	}

	// Given this is used to query canonical block. There should only be one saved canonical block of a given slot.
	if len(lastSaved) != 1 {
		return [32]byte{}, 0, fmt.Errorf("highest saved block does not equal to 1, it equals to %d", len(lastSaved))
	}
	if lastSaved[0] == nil || lastSaved[0].IsNil() || lastSaved[0].Block().IsNil() {
		return [32]byte{}, 0, nil
	}
	r, err := lastSaved[0].Block().HashTreeRoot()
	if err != nil {
		return [32]byte{}, 0, err
	}

	return r, lastSaved[0].Block().Slot(), nil
}

// This finds the last saved state in DB from searching backwards from input slot,
// it returns the block root of the block which was used to produce the state.
// This is used by both hot and cold state management.
func (s *State) lastSavedState(ctx context.Context, slot types.Slot) (iface.ReadOnlyBeaconState, error) {
	ctx, span := trace.StartSpan(ctx, "stateGen.lastSavedState")
	defer span.End()

	// Handle the genesis case where the input slot is 0.
	if slot == 0 {
		return s.beaconDB.GenesisState(ctx)
	}

	lastSaved, err := s.beaconDB.HighestSlotStatesBelow(ctx, slot+1)
	if err != nil {
		return nil, err
	}

	// Given this is used to query canonical state. There should only be one saved canonical block of a given slot.
	if len(lastSaved) != 1 {
		return nil, fmt.Errorf("highest saved state does not equal to 1, it equals to %d", len(lastSaved))
	}
	if lastSaved[0] == nil {
		return nil, errUnknownState
	}

	return lastSaved[0], nil
}

// This returns the genesis root.
func (s *State) genesisRoot(ctx context.Context) ([32]byte, error) {
	b, err := s.beaconDB.GenesisBlock(ctx)
	if err != nil {
		return [32]byte{}, err
	}
	return b.Block().HashTreeRoot()
}

// Given the start slot and the end slot, this returns the finalized beacon blocks in between.
// Since hot states don't have finalized blocks, this should ONLY be used for replaying cold state.
func (s *State) loadFinalizedBlocks(ctx context.Context, startSlot, endSlot types.Slot) ([]interfaces.SignedBeaconBlock, error) {
	f := filters.NewFilter().SetStartSlot(startSlot).SetEndSlot(endSlot)
	bs, bRoots, err := s.beaconDB.Blocks(ctx, f)
	if err != nil {
		return nil, err
	}
	if len(bs) != len(bRoots) {
		return nil, errors.New("length of blocks and roots don't match")
	}
	fbs := make([]interfaces.SignedBeaconBlock, 0, len(bs))
	for i := len(bs) - 1; i >= 0; i-- {
		if s.beaconDB.IsFinalizedBlock(ctx, bRoots[i]) {
			fbs = append(fbs, bs[i])
		}
	}
	return fbs, nil
}
