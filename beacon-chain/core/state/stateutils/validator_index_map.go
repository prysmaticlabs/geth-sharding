package stateutils

import (
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
)

// ValidatorIndexMap builds a lookup map for quickly determining the index of
// a validator by their public key.
func ValidatorIndexMap(state *pb.BeaconState) map[[48]byte]int {
	m := make(map[[48]byte]int)
	for idx, record := range state.Validators {
		key := bytesutil.ToBytes48(record.PublicKey)
		m[key] = idx
	}
	return m
}
