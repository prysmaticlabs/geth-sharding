package db

import (
	"context"

	"github.com/gogo/protobuf/proto"
	"github.com/pkg/errors"
	slashpb "github.com/prysmaticlabs/prysm/proto/slashing"
	"github.com/prysmaticlabs/prysm/shared/params"
	bolt "go.etcd.io/bbolt"
	"go.opencensus.io/trace"
)

func unmarshalAttestationHistory(enc []byte) (*slashpb.AttestationHistory, error) {
	history := &slashpb.AttestationHistory{}
	err := proto.Unmarshal(enc, history)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal encoding")
	}
	return history, nil
}

// AttestationHistoryForPubKeys accepts an array of validator public keys and returns a mapping of corresponding attestation history.
func (db *Store) AttestationHistoryForPubKeys(ctx context.Context, publicKeys [][48]byte) (map[[48]byte]*slashpb.AttestationHistory, error) {
	ctx, span := trace.StartSpan(ctx, "Validator.AttestationHistory")
	defer span.End()

	if len(publicKeys) == 0 {
		return make(map[[48]byte]*slashpb.AttestationHistory), nil
	}

	var err error
	attestationHistoryForVals := make(map[[48]byte]*slashpb.AttestationHistory)
	err = db.view(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(historicAttestationsBucket)
		for _, key := range publicKeys {
			enc := bucket.Get(key[:])
			var attestationHistory *slashpb.AttestationHistory
			if enc == nil {
				newMap := make(map[uint64]uint64)
				newMap[0] = params.BeaconConfig().FarFutureEpoch
				attestationHistory = &slashpb.AttestationHistory{
					TargetToSource: newMap,
				}
			} else {
				attestationHistory, err = unmarshalAttestationHistory(enc)
				if err != nil {
					return err
				}
			}
			attestationHistoryForVals[key] = attestationHistory
		}
		return nil
	})
	return attestationHistoryForVals, err
}

// SaveAttestationHistoryForPubKeys returns the attestation histories for the requested validator public keys.
func (db *Store) SaveAttestationHistoryForPubKeys(ctx context.Context, historyByPubKeys map[[48]byte]*slashpb.AttestationHistory) error {
	ctx, span := trace.StartSpan(ctx, "Validator.SaveAttestationHistory")
	defer span.End()

	encoded := make(map[[48]byte][]byte)
	for pubKey, history := range historyByPubKeys {
		enc, err := proto.Marshal(history)
		if err != nil {
			return errors.Wrap(err, "failed to encode attestation history")
		}
		encoded[pubKey] = enc
	}

	err := db.update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(historicAttestationsBucket)
		for pubKey, encodedHistory := range encoded {
			if err := bucket.Put(pubKey[:], encodedHistory); err != nil {
				return err
			}
		}
		return nil
	})
	return err
}

// DeleteAttestationHistory deletes the attestation history for the corresponding validator public key.
func (db *Store) DeleteAttestationHistory(ctx context.Context, pubkey []byte) error {
	ctx, span := trace.StartSpan(ctx, "Validator.DeleteAttestationHistory")
	defer span.End()

	return db.update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(historicAttestationsBucket)
		if err := bucket.Delete(pubkey); err != nil {
			return errors.Wrap(err, "failed to delete the attestation history")
		}
		return nil
	})
}
