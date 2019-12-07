package sync

import (
	"context"
	"crypto/rand"
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
)

func setupValidProposerSlashing(t *testing.T) (*ethpb.ProposerSlashing, *pb.BeaconState) {
	validators := make([]*ethpb.Validator, 100)
	for i := 0; i < len(validators); i++ {
		validators[i] = &ethpb.Validator{
			EffectiveBalance:  params.BeaconConfig().MaxEffectiveBalance,
			Slashed:           false,
			ExitEpoch:         params.BeaconConfig().FarFutureEpoch,
			WithdrawableEpoch: params.BeaconConfig().FarFutureEpoch,
			ActivationEpoch:   0,
		}
	}
	validatorBalances := make([]uint64, len(validators))
	for i := 0; i < len(validatorBalances); i++ {
		validatorBalances[i] = params.BeaconConfig().MaxEffectiveBalance
	}

	currentSlot := uint64(0)
	state := &pb.BeaconState{
		Validators: validators,
		Slot:       currentSlot,
		Balances:   validatorBalances,
		Fork: &pb.Fork{
			CurrentVersion:  params.BeaconConfig().GenesisForkVersion,
			PreviousVersion: params.BeaconConfig().GenesisForkVersion,
			Epoch:           0,
		},
		Slashings:   make([]uint64, params.BeaconConfig().EpochsPerSlashingsVector),
		RandaoMixes: make([][]byte, params.BeaconConfig().EpochsPerHistoricalVector),

		StateRoots:        make([][]byte, params.BeaconConfig().SlotsPerHistoricalRoot),
		BlockRoots:        make([][]byte, params.BeaconConfig().SlotsPerHistoricalRoot),
		LatestBlockHeader: &ethpb.BeaconBlockHeader{},
	}

	domain := helpers.Domain(
		state.Fork,
		helpers.CurrentEpoch(state),
		params.BeaconConfig().DomainBeaconProposer,
	)
	privKey := bls.RandKey()

	header1 := &ethpb.BeaconBlockHeader{
		Slot:      0,
		StateRoot: []byte("A"),
	}
	signingRoot, err := ssz.SigningRoot(header1)
	if err != nil {
		t.Errorf("Could not get signing root of beacon block header: %v", err)
	}
	header1.Signature = privKey.Sign(signingRoot[:], domain).Marshal()[:]

	header2 := &ethpb.BeaconBlockHeader{
		Slot:      0,
		StateRoot: []byte("B"),
	}
	signingRoot, err = ssz.SigningRoot(header2)
	if err != nil {
		t.Errorf("Could not get signing root of beacon block header: %v", err)
	}
	header2.Signature = privKey.Sign(signingRoot[:], domain).Marshal()[:]

	slashing := &ethpb.ProposerSlashing{
		ProposerIndex: 1,
		Header_1:      header1,
		Header_2:      header2,
	}

	state.Validators[1].PublicKey = privKey.PublicKey().Marshal()[:]

	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}

	return slashing, state
}

func TestValidateProposerSlashing_ValidSlashing(t *testing.T) {
	p2p := p2ptest.NewTestP2P(t)
	ctx := context.Background()

	slashing, s := setupValidProposerSlashing(t)

	r := &RegularSync{
		p2p:         p2p,
		chain:       &mock.ChainService{State: s},
		initialSync: &mockSync.Sync{IsSyncing: false},
	}

	valid, err := r.validateProposerSlashing(ctx, slashing, p2p, false /*fromSelf*/)
	if err != nil {
		t.Errorf("Failed validation: %v", err)
	}
	if !valid {
		t.Error("Failed validation")
	}

	if !p2p.BroadcastCalled {
		t.Error("Broadcast was not called")
	}

	time.Sleep(100 * time.Millisecond)
	// A second message with the same information should not be valid for processing or
	// propagation.
	p2p.BroadcastCalled = false
	valid, _ = r.validateProposerSlashing(ctx, slashing, p2p, false /*fromSelf*/)
	if valid {
		t.Error("Passed validation when should have failed")
	}

	if p2p.BroadcastCalled {
		t.Error("broadcast was called when it should not have been called")
	}
}

func TestValidateProposerSlashing_ValidSlashing_FromSelf(t *testing.T) {
	p2p := p2ptest.NewTestP2P(t)
	ctx := context.Background()

	slashing, s := setupValidProposerSlashing(t)

	r := &RegularSync{
		p2p:         p2p,
		chain:       &mock.ChainService{State: s},
		initialSync: &mockSync.Sync{IsSyncing: false},
	}

	valid, _ := r.validateProposerSlashing(ctx, slashing, p2p, true /*fromSelf*/)
	if valid {
		t.Error("Did not fail validation")
	}

	if p2p.BroadcastCalled {
		t.Error("Broadcast was called")
	}
}

func TestValidateProposerSlashing_ContextTimeout(t *testing.T) {
	p2p := p2ptest.NewTestP2P(t)

	slashing, state := setupValidProposerSlashing(t)
	slashing.Header_1.Slot = 100000000

	ctx, _ := context.WithTimeout(context.Background(), 100*time.Millisecond)

	r := &RegularSync{
		p2p:         p2p,
		chain:       &mock.ChainService{State: state},
		initialSync: &mockSync.Sync{IsSyncing: false},
	}

	valid, _ := r.validateProposerSlashing(ctx, slashing, p2p, false /*fromSelf*/)
	if valid {
		t.Error("slashing from the far distant future should have timed out and returned false")
	}
}

func TestValidateProposerSlashing_Syncing(t *testing.T) {
	p2p := p2ptest.NewTestP2P(t)
	ctx := context.Background()

	slashing, s := setupValidProposerSlashing(t)

	r := &RegularSync{
		p2p:         p2p,
		chain:       &mock.ChainService{State: s},
		initialSync: &mockSync.Sync{IsSyncing: true},
	}

	valid, _ := r.validateProposerSlashing(ctx, slashing, p2p, false /*fromSelf*/)
	if valid {
		t.Error("Did not fail validation")
	}

	if p2p.BroadcastCalled {
		t.Error("Broadcast was called")
	}
}
