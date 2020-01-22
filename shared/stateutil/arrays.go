package stateutil

import (
	"bytes"
	"errors"
	"sync"

	"github.com/protolambda/zssz/merkle"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/prysmaticlabs/prysm/shared/featureconfig"
	"github.com/prysmaticlabs/prysm/shared/hashutil"
)

var (
	leavesCache = make(map[string][][]byte)
	layersCache = make(map[string][][][]byte)
	lock        sync.RWMutex
)

// RootsArrayHashTreeRoot computes the Merkle root of arrays of 32-byte hashes, such as [64][32]byte
// according to the Simple Serialize specification of eth2.
func RootsArrayHashTreeRoot(vals [][]byte, length uint64, fieldName string) ([32]byte, error) {
	if featureconfig.Get().EnableSSZCache {
		return cachedHasher.arraysRoot(vals, length, fieldName)
	}
	return nocachedHasher.arraysRoot(vals, length, fieldName)
}

func (h *stateRootHasher) arraysRoot(input [][]byte, length uint64, fieldName string) ([32]byte, error) {
	lock.Lock()
	if _, ok := layersCache[fieldName]; !ok && h.rootsCache != nil {
		depth := merkle.GetDepth(length)
		layersCache[fieldName] = make([][][]byte, depth+1)
	}
	lock.Unlock()

	leaves := make([][]byte, length)
	copy(leaves, input)
	bytesProcessed := 0
	changedIndices := make([]int, 0)
	lock.RLock()
	prevLeaves, ok := leavesCache[fieldName]
	lock.RUnlock()
	if len(prevLeaves) == 0 || h.rootsCache == nil {
		prevLeaves = leaves
	}

	for i := 0; i < len(leaves); i++ {
		padded := bytesutil.ToBytes32(leaves[i])
		leaves[i] = padded[:]
		// We check if any items changed since the roots were last recomputed.
		notEqual := !bytes.Equal(leaves[i], prevLeaves[i])
		if ok && h.rootsCache != nil && notEqual {
			changedIndices = append(changedIndices, i)
		}
		bytesProcessed += 32
	}
	if len(changedIndices) > 0 && h.rootsCache != nil {
		var rt [32]byte
		var err error
		// If indices did change since last computation, we only recompute
		// the modified branches in the cached Merkle tree for this state field.
		chunks := leaves

		// We need to ensure we recompute indices of the Merkle tree which
		// changed in-between calls to this function. This check adds an offset
		// to the recomputed indices to ensure we do so evenly.
		maxChangedIndex := changedIndices[len(changedIndices)-1]
		if maxChangedIndex+2 == len(chunks) && maxChangedIndex%2 != 0 {
			changedIndices = append(changedIndices, maxChangedIndex+1)
		}
		for i := 0; i < len(changedIndices); i++ {
			rt, err = recomputeRoot(changedIndices[i], chunks, length, fieldName)
			if err != nil {
				return [32]byte{}, err
			}
		}
		lock.Lock()
		leavesCache[fieldName] = chunks
		lock.Unlock()
		return rt, nil
	}

	var res [32]byte
	res = h.merkleizeWithCache(leaves, length, fieldName)
	if h.rootsCache != nil {
		lock.Lock()
		leavesCache[fieldName] = leaves
		lock.Unlock()
	}
	return res, nil
}

func (h *stateRootHasher) merkleizeWithCache(leaves [][]byte, length uint64, fieldName string) [32]byte {
	lock.Lock()
	defer lock.Unlock()
	if len(leaves) == 1 {
		var root [32]byte
		copy(root[:], leaves[0])
		return root
	}
	hashLayer := leaves
	layers := make([][][]byte, merkle.GetDepth(length)+1)
	if items, ok := layersCache[fieldName]; ok && h.rootsCache != nil {
		if len(items[0]) == len(leaves) {
			layers = items
		}
	}
	layers[0] = hashLayer

	// We keep track of the hash layers of a Merkle trie until we reach
	// the top layer of length 1, which contains the single root element.
	//        [Root]      -> Top layer has length 1.
	//    [E]       [F]   -> This layer has length 2.
	// [A]  [B]  [C]  [D] -> The bottom layer has length 4 (needs to be a power of two).
	i := 1
	for len(hashLayer) > 1 && i < len(layers) {
		layer := make([][]byte, 0)
		for i := 0; i < len(hashLayer); i += 2 {
			hashedChunk := hashutil.Hash(append(hashLayer[i], hashLayer[i+1]...))
			layer = append(layer, hashedChunk[:])
		}
		hashLayer = layer
		layers[i] = hashLayer
		i++
	}
	var root [32]byte
	copy(root[:], hashLayer[0])
	if h.rootsCache != nil {
		layersCache[fieldName] = layers
	}
	return root
}

func recomputeRoot(idx int, chunks [][]byte, length uint64, fieldName string) ([32]byte, error) {
	lock.Lock()
	defer lock.Unlock()
	items, ok := layersCache[fieldName]
	if !ok {
		return [32]byte{}, errors.New("could not recompute root as there was no cache found")
	}
	if items == nil {
		return [32]byte{}, errors.New("could not recompute root as there were no items found in the layers cache")
	}
	layers := items
	root := chunks[idx]
	layers[0] = chunks
	// The merkle tree structure looks as follows:
	// [[r1, r2, r3, r4], [parent1, parent2], [root]]
	// Using information about the index which changed, idx, we recompute
	// only its branch up the tree.
	currentIndex := idx
	for i := 0; i < len(layers)-1; i++ {
		isLeft := currentIndex%2 == 0
		neighborIdx := currentIndex ^ 1

		neighbor := make([]byte, 32)
		if layers[i] != nil && len(layers[i]) != 0 && neighborIdx < len(layers[i]) {
			neighbor = layers[i][neighborIdx]
		}
		if isLeft {
			parentHash := hashutil.Hash(append(root, neighbor...))
			root = parentHash[:]
		} else {
			parentHash := hashutil.Hash(append(neighbor, root...))
			root = parentHash[:]
		}
		parentIdx := currentIndex / 2
		// Update the cached layers at the parent index.
		if len(layers[i+1]) == 0 {
			layers[i+1] = append(layers[i+1], root)
		} else {
			layers[i+1][parentIdx] = root
		}
		currentIndex = parentIdx
	}
	layersCache[fieldName] = layers
	// If there is only a single leaf, we return it (the identity element).
	if len(layers[0]) == 1 {
		return bytesutil.ToBytes32(layers[0][0]), nil
	}
	return bytesutil.ToBytes32(root), nil
}
