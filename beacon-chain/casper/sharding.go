package casper

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/prysmaticlabs/prysm/beacon-chain/params"
	"github.com/prysmaticlabs/prysm/beacon-chain/utils"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
)

// BeaconCommittee structure encompassing a specific shard and validator indices
// within that shard's committee.
type BeaconCommittee struct {
	ShardID   int
	Committee []uint32
}

// ValidatorsByHeightShard splits a shuffled validator list by height and by shard,
// it ensures there's enough validators per height and per shard, if not, it'll skip
// some heights and shards.
func ValidatorsByHeightShard(seed common.Hash, activeValidators []*pb.ValidatorRecord, dynasty uint64, crosslinkStartShard uint64) ([]*BeaconCommittee, error) {
	indices := ActiveValidatorIndices(activeValidators, dynasty)
	var committeesPerSlot int
	var slotsPerCommittee int
	var committees []*BeaconCommittee

	if len(indices) >= params.CycleLength*params.MinCommiteeSize {
		committeesPerSlot = len(indices)/params.CycleLength/(params.MinCommiteeSize*2) + 1
		slotsPerCommittee = 1
	} else {
		committeesPerSlot = 1
		slotsPerCommittee = 1
		for len(indices)*slotsPerCommittee < params.MinCommiteeSize && slotsPerCommittee < params.CycleLength {
			slotsPerCommittee *= 2
		}
	}

	// split the shuffled list for heights.
	shuffledList, err := utils.ShuffleIndices(seed, indices)
	if err != nil {
		return nil, err
	}

	heightList := utils.SplitIndices(shuffledList, params.CycleLength)

	// split the shuffled height list for shards
	for i, subList := range heightList {
		shardList := utils.SplitIndices(subList, params.MinCommiteeSize)
		for _, shardIndex := range shardList {
			shardID := int(crosslinkStartShard) + i*committeesPerSlot/slotsPerCommittee
			committees = append(committees, &BeaconCommittee{
				ShardID:   shardID,
				Committee: shardIndex,
			})
		}
	}
	return committees, nil
}
