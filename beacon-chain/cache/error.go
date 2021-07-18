package cache

import "errors"

// Sync committee cache related errors

// ErrNonExistingSyncCommitteeKey when sync committee key (root) does not exist in cache.
var ErrNonExistingSyncCommitteeKey = errors.New("does not exist sync committee key")
var errNotSyncCommitteeIndexPosition = errors.New("not syncCommitteeIndexPosition struct")
