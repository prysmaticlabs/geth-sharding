package aggregator

import (
	"context"
	"reflect"
	"strings"
	"testing"

	ethpb "github.com/prysmaticlabs/ethereumapis/eth/v1alpha1"
	"github.com/prysmaticlabs/go-bitfield"
	"github.com/prysmaticlabs/go-ssz"
	mock "github.com/prysmaticlabs/prysm/beacon-chain/blockchain/testing"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	dbutil "github.com/prysmaticlabs/prysm/beacon-chain/db/testing"
	"github.com/prysmaticlabs/prysm/beacon-chain/operations/attestations"
	mockp2p "github.com/prysmaticlabs/prysm/beacon-chain/p2p/testing"
	mockSync "github.com/prysmaticlabs/prysm/beacon-chain/sync/initial-sync/testing"
	pbp2p "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/rpc/v1"
	"github.com/prysmaticlabs/prysm/shared/bls"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/testutil"
)

func init() {
	// Use minimal config to reduce test setup time.
	params.OverrideBeaconConfig(params.MinimalSpecConfig())
}

func TestSubmitAggregateAndProof_Syncing(t *testing.T) {
	db := dbutil.SetupDB(t)
	defer dbutil.TeardownDB(t, db)
	ctx := context.Background()

	s := &pbp2p.BeaconState{}

	aggregatorServer := &Server{
		HeadFetcher: &mock.ChainService{State: s},
		SyncChecker: &mockSync.Sync{IsSyncing: true},
		BeaconDB:    db,
	}

	req := &pb.AggregationRequest{CommitteeIndex: 1}
	wanted := "Syncing to latest head, not ready to respond"
	if _, err := aggregatorServer.SubmitAggregateAndProof(ctx, req); !strings.Contains(err.Error(), wanted) {
		t.Error("Did not receive wanted error")
	}
}

func TestSubmitAggregateAndProof_CantFindValidatorIndex(t *testing.T) {
	db := dbutil.SetupDB(t)
	defer dbutil.TeardownDB(t, db)
	ctx := context.Background()

	s := &pbp2p.BeaconState{
		RandaoMixes: make([][]byte, params.BeaconConfig().EpochsPerHistoricalVector),
	}

	aggregatorServer := &Server{
		HeadFetcher: &mock.ChainService{State: s},
		SyncChecker: &mockSync.Sync{IsSyncing: false},
		BeaconDB:    db,
	}

	priv := bls.RandKey()
	sig := priv.Sign([]byte{'A'}, 0)
	req := &pb.AggregationRequest{CommitteeIndex: 1, SlotSignature: sig.Marshal()}
	wanted := "Could not locate validator index in DB"
	if _, err := aggregatorServer.SubmitAggregateAndProof(ctx, req); !strings.Contains(err.Error(), wanted) {
		t.Error("Did not receive wanted error")
	}
}

func TestSubmitAggregateAndProof_IsAggregator(t *testing.T) {
	db := dbutil.SetupDB(t)
	defer dbutil.TeardownDB(t, db)
	ctx := context.Background()

	s := &pbp2p.BeaconState{
		RandaoMixes: make([][]byte, params.BeaconConfig().EpochsPerHistoricalVector),
	}

	aggregatorServer := &Server{
		HeadFetcher: &mock.ChainService{State: s},
		SyncChecker: &mockSync.Sync{IsSyncing: false},
		BeaconDB:    db,
		AttPool:     attestations.NewPool(),
	}

	priv := bls.RandKey()
	sig := priv.Sign([]byte{'A'}, 0)
	pubKey := [48]byte{'A'}
	req := &pb.AggregationRequest{CommitteeIndex: 1, SlotSignature: sig.Marshal(), PublicKey: pubKey[:]}
	if err := aggregatorServer.BeaconDB.SaveValidatorIndex(ctx, pubKey, 100); err != nil {
		t.Fatal(err)
	}

	if _, err := aggregatorServer.SubmitAggregateAndProof(ctx, req); err != nil {
		t.Fatal(err)
	}
}

func TestSubmitAggregateAndProof_AggregateOk(t *testing.T) {
	params.UseMinimalConfig()
	c := params.MinimalSpecConfig()
	c.TargetAggregatorsPerCommittee = 16
	params.OverrideBeaconConfig(c)
	defer params.UseMainnetConfig()

	db := dbutil.SetupDB(t)
	defer dbutil.TeardownDB(t, db)
	ctx := context.Background()

	beaconState, privKeys := testutil.DeterministicGenesisState(t, 32)
	att0 := generateAtt(beaconState, 0, privKeys)
	att1 := generateAtt(beaconState, 1, privKeys)

	beaconState.Slot += params.BeaconConfig().MinAttestationInclusionDelay

	aggregatorServer := &Server{
		HeadFetcher: &mock.ChainService{State: beaconState},
		SyncChecker: &mockSync.Sync{IsSyncing: false},
		BeaconDB:    db,
		AttPool:     attestations.NewPool(),
		P2p:         &mockp2p.MockBroadcaster{},
	}

	priv := bls.RandKey()
	sig := priv.Sign([]byte{'B'}, 0)
	pubKey := [48]byte{'B'}
	req := &pb.AggregationRequest{CommitteeIndex: 1, SlotSignature: sig.Marshal(), PublicKey: pubKey[:]}
	if err := aggregatorServer.BeaconDB.SaveValidatorIndex(ctx, pubKey, 100); err != nil {
		t.Fatal(err)
	}

	if err := aggregatorServer.AttPool.SaveUnaggregatedAttestation(att0); err != nil {
		t.Fatal(err)
	}
	if err := aggregatorServer.AttPool.SaveUnaggregatedAttestation(att1); err != nil {
		t.Fatal(err)
	}

	if _, err := aggregatorServer.SubmitAggregateAndProof(ctx, req); err != nil {
		t.Fatal(err)
	}

	aggregatedAtts := aggregatorServer.AttPool.AggregatedAttestation()
	wanted, err := helpers.AggregateAttestation(att0, att1)
	if err != nil {
		t.Fatal(err)
	}
	if reflect.DeepEqual(aggregatedAtts, wanted) {
		t.Error("Did not receive wanted attestation")
	}
}

func TestSubmitAggregateAndProof_AggregateNotOk(t *testing.T) {
	params.UseMinimalConfig()
	c := params.MinimalSpecConfig()
	c.TargetAggregatorsPerCommittee = 16
	params.OverrideBeaconConfig(c)
	defer params.UseMainnetConfig()

	db := dbutil.SetupDB(t)
	defer dbutil.TeardownDB(t, db)
	ctx := context.Background()

	beaconState, privKeys := testutil.DeterministicGenesisState(t, 32)
	att0 := generateAtt(beaconState, 0, privKeys)

	beaconState.Slot += params.BeaconConfig().MinAttestationInclusionDelay

	aggregatorServer := &Server{
		HeadFetcher: &mock.ChainService{State: beaconState},
		SyncChecker: &mockSync.Sync{IsSyncing: false},
		BeaconDB:    db,
		AttPool:     attestations.NewPool(),
		P2p:         &mockp2p.MockBroadcaster{},
	}

	priv := bls.RandKey()
	sig := priv.Sign([]byte{'B'}, 0)
	pubKey := [48]byte{'B'}
	req := &pb.AggregationRequest{CommitteeIndex: 1, SlotSignature: sig.Marshal(), PublicKey: pubKey[:]}
	if err := aggregatorServer.BeaconDB.SaveValidatorIndex(ctx, pubKey, 100); err != nil {
		t.Fatal(err)
	}

	if err := aggregatorServer.AttPool.SaveUnaggregatedAttestation(att0); err != nil {
		t.Fatal(err)
	}

	if _, err := aggregatorServer.SubmitAggregateAndProof(ctx, req); err != nil {
		t.Fatal(err)
	}

	aggregatedAtts := aggregatorServer.AttPool.AggregatedAttestation()
	if len(aggregatedAtts) != 0 {
		t.Errorf("Wanted aggregated attestation 0, got %d", len(aggregatedAtts))
	}
}

func generateAtt(state *pbp2p.BeaconState, index uint64, privKeys []*bls.SecretKey) *ethpb.Attestation {
	aggBits := bitfield.NewBitlist(4)
	aggBits.SetBitAt(index, true)
	att := &ethpb.Attestation{
		Data: &ethpb.AttestationData{
			CommitteeIndex: 1,
			Source:         &ethpb.Checkpoint{Epoch: 0, Root: params.BeaconConfig().ZeroHash[:]},
			Target:         &ethpb.Checkpoint{Epoch: 0},
		},
		AggregationBits: aggBits,
	}
	attestingIndices, _ := helpers.AttestingIndices(state, att.Data, att.AggregationBits)
	domain := helpers.Domain(state.Fork, 0, params.BeaconConfig().DomainBeaconAttester)

	sigs := make([]*bls.Signature, len(attestingIndices))
	zeroSig := [96]byte{}
	att.Signature = zeroSig[:]

	for i, indice := range attestingIndices {
		hashTreeRoot, _ := ssz.HashTreeRoot(att.Data)
		sig := privKeys[indice].Sign(hashTreeRoot[:], domain)
		sigs[i] = sig
	}

	att.Signature = bls.AggregateSignatures(sigs).Marshal()[:]

	return att
}
