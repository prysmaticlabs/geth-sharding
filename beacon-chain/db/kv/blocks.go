package kv

import (
	"bytes"
	"context"
	"fmt"

	"github.com/boltdb/bolt"
	"github.com/gogo/protobuf/proto"
	"github.com/pkg/errors"
	"github.com/prysmaticlabs/go-ssz"
	"github.com/prysmaticlabs/prysm/beacon-chain/db/filters"
	ethpb "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/shared/sliceutil"
)

// Block retrieval by root.
func (k *Store) Block(ctx context.Context, blockRoot [32]byte) (*ethpb.BeaconBlock, error) {
	block := &ethpb.BeaconBlock{}
	err := k.db.View(func(tx *bolt.Tx) error {
		bkt := tx.Bucket(blocksBucket)
		enc := bkt.Get(blockRoot[:])
		if enc == nil {
			return nil
		}
		return proto.Unmarshal(enc, block)
	})
	return block, err
}

// HeadBlock returns the latest canonical block in eth2.
func (k *Store) HeadBlock(ctx context.Context) (*ethpb.BeaconBlock, error) {
	headBlock := &ethpb.BeaconBlock{}
	err := k.db.View(func(tx *bolt.Tx) error {
		bkt := tx.Bucket(validatorsBucket)
		headRoot := bkt.Get(headBlockRootKey)
		if headRoot == nil {
			return nil
		}
		enc := bkt.Get(headRoot)
		if enc == nil {
			return nil
		}
		return proto.Unmarshal(enc, headBlock)
	})
	return headBlock, err
}

// Blocks retrieves a list of beacon blocks by filter criteria.
func (k *Store) Blocks(ctx context.Context, f *filters.QueryFilter) ([]*ethpb.BeaconBlock, error) {
	blocks := make([]*ethpb.BeaconBlock, 0)
	err := k.db.Batch(func(tx *bolt.Tx) error {
		bkt := tx.Bucket(blocksBucket)

		// If no filter criteria are specified, return all blocks.
		if f == nil {
			return bkt.ForEach(func(k, v []byte) error {
				block := &ethpb.BeaconBlock{}
				if err := proto.Unmarshal(v, block); err != nil {
					return err
				}
				blocks = append(blocks, block)
				return nil
			})
		}

		// Check if has range filters.
		filtersMap := f.Filters()
		slotIndicesBkt := tx.Bucket(blockSlotIndicesBucket)
		rootsBySlotRange := fetchBlockRootsBySlotRange(
			slotIndicesBkt.Cursor(),
			filtersMap[filters.StartSlot].(uint64),
			filtersMap[filters.EndSlot].(uint64),
		)

		// Creates a list of indices from the passed in filter values, such as:
		// []byte("parent-root-0x2093923"), etc. to be used for looking up
		// block roots that were stored under each of those indices for O(1) lookup.
		indicesByBucket, err := createBlockIndicesFromFilters(f, tx.Bucket)
		if err != nil {
			return errors.Wrap(err, "could not determine block lookup indices")
		}
		// Once we have a list of block roots that correspond to each
		// lookup index, we find the intersection across all of them and use
		// that list of roots to lookup the block. These block will
		// meet the filter criteria.
		indices := lookupValuesForIndices(indicesByBucket)
		keys := vals
		if len(indices) > 0 {
			if len(vals) > 0 {
				joined := append([][][]byte{vals}, indices...)
				keys = sliceutil.IntersectionByteSlices(joined...)
			} else {
				keys = sliceutil.IntersectionByteSlices(indices...)
			}
		}
		for i := 0; i < len(keys); i++ {
			encoded := bkt.Get(keys[i])
			block := &ethpb.BeaconBlock{}
			if err := proto.Unmarshal(encoded, block); err != nil {
				return err
			}
			blocks = append(blocks, block)
		}
		return nil
	})
	return blocks, err
}

// BlockRoots retrieves a list of beacon block roots by filter criteria.
func (k *Store) BlockRoots(ctx context.Context, f *filters.QueryFilter) ([][]byte, error) {
	blocks, err := k.Blocks(ctx, f)
	if err != nil {
		return nil, err
	}
	roots := make([][]byte, len(blocks))
	for i, b := range blocks {
		root, err := ssz.SigningRoot(b)
		if err != nil {
			return nil, err
		}
		roots[i] = root[:]
	}
	return roots, nil
}

// HasBlock checks if a block by root exists in the db.
func (k *Store) HasBlock(ctx context.Context, blockRoot [32]byte) bool {
	exists := false
	// #nosec G104. Always returns nil.
	k.db.View(func(tx *bolt.Tx) error {
		bkt := tx.Bucket(blocksBucket)
		exists = bkt.Get(blockRoot[:]) != nil
		return nil
	})
	return exists
}

// DeleteBlock by block root.
// TODO(#3064): Add the ability for batch deletions.
func (k *Store) DeleteBlock(ctx context.Context, blockRoot [32]byte) error {
	return k.db.Update(func(tx *bolt.Tx) error {
		bkt := tx.Bucket(blocksBucket)
		enc := bkt.Get(blockRoot[:])
		if enc == nil {
			return nil
		}
		block := &ethpb.BeaconBlock{}
		if err := proto.Unmarshal(enc, block); err != nil {
			return err
		}
		indicesByBucket := createBlockIndicesFromBlock(block, tx)
		if err := deleteValueForIndices(indicesByBucket, blockRoot[:]); err != nil {
			return errors.Wrap(err, "could not delete root for DB indices")
		}
		return bkt.Delete(blockRoot[:])
	})
}

// SaveBlock to the db.
func (k *Store) SaveBlock(ctx context.Context, block *ethpb.BeaconBlock) error {
	blockRoot, err := ssz.SigningRoot(block)
	if err != nil {
		return err
	}
	enc, err := proto.Marshal(block)
	if err != nil {
		return err
	}
	return k.db.Update(func(tx *bolt.Tx) error {
		bkt := tx.Bucket(blocksBucket)
		indicesByBucket := createBlockIndicesFromBlock(block, tx)
		if err := updateValueForIndices(indicesByBucket, blockRoot[:]); err != nil {
			return errors.Wrap(err, "could not update DB indices")
		}
		return bkt.Put(blockRoot[:], enc)
	})
}

// SaveBlocks via batch updates to the db.
func (k *Store) SaveBlocks(ctx context.Context, blocks []*ethpb.BeaconBlock) error {
	encodedValues := make([][]byte, len(blocks))
	keys := make([][]byte, len(blocks))
	for i := 0; i < len(blocks); i++ {
		enc, err := proto.Marshal(blocks[i])
		if err != nil {
			return err
		}
		key, err := ssz.SigningRoot(blocks[i])
		if err != nil {
			return err
		}
		encodedValues[i] = enc
		keys[i] = key[:]
	}
	return k.db.Batch(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(blocksBucket)
		for i := 0; i < len(blocks); i++ {
			indicesByBucket := createBlockIndicesFromBlock(blocks[i], tx)
			if err := updateValueForIndices(indicesByBucket, keys[i]); err != nil {
				return errors.Wrap(err, "could not update DB indices")
			}
			if err := bucket.Put(keys[i], encodedValues[i]); err != nil {
				return err
			}
		}
		return nil
	})
}

// SaveHeadBlockRoot to the db.
func (k *Store) SaveHeadBlockRoot(ctx context.Context, blockRoot [32]byte) error {
	return k.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(blocksBucket)
		return bucket.Put(headBlockRootKey, blockRoot[:])
	})
}

func fetchBlockRootsBySlotRange(cursor *bolt.Cursor, startSlot, endSlot uint64) [][]byte {
	if startSlot == endSlot && startSlot == 0 {
		return nil
	}
	min := []byte(fmt.Sprintf("%07d", startSlot))
	max := []byte(fmt.Sprintf("%07d", endSlot))
	var conditional func(key, max []byte) bool
	if endSlot == 0 {
		conditional = func(k, max []byte) bool {
			return k != nil
		}
	} else {
		conditional = func(k, max []byte) bool {
			return k != nil && bytes.Compare(k, max) <= 0
		}
	}
	vals := make([][]byte, 0)
	if hasStart || hasEnd {
		for k, v := cursor.Seek(min); conditional(k, max); k, v = cursor.Next() {
			splitRoots := make([][]byte, 0)
			for i := 0; i < len(v); i += 32 {
				splitRoots = append(splitRoots, v[i:i+32])
			}
			vals = append(vals, splitRoots...)
		}
	}
	return vals
}

// createBlockIndicesFromBlock takes in a beacon block and returns
// a map of bolt DB index buckets corresponding to each particular key for indices for
// data, such as (shard indices bucket -> shard 5).
func createBlockIndicesFromBlock(block *ethpb.BeaconBlock, tx *bolt.Tx) map[*bolt.Bucket][]byte {
	indicesByBucket := make(map[*bolt.Bucket][]byte)
	// Every index has a unique bucket for fast, binary-search
	// range scans for filtering across keys.
	buckets := []*bolt.Bucket{
		tx.Bucket(parentRootIndicesBucket),
		tx.Bucket(blockSlotIndicesBucket),
	}
	indices := [][]byte{
		block.ParentRoot,
		[]byte(fmt.Sprintf("%07d", block.Slot)),
	}
	for i := 0; i < len(buckets); i++ {
		indicesByBucket[buckets[i]] = indices[i]
	}
	return indicesByBucket
}

// createBlockFiltersFromIndices takes in filter criteria and returns
// a list of of byte keys used to retrieve the values stored
// for the indices from the DB.
//
// For blocks, these are list of signing roots of block
// objects. If a certain filter criterion does not apply to
// blocks, an appropriate error is returned.
func createBlockIndicesFromFilters(f *filters.QueryFilter, readBucket func(b []byte) *bolt.Bucket) (map[*bolt.Bucket][]byte, error) {
	indicesByBucket := make(map[*bolt.Bucket][]byte)
	for k, v := range f.Filters() {
		switch k {
		case filters.ParentRoot:
			parentRoot := v.([]byte)
			indicesByBucket[readBucket(parentRootIndicesBucket)] = parentRoot
		case filters.StartSlot:
		case filters.EndSlot:
		default:
			return nil, fmt.Errorf("filter criterion %v not supported for blocks", k)
		}
	}
	return indicesByBucket, nil
}
