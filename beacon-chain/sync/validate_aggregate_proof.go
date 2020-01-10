package sync

import (
	"context"
	"fmt"

	"github.com/libp2p/go-libp2p-core/peer"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/pkg/errors"
	ethpb "github.com/prysmaticlabs/ethereumapis/eth/v1alpha1"
	"github.com/prysmaticlabs/go-ssz"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/blocks"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/state"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	"github.com/prysmaticlabs/prysm/shared/bls"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
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
	m, ok := raw.(*ethpb.AggregateAttestationAndProof)
	if !ok {
		return false
	}

	attSlot := m.Aggregate.Data.Slot

	// Verify aggregate attestation has not already been seen via aggregate gossip, within a block, or through the creation locally.
	seen, err := r.attPool.HasAggregatedAttestation(m.Aggregate)
	if err != nil {
		traceutil.AnnotateError(span, err)
		return false
	}
	if seen {
		return false
	}

	// Verify the block being voted for passes validation. The block should have passed validation if it's in the DB.
	if !r.db.HasBlock(ctx, bytesutil.ToBytes32(m.Aggregate.Data.BeaconBlockRoot)) {
		return false
	}

	// Verify attestation slot is within the last ATTESTATION_PROPAGATION_SLOT_RANGE slots.
	currentSlot := uint64(roughtime.Now().Unix()-r.chain.GenesisTime().Unix()) / params.BeaconConfig().SecondsPerSlot
	if attSlot > currentSlot || currentSlot > attSlot+params.BeaconConfig().AttestationPropagationSlotRange {
		traceutil.AnnotateError(span, fmt.Errorf("attestation slot out of range %d <= %d <= %d", attSlot, currentSlot, attSlot+params.BeaconConfig().AttestationPropagationSlotRange))
		return false

	}

	s, err := r.chain.HeadState(ctx)
	if err != nil {
		traceutil.AnnotateError(span, err)
		return false
	}

	// Only advance state if different epoch as the committee can only change on an epoch transition.
	if helpers.SlotToEpoch(attSlot) > helpers.SlotToEpoch(s.Slot) {
		s, err = state.ProcessSlots(ctx, s, attSlot)
		if err != nil {
			traceutil.AnnotateError(span, err)
			return false
		}
	}

	// Verify validator index is within the aggregate's committee.
	if err := validateIndexInCommittee(ctx, s, m.Aggregate, m.AggregatorIndex); err != nil {
		traceutil.AnnotateError(span, errors.Wrapf(err, "Could not validate index in committee"))
		return false
	}

	// Verify selection proof reflects to the right validator and signature is valid.
	if err := validateSelection(ctx, s, m.Aggregate.Data, m.AggregatorIndex, m.SelectionProof); err != nil {
		traceutil.AnnotateError(span, errors.Wrapf(err, "Could not validate selection for validator %d", m.AggregatorIndex))
		return false
	}

	// Verify aggregated attestation has a valid signature.
	if err := blocks.VerifyAttestation(ctx, s, m.Aggregate); err != nil {
		traceutil.AnnotateError(span, err)
		return false
	}

	msg.ValidatorData = m

	return true
}

// This validates the aggregator's index in state is within the attesting indices of the attestation.
func validateIndexInCommittee(ctx context.Context, s *pb.BeaconState, a *ethpb.Attestation, validatorIndex uint64) error {
	ctx, span := trace.StartSpan(ctx, "sync..validateIndexInCommittee")
	defer span.End()

	committee, err := helpers.BeaconCommitteeFromState(s, a.Data.Slot, a.Data.CommitteeIndex)
	if err != nil {
		return err
	}
	attestingIndices, err := helpers.AttestingIndices(a.AggregationBits, committee)
	if err != nil {
		return err
	}
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

// This validates selection proof by validating it's from the correct validator index of the slot and selection
// proof is a valid signature.
func validateSelection(ctx context.Context, s *pb.BeaconState, data *ethpb.AttestationData, validatorIndex uint64, proof []byte) error {
	_, span := trace.StartSpan(ctx, "sync.validateSelection")
	defer span.End()

	committee, err := helpers.BeaconCommitteeFromState(s, data.Slot, data.CommitteeIndex)
	if err != nil {
		return err
	}
	aggregator, err := helpers.IsAggregator(uint64(len(committee)), data.Slot, data.CommitteeIndex, proof)
	if err != nil {
		return err
	}
	if !aggregator {
		return fmt.Errorf("validator is not an aggregator for slot %d", data.Slot)
	}

	domain := helpers.Domain(s.Fork, helpers.SlotToEpoch(data.Slot), params.BeaconConfig().DomainBeaconAttester)
	slotMsg, err := ssz.HashTreeRoot(data.Slot)
	if err != nil {
		return err
	}
	pubKey, err := bls.PublicKeyFromBytes(s.Validators[validatorIndex].PublicKey)
	if err != nil {
		return err
	}
	slotSig, err := bls.SignatureFromBytes(proof)
	if err != nil {
		return err
	}
	if !slotSig.Verify(slotMsg[:], pubKey, domain) {
		return errors.New("could not validate slot signature")
	}

	return nil
}
