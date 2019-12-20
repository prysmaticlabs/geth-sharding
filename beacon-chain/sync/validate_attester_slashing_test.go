package sync

import (
	"context"
	"math/rand"
	"testing"
	"time"

	ethpb "github.com/prysmaticlabs/ethereumapis/eth/v1alpha1"
	"github.com/prysmaticlabs/go-ssz"
	mock "github.com/prysmaticlabs/prysm/beacon-chain/blockchain/testing"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	p2ptest "github.com/prysmaticlabs/prysm/beacon-chain/p2p/testing"
	mockSync "github.com/prysmaticlabs/prysm/beacon-chain/sync/initial-sync/testing"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	"github.com/prysmaticlabs/prysm/shared/bls"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/testutil"
)

func setupValidAttesterSlashing(t *testing.T) (*ethpb.AttesterSlashing, *pb.BeaconState) {
	state, privKeys := testutil.DeterministicGenesisState(t, 5)
	for _, vv := range state.Validators {
		vv.WithdrawableEpoch = 1 * params.BeaconConfig().SlotsPerEpoch
	}

	att1 := &ethpb.IndexedAttestation{
		Data: &ethpb.AttestationData{
			Source: &ethpb.Checkpoint{Epoch: 1},
			Target: &ethpb.Checkpoint{Epoch: 0},
		},
		AttestingIndices: []uint64{0, 1},
	}
	hashTreeRoot, err := ssz.HashTreeRoot(att1.Data)
	if err != nil {
		t.Error(err)
	}
	domain := helpers.Domain(state.Fork, 0, params.BeaconConfig().DomainBeaconAttester)
	sig0 := privKeys[0].Sign(hashTreeRoot[:], domain)
	sig1 := privKeys[1].Sign(hashTreeRoot[:], domain)
	aggregateSig := bls.AggregateSignatures([]*bls.Signature{sig0, sig1})
	att1.Signature = aggregateSig.Marshal()[:]

	att2 := &ethpb.IndexedAttestation{
		Data: &ethpb.AttestationData{
			Source: &ethpb.Checkpoint{Epoch: 0},
			Target: &ethpb.Checkpoint{Epoch: 0},
		},
		AttestingIndices: []uint64{0, 1},
	}
	hashTreeRoot, err = ssz.HashTreeRoot(att2.Data)
	if err != nil {
		t.Error(err)
	}
	sig0 = privKeys[0].Sign(hashTreeRoot[:], domain)
	sig1 = privKeys[1].Sign(hashTreeRoot[:], domain)
	aggregateSig = bls.AggregateSignatures([]*bls.Signature{sig0, sig1})
	att2.Signature = aggregateSig.Marshal()[:]

	slashing := &ethpb.AttesterSlashing{
		Attestation_1: att1,
		Attestation_2: att2,
	}

	currentSlot := 2 * params.BeaconConfig().SlotsPerEpoch
	state.Slot = currentSlot

	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}

	return slashing, state
}

func TestValidateAttesterSlashing_ValidSlashing(t *testing.T) {
	p2p := p2ptest.NewTestP2P(t)
	ctx := context.Background()

	slashing, s := setupValidAttesterSlashing(t)

	r := &Service{
		p2p:         p2p,
		chain:       &mock.ChainService{State: s},
		initialSync: &mockSync.Sync{IsSyncing: false},
	}

	valid, err := r.validateAttesterSlashing(ctx, slashing, p2p, false /*fromSelf*/)
	if err != nil {
		t.Errorf("Failed validation: %v", err)
	}
	if !valid {
		t.Error("Failed Validation")
	}

	if !p2p.BroadcastCalled {
		t.Error("Broadcast was not called")
	}

	time.Sleep(100 * time.Millisecond)
	// A second message with the same information should not be valid for processing or
	// propagation.
	p2p.BroadcastCalled = false
	valid, _ = r.validateAttesterSlashing(ctx, slashing, p2p, false /*fromSelf*/)

	if valid {
		t.Error("Passed validation when should have failed")
	}

	if p2p.BroadcastCalled {
		t.Error("broadcast was called when it should not have been called")
	}
}

func TestValidateAttesterSlashing_ValidSlashing_FromSelf(t *testing.T) {
	p2p := p2ptest.NewTestP2P(t)
	ctx := context.Background()

	slashing, s := setupValidAttesterSlashing(t)

	r := &Service{
		p2p:         p2p,
		chain:       &mock.ChainService{State: s},
		initialSync: &mockSync.Sync{IsSyncing: false},
	}

	valid, _ := r.validateAttesterSlashing(ctx, slashing, p2p, true /*fromSelf*/)
	if valid {
		t.Error("Passed validation")
	}

	if p2p.BroadcastCalled {
		t.Error("Broadcast was called")
	}
}

func TestValidateAttesterSlashing_ContextTimeout(t *testing.T) {
	p2p := p2ptest.NewTestP2P(t)

	slashing, state := setupValidAttesterSlashing(t)
	slashing.Attestation_1.Data.Target.Epoch = 100000000

	ctx, _ := context.WithTimeout(context.Background(), 100*time.Millisecond)

	r := &Service{
		p2p:         p2p,
		chain:       &mock.ChainService{State: state},
		initialSync: &mockSync.Sync{IsSyncing: false},
	}

	valid, _ := r.validateAttesterSlashing(ctx, slashing, p2p, false /*fromSelf*/)
	if valid {
		t.Error("slashing from the far distant future should have timed out and returned false")
	}
}

func TestValidateAttesterSlashing_Syncing(t *testing.T) {
	p2p := p2ptest.NewTestP2P(t)
	ctx := context.Background()

	slashing, s := setupValidAttesterSlashing(t)

	r := &Service{
		p2p:         p2p,
		chain:       &mock.ChainService{State: s},
		initialSync: &mockSync.Sync{IsSyncing: true},
	}

	valid, _ := r.validateAttesterSlashing(ctx, slashing, p2p, false /*fromSelf*/)
	if valid {
		t.Error("Passed validation")
	}

	if p2p.BroadcastCalled {
		t.Error("Broadcast was called")
	}
}
