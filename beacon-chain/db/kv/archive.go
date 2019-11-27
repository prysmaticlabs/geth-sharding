package kv

import (
	"context"

	"github.com/boltdb/bolt"
	ethpb "github.com/prysmaticlabs/ethereumapis/eth/v1alpha1"
	"github.com/prysmaticlabs/go-ssz"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	"go.opencensus.io/trace"
)

// ArchivedActiveValidatorChanges retrieval by epoch.
func (k *Store) ArchivedActiveValidatorChanges(ctx context.Context, epoch uint64) (*pb.ArchivedActiveSetChanges, error) {
	ctx, span := trace.StartSpan(ctx, "BeaconDB.ArchivedActiveValidatorChanges")
	defer span.End()

	buf := uint64ToBytes(epoch)
	var target *pb.ArchivedActiveSetChanges
	err := k.db.View(func(tx *bolt.Tx) error {
		bkt := tx.Bucket(archivedValidatorSetChangesBucket)
		enc := bkt.Get(buf)
		if enc == nil {
			return nil
		}
		target = &pb.ArchivedActiveSetChanges{}
		return decode(enc, target)
	})
	return target, err
}

// SaveArchivedActiveValidatorChanges by epoch.
func (k *Store) SaveArchivedActiveValidatorChanges(ctx context.Context, epoch uint64, changes *pb.ArchivedActiveSetChanges) error {
	ctx, span := trace.StartSpan(ctx, "BeaconDB.SaveArchivedActiveValidatorChanges")
	defer span.End()
	buf := uint64ToBytes(epoch)
	enc, err := encode(changes)
	if err != nil {
		return err
	}
	return k.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(archivedValidatorSetChangesBucket)
		return bucket.Put(buf, enc)
	})
}

// ArchivedCommitteeInfo retrieval by epoch.
func (k *Store) ArchivedCommitteeInfo(ctx context.Context, epoch uint64) (*pb.ArchivedCommitteeInfo, error) {
	ctx, span := trace.StartSpan(ctx, "BeaconDB.ArchivedCommitteeInfo")
	defer span.End()

	buf := uint64ToBytes(epoch)
	var target *pb.ArchivedCommitteeInfo
	err := k.db.View(func(tx *bolt.Tx) error {
		bkt := tx.Bucket(archivedCommitteeInfoBucket)
		enc := bkt.Get(buf)
		if enc == nil {
			return nil
		}
		target = &pb.ArchivedCommitteeInfo{}
		return decode(enc, target)
	})
	return target, err
}

// SaveArchivedCommitteeInfo by epoch.
func (k *Store) SaveArchivedCommitteeInfo(ctx context.Context, epoch uint64, info *pb.ArchivedCommitteeInfo) error {
	ctx, span := trace.StartSpan(ctx, "BeaconDB.SaveArchivedCommitteeInfo")
	defer span.End()
	buf := uint64ToBytes(epoch)
	enc, err := encode(info)
	if err != nil {
		return err
	}
	return k.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(archivedCommitteeInfoBucket)
		return bucket.Put(buf, enc)
	})
}

// ArchivedBalances retrieval by epoch.
func (k *Store) ArchivedBalances(ctx context.Context, epoch uint64) ([]uint64, error) {
	ctx, span := trace.StartSpan(ctx, "BeaconDB.ArchivedBalances")
	defer span.End()

	buf := uint64ToBytes(epoch)
	var target []uint64
	err := k.db.View(func(tx *bolt.Tx) error {
		bkt := tx.Bucket(archivedBalancesBucket)
		enc := bkt.Get(buf)
		if enc == nil {
			return nil
		}
		target = make([]uint64, 0)
		return ssz.Unmarshal(enc, &target)
	})
	return target, err
}

// SaveArchivedBalances by epoch.
func (k *Store) SaveArchivedBalances(ctx context.Context, epoch uint64, balances []uint64) error {
	ctx, span := trace.StartSpan(ctx, "BeaconDB.SaveArchivedBalances")
	defer span.End()
	buf := uint64ToBytes(epoch)
	enc, err := ssz.Marshal(balances)
	if err != nil {
		return err
	}
	return k.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(archivedBalancesBucket)
		return bucket.Put(buf, enc)
	})
}

// ArchivedValidatorParticipation retrieval by epoch.
func (k *Store) ArchivedValidatorParticipation(ctx context.Context, epoch uint64) (*ethpb.ValidatorParticipation, error) {
	ctx, span := trace.StartSpan(ctx, "BeaconDB.ArchivedValidatorParticipation")
	defer span.End()

	buf := uint64ToBytes(epoch)
	var target *ethpb.ValidatorParticipation
	err := k.db.View(func(tx *bolt.Tx) error {
		bkt := tx.Bucket(archivedValidatorParticipationBucket)
		enc := bkt.Get(buf)
		if enc == nil {
			return nil
		}
		target = &ethpb.ValidatorParticipation{}
		return decode(enc, target)
	})
	return target, err
}

// SaveArchivedValidatorParticipation by epoch.
func (k *Store) SaveArchivedValidatorParticipation(ctx context.Context, epoch uint64, part *ethpb.ValidatorParticipation) error {
	ctx, span := trace.StartSpan(ctx, "BeaconDB.SaveArchivedValidatorParticipation")
	defer span.End()
	buf := uint64ToBytes(epoch)
	enc, err := encode(part)
	if err != nil {
		return err
	}
	return k.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(archivedValidatorParticipationBucket)
		return bucket.Put(buf, enc)
	})
}
