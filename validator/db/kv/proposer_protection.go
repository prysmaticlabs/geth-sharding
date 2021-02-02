package kv

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/prysmaticlabs/prysm/shared/params"
	bolt "go.etcd.io/bbolt"
	"go.opencensus.io/trace"
)

// ProposalHistoryForPubkey for a validator public key.
type ProposalHistoryForPubkey struct {
	Proposals []Proposal
}

// Proposal representation for a validator public key.
type Proposal struct {
	Slot        uint64 `json:"slot"`
	SigningRoot []byte `json:"signing_root"`
}

// ProposedPublicKeys retrieves all public keys in our proposals history bucket.
func (s *Store) ProposedPublicKeys(ctx context.Context) ([][48]byte, error) {
	ctx, span := trace.StartSpan(ctx, "Validator.ProposedPublicKeys")
	defer span.End()
	var err error
	proposedPublicKeys := make([][48]byte, 0)
	err = s.view(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(historicProposalsBucket)
		return bucket.ForEach(func(key []byte, _ []byte) error {
			pubKeyBytes := [48]byte{}
			copy(pubKeyBytes[:], key)
			proposedPublicKeys = append(proposedPublicKeys, pubKeyBytes)
			return nil
		})
	})
	return proposedPublicKeys, err
}

// ProposalHistoryForPubKey returns the entire proposal history for a given public key.
func (s *Store) ProposalHistoryForPubKey(ctx context.Context, publicKey [48]byte) ([]*Proposal, error) {
	ctx, span := trace.StartSpan(ctx, "Validator.ProposalHistoryForPubKey")
	defer span.End()

	proposals := make([]*Proposal, 0)
	err := s.view(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(historicProposalsBucket)
		valBucket := bucket.Bucket(publicKey[:])
		if valBucket == nil {
			return nil
		}
		return valBucket.ForEach(func(slotKey, signingRootBytes []byte) error {
			slot := bytesutil.BytesToUint64BigEndian(slotKey)
			sr := make([]byte, 32)
			copy(sr, signingRootBytes)
			proposals = append(proposals, &Proposal{
				Slot:        slot,
				SigningRoot: sr,
			})
			return nil
		})
	})
	return proposals, err
}

// SaveProposalHistoryForSlot saves the proposal history for the requested validator public key.
// We also check if the incoming proposal slot is lower than the lowest signed proposal slot
// for the validator and override its value on disk.
func (s *Store) SaveProposalHistoryForSlot(ctx context.Context, pubKey [48]byte, slot uint64, signingRoot []byte) error {
	ctx, span := trace.StartSpan(ctx, "Validator.SaveProposalHistoryForEpoch")
	defer span.End()

	err := s.update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(historicProposalsBucket)
		valBucket, err := bucket.CreateBucketIfNotExists(pubKey[:])
		if err != nil {
			return fmt.Errorf("could not create bucket for public key %#x", pubKey)
		}

		// If the incoming slot is lower than the lowest signed proposal slot, override.
		lowestSignedBkt := tx.Bucket(lowestSignedProposalsBucket)
		lowestSignedProposalBytes := lowestSignedBkt.Get(pubKey[:])
		var lowestSignedProposalSlot uint64
		if len(lowestSignedProposalBytes) >= 8 {
			lowestSignedProposalSlot = bytesutil.BytesToUint64BigEndian(lowestSignedProposalBytes)
		}
		if len(lowestSignedProposalBytes) == 0 || slot < lowestSignedProposalSlot {
			if err := lowestSignedBkt.Put(pubKey[:], bytesutil.Uint64ToBytesBigEndian(slot)); err != nil {
				return err
			}
		}

		// If the incoming slot is higher than the highest signed proposal slot, override.
		highestSignedBkt := tx.Bucket(highestSignedProposalsBucket)
		highestSignedProposalBytes := highestSignedBkt.Get(pubKey[:])
		var highestSignedProposalSlot uint64
		if len(highestSignedProposalBytes) >= 8 {
			highestSignedProposalSlot = bytesutil.BytesToUint64BigEndian(highestSignedProposalBytes)
		}
		if len(highestSignedProposalBytes) == 0 || slot > highestSignedProposalSlot {
			if err := highestSignedBkt.Put(pubKey[:], bytesutil.Uint64ToBytesBigEndian(slot)); err != nil {
				return err
			}
		}

		if err := valBucket.Put(bytesutil.Uint64ToBytesBigEndian(slot), signingRoot); err != nil {
			return err
		}
		return pruneProposalHistoryBySlot(valBucket, slot)
	})
	return err
}

// LowestSignedProposal returns the lowest signed proposal slot for a validator public key.
// If no data exists, a boolean of value false is returned.
func (s *Store) LowestSignedProposal(ctx context.Context, publicKey [48]byte) (uint64, bool, error) {
	ctx, span := trace.StartSpan(ctx, "Validator.LowestSignedProposal")
	defer span.End()

	var err error
	var lowestSignedProposalSlot uint64
	var exists bool
	err = s.view(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(lowestSignedProposalsBucket)
		lowestSignedProposalBytes := bucket.Get(publicKey[:])
		// 8 because bytesutil.BytesToUint64BigEndian will return 0 if input is less than 8 bytes.
		if len(lowestSignedProposalBytes) < 8 {
			return nil
		}
		exists = true
		lowestSignedProposalSlot = bytesutil.BytesToUint64BigEndian(lowestSignedProposalBytes)
		return nil
	})
	return lowestSignedProposalSlot, exists, err
}

// HighestSignedProposal returns the highest signed proposal slot for a validator public key.
// If no data exists, a boolean of value false is returned.
func (s *Store) HighestSignedProposal(ctx context.Context, publicKey [48]byte) (uint64, bool, error) {
	ctx, span := trace.StartSpan(ctx, "Validator.HighestSignedProposal")
	defer span.End()

	var err error
	var highestSignedProposalSlot uint64
	var exists bool
	err = s.view(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(highestSignedProposalsBucket)
		highestSignedProposalBytes := bucket.Get(publicKey[:])
		// 8 because bytesutil.BytesToUint64BigEndian will return 0 if input is less than 8 bytes.
		if len(highestSignedProposalBytes) < 8 {
			return nil
		}
		exists = true
		highestSignedProposalSlot = bytesutil.BytesToUint64BigEndian(highestSignedProposalBytes)
		return nil
	})
	return highestSignedProposalSlot, exists, err
}

func pruneProposalHistoryBySlot(valBucket *bolt.Bucket, newestSlot uint64) error {
	c := valBucket.Cursor()
	for k, _ := c.First(); k != nil; k, _ = c.First() {
		slot := bytesutil.BytesToUint64BigEndian(k)
		epoch := helpers.SlotToEpoch(slot)
		newestEpoch := helpers.SlotToEpoch(newestSlot)
		// Only delete epochs that are older than the weak subjectivity period.
		if epoch+params.BeaconConfig().WeakSubjectivityPeriod <= newestEpoch {
			if err := c.Delete(); err != nil {
				return errors.Wrapf(err, "could not prune epoch %d in proposal history", epoch)
			}
		} else {
			// If starting from the oldest, we dont find anything prunable, stop pruning.
			break
		}
	}
	return nil
}
