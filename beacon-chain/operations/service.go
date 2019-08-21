// Package operations defines the life-cycle of beacon block operations.
package operations

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/gogo/protobuf/proto"
	"github.com/pkg/errors"
	"github.com/prysmaticlabs/go-ssz"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	ethpb "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	"github.com/sirupsen/logrus"
	"go.opencensus.io/trace"

	"github.com/prysmaticlabs/prysm/beacon-chain/core/blocks"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/state"
	"github.com/prysmaticlabs/prysm/beacon-chain/db"
	"github.com/prysmaticlabs/prysm/beacon-chain/p2p"
	"github.com/prysmaticlabs/prysm/shared/bls"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/prysmaticlabs/prysm/shared/event"
	"github.com/prysmaticlabs/prysm/shared/hashutil"
	handler "github.com/prysmaticlabs/prysm/shared/messagehandler"
	"github.com/prysmaticlabs/prysm/shared/params"
)

var log = logrus.WithField("prefix", "operation")

// Pool defines an interface for fetching the list of attestations
// which have been observed by the beacon node but not yet included in
// a beacon block by a proposer.
type Pool interface {
	AttestationPool(ctx context.Context, requestedSlot uint64) ([]*ethpb.Attestation, error)
}

// OperationFeeds inteface defines the informational feeds from the operations
// service.
type OperationFeeds interface {
	IncomingAttFeed() *event.Feed
	IncomingExitFeed() *event.Feed
	IncomingProcessedBlockFeed() *event.Feed
}

// Service represents a service that handles the internal
// logic of beacon block operations.
type Service struct {
	ctx                        context.Context
	cancel                     context.CancelFunc
	beaconDB                   db.Database
	incomingExitFeed           *event.Feed
	incomingValidatorExits     chan *ethpb.VoluntaryExit
	incomingAttFeed            *event.Feed
	incomingAtt                chan *ethpb.Attestation
	incomingProcessedBlockFeed *event.Feed
	incomingProcessedBlock     chan *ethpb.BeaconBlock
	p2p                        p2p.Broadcaster
	error                      error
	attestationLock            sync.Mutex
}

// Config options for the service.
type Config struct {
	BeaconDB db.Database
	P2P      p2p.Broadcaster
}

// NewOpsPoolService instantiates a new service instance that will
// be registered into a running beacon node.
func NewOpsPoolService(ctx context.Context, cfg *Config) *Service {
	ctx, cancel := context.WithCancel(ctx)
	return &Service{
		ctx:                        ctx,
		cancel:                     cancel,
		beaconDB:                   cfg.BeaconDB,
		incomingExitFeed:           new(event.Feed),
		incomingValidatorExits:     make(chan *ethpb.VoluntaryExit, params.BeaconConfig().DefaultBufferSize),
		incomingAttFeed:            new(event.Feed),
		incomingAtt:                make(chan *ethpb.Attestation, params.BeaconConfig().DefaultBufferSize),
		incomingProcessedBlockFeed: new(event.Feed),
		incomingProcessedBlock:     make(chan *ethpb.BeaconBlock, params.BeaconConfig().DefaultBufferSize),
		p2p:                        cfg.P2P,
	}
}

// Start an beacon block operation pool service's main event loop.
func (s *Service) Start() {
	log.Info("Starting service")
	go s.saveOperations()
	go s.removeOperations()
}

// Stop the beacon block operation pool service's main event loop
// and associated goroutines.
func (s *Service) Stop() error {
	defer s.cancel()
	log.Info("Stopping service")
	return nil
}

// Status returns the current service error if there's any.
func (s *Service) Status() error {
	if s.error != nil {
		return s.error
	}
	return nil
}

// IncomingExitFeed returns a feed that any service can send incoming p2p exits object into.
// The beacon block operation pool service will subscribe to this feed in order to relay incoming exits.
func (s *Service) IncomingExitFeed() *event.Feed {
	return s.incomingExitFeed
}

// IncomingAttFeed returns a feed that any service can send incoming p2p attestations into.
// The beacon block operation pool service will subscribe to this feed in order to relay incoming attestations.
func (s *Service) IncomingAttFeed() *event.Feed {
	return s.incomingAttFeed
}

// IncomingProcessedBlockFeed returns a feed that any service can send incoming p2p beacon blocks into.
// The beacon block operation pool service will subscribe to this feed in order to receive incoming beacon blocks.
func (s *Service) IncomingProcessedBlockFeed() *event.Feed {
	return s.incomingProcessedBlockFeed
}

// AttestationPool returns the attestations that have not seen on the beacon chain,
// the attestations are returned in slot ascending order and up to MaxAttestations
// capacity. The attestations get deleted in DB after they have been retrieved.
func (s *Service) AttestationPool(ctx context.Context, requestedSlot uint64) ([]*ethpb.Attestation, error) {
	var attestations []*ethpb.Attestation
	attestationsFromDB, err := s.beaconDB.Attestations(ctx, nil /*filter*/)
	if err != nil {
		return nil, errors.New("could not retrieve attestations from DB")
	}
	bState, err := s.beaconDB.HeadState(ctx)
	if err != nil {
		return nil, errors.New("could not retrieve attestations from DB")
	}

	bState, err = state.ProcessSlots(ctx, bState, requestedSlot)
	if err != nil {
		return nil, errors.Wrapf(err, "could not process slots up to %d", requestedSlot)
	}

	sort.Slice(attestationsFromDB, func(i, j int) bool {
		return attestationsFromDB[i].Data.Crosslink.Shard < attestationsFromDB[j].Data.Crosslink.Shard
	})

	var validAttsCount uint64
	for _, att := range attestationsFromDB {
		slot, err := helpers.AttestationDataSlot(bState, att.Data)
		if err != nil {
			return nil, errors.Wrap(err, "could not get attestation slot")
		}
		// Delete the attestation if the attestation is one epoch older than head state,
		// we don't want to pass these attestations to RPC for proposer to include.
		if slot+params.BeaconConfig().SlotsPerEpoch <= bState.Slot {
			hash, err := ssz.HashTreeRoot(att)
			if err != nil {
				return nil, err
			}
			if err := s.beaconDB.DeleteAttestation(ctx, hash); err != nil {
				return nil, err
			}
			continue
		}

		validAttsCount++
		// Stop the max attestation number per beacon block is reached.
		if validAttsCount == params.BeaconConfig().MaxAttestations {
			break
		}

		attestations = append(attestations, att)
	}
	return attestations, nil
}

// saveOperations saves the newly broadcasted beacon block operations
// that was received from sync service.
func (s *Service) saveOperations() {
	// TODO(1438): Add rest of operations (slashings, attestation, exists...etc)
	incomingSub := s.incomingExitFeed.Subscribe(s.incomingValidatorExits)
	defer incomingSub.Unsubscribe()
	incomingAttSub := s.incomingAttFeed.Subscribe(s.incomingAtt)
	defer incomingAttSub.Unsubscribe()

	for {
		select {
		case <-incomingSub.Err():
			log.Debug("Subscriber closed, exiting goroutine")
			return
		case <-s.ctx.Done():
			log.Debug("operations service context closed, exiting save goroutine")
			return
		// Listen for a newly received incoming exit from the sync service.
		case exit := <-s.incomingValidatorExits:
			handler.SafelyHandleMessage(s.ctx, s.HandleValidatorExits, exit)
		case attestation := <-s.incomingAtt:
			handler.SafelyHandleMessage(s.ctx, s.HandleAttestation, attestation)
		}
	}
}

// HandleValidatorExits processes a validator exit operation.
func (s *Service) HandleValidatorExits(ctx context.Context, message proto.Message) error {
	ctx, span := trace.StartSpan(ctx, "operations.HandleValidatorExits")
	defer span.End()

	exit := message.(*ethpb.VoluntaryExit)
	hash, err := hashutil.HashProto(exit)
	if err != nil {
		return err
	}
	// TODO
	if err := s.beaconDB.(*db.BeaconDB).SaveExit(ctx, exit); err != nil {
		return err
	}
	log.WithField("hash", fmt.Sprintf("%#x", hash)).Info("Exit request saved in DB")
	return nil
}

// HandleAttestation processes a received attestation message.
func (s *Service) HandleAttestation(ctx context.Context, message proto.Message) error {
	ctx, span := trace.StartSpan(ctx, "operations.HandleAttestation")
	defer span.End()
	s.attestationLock.Lock()
	defer s.attestationLock.Unlock()

	attestation := message.(*ethpb.Attestation)

	bState, err := s.beaconDB.HeadState(ctx)
	if err != nil {
		return err
	}

	attestationSlot := attestation.Data.Target.Epoch * params.BeaconConfig().SlotsPerEpoch
	if attestationSlot > bState.Slot {
		bState, err = state.ProcessSlots(ctx, bState, attestationSlot)
		if err != nil {
			return err
		}
	}

	if err := blocks.VerifyAttestation(bState, attestation); err != nil {
		return err
	}

	hash, err := hashutil.HashProto(attestation.Data)
	if err != nil {
		return err
	}

	incomingAttBits := attestation.AggregationBits
	if s.beaconDB.HasAttestation(ctx, hash) {
		dbAtt, err := s.beaconDB.Attestation(ctx, hash)
		if err != nil {
			return err
		}

		if !dbAtt.AggregationBits.Contains(incomingAttBits) {
			newAggregationBits := dbAtt.AggregationBits.Or(incomingAttBits)
			incomingAttSig, err := bls.SignatureFromBytes(attestation.Signature)
			if err != nil {
				return err
			}
			dbSig, err := bls.SignatureFromBytes(dbAtt.Signature)
			if err != nil {
				return err
			}
			aggregatedSig := bls.AggregateSignatures([]*bls.Signature{dbSig, incomingAttSig})
			dbAtt.Signature = aggregatedSig.Marshal()
			dbAtt.AggregationBits = newAggregationBits
			if err := s.beaconDB.SaveAttestation(ctx, dbAtt); err != nil {
				return err
			}
		} else {
			return nil
		}
	} else {
		if err := s.beaconDB.SaveAttestation(ctx, attestation); err != nil {
			return err
		}
	}
	return nil
}

// IsAttCanonical returns true if the input attestation is voting on the canonical chain, false
// otherwise. The steps to verify are:
//	1.) retrieve the voted block
//	2.) retrieve the canonical block by using voted block's slot number
//	3.) return true if voted block root and the canonical block root are the same
func (s *Service) IsAttCanonical(ctx context.Context, att *ethpb.Attestation) (bool, error) {
	votedBlk, err := s.beaconDB.Block(ctx, bytesutil.ToBytes32(att.Data.BeaconBlockRoot))
	if err != nil {
		return false, errors.Wrap(err, "could not hash block")
	}
	if votedBlk == nil {
		return false, nil
	}
	// TODO(3219): Replace with new fork choice service.
	canonicalBlk, err := s.beaconDB.(*db.BeaconDB).CanonicalBlockBySlot(ctx, votedBlk.Slot)
	if err != nil {
		return false, errors.Wrap(err, "could not hash block")
	}
	if canonicalBlk == nil {
		return false, nil
	}
	canonicalRoot, err := ssz.SigningRoot(canonicalBlk)
	if err != nil {
		return false, errors.Wrap(err, "could not hash block")
	}
	return bytes.Equal(att.Data.BeaconBlockRoot, canonicalRoot[:]), nil
}

// removeOperations removes the processed operations from operation pool and DB.
func (s *Service) removeOperations() {
	incomingBlockSub := s.incomingProcessedBlockFeed.Subscribe(s.incomingProcessedBlock)
	defer incomingBlockSub.Unsubscribe()

	for {
		select {
		case <-incomingBlockSub.Err():
			log.Debug("Subscriber closed, exiting goroutine")
		case <-s.ctx.Done():
			log.Debug("operations service context closed, exiting remove goroutine")
		// Listen for processed block from the block chain service.
		case block := <-s.incomingProcessedBlock:
			handler.SafelyHandleMessage(s.ctx, s.handleProcessedBlock, block)
		}
	}
}

func (s *Service) handleProcessedBlock(ctx context.Context, message proto.Message) error {
	block := message.(*ethpb.BeaconBlock)
	// Removes the attestations from the pool that have been included
	// in the received block.
	if err := s.removeAttestationsFromPool(ctx, block.Body.Attestations); err != nil {
		return errors.Wrap(err, "could not remove processed attestations from DB")
	}
	state, err := s.beaconDB.HeadState(s.ctx)
	if err != nil {
		return errors.New("could not retrieve attestations from DB")
	}
	if err := s.removeEpochOldAttestations(ctx, state); err != nil {
		return errors.Wrapf(err, "could not remove old attestations from DB at slot %d", block.Slot)
	}
	return nil
}

// removeAttestationsFromPool removes a list of attestations from the DB
// after they have been included in a beacon block.
func (s *Service) removeAttestationsFromPool(ctx context.Context, attestations []*ethpb.Attestation) error {
	for _, attestation := range attestations {
		hash, err := hashutil.HashProto(attestation.Data)
		if err != nil {
			return err
		}
		if s.beaconDB.HasAttestation(ctx, hash) {
			if err := s.beaconDB.DeleteAttestation(ctx, hash); err != nil {
				return err
			}
			log.WithField("root", fmt.Sprintf("%#x", hash)).Debug("AttestationDeprecated removed")
		}
	}
	return nil
}

// removeEpochOldAttestations removes attestations that's older than one epoch length from current slot.
func (s *Service) removeEpochOldAttestations(ctx context.Context, beaconState *pb.BeaconState) error {
	attestations, err := s.beaconDB.Attestations(ctx, nil /*filter*/)
	if err != nil {
		return err
	}
	for _, a := range attestations {
		slot, err := helpers.AttestationDataSlot(beaconState, a.Data)
		if err != nil {
			return errors.Wrap(err, "could not get attestation slot")
		}
		// Remove attestation from DB if it's one epoch older than slot.
		if slot-params.BeaconConfig().SlotsPerEpoch >= slot {
			hash, err := ssz.HashTreeRoot(a)
			if err != nil {
				return err
			}
			if err := s.beaconDB.DeleteAttestation(ctx, hash); err != nil {
				return err
			}
		}
	}
	return nil
}
