// Package blocks contains block processing libraries. These libraries
// process and verify block specific messages such as PoW receipt root,
// RANDAO, validator deposits, exits and slashing proofs.
package blocks

import (
	"fmt"

	"github.com/prysmaticlabs/prysm/beacon-chain/utils"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/ssz"
)

var clock utils.Clock = &utils.RealClock{}

// NewGenesisBlock returns the canonical, genesis block for the beacon chain protocol.
func NewGenesisBlock(stateRoot []byte) *pb.BeaconBlock {
	block := &pb.BeaconBlock{
		Slot:       0,
		ParentRoot: params.BeaconConfig().ZeroHash[:],
		StateRoot:  stateRoot,
		Signature:  params.BeaconConfig().EmptySignature[:],
		Body: &pb.BeaconBlockBody{
			RandaoReveal:      params.BeaconConfig().ZeroHash[:],
			ProposerSlashings: []*pb.ProposerSlashing{},
			AttesterSlashings: []*pb.AttesterSlashing{},
			Attestations:      []*pb.Attestation{},
			Deposits:          []*pb.Deposit{},
			VoluntaryExits:    []*pb.VoluntaryExit{},
			Eth1Data: &pb.Eth1Data{
				DepositRoot: params.BeaconConfig().ZeroHash[:],
				BlockRoot:   params.BeaconConfig().ZeroHash[:],
			},
		},
	}
	return block
}

// BlockRoot returns the block root stored in the BeaconState for a given slot.
// It returns an error if the requested block root is not within the BeaconState.
// Spec pseudocode definition:
// 	def get_block_root(state: BeaconState, slot: int) -> Hash32:
//		"""
//		returns the block root at a recent ``slot``.
//		"""
//		assert state.slot <= slot + SLOTS_PER_HISTORICAL_ROOT
//		assert slot < state.slote
//		return state.latest_block_roots[slot % SLOTS_PER_HISTORICAL_ROOT]
func BlockRoot(state *pb.BeaconState, slot uint64) ([]byte, error) {
	earliestSlot := uint64(0)
	if state.Slot > params.BeaconConfig().SlotsPerHistoricalRoot {
		earliestSlot = state.Slot - params.BeaconConfig().SlotsPerHistoricalRoot
	}

	if slot < earliestSlot || slot >= state.Slot {
		return []byte{}, fmt.Errorf("slot %d is not within expected range of %d to %d",
			slot,
			earliestSlot,
			state.Slot,
		)
	}

	return state.LatestBlockRoots[slot%params.BeaconConfig().SlotsPerHistoricalRoot], nil
}

// ProcessBlockRoots processes the previous block root into the state, by appending it
// to the most recent block roots.
// Spec:
//  Let previous_block_root be the tree_hash_root of the previous beacon block processed in the chain.
//	Set state.latest_block_roots[(state.slot - 1) % SLOTS_PER_HISTORICAL_ROOT] = previous_block_root.
func ProcessBlockRoots(state *pb.BeaconState, parentRoot [32]byte) *pb.BeaconState {
	state.LatestBlockRoots[(state.Slot-1)%params.BeaconConfig().SlotsPerHistoricalRoot] = parentRoot[:]
	return state
}

// BlockFromHeader manufactures a block from its header. It contains all its fields,
// expect for the block body.
func BlockFromHeader(header *pb.BeaconBlockHeader) *pb.BeaconBlock {
	return &pb.BeaconBlock{
		StateRoot:  header.StateRoot,
		Slot:       header.Slot,
		Signature:  header.Signature,
		ParentRoot: header.ParentRoot,
	}
}

// HeaderFromBlock extracts the block header from a block.
func HeaderFromBlock(block *pb.BeaconBlock) (*pb.BeaconBlockHeader, error) {
	header := &pb.BeaconBlockHeader{
		Slot:       block.Slot,
		ParentRoot: block.ParentRoot,
		Signature:  block.Signature,
		StateRoot:  block.StateRoot,
	}
	root, err := ssz.TreeHash(block.Body)
	if err != nil {
		return nil, fmt.Errorf("could not tree hash block body %v", err)
	}
	header.BodyRoot = root[:]
	return header, nil
}
