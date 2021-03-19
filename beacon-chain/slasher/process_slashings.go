package slasher

import (
	"context"

	ethpb "github.com/prysmaticlabs/ethereumapis/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/blocks"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
)

// Verifies attester slashings, logs them, and submits them to the slashing operations pool
// in the beacon node if they pass validation.
func (s *Service) processAttesterSlashings(ctx context.Context, slashings []*ethpb.AttesterSlashing) {
	for _, sl := range slashings {
		if err := s.verifyAttSignature(ctx, sl.Attestation_1); err != nil {
			log.WithField("a", sl.Attestation_1).Debug(
				"Invalid signature for attestation in detected slashing offense",
			)
			continue
		}
		if err := s.verifyAttSignature(ctx, sl.Attestation_2); err != nil {
			log.WithField("a", sl.Attestation_2).Debug(
				"Invalid signature for attestation in detected slashing offense",
			)
			continue
		}
		// TODO(#8331): Log the slashing event.

		if err := s.serviceCfg.SlashingPoolInserter.InsertAttesterSlashing(ctx, nil, sl); err != nil {
			log.WithError(err).Error("Could not insert attester slashing into operations pool")
		}
	}
}

// Verifies proposer slashings, logs them, and submits them to the slashing operations pool
// in the beacon node if they pass validation.
func (s *Service) processProposerSlashings(ctx context.Context, slashings []*ethpb.ProposerSlashing) {
	for _, sl := range slashings {
		if err := s.verifyBlockSignature(ctx, sl.Header_1); err != nil {
			log.WithField("a", sl.Header_1).Debug(
				"Invalid signature for block header in detected slashing offense",
			)
			continue
		}
		if err := s.verifyBlockSignature(ctx, sl.Header_2); err != nil {
			log.WithField("a", sl.Header_2).Debug(
				"Invalid signature for block header in detected slashing offense",
			)
			continue
		}
		// TODO(#8331): Log the slashing event.

		if err := s.serviceCfg.SlashingPoolInserter.InsertProposerSlashing(ctx, nil, sl); err != nil {
			log.WithError(err).Error("Could not insert attester slashing into operations pool")
		}
	}
}

func (s *Service) verifyBlockSignature(ctx context.Context, header *ethpb.SignedBeaconBlockHeader) error {
	parentState, err := s.serviceCfg.StateGen.StateByRoot(ctx, bytesutil.ToBytes32(header.Header.ParentRoot))
	if err != nil {
		return err
	}
	return blocks.VerifyBlockHeaderSignature(parentState, header)
}

func (s *Service) verifyAttSignature(ctx context.Context, att *ethpb.IndexedAttestation) error {
	preState, err := s.serviceCfg.StateFetcher.AttestationTargetState(ctx, att.Data.Target)
	if err != nil {
		return err
	}
	return blocks.VerifyIndexedAttestation(ctx, preState, att)
}
