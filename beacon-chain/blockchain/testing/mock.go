package testing

import (
	"bytes"
	"context"
	"time"

	"github.com/pkg/errors"
	ethpb "github.com/prysmaticlabs/ethereumapis/eth/v1alpha1"
	"github.com/prysmaticlabs/go-ssz"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/epoch/precompute"
	opfeed "github.com/prysmaticlabs/prysm/beacon-chain/core/feed/operation"
	statefeed "github.com/prysmaticlabs/prysm/beacon-chain/core/feed/state"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	"github.com/prysmaticlabs/prysm/beacon-chain/db"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	"github.com/prysmaticlabs/prysm/shared/event"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/sirupsen/logrus"
)

// ChainService defines the mock interface for testing
type ChainService struct {
	State                       *pb.BeaconState
	Root                        []byte
	Block                       *ethpb.SignedBeaconBlock
	FinalizedCheckPoint         *ethpb.Checkpoint
	CurrentJustifiedCheckPoint  *ethpb.Checkpoint
	PreviousJustifiedCheckPoint *ethpb.Checkpoint
	BlocksReceived              []*ethpb.SignedBeaconBlock
	Balance                     *precompute.Balance
	Genesis                     time.Time
	Fork                        *pb.Fork
	DB                          db.Database
	stateNotifier               statefeed.Notifier
	opNotifier                  opfeed.Notifier
}

// StateNotifier mocks the same method in the chain service.
func (ms *ChainService) StateNotifier() statefeed.Notifier {
	if ms.stateNotifier == nil {
		ms.stateNotifier = &MockStateNotifier{}
	}
	return ms.stateNotifier
}

// MockStateNotifier mocks the state notifier.
type MockStateNotifier struct {
	feed *event.Feed
}

// StateFeed returns a state feed.
func (msn *MockStateNotifier) StateFeed() *event.Feed {
	if msn.feed == nil {
		msn.feed = new(event.Feed)
	}
	return msn.feed
}

// OperationNotifier mocks the same method in the chain service.
func (ms *ChainService) OperationNotifier() opfeed.Notifier {
	if ms.opNotifier == nil {
		ms.opNotifier = &MockOperationNotifier{}
	}
	return ms.opNotifier
}

// MockOperationNotifier mocks the operation notifier.
type MockOperationNotifier struct {
	feed *event.Feed
}

// OperationFeed returns an operation feed.
func (mon *MockOperationNotifier) OperationFeed() *event.Feed {
	if mon.feed == nil {
		mon.feed = new(event.Feed)
	}
	return mon.feed
}

// ReceiveBlock mocks ReceiveBlock method in chain service.
func (ms *ChainService) ReceiveBlock(ctx context.Context, block *ethpb.SignedBeaconBlock) error {
	return nil
}

// ReceiveBlockNoVerify mocks ReceiveBlockNoVerify method in chain service.
func (ms *ChainService) ReceiveBlockNoVerify(ctx context.Context, block *ethpb.SignedBeaconBlock) error {
	return nil
}

// ReceiveBlockNoPubsub mocks ReceiveBlockNoPubsub method in chain service.
func (ms *ChainService) ReceiveBlockNoPubsub(ctx context.Context, block *ethpb.SignedBeaconBlock) error {
	return nil
}

// ReceiveBlockNoPubsubForkchoice mocks ReceiveBlockNoPubsubForkchoice method in chain service.
func (ms *ChainService) ReceiveBlockNoPubsubForkchoice(ctx context.Context, block *ethpb.SignedBeaconBlock) error {
	if ms.State == nil {
		ms.State = &pb.BeaconState{}
	}
	if !bytes.Equal(ms.Root, block.Block.ParentRoot) {
		return errors.Errorf("wanted %#x but got %#x", ms.Root, block.Block.ParentRoot)
	}
	ms.State.Slot = block.Block.Slot
	ms.BlocksReceived = append(ms.BlocksReceived, block)
	signingRoot, err := ssz.HashTreeRoot(block.Block)
	if err != nil {
		return err
	}
	if ms.DB != nil {
		if err := ms.DB.SaveBlock(ctx, block); err != nil {
			return err
		}
		logrus.Infof("Saved block with root: %#x at slot %d", signingRoot, block.Block.Slot)
	}
	ms.Root = signingRoot[:]
	ms.Block = block
	return nil
}

// HeadSlot mocks HeadSlot method in chain service.
func (ms *ChainService) HeadSlot() uint64 {
	if ms.State == nil {
		return 0
	}
	return ms.State.Slot

}

// HeadRoot mocks HeadRoot method in chain service.
func (ms *ChainService) HeadRoot(ctx context.Context) ([]byte, error) {
	return ms.Root, nil

}

// HeadBlock mocks HeadBlock method in chain service.
func (ms *ChainService) HeadBlock() *ethpb.SignedBeaconBlock {
	return ms.Block
}

// HeadState mocks HeadState method in chain service.
func (ms *ChainService) HeadState(context.Context) (*pb.BeaconState, error) {
	return ms.State, nil
}

// CurrentFork mocks HeadState method in chain service.
func (ms *ChainService) CurrentFork() *pb.Fork {
	return ms.Fork
}

// FinalizedCheckpt mocks FinalizedCheckpt method in chain service.
func (ms *ChainService) FinalizedCheckpt() *ethpb.Checkpoint {
	return ms.FinalizedCheckPoint
}

// CurrentJustifiedCheckpt mocks CurrentJustifiedCheckpt method in chain service.
func (ms *ChainService) CurrentJustifiedCheckpt() *ethpb.Checkpoint {
	return ms.CurrentJustifiedCheckPoint
}

// PreviousJustifiedCheckpt mocks PreviousJustifiedCheckpt method in chain service.
func (ms *ChainService) PreviousJustifiedCheckpt() *ethpb.Checkpoint {
	return ms.PreviousJustifiedCheckPoint
}

// ReceiveAttestation mocks ReceiveAttestation method in chain service.
func (ms *ChainService) ReceiveAttestation(context.Context, *ethpb.Attestation) error {
	return nil
}

// ReceiveAttestationNoPubsub mocks ReceiveAttestationNoPubsub method in chain service.
func (ms *ChainService) ReceiveAttestationNoPubsub(context.Context, *ethpb.Attestation) error {
	return nil
}

// HeadValidatorsIndices mocks the same method in the chain service.
func (ms *ChainService) HeadValidatorsIndices(epoch uint64) ([]uint64, error) {
	if ms.State == nil {
		return []uint64{}, nil
	}
	return helpers.ActiveValidatorIndices(ms.State, epoch)
}

// HeadSeed mocks the same method in the chain service.
func (ms *ChainService) HeadSeed(epoch uint64) ([32]byte, error) {
	return helpers.Seed(ms.State, epoch, params.BeaconConfig().DomainBeaconAttester)
}

// GenesisTime mocks the same method in the chain service.
func (ms *ChainService) GenesisTime() time.Time {
	return ms.Genesis
}

// Participation mocks the same method in the chain service.
func (ms *ChainService) Participation(epoch uint64) *precompute.Balance {
	return ms.Balance
}
