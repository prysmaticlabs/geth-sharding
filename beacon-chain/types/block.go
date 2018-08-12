// Package types defines the essential types used throughout the beacon-chain.
package types

import (
	"errors"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	"golang.org/x/crypto/blake2b"
)

// Block defines a beacon chain core primitive.
type Block struct {
	data *pb.BeaconBlock
}

// NewBlock explicitly sets the data field of a block.
// Return block with default fields if data is nil.
func NewBlock(data *pb.BeaconBlock) (*Block, error) {
	if data == nil {
		return &Block{
			data: &pb.BeaconBlock{
				ParentHash:            []byte{0},
				SlotNumber:            0,
				RandaoReveal:          []byte{0},
				Attestations:          []*pb.AttestationRecord{},
				PowChainRef:           []byte{0},
				ActiveStateHash:       []byte{0},
				CrystallizedStateHash: []byte{0},
			},
		}, nil
	}

	if len(data.ParentHash) != 32 {
		return nil, errors.New("invalid block data, parent hash should be 32 bytes")
	}

	return &Block{data}, nil
}

// NewGenesisBlock returns the canonical, genesis block for the beacon chain protocol.
//
// TODO: Add more default fields.
func NewGenesisBlock() (*Block, error) {
	genesisTime := time.Date(2018, time.July, 21, 12, 0, 0, 0, time.UTC)
	protoGenesis, err := ptypes.TimestampProto(genesisTime)
	if err != nil {
		return nil, err
	}
	return &Block{data: &pb.BeaconBlock{Timestamp: protoGenesis}}, nil
}

// Proto returns the underlying protobuf data within a block primitive.
func (b *Block) Proto() *pb.BeaconBlock {
	return b.data
}

// Marshal encodes block object into the wire format.
func (b *Block) Marshal() ([]byte, error) {
	return proto.Marshal(b.data)
}

// Hash generates the blake2b hash of the block
func (b *Block) Hash() ([32]byte, error) {
	data, err := proto.Marshal(b.data)
	if err != nil {
		return [32]byte{}, fmt.Errorf("could not marshal block proto data: %v", err)
	}
	return blake2b.Sum256(data), nil
}

// ParentHash corresponding to parent beacon block.
func (b *Block) ParentHash() [32]byte {
	var h [32]byte
	copy(h[:], b.data.ParentHash[:32])
	return h
}

// SlotNumber of the beacon block.
func (b *Block) SlotNumber() uint64 {
	return b.data.SlotNumber
}

// PowChainRef returns a keccak256 hash corresponding to a PoW chain block.
func (b *Block) PowChainRef() common.Hash {
	return common.BytesToHash(b.data.PowChainRef)
}

// RandaoReveal returns the blake2b randao hash.
func (b *Block) RandaoReveal() [32]byte {
	var h [32]byte
	copy(h[:], b.data.RandaoReveal)
	return h
}

// ActiveStateHash returns the active state hash.
func (b *Block) ActiveStateHash() [32]byte {
	var h [32]byte
	copy(h[:], b.data.ActiveStateHash)
	return h
}

// CrystallizedStateHash returns the crystallized state hash.
func (b *Block) CrystallizedStateHash() [32]byte {
	var h [32]byte
	copy(h[:], b.data.CrystallizedStateHash)
	return h
}

// AttestationCount returns the number of attestations.
func (b *Block) AttestationCount() int {
	return len(b.data.Attestations)
}

// Timestamp returns the Go type time.Time from the protobuf type contained in the block.
func (b *Block) Timestamp() (time.Time, error) {
	return ptypes.Timestamp(b.data.Timestamp)
}
