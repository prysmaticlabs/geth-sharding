package v2

import (
	"bytes"
	"encoding/binary"

	"github.com/pkg/errors"
	"github.com/prysmaticlabs/prysm/beacon-chain/state/stateutil"
	ethpb "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/shared/featureconfig"
	"github.com/prysmaticlabs/prysm/shared/hashutil"
	"github.com/prysmaticlabs/prysm/shared/htrutils"
	"github.com/prysmaticlabs/prysm/shared/params"
)

func (h *stateRootHasher) validatorRegistryRoot(validators []*ethpb.Validator) ([32]byte, error) {
	hashKeyElements := make([]byte, len(validators)*32)
	roots := make([][32]byte, len(validators))
	emptyKey := hashutil.FastSum256(hashKeyElements)
	hasher := hashutil.CustomSHA256Hasher()
	bytesProcessed := 0
	for i := 0; i < len(validators); i++ {
		val, err := h.validatorRoot(hasher, validators[i])
		if err != nil {
			return [32]byte{}, errors.Wrap(err, "could not compute validators merkleization")
		}
		copy(hashKeyElements[bytesProcessed:bytesProcessed+32], val[:])
		roots[i] = val
		bytesProcessed += 32
	}

	hashKey := hashutil.FastSum256(hashKeyElements)
	if hashKey != emptyKey && h.rootsCache != nil {
		if found, ok := h.rootsCache.Get(string(hashKey[:])); found != nil && ok {
			return found.([32]byte), nil
		}
	}

	validatorsRootsRoot, err := htrutils.BitwiseMerkleizeArrays(hasher, roots, uint64(len(roots)), params.BeaconConfig().ValidatorRegistryLimit)
	if err != nil {
		return [32]byte{}, errors.Wrap(err, "could not compute validator registry merkleization")
	}
	validatorsRootsBuf := new(bytes.Buffer)
	if err := binary.Write(validatorsRootsBuf, binary.LittleEndian, uint64(len(validators))); err != nil {
		return [32]byte{}, errors.Wrap(err, "could not marshal validator registry length")
	}
	// We need to mix in the length of the slice.
	var validatorsRootsBufRoot [32]byte
	copy(validatorsRootsBufRoot[:], validatorsRootsBuf.Bytes())
	res := htrutils.MixInLength(validatorsRootsRoot, validatorsRootsBufRoot[:])
	if hashKey != emptyKey && h.rootsCache != nil {
		h.rootsCache.Set(string(hashKey[:]), res, 32)
	}
	return res, nil
}

func (h *stateRootHasher) validatorRoot(hasher htrutils.HashFn, validator *ethpb.Validator) ([32]byte, error) {
	if validator == nil {
		return [32]byte{}, errors.New("nil validator")
	}

	enc := stateutil.ValidatorEncKey(validator)
	// Check if it exists in cache:
	if h.rootsCache != nil {
		if found, ok := h.rootsCache.Get(string(enc)); found != nil && ok {
			return found.([32]byte), nil
		}
	}

	valRoot, err := stateutil.ValidatorRootWithHasher(hasher, validator)
	if err != nil {
		return [32]byte{}, err
	}

	if h.rootsCache != nil {
		h.rootsCache.Set(string(enc), valRoot, 32)
	}
	return valRoot, nil
}

// ValidatorRegistryRoot computes the HashTreeRoot Merkleization of
// a list of validator structs according to the eth2
// Simple Serialize specification.
func ValidatorRegistryRoot(vals []*ethpb.Validator) ([32]byte, error) {
	if featureconfig.Get().EnableSSZCache {
		return cachedHasher.validatorRegistryRoot(vals)
	}
	return nocachedHasher.validatorRegistryRoot(vals)
}
