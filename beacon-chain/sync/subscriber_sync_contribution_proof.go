package sync

import (
	"context"
	"errors"
	"fmt"

	ethpb "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	"google.golang.org/protobuf/proto"
)

// syncContributionAndProofSubscriber forwards the incoming validated sync contributions and proof to the
// contribution pool for processing.
func (s *Service) syncContributionAndProofSubscriber(_ context.Context, msg proto.Message) error {
	a, ok := msg.(*ethpb.SignedContributionAndProof)
	if !ok {
		return fmt.Errorf("message was not type *eth.SignedAggregateAttestationAndProof, type=%T", msg)
	}

	if a.Message == nil || a.Message.Contribution == nil {
		return errors.New("nil contribution")
	}

	return s.cfg.SyncCommsPool.SaveSyncCommitteeContribution(a.Message.Contribution)
}
