package attester

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/gogo/protobuf/proto"
	ethpb "github.com/prysmaticlabs/ethereumapis/eth/v1alpha1"
	"github.com/prysmaticlabs/go-ssz"
	mock "github.com/prysmaticlabs/prysm/beacon-chain/blockchain/testing"
	"github.com/prysmaticlabs/prysm/beacon-chain/cache"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	dbutil "github.com/prysmaticlabs/prysm/beacon-chain/db/testing"
	mockOps "github.com/prysmaticlabs/prysm/beacon-chain/operations/testing"
	mockp2p "github.com/prysmaticlabs/prysm/beacon-chain/p2p/testing"
	mockSync "github.com/prysmaticlabs/prysm/beacon-chain/sync/initial-sync/testing"
	pbp2p "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/rpc/v1"
	"github.com/prysmaticlabs/prysm/shared/featureconfig"
	"github.com/prysmaticlabs/prysm/shared/params"
)

func init() {
	// Use minimal config to reduce test setup time.
	params.OverrideBeaconConfig(params.MinimalSpecConfig())
}

func TestSubmitAttestation_OK(t *testing.T) {
	db := dbutil.SetupDB(t)
	defer dbutil.TeardownDB(t, db)
	ctx := context.Background()

	attesterServer := &Server{
		HeadFetcher:       &mock.ChainService{},
		AttReceiver:       &mock.ChainService{},
		OperationsHandler: &mockOps.Operations{},
		P2p:               &mockp2p.MockBroadcaster{},
		BeaconDB:          db,
		AttestationCache:  cache.NewAttestationCache(),
	}
	head := &ethpb.BeaconBlock{
		Slot:       999,
		ParentRoot: []byte{'a'},
	}
	if err := db.SaveBlock(ctx, head); err != nil {
		t.Fatal(err)
	}
	root, err := ssz.SigningRoot(head)
	if err != nil {
		t.Fatal(err)
	}

	validators := make([]*ethpb.Validator, params.BeaconConfig().MinGenesisActiveValidatorCount/16)
	for i := 0; i < len(validators); i++ {
		validators[i] = &ethpb.Validator{
			ExitEpoch:        params.BeaconConfig().FarFutureEpoch,
			EffectiveBalance: params.BeaconConfig().MaxEffectiveBalance,
		}
	}

	state := &pbp2p.BeaconState{
		Slot:        params.BeaconConfig().SlotsPerEpoch + 1,
		Validators:  validators,
		RandaoMixes: make([][]byte, params.BeaconConfig().EpochsPerHistoricalVector),
	}

	if err := db.SaveState(ctx, state, root); err != nil {
		t.Fatal(err)
	}
	if err := db.SaveHeadBlockRoot(ctx, root); err != nil {
		t.Fatal(err)
	}

	req := &ethpb.Attestation{
		Data: &ethpb.AttestationData{
			BeaconBlockRoot: root[:],
			Source:          &ethpb.Checkpoint{},
			Target:          &ethpb.Checkpoint{},
		},
	}
	if _, err := attesterServer.SubmitAttestation(context.Background(), req); err != nil {
		t.Errorf("Could not attest head correctly: %v", err)
	}
}

func TestRequestAttestation_OK(t *testing.T) {
	block := &ethpb.BeaconBlock{
		Slot: 3*params.BeaconConfig().SlotsPerEpoch + 1,
	}
	targetBlock := &ethpb.BeaconBlock{
		Slot: 1 * params.BeaconConfig().SlotsPerEpoch,
	}
	justifiedBlock := &ethpb.BeaconBlock{
		Slot: 2 * params.BeaconConfig().SlotsPerEpoch,
	}
	blockRoot, err := ssz.SigningRoot(block)
	if err != nil {
		t.Fatalf("Could not hash beacon block: %v", err)
	}
	justifiedRoot, err := ssz.SigningRoot(justifiedBlock)
	if err != nil {
		t.Fatalf("Could not get signing root for justified block: %v", err)
	}
	targetRoot, err := ssz.SigningRoot(targetBlock)
	if err != nil {
		t.Fatalf("Could not get signing root for target block: %v", err)
	}

	beaconState := &pbp2p.BeaconState{
		Slot:       3*params.BeaconConfig().SlotsPerEpoch + 1,
		BlockRoots: make([][]byte, params.BeaconConfig().SlotsPerHistoricalRoot),
		CurrentJustifiedCheckpoint: &ethpb.Checkpoint{
			Epoch: 2,
			Root:  justifiedRoot[:],
		},
	}
	beaconState.BlockRoots[1] = blockRoot[:]
	beaconState.BlockRoots[1*params.BeaconConfig().SlotsPerEpoch] = targetRoot[:]
	beaconState.BlockRoots[2*params.BeaconConfig().SlotsPerEpoch] = justifiedRoot[:]
	attesterServer := &Server{
		P2p:              &mockp2p.MockBroadcaster{},
		SyncChecker:      &mockSync.Sync{IsSyncing: false},
		AttestationCache: cache.NewAttestationCache(),
		HeadFetcher:      &mock.ChainService{State: beaconState, Root: blockRoot[:]},
		AttReceiver:      &mock.ChainService{State: beaconState, Root: blockRoot[:]},
	}

	req := &pb.AttestationRequest{
		CommitteeIndex: 0,
		Slot:           3*params.BeaconConfig().SlotsPerEpoch + 1,
	}
	res, err := attesterServer.RequestAttestation(context.Background(), req)
	if err != nil {
		t.Fatalf("Could not get attestation info at slot: %v", err)
	}

	expectedInfo := &ethpb.AttestationData{
		Slot:            3*params.BeaconConfig().SlotsPerEpoch + 1,
		BeaconBlockRoot: blockRoot[:],
		Source: &ethpb.Checkpoint{
			Epoch: 2,
			Root:  justifiedRoot[:],
		},
		Target: &ethpb.Checkpoint{
			Epoch: 3,
		},
	}

	if !proto.Equal(res, expectedInfo) {
		t.Errorf("Expected attestation info to match, received %v, wanted %v", res, expectedInfo)
	}
}

func TestRequestAttestation_SyncNotReady(t *testing.T) {
	as := &Server{
		SyncChecker: &mockSync.Sync{IsSyncing: true},
	}
	_, err := as.RequestAttestation(context.Background(), &pb.AttestationRequest{})
	if strings.Contains(err.Error(), "syncing to latest head") {
		t.Error("Did not get wanted error")
	}
}

func TestAttestationDataAtSlot_handlesFarAwayJustifiedEpoch(t *testing.T) {
	// Scenario:
	//
	// State slot = 10000
	// Last justified slot = epoch start of 1500
	// HistoricalRootsLimit = 8192
	//
	// More background: https://github.com/prysmaticlabs/prysm/issues/2153
	// This test breaks if it doesnt use mainnet config
	params.OverrideBeaconConfig(params.MainnetConfig())
	defer params.OverrideBeaconConfig(params.MinimalSpecConfig())

	// Ensure HistoricalRootsLimit matches scenario
	cfg := params.BeaconConfig()
	cfg.HistoricalRootsLimit = 8192
	params.OverrideBeaconConfig(cfg)

	block := &ethpb.BeaconBlock{
		Slot: 10000,
	}
	epochBoundaryBlock := &ethpb.BeaconBlock{
		Slot: helpers.StartSlot(helpers.SlotToEpoch(10000)),
	}
	justifiedBlock := &ethpb.BeaconBlock{
		Slot: helpers.StartSlot(helpers.SlotToEpoch(1500)) - 2, // Imagine two skip block
	}
	blockRoot, err := ssz.SigningRoot(block)
	if err != nil {
		t.Fatalf("Could not hash beacon block: %v", err)
	}
	justifiedBlockRoot, err := ssz.SigningRoot(justifiedBlock)
	if err != nil {
		t.Fatalf("Could not hash justified block: %v", err)
	}
	epochBoundaryRoot, err := ssz.SigningRoot(epochBoundaryBlock)
	if err != nil {
		t.Fatalf("Could not hash justified block: %v", err)
	}
	beaconState := &pbp2p.BeaconState{
		Slot:       10000,
		BlockRoots: make([][]byte, params.BeaconConfig().SlotsPerHistoricalRoot),
		CurrentJustifiedCheckpoint: &ethpb.Checkpoint{
			Epoch: helpers.SlotToEpoch(1500),
			Root:  justifiedBlockRoot[:],
		},
	}
	beaconState.BlockRoots[1] = blockRoot[:]
	beaconState.BlockRoots[1*params.BeaconConfig().SlotsPerEpoch] = epochBoundaryRoot[:]
	beaconState.BlockRoots[2*params.BeaconConfig().SlotsPerEpoch] = justifiedBlockRoot[:]
	attesterServer := &Server{
		P2p:              &mockp2p.MockBroadcaster{},
		AttestationCache: cache.NewAttestationCache(),
		HeadFetcher:      &mock.ChainService{State: beaconState, Root: blockRoot[:]},
		AttReceiver:      &mock.ChainService{State: beaconState, Root: blockRoot[:]},
		SyncChecker:      &mockSync.Sync{IsSyncing: false},
	}

	req := &pb.AttestationRequest{
		CommitteeIndex: 0,
		Slot:           10000,
	}
	res, err := attesterServer.RequestAttestation(context.Background(), req)
	if err != nil {
		t.Fatalf("Could not get attestation info at slot: %v", err)
	}

	expectedInfo := &ethpb.AttestationData{
		Slot:            req.Slot,
		BeaconBlockRoot: blockRoot[:],
		Source: &ethpb.Checkpoint{
			Epoch: helpers.SlotToEpoch(1500),
			Root:  justifiedBlockRoot[:],
		},
		Target: &ethpb.Checkpoint{
			Epoch: 312,
		},
	}

	if !proto.Equal(res, expectedInfo) {
		t.Errorf("Expected attestation info to match, received %v, wanted %v", res, expectedInfo)
	}
}

func TestAttestationDataAtSlot_handlesInProgressRequest(t *testing.T) {
	// Cache toggled by feature flag for now. See https://github.com/prysmaticlabs/prysm/issues/3106.
	featureconfig.Init(&featureconfig.Flags{
		EnableAttestationCache: true,
	})
	defer func() {
		featureconfig.Init(nil)
	}()

	ctx := context.Background()
	server := &Server{
		AttestationCache: cache.NewAttestationCache(),
		SyncChecker:      &mockSync.Sync{IsSyncing: false},
	}

	req := &pb.AttestationRequest{
		CommitteeIndex: 1,
		Slot:           2,
	}

	res := &ethpb.AttestationData{
		Target: &ethpb.Checkpoint{Epoch: 55},
	}

	if err := server.AttestationCache.MarkInProgress(req); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		response, err := server.RequestAttestation(ctx, req)
		if err != nil {
			t.Error(err)
		}
		if !proto.Equal(res, response) {
			t.Error("Expected  equal responses from cache")
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()

		if err := server.AttestationCache.Put(ctx, req, res); err != nil {
			t.Error(err)
		}
		if err := server.AttestationCache.MarkNotInProgress(req); err != nil {
			t.Error(err)
		}
	}()

	wg.Wait()
}
