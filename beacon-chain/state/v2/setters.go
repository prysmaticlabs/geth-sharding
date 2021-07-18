package v2

import (
	"fmt"

	"github.com/pkg/errors"
	types "github.com/prysmaticlabs/eth2-types"
	"github.com/prysmaticlabs/go-bitfield"
	"github.com/prysmaticlabs/prysm/beacon-chain/state/stateutil"
	pbp2p "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	ethpb "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/prysmaticlabs/prysm/shared/copyutil"
	"github.com/prysmaticlabs/prysm/shared/hashutil"
	"google.golang.org/protobuf/proto"
)

// For our setters, we have a field reference counter through
// which we can track shared field references. This helps when
// performing state copies, as we simply copy the reference to the
// field. When we do need to do need to modify these fields, we
// perform a full copy of the field. This is true of most of our
// fields except for the following below.
// 1) BlockRoots
// 2) StateRoots
// 3) Eth1DataVotes
// 4) RandaoMixes
// 5) HistoricalRoots
// 6) CurrentParticipationBits
// 7) PreviousParticipationBits
//
// The fields referred to above are instead copied by reference, where
// we simply copy the reference to the underlying object instead of the
// whole object. This is possible due to how we have structured our state
// as we copy the value on read, so as to ensure the underlying object is
// not mutated while it is being accessed during a state read.

// SetGenesisTime for the beacon state.
func (b *BeaconState) SetGenesisTime(val uint64) error {
	b.lock.Lock()
	defer b.lock.Unlock()

	b.state.GenesisTime = val
	b.markFieldAsDirty(genesisTime)
	return nil
}

// SetGenesisValidatorRoot for the beacon state.
func (b *BeaconState) SetGenesisValidatorRoot(val []byte) error {
	b.lock.Lock()
	defer b.lock.Unlock()

	b.state.GenesisValidatorsRoot = val
	b.markFieldAsDirty(genesisValidatorRoot)
	return nil
}

// SetSlot for the beacon state.
func (b *BeaconState) SetSlot(val types.Slot) error {
	if !b.hasInnerState() {
		return ErrNilInnerState
	}
	b.lock.Lock()
	defer b.lock.Unlock()

	b.state.Slot = val
	b.markFieldAsDirty(slot)
	return nil
}

// SetFork version for the beacon chain.
func (b *BeaconState) SetFork(val *pbp2p.Fork) error {
	if !b.hasInnerState() {
		return ErrNilInnerState
	}
	b.lock.Lock()
	defer b.lock.Unlock()

	fk, ok := proto.Clone(val).(*pbp2p.Fork)
	if !ok {
		return errors.New("proto.Clone did not return a fork proto")
	}
	b.state.Fork = fk
	b.markFieldAsDirty(fork)
	return nil
}

// SetLatestBlockHeader in the beacon state.
func (b *BeaconState) SetLatestBlockHeader(val *ethpb.BeaconBlockHeader) error {
	if !b.hasInnerState() {
		return ErrNilInnerState
	}
	b.lock.Lock()
	defer b.lock.Unlock()

	b.state.LatestBlockHeader = copyutil.CopyBeaconBlockHeader(val)
	b.markFieldAsDirty(latestBlockHeader)
	return nil
}

// SetBlockRoots for the beacon state. Updates the entire
// list to a new value by overwriting the previous one.
func (b *BeaconState) SetBlockRoots(val [][]byte) error {
	if !b.hasInnerState() {
		return ErrNilInnerState
	}
	b.lock.Lock()
	defer b.lock.Unlock()

	b.sharedFieldReferences[blockRoots].MinusRef()
	b.sharedFieldReferences[blockRoots] = stateutil.NewRef(1)

	b.state.BlockRoots = val
	b.markFieldAsDirty(blockRoots)
	b.rebuildTrie[blockRoots] = true
	return nil
}

// UpdateBlockRootAtIndex for the beacon state. Updates the block root
// at a specific index to a new value.
func (b *BeaconState) UpdateBlockRootAtIndex(idx uint64, blockRoot [32]byte) error {
	if !b.hasInnerState() {
		return ErrNilInnerState
	}
	if uint64(len(b.state.BlockRoots)) <= idx {
		return fmt.Errorf("invalid index provided %d", idx)
	}
	b.lock.Lock()
	defer b.lock.Unlock()

	r := b.state.BlockRoots
	if ref := b.sharedFieldReferences[blockRoots]; ref.Refs() > 1 {
		// Copy elements in underlying array by reference.
		r = make([][]byte, len(b.state.BlockRoots))
		copy(r, b.state.BlockRoots)
		ref.MinusRef()
		b.sharedFieldReferences[blockRoots] = stateutil.NewRef(1)
	}

	r[idx] = blockRoot[:]
	b.state.BlockRoots = r

	b.markFieldAsDirty(blockRoots)
	b.addDirtyIndices(blockRoots, []uint64{idx})
	return nil
}

// SetStateRoots for the beacon state. Updates the state roots
// to a new value by overwriting the previous value.
func (b *BeaconState) SetStateRoots(val [][]byte) error {
	if !b.hasInnerState() {
		return ErrNilInnerState
	}
	b.lock.Lock()
	defer b.lock.Unlock()

	b.sharedFieldReferences[stateRoots].MinusRef()
	b.sharedFieldReferences[stateRoots] = stateutil.NewRef(1)

	b.state.StateRoots = val
	b.markFieldAsDirty(stateRoots)
	b.rebuildTrie[stateRoots] = true
	return nil
}

// UpdateStateRootAtIndex for the beacon state. Updates the state root
// at a specific index to a new value.
func (b *BeaconState) UpdateStateRootAtIndex(idx uint64, stateRoot [32]byte) error {
	if !b.hasInnerState() {
		return ErrNilInnerState
	}

	b.lock.RLock()
	if uint64(len(b.state.StateRoots)) <= idx {
		b.lock.RUnlock()
		return errors.Errorf("invalid index provided %d", idx)
	}
	b.lock.RUnlock()

	b.lock.Lock()
	defer b.lock.Unlock()

	// Check if we hold the only reference to the shared state roots slice.
	r := b.state.StateRoots
	if ref := b.sharedFieldReferences[stateRoots]; ref.Refs() > 1 {
		// Copy elements in underlying array by reference.
		r = make([][]byte, len(b.state.StateRoots))
		copy(r, b.state.StateRoots)
		ref.MinusRef()
		b.sharedFieldReferences[stateRoots] = stateutil.NewRef(1)
	}

	r[idx] = stateRoot[:]
	b.state.StateRoots = r

	b.markFieldAsDirty(stateRoots)
	b.addDirtyIndices(stateRoots, []uint64{idx})
	return nil
}

// SetHistoricalRoots for the beacon state. Updates the entire
// list to a new value by overwriting the previous one.
func (b *BeaconState) SetHistoricalRoots(val [][]byte) error {
	if !b.hasInnerState() {
		return ErrNilInnerState
	}
	b.lock.Lock()
	defer b.lock.Unlock()

	b.sharedFieldReferences[historicalRoots].MinusRef()
	b.sharedFieldReferences[historicalRoots] = stateutil.NewRef(1)

	b.state.HistoricalRoots = val
	b.markFieldAsDirty(historicalRoots)
	return nil
}

// SetEth1Data for the beacon state.
func (b *BeaconState) SetEth1Data(val *ethpb.Eth1Data) error {
	if !b.hasInnerState() {
		return ErrNilInnerState
	}
	b.lock.Lock()
	defer b.lock.Unlock()

	b.state.Eth1Data = val
	b.markFieldAsDirty(eth1Data)
	return nil
}

// SetEth1DataVotes for the beacon state. Updates the entire
// list to a new value by overwriting the previous one.
func (b *BeaconState) SetEth1DataVotes(val []*ethpb.Eth1Data) error {
	if !b.hasInnerState() {
		return ErrNilInnerState
	}
	b.lock.Lock()
	defer b.lock.Unlock()

	b.sharedFieldReferences[eth1DataVotes].MinusRef()
	b.sharedFieldReferences[eth1DataVotes] = stateutil.NewRef(1)

	b.state.Eth1DataVotes = val
	b.markFieldAsDirty(eth1DataVotes)
	b.rebuildTrie[eth1DataVotes] = true
	return nil
}

// AppendEth1DataVotes for the beacon state. Appends the new value
// to the the end of list.
func (b *BeaconState) AppendEth1DataVotes(val *ethpb.Eth1Data) error {
	if !b.hasInnerState() {
		return ErrNilInnerState
	}
	b.lock.Lock()
	defer b.lock.Unlock()

	votes := b.state.Eth1DataVotes
	if b.sharedFieldReferences[eth1DataVotes].Refs() > 1 {
		// Copy elements in underlying array by reference.
		votes = make([]*ethpb.Eth1Data, len(b.state.Eth1DataVotes))
		copy(votes, b.state.Eth1DataVotes)
		b.sharedFieldReferences[eth1DataVotes].MinusRef()
		b.sharedFieldReferences[eth1DataVotes] = stateutil.NewRef(1)
	}

	b.state.Eth1DataVotes = append(votes, val)
	b.markFieldAsDirty(eth1DataVotes)
	b.addDirtyIndices(eth1DataVotes, []uint64{uint64(len(b.state.Eth1DataVotes) - 1)})
	return nil
}

// SetEth1DepositIndex for the beacon state.
func (b *BeaconState) SetEth1DepositIndex(val uint64) error {
	if !b.hasInnerState() {
		return ErrNilInnerState
	}
	b.lock.Lock()
	defer b.lock.Unlock()

	b.state.Eth1DepositIndex = val
	b.markFieldAsDirty(eth1DepositIndex)
	return nil
}

// SetValidators for the beacon state. Updates the entire
// to a new value by overwriting the previous one.
func (b *BeaconState) SetValidators(val []*ethpb.Validator) error {
	if !b.hasInnerState() {
		return ErrNilInnerState
	}
	b.lock.Lock()
	defer b.lock.Unlock()

	b.state.Validators = val
	b.sharedFieldReferences[validators].MinusRef()
	b.sharedFieldReferences[validators] = stateutil.NewRef(1)
	b.markFieldAsDirty(validators)
	b.rebuildTrie[validators] = true
	b.valMapHandler = stateutil.NewValMapHandler(b.state.Validators)
	return nil
}

// ApplyToEveryValidator applies the provided callback function to each validator in the
// validator registry.
func (b *BeaconState) ApplyToEveryValidator(f func(idx int, val *ethpb.Validator) (bool, *ethpb.Validator, error)) error {
	if !b.hasInnerState() {
		return ErrNilInnerState
	}
	b.lock.Lock()
	v := b.state.Validators
	if ref := b.sharedFieldReferences[validators]; ref.Refs() > 1 {
		v = b.validatorsReferences()
		ref.MinusRef()
		b.sharedFieldReferences[validators] = stateutil.NewRef(1)
	}
	b.lock.Unlock()
	var changedVals []uint64
	for i, val := range v {
		changed, newVal, err := f(i, val)
		if err != nil {
			return err
		}
		if changed {
			changedVals = append(changedVals, uint64(i))
			v[i] = newVal
		}
	}

	b.lock.Lock()
	defer b.lock.Unlock()

	b.state.Validators = v
	b.markFieldAsDirty(validators)
	b.addDirtyIndices(validators, changedVals)

	return nil
}

// UpdateValidatorAtIndex for the beacon state. Updates the validator
// at a specific index to a new value.
func (b *BeaconState) UpdateValidatorAtIndex(idx types.ValidatorIndex, val *ethpb.Validator) error {
	if !b.hasInnerState() {
		return ErrNilInnerState
	}
	if uint64(len(b.state.Validators)) <= uint64(idx) {
		return errors.Errorf("invalid index provided %d", idx)
	}
	b.lock.Lock()
	defer b.lock.Unlock()

	v := b.state.Validators
	if ref := b.sharedFieldReferences[validators]; ref.Refs() > 1 {
		v = b.validatorsReferences()
		ref.MinusRef()
		b.sharedFieldReferences[validators] = stateutil.NewRef(1)
	}

	v[idx] = val
	b.state.Validators = v
	b.markFieldAsDirty(validators)
	b.addDirtyIndices(validators, []uint64{uint64(idx)})

	return nil
}

// SetBalances for the beacon state. Updates the entire
// list to a new value by overwriting the previous one.
func (b *BeaconState) SetBalances(val []uint64) error {
	if !b.hasInnerState() {
		return ErrNilInnerState
	}
	b.lock.Lock()
	defer b.lock.Unlock()

	b.sharedFieldReferences[balances].MinusRef()
	b.sharedFieldReferences[balances] = stateutil.NewRef(1)

	b.state.Balances = val
	b.markFieldAsDirty(balances)
	return nil
}

// UpdateBalancesAtIndex for the beacon state. This method updates the balance
// at a specific index to a new value.
func (b *BeaconState) UpdateBalancesAtIndex(idx types.ValidatorIndex, val uint64) error {
	if !b.hasInnerState() {
		return ErrNilInnerState
	}
	if uint64(len(b.state.Balances)) <= uint64(idx) {
		return errors.Errorf("invalid index provided %d", idx)
	}
	b.lock.Lock()
	defer b.lock.Unlock()

	bals := b.state.Balances
	if b.sharedFieldReferences[balances].Refs() > 1 {
		bals = b.balances()
		b.sharedFieldReferences[balances].MinusRef()
		b.sharedFieldReferences[balances] = stateutil.NewRef(1)
	}

	bals[idx] = val
	b.state.Balances = bals
	b.markFieldAsDirty(balances)
	return nil
}

// SetRandaoMixes for the beacon state. Updates the entire
// randao mixes to a new value by overwriting the previous one.
func (b *BeaconState) SetRandaoMixes(val [][]byte) error {
	if !b.hasInnerState() {
		return ErrNilInnerState
	}
	b.lock.Lock()
	defer b.lock.Unlock()

	b.sharedFieldReferences[randaoMixes].MinusRef()
	b.sharedFieldReferences[randaoMixes] = stateutil.NewRef(1)

	b.state.RandaoMixes = val
	b.markFieldAsDirty(randaoMixes)
	b.rebuildTrie[randaoMixes] = true
	return nil
}

// UpdateRandaoMixesAtIndex for the beacon state. Updates the randao mixes
// at a specific index to a new value.
func (b *BeaconState) UpdateRandaoMixesAtIndex(idx uint64, val []byte) error {
	if !b.hasInnerState() {
		return ErrNilInnerState
	}
	if uint64(len(b.state.RandaoMixes)) <= idx {
		return errors.Errorf("invalid index provided %d", idx)
	}
	b.lock.Lock()
	defer b.lock.Unlock()

	mixes := b.state.RandaoMixes
	if refs := b.sharedFieldReferences[randaoMixes].Refs(); refs > 1 {
		// Copy elements in underlying array by reference.
		mixes = make([][]byte, len(b.state.RandaoMixes))
		copy(mixes, b.state.RandaoMixes)
		b.sharedFieldReferences[randaoMixes].MinusRef()
		b.sharedFieldReferences[randaoMixes] = stateutil.NewRef(1)
	}

	mixes[idx] = val
	b.state.RandaoMixes = mixes
	b.markFieldAsDirty(randaoMixes)
	b.addDirtyIndices(randaoMixes, []uint64{idx})

	return nil
}

// SetSlashings for the beacon state. Updates the entire
// list to a new value by overwriting the previous one.
func (b *BeaconState) SetSlashings(val []uint64) error {
	if !b.hasInnerState() {
		return ErrNilInnerState
	}
	b.lock.Lock()
	defer b.lock.Unlock()

	b.sharedFieldReferences[slashings].MinusRef()
	b.sharedFieldReferences[slashings] = stateutil.NewRef(1)

	b.state.Slashings = val
	b.markFieldAsDirty(slashings)
	return nil
}

// UpdateSlashingsAtIndex for the beacon state. Updates the slashings
// at a specific index to a new value.
func (b *BeaconState) UpdateSlashingsAtIndex(idx, val uint64) error {
	if !b.hasInnerState() {
		return ErrNilInnerState
	}
	if uint64(len(b.state.Slashings)) <= idx {
		return errors.Errorf("invalid index provided %d", idx)
	}
	b.lock.Lock()
	defer b.lock.Unlock()

	s := b.state.Slashings
	if b.sharedFieldReferences[slashings].Refs() > 1 {
		s = b.slashings()
		b.sharedFieldReferences[slashings].MinusRef()
		b.sharedFieldReferences[slashings] = stateutil.NewRef(1)
	}

	s[idx] = val

	b.state.Slashings = s

	b.markFieldAsDirty(slashings)
	return nil
}

// SetPreviousParticipationBits for the beacon state. Updates the entire
// list to a new value by overwriting the previous one.
func (b *BeaconState) SetPreviousParticipationBits(val []byte) error {
	if !b.hasInnerState() {
		return ErrNilInnerState
	}
	b.lock.Lock()
	defer b.lock.Unlock()

	b.sharedFieldReferences[previousEpochParticipationBits].MinusRef()
	b.sharedFieldReferences[previousEpochParticipationBits] = stateutil.NewRef(1)

	b.state.PreviousEpochParticipation = val
	b.markFieldAsDirty(previousEpochParticipationBits)
	b.rebuildTrie[previousEpochParticipationBits] = true
	return nil
}

// SetCurrentParticipationBits for the beacon state. Updates the entire
// list to a new value by overwriting the previous one.
func (b *BeaconState) SetCurrentParticipationBits(val []byte) error {
	if !b.hasInnerState() {
		return ErrNilInnerState
	}
	b.lock.Lock()
	defer b.lock.Unlock()

	b.sharedFieldReferences[currentEpochParticipationBits].MinusRef()
	b.sharedFieldReferences[currentEpochParticipationBits] = stateutil.NewRef(1)

	b.state.CurrentEpochParticipation = val
	b.markFieldAsDirty(currentEpochParticipationBits)
	b.rebuildTrie[currentEpochParticipationBits] = true
	return nil
}

// AppendHistoricalRoots for the beacon state. Appends the new value
// to the the end of list.
func (b *BeaconState) AppendHistoricalRoots(root [32]byte) error {
	if !b.hasInnerState() {
		return ErrNilInnerState
	}
	b.lock.Lock()
	defer b.lock.Unlock()

	roots := b.state.HistoricalRoots
	if b.sharedFieldReferences[historicalRoots].Refs() > 1 {
		roots = make([][]byte, len(b.state.HistoricalRoots))
		copy(roots, b.state.HistoricalRoots)
		b.sharedFieldReferences[historicalRoots].MinusRef()
		b.sharedFieldReferences[historicalRoots] = stateutil.NewRef(1)
	}

	b.state.HistoricalRoots = append(roots, root[:])
	b.markFieldAsDirty(historicalRoots)
	return nil
}

// AppendCurrentParticipationBits for the beacon state. Appends the new value
// to the the end of list.
func (b *BeaconState) AppendCurrentParticipationBits(val byte) error {
	if !b.hasInnerState() {
		return ErrNilInnerState
	}
	b.lock.Lock()
	defer b.lock.Unlock()

	participation := b.state.CurrentEpochParticipation
	if b.sharedFieldReferences[currentEpochParticipationBits].Refs() > 1 {
		// Copy elements in underlying array by reference.
		participation = make([]byte, len(b.state.CurrentEpochParticipation))
		copy(participation, b.state.CurrentEpochParticipation)
		b.sharedFieldReferences[currentEpochParticipationBits].MinusRef()
		b.sharedFieldReferences[currentEpochParticipationBits] = stateutil.NewRef(1)
	}

	b.state.CurrentEpochParticipation = append(participation, val)
	b.markFieldAsDirty(currentEpochParticipationBits)
	b.addDirtyIndices(currentEpochParticipationBits, []uint64{uint64(len(b.state.CurrentEpochParticipation) - 1)})
	return nil
}

// AppendPreviousParticipationBits for the beacon state. Appends the new value
// to the the end of list.
func (b *BeaconState) AppendPreviousParticipationBits(val byte) error {
	if !b.hasInnerState() {
		return ErrNilInnerState
	}
	b.lock.Lock()
	defer b.lock.Unlock()

	bits := b.state.PreviousEpochParticipation
	if b.sharedFieldReferences[previousEpochParticipationBits].Refs() > 1 {
		bits = make([]byte, len(b.state.PreviousEpochParticipation))
		copy(bits, b.state.PreviousEpochParticipation)
		b.sharedFieldReferences[previousEpochParticipationBits].MinusRef()
		b.sharedFieldReferences[previousEpochParticipationBits] = stateutil.NewRef(1)
	}

	b.state.PreviousEpochParticipation = append(bits, val)
	b.markFieldAsDirty(previousEpochParticipationBits)
	b.addDirtyIndices(previousEpochParticipationBits, []uint64{uint64(len(b.state.PreviousEpochParticipation) - 1)})

	return nil
}

// AppendValidator for the beacon state. Appends the new value
// to the the end of list.
func (b *BeaconState) AppendValidator(val *ethpb.Validator) error {
	if !b.hasInnerState() {
		return ErrNilInnerState
	}
	b.lock.Lock()
	defer b.lock.Unlock()

	vals := b.state.Validators
	if b.sharedFieldReferences[validators].Refs() > 1 {
		vals = b.validatorsReferences()
		b.sharedFieldReferences[validators].MinusRef()
		b.sharedFieldReferences[validators] = stateutil.NewRef(1)
	}

	// append validator to slice
	b.state.Validators = append(vals, val)
	valIdx := types.ValidatorIndex(len(b.state.Validators) - 1)

	b.valMapHandler.Set(bytesutil.ToBytes48(val.PublicKey), valIdx)

	b.markFieldAsDirty(validators)
	b.addDirtyIndices(validators, []uint64{uint64(valIdx)})
	return nil
}

// AppendBalance for the beacon state. Appends the new value
// to the the end of list.
func (b *BeaconState) AppendBalance(bal uint64) error {
	if !b.hasInnerState() {
		return ErrNilInnerState
	}
	b.lock.Lock()
	defer b.lock.Unlock()

	bals := b.state.Balances
	if b.sharedFieldReferences[balances].Refs() > 1 {
		bals = b.balances()
		b.sharedFieldReferences[balances].MinusRef()
		b.sharedFieldReferences[balances] = stateutil.NewRef(1)
	}

	b.state.Balances = append(bals, bal)
	b.markFieldAsDirty(balances)
	return nil
}

// SetJustificationBits for the beacon state.
func (b *BeaconState) SetJustificationBits(val bitfield.Bitvector4) error {
	if !b.hasInnerState() {
		return ErrNilInnerState
	}
	b.lock.Lock()
	defer b.lock.Unlock()

	b.state.JustificationBits = val
	b.markFieldAsDirty(justificationBits)
	return nil
}

// SetPreviousJustifiedCheckpoint for the beacon state.
func (b *BeaconState) SetPreviousJustifiedCheckpoint(val *ethpb.Checkpoint) error {
	if !b.hasInnerState() {
		return ErrNilInnerState
	}
	b.lock.Lock()
	defer b.lock.Unlock()

	b.state.PreviousJustifiedCheckpoint = val
	b.markFieldAsDirty(previousJustifiedCheckpoint)
	return nil
}

// SetCurrentJustifiedCheckpoint for the beacon state.
func (b *BeaconState) SetCurrentJustifiedCheckpoint(val *ethpb.Checkpoint) error {
	if !b.hasInnerState() {
		return ErrNilInnerState
	}
	b.lock.Lock()
	defer b.lock.Unlock()

	b.state.CurrentJustifiedCheckpoint = val
	b.markFieldAsDirty(currentJustifiedCheckpoint)
	return nil
}

// SetFinalizedCheckpoint for the beacon state.
func (b *BeaconState) SetFinalizedCheckpoint(val *ethpb.Checkpoint) error {
	if !b.hasInnerState() {
		return ErrNilInnerState
	}
	b.lock.Lock()
	defer b.lock.Unlock()

	b.state.FinalizedCheckpoint = val
	b.markFieldAsDirty(finalizedCheckpoint)
	return nil
}

// SetCurrentSyncCommittee for the beacon state.
func (b *BeaconState) SetCurrentSyncCommittee(val *pbp2p.SyncCommittee) error {
	if !b.hasInnerState() {
		return ErrNilInnerState
	}
	b.lock.Lock()
	defer b.lock.Unlock()

	b.state.CurrentSyncCommittee = val
	b.markFieldAsDirty(currentSyncCommittee)
	return nil
}

// SetNextSyncCommittee for the beacon state.
func (b *BeaconState) SetNextSyncCommittee(val *pbp2p.SyncCommittee) error {
	if !b.hasInnerState() {
		return ErrNilInnerState
	}
	b.lock.Lock()
	defer b.lock.Unlock()

	b.state.NextSyncCommittee = val
	b.markFieldAsDirty(nextSyncCommittee)
	return nil
}

// AppendInactivityScore for the beacon state.
func (b *BeaconState) AppendInactivityScore(s uint64) error {
	if !b.hasInnerState() {
		return ErrNilInnerState
	}
	b.lock.Lock()
	defer b.lock.Unlock()

	scores := b.state.InactivityScores
	if b.sharedFieldReferences[inactivityScores].Refs() > 1 {
		scores = b.inactivityScores()
		b.sharedFieldReferences[inactivityScores].MinusRef()
		b.sharedFieldReferences[inactivityScores] = stateutil.NewRef(1)
	}

	b.state.InactivityScores = append(scores, s)
	b.markFieldAsDirty(inactivityScores)
	return nil
}

// SetInactivityScores for the beacon state. Updates the entire
// list to a new value by overwriting the previous one.
func (b *BeaconState) SetInactivityScores(val []uint64) error {
	if !b.hasInnerState() {
		return ErrNilInnerState
	}
	b.lock.Lock()
	defer b.lock.Unlock()

	b.sharedFieldReferences[inactivityScores].MinusRef()
	b.sharedFieldReferences[inactivityScores] = stateutil.NewRef(1)

	b.state.InactivityScores = val
	b.markFieldAsDirty(inactivityScores)
	return nil
}

// Recomputes the branch up the index in the Merkle trie representation
// of the beacon state. This method performs map reads and the caller MUST
// hold the lock before calling this method.
func (b *BeaconState) recomputeRoot(idx int) {
	hashFunc := hashutil.CustomSHA256Hasher()
	layers := b.merkleLayers
	// The merkle tree structure looks as follows:
	// [[r1, r2, r3, r4], [parent1, parent2], [root]]
	// Using information about the index which changed, idx, we recompute
	// only its branch up the tree.
	currentIndex := idx
	root := b.merkleLayers[0][idx]
	for i := 0; i < len(layers)-1; i++ {
		isLeft := currentIndex%2 == 0
		neighborIdx := currentIndex ^ 1

		neighbor := make([]byte, 32)
		if layers[i] != nil && len(layers[i]) != 0 && neighborIdx < len(layers[i]) {
			neighbor = layers[i][neighborIdx]
		}
		if isLeft {
			parentHash := hashFunc(append(root, neighbor...))
			root = parentHash[:]
		} else {
			parentHash := hashFunc(append(neighbor, root...))
			root = parentHash[:]
		}
		parentIdx := currentIndex / 2
		// Update the cached layers at the parent index.
		layers[i+1][parentIdx] = root
		currentIndex = parentIdx
	}
	b.merkleLayers = layers
}

func (b *BeaconState) markFieldAsDirty(field fieldIndex) {
	_, ok := b.dirtyFields[field]
	if !ok {
		b.dirtyFields[field] = true
	}
	// do nothing if field already exists
}

// addDirtyIndices adds the relevant dirty field indices, so that they
// can be recomputed.
func (b *BeaconState) addDirtyIndices(index fieldIndex, indices []uint64) {
	b.dirtyIndices[index] = append(b.dirtyIndices[index], indices...)
}
