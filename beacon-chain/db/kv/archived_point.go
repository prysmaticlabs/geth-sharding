package kv

import (
	"context"

	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	bolt "go.etcd.io/bbolt"
	"go.opencensus.io/trace"
)

// SaveArchivedPointRoot saves an archived point root to the DB. This is used for cold state management.
func (k *Store) SaveArchivedPointRoot(ctx context.Context, blockRoot [32]byte, index uint64) error {
	ctx, span := trace.StartSpan(ctx, "BeaconDB.SaveArchivedPointRoot")
	defer span.End()

	return k.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(archivedIndexRootBucket)
		return bucket.Put(uint64ToBytes(index), blockRoot[:])
	})
}

// SaveLastArchivedIndex to the db.
func (k *Store) SaveLastArchivedIndex(ctx context.Context, index uint64) error {
	ctx, span := trace.StartSpan(ctx, "BeaconDB.SaveHeadBlockRoot")
	defer span.End()
	return k.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(archivedIndexRootBucket)
		return bucket.Put(lastArchivedIndexKey, uint64ToBytes(index))
	})
}

// LastArchivedIndexRoot from the db.
func (k *Store) LastArchivedIndexRoot(ctx context.Context) [32]byte {
	ctx, span := trace.StartSpan(ctx, "BeaconDB.LastArchivedIndexRoot")
	defer span.End()

	var blockRoot []byte
	// #nosec G104. Always returns nil.
	k.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(archivedIndexRootBucket)
		lastArchivedIndex := bucket.Get(lastArchivedIndexKey)
		if lastArchivedIndex == nil {
			return nil
		}
		blockRoot = bucket.Get(lastArchivedIndex)
		return nil
	})

	return bytesutil.ToBytes32(blockRoot)
}

// ArchivedPointRoot returns the block root of an archived point from the DB.
// This is essential for cold state management and to restore a cold state.
func (k *Store) ArchivedPointRoot(ctx context.Context, index uint64) [32]byte {
	ctx, span := trace.StartSpan(ctx, "BeaconDB.ArchivePointRoot")
	defer span.End()

	var blockRoot []byte
	// #nosec G104. Always returns nil.
	k.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(archivedIndexRootBucket)
		blockRoot = bucket.Get(uint64ToBytes(index))
		return nil
	})

	return bytesutil.ToBytes32(blockRoot)
}

// HasArchivedPoint returns true if an archived point exists in DB.
func (k *Store) HasArchivedPoint(ctx context.Context, index uint64) bool {
	ctx, span := trace.StartSpan(ctx, "BeaconDB.HasArchivedPoint")
	defer span.End()
	var exists bool
	// #nosec G104. Always returns nil.
	k.db.View(func(tx *bolt.Tx) error {
		iBucket := tx.Bucket(archivedIndexRootBucket)
		exists = iBucket.Get(uint64ToBytes(index)) != nil
		return nil
	})
	return exists
}
