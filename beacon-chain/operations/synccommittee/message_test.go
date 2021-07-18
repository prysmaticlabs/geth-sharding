package synccommittee

import (
	"testing"

	prysmv2 "github.com/prysmaticlabs/prysm/proto/prysm/v2"
	"github.com/prysmaticlabs/prysm/shared/testutil/require"
)

func TestSyncCommitteeSignatureCache_Nil(t *testing.T) {
	store := NewStore()
	require.Equal(t, nilMessageErr, store.SaveSyncCommitteeMessage(nil))
}

func TestSyncCommitteeSignatureCache_RoundTrip(t *testing.T) {
	store := NewStore()

	msgs := []*prysmv2.SyncCommitteeMessage{
		{Slot: 1, ValidatorIndex: 0, Signature: []byte{'a'}},
		{Slot: 1, ValidatorIndex: 1, Signature: []byte{'b'}},
		{Slot: 2, ValidatorIndex: 0, Signature: []byte{'c'}},
		{Slot: 2, ValidatorIndex: 1, Signature: []byte{'d'}},
		{Slot: 3, ValidatorIndex: 0, Signature: []byte{'e'}},
		{Slot: 3, ValidatorIndex: 1, Signature: []byte{'f'}},
		{Slot: 4, ValidatorIndex: 0, Signature: []byte{'g'}},
		{Slot: 4, ValidatorIndex: 1, Signature: []byte{'h'}},
		{Slot: 5, ValidatorIndex: 0, Signature: []byte{'i'}},
		{Slot: 5, ValidatorIndex: 1, Signature: []byte{'j'}},
		{Slot: 6, ValidatorIndex: 0, Signature: []byte{'k'}},
		{Slot: 6, ValidatorIndex: 1, Signature: []byte{'l'}},
	}

	for _, msg := range msgs {
		require.NoError(t, store.SaveSyncCommitteeMessage(msg))
	}

	msgs, err := store.SyncCommitteeMessages(1)
	require.NoError(t, err)
	require.DeepSSZEqual(t, []*prysmv2.SyncCommitteeMessage(nil), msgs)

	msgs, err = store.SyncCommitteeMessages(2)
	require.NoError(t, err)
	require.DeepSSZEqual(t, []*prysmv2.SyncCommitteeMessage(nil), msgs)

	msgs, err = store.SyncCommitteeMessages(3)
	require.NoError(t, err)
	require.DeepSSZEqual(t, []*prysmv2.SyncCommitteeMessage{
		{Slot: 3, ValidatorIndex: 0, Signature: []byte{'e'}},
		{Slot: 3, ValidatorIndex: 1, Signature: []byte{'f'}},
	}, msgs)

	msgs, err = store.SyncCommitteeMessages(4)
	require.NoError(t, err)
	require.DeepSSZEqual(t, []*prysmv2.SyncCommitteeMessage{
		{Slot: 4, ValidatorIndex: 0, Signature: []byte{'g'}},
		{Slot: 4, ValidatorIndex: 1, Signature: []byte{'h'}},
	}, msgs)

	msgs, err = store.SyncCommitteeMessages(5)
	require.NoError(t, err)
	require.DeepSSZEqual(t, []*prysmv2.SyncCommitteeMessage{
		{Slot: 5, ValidatorIndex: 0, Signature: []byte{'i'}},
		{Slot: 5, ValidatorIndex: 1, Signature: []byte{'j'}},
	}, msgs)

	msgs, err = store.SyncCommitteeMessages(6)
	require.NoError(t, err)
	require.DeepSSZEqual(t, []*prysmv2.SyncCommitteeMessage{
		{Slot: 6, ValidatorIndex: 0, Signature: []byte{'k'}},
		{Slot: 6, ValidatorIndex: 1, Signature: []byte{'l'}},
	}, msgs)

	// Messages should persist after retrieval.
	msgs, err = store.SyncCommitteeMessages(1)
	require.NoError(t, err)
	require.DeepSSZEqual(t, []*prysmv2.SyncCommitteeMessage(nil), msgs)

	msgs, err = store.SyncCommitteeMessages(2)
	require.NoError(t, err)
	require.DeepSSZEqual(t, []*prysmv2.SyncCommitteeMessage(nil), msgs)

	msgs, err = store.SyncCommitteeMessages(3)
	require.NoError(t, err)
	require.DeepSSZEqual(t, []*prysmv2.SyncCommitteeMessage{
		{Slot: 3, ValidatorIndex: 0, Signature: []byte{'e'}},
		{Slot: 3, ValidatorIndex: 1, Signature: []byte{'f'}},
	}, msgs)

	msgs, err = store.SyncCommitteeMessages(4)
	require.NoError(t, err)
	require.DeepSSZEqual(t, []*prysmv2.SyncCommitteeMessage{
		{Slot: 4, ValidatorIndex: 0, Signature: []byte{'g'}},
		{Slot: 4, ValidatorIndex: 1, Signature: []byte{'h'}},
	}, msgs)

	msgs, err = store.SyncCommitteeMessages(5)
	require.NoError(t, err)
	require.DeepSSZEqual(t, []*prysmv2.SyncCommitteeMessage{
		{Slot: 5, ValidatorIndex: 0, Signature: []byte{'i'}},
		{Slot: 5, ValidatorIndex: 1, Signature: []byte{'j'}},
	}, msgs)

	msgs, err = store.SyncCommitteeMessages(6)
	require.NoError(t, err)
	require.DeepSSZEqual(t, []*prysmv2.SyncCommitteeMessage{
		{Slot: 6, ValidatorIndex: 0, Signature: []byte{'k'}},
		{Slot: 6, ValidatorIndex: 1, Signature: []byte{'l'}},
	}, msgs)
}
