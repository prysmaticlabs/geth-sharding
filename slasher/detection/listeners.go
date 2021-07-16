/*
Package detection defines a service that reacts to incoming blocks/attestations
by running slashing detection for double proposals, double votes, and surround votes
according to the Ethereum Beacon Chain specification. As soon as slashing objects are found, they are
sent over a feed for the beaconclient service to submit to a beacon node via gRPC.
*/
package detection

import (
	"context"

	ethpb "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/proto/eth/v1alpha1/wrapper"
	"github.com/prysmaticlabs/prysm/shared/blockutil"
	"go.opencensus.io/trace"
)

// detectIncomingBlocks subscribes to an event feed for
// block objects from a notifier interface. Upon receiving
// a signed beacon block from the feed, we run proposer slashing
// detection on the block.
func (s *Service) detectIncomingBlocks(ctx context.Context, ch chan *ethpb.SignedBeaconBlock) {
	ctx, span := trace.StartSpan(ctx, "detection.detectIncomingBlocks")
	defer span.End()
	sub := s.cfg.Notifier.BlockFeed().Subscribe(ch)
	defer sub.Unsubscribe()
	for {
		select {
		case signedBlock := <-ch:
			wrap := wrapper.WrappedPhase0SignedBeaconBlock(signedBlock)
			signedBlkHdr, err := blockutil.SignedBeaconBlockHeaderFromBlock(wrap)
			if err != nil {
				log.WithError(err).Error("Could not get block header from block")
				continue
			}
			slashing, err := s.proposalsDetector.DetectDoublePropose(ctx, signedBlkHdr)
			if err != nil {
				log.WithError(err).Error("Could not perform detection on block header")
				continue
			}
			s.submitProposerSlashing(ctx, slashing)
		case <-sub.Err():
			log.Error("Subscriber closed, exiting goroutine")
			return
		case <-ctx.Done():
			log.Error("Context canceled")
			return
		}
	}
}

// detectIncomingAttestations subscribes to an event feed for
// attestation objects from a notifier interface. Upon receiving
// an attestation from the feed, we run surround vote and double vote
// detection on the attestation.
func (s *Service) detectIncomingAttestations(ctx context.Context, ch chan *ethpb.IndexedAttestation) {
	ctx, span := trace.StartSpan(ctx, "detection.detectIncomingAttestations")
	defer span.End()
	sub := s.cfg.Notifier.AttestationFeed().Subscribe(ch)
	defer sub.Unsubscribe()
	for {
		select {
		case indexedAtt := <-ch:
			slashings, err := s.DetectAttesterSlashings(ctx, indexedAtt)
			if err != nil {
				log.WithError(err).Error("Could not detect attester slashings")
				continue
			}
			if len(slashings) < 1 {
				if err := s.minMaxSpanDetector.UpdateSpans(ctx, indexedAtt); err != nil {
					log.WithError(err).Error("Could not update spans")
				}
			}
			s.submitAttesterSlashings(ctx, slashings)

			if err := s.UpdateHighestAttestation(ctx, indexedAtt); err != nil {
				log.WithError(err).Error("Could not update highest attestation")
			}
		case <-sub.Err():
			log.Error("Subscriber closed, exiting goroutine")
			return
		case <-ctx.Done():
			log.Error("Context canceled")
			return
		}
	}
}
