package types

import (
	"github.com/gogo/protobuf/proto"
	"github.com/prysmaticlabs/prysm/beacon-chain/casper"
	"github.com/prysmaticlabs/prysm/beacon-chain/params"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	"github.com/prysmaticlabs/prysm/shared/hashutil"
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

// NewGenesisCrystallizedState initializes the crystallized state for slot 0.
func NewGenesisCrystallizedState(genesisValidators []*pb.ValidatorRecord) (*CrystallizedState, error) {
	// We seed the genesis crystallized state with a bunch of validators to
	// bootstrap the system.
	var err error
	if genesisValidators == nil {
		genesisValidators = casper.InitialValidators()

	}
	// Bootstrap attester indices for slots, each slot contains an array of attester indices.
	shardAndCommitteesForSlots, err := casper.InitialShardAndCommitteesForSlots(genesisValidators)
	if err != nil {
		return nil, err
	}

	// Bootstrap cross link records.
	var crosslinks []*pb.CrosslinkRecord
	for i := 0; i < shardCount; i++ {
		crosslinks = append(crosslinks, &pb.CrosslinkRecord{
			RecentlyChanged: false,
			ShardBlockHash:  make([]byte, 0, 32),
			Slot:            0,
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
			ValidatorSetChangeSlot:     0,
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
	return hashutil.Hash(data), nil
}

// CopyState returns a deep copy of the current state.
func (c *CrystallizedState) CopyState() *CrystallizedState {
	crosslinks := make([]*pb.CrosslinkRecord, len(c.Crosslinks()))
	for index, crossLink := range c.Crosslinks() {
		crosslinks[index] = &pb.CrosslinkRecord{
			RecentlyChanged: crossLink.GetRecentlyChanged(),
			ShardBlockHash:  crossLink.GetShardBlockHash(),
			Slot:            crossLink.GetSlot(),
		}
	}

	validators := make([]*pb.ValidatorRecord, len(c.Validators()))
	for index, validator := range c.Validators() {
		validators[index] = &pb.ValidatorRecord{
			Pubkey:            validator.GetPubkey(),
			WithdrawalShard:   validator.GetWithdrawalShard(),
			WithdrawalAddress: validator.GetWithdrawalAddress(),
			RandaoCommitment:  validator.GetRandaoCommitment(),
			Balance:           validator.GetBalance(),
			Status:            validator.GetStatus(),
			ExitSlot:          validator.GetExitSlot(),
		}
	}

	shardAndCommitteesForSlots := make([]*pb.ShardAndCommitteeArray, len(c.ShardAndCommitteesForSlots()))
	for index, shardAndCommitteesForSlot := range c.ShardAndCommitteesForSlots() {
		shardAndCommittees := make([]*pb.ShardAndCommittee, len(shardAndCommitteesForSlot.GetArrayShardAndCommittee()))
		for index, shardAndCommittee := range shardAndCommitteesForSlot.GetArrayShardAndCommittee() {
			shardAndCommittees[index] = &pb.ShardAndCommittee{
				Shard:     shardAndCommittee.GetShard(),
				Committee: shardAndCommittee.GetCommittee(),
			}
		}
		shardAndCommitteesForSlots[index] = &pb.ShardAndCommitteeArray{
			ArrayShardAndCommittee: shardAndCommittees,
		}
	}

	newState := CrystallizedState{&pb.CrystallizedState{
		LastStateRecalculationSlot: c.LastStateRecalculationSlot(),
		JustifiedStreak:            c.JustifiedStreak(),
		LastJustifiedSlot:          c.LastJustifiedSlot(),
		LastFinalizedSlot:          c.LastFinalizedSlot(),
		ValidatorSetChangeSlot:     c.ValidatorSetChangeSlot(),
		Crosslinks:                 crosslinks,
		Validators:                 validators,
		ShardAndCommitteesForSlots: shardAndCommitteesForSlots,
		DepositsPenalizedInPeriod:  c.DepositsPenalizedInPeriod(),
		ValidatorSetDeltaHashChain: c.data.ValidatorSetDeltaHashChain,
		PreForkVersion:             c.data.PreForkVersion,
		PostForkVersion:            c.data.PostForkVersion,
		ForkSlotNumber:             c.data.ForkSlotNumber,
	}}

	return &newState
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

// TotalDeposits returns total balance of the deposits of the active validators.
func (c *CrystallizedState) TotalDeposits() uint64 {
	validators := c.data.Validators
	totalDeposit := casper.TotalActiveValidatorDeposit(validators)
	return totalDeposit
}

// ValidatorSetChangeSlot returns the slot of last time validator set changes.
func (c *CrystallizedState) ValidatorSetChangeSlot() uint64 {
	return c.data.ValidatorSetChangeSlot
}

// ShardAndCommitteesForSlots returns the shard committee object.
func (c *CrystallizedState) ShardAndCommitteesForSlots() []*pb.ShardAndCommitteeArray {
	return c.data.ShardAndCommitteesForSlots
}

// Crosslinks returns the cross link records of the all the shards.
func (c *CrystallizedState) Crosslinks() []*pb.CrosslinkRecord {
	return c.data.Crosslinks
}

// Validators returns list of validators.
func (c *CrystallizedState) Validators() []*pb.ValidatorRecord {
	return c.data.Validators
}

// DepositsPenalizedInPeriod returns total deposits penalized in the given withdrawal period.
func (c *CrystallizedState) DepositsPenalizedInPeriod() []uint32 {
	return c.data.DepositsPenalizedInPeriod
}

// IsCycleTransition checks if a new cycle has been reached. At that point,
// a new crystallized state and active state transition will occur.
func (c *CrystallizedState) IsCycleTransition(slotNumber uint64) bool {
	return slotNumber >= c.LastStateRecalculationSlot()+params.GetConfig().CycleLength
}

// isValidatorSetChange checks if a validator set change transition can be processed. At that point,
// validator shuffle will occur.
func (c *CrystallizedState) isValidatorSetChange(slotNumber uint64) bool {
	if c.LastFinalizedSlot() <= c.ValidatorSetChangeSlot() {
		return false
	}
	if slotNumber-c.ValidatorSetChangeSlot() < params.GetConfig().MinValidatorSetChangeInterval {
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
		if c.ValidatorSetChangeSlot() >= crosslinks[shard].Slot {
			return false
		}
	}
	return true
}

// getAttesterIndices fetches the attesters for a given attestation record.
func (c *CrystallizedState) getAttesterIndices(attestation *pb.AggregatedAttestation) ([]uint32, error) {
	slotsStart := c.LastStateRecalculationSlot() - params.GetConfig().CycleLength
	slotIndex := (attestation.Slot - slotsStart) % params.GetConfig().CycleLength
	return casper.CommitteeInShardAndSlot(slotIndex, attestation.GetShard(), c.data.GetShardAndCommitteesForSlots())
}

// NewStateRecalculations computes the new crystallized state, given the previous crystallized state
// and the current active state. This method is called during a cycle transition.
// We also check for validator set change transition and compute for new committees if necessary during this transition.
func (c *CrystallizedState) NewStateRecalculations(aState *ActiveState, block *Block, enableCrossLinks bool, enableRewardChecking bool) (*CrystallizedState, error) {
	var lastStateRecalculationSlotCycleBack uint64
	var err error

	newState := c.CopyState()
	justifiedStreak := c.JustifiedStreak()
	justifiedSlot := c.LastJustifiedSlot()
	finalizedSlot := c.LastFinalizedSlot()
	blockVoteCache := aState.GetBlockVoteCache()
	timeSinceFinality := block.SlotNumber() - newState.LastFinalizedSlot()
	recentBlockHashes := aState.RecentBlockHashes()
	newState.data.Validators = casper.CopyValidators(newState.Validators())

	if c.LastStateRecalculationSlot() < params.GetConfig().CycleLength {
		lastStateRecalculationSlotCycleBack = 0
	} else {
		lastStateRecalculationSlotCycleBack = c.LastStateRecalculationSlot() - params.GetConfig().CycleLength
	}

	// If reward checking is disabled, the new set of validators for the cycle
	// will remain the same.
	if !enableRewardChecking {
		newState.data.Validators = c.Validators()
	}

	// walk through all the slots from LastStateRecalculationSlot - cycleLength to LastStateRecalculationSlot - 1.
	for i := uint64(0); i < params.GetConfig().CycleLength; i++ {
		var blockVoteBalance uint64

		slot := lastStateRecalculationSlotCycleBack + i
		blockHash := recentBlockHashes[i]

		blockVoteBalance, newState.data.Validators = casper.TallyVoteBalances(blockHash, slot,
			blockVoteCache, newState.Validators(), timeSinceFinality, enableRewardChecking)

		justifiedSlot, finalizedSlot, justifiedStreak = casper.FinalizeAndJustifySlots(slot, justifiedSlot, finalizedSlot,
			justifiedStreak, blockVoteBalance, c.TotalDeposits())

		if enableCrossLinks {
			newState.data.Crosslinks, err = newState.processCrosslinks(aState.PendingAttestations(), slot, newState.Validators(), block.SlotNumber())
			if err != nil {
				return nil, err
			}
		}
	}

	newState.data.LastJustifiedSlot = justifiedSlot
	newState.data.LastFinalizedSlot = finalizedSlot
	newState.data.JustifiedStreak = justifiedStreak
	newState.data.LastStateRecalculationSlot = newState.LastStateRecalculationSlot() + params.GetConfig().CycleLength

	// Process the pending special records gathered from last cycle.
	newState.data.Validators, err = casper.ProcessSpecialRecords(block.SlotNumber(), newState.Validators(), aState.PendingSpecials())
	if err != nil {
		return nil, err
	}

	// Exit the validators when their balance fall below min online deposit size.
	newState.data.Validators = casper.CheckValidatorMinDeposit(newState.Validators(), block.SlotNumber())

	newState.data.LastFinalizedSlot = finalizedSlot

	// Entering new validator set change transition.
	if c.isValidatorSetChange(block.SlotNumber()) {
		log.Info("Entering validator set change transition")
		newState.data.ValidatorSetChangeSlot = newState.LastStateRecalculationSlot()
		newState.data.ShardAndCommitteesForSlots, err = newState.newValidatorSetRecalculations(block.ParentHash())
		if err != nil {
			return nil, err
		}

		period := block.SlotNumber() / params.GetConfig().WithdrawalPeriod
		totalPenalties := newState.penalizedETH(period)
		newState.data.Validators = casper.ChangeValidators(block.SlotNumber(), totalPenalties, newState.Validators())
	}

	return newState, nil
}

// newValidatorSetRecalculations recomputes the validator set.
func (c *CrystallizedState) newValidatorSetRecalculations(seed [32]byte) ([]*pb.ShardAndCommitteeArray, error) {
	lastSlot := len(c.data.ShardAndCommitteesForSlots) - 1
	lastCommitteeFromLastSlot := len(c.ShardAndCommitteesForSlots()[lastSlot].ArrayShardAndCommittee) - 1
	crosslinkLastShard := c.ShardAndCommitteesForSlots()[lastSlot].ArrayShardAndCommittee[lastCommitteeFromLastSlot].Shard
	crosslinkNextShard := (crosslinkLastShard + 1) % uint64(shardCount)

	newShardCommitteeArray, err := casper.ShuffleValidatorsToCommittees(
		seed,
		c.data.Validators,
		crosslinkNextShard,
	)
	if err != nil {
		return nil, err
	}

	return append(c.data.ShardAndCommitteesForSlots[:params.GetConfig().CycleLength], newShardCommitteeArray...), nil
}

func copyCrosslinks(existing []*pb.CrosslinkRecord) []*pb.CrosslinkRecord {
	new := make([]*pb.CrosslinkRecord, len(existing))
	for i := 0; i < len(existing); i++ {
		oldCL := existing[i]
		newBlockhash := make([]byte, len(oldCL.ShardBlockHash))
		copy(newBlockhash, oldCL.ShardBlockHash)
		newCL := &pb.CrosslinkRecord{
			RecentlyChanged: oldCL.RecentlyChanged,
			ShardBlockHash:  newBlockhash,
			Slot:            oldCL.Slot,
		}
		new[i] = newCL
	}

	return new
}

// processCrosslinks checks if the proposed shard block has recevied
// 2/3 of the votes. If yes, we update crosslink record to point to
// the proposed shard block with latest beacon chain slot numbers.
func (c *CrystallizedState) processCrosslinks(pendingAttestations []*pb.AggregatedAttestation, slot uint64,
	validators []*pb.ValidatorRecord, currentSlot uint64) ([]*pb.CrosslinkRecord, error) {
	crosslinkRecords := copyCrosslinks(c.data.Crosslinks)

	for _, attestation := range pendingAttestations {
		indices, err := c.getAttesterIndices(attestation)
		if err != nil {
			return nil, err
		}

		totalBalance, voteBalance, err := casper.VotedBalanceInAttestation(validators, indices, attestation)
		if err != nil {
			return nil, err
		}

		err = casper.ApplyCrosslinkRewardsAndPenalties(crosslinkRecords, currentSlot, indices, attestation,
			validators, totalBalance, voteBalance)
		if err != nil {
			return nil, err
		}

		crosslinkRecords = casper.ProcessBalancesInCrosslink(slot, voteBalance, totalBalance, attestation, crosslinkRecords)

	}
	return crosslinkRecords, nil
}

// penalizedETH calculates penalized total ETH during the last 3 withdrawal periods.
func (c *CrystallizedState) penalizedETH(periodIndex uint64) uint64 {
	var penalties uint64

	depositsPenalizedInPeriod := c.DepositsPenalizedInPeriod()
	penalties += uint64(depositsPenalizedInPeriod[periodIndex])

	if periodIndex >= 1 {
		penalties += uint64(depositsPenalizedInPeriod[periodIndex-1])
	}

	if periodIndex >= 2 {
		penalties += uint64(depositsPenalizedInPeriod[periodIndex-2])
	}

	return penalties
}
