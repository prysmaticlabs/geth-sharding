package state

import (
	"context"
	"testing"

	fuzz "github.com/google/gofuzz"
	types "github.com/prysmaticlabs/eth2-types"
	v1 "github.com/prysmaticlabs/prysm/beacon-chain/state/v1"
	ethpb "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/proto/eth/v1alpha1/wrapper"
)

func TestFuzzExecuteStateTransition_1000(t *testing.T) {
	SkipSlotCache.Disable()
	defer SkipSlotCache.Enable()
	ctx := context.Background()
	state := &v1.BeaconState{}
	sb := &ethpb.SignedBeaconBlock{}
	fuzzer := fuzz.NewWithSeed(0)
	fuzzer.NilChance(0.1)
	for i := 0; i < 1000; i++ {
		fuzzer.Fuzz(state)
		fuzzer.Fuzz(sb)
		s, err := ExecuteStateTransition(ctx, state, wrapper.WrappedPhase0SignedBeaconBlock(sb))
		if err != nil && s != nil {
			t.Fatalf("state should be nil on err. found: %v on error: %v for state: %v and signed block: %v", s, err, state, sb)
		}
	}
}

func TestFuzzCalculateStateRoot_1000(t *testing.T) {
	SkipSlotCache.Disable()
	defer SkipSlotCache.Enable()
	ctx := context.Background()
	state := &v1.BeaconState{}
	sb := &ethpb.SignedBeaconBlock{}
	fuzzer := fuzz.NewWithSeed(0)
	fuzzer.NilChance(0.1)
	for i := 0; i < 1000; i++ {
		fuzzer.Fuzz(state)
		fuzzer.Fuzz(sb)
		stateRoot, err := CalculateStateRoot(ctx, state, wrapper.WrappedPhase0SignedBeaconBlock(sb))
		if err != nil && stateRoot != [32]byte{} {
			t.Fatalf("state root should be empty on err. found: %v on error: %v for signed block: %v", stateRoot, err, sb)
		}
	}
}

func TestFuzzProcessSlot_1000(t *testing.T) {
	SkipSlotCache.Disable()
	defer SkipSlotCache.Enable()
	ctx := context.Background()
	state := &v1.BeaconState{}
	fuzzer := fuzz.NewWithSeed(0)
	fuzzer.NilChance(0.1)
	for i := 0; i < 1000; i++ {
		fuzzer.Fuzz(state)
		s, err := ProcessSlot(ctx, state)
		if err != nil && s != nil {
			t.Fatalf("state should be nil on err. found: %v on error: %v for state: %v", s, err, state)
		}
	}
}

func TestFuzzProcessSlots_1000(t *testing.T) {
	SkipSlotCache.Disable()
	defer SkipSlotCache.Enable()
	ctx := context.Background()
	state := &v1.BeaconState{}
	slot := types.Slot(0)
	fuzzer := fuzz.NewWithSeed(0)
	fuzzer.NilChance(0.1)
	for i := 0; i < 1000; i++ {
		fuzzer.Fuzz(state)
		fuzzer.Fuzz(&slot)
		s, err := ProcessSlots(ctx, state, slot)
		if err != nil && s != nil {
			t.Fatalf("state should be nil on err. found: %v on error: %v for state: %v", s, err, state)
		}
	}
}

func TestFuzzProcessBlock_1000(t *testing.T) {
	SkipSlotCache.Disable()
	defer SkipSlotCache.Enable()
	ctx := context.Background()
	state := &v1.BeaconState{}
	sb := &ethpb.SignedBeaconBlock{}
	fuzzer := fuzz.NewWithSeed(0)
	fuzzer.NilChance(0.1)
	for i := 0; i < 1000; i++ {
		fuzzer.Fuzz(state)
		fuzzer.Fuzz(sb)
		s, err := ProcessBlock(ctx, state, wrapper.WrappedPhase0SignedBeaconBlock(sb))
		if err != nil && s != nil {
			t.Fatalf("state should be nil on err. found: %v on error: %v for signed block: %v", s, err, sb)
		}
	}
}

func TestFuzzProcessOperations_1000(t *testing.T) {
	SkipSlotCache.Disable()
	defer SkipSlotCache.Enable()
	ctx := context.Background()
	state := &v1.BeaconState{}
	bb := &ethpb.SignedBeaconBlock{}
	fuzzer := fuzz.NewWithSeed(0)
	fuzzer.NilChance(0.1)
	for i := 0; i < 1000; i++ {
		fuzzer.Fuzz(state)
		fuzzer.Fuzz(bb)
		s, err := ProcessBlock(ctx, state, wrapper.WrappedPhase0SignedBeaconBlock(bb))
		if err != nil && s != nil {
			t.Fatalf("state should be nil on err. found: %v on error: %v for block body: %v", s, err, bb)
		}
	}
}

func TestFuzzprocessOperationsNoVerify_1000(t *testing.T) {
	SkipSlotCache.Disable()
	defer SkipSlotCache.Enable()
	ctx := context.Background()
	state := &v1.BeaconState{}
	bb := &ethpb.SignedBeaconBlock{}
	fuzzer := fuzz.NewWithSeed(0)
	fuzzer.NilChance(0.1)
	for i := 0; i < 1000; i++ {
		fuzzer.Fuzz(state)
		fuzzer.Fuzz(bb)
		s, err := ProcessOperationsNoVerifyAttsSigs(ctx, state, wrapper.WrappedPhase0SignedBeaconBlock(bb))
		if err != nil && s != nil {
			t.Fatalf("state should be nil on err. found: %v on error: %v for block body: %v", s, err, bb)
		}
	}
}

func TestFuzzverifyOperationLengths_10000(t *testing.T) {
	SkipSlotCache.Disable()
	defer SkipSlotCache.Enable()
	state := &v1.BeaconState{}
	bb := &ethpb.SignedBeaconBlock{}
	fuzzer := fuzz.NewWithSeed(0)
	fuzzer.NilChance(0.1)
	for i := 0; i < 10000; i++ {
		fuzzer.Fuzz(state)
		fuzzer.Fuzz(bb)
		_, err := VerifyOperationLengths(context.Background(), state, wrapper.WrappedPhase0SignedBeaconBlock(bb))
		_ = err
	}
}

func TestFuzzCanProcessEpoch_10000(t *testing.T) {
	SkipSlotCache.Disable()
	defer SkipSlotCache.Enable()
	state := &v1.BeaconState{}
	fuzzer := fuzz.NewWithSeed(0)
	fuzzer.NilChance(0.1)
	for i := 0; i < 10000; i++ {
		fuzzer.Fuzz(state)
		CanProcessEpoch(state)
	}
}

func TestFuzzProcessEpochPrecompute_1000(t *testing.T) {
	SkipSlotCache.Disable()
	defer SkipSlotCache.Enable()
	ctx := context.Background()
	state := &v1.BeaconState{}
	fuzzer := fuzz.NewWithSeed(0)
	fuzzer.NilChance(0.1)
	for i := 0; i < 1000; i++ {
		fuzzer.Fuzz(state)
		s, err := ProcessEpochPrecompute(ctx, state)
		if err != nil && s != nil {
			t.Fatalf("state should be nil on err. found: %v on error: %v for state: %v", s, err, state)
		}
	}
}

func TestFuzzProcessBlockForStateRoot_1000(t *testing.T) {
	SkipSlotCache.Disable()
	defer SkipSlotCache.Enable()
	ctx := context.Background()
	state := &v1.BeaconState{}
	sb := &ethpb.SignedBeaconBlock{}
	fuzzer := fuzz.NewWithSeed(0)
	fuzzer.NilChance(0.1)
	for i := 0; i < 1000; i++ {
		fuzzer.Fuzz(state)
		fuzzer.Fuzz(sb)
		s, err := ProcessBlockForStateRoot(ctx, state, wrapper.WrappedPhase0SignedBeaconBlock(sb))
		if err != nil && s != nil {
			t.Fatalf("state should be nil on err. found: %v on error: %v for signed block: %v", s, err, sb)
		}
	}
}
