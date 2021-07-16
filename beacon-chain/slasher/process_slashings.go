package slasher

import (
	"context"
	"errors"

	"github.com/prysmaticlabs/prysm/beacon-chain/core/blocks"
	iface "github.com/prysmaticlabs/prysm/beacon-chain/state/interface"
	ethpb "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
)

// Verifies attester slashings, logs them, and submits them to the slashing operations pool
// in the beacon node if they pass validation.
func (s *Service) processAttesterSlashings(ctx context.Context, slashings []*ethpb.AttesterSlashing) error {
	var beaconState iface.BeaconState
	var err error
	if len(slashings) > 0 {
		beaconState, err = s.serviceCfg.HeadStateFetcher.HeadState(ctx)
		if err != nil {
			return err
		}
	}
	for _, sl := range slashings {
		if err := s.verifyAttSignature(ctx, sl.Attestation_1); err != nil {
			log.WithError(err).WithField("a", sl.Attestation_1).Warn(
				"Invalid signature for attestation in detected slashing offense",
			)
			continue
		}
		if err := s.verifyAttSignature(ctx, sl.Attestation_2); err != nil {
			log.WithError(err).WithField("b", sl.Attestation_2).Warn(
				"Invalid signature for attestation in detected slashing offense",
			)
			continue
		}

		// Log the slashing event and insert into the beacon node's operations pool.
		logAttesterSlashing(sl)
		if err := s.serviceCfg.SlashingPoolInserter.InsertAttesterSlashing(
			ctx, beaconState, sl,
		); err != nil {
			log.WithError(err).Error("Could not insert attester slashing into operations pool")
		}
	}
	return nil
}

// Verifies proposer slashings, logs them, and submits them to the slashing operations pool
// in the beacon node if they pass validation.
func (s *Service) processProposerSlashings(ctx context.Context, slashings []*ethpb.ProposerSlashing) error {
	var beaconState iface.BeaconState
	var err error
	if len(slashings) > 0 {
		beaconState, err = s.serviceCfg.HeadStateFetcher.HeadState(ctx)
		if err != nil {
			return err
		}
	}
	for _, sl := range slashings {
		if err := s.verifyBlockSignature(ctx, sl.Header_1); err != nil {
			log.WithError(err).WithField("a", sl.Header_1).Warn(
				"Invalid signature for block header in detected slashing offense",
			)
			continue
		}
		if err := s.verifyBlockSignature(ctx, sl.Header_2); err != nil {
			log.WithError(err).WithField("b", sl.Header_2).Warn(
				"Invalid signature for block header in detected slashing offense",
			)
			continue
		}
		// Log the slashing event and insert into the beacon node's operations pool.
		logProposerSlashing(sl)
		if err := s.serviceCfg.SlashingPoolInserter.InsertProposerSlashing(ctx, beaconState, sl); err != nil {
			log.WithError(err).Error("Could not insert attester slashing into operations pool")
		}
	}
	return nil
}

func (s *Service) verifyBlockSignature(ctx context.Context, header *ethpb.SignedBeaconBlockHeader) error {
	parentState, err := s.serviceCfg.StateGen.StateByRoot(ctx, bytesutil.ToBytes32(header.Header.ParentRoot))
	if err != nil {
		return err
	}
	if parentState == nil || parentState.IsNil() {
		return errors.New("could not verify block signature as parent state is nil")
	}
	return blocks.VerifyBlockHeaderSignature(parentState, header)
}

func (s *Service) verifyAttSignature(ctx context.Context, att *ethpb.IndexedAttestation) error {
	preState, err := s.serviceCfg.AttestationStateFetcher.AttestationTargetState(ctx, att.Data.Target)
	if err != nil {
		return err
	}
	if preState == nil || preState.IsNil() {
		return errors.New("could not verify attestation signature as pre-state is nil")
	}
	return blocks.VerifyIndexedAttestation(ctx, preState, att)
}
