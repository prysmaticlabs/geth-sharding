package epoch

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	"github.com/prysmaticlabs/prysm/shared/featureconfig"
	"github.com/prysmaticlabs/prysm/shared/params"
)

func init() {
	featureconfig.InitFeatureConfig(&featureconfig.FeatureFlagConfig{
		EnableCrosslinks: true,
	})
	helpers.ClearShuffledValidatorCache()
}

func TestUnslashedAttestingIndices_CanSortAndFilter(t *testing.T) {
	// Generate 2 attestations.
	atts := make([]*pb.PendingAttestation, 2)
	for i := 0; i < len(atts); i++ {
		atts[i] = &pb.PendingAttestation{
			Data: &pb.AttestationData{
				TargetEpoch: 0,
				Crosslink: &pb.Crosslink{
					Shard: uint64(i),
				},
			},
			AggregationBits: []byte{0xC0, 0xC0},
		}
	}

	// Generate validators and state for the 2 attestations.
	validators := make([]*pb.Validator, params.BeaconConfig().DepositsForChainStart/16)
	for i := 0; i < len(validators); i++ {
		validators[i] = &pb.Validator{
			ExitEpoch: params.BeaconConfig().FarFutureEpoch,
		}
	}
	state := &pb.BeaconState{
		Slot:             0,
		Validators:       validators,
		RandaoMixes:      make([][]byte, params.BeaconConfig().EpochsPerHistoricalVector),
		ActiveIndexRoots: make([][]byte, params.BeaconConfig().EpochsPerHistoricalVector),
	}

	indices, err := unslashedAttestingIndices(state, atts)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < len(indices)-1; i++ {
		if indices[i] > indices[i+1] {
			t.Error("sorted indices not sorted")
		}
	}

	// Verify the slashed validator is filtered.
	slashedValidator := indices[0]
	state.Validators[slashedValidator].Slashed = true
	indices, err = unslashedAttestingIndices(state, atts)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < len(indices); i++ {
		if indices[i] == slashedValidator {
			t.Errorf("Slashed validator %d is not filtered", slashedValidator)
		}
	}
}

func TestUnslashedAttestingIndices_CantGetIndicesBitfieldError(t *testing.T) {
	atts := make([]*pb.PendingAttestation, 2)
	for i := 0; i < len(atts); i++ {
		atts[i] = &pb.PendingAttestation{
			Data: &pb.AttestationData{
				TargetEpoch: 0,
				Crosslink: &pb.Crosslink{
					Shard: uint64(i),
				},
			},
			AggregationBits: []byte{0xff},
		}
	}

	state := &pb.BeaconState{
		Slot:             0,
		RandaoMixes:      make([][]byte, params.BeaconConfig().EpochsPerHistoricalVector),
		ActiveIndexRoots: make([][]byte, params.BeaconConfig().EpochsPerHistoricalVector),
	}
	const wantedErr = "could not get attester indices: wanted participants bitfield length 2, got: 1"
	if _, err := unslashedAttestingIndices(state, atts); !strings.Contains(err.Error(), wantedErr) {
		t.Errorf("wanted: %v, got: %v", wantedErr, err.Error())
	}
}

func TestAttestingBalance_CorrectBalance(t *testing.T) {
	helpers.ClearAllCaches()

	// Generate 2 attestations.
	atts := make([]*pb.PendingAttestation, 2)
	for i := 0; i < len(atts); i++ {
		atts[i] = &pb.PendingAttestation{
			Data: &pb.AttestationData{
				Crosslink: &pb.Crosslink{
					Shard: uint64(i),
				},
			},
			AggregationBits: []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
				0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
		}
	}

	// Generate validators with balances and state for the 2 attestations.
	validators := make([]*pb.Validator, params.BeaconConfig().DepositsForChainStart)
	balances := make([]uint64, params.BeaconConfig().DepositsForChainStart)
	for i := 0; i < len(validators); i++ {
		validators[i] = &pb.Validator{
			ExitEpoch:        params.BeaconConfig().FarFutureEpoch,
			EffectiveBalance: params.BeaconConfig().MaxEffectiveBalance,
		}
		balances[i] = params.BeaconConfig().MaxEffectiveBalance
	}
	state := &pb.BeaconState{
		Slot:             0,
		RandaoMixes:      make([][]byte, params.BeaconConfig().EpochsPerHistoricalVector),
		ActiveIndexRoots: make([][]byte, params.BeaconConfig().EpochsPerHistoricalVector),
		Validators:       validators,
		Balances:         balances,
	}

	balance, err := AttestingBalance(state, atts)
	if err != nil {
		t.Fatal(err)
	}
	wanted := 256 * params.BeaconConfig().MaxEffectiveBalance
	if balance != wanted {
		t.Errorf("wanted balance: %d, got: %d", wanted, balance)
	}
}

func TestAttestingBalance_CantGetIndicesBitfieldError(t *testing.T) {
	helpers.ClearAllCaches()

	atts := make([]*pb.PendingAttestation, 2)
	for i := 0; i < len(atts); i++ {
		atts[i] = &pb.PendingAttestation{
			Data: &pb.AttestationData{
				TargetEpoch: 0,
				Crosslink: &pb.Crosslink{
					Shard: uint64(i),
				},
			},
			AggregationBits: []byte{0xFF},
		}
	}

	state := &pb.BeaconState{
		Slot:             0,
		RandaoMixes:      make([][]byte, params.BeaconConfig().EpochsPerHistoricalVector),
		ActiveIndexRoots: make([][]byte, params.BeaconConfig().EpochsPerHistoricalVector),
	}
	const wantedErr = "could not get attester indices: wanted participants bitfield length 0, got: 1"
	if _, err := AttestingBalance(state, atts); !strings.Contains(err.Error(), wantedErr) {
		t.Errorf("wanted: %v, got: %v", wantedErr, err.Error())
	}
}

func TestMatchAttestations_PrevEpoch(t *testing.T) {
	helpers.ClearAllCaches()
	e := params.BeaconConfig().SlotsPerEpoch
	s := uint64(0) // slot

	// The correct epoch for source is the first epoch
	// The correct vote for target is '1'
	// The correct vote for head is '2'
	prevAtts := []*pb.PendingAttestation{
		{Data: &pb.AttestationData{Crosslink: &pb.Crosslink{Shard: s + 1}}},                                                     // source
		{Data: &pb.AttestationData{Crosslink: &pb.Crosslink{Shard: s + 1}, TargetRoot: []byte{1}}},                              // source, target
		{Data: &pb.AttestationData{Crosslink: &pb.Crosslink{Shard: s + 1}, TargetRoot: []byte{3}}},                              // source
		{Data: &pb.AttestationData{Crosslink: &pb.Crosslink{Shard: s + 1}, TargetRoot: []byte{1}}},                              // source, target
		{Data: &pb.AttestationData{Crosslink: &pb.Crosslink{Shard: s + 1}, BeaconBlockRoot: []byte{66}}},                        // source, head
		{Data: &pb.AttestationData{Crosslink: &pb.Crosslink{Shard: s + 1}, BeaconBlockRoot: []byte{4}}},                         // source
		{Data: &pb.AttestationData{Crosslink: &pb.Crosslink{Shard: s + 1}, BeaconBlockRoot: []byte{66}, TargetRoot: []byte{1}}}, // source, target, head
		{Data: &pb.AttestationData{Crosslink: &pb.Crosslink{Shard: s + 1}, BeaconBlockRoot: []byte{5}, TargetRoot: []byte{1}}},  // source, target
		{Data: &pb.AttestationData{Crosslink: &pb.Crosslink{Shard: s + 1}, BeaconBlockRoot: []byte{66}, TargetRoot: []byte{6}}}, // source, head
	}

	currentAtts := []*pb.PendingAttestation{
		{Data: &pb.AttestationData{Crosslink: &pb.Crosslink{Shard: s + e + 1}}},                                                    // none
		{Data: &pb.AttestationData{Crosslink: &pb.Crosslink{Shard: s + e + 1}, BeaconBlockRoot: []byte{2}, TargetRoot: []byte{1}}}, // none
	}

	blockRoots := make([][]byte, 128)
	for i := 0; i < len(blockRoots); i++ {
		blockRoots[i] = []byte{byte(i + 1)}
	}
	state := &pb.BeaconState{
		Slot:                      s + e + 2,
		CurrentEpochAttestations:  currentAtts,
		PreviousEpochAttestations: prevAtts,
		BlockRoots:                blockRoots,
		RandaoMixes:               make([][]byte, params.BeaconConfig().EpochsPerHistoricalVector),
		ActiveIndexRoots:          make([][]byte, params.BeaconConfig().EpochsPerHistoricalVector),
	}

	mAtts, err := MatchAttestations(state, 0)
	if err != nil {
		t.Fatal(err)
	}

	wantedSrcAtts := []*pb.PendingAttestation{
		{Data: &pb.AttestationData{Crosslink: &pb.Crosslink{Shard: s + 1}}},
		{Data: &pb.AttestationData{Crosslink: &pb.Crosslink{Shard: s + 1}, TargetRoot: []byte{1}}},
		{Data: &pb.AttestationData{Crosslink: &pb.Crosslink{Shard: s + 1}, TargetRoot: []byte{3}}},
		{Data: &pb.AttestationData{Crosslink: &pb.Crosslink{Shard: s + 1}, TargetRoot: []byte{1}}},
		{Data: &pb.AttestationData{Crosslink: &pb.Crosslink{Shard: s + 1}, BeaconBlockRoot: []byte{66}}},
		{Data: &pb.AttestationData{Crosslink: &pb.Crosslink{Shard: s + 1}, BeaconBlockRoot: []byte{4}}},
		{Data: &pb.AttestationData{Crosslink: &pb.Crosslink{Shard: s + 1}, BeaconBlockRoot: []byte{66}, TargetRoot: []byte{1}}},
		{Data: &pb.AttestationData{Crosslink: &pb.Crosslink{Shard: s + 1}, BeaconBlockRoot: []byte{5}, TargetRoot: []byte{1}}},
		{Data: &pb.AttestationData{Crosslink: &pb.Crosslink{Shard: s + 1}, BeaconBlockRoot: []byte{66}, TargetRoot: []byte{6}}},
	}
	if !reflect.DeepEqual(mAtts.source, wantedSrcAtts) {
		t.Error("source attestations don't match")
	}

	wantedTgtAtts := []*pb.PendingAttestation{
		{Data: &pb.AttestationData{Crosslink: &pb.Crosslink{Shard: s + 1}, TargetRoot: []byte{1}}},
		{Data: &pb.AttestationData{Crosslink: &pb.Crosslink{Shard: s + 1}, TargetRoot: []byte{1}}},
		{Data: &pb.AttestationData{Crosslink: &pb.Crosslink{Shard: s + 1}, BeaconBlockRoot: []byte{66}, TargetRoot: []byte{1}}},
		{Data: &pb.AttestationData{Crosslink: &pb.Crosslink{Shard: s + 1}, BeaconBlockRoot: []byte{5}, TargetRoot: []byte{1}}},
	}
	if !reflect.DeepEqual(mAtts.Target, wantedTgtAtts) {
		t.Error("target attestations don't match")
	}

	wantedHeadAtts := []*pb.PendingAttestation{
		{Data: &pb.AttestationData{Crosslink: &pb.Crosslink{Shard: s + 1}, BeaconBlockRoot: []byte{66}}},
		{Data: &pb.AttestationData{Crosslink: &pb.Crosslink{Shard: s + 1}, BeaconBlockRoot: []byte{66}, TargetRoot: []byte{1}}},
		{Data: &pb.AttestationData{Crosslink: &pb.Crosslink{Shard: s + 1}, BeaconBlockRoot: []byte{66}, TargetRoot: []byte{6}}},
	}
	if !reflect.DeepEqual(mAtts.head, wantedHeadAtts) {
		t.Error("head attestations don't match")
	}
}

func TestMatchAttestations_CurrentEpoch(t *testing.T) {
	helpers.ClearAllCaches()
	e := params.BeaconConfig().SlotsPerEpoch
	s := uint64(0) // slot

	// The correct epoch for source is the first epoch
	// The correct vote for target is '65'
	// The correct vote for head is '66'
	prevAtts := []*pb.PendingAttestation{
		{Data: &pb.AttestationData{Crosslink: &pb.Crosslink{Shard: s + 1}}},                                                    // none
		{Data: &pb.AttestationData{Crosslink: &pb.Crosslink{Shard: s + 1}, BeaconBlockRoot: []byte{2}, TargetRoot: []byte{1}}}, // none
		{Data: &pb.AttestationData{Crosslink: &pb.Crosslink{Shard: s + 1}, BeaconBlockRoot: []byte{5}, TargetRoot: []byte{1}}}, // none
		{Data: &pb.AttestationData{Crosslink: &pb.Crosslink{Shard: s + 1}, BeaconBlockRoot: []byte{2}, TargetRoot: []byte{6}}}, // none
	}

	currentAtts := []*pb.PendingAttestation{
		{Data: &pb.AttestationData{Crosslink: &pb.Crosslink{Shard: s + 1}}},                                                      // source
		{Data: &pb.AttestationData{Crosslink: &pb.Crosslink{Shard: s + 1}, BeaconBlockRoot: []byte{66}, TargetRoot: []byte{65}}}, // source, target, head
		{Data: &pb.AttestationData{Crosslink: &pb.Crosslink{Shard: s + 1}, BeaconBlockRoot: []byte{69}, TargetRoot: []byte{65}}}, // source, target
		{Data: &pb.AttestationData{Crosslink: &pb.Crosslink{Shard: s + 1}, BeaconBlockRoot: []byte{66}, TargetRoot: []byte{68}}}, // source, head
	}

	blockRoots := make([][]byte, 128)
	for i := 0; i < len(blockRoots); i++ {
		blockRoots[i] = []byte{byte(i + 1)}
	}
	state := &pb.BeaconState{
		Slot:                      s + e + 2,
		CurrentEpochAttestations:  currentAtts,
		PreviousEpochAttestations: prevAtts,
		BlockRoots:                blockRoots,
	}

	mAtts, err := MatchAttestations(state, 1)
	if err != nil {
		t.Fatal(err)
	}

	wantedSrcAtts := []*pb.PendingAttestation{
		{Data: &pb.AttestationData{Crosslink: &pb.Crosslink{Shard: s + 1}}},
		{Data: &pb.AttestationData{Crosslink: &pb.Crosslink{Shard: s + 1}, BeaconBlockRoot: []byte{66}, TargetRoot: []byte{65}}},
		{Data: &pb.AttestationData{Crosslink: &pb.Crosslink{Shard: s + 1}, BeaconBlockRoot: []byte{69}, TargetRoot: []byte{65}}},
		{Data: &pb.AttestationData{Crosslink: &pb.Crosslink{Shard: s + 1}, BeaconBlockRoot: []byte{66}, TargetRoot: []byte{68}}},
	}
	if !reflect.DeepEqual(mAtts.source, wantedSrcAtts) {
		t.Error("source attestations don't match")
	}

	wantedTgtAtts := []*pb.PendingAttestation{
		{Data: &pb.AttestationData{Crosslink: &pb.Crosslink{Shard: s + 1}, BeaconBlockRoot: []byte{66}, TargetRoot: []byte{65}}},
		{Data: &pb.AttestationData{Crosslink: &pb.Crosslink{Shard: s + 1}, BeaconBlockRoot: []byte{69}, TargetRoot: []byte{65}}},
	}
	if !reflect.DeepEqual(mAtts.Target, wantedTgtAtts) {
		t.Error("target attestations don't match")
	}

	wantedHeadAtts := []*pb.PendingAttestation{
		{Data: &pb.AttestationData{Crosslink: &pb.Crosslink{Shard: s + 1}, BeaconBlockRoot: []byte{66}, TargetRoot: []byte{65}}},
		{Data: &pb.AttestationData{Crosslink: &pb.Crosslink{Shard: s + 1}, BeaconBlockRoot: []byte{66}, TargetRoot: []byte{68}}},
	}
	if !reflect.DeepEqual(mAtts.head, wantedHeadAtts) {
		t.Error("head attestations don't match")
	}
}

func TestMatchAttestations_EpochOutOfBound(t *testing.T) {
	_, err := MatchAttestations(&pb.BeaconState{Slot: 1}, 2 /* epoch */)
	if !strings.Contains(err.Error(), "input epoch: 2 != current epoch: 0") {
		t.Fatal("Did not receive wanted error")
	}
}

func TestAttsForCrosslink_CanGetAttestations(t *testing.T) {
	c := &pb.Crosslink{
		DataRoot: []byte{'B'},
	}
	atts := []*pb.PendingAttestation{
		{Data: &pb.AttestationData{Crosslink: &pb.Crosslink{DataRoot: []byte{'A'}}}},
		{Data: &pb.AttestationData{Crosslink: &pb.Crosslink{DataRoot: []byte{'B'}}}}, // Selected
		{Data: &pb.AttestationData{Crosslink: &pb.Crosslink{DataRoot: []byte{'C'}}}},
		{Data: &pb.AttestationData{Crosslink: &pb.Crosslink{DataRoot: []byte{'B'}}}}} // Selected

	if !reflect.DeepEqual(attsForCrosslink(c, atts), []*pb.PendingAttestation{
		{Data: &pb.AttestationData{Crosslink: &pb.Crosslink{DataRoot: []byte{'B'}}}},
		{Data: &pb.AttestationData{Crosslink: &pb.Crosslink{DataRoot: []byte{'B'}}}}}) {
		t.Error("Incorrect attestations for crosslink")
	}
}

func TestWinningCrosslink_CantGetMatchingAtts(t *testing.T) {
	wanted := fmt.Sprintf("could not get matching attestations: input epoch: %d != current epoch: %d or previous epoch: %d",
		100, 0, 0)
	_, _, err := winningCrosslink(&pb.BeaconState{Slot: 0}, 0, 100)
	if err.Error() != wanted {
		t.Fatal(err)
	}
}

func TestWinningCrosslink_ReturnGensisCrosslink(t *testing.T) {
	e := params.BeaconConfig().SlotsPerEpoch
	gs := uint64(0) // genesis slot
	ge := uint64(0) // genesis epoch

	state := &pb.BeaconState{
		Slot:                      gs + e + 2,
		PreviousEpochAttestations: []*pb.PendingAttestation{},
		BlockRoots:                make([][]byte, 128),
		CurrentCrosslinks:         []*pb.Crosslink{{StartEpoch: ge}},
	}

	gCrosslink := &pb.Crosslink{
		StartEpoch: 0,
		DataRoot:   params.BeaconConfig().ZeroHash[:],
		ParentRoot: params.BeaconConfig().ZeroHash[:],
	}

	crosslink, indices, err := winningCrosslink(state, 0, ge)
	if err != nil {
		t.Fatal(err)
	}
	if len(indices) != 0 {
		t.Errorf("gensis crosslink indices is not 0, got: %d", len(indices))
	}
	if !reflect.DeepEqual(crosslink, gCrosslink) {
		t.Errorf("Did not get genesis crosslink, got: %v", crosslink)
	}
}

func TestWinningCrosslink_CanGetWinningRoot(t *testing.T) {
	helpers.ClearAllCaches()
	e := params.BeaconConfig().SlotsPerEpoch
	gs := uint64(0) // genesis slot
	ge := uint64(0) // genesis epoch

	atts := []*pb.PendingAttestation{
		{
			Data: &pb.AttestationData{
				Crosslink: &pb.Crosslink{
					Shard:    1,
					DataRoot: []byte{'A'},
				},
			},
		},
		{
			Data: &pb.AttestationData{
				Crosslink: &pb.Crosslink{
					Shard:    1,
					DataRoot: []byte{'B'}, // Winner
				},
			},
		},
		{
			Data: &pb.AttestationData{
				Crosslink: &pb.Crosslink{
					Shard:    1,
					DataRoot: []byte{'C'},
				},
			},
		},
	}

	blockRoots := make([][]byte, 128)
	for i := 0; i < len(blockRoots); i++ {
		blockRoots[i] = []byte{byte(i + 1)}
	}

	crosslinks := make([]*pb.Crosslink, params.BeaconConfig().ShardCount)
	for i := uint64(0); i < params.BeaconConfig().ShardCount; i++ {
		crosslinks[i] = &pb.Crosslink{
			StartEpoch: ge,
			Shard:      1,
			DataRoot:   []byte{'B'},
		}
	}
	state := &pb.BeaconState{
		Slot:                      gs + e + 2,
		PreviousEpochAttestations: atts,
		BlockRoots:                blockRoots,
		CurrentCrosslinks:         crosslinks,
		RandaoMixes:               make([][]byte, params.BeaconConfig().EpochsPerHistoricalVector),
		ActiveIndexRoots:          make([][]byte, params.BeaconConfig().EpochsPerHistoricalVector),
	}

	winner, indices, err := winningCrosslink(state, 1, ge)
	if err != nil {
		t.Fatal(err)
	}
	if len(indices) != 0 {
		t.Errorf("gensis crosslink indices is not 0, got: %d", len(indices))
	}
	want := &pb.Crosslink{StartEpoch: ge, Shard: 1, DataRoot: []byte{'B'}}
	if !reflect.DeepEqual(winner, want) {
		t.Errorf("Did not get wanted crosslink, got: %v", winner)
	}
}

func TestProcessCrosslinks_NoUpdate(t *testing.T) {
	helpers.ClearAllCaches()

	validatorCount := 128
	validators := make([]*pb.Validator, validatorCount)
	balances := make([]uint64, validatorCount)
	for i := 0; i < len(validators); i++ {
		validators[i] = &pb.Validator{
			ExitEpoch:        params.BeaconConfig().FarFutureEpoch,
			EffectiveBalance: params.BeaconConfig().MaxEffectiveBalance,
		}
		balances[i] = params.BeaconConfig().MaxEffectiveBalance
	}
	blockRoots := make([][]byte, 128)
	for i := 0; i < len(blockRoots); i++ {
		blockRoots[i] = []byte{byte(i + 1)}
	}

	var crosslinks []*pb.Crosslink
	for i := uint64(0); i < params.BeaconConfig().ShardCount; i++ {
		crosslinks = append(crosslinks, &pb.Crosslink{
			StartEpoch: 0,
			DataRoot:   []byte{'A'},
		})
	}
	state := &pb.BeaconState{
		Slot:              params.BeaconConfig().SlotsPerEpoch + 1,
		Validators:        validators,
		Balances:          balances,
		BlockRoots:        blockRoots,
		RandaoMixes:       make([][]byte, params.BeaconConfig().EpochsPerHistoricalVector),
		ActiveIndexRoots:  make([][]byte, params.BeaconConfig().EpochsPerHistoricalVector),
		CurrentCrosslinks: crosslinks,
	}
	newState, err := ProcessCrosslinks(state)
	if err != nil {
		t.Fatal(err)
	}

	wanted := &pb.Crosslink{
		StartEpoch: 0,
		DataRoot:   []byte{'A'},
	}
	// Since there has been no attestation, crosslink stayed the same.
	if !reflect.DeepEqual(wanted, newState.CurrentCrosslinks[0]) {
		t.Errorf("Did not get correct crosslink back")
	}
}

func TestProcessCrosslinks_SuccessfulUpdate(t *testing.T) {
	e := params.BeaconConfig().SlotsPerEpoch
	gs := uint64(0) // genesis slot
	ge := uint64(0) // genesis epoch

	validators := make([]*pb.Validator, params.BeaconConfig().DepositsForChainStart/8)
	balances := make([]uint64, params.BeaconConfig().DepositsForChainStart/8)
	for i := 0; i < len(validators); i++ {
		validators[i] = &pb.Validator{
			ExitEpoch:        params.BeaconConfig().FarFutureEpoch,
			EffectiveBalance: params.BeaconConfig().MaxEffectiveBalance,
		}
		balances[i] = params.BeaconConfig().MaxEffectiveBalance
	}
	blockRoots := make([][]byte, 128)
	for i := 0; i < len(blockRoots); i++ {
		blockRoots[i] = []byte{byte(i + 1)}
	}

	crosslinks := make([]*pb.Crosslink, params.BeaconConfig().ShardCount)
	for i := uint64(0); i < params.BeaconConfig().ShardCount; i++ {
		crosslinks[i] = &pb.Crosslink{
			StartEpoch: ge,
			DataRoot:   []byte{'B'},
		}
	}
	var atts []*pb.PendingAttestation
	startShard := uint64(960)
	for s := uint64(0); s < params.BeaconConfig().SlotsPerEpoch; s++ {
		atts = append(atts, &pb.PendingAttestation{
			Data: &pb.AttestationData{
				Crosslink: &pb.Crosslink{
					Shard:    startShard + s,
					DataRoot: []byte{'B'},
				},
				TargetEpoch: 0,
			},
			AggregationBits: []byte{0xC0, 0xC0, 0xC0, 0xC0},
		})
	}
	state := &pb.BeaconState{
		Slot:                      gs + e + 2,
		Validators:                validators,
		PreviousEpochAttestations: atts,
		Balances:                  balances,
		BlockRoots:                blockRoots,
		CurrentCrosslinks:         crosslinks,
		RandaoMixes:               make([][]byte, params.BeaconConfig().EpochsPerHistoricalVector),
		ActiveIndexRoots:          make([][]byte, params.BeaconConfig().EpochsPerHistoricalVector),
	}
	newState, err := ProcessCrosslinks(state)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(crosslinks[0], newState.CurrentCrosslinks[0]) {
		t.Errorf("Crosslink is not the same")
	}
}

func TestBaseReward_AccurateRewards(t *testing.T) {
	helpers.ClearAllCaches()

	tests := []struct {
		a uint64
		b uint64
		c uint64
	}{
		{params.BeaconConfig().MinDepositAmount, params.BeaconConfig().MinDepositAmount, 404781},
		{30 * 1e9, 30 * 1e9, 2217026},
		{params.BeaconConfig().MaxEffectiveBalance, params.BeaconConfig().MaxEffectiveBalance, 2289739},
		{40 * 1e9, params.BeaconConfig().MaxEffectiveBalance, 2289739},
	}
	for _, tt := range tests {
		helpers.ClearAllCaches()
		state := &pb.BeaconState{
			Validators: []*pb.Validator{
				{ExitEpoch: params.BeaconConfig().FarFutureEpoch, EffectiveBalance: tt.b}},
			Balances: []uint64{tt.a},
		}
		c, err := baseReward(state, 0)
		if err != nil {
			t.Fatal(err)
		}
		if c != tt.c {
			t.Errorf("baseReward(%d) = %d, want = %d",
				tt.a, c, tt.c)
		}
	}
}

func TestProcessJustificationFinalization_LessThan2ndEpoch(t *testing.T) {
	state := &pb.BeaconState{
		Slot: params.BeaconConfig().SlotsPerEpoch,
	}
	newState, err := ProcessJustificationAndFinalization(state, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(state, newState) {
		t.Error("Did not get the original state")
	}
}

func TestProcessJustificationFinalization_CantJustifyFinalize(t *testing.T) {
	e := params.BeaconConfig().FarFutureEpoch
	a := params.BeaconConfig().MaxEffectiveBalance
	state := &pb.BeaconState{
		JustificationBits: []byte{0x00},
		Slot:              params.BeaconConfig().SlotsPerEpoch * 2,
		PreviousJustifiedCheckpoint: &pb.Checkpoint{
			Epoch: 0,
			Root:  params.BeaconConfig().ZeroHash[:],
		},
		CurrentJustifiedCheckpoint: &pb.Checkpoint{
			Epoch: 0,
			Root:  params.BeaconConfig().ZeroHash[:],
		},
		Validators: []*pb.Validator{{ExitEpoch: e, EffectiveBalance: a}, {ExitEpoch: e, EffectiveBalance: a},
			{ExitEpoch: e, EffectiveBalance: a}, {ExitEpoch: e, EffectiveBalance: a}},
	}
	// Since Attested balances are less than total balances, nothing happened.
	newState, err := ProcessJustificationAndFinalization(state, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(state, newState) {
		t.Error("Did not get the original state")
	}
}

func TestProcessJustificationFinalization_NoBlockRootCurrentEpoch(t *testing.T) {
	e := params.BeaconConfig().FarFutureEpoch
	a := params.BeaconConfig().MaxEffectiveBalance
	blockRoots := make([][]byte, params.BeaconConfig().SlotsPerEpoch*2+1)
	for i := 0; i < len(blockRoots); i++ {
		blockRoots[i] = []byte{byte(i)}
	}
	state := &pb.BeaconState{
		Slot: params.BeaconConfig().SlotsPerEpoch * 2,
		PreviousJustifiedCheckpoint: &pb.Checkpoint{
			Epoch: 0,
			Root:  params.BeaconConfig().ZeroHash[:],
		},
		CurrentJustifiedCheckpoint: &pb.Checkpoint{
			Epoch: 0,
			Root:  params.BeaconConfig().ZeroHash[:],
		},
		JustificationBits: []byte{0x03}, // 0b0011
		Validators:        []*pb.Validator{{ExitEpoch: e}, {ExitEpoch: e}, {ExitEpoch: e}, {ExitEpoch: e}},
		Balances:          []uint64{a, a, a, a}, // validator total balance should be 128000000000
		BlockRoots:        blockRoots,
	}
	attestedBalance := 4 * e * 3 / 2
	_, err := ProcessJustificationAndFinalization(state, 0, attestedBalance)
	want := "could not get block root for current epoch"
	if !strings.Contains(err.Error(), want) {
		t.Fatal("Did not receive correct error")
	}
}

func TestProcessJustificationFinalization_JustifyCurrentEpoch(t *testing.T) {
	e := params.BeaconConfig().FarFutureEpoch
	a := params.BeaconConfig().MaxEffectiveBalance
	blockRoots := make([][]byte, params.BeaconConfig().SlotsPerEpoch*2+1)
	for i := 0; i < len(blockRoots); i++ {
		blockRoots[i] = []byte{byte(i)}
	}
	state := &pb.BeaconState{
		Slot: params.BeaconConfig().SlotsPerEpoch*2 + 1,
		PreviousJustifiedCheckpoint: &pb.Checkpoint{
			Epoch: 0,
			Root:  params.BeaconConfig().ZeroHash[:],
		},
		CurrentJustifiedCheckpoint: &pb.Checkpoint{
			Epoch: 0,
			Root:  params.BeaconConfig().ZeroHash[:],
		},
		FinalizedCheckpoint: &pb.Checkpoint{},
		JustificationBits:   []byte{0x03}, // 0b0011
		Validators:          []*pb.Validator{{ExitEpoch: e}, {ExitEpoch: e}, {ExitEpoch: e}, {ExitEpoch: e}},
		Balances:            []uint64{a, a, a, a}, // validator total balance should be 128000000000
		BlockRoots:          blockRoots,
	}
	attestedBalance := 4 * e * 3 / 2
	newState, err := ProcessJustificationAndFinalization(state, 0, attestedBalance)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(newState.CurrentJustifiedCheckpoint.Root, []byte{byte(128)}) {
		t.Errorf("Wanted current justified root: %v, got: %v",
			[]byte{byte(128)}, newState.CurrentJustifiedCheckpoint.Root)
	}
	if newState.CurrentJustifiedCheckpoint.Epoch != 2 {
		t.Errorf("Wanted justified epoch: %d, got: %d",
			2, newState.CurrentJustifiedCheckpoint.Epoch)
	}
	if !bytes.Equal(newState.FinalizedCheckpoint.Root, params.BeaconConfig().ZeroHash[:]) {
		t.Errorf("Wanted current finalized root: %v, got: %v",
			params.BeaconConfig().ZeroHash, newState.FinalizedCheckpoint.Root)
	}
	if newState.FinalizedCheckpoint.Epoch != 0 {
		t.Errorf("Wanted finalized epoch: %d, got: %d", 0, newState.FinalizedCheckpoint.Epoch)
	}
}

func TestProcessJustificationFinalization_JustifyPrevEpoch(t *testing.T) {
	helpers.ClearAllCaches()
	e := params.BeaconConfig().FarFutureEpoch
	a := params.BeaconConfig().MaxEffectiveBalance
	blockRoots := make([][]byte, params.BeaconConfig().SlotsPerEpoch*2+1)
	for i := 0; i < len(blockRoots); i++ {
		blockRoots[i] = []byte{byte(i)}
	}
	state := &pb.BeaconState{
		Slot: params.BeaconConfig().SlotsPerEpoch*2 + 1,
		PreviousJustifiedCheckpoint: &pb.Checkpoint{
			Epoch: 0,
			Root:  params.BeaconConfig().ZeroHash[:],
		},
		CurrentJustifiedCheckpoint: &pb.Checkpoint{
			Epoch: 0,
			Root:  params.BeaconConfig().ZeroHash[:],
		},
		JustificationBits: []byte{0x03}, // 0b0011
		Validators:        []*pb.Validator{{ExitEpoch: e}, {ExitEpoch: e}, {ExitEpoch: e}, {ExitEpoch: e}},
		Balances:          []uint64{a, a, a, a}, // validator total balance should be 128000000000
		BlockRoots:        blockRoots, FinalizedCheckpoint: &pb.Checkpoint{},
	}
	attestedBalance := 4 * e * 3 / 2
	newState, err := ProcessJustificationAndFinalization(state, attestedBalance, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(newState.CurrentJustifiedCheckpoint.Root, []byte{byte(128)}) {
		t.Errorf("Wanted current justified root: %v, got: %v",
			[]byte{byte(128)}, newState.CurrentJustifiedCheckpoint.Root)
	}
	if newState.CurrentJustifiedCheckpoint.Epoch != 2 {
		t.Errorf("Wanted justified epoch: %d, got: %d",
			2, newState.CurrentJustifiedCheckpoint.Epoch)
	}
	if !bytes.Equal(newState.FinalizedCheckpoint.Root, params.BeaconConfig().ZeroHash[:]) {
		t.Errorf("Wanted current finalized root: %v, got: %v",
			params.BeaconConfig().ZeroHash, newState.FinalizedCheckpoint.Root)
	}
	if newState.FinalizedCheckpoint.Epoch != 0 {
		t.Errorf("Wanted finalized epoch: %d, got: %d", 0, newState.FinalizedCheckpoint.Epoch)
	}
}

func TestProcessSlashings_NotSlashed(t *testing.T) {
	s := &pb.BeaconState{
		Slot:       0,
		Validators: []*pb.Validator{{Slashed: true}},
		Balances:   []uint64{params.BeaconConfig().MaxEffectiveBalance},
		Slashings:  []uint64{0, 1e9},
	}
	newState, err := ProcessSlashings(s)
	if err != nil {
		t.Fatal(err)
	}
	wanted := params.BeaconConfig().MaxEffectiveBalance
	if newState.Balances[0] != wanted {
		t.Errorf("Wanted slashed balance: %d, got: %d", wanted, newState.Balances[0])
	}
}

func TestProcessSlashings_SlashedLess(t *testing.T) {
	helpers.ClearAllCaches()
	s := &pb.BeaconState{
		Slot: 0,
		Validators: []*pb.Validator{
			{Slashed: true,
				WithdrawableEpoch: params.BeaconConfig().EpochsPerSlashingsVector / 2,
				EffectiveBalance:  params.BeaconConfig().MaxEffectiveBalance},
			{ExitEpoch: params.BeaconConfig().FarFutureEpoch, EffectiveBalance: params.BeaconConfig().MaxEffectiveBalance}},
		Balances:  []uint64{params.BeaconConfig().MaxEffectiveBalance, params.BeaconConfig().MaxEffectiveBalance},
		Slashings: []uint64{0, 1e9},
	}
	newState, err := ProcessSlashings(s)
	if err != nil {
		t.Fatal(err)
	}
	wanted := uint64(31 * 1e9)
	if newState.Balances[0] != wanted {
		t.Errorf("Wanted slashed balance: %d, got: %d", wanted, newState.Balances[0])
	}
}

func TestProcessFinalUpdates_CanProcess(t *testing.T) {
	s := buildState(params.BeaconConfig().HistoricalRootsLimit-1, params.BeaconConfig().SlotsPerEpoch)
	ce := helpers.CurrentEpoch(s)
	ne := ce + 1
	s.Eth1DataVotes = []*pb.Eth1Data{}
	s.Balances[0] = 29 * 1e9
	s.Slashings[ce] = 100
	s.RandaoMixes[ce] = []byte{'A'}
	newS, err := ProcessFinalUpdates(s)
	if err != nil {
		t.Fatal(err)
	}

	// Verify effective balance is correctly updated.
	if newS.Validators[0].EffectiveBalance != 29*1e9 {
		t.Errorf("effective balance incorrectly updated, got %d", s.Validators[0].EffectiveBalance)
	}

	// Verify start shard is correctly updated.
	if newS.StartShard != 64 {
		t.Errorf("start shard incorrectly updated, got %d", 64)
	}

	// Verify latest active index root is correctly updated in the right position.
	pos := (ne + params.BeaconConfig().ActivationExitDelay) % params.BeaconConfig().EpochsPerHistoricalVector
	if bytes.Equal(newS.ActiveIndexRoots[pos], params.BeaconConfig().ZeroHash[:]) {
		t.Error("latest active index roots still zero hashes")
	}

	// Verify slashed balances correctly updated.
	if newS.Slashings[ce] != newS.Slashings[ne] {
		t.Errorf("wanted slashed balance %d, got %d",
			newS.Slashings[ce],
			newS.Slashings[ne])
	}

	// Verify randao is correctly updated in the right position.
	if bytes.Equal(newS.RandaoMixes[ne], params.BeaconConfig().ZeroHash[:]) {
		t.Error("latest RANDAO still zero hashes")
	}

	// Verify historical root accumulator was appended.
	if len(newS.HistoricalRoots) != 1 {
		t.Errorf("wanted slashed balance %d, got %d", 1, len(newS.HistoricalRoots[ce]))
	}

	if newS.CurrentEpochAttestations == nil {
		t.Error("nil value stored in current epoch attestations instead of empty slice")
	}
}

func TestCrosslinkDelta_NoOneAttested(t *testing.T) {
	e := params.BeaconConfig().SlotsPerEpoch

	validatorCount := uint64(128)
	state := buildState(e+2, validatorCount)

	rewards, penalties, err := crosslinkDelta(state)
	if err != nil {
		t.Fatal(err)
	}
	for i := uint64(0); i < validatorCount; i++ {
		// Since no one attested, all the validators should gain 0 reward
		if rewards[i] != 0 {
			t.Errorf("Wanted reward balance 0, got %d", rewards[i])
		}
		// Since no one attested, all the validators should get penalized the same
		base, err := baseReward(state, i)
		if err != nil {
			t.Fatal(err)
		}
		if penalties[i] != base {
			t.Errorf("Wanted penalty balance %d, got %d",
				base, penalties[i])
		}
	}
}

func TestProcessRegistryUpdates_NoRotation(t *testing.T) {
	state := &pb.BeaconState{
		Slot: 5 * params.BeaconConfig().SlotsPerEpoch,
		Validators: []*pb.Validator{
			{ExitEpoch: params.BeaconConfig().ActivationExitDelay},
			{ExitEpoch: params.BeaconConfig().ActivationExitDelay},
		},
		Balances: []uint64{
			params.BeaconConfig().MaxEffectiveBalance,
			params.BeaconConfig().MaxEffectiveBalance,
		},
		FinalizedCheckpoint: &pb.Checkpoint{},
	}
	newState, err := ProcessRegistryUpdates(state)
	if err != nil {
		t.Fatal(err)
	}
	for i, validator := range newState.Validators {
		if validator.ExitEpoch != params.BeaconConfig().ActivationExitDelay {
			t.Errorf("could not update registry %d, wanted exit slot %d got %d",
				i, params.BeaconConfig().ActivationExitDelay, validator.ExitEpoch)
		}
	}
}

func TestCrosslinkDelta_SomeAttested(t *testing.T) {
	helpers.ClearAllCaches()
	e := params.BeaconConfig().SlotsPerEpoch
	helpers.ClearShuffledValidatorCache()
	state := buildState(e+2, params.BeaconConfig().DepositsForChainStart/8)
	startShard := uint64(960)
	atts := make([]*pb.PendingAttestation, 2)
	for i := 0; i < len(atts); i++ {
		atts[i] = &pb.PendingAttestation{
			Data: &pb.AttestationData{
				Crosslink: &pb.Crosslink{
					Shard:    startShard + uint64(i),
					DataRoot: []byte{'A'},
				},
			},
			InclusionDelay:  uint64(i + 100),
			AggregationBits: []byte{0xC0, 0xC0, 0xC0, 0xC0},
		}
	}
	state.PreviousEpochAttestations = atts
	state.CurrentCrosslinks[startShard] = &pb.Crosslink{
		DataRoot: []byte{'A'}, Shard: startShard,
	}
	state.CurrentCrosslinks[startShard+1] = &pb.Crosslink{
		DataRoot: []byte{'A'}, Shard: startShard + 1,
	}

	rewards, penalties, err := crosslinkDelta(state)
	if err != nil {
		t.Fatal(err)
	}

	attestedIndices := []uint64{5, 16, 336, 797, 1082, 1450, 1770, 1958}
	for _, i := range attestedIndices {
		// Since all these validators attested, they should get the same rewards.
		want := uint64(12649)
		if rewards[i] != want {
			t.Errorf("Wanted reward balance %d, got %d", want, rewards[i])
		}
		// Since all these validators attested, they shouldn't get penalized.
		if penalties[i] != 0 {
			t.Errorf("Wanted penalty balance %d, got %d",
				0, penalties[i])
		}
	}
}

func TestCrosslinkDelta_CantGetWinningCrosslink(t *testing.T) {
	state := buildState(0, 1)

	_, _, err := crosslinkDelta(state)
	wanted := "could not get winning crosslink: could not get matching attestations"
	if !strings.Contains(err.Error(), wanted) {
		t.Fatalf("Got: %v, want: %v", err.Error(), wanted)
	}
}

func TestAttestationDelta_CantGetBlockRoot(t *testing.T) {
	e := params.BeaconConfig().SlotsPerEpoch

	state := buildState(2*e, 1)
	state.Slot = 0

	_, _, err := attestationDelta(state)
	wanted := "could not get block root for epoch"
	if !strings.Contains(err.Error(), wanted) {
		t.Fatalf("Got: %v, want: %v", err.Error(), wanted)
	}
}

func TestAttestationDelta_CantGetAttestation(t *testing.T) {
	state := buildState(0, 1)

	_, _, err := attestationDelta(state)
	wanted := "could not get source, target and head attestations"
	if !strings.Contains(err.Error(), wanted) {
		t.Fatalf("Got: %v, want: %v", err.Error(), wanted)
	}
}

func TestAttestationDelta_CantGetAttestationIndices(t *testing.T) {
	e := params.BeaconConfig().SlotsPerEpoch

	state := buildState(e+2, 1)
	atts := make([]*pb.PendingAttestation, 2)
	for i := 0; i < len(atts); i++ {
		atts[i] = &pb.PendingAttestation{
			Data: &pb.AttestationData{
				Crosslink: &pb.Crosslink{
					Shard: uint64(i),
				},
			},
			InclusionDelay:  uint64(i + 100),
			AggregationBits: []byte{0xff},
		}
	}
	state.PreviousEpochAttestations = atts

	_, _, err := attestationDelta(state)
	wanted := "could not get attestation indices"
	if !strings.Contains(err.Error(), wanted) {
		t.Fatalf("Got: %v, want: %v", err.Error(), wanted)
	}
}

func TestAttestationDelta_NoOneAttested(t *testing.T) {
	e := params.BeaconConfig().SlotsPerEpoch
	validatorCount := params.BeaconConfig().DepositsForChainStart / 32
	state := buildState(e+2, validatorCount)
	//startShard := uint64(960)
	atts := make([]*pb.PendingAttestation, 2)
	for i := 0; i < len(atts); i++ {
		atts[i] = &pb.PendingAttestation{
			Data: &pb.AttestationData{
				Crosslink: &pb.Crosslink{
					Shard:    uint64(i),
					DataRoot: []byte{'A'},
				},
			},
			InclusionDelay:  uint64(i + 100),
			AggregationBits: []byte{0xC0},
		}
	}

	rewards, penalties, err := attestationDelta(state)
	if err != nil {
		t.Fatal(err)
	}
	for i := uint64(0); i < validatorCount; i++ {
		// Since no one attested, all the validators should gain 0 reward
		if rewards[i] != 0 {
			t.Errorf("Wanted reward balance 0, got %d", rewards[i])
		}
		// Since no one attested, all the validators should get penalized the same
		// it's 3 times the penalized amount because source, target and head.
		base, _ := baseReward(state, i)
		wanted := 3 * base
		if penalties[i] != wanted {
			t.Errorf("Wanted penalty balance %d, got %d",
				wanted, penalties[i])
		}
	}
}

func TestAttestationDelta_SomeAttested(t *testing.T) {
	helpers.ClearAllCaches()
	e := params.BeaconConfig().SlotsPerEpoch
	validatorCount := params.BeaconConfig().DepositsForChainStart / 8
	state := buildState(e+2, validatorCount)
	startShard := uint64(960)
	atts := make([]*pb.PendingAttestation, 3)
	for i := 0; i < len(atts); i++ {
		atts[i] = &pb.PendingAttestation{
			Data: &pb.AttestationData{
				Crosslink: &pb.Crosslink{
					Shard:    startShard + uint64(i),
					DataRoot: []byte{'A'},
				},
			},
			AggregationBits: []byte{0xC0, 0xC0, 0xC0, 0xC0},
			InclusionDelay:  1,
		}
	}
	state.PreviousEpochAttestations = atts
	state.CurrentCrosslinks[startShard] = &pb.Crosslink{
		DataRoot: []byte{'A'},
	}
	state.CurrentCrosslinks[startShard+1] = &pb.Crosslink{
		DataRoot: []byte{'A'},
	}

	rewards, penalties, err := attestationDelta(state)
	if err != nil {
		t.Fatal(err)
	}

	attestedIndices := []uint64{5, 754, 797, 1637, 1770, 1862, 1192}

	attestedBalance, err := AttestingBalance(state, atts)
	totalBalance, _ := helpers.TotalActiveBalance(state)
	if err != nil {
		t.Fatal(err)
	}
	for _, i := range attestedIndices {
		base, _ := baseReward(state, i)
		// Base rewards for getting source right
		wanted := 3 * (base * attestedBalance / totalBalance)
		// Base rewards for proposer and attesters working together getting attestation
		// on chain in the fatest manner
		proposerReward := base / params.BeaconConfig().ProposerRewardQuotient
		wanted += (base - proposerReward) * params.BeaconConfig().MinAttestationInclusionDelay
		if rewards[i] != wanted {
			t.Errorf("Wanted reward balance %d, got %d", wanted, rewards[i])
		}
		// Since all these validators attested, they shouldn't get penalized.
		if penalties[i] != 0 {
			t.Errorf("Wanted penalty balance %d, got %d",
				0, penalties[i])
		}
	}
}

func TestProcessRegistryUpdates_EligibleToActivate(t *testing.T) {
	state := &pb.BeaconState{
		Slot:                5 * params.BeaconConfig().SlotsPerEpoch,
		FinalizedCheckpoint: &pb.Checkpoint{},
	}
	limit, _ := helpers.ChurnLimit(state)
	for i := 0; i < int(limit)+10; i++ {
		state.Validators = append(state.Validators, &pb.Validator{
			ActivationEligibilityEpoch: params.BeaconConfig().FarFutureEpoch,
			EffectiveBalance:           params.BeaconConfig().MaxEffectiveBalance,
			ActivationEpoch:            params.BeaconConfig().FarFutureEpoch,
		})
	}
	currentEpoch := helpers.CurrentEpoch(state)
	newState, _ := ProcessRegistryUpdates(state)
	for i, validator := range newState.Validators {
		if validator.ActivationEligibilityEpoch != currentEpoch {
			t.Errorf("could not update registry %d, wanted activation eligibility epoch %d got %d",
				i, currentEpoch, validator.ActivationEligibilityEpoch)
		}
		if i < int(limit) && validator.ActivationEpoch != helpers.DelayedActivationExitEpoch(currentEpoch) {
			t.Errorf("could not update registry %d, validators failed to activate wanted activation epoch %d got %d",
				i, helpers.DelayedActivationExitEpoch(currentEpoch), validator.ActivationEpoch)
		}
		if i >= int(limit) && validator.ActivationEpoch != params.BeaconConfig().FarFutureEpoch {
			t.Errorf("could not update registry %d, validators should not have been activated wanted activation epoch: %d got %d",
				i, params.BeaconConfig().FarFutureEpoch, validator.ActivationEpoch)
		}
	}
}

func TestProcessRegistryUpdates_ActivationCompletes(t *testing.T) {
	state := &pb.BeaconState{
		Slot: 5 * params.BeaconConfig().SlotsPerEpoch,
		Validators: []*pb.Validator{
			{ExitEpoch: params.BeaconConfig().ActivationExitDelay,
				ActivationEpoch: 5 + params.BeaconConfig().ActivationExitDelay + 1},
			{ExitEpoch: params.BeaconConfig().ActivationExitDelay,
				ActivationEpoch: 5 + params.BeaconConfig().ActivationExitDelay + 1},
		},
		Balances: []uint64{
			params.BeaconConfig().MaxEffectiveBalance,
			params.BeaconConfig().MaxEffectiveBalance,
		},
		FinalizedCheckpoint: &pb.Checkpoint{},
	}
	newState, _ := ProcessRegistryUpdates(state)
	for i, validator := range newState.Validators {
		if validator.ExitEpoch != params.BeaconConfig().ActivationExitDelay {
			t.Errorf("could not update registry %d, wanted exit slot %d got %d",
				i, params.BeaconConfig().ActivationExitDelay, validator.ExitEpoch)
		}
	}
}

func TestProcessRegistryUpdates_CanExits(t *testing.T) {
	epoch := uint64(5)
	exitEpoch := helpers.DelayedActivationExitEpoch(epoch)
	minWithdrawalDelay := params.BeaconConfig().MinValidatorWithdrawabilityDelay
	state := &pb.BeaconState{
		Slot: epoch * params.BeaconConfig().SlotsPerEpoch,
		Validators: []*pb.Validator{
			{
				ExitEpoch:         exitEpoch,
				WithdrawableEpoch: exitEpoch + minWithdrawalDelay},
			{
				ExitEpoch:         exitEpoch,
				WithdrawableEpoch: exitEpoch + minWithdrawalDelay},
		},
		Balances: []uint64{
			params.BeaconConfig().MaxEffectiveBalance,
			params.BeaconConfig().MaxEffectiveBalance,
		},
		FinalizedCheckpoint: &pb.Checkpoint{},
	}
	newState, err := ProcessRegistryUpdates(state)
	if err != nil {
		t.Fatal(err)
	}
	for i, validator := range newState.Validators {
		if validator.ExitEpoch != exitEpoch {
			t.Errorf("could not update registry %d, wanted exit slot %d got %d",
				i,
				exitEpoch,
				validator.ExitEpoch,
			)
		}
	}
}

func TestProcessRewardsAndPenalties_GenesisEpoch(t *testing.T) {
	state := &pb.BeaconState{Slot: params.BeaconConfig().SlotsPerEpoch - 1, StartShard: 999}
	newState, err := ProcessRewardsAndPenalties(state)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(state, newState) {
		t.Error("genesis state mutated")
	}
}

func TestProcessRewardsAndPenalties_SomeAttested(t *testing.T) {
	helpers.ClearAllCaches()
	e := params.BeaconConfig().SlotsPerEpoch
	validatorCount := params.BeaconConfig().DepositsForChainStart / 8
	state := buildState(e+2, validatorCount)
	startShard := uint64(960)
	atts := make([]*pb.PendingAttestation, 3)
	for i := 0; i < len(atts); i++ {
		atts[i] = &pb.PendingAttestation{
			Data: &pb.AttestationData{
				Crosslink: &pb.Crosslink{
					Shard:    startShard + uint64(i),
					DataRoot: []byte{'A'},
				},
			},
			AggregationBits: []byte{0xC0, 0xC0, 0xC0, 0xC0},
			InclusionDelay:  1,
		}
	}
	state.PreviousEpochAttestations = atts
	state.CurrentCrosslinks[startShard] = &pb.Crosslink{
		DataRoot: []byte{'A'},
	}
	state.CurrentCrosslinks[startShard+1] = &pb.Crosslink{
		DataRoot: []byte{'A'},
	}
	state.CurrentCrosslinks[startShard+2] = &pb.Crosslink{
		DataRoot: []byte{'A'},
	}

	state, err := ProcessRewardsAndPenalties(state)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(state.Balances)
	wanted := uint64(31999797616)
	if state.Balances[0] != wanted {
		t.Errorf("wanted balance: %d, got: %d",
			wanted, state.Balances[0])
	}
	wanted = uint64(31999995452)
	if state.Balances[4] != wanted {
		t.Errorf("wanted balance: %d, got: %d",
			wanted, state.Balances[1])
	}
}

func buildState(slot uint64, validatorCount uint64) *pb.BeaconState {
	validators := make([]*pb.Validator, validatorCount)
	for i := 0; i < len(validators); i++ {
		validators[i] = &pb.Validator{
			ExitEpoch:        params.BeaconConfig().FarFutureEpoch,
			EffectiveBalance: params.BeaconConfig().MaxEffectiveBalance,
		}
	}
	validatorBalances := make([]uint64, len(validators))
	for i := 0; i < len(validatorBalances); i++ {
		validatorBalances[i] = params.BeaconConfig().MaxEffectiveBalance
	}
	latestActiveIndexRoots := make(
		[][]byte,
		params.BeaconConfig().EpochsPerHistoricalVector,
	)
	for i := 0; i < len(latestActiveIndexRoots); i++ {
		latestActiveIndexRoots[i] = params.BeaconConfig().ZeroHash[:]
	}
	latestRandaoMixes := make(
		[][]byte,
		params.BeaconConfig().EpochsPerHistoricalVector,
	)
	for i := 0; i < len(latestRandaoMixes); i++ {
		latestRandaoMixes[i] = params.BeaconConfig().ZeroHash[:]
	}
	return &pb.BeaconState{
		Slot:                        slot,
		Balances:                    validatorBalances,
		Validators:                  validators,
		CurrentCrosslinks:           make([]*pb.Crosslink, params.BeaconConfig().ShardCount),
		RandaoMixes:                 make([][]byte, params.BeaconConfig().EpochsPerHistoricalVector),
		ActiveIndexRoots:            make([][]byte, params.BeaconConfig().EpochsPerHistoricalVector),
		Slashings:                   make([]uint64, params.BeaconConfig().EpochsPerSlashingsVector),
		BlockRoots:                  make([][]byte, params.BeaconConfig().SlotsPerEpoch*10),
		FinalizedCheckpoint:         &pb.Checkpoint{},
		PreviousJustifiedCheckpoint: &pb.Checkpoint{},
		CurrentJustifiedCheckpoint:  &pb.Checkpoint{},
	}
}
