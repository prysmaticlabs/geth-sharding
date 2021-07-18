// Package testing includes useful mocks for writing unit
// tests which depend on logic from the blockchain package.
package testing

import (
	"bytes"
	"context"
	"sync"
	"time"

	"github.com/pkg/errors"
	types "github.com/prysmaticlabs/eth2-types"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/epoch/precompute"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/feed"
	blockfeed "github.com/prysmaticlabs/prysm/beacon-chain/core/feed/block"
	opfeed "github.com/prysmaticlabs/prysm/beacon-chain/core/feed/operation"
	statefeed "github.com/prysmaticlabs/prysm/beacon-chain/core/feed/state"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	"github.com/prysmaticlabs/prysm/beacon-chain/db"
	"github.com/prysmaticlabs/prysm/beacon-chain/forkchoice/protoarray"
	iface "github.com/prysmaticlabs/prysm/beacon-chain/state/interface"
	v1 "github.com/prysmaticlabs/prysm/beacon-chain/state/v1"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	ethpb "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/proto/interfaces"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/prysmaticlabs/prysm/shared/event"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/sirupsen/logrus"
)

// ChainService defines the mock interface for testing
type ChainService struct {
	State                       iface.BeaconState
	Root                        []byte
	Block                       interfaces.SignedBeaconBlock
	FinalizedCheckPoint         *ethpb.Checkpoint
	CurrentJustifiedCheckPoint  *ethpb.Checkpoint
	PreviousJustifiedCheckPoint *ethpb.Checkpoint
	BlocksReceived              []interfaces.SignedBeaconBlock
	Balance                     *precompute.Balance
	Genesis                     time.Time
	ValidatorsRoot              [32]byte
	CanonicalRoots              map[[32]byte]bool
	Fork                        *pb.Fork
	ETH1Data                    *ethpb.Eth1Data
	DB                          db.Database
	stateNotifier               statefeed.Notifier
	blockNotifier               blockfeed.Notifier
	opNotifier                  opfeed.Notifier
	ValidAttestation            bool
	ForkChoiceStore             *protoarray.Store
	VerifyBlkDescendantErr      error
	Slot                        *types.Slot // Pointer because 0 is a useful value, so checking against it can be incorrect.
}

// StateNotifier mocks the same method in the chain service.
func (s *ChainService) StateNotifier() statefeed.Notifier {
	if s.stateNotifier == nil {
		s.stateNotifier = &MockStateNotifier{}
	}
	return s.stateNotifier
}

// BlockNotifier mocks the same method in the chain service.
func (s *ChainService) BlockNotifier() blockfeed.Notifier {
	if s.blockNotifier == nil {
		s.blockNotifier = &MockBlockNotifier{}
	}
	return s.blockNotifier
}

// MockBlockNotifier mocks the block notifier.
type MockBlockNotifier struct {
	feed *event.Feed
}

// BlockFeed returns a block feed.
func (mbn *MockBlockNotifier) BlockFeed() *event.Feed {
	if mbn.feed == nil {
		mbn.feed = new(event.Feed)
	}
	return mbn.feed
}

// MockStateNotifier mocks the state notifier.
type MockStateNotifier struct {
	feed     *event.Feed
	feedLock sync.Mutex

	recv     []*feed.Event
	recvLock sync.Mutex
	recvCh   chan *feed.Event

	RecordEvents bool
}

// ReceivedEvents returns the events received by the state feed in this mock.
func (msn *MockStateNotifier) ReceivedEvents() []*feed.Event {
	msn.recvLock.Lock()
	defer msn.recvLock.Unlock()
	return msn.recv
}

// StateFeed returns a state feed.
func (msn *MockStateNotifier) StateFeed() *event.Feed {
	msn.feedLock.Lock()
	defer msn.feedLock.Unlock()

	if msn.feed == nil && msn.recvCh == nil {
		msn.feed = new(event.Feed)
		if msn.RecordEvents {
			msn.recvCh = make(chan *feed.Event)
			sub := msn.feed.Subscribe(msn.recvCh)

			go func() {
				select {
				case evt := <-msn.recvCh:
					msn.recvLock.Lock()
					msn.recv = append(msn.recv, evt)
					msn.recvLock.Unlock()
				case <-sub.Err():
					sub.Unsubscribe()
				}
			}()
		}
	}
	return msn.feed
}

// OperationNotifier mocks the same method in the chain service.
func (s *ChainService) OperationNotifier() opfeed.Notifier {
	if s.opNotifier == nil {
		s.opNotifier = &MockOperationNotifier{}
	}
	return s.opNotifier
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

// ReceiveBlockInitialSync mocks ReceiveBlockInitialSync method in chain service.
func (s *ChainService) ReceiveBlockInitialSync(ctx context.Context, block interfaces.SignedBeaconBlock, _ [32]byte) error {
	if s.State == nil {
		s.State = &v1.BeaconState{}
	}
	if !bytes.Equal(s.Root, block.Block().ParentRoot()) {
		return errors.Errorf("wanted %#x but got %#x", s.Root, block.Block().ParentRoot())
	}
	if err := s.State.SetSlot(block.Block().Slot()); err != nil {
		return err
	}
	s.BlocksReceived = append(s.BlocksReceived, block)
	signingRoot, err := block.Block().HashTreeRoot()
	if err != nil {
		return err
	}
	if s.DB != nil {
		if err := s.DB.SaveBlock(ctx, block); err != nil {
			return err
		}
		logrus.Infof("Saved block with root: %#x at slot %d", signingRoot, block.Block().Slot())
	}
	s.Root = signingRoot[:]
	s.Block = block
	return nil
}

// ReceiveBlockBatch processes blocks in batches from initial-sync.
func (s *ChainService) ReceiveBlockBatch(ctx context.Context, blks []interfaces.SignedBeaconBlock, _ [][32]byte) error {
	if s.State == nil {
		s.State = &v1.BeaconState{}
	}
	for _, block := range blks {
		if !bytes.Equal(s.Root, block.Block().ParentRoot()) {
			return errors.Errorf("wanted %#x but got %#x", s.Root, block.Block().ParentRoot())
		}
		if err := s.State.SetSlot(block.Block().Slot()); err != nil {
			return err
		}
		s.BlocksReceived = append(s.BlocksReceived, block)
		signingRoot, err := block.Block().HashTreeRoot()
		if err != nil {
			return err
		}
		if s.DB != nil {
			if err := s.DB.SaveBlock(ctx, block); err != nil {
				return err
			}
			logrus.Infof("Saved block with root: %#x at slot %d", signingRoot, block.Block().Slot())
		}
		s.Root = signingRoot[:]
		s.Block = block
	}
	return nil
}

// ReceiveBlock mocks ReceiveBlock method in chain service.
func (s *ChainService) ReceiveBlock(ctx context.Context, block interfaces.SignedBeaconBlock, _ [32]byte) error {
	if s.State == nil {
		s.State = &v1.BeaconState{}
	}
	if !bytes.Equal(s.Root, block.Block().ParentRoot()) {
		return errors.Errorf("wanted %#x but got %#x", s.Root, block.Block().ParentRoot())
	}
	if err := s.State.SetSlot(block.Block().Slot()); err != nil {
		return err
	}
	s.BlocksReceived = append(s.BlocksReceived, block)
	signingRoot, err := block.Block().HashTreeRoot()
	if err != nil {
		return err
	}
	if s.DB != nil {
		if err := s.DB.SaveBlock(ctx, block); err != nil {
			return err
		}
		logrus.Infof("Saved block with root: %#x at slot %d", signingRoot, block.Block().Slot())
	}
	s.Root = signingRoot[:]
	s.Block = block
	return nil
}

// HeadSlot mocks HeadSlot method in chain service.
func (s *ChainService) HeadSlot() types.Slot {
	if s.State == nil {
		return 0
	}
	return s.State.Slot()
}

// HeadRoot mocks HeadRoot method in chain service.
func (s *ChainService) HeadRoot(_ context.Context) ([]byte, error) {
	if len(s.Root) > 0 {
		return s.Root, nil
	}
	return make([]byte, 32), nil
}

// HeadBlock mocks HeadBlock method in chain service.
func (s *ChainService) HeadBlock(context.Context) (interfaces.SignedBeaconBlock, error) {
	return s.Block, nil
}

// HeadState mocks HeadState method in chain service.
func (s *ChainService) HeadState(context.Context) (iface.BeaconState, error) {
	return s.State, nil
}

// CurrentFork mocks HeadState method in chain service.
func (s *ChainService) CurrentFork() *pb.Fork {
	return s.Fork
}

// FinalizedCheckpt mocks FinalizedCheckpt method in chain service.
func (s *ChainService) FinalizedCheckpt() *ethpb.Checkpoint {
	return s.FinalizedCheckPoint
}

// CurrentJustifiedCheckpt mocks CurrentJustifiedCheckpt method in chain service.
func (s *ChainService) CurrentJustifiedCheckpt() *ethpb.Checkpoint {
	return s.CurrentJustifiedCheckPoint
}

// PreviousJustifiedCheckpt mocks PreviousJustifiedCheckpt method in chain service.
func (s *ChainService) PreviousJustifiedCheckpt() *ethpb.Checkpoint {
	return s.PreviousJustifiedCheckPoint
}

// ReceiveAttestation mocks ReceiveAttestation method in chain service.
func (s *ChainService) ReceiveAttestation(_ context.Context, _ *ethpb.Attestation) error {
	return nil
}

// ReceiveAttestationNoPubsub mocks ReceiveAttestationNoPubsub method in chain service.
func (s *ChainService) ReceiveAttestationNoPubsub(context.Context, *ethpb.Attestation) error {
	return nil
}

// AttestationPreState mocks AttestationPreState method in chain service.
func (s *ChainService) AttestationPreState(_ context.Context, _ *ethpb.Attestation) (iface.BeaconState, error) {
	return s.State, nil
}

// HeadValidatorsIndices mocks the same method in the chain service.
func (s *ChainService) HeadValidatorsIndices(_ context.Context, epoch types.Epoch) ([]types.ValidatorIndex, error) {
	if s.State == nil {
		return []types.ValidatorIndex{}, nil
	}
	return helpers.ActiveValidatorIndices(s.State, epoch)
}

// HeadSeed mocks the same method in the chain service.
func (s *ChainService) HeadSeed(_ context.Context, epoch types.Epoch) ([32]byte, error) {
	return helpers.Seed(s.State, epoch, params.BeaconConfig().DomainBeaconAttester)
}

// HeadETH1Data provides the current ETH1Data of the head state.
func (s *ChainService) HeadETH1Data() *ethpb.Eth1Data {
	return s.ETH1Data
}

// ProtoArrayStore mocks the same method in the chain service.
func (s *ChainService) ProtoArrayStore() *protoarray.Store {
	return s.ForkChoiceStore
}

// GenesisTime mocks the same method in the chain service.
func (s *ChainService) GenesisTime() time.Time {
	return s.Genesis
}

// GenesisValidatorRoot mocks the same method in the chain service.
func (s *ChainService) GenesisValidatorRoot() [32]byte {
	return s.ValidatorsRoot
}

// CurrentSlot mocks the same method in the chain service.
func (s *ChainService) CurrentSlot() types.Slot {
	if s.Slot != nil {
		return *s.Slot
	}
	return types.Slot(uint64(time.Now().Unix()-s.Genesis.Unix()) / params.BeaconConfig().SecondsPerSlot)
}

// Participation mocks the same method in the chain service.
func (s *ChainService) Participation(_ uint64) *precompute.Balance {
	return s.Balance
}

// IsValidAttestation always returns true.
func (s *ChainService) IsValidAttestation(_ context.Context, _ *ethpb.Attestation) bool {
	return s.ValidAttestation
}

// IsCanonical returns and determines whether a block with the provided root is part of
// the canonical chain.
func (s *ChainService) IsCanonical(_ context.Context, r [32]byte) (bool, error) {
	if s.CanonicalRoots != nil {
		_, ok := s.CanonicalRoots[r]
		return ok, nil
	}
	return true, nil
}

// HasInitSyncBlock mocks the same method in the chain service.
func (s *ChainService) HasInitSyncBlock(_ [32]byte) bool {
	return false
}

// HeadGenesisValidatorRoot mocks HeadGenesisValidatorRoot method in chain service.
func (s *ChainService) HeadGenesisValidatorRoot() [32]byte {
	return [32]byte{}
}

// VerifyBlkDescendant mocks VerifyBlkDescendant and always returns nil.
func (s *ChainService) VerifyBlkDescendant(_ context.Context, _ [32]byte) error {
	return s.VerifyBlkDescendantErr
}

// VerifyLmdFfgConsistency mocks VerifyLmdFfgConsistency and always returns nil.
func (s *ChainService) VerifyLmdFfgConsistency(_ context.Context, a *ethpb.Attestation) error {
	if !bytes.Equal(a.Data.BeaconBlockRoot, a.Data.Target.Root) {
		return errors.New("LMD and FFG miss matched")
	}
	return nil
}

// VerifyFinalizedConsistency mocks VerifyFinalizedConsistency and always returns nil.
func (s *ChainService) VerifyFinalizedConsistency(_ context.Context, r []byte) error {
	if !bytes.Equal(r, s.FinalizedCheckPoint.Root) {
		return errors.New("Root and finalized store are not consistent")
	}
	return nil
}

// ChainHeads mocks ChainHeads and always return nil.
func (s *ChainService) ChainHeads() ([][32]byte, []types.Slot) {
	return [][32]byte{
			bytesutil.ToBytes32(bytesutil.PadTo([]byte("foo"), 32)),
			bytesutil.ToBytes32(bytesutil.PadTo([]byte("bar"), 32)),
		},
		[]types.Slot{0, 1}
}
