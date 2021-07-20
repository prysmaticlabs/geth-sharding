package sync

import (
	"context"

	"github.com/libp2p/go-libp2p-core/peer"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	types "github.com/prysmaticlabs/eth2-types"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/altair"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	p2ptypes "github.com/prysmaticlabs/prysm/beacon-chain/p2p/types"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	prysmv2 "github.com/prysmaticlabs/prysm/proto/prysm/v2"
	"github.com/prysmaticlabs/prysm/shared/bls"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/traceutil"
	"go.opencensus.io/trace"
)

// validateSyncContributionAndProof verifies the aggregated signature and the selection proof is valid before forwarding to the
// network and downstream services.
func (s *Service) validateSyncContributionAndProof(ctx context.Context, pid peer.ID, msg *pubsub.Message) pubsub.ValidationResult {
	ctx, span := trace.StartSpan(ctx, "sync.validateSyncContributionAndProof")
	defer span.End()

	// Accept the sync committee contribution if the contribution came from itself.
	if pid == s.cfg.P2P.PeerID() {
		return pubsub.ValidationAccept
	}

	// Ignore the sync committee contribution if the beacon node is syncing.
	if s.cfg.InitialSync.Syncing() {
		return pubsub.ValidationIgnore
	}

	raw, err := s.decodePubsubMessage(msg)
	if err != nil {
		log.WithError(err).Debug("Could not decode message")
		traceutil.AnnotateError(span, err)
		return pubsub.ValidationReject
	}
	m, ok := raw.(*prysmv2.SignedContributionAndProof)
	if !ok {
		return pubsub.ValidationReject
	}
	if m == nil || m.Message == nil {
		return pubsub.ValidationReject
	}
	if err := altair.ValidateNilSyncContribution(m); err != nil {
		return pubsub.ValidationReject
	}

	// The contribution's `slot` is for the current slot (with a MAXIMUM_GOSSIP_CLOCK_DISPARITY allowance).
	if err := helpers.VerifySlotTime(uint64(s.cfg.Chain.GenesisTime().Unix()), m.Message.Contribution.Slot, params.BeaconNetworkConfig().MaximumGossipClockDisparity); err != nil {
		traceutil.AnnotateError(span, err)
		return pubsub.ValidationIgnore
	}

	// The subcommittee index is in the allowed range, i.e. `contribution.subcommittee_index` < `SYNC_COMMITTEE_SUBNET_COUNT`.
	if m.Message.Contribution.SubcommitteeIndex >= params.BeaconConfig().SyncCommitteeSubnetCount {
		return pubsub.ValidationReject
	}

	// The sync committee contribution is the first valid contribution received for the aggregator with index
	// `contribution_and_proof.aggregator_index` for the slot `contribution.slot` and subcommittee index `contribution.subcommittee_index`.
	if s.hasSeenSyncContributionIndexSlot(m.Message.Contribution.Slot, m.Message.AggregatorIndex, types.CommitteeIndex(m.Message.Contribution.SubcommitteeIndex)) {
		return pubsub.ValidationIgnore
	}

	// The `contribution_and_proof.selection_proof` selects the validator as an aggregator for the slot.
	if !altair.IsSyncCommitteeAggregator(m.Message.SelectionProof) {
		return pubsub.ValidationReject
	}

	// The aggregator's validator index is in the declared subcommittee of the current sync committee.
	committeeIndices, err := s.cfg.Chain.HeadCurrentSyncCommitteeIndices(ctx, m.Message.AggregatorIndex, m.Message.Contribution.Slot)
	if err != nil {
		traceutil.AnnotateError(span, err)
		return pubsub.ValidationIgnore
	}
	if len(committeeIndices) == 0 {
		traceutil.AnnotateError(span, err)
		return pubsub.ValidationIgnore
	}
	isValid := false
	subCommitteeSize := params.BeaconConfig().SyncCommitteeSize / params.BeaconConfig().SyncCommitteeSubnetCount
	for _, i := range committeeIndices {
		if uint64(i)/subCommitteeSize == m.Message.Contribution.SubcommitteeIndex {
			isValid = true
			break
		}
	}
	if !isValid {
		return pubsub.ValidationReject
	}

	// The `contribution_and_proof.selection_proof` is a valid signature of the `SyncAggregatorSelectionData`.
	if err := s.verifySyncSelectionData(ctx, m.Message); err != nil {
		traceutil.AnnotateError(span, err)
		return pubsub.ValidationReject
	}

	// The aggregator signature, `signed_contribution_and_proof.signature`, is valid.
	d, err := s.cfg.Chain.HeadSyncContributionProofDomain(ctx, m.Message.Contribution.Slot)
	if err != nil {
		traceutil.AnnotateError(span, err)
		return pubsub.ValidationIgnore
	}
	pubkey, err := s.cfg.Chain.HeadValidatorIndexToPublicKey(ctx, m.Message.AggregatorIndex)
	if err != nil {
		return pubsub.ValidationIgnore
	}
	if err := helpers.VerifySigningRoot(m.Message, pubkey[:], m.Signature, d); err != nil {
		traceutil.AnnotateError(span, err)
		return pubsub.ValidationReject
	}

	// The aggregate signature is valid for the message `beacon_block_root` and aggregate pubkey
	// derived from the participation info in `aggregation_bits` for the subcommittee specified by the `contribution.subcommittee_index`.
	activePubkeys := []bls.PublicKey{}
	syncPubkeys, err := s.cfg.Chain.HeadSyncCommitteePubKeys(ctx, m.Message.Contribution.Slot, types.CommitteeIndex(m.Message.Contribution.SubcommitteeIndex))
	if err != nil {
		return pubsub.ValidationIgnore
	}
	bVector := m.Message.Contribution.AggregationBits
	for i, pk := range syncPubkeys {
		if bVector.BitAt(uint64(i)) {
			pubK, err := bls.PublicKeyFromBytes(pk)
			if err != nil {
				traceutil.AnnotateError(span, err)
				return pubsub.ValidationIgnore
			}
			activePubkeys = append(activePubkeys, pubK)
		}
	}
	sig, err := bls.SignatureFromBytes(m.Message.Contribution.Signature)
	if err != nil {
		traceutil.AnnotateError(span, err)
		return pubsub.ValidationReject
	}
	d, err = s.cfg.Chain.HeadSyncCommitteeDomain(ctx, m.Message.Contribution.Slot)
	if err != nil {
		traceutil.AnnotateError(span, err)
		return pubsub.ValidationIgnore
	}
	rawBytes := p2ptypes.SSZBytes(m.Message.Contribution.BlockRoot)
	sigRoot, err := helpers.ComputeSigningRoot(&rawBytes, d)
	if err != nil {
		traceutil.AnnotateError(span, err)
		return pubsub.ValidationIgnore
	}
	verified := sig.Eth2FastAggregateVerify(activePubkeys, sigRoot)
	if !verified {
		return pubsub.ValidationReject
	}

	s.setSyncContributionIndexSlotSeen(m.Message.Contribution.Slot, m.Message.AggregatorIndex, types.CommitteeIndex(m.Message.Contribution.SubcommitteeIndex))

	msg.ValidatorData = m

	return pubsub.ValidationAccept
}

// Returns true if the node has received sync contribution for the aggregator with index, slot and subcommittee index.
func (s *Service) hasSeenSyncContributionIndexSlot(slot types.Slot, aggregatorIndex types.ValidatorIndex, subComIdx types.CommitteeIndex) bool {
	s.seenSyncContributionLock.RLock()
	defer s.seenSyncContributionLock.RUnlock()

	b := append(bytesutil.Bytes32(uint64(aggregatorIndex)), bytesutil.Bytes32(uint64(slot))...)
	b = append(b, bytesutil.Bytes32(uint64(subComIdx))...)
	_, seen := s.seenSyncContributionCache.Get(string(b))
	return seen
}

// Set sync contributor's aggregate index, slot and subcommittee index as seen.
func (s *Service) setSyncContributionIndexSlotSeen(slot types.Slot, aggregatorIndex types.ValidatorIndex, subComIdx types.CommitteeIndex) {
	s.seenSyncContributionLock.Lock()
	defer s.seenSyncContributionLock.Unlock()
	b := append(bytesutil.Bytes32(uint64(aggregatorIndex)), bytesutil.Bytes32(uint64(slot))...)
	b = append(b, bytesutil.Bytes32(uint64(subComIdx))...)
	s.seenSyncContributionCache.Add(string(b), true)
}

// verifySyncSelectionData verifies that the provided sync contribution has a valid
// selection proof.
func (s *Service) verifySyncSelectionData(ctx context.Context, m *prysmv2.ContributionAndProof) error {
	selectionData := &pb.SyncAggregatorSelectionData{Slot: m.Contribution.Slot, SubcommitteeIndex: uint64(m.Contribution.SubcommitteeIndex)}
	domain, err := s.cfg.Chain.HeadSyncSelectionProofDomain(ctx, m.Contribution.Slot)
	if err != nil {
		return err
	}
	pubkey, err := s.cfg.Chain.HeadValidatorIndexToPublicKey(ctx, m.AggregatorIndex)
	if err != nil {
		return err
	}
	return helpers.VerifySigningRoot(selectionData, pubkey[:], m.SelectionProof, domain)
}
