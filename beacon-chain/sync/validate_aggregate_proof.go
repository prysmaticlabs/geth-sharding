package sync

import (
	"context"
	"fmt"

	"github.com/libp2p/go-libp2p-core/peer"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/pkg/errors"
	ethpb "github.com/prysmaticlabs/ethereumapis/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/blocks"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/state"
	stateTrie "github.com/prysmaticlabs/prysm/beacon-chain/state"
	"github.com/prysmaticlabs/prysm/shared/attestationutil"
	"github.com/prysmaticlabs/prysm/shared/bls"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/prysmaticlabs/prysm/shared/featureconfig"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/roughtime"
	"github.com/prysmaticlabs/prysm/shared/traceutil"
	"go.opencensus.io/trace"
)

// validateAggregateAndProof verifies the aggregated signature and the selection proof is valid before forwarding to the
// network and downstream services.
func (r *Service) validateAggregateAndProof(ctx context.Context, pid peer.ID, msg *pubsub.Message) bool {
	if pid == r.p2p.PeerID() {
		return true
	}

	ctx, span := trace.StartSpan(ctx, "sync.validateAggregateAndProof")
	defer span.End()

	// To process the following it requires the recent blocks to be present in the database, so we'll skip
	// validating or processing aggregated attestations until fully synced.
	if r.initialSync.Syncing() {
		return false
	}

	raw, err := r.decodePubsubMessage(msg)
	if err != nil {
		log.WithError(err).Error("Failed to decode message")
		traceutil.AnnotateError(span, err)
		return false
	}
	m, ok := raw.(*ethpb.SignedAggregateAttestationAndProof)
	if !ok {
		return false
	}

	if m.Message == nil || m.Message.Aggregate == nil || m.Message.Aggregate.Data == nil {
		return false
	}
	// Verify this is the first aggregate received from the aggregator with index and slot.
	if r.hasSeenAggregatorIndexSlot(m.Message.Aggregate.Data.Slot, m.Message.AggregatorIndex) {
		return false
	}

	// Verify aggregate attestation has not already been seen via aggregate gossip, within a block, or through the creation locally.
	seen, err := r.attPool.HasAggregatedAttestation(m.Message.Aggregate)
	if err != nil {
		traceutil.AnnotateError(span, err)
		return false
	}
	if seen {
		return false
	}
	if !r.validateBlockInAttestation(ctx, m.Message) {
		return false
	}

	if !r.validateAggregatedAtt(ctx, m.Message) {
		return false
	}

	if !featureconfig.Get().DisableStrictAttestationPubsubVerification && !r.chain.IsValidAttestation(ctx, m.Message.Aggregate) {
		return false
	}

	r.setAggregatorIndexSlotSeen(m.Message.Aggregate.Data.Slot, m.Message.AggregatorIndex)

	msg.ValidatorData = m

	return true
}

func (r *Service) validateAggregatedAtt(ctx context.Context, a *ethpb.AggregateAttestationAndProof) bool {
	ctx, span := trace.StartSpan(ctx, "sync.validateAggregatedAtt")
	defer span.End()

	attSlot := a.Aggregate.Data.Slot
	if err := validateAggregateAttTime(attSlot, uint64(r.chain.GenesisTime().Unix())); err != nil {
		traceutil.AnnotateError(span, err)
		return false
	}

	s, err := r.chain.HeadState(ctx)
	if err != nil {
		traceutil.AnnotateError(span, err)
		return false
	}

	// Only advance state if different epoch as the committee can only change on an epoch transition.
	if helpers.SlotToEpoch(attSlot) > helpers.SlotToEpoch(s.Slot()) {
		s, err = state.ProcessSlots(ctx, s, helpers.StartSlot(helpers.SlotToEpoch(attSlot)))
		if err != nil {
			traceutil.AnnotateError(span, err)
			return false
		}
	}

	// Verify validator index is within the aggregate's committee.
	if err := validateIndexInCommittee(ctx, s, a.Aggregate, a.AggregatorIndex); err != nil {
		traceutil.AnnotateError(span, errors.Wrapf(err, "Could not validate index in committee"))
		return false
	}

	// Verify selection proof reflects to the right validator and signature is valid.
	if err := validateSelection(ctx, s, a.Aggregate.Data, a.AggregatorIndex, a.SelectionProof); err != nil {
		traceutil.AnnotateError(span, errors.Wrapf(err, "Could not validate selection for validator %d", a.AggregatorIndex))
		return false
	}

	// Verify the aggregator's signature is valid.
	if err := validateAggregatorSignature(s, a); err != nil {
		return false
	}

	// Verify aggregated attestation has a valid signature.
	if err := blocks.VerifyAttestation(ctx, s, a.Aggregate); err != nil {
		traceutil.AnnotateError(span, err)
		return false
	}

	return true
}

func (r *Service) validateBlockInAttestation(ctx context.Context, a *ethpb.AggregateAttestationAndProof) bool {
	// Verify the block being voted and the processed state is in DB. The block should have passed validation if it's in the DB.
	hasStateSummary := featureconfig.Get().NewStateMgmt && r.db.HasStateSummary(ctx, bytesutil.ToBytes32(a.Aggregate.Data.BeaconBlockRoot))
	hasState := r.db.HasState(ctx, bytesutil.ToBytes32(a.Aggregate.Data.BeaconBlockRoot)) || hasStateSummary
	hasBlock := r.db.HasBlock(ctx, bytesutil.ToBytes32(a.Aggregate.Data.BeaconBlockRoot))
	if !(hasState && hasBlock) {
		// A node doesn't have the block, it'll request from peer while saving the pending attestation to a queue.
		r.savePendingAtt(a)
		return false
	}
	return true
}

// Returns true if the node has received aggregate for the aggregator with index and slot.
func (r *Service) hasSeenAggregatorIndexSlot(slot uint64, aggregatorIndex uint64) bool {
	r.seenAttestationLock.RLock()
	defer r.seenAttestationLock.RUnlock()
	b := append(bytesutil.Bytes32(slot), bytesutil.Bytes32(aggregatorIndex)...)
	_, seen := r.seenAttestationCache.Get(string(b))
	return seen
}

// Set aggregate's aggregator index slot as seen.
func (r *Service) setAggregatorIndexSlotSeen(slot uint64, aggregatorIndex uint64) {
	r.seenAttestationLock.Lock()
	defer r.seenAttestationLock.Unlock()
	b := append(bytesutil.Bytes32(slot), bytesutil.Bytes32(aggregatorIndex)...)
	r.seenAttestationCache.Add(string(b), true)
}

// This validates the aggregator's index in state is within the attesting indices of the attestation.
func validateIndexInCommittee(ctx context.Context, s *stateTrie.BeaconState, a *ethpb.Attestation, validatorIndex uint64) error {
	ctx, span := trace.StartSpan(ctx, "sync.validateIndexInCommittee")
	defer span.End()

	committee, err := helpers.BeaconCommitteeFromState(s, a.Data.Slot, a.Data.CommitteeIndex)
	if err != nil {
		return err
	}
	attestingIndices := attestationutil.AttestingIndices(a.AggregationBits, committee)
	var withinCommittee bool
	for _, i := range attestingIndices {
		if validatorIndex == i {
			withinCommittee = true
			break
		}
	}
	if !withinCommittee {
		return fmt.Errorf("validator index %d is not within the committee: %v",
			validatorIndex, attestingIndices)
	}
	return nil
}

// Validates that the incoming aggregate attestation is in the desired time range.
func validateAggregateAttTime(attSlot uint64, genesisTime uint64) error {
	// in milliseconds
	attTime := 1000 * (genesisTime + (attSlot * params.BeaconConfig().SecondsPerSlot))
	attSlotRange := attSlot + params.BeaconConfig().AttestationPropagationSlotRange
	attTimeRange := 1000 * (genesisTime + (attSlotRange * params.BeaconConfig().SecondsPerSlot))
	currentTimeInSec := roughtime.Now().Unix()
	currentTime := 1000 * currentTimeInSec

	// Verify attestation slot is within the last ATTESTATION_PROPAGATION_SLOT_RANGE slots.
	currentSlot := (uint64(currentTimeInSec) - genesisTime) / params.BeaconConfig().SecondsPerSlot
	if attTime-uint64(maximumGossipClockDisparity.Milliseconds()) > uint64(currentTime) ||
		uint64(currentTime-maximumGossipClockDisparity.Milliseconds()) > attTimeRange {
		return fmt.Errorf("attestation slot out of range %d <= %d <= %d", attSlot, currentSlot, attSlot+params.BeaconConfig().AttestationPropagationSlotRange)
	}
	return nil
}

// This validates selection proof by validating it's from the correct validator index of the slot and selection
// proof is a valid signature.
func validateSelection(ctx context.Context, s *stateTrie.BeaconState, data *ethpb.AttestationData, validatorIndex uint64, proof []byte) error {
	_, span := trace.StartSpan(ctx, "sync.validateSelection")
	defer span.End()

	committee, err := helpers.BeaconCommitteeFromState(s, data.Slot, data.CommitteeIndex)
	if err != nil {
		return err
	}
	aggregator, err := helpers.IsAggregator(uint64(len(committee)), proof)
	if err != nil {
		return err
	}
	if !aggregator {
		return fmt.Errorf("validator is not an aggregator for slot %d", data.Slot)
	}

	domain, err := helpers.Domain(s.Fork(), helpers.SlotToEpoch(data.Slot), params.BeaconConfig().DomainBeaconAttester, s.GenesisValidatorRoot())
	if err != nil {
		return err
	}
	slotMsg, err := helpers.ComputeSigningRoot(data.Slot, domain)
	if err != nil {
		return err
	}
	pubkeyState := s.PubkeyAtIndex(validatorIndex)
	pubKey, err := bls.PublicKeyFromBytes(pubkeyState[:])
	if err != nil {
		return err
	}
	slotSig, err := bls.SignatureFromBytes(proof)
	if err != nil {
		return err
	}
	if !slotSig.Verify(slotMsg[:], pubKey) {
		return errors.New("could not validate slot signature")
	}

	return nil
}

// This verifies aggregator signature over the signed aggregate and proof object.
func validateAggregatorSignature(s *stateTrie.BeaconState, a *ethpb.SignedAggregateAttestationAndProof) error {
	aggregator, err := s.ValidatorAtIndex(a.Message.AggregatorIndex)
	if err != nil {
		return err
	}

	currentEpoch := helpers.SlotToEpoch(a.Message.Aggregate.Data.Slot)
	domain, err := helpers.Domain(s.Fork(), currentEpoch, params.BeaconConfig().DomainAggregateAndProof, s.GenesisValidatorRoot())
	if err != nil {
		return err
	}

	return helpers.VerifySigningRoot(a.Message, aggregator.PublicKey, a.Signature, domain)

}
