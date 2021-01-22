package kv

import (
	"github.com/pkg/errors"
	ethpb "github.com/prysmaticlabs/ethereumapis/eth/v1alpha1"
	stateTrie "github.com/prysmaticlabs/prysm/beacon-chain/state"
)

// SaveForkchoiceAttestation saves an forkchoice attestation in cache.
func (c *AttCaches) SaveForkchoiceAttestation(att *ethpb.Attestation) error {
	if att == nil {
		return nil
	}
	r, err := hashFn(att)
	if err != nil {
		return errors.Wrap(err, "could not tree hash attestation")
	}

	att = stateTrie.CopyAttestation(att)
	c.forkchoiceAttLock.Lock()
	defer c.forkchoiceAttLock.Unlock()
	c.forkchoiceAtt[r] = att

	return nil
}

// SaveForkchoiceAttestations saves a list of forkchoice attestations in cache.
func (c *AttCaches) SaveForkchoiceAttestations(atts []*ethpb.Attestation) error {
	for _, att := range atts {
		if err := c.SaveForkchoiceAttestation(att); err != nil {
			return err
		}
	}

	return nil
}

// ForkchoiceAttestations returns the forkchoice attestations in cache.
func (c *AttCaches) ForkchoiceAttestations() []*ethpb.Attestation {
	c.forkchoiceAttLock.RLock()
	defer c.forkchoiceAttLock.RUnlock()

	atts := make([]*ethpb.Attestation, 0, len(c.forkchoiceAtt))
	for _, att := range c.forkchoiceAtt {
		atts = append(atts, stateTrie.CopyAttestation(att) /* Copied */)
	}

	return atts
}

// DeleteForkchoiceAttestation deletes a forkchoice attestation in cache.
func (c *AttCaches) DeleteForkchoiceAttestation(att *ethpb.Attestation) error {
	if att == nil {
		return nil
	}
	r, err := hashFn(att)
	if err != nil {
		return errors.Wrap(err, "could not tree hash attestation")
	}

	c.forkchoiceAttLock.Lock()
	defer c.forkchoiceAttLock.Unlock()
	delete(c.forkchoiceAtt, r)

	return nil
}
