package state_test

import (
	"context"
	"testing"

	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	coreState "github.com/prysmaticlabs/prysm/beacon-chain/core/state"
	iface "github.com/prysmaticlabs/prysm/beacon-chain/state/interface"
	v1 "github.com/prysmaticlabs/prysm/beacon-chain/state/v1"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	"github.com/prysmaticlabs/prysm/proto/eth/v1alpha1/wrapper"
	"github.com/prysmaticlabs/prysm/shared/benchutil"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/testutil/require"
	"google.golang.org/protobuf/proto"
)

var runAmount = 25

func BenchmarkExecuteStateTransition_FullBlock(b *testing.B) {
	benchutil.SetBenchmarkConfig()
	beaconState, err := benchutil.PreGenState1Epoch()
	require.NoError(b, err)
	cleanStates := clonedStates(beaconState)
	block, err := benchutil.PreGenFullBlock()
	require.NoError(b, err)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := coreState.ExecuteStateTransition(context.Background(), cleanStates[i], wrapper.WrappedPhase0SignedBeaconBlock(block))
		require.NoError(b, err)
	}
}

func BenchmarkExecuteStateTransition_WithCache(b *testing.B) {
	benchutil.SetBenchmarkConfig()

	beaconState, err := benchutil.PreGenState1Epoch()
	require.NoError(b, err)
	cleanStates := clonedStates(beaconState)
	block, err := benchutil.PreGenFullBlock()
	require.NoError(b, err)

	// We have to reset slot back to last epoch to hydrate cache. Since
	// some attestations in block are from previous epoch
	currentSlot := beaconState.Slot()
	require.NoError(b, beaconState.SetSlot(beaconState.Slot()-params.BeaconConfig().SlotsPerEpoch))
	require.NoError(b, helpers.UpdateCommitteeCache(beaconState, helpers.CurrentEpoch(beaconState)))
	require.NoError(b, beaconState.SetSlot(currentSlot))
	// Run the state transition once to populate the cache.
	_, err = coreState.ExecuteStateTransition(context.Background(), beaconState, wrapper.WrappedPhase0SignedBeaconBlock(block))
	require.NoError(b, err, "Failed to process block, benchmarks will fail")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := coreState.ExecuteStateTransition(context.Background(), cleanStates[i], wrapper.WrappedPhase0SignedBeaconBlock(block))
		require.NoError(b, err, "Failed to process block, benchmarks will fail")
	}
}

func BenchmarkProcessEpoch_2FullEpochs(b *testing.B) {
	benchutil.SetBenchmarkConfig()
	beaconState, err := benchutil.PreGenState2FullEpochs()
	require.NoError(b, err)

	// We have to reset slot back to last epoch to hydrate cache. Since
	// some attestations in block are from previous epoch
	currentSlot := beaconState.Slot()
	require.NoError(b, beaconState.SetSlot(beaconState.Slot()-params.BeaconConfig().SlotsPerEpoch))
	require.NoError(b, helpers.UpdateCommitteeCache(beaconState, helpers.CurrentEpoch(beaconState)))
	require.NoError(b, beaconState.SetSlot(currentSlot))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// ProcessEpochPrecompute is the optimized version of process epoch. It's enabled by default
		// at run time.
		_, err := coreState.ProcessEpochPrecompute(context.Background(), beaconState.Copy())
		require.NoError(b, err)
	}
}

func BenchmarkHashTreeRoot_FullState(b *testing.B) {
	beaconState, err := benchutil.PreGenState2FullEpochs()
	require.NoError(b, err)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := beaconState.HashTreeRoot(context.Background())
		require.NoError(b, err)
	}
}

func BenchmarkHashTreeRootState_FullState(b *testing.B) {
	beaconState, err := benchutil.PreGenState2FullEpochs()
	require.NoError(b, err)

	ctx := context.Background()

	// Hydrate the HashTreeRootState cache.
	_, err = beaconState.HashTreeRoot(ctx)
	require.NoError(b, err)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := beaconState.HashTreeRoot(ctx)
		require.NoError(b, err)
	}
}

func BenchmarkMarshalState_FullState(b *testing.B) {
	beaconState, err := benchutil.PreGenState2FullEpochs()
	require.NoError(b, err)
	natState, err := v1.ProtobufBeaconState(beaconState.InnerStateUnsafe())
	require.NoError(b, err)
	b.Run("Proto_Marshal", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, err := proto.Marshal(natState)
			require.NoError(b, err)
		}
	})

	b.Run("Fast_SSZ_Marshal", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, err := natState.MarshalSSZ()
			require.NoError(b, err)
		}
	})
}

func BenchmarkUnmarshalState_FullState(b *testing.B) {
	beaconState, err := benchutil.PreGenState2FullEpochs()
	require.NoError(b, err)
	natState, err := v1.ProtobufBeaconState(beaconState.InnerStateUnsafe())
	require.NoError(b, err)
	protoObject, err := proto.Marshal(natState)
	require.NoError(b, err)
	sszObject, err := natState.MarshalSSZ()
	require.NoError(b, err)

	b.Run("Proto_Unmarshal", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			require.NoError(b, proto.Unmarshal(protoObject, &pb.BeaconState{}))
		}
	})

	b.Run("Fast_SSZ_Unmarshal", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			sszState := &pb.BeaconState{}
			require.NoError(b, sszState.UnmarshalSSZ(sszObject))
		}
	})
}

func clonedStates(beaconState iface.BeaconState) []iface.BeaconState {
	clonedStates := make([]iface.BeaconState, runAmount)
	for i := 0; i < runAmount; i++ {
		clonedStates[i] = beaconState.Copy()
	}
	return clonedStates
}
