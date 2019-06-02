package helpers

import (
	"bytes"
	"encoding/binary"
	"testing"

	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	"github.com/prysmaticlabs/prysm/shared/params"
)

func TestRandaoMix_OK(t *testing.T) {
	randaoMixes := make([][]byte, params.BeaconConfig().LatestRandaoMixesLength)
	for i := 0; i < len(randaoMixes); i++ {
		intInBytes := make([]byte, 32)
		binary.LittleEndian.PutUint64(intInBytes, uint64(i))
		randaoMixes[i] = intInBytes
	}
	state := &pb.BeaconState{LatestRandaoMixes: randaoMixes}
	tests := []struct {
		epoch     uint64
		randaoMix []byte
	}{
		{
			epoch:     10,
			randaoMix: randaoMixes[10],
		},
		{
			epoch:     2344,
			randaoMix: randaoMixes[2344],
		},
		{
			epoch:     99999,
			randaoMix: randaoMixes[99999%params.BeaconConfig().LatestRandaoMixesLength],
		},
	}
	for _, test := range tests {
		state.Slot = (test.epoch + 1) * params.BeaconConfig().SlotsPerEpoch
		mix := RandaoMix(state, test.epoch)
		if !bytes.Equal(test.randaoMix, mix) {
			t.Errorf("Incorrect randao mix. Wanted: %#x, got: %#x",
				test.randaoMix, mix)
		}
	}
}

func TestActiveIndexRoot_OK(t *testing.T) {
	activeIndexRoots := make([][]byte, params.BeaconConfig().LatestActiveIndexRootsLength)
	for i := 0; i < len(activeIndexRoots); i++ {
		intInBytes := make([]byte, 32)
		binary.LittleEndian.PutUint64(intInBytes, uint64(i))
		activeIndexRoots[i] = intInBytes
	}
	state := &pb.BeaconState{LatestActiveIndexRoots: activeIndexRoots}
	tests := []struct {
		epoch uint64
	}{
		{
			epoch: 34,
		},
		{
			epoch: 3444,
		},
		{
			epoch: 999999,
		},
	}
	for _, test := range tests {
		state.Slot = (test.epoch) * params.BeaconConfig().SlotsPerEpoch
		for i := 0; i <= int(params.BeaconConfig().ActivationExitDelay); i++ {
			indexRoot := ActiveIndexRoot(state, test.epoch+uint64(i))

			if !bytes.Equal(activeIndexRoots[(test.epoch+uint64(i))%params.BeaconConfig().LatestActiveIndexRootsLength], indexRoot) {
				t.Errorf("Incorrect index root. Wanted: %#x, got: %#x",
					activeIndexRoots[(test.epoch+uint64(i))%params.BeaconConfig().LatestActiveIndexRootsLength], indexRoot)
			}
		}

	}
}

func TestGenerateSeed_OK(t *testing.T) {
	ClearAllCaches()

	activeIndexRoots := make([][]byte, params.BeaconConfig().LatestActiveIndexRootsLength)
	for i := 0; i < len(activeIndexRoots); i++ {
		intInBytes := make([]byte, 32)
		binary.LittleEndian.PutUint64(intInBytes, uint64(i))
		activeIndexRoots[i] = intInBytes
	}
	randaoMixes := make([][]byte, params.BeaconConfig().LatestRandaoMixesLength)
	for i := 0; i < len(randaoMixes); i++ {
		intInBytes := make([]byte, 32)
		binary.LittleEndian.PutUint64(intInBytes, uint64(i))
		randaoMixes[i] = intInBytes
	}
	slot := 10 * params.BeaconConfig().MinSeedLookahead * params.BeaconConfig().SlotsPerEpoch
	state := &pb.BeaconState{
		LatestActiveIndexRoots: activeIndexRoots,
		LatestRandaoMixes:      randaoMixes,
		Slot:                   slot}

	got := GenerateSeed(state, 10)

	wanted := [32]byte{239, 112, 63, 86, 124, 180, 155, 181, 91, 67, 231, 178,
		94, 149, 243, 101, 176, 169, 153, 35, 37, 19, 115, 154, 6, 102, 125, 91, 81, 153, 186, 84}
	if got != wanted {
		t.Errorf("Incorrect generated seeds. Got: %v, wanted: %v",
			got, wanted)
	}
}
