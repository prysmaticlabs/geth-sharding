package kv

import (
	"context"
	"testing"

	"github.com/prysmaticlabs/prysm/shared/testutil"
	"github.com/prysmaticlabs/prysm/shared/testutil/assert"
	"github.com/prysmaticlabs/prysm/shared/testutil/require"
)

func TestArchivedPointIndexRoot_CanSaveRetrieve(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()
	i1 := uint64(100)
	r1 := [32]byte{'A'}

	received := db.ArchivedPointRoot(ctx, i1)
	require.NotEqual(t, r1, received, "Should not have been saved")
	st, err := testutil.NewBeaconState()
	require.NoError(t, err)
	require.NoError(t, st.SetSlot(i1))
	require.NoError(t, db.SaveState(ctx, st, r1))
	received = db.ArchivedPointRoot(ctx, i1)
	assert.Equal(t, r1, received, "Should have been saved")
}

func TestLastArchivedPoint_CanRetrieve(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()
	i, err := db.LastArchivedSlot(ctx)
	require.NoError(t, err)
	assert.Equal(t, uint64(0), i, "Did not get correct index")

	st, err := testutil.NewBeaconState()
	require.NoError(t, err)
	assert.NoError(t, db.SaveState(ctx, st, [32]byte{'A'}))
	assert.Equal(t, [32]byte{'A'}, db.LastArchivedRoot(ctx), "Did not get wanted root")

	assert.NoError(t, st.SetSlot(2))
	assert.NoError(t, db.SaveState(ctx, st, [32]byte{'B'}))
	assert.Equal(t, [32]byte{'B'}, db.LastArchivedRoot(ctx))

	assert.NoError(t, st.SetSlot(3))
	assert.NoError(t, db.SaveState(ctx, st, [32]byte{'C'}))

	i, err = db.LastArchivedSlot(ctx)
	require.NoError(t, err)
	assert.Equal(t, uint64(3), i, "Did not get correct index")
}
