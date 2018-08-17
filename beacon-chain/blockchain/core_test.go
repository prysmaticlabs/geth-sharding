package blockchain

import (
	"bytes"
	"context"
	"errors"
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	gethTypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/gogo/protobuf/proto"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/prysmaticlabs/prysm/beacon-chain/params"
	"github.com/prysmaticlabs/prysm/beacon-chain/types"
	"github.com/prysmaticlabs/prysm/beacon-chain/utils"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	"github.com/prysmaticlabs/prysm/shared/database"
	logTest "github.com/sirupsen/logrus/hooks/test"
)

// FakeClock represents an mocked clock for testing purposes.
type fakeClock struct{}

// Now represents the mocked functionality of a Clock.Now().
func (fakeClock) Now() time.Time {
	return time.Date(1970, 2, 1, 1, 0, 0, 0, time.UTC)
}

type faultyFetcher struct{}

func (f *faultyFetcher) BlockByHash(ctx context.Context, hash common.Hash) (*gethTypes.Block, error) {
	return nil, errors.New("cannot fetch block")
}

type mockFetcher struct{}

func (m *mockFetcher) BlockByHash(ctx context.Context, hash common.Hash) (*gethTypes.Block, error) {
	block := gethTypes.NewBlock(&gethTypes.Header{}, nil, nil, nil)
	return block, nil
}

func startInMemoryBeaconChain(t *testing.T) (*BeaconChain, *database.DB) {
	config := &database.DBConfig{DataDir: "", Name: "", InMemory: true}
	db, err := database.NewDB(config)
	if err != nil {
		t.Fatalf("unable to setup db: %v", err)
	}
	beaconChain, err := NewBeaconChain(db.DB())
	if err != nil {
		t.Fatalf("unable to setup beacon chain: %v", err)
	}

	return beaconChain, db
}

func TestNewBeaconChain(t *testing.T) {
	hook := logTest.NewGlobal()
	beaconChain, db := startInMemoryBeaconChain(t)
	defer db.Close()

	msg := hook.LastEntry().Message
	want := "No chainstate found on disk, initializing beacon from genesis"
	if msg != want {
		t.Errorf("incorrect log, expected %s, got %s", want, msg)
	}

	hook.Reset()
	active, crystallized := types.NewGenesisStates()
	if _, err := types.NewGenesisBlock(); err != nil {
		t.Errorf("Creating a new genesis block failed %v", err)
	}
	if !reflect.DeepEqual(beaconChain.ActiveState(), active) {
		t.Errorf("active states not equal. received: %v, wanted: %v", beaconChain.ActiveState(), active)
	}
	if !reflect.DeepEqual(beaconChain.CrystallizedState(), crystallized) {
		t.Errorf("crystallized states not equal. received: %v, wanted: %v", beaconChain.CrystallizedState(), crystallized)
	}
	if _, err := beaconChain.GenesisBlock(); err != nil {
		t.Errorf("Getting new beaconchain genesis failed: %v", err)
	}
}

func TestGetGenesisBlock(t *testing.T) {
	beaconChain, db := startInMemoryBeaconChain(t)
	defer db.Close()

	block := &pb.BeaconBlock{
		ParentHash: make([]byte, 32),
		Timestamp: &timestamp.Timestamp{
			Seconds: 13000000,
		},
	}
	bytes, err := proto.Marshal(block)
	if err != nil {
		t.Errorf("unable to Marshal genesis block: %v", err)
	}

	if err := db.DB().Put([]byte("genesis"), bytes); err != nil {
		t.Errorf("unable to save key value of genesis: %v", err)
	}

	genesisBlock, err := beaconChain.GenesisBlock()
	if err != nil {
		t.Errorf("unable to get key value of genesis: %v", err)
	}

	time, err := genesisBlock.Timestamp()
	if err != nil {
		t.Errorf("Timestamp could not be retrieved: %v", err)
	}
	if time.Second() != 40 {
		t.Errorf("Timestamp was not saved properly: %v", time.Second())
	}
}

func TestSetActiveState(t *testing.T) {
	beaconChain, db := startInMemoryBeaconChain(t)
	defer db.Close()

	data := &pb.ActiveState{
		PendingAttestations: []*pb.AttestationRecord{
			{Slot: 0, ShardBlockHash: []byte{1}}, {Slot: 1, ShardBlockHash: []byte{2}},
		},
		RecentBlockHashes: [][]byte{
			{'A'}, {'B'}, {'C'}, {'D'},
		},
	}
	active := types.NewActiveState(data)

	if err := beaconChain.SetActiveState(active); err != nil {
		t.Fatalf("unable to mutate active state: %v", err)
	}
	if !reflect.DeepEqual(beaconChain.state.ActiveState, active) {
		t.Errorf("active state was not updated. wanted %v, got %v", active, beaconChain.state.ActiveState)
	}

	// Initializing a new beacon chain should deserialize persisted state from disk.
	newBeaconChain, err := NewBeaconChain(db.DB())
	if err != nil {
		t.Fatalf("unable to setup second beacon chain: %v", err)
	}

	// The active state should still be the one we mutated and persited earlier
	for i, hash := range active.RecentBlockHashes() {
		if hash.Hex() != newBeaconChain.ActiveState().RecentBlockHashes()[i].Hex() {
			t.Errorf("active state block hash. wanted %v, got %v", hash.Hex(), newBeaconChain.ActiveState().RecentBlockHashes()[i].Hex())
		}
	}
	if reflect.DeepEqual(active.PendingAttestations(), newBeaconChain.state.ActiveState.RecentBlockHashes()) {
		t.Errorf("active state pending attestation incorrect. wanted %v, got %v", active.PendingAttestations(), newBeaconChain.state.ActiveState.RecentBlockHashes())
	}
}

func TestSetCrystallizedState(t *testing.T) {
	beaconChain, db := startInMemoryBeaconChain(t)
	defer db.Close()

	data := &pb.CrystallizedState{
		CurrentDynasty: 3,
		DynastySeed:    []byte{'A'},
	}
	crystallized := types.NewCrystallizedState(data)

	if err := beaconChain.SetCrystallizedState(crystallized); err != nil {
		t.Fatalf("unable to mutate crystallized state: %v", err)
	}
	if !reflect.DeepEqual(beaconChain.state.CrystallizedState, crystallized) {
		t.Errorf("crystallized state was not updated. wanted %v, got %v", crystallized, beaconChain.state.CrystallizedState)
	}

	// Initializing a new beacon chain should deserialize persisted state from disk.
	newBeaconChain, err := NewBeaconChain(db.DB())
	if err != nil {
		t.Fatalf("unable to setup second beacon chain: %v", err)
	}

	// The crystallized state should still be the one we mutated and persited earlier.
	if crystallized.CurrentDynasty() != newBeaconChain.state.CrystallizedState.CurrentDynasty() {
		t.Errorf("crystallized state dynasty incorrect. wanted %v, got %v", crystallized.CurrentDynasty(), newBeaconChain.state.CrystallizedState.CurrentDynasty())
	}
	if crystallized.DynastySeed() != newBeaconChain.state.CrystallizedState.DynastySeed() {
		t.Errorf("crystallized state current checkpoint incorrect. wanted %v, got %v", crystallized.DynastySeed(), newBeaconChain.state.CrystallizedState.DynastySeed())
	}
}

func TestGetAttestersProposer(t *testing.T) {
	beaconChain, db := startInMemoryBeaconChain(t)
	defer db.Close()
	// Create validators more than params.MaxValidators, this should fail.
	var validators []*pb.ValidatorRecord
	for i := 0; i < params.MaxValidators+1; i++ {
		validator := &pb.ValidatorRecord{StartDynasty: 1, EndDynasty: 100}
		validators = append(validators, validator)
	}
	_, crystallized := types.NewGenesisStates()
	crystallized.SetValidators(validators)
	crystallized.IncrementCurrentDynasty()
	beaconChain.SetCrystallizedState(crystallized)

	_, _, err := beaconChain.getAttestersProposer(common.Hash{'A'})
	if err == nil {
		t.Errorf("GetAttestersProposer should have failed")
	}

	// computeNewActiveState should fail the same.
	_, err = beaconChain.computeNewActiveState(common.BytesToHash([]byte{'A'}))
	if err == nil {
		t.Errorf("computeNewActiveState should have failed")
	}

	// validatorsByHeightShard should fail the same.
	_, err = beaconChain.validatorsByHeightShard()
	if err == nil {
		t.Errorf("validatorsByHeightShard should have failed")
	}

	// Create 1000 validators in ActiveValidators.
	validators = validators[:0]
	for i := 0; i < 1000; i++ {
		validator := &pb.ValidatorRecord{StartDynasty: 1, EndDynasty: 100}
		validators = append(validators, validator)
	}

	_, crystallized = types.NewGenesisStates()
	crystallized.SetValidators(validators)
	crystallized.IncrementCurrentDynasty()
	beaconChain.SetCrystallizedState(crystallized)

	attesters, proposer, err := beaconChain.getAttestersProposer(common.Hash{'A'})
	if err != nil {
		t.Errorf("GetAttestersProposer function failed: %v", err)
	}

	activeValidators := beaconChain.activeValidatorIndices()

	validatorList, err := utils.ShuffleIndices(common.Hash{'A'}, activeValidators)
	if err != nil {
		t.Errorf("Shuffle function function failed: %v", err)
	}

	if !reflect.DeepEqual(proposer, validatorList[len(validatorList)-1]) {
		t.Errorf("Get proposer failed, expected: %v got: %v", validatorList[len(validatorList)-1], proposer)
	}
	if !reflect.DeepEqual(attesters, validatorList[:len(attesters)]) {
		t.Errorf("Get attesters failed, expected: %v got: %v", validatorList[:len(attesters)], attesters)
	}

	indices, err := beaconChain.validatorsByHeightShard()
	if err != nil {
		t.Errorf("validatorsByHeightShard failed with %v:", err)
	}
	if len(indices) != 8192 {
		t.Errorf("incorret length for validator indices. Want: 8192. Got: %v", len(indices))
	}
}

func TestCanProcessBlockValidator(t *testing.T) {
	beaconChain, db := startInMemoryBeaconChain(t)
	defer db.Close()

	clock = &fakeClock{}

	// Initialize a parent block.
	parentBlock := NewBlock(t, &pb.BeaconBlock{
		SlotNumber: 1,
	})
	parentHash, err := parentBlock.Hash()
	if err != nil {
		t.Fatalf("Failed to compute parent block's hash: %v", err)
	}
	if err = db.DB().Put(parentHash[:], []byte{}); err != nil {
		t.Fatalf("Failed to put parent block on db: %v", err)
	}

	// Using a faulty fetcher should throw an error.
	block := NewBlock(t, &pb.BeaconBlock{
		SlotNumber: 2,
	})
	if _, err := beaconChain.CanProcessBlockValidator(&faultyFetcher{}, block); err == nil {
		t.Error("Using a faulty fetcher should throw an error, received nil")
	}

	// Initialize initial state.
	activeState := types.NewActiveState(&pb.ActiveState{RecentBlockHashes: [][]byte{{'A'}}})
	beaconChain.state.ActiveState = activeState
	activeHash, err := activeState.Hash()
	if err != nil {
		t.Fatalf("Cannot hash active state: %v", err)
	}

	crystallized := types.NewCrystallizedState(&pb.CrystallizedState{})
	beaconChain.state.CrystallizedState = crystallized
	crystallizedHash, err := crystallized.Hash()
	if err != nil {
		t.Fatalf("Compute crystallized state hash failed: %v", err)
	}

	block = NewBlock(t, &pb.BeaconBlock{
		SlotNumber:            2,
		ActiveStateHash:       activeHash[:],
		CrystallizedStateHash: crystallizedHash[:],
		ParentHash:            parentHash[:],
	})

	// A properly initialize block should not fail.
	canProcess, err := beaconChain.CanProcessBlockValidator(&mockFetcher{}, block)
	if err != nil {
		t.Fatalf("CanProcessBlocks failed: %v", err)
	}
	if !canProcess {
		t.Error("Should be able to process block, could not")
	}

	// Test timestamp validity condition.
	block = NewBlock(t, &pb.BeaconBlock{
		SlotNumber:            1000000,
		ActiveStateHash:       activeHash[:],
		CrystallizedStateHash: crystallizedHash[:],
		ParentHash:            parentHash[:],
	})
	canProcess, err = beaconChain.CanProcessBlockValidator(&mockFetcher{}, block)
	if err != nil {
		t.Fatalf("CanProcessBlocks failed: %v", err)
	}
	if canProcess {
		t.Error("Should not be able to process block with invalid timestamp condition")
	}
}

func TestProcessBlockWithBadHashes(t *testing.T) {
	beaconChain, db := startInMemoryBeaconChain(t)
	defer db.Close()

	// Test negative scenario where active state hash is different than node's compute.
	parentBlock := NewBlock(t, nil)
	parentHash, err := parentBlock.Hash()
	if err != nil {
		t.Fatalf("Failed to compute parent block's hash: %v", err)
	}
	if err = db.DB().Put(parentHash[:], []byte{}); err != nil {
		t.Fatalf("Failed to put parent block on db: %v", err)
	}

	// Initialize state.
	active := types.NewActiveState(&pb.ActiveState{RecentBlockHashes: [][]byte{{'A'}}})
	activeStateHash, err := active.Hash()
	if err != nil {
		t.Fatalf("Cannot hash active state: %v", err)
	}

	crystallized := types.NewCrystallizedState(&pb.CrystallizedState{LastStateRecalc: 10000})
	crystallizedStateHash, err := crystallized.Hash()
	if err != nil {
		t.Fatalf("Cannot hash crystallized state: %v", err)
	}

	block := NewBlock(t, &pb.BeaconBlock{
		SlotNumber:            1,
		ActiveStateHash:       activeStateHash[:],
		CrystallizedStateHash: crystallizedStateHash[:],
		ParentHash:            parentHash[:],
	})

	// Test negative scenario where active state hash is different than node's compute.
	beaconChain.state.ActiveState = types.NewActiveState(&pb.ActiveState{RecentBlockHashes: [][]byte{{'B'}}})

	canProcess, err := beaconChain.CanProcessBlockValidator(&mockFetcher{}, block)
	if err == nil {
		t.Fatal("CanProcessBlocks should have failed with diff state hashes")
	}
	if canProcess {
		t.Error("CanProcessBlocks should have returned false")
	}

	// Test negative scenario where crystallized state hash is different than node's compute.
	beaconChain.state.CrystallizedState = types.NewCrystallizedState(&pb.CrystallizedState{LastStateRecalc: 9999})

	canProcess, err = beaconChain.CanProcessBlockValidator(&mockFetcher{}, block)
	if err == nil {
		t.Fatal("CanProcessBlocks should have failed with diff state hashes")
	}
	if canProcess {
		t.Error("CanProcessBlocks should have returned false")
	}
}

func TestProcessBlockWithInvalidParent(t *testing.T) {
	beaconChain, db := startInMemoryBeaconChain(t)
	defer db.Close()

	// If parent hash is non-existent, processing block should fail.
	active := types.NewActiveState(&pb.ActiveState{RecentBlockHashes: [][]byte{{'A'}}})
	activeStateHash, err := active.Hash()
	if err != nil {
		t.Fatalf("Cannot hash active state: %v", err)
	}
	beaconChain.state.ActiveState = active

	crystallized := types.NewCrystallizedState(&pb.CrystallizedState{LastStateRecalc: 10000})
	crystallizedStateHash, err := crystallized.Hash()
	if err != nil {
		t.Fatalf("Cannot hash crystallized state: %v", err)
	}
	beaconChain.state.CrystallizedState = crystallized

	// Test that block processing is invalid without a parent hash.
	block := NewBlock(t, &pb.BeaconBlock{
		SlotNumber:            2,
		ActiveStateHash:       activeStateHash[:],
		CrystallizedStateHash: crystallizedStateHash[:],
	})
	if _, err = beaconChain.CanProcessBlockValidator(&mockFetcher{}, block); err == nil {
		t.Error("Processing without a valid parent hash should fail")
	}

	// If parent hash is not stored in db, processing block should fail.
	parentBlock := NewBlock(t, &pb.BeaconBlock{
		SlotNumber: 1,
	})
	parentHash, err := parentBlock.Hash()
	if err != nil {
		t.Fatalf("Failed to compute parent block's hash: %v", err)
	}
	block = NewBlock(t, &pb.BeaconBlock{
		SlotNumber:            2,
		ActiveStateHash:       activeStateHash[:],
		CrystallizedStateHash: crystallizedStateHash[:],
		ParentHash:            parentHash[:],
	})

	if _, err := beaconChain.CanProcessBlockValidator(&mockFetcher{}, block); err == nil {
		t.Error("Processing block should fail when parent hash is not in db")
	}

	if err = db.DB().Put(parentHash[:], nil); err != nil {
		t.Fatalf("Failed to put parent block on db: %v", err)
	}
	_, err = beaconChain.CanProcessBlockValidator(&mockFetcher{}, block)
	if err == nil {
		t.Error("Processing block should fail when parent hash points to nil in db")
	}
	want := "unable to process block: parent hash points to nil in beaconDB"
	if err.Error() != want {
		t.Errorf("invalid log, expected \"%s\", got \"%s\"", want, err.Error())
	}

	if err = db.DB().Put(parentHash[:], []byte{}); err != nil {
		t.Fatalf("Failed to put parent block on db: %v", err)
	}

	canProcess, err := beaconChain.CanProcessBlockValidator(&mockFetcher{}, block)
	if err != nil {
		t.Errorf("Should have been able to process block: %v", err)
	}
	if !canProcess {
		t.Error("Should have been able to process block")
	}
}

func TestRotateValidatorSet(t *testing.T) {
	beaconChain, db := startInMemoryBeaconChain(t)
	defer db.Close()

	validators := []*pb.ValidatorRecord{
		{Balance: 10, StartDynasty: 0, EndDynasty: params.DefaultEndDynasty}, // half below default balance, should be moved to exit.
		{Balance: 15, StartDynasty: 1, EndDynasty: params.DefaultEndDynasty}, // half below default balance, should be moved to exit.
		{Balance: 20, StartDynasty: 2, EndDynasty: params.DefaultEndDynasty}, // stays in active.
		{Balance: 25, StartDynasty: 3, EndDynasty: params.DefaultEndDynasty}, // stays in active.
		{Balance: 30, StartDynasty: 4, EndDynasty: params.DefaultEndDynasty}, // stays in active.
	}

	data := &pb.CrystallizedState{
		Validators:     validators,
		CurrentDynasty: 10,
	}
	state := types.NewCrystallizedState(data)
	beaconChain.SetCrystallizedState(state)

	// rotate validator set and increment dynasty count by 1.
	beaconChain.rotateValidatorSet()
	beaconChain.CrystallizedState().IncrementCurrentDynasty()

	if !reflect.DeepEqual(beaconChain.activeValidatorIndices(), []int{2, 3, 4}) {
		t.Errorf("active validator indices should be [2,3,4], got: %v", beaconChain.activeValidatorIndices())
	}
	if len(beaconChain.queuedValidatorIndices()) != 0 {
		t.Errorf("queued validator indices should be [], got: %v", beaconChain.queuedValidatorIndices())
	}
	if !reflect.DeepEqual(beaconChain.exitedValidatorIndices(), []int{0, 1}) {
		t.Errorf("exited validator indices should be [0,1], got: %v", beaconChain.exitedValidatorIndices())
	}
}

func TestIsSlotTransition(t *testing.T) {
	beaconChain, db := startInMemoryBeaconChain(t)
	defer db.Close()

	if err := beaconChain.SetCrystallizedState(types.NewCrystallizedState(&pb.CrystallizedState{LastStateRecalc: params.CycleLength})); err != nil {
		t.Fatalf("unable to mutate crystallizedstate: %v", err)
	}
	if !beaconChain.IsEpochTransition(128) {
		t.Errorf("there was supposed to be an Slot transition but there isn't one now")
	}
	if beaconChain.IsEpochTransition(80) {
		t.Errorf("there is not supposed to be an Slot transition but there is one now")
	}
}

func TestHasVoted(t *testing.T) {
	beaconChain, db := startInMemoryBeaconChain(t)
	defer db.Close()

	// Setting bit field to 11111111.
	pendingAttestation := &pb.AttestationRecord{
		AttesterBitfield: []byte{255},
	}
	beaconChain.ActiveState().NewPendingAttestation(pendingAttestation)

	for i := 0; i < len(beaconChain.ActiveState().LatestPendingAttestation().AttesterBitfield); i++ {
		voted, err := utils.CheckBit(beaconChain.ActiveState().LatestPendingAttestation().AttesterBitfield, i)
		if err != nil {
			t.Errorf("checking bitfield for vote failed: %v", err)
		}
		if !voted {
			t.Error("validator voted but received didn't vote")
		}
	}

	// Setting bit field to 01010101.
	pendingAttestation = &pb.AttestationRecord{
		AttesterBitfield: []byte{85},
	}
	beaconChain.ActiveState().NewPendingAttestation(pendingAttestation)

	for i := 0; i < len(beaconChain.ActiveState().LatestPendingAttestation().AttesterBitfield); i++ {
		voted, err := utils.CheckBit(beaconChain.ActiveState().LatestPendingAttestation().AttesterBitfield, i)
		if err != nil {
			t.Errorf("checking bitfield for vote failed: %v", err)
		}
		if i%2 == 0 && voted {
			t.Error("validator didn't vote but received voted")
		}
		if i%2 == 1 && !voted {
			t.Error("validator voted but received didn't vote")
		}
	}
}

func TestClearAttesterBitfields(t *testing.T) {
	beaconChain, db := startInMemoryBeaconChain(t)
	defer db.Close()

	// Testing validator set sizes from 1 to 100.
	for j := 1; j <= 100; j++ {
		var validators []*pb.ValidatorRecord

		for i := 0; i < j; i++ {
			validator := &pb.ValidatorRecord{WithdrawalAddress: []byte{}, PublicKey: 0}
			validators = append(validators, validator)
		}

		testAttesterBitfield := []byte{1, 2, 3, 4}
		beaconChain.CrystallizedState().SetValidators(validators)
		beaconChain.ActiveState().NewPendingAttestation(&pb.AttestationRecord{AttesterBitfield: testAttesterBitfield})
		beaconChain.ActiveState().ClearPendingAttestations()

		if bytes.Equal(testAttesterBitfield, beaconChain.state.ActiveState.LatestPendingAttestation().AttesterBitfield) {
			t.Fatalf("attester bitfields have not been able to be reset: %v", testAttesterBitfield)
		}

		if !bytes.Equal(beaconChain.state.ActiveState.LatestPendingAttestation().AttesterBitfield, []byte{}) {
			t.Fatalf("attester bitfields are not zeroed out: %v", beaconChain.state.ActiveState.LatestPendingAttestation().AttesterBitfield)
		}
	}
}

func TestClearRecentBlockHashes(t *testing.T) {
	beaconChain, db := startInMemoryBeaconChain(t)
	defer db.Close()

	active := types.NewActiveState(&pb.ActiveState{RecentBlockHashes: [][]byte{{'A'}, {'B'}, {'C'}}})
	if err := beaconChain.SetActiveState(active); err != nil {
		t.Fatalf("unable to Mutate Active state: %v", err)
	}
	if reflect.DeepEqual(beaconChain.state.ActiveState.RecentBlockHashes(), [][]byte{{'A'}, {'B'}, {'C'}}) {
		t.Fatalf("recent block hash was not saved: %d", beaconChain.state.ActiveState.RecentBlockHashes())
	}
	beaconChain.ActiveState().ClearRecentBlockHashes()
	if reflect.DeepEqual(beaconChain.state.ActiveState.RecentBlockHashes(), [][]byte{}) {
		t.Fatalf("attester deposit was not able to be reset: %d", beaconChain.state.ActiveState.RecentBlockHashes())
	}
}

func TestUpdateJustifiedSlot(t *testing.T) {
	beaconChain, db := startInMemoryBeaconChain(t)
	defer db.Close()

	data := &pb.CrystallizedState{LastStateRecalc: 5 * params.CycleLength, LastJustifiedSlot: 4, LastFinalizedSlot: 3}
	beaconChain.SetCrystallizedState(types.NewCrystallizedState(data))
	if beaconChain.state.CrystallizedState.LastFinalizedSlot() != uint64(3) ||
		beaconChain.state.CrystallizedState.LastJustifiedSlot() != uint64(4) {
		t.Fatal("crystallized state unable to be saved")
	}

	beaconChain.state.CrystallizedState.UpdateJustifiedSlot(5)

	if beaconChain.state.CrystallizedState.LastJustifiedSlot() != uint64(5) {
		t.Fatalf("unable to update last justified Slot: %d", beaconChain.state.CrystallizedState.LastJustifiedSlot())
	}
	if beaconChain.state.CrystallizedState.LastFinalizedSlot() != uint64(4) {
		t.Fatalf("unable to update last finalized Slot: %d", beaconChain.state.CrystallizedState.LastFinalizedSlot())
	}

	data = &pb.CrystallizedState{LastStateRecalc: 8 * params.CycleLength, LastJustifiedSlot: 4, LastFinalizedSlot: 3}
	beaconChain.SetCrystallizedState(types.NewCrystallizedState(data))

	if beaconChain.state.CrystallizedState.LastFinalizedSlot() != uint64(3) ||
		beaconChain.state.CrystallizedState.LastJustifiedSlot() != uint64(4) {
		t.Fatal("crystallized state unable to be saved")
	}

	beaconChain.state.CrystallizedState.UpdateJustifiedSlot(8)

	if beaconChain.state.CrystallizedState.LastJustifiedSlot() != uint64(8) {
		t.Fatalf("unable to update last justified Slot: %d", beaconChain.state.CrystallizedState.LastJustifiedSlot())
	}
	if beaconChain.state.CrystallizedState.LastFinalizedSlot() != uint64(3) {
		t.Fatalf("unable to update last finalized Slot: %d", beaconChain.state.CrystallizedState.LastFinalizedSlot())
	}
}

func TestComputeValidatorRewardsAndPenalties(t *testing.T) {
	beaconChain, db := startInMemoryBeaconChain(t)
	defer db.Close()

	var validators []*pb.ValidatorRecord
	for i := 0; i < 40; i++ {
		validator := &pb.ValidatorRecord{Balance: 32, StartDynasty: 1, EndDynasty: 10}
		validators = append(validators, validator)
	}

	block := NewBlock(t, &pb.BeaconBlock{
		SlotNumber: 5,
	})

	data := &pb.CrystallizedState{
		Validators:        validators,
		CurrentDynasty:    1,
		TotalDeposits:     100,
		LastJustifiedSlot: 4,
		LastFinalizedSlot: 3,
	}
	if err := beaconChain.SetCrystallizedState(types.NewCrystallizedState(data)); err != nil {
		t.Fatalf("unable to mutate crystallizedstate: %v", err)
	}

	//Binary representation of bitfield: 11001000 10010100 10010010 10110011 00110001
	testAttesterBitfield := []byte{200, 148, 146, 179, 49}
	state := types.NewActiveState(&pb.ActiveState{PendingAttestations: []*pb.AttestationRecord{{AttesterBitfield: testAttesterBitfield}}})
	if err := beaconChain.SetActiveState(state); err != nil {
		t.Fatalf("unable to Mutate Active state: %v", err)
	}
	if err := beaconChain.calculateRewardsFFG(block); err != nil {
		t.Fatalf("could not compute validator rewards and penalties: %v", err)
	}
	if beaconChain.state.CrystallizedState.LastJustifiedSlot() != uint64(5) {
		t.Fatalf("unable to update last justified Slot: %d", beaconChain.state.CrystallizedState.LastJustifiedSlot())
	}
	if beaconChain.state.CrystallizedState.LastFinalizedSlot() != uint64(4) {
		t.Fatalf("unable to update last finalized Slot: %d", beaconChain.state.CrystallizedState.LastFinalizedSlot())
	}
	if beaconChain.CrystallizedState().Validators()[0].Balance != uint64(33) {
		t.Fatalf("validator balance not updated: %d", beaconChain.CrystallizedState().Validators()[1].Balance)
	}
	if beaconChain.CrystallizedState().Validators()[7].Balance != uint64(31) {
		t.Fatalf("validator balance not updated: %d", beaconChain.CrystallizedState().Validators()[1].Balance)
	}
	if beaconChain.CrystallizedState().Validators()[29].Balance != uint64(31) {
		t.Fatalf("validator balance not updated: %d", beaconChain.CrystallizedState().Validators()[1].Balance)
	}
}

func TestValidatorIndices(t *testing.T) {
	beaconChain, db := startInMemoryBeaconChain(t)
	defer db.Close()

	data := &pb.CrystallizedState{
		Validators: []*pb.ValidatorRecord{
			{PublicKey: 0, StartDynasty: 0, EndDynasty: 2},                   // active.
			{PublicKey: 0, StartDynasty: 0, EndDynasty: 2},                   // active.
			{PublicKey: 0, StartDynasty: 1, EndDynasty: 2},                   // active.
			{PublicKey: 0, StartDynasty: 0, EndDynasty: 2},                   // active.
			{PublicKey: 0, StartDynasty: 0, EndDynasty: 3},                   // active.
			{PublicKey: 0, StartDynasty: 2, EndDynasty: uint64(math.Inf(0))}, // queued.
		},
		CurrentDynasty: 1,
	}

	crystallized := types.NewCrystallizedState(data)
	if err := beaconChain.SetCrystallizedState(crystallized); err != nil {
		t.Fatalf("unable to mutate crystallized state: %v", err)
	}

	if !reflect.DeepEqual(beaconChain.activeValidatorIndices(), []int{0, 1, 2, 3, 4}) {
		t.Errorf("active validator indices should be [0 1 2 3 4], got: %v", beaconChain.activeValidatorIndices())
	}
	if !reflect.DeepEqual(beaconChain.queuedValidatorIndices(), []int{5}) {
		t.Errorf("queued validator indices should be [5], got: %v", beaconChain.queuedValidatorIndices())
	}
	if len(beaconChain.exitedValidatorIndices()) != 0 {
		t.Errorf("exited validator indices to be empty, got: %v", beaconChain.exitedValidatorIndices())
	}

	data = &pb.CrystallizedState{
		Validators: []*pb.ValidatorRecord{
			{PublicKey: 0, StartDynasty: 1, EndDynasty: uint64(math.Inf(0))}, // active.
			{PublicKey: 0, StartDynasty: 2, EndDynasty: uint64(math.Inf(0))}, // active.
			{PublicKey: 0, StartDynasty: 6, EndDynasty: uint64(math.Inf(0))}, // queued.
			{PublicKey: 0, StartDynasty: 7, EndDynasty: uint64(math.Inf(0))}, // queued.
			{PublicKey: 0, StartDynasty: 1, EndDynasty: 2},                   // exited.
			{PublicKey: 0, StartDynasty: 1, EndDynasty: 3},                   // exited.
		},
		CurrentDynasty: 5,
	}

	crystallized = types.NewCrystallizedState(data)
	if err := beaconChain.SetCrystallizedState(crystallized); err != nil {
		t.Fatalf("unable to mutate crystallized state: %v", err)
	}

	if !reflect.DeepEqual(beaconChain.activeValidatorIndices(), []int{0, 1}) {
		t.Errorf("active validator indices should be [0 1 2 4 5], got: %v", beaconChain.activeValidatorIndices())
	}
	if !reflect.DeepEqual(beaconChain.queuedValidatorIndices(), []int{2, 3}) {
		t.Errorf("queued validator indices should be [3], got: %v", beaconChain.queuedValidatorIndices())
	}
	if !reflect.DeepEqual(beaconChain.exitedValidatorIndices(), []int{4, 5}) {
		t.Errorf("exited validator indices should be [3], got: %v", beaconChain.exitedValidatorIndices())
	}
}

func TestCanProcessBlockObserver(t *testing.T) {
	beaconChain, db := startInMemoryBeaconChain(t)
	defer db.Close()

	clock = &fakeClock{}

	// Initialize a parent block.
	parentBlock := NewBlock(t, &pb.BeaconBlock{
		SlotNumber: 1,
	})
	parentHash, err := parentBlock.Hash()
	if err != nil {
		t.Fatalf("Failed to compute parent block's hash: %v", err)
	}
	if err = db.DB().Put(parentHash[:], []byte{}); err != nil {
		t.Fatalf("Failed to put parent block on db: %v", err)
	}

	// Using a faulty fetcher should throw an error.
	block := NewBlock(t, &pb.BeaconBlock{
		SlotNumber: 2,
	})

	// Initialize initial state.
	activeState := types.NewActiveState(&pb.ActiveState{RecentBlockHashes: [][]byte{{'A'}}})
	beaconChain.state.ActiveState = activeState
	activeHash, err := activeState.Hash()
	if err != nil {
		t.Fatalf("Cannot hash active state: %v", err)
	}

	crystallized := types.NewCrystallizedState(&pb.CrystallizedState{})
	beaconChain.state.CrystallizedState = crystallized
	crystallizedHash, err := crystallized.Hash()
	if err != nil {
		t.Fatalf("Compute crystallized state hash failed: %v", err)
	}

	block = NewBlock(t, &pb.BeaconBlock{
		SlotNumber:            2,
		ActiveStateHash:       activeHash[:],
		CrystallizedStateHash: crystallizedHash[:],
		ParentHash:            parentHash[:],
	})

	// A properly initialize block should not fail.
	canProcess, err := beaconChain.CanProcessBlockObserver(block)
	if err != nil {
		t.Fatalf("CanProcessBlocks failed: %v", err)
	}
	if !canProcess {
		t.Error("Should be able to process block, could not")
	}

	block = NewBlock(t, &pb.BeaconBlock{
		SlotNumber:            2,
		ActiveStateHash:       activeHash[:],
		CrystallizedStateHash: []byte{'A'},
		ParentHash:            parentHash[:],
	})
	canProcess, err = beaconChain.CanProcessBlockObserver(block)
	if err != nil {
		t.Fatalf("CanProcessBlocks failed: %v", err)
	}
	if canProcess {
		t.Error("Should not be able to process block with invalid crystallized hash")
	}

	// Test timestamp validity condition.
	block = NewBlock(t, &pb.BeaconBlock{
		SlotNumber:            1000000,
		ActiveStateHash:       activeHash[:],
		CrystallizedStateHash: crystallizedHash[:],
		ParentHash:            parentHash[:],
	})
	canProcess, err = beaconChain.CanProcessBlockObserver(block)
	if err != nil {
		t.Fatalf("CanProcessBlocks failed: %v", err)
	}
	if canProcess {
		t.Error("Should not be able to process block with invalid timestamp condition")
	}
}

// NewBlock is a helper method to create blocks with valid defaults.
// For a generic block, use NewBlock(t, nil).
func NewBlock(t *testing.T, b *pb.BeaconBlock) *types.Block {
	if b == nil {
		b = &pb.BeaconBlock{}
	}
	if b.ActiveStateHash == nil {
		b.ActiveStateHash = make([]byte, 32)
	}
	if b.CrystallizedStateHash == nil {
		b.CrystallizedStateHash = make([]byte, 32)
	}
	if b.ParentHash == nil {
		b.ParentHash = make([]byte, 32)
	}

	blk, err := types.NewBlock(b)
	if err != nil {
		t.Fatalf("failed to instantiate block with slot number: %v", err)
	}
	return blk
}
