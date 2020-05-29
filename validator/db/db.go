// Package db defines a persistent backend for the validator service.
package db

import (
	"os"
	"path/filepath"
	"time"

	"github.com/pkg/errors"
	"github.com/prysmaticlabs/prysm/validator/db/iface"
	"github.com/sirupsen/logrus"
	bolt "go.etcd.io/bbolt"
)

var log = logrus.WithField("prefix", "db")

var _ = iface.ValidatorDB(&Store{})

var databaseFileName = "validator.db"

// Store defines an implementation of the Prysm Database interface
// using BoltDB as the underlying persistent kv-store for eth2.
type Store struct {
	db           *bolt.DB
	databasePath string
}

// Close closes the underlying boltdb database.
func (db *Store) Close() error {
	return db.db.Close()
}

func (db *Store) update(fn func(*bolt.Tx) error) error {
	return db.db.Update(fn)
}
func (db *Store) batch(fn func(*bolt.Tx) error) error {
	return db.db.Batch(fn)
}
func (db *Store) view(fn func(*bolt.Tx) error) error {
	return db.db.View(fn)
}

// ClearDB removes any previously stored data at the configured data directory.
func (db *Store) ClearDB() error {
	if _, err := os.Stat(db.databasePath); os.IsNotExist(err) {
		return nil
	}
	return os.Remove(filepath.Join(db.databasePath, databaseFileName))
}

// DatabasePath at which this database writes files.
func (db *Store) DatabasePath() string {
	return db.databasePath
}

func createBuckets(tx *bolt.Tx, buckets ...[]byte) error {
	for _, bucket := range buckets {
		if _, err := tx.CreateBucketIfNotExists(bucket); err != nil {
			return err
		}
	}
	return nil
}

// NewKVStoreWithPublicKeyBuckets initializes a new boltDB key-value store at the directory
// path specified, creates the kv-buckets based on the schema and provided public keys,
// and stores an open connection db object as a property of the Store struct.
func NewKVStoreWithPublicKeyBuckets(dirPath string, pubKeys [][48]byte) (*Store, error) {
	kv, err := NewKVStore(dirPath)
	if err != nil {
		return nil, err
	}
	// Initialize the required public keys into the DB to ensure they're not empty.
	if err := kv.initializeSubBuckets(pubKeys); err != nil {
		return nil, err
	}
	return kv, err
}

// NewKVStore initializes a new boltDB key-value store at the directory path specified
// and stores an open connection db object as a property of the Store struct.
func NewKVStore(dirPath string) (*Store, error) {
	if err := os.MkdirAll(dirPath, 0700); err != nil {
		return nil, err
	}
	datafile := filepath.Join(dirPath, databaseFileName)
	boltDB, err := bolt.Open(datafile, 0600, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		if err == bolt.ErrTimeout {
			return nil, errors.New("cannot obtain database lock, database may be in use by another process")
		}
		return nil, err
	}

	kv := &Store{db: boltDB, databasePath: dirPath}

	if err := kv.db.Update(func(tx *bolt.Tx) error {
		return createBuckets(
			tx,
			historicProposalsBucket,
			historicAttestationsBucket,
		)
	}); err != nil {
		return nil, err
	}

	return kv, err
}

// GetKVStore returns the validator boltDB key-value store from directory. Returns nil if no such store exists.
func GetKVStore(directory string) (*Store, error) {
	fileName := filepath.Join(directory, databaseFileName)
	if _, err := os.Stat(fileName); os.IsNotExist(err) {
		return nil, nil
	}
	boltDb, err := bolt.Open(fileName, 0600, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		if err == bolt.ErrTimeout {
			return nil, errors.New("cannot obtain database lock, database may be in use by another process")
		}
		return nil, err
	}

	return &Store{db: boltDb, databasePath: directory}, nil
}

// Size returns the db size in bytes.
func (db *Store) Size() (int64, error) {
	var size int64
	err := db.db.View(func(tx *bolt.Tx) error {
		size = tx.Size()
		return nil
	})
	return size, err
}
