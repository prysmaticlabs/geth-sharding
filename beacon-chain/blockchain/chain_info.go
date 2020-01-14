package blockchain

import (
	"bytes"
	"context"
	"time"

	"github.com/gogo/protobuf/proto"
	ethpb "github.com/prysmaticlabs/ethereumapis/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/epoch/precompute"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	"github.com/prysmaticlabs/prysm/shared/params"
)

// ChainInfoFetcher defines a common interface for methods in blockchain service which
// directly retrieves chain info related data.
type ChainInfoFetcher interface {
	HeadFetcher
	FinalizationFetcher
}

// GenesisTimeFetcher retrieves the Eth2 genesis timestamp.
type GenesisTimeFetcher interface {
	GenesisTime() time.Time
}

// HeadFetcher defines a common interface for methods in blockchain service which
// directly retrieves head related data.
type HeadFetcher interface {
	HeadSlot() uint64
	HeadRoot() []byte
	HeadBlock() *ethpb.SignedBeaconBlock
	HeadState(ctx context.Context) (*pb.BeaconState, error)
	HeadValidatorsIndices(epoch uint64) ([]uint64, error)
	HeadSeed(epoch uint64) ([32]byte, error)
}

// ForkFetcher retrieves the current fork information of the Ethereum beacon chain.
type ForkFetcher interface {
	CurrentFork() *pb.Fork
}

// FinalizationFetcher defines a common interface for methods in blockchain service which
// directly retrieves finalization and justification related data.
type FinalizationFetcher interface {
	FinalizedCheckpt() *ethpb.Checkpoint
	CurrentJustifiedCheckpt() *ethpb.Checkpoint
	PreviousJustifiedCheckpt() *ethpb.Checkpoint
}

// ParticipationFetcher defines a common interface for methods in blockchain service which
// directly retrieves validator participation related data.
type ParticipationFetcher interface {
	Participation(epoch uint64) *precompute.Balance
}

// FinalizedCheckpt returns the latest finalized checkpoint from head state.
func (s *Service) FinalizedCheckpt() *ethpb.Checkpoint {
	if s.headState == nil || s.headState.FinalizedCheckpoint == nil {
		return &ethpb.Checkpoint{Root: params.BeaconConfig().ZeroHash[:]}
	}

	// If head state exists but there hasn't been a finalized check point,
	// the check point's root should refer to genesis block root.
	if bytes.Equal(s.headState.FinalizedCheckpoint.Root, params.BeaconConfig().ZeroHash[:]) {
		return &ethpb.Checkpoint{Root: s.genesisRoot[:]}
	}

	return proto.Clone(s.headState.FinalizedCheckpoint).(*ethpb.Checkpoint)
}

// CurrentJustifiedCheckpt returns the current justified checkpoint from head state.
func (s *Service) CurrentJustifiedCheckpt() *ethpb.Checkpoint {
	if s.headState == nil || s.headState.CurrentJustifiedCheckpoint == nil {
		return &ethpb.Checkpoint{Root: params.BeaconConfig().ZeroHash[:]}
	}

	// If head state exists but there hasn't been a justified check point,
	// the check point root should refer to genesis block root.
	if bytes.Equal(s.headState.CurrentJustifiedCheckpoint.Root, params.BeaconConfig().ZeroHash[:]) {
		return &ethpb.Checkpoint{Root: s.genesisRoot[:]}
	}

	return proto.Clone(s.headState.CurrentJustifiedCheckpoint).(*ethpb.Checkpoint)
}

// PreviousJustifiedCheckpt returns the previous justified checkpoint from head state.
func (s *Service) PreviousJustifiedCheckpt() *ethpb.Checkpoint {
	if s.headState == nil || s.headState.PreviousJustifiedCheckpoint == nil {
		return &ethpb.Checkpoint{Root: params.BeaconConfig().ZeroHash[:]}
	}

	// If head state exists but there hasn't been a justified check point,
	// the check point root should refer to genesis block root.
	if bytes.Equal(s.headState.PreviousJustifiedCheckpoint.Root, params.BeaconConfig().ZeroHash[:]) {
		return &ethpb.Checkpoint{Root: s.genesisRoot[:]}
	}

	return proto.Clone(s.headState.PreviousJustifiedCheckpoint).(*ethpb.Checkpoint)
}

// HeadSlot returns the slot of the head of the chain.
func (s *Service) HeadSlot() uint64 {
	s.headLock.RLock()
	defer s.headLock.RUnlock()

	return s.headSlot
}

// HeadRoot returns the root of the head of the chain.
func (s *Service) HeadRoot() []byte {
	s.headLock.RLock()
	defer s.headLock.RUnlock()

	root := s.canonicalRoots[s.headSlot]
	if len(root) != 0 {
		return root
	}

	return params.BeaconConfig().ZeroHash[:]
}

// HeadBlock returns the head block of the chain.
func (s *Service) HeadBlock() *ethpb.SignedBeaconBlock {
	s.headLock.RLock()
	defer s.headLock.RUnlock()

	return proto.Clone(s.headBlock).(*ethpb.SignedBeaconBlock)
}

// HeadState returns the head state of the chain.
// If the head state is nil from service struct,
// it will attempt to get from DB and error if nil again.
func (s *Service) HeadState(ctx context.Context) (*pb.BeaconState, error) {
	s.headLock.RLock()
	defer s.headLock.RUnlock()

	if s.headState == nil {
		return s.beaconDB.HeadState(ctx)
	}

	return proto.Clone(s.headState).(*pb.BeaconState), nil
}

// HeadValidatorsIndices returns a list of active validator indices from the head view of a given epoch.
func (s *Service) HeadValidatorsIndices(epoch uint64) ([]uint64, error) {
	if s.headState == nil {
		return []uint64{}, nil
	}
	return helpers.ActiveValidatorIndices(s.headState, epoch)
}

// HeadSeed returns the seed from the head view of a given epoch.
func (s *Service) HeadSeed(epoch uint64) ([32]byte, error) {
	if s.headState == nil {
		return [32]byte{}, nil
	}

	return helpers.Seed(s.headState, epoch, params.BeaconConfig().DomainBeaconAttester)
}

// GenesisTime returns the genesis time of beacon chain.
func (s *Service) GenesisTime() time.Time {
	return s.genesisTime
}

// CurrentFork retrieves the latest fork information of the beacon chain.
func (s *Service) CurrentFork() *pb.Fork {
	if s.headState == nil {
		return &pb.Fork{
			PreviousVersion: params.BeaconConfig().GenesisForkVersion,
			CurrentVersion:  params.BeaconConfig().GenesisForkVersion,
		}
	}
	return proto.Clone(s.headState.Fork).(*pb.Fork)
}

// Participation returns the participation stats of a given epoch.
func (s *Service) Participation(epoch uint64) *precompute.Balance {
	s.epochParticipationLock.RLock()
	defer s.epochParticipationLock.RUnlock()

	return s.epochParticipation[epoch]
}
