package state

import (
	"bytes"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/gogo/protobuf/proto"
	b "github.com/prysmaticlabs/prysm/beacon-chain/core/blocks"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	"github.com/prysmaticlabs/prysm/shared/hashutil"
	"github.com/prysmaticlabs/prysm/shared/params"
)

func TestInitialBeaconState_Ok(t *testing.T) {
	if params.BeaconConfig().EpochLength != 64 {
		t.Errorf("EpochLength should be 64 for these tests to pass")
	}
	epochLength := params.BeaconConfig().EpochLength

	if params.BeaconConfig().GenesisSlot != 0 {
		t.Error("GenesisSlot should be 0 for these tests to pass")
	}
	initialSlotNumber := params.BeaconConfig().GenesisSlot

	if params.BeaconConfig().GenesisForkVersion != 0 {
		t.Error("InitialForkSlot should be 0 for these tests to pass")
	}
	initialForkVersion := params.BeaconConfig().GenesisForkVersion

	if params.BeaconConfig().ZeroHash != [32]byte{} {
		t.Error("ZeroHash should be all 0s for these tests to pass")
	}

	if params.BeaconConfig().LatestRandaoMixesLength != 8192 {
		t.Error("LatestRandaoMixesLength should be 8192 for these tests to pass")
	}
	latestRandaoMixesLength := int(params.BeaconConfig().LatestRandaoMixesLength)
	LatestVdfMixesLength := int(params.BeaconConfig().LatestRandaoMixesLength /
		params.BeaconConfig().EpochLength)

	if params.BeaconConfig().ShardCount != 1024 {
		t.Error("ShardCount should be 1024 for these tests to pass")
	}
	shardCount := int(params.BeaconConfig().ShardCount)

	if params.BeaconConfig().LatestBlockRootsLength != 8192 {
		t.Error("LatestBlockRootsLength should be 8192 for these tests to pass")
	}

	if params.BeaconConfig().DepositsForChainStart != 16384 {
		t.Error("DepositsForChainStart should be 16384 for these tests to pass")
	}
	depositsForChainStart := int(params.BeaconConfig().DepositsForChainStart)

	if params.BeaconConfig().LatestPenalizedExitLength != 8192 {
		t.Error("LatestPenalizedExitLength should be 8192 for these tests to pass")
	}
	latestPenalizedExitLength := int(params.BeaconConfig().LatestPenalizedExitLength)

	genesisTime := uint64(99999)
	processedPowReceiptRoot := []byte{'A', 'B', 'C'}
	maxDeposit := params.BeaconConfig().MaxDepositInGwei
	var deposits []*pb.Deposit
	for i := 0; i < depositsForChainStart; i++ {
		depositData, err := b.EncodeDepositData(
			&pb.DepositInput{
				Pubkey: []byte(strconv.Itoa(i)), ProofOfPossession: []byte{'B'},
				WithdrawalCredentialsHash32: []byte{'C'}, RandaoCommitmentHash32: []byte{'D'},
				CustodyCommitmentHash32: []byte{'D'},
			},
			maxDeposit,
			time.Now().Unix(),
		)
		if err != nil {
			t.Fatalf("Could not encode deposit data: %v", err)
		}
		deposits = append(deposits, &pb.Deposit{
			MerkleBranchHash32S: [][]byte{{1}, {2}, {3}},
			MerkleTreeIndex:     0,
			DepositData:         depositData,
		})
	}

	state, err := InitialBeaconState(
		deposits,
		genesisTime,
		processedPowReceiptRoot)
	if err != nil {
		t.Fatalf("could not execute InitialBeaconState: %v", err)
	}

	// Misc fields checks.
	if state.Slot != initialSlotNumber {
		t.Error("Slot was not correctly initialized")
	}
	if state.GenesisTime != genesisTime {
		t.Error("GenesisTime was not correctly initialized")
	}
	if !reflect.DeepEqual(*state.ForkData, pb.ForkData{
		PreForkVersion:  initialForkVersion,
		PostForkVersion: initialForkVersion,
		ForkSlot:        initialSlotNumber,
	}) {
		t.Error("ForkData was not correctly initialized")
	}

	// Validator registry fields checks.
	if state.ValidatorRegistryLatestChangeSlot != initialSlotNumber {
		t.Error("ValidatorRegistryLatestChangeSlot was not correctly initialized")
	}
	if state.ValidatorRegistryExitCount != 0 {
		t.Error("ValidatorRegistryExitCount was not correctly initialized")
	}
	if len(state.ValidatorRegistry) != depositsForChainStart {
		t.Error("ValidatorRegistry was not correctly initialized")
	}
	if len(state.ValidatorBalances) != depositsForChainStart {
		t.Error("ValidatorBalances was not correctly initialized")
	}

	// Randomness and committees fields checks.
	if len(state.LatestRandaoMixesHash32S) != latestRandaoMixesLength {
		t.Error("Length of LatestRandaoMixesHash32S was not correctly initialized")
	}
	if len(state.LatestVdfOutputsHash32S) != LatestVdfMixesLength {
		t.Error("Length of LatestRandaoMixesHash32S was not correctly initialized")
	}

	// Proof of custody field check.
	if !reflect.DeepEqual(state.CustodyChallenges, []*pb.CustodyChallenge{}) {
		t.Error("CustodyChallenges was not correctly initialized")
	}

	// Finality fields checks.
	if state.PreviousJustifiedSlot != initialSlotNumber {
		t.Error("PreviousJustifiedSlot was not correctly initialized")
	}
	if state.JustifiedSlot != initialSlotNumber {
		t.Error("JustifiedSlot was not correctly initialized")
	}
	if state.FinalizedSlot != initialSlotNumber {
		t.Error("FinalizedSlot was not correctly initialized")
	}
	if state.JustificationBitfield != 0 {
		t.Error("JustificationBitfield was not correctly initialized")
	}

	// Recent state checks.
	if len(state.LatestCrosslinks) != shardCount {
		t.Error("Length of LatestCrosslinks was not correctly initialized")
	}
	if !reflect.DeepEqual(state.LatestPenalizedExitBalances,
		make([]uint64, latestPenalizedExitLength)) {
		t.Error("LatestPenalizedExitBalances was not correctly initialized")
	}
	if !reflect.DeepEqual(state.LatestAttestations, []*pb.PendingAttestationRecord{}) {
		t.Error("LatestAttestations was not correctly initialized")
	}
	if !reflect.DeepEqual(state.BatchedBlockRootHash32S, [][]byte{}) {
		t.Error("BatchedBlockRootHash32S was not correctly initialized")
	}

	// deposit root checks.
	if !bytes.Equal(state.LatestDepositRootHash32, processedPowReceiptRoot) {
		t.Error("LatestDepositRootHash32 was not correctly initialized")
	}
	if !reflect.DeepEqual(state.DepositRootVotes, []*pb.DepositRootVote{}) {
		t.Error("DepositRootVotes was not correctly initialized")
	}

	// Initial committee shuffling check.
	if len(state.ShardCommitteesAtSlots) != int(2*epochLength) {
		t.Error("ShardCommitteesAtSlots was not correctly initialized")
	}

	for i := 0; i < len(state.ShardCommitteesAtSlots); i++ {
		if len(state.ShardCommitteesAtSlots[i].ArrayShardCommittee[0].Committee) !=
			int(params.BeaconConfig().TargetCommitteeSize) {
			t.Errorf("ShardCommittees was not correctly initialized %d",
				len(state.ShardCommitteesAtSlots[i].ArrayShardCommittee[0].Committee))
		}
	}
}

func TestGenesisState_HashEquality(t *testing.T) {
	state1, _ := InitialBeaconState(nil, 0, nil)
	state2, _ := InitialBeaconState(nil, 0, nil)

	enc1, err1 := proto.Marshal(state1)
	enc2, err2 := proto.Marshal(state2)

	if err1 != nil || err2 != nil {
		t.Fatalf("Failed to marshal state to bytes: %v %v", err1, err2)
	}

	h1 := hashutil.Hash(enc1)
	h2 := hashutil.Hash(enc2)
	if h1 != h2 {
		t.Fatalf("Hash of two genesis states should be equal: %#x", h1)
	}
}

func TestGenesisState_InitializesLatestBlockHashes(t *testing.T) {
	s, _ := InitialBeaconState(nil, 0, nil)
	want, got := len(s.LatestBlockRootHash32S), int(params.BeaconConfig().LatestBlockRootsLength)
	if want != got {
		t.Errorf("Wrong number of recent block hashes. Got: %d Want: %d", got, want)
	}

	want = cap(s.LatestBlockRootHash32S)
	if want != got {
		t.Errorf("The slice underlying array capacity is wrong. Got: %d Want: %d", got, want)
	}

	for _, h := range s.LatestBlockRootHash32S {
		if !bytes.Equal(h, params.BeaconConfig().ZeroHash[:]) {
			t.Errorf("Unexpected non-zero hash data: %v", h)
		}
	}
}
