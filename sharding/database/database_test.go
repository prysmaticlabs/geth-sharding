package database

import (
	"strconv"
	"testing"

	"github.com/ethereum/go-ethereum/sharding"
)

// Verifies that ShardDB implements the sharding Service inteface.
var _ = sharding.Service(&ShardDB{})

var testdb ShardDB

func init() {
	shardDB, err := NewShardDB("/tmp/datadir", "shardchaindata")
	if err != nil {
		panic(err)
	}

	testdb = *shardDB
}

// Testing the concurrency of the shardDB with multiple processes attempting to write.
func Test_DBConcurrent(t *testing.T) {
	for i := 0; i < 100; i++ {
		go func(val string) {
			if err := testdb.db.Put([]byte("ralph merkle"), []byte(val)); err != nil {
				t.Errorf("could not save value in db: %v", err)
			}
		}(strconv.Itoa(i))
	}
}

func Test_DBPut(t *testing.T) {
	if err := testdb.db.Put([]byte("ralph merkle"), []byte{1, 2, 3}); err != nil {
		t.Errorf("could not save value in db: %v", err)
	}
}

func Test_DBHas(t *testing.T) {
	key := []byte("ralph merkle")

	if err := testdb.db.Put(key, []byte{1, 2, 3}); err != nil {
		t.Fatalf("could not save value in db: %v", err)
	}

	has, err := testdb.db.Has(key)
	if err != nil {
		t.Errorf("could not check if db has key: %v", err)
	}
	if !has {
		t.Errorf("db should have key: %v", key)
	}

	key2 := []byte{}
	has2, err := testdb.db.Has(key2)
	if err != nil {
		t.Errorf("could not check if db has key: %v", err)
	}
	if has2 {
		t.Errorf("db should not have non-existent key: %v", key2)
	}
}

func Test_DBGet(t *testing.T) {
	key := []byte("ralph merkle")

	if err := testdb.db.Put(key, []byte{1, 2, 3}); err != nil {
		t.Fatalf("could not save value in db: %v", err)
	}

	val, err := testdb.db.Get(key)
	if err != nil {
		t.Errorf("get failed: %v", err)
	}
	if len(val) == 0 {
		t.Errorf("no value stored for key")
	}

	key2 := []byte{}
	val2, err := testdb.db.Get(key2)
	if len(val2) != 0 {
		t.Errorf("non-existent key should not have a value. key=%v, value=%v", key2, val2)
	}
}

func Test_DBDelete(t *testing.T) {
	key := []byte("ralph merkle")

	if err := testdb.db.Put(key, []byte{1, 2, 3}); err != nil {
		t.Fatalf("could not save value in db: %v", err)
	}

	if err := testdb.db.Delete(key); err != nil {
		t.Errorf("could not delete key: %v", key)
	}
}
