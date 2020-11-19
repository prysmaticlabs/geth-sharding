package kv

import (
	"context"
	"fmt"

	bolt "go.etcd.io/bbolt"
)

// SaveGenesisValidatorsRoot saves the genesis validator root to db.
func (store *Store) SaveGenesisValidatorsRoot(ctx context.Context, genValRoot []byte) error {
	err := store.update(func(tx *bolt.Tx) error {
		bkt := tx.Bucket(genesisInfoBucket)
		enc := bkt.Get(genesisValidatorsRootKey)
		if len(enc) != 0 {
			return fmt.Errorf("cannot overwite existing genesis validators root: %#x", enc)
		}
		return bkt.Put(genesisValidatorsRootKey, genValRoot)
	})
	return err
}

// GenesisValidatorsRoot retrieves the genesis validator root from db.
func (store *Store) GenesisValidatorsRoot(ctx context.Context) ([]byte, error) {
	var genValRoot []byte
	err := store.view(func(tx *bolt.Tx) error {
		bkt := tx.Bucket(genesisInfoBucket)
		enc := bkt.Get(genesisValidatorsRootKey)
		if len(enc) == 0 {
			return nil
		}
		genValRoot = enc
		return nil
	})
	return genValRoot, err
}
