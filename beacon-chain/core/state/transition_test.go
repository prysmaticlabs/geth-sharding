package state_test

import (
	"context"
	"encoding/binary"
	"fmt"
	"testing"

	types "github.com/prysmaticlabs/eth2-types"
	"github.com/prysmaticlabs/go-bitfield"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/blocks"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/state"
	iface "github.com/prysmaticlabs/prysm/beacon-chain/state/interface"
	v1 "github.com/prysmaticlabs/prysm/beacon-chain/state/v1"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	ethpb "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/proto/eth/v1alpha1/wrapper"
	"github.com/prysmaticlabs/prysm/shared/attestationutil"
	"github.com/prysmaticlabs/prysm/shared/bls"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/prysmaticlabs/prysm/shared/hashutil"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/testutil"
	"github.com/prysmaticlabs/prysm/shared/testutil/assert"
	"github.com/prysmaticlabs/prysm/shared/testutil/require"
	"github.com/prysmaticlabs/prysm/shared/trieutil"
	"github.com/sirupsen/logrus"
)

func init() {
	state.SkipSlotCache.Disable()
}

func TestExecuteStateTransition_IncorrectSlot(t *testing.T) {
	base := &pb.BeaconState{
		Slot: 5,
	}
	beaconState, err := v1.InitializeFromProto(base)
	require.NoError(t, err)
	block := &ethpb.SignedBeaconBlock{
		Block: &ethpb.BeaconBlock{
			Slot: 4,
			Body: &ethpb.BeaconBlockBody{},
		},
	}
	want := "expected state.slot"
	_, err = state.ExecuteStateTransition(context.Background(), beaconState, wrapper.WrappedPhase0SignedBeaconBlock(block))
	assert.ErrorContains(t, want, err)
}

func TestExecuteStateTransition_FullProcess(t *testing.T) {
	beaconState, privKeys := testutil.DeterministicGenesisState(t, 100)

	eth1Data := &ethpb.Eth1Data{
		DepositCount: 100,
		DepositRoot:  bytesutil.PadTo([]byte{2}, 32),
		BlockHash:    make([]byte, 32),
	}
	require.NoError(t, beaconState.SetSlot(params.BeaconConfig().SlotsPerEpoch-1))
	e := beaconState.Eth1Data()
	e.DepositCount = 100
	require.NoError(t, beaconState.SetEth1Data(e))
	bh := beaconState.LatestBlockHeader()
	bh.Slot = beaconState.Slot()
	require.NoError(t, beaconState.SetLatestBlockHeader(bh))
	require.NoError(t, beaconState.SetEth1DataVotes([]*ethpb.Eth1Data{eth1Data}))

	oldMix, err := beaconState.RandaoMixAtIndex(1)
	require.NoError(t, err)

	require.NoError(t, beaconState.SetSlot(beaconState.Slot()+1))
	epoch := helpers.CurrentEpoch(beaconState)
	randaoReveal, err := testutil.RandaoReveal(beaconState, epoch, privKeys)
	require.NoError(t, err)
	require.NoError(t, beaconState.SetSlot(beaconState.Slot()-1))

	nextSlotState, err := state.ProcessSlots(context.Background(), beaconState.Copy(), beaconState.Slot()+1)
	require.NoError(t, err)
	parentRoot, err := nextSlotState.LatestBlockHeader().HashTreeRoot()
	require.NoError(t, err)
	proposerIdx, err := helpers.BeaconProposerIndex(nextSlotState)
	require.NoError(t, err)
	block := testutil.NewBeaconBlock()
	block.Block.ProposerIndex = proposerIdx
	block.Block.Slot = beaconState.Slot() + 1
	block.Block.ParentRoot = parentRoot[:]
	block.Block.Body.RandaoReveal = randaoReveal
	block.Block.Body.Eth1Data = eth1Data

	stateRoot, err := state.CalculateStateRoot(context.Background(), beaconState, wrapper.WrappedPhase0SignedBeaconBlock(block))
	require.NoError(t, err)

	block.Block.StateRoot = stateRoot[:]

	sig, err := testutil.BlockSignature(beaconState, block.Block, privKeys)
	require.NoError(t, err)
	block.Signature = sig.Marshal()

	beaconState, err = state.ExecuteStateTransition(context.Background(), beaconState, wrapper.WrappedPhase0SignedBeaconBlock(block))
	require.NoError(t, err)

	assert.Equal(t, params.BeaconConfig().SlotsPerEpoch, beaconState.Slot(), "Unexpected Slot number")

	mix, err := beaconState.RandaoMixAtIndex(1)
	require.NoError(t, err)
	assert.DeepNotEqual(t, oldMix, mix, "Did not expect new and old randao mix to equal")
}

func TestProcessBlock_IncorrectProposerSlashing(t *testing.T) {
	beaconState, privKeys := testutil.DeterministicGenesisState(t, 100)

	block, err := testutil.GenerateFullBlock(beaconState, privKeys, nil, 1)
	require.NoError(t, err)
	slashing := &ethpb.ProposerSlashing{
		Header_1: testutil.HydrateSignedBeaconHeader(&ethpb.SignedBeaconBlockHeader{
			Header: &ethpb.BeaconBlockHeader{
				Slot: params.BeaconConfig().SlotsPerEpoch,
			},
		}),
		Header_2: testutil.HydrateSignedBeaconHeader(&ethpb.SignedBeaconBlockHeader{
			Header: &ethpb.BeaconBlockHeader{
				Slot: params.BeaconConfig().SlotsPerEpoch * 2,
			},
		}),
	}
	block.Block.Body.ProposerSlashings = []*ethpb.ProposerSlashing{slashing}

	require.NoError(t, beaconState.SetSlot(beaconState.Slot()+1))
	proposerIdx, err := helpers.BeaconProposerIndex(beaconState)
	require.NoError(t, err)
	require.NoError(t, beaconState.SetSlot(beaconState.Slot()-1))
	block.Signature, err = helpers.ComputeDomainAndSign(beaconState, helpers.CurrentEpoch(beaconState), block.Block, params.BeaconConfig().DomainBeaconProposer, privKeys[proposerIdx])
	require.NoError(t, err)

	beaconState, err = state.ProcessSlots(context.Background(), beaconState, 1)
	require.NoError(t, err)
	want := "could not verify proposer slashing"
	_, err = state.ProcessBlock(context.Background(), beaconState, wrapper.WrappedPhase0SignedBeaconBlock(block))
	assert.ErrorContains(t, want, err)
}

func TestProcessBlock_IncorrectProcessBlockAttestations(t *testing.T) {
	beaconState, privKeys := testutil.DeterministicGenesisState(t, 100)
	priv, err := bls.RandKey()
	require.NoError(t, err)
	att := testutil.HydrateAttestation(&ethpb.Attestation{
		AggregationBits: bitfield.NewBitlist(3),
		Signature:       priv.Sign([]byte("foo")).Marshal(),
	})

	block, err := testutil.GenerateFullBlock(beaconState, privKeys, nil, 1)
	require.NoError(t, err)
	block.Block.Body.Attestations = []*ethpb.Attestation{att}
	require.NoError(t, beaconState.SetSlot(beaconState.Slot()+1))
	proposerIdx, err := helpers.BeaconProposerIndex(beaconState)
	require.NoError(t, err)
	require.NoError(t, beaconState.SetSlot(beaconState.Slot()-1))
	block.Signature, err = helpers.ComputeDomainAndSign(beaconState, helpers.CurrentEpoch(beaconState), block.Block, params.BeaconConfig().DomainBeaconProposer, privKeys[proposerIdx])
	require.NoError(t, err)

	beaconState, err = state.ProcessSlots(context.Background(), beaconState, 1)
	require.NoError(t, err)

	want := "could not verify attestation"
	_, err = state.ProcessBlock(context.Background(), beaconState, wrapper.WrappedPhase0SignedBeaconBlock(block))
	assert.ErrorContains(t, want, err)
}

func TestProcessBlock_IncorrectProcessExits(t *testing.T) {
	beaconState, _ := testutil.DeterministicGenesisState(t, 100)

	proposerSlashings := []*ethpb.ProposerSlashing{
		{
			Header_1: testutil.HydrateSignedBeaconHeader(&ethpb.SignedBeaconBlockHeader{
				Header: &ethpb.BeaconBlockHeader{
					ProposerIndex: 3,
					Slot:          1,
				},
				Signature: bytesutil.PadTo([]byte("A"), 96),
			}),
			Header_2: testutil.HydrateSignedBeaconHeader(&ethpb.SignedBeaconBlockHeader{
				Header: &ethpb.BeaconBlockHeader{
					ProposerIndex: 3,
					Slot:          1,
				},
				Signature: bytesutil.PadTo([]byte("B"), 96),
			}),
		},
	}
	attesterSlashings := []*ethpb.AttesterSlashing{
		{
			Attestation_1: &ethpb.IndexedAttestation{
				Data:             testutil.HydrateAttestationData(&ethpb.AttestationData{}),
				AttestingIndices: []uint64{0, 1},
				Signature:        make([]byte, 96),
			},
			Attestation_2: &ethpb.IndexedAttestation{
				Data:             testutil.HydrateAttestationData(&ethpb.AttestationData{}),
				AttestingIndices: []uint64{0, 1},
				Signature:        make([]byte, 96),
			},
		},
	}
	var blockRoots [][]byte
	for i := uint64(0); i < uint64(params.BeaconConfig().SlotsPerHistoricalRoot); i++ {
		blockRoots = append(blockRoots, []byte{byte(i)})
	}
	require.NoError(t, beaconState.SetBlockRoots(blockRoots))
	blockAtt := testutil.HydrateAttestation(&ethpb.Attestation{
		Data: &ethpb.AttestationData{
			Target: &ethpb.Checkpoint{Root: bytesutil.PadTo([]byte("hello-world"), 32)},
		},
		AggregationBits: bitfield.Bitlist{0xC0, 0xC0, 0xC0, 0xC0, 0x01},
	})
	attestations := []*ethpb.Attestation{blockAtt}
	var exits []*ethpb.SignedVoluntaryExit
	for i := uint64(0); i < params.BeaconConfig().MaxVoluntaryExits+1; i++ {
		exits = append(exits, &ethpb.SignedVoluntaryExit{})
	}
	genesisBlock := blocks.NewGenesisBlock([]byte{})
	bodyRoot, err := genesisBlock.Block.HashTreeRoot()
	require.NoError(t, err)
	err = beaconState.SetLatestBlockHeader(testutil.HydrateBeaconHeader(&ethpb.BeaconBlockHeader{
		Slot:       genesisBlock.Block.Slot,
		ParentRoot: genesisBlock.Block.ParentRoot,
		BodyRoot:   bodyRoot[:],
	}))
	require.NoError(t, err)
	parentRoot, err := beaconState.LatestBlockHeader().HashTreeRoot()
	require.NoError(t, err)
	block := testutil.NewBeaconBlock()
	block.Block.Slot = 1
	block.Block.ParentRoot = parentRoot[:]
	block.Block.Body.ProposerSlashings = proposerSlashings
	block.Block.Body.Attestations = attestations
	block.Block.Body.AttesterSlashings = attesterSlashings
	block.Block.Body.VoluntaryExits = exits
	block.Block.Body.Eth1Data.DepositRoot = bytesutil.PadTo([]byte{2}, 32)
	block.Block.Body.Eth1Data.BlockHash = bytesutil.PadTo([]byte{3}, 32)
	err = beaconState.SetSlot(beaconState.Slot() + params.BeaconConfig().MinAttestationInclusionDelay)
	require.NoError(t, err)
	cp := beaconState.CurrentJustifiedCheckpoint()
	cp.Root = []byte("hello-world")
	require.NoError(t, beaconState.SetCurrentJustifiedCheckpoint(cp))
	require.NoError(t, beaconState.AppendCurrentEpochAttestations(&pb.PendingAttestation{}))
	_, err = state.VerifyOperationLengths(context.Background(), beaconState, wrapper.WrappedPhase0SignedBeaconBlock(block))
	wanted := "number of voluntary exits (17) in block body exceeds allowed threshold of 16"
	assert.ErrorContains(t, wanted, err)
}

func createFullBlockWithOperations(t *testing.T) (iface.BeaconState,
	*ethpb.SignedBeaconBlock, []*ethpb.Attestation, []*ethpb.ProposerSlashing, []*ethpb.SignedVoluntaryExit) {
	beaconState, privKeys := testutil.DeterministicGenesisState(t, 32)
	genesisBlock := blocks.NewGenesisBlock([]byte{})
	bodyRoot, err := genesisBlock.Block.HashTreeRoot()
	require.NoError(t, err)
	err = beaconState.SetLatestBlockHeader(&ethpb.BeaconBlockHeader{
		Slot:       genesisBlock.Block.Slot,
		ParentRoot: genesisBlock.Block.ParentRoot,
		StateRoot:  params.BeaconConfig().ZeroHash[:],
		BodyRoot:   bodyRoot[:],
	})
	require.NoError(t, err)
	err = beaconState.SetSlashings(make([]uint64, params.BeaconConfig().EpochsPerSlashingsVector))
	require.NoError(t, err)
	cp := beaconState.CurrentJustifiedCheckpoint()
	mockRoot := [32]byte{}
	copy(mockRoot[:], "hello-world")
	cp.Root = mockRoot[:]
	require.NoError(t, beaconState.SetCurrentJustifiedCheckpoint(cp))
	require.NoError(t, beaconState.AppendCurrentEpochAttestations(&pb.PendingAttestation{}))

	proposerSlashIdx := types.ValidatorIndex(3)
	slotsPerEpoch := params.BeaconConfig().SlotsPerEpoch
	err = beaconState.SetSlot(slotsPerEpoch.Mul(uint64(params.BeaconConfig().ShardCommitteePeriod)) + params.BeaconConfig().MinAttestationInclusionDelay)
	require.NoError(t, err)

	currentEpoch := helpers.CurrentEpoch(beaconState)
	header1 := testutil.HydrateSignedBeaconHeader(&ethpb.SignedBeaconBlockHeader{
		Header: &ethpb.BeaconBlockHeader{
			ProposerIndex: proposerSlashIdx,
			Slot:          1,
			StateRoot:     bytesutil.PadTo([]byte("A"), 32),
		},
	})
	header1.Signature, err = helpers.ComputeDomainAndSign(beaconState, currentEpoch, header1.Header, params.BeaconConfig().DomainBeaconProposer, privKeys[proposerSlashIdx])
	require.NoError(t, err)

	header2 := testutil.HydrateSignedBeaconHeader(&ethpb.SignedBeaconBlockHeader{
		Header: &ethpb.BeaconBlockHeader{
			ProposerIndex: proposerSlashIdx,
			Slot:          1,
			StateRoot:     bytesutil.PadTo([]byte("B"), 32),
		},
	})
	header2.Signature, err = helpers.ComputeDomainAndSign(beaconState, helpers.CurrentEpoch(beaconState), header2.Header, params.BeaconConfig().DomainBeaconProposer, privKeys[proposerSlashIdx])
	require.NoError(t, err)

	proposerSlashings := []*ethpb.ProposerSlashing{
		{
			Header_1: header1,
			Header_2: header2,
		},
	}
	validators := beaconState.Validators()
	validators[proposerSlashIdx].PublicKey = privKeys[proposerSlashIdx].PublicKey().Marshal()
	require.NoError(t, beaconState.SetValidators(validators))

	mockRoot2 := [32]byte{'A'}
	att1 := testutil.HydrateIndexedAttestation(&ethpb.IndexedAttestation{
		Data: &ethpb.AttestationData{
			Source: &ethpb.Checkpoint{Epoch: 0, Root: mockRoot2[:]},
		},
		AttestingIndices: []uint64{0, 1},
	})
	domain, err := helpers.Domain(beaconState.Fork(), currentEpoch, params.BeaconConfig().DomainBeaconAttester, beaconState.GenesisValidatorRoot())
	require.NoError(t, err)
	hashTreeRoot, err := helpers.ComputeSigningRoot(att1.Data, domain)
	require.NoError(t, err)
	sig0 := privKeys[0].Sign(hashTreeRoot[:])
	sig1 := privKeys[1].Sign(hashTreeRoot[:])
	aggregateSig := bls.AggregateSignatures([]bls.Signature{sig0, sig1})
	att1.Signature = aggregateSig.Marshal()

	mockRoot3 := [32]byte{'B'}
	att2 := testutil.HydrateIndexedAttestation(&ethpb.IndexedAttestation{
		Data: &ethpb.AttestationData{
			Source: &ethpb.Checkpoint{Epoch: 0, Root: mockRoot3[:]},
			Target: &ethpb.Checkpoint{Epoch: 0, Root: make([]byte, 32)},
		},
		AttestingIndices: []uint64{0, 1},
	})

	hashTreeRoot, err = helpers.ComputeSigningRoot(att2.Data, domain)
	require.NoError(t, err)
	sig0 = privKeys[0].Sign(hashTreeRoot[:])
	sig1 = privKeys[1].Sign(hashTreeRoot[:])
	aggregateSig = bls.AggregateSignatures([]bls.Signature{sig0, sig1})
	att2.Signature = aggregateSig.Marshal()

	attesterSlashings := []*ethpb.AttesterSlashing{
		{
			Attestation_1: att1,
			Attestation_2: att2,
		},
	}

	var blockRoots [][]byte
	for i := uint64(0); i < uint64(params.BeaconConfig().SlotsPerHistoricalRoot); i++ {
		blockRoots = append(blockRoots, []byte{byte(i)})
	}
	require.NoError(t, beaconState.SetBlockRoots(blockRoots))

	aggBits := bitfield.NewBitlist(1)
	aggBits.SetBitAt(0, true)
	blockAtt := testutil.HydrateAttestation(&ethpb.Attestation{
		Data: &ethpb.AttestationData{
			Slot:   beaconState.Slot(),
			Target: &ethpb.Checkpoint{Epoch: helpers.CurrentEpoch(beaconState)},
			Source: &ethpb.Checkpoint{Root: mockRoot[:]}},
		AggregationBits: aggBits,
	})

	committee, err := helpers.BeaconCommitteeFromState(beaconState, blockAtt.Data.Slot, blockAtt.Data.CommitteeIndex)
	assert.NoError(t, err)
	attestingIndices, err := attestationutil.AttestingIndices(blockAtt.AggregationBits, committee)
	require.NoError(t, err)
	assert.NoError(t, err)
	hashTreeRoot, err = helpers.ComputeSigningRoot(blockAtt.Data, domain)
	assert.NoError(t, err)
	sigs := make([]bls.Signature, len(attestingIndices))
	for i, indice := range attestingIndices {
		sig := privKeys[indice].Sign(hashTreeRoot[:])
		sigs[i] = sig
	}
	blockAtt.Signature = bls.AggregateSignatures(sigs).Marshal()

	exit := &ethpb.SignedVoluntaryExit{
		Exit: &ethpb.VoluntaryExit{
			ValidatorIndex: 10,
			Epoch:          0,
		},
	}
	exit.Signature, err = helpers.ComputeDomainAndSign(beaconState, currentEpoch, exit.Exit, params.BeaconConfig().DomainVoluntaryExit, privKeys[exit.Exit.ValidatorIndex])
	require.NoError(t, err)

	header := beaconState.LatestBlockHeader()
	prevStateRoot, err := beaconState.HashTreeRoot(context.Background())
	require.NoError(t, err)
	header.StateRoot = prevStateRoot[:]
	require.NoError(t, beaconState.SetLatestBlockHeader(header))
	parentRoot, err := beaconState.LatestBlockHeader().HashTreeRoot()
	require.NoError(t, err)
	copied := beaconState.Copy()
	require.NoError(t, copied.SetSlot(beaconState.Slot()+1))
	randaoReveal, err := testutil.RandaoReveal(copied, currentEpoch, privKeys)
	require.NoError(t, err)
	proposerIndex, err := helpers.BeaconProposerIndex(copied)
	require.NoError(t, err)
	block := testutil.HydrateSignedBeaconBlock(&ethpb.SignedBeaconBlock{
		Block: &ethpb.BeaconBlock{
			ParentRoot:    parentRoot[:],
			Slot:          beaconState.Slot() + 1,
			ProposerIndex: proposerIndex,
			Body: &ethpb.BeaconBlockBody{
				RandaoReveal:      randaoReveal,
				ProposerSlashings: proposerSlashings,
				AttesterSlashings: attesterSlashings,
				Attestations:      []*ethpb.Attestation{blockAtt},
				VoluntaryExits:    []*ethpb.SignedVoluntaryExit{exit},
			},
		},
	})

	sig, err := testutil.BlockSignature(beaconState, block.Block, privKeys)
	require.NoError(t, err)
	block.Signature = sig.Marshal()

	require.NoError(t, beaconState.SetSlot(block.Block.Slot))
	return beaconState, block, []*ethpb.Attestation{blockAtt}, proposerSlashings, []*ethpb.SignedVoluntaryExit{exit}
}

func TestProcessBlock_PassesProcessingConditions(t *testing.T) {
	beaconState, block, _, proposerSlashings, exits := createFullBlockWithOperations(t)
	exit := exits[0]

	beaconState, err := state.ProcessBlock(context.Background(), beaconState, wrapper.WrappedPhase0SignedBeaconBlock(block))
	require.NoError(t, err, "Expected block to pass processing conditions")

	v, err := beaconState.ValidatorAtIndex(proposerSlashings[0].Header_1.Header.ProposerIndex)
	require.NoError(t, err)
	assert.Equal(t, true, v.Slashed, "Expected validator at index %d to be slashed", proposerSlashings[0].Header_1.Header.ProposerIndex)
	v, err = beaconState.ValidatorAtIndex(1)
	require.NoError(t, err)
	assert.Equal(t, true, v.Slashed, "Expected validator at index 1 to be slashed, received false")

	v, err = beaconState.ValidatorAtIndex(exit.Exit.ValidatorIndex)
	require.NoError(t, err)
	received := v.ExitEpoch
	wanted := params.BeaconConfig().FarFutureEpoch
	assert.NotEqual(t, wanted, received, "Expected validator at index %d to be exiting", exit.Exit.ValidatorIndex)
}

func TestProcessEpochPrecompute_CanProcess(t *testing.T) {
	epoch := types.Epoch(1)

	atts := []*pb.PendingAttestation{{Data: &ethpb.AttestationData{Target: &ethpb.Checkpoint{Root: make([]byte, 32)}}, InclusionDelay: 1}}
	slashing := make([]uint64, params.BeaconConfig().EpochsPerSlashingsVector)
	base := &pb.BeaconState{
		Slot:                       params.BeaconConfig().SlotsPerEpoch.Mul(uint64(epoch)) + 1,
		BlockRoots:                 make([][]byte, 128),
		Slashings:                  slashing,
		RandaoMixes:                make([][]byte, params.BeaconConfig().EpochsPerHistoricalVector),
		CurrentEpochAttestations:   atts,
		FinalizedCheckpoint:        &ethpb.Checkpoint{Root: make([]byte, 32)},
		JustificationBits:          bitfield.Bitvector4{0x00},
		CurrentJustifiedCheckpoint: &ethpb.Checkpoint{Root: make([]byte, 32)},
	}
	s, err := v1.InitializeFromProto(base)
	require.NoError(t, err)
	require.NoError(t, s.SetValidators([]*ethpb.Validator{}))
	newState, err := state.ProcessEpochPrecompute(context.Background(), s)
	require.NoError(t, err)
	assert.Equal(t, uint64(0), newState.Slashings()[2], "Unexpected slashed balance")
}

func BenchmarkProcessBlk_65536Validators_FullBlock(b *testing.B) {
	logrus.SetLevel(logrus.PanicLevel)

	validatorCount := params.BeaconConfig().MinGenesisActiveValidatorCount * 4
	committeeCount := validatorCount / params.BeaconConfig().TargetCommitteeSize
	validators := make([]*ethpb.Validator, validatorCount)
	for i := 0; i < len(validators); i++ {
		validators[i] = &ethpb.Validator{
			EffectiveBalance:           params.BeaconConfig().MaxEffectiveBalance,
			ExitEpoch:                  params.BeaconConfig().FarFutureEpoch,
			WithdrawableEpoch:          params.BeaconConfig().FarFutureEpoch,
			ActivationEligibilityEpoch: params.BeaconConfig().FarFutureEpoch,
		}
	}
	validatorBalances := make([]uint64, len(validators))
	for i := 0; i < len(validatorBalances); i++ {
		validatorBalances[i] = params.BeaconConfig().MaxEffectiveBalance
	}

	randaoMixes := make([][]byte, params.BeaconConfig().EpochsPerHistoricalVector)
	for i := 0; i < len(randaoMixes); i++ {
		randaoMixes[i] = params.BeaconConfig().ZeroHash[:]
	}

	base := &pb.BeaconState{
		Slot:              20,
		LatestBlockHeader: &ethpb.BeaconBlockHeader{},
		BlockRoots:        make([][]byte, 254),
		RandaoMixes:       randaoMixes,
		Validators:        validators,
		Balances:          validatorBalances,
		Slashings:         make([]uint64, params.BeaconConfig().EpochsPerSlashingsVector),
		CurrentJustifiedCheckpoint: &ethpb.Checkpoint{
			Root: []byte("hello-world"),
		},
		Fork: &pb.Fork{
			PreviousVersion: []byte{0, 0, 0, 0},
			CurrentVersion:  []byte{0, 0, 0, 0},
		},
	}
	s, err := v1.InitializeFromProto(base)
	require.NoError(b, err)

	// Set up proposer slashing object for block
	proposerSlashings := []*ethpb.ProposerSlashing{
		{
			Header_1: &ethpb.SignedBeaconBlockHeader{
				Header: &ethpb.BeaconBlockHeader{
					ProposerIndex: 1,
					Slot:          0,
				},
				Signature: bytesutil.PadTo([]byte("A"), 96),
			},
			Header_2: &ethpb.SignedBeaconBlockHeader{
				Header: &ethpb.BeaconBlockHeader{
					ProposerIndex: 1,
					Slot:          0,
				},
				Signature: bytesutil.PadTo([]byte("B"), 96),
			},
		},
	}

	// Set up attester slashing object for block
	attesterSlashings := []*ethpb.AttesterSlashing{
		{
			Attestation_1: testutil.HydrateIndexedAttestation(&ethpb.IndexedAttestation{
				AttestingIndices: []uint64{2, 3},
			}),
			Attestation_2: testutil.HydrateIndexedAttestation(&ethpb.IndexedAttestation{
				AttestingIndices: []uint64{2, 3},
			}),
		},
	}

	// Set up deposit object for block
	deposit := &ethpb.Deposit{
		Data: &ethpb.Deposit_Data{
			PublicKey: []byte{1, 2, 3},
			Amount:    params.BeaconConfig().MaxEffectiveBalance,
		},
	}
	leaf, err := deposit.Data.HashTreeRoot()
	require.NoError(b, err)
	depositTrie, err := trieutil.GenerateTrieFromItems([][]byte{leaf[:]}, params.BeaconConfig().DepositContractTreeDepth)
	require.NoError(b, err, "Could not generate trie")
	proof, err := depositTrie.MerkleProof(0)
	require.NoError(b, err, "Could not generate proof")
	deposit.Proof = proof
	root := depositTrie.Root()

	// Set up randao reveal object for block
	proposerIdx, err := helpers.BeaconProposerIndex(s)
	require.NoError(b, err)
	priv, err := bls.RandKey()
	require.NoError(b, err)
	v := s.Validators()
	v[proposerIdx].PublicKey = priv.PublicKey().Marshal()
	buf := make([]byte, 32)
	binary.LittleEndian.PutUint64(buf, 0)
	domain, err := helpers.Domain(s.Fork(), 0, params.BeaconConfig().DomainRandao, s.GenesisValidatorRoot())
	require.NoError(b, err)
	ctr := &pb.SigningData{
		ObjectRoot: buf,
		Domain:     domain,
	}
	root, err = ctr.HashTreeRoot()
	require.NoError(b, err)
	epochSignature := priv.Sign(root[:])

	buf = []byte{params.BeaconConfig().BLSWithdrawalPrefixByte}
	pubKey := []byte("A")
	hashed := hashutil.Hash(pubKey)
	buf = append(buf, hashed[:]...)
	v[3].WithdrawalCredentials = buf

	require.NoError(b, s.SetValidators(v))

	attestations := make([]*ethpb.Attestation, 128)
	for i := 0; i < len(attestations); i++ {
		attestations[i] = &ethpb.Attestation{
			Data: &ethpb.AttestationData{
				Source: &ethpb.Checkpoint{Root: []byte("hello-world")}},
			AggregationBits: bitfield.Bitlist{0xC0, 0xC0, 0xC0, 0xC0, 0xC0, 0xC0, 0xC0,
				0xC0, 0xC0, 0xC0, 0xC0, 0xC0, 0xC0, 0xC0, 0xC0, 0xC0, 0x01},
		}
	}

	blk := &ethpb.SignedBeaconBlock{
		Block: &ethpb.BeaconBlock{
			Slot: s.Slot(),
			Body: &ethpb.BeaconBlockBody{
				Eth1Data: &ethpb.Eth1Data{
					DepositRoot: root[:],
					BlockHash:   root[:],
				},
				RandaoReveal:      epochSignature.Marshal(),
				Attestations:      attestations,
				ProposerSlashings: proposerSlashings,
				AttesterSlashings: attesterSlashings,
			},
		},
	}

	// Precache the shuffled indices
	for i := uint64(0); i < committeeCount; i++ {
		_, err := helpers.BeaconCommitteeFromState(s, 0, types.CommitteeIndex(i))
		require.NoError(b, err)
	}

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		_, err := state.ProcessBlock(context.Background(), s, wrapper.WrappedPhase0SignedBeaconBlock(blk))
		require.NoError(b, err)
		// Reset state fields to process block again
		v := s.Validators()
		v[1].Slashed = false
		v[2].Slashed = false
		require.NoError(b, s.SetValidators(v))
		balances := s.Balances()
		balances[3] += 2 * params.BeaconConfig().MinDepositAmount
		require.NoError(b, s.SetBalances(balances))
	}
}

func TestProcessBlk_AttsBasedOnValidatorCount(t *testing.T) {
	logrus.SetLevel(logrus.PanicLevel)

	// Default at 256 validators, can raise this number with faster BLS.
	validatorCount := uint64(256)
	s, privKeys := testutil.DeterministicGenesisState(t, validatorCount)
	require.NoError(t, s.SetSlot(params.BeaconConfig().SlotsPerEpoch))

	bitCount := validatorCount / uint64(params.BeaconConfig().SlotsPerEpoch)
	aggBits := bitfield.NewBitlist(bitCount)
	for i := uint64(1); i < bitCount; i++ {
		aggBits.SetBitAt(i, true)
	}
	atts := make([]*ethpb.Attestation, 1)

	for i := 0; i < len(atts); i++ {
		att := testutil.HydrateAttestation(&ethpb.Attestation{
			Data:            &ethpb.AttestationData{Slot: 1},
			AggregationBits: aggBits,
		})

		committee, err := helpers.BeaconCommitteeFromState(s, att.Data.Slot, att.Data.CommitteeIndex)
		assert.NoError(t, err)
		attestingIndices, err := attestationutil.AttestingIndices(att.AggregationBits, committee)
		require.NoError(t, err)
		domain, err := helpers.Domain(s.Fork(), 0, params.BeaconConfig().DomainBeaconAttester, s.GenesisValidatorRoot())
		require.NoError(t, err)
		sigs := make([]bls.Signature, len(attestingIndices))
		for i, indice := range attestingIndices {
			hashTreeRoot, err := helpers.ComputeSigningRoot(att.Data, domain)
			assert.NoError(t, err)
			sig := privKeys[indice].Sign(hashTreeRoot[:])
			sigs[i] = sig
		}
		att.Signature = bls.AggregateSignatures(sigs).Marshal()
		atts[i] = att
	}

	copied := s.Copy()
	require.NoError(t, copied.SetSlot(s.Slot()+1))
	epochSignature, err := testutil.RandaoReveal(copied, helpers.CurrentEpoch(copied), privKeys)
	require.NoError(t, err)
	header := s.LatestBlockHeader()
	prevStateRoot, err := s.HashTreeRoot(context.Background())
	require.NoError(t, err)
	header.StateRoot = prevStateRoot[:]
	require.NoError(t, s.SetLatestBlockHeader(header))

	parentRoot, err := s.LatestBlockHeader().HashTreeRoot()
	require.NoError(t, err)

	nextSlotState := s.Copy()
	require.NoError(t, nextSlotState.SetSlot(s.Slot()+1))
	proposerIdx, err := helpers.BeaconProposerIndex(nextSlotState)
	require.NoError(t, err)
	blk := testutil.NewBeaconBlock()
	blk.Block.ProposerIndex = proposerIdx
	blk.Block.Slot = s.Slot() + 1
	blk.Block.ParentRoot = parentRoot[:]
	blk.Block.Body.RandaoReveal = epochSignature
	blk.Block.Body.Attestations = atts
	sig, err := testutil.BlockSignature(s, blk.Block, privKeys)
	require.NoError(t, err)
	blk.Signature = sig.Marshal()

	params.SetupTestConfigCleanup(t)
	config := params.BeaconConfig()
	config.MinAttestationInclusionDelay = 0
	params.OverrideBeaconConfig(config)

	require.NoError(t, s.SetSlot(s.Slot()+1))
	_, err = state.ProcessBlock(context.Background(), s, wrapper.WrappedPhase0SignedBeaconBlock(blk))
	require.NoError(t, err)
}

func TestCanProcessEpoch_TrueOnEpochs(t *testing.T) {
	tests := []struct {
		slot            types.Slot
		canProcessEpoch bool
	}{
		{
			slot:            1,
			canProcessEpoch: false,
		}, {
			slot:            63,
			canProcessEpoch: true,
		},
		{
			slot:            64,
			canProcessEpoch: false,
		}, {
			slot:            127,
			canProcessEpoch: true,
		}, {
			slot:            1000000000,
			canProcessEpoch: false,
		},
	}

	for _, tt := range tests {
		b := &pb.BeaconState{Slot: tt.slot}
		s, err := v1.InitializeFromProto(b)
		require.NoError(t, err)
		assert.Equal(t, tt.canProcessEpoch, state.CanProcessEpoch(s), "CanProcessEpoch(%d)", tt.slot)
	}
}

func TestProcessBlock_OverMaxProposerSlashings(t *testing.T) {
	maxSlashings := params.BeaconConfig().MaxProposerSlashings
	b := &ethpb.SignedBeaconBlock{
		Block: &ethpb.BeaconBlock{
			Body: &ethpb.BeaconBlockBody{
				ProposerSlashings: make([]*ethpb.ProposerSlashing, maxSlashings+1),
			},
		},
	}
	want := fmt.Sprintf("number of proposer slashings (%d) in block body exceeds allowed threshold of %d",
		len(b.Block.Body.ProposerSlashings), params.BeaconConfig().MaxProposerSlashings)
	_, err := state.VerifyOperationLengths(context.Background(), &v1.BeaconState{}, wrapper.WrappedPhase0SignedBeaconBlock(b))
	assert.ErrorContains(t, want, err)
}

func TestProcessBlock_OverMaxAttesterSlashings(t *testing.T) {
	maxSlashings := params.BeaconConfig().MaxAttesterSlashings
	b := &ethpb.SignedBeaconBlock{
		Block: &ethpb.BeaconBlock{
			Body: &ethpb.BeaconBlockBody{
				AttesterSlashings: make([]*ethpb.AttesterSlashing, maxSlashings+1),
			},
		},
	}
	want := fmt.Sprintf("number of attester slashings (%d) in block body exceeds allowed threshold of %d",
		len(b.Block.Body.AttesterSlashings), params.BeaconConfig().MaxAttesterSlashings)
	_, err := state.VerifyOperationLengths(context.Background(), &v1.BeaconState{}, wrapper.WrappedPhase0SignedBeaconBlock(b))
	assert.ErrorContains(t, want, err)
}

func TestProcessBlock_OverMaxAttestations(t *testing.T) {
	b := &ethpb.SignedBeaconBlock{
		Block: &ethpb.BeaconBlock{
			Body: &ethpb.BeaconBlockBody{
				Attestations: make([]*ethpb.Attestation, params.BeaconConfig().MaxAttestations+1),
			},
		},
	}
	want := fmt.Sprintf("number of attestations (%d) in block body exceeds allowed threshold of %d",
		len(b.Block.Body.Attestations), params.BeaconConfig().MaxAttestations)
	_, err := state.VerifyOperationLengths(context.Background(), &v1.BeaconState{}, wrapper.WrappedPhase0SignedBeaconBlock(b))
	assert.ErrorContains(t, want, err)
}

func TestProcessBlock_OverMaxVoluntaryExits(t *testing.T) {
	maxExits := params.BeaconConfig().MaxVoluntaryExits
	b := &ethpb.SignedBeaconBlock{
		Block: &ethpb.BeaconBlock{
			Body: &ethpb.BeaconBlockBody{
				VoluntaryExits: make([]*ethpb.SignedVoluntaryExit, maxExits+1),
			},
		},
	}
	want := fmt.Sprintf("number of voluntary exits (%d) in block body exceeds allowed threshold of %d",
		len(b.Block.Body.VoluntaryExits), maxExits)
	_, err := state.VerifyOperationLengths(context.Background(), &v1.BeaconState{}, wrapper.WrappedPhase0SignedBeaconBlock(b))
	assert.ErrorContains(t, want, err)
}

func TestProcessBlock_IncorrectDeposits(t *testing.T) {
	base := &pb.BeaconState{
		Eth1Data:         &ethpb.Eth1Data{DepositCount: 100},
		Eth1DepositIndex: 98,
	}
	s, err := v1.InitializeFromProto(base)
	require.NoError(t, err)
	b := &ethpb.SignedBeaconBlock{
		Block: &ethpb.BeaconBlock{
			Body: &ethpb.BeaconBlockBody{
				Deposits: []*ethpb.Deposit{{}},
			},
		},
	}
	want := fmt.Sprintf("incorrect outstanding deposits in block body, wanted: %d, got: %d",
		s.Eth1Data().DepositCount-s.Eth1DepositIndex(), len(b.Block.Body.Deposits))
	_, err = state.VerifyOperationLengths(context.Background(), s, wrapper.WrappedPhase0SignedBeaconBlock(b))
	assert.ErrorContains(t, want, err)
}

func TestProcessSlots_SameSlotAsParentState(t *testing.T) {
	slot := types.Slot(2)
	parentState, err := v1.InitializeFromProto(&pb.BeaconState{Slot: slot})
	require.NoError(t, err)

	_, err = state.ProcessSlots(context.Background(), parentState, slot)
	assert.ErrorContains(t, "expected state.slot 2 < slot 2", err)
}

func TestProcessSlots_LowerSlotAsParentState(t *testing.T) {
	slot := types.Slot(2)
	parentState, err := v1.InitializeFromProto(&pb.BeaconState{Slot: slot})
	require.NoError(t, err)

	_, err = state.ProcessSlots(context.Background(), parentState, slot-1)
	assert.ErrorContains(t, "expected state.slot 2 < slot 1", err)
}

func TestProcessSlotsUsingNextSlotCache(t *testing.T) {
	ctx := context.Background()
	s, _ := testutil.DeterministicGenesisState(t, 1)
	r := []byte{'a'}
	s, err := state.ProcessSlotsUsingNextSlotCache(ctx, s, r, 5)
	require.NoError(t, err)
	require.Equal(t, types.Slot(5), s.Slot())
}
