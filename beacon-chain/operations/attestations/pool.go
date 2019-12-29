package attestations

import (
	ethpb "github.com/prysmaticlabs/ethereumapis/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/beacon-chain/operations/attestations/kv"
)

// Pool defines the necessary methods for Prysm attestations pool to serve
// fork choice and validators. In the current design, aggregated attestations
// are used by proposer actor. Unaggregated attestations are used by
// for aggregator actor.
type Pool interface {
	// For Aggregated attestations
	SaveAggregatedAttestation(att *ethpb.Attestation) error
	SaveAggregatedAttestations(atts []*ethpb.Attestation) error
	AggregatedAttestations() []*ethpb.Attestation
	DeleteAggregatedAttestation(att *ethpb.Attestation) error
	// For unaggregated attestations
	SaveUnaggregatedAttestation(att *ethpb.Attestation) error
	SaveUnaggregatedAttestations(atts []*ethpb.Attestation) error
	UnaggregatedAttestationsBySlotIndex(slot uint64, committeeIndex uint64) []*ethpb.Attestation
	UnaggregatedAttestations() []*ethpb.Attestation
	DeleteUnaggregatedAttestation(att *ethpb.Attestation) error
	// For attestations that were included in the block
	SaveBlockAttestation(att *ethpb.Attestation) error
	SaveBlockAttestations(atts []*ethpb.Attestation) error
	BlockAttestations() []*ethpb.Attestation
	DeleteBlockAttestation(att *ethpb.Attestation) error
	// For attestations to be passed to fork choice
	SaveForkchoiceAttestation(att *ethpb.Attestation) error
	SaveForkchoiceAttestations(atts []*ethpb.Attestation) error
	ForkchoiceAttestations() []*ethpb.Attestation
	DeleteForkchoiceAttestation(att *ethpb.Attestation) error
}

// NewPool initializes a new attestation pool.
func NewPool() *kv.AttCaches {
	return kv.NewAttCaches()
}
