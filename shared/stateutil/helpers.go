package stateutil

import (
	"bytes"
	"encoding/binary"

	"github.com/minio/sha256-simd"
	"github.com/pkg/errors"
	"github.com/protolambda/zssz/htr"
	"github.com/protolambda/zssz/merkle"
	"github.com/prysmaticlabs/go-bitfield"
	"github.com/prysmaticlabs/prysm/shared/hashutil"
)

func bitlistRoot(bfield bitfield.Bitfield, maxCapacity uint64) ([32]byte, error) {
	limit := (maxCapacity + 255) / 256
	if bfield == nil || bfield.Len() == 0 {
		length := make([]byte, 32)
		root, err := bitwiseMerkleize([][]byte{}, 0, limit)
		if err != nil {
			return [32]byte{}, err
		}
		return mixInLength(root, length), nil
	}
	chunks, err := pack([][]byte{bfield.Bytes()})
	if err != nil {
		return [32]byte{}, err
	}
	buf := new(bytes.Buffer)
	if err := binary.Write(buf, binary.LittleEndian, bfield.Len()); err != nil {
		return [32]byte{}, err
	}
	output := make([]byte, 32)
	copy(output, buf.Bytes())
	root, err := bitwiseMerkleize(chunks, uint64(len(chunks)), limit)
	if err != nil {
		return [32]byte{}, err
	}
	return mixInLength(root, output), nil
}

// Given ordered BYTES_PER_CHUNK-byte chunks, if necessary utilize zero chunks so that the
// number of chunks is a power of two, Merkleize the chunks, and return the root.
// Note that merkleize on a single chunk is simply that chunk, i.e. the identity
// when the number of chunks is one.
func bitwiseMerkleize(chunks [][]byte, count uint64, limit uint64) ([32]byte, error) {
	if count > limit {
		return [32]byte{}, errors.New("merkleizing list that is too large, over limit")
	}
	hasher := htr.HashFn(hashutil.Hash)
	leafIndexer := func(i uint64) []byte {
		return chunks[i]
	}
	return merkle.Merkleize(hasher, count, limit, leafIndexer), nil
}

func pack(serializedItems [][]byte) ([][]byte, error) {
	areAllEmpty := true
	for _, item := range serializedItems {
		if !bytes.Equal(item, []byte{}) {
			areAllEmpty = false
			break
		}
	}
	// If there are no items, we return an empty chunk.
	if len(serializedItems) == 0 || areAllEmpty {
		emptyChunk := make([]byte, bytesPerChunk)
		return [][]byte{emptyChunk}, nil
	} else if len(serializedItems[0]) == bytesPerChunk {
		// If each item has exactly BYTES_PER_CHUNK length, we return the list of serialized items.
		return serializedItems, nil
	}
	// We flatten the list in order to pack its items into byte chunks correctly.
	orderedItems := []byte{}
	for _, item := range serializedItems {
		orderedItems = append(orderedItems, item...)
	}
	numItems := len(orderedItems)
	chunks := [][]byte{}
	for i := 0; i < numItems; i += bytesPerChunk {
		j := i + bytesPerChunk
		// We create our upper bound index of the chunk, if it is greater than numItems,
		// we set it as numItems itself.
		if j > numItems {
			j = numItems
		}
		// We create chunks from the list of items based on the
		// indices determined above.
		chunks = append(chunks, orderedItems[i:j])
	}
	// Right-pad the last chunk with zero bytes if it does not
	// have length bytesPerChunk.
	lastChunk := chunks[len(chunks)-1]
	for len(lastChunk) < bytesPerChunk {
		lastChunk = append(lastChunk, 0)
	}
	chunks[len(chunks)-1] = lastChunk
	return chunks, nil
}

func mixInLength(root [32]byte, length []byte) [32]byte {
	var hash [32]byte
	h := sha256.New()
	h.Write(root[:])
	h.Write(length)
	// The hash interface never returns an error, for that reason
	// we are not handling the error below. For reference, it is
	// stated here https://golang.org/pkg/hash/#Hash
	// #nosec G104
	h.Sum(hash[:0])
	return hash
}
