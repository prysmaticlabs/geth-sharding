package sync

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/libp2p/go-libp2p-core/peer"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	eth "github.com/prysmaticlabs/ethereumapis/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/blocks"
	"github.com/prysmaticlabs/prysm/beacon-chain/p2p"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/prysmaticlabs/prysm/shared/featureconfig"
	"github.com/prysmaticlabs/prysm/shared/traceutil"
	"go.opencensus.io/trace"
)

// Validation
// - The attestation's committee index (attestation.data.index) is for the correct subnet.
// - The attestation is unaggregated -- that is, it has exactly one participating validator (len([bit for bit in attestation.aggregation_bits if bit == 0b1]) == 1).
// - The block being voted for (attestation.data.beacon_block_root) passes validation.
// - attestation.data.slot is within the last ATTESTATION_PROPAGATION_SLOT_RANGE slots (attestation.data.slot + ATTESTATION_PROPAGATION_SLOT_RANGE >= current_slot >= attestation.data.slot).
// - The signature of attestation is valid.
func (s *Service) validateCommitteeIndexBeaconAttestation(ctx context.Context, pid peer.ID, msg *pubsub.Message) pubsub.ValidationResult {
	if pid == s.p2p.PeerID() {
		return pubsub.ValidationAccept
	}
	// Attestation processing requires the target block to be present in the database, so we'll skip
	// validating or processing attestations until fully synced.
	if s.initialSync.Syncing() {
		return pubsub.ValidationIgnore
	}
	ctx, span := trace.StartSpan(ctx, "sync.validateCommitteeIndexBeaconAttestation")
	defer span.End()

	// Override topic for decoding.
	originalTopic := msg.TopicIDs[0]
	format := p2p.GossipTypeMapping[reflect.TypeOf(&eth.Attestation{})]
	msg.TopicIDs[0] = format

	m, err := s.decodePubsubMessage(msg)
	if err != nil {
		log.WithError(err).Error("Failed to decode message")
		traceutil.AnnotateError(span, err)
		return pubsub.ValidationReject
	}
	// Restore topic.
	msg.TopicIDs[0] = originalTopic

	att, ok := m.(*eth.Attestation)
	if !ok {
		return pubsub.ValidationReject
	}

	if att.Data == nil {
		return pubsub.ValidationReject
	}
	// Verify this the first attestation received for the participating validator for the slot.
	if s.hasSeenCommitteeIndicesSlot(att.Data.Slot, att.Data.CommitteeIndex, att.AggregationBits) {
		return pubsub.ValidationIgnore
	}

	// The attestation's committee index (attestation.data.index) is for the correct subnet.
	digest, err := s.forkDigest()
	if err != nil {
		log.WithError(err).Error("Failed to compute fork digest")
		traceutil.AnnotateError(span, err)
		return pubsub.ValidationIgnore
	}
	preState, err := s.chain.AttestationPreState(ctx, att)
	if err != nil {
		log.WithError(err).Error("Failed to retrieve pre state")
		traceutil.AnnotateError(span, err)
		return pubsub.ValidationIgnore
	}
	if !strings.HasPrefix(originalTopic, fmt.Sprintf(format, digest, att.Data.CommitteeIndex)) {
		return pubsub.ValidationIgnore
	}

	// Attestation must be unaggregated.
	if att.AggregationBits == nil || att.AggregationBits.Count() != 1 {
		return pubsub.ValidationReject
	}

	// Attestation's slot is within ATTESTATION_PROPAGATION_SLOT_RANGE.
	if err := validateAggregateAttTime(att.Data.Slot, uint64(s.chain.GenesisTime().Unix())); err != nil {
		traceutil.AnnotateError(span, err)
		return pubsub.ValidationIgnore
	}

	// Verify the block being voted and the processed state is in DB and. The block should have passed validation if it's in the DB.
	blockRoot := bytesutil.ToBytes32(att.Data.BeaconBlockRoot)
	hasStateSummary := featureconfig.Get().NewStateMgmt && s.db.HasStateSummary(ctx, blockRoot) || s.stateSummaryCache.Has(blockRoot)
	hasState := s.db.HasState(ctx, blockRoot) || hasStateSummary
	hasBlock := s.db.HasBlock(ctx, blockRoot)
	if !(hasState && hasBlock) {
		// A node doesn't have the block, it'll request from peer while saving the pending attestation to a queue.
		s.savePendingAtt(&eth.SignedAggregateAttestationAndProof{Message: &eth.AggregateAttestationAndProof{Aggregate: att}})
		return pubsub.ValidationIgnore
	}

	// Attestation's signature is a valid BLS signature and belongs to correct public key..
	if !featureconfig.Get().DisableStrictAttestationPubsubVerification {
		if err := blocks.VerifyAttestation(ctx, preState, att); err != nil {
			log.WithError(err).Error("Could not verify attestation")
			traceutil.AnnotateError(span, err)
			return pubsub.ValidationReject
		}
	}

	s.setSeenCommitteeIndicesSlot(att.Data.Slot, att.Data.CommitteeIndex, att.AggregationBits)

	msg.ValidatorData = att

	return pubsub.ValidationAccept
}

// Returns true if the attestation was already seen for the participating validator for the slot.
func (s *Service) hasSeenCommitteeIndicesSlot(slot uint64, committeeID uint64, aggregateBits []byte) bool {
	s.seenAttestationLock.RLock()
	defer s.seenAttestationLock.RUnlock()
	b := append(bytesutil.Bytes32(slot), bytesutil.Bytes32(committeeID)...)
	b = append(b, aggregateBits...)
	_, seen := s.seenAttestationCache.Get(string(b))
	return seen
}

// Set committee's indices and slot as seen for incoming attestations.
func (s *Service) setSeenCommitteeIndicesSlot(slot uint64, committeeID uint64, aggregateBits []byte) {
	s.seenAttestationLock.Lock()
	defer s.seenAttestationLock.Unlock()
	b := append(bytesutil.Bytes32(slot), bytesutil.Bytes32(committeeID)...)
	b = append(b, aggregateBits...)
	s.seenAttestationCache.Add(string(b), true)
}
