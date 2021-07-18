// +build libfuzzer

// This file is used in fuzzer builds to bypass global committee caches.
package cache

import types "github.com/prysmaticlabs/eth2-types"

// FakeCommitteeCache is a struct with 1 queue for looking up shuffled indices list by seed.
type FakeCommitteeCache struct {
}

// NewCommitteesCache creates a new committee cache for storing/accessing shuffled indices of a committee.
func NewCommitteesCache() *FakeCommitteeCache {
	return &FakeCommitteeCache{}
}

// Committee fetches the shuffled indices by slot and committee index. Every list of indices
// represent one committee. Returns true if the list exists with slot and committee index. Otherwise returns false, nil.
func (c *FakeCommitteeCache) Committee(slot types.Slot, seed [32]byte, index types.CommitteeIndex) ([]types.ValidatorIndex, error) {
	return nil, nil
}

// AddCommitteeShuffledList adds Committee shuffled list object to the cache. T
// his method also trims the least recently list if the cache size has ready the max cache size limit.
func (c *FakeCommitteeCache) AddCommitteeShuffledList(committees *Committees) error {
	return nil
}

// AddProposerIndicesList updates the committee shuffled list with proposer indices.
func (c *FakeCommitteeCache) AddProposerIndicesList(seed [32]byte, indices []types.ValidatorIndex) error {
	return nil
}

// ActiveIndices returns the active indices of a given seed stored in cache.
func (c *FakeCommitteeCache) ActiveIndices(seed [32]byte) ([]types.ValidatorIndex, error) {
	return nil, nil
}

// ActiveIndicesCount returns the active indices count of a given seed stored in cache.
func (c *FakeCommitteeCache) ActiveIndicesCount(seed [32]byte) (int, error) {
	return 0, nil
}

// ActiveBalance returns the active balance of a given seed stored in cache.
func (c *FakeCommitteeCache) ActiveBalance(seed [32]byte) (uint64, error) {
	return 0, nil
}

// ProposerIndices returns the proposer indices of a given seed.
func (c *FakeCommitteeCache) ProposerIndices(seed [32]byte) ([]types.ValidatorIndex, error) {
	return nil, nil
}

// HasEntry returns true if the committee cache has a value.
func (c *FakeCommitteeCache) HasEntry(string) bool {
	return false
}
