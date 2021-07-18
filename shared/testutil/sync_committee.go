package testutil

import (
	prysmv2 "github.com/prysmaticlabs/prysm/proto/prysm/v2"
)

// HydrateSyncCommittee hydrates the provided sync committee message.
func HydrateSyncCommittee(s *prysmv2.SyncCommitteeMessage) *prysmv2.SyncCommitteeMessage {
	if s.Signature == nil {
		s.Signature = make([]byte, 96)
	}
	if s.BlockRoot == nil {
		s.BlockRoot = make([]byte, 32)
	}
	return s
}
