package blockchain

import (
	"context"
	"time"

	types "github.com/prysmaticlabs/eth2-types"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	"github.com/prysmaticlabs/prysm/beacon-chain/forkchoice/protoarray"
	iface "github.com/prysmaticlabs/prysm/beacon-chain/state/interface"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	ethpb "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/proto/interfaces"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/prysmaticlabs/prysm/shared/copyutil"
	"github.com/prysmaticlabs/prysm/shared/params"
	"go.opencensus.io/trace"
)

// ChainInfoFetcher defines a common interface for methods in blockchain service which
// directly retrieves chain info related data.
type ChainInfoFetcher interface {
	HeadFetcher
	FinalizationFetcher
	GenesisFetcher
	CanonicalFetcher
	ForkFetcher
	TimeFetcher
	HeadDomainFetcher
}

// TimeFetcher retrieves the Ethereum consensus data that's related to time.
type TimeFetcher interface {
	GenesisTime() time.Time
	CurrentSlot() types.Slot
}

// GenesisFetcher retrieves the Ethereum consensus data related to its genesis.
type GenesisFetcher interface {
	GenesisValidatorRoot() [32]byte
}

// HeadFetcher defines a common interface for methods in blockchain service which
// directly retrieves head related data.
type HeadFetcher interface {
	HeadSlot() types.Slot
	HeadRoot(ctx context.Context) ([]byte, error)
	HeadBlock(ctx context.Context) (interfaces.SignedBeaconBlock, error)
	HeadState(ctx context.Context) (iface.BeaconState, error)
	HeadValidatorsIndices(ctx context.Context, epoch types.Epoch) ([]types.ValidatorIndex, error)
	HeadSeed(ctx context.Context, epoch types.Epoch) ([32]byte, error)
	HeadGenesisValidatorRoot() [32]byte
	HeadETH1Data() *ethpb.Eth1Data
	HeadPublicKeyToValidatorIndex(ctx context.Context, pubKey [48]byte) (types.ValidatorIndex, bool)
	HeadValidatorIndexToPublicKey(ctx context.Context, index types.ValidatorIndex) ([48]byte, error)
	ProtoArrayStore() *protoarray.Store
	ChainHeads() ([][32]byte, []types.Slot)
	HeadSyncCommitteeFetcher
	HeadDomainFetcher
}

// ForkFetcher retrieves the current fork information of the Ethereum beacon chain.
type ForkFetcher interface {
	CurrentFork() *pb.Fork
}

// CanonicalFetcher retrieves the current chain's canonical information.
type CanonicalFetcher interface {
	IsCanonical(ctx context.Context, blockRoot [32]byte) (bool, error)
	VerifyBlkDescendant(ctx context.Context, blockRoot [32]byte) error
}

// FinalizationFetcher defines a common interface for methods in blockchain service which
// directly retrieves finalization and justification related data.
type FinalizationFetcher interface {
	FinalizedCheckpt() *ethpb.Checkpoint
	CurrentJustifiedCheckpt() *ethpb.Checkpoint
	PreviousJustifiedCheckpt() *ethpb.Checkpoint
}

// FinalizedCheckpt returns the latest finalized checkpoint from head state.
func (s *Service) FinalizedCheckpt() *ethpb.Checkpoint {
	if s.finalizedCheckpt == nil {
		return &ethpb.Checkpoint{Root: params.BeaconConfig().ZeroHash[:]}
	}

	return copyutil.CopyCheckpoint(s.finalizedCheckpt)
}

// CurrentJustifiedCheckpt returns the current justified checkpoint from head state.
func (s *Service) CurrentJustifiedCheckpt() *ethpb.Checkpoint {
	if s.justifiedCheckpt == nil {
		return &ethpb.Checkpoint{Root: params.BeaconConfig().ZeroHash[:]}
	}

	return copyutil.CopyCheckpoint(s.justifiedCheckpt)
}

// PreviousJustifiedCheckpt returns the previous justified checkpoint from head state.
func (s *Service) PreviousJustifiedCheckpt() *ethpb.Checkpoint {
	if s.prevJustifiedCheckpt == nil {
		return &ethpb.Checkpoint{Root: params.BeaconConfig().ZeroHash[:]}
	}

	return copyutil.CopyCheckpoint(s.prevJustifiedCheckpt)
}

// HeadSlot returns the slot of the head of the chain.
func (s *Service) HeadSlot() types.Slot {
	s.headLock.RLock()
	defer s.headLock.RUnlock()

	if !s.hasHeadState() {
		return 0
	}

	return s.headSlot()
}

// HeadRoot returns the root of the head of the chain.
func (s *Service) HeadRoot(ctx context.Context) ([]byte, error) {
	s.headLock.RLock()
	defer s.headLock.RUnlock()

	if s.headRoot() != params.BeaconConfig().ZeroHash {
		r := s.headRoot()
		return r[:], nil
	}

	b, err := s.cfg.BeaconDB.HeadBlock(ctx)
	if err != nil {
		return nil, err
	}
	if b == nil || b.IsNil() {
		return params.BeaconConfig().ZeroHash[:], nil
	}

	r, err := b.Block().HashTreeRoot()
	if err != nil {
		return nil, err
	}

	return r[:], nil
}

// HeadBlock returns the head block of the chain.
// If the head is nil from service struct,
// it will attempt to get the head block from DB.
func (s *Service) HeadBlock(ctx context.Context) (interfaces.SignedBeaconBlock, error) {
	s.headLock.RLock()
	defer s.headLock.RUnlock()

	if s.hasHeadState() {
		return s.headBlock(), nil
	}

	return s.cfg.BeaconDB.HeadBlock(ctx)
}

// HeadState returns the head state of the chain.
// If the head is nil from service struct,
// it will attempt to get the head state from DB.
func (s *Service) HeadState(ctx context.Context) (iface.BeaconState, error) {
	ctx, span := trace.StartSpan(ctx, "blockChain.HeadState")
	defer span.End()
	s.headLock.RLock()
	defer s.headLock.RUnlock()

	ok := s.hasHeadState()
	span.AddAttributes(trace.BoolAttribute("cache_hit", ok))

	if ok {
		return s.headState(ctx), nil
	}

	return s.cfg.StateGen.StateByRoot(ctx, s.headRoot())
}

// HeadValidatorsIndices returns a list of active validator indices from the head view of a given epoch.
func (s *Service) HeadValidatorsIndices(ctx context.Context, epoch types.Epoch) ([]types.ValidatorIndex, error) {
	s.headLock.RLock()
	defer s.headLock.RUnlock()

	if !s.hasHeadState() {
		return []types.ValidatorIndex{}, nil
	}
	return helpers.ActiveValidatorIndices(s.headState(ctx), epoch)
}

// HeadSeed returns the seed from the head view of a given epoch.
func (s *Service) HeadSeed(ctx context.Context, epoch types.Epoch) ([32]byte, error) {
	s.headLock.RLock()
	defer s.headLock.RUnlock()

	if !s.hasHeadState() {
		return [32]byte{}, nil
	}

	return helpers.Seed(s.headState(ctx), epoch, params.BeaconConfig().DomainBeaconAttester)
}

// HeadGenesisValidatorRoot returns genesis validator root of the head state.
func (s *Service) HeadGenesisValidatorRoot() [32]byte {
	s.headLock.RLock()
	defer s.headLock.RUnlock()

	if !s.hasHeadState() {
		return [32]byte{}
	}

	return s.headGenesisValidatorRoot()
}

// HeadETH1Data returns the eth1data of the current head state.
func (s *Service) HeadETH1Data() *ethpb.Eth1Data {
	s.headLock.RLock()
	defer s.headLock.RUnlock()

	if !s.hasHeadState() {
		return &ethpb.Eth1Data{}
	}
	return s.head.state.Eth1Data()
}

// ProtoArrayStore returns the proto array store object.
func (s *Service) ProtoArrayStore() *protoarray.Store {
	return s.cfg.ForkChoiceStore.Store()
}

// GenesisTime returns the genesis time of beacon chain.
func (s *Service) GenesisTime() time.Time {
	return s.genesisTime
}

// GenesisValidatorRoot returns the genesis validator
// root of the chain.
func (s *Service) GenesisValidatorRoot() [32]byte {
	s.headLock.RLock()
	defer s.headLock.RUnlock()

	if !s.hasHeadState() {
		return [32]byte{}
	}
	return bytesutil.ToBytes32(s.head.state.GenesisValidatorRoot())
}

// CurrentFork retrieves the latest fork information of the beacon chain.
func (s *Service) CurrentFork() *pb.Fork {
	s.headLock.RLock()
	defer s.headLock.RUnlock()

	if !s.hasHeadState() {
		return &pb.Fork{
			PreviousVersion: params.BeaconConfig().GenesisForkVersion,
			CurrentVersion:  params.BeaconConfig().GenesisForkVersion,
		}
	}
	return s.head.state.Fork()
}

// IsCanonical returns true if the input block root is part of the canonical chain.
func (s *Service) IsCanonical(ctx context.Context, blockRoot [32]byte) (bool, error) {
	// If the block has been finalized, the block will always be part of the canonical chain.
	if s.cfg.BeaconDB.IsFinalizedBlock(ctx, blockRoot) {
		return true, nil
	}

	// If the block has not been finalized, check fork choice store to see if the block is canonical
	return s.cfg.ForkChoiceStore.IsCanonical(blockRoot), nil
}

// ChainHeads returns all possible chain heads (leaves of fork choice tree).
// Heads roots and heads slots are returned.
func (s *Service) ChainHeads() ([][32]byte, []types.Slot) {
	nodes := s.ProtoArrayStore().Nodes()

	// Deliberate choice to not preallocate space for below.
	// Heads cant be more than 2-3 in the worst case where pre-allocation will be 64 to begin with.
	headsRoots := make([][32]byte, 0)
	headsSlots := make([]types.Slot, 0)

	nonExistentNode := ^uint64(0)
	for _, node := range nodes {
		// Possible heads have no children.
		if node.BestDescendant() == nonExistentNode && node.BestChild() == nonExistentNode {
			headsRoots = append(headsRoots, node.Root())
			headsSlots = append(headsSlots, node.Slot())
		}
	}

	return headsRoots, headsSlots
}

// HeadPublicKeyToValidatorIndex returns the validator index of the `pubkey` in current head state.
func (s *Service) HeadPublicKeyToValidatorIndex(ctx context.Context, pubKey [48]byte) (types.ValidatorIndex, bool) {
	s.headLock.RLock()
	defer s.headLock.RUnlock()

	return s.headState(ctx).ValidatorIndexByPubkey(pubKey)
}

// HeadValidatorIndexToPublicKey returns the pubkey of the validator `index`  in current head state.
func (s *Service) HeadValidatorIndexToPublicKey(ctx context.Context, index types.ValidatorIndex) ([48]byte, error) {
	s.headLock.RLock()
	defer s.headLock.RUnlock()

	v, err := s.headState(ctx).ValidatorAtIndexReadOnly(index)
	if err != nil {
		return [48]byte{}, err
	}
	return v.PublicKey(), nil
}
