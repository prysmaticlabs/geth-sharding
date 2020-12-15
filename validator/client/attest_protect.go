package client

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	ethpb "github.com/prysmaticlabs/ethereumapis/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/shared/featureconfig"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/validator/db/kv"
	attestinghistory "github.com/prysmaticlabs/prysm/validator/slashing-protection/local/attesting-history"
	"go.opencensus.io/trace"
)

var failedAttLocalProtectionErr = "attempted to make slashable attestation, rejected by local slashing protection"
var failedPreAttSignExternalErr = "attempted to make slashable attestation, rejected by external slasher service"
var failedPostAttSignExternalErr = "external slasher service detected a submitted slashable attestation"

// Checks if an attestation is slashable by comparing it with the attesting
// history for the given public key in our DB. If it is not, we then update the history
// with new values and save it to the database.
func (v *validator) slashableAttestationCheck(
	ctx context.Context,
	indexedAtt *ethpb.IndexedAttestation,
	pubKey [48]byte,
	signingRoot [32]byte,
) error {
	ctx, span := trace.StartSpan(ctx, "validator.postAttSignUpdate")
	defer span.End()

	fmtKey := fmt.Sprintf("%#x", pubKey[:])
	attesterHistory, err := v.db.AttestationHistoryForPubKeyV2(ctx, pubKey)
	if err != nil {
		return errors.Wrap(err, "could not get attester history")
	}
	if attesterHistory == nil {
		return nil
	}
	historicalAttestation, err := attesterHistory.GetTargetData(ctx, indexedAtt.Data.Target.Epoch)
	if err != nil {
		return err
	}
	var prevSigningRoot [32]byte
	if !historicalAttestation.IsEmpty() {
		copy(prevSigningRoot[:], historicalAttestation.SigningRoot)
	}
	signingRootIsDifferent := prevSigningRoot == params.BeaconConfig().ZeroHash || prevSigningRoot != signingRoot

	// Based on EIP3076, validator should refuse to sign any attestation with source epoch less
	// than the minimum source epoch present in that signer’s attestations.
	lowestSourceEpoch, exists, err := v.db.LowestSignedSourceEpoch(ctx, pubKey)
	if err != nil {
		return err
	}
	if exists && lowestSourceEpoch > indexedAtt.Data.Source.Epoch {
		return fmt.Errorf("could not sign attestation lower than lowest source epoch in db, %d > %d", lowestSourceEpoch, indexedAtt.Data.Source.Epoch)
	}
	// Based on EIP3076, validator should refuse to sign any attestation with target epoch less
	// than or equal to the minimum target epoch present in that signer’s attestations.
	lowestTargetEpoch, exists, err := v.db.LowestSignedTargetEpoch(ctx, pubKey)
	if err != nil {
		return err
	}
	if exists && signingRootIsDifferent && lowestTargetEpoch >= indexedAtt.Data.Target.Epoch {
		return fmt.Errorf(
			"could not sign attestation lower than or equal to lowest target epoch in db, %d >= %d",
			lowestTargetEpoch,
			indexedAtt.Data.Target.Epoch,
		)
	}

	slashable, err := attestinghistory.IsNewAttSlashable(
		ctx,
		attesterHistory,
		indexedAtt.Data.Source.Epoch,
		indexedAtt.Data.Target.Epoch,
		signingRoot,
	)
	if err != nil {
		return errors.Wrap(err, "could not check if attestation is slashable")
	}
	if slashable {
		if v.emitAccountMetrics {
			ValidatorAttestFailVec.WithLabelValues(fmtKey).Inc()
		}
		return errors.New(failedAttLocalProtectionErr)
	}
	newHistory, err := attesterHistory.SetTargetData(
		ctx,
		indexedAtt.Data.Target.Epoch,
		&kv.HistoryData{
			Source:      indexedAtt.Data.Source.Epoch,
			SigningRoot: signingRoot[:],
		},
	)
	if err != nil {
		return errors.Wrapf(err, "could not mark epoch %d as attested", indexedAtt.Data.Target.Epoch)
	}
	newHistory, err = newHistory.SetLatestEpochWritten(ctx, indexedAtt.Data.Target.Epoch)
	if err != nil {
		return err
	}
	if err := v.db.SaveAttestationHistoryForPubKeyV2(ctx, pubKey, newHistory); err != nil {
		return errors.Wrapf(err, "could not save attestation history for public key: %#x", pubKey)
	}
	// Save source and target epochs to satisfy EIP3076 requirements.
	// The DB methods below will replace the lowest epoch in DB if necessary.
	if err := v.db.SaveLowestSignedSourceEpoch(ctx, pubKey, indexedAtt.Data.Source.Epoch); err != nil {
		return err
	}
	if err := v.db.SaveLowestSignedTargetEpoch(ctx, pubKey, indexedAtt.Data.Target.Epoch); err != nil {
		return err
	}
	if featureconfig.Get().SlasherProtection && v.protector != nil {
		if !v.protector.CheckAttestationSafety(ctx, indexedAtt) {
			if v.emitAccountMetrics {
				ValidatorAttestFailVecSlasher.WithLabelValues(fmtKey).Inc()
			}
			return errors.New(failedPreAttSignExternalErr)
		}
		if !v.protector.CommitAttestation(ctx, indexedAtt) {
			if v.emitAccountMetrics {
				ValidatorAttestFailVecSlasher.WithLabelValues(fmtKey).Inc()
			}
			return errors.New(failedPostAttSignExternalErr)
		}
	}
	return nil
}
