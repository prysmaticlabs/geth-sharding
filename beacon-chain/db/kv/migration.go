package kv

import (
	"context"

	bolt "go.etcd.io/bbolt"
)

var migrationCompleted = []byte("done")

type migration func(*bolt.Tx) error

var migrations = []migration{
	migrateArchivedIndex,
	migrateBlockSlotIndex,
	migrateStateValidators,
}

// RunMigrations defined in the migrations array.
func (s *Store) RunMigrations(ctx context.Context) error {
	for _, m := range migrations {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		if err := s.db.Update(m); err != nil {
			return err
		}
	}
	return nil
}
