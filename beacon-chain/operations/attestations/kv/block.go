package kv

import (
	"github.com/pkg/errors"
	ethpb "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/shared/copyutil"
)

// SaveBlockAttestation saves an block attestation in cache.
func (c *AttCaches) SaveBlockAttestation(att *ethpb.Attestation) error {
	if att == nil {
		return nil
	}
	r, err := hashFn(att.Data)
	if err != nil {
		return errors.Wrap(err, "could not tree hash attestation")
	}

	c.blockAttLock.Lock()
	defer c.blockAttLock.Unlock()
	atts, ok := c.blockAtt[r]
	if !ok {
		atts = make([]*ethpb.Attestation, 0, 1)
	}

	// Ensure that this attestation is not already fully contained in an existing attestation.
	for _, a := range atts {
		if a.AggregationBits.Len() == att.AggregationBits.Len() && a.AggregationBits.Contains(att.AggregationBits) {
			return nil
		}
	}

	c.blockAtt[r] = append(atts, copyutil.CopyAttestation(att))

	return nil
}

// SaveBlockAttestations saves a list of block attestations in cache.
func (c *AttCaches) SaveBlockAttestations(atts []*ethpb.Attestation) error {
	for _, att := range atts {
		if err := c.SaveBlockAttestation(att); err != nil {
			return err
		}
	}

	return nil
}

// BlockAttestations returns the block attestations in cache.
func (c *AttCaches) BlockAttestations() []*ethpb.Attestation {
	atts := make([]*ethpb.Attestation, 0)

	c.blockAttLock.RLock()
	defer c.blockAttLock.RUnlock()
	for _, att := range c.blockAtt {
		atts = append(atts, att...)
	}

	return atts
}

// DeleteBlockAttestation deletes a block attestation in cache.
func (c *AttCaches) DeleteBlockAttestation(att *ethpb.Attestation) error {
	if att == nil {
		return nil
	}
	r, err := hashFn(att.Data)
	if err != nil {
		return errors.Wrap(err, "could not tree hash attestation")
	}

	c.blockAttLock.Lock()
	defer c.blockAttLock.Unlock()
	delete(c.blockAtt, r)

	return nil
}
