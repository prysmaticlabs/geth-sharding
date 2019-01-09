package hashutil

import (
	"golang.org/x/crypto/sha3"
)

// Hash defines a function that returns the
// Keccak-256/SHA3 hash of the data passed in.
// https://github.com/ethereum/eth2.0-specs/blob/master/specs/core/0_beacon-chain.md#appendix
func Hash(data []byte) [32]byte {
	var hash [32]byte

	h := sha3.NewLegacyKeccak256()

	// The hash interface never returns an error, for that reason
	// we are not handling the error below. For reference, it is
	// stated here https://golang.org/pkg/hash/#Hash

	// #nosec G104
	h.Write(data)
	h.Sum(hash[:0])

	return hash
}

// RepeatHash applies the Keccak-256/SHA3 hash function repeatedly
// numTimes on a [32]byte array.
func RepeatHash(data [32]byte, numTimes uint64) [32]byte {
	if numTimes == 0 {
		return data
	}
	return RepeatHash(Hash(data[:]), numTimes-1)
}
