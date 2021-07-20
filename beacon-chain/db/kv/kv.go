// Package kv defines a bolt-db, key-value store implementation
// of the Database interface defined by a Prysm beacon node.
package kv

import (
	"context"
	"os"
	"path"
	"time"

	"github.com/dgraph-io/ristretto"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	prombolt "github.com/prysmaticlabs/prombbolt"
	"github.com/prysmaticlabs/prysm/beacon-chain/db/iface"
	"github.com/prysmaticlabs/prysm/shared/fileutil"
	"github.com/prysmaticlabs/prysm/shared/params"
	bolt "go.etcd.io/bbolt"
)

var _ iface.Database = (*Store)(nil)

const (
	// VotesCacheSize with 1M validators will be 8MB.
	VotesCacheSize = 1 << 23
	// NumOfVotes specifies the vote cache size.
	NumOfVotes = 1 << 20
	// ValidatorEntryCacheSize is set to 1MB since we expect ~100K validator entries (10 times the expected size)
	ValidatorEntryCacheSize = 1 << 20
	// NumOfValidatorEntries is set to ~32Mb so as to allow at least 100K validators entries.
	NumOfValidatorEntries = 1 << 25
	// BeaconNodeDbDirName is the name of the directory containing the beacon node database.
	BeaconNodeDbDirName = "beaconchaindata"
	// DatabaseFileName is the name of the beacon node database.
	DatabaseFileName = "beaconchain.db"

	boltAllocSize = 8 * 1024 * 1024
	// The size of hash length in bytes
	hashLength = 32
)

var (
	// Metrics for the validator cache.
	validatorEntryCacheHit = promauto.NewCounter(prometheus.CounterOpts{
		Name: "validator_entry_cache_hit",
		Help: "The total number of cache hits on the validator entry cache.",
	})
	validatorEntryCacheMiss = promauto.NewCounter(prometheus.CounterOpts{
		Name: "validator_entry_cache_miss",
		Help: "The total number of cache misses on the validator entry cache.",
	})
)

// BlockCacheSize specifies 1000 slots worth of blocks cached, which
// would be approximately 2MB
var BlockCacheSize = int64(1 << 21)

// blockedBuckets represents the buckets that we want to restrict
// from our metrics fetching for performance reasons. For a detailed
// summary, it can be read in https://github.com/prysmaticlabs/prysm/issues/8274.
var blockedBuckets = [][]byte{
	blocksBucket,
	stateSummaryBucket,
	blockParentRootIndicesBucket,
	blockSlotIndicesBucket,
	finalizedBlockRootsIndexBucket,
}

// Config for the bolt db kv store.
type Config struct {
	InitialMMapSize int
}

// Store defines an implementation of the Prysm Database interface
// using BoltDB as the underlying persistent kv-store for Ethereum Beacon Nodes.
type Store struct {
	db                  *bolt.DB
	databasePath        string
	blockCache          *ristretto.Cache
	validatorIndexCache *ristretto.Cache
	validatorEntryCache *ristretto.Cache
	stateSummaryCache   *stateSummaryCache
	ctx                 context.Context
}

// KVStoreDatafilePath is the canonical construction of a full
// database file path from the directory path, so that code outside
// this package can find the full path in a consistent way.
func KVStoreDatafilePath(dirPath string) string {
	return path.Join(dirPath, DatabaseFileName)
}

// NewKVStore initializes a new boltDB key-value store at the directory
// path specified, creates the kv-buckets based on the schema, and stores
// an open connection db object as a property of the Store struct.
func NewKVStore(ctx context.Context, dirPath string, config *Config) (*Store, error) {
	hasDir, err := fileutil.HasDir(dirPath)
	if err != nil {
		return nil, err
	}
	if !hasDir {
		if err := fileutil.MkdirAll(dirPath); err != nil {
			return nil, err
		}
	}
	datafile := KVStoreDatafilePath(dirPath)
	boltDB, err := bolt.Open(
		datafile,
		params.BeaconIoConfig().ReadWritePermissions,
		&bolt.Options{
			Timeout:         1 * time.Second,
			InitialMmapSize: config.InitialMMapSize,
		},
	)
	if err != nil {
		if errors.Is(err, bolt.ErrTimeout) {
			return nil, errors.New("cannot obtain database lock, database may be in use by another process")
		}
		return nil, err
	}
	boltDB.AllocSize = boltAllocSize
	blockCache, err := ristretto.NewCache(&ristretto.Config{
		NumCounters: 1000,           // number of keys to track frequency of (1000).
		MaxCost:     BlockCacheSize, // maximum cost of cache (1000 Blocks).
		BufferItems: 64,             // number of keys per Get buffer.
	})
	if err != nil {
		return nil, err
	}

	validatorIndexCache, err := ristretto.NewCache(&ristretto.Config{
		NumCounters: NumOfVotes,     // number of keys to track frequency of (1M).
		MaxCost:     VotesCacheSize, // maximum cost of cache (8MB).
		BufferItems: 64,             // number of keys per Get buffer.
	})
	if err != nil {
		return nil, err
	}

	validatorCache, err := ristretto.NewCache(&ristretto.Config{
		NumCounters: ValidatorEntryCacheSize, // number of keys to track frequency of (1M).
		MaxCost:     NumOfValidatorEntries,   // maximum size of cache
		BufferItems: 64,                      // number of keys per Get buffer.
	})
	if err != nil {
		return nil, err
	}

	kv := &Store{
		db:                  boltDB,
		databasePath:        dirPath,
		blockCache:          blockCache,
		validatorIndexCache: validatorIndexCache,
		validatorEntryCache: validatorCache,
		stateSummaryCache:   newStateSummaryCache(),
		ctx:                 ctx,
	}

	if err := kv.db.Update(func(tx *bolt.Tx) error {
		return createBuckets(
			tx,
			attestationsBucket,
			blocksBucket,
			stateBucket,
			proposerSlashingsBucket,
			attesterSlashingsBucket,
			voluntaryExitsBucket,
			chainMetadataBucket,
			checkpointBucket,
			powchainBucket,
			stateSummaryBucket,
			stateValidatorsBucket,
			// Indices buckets.
			attestationHeadBlockRootBucket,
			attestationSourceRootIndicesBucket,
			attestationSourceEpochIndicesBucket,
			attestationTargetRootIndicesBucket,
			attestationTargetEpochIndicesBucket,
			blockSlotIndicesBucket,
			stateSlotIndicesBucket,
			blockParentRootIndicesBucket,
			finalizedBlockRootsIndexBucket,
			blockRootValidatorHashesBucket,
			// State management service bucket.
			newStateServiceCompatibleBucket,
			// Migrations
			migrationsBucket,
		)
	}); err != nil {
		return nil, err
	}

	err = prometheus.Register(createBoltCollector(kv.db))

	return kv, err
}

// ClearDB removes the previously stored database in the data directory.
func (s *Store) ClearDB() error {
	if _, err := os.Stat(s.databasePath); os.IsNotExist(err) {
		return nil
	}
	prometheus.Unregister(createBoltCollector(s.db))
	if err := os.Remove(path.Join(s.databasePath, DatabaseFileName)); err != nil {
		return errors.Wrap(err, "could not remove database file")
	}
	return nil
}

// Close closes the underlying BoltDB database.
func (s *Store) Close() error {
	prometheus.Unregister(createBoltCollector(s.db))

	// Before DB closes, we should dump the cached state summary objects to DB.
	if err := s.saveCachedStateSummariesDB(s.ctx); err != nil {
		return err
	}

	return s.db.Close()
}

// DatabasePath at which this database writes files.
func (s *Store) DatabasePath() string {
	return s.databasePath
}

func createBuckets(tx *bolt.Tx, buckets ...[]byte) error {
	for _, bucket := range buckets {
		if _, err := tx.CreateBucketIfNotExists(bucket); err != nil {
			return err
		}
	}
	return nil
}

// createBoltCollector returns a prometheus collector specifically configured for boltdb.
func createBoltCollector(db *bolt.DB) prometheus.Collector {
	return prombolt.New("boltDB", db, blockedBuckets...)
}
