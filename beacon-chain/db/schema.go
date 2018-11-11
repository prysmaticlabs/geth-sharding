package db

import (
	"github.com/prysmaticlabs/prysm/shared/bytes"
)

// The Schema will define how to store and retrieve data from the db.
// Currently we store blocks by prefixing `block` to their hash and
// using that as the key to store blocks.
// `block` + hash -> block
//
// We store the crystallized state using the crystallized state lookup key, and
// also the genesis block using the genesis lookup key.
// The canonical head is stored using the canonical head lookup key.

// The fields below define the suffix of keys in the db.
var (
	attestationBucket = []byte("attestation-bucket")
	blockBucket       = []byte("block-bucket")
	mainChainBucket   = []byte("main-chain-bucket")
	chainInfoBucket   = []byte("chain-info")

	mainChainHeightKey = []byte("chain-height")
	aStateLookupKey    = []byte("active-state")
	cStateLookupKey    = []byte("crystallized-state")
)

// encodeSlotNumber encodes a slot number as big endian uint64.
func encodeSlotNumber(number uint64) []byte {
	return bytes.Bytes8(number)
}
