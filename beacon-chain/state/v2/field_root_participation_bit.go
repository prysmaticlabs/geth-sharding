package v2

import (
	"encoding/binary"

	"github.com/pkg/errors"
	"github.com/prysmaticlabs/prysm/shared/hashutil"
	"github.com/prysmaticlabs/prysm/shared/htrutils"
	"github.com/prysmaticlabs/prysm/shared/params"
)

// participationBitsRoot computes the HashTreeRoot merkleization of
// participation roots.
func participationBitsRoot(bits []byte) ([32]byte, error) {
	hasher := hashutil.CustomSHA256Hasher()
	chunkedRoots, err := packParticipationBits(bits)
	if err != nil {
		return [32]byte{}, err
	}

	limit := (params.BeaconConfig().ValidatorRegistryLimit + 31) / 32
	if limit == 0 {
		if len(bits) == 0 {
			limit = 1
		} else {
			limit = uint64(len(bits))
		}
	}

	bytesRoot, err := htrutils.BitwiseMerkleize(hasher, chunkedRoots, uint64(len(chunkedRoots)), limit)
	if err != nil {
		return [32]byte{}, errors.Wrap(err, "could not compute merkleization")
	}

	bytesRootBufRoot := make([]byte, 32)
	binary.LittleEndian.PutUint64(bytesRootBufRoot[:8], uint64(len(bits)))
	return htrutils.MixInLength(bytesRoot, bytesRootBufRoot), nil
}

// packParticipationBits into chunks. It'll pad the last chunk with zero bytes if
// it does not have length bytes per chunk.
func packParticipationBits(bytes []byte) ([][]byte, error) {
	numItems := len(bytes)
	var chunks [][]byte
	for i := 0; i < numItems; i += 32 {
		j := i + 32
		// We create our upper bound index of the chunk, if it is greater than numItems,
		// we set it as numItems itself.
		if j > numItems {
			j = numItems
		}
		// We create chunks from the list of items based on the
		// indices determined above.
		chunks = append(chunks, bytes[i:j])
	}

	if len(chunks) == 0 {
		return chunks, nil
	}

	// Right-pad the last chunk with zero bytes if it does not
	// have length bytes.
	lastChunk := chunks[len(chunks)-1]
	for len(lastChunk) < 32 {
		lastChunk = append(lastChunk, 0)
	}
	chunks[len(chunks)-1] = lastChunk
	return chunks, nil
}
