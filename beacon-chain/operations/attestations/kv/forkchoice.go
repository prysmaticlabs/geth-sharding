package kv

import (
	"github.com/pkg/errors"
	ethpb "github.com/prysmaticlabs/ethereumapis/eth/v1alpha1"
	"github.com/prysmaticlabs/go-ssz"
	stateTrie "github.com/prysmaticlabs/prysm/beacon-chain/state"
)

// SaveForkchoiceAttestation saves an forkchoice attestation in cache.
func (p *AttCaches) SaveForkchoiceAttestation(att *ethpb.Attestation) error {
	r, err := ssz.HashTreeRoot(att)
	if err != nil {
		return errors.Wrap(err, "could not tree hash attestation")
	}

	p.forkchoiceAtt[r] = stateTrie.CopyAttestation(att) // Copied.

	return nil
}

// SaveForkchoiceAttestations saves a list of forkchoice attestations in cache.
func (p *AttCaches) SaveForkchoiceAttestations(atts []*ethpb.Attestation) error {
	for _, att := range atts {
		if err := p.SaveForkchoiceAttestation(att); err != nil {
			return err
		}
	}

	return nil
}

// ForkchoiceAttestations returns the forkchoice attestations in cache.
func (p *AttCaches) ForkchoiceAttestations() []*ethpb.Attestation {
	atts := make([]*ethpb.Attestation, 0)
	for _, att := range p.forkchoiceAtt {
		atts = append(atts, stateTrie.CopyAttestation(att) /* Copied */)
	}

	return atts
}

// DeleteForkchoiceAttestation deletes a forkchoice attestation in cache.
func (p *AttCaches) DeleteForkchoiceAttestation(att *ethpb.Attestation) error {
	r, err := ssz.HashTreeRoot(att)
	if err != nil {
		return errors.Wrap(err, "could not tree hash attestation")
	}

	delete(p.forkchoiceAtt, r)

	return nil
}
