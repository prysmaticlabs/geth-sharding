package db

import (
	"bytes"

	"github.com/boltdb/bolt"
	"github.com/gogo/protobuf/proto"
	"github.com/pkg/errors"
	ethpb "github.com/prysmaticlabs/ethereumapis/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/shared/hashutil"
	"github.com/prysmaticlabs/prysm/slasher/db/types"
)

func unmarshalProposerSlashing(enc []byte) (*ethpb.ProposerSlashing, error) {
	protoSlashing := &ethpb.ProposerSlashing{}
	if err := proto.Unmarshal(enc, protoSlashing); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal encoded proposer slashing")
	}
	return protoSlashing, nil
}

func unmarshalProposerSlashingArray(encoded [][]byte) ([]*ethpb.ProposerSlashing, error) {
	proposerSlashings := make([]*ethpb.ProposerSlashing, len(encoded))
	for i, enc := range encoded {
		ps, err := unmarshalProposerSlashing(enc)
		if err != nil {
			return nil, err
		}
		proposerSlashings[i] = ps
	}
	return proposerSlashings, nil
}

// ProposalSlashingsByStatus returns all the proposal slashing proofs with a certain status.
func (db *Store) ProposalSlashingsByStatus(status types.SlashingStatus) ([]*ethpb.ProposerSlashing, error) {
	encoded := make([][]byte, 0)
	err := db.view(func(tx *bolt.Tx) error {
		c := tx.Bucket(slashingBucket).Cursor()
		prefix := encodeType(types.SlashingType(types.Proposal))
		for k, v := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, v = c.Next() {
			if v[0] == byte(status) {
				encoded = append(encoded, v[1:])
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return unmarshalProposerSlashingArray(encoded)
}

// DeleteProposerSlashing deletes a proposer slashing proof.
func (db *Store) DeleteProposerSlashing(slashing *ethpb.ProposerSlashing) error {
	root, err := hashutil.HashProto(slashing)
	if err != nil {
		return errors.Wrap(err, "failed to get hash root of proposerSlashing")
	}
	err = db.update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(slashingBucket)
		k := encodeTypeRoot(types.SlashingType(types.Proposal), root)
		if err := bucket.Delete(k); err != nil {
			return errors.Wrap(err, "failed to delete the slashing proof from slashing bucket")
		}
		return nil
	})
	return err
}

// HasProposerSlashing returns the slashing key if it is found in db.
func (db *Store) HasProposerSlashing(slashing *ethpb.ProposerSlashing) (bool, types.SlashingStatus, error) {
	var status types.SlashingStatus
	var found bool
	root, err := hashutil.HashProto(slashing)
	if err != nil {
		return found, status, errors.Wrap(err, "failed to get hash root of proposerSlashing")
	}
	key := encodeTypeRoot(types.SlashingType(types.Proposal), root)

	err = db.view(func(tx *bolt.Tx) error {
		b := tx.Bucket(slashingBucket)
		enc := b.Get(key)
		if enc != nil {
			found = true
			status = types.SlashingStatus(enc[0])
		}
		return nil
	})
	return found, status, err
}

// SaveProposerSlashing accepts a proposer slashing and its status header and writes it to disk.
func (db *Store) SaveProposerSlashing(status types.SlashingStatus, slashing *ethpb.ProposerSlashing) error {
	enc, err := proto.Marshal(slashing)
	if err != nil {
		return errors.Wrap(err, "failed to marshal")
	}
	root := hashutil.Hash(enc)
	key := encodeTypeRoot(types.SlashingType(types.Proposal), root)
	return db.update(func(tx *bolt.Tx) error {
		b := tx.Bucket(slashingBucket)
		e := b.Put(key, append([]byte{byte(status)}, enc...))
		return e
	})
}

// SaveProposerSlashings accepts a slice of slashing proof and its status and writes it to disk.
func (db *Store) SaveProposerSlashings(status types.SlashingStatus, slashings []*ethpb.ProposerSlashing) error {
	encSlashings := make([][]byte, len(slashings))
	keys := make([][]byte, len(slashings))
	var err error
	for i, slashing := range slashings {
		encSlashings[i], err = proto.Marshal(slashing)
		if err != nil {
			return errors.Wrap(err, "failed to marshal")
		}
		root := hashutil.Hash(encSlashings[i])
		keys[i] = encodeTypeRoot(types.SlashingType(types.Proposal), root)
	}

	return db.update(func(tx *bolt.Tx) error {
		b := tx.Bucket(slashingBucket)
		for i := 0; i < len(keys); i++ {
			err := b.Put(keys[i], append([]byte{byte(status)}, encSlashings[i]...))
			if err != nil {
				return err
			}
		}
		return nil
	})
}
