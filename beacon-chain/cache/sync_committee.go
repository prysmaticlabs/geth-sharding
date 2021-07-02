// +build !libfuzzer

package cache

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	types "github.com/prysmaticlabs/eth2-types"
	iface "github.com/prysmaticlabs/prysm/beacon-chain/state/interface"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"k8s.io/client-go/tools/cache"
)

var (
	maxSyncCommitteeSize = uint64(3) // Allows 3 forks to happen around `EPOCHS_PER_SYNC_COMMITTEE_PERIOD` boundary.

	// SyncCommitteeCacheMiss tracks the number of committee requests that aren't present in the cache.
	SyncCommitteeCacheMiss = promauto.NewCounter(prometheus.CounterOpts{
		Name: "sync_committee_index_cache_miss",
		Help: "The number of committee requests that aren't present in the sync committee index cache.",
	})
	// SyncCommitteeCacheHit tracks the number of committee requests that are in the cache.
	SyncCommitteeCacheHit = promauto.NewCounter(prometheus.CounterOpts{
		Name: "sync_committee_index_cache_hit",
		Help: "The number of committee requests that are present in the sync committee index cache.",
	})
)

// SyncCommitteeCache utilizes a FIFO cache to sufficiently cache validator position within sync committee.
// It is thread safe with concurrent read write.
type SyncCommitteeCache struct {
	cache *cache.FIFO
	lock  sync.RWMutex
}

// Index position of all validators in sync committee where `currentSyncCommitteeRoot` is the
// key and `vIndexToPositionMap` is value. Inside `vIndexToPositionMap`, validator positions
// are cached where key is the validator index and the value is the `positionInCommittee` struct.
type syncCommitteeIndexPosition struct {
	currentSyncCommitteeRoot [32]byte
	vIndexToPositionMap      map[types.ValidatorIndex]*positionInCommittee
}

// Index position of individual validator of current epoch and previous epoch sync committee.
type positionInCommittee struct {
	currentEpoch []uint64
	nextEpoch    []uint64
}

// NewSyncCommittee initializes and returns a new SyncCommitteeCache.
func NewSyncCommittee() *SyncCommitteeCache {
	return &SyncCommitteeCache{
		cache: cache.NewFIFO(keyFn),
	}
}

// CurrentEpochIndexPosition returns current epoch index position of a validator index with respect with
// sync committee. If the input validator index has no assignment, an empty list will be returned.
// If the input root does not exist in cache, ErrNonExistingSyncCommitteeKey is returned.
// Then performing manual checking of state for index position in state is recommended.
func (s *SyncCommitteeCache) CurrentEpochIndexPosition(root [32]byte, valIdx types.ValidatorIndex) ([]uint64, error) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	pos, err := s.idxPositionInCommittee(root, valIdx)
	if err != nil {
		return nil, err
	}
	if pos == nil {
		return []uint64{}, nil
	}

	return pos.currentEpoch, nil
}

// NextEpochIndexPosition returns next epoch index position of a validator index in respect with sync committee.
// If the input validator index has no assignment, an empty list will be returned.
// If the input root does not exist in cache, ErrNonExistingSyncCommitteeKey is returned.
// Then performing manual checking of state for index position in state is recommended.
func (s *SyncCommitteeCache) NextEpochIndexPosition(root [32]byte, valIdx types.ValidatorIndex) ([]uint64, error) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	pos, err := s.idxPositionInCommittee(root, valIdx)
	if err != nil {
		return nil, err
	}
	if pos == nil {
		return []uint64{}, nil
	}
	return pos.nextEpoch, nil
}

// Helper function for `CurrentEpochIndexPosition` and `NextEpochIndexPosition` to return a mapping
// of validator index to its index(s) position in the sync committee.
func (s *SyncCommitteeCache) idxPositionInCommittee(
	root [32]byte, valIdx types.ValidatorIndex,
) (*positionInCommittee, error) {
	obj, exists, err := s.cache.GetByKey(key(root))
	if err != nil {
		return nil, err
	}
	if !exists {
		SyncCommitteeCacheMiss.Inc()
		return nil, ErrNonExistingSyncCommitteeKey
	}
	item, ok := obj.(*syncCommitteeIndexPosition)
	if !ok {
		return nil, errNotSyncCommitteeIndexPosition
	}
	idxInCommittee, ok := item.vIndexToPositionMap[valIdx]
	if !ok {
		SyncCommitteeCacheMiss.Inc()
		return nil, nil
	}
	SyncCommitteeCacheHit.Inc()
	return idxInCommittee, nil
}

// UpdatePositionsInCommittee updates caching of validators position in sync committee in respect to
// current epoch and next epoch. This should be called when `current_sync_committee` and `next_sync_committee`
// change and that happens every `EPOCHS_PER_SYNC_COMMITTEE_PERIOD`.
func (s *SyncCommitteeCache) UpdatePositionsInCommittee(syncCommitteeBoundaryRoot [32]byte, state iface.BeaconStateAltair) error {
	csc, err := state.CurrentSyncCommittee()
	if err != nil {
		return err
	}
	positionsMap := make(map[types.ValidatorIndex]*positionInCommittee)
	for i, pubkey := range csc.Pubkeys {
		p := bytesutil.ToBytes48(pubkey)
		validatorIndex, ok := state.ValidatorIndexByPubkey(p)
		if !ok {
			continue
		}
		if _, ok := positionsMap[validatorIndex]; !ok {
			m := &positionInCommittee{currentEpoch: []uint64{uint64(i)}, nextEpoch: []uint64{}}
			positionsMap[validatorIndex] = m
		} else {
			positionsMap[validatorIndex].currentEpoch = append(positionsMap[validatorIndex].currentEpoch, uint64(i))
		}
	}

	nsc, err := state.NextSyncCommittee()
	if err != nil {
		return err
	}
	for i, pubkey := range nsc.Pubkeys {
		p := bytesutil.ToBytes48(pubkey)
		validatorIndex, ok := state.ValidatorIndexByPubkey(p)
		if !ok {
			continue
		}
		if _, ok := positionsMap[validatorIndex]; !ok {
			m := &positionInCommittee{nextEpoch: []uint64{uint64(i)}, currentEpoch: []uint64{}}
			positionsMap[validatorIndex] = m
		} else {
			positionsMap[validatorIndex].nextEpoch = append(positionsMap[validatorIndex].nextEpoch, uint64(i))
		}
	}

	s.lock.Lock()
	defer s.lock.Unlock()

	if err := s.cache.Add(&syncCommitteeIndexPosition{
		currentSyncCommitteeRoot: syncCommitteeBoundaryRoot,
		vIndexToPositionMap:      positionsMap,
	}); err != nil {
		return err
	}
	trim(s.cache, maxSyncCommitteeSize)

	return nil
}

// Given the `syncCommitteeIndexPosition` object, this returns the key of the object.
// The key is the `currentSyncCommitteeRoot` within the field.
// Error gets returned if input does not comply with `currentSyncCommitteeRoot` object.
func keyFn(obj interface{}) (string, error) {
	info, ok := obj.(*syncCommitteeIndexPosition)
	if !ok {
		return "", errNotSyncCommitteeIndexPosition
	}

	return string(info.currentSyncCommitteeRoot[:]), nil
}
