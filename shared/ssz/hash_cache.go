package ssz

import (
	"errors"
	"fmt"
	"reflect"
	"sort"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/prysmaticlabs/prysm/shared/hashutil"
	"github.com/prysmaticlabs/prysm/shared/params"
)

var (
	// ErrNotMerkleRoot will be returned when a cache object is not a merkle root
	ErrNotMerkleRoot = errors.New("object is not a merkle root")
	// maxCacheSize is 2x of the follow distance for additional cache padding.
	// Requests should be only accessing blocks within recent blocks within the
	// Eth1FollowDistance.
	maxCacheSize = params.BeaconConfig().HashCacheSize

	// Metrics
	hashCacheMiss = promauto.NewCounter(prometheus.CounterOpts{
		Name: "powchain_hash_cache_miss",
		Help: "The number of hash requests that aren't present in the cache.",
	})
	hashCacheHit = promauto.NewCounter(prometheus.CounterOpts{
		Name: "powchain_hash_cache_hit",
		Help: "The number of hash requests that are present in the cache.",
	})
	hashCacheSize = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "powchain_hash_cache_size",
		Help: "The number of hashes in the block cache",
	})
)

// hashCacheS struct with one queue for looking up by hash.
type hashCacheS struct {
	hashCache *ExpirationByUseCache
}

// markleRoot specifies the hash of data in a struct
type root struct {
	Hash       common.Hash
	MarkleRoot []byte
}

// hashKeyFn takes the  representation as the key for a hashInfo.
func hashKeyFn(obj interface{}) (string, error) {
	mRoot, ok := obj.(*root)
	if !ok {
		return "", ErrNotMerkleRoot
	}

	return mRoot.Hash.Hex(), nil
}

// newHashCache creates a new hash cache for storing/accessing root hashes from
// memory.
func newHashCache() *hashCacheS {
	return &hashCacheS{
		hashCache: NewTTLStoreRestampOnGet(hashKeyFn, time.Hour),
	}
}

// RootByEncodedHash fetches Root by the encoded hash of the object. Returns true with a
// reference to the root if exists. Otherwise returns false, nil.
func (b *hashCacheS) RootByEncodedHash(h common.Hash) (bool, *root, error) {

	obj, exists, err := b.hashCache.GetByKey(h.Hex())
	if err != nil {
		return false, nil, err
	}

	if exists {
		hashCacheHit.Inc()
	} else {
		hashCacheMiss.Inc()
		return false, nil, nil
	}

	hInfo, ok := obj.(*root)
	if !ok {
		return false, nil, ErrNotMerkleRoot
	}

	return true, hInfo, nil
}

// TrieRootCached computes a trie root and add it to the cache.
// if the encoded hash of the object is in cache, it will be retrieved from cache.
// This method also trims the least recently added root info. if the cache size
// has reached the max cache size limit.
func (b *hashCacheS) TrieRootCached(val interface{}) ([32]byte, error) {
	if val == nil {
		return [32]byte{}, newHashError("untyped nil is not supported", nil)
	}
	rval := reflect.ValueOf(val)
	hs, err := hashedEncoding(rval)
	if err != nil {
		return [32]byte{}, newHashError(fmt.Sprint(err), rval.Type())
	}
	exists, fetchedInfo, err := b.RootByEncodedHash(bytesutil.ToBytes32(hs))
	if err != nil {
		return [32]byte{}, newHashError(fmt.Sprint(err), rval.Type())
	}
	var paddedOutput [32]byte
	if exists {
		paddedOutput = bytesutil.ToBytes32(fetchedInfo.MarkleRoot)
	} else {
		sszUtils, err := cachedSSZUtils(rval.Type())
		if err != nil {
			return [32]byte{}, newHashError(fmt.Sprint(err), rval.Type())
		}
		output, err := sszUtils.hasher(rval)
		if err != nil {
			return [32]byte{}, newHashError(fmt.Sprint(err), rval.Type())
		}
		// Right-pad with 0 to make 32 bytes long, if necessary
		paddedOutput = bytesutil.ToBytes32(output)
		err = b.AddRoot(bytesutil.ToBytes32(hs), paddedOutput[:])
		if err != nil {
			return [32]byte{}, newHashError(fmt.Sprint(err), rval.Type())
		}
	}

	return paddedOutput, nil
}

// MerkleHashCached adds a mrakle object to the cache. This method also trims the
// least recently added root info if the cache size has reached the max cache
// size limit.
func (b *hashCacheS) MerkleHashCached(byteSlice [][]byte) ([]byte, error) {
	mh := []byte{}
	hs, err := hashedEncoding(reflect.ValueOf(byteSlice))
	if err != nil {
		return mh, newHashError(fmt.Sprint(err), reflect.TypeOf(byteSlice))
	}
	exists, fetchedInfo, err := b.RootByEncodedHash(bytesutil.ToBytes32(hs))
	if err != nil {
		return mh, newHashError(fmt.Sprint(err), reflect.TypeOf(byteSlice))
	}
	if exists {
		mh = fetchedInfo.MarkleRoot
	} else {
		mh, err = merkleHash(byteSlice)
		if err != nil {
			return nil, err
		}
		mr := &root{
			Hash:       bytesutil.ToBytes32(hs),
			MarkleRoot: mh,
		}
		if err := b.hashCache.Add(mr); err != nil {
			return nil, err
		}

		b.trim(maxCacheSize)

		hashCacheSize.Set(float64(len(b.hashCache.ListKeys())))
	}

	return mh, nil
}

// AddRoot adds an encodedhash of the object as key and a rootHash object to the cache.
// This method also trims the
// least recently added root info if the cache size has reached the max cache
// size limit.
func (b *hashCacheS) AddRoot(h common.Hash, rootB []byte) error {

	mr := &root{
		Hash:       h,
		MarkleRoot: rootB,
	}
	if err := b.hashCache.Add(mr); err != nil {
		return err
	}

	b.trim(maxCacheSize)

	hashCacheSize.Set(float64(len(b.hashCache.ListKeys())))

	return nil
}

// MakeSliceHasherCache add caching mechanism to slice hasher
func makeSliceHasherCache(typ reflect.Type) (hasher, error) {
	elemSSZUtils, err := cachedSSZUtilsNoAcquireLock(typ.Elem())
	if err != nil {
		return nil, fmt.Errorf("failed to get ssz utils: %v", err)
	}
	hasher := func(val reflect.Value) ([]byte, error) {
		hs, err := hashedEncoding(val)
		if err != nil {
			return nil, fmt.Errorf("failed to encode element of slice/array: %v", err)
		}
		exists, fetchedInfo, err := hashCache.RootByEncodedHash(bytesutil.ToBytes32(hs))
		if err != nil {
			return nil, fmt.Errorf("failed to encode element of slice/array: %v", err)
		}
		var output []byte
		if exists {
			output = fetchedInfo.MarkleRoot
		} else {
			var elemHashList [][]byte
			for i := 0; i < val.Len(); i++ {
				elemHash, err := elemSSZUtils.hasher(val.Index(i))
				if err != nil {
					return nil, fmt.Errorf("failed to hash element of slice/array: %v", err)
				}
				elemHashList = append(elemHashList, elemHash)
			}
			output, err = hashCache.MerkleHashCached(elemHashList)
			if err != nil {
				return nil, fmt.Errorf("failed to calculate merkle hash of element hash list: %v", err)
			}
			err := hashCache.AddRoot(bytesutil.ToBytes32(hs), output)
			if err != nil {
				return nil, fmt.Errorf("failed to add root to cache: %v", err)
			}
		}

		return output, nil
	}
	return hasher, nil
}

func makeStructHasherCache(typ reflect.Type) (hasher, error) {
	fields, err := structFields(typ)
	if err != nil {
		return nil, err
	}
	hasher := func(val reflect.Value) ([]byte, error) {
		hs, err := hashedEncoding(val)
		if err != nil {
			return nil, fmt.Errorf("failed to encode element of slice/array: %v", err)
		}
		exists, fetchedInfo, err := hashCache.RootByEncodedHash(bytesutil.ToBytes32(hs))
		if err != nil {
			return nil, fmt.Errorf("failed to encode element of slice/array: %v", err)
		}
		var result [32]byte
		if exists {
			result = bytesutil.ToBytes32(fetchedInfo.MarkleRoot)
		} else {
			concatElemHash := make([]byte, 0)
			for _, f := range fields {
				elemHash, err := f.sszUtils.hasher(val.Field(f.index))
				if err != nil {
					return nil, fmt.Errorf("failed to hash field of struct: %v", err)
				}
				concatElemHash = append(concatElemHash, elemHash...)
			}
			result = hashutil.Hash(concatElemHash)

		}

		return result[:], nil
	}
	return hasher, nil
}

// trim the store to the maxSize.
func (b *hashCacheS) trim(maxSize int) {
	b.hashCache.PurgeByDateAndSize(maxSize)
}

type byLeastUsed []timestampedKey

func (a byLeastUsed) Len() int           { return len(a) }
func (a byLeastUsed) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a byLeastUsed) Less(i, j int) bool { return a[i].timestamp.After(a[j].timestamp) }

// PurgeByDateAndSize purges the hash to its max size by least used items
func (c *ExpirationByUseCache) PurgeByDateAndSize(maxSize int) {
	items := c.cacheStorage.List()

	if len(items) > maxSize {
		list := make(byLeastUsed, 0, len(items))
		for _, item := range items {
			ts := item.(*timestampedEntry).timestamp
			obj := item.(*timestampedEntry).obj
			key, err := c.keyFunc(obj)
			if err == nil {
				list = append(list, timestampedKey{timestamp: ts, key: key})
			}
		}
		sort.Sort(list)
		c.expirationLock.Lock()
		defer c.expirationLock.Unlock()
		for s := len(list) - 1; s > maxSize-1; s-- {

			c.cacheStorage.Delete(list[s].key)
		}
	}

}
