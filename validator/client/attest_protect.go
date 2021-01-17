package client

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	ethpb "github.com/prysmaticlabs/ethereumapis/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/shared/featureconfig"
	"github.com/prysmaticlabs/prysm/validator/db/kv"
	"go.opencensus.io/trace"
)

var failedAttLocalProtectionErr = "attempted to make slashable attestation, rejected by local slashing protection"
var failedPostAttSignExternalErr = "attempted to make slashable attestation, rejected by external slasher service"

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
	slashingKind, err := v.db.CheckSlashableAttestation(ctx, pubKey, signingRoot, indexedAtt)
	if err != nil {
		if v.emitAccountMetrics {
			ValidatorAttestFailVec.WithLabelValues(fmtKey).Inc()
		}
		switch slashingKind {
		case kv.DoubleVote:
			log.Warn("Attestation is slashable as it is a double vote")
		case kv.SurroundingVote:
			log.Warn("Attestation is slashable as it is surrounding a previous attestation")
		case kv.SurroundedVote:
			log.Warn("Attestation is slashable as it is surrounded by a previous attestation")
		}
		return errors.Wrap(err, failedAttLocalProtectionErr)
	}

	if err := v.db.SaveAttestationForPubKey(ctx, pubKey, signingRoot, indexedAtt); err != nil {
		return errors.Wrap(err, "could not save attestation history for validator public key")
	}

	if featureconfig.Get().SlasherProtection && v.protector != nil {
		if !v.protector.CommitAttestation(ctx, indexedAtt) {
			if v.emitAccountMetrics {
				ValidatorAttestFailVecSlasher.WithLabelValues(fmtKey).Inc()
			}
			return errors.New(failedPostAttSignExternalErr)
		}
	}
	return nil
}
