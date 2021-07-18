package v2

import (
	pbp2p "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
)

// CopySyncCommittee copies the provided sync committee object.
func CopySyncCommittee(data *pbp2p.SyncCommittee) *pbp2p.SyncCommittee {
	if data == nil {
		return nil
	}
	return &pbp2p.SyncCommittee{
		Pubkeys:         bytesutil.Copy2dBytes(data.Pubkeys),
		AggregatePubkey: bytesutil.SafeCopyBytes(data.AggregatePubkey),
	}
}
