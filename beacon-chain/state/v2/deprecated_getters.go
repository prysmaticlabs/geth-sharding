package v2

import (
	"github.com/pkg/errors"
	pbp2p "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
)

// PreviousEpochAttestations is not supported for HF1 beacon state.
func (b *BeaconState) PreviousEpochAttestations() ([]*pbp2p.PendingAttestation, error) {
	return nil, errors.New("PreviousEpochAttestations is not supported for hard fork 1 beacon state")
}

// CurrentEpochAttestations is not supported for HF1 beacon state.
func (b *BeaconState) CurrentEpochAttestations() ([]*pbp2p.PendingAttestation, error) {
	return nil, errors.New("CurrentEpochAttestations is not supported for hard fork 1 beacon state")
}
