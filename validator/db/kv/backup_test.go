package kv

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/prysmaticlabs/prysm/shared/testutil/require"
)

func TestStore_Backup(t *testing.T) {
	db := setupDB(t, nil)
	ctx := context.Background()
	root := [32]byte{1}
	require.NoError(t, db.SaveGenesisValidatorsRoot(ctx, root[:]))
	require.NoError(t, db.Backup(ctx, ""))

	backupsPath := filepath.Join(db.databasePath, backupsDirectoryName)
	files, err := ioutil.ReadDir(backupsPath)
	require.NoError(t, err)
	require.NotEqual(t, 0, len(files), "No backups created")
	require.NoError(t, db.Close(), "Failed to close database")

	oldFilePath := filepath.Join(backupsPath, files[0].Name())
	newFilePath := filepath.Join(backupsPath, ProtectionDbFileName)
	// We rename the file to match the database file name
	// our NewKVStore function expects when opening a database.
	require.NoError(t, os.Rename(oldFilePath, newFilePath))

	backedDB, err := NewKVStore(ctx, backupsPath, &Config{})
	require.NoError(t, err, "Failed to instantiate DB")
	t.Cleanup(func() {
		require.NoError(t, backedDB.Close(), "Failed to close database")
	})
	genesisRoot, err := backedDB.GenesisValidatorsRoot(ctx)
	require.NoError(t, err)
	require.DeepEqual(t, root[:], genesisRoot)
}
