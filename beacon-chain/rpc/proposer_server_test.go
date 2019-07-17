package rpc

import (
	"bytes"
	"context"
	"math/big"
	"reflect"
	"strings"
	"testing"

	"github.com/prysmaticlabs/go-bitfield"
	"github.com/prysmaticlabs/go-ssz"
	b "github.com/prysmaticlabs/prysm/beacon-chain/core/blocks"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/state"
	"github.com/prysmaticlabs/prysm/beacon-chain/db"
	"github.com/prysmaticlabs/prysm/beacon-chain/internal"
	pbp2p "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	"github.com/prysmaticlabs/prysm/shared/hashutil"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/testutil"
	"github.com/prysmaticlabs/prysm/shared/trieutil"
)

func TestProposeBlock_OK(t *testing.T) {
	db := internal.SetupDB(t)
	defer internal.TeardownDB(t, db)
	mockChain := &mockChainService{}
	ctx := context.Background()

	genesis := b.NewGenesisBlock([]byte{})
	if err := db.SaveBlock(genesis); err != nil {
		t.Fatalf("Could not save genesis block: %v", err)
	}

	numDeposits := params.BeaconConfig().MinGenesisActiveValidatorCount
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
	helpers.ClearAllCaches()

	mockChain := &mockChainService{}

	deposits, _ := testutil.SetupInitialDeposits(t, params.BeaconConfig().MinGenesisActiveValidatorCount, false)
	beaconState, err := state.GenesisBeaconState(deposits, 0, nil)
	if err != nil {
		t.Fatalf("Could not instantiate genesis state: %v", err)
	}

	stateRoot, err := ssz.HashTreeRoot(beaconState)
	if err != nil {
		t.Fatalf("Could not hash genesis state: %v", err)
	}

	genesis := b.NewGenesisBlock(stateRoot[:])
	if err := db.SaveBlock(genesis); err != nil {
		t.Fatalf("Could not save genesis block: %v", err)
	}

	if err := db.UpdateChainHead(ctx, genesis, beaconState); err != nil {
		t.Fatalf("Could not save genesis state: %v", err)
	}

	parentRoot, err := ssz.SigningRoot(genesis)
	if err != nil {
		t.Fatalf("Could not get signing root %v", err)
	}

	proposerServer := &ProposerServer{
		chainService:    mockChain,
		beaconDB:        db,
		powChainService: &mockPOWChainService{},
	}

	req := &pbp2p.BeaconBlock{
		ParentRoot: parentRoot[:],
		Slot:       8,
		Body: &pbp2p.BeaconBlockBody{
			RandaoReveal:      nil,
			ProposerSlashings: nil,
			AttesterSlashings: nil,
			Eth1Data:          &pbp2p.Eth1Data{},
		},
	}

	_, err = proposerServer.computeStateRoot(context.Background(), req)
	if err != nil {
		t.Error(err)
	}
}

func TestPendingAttestations_FiltersWithinInclusionDelay(t *testing.T) {
	helpers.ClearAllCaches()

	db := internal.SetupDB(t)
	defer internal.TeardownDB(t, db)
	ctx := context.Background()

	validators := make([]*pbp2p.Validator, params.BeaconConfig().MinGenesisActiveValidatorCount/8)
	for i := 0; i < len(validators); i++ {
		validators[i] = &pbp2p.Validator{
			ExitEpoch: params.BeaconConfig().FarFutureEpoch,
		}
	}

	crosslinks := make([]*pbp2p.Crosslink, params.BeaconConfig().ShardCount)
	for i := 0; i < len(crosslinks); i++ {
		crosslinks[i] = &pbp2p.Crosslink{
			StartEpoch: 1,
			DataRoot:   params.BeaconConfig().ZeroHash[:],
		}
	}

	stateSlot := uint64(100)
	beaconState := &pbp2p.BeaconState{
		Slot:                        stateSlot,
		Validators:                  validators,
		CurrentCrosslinks:           crosslinks,
		PreviousCrosslinks:          crosslinks,
		StartShard:                  100,
		RandaoMixes:                 make([][]byte, params.BeaconConfig().EpochsPerHistoricalVector),
		ActiveIndexRoots:            make([][]byte, params.BeaconConfig().EpochsPerHistoricalVector),
		FinalizedCheckpoint:         &pbp2p.Checkpoint{},
		PreviousJustifiedCheckpoint: &pbp2p.Checkpoint{},
		CurrentJustifiedCheckpoint:  &pbp2p.Checkpoint{},
	}

	encoded, err := ssz.HashTreeRoot(beaconState.PreviousCrosslinks[0])
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
					Source: &pbp2p.Checkpoint{},
					Target: &pbp2p.Checkpoint{},
				},
					AggregationBits: bitfield.Bitlist{0xC0, 0xC0, 0xC0, 0xC0, 0x01},
					CustodyBits:     []byte{0x00, 0x00, 0x00, 0x00},
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

	atts, err := proposerServer.attestations(context.Background(), stateSlot)
	if err != nil {
		t.Fatalf("Unexpected error fetching pending attestations: %v", err)
	}
	if len(atts) == 0 {
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
	crosslink := &pbp2p.Crosslink{StartEpoch: 9, DataRoot: params.BeaconConfig().ZeroHash[:]}
	encoded, err := ssz.HashTreeRoot(crosslink)
	if err != nil {
		t.Fatal(err)
	}

	opService := &mockOperationService{
		pendingAttestations: []*pbp2p.Attestation{
			//Expired attestations
			{Data: &pbp2p.AttestationData{
				Target: &pbp2p.Checkpoint{Epoch: 10},
				Source: &pbp2p.Checkpoint{Epoch: expectedEpoch},

				Crosslink: &pbp2p.Crosslink{DataRoot: params.BeaconConfig().ZeroHash[:]},
			}},
			{Data: &pbp2p.AttestationData{
				Target:    &pbp2p.Checkpoint{Epoch: 10},
				Source:    &pbp2p.Checkpoint{Epoch: expectedEpoch},
				Crosslink: &pbp2p.Crosslink{DataRoot: params.BeaconConfig().ZeroHash[:]},
			}},
			{Data: &pbp2p.AttestationData{
				Target:    &pbp2p.Checkpoint{Epoch: 10},
				Source:    &pbp2p.Checkpoint{Epoch: expectedEpoch},
				Crosslink: &pbp2p.Crosslink{DataRoot: params.BeaconConfig().ZeroHash[:]},
			}},
			{Data: &pbp2p.AttestationData{
				Target:    &pbp2p.Checkpoint{Epoch: 10},
				Source:    &pbp2p.Checkpoint{Epoch: expectedEpoch},
				Crosslink: &pbp2p.Crosslink{DataRoot: params.BeaconConfig().ZeroHash[:]},
			}},
			{Data: &pbp2p.AttestationData{
				Target:    &pbp2p.Checkpoint{Epoch: 10},
				Source:    &pbp2p.Checkpoint{Epoch: expectedEpoch},
				Crosslink: &pbp2p.Crosslink{DataRoot: params.BeaconConfig().ZeroHash[:]},
			}},
			// Non-expired attestation with incorrect justified epoch
			{Data: &pbp2p.AttestationData{
				Target:    &pbp2p.Checkpoint{Epoch: 10},
				Source:    &pbp2p.Checkpoint{Epoch: expectedEpoch - 1},
				Crosslink: &pbp2p.Crosslink{DataRoot: params.BeaconConfig().ZeroHash[:]},
			}},
			// Non-expired attestations with correct justified epoch
			{Data: &pbp2p.AttestationData{
				Target:    &pbp2p.Checkpoint{Epoch: 10},
				Source:    &pbp2p.Checkpoint{Epoch: expectedEpoch},
				Crosslink: &pbp2p.Crosslink{EndEpoch: 10, DataRoot: params.BeaconConfig().ZeroHash[:], ParentRoot: encoded[:]},
			}, AggregationBits: bitfield.Bitlist{0xC0, 0xC0, 0xC0, 0xC0, 0x01}},
			{Data: &pbp2p.AttestationData{
				Target:    &pbp2p.Checkpoint{Epoch: 10},
				Source:    &pbp2p.Checkpoint{Epoch: expectedEpoch},
				Crosslink: &pbp2p.Crosslink{EndEpoch: 10, DataRoot: params.BeaconConfig().ZeroHash[:], ParentRoot: encoded[:]},
			}, AggregationBits: bitfield.Bitlist{0xC0, 0xC0, 0xC0, 0xC0, 0x01}},
			{Data: &pbp2p.AttestationData{
				Target:    &pbp2p.Checkpoint{Epoch: 10},
				Source:    &pbp2p.Checkpoint{Epoch: expectedEpoch},
				Crosslink: &pbp2p.Crosslink{EndEpoch: 10, DataRoot: params.BeaconConfig().ZeroHash[:], ParentRoot: encoded[:]},
			}, AggregationBits: bitfield.Bitlist{0xC0, 0xC0, 0xC0, 0xC0, 0x01}},
		},
	}
	expectedNumberOfAttestations := 3
	proposerServer := &ProposerServer{
		operationService: opService,
		chainService:     &mockChainService{},
		beaconDB:         db,
	}

	validators := make([]*pbp2p.Validator, params.BeaconConfig().MinGenesisActiveValidatorCount/8)
	for i := 0; i < len(validators); i++ {
		validators[i] = &pbp2p.Validator{
			ExitEpoch: params.BeaconConfig().FarFutureEpoch,
		}
	}

	beaconState := &pbp2p.BeaconState{
		Validators: validators,
		Slot:       currentSlot + params.BeaconConfig().MinAttestationInclusionDelay,
		CurrentJustifiedCheckpoint: &pbp2p.Checkpoint{
			Epoch: expectedEpoch,
		},
		PreviousJustifiedCheckpoint: &pbp2p.Checkpoint{
			Epoch: expectedEpoch,
		},
		CurrentCrosslinks: []*pbp2p.Crosslink{{
			StartEpoch: 9,
			DataRoot:   params.BeaconConfig().ZeroHash[:],
		}},
		RandaoMixes:      make([][]byte, params.BeaconConfig().EpochsPerHistoricalVector),
		ActiveIndexRoots: make([][]byte, params.BeaconConfig().EpochsPerHistoricalVector),
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

	atts, err := proposerServer.attestations(context.Background(), currentSlot+params.BeaconConfig().MinAttestationInclusionDelay+1)
	if err != nil {
		t.Fatalf("Unexpected error fetching pending attestations: %v", err)
	}
	if len(atts) != expectedNumberOfAttestations {
		t.Errorf(
			"Expected pending attestations list length %d, but was %d",
			expectedNumberOfAttestations,
			len(atts),
		)
	}

	expectedAtts := []*pbp2p.Attestation{
		{Data: &pbp2p.AttestationData{
			Target:    &pbp2p.Checkpoint{Epoch: 10},
			Source:    &pbp2p.Checkpoint{Epoch: expectedEpoch},
			Crosslink: &pbp2p.Crosslink{EndEpoch: 10, DataRoot: params.BeaconConfig().ZeroHash[:], ParentRoot: encoded[:]},
		}, AggregationBits: bitfield.Bitlist{0xC0, 0xC0, 0xC0, 0xC0, 0x01}},
		{Data: &pbp2p.AttestationData{
			Target:    &pbp2p.Checkpoint{Epoch: 10},
			Source:    &pbp2p.Checkpoint{Epoch: expectedEpoch},
			Crosslink: &pbp2p.Crosslink{EndEpoch: 10, DataRoot: params.BeaconConfig().ZeroHash[:], ParentRoot: encoded[:]},
		}, AggregationBits: bitfield.Bitlist{0xC0, 0xC0, 0xC0, 0xC0, 0x01}},
		{Data: &pbp2p.AttestationData{
			Target:    &pbp2p.Checkpoint{Epoch: 10},
			Source:    &pbp2p.Checkpoint{Epoch: expectedEpoch},
			Crosslink: &pbp2p.Crosslink{EndEpoch: 10, DataRoot: params.BeaconConfig().ZeroHash[:], ParentRoot: encoded[:]},
		}, AggregationBits: bitfield.Bitlist{0xC0, 0xC0, 0xC0, 0xC0, 0x01}},
	}
	if !reflect.DeepEqual(atts, expectedAtts) {
		t.Error("Did not receive expected attestations")
	}
}

func TestPendingDeposits_UnknownBlockNum(t *testing.T) {
	p := &mockPOWChainService{
		latestBlockNumber: nil,
	}
	ps := ProposerServer{powChainService: p}

	_, err := ps.deposits(context.Background())
	if err.Error() != "latest PoW block number is unknown" {
		t.Errorf("Received unexpected error: %v", err)
	}
}

func TestPendingDeposits_OutsideEth1FollowWindow(t *testing.T) {
	ctx := context.Background()

	height := big.NewInt(int64(params.BeaconConfig().Eth1FollowDistance))
	p := &mockPOWChainService{
		latestBlockNumber: height,
		hashesByHeight: map[int][]byte{
			int(height.Int64()): []byte("0x0"),
		},
	}
	d := internal.SetupDB(t)

	beaconState := &pbp2p.BeaconState{
		Eth1Data: &pbp2p.Eth1Data{
			BlockHash: []byte("0x0"),
		},
		Eth1DepositIndex: 2,
	}
	if err := d.SaveState(ctx, beaconState); err != nil {
		t.Fatal(err)
	}

	var mockSig [96]byte
	var mockCreds [32]byte

	// Using the merkleTreeIndex as the block number for this test...
	readyDeposits := []*db.DepositContainer{
		{
			Index: 0,
			Deposit: &pbp2p.Deposit{
				Data: &pbp2p.DepositData{
					Pubkey:                []byte("a"),
					Signature:             mockSig[:],
					WithdrawalCredentials: mockCreds[:],
				}},
		},
		{
			Index: 1,
			Deposit: &pbp2p.Deposit{
				Data: &pbp2p.DepositData{
					Pubkey:                []byte("b"),
					Signature:             mockSig[:],
					WithdrawalCredentials: mockCreds[:],
				}},
		},
	}

	recentDeposits := []*db.DepositContainer{
		{
			Index: 2,
			Deposit: &pbp2p.Deposit{
				Data: &pbp2p.DepositData{
					Pubkey:                []byte("c"),
					Signature:             mockSig[:],
					WithdrawalCredentials: mockCreds[:],
				}},
		},
		{
			Index: 3,
			Deposit: &pbp2p.Deposit{
				Data: &pbp2p.DepositData{
					Pubkey:                []byte("d"),
					Signature:             mockSig[:],
					WithdrawalCredentials: mockCreds[:],
				}},
		},
	}
	depositTrie, err := trieutil.NewTrie(int(params.BeaconConfig().DepositContractTreeDepth))
	if err != nil {
		t.Fatalf("could not setup deposit trie: %v", err)
	}
	for _, dp := range append(readyDeposits, recentDeposits...) {
		depositHash, err := hashutil.DepositHash(dp.Deposit.Data)
		if err != nil {
			t.Fatalf("Unable to determine hashed value of deposit %v", err)
		}

		if err := depositTrie.InsertIntoTrie(depositHash[:], int(dp.Index)); err != nil {
			t.Fatalf("Unable to insert deposit into trie %v", err)
		}

		d.InsertDeposit(ctx, dp.Deposit, big.NewInt(int64(dp.Index)), dp.Index, depositTrie.Root())
	}
	for _, dp := range recentDeposits {
		d.InsertPendingDeposit(ctx, dp.Deposit, big.NewInt(int64(dp.Index)), dp.Index, depositTrie.Root())
	}

	bs := &ProposerServer{
		beaconDB:        d,
		powChainService: p,
		chainService:    newMockChainService(),
	}

	deposits, err := bs.deposits(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(deposits) != 0 {
		t.Errorf("Received unexpected list of deposits: %+v, wanted: 0", len(deposits))
	}

	// It should also return the recent deposits after their follow window.
	p.latestBlockNumber = big.NewInt(0).Add(p.latestBlockNumber, big.NewInt(10000))
	deposits, err = bs.deposits(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(deposits) != len(recentDeposits) {
		t.Errorf(
			"Received unexpected number of pending deposits: %d, wanted: %d",
			len(deposits),
			len(recentDeposits),
		)
	}
}

func TestPendingDeposits_CantReturnBelowStateEth1DepositIndex(t *testing.T) {
	ctx := context.Background()

	height := big.NewInt(int64(params.BeaconConfig().Eth1FollowDistance))
	p := &mockPOWChainService{
		latestBlockNumber: height,
		hashesByHeight: map[int][]byte{
			int(height.Int64()): []byte("0x0"),
		},
	}
	d := internal.SetupDB(t)

	beaconState := &pbp2p.BeaconState{
		Eth1Data: &pbp2p.Eth1Data{
			BlockHash: []byte("0x0"),
		},
		Eth1DepositIndex: 10,
	}
	if err := d.SaveState(ctx, beaconState); err != nil {
		t.Fatal(err)
	}

	var mockSig [96]byte
	var mockCreds [32]byte

	readyDeposits := []*db.DepositContainer{
		{
			Index: 0,
			Deposit: &pbp2p.Deposit{
				Data: &pbp2p.DepositData{
					Pubkey:                []byte("a"),
					Signature:             mockSig[:],
					WithdrawalCredentials: mockCreds[:],
				}},
		},
		{
			Index: 1,
			Deposit: &pbp2p.Deposit{
				Data: &pbp2p.DepositData{
					Pubkey:                []byte("b"),
					Signature:             mockSig[:],
					WithdrawalCredentials: mockCreds[:],
				}},
		},
	}

	var recentDeposits []*db.DepositContainer
	for i := 2; i < 16; i++ {
		recentDeposits = append(recentDeposits, &db.DepositContainer{
			Index: i,
			Deposit: &pbp2p.Deposit{
				Data: &pbp2p.DepositData{
					Pubkey:                []byte{byte(i)},
					Signature:             mockSig[:],
					WithdrawalCredentials: mockCreds[:],
				}},
		})
	}
	depositTrie, err := trieutil.NewTrie(int(params.BeaconConfig().DepositContractTreeDepth))
	if err != nil {
		t.Fatalf("could not setup deposit trie: %v", err)
	}
	for _, dp := range append(readyDeposits, recentDeposits...) {
		depositHash, err := hashutil.DepositHash(dp.Deposit.Data)
		if err != nil {
			t.Fatalf("Unable to determine hashed value of deposit %v", err)
		}

		if err := depositTrie.InsertIntoTrie(depositHash[:], int(dp.Index)); err != nil {
			t.Fatalf("Unable to insert deposit into trie %v", err)
		}

		d.InsertDeposit(ctx, dp.Deposit, big.NewInt(int64(dp.Index)), dp.Index, depositTrie.Root())
	}
	for _, dp := range recentDeposits {
		d.InsertPendingDeposit(ctx, dp.Deposit, big.NewInt(int64(dp.Index)), dp.Index, depositTrie.Root())
	}

	bs := &ProposerServer{
		beaconDB:        d,
		powChainService: p,
		chainService:    newMockChainService(),
	}

	// It should also return the recent deposits after their follow window.
	p.latestBlockNumber = big.NewInt(0).Add(p.latestBlockNumber, big.NewInt(10000))
	deposits, err := bs.deposits(ctx)
	if err != nil {
		t.Fatal(err)
	}

	expectedDeposits := 6
	if len(deposits) != expectedDeposits {
		t.Errorf(
			"Received unexpected number of pending deposits: %d, wanted: %d",
			len(deposits),
			expectedDeposits,
		)
	}
}

func TestPendingDeposits_CantReturnMoreThanMax(t *testing.T) {
	ctx := context.Background()

	height := big.NewInt(int64(params.BeaconConfig().Eth1FollowDistance))
	p := &mockPOWChainService{
		latestBlockNumber: height,
		hashesByHeight: map[int][]byte{
			int(height.Int64()): []byte("0x0"),
		},
	}
	d := internal.SetupDB(t)

	beaconState := &pbp2p.BeaconState{
		Eth1Data: &pbp2p.Eth1Data{
			BlockHash: []byte("0x0"),
		},
		Eth1DepositIndex: 2,
	}
	if err := d.SaveState(ctx, beaconState); err != nil {
		t.Fatal(err)
	}
	var mockSig [96]byte
	var mockCreds [32]byte

	readyDeposits := []*db.DepositContainer{
		{
			Index: 0,
			Deposit: &pbp2p.Deposit{
				Data: &pbp2p.DepositData{
					Pubkey:                []byte("a"),
					Signature:             mockSig[:],
					WithdrawalCredentials: mockCreds[:],
				}},
		},
		{
			Index: 1,
			Deposit: &pbp2p.Deposit{
				Data: &pbp2p.DepositData{
					Pubkey:                []byte("b"),
					Signature:             mockSig[:],
					WithdrawalCredentials: mockCreds[:],
				}},
		},
	}

	var recentDeposits []*db.DepositContainer
	for i := 2; i < 22; i++ {
		recentDeposits = append(recentDeposits, &db.DepositContainer{
			Index: i,
			Deposit: &pbp2p.Deposit{
				Data: &pbp2p.DepositData{
					Pubkey:                []byte{byte(i)},
					Signature:             mockSig[:],
					WithdrawalCredentials: mockCreds[:],
				}},
		})
	}
	depositTrie, err := trieutil.NewTrie(int(params.BeaconConfig().DepositContractTreeDepth))
	if err != nil {
		t.Fatalf("could not setup deposit trie: %v", err)
	}
	for _, dp := range append(readyDeposits, recentDeposits...) {
		depositHash, err := hashutil.DepositHash(dp.Deposit.Data)
		if err != nil {
			t.Fatalf("Unable to determine hashed value of deposit %v", err)
		}

		if err := depositTrie.InsertIntoTrie(depositHash[:], int(dp.Index)); err != nil {
			t.Fatalf("Unable to insert deposit into trie %v", err)
		}

		d.InsertDeposit(ctx, dp.Deposit, big.NewInt(int64(dp.Index)), dp.Index, depositTrie.Root())
	}
	for _, dp := range recentDeposits {
		d.InsertPendingDeposit(ctx, dp.Deposit, big.NewInt(int64(dp.Index)), dp.Index, depositTrie.Root())
	}

	bs := &ProposerServer{
		beaconDB:        d,
		powChainService: p,
		chainService:    newMockChainService(),
	}

	// It should also return the recent deposits after their follow window.
	p.latestBlockNumber = big.NewInt(0).Add(p.latestBlockNumber, big.NewInt(10000))
	deposits, err := bs.deposits(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(deposits) != int(params.BeaconConfig().MaxDeposits) {
		t.Errorf(
			"Received unexpected number of pending deposits: %d, wanted: %d",
			len(deposits),
			int(params.BeaconConfig().MaxDeposits),
		)
	}
}

func TestEth1Data_EmptyVotesFetchBlockHashFailure(t *testing.T) {
	db := internal.SetupDB(t)
	defer internal.TeardownDB(t, db)
	ctx := context.Background()

	proposerServer := &ProposerServer{
		beaconDB: db,
		powChainService: &faultyPOWChainService{
			hashesByHeight: make(map[int][]byte),
		},
	}
	beaconState := &pbp2p.BeaconState{
		Eth1Data: &pbp2p.Eth1Data{
			BlockHash: []byte{'a'},
		},
		Eth1DataVotes: []*pbp2p.Eth1Data{},
	}
	if err := proposerServer.beaconDB.SaveState(ctx, beaconState); err != nil {
		t.Fatal(err)
	}
	want := "could not fetch ETH1_FOLLOW_DISTANCE ancestor"
	if _, err := proposerServer.eth1Data(context.Background()); !strings.Contains(err.Error(), want) {
		t.Errorf("Expected error %v, received %v", want, err)
	}
}

func TestEth1Data_EmptyVotesOk(t *testing.T) {
	beaconDB := internal.SetupDB(t)
	defer internal.TeardownDB(t, beaconDB)
	ctx := context.Background()

	height := big.NewInt(int64(params.BeaconConfig().Eth1FollowDistance))
	deps := []*db.DepositContainer{
		{
			Index: 0,
			Deposit: &pbp2p.Deposit{
				Data: &pbp2p.DepositData{
					Pubkey:                []byte("a"),
					Signature:             mockSig[:],
					WithdrawalCredentials: mockCreds[:],
				}},
		},
		{
			Index: 1,
			Deposit: &pbp2p.Deposit{
				Data: &pbp2p.DepositData{
					Pubkey:                []byte("b"),
					Signature:             mockSig[:],
					WithdrawalCredentials: mockCreds[:],
				}},
		},
	}
	depositTrie, err := trieutil.NewTrie(int(params.BeaconConfig().DepositContractTreeDepth))
	if err != nil {
		t.Fatalf("could not setup deposit trie: %v", err)
	}
	for _, dp := range deps {
		beaconDB.InsertDeposit(context.Background(), dp.Deposit, big.NewInt(0), dp.Index, depositTrie.Root())
	}
	depositRoot := depositTrie.Root()
	beaconState := &pbp2p.BeaconState{
		Eth1Data: &pbp2p.Eth1Data{
			BlockHash:   []byte("hash0"),
			DepositRoot: depositRoot[:],
		},
		Eth1DataVotes: []*pbp2p.Eth1Data{},
	}

	powChainService := &mockPOWChainService{
		latestBlockNumber: height,
		hashesByHeight: map[int][]byte{
			0: []byte("hash0"),
			1: beaconState.Eth1Data.DepositRoot,
		},
	}
	proposerServer := &ProposerServer{
		beaconDB:        beaconDB,
		powChainService: powChainService,
	}

	if err := proposerServer.beaconDB.SaveState(ctx, beaconState); err != nil {
		t.Fatal(err)
	}
	result, err := proposerServer.eth1Data(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	// If the data vote objects are empty, the deposit root should be the one corresponding
	// to the deposit contract in the powchain service, fetched using powChainService.DepositRoot()
	if !bytes.Equal(result.DepositRoot, depositRoot[:]) {
		t.Errorf(
			"Expected deposit roots to match, received %#x == %#x",
			result.DepositRoot,
			depositRoot,
		)
	}
}

func TestEth1Data_NonEmptyVotesSelectsBestVote(t *testing.T) {
	beaconDB := internal.SetupDB(t)
	defer internal.TeardownDB(t, beaconDB)

	ctx := context.Background()
	eth1DataVotes := []*pbp2p.Eth1Data{
		{
			BlockHash:    []byte("block0"),
			DepositRoot:  []byte("deposit0001234567890123456789012"),
			DepositCount: 2,
		},
		{
			BlockHash:    []byte("block1"),
			DepositRoot:  []byte("deposit1001234567890123456789012"),
			DepositCount: 2,
		},
		{
			BlockHash:    []byte("block1"),
			DepositRoot:  []byte("deposit1001234567890123456789012"),
			DepositCount: 2,
		},
		// We include the case in which the vote counts might match, and in that
		// case we break ties by checking which block hash has the highest
		// block height in the eth1.0 chain.
		{
			BlockHash:    []byte("block2"),
			DepositRoot:  []byte("deposit2001234567890123456789012"),
			DepositCount: 2,
		},
		{
			BlockHash:    []byte("block2"),
			DepositRoot:  []byte("deposit2001234567890123456789012"),
			DepositCount: 2,
		},
		{
			BlockHash:    []byte("block2"),
			DepositRoot:  []byte("deposit2001234567890123456789012"),
			DepositCount: 2,
		},
		{
			BlockHash:    []byte("block4"),
			DepositRoot:  []byte("deposit3001234567890123456789012"),
			DepositCount: 2,
		},
		{
			BlockHash:    []byte("block4"),
			DepositRoot:  []byte("deposit3001234567890123456789012"),
			DepositCount: 2,
		},
		{
			BlockHash:    []byte("block4"),
			DepositRoot:  []byte("deposit3001234567890123456789012"),
			DepositCount: 2,
		},
		// We include a case with higher vote count but wrong deposit count
		// that shouldnt be counted at all.
		{
			BlockHash:    []byte("block4"),
			DepositRoot:  []byte("deposit4001234567890123456789012"),
			DepositCount: 1,
		},
		{
			BlockHash:    []byte("block4"),
			DepositRoot:  []byte("deposit4001234567890123456789012"),
			DepositCount: 1,
		},
		{
			BlockHash:    []byte("block4"),
			DepositRoot:  []byte("deposit4001234567890123456789012"),
			DepositCount: 1,
		},
		{
			BlockHash:    []byte("block4"),
			DepositRoot:  []byte("deposit4001234567890123456789012"),
			DepositCount: 1,
		},
	}

	var mockSig [96]byte
	var mockCreds [32]byte
	deposits := []*db.DepositContainer{
		{
			Index: 0,
			Deposit: &pbp2p.Deposit{
				Data: &pbp2p.DepositData{
					Pubkey:                []byte("a"),
					Signature:             mockSig[:],
					WithdrawalCredentials: mockCreds[:],
				}},
		},
		{
			Index: 1,
			Deposit: &pbp2p.Deposit{
				Data: &pbp2p.DepositData{
					Pubkey:                []byte("b"),
					Signature:             mockSig[:],
					WithdrawalCredentials: mockCreds[:],
				}},
		},
	}

	for _, dp := range deposits {
		var root [32]byte
		copy(root[:], eth1DataVotes[dp.Index].DepositRoot)
		beaconDB.InsertDeposit(ctx, dp.Deposit, big.NewInt(int64(dp.Index)), dp.Index, root)
	}
	beaconState := &pbp2p.BeaconState{
		Eth1DataVotes: eth1DataVotes,
		Eth1Data: &pbp2p.Eth1Data{
			BlockHash:   []byte("stub"),
			DepositRoot: []byte("first"),
		},
	}
	if err := beaconDB.SaveState(ctx, beaconState); err != nil {
		t.Fatal(err)
	}
	currentHeight := params.BeaconConfig().Eth1FollowDistance + 9
	proposerServer := &ProposerServer{
		beaconDB: beaconDB,
		powChainService: &mockPOWChainService{
			latestBlockNumber: big.NewInt(int64(currentHeight)),
			hashesByHeight: map[int][]byte{
				0: beaconState.Eth1Data.BlockHash,
				// adding some not relevant blocks heights to test that search works
				1: {1},
				2: beaconState.Eth1DataVotes[0].BlockHash,
				3: {3},
				4: beaconState.Eth1DataVotes[1].BlockHash,
				5: {5},
				6: beaconState.Eth1DataVotes[3].BlockHash,
				7: {7},
				// We will give the hash at index 2 in the beacon state's latest eth1 votes
				// priority in being selected as the best vote by giving it the highest block number.
				8: beaconState.Eth1DataVotes[2].BlockHash,
				9: {9},
			},
		},
	}

	eth1data, err := proposerServer.eth1Data(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	// Vote at index 2 should have won the best vote selection mechanism as it had the highest block number
	// despite being tied at vote count with the vote at index 3.
	if !bytes.Equal(eth1data.BlockHash, beaconState.Eth1DataVotes[2].BlockHash) {
		t.Errorf(
			"Expected block hashes to match, received %#x == %#x",
			eth1data.BlockHash,
			beaconState.Eth1DataVotes[2].BlockHash,
		)
	}
	if !bytes.Equal(eth1data.DepositRoot, beaconState.Eth1DataVotes[2].DepositRoot) {
		t.Errorf(
			"Expected deposit roots to match, received %#x == %#x",
			eth1data.DepositRoot,
			beaconState.Eth1DataVotes[2].DepositRoot,
		)
	}
}

func Benchmark_Eth1Data(b *testing.B) {
	beaconDB := internal.SetupDB(b)
	defer internal.TeardownDB(b, beaconDB)
	ctx := context.Background()

	hashesByHeight := make(map[int][]byte)

	beaconState := &pbp2p.BeaconState{
		Eth1DataVotes: []*pbp2p.Eth1Data{},
		Eth1Data: &pbp2p.Eth1Data{
			BlockHash: []byte("stub"),
		},
	}
	var mockSig [96]byte
	var mockCreds [32]byte
	deposits := []*db.DepositContainer{
		{
			Index: 0,
			Deposit: &pbp2p.Deposit{
				Data: &pbp2p.DepositData{
					Pubkey:                []byte("a"),
					Signature:             mockSig[:],
					WithdrawalCredentials: mockCreds[:],
				}},
		},
		{
			Index: 1,
			Deposit: &pbp2p.Deposit{
				Data: &pbp2p.DepositData{
					Pubkey:                []byte("b"),
					Signature:             mockSig[:],
					WithdrawalCredentials: mockCreds[:],
				}},
		},
	}

	for i, dp := range deposits {
		var root [32]byte
		copy(root[:], []byte{'d', 'e', 'p', 'o', 's', 'i', 't', byte(i)})
		beaconDB.InsertDeposit(ctx, dp.Deposit, big.NewInt(int64(dp.Index)), dp.Index, root)
	}
	numOfVotes := 1000
	for i := 0; i < numOfVotes; i++ {
		blockhash := []byte{'b', 'l', 'o', 'c', 'k', byte(i)}
		deposit := []byte{'d', 'e', 'p', 'o', 's', 'i', 't', byte(i)}
		beaconState.Eth1DataVotes = append(beaconState.Eth1DataVotes, &pbp2p.Eth1Data{
			BlockHash:   blockhash,
			DepositRoot: deposit,
		})
		hashesByHeight[i] = blockhash
	}
	hashesByHeight[numOfVotes+1] = []byte("stub")

	if err := beaconDB.SaveState(ctx, beaconState); err != nil {
		b.Fatal(err)
	}
	currentHeight := params.BeaconConfig().Eth1FollowDistance + 5
	proposerServer := &ProposerServer{
		beaconDB: beaconDB,
		powChainService: &mockPOWChainService{
			latestBlockNumber: big.NewInt(int64(currentHeight)),
			hashesByHeight:    hashesByHeight,
		},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := proposerServer.eth1Data(context.Background())
		if err != nil {
			b.Fatal(err)
		}
	}
}
