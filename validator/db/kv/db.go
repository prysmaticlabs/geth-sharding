// Package kv defines a persistent backend for the validator service.
package kv

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	prombolt "github.com/prysmaticlabs/prombbolt"
	"github.com/prysmaticlabs/prysm/shared/abool"
	"github.com/prysmaticlabs/prysm/shared/event"
	"github.com/prysmaticlabs/prysm/shared/fileutil"
	"github.com/prysmaticlabs/prysm/shared/params"
	bolt "go.etcd.io/bbolt"
)

const (
	// Number of attestation records we can hold in memory
	// before we flush them to the database. Roughly corresponds
	// to the max number of keys per validator client, but there is no
	// detriment if there are more keys than this capacity, as attestations
	// for those keys will simply be flushed at the next flush interval.
	attestationBatchCapacity = 2048
	// Time interval after which we flush attestation records to the database
	// from a batch kept in memory for slashing protection.
	attestationBatchWriteInterval = time.Millisecond * 100
)

// ProtectionDbFileName Validator slashing protection db file name.
var (
	ProtectionDbFileName = "validator.db"
)

// blockedBuckets represents the buckets that we want to restrict
// from our metrics fetching for performance reasons. For a detailed
// summary, it can be read in https://github.com/prysmaticlabs/prysm/issues/8274.
var blockedBuckets = [][]byte{
	deprecatedAttestationHistoryBucket,
	lowestSignedSourceBucket,
	lowestSignedTargetBucket,
	lowestSignedProposalsBucket,
	highestSignedProposalsBucket,
	pubKeysBucket,
	attestationSigningRootsBucket,
	attestationSourceEpochsBucket,
}

// Store defines an implementation of the Prysm Database interface
// using BoltDB as the underlying persistent kv-store for eth2.
type Store struct {
	db                                 *bolt.DB
	databasePath                       string
	batchedAttestations                *QueuedAttestationRecords
	batchedAttestationsChan            chan *AttestationRecord
	batchAttestationsFlushedFeed       *event.Feed
	batchedAttestationsFlushInProgress abool.AtomicBool
}

// Close closes the underlying boltdb database.
func (s *Store) Close() error {
	prometheus.Unregister(createBoltCollector(s.db))
	return s.db.Close()
}

func (s *Store) update(fn func(*bolt.Tx) error) error {
	return s.db.Update(fn)
}
func (s *Store) view(fn func(*bolt.Tx) error) error {
	return s.db.View(fn)
}

// ClearDB removes any previously stored data at the configured data directory.
func (s *Store) ClearDB() error {
	if _, err := os.Stat(s.databasePath); os.IsNotExist(err) {
		return nil
	}
	prometheus.Unregister(createBoltCollector(s.db))
	return os.Remove(filepath.Join(s.databasePath, ProtectionDbFileName))
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

// NewKVStore initializes a new boltDB key-value store at the directory
// path specified, creates the kv-buckets based on the schema, and stores
// an open connection db object as a property of the Store struct.
func NewKVStore(ctx context.Context, dirPath string, pubKeys [][48]byte) (*Store, error) {
	hasDir, err := fileutil.HasDir(dirPath)
	if err != nil {
		return nil, err
	}
	if !hasDir {
		if err := fileutil.MkdirAll(dirPath); err != nil {
			return nil, err
		}
	}
	datafile := filepath.Join(dirPath, ProtectionDbFileName)
	boltDB, err := bolt.Open(datafile, params.BeaconIoConfig().ReadWritePermissions, &bolt.Options{Timeout: params.BeaconIoConfig().BoltTimeout})
	if err != nil {
		if errors.Is(err, bolt.ErrTimeout) {
			return nil, errors.New("cannot obtain database lock, database may be in use by another process")
		}
		return nil, err
	}

	kv := &Store{
		db:                           boltDB,
		databasePath:                 dirPath,
		batchedAttestations:          NewPendingAttestationRecords(),
		batchedAttestationsChan:      make(chan *AttestationRecord, attestationBatchCapacity),
		batchAttestationsFlushedFeed: new(event.Feed),
	}

	if err := kv.db.Update(func(tx *bolt.Tx) error {
		return createBuckets(
			tx,
			genesisInfoBucket,
			deprecatedAttestationHistoryBucket,
			historicProposalsBucket,
			lowestSignedSourceBucket,
			lowestSignedTargetBucket,
			lowestSignedProposalsBucket,
			highestSignedProposalsBucket,
			slashablePublicKeysBucket,
			pubKeysBucket,
			migrationsBucket,
		)
	}); err != nil {
		return nil, err
	}

	// Initialize the required public keys into the DB to ensure they're not empty.
	if pubKeys != nil {
		if err := kv.UpdatePublicKeysBuckets(pubKeys); err != nil {
			return nil, err
		}
	}

	// Prune attesting records older than the current weak subjectivity period.
	if err := kv.PruneAttestationsOlderThanCurrentWeakSubjectivity(ctx); err != nil {
		return nil, errors.Wrap(err, "could not prune old attestations from DB")
	}

	// Batch save attestation records for slashing protection at timed
	// intervals to our database.
	go kv.batchAttestationWrites(ctx)

	return kv, prometheus.Register(createBoltCollector(kv.db))
}

// UpdatePublicKeysBuckets for a specified list of keys.
func (s *Store) UpdatePublicKeysBuckets(pubKeys [][48]byte) error {
	return s.update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(historicProposalsBucket)
		for _, pubKey := range pubKeys {
			if _, err := bucket.CreateBucketIfNotExists(pubKey[:]); err != nil {
				return errors.Wrap(err, "failed to create proposal history bucket")
			}
		}
		return nil
	})
}

// Size returns the db size in bytes.
func (s *Store) Size() (int64, error) {
	var size int64
	err := s.db.View(func(tx *bolt.Tx) error {
		size = tx.Size()
		return nil
	})
	return size, err
}

// createBoltCollector returns a prometheus collector specifically configured for boltdb.
func createBoltCollector(db *bolt.DB) prometheus.Collector {
	return prombolt.New("boltDB", db, blockedBuckets...)
}
