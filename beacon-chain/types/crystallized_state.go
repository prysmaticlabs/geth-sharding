package types

import (
	"fmt"
	"os"

	"github.com/ethereum/go-ethereum/common"
	"github.com/gogo/protobuf/proto"
	"github.com/golang/protobuf/jsonpb"
	"github.com/prysmaticlabs/prysm/beacon-chain/casper"
	"github.com/prysmaticlabs/prysm/beacon-chain/params"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	"github.com/prysmaticlabs/prysm/shared/bitutil"
	"golang.org/x/crypto/blake2b"
)

var shardCount = params.GetConfig().ShardCount

// CrystallizedState contains fields of every Slot state,
// it changes every Slot.
type CrystallizedState struct {
	data *pb.CrystallizedState
}

// NewCrystallizedState creates a new crystallized state with a explicitly set data field.
func NewCrystallizedState(data *pb.CrystallizedState) *CrystallizedState {
	return &CrystallizedState{data: data}
}

func initialValidators() []*pb.ValidatorRecord {
	var validators []*pb.ValidatorRecord
	for i := 0; i < params.GetConfig().BootstrappedValidatorsCount; i++ {
		validator := &pb.ValidatorRecord{
			Status:            uint64(params.Active),
			Balance:           uint64(params.GetConfig().DepositSize),
			WithdrawalAddress: []byte{},
			Pubkey:            []byte{},
		}
		validators = append(validators, validator)
	}
	return validators
}

func initialValidatorsFromJSON(genesisJSONPath string) ([]*pb.ValidatorRecord, error) {
	// #nosec G304
	// genesisJSONPath is a user input for the path of genesis.json.
	// Ex: /path/to/my/genesis.json.
	f, err := os.Open(genesisJSONPath)
	if err != nil {
		return nil, err
	}

	cState := &pb.CrystallizedState{}
	if err := jsonpb.Unmarshal(f, cState); err != nil {
		return nil, fmt.Errorf("error converting JSON to proto: %v", err)
	}

	return cState.Validators, nil
}

func initialShardAndCommitteesForSlots(validators []*pb.ValidatorRecord) ([]*pb.ShardAndCommitteeArray, error) {
	seed := make([]byte, 0, 32)
	committees, err := casper.ShuffleValidatorsToCommittees(common.BytesToHash(seed), validators, 1)
	if err != nil {
		return nil, err
	}

	// Starting with 2 cycles (128 slots) with the same committees.
	return append(committees, committees...), nil
}

// NewGenesisCrystallizedState initializes the crystallized state for slot 0.
func NewGenesisCrystallizedState(genesisJSONPath string) (*CrystallizedState, error) {
	// We seed the genesis crystallized state with a bunch of validators to
	// bootstrap the system.
	var genesisValidators []*pb.ValidatorRecord
	var err error
	if genesisJSONPath != "" {
		log.Infof("Initializing crystallized state from %s", genesisJSONPath)
		genesisValidators, err = initialValidatorsFromJSON(genesisJSONPath)
		if err != nil {
			return nil, err
		}
	} else {
		genesisValidators = initialValidators()
	}

	// Bootstrap attester indices for slots, each slot contains an array of attester indices.
	shardAndCommitteesForSlots, err := initialShardAndCommitteesForSlots(genesisValidators)
	if err != nil {
		return nil, err
	}

	// Bootstrap cross link records.
	var crosslinks []*pb.CrosslinkRecord
	for i := 0; i < shardCount; i++ {
		crosslinks = append(crosslinks, &pb.CrosslinkRecord{
			Dynasty:        0,
			ShardBlockHash: make([]byte, 0, 32),
			Slot:           0,
		})
	}

	// Calculate total deposit from boot strapped validators.
	var totalDeposit uint64
	for _, v := range genesisValidators {
		totalDeposit += v.Balance
	}

	return &CrystallizedState{
		data: &pb.CrystallizedState{
			LastStateRecalculationSlot: 0,
			JustifiedStreak:            0,
			LastJustifiedSlot:          0,
			LastFinalizedSlot:          0,
			Dynasty:                    1,
			DynastySeed:                []byte{},
			DynastyStartSlot:           0,
			Crosslinks:                 crosslinks,
			Validators:                 genesisValidators,
			ShardAndCommitteesForSlots: shardAndCommitteesForSlots,
		},
	}, nil
}

// Proto returns the underlying protobuf data within a state primitive.
func (c *CrystallizedState) Proto() *pb.CrystallizedState {
	return c.data
}

// Marshal encodes crystallized state object into the wire format.
func (c *CrystallizedState) Marshal() ([]byte, error) {
	return proto.Marshal(c.data)
}

// Hash serializes the crystallized state object then uses
// blake2b to hash the serialized object.
func (c *CrystallizedState) Hash() ([32]byte, error) {
	data, err := proto.Marshal(c.data)
	if err != nil {
		return [32]byte{}, err
	}
	var hash [32]byte
	h := blake2b.Sum512(data)
	copy(hash[:], h[:32])
	return hash, nil
}

// LastStateRecalculationSlot returns when the last time crystallized state recalculated.
func (c *CrystallizedState) LastStateRecalculationSlot() uint64 {
	return c.data.LastStateRecalculationSlot
}

// JustifiedStreak returns number of consecutive justified slots ending at head.
func (c *CrystallizedState) JustifiedStreak() uint64 {
	return c.data.JustifiedStreak
}

// LastJustifiedSlot return the last justified slot of the beacon chain.
func (c *CrystallizedState) LastJustifiedSlot() uint64 {
	return c.data.LastJustifiedSlot
}

// LastFinalizedSlot returns the last finalized Slot of the beacon chain.
func (c *CrystallizedState) LastFinalizedSlot() uint64 {
	return c.data.LastFinalizedSlot
}

// Dynasty returns the current dynasty of the beacon chain.
func (c *CrystallizedState) Dynasty() uint64 {
	return c.data.Dynasty
}

// TotalDeposits returns total balance of the deposits of the active validators.
func (c *CrystallizedState) TotalDeposits() uint64 {
	validators := c.data.Validators
	totalDeposit := casper.TotalActiveValidatorDeposit(validators)
	return totalDeposit
}

// DynastyStartSlot returns the last dynasty start number.
func (c *CrystallizedState) DynastyStartSlot() uint64 {
	return c.data.DynastyStartSlot
}

// ShardAndCommitteesForSlots returns the shard committee object.
func (c *CrystallizedState) ShardAndCommitteesForSlots() []*pb.ShardAndCommitteeArray {
	return c.data.ShardAndCommitteesForSlots
}

// Crosslinks returns the cross link records of the all the shards.
func (c *CrystallizedState) Crosslinks() []*pb.CrosslinkRecord {
	return c.data.Crosslinks
}

// DynastySeed is used to select the committee for each shard.
func (c *CrystallizedState) DynastySeed() [32]byte {
	var h [32]byte
	copy(h[:], c.data.DynastySeed)
	return h
}

// Validators returns list of validators.
func (c *CrystallizedState) Validators() []*pb.ValidatorRecord {
	return c.data.Validators
}

// IsCycleTransition checks if a new cycle has been reached. At that point,
// a new crystallized state and active state transition will occur.
func (c *CrystallizedState) IsCycleTransition(slotNumber uint64) bool {
	if c.LastStateRecalculationSlot() == 0 && slotNumber == params.GetConfig().CycleLength-1 {
		return true
	}
	return slotNumber >= c.LastStateRecalculationSlot()+params.GetConfig().CycleLength-1
}

// isDynastyTransition checks if a dynasty transition can be processed. At that point,
// validator shuffle will occur.
func (c *CrystallizedState) isDynastyTransition(slotNumber uint64) bool {
	if c.LastFinalizedSlot() <= c.DynastyStartSlot() {
		return false
	}
	if slotNumber-c.DynastyStartSlot() < params.GetConfig().MinDynastyLength {
		return false
	}

	shardProcessed := map[uint64]bool{}

	for _, shardAndCommittee := range c.ShardAndCommitteesForSlots() {
		for _, committee := range shardAndCommittee.ArrayShardAndCommittee {
			shardProcessed[committee.Shard] = true
		}
	}

	crosslinks := c.Crosslinks()
	for shard := range shardProcessed {
		if c.DynastyStartSlot() >= crosslinks[shard].Slot {
			return false
		}
	}
	return true
}

// getAttesterIndices fetches the attesters for a given attestation record.
func (c *CrystallizedState) getAttesterIndices(attestation *pb.AggregatedAttestation) ([]uint32, error) {
	slotsStart := c.LastStateRecalculationSlot() - params.GetConfig().CycleLength
	slotIndex := (attestation.Slot - slotsStart) % params.GetConfig().CycleLength
	shardCommitteeArray := c.data.ShardAndCommitteesForSlots
	shardCommittee := shardCommitteeArray[slotIndex].ArrayShardAndCommittee
	for i := 0; i < len(shardCommittee); i++ {
		if attestation.Shard == shardCommittee[i].Shard {
			return shardCommittee[i].Committee, nil
		}
	}
	return nil, fmt.Errorf("unable to find attestation based on slot: %v, Shard: %v", attestation.Slot, attestation.Shard)
}

// NewStateRecalculations computes the new crystallized state, given the previous crystallized state
// and the current active state. This method is called during a cycle transition.
// We also check for dynasty transition and compute for a new dynasty if necessary during this transition.
func (c *CrystallizedState) NewStateRecalculations(aState *ActiveState, block *Block, enableCrossLinks bool, enableRewardChecking bool) (*CrystallizedState, *ActiveState, error) {
	var blockVoteBalance uint64
	var LastStateRecalculationSlotCycleBack uint64
	var newValidators []*pb.ValidatorRecord
	var newCrosslinks []*pb.CrosslinkRecord
	var err error

	justifiedStreak := c.JustifiedStreak()
	justifiedSlot := c.LastJustifiedSlot()
	finalizedSlot := c.LastFinalizedSlot()
	LastStateRecalculationSlot := c.LastStateRecalculationSlot()
	Dynasty := c.Dynasty()
	DynastyStartSlot := c.DynastyStartSlot()
	blockVoteCache := aState.GetBlockVoteCache()
	ShardAndCommitteesForSlots := c.ShardAndCommitteesForSlots()
	timeSinceFinality := block.SlotNumber() - c.LastFinalizedSlot()
	recentBlockHashes := aState.RecentBlockHashes()

	if LastStateRecalculationSlot < params.GetConfig().CycleLength {
		LastStateRecalculationSlotCycleBack = 0
	} else {
		LastStateRecalculationSlotCycleBack = LastStateRecalculationSlot - params.GetConfig().CycleLength
	}

	// If reward checking is disabled, the new set of validators for the cycle
	// will remain the same.
	if !enableRewardChecking {
		newValidators = c.data.Validators
	}

	// walk through all the slots from LastStateRecalculationSlot - cycleLength to LastStateRecalculationSlot - 1.
	for i := uint64(0); i < params.GetConfig().CycleLength; i++ {
		var voterIndices []uint32

		slot := LastStateRecalculationSlotCycleBack + i
		blockHash := recentBlockHashes[i]
		if _, ok := blockVoteCache[blockHash]; ok {
			blockVoteBalance = blockVoteCache[blockHash].VoteTotalDeposit
			voterIndices = blockVoteCache[blockHash].VoterIndices

			// Apply Rewards for each slot.
			if enableRewardChecking {
				newValidators = casper.CalculateRewards(
					slot,
					voterIndices,
					c.Validators(),
					blockVoteBalance,
					timeSinceFinality)
			}
		} else {
			blockVoteBalance = 0
		}

		if 3*blockVoteBalance >= 2*c.TotalDeposits() {
			if slot > justifiedSlot {
				justifiedSlot = slot
			}
			justifiedStreak++
		} else {
			justifiedStreak = 0
		}

		if slot > params.GetConfig().CycleLength && justifiedStreak >= params.GetConfig().CycleLength+1 && slot-params.GetConfig().CycleLength-1 > finalizedSlot {
			finalizedSlot = slot - params.GetConfig().CycleLength - 1
		}

		if enableCrossLinks {
			newCrosslinks, err = c.processCrosslinks(aState.PendingAttestations(), slot, block.SlotNumber())
			if err != nil {
				return nil, nil, err
			}
		}
	}

	// Clean up old attestations.
	newPendingAttestations := aState.cleanUpAttestations(LastStateRecalculationSlot)

	c.data.LastFinalizedSlot = finalizedSlot
	// Entering new dynasty transition.
	if c.isDynastyTransition(block.SlotNumber()) {
		log.Info("Entering dynasty transition")
		DynastyStartSlot = LastStateRecalculationSlot
		Dynasty, ShardAndCommitteesForSlots, err = c.newDynastyRecalculations(block.ParentHash())
		if err != nil {
			return nil, nil, err
		}
	}

	// Construct new crystallized state after cycle and dynasty transition.
	newCrystallizedState := NewCrystallizedState(&pb.CrystallizedState{
		DynastySeed:                c.data.DynastySeed,
		ShardAndCommitteesForSlots: ShardAndCommitteesForSlots,
		Validators:                 newValidators,
		LastStateRecalculationSlot: LastStateRecalculationSlot + params.GetConfig().CycleLength,
		LastJustifiedSlot:          justifiedSlot,
		JustifiedStreak:            justifiedStreak,
		LastFinalizedSlot:          finalizedSlot,
		Crosslinks:                 newCrosslinks,
		DynastyStartSlot:           DynastyStartSlot,
		Dynasty:                    Dynasty,
	})

	// Construct new active state after clean up pending attestations.
	newActiveState := NewActiveState(&pb.ActiveState{
		PendingAttestations: newPendingAttestations,
		RecentBlockHashes:   aState.data.RecentBlockHashes,
	}, aState.blockVoteCache)

	return newCrystallizedState, newActiveState, nil
}

// newDynastyRecalculations recomputes the validator set. This method is called during a dynasty transition.
func (c *CrystallizedState) newDynastyRecalculations(seed [32]byte) (uint64, []*pb.ShardAndCommitteeArray, error) {
	lastSlot := len(c.data.ShardAndCommitteesForSlots) - 1
	lastCommitteeFromLastSlot := len(c.ShardAndCommitteesForSlots()[lastSlot].ArrayShardAndCommittee) - 1
	crosslinkLastShard := c.ShardAndCommitteesForSlots()[lastSlot].ArrayShardAndCommittee[lastCommitteeFromLastSlot].Shard
	crosslinkNextShard := (crosslinkLastShard + 1) % uint64(shardCount)
	nextDynasty := c.Dynasty() + 1

	newShardCommitteeArray, err := casper.ShuffleValidatorsToCommittees(
		seed,
		c.data.Validators,
		crosslinkNextShard,
	)
	if err != nil {
		return 0, nil, err
	}

	return nextDynasty, append(c.data.ShardAndCommitteesForSlots[:params.GetConfig().CycleLength], newShardCommitteeArray...), nil
}

type shardAttestation struct {
	Shard          uint64
	shardBlockHash [32]byte
}

func copyCrosslinks(existing []*pb.CrosslinkRecord) []*pb.CrosslinkRecord {
	new := make([]*pb.CrosslinkRecord, len(existing))
	for i := 0; i < len(existing); i++ {
		oldCL := existing[i]
		newBlockhash := make([]byte, len(oldCL.ShardBlockHash))
		copy(newBlockhash, oldCL.ShardBlockHash)
		newCL := &pb.CrosslinkRecord{
			Dynasty:        oldCL.Dynasty,
			ShardBlockHash: newBlockhash,
			Slot:           oldCL.Slot,
		}
		new[i] = newCL
	}

	return new
}

// processCrosslinks checks if the proposed shard block has recevied
// 2/3 of the votes. If yes, we update crosslink record to point to
// the proposed shard block with latest dynasty and slot numbers.
func (c *CrystallizedState) processCrosslinks(pendingAttestations []*pb.AggregatedAttestation, slot uint64, currentSlot uint64) ([]*pb.CrosslinkRecord, error) {
	validators := c.data.Validators
	dynasty := c.data.Dynasty
	crosslinkRecords := copyCrosslinks(c.data.Crosslinks)
	rewardQuotient := casper.RewardQuotient(validators)

	shardAttestationBalance := map[shardAttestation]uint64{}
	for _, attestation := range pendingAttestations {
		indices, err := c.getAttesterIndices(attestation)
		if err != nil {
			return nil, err
		}

		shardBlockHash := [32]byte{}
		copy(shardBlockHash[:], attestation.ShardBlockHash)
		shardAtt := shardAttestation{
			Shard:          attestation.Shard,
			shardBlockHash: shardBlockHash,
		}
		if _, ok := shardAttestationBalance[shardAtt]; !ok {
			shardAttestationBalance[shardAtt] = 0
		}

		// find the total and vote balance of the shard committee.
		var totalBalance uint64
		var voteBalance uint64
		for _, attesterIndex := range indices {
			// find balance of validators who voted.
			if bitutil.CheckBit(attestation.AttesterBitfield, int(attesterIndex)) {
				voteBalance += validators[attesterIndex].Balance
			}
			// add to total balance of the committee.
			totalBalance += validators[attesterIndex].Balance
		}

		for _, attesterIndex := range indices {
			timeSinceLastConfirmation := currentSlot - crosslinkRecords[attestation.Shard].GetSlot()

			if crosslinkRecords[attestation.Slot].GetDynasty() != dynasty {
				if bitutil.CheckBit(attestation.AttesterBitfield, int(attesterIndex)) {
					casper.RewardValidatorCrosslink(totalBalance, voteBalance, rewardQuotient, validators[attesterIndex])
				} else {
					casper.PenaliseValidatorCrosslink(timeSinceLastConfirmation, rewardQuotient, validators[attesterIndex])
				}
			}
		}

		shardAttestationBalance[shardAtt] += voteBalance

		// if 2/3 of committee voted on this crosslink, update the crosslink
		// with latest dynasty number, shard block hash, and slot number.
		if 3*voteBalance >= 2*totalBalance && dynasty > crosslinkRecords[attestation.Shard].Dynasty {
			crosslinkRecords[attestation.Shard] = &pb.CrosslinkRecord{
				Dynasty:        dynasty,
				ShardBlockHash: attestation.ShardBlockHash,
				Slot:           slot,
			}
		}
	}
	return crosslinkRecords, nil
}
