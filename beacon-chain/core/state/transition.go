// Package state implements the whole state transition
// function which consists of per slot, per-epoch transitions.
// It also bootstraps the genesis beacon state for slot 0.
package state

import (
	"bytes"
	"context"
	"fmt"

	"github.com/pkg/errors"
	types "github.com/prysmaticlabs/eth2-types"
	"github.com/prysmaticlabs/prysm/beacon-chain/cache"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/altair"
	b "github.com/prysmaticlabs/prysm/beacon-chain/core/blocks"
	e "github.com/prysmaticlabs/prysm/beacon-chain/core/epoch"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/epoch/precompute"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	v "github.com/prysmaticlabs/prysm/beacon-chain/core/validators"
	iface "github.com/prysmaticlabs/prysm/beacon-chain/state/interface"
	"github.com/prysmaticlabs/prysm/proto/interfaces"
	"github.com/prysmaticlabs/prysm/shared/mathutil"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/traceutil"
	"github.com/prysmaticlabs/prysm/shared/version"
	"go.opencensus.io/trace"
)

// processFunc is a function that processes a block with a given state. State is mutated.
type processFunc func(context.Context, iface.BeaconState, interfaces.SignedBeaconBlock) (iface.BeaconState, error)

var processDepositsFunc = func(ctx context.Context, s iface.BeaconState, blk interfaces.SignedBeaconBlock) (iface.BeaconState, error) {
	return b.ProcessDeposits(ctx, s, blk.Block().Body().Deposits())
}
var processProposerSlashingFunc = func(ctx context.Context, s iface.BeaconState, blk interfaces.SignedBeaconBlock) (iface.BeaconState, error) {
	return b.ProcessProposerSlashings(ctx, s, blk.Block().Body().ProposerSlashings(), v.SlashValidator)
}

var processAttesterSlashingFunc = func(ctx context.Context, s iface.BeaconState, blk interfaces.SignedBeaconBlock) (iface.BeaconState, error) {
	return b.ProcessAttesterSlashings(ctx, s, blk.Block().Body().AttesterSlashings(), v.SlashValidator)
}

var processEth1DataFunc = func(ctx context.Context, s iface.BeaconState, blk interfaces.SignedBeaconBlock) (iface.BeaconState, error) {
	return b.ProcessEth1DataInBlock(ctx, s, blk.Block().Body().Eth1Data())
}

var processExitFunc = func(ctx context.Context, s iface.BeaconState, blk interfaces.SignedBeaconBlock) (iface.BeaconState, error) {
	return b.ProcessVoluntaryExits(ctx, s, blk.Block().Body().VoluntaryExits())
}

// This defines the processing block routine as outlined in the Ethereum Beacon Chain spec:
// https://github.com/ethereum/eth2.0-specs/blob/dev/specs/phase0/beacon-chain.md#block-processing
var processingPipeline = []processFunc{
	b.ProcessBlockHeader,
	b.ProcessRandao,
	processEth1DataFunc,
	VerifyOperationLengths,
	processProposerSlashingFunc,
	processAttesterSlashingFunc,
	b.ProcessAttestations,
	processDepositsFunc,
	processExitFunc,
}

// ExecuteStateTransition defines the procedure for a state transition function.
//
// Note: This method differs from the spec pseudocode as it uses a batch signature verification.
// See: ExecuteStateTransitionNoVerifyAnySig
//
// Spec pseudocode definition:
//  def state_transition(state: BeaconState, signed_block: SignedBeaconBlock, validate_result: bool=True) -> None:
//    block = signed_block.message
//    # Process slots (including those with no blocks) since block
//    process_slots(state, block.slot)
//    # Verify signature
//    if validate_result:
//        assert verify_block_signature(state, signed_block)
//    # Process block
//    process_block(state, block)
//    # Verify state root
//    if validate_result:
//        assert block.state_root == hash_tree_root(state)
func ExecuteStateTransition(
	ctx context.Context,
	state iface.BeaconState,
	signed interfaces.SignedBeaconBlock,
) (iface.BeaconState, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if signed == nil || signed.IsNil() || signed.Block().IsNil() {
		return nil, errors.New("nil block")
	}

	ctx, span := trace.StartSpan(ctx, "core.state.ExecuteStateTransition")
	defer span.End()
	var err error

	set, postState, err := ExecuteStateTransitionNoVerifyAnySig(ctx, state, signed)
	if err != nil {
		return nil, errors.Wrap(err, "could not execute state transition")
	}
	valid, err := set.Verify()
	if err != nil {
		return nil, errors.Wrap(err, "could not batch verify signature")
	}
	if !valid {
		return nil, errors.New("signature in block failed to verify")
	}

	return postState, nil
}

// ProcessSlot happens every slot and focuses on the slot counter and block roots record updates.
// It happens regardless if there's an incoming block or not.
// Spec pseudocode definition:
//
//  def process_slot(state: BeaconState) -> None:
//    # Cache state root
//    previous_state_root = hash_tree_root(state)
//    state.state_roots[state.slot % SLOTS_PER_HISTORICAL_ROOT] = previous_state_root
//    # Cache latest block header state root
//    if state.latest_block_header.state_root == Bytes32():
//        state.latest_block_header.state_root = previous_state_root
//    # Cache block root
//    previous_block_root = hash_tree_root(state.latest_block_header)
//    state.block_roots[state.slot % SLOTS_PER_HISTORICAL_ROOT] = previous_block_root
func ProcessSlot(ctx context.Context, state iface.BeaconState) (iface.BeaconState, error) {
	ctx, span := trace.StartSpan(ctx, "core.state.ProcessSlot")
	defer span.End()
	span.AddAttributes(trace.Int64Attribute("slot", int64(state.Slot())))

	prevStateRoot, err := state.HashTreeRoot(ctx)
	if err != nil {
		return nil, err
	}
	if err := state.UpdateStateRootAtIndex(
		uint64(state.Slot()%params.BeaconConfig().SlotsPerHistoricalRoot),
		prevStateRoot,
	); err != nil {
		return nil, err
	}

	zeroHash := params.BeaconConfig().ZeroHash
	// Cache latest block header state root.
	header := state.LatestBlockHeader()
	if header.StateRoot == nil || bytes.Equal(header.StateRoot, zeroHash[:]) {
		header.StateRoot = prevStateRoot[:]
		if err := state.SetLatestBlockHeader(header); err != nil {
			return nil, err
		}
	}
	prevBlockRoot, err := state.LatestBlockHeader().HashTreeRoot()
	if err != nil {
		traceutil.AnnotateError(span, err)
		return nil, errors.Wrap(err, "could not determine prev block root")
	}
	// Cache the block root.
	if err := state.UpdateBlockRootAtIndex(
		uint64(state.Slot()%params.BeaconConfig().SlotsPerHistoricalRoot),
		prevBlockRoot,
	); err != nil {
		return nil, err
	}
	return state, nil
}

// ProcessSlotsUsingNextSlotCache processes slots by using next slot cache for higher efficiency.
func ProcessSlotsUsingNextSlotCache(
	ctx context.Context,
	parentState iface.BeaconState,
	parentRoot []byte,
	slot types.Slot) (iface.BeaconState, error) {
	ctx, span := trace.StartSpan(ctx, "core.state.ProcessSlotsUsingNextSlotCache")
	defer span.End()

	// Check whether the parent state has been advanced by 1 slot in next slot cache.
	nextSlotState, err := NextSlotState(ctx, parentRoot)
	if err != nil {
		return nil, err
	}
	// If the next slot state is not nil (i.e. cache hit).
	// We replace next slot state with parent state.
	if nextSlotState != nil && !nextSlotState.IsNil() {
		parentState = nextSlotState
	}

	// Since next slot cache only advances state by 1 slot,
	// we check if there's more slots that need to process.
	if slot > parentState.Slot() {
		parentState, err = ProcessSlots(ctx, parentState, slot)
		if err != nil {
			return nil, errors.Wrap(err, "could not process slots")
		}
	}
	return parentState, nil
}

// ProcessSlots process through skip slots and apply epoch transition when it's needed
//
// Spec pseudocode definition:
//  def process_slots(state: BeaconState, slot: Slot) -> None:
//    assert state.slot < slot
//    while state.slot < slot:
//        process_slot(state)
//        # Process epoch on the start slot of the next epoch
//        if (state.slot + 1) % SLOTS_PER_EPOCH == 0:
//            process_epoch(state)
//        state.slot = Slot(state.slot + 1)
func ProcessSlots(ctx context.Context, state iface.BeaconState, slot types.Slot) (iface.BeaconState, error) {
	ctx, span := trace.StartSpan(ctx, "core.state.ProcessSlots")
	defer span.End()
	if state == nil || state.IsNil() {
		return nil, errors.New("nil state")
	}
	span.AddAttributes(trace.Int64Attribute("slots", int64(slot)-int64(state.Slot())))

	// The block must have a higher slot than parent state.
	if state.Slot() >= slot {
		err := fmt.Errorf("expected state.slot %d < slot %d", state.Slot(), slot)
		traceutil.AnnotateError(span, err)
		return nil, err
	}

	highestSlot := state.Slot()
	key, err := CacheKey(ctx, state)
	if err != nil {
		return nil, err
	}

	// Restart from cached value, if one exists.
	cachedState, err := SkipSlotCache.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	if cachedState != nil && !cachedState.IsNil() && cachedState.Slot() < slot {
		highestSlot = cachedState.Slot()
		state = cachedState
	}
	if err := SkipSlotCache.MarkInProgress(key); errors.Is(err, cache.ErrAlreadyInProgress) {
		cachedState, err = SkipSlotCache.Get(ctx, key)
		if err != nil {
			return nil, err
		}
		if cachedState != nil && !cachedState.IsNil() && cachedState.Slot() < slot {
			highestSlot = cachedState.Slot()
			state = cachedState
		}
	} else if err != nil {
		return nil, err
	}
	defer func() {
		if err := SkipSlotCache.MarkNotInProgress(key); err != nil {
			traceutil.AnnotateError(span, err)
			log.WithError(err).Error("Failed to mark skip slot no longer in progress")
		}
	}()

	for state.Slot() < slot {
		if ctx.Err() != nil {
			traceutil.AnnotateError(span, ctx.Err())
			// Cache last best value.
			if highestSlot < state.Slot() {
				if err := SkipSlotCache.Put(ctx, key, state); err != nil {
					log.WithError(err).Error("Failed to put skip slot cache value")
				}
			}
			return nil, ctx.Err()
		}
		state, err = ProcessSlot(ctx, state)
		if err != nil {
			traceutil.AnnotateError(span, err)
			return nil, errors.Wrap(err, "could not process slot")
		}
		if CanProcessEpoch(state) {
			switch state.Version() {
			case version.Phase0:
				state, err = ProcessEpochPrecompute(ctx, state)
				if err != nil {
					traceutil.AnnotateError(span, err)
					return nil, errors.Wrap(err, "could not process epoch with optimizations")
				}
			case version.Altair:
				state, err = altair.ProcessEpoch(ctx, state)
				if err != nil {
					traceutil.AnnotateError(span, err)
					return nil, errors.Wrap(err, "could not process epoch")
				}
			default:
				return nil, errors.New("beacon state should have a version")
			}
		}
		if err := state.SetSlot(state.Slot() + 1); err != nil {
			traceutil.AnnotateError(span, err)
			return nil, errors.Wrap(err, "failed to increment state slot")
		}

		// Transition to Altair state.
		if helpers.IsEpochStart(state.Slot()) && helpers.SlotToEpoch(state.Slot()) == params.BeaconConfig().AltairForkEpoch {
			state, err = altair.UpgradeToAltair(state)
			if err != nil {
				return nil, err
			}
		}
	}

	if highestSlot < state.Slot() {
		if err := SkipSlotCache.Put(ctx, key, state); err != nil {
			log.WithError(err).Error("Failed to put skip slot cache value")
			traceutil.AnnotateError(span, err)
		}
	}

	return state, nil
}

// ProcessBlock creates a new, modified beacon state by applying block operation
// transformations as defined in the Ethereum Serenity specification, including processing proposer slashings,
// processing block attestations, and more.
//
// Spec pseudocode definition:
//
//  def process_block(state: BeaconState, block: BeaconBlock) -> None:
//    process_block_header(state, block)
//    process_randao(state, block.body)
//    process_eth1_data(state, block.body)
//    process_operations(state, block.body)
func ProcessBlock(
	ctx context.Context,
	state iface.BeaconState,
	signed interfaces.SignedBeaconBlock,
) (iface.BeaconState, error) {
	ctx, span := trace.StartSpan(ctx, "core.state.ProcessBlock")
	defer span.End()

	var err error
	if err = helpers.VerifyNilBeaconBlock(signed); err != nil {
		return nil, err
	}

	for _, p := range processingPipeline {
		state, err = p(ctx, state, signed)
		if err != nil {
			return nil, errors.Wrap(err, "Could not process block")
		}
	}

	return state, nil
}

// VerifyOperationLengths verifies that block operation lengths are valid.
func VerifyOperationLengths(_ context.Context, state iface.BeaconState, b interfaces.SignedBeaconBlock) (iface.BeaconState, error) {
	if err := helpers.VerifyNilBeaconBlock(b); err != nil {
		return nil, err
	}
	body := b.Block().Body()

	if uint64(len(body.ProposerSlashings())) > params.BeaconConfig().MaxProposerSlashings {
		return nil, fmt.Errorf(
			"number of proposer slashings (%d) in block body exceeds allowed threshold of %d",
			len(body.ProposerSlashings()),
			params.BeaconConfig().MaxProposerSlashings,
		)
	}

	if uint64(len(body.AttesterSlashings())) > params.BeaconConfig().MaxAttesterSlashings {
		return nil, fmt.Errorf(
			"number of attester slashings (%d) in block body exceeds allowed threshold of %d",
			len(body.AttesterSlashings()),
			params.BeaconConfig().MaxAttesterSlashings,
		)
	}

	if uint64(len(body.Attestations())) > params.BeaconConfig().MaxAttestations {
		return nil, fmt.Errorf(
			"number of attestations (%d) in block body exceeds allowed threshold of %d",
			len(body.Attestations()),
			params.BeaconConfig().MaxAttestations,
		)
	}

	if uint64(len(body.VoluntaryExits())) > params.BeaconConfig().MaxVoluntaryExits {
		return nil, fmt.Errorf(
			"number of voluntary exits (%d) in block body exceeds allowed threshold of %d",
			len(body.VoluntaryExits()),
			params.BeaconConfig().MaxVoluntaryExits,
		)
	}
	eth1Data := state.Eth1Data()
	if eth1Data == nil {
		return nil, errors.New("nil eth1data in state")
	}
	if state.Eth1DepositIndex() > eth1Data.DepositCount {
		return nil, fmt.Errorf("expected state.deposit_index %d <= eth1data.deposit_count %d", state.Eth1DepositIndex(), eth1Data.DepositCount)
	}
	maxDeposits := mathutil.Min(params.BeaconConfig().MaxDeposits, eth1Data.DepositCount-state.Eth1DepositIndex())
	// Verify outstanding deposits are processed up to max number of deposits
	if uint64(len(body.Deposits())) != maxDeposits {
		return nil, fmt.Errorf("incorrect outstanding deposits in block body, wanted: %d, got: %d",
			maxDeposits, len(body.Deposits()))
	}

	return state, nil
}

// CanProcessEpoch checks the eligibility to process epoch.
// The epoch can be processed at the end of the last slot of every epoch
//
// Spec pseudocode definition:
//    If (state.slot + 1) % SLOTS_PER_EPOCH == 0:
func CanProcessEpoch(state iface.ReadOnlyBeaconState) bool {
	return (state.Slot()+1)%params.BeaconConfig().SlotsPerEpoch == 0
}

// ProcessEpochPrecompute describes the per epoch operations that are performed on the beacon state.
// It's optimized by pre computing validator attested info and epoch total/attested balances upfront.
func ProcessEpochPrecompute(ctx context.Context, state iface.BeaconState) (iface.BeaconState, error) {
	ctx, span := trace.StartSpan(ctx, "core.state.ProcessEpochPrecompute")
	defer span.End()
	span.AddAttributes(trace.Int64Attribute("epoch", int64(helpers.CurrentEpoch(state))))

	if state == nil || state.IsNil() {
		return nil, errors.New("nil state")
	}
	vp, bp, err := precompute.New(ctx, state)
	if err != nil {
		return nil, err
	}
	vp, bp, err = precompute.ProcessAttestations(ctx, state, vp, bp)
	if err != nil {
		return nil, err
	}

	state, err = precompute.ProcessJustificationAndFinalizationPreCompute(state, bp)
	if err != nil {
		return nil, errors.Wrap(err, "could not process justification")
	}

	state, err = precompute.ProcessRewardsAndPenaltiesPrecompute(state, bp, vp, precompute.AttestationsDelta, precompute.ProposersDelta)
	if err != nil {
		return nil, errors.Wrap(err, "could not process rewards and penalties")
	}

	state, err = e.ProcessRegistryUpdates(state)
	if err != nil {
		return nil, errors.Wrap(err, "could not process registry updates")
	}

	err = precompute.ProcessSlashingsPrecompute(state, bp)
	if err != nil {
		return nil, err
	}

	state, err = e.ProcessFinalUpdates(state)
	if err != nil {
		return nil, errors.Wrap(err, "could not process final updates")
	}
	return state, nil
}
