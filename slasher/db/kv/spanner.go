package kv

import (
	"context"

	"github.com/boltdb/bolt"
	"github.com/pkg/errors"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/slasher/detection/attestations/types"
	log "github.com/sirupsen/logrus"
	"go.opencensus.io/trace"
)

// Tracks the highest and lowest observed epochs from the validator span maps
// used for attester slashing detection. This value is purely used
// as a cache key and only needs to be maintained in memory.
var highestObservedEpoch uint64
var lowestObservedEpoch = params.BeaconConfig().FarFutureEpoch

// This function defines a function which triggers upon a span map being
// evicted from the cache. It allows us to persist the span map by the epoch value
// to the database itself in the validatorsMinMaxSpanBucket.
func persistSpanMapsOnEviction(db *Store) func(key interface{}, value interface{}) {
	// We use a closure here so we can access the database itself
	// on the eviction of a span map from the cache. The function has the signature
	// required by the ristretto cache OnEvict method.
	// See https://godoc.org/github.com/dgraph-io/ristretto#Config.
	return func(key interface{}, value interface{}) {
		log.Tracef("Evicting span map for epoch: %d", key)
		err := db.update(func(tx *bolt.Tx) error {
			epoch, keyOK := key.(uint64)
			spanMap, valueOK := value.(map[uint64]types.Span)
			if !keyOK || !valueOK {
				return errors.New("could not cast key and value into needed types")
			}

			bucket := tx.Bucket(validatorsMinMaxSpanBucket)
			epochBucket, err := bucket.CreateBucketIfNotExists(bytesutil.Bytes8(epoch))
			if err != nil {
				return err
			}
			for k, v := range spanMap {
				if err = epochBucket.Put(bytesutil.Bytes8(k), marshalSpan(v)); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			log.Errorf("Failed to save span map to db on cache eviction: %v", err)
		}
	}
}

// Unmarshal a span map from an encoded, flattened array.
func unmarshalSpan(ctx context.Context, enc []byte) (types.Span, error) {
	ctx, span := trace.StartSpan(ctx, "slasherDB.unmarshalSpan")
	defer span.End()
	r := types.Span{}
	if len(enc) != spannerEncodedLength {
		return r, errors.New("wrong data length for min max span")
	}
	r.MinSpan = bytesutil.FromBytes2(enc[:2])
	r.MaxSpan = bytesutil.FromBytes2(enc[2:4])
	sigB := [2]byte{}
	copy(sigB[:], enc[4:6])
	r.SigBytes = sigB
	r.HasAttested = bytesutil.ToBool(enc[6])
	return r, nil
}

// Convert the span struct into a flattened array.
func marshalSpan(span types.Span) []byte {
	return append(append(append(
		bytesutil.Bytes2(uint64(span.MinSpan)),
		bytesutil.Bytes2(uint64(span.MaxSpan))...),
		span.SigBytes[:]...),
		bytesutil.FromBool(span.HasAttested),
	)
}

// EpochSpansMap accepts epoch and returns the corresponding spans map epoch=>spans
// for slashing detection. This function reads spans from cache if caching is
// enabled and the epoch key exists. Returns nil if the span map
// for this validator index does not exist.
func (db *Store) EpochSpansMap(ctx context.Context, epoch uint64) (map[uint64]types.Span, error) {
	ctx, span := trace.StartSpan(ctx, "slasherDB.EpochSpansMap")
	defer span.End()
	if db.spanCacheEnabled {
		spanMap, ok := db.spanCache.Get(epoch)
		if ok {
			return spanMap, nil
		}
	}

	var err error
	var spanMap map[uint64]types.Span
	err = db.view(func(tx *bolt.Tx) error {
		b := tx.Bucket(validatorsMinMaxSpanBucket)
		epochBucket := b.Bucket(bytesutil.Bytes8(epoch))
		if epochBucket == nil {
			return nil
		}
		keysLength := epochBucket.Stats().KeyN
		spanMap = make(map[uint64]types.Span, keysLength)
		return epochBucket.ForEach(func(k, v []byte) error {
			key := bytesutil.FromBytes8(k)
			value, err := unmarshalSpan(ctx, v)
			if err != nil {
				return err
			}
			spanMap[key] = value
			return nil
		})
	})
	if spanMap == nil {
		spanMap = make(map[uint64]types.Span)
	}
	return spanMap, err
}

// EpochSpanByValidatorIndex accepts validator index and epoch returns the corresponding spans
// for slashing detection.
// it reads the epoch spans from cache and gets the requested value from there if it exists
// when caching is enabled.
// Returns error if the spans for this validator index and epoch does not exist.
func (db *Store) EpochSpanByValidatorIndex(ctx context.Context, validatorIdx uint64, epoch uint64) (types.Span, error) {
	ctx, span := trace.StartSpan(ctx, "slasherDB.EpochSpanByValidatorIndex")
	defer span.End()
	if db.spanCacheEnabled {
		setObservedEpochs(epoch)
		spanMap, err := db.findOrLoadEpochInCache(ctx, epoch)
		if err != nil {
			return types.Span{}, err
		}
		spans, ok := spanMap[validatorIdx]
		if ok {
			return spans, nil
		}
		return types.Span{}, nil
	}

	var spans types.Span
	err := db.view(func(tx *bolt.Tx) error {
		b := tx.Bucket(validatorsMinMaxSpanBucket)
		epochBucket := b.Bucket(bytesutil.Bytes8(epoch))
		if epochBucket == nil {
			return nil
		}
		key := bytesutil.Bytes8(validatorIdx)
		v := epochBucket.Get(key)
		if v == nil {
			return nil
		}
		value, err := unmarshalSpan(ctx, v)
		if err != nil {
			return err
		}
		spans = value
		return nil
	})
	return spans, err
}

// SaveValidatorEpochSpan accepts validator index epoch and spans returns.
// it reads the epoch spans from cache, updates it and save it back to cache
// if caching is enabled.
// Returns error if the spans for this validator index and epoch does not exist.
func (db *Store) SaveValidatorEpochSpan(
	ctx context.Context,
	validatorIdx uint64,
	epoch uint64,
	span types.Span,
) error {
	ctx, traceSpan := trace.StartSpan(ctx, "slasherDB.SaveValidatorEpochSpan")
	defer traceSpan.End()
	if db.spanCacheEnabled {
		setObservedEpochs(epoch)
		spanMap, err := db.findOrLoadEpochInCache(ctx, epoch)
		if err != nil {
			return err
		}
		spanMap[validatorIdx] = span
		db.spanCache.Set(epoch, spanMap)
		return nil
	}

	return db.update(func(tx *bolt.Tx) error {
		b := tx.Bucket(validatorsMinMaxSpanBucket)
		epochBucket, err := b.CreateBucketIfNotExists(bytesutil.Bytes8(epoch))
		if err != nil {
			return err
		}
		key := bytesutil.Bytes8(validatorIdx)
		value := marshalSpan(span)
		return epochBucket.Put(key, value)
	})
}

// SaveEpochSpansMap accepts a epoch and span map epoch=>spans and writes it to disk.
// saves the spans to cache if caching is enabled. The key in the cache is the
// epoch and the value is the span map itself.
func (db *Store) SaveEpochSpansMap(ctx context.Context, epoch uint64, spanMap map[uint64]types.Span) error {
	ctx, span := trace.StartSpan(ctx, "slasherDB.SaveEpochSpansMap")
	defer span.End()
	if db.spanCacheEnabled {
		setObservedEpochs(epoch)
		db.spanCache.Set(epoch, spanMap)
		return nil
	}

	return db.update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(validatorsMinMaxSpanBucket)
		valBucket, err := bucket.CreateBucketIfNotExists(bytesutil.Bytes8(epoch))
		if err != nil {
			return err
		}
		for k, v := range spanMap {
			err = valBucket.Put(bytesutil.Bytes8(k), marshalSpan(v))
			if err != nil {
				return err
			}
		}
		return nil
	})
}

func (db *Store) enableSpanCache(enable bool) {
	db.spanCacheEnabled = enable
}

// SaveCachedSpansMaps saves all span maps that are currently
// in memory into the DB. if no span maps are in db or cache is disabled it returns nil.
func (db *Store) SaveCachedSpansMaps(ctx context.Context) error {
	ctx, span := trace.StartSpan(ctx, "slasherDB.SaveCachedSpansMaps")
	defer span.End()
	if db.spanCacheEnabled {
		db.enableSpanCache(false)
		defer db.enableSpanCache(true)
		for epoch := lowestObservedEpoch; epoch <= highestObservedEpoch; epoch++ {
			spanMap, ok := db.spanCache.Get(epoch)
			if ok {
				if err := db.SaveEpochSpansMap(ctx, epoch, spanMap); err != nil {
					return errors.Wrap(err, "failed to save span maps from cache")
				}
			}
		}
		// Reset the observed epochs after saving to the DB.
		lowestObservedEpoch = params.BeaconConfig().FarFutureEpoch
		highestObservedEpoch = 0
		log.Debugf("Epochs %d to %d have been saved", lowestObservedEpoch, highestObservedEpoch)
	}
	return nil
}

// DeleteEpochSpans deletes a epochs validators span map using a epoch index as bucket key.
func (db *Store) DeleteEpochSpans(ctx context.Context, epoch uint64) error {
	ctx, span := trace.StartSpan(ctx, "slasherDB.DeleteEpochSpans")
	defer span.End()
	if db.spanCacheEnabled {
		_ = db.spanCache.Delete(epoch)
		return nil
	}
	return db.update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(validatorsMinMaxSpanBucket)
		key := bytesutil.Bytes8(epoch)
		return bucket.DeleteBucket(key)
	})
}

// DeleteValidatorSpanByEpoch deletes a validator span for a certain epoch
// deletes spans from cache if caching is enabled.
// using a validator index as bucket key.
func (db *Store) DeleteValidatorSpanByEpoch(ctx context.Context, validatorIdx uint64, epoch uint64) error {
	ctx, span := trace.StartSpan(ctx, "slasherDB.DeleteValidatorSpanByEpoch")
	defer span.End()
	if db.spanCacheEnabled {
		spanMap, ok := db.spanCache.Get(epoch)
		if ok {
			delete(spanMap, validatorIdx)
			db.spanCache.Set(epoch, spanMap)
			return nil
		}
	}

	return db.update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(validatorsMinMaxSpanBucket)
		e := bytesutil.Bytes8(epoch)
		epochBucket := bucket.Bucket(e)
		v := bytesutil.Bytes8(validatorIdx)
		return epochBucket.Delete(v)
	})
}

// findOrLoadEpochInCache checks if the requested epoch is in the cache, and if not, we load it from the DB.
func (db *Store) findOrLoadEpochInCache(ctx context.Context, epoch uint64) (map[uint64]types.Span, error) {
	ctx, span := trace.StartSpan(ctx, "slasherDB.findOrLoadEpochInCache")
	defer span.End()
	spanMap, epochFound := db.spanCache.Get(epoch)
	if epochFound {
		return spanMap, nil
	}

	db.enableSpanCache(false)
	defer db.enableSpanCache(true)
	// If the epoch we want isn't in the cache, load it in.
	spanForEpoch, err := db.EpochSpansMap(ctx, epoch)
	if err != nil {
		return make(map[uint64]types.Span), errors.Wrap(err, "failed to get span map for epoch")
	}
	db.spanCache.Set(epoch, spanForEpoch)
	return spanForEpoch, nil
}

func setObservedEpochs(epoch uint64) {
	if epoch > highestObservedEpoch {
		highestObservedEpoch = epoch
	}
	if epoch < lowestObservedEpoch {
		lowestObservedEpoch = epoch
	}
}
