// Package p2putils contains useful helpers for Ethereum consensus fork-related functionality.
package p2putils

import (
	"math"
	"sort"
	"time"

	"github.com/pkg/errors"
	types "github.com/prysmaticlabs/eth2-types"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/prysmaticlabs/prysm/shared/params"
)

// IsForkNextEpoch checks if an alloted fork is in the following epoch.
func IsForkNextEpoch(genesisTime time.Time, genesisValidatorsRoot []byte) (bool, error) {
	if genesisTime.IsZero() {
		return false, errors.New("genesis time is not set")
	}
	if len(genesisValidatorsRoot) == 0 {
		return false, errors.New("genesis validators root is not set")
	}
	currentSlot := helpers.SlotsSince(genesisTime)
	currentEpoch := helpers.SlotToEpoch(currentSlot)
	fSchedule := params.BeaconConfig().ForkVersionSchedule
	scheduledForks := SortedForkVersions(fSchedule)
	isForkEpoch := false
	for _, forkVersion := range scheduledForks {
		epoch := fSchedule[forkVersion]
		if currentEpoch+1 == epoch {
			isForkEpoch = true
			break
		}
	}
	return isForkEpoch, nil
}

// ForkDigestFromEpoch retrieves the fork digest from the current schedule determined
// by the provided epoch.
func ForkDigestFromEpoch(currentEpoch types.Epoch, genesisValidatorsRoot []byte) ([4]byte, error) {
	if len(genesisValidatorsRoot) == 0 {
		return [4]byte{}, errors.New("genesis validators root is not set")
	}
	forkData, err := Fork(currentEpoch)
	if err != nil {
		return [4]byte{}, err
	}
	return helpers.ComputeForkDigest(forkData.CurrentVersion, genesisValidatorsRoot)
}

// CreateForkDigest creates a fork digest from a genesis time and genesis
// validators root, utilizing the current slot to determine
// the active fork version in the node.
func CreateForkDigest(
	genesisTime time.Time,
	genesisValidatorsRoot []byte,
) ([4]byte, error) {
	if genesisTime.IsZero() {
		return [4]byte{}, errors.New("genesis time is not set")
	}
	if len(genesisValidatorsRoot) == 0 {
		return [4]byte{}, errors.New("genesis validators root is not set")
	}
	currentSlot := helpers.SlotsSince(genesisTime)
	currentEpoch := helpers.SlotToEpoch(currentSlot)

	forkData, err := Fork(currentEpoch)
	if err != nil {
		return [4]byte{}, err
	}

	digest, err := helpers.ComputeForkDigest(forkData.CurrentVersion, genesisValidatorsRoot)
	if err != nil {
		return [4]byte{}, err
	}
	return digest, nil
}

// Fork given a target epoch,
// returns the active fork version during this epoch.
func Fork(
	targetEpoch types.Epoch,
) (*pb.Fork, error) {
	currentForkVersion := bytesutil.ToBytes4(params.BeaconConfig().GenesisForkVersion)
	previousForkVersion := bytesutil.ToBytes4(params.BeaconConfig().GenesisForkVersion)
	fSchedule := params.BeaconConfig().ForkVersionSchedule
	sortedForkVersions := SortedForkVersions(fSchedule)
	forkEpoch := types.Epoch(0)
	for _, forkVersion := range sortedForkVersions {
		epoch := fSchedule[forkVersion]
		if targetEpoch >= epoch {
			previousForkVersion = currentForkVersion
			currentForkVersion = forkVersion
			forkEpoch = epoch
		}
	}
	return &pb.Fork{
		PreviousVersion: previousForkVersion[:],
		CurrentVersion:  currentForkVersion[:],
		Epoch:           forkEpoch,
	}, nil
}

// RetrieveForkDataFromDigest performs the inverse, where it tries to determine the fork version
// and epoch from a provided digest by looping through our current fork schedule.
func RetrieveForkDataFromDigest(digest [4]byte, genesisValidatorsRoot []byte) ([4]byte, types.Epoch, error) {
	fSchedule := params.BeaconConfig().ForkVersionSchedule
	for v, e := range fSchedule {
		rDigest, err := helpers.ComputeForkDigest(v[:], genesisValidatorsRoot)
		if err != nil {
			return [4]byte{}, 0, err
		}
		if rDigest == digest {
			return v, e, nil
		}
	}
	return [4]byte{}, 0, errors.Errorf("no fork exists for a digest of %#x", digest)
}

// NextForkData retrieves the next fork data according to the
// provided current epoch.
func NextForkData(currEpoch types.Epoch) ([4]byte, types.Epoch) {
	fSchedule := params.BeaconConfig().ForkVersionSchedule
	sortedForkVersions := SortedForkVersions(fSchedule)
	nextForkEpoch := types.Epoch(math.MaxUint64)
	nextForkVersion := [4]byte{}
	for _, forkVersion := range sortedForkVersions {
		epoch := fSchedule[forkVersion]
		// If we get an epoch larger than out current epoch
		// we set this as our next fork epoch and exit the
		// loop.
		if epoch > currEpoch {
			nextForkEpoch = epoch
			nextForkVersion = forkVersion
			break
		}
		// In the event the retrieved epoch is less than
		// our current epoch, we mark the previous
		// fork's version as the next fork version.
		if epoch <= currEpoch {
			// The next fork version is updated to
			// always include the most current fork version.
			nextForkVersion = forkVersion
		}
	}
	return nextForkVersion, nextForkEpoch
}

// SortedForkVersions sorts the provided fork schedule in ascending order
// by epoch.
func SortedForkVersions(forkSchedule map[[4]byte]types.Epoch) [][4]byte {
	sortedVersions := make([][4]byte, len(forkSchedule))
	i := 0
	for k := range forkSchedule {
		sortedVersions[i] = k
		i++
	}
	sort.Slice(sortedVersions, func(a, b int) bool {
		return forkSchedule[sortedVersions[a]] < forkSchedule[sortedVersions[b]]
	})
	return sortedVersions
}
