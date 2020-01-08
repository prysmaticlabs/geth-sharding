package cache

import (
	"reflect"
	"sort"
	"strconv"
	"testing"

	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/prysmaticlabs/prysm/shared/params"
)

func TestCommitteeKeyFn_OK(t *testing.T) {
	item := &Committees{
		CommitteeCount:  1,
		Seed:            [32]byte{'A'},
		ShuffledIndices: []uint64{1, 2, 3, 4, 5},
	}

	k, err := committeeKeyFn(item)
	if err != nil {
		t.Fatal(err)
	}
	if k != key(item.Seed) {
		t.Errorf("Incorrect hash k: %s, expected %s", k, key(item.Seed))
	}
}

func TestCommitteeKeyFn_InvalidObj(t *testing.T) {
	_, err := committeeKeyFn("bad")
	if err != ErrNotCommittee {
		t.Errorf("Expected error %v, got %v", ErrNotCommittee, err)
	}
}

func TestCommitteeCache_CommitteesByEpoch(t *testing.T) {
	cache := NewCommitteesCache()

	item := &Committees{
		ShuffledIndices: []uint64{1, 2, 3, 4, 5, 6},
		Seed:            [32]byte{'A'},
		CommitteeCount:  3,
	}

	slot := params.BeaconConfig().SlotsPerEpoch
	committeeIndex := uint64(1)
	indices, err := cache.Committee(slot, item.Seed, committeeIndex)
	if err != nil {
		t.Fatal(err)
	}
	if indices != nil {
		t.Error("Expected committee not to exist in empty cache")
	}

	if err := cache.AddCommitteeShuffledList(item); err != nil {
		t.Fatal(err)
	}
	wantedIndex := uint64(0)
	indices, err = cache.Committee(slot, item.Seed, wantedIndex)
	if err != nil {
		t.Fatal(err)
	}

	start, end := startEndIndices(item, wantedIndex)
	if !reflect.DeepEqual(indices, item.ShuffledIndices[start:end]) {
		t.Errorf(
			"Expected fetched active indices to be %v, got %v",
			indices,
			item.ShuffledIndices[start:end],
		)
	}
}

func TestCommitteeCache_ActiveIndices(t *testing.T) {
	cache := NewCommitteesCache()

	item := &Committees{Seed: [32]byte{'A'}, SortedIndices: []uint64{1, 2, 3, 4, 5, 6}}
	indices, err := cache.ActiveIndices(item.Seed)
	if err != nil {
		t.Fatal(err)
	}
	if indices != nil {
		t.Error("Expected committee count not to exist in empty cache")
	}

	if err := cache.AddCommitteeShuffledList(item); err != nil {
		t.Fatal(err)
	}

	indices, err = cache.ActiveIndices(item.Seed)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(indices, item.SortedIndices) {
		t.Error("Did not receive correct active indices from cache")
	}
}

func TestCommitteeCache_CanRotate(t *testing.T) {
	cache := NewCommitteesCache()

	// Should rotate out all the epochs except 190 through 199.
	for i := 100; i < 200; i++ {
		s := []byte(strconv.Itoa(i))
		item := &Committees{Seed: bytesutil.ToBytes32(s)}
		if err := cache.AddCommitteeShuffledList(item); err != nil {
			t.Fatal(err)
		}
	}

	k := cache.CommitteeCache.ListKeys()
	if len(k) != maxCommitteesCacheSize {
		t.Errorf("wanted: %d, got: %d", maxCommitteesCacheSize, len(k))
	}

	sort.Slice(k, func(i, j int) bool {
		return k[i] < k[j]
	})
	s := bytesutil.ToBytes32([]byte(strconv.Itoa(190)))
	if k[0] != key(s) {
		t.Error("incorrect key received for slot 190")
	}
	s = bytesutil.ToBytes32([]byte(strconv.Itoa(199)))
	if k[len(k)-1] != key(s) {
		t.Error("incorrect key received for slot 199")
	}
}
