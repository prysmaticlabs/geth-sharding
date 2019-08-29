package blockchain

import (
	"time"

	"github.com/gogo/protobuf/proto"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	ethpb "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
)

// ChainInfoRetriever defines a common interface for methods in blockchain service which
// directly retrieves chain info related data.
type ChainInfoRetriever interface {
	HeadRetriever
	CanonicalRetriever
	FinalizationRetriever
	GenesisTime() time.Time
}

// HeadRetriever defines a common interface for methods in blockchain service which
// directly retrieves head related data.
type HeadRetriever interface {
	HeadSlot() uint64
	HeadRoot() []byte
	HeadBlock() *ethpb.BeaconBlock
	HeadState() *pb.BeaconState
}

// CanonicalRetriever defines a common interface for methods in blockchain service which
// directly retrieves canonical roots related data.
type CanonicalRetriever interface {
	CanonicalRoot(slot uint64) []byte
}

// FinalizationRetriever defines a common interface for methods in blockchain service which
// directly retrieves finalization related data.
type FinalizationRetriever interface {
	FinalizedCheckpt() *ethpb.Checkpoint
}

// FinalizedCheckpt returns the latest finalized checkpoint tracked in fork choice service.
func (c *ChainService) FinalizedCheckpt() *ethpb.Checkpoint {
	return c.forkChoiceStore.FinalizedCheckpt()
}

// HeadSlot returns the slot of the head of the chain.
func (c *ChainService) HeadSlot() uint64 {
	return c.headSlot
}

// HeadRoot returns the root of the head of the chain.
func (c *ChainService) HeadRoot() []byte {
	c.canonicalRootsLock.RLock()
	defer c.canonicalRootsLock.RUnlock()

	return c.canonicalRoots[c.headSlot]
}

// HeadBlock returns the head block of the chain.
func (c *ChainService) HeadBlock() *ethpb.BeaconBlock {
	return proto.Clone(c.headBlock).(*ethpb.BeaconBlock)
}

// HeadState returns the head state of the chain.
func (c *ChainService) HeadState() *pb.BeaconState {
	return proto.Clone(c.headState).(*pb.BeaconState)
}

// CanonicalRoot returns the canonical root of a given slot.
func (c *ChainService) CanonicalRoot(slot uint64) []byte {
	c.canonicalRootsLock.RLock()
	defer c.canonicalRootsLock.RUnlock()

	return c.canonicalRoots[slot]
}

// GenesisTime returns the genesis time of beacon chain.
func (c *ChainService) GenesisTime() time.Time {
	return c.genesisTime
}
