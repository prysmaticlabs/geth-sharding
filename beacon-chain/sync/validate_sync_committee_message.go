package sync

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/libp2p/go-libp2p-core/peer"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	types "github.com/prysmaticlabs/eth2-types"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	"github.com/prysmaticlabs/prysm/beacon-chain/p2p"
	p2ptypes "github.com/prysmaticlabs/prysm/beacon-chain/p2p/types"
	prysmv2 "github.com/prysmaticlabs/prysm/proto/prysm/v2"
	"github.com/prysmaticlabs/prysm/shared/bls"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/traceutil"
	"go.opencensus.io/trace"
)

func (s *Service) validateSyncCommitteeMessage(ctx context.Context, pid peer.ID, msg *pubsub.Message) pubsub.ValidationResult {
	ctx, span := trace.StartSpan(ctx, "sync.validateSyncCommitteeMessage")
	defer span.End()

	// Accept the sync committee message if the message came from itself.
	if pid == s.cfg.P2P.PeerID() {
		return pubsub.ValidationAccept
	}

	// Ignore the sync committee message if the beacon node is syncing.
	if s.cfg.InitialSync.Syncing() {
		return pubsub.ValidationIgnore
	}

	if msg.Topic == nil {
		return pubsub.ValidationReject
	}

	raw, err := s.decodePubsubMessage(msg)
	if err != nil {
		log.WithError(err).Debug("Could not decode message")
		traceutil.AnnotateError(span, err)
		return pubsub.ValidationReject
	}

	m, ok := raw.(*prysmv2.SyncCommitteeMessage)
	if !ok {
		return pubsub.ValidationReject
	}
	if m == nil {
		return pubsub.ValidationReject
	}

	// The message's `slot` is for the current slot (with a MAXIMUM_GOSSIP_CLOCK_DISPARITY allowance)
	if err := helpers.VerifySlotTime(uint64(s.cfg.Chain.GenesisTime().Unix()), m.Slot, params.BeaconNetworkConfig().MaximumGossipClockDisparity); err != nil {
		traceutil.AnnotateError(span, err)
		return pubsub.ValidationIgnore
	}

	// The `subnet_id` is valid for the given validator. This implies the validator is part of the broader current sync committee along with the correct subcommittee.
	// Check for validity of validator index.
	pubKey, err := s.cfg.Chain.HeadValidatorIndexToPublicKey(ctx, m.ValidatorIndex)
	if err != nil {
		traceutil.AnnotateError(span, err)
		return pubsub.ValidationReject
	}
	committeeIndices, err := s.cfg.Chain.HeadCurrentSyncCommitteeIndices(ctx, m.ValidatorIndex, m.Slot)
	if err != nil {
		traceutil.AnnotateError(span, err)
		return pubsub.ValidationIgnore
	}
	if len(committeeIndices) == 0 {
		return pubsub.ValidationIgnore
	}

	isValid := false
	digest, err := s.currentForkDigest()
	if err != nil {
		traceutil.AnnotateError(span, err)
		return pubsub.ValidationIgnore
	}

	format := p2p.GossipTypeMapping[reflect.TypeOf(&prysmv2.SyncCommitteeMessage{})]
	// Validate that the validator is in the correct committee.
	subCommitteeSize := params.BeaconConfig().SyncCommitteeSize / params.BeaconConfig().SyncCommitteeSubnetCount
	for _, idx := range committeeIndices {
		subnet := uint64(idx) / subCommitteeSize
		if strings.HasPrefix(*msg.Topic, fmt.Sprintf(format, digest, subnet)) {
			isValid = true
			break
		}
	}
	if !isValid {
		return pubsub.ValidationReject
	}

	// There has been no other valid sync committee signature for the declared `slot`, `validator_index` and `subcommittee_index`.
	// In the event of `validator_index` belongs to multiple subnets, as long as one subnet has not been seen, we should let it in.
	for _, idx := range committeeIndices {
		subnet := uint64(idx) / subCommitteeSize
		if s.hasSeenSyncMessageIndexSlot(m.Slot, m.ValidatorIndex, subnet) {
			isValid = false
		} else {
			isValid = true
		}
	}
	if !isValid {
		return pubsub.ValidationIgnore
	}

	// The signature is valid for the message `beacon_block_root` for the validator referenced by `validator_index`.
	d, err := s.cfg.Chain.HeadSyncCommitteeDomain(ctx, m.Slot)
	if err != nil {
		traceutil.AnnotateError(span, err)
		return pubsub.ValidationIgnore
	}
	rawBytes := p2ptypes.SSZBytes(m.BlockRoot)
	sigRoot, err := helpers.ComputeSigningRoot(&rawBytes, d)
	if err != nil {
		traceutil.AnnotateError(span, err)
		return pubsub.ValidationIgnore
	}

	blsSig, err := bls.SignatureFromBytes(m.Signature)
	if err != nil {
		traceutil.AnnotateError(span, err)
		return pubsub.ValidationReject
	}
	pKey, err := bls.PublicKeyFromBytes(pubKey[:])
	if err != nil {
		traceutil.AnnotateError(span, err)
		return pubsub.ValidationIgnore
	}
	verified := blsSig.Verify(pKey, sigRoot[:])
	if !verified {
		return pubsub.ValidationReject
	}

	for _, idx := range committeeIndices {
		subnet := uint64(idx) / subCommitteeSize
		s.setSeenSyncMessageIndexSlot(m.Slot, m.ValidatorIndex, subnet)
	}

	msg.ValidatorData = m
	return pubsub.ValidationAccept
}

// Returns true if the node has received sync committee for the validator with index and slot.
func (s *Service) hasSeenSyncMessageIndexSlot(slot types.Slot, valIndex types.ValidatorIndex, subCommitteeIndex uint64) bool {
	s.seenSyncMessageLock.RLock()
	defer s.seenSyncMessageLock.RUnlock()

	b := append(bytesutil.Bytes32(uint64(slot)), bytesutil.Bytes32(uint64(valIndex))...)
	b = append(b, bytesutil.Bytes32(subCommitteeIndex)...)
	_, seen := s.seenSyncMessageCache.Get(string(b))
	return seen
}

// Set sync committee message validator index and slot as seen.
func (s *Service) setSeenSyncMessageIndexSlot(slot types.Slot, valIndex types.ValidatorIndex, subCommitteeIndex uint64) {
	s.seenSyncMessageLock.Lock()
	defer s.seenSyncMessageLock.Unlock()

	b := append(bytesutil.Bytes32(uint64(slot)), bytesutil.Bytes32(uint64(valIndex))...)
	b = append(b, bytesutil.Bytes32(subCommitteeIndex)...)
	s.seenSyncMessageCache.Add(string(b), true)
}
