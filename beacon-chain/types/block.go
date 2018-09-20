// Package types defines the essential types used throughout the beacon-chain.
package types

import (
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/prysmaticlabs/prysm/beacon-chain/casper"
	"github.com/prysmaticlabs/prysm/beacon-chain/params"
	"github.com/prysmaticlabs/prysm/beacon-chain/utils"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	"github.com/prysmaticlabs/prysm/shared"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/blake2b"
)

var log = logrus.WithField("prefix", "types")

var genesisTime = time.Date(2018, 9, 0, 0, 0, 0, 0, time.UTC) // September 2019
var clock utils.Clock = &utils.RealClock{}

// Block defines a beacon chain core primitive.
type Block struct {
	data *pb.BeaconBlock
}

// NewBlock explicitly sets the data field of a block.
// Return block with default fields if data is nil.
func NewBlock(data *pb.BeaconBlock) *Block {
	if data == nil {
		//It is assumed when data==nil, you're asking for a Genesis Block
		return &Block{
			data: &pb.BeaconBlock{
				ParentHash:            []byte{0},
				RandaoReveal:          []byte{0},
				PowChainRef:           []byte{0},
				ActiveStateHash:       []byte{0},
				CrystallizedStateHash: []byte{0},
			},
		}
	}

	return &Block{data: data}
}

// NewGenesisBlock returns the canonical, genesis block for the beacon chain protocol.
func NewGenesisBlock() (*Block, error) {
	protoGenesis, err := ptypes.TimestampProto(genesisTime)
	if err != nil {
		return nil, err
	}

	gb := NewBlock(nil)
	gb.data.Timestamp = protoGenesis

	return gb, nil
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
	var hash [32]byte
	h := blake2b.Sum512(data)
	copy(hash[:], h[:32])
	return hash, nil
}

// ParentHash corresponding to parent beacon block.
func (b *Block) ParentHash() [32]byte {
	var h [32]byte
	copy(h[:], b.data.ParentHash)
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

// Attestations returns an array of attestations in the block.
func (b *Block) Attestations() []*pb.AggregatedAttestation {
	return b.data.Attestations
}

// Timestamp returns the Go type time.Time from the protobuf type contained in the block.
func (b *Block) Timestamp() (time.Time, error) {
	return ptypes.Timestamp(b.data.Timestamp)
}

// isSlotValid compares the slot to the system clock to determine if the block is valid.
func (b *Block) isSlotValid() bool {
	slotDuration := time.Duration(b.SlotNumber()*params.SlotDuration) * time.Second
	validTimeThreshold := genesisTime.Add(slotDuration)

	return clock.Now().After(validTimeThreshold)
}

// IsValid is called to decide if an incoming p2p block can be processed. It checks for following conditions:
// 1.) Ensure parent processed.
// 2.) Ensure pow_chain_ref processed.
// 3.) Ensure local time is large enough to process this block's slot.
// 4.) Verify that the parent block's proposer's attestation is included.
func (b *Block) IsValid(aState *ActiveState, cState *CrystallizedState, parentSlot uint64) bool {
	_, err := b.Hash()
	if err != nil {
		log.Errorf("Could not hash incoming block: %v", err)
		return false
	}

	if b.SlotNumber() == 0 {
		log.Error("Can not process a genesis block")
		return false
	}

	if !b.isSlotValid() {
		log.Errorf("Slot of block is too high: %d", b.SlotNumber())
		return false
	}

	// verify proposer from last slot is in one of the AggregatedAttestation.
	var proposerAttested bool
	_, proposerIndex, err := casper.GetProposerIndexAndShard(
		cState.ShardAndCommitteesForSlots(),
		cState.LastStateRecalc(),
		parentSlot)
	if err != nil {
		log.Errorf("Can not get proposer index %v", err)
		return false
	}
	for index, attestation := range b.Attestations() {
		if !b.isAttestationValid(index, aState, cState, parentSlot) {
			log.Debugf("attestation invalid: %v", attestation)
			return false
		}
		if shared.BitSetCount(attestation.AttesterBitfield) == 1 && shared.CheckBit(attestation.AttesterBitfield, int(proposerIndex)) {
			proposerAttested = true
		}
	}

	return proposerAttested
}

// isAttestationValid validates an attestation in a block.
// Attestations are cross-checked against validators in CrystallizedState.ShardAndCommitteesForSlots.
// In addition, the signature is verified by constructing the list of parent hashes using ActiveState.RecentBlockHashes.
func (b *Block) isAttestationValid(attestationIndex int, aState *ActiveState, cState *CrystallizedState, parentSlot uint64) bool {
	// Validate attestation's slot number has is within range of incoming block number.
	attestation := b.Attestations()[attestationIndex]

	if !isAttestationSlotNumberValid(attestation.Slot, parentSlot) {
		return false
	}

	if attestation.JustifiedSlot > cState.LastJustifiedSlot() {
		log.Debugf("attestation's last justified slot has to match crystallied state's last justified slot. Found: %d. Want: %d",
			attestation.JustifiedSlot,
			cState.LastJustifiedSlot())
		return false
	}

	// TODO(#468): Validate last justified block hash matches in the crystallizedState.

	// Get all the block hashes up to cycle length.
	parentHashes := aState.getSignedParentHashes(b, attestation)
	attesterIndices, err := cState.getAttesterIndices(attestation)
	if err != nil {
		log.Debugf("unable to get validator committee: %v", attesterIndices)
		return false
	}

	// Verify attester bitfields matches crystallized state's prev computed bitfield.
	if !casper.AreAttesterBitfieldsValid(attestation, attesterIndices) {
		return false
	}

	// TODO(#258): Generate validators aggregated pub key.

	attestationMsg := AttestationMsg(
		parentHashes,
		attestation.ShardBlockHash,
		attestation.Slot,
		attestation.ShardId,
		attestation.JustifiedSlot)

	log.Debugf("Attestation message for shard: %v, slot %v, block hash %v is: %v",
		attestation.ShardId, attestation.Slot, attestation.ShardBlockHash, attestationMsg)

	// TODO(#258): Verify msgHash against aggregated pub key and aggregated signature.

	return true
}

func isAttestationSlotNumberValid(attestationSlot uint64, parentSlot uint64) bool {
	if attestationSlot > parentSlot {
		log.Debugf("attestation slot number can't be higher than parent block's slot number. Found: %d, Needed lower than: %d",
			attestationSlot,
			parentSlot)
		return false
	}

	if parentSlot >= params.CycleLength-1 && attestationSlot < parentSlot-params.CycleLength+1 {
		log.Debugf("attestation slot number can't be lower than parent block's slot number by one CycleLength. Found: %d, Needed greater than: %d",
			attestationSlot,
			parentSlot-params.CycleLength+1)
		return false
	}

	return true
}
