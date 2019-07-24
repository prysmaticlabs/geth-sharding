package blocks_test

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io/ioutil"
	"reflect"
	"strings"
	"testing"

	"github.com/gogo/protobuf/proto"
	"github.com/prysmaticlabs/go-bitfield"
	"github.com/prysmaticlabs/go-ssz"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/blocks"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/state"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	ethpb "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/shared/bls"
	"github.com/prysmaticlabs/prysm/shared/hashutil"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/testutil"
	"github.com/prysmaticlabs/prysm/shared/trieutil"
	"github.com/sirupsen/logrus"
	"gopkg.in/d4l3k/messagediff.v1"
)

func init() {
	logrus.SetOutput(ioutil.Discard) // Ignore "validator activated" logs
}

func TestProcessBlockHeader_WrongProposerSig(t *testing.T) {
	t.Skip("Skip until bls.Verify is finished")
	// TODO(#2307) unskip after bls.Verify is finished
	if params.BeaconConfig().SlotsPerEpoch != 64 {
		t.Errorf("SlotsPerEpoch should be 64 for these tests to pass")
	}

	validators := make([]*ethpb.Validator, params.BeaconConfig().MinGenesisActiveValidatorCount)
	for i := 0; i < len(validators); i++ {
		validators[i] = &ethpb.Validator{
			ExitEpoch: params.BeaconConfig().FarFutureEpoch,
			Slashed:   true,
		}
	}

	state := &pb.BeaconState{
		Validators:        validators,
		Slot:              0,
		LatestBlockHeader: &ethpb.BeaconBlockHeader{Slot: 9},
		Fork: &pb.Fork{
			PreviousVersion: []byte{0, 0, 0, 0},
			CurrentVersion:  []byte{0, 0, 0, 0},
		},
		RandaoMixes:      make([][]byte, params.BeaconConfig().EpochsPerHistoricalVector),
		ActiveIndexRoots: make([][]byte, params.BeaconConfig().EpochsPerHistoricalVector),
	}

	validators[5896].Slashed = false

	lbhsr, err := ssz.HashTreeRoot(state.LatestBlockHeader)
	if err != nil {
		t.Error(err)
	}
	currentEpoch := helpers.CurrentEpoch(state)
	dt := helpers.Domain(state, currentEpoch, params.BeaconConfig().DomainBeaconProposer)
	priv, err := bls.RandKey(rand.Reader)
	if err != nil {
		t.Errorf("failed to generate private key got: %v", err)
	}
	priv2, err := bls.RandKey(rand.Reader)
	if err != nil {
		t.Errorf("failed to generate private key got: %v", err)
	}
	validators[5896].PublicKey = priv.PublicKey().Marshal()
	block := &ethpb.BeaconBlock{
		Slot: 0,
		Body: &ethpb.BeaconBlockBody{
			RandaoReveal: []byte{'A', 'B', 'C'},
		},
		ParentRoot: lbhsr[:],
	}
	signingRoot, err := ssz.SigningRoot(block)
	if err != nil {
		t.Fatalf("Failed to get signing root of block: %v", err)
	}
	blockSig := priv2.Sign(signingRoot[:], dt)
	block.Signature = blockSig.Marshal()[:]

	_, err = blocks.ProcessBlockHeader(state, block, true)
	want := "verify signature failed"
	if !strings.Contains(err.Error(), want) {
		t.Errorf("Expected %v, received %v", want, err)
	}

}

func TestProcessBlockHeader_DifferentSlots(t *testing.T) {
	if params.BeaconConfig().SlotsPerEpoch != 64 {
		t.Errorf("SlotsPerEpoch should be 64 for these tests to pass")
	}

	validators := make([]*ethpb.Validator, params.BeaconConfig().MinGenesisActiveValidatorCount)
	for i := 0; i < len(validators); i++ {
		validators[i] = &ethpb.Validator{
			ExitEpoch: params.BeaconConfig().FarFutureEpoch,
			Slashed:   true,
		}
	}

	state := &pb.BeaconState{
		Validators:        validators,
		Slot:              0,
		LatestBlockHeader: &ethpb.BeaconBlockHeader{Slot: 9},
		Fork: &pb.Fork{
			PreviousVersion: []byte{0, 0, 0, 0},
			CurrentVersion:  []byte{0, 0, 0, 0},
		},
		RandaoMixes:      make([][]byte, params.BeaconConfig().EpochsPerHistoricalVector),
		ActiveIndexRoots: make([][]byte, params.BeaconConfig().EpochsPerHistoricalVector),
	}

	lbhsr, err := ssz.HashTreeRoot(state.LatestBlockHeader)
	if err != nil {
		t.Error(err)
	}
	currentEpoch := helpers.CurrentEpoch(state)
	dt := helpers.Domain(state, currentEpoch, params.BeaconConfig().DomainBeaconProposer)
	priv, err := bls.RandKey(rand.Reader)
	if err != nil {
		t.Errorf("failed to generate private key got: %v", err)
	}
	blockSig := priv.Sign([]byte("hello"), dt)
	validators[5896].PublicKey = priv.PublicKey().Marshal()
	block := &ethpb.BeaconBlock{
		Slot: 1,
		Body: &ethpb.BeaconBlockBody{
			RandaoReveal: []byte{'A', 'B', 'C'},
		},
		ParentRoot: lbhsr[:],
		Signature:  blockSig.Marshal(),
	}

	_, err = blocks.ProcessBlockHeader(state, block, false)
	want := "is different then block slot"
	if !strings.Contains(err.Error(), want) {
		t.Errorf("Expected %v, received %v", want, err)
	}
}

func TestProcessBlockHeader_PreviousBlockRootNotSignedRoot(t *testing.T) {
	if params.BeaconConfig().SlotsPerEpoch != 64 {
		t.Errorf("SlotsPerEpoch should be 64 for these tests to pass")
	}

	validators := make([]*ethpb.Validator, params.BeaconConfig().MinGenesisActiveValidatorCount)
	for i := 0; i < len(validators); i++ {
		validators[i] = &ethpb.Validator{
			ExitEpoch: params.BeaconConfig().FarFutureEpoch,
			Slashed:   true,
		}
	}

	state := &pb.BeaconState{
		Validators:        validators,
		Slot:              0,
		LatestBlockHeader: &ethpb.BeaconBlockHeader{Slot: 9},
		Fork: &pb.Fork{
			PreviousVersion: []byte{0, 0, 0, 0},
			CurrentVersion:  []byte{0, 0, 0, 0},
		},
		RandaoMixes:      make([][]byte, params.BeaconConfig().EpochsPerHistoricalVector),
		ActiveIndexRoots: make([][]byte, params.BeaconConfig().EpochsPerHistoricalVector),
	}

	currentEpoch := helpers.CurrentEpoch(state)
	dt := helpers.Domain(state, currentEpoch, params.BeaconConfig().DomainBeaconProposer)
	priv, err := bls.RandKey(rand.Reader)
	if err != nil {
		t.Errorf("failed to generate private key got: %v", err)
	}
	blockSig := priv.Sign([]byte("hello"), dt)
	validators[5896].PublicKey = priv.PublicKey().Marshal()
	block := &ethpb.BeaconBlock{
		Slot: 0,
		Body: &ethpb.BeaconBlockBody{
			RandaoReveal: []byte{'A', 'B', 'C'},
		},
		ParentRoot: []byte{'A'},
		Signature:  blockSig.Marshal(),
	}

	_, err = blocks.ProcessBlockHeader(state, block, false)
	want := "does not match"
	if !strings.Contains(err.Error(), want) {
		t.Errorf("Expected %v, received %v", want, err)
	}
}

func TestProcessBlockHeader_SlashedProposer(t *testing.T) {
	if params.BeaconConfig().SlotsPerEpoch != 64 {
		t.Errorf("SlotsPerEpoch should be 64 for these tests to pass")
	}

	validators := make([]*ethpb.Validator, params.BeaconConfig().MinGenesisActiveValidatorCount)
	for i := 0; i < len(validators); i++ {
		validators[i] = &ethpb.Validator{
			ExitEpoch: params.BeaconConfig().FarFutureEpoch,
			Slashed:   true,
		}
	}

	state := &pb.BeaconState{
		Validators:        validators,
		Slot:              0,
		LatestBlockHeader: &ethpb.BeaconBlockHeader{Slot: 9},
		Fork: &pb.Fork{
			PreviousVersion: []byte{0, 0, 0, 0},
			CurrentVersion:  []byte{0, 0, 0, 0},
		},
		RandaoMixes:      make([][]byte, params.BeaconConfig().EpochsPerHistoricalVector),
		ActiveIndexRoots: make([][]byte, params.BeaconConfig().EpochsPerHistoricalVector),
	}

	parentRoot, err := ssz.SigningRoot(state.LatestBlockHeader)
	if err != nil {
		t.Error(err)
	}
	currentEpoch := helpers.CurrentEpoch(state)
	dt := helpers.Domain(state, currentEpoch, params.BeaconConfig().DomainBeaconProposer)
	priv, err := bls.RandKey(rand.Reader)
	if err != nil {
		t.Errorf("failed to generate private key got: %v", err)
	}
	blockSig := priv.Sign([]byte("hello"), dt)
	validators[12683].PublicKey = priv.PublicKey().Marshal()
	block := &ethpb.BeaconBlock{
		Slot: 0,
		Body: &ethpb.BeaconBlockBody{
			RandaoReveal: []byte{'A', 'B', 'C'},
		},
		ParentRoot: parentRoot[:],
		Signature:  blockSig.Marshal(),
	}

	_, err = blocks.ProcessBlockHeader(state, block, false)
	want := "was previously slashed"
	if !strings.Contains(err.Error(), want) {
		t.Errorf("Expected %v, received %v", want, err)
	}
}

func TestProcessBlockHeader_OK(t *testing.T) {
	helpers.ClearAllCaches()

	if params.BeaconConfig().SlotsPerEpoch != 64 {
		t.Fatalf("SlotsPerEpoch should be 64 for these tests to pass")
	}

	validators := make([]*ethpb.Validator, params.BeaconConfig().MinGenesisActiveValidatorCount)
	for i := 0; i < len(validators); i++ {
		validators[i] = &ethpb.Validator{
			ExitEpoch: params.BeaconConfig().FarFutureEpoch,
			Slashed:   true,
		}
	}

	state := &pb.BeaconState{
		Validators:        validators,
		Slot:              0,
		LatestBlockHeader: &ethpb.BeaconBlockHeader{Slot: 9},
		Fork: &pb.Fork{
			PreviousVersion: []byte{0, 0, 0, 0},
			CurrentVersion:  []byte{0, 0, 0, 0},
		},
		RandaoMixes:      make([][]byte, params.BeaconConfig().EpochsPerHistoricalVector),
		ActiveIndexRoots: make([][]byte, params.BeaconConfig().EpochsPerHistoricalVector),
	}

	latestBlockSignedRoot, err := ssz.SigningRoot(state.LatestBlockHeader)
	if err != nil {
		t.Error(err)
	}
	currentEpoch := helpers.CurrentEpoch(state)
	dt := helpers.Domain(state, currentEpoch, params.BeaconConfig().DomainBeaconProposer)
	priv, err := bls.RandKey(rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate private key got: %v", err)
	}
	block := &ethpb.BeaconBlock{
		Slot: 0,
		Body: &ethpb.BeaconBlockBody{
			RandaoReveal: []byte{'A', 'B', 'C'},
		},
		ParentRoot: latestBlockSignedRoot[:],
	}
	signingRoot, err := ssz.SigningRoot(block)
	if err != nil {
		t.Fatalf("Failed to get signing root of block: %v", err)
	}
	blockSig := priv.Sign(signingRoot[:], dt)
	block.Signature = blockSig.Marshal()[:]
	bodyRoot, err := ssz.HashTreeRoot(block.Body)
	if err != nil {
		t.Fatalf("Failed to hash block bytes got: %v", err)
	}

	proposerIdx, err := helpers.BeaconProposerIndex(state)
	if err != nil {
		t.Fatal(err)
	}
	validators[proposerIdx].Slashed = false
	validators[proposerIdx].PublicKey = priv.PublicKey().Marshal()

	newState, err := blocks.ProcessBlockHeader(state, block, true)
	if err != nil {
		t.Fatalf("Failed to process block header got: %v", err)
	}
	var zeroHash [32]byte
	var zeroSig [96]byte
	nsh := newState.LatestBlockHeader
	expected := &ethpb.BeaconBlockHeader{
		Slot:       block.Slot,
		ParentRoot: latestBlockSignedRoot[:],
		BodyRoot:   bodyRoot[:],
		StateRoot:  zeroHash[:],
		Signature:  zeroSig[:],
	}
	if !proto.Equal(nsh, expected) {
		t.Errorf("Expected %v, received %vk9k", expected, nsh)
	}
}

func TestProcessRandao_IncorrectProposerFailsVerification(t *testing.T) {
	helpers.ClearAllCaches()

	deposits, privKeys := testutil.SetupInitialDeposits(t, 100)
	beaconState, err := state.GenesisBeaconState(deposits, uint64(0), &ethpb.Eth1Data{})
	if err != nil {
		t.Fatal(err)
	}
	// We fetch the proposer's index as that is whom the RANDAO will be verified against.
	proposerIdx, err := helpers.BeaconProposerIndex(beaconState)
	if err != nil {
		t.Fatal(err)
	}
	epoch := uint64(0)
	buf := make([]byte, 32)
	binary.LittleEndian.PutUint64(buf, epoch)
	domain := helpers.Domain(beaconState, epoch, params.BeaconConfig().DomainRandao)

	// We make the previous validator's index sign the message instead of the proposer.
	epochSignature := privKeys[proposerIdx-1].Sign(buf, domain)
	block := &ethpb.BeaconBlock{
		Body: &ethpb.BeaconBlockBody{
			RandaoReveal: epochSignature.Marshal(),
		},
	}

	want := "block randao: signature did not verify"
	if _, err := blocks.ProcessRandao(
		beaconState,
		block.Body,
		true, /* verify signatures */
	); !strings.Contains(err.Error(), want) {
		t.Errorf("Expected %v, received %v", want, err)
	}
}

func TestProcessRandao_SignatureVerifiesAndUpdatesLatestStateMixes(t *testing.T) {
	deposits, privKeys := testutil.SetupInitialDeposits(t, 100)
	beaconState, err := state.GenesisBeaconState(deposits, uint64(0), &ethpb.Eth1Data{})
	if err != nil {
		t.Fatal(err)
	}

	epoch := helpers.CurrentEpoch(beaconState)
	epochSignature, err := helpers.CreateRandaoReveal(beaconState, epoch, privKeys)
	if err != nil {
		t.Fatal(err)
	}

	block := &ethpb.BeaconBlock{
		Body: &ethpb.BeaconBlockBody{
			RandaoReveal: epochSignature,
		},
	}

	newState, err := blocks.ProcessRandao(
		beaconState,
		block.Body,
		true, /* verify signatures */
	)
	if err != nil {
		t.Errorf("Unexpected error processing block randao: %v", err)
	}
	currentEpoch := helpers.CurrentEpoch(beaconState)
	mix := newState.RandaoMixes[currentEpoch%params.BeaconConfig().EpochsPerHistoricalVector]

	if bytes.Equal(mix, params.BeaconConfig().ZeroHash[:]) {
		t.Errorf(
			"Expected empty signature to be overwritten by randao reveal, received %v",
			params.BeaconConfig().EmptySignature,
		)
	}
}

func TestProcessEth1Data_SetsCorrectly(t *testing.T) {
	beaconState := &pb.BeaconState{
		Eth1DataVotes: []*ethpb.Eth1Data{},
	}

	block := &ethpb.BeaconBlock{
		Body: &ethpb.BeaconBlockBody{
			Eth1Data: &ethpb.Eth1Data{
				DepositRoot: []byte{2},
				BlockHash:   []byte{3},
			},
		},
	}
	var err error
	for i := uint64(0); i < params.BeaconConfig().SlotsPerEth1VotingPeriod; i++ {
		beaconState, err = blocks.ProcessEth1DataInBlock(beaconState, block)
		if err != nil {
			t.Fatal(err)
		}
	}

	newETH1DataVotes := beaconState.Eth1DataVotes
	if len(newETH1DataVotes) <= 1 {
		t.Error("Expected new ETH1 data votes to have length > 1")
	}
	if !proto.Equal(beaconState.Eth1Data, block.Body.Eth1Data) {
		t.Errorf(
			"Expected latest eth1 data to have been set to %v, received %v",
			block.Body.Eth1Data,
			beaconState.Eth1Data,
		)
	}
}
func TestProcessProposerSlashings_UnmatchedHeaderEpochs(t *testing.T) {
	registry := make([]*ethpb.Validator, 2)
	currentSlot := uint64(0)
	slashings := []*ethpb.ProposerSlashing{
		{
			ProposerIndex: 1,
			Header_1: &ethpb.BeaconBlockHeader{
				Slot: params.BeaconConfig().SlotsPerEpoch + 1,
			},
			Header_2: &ethpb.BeaconBlockHeader{
				Slot: 0,
			},
		},
	}

	beaconState := &pb.BeaconState{
		Validators: registry,
		Slot:       currentSlot,
	}
	block := &ethpb.BeaconBlock{
		Body: &ethpb.BeaconBlockBody{
			ProposerSlashings: slashings,
		},
	}
	want := "mismatched header epochs"
	if _, err := blocks.ProcessProposerSlashings(
		beaconState,
		block.Body,
	); !strings.Contains(err.Error(), want) {
		t.Errorf("Expected %s, received %v", want, err)
	}
}

func TestProcessProposerSlashings_SameHeaders(t *testing.T) {
	registry := make([]*ethpb.Validator, 2)
	currentSlot := uint64(0)
	slashings := []*ethpb.ProposerSlashing{
		{
			ProposerIndex: 1,
			Header_1: &ethpb.BeaconBlockHeader{
				Slot: 0,
			},
			Header_2: &ethpb.BeaconBlockHeader{
				Slot: 0,
			},
		},
	}

	beaconState := &pb.BeaconState{
		Validators: registry,
		Slot:       currentSlot,
	}
	block := &ethpb.BeaconBlock{
		Body: &ethpb.BeaconBlockBody{
			ProposerSlashings: slashings,
		},
	}
	want := "expected slashing headers to differ"
	if _, err := blocks.ProcessProposerSlashings(
		beaconState,
		block.Body,
	); !strings.Contains(err.Error(), want) {
		t.Errorf("Expected %s, received %v", want, err)
	}
}

func TestProcessProposerSlashings_ValidatorNotSlashable(t *testing.T) {
	registry := []*ethpb.Validator{
		{
			PublicKey:         []byte("key"),
			Slashed:           true,
			ActivationEpoch:   0,
			WithdrawableEpoch: 0,
		},
	}
	currentSlot := uint64(0)
	slashings := []*ethpb.ProposerSlashing{
		{
			ProposerIndex: 0,
			Header_1: &ethpb.BeaconBlockHeader{
				Slot:      0,
				Signature: []byte("A"),
			},
			Header_2: &ethpb.BeaconBlockHeader{
				Slot:      0,
				Signature: []byte("B"),
			},
		},
	}

	beaconState := &pb.BeaconState{
		Validators: registry,
		Slot:       currentSlot,
	}
	block := &ethpb.BeaconBlock{
		Body: &ethpb.BeaconBlockBody{
			ProposerSlashings: slashings,
		},
	}
	want := fmt.Sprintf(
		"validator with key %#x is not slashable",
		beaconState.Validators[0].PublicKey,
	)

	if _, err := blocks.ProcessProposerSlashings(
		beaconState,
		block.Body,
	); !strings.Contains(err.Error(), want) {
		t.Errorf("Expected %s, received %v", want, err)
	}
}

func TestProcessProposerSlashings_AppliesCorrectStatus(t *testing.T) {
	// We test the case when data is correct and verify the validator
	// registry has been updated.
	helpers.ClearShuffledValidatorCache()
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
	beaconState := &pb.BeaconState{
		Validators: validators,
		Slot:       currentSlot,
		Balances:   validatorBalances,
		Fork: &pb.Fork{
			CurrentVersion:  params.BeaconConfig().GenesisForkVersion,
			PreviousVersion: params.BeaconConfig().GenesisForkVersion,
			Epoch:           0,
		},
		Slashings:        make([]uint64, params.BeaconConfig().EpochsPerSlashingsVector),
		RandaoMixes:      make([][]byte, params.BeaconConfig().EpochsPerHistoricalVector),
		ActiveIndexRoots: make([][]byte, params.BeaconConfig().EpochsPerHistoricalVector),
	}

	domain := helpers.Domain(
		beaconState,
		helpers.CurrentEpoch(beaconState),
		params.BeaconConfig().DomainBeaconProposer,
	)
	privKey, err := bls.RandKey(rand.Reader)
	if err != nil {
		t.Errorf("Could not generate random private key: %v", err)
	}

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

	slashings := []*ethpb.ProposerSlashing{
		{
			ProposerIndex: 1,
			Header_1:      header1,
			Header_2:      header2,
		},
	}

	beaconState.Validators[1].PublicKey = privKey.PublicKey().Marshal()[:]

	block := &ethpb.BeaconBlock{
		Body: &ethpb.BeaconBlockBody{
			ProposerSlashings: slashings,
		},
	}

	newState, err := blocks.ProcessProposerSlashings(beaconState, block.Body)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	newStateVals := newState.Validators
	if newStateVals[1].ExitEpoch != validators[1].ExitEpoch {
		t.Errorf("Proposer with index 1 did not correctly exit,"+"wanted slot:%d, got:%d",
			newStateVals[1].ExitEpoch, validators[1].ExitEpoch)
	}
}
func TestSlashableAttestationData_CanSlash(t *testing.T) {
	att1 := &ethpb.AttestationData{
		Target: &ethpb.Checkpoint{Epoch: 1},
		Source: &ethpb.Checkpoint{Root: []byte{'A'}},
	}
	att2 := &ethpb.AttestationData{
		Target: &ethpb.Checkpoint{Epoch: 1},
		Source: &ethpb.Checkpoint{Root: []byte{'B'}},
	}
	if !blocks.IsSlashableAttestationData(att1, att2) {
		t.Error("atts should have been slashable")
	}
	att1.Target.Epoch = 4
	att1.Source.Epoch = 2
	att2.Source.Epoch = 3
	if !blocks.IsSlashableAttestationData(att1, att2) {
		t.Error("atts should have been slashable")
	}
}

func TestProcessAttesterSlashings_DataNotSlashable(t *testing.T) {
	slashings := []*ethpb.AttesterSlashing{
		{
			Attestation_1: &ethpb.IndexedAttestation{
				Data: &ethpb.AttestationData{
					Source: &ethpb.Checkpoint{Epoch: 0},
					Target: &ethpb.Checkpoint{Epoch: 0},
					Crosslink: &ethpb.Crosslink{
						Shard: 4,
					},
				},
			},
			Attestation_2: &ethpb.IndexedAttestation{
				Data: &ethpb.AttestationData{
					Source: &ethpb.Checkpoint{Epoch: 1},
					Target: &ethpb.Checkpoint{Epoch: 1},
					Crosslink: &ethpb.Crosslink{
						Shard: 3,
					},
				},
			},
		},
	}
	registry := []*ethpb.Validator{}
	currentSlot := uint64(0)

	beaconState := &pb.BeaconState{
		Validators: registry,
		Slot:       currentSlot,
	}
	block := &ethpb.BeaconBlock{
		Body: &ethpb.BeaconBlockBody{
			AttesterSlashings: slashings,
		},
	}
	want := fmt.Sprint("attestations are not slashable")

	if _, err := blocks.ProcessAttesterSlashings(
		beaconState,
		block.Body,
		false,
	); !strings.Contains(err.Error(), want) {
		t.Errorf("Expected %s, received %v", want, err)
	}
}

func TestProcessAttesterSlashings_IndexedAttestationFailedToVerify(t *testing.T) {
	slashings := []*ethpb.AttesterSlashing{
		{
			Attestation_1: &ethpb.IndexedAttestation{
				Data: &ethpb.AttestationData{
					Source: &ethpb.Checkpoint{Epoch: 1},
					Target: &ethpb.Checkpoint{Epoch: 0},
					Crosslink: &ethpb.Crosslink{
						Shard: 4,
					},
				},
				CustodyBit_0Indices: []uint64{0, 1, 2},
				CustodyBit_1Indices: []uint64{0, 1, 2},
			},
			Attestation_2: &ethpb.IndexedAttestation{
				Data: &ethpb.AttestationData{
					Source: &ethpb.Checkpoint{Epoch: 0},
					Target: &ethpb.Checkpoint{Epoch: 0},
					Crosslink: &ethpb.Crosslink{
						Shard: 4,
					},
				},
				CustodyBit_0Indices: []uint64{0, 1, 2},
				CustodyBit_1Indices: []uint64{0, 1, 2},
			},
		},
	}
	registry := []*ethpb.Validator{}
	currentSlot := uint64(0)

	beaconState := &pb.BeaconState{
		Validators: registry,
		Slot:       currentSlot,
	}
	block := &ethpb.BeaconBlock{
		Body: &ethpb.BeaconBlockBody{
			AttesterSlashings: slashings,
		},
	}
	want := fmt.Sprint("expected no bit 1 indices")

	if _, err := blocks.ProcessAttesterSlashings(
		beaconState,
		block.Body,
		false,
	); !strings.Contains(err.Error(), want) {
		t.Errorf("Expected %s, received %v", want, err)
	}

	slashings = []*ethpb.AttesterSlashing{
		{
			Attestation_1: &ethpb.IndexedAttestation{
				Data: &ethpb.AttestationData{
					Source: &ethpb.Checkpoint{Epoch: 1},
					Target: &ethpb.Checkpoint{Epoch: 0},
					Crosslink: &ethpb.Crosslink{
						Shard: 4,
					},
				},
				CustodyBit_0Indices: make([]uint64, params.BeaconConfig().MaxValidatorsPerCommittee+1),
			},
			Attestation_2: &ethpb.IndexedAttestation{
				Data: &ethpb.AttestationData{
					Source: &ethpb.Checkpoint{Epoch: 0},
					Target: &ethpb.Checkpoint{Epoch: 0},
					Crosslink: &ethpb.Crosslink{
						Shard: 4,
					},
				},
				CustodyBit_0Indices: make([]uint64, params.BeaconConfig().MaxValidatorsPerCommittee+1),
			},
		},
	}

	block.Body.AttesterSlashings = slashings
	want = fmt.Sprint("over max number of allowed indices")

	if _, err := blocks.ProcessAttesterSlashings(
		beaconState,
		block.Body,
		false,
	); !strings.Contains(err.Error(), want) {
		t.Errorf("Expected %s, received %v", want, err)
	}
}

func TestProcessAttesterSlashings_AppliesCorrectStatus(t *testing.T) {
	// We test the case when data is correct and verify the validator
	// registry has been updated.
	validators := make([]*ethpb.Validator, params.BeaconConfig().MinGenesisActiveValidatorCount)
	for i := 0; i < len(validators); i++ {
		validators[i] = &ethpb.Validator{
			ActivationEpoch:   0,
			ExitEpoch:         params.BeaconConfig().FarFutureEpoch,
			Slashed:           false,
			WithdrawableEpoch: 1 * params.BeaconConfig().SlotsPerEpoch,
		}
	}
	validatorBalances := make([]uint64, len(validators))
	for i := 0; i < len(validatorBalances); i++ {
		validatorBalances[i] = params.BeaconConfig().MaxEffectiveBalance
	}

	slashings := []*ethpb.AttesterSlashing{
		{
			Attestation_1: &ethpb.IndexedAttestation{
				Data: &ethpb.AttestationData{
					Source: &ethpb.Checkpoint{Epoch: 1},
					Target: &ethpb.Checkpoint{Epoch: 0},
					Crosslink: &ethpb.Crosslink{
						Shard: 4,
					},
				},
				CustodyBit_0Indices: []uint64{0, 1},
			},
			Attestation_2: &ethpb.IndexedAttestation{
				Data: &ethpb.AttestationData{
					Source: &ethpb.Checkpoint{Epoch: 0},
					Target: &ethpb.Checkpoint{Epoch: 0},
					Crosslink: &ethpb.Crosslink{
						Shard: 4,
					},
				},
				CustodyBit_0Indices: []uint64{0, 1},
			},
		},
	}

	currentSlot := 2 * params.BeaconConfig().SlotsPerEpoch
	beaconState := &pb.BeaconState{
		Validators:       validators,
		Slot:             currentSlot,
		Balances:         validatorBalances,
		RandaoMixes:      make([][]byte, params.BeaconConfig().EpochsPerHistoricalVector),
		Slashings:        make([]uint64, params.BeaconConfig().EpochsPerSlashingsVector),
		ActiveIndexRoots: make([][]byte, params.BeaconConfig().EpochsPerHistoricalVector),
	}
	block := &ethpb.BeaconBlock{
		Body: &ethpb.BeaconBlockBody{
			AttesterSlashings: slashings,
		},
	}
	newState, err := blocks.ProcessAttesterSlashings(
		beaconState,
		block.Body,
		false,
	)
	if err != nil {
		t.Fatal(err)
	}
	newRegistry := newState.Validators

	// Given the intersection of slashable indices is [1], only validator
	// at index 1 should be slashed and exited. We confirm this below.
	if newRegistry[1].ExitEpoch != validators[1].ExitEpoch {
		t.Errorf(
			`
			Expected validator at index 1's exit epoch to match
			%d, received %d instead
			`,
			validators[1].ExitEpoch,
			newRegistry[1].ExitEpoch,
		)
	}
}

func TestProcessAttestations_InclusionDelayFailure(t *testing.T) {
	attestations := []*ethpb.Attestation{
		{
			Data: &ethpb.AttestationData{
				Target: &ethpb.Checkpoint{Epoch: 0},
				Crosslink: &ethpb.Crosslink{
					Shard: 0,
				},
			},
		},
	}
	block := &ethpb.BeaconBlock{
		Body: &ethpb.BeaconBlockBody{
			Attestations: attestations,
		},
	}
	deposits, _ := testutil.SetupInitialDeposits(t, 100)
	beaconState, err := state.GenesisBeaconState(deposits, uint64(0), &ethpb.Eth1Data{})
	if err != nil {
		t.Fatal(err)
	}

	attestationSlot, err := helpers.AttestationDataSlot(beaconState, attestations[0].Data)
	if err != nil {
		t.Fatal(err)
	}

	want := fmt.Sprintf(
		"attestation slot %d + inclusion delay %d > state slot %d",
		attestationSlot,
		params.BeaconConfig().MinAttestationInclusionDelay,
		beaconState.Slot,
	)
	if _, err := blocks.ProcessAttestations(
		beaconState,
		block.Body,
		false,
	); !strings.Contains(err.Error(), want) {
		t.Errorf("Expected %s, received %v", want, err)
	}
}

func TestProcessAttestations_NeitherCurrentNorPrevEpoch(t *testing.T) {
	helpers.ClearActiveIndicesCache()
	helpers.ClearActiveCountCache()
	helpers.ClearStartShardCache()

	attestations := []*ethpb.Attestation{
		{
			Data: &ethpb.AttestationData{
				Source: &ethpb.Checkpoint{Epoch: 0},
				Target: &ethpb.Checkpoint{Epoch: 0},
				Crosslink: &ethpb.Crosslink{
					Shard: 0,
				},
			},
		},
	}
	block := &ethpb.BeaconBlock{
		Body: &ethpb.BeaconBlockBody{
			Attestations: attestations,
		},
	}
	deposits, _ := testutil.SetupInitialDeposits(t, 100)
	beaconState, err := state.GenesisBeaconState(deposits, uint64(0), &ethpb.Eth1Data{})
	if err != nil {
		t.Fatal(err)
	}
	helpers.ClearAllCaches()
	beaconState.Slot += params.BeaconConfig().SlotsPerEpoch*4 + params.BeaconConfig().MinAttestationInclusionDelay

	want := fmt.Sprintf(
		"expected target epoch %d == %d or %d",
		attestations[0].Data.Target.Epoch,
		helpers.PrevEpoch(beaconState),
		helpers.CurrentEpoch(beaconState),
	)
	if _, err := blocks.ProcessAttestations(
		beaconState,
		block.Body,
		false,
	); !strings.Contains(err.Error(), want) {
		t.Errorf("Expected %s, received %v", want, err)
	}
}

func TestProcessAttestations_CurrentEpochFFGDataMismatches(t *testing.T) {
	helpers.ClearAllCaches()

	attestations := []*ethpb.Attestation{
		{
			Data: &ethpb.AttestationData{
				Target: &ethpb.Checkpoint{Epoch: 0},
				Source: &ethpb.Checkpoint{Epoch: 1},
				Crosslink: &ethpb.Crosslink{
					Shard: 0,
				},
			},
		},
	}
	block := &ethpb.BeaconBlock{
		Body: &ethpb.BeaconBlockBody{
			Attestations: attestations,
		},
	}
	deposits, _ := testutil.SetupInitialDeposits(t, 100)
	beaconState, err := state.GenesisBeaconState(deposits, uint64(0), &ethpb.Eth1Data{})
	if err != nil {
		t.Fatal(err)
	}
	beaconState.Slot += params.BeaconConfig().MinAttestationInclusionDelay
	beaconState.CurrentCrosslinks = []*ethpb.Crosslink{
		{
			Shard: 0,
		},
	}
	beaconState.CurrentJustifiedCheckpoint.Root = []byte("hello-world")
	beaconState.CurrentEpochAttestations = []*pb.PendingAttestation{}

	want := fmt.Sprintf(
		"expected source epoch %d, received %d",
		helpers.CurrentEpoch(beaconState),
		attestations[0].Data.Source.Epoch,
	)
	if _, err := blocks.ProcessAttestations(
		beaconState,
		block.Body,
		false,
	); !strings.Contains(err.Error(), want) {
		t.Errorf("Expected %s, received %v", want, err)
	}

	block.Body.Attestations[0].Data.Source.Epoch = helpers.CurrentEpoch(beaconState)
	block.Body.Attestations[0].Data.Source.Root = []byte{}

	want = fmt.Sprintf(
		"expected source root %#x, received %#x",
		beaconState.CurrentJustifiedCheckpoint.Root,
		attestations[0].Data.Source.Root,
	)
	if _, err := blocks.ProcessAttestations(
		beaconState,
		block.Body,
		false,
	); !strings.Contains(err.Error(), want) {
		t.Errorf("Expected %s, received %v", want, err)
	}
}

func TestProcessAttestations_PrevEpochFFGDataMismatches(t *testing.T) {
	helpers.ClearAllCaches()

	attestations := []*ethpb.Attestation{
		{
			Data: &ethpb.AttestationData{
				Source: &ethpb.Checkpoint{Epoch: 1},
				Target: &ethpb.Checkpoint{Epoch: 0},
				Crosslink: &ethpb.Crosslink{
					Shard: 0,
				},
			},
		},
	}
	block := &ethpb.BeaconBlock{
		Body: &ethpb.BeaconBlockBody{
			Attestations: attestations,
		},
	}
	deposits, _ := testutil.SetupInitialDeposits(t, 100)
	beaconState, err := state.GenesisBeaconState(deposits, uint64(0), &ethpb.Eth1Data{})
	if err != nil {
		t.Fatal(err)
	}
	helpers.ClearAllCaches()
	beaconState.Slot += params.BeaconConfig().SlotsPerEpoch + params.BeaconConfig().MinAttestationInclusionDelay
	beaconState.PreviousCrosslinks = []*ethpb.Crosslink{
		{
			Shard: 0,
		},
	}
	beaconState.PreviousJustifiedCheckpoint.Root = []byte("hello-world")
	beaconState.PreviousEpochAttestations = []*pb.PendingAttestation{}

	want := fmt.Sprintf(
		"expected source epoch %d, received %d",
		helpers.PrevEpoch(beaconState),
		attestations[0].Data.Source.Epoch,
	)
	if _, err := blocks.ProcessAttestations(
		beaconState,
		block.Body,
		false,
	); !strings.Contains(err.Error(), want) {
		t.Errorf("Expected %s, received %v", want, err)
	}

	block.Body.Attestations[0].Data.Source.Epoch = helpers.PrevEpoch(beaconState)
	block.Body.Attestations[0].Data.Source.Root = []byte{}

	want = fmt.Sprintf(
		"expected source root %#x, received %#x",
		beaconState.PreviousJustifiedCheckpoint.Root,
		attestations[0].Data.Source.Root,
	)
	if _, err := blocks.ProcessAttestations(
		beaconState,
		block.Body,
		false,
	); !strings.Contains(err.Error(), want) {
		t.Errorf("Expected %s, received %v", want, err)
	}
}

func TestProcessAttestations_CrosslinkMismatches(t *testing.T) {
	helpers.ClearAllCaches()

	attestations := []*ethpb.Attestation{
		{
			Data: &ethpb.AttestationData{
				Source: &ethpb.Checkpoint{Epoch: 0, Root: []byte("hello-world")},
				Target: &ethpb.Checkpoint{Epoch: 0},
				Crosslink: &ethpb.Crosslink{
					Shard: 0,
				},
			},
		},
	}
	block := &ethpb.BeaconBlock{
		Body: &ethpb.BeaconBlockBody{
			Attestations: attestations,
		},
	}
	deposits, _ := testutil.SetupInitialDeposits(t, 100)
	beaconState, err := state.GenesisBeaconState(deposits, uint64(0), &ethpb.Eth1Data{})
	if err != nil {
		t.Fatal(err)
	}
	beaconState.Slot += params.BeaconConfig().MinAttestationInclusionDelay
	beaconState.CurrentCrosslinks = []*ethpb.Crosslink{
		{
			Shard:      0,
			StartEpoch: 0,
		},
	}
	beaconState.CurrentJustifiedCheckpoint.Root = []byte("hello-world")
	beaconState.CurrentEpochAttestations = []*pb.PendingAttestation{}

	want := "mismatched parent crosslink root"
	if _, err := blocks.ProcessAttestations(
		beaconState,
		block.Body,
		false,
	); !strings.Contains(err.Error(), want) {
		t.Errorf("Expected %s, received %v", want, err)
	}

	block.Body.Attestations[0].Data.Crosslink.StartEpoch = 0
	if _, err := blocks.ProcessAttestations(
		beaconState,
		block.Body,
		false,
	); !strings.Contains(err.Error(), want) {
		t.Errorf("Expected %s, received %v", want, err)
	}
	encoded, err := ssz.HashTreeRoot(beaconState.CurrentCrosslinks[0])
	if err != nil {
		t.Fatal(err)
	}
	block.Body.Attestations[0].Data.Crosslink.ParentRoot = encoded[:]
	block.Body.Attestations[0].Data.Crosslink.DataRoot = encoded[:]

	want = fmt.Sprintf("expected data root %#x == ZERO_HASH", encoded)
	if _, err := blocks.ProcessAttestations(
		beaconState,
		block.Body,
		false,
	); !strings.Contains(err.Error(), want) {
		t.Errorf("Expected %s, received %v", want, err)
	}
}

func TestProcessAttestations_OK(t *testing.T) {
	helpers.ClearAllCaches()

	attestations := []*ethpb.Attestation{
		{
			Data: &ethpb.AttestationData{
				Source: &ethpb.Checkpoint{Epoch: 0, Root: []byte("hello-world")},
				Target: &ethpb.Checkpoint{Epoch: 0},
				Crosslink: &ethpb.Crosslink{
					Shard:      0,
					StartEpoch: 0,
				},
			},
			AggregationBits: bitfield.Bitlist{0xC0, 0xC0, 0xC0, 0xC0, 0x01},
			CustodyBits:     bitfield.Bitlist{0x00, 0x00, 0x00, 0x00, 0x01},
		},
	}
	block := &ethpb.BeaconBlock{
		Body: &ethpb.BeaconBlockBody{
			Attestations: attestations,
		},
	}
	deposits, _ := testutil.SetupInitialDeposits(t, 100)
	beaconState, err := state.GenesisBeaconState(deposits, uint64(0), &ethpb.Eth1Data{})
	if err != nil {
		t.Fatal(err)
	}
	beaconState.Slot += params.BeaconConfig().MinAttestationInclusionDelay
	beaconState.CurrentCrosslinks = []*ethpb.Crosslink{
		{
			Shard:      0,
			StartEpoch: 0,
		},
	}
	beaconState.CurrentJustifiedCheckpoint.Root = []byte("hello-world")
	beaconState.CurrentEpochAttestations = []*pb.PendingAttestation{}

	encoded, err := ssz.HashTreeRoot(beaconState.CurrentCrosslinks[0])
	if err != nil {
		t.Fatal(err)
	}
	block.Body.Attestations[0].Data.Crosslink.ParentRoot = encoded[:]
	block.Body.Attestations[0].Data.Crosslink.DataRoot = params.BeaconConfig().ZeroHash[:]

	if _, err := blocks.ProcessAttestations(
		beaconState,
		block.Body,
		false,
	); err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestConvertToIndexed_OK(t *testing.T) {
	helpers.ClearActiveIndicesCache()
	helpers.ClearActiveCountCache()
	helpers.ClearStartShardCache()
	helpers.ClearShuffledValidatorCache()

	if params.BeaconConfig().SlotsPerEpoch != 64 {
		t.Errorf("SlotsPerEpoch should be 64 for these tests to pass")
	}

	validators := make([]*ethpb.Validator, 2*params.BeaconConfig().SlotsPerEpoch)
	for i := 0; i < len(validators); i++ {
		validators[i] = &ethpb.Validator{
			ExitEpoch: params.BeaconConfig().FarFutureEpoch,
		}
	}

	state := &pb.BeaconState{
		Slot:             5,
		Validators:       validators,
		RandaoMixes:      make([][]byte, params.BeaconConfig().EpochsPerHistoricalVector),
		ActiveIndexRoots: make([][]byte, params.BeaconConfig().EpochsPerHistoricalVector),
	}
	tests := []struct {
		aggregationBitfield      bitfield.Bitlist
		custodyBitfield          bitfield.Bitlist
		wantedCustodyBit0Indices []uint64
		wantedCustodyBit1Indices []uint64
	}{
		{
			aggregationBitfield:      bitfield.Bitlist{0x07},
			custodyBitfield:          bitfield.Bitlist{0x05},
			wantedCustodyBit0Indices: []uint64{71},
			wantedCustodyBit1Indices: []uint64{127},
		},
		{
			aggregationBitfield:      bitfield.Bitlist{0x07},
			custodyBitfield:          bitfield.Bitlist{0x06},
			wantedCustodyBit0Indices: []uint64{127},
			wantedCustodyBit1Indices: []uint64{71},
		},
		{
			aggregationBitfield:      bitfield.Bitlist{0x07},
			custodyBitfield:          bitfield.Bitlist{0x07},
			wantedCustodyBit0Indices: []uint64{},
			wantedCustodyBit1Indices: []uint64{71, 127},
		},
	}

	attestation := &ethpb.Attestation{
		Signature: []byte("signed"),
		Data: &ethpb.AttestationData{
			Source: &ethpb.Checkpoint{Epoch: 0},
			Target: &ethpb.Checkpoint{Epoch: 0},
			Crosslink: &ethpb.Crosslink{
				Shard: 3,
			},
		},
	}
	for _, tt := range tests {
		helpers.ClearAllCaches()

		attestation.AggregationBits = tt.aggregationBitfield
		attestation.CustodyBits = tt.custodyBitfield
		wanted := &ethpb.IndexedAttestation{
			CustodyBit_0Indices: tt.wantedCustodyBit0Indices,
			CustodyBit_1Indices: tt.wantedCustodyBit1Indices,
			Data:                attestation.Data,
			Signature:           attestation.Signature,
		}
		ia, err := blocks.ConvertToIndexed(state, attestation)
		if err != nil {
			t.Errorf("failed to convert attestation to indexed attestation: %v", err)
		}
		if !reflect.DeepEqual(wanted, ia) {
			diff, _ := messagediff.PrettyDiff(ia, wanted)
			t.Log(diff)
			t.Error("convert attestation to indexed attestation didn't result as wanted")
		}
	}
}

func TestValidateIndexedAttestation_AboveMaxLength(t *testing.T) {
	indexedAtt1 := &ethpb.IndexedAttestation{
		CustodyBit_0Indices: make([]uint64, params.BeaconConfig().MaxValidatorsPerCommittee+5),
		CustodyBit_1Indices: []uint64{},
	}

	for i := uint64(0); i < params.BeaconConfig().MaxValidatorsPerCommittee+5; i++ {
		indexedAtt1.CustodyBit_0Indices[i] = i
	}

	want := "over max number of allowed indices"
	if err := blocks.VerifyIndexedAttestation(
		&pb.BeaconState{},
		indexedAtt1,
		false,
	); !strings.Contains(err.Error(), want) {
		t.Errorf("Expected verification to fail return false, received: %v", err)
	}
}

func TestProcessDeposits_MerkleBranchFailsVerification(t *testing.T) {
	deposit := &ethpb.Deposit{
		Data: &ethpb.Deposit_Data{
			PublicKey: []byte{1, 2, 3},
			Signature: make([]byte, 96),
		},
	}
	leaf, err := ssz.HashTreeRoot(deposit.Data)
	if err != nil {
		t.Fatal(err)
	}

	// We then create a merkle branch for the test.
	depositTrie, err := trieutil.GenerateTrieFromItems([][]byte{leaf[:]}, int(params.BeaconConfig().DepositContractTreeDepth))
	if err != nil {
		t.Fatalf("Could not generate trie: %v", err)
	}
	proof, err := depositTrie.MerkleProof(0)
	if err != nil {
		t.Fatalf("Could not generate proof: %v", err)
	}

	deposit.Proof = proof
	block := &ethpb.BeaconBlock{
		Body: &ethpb.BeaconBlockBody{
			Deposits: []*ethpb.Deposit{deposit},
		},
	}
	beaconState := &pb.BeaconState{
		Eth1Data: &ethpb.Eth1Data{
			DepositRoot: []byte{0},
			BlockHash:   []byte{1},
		},
	}
	want := "deposit root did not verify"
	if _, err := blocks.ProcessDeposits(
		beaconState,
		block.Body,
	); !strings.Contains(err.Error(), want) {
		t.Errorf("Expected error: %s, received %v", want, err)
	}
}

func TestProcessDeposits_AddsNewValidatorDeposit(t *testing.T) {
	dep, _ := testutil.SetupInitialDeposits(t, 1)
	eth1Data := testutil.GenerateEth1Data(t, dep)

	block := &ethpb.BeaconBlock{
		Body: &ethpb.BeaconBlockBody{
			Deposits: []*ethpb.Deposit{dep[0]},
		},
	}
	registry := []*ethpb.Validator{
		{
			PublicKey:             []byte{1},
			WithdrawalCredentials: []byte{1, 2, 3},
		},
	}
	balances := []uint64{0}
	beaconState := &pb.BeaconState{
		Validators: registry,
		Balances:   balances,
		Eth1Data:   eth1Data,
		Fork: &pb.Fork{
			PreviousVersion: params.BeaconConfig().GenesisForkVersion,
			CurrentVersion:  params.BeaconConfig().GenesisForkVersion,
		},
	}
	newState, err := blocks.ProcessDeposits(
		beaconState,
		block.Body,
	)
	if err != nil {
		t.Fatalf("Expected block deposits to process correctly, received: %v", err)
	}
	if newState.Balances[1] != dep[0].Data.Amount {
		t.Errorf(
			"Expected state validator balances index 0 to equal %d, received %d",
			dep[0].Data.Amount,
			newState.Balances[1],
		)
	}
}

func TestProcessDeposit_RepeatedDeposit_IncreasesValidatorBalance(t *testing.T) {
	sk, err := bls.RandKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	deposit := &ethpb.Deposit{
		Data: &ethpb.Deposit_Data{
			PublicKey: sk.PublicKey().Marshal(),
			Amount:    1000,
		},
	}
	sr, err := ssz.SigningRoot(deposit.Data)
	if err != nil {
		t.Fatal(err)
	}
	sig := sk.Sign(sr[:], 3)
	deposit.Data.Signature = sig.Marshal()
	leaf, err := hashutil.DepositHash(deposit.Data)
	if err != nil {
		t.Fatal(err)
	}

	// We then create a merkle branch for the test.
	depositTrie, err := trieutil.GenerateTrieFromItems([][]byte{leaf[:]}, int(params.BeaconConfig().DepositContractTreeDepth))
	if err != nil {
		t.Fatalf("Could not generate trie: %v", err)
	}
	proof, err := depositTrie.MerkleProof(0)
	if err != nil {
		t.Fatalf("Could not generate proof: %v", err)
	}

	deposit.Proof = proof
	block := &ethpb.BeaconBlock{
		Body: &ethpb.BeaconBlockBody{
			Deposits: []*ethpb.Deposit{deposit},
		},
	}
	registry := []*ethpb.Validator{
		{
			PublicKey: []byte{1, 2, 3},
		},
		{
			PublicKey:             sk.PublicKey().Marshal(),
			WithdrawalCredentials: []byte{1},
		},
	}
	balances := []uint64{0, 50}
	root := depositTrie.Root()
	beaconState := &pb.BeaconState{
		Validators: registry,
		Balances:   balances,
		Eth1Data: &ethpb.Eth1Data{
			DepositRoot: root[:],
			BlockHash:   root[:],
		},
	}
	newState, err := blocks.ProcessDeposits(
		beaconState,
		block.Body,
	)
	if err != nil {
		t.Fatalf("Process deposit failed: %v", err)
	}
	if newState.Balances[1] != 1000+50 {
		t.Errorf("Expected balance at index 1 to be 1050, received %d", newState.Balances[1])
	}
}

func TestProcessDeposit_SkipsInvalidDeposit(t *testing.T) {
	// Same test settings as in TestProcessDeposit_PublicKeyDoesNotExist, except that we add an explicit invalid
	// signature and test the signature validity this time.
	registry := []*ethpb.Validator{
		{
			PublicKey:             []byte{1, 2, 3},
			WithdrawalCredentials: []byte{2},
		},
		{
			PublicKey:             []byte{4, 5, 6},
			WithdrawalCredentials: []byte{1},
		},
	}
	balances := []uint64{1000, 1000}
	beaconState := &pb.BeaconState{
		Balances: balances,
		Fork: &pb.Fork{
			Epoch:          0,
			CurrentVersion: []byte{0, 0, 0, 0},
		},
		Validators: registry,
	}

	deposit := &ethpb.Deposit{
		Proof: [][]byte{},
		Data: &ethpb.Deposit_Data{
			PublicKey:             []byte{7, 8, 9},
			WithdrawalCredentials: []byte{1},
			Amount:                uint64(2000),
			Signature:             make([]byte, 96),
		},
	}

	newState, err := blocks.ProcessDeposit(
		beaconState,
		deposit,
		stateutils.ValidatorIndexMap(beaconState),
		true,
		false,
	)

	if err != nil {
		t.Fatalf("Expected invalid block deposit to be ignored without error, received: %v", err)
	}
	if newState.Eth1DepositIndex != 1 {
		t.Errorf(
			"Expected Eth1DepositIndex to be increased by 1 after processing an invalid deposit, received change: %v",
			newState.Eth1DepositIndex,
		)
	}
	if len(newState.Validators) != 2 {
		t.Errorf("Expected validator list to have length 2, received: %v", len(newState.Validators))
	}
	if len(newState.Balances) != 2 {
		t.Errorf("Expected validator balances list to have length 2, received: %v", len(newState.Balances))
	}
}

func TestProcessDeposit_PublicKeyDoesNotExist_ValidSignature(t *testing.T) {
	// Same test settings as in TestProcessDeposit_PublicKeyDoesNotExist but with a valid signature and signature test
	// on.
	//
	// The key pair was obtained by using the private key (in bytes) 1, 2, 3, ..., 32
	// An example for computing the pair and the signature using the bls library:
	//   key, _ := bls.SecretKeyFromBytes([]byte{1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16,17,18,19,20,21,22,23,24,25,26,
	//                                           27,28,29,30,31,32})
	//   pubBytes := key.PublicKey().Marshal()
	//	 deposit.Data.PublicKey = pubBytes
	//   root, _ := ssz.SigningRoot(deposit.Data)
	//   sig := key.Sign(root[:], domain)
	//   sigBytes := sig.Marshal()
	//   fmt.Printf("%#v\n", pubBytes)
	//   fmt.Printf("%#v", sigBytes)
	registry := []*ethpb.Validator{
		{
			PublicKey:             []byte{1, 2, 3},
			WithdrawalCredentials: []byte{2},
		},
		{
			PublicKey:             []byte{4, 5, 6},
			WithdrawalCredentials: []byte{1},
		},
	}
	balances := []uint64{1000, 1000}
	beaconState := &pb.BeaconState{
		Balances: balances,
		Fork: &pb.Fork{
			Epoch:          0,
			CurrentVersion: []byte{0, 0, 0, 0},
		},
		Validators: registry,
	}

	deposit := &ethpb.Deposit{
		Proof: [][]byte{},
		Data: &ethpb.Deposit_Data{
			PublicKey: []byte{0x96, 0xa2, 0xb, 0xb9, 0x48, 0x5f, 0xf6, 0xd8, 0x95, 0x9, 0x55, 0xa6, 0x29,
				0xe8, 0x4, 0x3a, 0x43, 0x77, 0x59, 0x68, 0xac, 0x13, 0x3e, 0xb7, 0xb1, 0x9c, 0x5f, 0x3, 0x89, 0xa2,
				0x25, 0x36, 0x76, 0xab, 0xdd, 0x6c, 0x86, 0xc7, 0xb6, 0x8d, 0x38, 0xa1, 0xb7, 0xf6, 0xaf, 0x86, 0x50,
				0xe7},
			WithdrawalCredentials: []byte{1},
			Amount:                uint64(2000),
			Signature: []byte{0xb6, 0xa0, 0x39, 0x7d, 0x37, 0xa9, 0xed, 0x30, 0xd5, 0xc3, 0x9b, 0x3b, 0xb3, 0x7, 0x62,
				0xa0, 0xf4, 0xbc, 0xfc, 0x4b, 0x98, 0x79, 0x6e, 0x10, 0x8, 0x85, 0xd0, 0x6a, 0xae, 0xe6, 0x87, 0x60,
				0x71, 0x32, 0xde, 0x2d, 0xcc, 0xa2, 0x2c, 0x52, 0x2a, 0xc, 0xb2, 0xc0, 0xa4, 0xf8, 0x89, 0xe4, 0x11,
				0x23, 0x18, 0x37, 0xfd, 0xd, 0x35, 0x99, 0x2a, 0x8f, 0xb, 0xe8, 0xc8, 0x36, 0x8a, 0x6c, 0x6f, 0x1a,
				0x27, 0xe, 0xd1, 0xea, 0x6d, 0x5b, 0x45, 0x8, 0xce, 0xf9, 0x67, 0x9, 0xc9, 0x91, 0xdf, 0xff, 0x31, 0x87,
				0x37, 0x20, 0x85, 0x82, 0x76, 0x92, 0xea, 0x3c, 0xf0, 0x71, 0x9e, 0x43},
		},
	}

	newState, err := blocks.ProcessDeposit(
		beaconState,
		deposit,
		stateutils.ValidatorIndexMap(beaconState),
		true,
		false,
	)
	if err != nil {
		t.Fatalf("Process deposit failed: %v", err)
	}
	if len(newState.Balances) != 3 {
		t.Errorf("Expected validator balances list to increase by 1, received len %d", len(newState.Balances))
	}
	if newState.Balances[2] != 2000 {
		t.Errorf("Expected new validator have balance of %d, received %d", 2000, newState.Balances[2])
	}
}

func TestProcessDeposit_SkipsDepositWithUncompressedSignature(t *testing.T) {
	// Same test settings as in TestProcessDeposit_PublicKeyDoesNotExist but with a valid signature and signature test
	// on.
	//
	// The key pair was obtained by using the private key (in bytes) 1, 2, 3, ..., 32
	// An example for computing the pair and the signature using the bls library:
	//   key, _ := bls.SecretKeyFromBytes([]byte{1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16,17,18,19,20,21,22,23,24,25,26,
	//                                           27,28,29,30,31,32})
	//   pubBytes := key.PublicKey().Marshal()
	//   deposit.Data.PublicKey = pubBytes
	//   root, _ := ssz.SigningRoot(deposit.Data)
	//   sig := key.Sign(root[:], domain)
	//   b := bytesutil.ToBytes96(sig.Marshal())
	//	 a, _ := blsintern.DecompressG2(b)
	//	 SigDecompressedBytes := a.SerializeBytes()
	//   fmt.Printf("%#v\n", pubBytes)
	//	 fmt.Printf("%#v", SigDecompressedBytes[:])
	registry := []*ethpb.Validator{
		{
			PublicKey:             []byte{1, 2, 3},
			WithdrawalCredentials: []byte{2},
		},
		{
			PublicKey:             []byte{4, 5, 6},
			WithdrawalCredentials: []byte{1},
		},
	}
	balances := []uint64{1000, 1000}
	beaconState := &pb.BeaconState{
		Balances: balances,
		Fork: &pb.Fork{
			Epoch:          0,
			CurrentVersion: []byte{0, 0, 0, 0},
		},
		Validators: registry,
	}

	deposit := &ethpb.Deposit{
		Proof: [][]byte{},
		Data: &ethpb.Deposit_Data{
			PublicKey: []byte{0x96, 0xa2, 0xb, 0xb9, 0x48, 0x5f, 0xf6, 0xd8, 0x95, 0x9, 0x55, 0xa6, 0x29,
				0xe8, 0x4, 0x3a, 0x43, 0x77, 0x59, 0x68, 0xac, 0x13, 0x3e, 0xb7, 0xb1, 0x9c, 0x5f, 0x3, 0x89, 0xa2,
				0x25, 0x36, 0x76, 0xab, 0xdd, 0x6c, 0x86, 0xc7, 0xb6, 0x8d, 0x38, 0xa1, 0xb7, 0xf6, 0xaf, 0x86, 0x50,
				0xe7},
			WithdrawalCredentials: []byte{1},
			Amount:                uint64(2000),
			Signature: []byte{0x11, 0x23, 0x18, 0x37, 0xfd, 0xd, 0x35, 0x99, 0x2a, 0x8f, 0xb, 0xe8, 0xc8, 0x36, 0x8a,
				0x6c, 0x6f, 0x1a, 0x27, 0xe, 0xd1, 0xea, 0x6d, 0x5b, 0x45, 0x8, 0xce, 0xf9, 0x67, 0x9, 0xc9, 0x91, 0xdf,
				0xff, 0x31, 0x87, 0x37, 0x20, 0x85, 0x82, 0x76, 0x92, 0xea, 0x3c, 0xf0, 0x71, 0x9e, 0x43, 0x16, 0xa0,
				0x39, 0x7d, 0x37, 0xa9, 0xed, 0x30, 0xd5, 0xc3, 0x9b, 0x3b, 0xb3, 0x7, 0x62, 0xa0, 0xf4, 0xbc, 0xfc,
				0x4b, 0x98, 0x79, 0x6e, 0x10, 0x8, 0x85, 0xd0, 0x6a, 0xae, 0xe6, 0x87, 0x60, 0x71, 0x32, 0xde, 0x2d,
				0xcc, 0xa2, 0x2c, 0x52, 0x2a, 0xc, 0xb2, 0xc0, 0xa4, 0xf8, 0x89, 0xe4, 0x8, 0x36, 0xd4, 0x83, 0xf3,
				0xb3, 0x94, 0xed, 0x70, 0x4b, 0x88, 0x0, 0x42, 0x64, 0x22, 0xfb, 0x78, 0x37, 0x2b, 0x6, 0xdb, 0xe6,
				0xd4, 0x2e, 0x97, 0x5b, 0x18, 0xbe, 0xf6, 0x16, 0x21, 0x6, 0xd5, 0xd1, 0x46, 0x31, 0x6e, 0xb5, 0xfc,
				0xc2, 0x1c, 0xe, 0xa7, 0xd2, 0x32, 0x73, 0x67, 0x87, 0x19, 0x31, 0x7e, 0xe9, 0x99, 0xde, 0x6d, 0xdb,
				0xf3, 0x8d, 0x2d, 0x43, 0x27, 0x75, 0x7a, 0x47, 0xff, 0x8, 0xf0, 0x1e, 0x86, 0x4d, 0xa6, 0xff, 0x5a,
				0x41, 0x6f, 0x9a, 0x88, 0x4, 0x34, 0xe9, 0x63, 0xda, 0x9e, 0x37, 0x5d, 0x42, 0xb, 0x24, 0x13, 0x3c,
				0xc0, 0x80, 0x90, 0xd9, 0x7f, 0x5f},
		},
	}

	newState, err := blocks.ProcessDeposit(
		beaconState,
		deposit,
		stateutils.ValidatorIndexMap(beaconState),
		true,
		false,
	)
	if err != nil {
		t.Fatalf("Expected invalid block deposit to be ignored without error, received: %v", err)
	}
	if newState.Eth1DepositIndex != 1 {
		t.Errorf(
			"Expected Eth1DepositIndex to be increased by 1 after processing an invalid deposit, received change: %v",
			newState.Eth1DepositIndex,
		)
	}
	if len(newState.Validators) != 2 {
		t.Errorf("Expected validator list to have length 2, received: %v", len(newState.Validators))
	}
	if len(newState.Balances) != 2 {
		t.Errorf("Expected validator balances list to have length 2, received: %v", len(newState.Balances))
	}
}

func TestProcessVoluntaryExits_ValidatorNotActive(t *testing.T) {
	exits := []*ethpb.VoluntaryExit{
		{
			ValidatorIndex: 0,
		},
	}
	registry := []*ethpb.Validator{
		{
			ExitEpoch: 0,
		},
	}
	state := &pb.BeaconState{
		Validators: registry,
	}
	block := &ethpb.BeaconBlock{
		Body: &ethpb.BeaconBlockBody{
			VoluntaryExits: exits,
		},
	}

	want := "non-active validator cannot exit"

	if _, err := blocks.ProcessVoluntaryExits(
		state,
		block.Body,
		false,
	); !strings.Contains(err.Error(), want) {
		t.Errorf("Expected %s, received %v", want, err)
	}
}

func TestProcessVoluntaryExits_InvalidExitEpoch(t *testing.T) {
	exits := []*ethpb.VoluntaryExit{
		{
			Epoch: 10,
		},
	}
	registry := []*ethpb.Validator{
		{
			ExitEpoch: params.BeaconConfig().FarFutureEpoch,
		},
	}
	state := &pb.BeaconState{
		Validators: registry,
		Slot:       0,
	}
	block := &ethpb.BeaconBlock{
		Body: &ethpb.BeaconBlockBody{
			VoluntaryExits: exits,
		},
	}

	want := "expected current epoch >= exit epoch"

	if _, err := blocks.ProcessVoluntaryExits(

		state,
		block.Body,
		false,
	); !strings.Contains(err.Error(), want) {
		t.Errorf("Expected %s, received %v", want, err)
	}
}

func TestProcessVoluntaryExits_NotActiveLongEnoughToExit(t *testing.T) {
	exits := []*ethpb.VoluntaryExit{
		{
			ValidatorIndex: 0,
			Epoch:          0,
		},
	}
	registry := []*ethpb.Validator{
		{
			ExitEpoch: params.BeaconConfig().FarFutureEpoch,
		},
	}
	state := &pb.BeaconState{
		Validators: registry,
		Slot:       10,
	}
	block := &ethpb.BeaconBlock{
		Body: &ethpb.BeaconBlockBody{
			VoluntaryExits: exits,
		},
	}

	want := "validator has not been active long enough to exit"
	if _, err := blocks.ProcessVoluntaryExits(

		state,
		block.Body,
		false,
	); !strings.Contains(err.Error(), want) {
		t.Errorf("Expected %s, received %v", want, err)
	}
}

func TestProcessVoluntaryExits_AppliesCorrectStatus(t *testing.T) {
	exits := []*ethpb.VoluntaryExit{
		{
			ValidatorIndex: 0,
			Epoch:          0,
		},
	}
	registry := []*ethpb.Validator{
		{
			ExitEpoch:       params.BeaconConfig().FarFutureEpoch,
			ActivationEpoch: 0,
		},
	}
	state := &pb.BeaconState{
		Validators: registry,
		Slot:       params.BeaconConfig().SlotsPerEpoch * 5,
	}
	state.Slot = state.Slot + (params.BeaconConfig().PersistentCommitteePeriod * params.BeaconConfig().SlotsPerEpoch)
	block := &ethpb.BeaconBlock{
		Body: &ethpb.BeaconBlockBody{
			VoluntaryExits: exits,
		},
	}
	newState, err := blocks.ProcessVoluntaryExits(state, block.Body, false)
	if err != nil {
		t.Fatalf("Could not process exits: %v", err)
	}
	newRegistry := newState.Validators
	if newRegistry[0].ExitEpoch != helpers.DelayedActivationExitEpoch(state.Slot/params.BeaconConfig().SlotsPerEpoch) {
		t.Errorf("Expected validator exit epoch to be %d, got %d",
			helpers.DelayedActivationExitEpoch(state.Slot/params.BeaconConfig().SlotsPerEpoch), newRegistry[0].ExitEpoch)
	}
}

func TestProcessBeaconTransfers_NotEnoughSenderIndexBalance(t *testing.T) {
	registry := []*ethpb.Validator{
		{
			ActivationEligibilityEpoch: params.BeaconConfig().FarFutureEpoch,
		},
	}
	balances := []uint64{params.BeaconConfig().MaxEffectiveBalance}
	state := &pb.BeaconState{
		Validators: registry,
		Balances:   balances,
	}
	transfers := []*ethpb.Transfer{
		{
			Fee:    params.BeaconConfig().MaxEffectiveBalance,
			Amount: params.BeaconConfig().MaxEffectiveBalance,
		},
	}
	block := &ethpb.BeaconBlock{
		Body: &ethpb.BeaconBlockBody{
			Transfers: transfers,
		},
	}
	want := fmt.Sprintf(
		"expected sender balance %d >= %d",
		balances[0],
		transfers[0].Fee+transfers[0].Amount,
	)
	if _, err := blocks.ProcessTransfers(
		state,
		block.Body,
	); !strings.Contains(err.Error(), want) {
		t.Errorf("Expected %s, received %v", want, err)
	}
}

func TestProcessBeaconTransfers_FailsVerification(t *testing.T) {
	testConfig := params.BeaconConfig()
	testConfig.MaxTransfers = 1
	params.OverrideBeaconConfig(testConfig)
	registry := []*ethpb.Validator{
		{
			ActivationEligibilityEpoch: params.BeaconConfig().FarFutureEpoch,
		},
		{
			ActivationEligibilityEpoch: params.BeaconConfig().FarFutureEpoch,
		},
	}
	balances := []uint64{params.BeaconConfig().MaxEffectiveBalance}
	state := &pb.BeaconState{
		Slot:       0,
		Validators: registry,
		Balances:   balances,
	}
	transfers := []*ethpb.Transfer{
		{
			Fee: params.BeaconConfig().MaxEffectiveBalance + 1,
		},
	}
	block := &ethpb.BeaconBlock{
		Body: &ethpb.BeaconBlockBody{
			Transfers: transfers,
		},
	}
	want := fmt.Sprintf(
		"expected sender balance %d >= %d",
		balances[0],
		transfers[0].Fee,
	)
	if _, err := blocks.ProcessTransfers(
		state,
		block.Body,
	); !strings.Contains(err.Error(), want) {
		t.Errorf("Expected %s, received %v", want, err)
	}

	block.Body.Transfers = []*ethpb.Transfer{
		{
			Fee:  params.BeaconConfig().MinDepositAmount,
			Slot: state.Slot + 1,
		},
	}
	want = fmt.Sprintf(
		"expected beacon state slot %d == transfer slot %d",
		state.Slot,
		block.Body.Transfers[0].Slot,
	)
	if _, err := blocks.ProcessTransfers(
		state,
		block.Body,
	); !strings.Contains(err.Error(), want) {
		t.Errorf("Expected %s, received %v", want, err)
	}

	state.Validators[0].WithdrawableEpoch = params.BeaconConfig().FarFutureEpoch
	state.Validators[0].ActivationEligibilityEpoch = 0
	state.Balances[0] = params.BeaconConfig().MinDepositAmount + params.BeaconConfig().MaxEffectiveBalance
	block.Body.Transfers = []*ethpb.Transfer{
		{
			Fee:    params.BeaconConfig().MinDepositAmount,
			Amount: params.BeaconConfig().MaxEffectiveBalance,
			Slot:   state.Slot,
		},
	}
	want = "over max transfer"
	if _, err := blocks.ProcessTransfers(
		state,
		block.Body,
	); !strings.Contains(err.Error(), want) {
		t.Errorf("Expected %s, received %v", want, err)
	}

	state.Validators[0].WithdrawableEpoch = 0
	state.Validators[0].ActivationEligibilityEpoch = params.BeaconConfig().FarFutureEpoch
	buf := []byte{params.BeaconConfig().BLSWithdrawalPrefixByte}
	pubKey := []byte("B")
	hashed := hashutil.Hash(pubKey)
	buf = append(buf, hashed[:]...)
	state.Validators[0].WithdrawalCredentials = buf
	block.Body.Transfers = []*ethpb.Transfer{
		{
			Fee:                       params.BeaconConfig().MinDepositAmount,
			Amount:                    params.BeaconConfig().MinDepositAmount,
			Slot:                      state.Slot,
			SenderWithdrawalPublicKey: []byte("A"),
		},
	}
	want = "invalid public key"
	if _, err := blocks.ProcessTransfers(
		state,
		block.Body,
	); !strings.Contains(err.Error(), want) {
		t.Errorf("Expected %s, received %v", want, err)
	}
}

func TestProcessBeaconTransfers_OK(t *testing.T) {
	helpers.ClearShuffledValidatorCache()
	testConfig := params.BeaconConfig()
	testConfig.MaxTransfers = 1
	params.OverrideBeaconConfig(testConfig)
	validators := make([]*ethpb.Validator, params.BeaconConfig().MinGenesisActiveValidatorCount/32)
	for i := 0; i < len(validators); i++ {
		validators[i] = &ethpb.Validator{
			ActivationEpoch:   0,
			ExitEpoch:         params.BeaconConfig().FarFutureEpoch,
			Slashed:           false,
			WithdrawableEpoch: 0,
		}
	}
	validatorBalances := make([]uint64, len(validators))
	for i := 0; i < len(validatorBalances); i++ {
		validatorBalances[i] = params.BeaconConfig().MaxEffectiveBalance
	}

	state := &pb.BeaconState{
		Validators: validators,
		Slot:       0,
		Balances:   validatorBalances,
		Fork: &pb.Fork{
			CurrentVersion:  params.BeaconConfig().GenesisForkVersion,
			PreviousVersion: params.BeaconConfig().GenesisForkVersion,
		},
		RandaoMixes:      make([][]byte, params.BeaconConfig().EpochsPerHistoricalVector),
		Slashings:        make([]uint64, params.BeaconConfig().EpochsPerSlashingsVector),
		ActiveIndexRoots: make([][]byte, params.BeaconConfig().EpochsPerHistoricalVector),
	}

	transfer := &ethpb.Transfer{
		SenderIndex:    0,
		RecipientIndex: 1,
		Fee:            params.BeaconConfig().MinDepositAmount,
		Amount:         params.BeaconConfig().MinDepositAmount,
		Slot:           state.Slot,
	}

	priv, err := bls.RandKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	pubKey := priv.PublicKey().Marshal()[:]
	transfer.SenderWithdrawalPublicKey = pubKey
	state.Validators[transfer.SenderIndex].PublicKey = pubKey
	signingRoot, err := ssz.SigningRoot(transfer)
	if err != nil {
		t.Fatalf("Failed to get signing root of block: %v", err)
	}
	epoch := helpers.CurrentEpoch(state)
	dt := helpers.Domain(state, epoch, params.BeaconConfig().DomainTransfer)
	transferSig := priv.Sign(signingRoot[:], dt)
	transfer.Signature = transferSig.Marshal()[:]

	block := &ethpb.BeaconBlock{
		Body: &ethpb.BeaconBlockBody{
			Transfers: []*ethpb.Transfer{transfer},
		},
	}
	buf := []byte{params.BeaconConfig().BLSWithdrawalPrefixByte}
	hashed := hashutil.Hash(pubKey)
	buf = append(buf, hashed[:][1:]...)
	state.Validators[0].WithdrawalCredentials = buf
	state.Validators[0].ActivationEligibilityEpoch = params.BeaconConfig().FarFutureEpoch
	newState, err := blocks.ProcessTransfers(state, block.Body)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	expectedRecipientIndex := params.BeaconConfig().MaxEffectiveBalance + block.Body.Transfers[0].Amount
	if newState.Balances[1] != expectedRecipientIndex {
		t.Errorf("Expected recipient balance %d, received %d", newState.Balances[1], expectedRecipientIndex)
	}
	expectedSenderIndex := params.BeaconConfig().MaxEffectiveBalance - block.Body.Transfers[0].Amount - block.Body.Transfers[0].Fee
	if newState.Balances[0] != expectedSenderIndex {
		t.Errorf("Expected sender balance %d, received %d", newState.Balances[0], expectedSenderIndex)
	}
}
