package rpc

import (
	"context"
	"github.com/prysmaticlabs/prysm/shared/testutil"
	"reflect"
	"testing"

	b "github.com/prysmaticlabs/prysm/beacon-chain/core/blocks"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/state"
	"github.com/prysmaticlabs/prysm/beacon-chain/internal"
	pbp2p "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/rpc/v1"
	"github.com/prysmaticlabs/prysm/shared/featureconfig"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/ssz"
)

func init() {
	featureconfig.InitFeatureConfig(&featureconfig.FeatureFlagConfig{
		EnableComputeStateRoot: true,
	})
}

func TestProposeBlock_OK(t *testing.T) {
	db := internal.SetupDB(t)
	defer internal.TeardownDB(t, db)
	mockChain := &mockChainService{}
	ctx := context.Background()

	genesis := b.NewGenesisBlock([]byte{})
	if err := db.SaveBlock(genesis); err != nil {
		t.Fatalf("Could not save genesis block: %v", err)
	}

	numDeposits := params.BeaconConfig().DepositsForChainStart
	deposits, _ := testutil.SetupInitialDeposits(t, numDeposits, false)
	beaconState, err := state.GenesisBeaconState(deposits, 0, nil)
	if err != nil {
		t.Fatalf("Could not instantiate genesis state: %v", err)
	}

	if err := db.UpdateChainHead(ctx, genesis, beaconState); err != nil {
		t.Fatalf("Could not save genesis state: %v", err)
	}

	proposerServer := &ProposerServer{
		chainService:    mockChain,
		beaconDB:        db,
		powChainService: &mockPOWChainService{},
	}
	req := &pbp2p.BeaconBlock{
		Slot:       5,
		ParentRoot: []byte("parent-hash"),
	}
	if err := proposerServer.beaconDB.SaveBlock(req); err != nil {
		t.Fatal(err)
	}
	if _, err := proposerServer.ProposeBlock(context.Background(), req); err != nil {
		t.Errorf("Could not propose block correctly: %v", err)
	}
}

func TestComputeStateRoot_OK(t *testing.T) {
	db := internal.SetupDB(t)
	defer internal.TeardownDB(t, db)
	ctx := context.Background()

	mockChain := &mockChainService{}

	genesis := b.NewGenesisBlock([]byte{})
	if err := db.SaveBlock(genesis); err != nil {
		t.Fatalf("Could not save genesis block: %v", err)
	}

	deposits, _ := testutil.SetupInitialDeposits(t, params.BeaconConfig().DepositsForChainStart, false)
	eth1Data := testutil.GenerateEth1Data(t, deposits)
	beaconState, err := state.GenesisBeaconState(deposits, 0, eth1Data)
	if err != nil {
		t.Fatalf("Could not instantiate genesis state: %v", err)
	}
	beaconState.LatestStateRoots = make([][]byte, params.BeaconConfig().SlotsPerHistoricalRoot)
	beaconState.LatestBlockHeader = &pbp2p.BeaconBlockHeader{
		StateRoot: []byte{},
	}
	beaconState.Slot = 10

	if err := db.UpdateChainHead(ctx, genesis, beaconState); err != nil {
		t.Fatalf("Could not save genesis state: %v", err)
	}

	proposerServer := &ProposerServer{
		chainService:    mockChain,
		beaconDB:        db,
		powChainService: &mockPOWChainService{},
	}

	req := &pbp2p.BeaconBlock{
		ParentRoot: nil,
		Slot:       11,
		Body: &pbp2p.BeaconBlockBody{
			RandaoReveal:      nil,
			ProposerSlashings: nil,
			AttesterSlashings: nil,
			Eth1Data:          &pbp2p.Eth1Data{},
		},
	}

	_, _ = proposerServer.ComputeStateRoot(context.Background(), req)
}

func TestPendingAttestations_FiltersWithinInclusionDelay(t *testing.T) {
	helpers.ClearAllCaches()

	db := internal.SetupDB(t)
	defer internal.TeardownDB(t, db)
	ctx := context.Background()

	validators := make([]*pbp2p.Validator, params.BeaconConfig().DepositsForChainStart/8)
	for i := 0; i < len(validators); i++ {
		validators[i] = &pbp2p.Validator{
			ExitEpoch: params.BeaconConfig().FarFutureEpoch,
		}
	}

	crosslinks := make([]*pbp2p.Crosslink, params.BeaconConfig().ShardCount)
	for i := 0; i < len(crosslinks); i++ {
		crosslinks[i] = &pbp2p.Crosslink{
			Epoch:    1,
			DataRoot: params.BeaconConfig().ZeroHash[:],
		}
	}

	stateSlot := uint64(100)
	beaconState := &pbp2p.BeaconState{
		Slot:                   stateSlot,
		ValidatorRegistry:      validators,
		CurrentCrosslinks:      crosslinks,
		PreviousCrosslinks:     crosslinks,
		LatestStartShard:       100,
		LatestRandaoMixes:      make([][]byte, params.BeaconConfig().LatestRandaoMixesLength),
		LatestActiveIndexRoots: make([][]byte, params.BeaconConfig().LatestActiveIndexRootsLength),
	}

	encoded, err := ssz.TreeHash(beaconState.PreviousCrosslinks[0])
	if err != nil {
		t.Fatal(err)
	}

	proposerServer := &ProposerServer{
		operationService: &mockOperationService{
			pendingAttestations: []*pbp2p.Attestation{
				{Data: &pbp2p.AttestationData{
					Crosslink: &pbp2p.Crosslink{
						Shard:      beaconState.Slot - params.BeaconConfig().MinAttestationInclusionDelay,
						DataRoot:   params.BeaconConfig().ZeroHash[:],
						ParentRoot: encoded[:]},
				},
					AggregationBitfield: []byte{0xC0, 0xC0, 0xC0, 0xC0},
				},
			},
		},
		chainService: &mockChainService{},
		beaconDB:     db,
	}
	if err := db.SaveState(ctx, beaconState); err != nil {
		t.Fatal(err)
	}

	blk := &pbp2p.BeaconBlock{
		Slot: beaconState.Slot,
	}

	if err := db.SaveBlock(blk); err != nil {
		t.Fatalf("failed to save block %v", err)
	}

	if err := db.UpdateChainHead(ctx, blk, beaconState); err != nil {
		t.Fatalf("couldnt update chainhead: %v", err)
	}

	res, err := proposerServer.PendingAttestations(context.Background(), &pb.PendingAttestationsRequest{
		ProposalBlockSlot: blk.Slot + 1,
	})
	if err != nil {
		t.Fatalf("Unexpected error fetching pending attestations: %v", err)
	}
	if len(res.PendingAttestations) == 0 {
		t.Error("Expected pending attestations list to be non-empty")
	}
}

func TestPendingAttestations_FiltersExpiredAttestations(t *testing.T) {
	db := internal.SetupDB(t)
	defer internal.TeardownDB(t, db)
	ctx := context.Background()

	// Edge case: current slot is at the end of an epoch. The pending attestation
	// for the next slot should come from currentSlot + 1.
	currentSlot := helpers.StartSlot(
		10,
	) - 1

	expectedEpoch := uint64(100)
	crosslink := &pbp2p.Crosslink{Epoch: 9, DataRoot: params.BeaconConfig().ZeroHash[:]}
	encoded, err := ssz.TreeHash(crosslink)
	if err != nil {
		t.Fatal(err)
	}

	opService := &mockOperationService{
		pendingAttestations: []*pbp2p.Attestation{
			//Expired attestations
			{Data: &pbp2p.AttestationData{
				TargetEpoch: 10,
				SourceEpoch: expectedEpoch,
				Crosslink:   &pbp2p.Crosslink{DataRoot: params.BeaconConfig().ZeroHash[:]},
			}},
			{Data: &pbp2p.AttestationData{
				TargetEpoch: 10,

				SourceEpoch: expectedEpoch,
				Crosslink:   &pbp2p.Crosslink{DataRoot: params.BeaconConfig().ZeroHash[:]},
			}},
			{Data: &pbp2p.AttestationData{
				TargetEpoch: 10,
				SourceEpoch: expectedEpoch,
				Crosslink:   &pbp2p.Crosslink{DataRoot: params.BeaconConfig().ZeroHash[:]},
			}},
			{Data: &pbp2p.AttestationData{
				TargetEpoch: 10,
				SourceEpoch: expectedEpoch,
				Crosslink:   &pbp2p.Crosslink{DataRoot: params.BeaconConfig().ZeroHash[:]},
			}},
			{Data: &pbp2p.AttestationData{
				TargetEpoch: 10,
				SourceEpoch: expectedEpoch,
				Crosslink:   &pbp2p.Crosslink{DataRoot: params.BeaconConfig().ZeroHash[:]},
			}},
			// Non-expired attestation with incorrect justified epoch
			{Data: &pbp2p.AttestationData{
				TargetEpoch: 10,
				SourceEpoch: expectedEpoch - 1,
				Crosslink:   &pbp2p.Crosslink{DataRoot: params.BeaconConfig().ZeroHash[:]},
			}},
			// Non-expired attestations with correct justified epoch
			{Data: &pbp2p.AttestationData{
				TargetEpoch: 10,
				SourceEpoch: expectedEpoch,
				Crosslink:   &pbp2p.Crosslink{Epoch: 10, DataRoot: params.BeaconConfig().ZeroHash[:], ParentRoot: encoded[:]},
			}, AggregationBitfield: []byte{0xC0, 0xC0, 0xC0, 0xC0}},
			{Data: &pbp2p.AttestationData{
				TargetEpoch: 10,
				SourceEpoch: expectedEpoch,
				Crosslink:   &pbp2p.Crosslink{Epoch: 10, DataRoot: params.BeaconConfig().ZeroHash[:], ParentRoot: encoded[:]},
			}, AggregationBitfield: []byte{0xC0, 0xC0, 0xC0, 0xC0}},
			{Data: &pbp2p.AttestationData{
				TargetEpoch: 10,
				SourceEpoch: expectedEpoch,
				Crosslink:   &pbp2p.Crosslink{Epoch: 10, DataRoot: params.BeaconConfig().ZeroHash[:], ParentRoot: encoded[:]},
			}, AggregationBitfield: []byte{0xC0, 0xC0, 0xC0, 0xC0}},
		},
	}
	expectedNumberOfAttestations := 3
	proposerServer := &ProposerServer{
		operationService: opService,
		chainService:     &mockChainService{},
		beaconDB:         db,
	}

	validators := make([]*pbp2p.Validator, params.BeaconConfig().DepositsForChainStart/8)
	for i := 0; i < len(validators); i++ {
		validators[i] = &pbp2p.Validator{
			ExitEpoch: params.BeaconConfig().FarFutureEpoch,
		}
	}

	beaconState := &pbp2p.BeaconState{
		ValidatorRegistry:      validators,
		Slot:                   currentSlot + params.BeaconConfig().MinAttestationInclusionDelay,
		CurrentJustifiedEpoch:  expectedEpoch,
		PreviousJustifiedEpoch: expectedEpoch,
		CurrentCrosslinks: []*pbp2p.Crosslink{{
			Epoch:    9,
			DataRoot: params.BeaconConfig().ZeroHash[:],
		}},
		LatestRandaoMixes:      make([][]byte, params.BeaconConfig().LatestRandaoMixesLength),
		LatestActiveIndexRoots: make([][]byte, params.BeaconConfig().LatestActiveIndexRootsLength),
	}

	if err := db.SaveState(ctx, beaconState); err != nil {
		t.Fatal(err)
	}

	blk := &pbp2p.BeaconBlock{
		Slot: beaconState.Slot,
	}

	if err := db.SaveBlock(blk); err != nil {
		t.Fatalf("failed to save block %v", err)
	}

	if err := db.UpdateChainHead(ctx, blk, beaconState); err != nil {
		t.Fatalf("couldnt update chainhead: %v", err)
	}

	res, err := proposerServer.PendingAttestations(
		context.Background(),
		&pb.PendingAttestationsRequest{
			ProposalBlockSlot: currentSlot,
		},
	)
	if err != nil {
		t.Fatalf("Unexpected error fetching pending attestations: %v", err)
	}
	if len(res.PendingAttestations) != expectedNumberOfAttestations {
		t.Errorf(
			"Expected pending attestations list length %d, but was %d",
			expectedNumberOfAttestations,
			len(res.PendingAttestations),
		)
	}

	expectedAtts := []*pbp2p.Attestation{
		{Data: &pbp2p.AttestationData{
			TargetEpoch: 10,
			SourceEpoch: expectedEpoch,
			Crosslink:   &pbp2p.Crosslink{Epoch: 10, DataRoot: params.BeaconConfig().ZeroHash[:], ParentRoot: encoded[:]},
		}, AggregationBitfield: []byte{0xC0, 0xC0, 0xC0, 0xC0}},
		{Data: &pbp2p.AttestationData{
			TargetEpoch: 10,
			SourceEpoch: expectedEpoch,
			Crosslink:   &pbp2p.Crosslink{Epoch: 10, DataRoot: params.BeaconConfig().ZeroHash[:], ParentRoot: encoded[:]},
		}, AggregationBitfield: []byte{0xC0, 0xC0, 0xC0, 0xC0}},
		{Data: &pbp2p.AttestationData{
			TargetEpoch: 10,
			SourceEpoch: expectedEpoch,
			Crosslink:   &pbp2p.Crosslink{Epoch: 10, DataRoot: params.BeaconConfig().ZeroHash[:], ParentRoot: encoded[:]},
		}, AggregationBitfield: []byte{0xC0, 0xC0, 0xC0, 0xC0}},
	}
	if !reflect.DeepEqual(res.PendingAttestations, expectedAtts) {
		t.Error("Did not receive expected attestations")
	}
}
