package trieutil

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/prysmaticlabs/prysm/shared/hashutil"
)

var zeroHashes = make([][]byte, 100)

func init() {
	zeroHashes[0] = make([]byte, 32)
	for i := 1; i < 100; i++ {
		leaf := append(zeroHashes[i-1], zeroHashes[i-1]...)
		result := hashutil.Hash(leaf)
		zeroHashes[i] = result[:]
	}
}

// MerkleTrie implements a sparse, general purpose Merkle trie to be used
// across ETH2.0 Phase 0 functionality.
type MerkleTrie struct {
	depth         uint
	branches      [][][]byte
	originalItems [][]byte // list of provided items before hashing them into leaves.
}

// NewTrie returns a new merkle trie filled with zerohashes to use.
func NewTrie(depth int) (*MerkleTrie, error) {
	var zeroBytes [32]byte
	items := [][]byte{zeroBytes[:]}
	return GenerateTrieFromItems(items, depth)
}

// InsertIntoTrie inserts an item(deposit hash) into the trie.
func (m *MerkleTrie) InsertIntoTrie(item []byte, index int) error {
	// Only insert new items which follow directly after the last
	// added element
	if index > len(m.originalItems) {
		return errors.New("invalid index to be inserting")
	}
	if index == len(m.originalItems) {
		m.originalItems = append(m.originalItems, item)
		return m.updateTrie()
	}

	m.originalItems[index] = item
	return m.updateTrie()
}

// GenerateTrieFromItems constructs a Merkle trie from a sequence of byte slices.
func GenerateTrieFromItems(items [][]byte, depth int) (*MerkleTrie, error) {
	if len(items) == 0 {
		return nil, errors.New("no items provided to generate Merkle trie")
	}
	layers := calcTreeFromLeaves(items, depth)
	return &MerkleTrie{
		branches:      layers,
		originalItems: items,
		depth:         uint(depth),
	}, nil
}

// Items returns the original items passed in when creating the Merkle trie.
func (m *MerkleTrie) Items() [][]byte {
	return m.originalItems
}

// Root returns the top-most, Merkle root of the trie.
func (m *MerkleTrie) Root() [32]byte {
	return bytesutil.ToBytes32(m.branches[len(m.branches)-1][0])
}

// MerkleProof computes a proof from a trie's branches using a Merkle index.
func (m *MerkleTrie) MerkleProof(index int) ([][]byte, error) {
	merkleIndex := uint(index)
	leaves := m.branches[0]
	if index >= len(leaves) {
		return nil, fmt.Errorf("merkle index out of range in trie, max range: %d, received: %d", len(leaves), index)
	}
	proof := make([][]byte, m.depth)
	for i := uint(0); i < m.depth; i++ {
		subIndex := (merkleIndex / (1 << i)) ^ 1
		if subIndex < uint(len(m.branches[i])) {
			proof[i] = m.branches[i][subIndex]
		} else {
			proof[i] = zeroHashes[i]
		}
	}
	return proof, nil
}

// HashTreeRoot of the Merkle trie as defined in the deposit contract.
//  Spec Definition:
//   sha256(concat(node, self.to_little_endian_64(self.deposit_count), slice(zero_bytes32, start=0, len=24)))
func (m *MerkleTrie) HashTreeRoot() [32]byte {
	var zeroBytes [32]byte
	depositCount := uint64(len(m.originalItems))
	if len(m.originalItems) == 1 && bytes.Equal(m.originalItems[0], zeroBytes[:]) {
		// Accounting for empty tries
		depositCount = 0
	}
	newNode := append(m.branches[len(m.branches)-1][0], bytesutil.Bytes8(depositCount)...)
	newNode = append(newNode, zeroBytes[:24]...)
	return hashutil.Hash(newNode)
}

// VerifyMerkleProof verifies a Merkle branch against a root of a trie.
func VerifyMerkleProof(root []byte, item []byte, merkleIndex int, proof [][]byte) bool {
	node := item
	branchIndices := branchIndices(merkleIndex, len(proof))
	for i := 0; i < len(proof); i++ {
		if branchIndices[i]%2 == 0 {
			parentHash := hashutil.Hash(append(node[:], proof[i]...))
			node = parentHash[:]
		} else {
			parentHash := hashutil.Hash(append(proof[i], node[:]...))
			node = parentHash[:]
		}
	}
	return bytes.Equal(root, node)
}

func calcTreeFromLeaves(leaves [][]byte, depth int) [][][]byte {
	layers := make([][][]byte, depth+1)
	layers[0] = leaves
	for i := 0; i < depth; i++ {
		if len(layers[i])%2 == 1 {
			layers[i] = append(layers[i], zeroHashes[i])
		}
		updatedValues := make([][]byte, 0, 0)
		for j := 0; j < len(layers[i]); j += 2 {
			concat := hashutil.Hash(append(layers[i][j], layers[i][j+1]...))
			updatedValues = append(updatedValues, concat[:])
		}
		layers[i+1] = updatedValues
	}
	return layers
}

func branchIndices(merkleIndex int, depth int) []int {
	indices := make([]int, depth)
	idx := merkleIndex
	indices[0] = idx
	for i := 1; i < depth; i++ {
		idx /= 2
		indices[i] = idx
	}
	return indices
}

func (m *MerkleTrie) updateTrie() error {
	trie, err := GenerateTrieFromItems(m.originalItems, int(m.depth))
	if err != nil {
		return err
	}
	m.branches = trie.branches
	return nil
}
