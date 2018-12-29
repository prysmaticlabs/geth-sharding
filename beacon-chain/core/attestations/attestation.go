package attestations

import (
	"encoding/binary"

	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	"github.com/prysmaticlabs/prysm/shared/hashutil"
	"github.com/prysmaticlabs/prysm/shared/params"
)

// CreateAttestationMsg hashes parentHashes + shardID + slotNumber +
// shardBlockHash + justifiedSlot into a message to use for verifying
// with aggregated public key and signature.
func CreateAttestationMsg(
	blockHash []byte,
	slot uint64,
	shardID uint64,
	justifiedSlot uint64,
	forkVersion uint64,
) [32]byte {
	msg := make([]byte, binary.MaxVarintLen64)
	binary.BigEndian.PutUint64(msg, forkVersion)
	binary.PutUvarint(msg, slot%params.BeaconConfig().CycleLength)
	binary.PutUvarint(msg, shardID)
	msg = append(msg, blockHash...)
	binary.PutUvarint(msg, justifiedSlot)
	return hashutil.Hash(msg)
}

// Key generates the blake2b hash of the following attestation fields:
// slotNumber + shardID + blockHash + obliqueParentHash
// This is used for attestation table look up in localDB.
func Key(att *pb.AttestationData) [32]byte {
	key := make([]byte, binary.MaxVarintLen64)
	binary.PutUvarint(key, att.GetSlot())
	binary.PutUvarint(key, att.GetShard())
	key = append(key, att.GetShardBlockRootHash32()...)
	return hashutil.Hash(key)
}

// VerifyProposerAttestation verifies the proposer's attestation of the block.
// Proposers broadcast the attestation along with the block to its peers.
func VerifyProposerAttestation(att *pb.AttestationData, pubKey [32]byte, proposerShardID uint64) error {
	// Verify the attestation attached with block response.
	// Get proposer index and shardID.
	attestationMsg := CreateAttestationMsg(
		att.GetShardBlockRootHash32(),
		att.GetSlot(),
		proposerShardID,
		att.GetJustifiedSlot(),
		params.BeaconConfig().InitialForkVersion,
	)
	_ = attestationMsg
	_ = pubKey
	// TODO(#258): use attestationMsg to verify against signature
	// and public key. Return error if incorrect.
	return nil
}

// ContainsValidator checks if the validator is included in the attestation.
// TODO(#569): Modify method to accept a single index rather than a bitfield.
func ContainsValidator(attesterBitfield []byte, bitfield []byte) bool {
	for i := 0; i < len(bitfield); i++ {
		if bitfield[i]&attesterBitfield[i] != 0 {
			return true
		}
	}
	return false
}

// IsDoubleVote checks if both of the attestations have been used to vote for the same slot.
// Spec:
//	def is_double_vote(attestation_data_1: AttestationData,
//                   attestation_data_2: AttestationData) -> bool
//    """
//    Assumes ``attestation_data_1`` is distinct from ``attestation_data_2``.
//    Returns True if the provided ``AttestationData`` are slashable
//    due to a 'double vote'.
//    """
//    return attestation_data_1.slot == attestation_data_2.slot
func IsDoubleVote(attestation1 *pb.AttestationData, attestation2 *pb.AttestationData) bool {
	return attestation1.Slot == attestation2.Slot
}

// IsSurroundVote checks if the data provided by the attestations fulfill the conditions for
// a surround vote.
// Spec:
//	def is_surround_vote(attestation_data_1: AttestationData,
//                     attestation_data_2: AttestationData) -> bool:
//    """
//    Assumes ``attestation_data_1`` is distinct from ``attestation_data_2``.
//    Returns True if the provided ``AttestationData`` are slashable
//    due to a 'surround vote'.
//    Note: parameter order matters as this function only checks
//    that ``attestation_data_1`` surrounds ``attestation_data_2``.
//    """
//    return (
//        (attestation_data_1.justified_slot < attestation_data_2.justified_slot) and
//        (attestation_data_2.justified_slot + 1 == attestation_data_2.slot) and
//        (attestation_data_2.slot < attestation_data_1.slot)
//    )
func IsSurroundVote(attestation1 *pb.AttestationData, attestation2 *pb.AttestationData) bool {
	return attestation1.JustifiedSlot < attestation2.JustifiedSlot &&
		attestation2.JustifiedSlot+1 == attestation2.Slot &&
		attestation2.Slot < attestation1.Slot

}
