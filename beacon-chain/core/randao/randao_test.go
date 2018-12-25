package randao

import (
	"bytes"
	"testing"

	v "github.com/prysmaticlabs/prysm/beacon-chain/core/validators"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	"github.com/prysmaticlabs/prysm/shared/params"
)

func TestUpdateRandaoLayers(t *testing.T) {
	beaconState := &pb.BeaconState{}
	genesisValidatorRegistry := v.InitialValidatorRegistry()
	beaconState.ValidatorRegistry = genesisValidatorRegistry

	var shardAndCommittees []*pb.ShardAndCommitteeArray
	for i := uint64(0); i < params.BeaconConfig().EpochLength*2; i++ {
		shardAndCommittees = append(shardAndCommittees, &pb.ShardAndCommitteeArray{
			ArrayShardAndCommittee: []*pb.ShardAndCommittee{
				{Committee: []uint32{9, 8, 311, 12, 92, 1, 23, 17}},
			},
		})
	}

	beaconState.ShardAndCommitteesAtSlots = shardAndCommittees

	newState, err := UpdateRandaoLayers(beaconState, 1)
	if err != nil {
		t.Fatalf("failed to update randao layers: %v", err)
	}

	vreg := newState.GetValidatorRegistry()

	// Since slot 1 has proposer index 8
	if vreg[8].GetRandaoLayers() != 1 {
		t.Fatalf("randao layers not updated %d", vreg[9].GetRandaoLayers())
	}

	if vreg[9].GetRandaoLayers() != 0 {
		t.Errorf("randao layers updated when they were not supposed to %d", vreg[9].GetRandaoLayers())
	}
}

func TestUpdateLatestRandaoMixes(t *testing.T) {
	beaconState := &pb.BeaconState{
		LatestRandaoMixesHash32S: make([][]byte, params.BeaconConfig().LatestRandaoMixesLength),
		Slot:                     5,
	}
	beaconState.LatestRandaoMixesHash32S[4%params.BeaconConfig().LatestRandaoMixesLength] = []byte{1, 2, 3}
	beaconState.LatestRandaoMixesHash32S[5%params.BeaconConfig().LatestRandaoMixesLength] = []byte{4, 5, 6}
	newState := UpdateRandaoMixes(beaconState)
	prevSlotMix := newState.LatestRandaoMixesHash32S[4%params.BeaconConfig().LatestRandaoMixesLength]
	currSlotMix := newState.LatestRandaoMixesHash32S[5%params.BeaconConfig().LatestRandaoMixesLength]
	if !bytes.Equal(currSlotMix, prevSlotMix) {
		t.Errorf("Latest randao mix not updated, wanted %#x, received %#x", prevSlotMix, currSlotMix)
	}
}
