package kv

import (
	"context"

	"github.com/boltdb/bolt"
	"github.com/gogo/protobuf/proto"
	"github.com/pkg/errors"
	"github.com/prysmaticlabs/go-ssz"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/state"
	"github.com/prysmaticlabs/prysm/beacon-chain/db/filters"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	ethpb "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/shared/params"
	"go.opencensus.io/trace"
)

// State returns the saved state using block's signing root,
// this particular block was used to generate the state.
func (k *Store) State(ctx context.Context, blockRoot [32]byte) (*pb.BeaconState, error) {
	ctx, span := trace.StartSpan(ctx, "BeaconDB.State")
	defer span.End()
	var s *pb.BeaconState
	err := k.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(stateBucket)
		enc := bucket.Get(blockRoot[:])
		if enc == nil {
			return nil
		}

		var err error
		s, err = createState(enc)
		return err
	})
	return s, err
}

// GenerateStateAtSlot generates state from the latest saved slot till the specified slot.
func (k *Store) GenerateStateAtSlot(ctx context.Context, slot uint64) (*pb.BeaconState, error) {
	pBlocks, err := k.savedBlocks(ctx, slot)
	savedRoot, err := ssz.SigningRoot(pBlocks[0])
	if err != nil {
		return nil, errors.Wrap(err, "could not get signing root of block")
	}
	savedState, err := k.State(ctx, savedRoot)
	if err != nil {
		return nil, err
	}

	// run N state transitions to generate state
	for i := 1; i < len(pBlocks); i++ {
		savedState, err = state.ExecuteStateTransitionNoVerify(
			ctx,
			savedState,
			pBlocks[i],
		)
		if err != nil {
			return nil, errors.Wrap(err, "could not execute state transition")
		}
	}

	return savedState, nil
}

func (k *Store) savedBlocks(ctx context.Context, slot uint64) ([]*ethpb.BeaconBlock, error) {
	savingInterval := params.BeaconConfig().SavingInterval
	// Filtering from the slot we know we have a saved state for.
	currentSlot := slot - (slot % savingInterval)
	savedSlot := slot
	var err error
	var pBlocks []*ethpb.BeaconBlock
	for savedSlot == slot {
		// Looping through recursively until we find a state we have saved.
		filter := filters.NewFilter()
		filter.SetStartSlot(currentSlot)
		filter.SetEndSlot(slot)
		pBlocks, err = k.Blocks(ctx, filter)
		if err != nil {
			return nil, errors.Wrap(err, "could not retrieve block")
		}
		if pBlocks[0].Slot < savedSlot {
			savedSlot = pBlocks[0].Slot
		} else if currentSlot != 0 {
			currentSlot = currentSlot - savingInterval
		} else {
			return nil, errors.New("could not find a saved state")
		}
	}
	return pBlocks, nil
}

// PruneSavedStates starts from the passed in previous finalized epoch, and
// deletes the state for all slots until just before the current finalized epoch.
func (k *Store) PruneSavedStates(
	ctx context.Context,
	prevFinalizedEpoch uint64,
	finalizedEpoch uint64,
) error {
	ctx, span := trace.StartSpan(ctx, "BeaconDB.PruneSavedStates")
	defer span.End()

	startSlot := helpers.StartSlot(prevFinalizedEpoch)
	endSlot := helpers.StartSlot(finalizedEpoch) - params.BeaconConfig().SavingInterval
	filter := filters.NewFilter()
	filter.SetStartSlot(startSlot)
	filter.SetEndSlot(endSlot)
	blockRoots, err := k.BlockRoots(ctx, filter)
	if err != nil {
		return errors.Wrap(err, "could not get block roots")
	}

	var root [32]byte
	for i := startSlot; i < endSlot; i += params.BeaconConfig().SavingInterval {
		copy(root[:], blockRoots[i-startSlot])
		if err := k.DeleteState(ctx, root); err != nil {
			return errors.Wrap(err, "could not delete saved state")
		}
	}
	return nil
}

func (k *Store) DeleteState(ctx context.Context, blockRoot [32]byte) error {
	ctx, span := trace.StartSpan(ctx, "BeaconDB.DeleteState")
	defer span.End()
	return k.db.Update(func(tx *bolt.Tx) error {
		bkt := tx.Bucket(stateBucket)
		return bkt.Delete(blockRoot[:])
	})
}

// HeadState returns the latest canonical state in beacon chain.
func (k *Store) HeadState(ctx context.Context) (*pb.BeaconState, error) {
	ctx, span := trace.StartSpan(ctx, "BeaconDB.HeadState")
	defer span.End()
	var s *pb.BeaconState
	err := k.db.View(func(tx *bolt.Tx) error {
		// Retrieve head block's signing root from blocks bucket,
		// to look up what the head state is.
		bucket := tx.Bucket(blocksBucket)
		headBlkRoot := bucket.Get(headBlockRootKey)

		bucket = tx.Bucket(stateBucket)
		enc := bucket.Get(headBlkRoot)
		if enc == nil {
			return nil
		}

		var err error
		s, err = createState(enc)
		return err
	})
	span.AddAttributes(trace.BoolAttribute("exists", s != nil))
	if s != nil {
		span.AddAttributes(trace.Int64Attribute("slot", int64(s.Slot)))
	}
	return s, err
}

// GenesisState returns the genesis state in beacon chain.
func (k *Store) GenesisState(ctx context.Context) (*pb.BeaconState, error) {
	ctx, span := trace.StartSpan(ctx, "BeaconDB.GenesisState")
	defer span.End()
	var s *pb.BeaconState
	err := k.db.View(func(tx *bolt.Tx) error {
		// Retrieve genesis block's signing root from blocks bucket,
		// to look up what the genesis state is.
		bucket := tx.Bucket(blocksBucket)
		genesisBlockRoot := bucket.Get(genesisBlockRootKey)

		bucket = tx.Bucket(stateBucket)
		enc := bucket.Get(genesisBlockRoot)
		if enc == nil {
			return nil
		}

		var err error
		s, err = createState(enc)
		return err
	})
	return s, err
}

// SaveState stores a state to the db using block's signing root which was used to generate the state.
func (k *Store) SaveState(ctx context.Context, state *pb.BeaconState, blockRoot [32]byte) error {
	ctx, span := trace.StartSpan(ctx, "BeaconDB.SaveState")
	defer span.End()
	enc, err := proto.Marshal(state)
	if err != nil {
		return err
	}

	return k.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(stateBucket)
		return bucket.Put(blockRoot[:], enc)
	})
}

// creates state from marshaled proto state bytes.
func createState(enc []byte) (*pb.BeaconState, error) {
	protoState := &pb.BeaconState{}
	err := proto.Unmarshal(enc, protoState)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal encoding")
	}
	return protoState, nil
}
