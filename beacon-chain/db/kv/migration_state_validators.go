package kv

import (
	"bytes"
	"context"

	"github.com/golang/snappy"
	v1 "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	"github.com/prysmaticlabs/prysm/shared/featureconfig"
	"github.com/prysmaticlabs/prysm/shared/hashutil"
	bolt "go.etcd.io/bbolt"
)

var migrationStateValidators0Key = []byte("state_validators_0")

func migrateStateValidators(tx *bolt.Tx) error {
	if !featureconfig.Get().EnableHistoricalSpaceRepresentation {
		return nil
	}
	mb := tx.Bucket(migrationsBucket)
	if b := mb.Get(migrationStateValidators0Key); bytes.Equal(b, migrationCompleted) {
		return nil // Migration already completed.
	}

	// get the source and destination buckets.
	log.Infof("migrating %s. It will take few minutes. Please wait.", stateBucket)
	stateBkt := tx.Bucket(stateBucket)
	if stateBkt == nil {
		return nil
	}
	valBkt := tx.Bucket(stateValidatorsBucket)
	if valBkt == nil {
		return nil
	}
	indexBkt := tx.Bucket(blockRootValidatorHashesBucket)
	if indexBkt == nil {
		return nil
	}

	// for each of the state in the stateBucket, do the migration.
	ctx := context.Background()
	c := stateBkt.Cursor()
	for k, v := c.First(); k != nil; k, v = c.Next() {
		state := &v1.BeaconState{}
		if decodeErr := decode(ctx, v, state); decodeErr != nil {
			return decodeErr
		}

		// move all the validators in this state registry out to a new bucket.
		var validatorKeys []byte
		for _, val := range state.Validators {
			valBytes, encodeErr := encode(ctx, val)
			if encodeErr != nil {
				return encodeErr
			}

			// create the unique hash for that validator entry.
			hash := hashutil.Hash(valBytes)

			// add the validator in the stateValidatorsBucket, if it is not present.
			if valEntry := valBkt.Get(hash[:]); valEntry == nil {
				if putErr := valBkt.Put(hash[:], valBytes); putErr != nil {
					return putErr
				}
			}

			// note down the pointer of the stateValidatorsBucket.
			validatorKeys = append(validatorKeys, hash[:]...)
		}

		// add the validator entry keys for a given block root.
		compValidatorKeys := snappy.Encode(nil, validatorKeys)
		idxErr := indexBkt.Put(k, compValidatorKeys)
		if idxErr != nil {
			return idxErr
		}

		// zero the validator entries in BeaconState object .
		state.Validators = nil
		stateBytes, encodeErr := encode(ctx, state)
		if encodeErr != nil {
			return encodeErr
		}
		if stateErr := stateBkt.Put(k, stateBytes); stateErr != nil {
			return stateErr
		}
	}

	// Mark migration complete.
	return mb.Put(migrationStateValidators0Key, migrationCompleted)
}
