package synccommittee

import (
	types "github.com/prysmaticlabs/eth2-types"
	prysmv2 "github.com/prysmaticlabs/prysm/proto/prysm/v2"
)

var _ = Pool(&Store{})

// Pool defines the necessary methods for Prysm sync pool to serve
// validators. In the current design, aggregated attestations
// are used by proposers and sync committee messages are used by
// sync aggregators.
type Pool interface {
	// Methods for Sync Contributions.
	SaveSyncCommitteeContribution(contr *prysmv2.SyncCommitteeContribution) error
	SyncCommitteeContributions(slot types.Slot) ([]*prysmv2.SyncCommitteeContribution, error)

	// Methods for Sync Committee Messages.
	SaveSyncCommitteeMessage(sig *prysmv2.SyncCommitteeMessage) error
	SyncCommitteeMessages(slot types.Slot) ([]*prysmv2.SyncCommitteeMessage, error)
}

// NewPool returns the sync committee store fulfilling the pool interface.
func NewPool() Pool {
	return NewStore()
}
