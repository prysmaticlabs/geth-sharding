package cache

import (
	"errors"

	types "github.com/prysmaticlabs/eth2-types"
)

// ErrNotCommittee will be returned when a cache object is not a pointer to
// a Committee struct.
var ErrNotCommittee = errors.New("object is not a committee struct")

// ErrNonCommitteeKey will be returned when the committee key does not exist in cache.
var ErrNonCommitteeKey = errors.New("committee key does not exist")

// Committees defines the shuffled committees seed.
type Committees struct {
	CommitteeCount  uint64
	Seed            [32]byte
	ShuffledIndices []types.ValidatorIndex
	SortedIndices   []types.ValidatorIndex
	ActiveBalance   *Balance
}

// Balance tracks active balance.
// Given default uint64 is 0, `Exist` is used to distinguish whether
// this field has been filed.
type Balance struct {
	Exist bool
	Total uint64
}
