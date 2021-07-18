package sync

import (
	"bytes"
	"context"

	"github.com/libp2p/go-libp2p-core/peer"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	types "github.com/prysmaticlabs/eth2-types"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/altair"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	p2ptypes "github.com/prysmaticlabs/prysm/beacon-chain/p2p/types"
	iface "github.com/prysmaticlabs/prysm/beacon-chain/state/interface"
	prysmv2 "github.com/prysmaticlabs/prysm/proto/prysm/v2"
	"github.com/prysmaticlabs/prysm/shared/bls"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/traceutil"
	"github.com/prysmaticlabs/prysm/shared/version"
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
	// This could be better, retrieving the state multiple times with copies can easily lead to higher resource consumption by the node.
	blkState, err := s.cfg.StateGen.StateByRoot(ctx, bytesutil.ToBytes32(m.Message.Contribution.BlockRoot))
	if err != nil {
		traceutil.AnnotateError(span, err)
		return pubsub.ValidationIgnore
	}
	bState, ok := blkState.(iface.BeaconStateAltair)
	if !ok || bState.Version() != version.Altair {
		log.Errorf("Sync contribution referencing non-altair state")
		return pubsub.ValidationReject
	}
	syncPubkeys, err := altair.SyncSubCommitteePubkeys(bState, types.CommitteeIndex(m.Message.Contribution.SubcommitteeIndex))
	if err != nil {
		traceutil.AnnotateError(span, err)
		return pubsub.ValidationIgnore
	}
	aggregator, err := bState.ValidatorAtIndexReadOnly(m.Message.AggregatorIndex)
	if err != nil {
		traceutil.AnnotateError(span, err)
		return pubsub.ValidationIgnore
	}
	aggPubkey := aggregator.PublicKey()
	isValid := false
	for _, pk := range syncPubkeys {
		if bytes.Equal(pk, aggPubkey[:]) {
			isValid = true
			break
		}
	}
	if !isValid {
		return pubsub.ValidationReject
	}

	// The `contribution_and_proof.selection_proof` is a valid signature of the `SyncAggregatorSelectionData`.
	if err := altair.VerifySyncSelectionData(bState, m.Message); err != nil {
		traceutil.AnnotateError(span, err)
		return pubsub.ValidationReject
	}

	// The aggregator signature, `signed_contribution_and_proof.signature`, is valid.
	d, err := helpers.Domain(bState.Fork(), helpers.SlotToEpoch(bState.Slot()), params.BeaconConfig().DomainContributionAndProof, bState.GenesisValidatorRoot())
	if err != nil {
		traceutil.AnnotateError(span, err)
		return pubsub.ValidationIgnore
	}
	if err := helpers.VerifySigningRoot(m.Message, aggPubkey[:], m.Signature, d); err != nil {
		traceutil.AnnotateError(span, err)
		return pubsub.ValidationReject
	}

	// The aggregate signature is valid for the message `beacon_block_root` and aggregate pubkey
	// derived from the participation info in `aggregation_bits` for the subcommittee specified by the `contribution.subcommittee_index`.
	activePubkeys := []bls.PublicKey{}
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
	d, err = helpers.Domain(bState.Fork(), helpers.SlotToEpoch(bState.Slot()), params.BeaconConfig().DomainSyncCommittee, bState.GenesisValidatorRoot())
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
