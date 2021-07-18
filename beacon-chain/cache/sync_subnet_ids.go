package cache

import (
	"sync"
	"time"

	"github.com/patrickmn/go-cache"
	types "github.com/prysmaticlabs/eth2-types"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/rand"
	"github.com/prysmaticlabs/prysm/shared/sliceutil"
)

type syncSubnetIDs struct {
	sCommittee    *cache.Cache
	sCommiteeLock sync.RWMutex
}

// SyncSubnetIDs for sync committee participant.
var SyncSubnetIDs = newSyncSubnetIDs()

func newSyncSubnetIDs() *syncSubnetIDs {
	epochDuration := time.Duration(params.BeaconConfig().SlotsPerEpoch.Mul(params.BeaconConfig().SecondsPerSlot))
	subLength := epochDuration * time.Duration(params.BeaconConfig().EpochsPerSyncCommitteePeriod)
	persistentCache := cache.New(subLength*time.Second, epochDuration*time.Second)
	return &syncSubnetIDs{sCommittee: persistentCache}
}

// GetSyncCommitteeSubnets retrieves the sync committee subnet and expiration time of that validator's
// subscription.
func (s *syncSubnetIDs) GetSyncCommitteeSubnets(pubkey []byte, epoch types.Epoch) ([]uint64, types.Epoch, bool, time.Time) {
	s.sCommiteeLock.RLock()
	defer s.sCommiteeLock.RUnlock()

	id, duration, ok := s.sCommittee.GetWithExpiration(keyBuilder(pubkey, epoch))
	if !ok {
		return []uint64{}, 0, ok, time.Time{}
	}
	// Retrieve the slot from the cache.
	idxs, ok := id.([]uint64)
	if !ok {
		return []uint64{}, 0, ok, time.Time{}
	}
	// If no committees are saved, we return
	// nothing.
	if len(idxs) <= 1 {
		return []uint64{}, 0, ok, time.Time{}
	}
	return idxs[1:], types.Epoch(idxs[0]), ok, duration
}

// GetAllSubnets retrieves all the non-expired subscribed subnets of all the validators
// in the cache. This method also takes the epoch as an argument to only retrieve
// assingments for epochs that have happened.
func (s *syncSubnetIDs) GetAllSubnets(currEpoch types.Epoch) []uint64 {
	s.sCommiteeLock.RLock()
	defer s.sCommiteeLock.RUnlock()

	itemsMap := s.sCommittee.Items()
	var committees []uint64

	for _, v := range itemsMap {
		if v.Expired() {
			continue
		}
		idxs, ok := v.Object.([]uint64)
		if !ok {
			continue
		}
		if len(idxs) <= 1 {
			continue
		}
		// Check if the subnet is valid in the current epoch.
		if types.Epoch(idxs[0]) > currEpoch {
			continue
		}
		// Ignore the first index as that represents the
		// epoch of the validator's assignments.
		committees = append(committees, idxs[1:]...)
	}
	return sliceutil.SetUint64(committees)
}

// AddSyncCommitteeSubnets adds the relevant committee for that particular validator along with its
// expiration period. An Epoch argument here denotes the epoch from which the sync committee subnets
// will be active.
func (s *syncSubnetIDs) AddSyncCommitteeSubnets(pubkey []byte, epoch types.Epoch, comIndex []uint64, duration time.Duration) {
	s.sCommiteeLock.Lock()
	defer s.sCommiteeLock.Unlock()
	subComCount := params.BeaconConfig().SyncCommitteeSubnetCount
	// To join a sync committee subnet, select a random number of epochs before the end of the
	// current sync committee period between 1 and SYNC_COMMITTEE_SUBNET_COUNT, inclusive.
	diff := (rand.NewGenerator().Uint64() % subComCount) + 1
	joinEpoch, err := epoch.SafeSub(diff)
	if err != nil {
		// If we overflow here, we simply set the value to join
		// at 0.
		joinEpoch = 0
	}
	// Append the epoch of the subnet into the first index here.
	s.sCommittee.Set(keyBuilder(pubkey, epoch), append([]uint64{uint64(joinEpoch)}, comIndex...), duration)
}

// EmptyAllCaches empties out all the related caches and flushes any stored
// entries on them. This should only ever be used for testing, in normal
// production, handling of the relevant subnets for each role is done
// separately.
func (s *syncSubnetIDs) EmptyAllCaches() {
	// Clear the cache.

	s.sCommiteeLock.Lock()
	s.sCommittee.Flush()
	s.sCommiteeLock.Unlock()
}

func keyBuilder(pubkey []byte, epoch types.Epoch) string {
	epochBytes := bytesutil.Bytes8(uint64(epoch))
	return string(append(pubkey, epochBytes...))
}
