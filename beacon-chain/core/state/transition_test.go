package state_test

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"strconv"
	"strings"
	"testing"

	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/state"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	"github.com/prysmaticlabs/prysm/shared/bls"
	"github.com/prysmaticlabs/prysm/shared/featureconfig"
	"github.com/prysmaticlabs/prysm/shared/hashutil"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/ssz"
	"github.com/prysmaticlabs/prysm/shared/trieutil"
	"github.com/sirupsen/logrus"
)

func init() {
	featureconfig.InitFeatureConfig(&featureconfig.FeatureFlagConfig{
		EnableCrosslinks: true,
	})
}

func setupInitialDeposits(t *testing.T, numDeposits uint64) ([]*pb.Deposit, []*bls.SecretKey) {
	privKeys := make([]*bls.SecretKey, numDeposits)
	deposits := make([]*pb.Deposit, numDeposits)
	for i := 0; i < len(deposits); i++ {
		priv, err := bls.RandKey(rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		depositData := &pb.DepositData{
			Pubkey: priv.PublicKey().Marshal(),
			Amount: params.BeaconConfig().MaxDepositAmount,
		}

		deposits[i] = &pb.Deposit{Data: depositData}
		privKeys[i] = priv
	}
	return deposits, privKeys
}

func createRandaoReveal(t *testing.T, beaconState *pb.BeaconState, privKeys []*bls.SecretKey) []byte {
	// We fetch the proposer's index as that is whom the RANDAO will be verified against.
	proposerIdx, err := helpers.BeaconProposerIndex(beaconState)
	if err != nil {
		t.Fatal(err)
	}
	epoch := uint64(0)
	buf := make([]byte, 32)
	binary.LittleEndian.PutUint64(buf, epoch)
	domain := helpers.DomainVersion(beaconState, epoch, params.BeaconConfig().DomainRandao)
	// We make the previous validator's index sign the message instead of the proposer.
	epochSignature := privKeys[proposerIdx].Sign(buf, domain)
	return epochSignature.Marshal()
}

func TestExecuteStateTransition_IncorrectSlot(t *testing.T) {
	beaconState := &pb.BeaconState{
		Slot: 5,
	}
	block := &pb.BeaconBlock{
		Slot: 4,
	}
	want := "expected state.slot"
	if _, err := state.ExecuteStateTransition(context.Background(), beaconState, block, state.DefaultConfig()); !strings.Contains(err.Error(), want) {
		t.Errorf("Expected %s, received %v", want, err)
	}
}

func TestProcessBlock_IncorrectProposerSlashing(t *testing.T) {
	helpers.ClearAllCaches()

	deposits, privKeys := setupInitialDeposits(t, params.BeaconConfig().SlotsPerEpoch)
	beaconState, err := state.GenesisBeaconState(deposits, uint64(0), &pb.Eth1Data{})
	if err != nil {
		t.Fatal(err)
	}
	var slashings []*pb.ProposerSlashing
	for i := uint64(0); i < params.BeaconConfig().MaxProposerSlashings+1; i++ {
		slashings = append(slashings, &pb.ProposerSlashing{})
	}
	randaoReveal := createRandaoReveal(t, beaconState, privKeys)
	block := &pb.BeaconBlock{
		Slot: 0,
		Body: &pb.BeaconBlockBody{
			RandaoReveal:      randaoReveal,
			ProposerSlashings: slashings,
			Eth1Data: &pb.Eth1Data{
				DepositRoot: []byte{2},
				BlockRoot:   []byte{3},
			},
		},
	}
	want := "could not verify block proposer slashing"
	if _, err := state.ProcessBlock(context.Background(), beaconState, block, state.DefaultConfig()); !strings.Contains(err.Error(), want) {
		t.Errorf("Expected %s, received %v", want, err)
	}
}

func TestProcessBlock_IncorrectAttesterSlashing(t *testing.T) {
	deposits, privKeys := setupInitialDeposits(t, params.BeaconConfig().SlotsPerEpoch)
	beaconState, err := state.GenesisBeaconState(deposits, uint64(0), &pb.Eth1Data{})
	if err != nil {
		t.Fatal(err)
	}
	slashings := []*pb.ProposerSlashing{
		{
			ProposerIndex: 1,
			Header_1: &pb.BeaconBlockHeader{
				Slot:      1,
				Signature: []byte("A"),
			},
			Header_2: &pb.BeaconBlockHeader{
				Slot:      1,
				Signature: []byte("B"),
			},
		},
	}
	var attesterSlashings []*pb.AttesterSlashing
	for i := uint64(0); i < params.BeaconConfig().MaxAttesterSlashings+1; i++ {
		attesterSlashings = append(attesterSlashings, &pb.AttesterSlashing{})
	}
	randaoReveal := createRandaoReveal(t, beaconState, privKeys)
	block := &pb.BeaconBlock{
		Slot: 0,
		Body: &pb.BeaconBlockBody{
			RandaoReveal:      randaoReveal,
			ProposerSlashings: slashings,
			AttesterSlashings: attesterSlashings,
			Eth1Data: &pb.Eth1Data{
				DepositRoot: []byte{2},
				BlockRoot:   []byte{3},
			},
		},
	}
	want := "could not verify block attester slashing"
	if _, err := state.ProcessBlock(context.Background(), beaconState, block, state.DefaultConfig()); !strings.Contains(err.Error(), want) {
		t.Errorf("Expected %s, received %v", want, err)
	}
}

func TestProcessBlock_IncorrectProcessBlockAttestations(t *testing.T) {
	deposits, privKeys := setupInitialDeposits(t, params.BeaconConfig().SlotsPerEpoch)
	beaconState, err := state.GenesisBeaconState(deposits, uint64(0), &pb.Eth1Data{})
	if err != nil {
		t.Fatal(err)
	}
	beaconState.LatestSlashedBalances = make([]uint64, params.BeaconConfig().LatestSlashedExitLength)
	proposerSlashings := []*pb.ProposerSlashing{
		{
			ProposerIndex: 3,
			Header_1: &pb.BeaconBlockHeader{
				Slot:      1,
				Signature: []byte("A"),
			},
			Header_2: &pb.BeaconBlockHeader{
				Slot:      1,
				Signature: []byte("B"),
			},
		},
	}
	attesterSlashings := []*pb.AttesterSlashing{
		{
			Attestation_1: &pb.IndexedAttestation{
				Data: &pb.AttestationData{
					SourceEpoch: 0,
					TargetEpoch: 0,
					Crosslink: &pb.Crosslink{
						Shard: 4,
					},
				},
				CustodyBit_0Indices: []uint64{0, 1},
			},
			Attestation_2: &pb.IndexedAttestation{
				Data: &pb.AttestationData{
					SourceEpoch: 0,
					TargetEpoch: 0,
					Crosslink: &pb.Crosslink{
						Shard: 4,
					},
				},
				CustodyBit_0Indices: []uint64{0, 1},
			},
		},
	}

	var blockAttestations []*pb.Attestation
	for i := uint64(0); i < params.BeaconConfig().MaxAttestations+1; i++ {
		blockAttestations = append(blockAttestations, &pb.Attestation{})
	}
	randaoReveal := createRandaoReveal(t, beaconState, privKeys)
	block := &pb.BeaconBlock{
		Slot: 0,
		Body: &pb.BeaconBlockBody{
			RandaoReveal:      randaoReveal,
			ProposerSlashings: proposerSlashings,
			AttesterSlashings: attesterSlashings,
			Attestations:      blockAttestations,
			Eth1Data: &pb.Eth1Data{
				DepositRoot: []byte{2},
				BlockRoot:   []byte{3},
			},
		},
	}
	want := "could not process block attestations"
	if _, err := state.ProcessBlock(context.Background(), beaconState, block, state.DefaultConfig()); !strings.Contains(err.Error(), want) {
		t.Errorf("Expected %s, received %v", want, err)
	}
}

func TestProcessBlock_IncorrectProcessExits(t *testing.T) {
	helpers.ClearAllCaches()

	deposits := make([]*pb.Deposit, params.BeaconConfig().DepositsForChainStart/8)
	for i := 0; i < len(deposits); i++ {
		depositData := &pb.DepositData{
			Pubkey: []byte(strconv.Itoa(i)),
			Amount: params.BeaconConfig().MaxDepositAmount,
		}

		deposits[i] = &pb.Deposit{Data: depositData}
	}
	beaconState, err := state.GenesisBeaconState(deposits, uint64(0), &pb.Eth1Data{})
	if err != nil {
		t.Fatal(err)
	}
	beaconState.LatestSlashedBalances = make([]uint64, params.BeaconConfig().LatestSlashedExitLength)
	proposerSlashings := []*pb.ProposerSlashing{
		{
			ProposerIndex: 3,
			Header_1: &pb.BeaconBlockHeader{
				Slot:      1,
				Signature: []byte("A"),
			},
			Header_2: &pb.BeaconBlockHeader{
				Slot:      1,
				Signature: []byte("B"),
			},
		},
	}
	attesterSlashings := []*pb.AttesterSlashing{
		{
			Attestation_1: &pb.IndexedAttestation{
				Data: &pb.AttestationData{
					SourceEpoch: 0,
					TargetEpoch: 0,
					Crosslink: &pb.Crosslink{
						Shard: 4,
					}},
				CustodyBit_0Indices: []uint64{0, 1},
			},
			Attestation_2: &pb.IndexedAttestation{
				Data: &pb.AttestationData{
					SourceEpoch: 0,
					TargetEpoch: 0,
					Crosslink: &pb.Crosslink{
						Shard: 4,
					}},
				CustodyBit_0Indices: []uint64{0, 1},
			},
		},
	}
	var blockRoots [][]byte
	for i := uint64(0); i < params.BeaconConfig().SlotsPerHistoricalRoot; i++ {
		blockRoots = append(blockRoots, []byte{byte(i)})
	}
	beaconState.LatestBlockRoots = blockRoots
	beaconState.CurrentCrosslinks = []*pb.Crosslink{
		{
			DataRoot: []byte{1},
		},
	}
	blockAtt := &pb.Attestation{
		Data: &pb.AttestationData{
			TargetEpoch: 0,
			SourceEpoch: 0,
			SourceRoot:  []byte("tron-sucks"),
			Crosslink: &pb.Crosslink{
				Shard: 0,
				Epoch: 0,
			},
		},
		AggregationBitfield: []byte{0xC0, 0xC0, 0xC0, 0xC0},
		CustodyBitfield:     []byte{},
	}
	attestations := []*pb.Attestation{blockAtt}
	var exits []*pb.VoluntaryExit
	for i := uint64(0); i < params.BeaconConfig().MaxVoluntaryExits+1; i++ {
		exits = append(exits, &pb.VoluntaryExit{})
	}
	block := &pb.BeaconBlock{
		Slot: 4,
		Body: &pb.BeaconBlockBody{
			RandaoReveal:      []byte{},
			ProposerSlashings: proposerSlashings,
			AttesterSlashings: attesterSlashings,
			Attestations:      attestations,
			VoluntaryExits:    exits,
			Eth1Data: &pb.Eth1Data{
				DepositRoot: []byte{2},
				BlockRoot:   []byte{3},
			},
		},
	}
	beaconState.Slot += params.BeaconConfig().MinAttestationInclusionDelay
	beaconState.CurrentCrosslinks = []*pb.Crosslink{
		{
			Shard: 0,
			Epoch: 0,
		},
	}
	beaconState.CurrentJustifiedRoot = []byte("tron-sucks")
	beaconState.CurrentEpochAttestations = []*pb.PendingAttestation{}

	encoded, err := ssz.TreeHash(beaconState.CurrentCrosslinks[0])
	if err != nil {
		t.Fatal(err)
	}
	block.Body.Attestations[0].Data.Crosslink.ParentRoot = encoded[:]
	block.Body.Attestations[0].Data.Crosslink.DataRoot = params.BeaconConfig().ZeroHash[:]
	want := "could not process validator exits"
	if _, err := state.ProcessBlock(context.Background(), beaconState, block, state.DefaultConfig()); !strings.Contains(err.Error(), want) {
		t.Errorf("Expected %s, received %v", want, err)
	}
}

func TestProcessBlock_PassesProcessingConditions(t *testing.T) {
	deposits := make([]*pb.Deposit, params.BeaconConfig().DepositsForChainStart/8)
	for i := 0; i < len(deposits); i++ {
		depositData := &pb.DepositData{
			Pubkey: []byte(strconv.Itoa(i)),
			Amount: params.BeaconConfig().MaxDepositAmount,
		}

		deposits[i] = &pb.Deposit{Data: depositData}
	}
	beaconState, err := state.GenesisBeaconState(deposits, uint64(0), &pb.Eth1Data{})
	if err != nil {
		t.Fatal(err)
	}
	beaconState.LatestSlashedBalances = make([]uint64, params.BeaconConfig().LatestSlashedExitLength)
	proposerSlashings := []*pb.ProposerSlashing{
		{
			ProposerIndex: 3,
			Header_1: &pb.BeaconBlockHeader{
				Slot:      1,
				Signature: []byte("A"),
			},
			Header_2: &pb.BeaconBlockHeader{
				Slot:      1,
				Signature: []byte("B"),
			},
		},
	}
	attesterSlashings := []*pb.AttesterSlashing{
		{
			Attestation_1: &pb.IndexedAttestation{
				Data: &pb.AttestationData{
					SourceEpoch: 0,
					TargetEpoch: 0,
					Crosslink: &pb.Crosslink{
						Shard: 4,
					},
				},
				CustodyBit_0Indices: []uint64{0, 1},
			},
			Attestation_2: &pb.IndexedAttestation{
				Data: &pb.AttestationData{
					SourceEpoch: 0,
					TargetEpoch: 0,
					Crosslink: &pb.Crosslink{
						Shard: 4,
					},
				},
				CustodyBit_0Indices: []uint64{0, 1},
			},
		},
	}
	var blockRoots [][]byte
	for i := uint64(0); i < params.BeaconConfig().SlotsPerHistoricalRoot; i++ {
		blockRoots = append(blockRoots, []byte{byte(i)})
	}
	beaconState.LatestBlockRoots = blockRoots
	beaconState.CurrentCrosslinks = []*pb.Crosslink{
		{
			DataRoot: []byte{1},
		},
	}
	slotsPerEpoch := params.BeaconConfig().SlotsPerEpoch
	beaconState.Slot = (params.BeaconConfig().PersistentCommitteePeriod * slotsPerEpoch) + params.BeaconConfig().MinAttestationInclusionDelay
	blockAtt := &pb.Attestation{
		Data: &pb.AttestationData{
			TargetEpoch: helpers.SlotToEpoch(beaconState.Slot),
			SourceEpoch: 0,
			SourceRoot:  []byte("tron-sucks"),
			Crosslink: &pb.Crosslink{
				Shard: 0,
				Epoch: helpers.SlotToEpoch(beaconState.Slot),
			},
		},
		AggregationBitfield: []byte{0xC0, 0xC0, 0xC0, 0xC0},
		CustodyBitfield:     []byte{},
	}
	attestations := []*pb.Attestation{blockAtt}
	exits := []*pb.VoluntaryExit{
		{
			ValidatorIndex: 10,
			Epoch:          0,
		},
	}
	block := &pb.BeaconBlock{
		Slot: beaconState.Slot,
		Body: &pb.BeaconBlockBody{
			RandaoReveal:      []byte{},
			ProposerSlashings: proposerSlashings,
			AttesterSlashings: attesterSlashings,
			Attestations:      attestations,
			VoluntaryExits:    exits,
			Eth1Data: &pb.Eth1Data{
				DepositRoot: []byte{2},
				BlockRoot:   []byte{3},
			},
		},
	}
	beaconState.CurrentCrosslinks = []*pb.Crosslink{
		{
			Shard: 0,
			Epoch: helpers.SlotToEpoch(beaconState.Slot),
		},
	}
	beaconState.CurrentJustifiedRoot = []byte("tron-sucks")
	beaconState.CurrentEpochAttestations = []*pb.PendingAttestation{}

	encoded, err := ssz.TreeHash(beaconState.CurrentCrosslinks[0])
	if err != nil {
		t.Fatal(err)
	}
	block.Body.Attestations[0].Data.Crosslink.ParentRoot = encoded[:]
	block.Body.Attestations[0].Data.Crosslink.DataRoot = params.BeaconConfig().ZeroHash[:]
	if _, err := state.ProcessBlock(context.Background(), beaconState, block, state.DefaultConfig()); err != nil {
		t.Errorf("Expected block to pass processing conditions: %v", err)
	}
}

func TestProcessEpoch_CantGetTgtAttsPrevEpoch(t *testing.T) {
	atts := []*pb.PendingAttestation{{Data: &pb.AttestationData{TargetEpoch: 1}}}
	_, err := state.ProcessEpoch(context.Background(), &pb.BeaconState{CurrentEpochAttestations: atts})
	if !strings.Contains(err.Error(), "could not get target atts prev epoch") {
		t.Fatal("Did not receive wanted error")
	}
}

func TestProcessEpoch_CantGetTgtAttsCurrEpoch(t *testing.T) {
	epoch := uint64(1)

	atts := []*pb.PendingAttestation{{Data: &pb.AttestationData{Crosslink: &pb.Crosslink{Shard: 100}}}}
	_, err := state.ProcessEpoch(context.Background(), &pb.BeaconState{
		Slot:                     epoch * params.BeaconConfig().SlotsPerEpoch,
		LatestBlockRoots:         make([][]byte, 128),
		LatestRandaoMixes:        make([][]byte, params.BeaconConfig().LatestRandaoMixesLength),
		LatestActiveIndexRoots:   make([][]byte, params.BeaconConfig().LatestActiveIndexRootsLength),
		CurrentEpochAttestations: atts})
	if !strings.Contains(err.Error(), "could not get target atts current epoch") {
		t.Fatal("Did not receive wanted error")
	}
}

func TestProcessEpoch_CantGetAttsBalancePrevEpoch(t *testing.T) {
	helpers.ClearAllCaches()

	epoch := uint64(1)

	atts := []*pb.PendingAttestation{{Data: &pb.AttestationData{Crosslink: &pb.Crosslink{Shard: 961}}, AggregationBitfield: []byte{1}}}
	_, err := state.ProcessEpoch(context.Background(), &pb.BeaconState{
		Slot:                      epoch*params.BeaconConfig().SlotsPerEpoch + 1,
		LatestBlockRoots:          make([][]byte, 128),
		LatestRandaoMixes:         make([][]byte, params.BeaconConfig().LatestRandaoMixesLength),
		LatestActiveIndexRoots:    make([][]byte, params.BeaconConfig().LatestActiveIndexRootsLength),
		PreviousEpochAttestations: atts})
	if !strings.Contains(err.Error(), "could not get attesting balance prev epoch") {
		t.Fatal("Did not receive wanted error")
	}
}

func TestProcessEpoch_CantGetAttsBalanceCurrentEpoch(t *testing.T) {
	epoch := uint64(1)

	atts := []*pb.PendingAttestation{{Data: &pb.AttestationData{Crosslink: &pb.Crosslink{Shard: 961}}, AggregationBitfield: []byte{1}}}
	_, err := state.ProcessEpoch(context.Background(), &pb.BeaconState{
		Slot:                     epoch*params.BeaconConfig().SlotsPerEpoch + 1,
		LatestBlockRoots:         make([][]byte, 128),
		LatestRandaoMixes:        make([][]byte, params.BeaconConfig().LatestRandaoMixesLength),
		LatestActiveIndexRoots:   make([][]byte, params.BeaconConfig().LatestActiveIndexRootsLength),
		CurrentEpochAttestations: atts})
	if !strings.Contains(err.Error(), "could not get attesting balance current epoch") {
		t.Fatal("Did not receive wanted error")
	}
}

func TestProcessEpoch_CanProcess(t *testing.T) {
	epoch := uint64(1)

	atts := []*pb.PendingAttestation{{Data: &pb.AttestationData{Crosslink: &pb.Crosslink{Shard: 961}}}}
	var crosslinks []*pb.Crosslink
	for i := uint64(0); i < params.BeaconConfig().ShardCount; i++ {
		crosslinks = append(crosslinks, &pb.Crosslink{
			Epoch:    0,
			DataRoot: []byte{'A'},
		})
	}
	newState, err := state.ProcessEpoch(context.Background(), &pb.BeaconState{
		Slot:                     epoch*params.BeaconConfig().SlotsPerEpoch + 1,
		LatestBlockRoots:         make([][]byte, 128),
		LatestSlashedBalances:    []uint64{0, 1e9, 0},
		LatestRandaoMixes:        make([][]byte, params.BeaconConfig().LatestRandaoMixesLength),
		LatestActiveIndexRoots:   make([][]byte, params.BeaconConfig().LatestActiveIndexRootsLength),
		CurrentCrosslinks:        crosslinks,
		CurrentEpochAttestations: atts})
	if err != nil {
		t.Fatal(err)
	}

	wanted := uint64(1e9)
	if newState.LatestSlashedBalances[2] != wanted {
		t.Errorf("Wanted slashed balance: %d, got: %d", wanted, newState.Balances[2])
	}
}

func TestProcessEpoch_NotPanicOnEmptyActiveValidatorIndices(t *testing.T) {
	newState := &pb.BeaconState{
		LatestActiveIndexRoots: make([][]byte, params.BeaconConfig().LatestActiveIndexRootsLength),
		LatestSlashedBalances:  make([]uint64, params.BeaconConfig().LatestSlashedExitLength),
		LatestRandaoMixes:      make([][]byte, params.BeaconConfig().SlotsPerEpoch),
	}
	config := state.DefaultConfig()
	config.Logging = true

	state.ProcessEpoch(context.Background(), newState)
}

func BenchmarkProcessEpoch65536Validators(b *testing.B) {
	logrus.SetLevel(logrus.PanicLevel)

	helpers.ClearAllCaches()
	epoch := uint64(1)

	validatorCount := params.BeaconConfig().DepositsForChainStart * 4
	shardCount := validatorCount / params.BeaconConfig().TargetCommitteeSize
	validators := make([]*pb.Validator, validatorCount)
	balances := make([]uint64, validatorCount)

	for i := 0; i < len(validators); i++ {
		validators[i] = &pb.Validator{
			ExitEpoch:        params.BeaconConfig().FarFutureEpoch,
			EffectiveBalance: params.BeaconConfig().MaxDepositAmount,
		}
		balances[i] = params.BeaconConfig().MaxDepositAmount
	}

	var atts []*pb.PendingAttestation
	for i := uint64(0); i < shardCount; i++ {
		atts = append(atts, &pb.PendingAttestation{
			Data: &pb.AttestationData{
				Crosslink: &pb.Crosslink{
					Shard: i,
				},
			},
			AggregationBitfield: []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
				0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
			InclusionDelay: 1,
		})
	}

	var crosslinks []*pb.Crosslink
	for i := uint64(0); i < params.BeaconConfig().ShardCount; i++ {
		crosslinks = append(crosslinks, &pb.Crosslink{
			Epoch:    0,
			DataRoot: []byte{'A'},
		})
	}

	s := &pb.BeaconState{
		Slot:                      epoch*params.BeaconConfig().SlotsPerEpoch + 1,
		ValidatorRegistry:         validators,
		Balances:                  balances,
		LatestStartShard:          512,
		LatestBlockRoots:          make([][]byte, 254),
		LatestSlashedBalances:     []uint64{0, 1e9, 0},
		LatestRandaoMixes:         make([][]byte, params.BeaconConfig().LatestRandaoMixesLength),
		LatestActiveIndexRoots:    make([][]byte, params.BeaconConfig().LatestActiveIndexRootsLength),
		CurrentCrosslinks:         crosslinks,
		PreviousEpochAttestations: atts,
	}

	// Precache the shuffled indices
	for i := uint64(0); i < shardCount; i++ {
		if _, err := helpers.CrosslinkCommitteeAtEpoch(s, 0, i); err != nil {
			b.Fatal(err)
		}
	}

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		_, err := state.ProcessEpoch(context.Background(), s)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkProcessBlk_65536Validators_FullBlock(b *testing.B) {
	logrus.SetLevel(logrus.PanicLevel)
	helpers.ClearAllCaches()
	testConfig := params.BeaconConfig()
	testConfig.MaxTransfers = 1

	validatorCount := params.BeaconConfig().DepositsForChainStart * 4
	shardCount := validatorCount / params.BeaconConfig().TargetCommitteeSize
	validators := make([]*pb.Validator, validatorCount)
	for i := 0; i < len(validators); i++ {
		validators[i] = &pb.Validator{
			EffectiveBalance:           params.BeaconConfig().MaxDepositAmount,
			ExitEpoch:                  params.BeaconConfig().FarFutureEpoch,
			WithdrawableEpoch:          params.BeaconConfig().FarFutureEpoch,
			ActivationEligibilityEpoch: params.BeaconConfig().FarFutureEpoch,
		}
	}
	validatorBalances := make([]uint64, len(validators))
	for i := 0; i < len(validatorBalances); i++ {
		validatorBalances[i] = params.BeaconConfig().MaxDepositAmount
	}

	randaoMixes := make([][]byte, params.BeaconConfig().LatestRandaoMixesLength)
	for i := 0; i < len(randaoMixes); i++ {
		randaoMixes[i] = params.BeaconConfig().ZeroHash[:]
	}

	var crosslinks []*pb.Crosslink
	for i := uint64(0); i < params.BeaconConfig().ShardCount; i++ {
		crosslinks = append(crosslinks, &pb.Crosslink{
			Epoch:    0,
			DataRoot: []byte{'A'},
		})
	}

	s := &pb.BeaconState{
		Slot:                   20,
		LatestBlockRoots:       make([][]byte, 254),
		LatestRandaoMixes:      randaoMixes,
		ValidatorRegistry:      validators,
		Balances:               validatorBalances,
		LatestSlashedBalances:  make([]uint64, params.BeaconConfig().LatestSlashedExitLength),
		LatestActiveIndexRoots: make([][]byte, params.BeaconConfig().LatestActiveIndexRootsLength),
		CurrentJustifiedRoot:   []byte("tron-sucks"),
		Fork: &pb.Fork{
			PreviousVersion: []byte{0, 0, 0, 0},
			CurrentVersion:  []byte{0, 0, 0, 0},
		},
		CurrentCrosslinks: crosslinks,
	}

	c := &state.TransitionConfig{
		VerifySignatures: true,
		Logging:          false, // We enable logging in this state transition call.
	}

	// Set up proposer slashing object for block
	proposerSlashings := []*pb.ProposerSlashing{
		{
			ProposerIndex: 1,
			Header_1: &pb.BeaconBlockHeader{
				Slot:      0,
				Signature: []byte("A"),
			},
			Header_2: &pb.BeaconBlockHeader{
				Slot:      0,
				Signature: []byte("B"),
			},
		},
	}

	// Set up attester slashing object for block
	attesterSlashings := []*pb.AttesterSlashing{
		{
			Attestation_1: &pb.IndexedAttestation{
				Data: &pb.AttestationData{
					Crosslink: &pb.Crosslink{
						Shard: 5,
					},
				},
				CustodyBit_0Indices: []uint64{2, 3},
			},
			Attestation_2: &pb.IndexedAttestation{
				Data: &pb.AttestationData{
					Crosslink: &pb.Crosslink{
						Shard: 5,
					},
				},
				CustodyBit_0Indices: []uint64{2, 3},
			},
		},
	}

	// Set up deposit object for block
	deposit := &pb.Deposit{
		Data: &pb.DepositData{
			Pubkey: []byte{1, 2, 3},
			Amount: params.BeaconConfig().MaxDepositAmount,
		},
	}
	leaf, err := ssz.TreeHash(deposit.Data)
	if err != nil {
		b.Fatal(err)
	}
	depositTrie, err := trieutil.GenerateTrieFromItems([][]byte{leaf[:]}, int(params.BeaconConfig().DepositContractTreeDepth))
	if err != nil {
		b.Fatalf("Could not generate trie: %v", err)
	}
	proof, err := depositTrie.MerkleProof(0)
	if err != nil {
		b.Fatalf("Could not generate proof: %v", err)
	}
	deposit.Proof = proof
	deposit.Index = 0
	root := depositTrie.Root()

	// Set up randao reveal object for block
	proposerIdx, err := helpers.BeaconProposerIndex(s)
	if err != nil {
		b.Fatal(err)
	}
	priv, err := bls.RandKey(rand.Reader)
	if err != nil {
		b.Fatal(err)
	}
	s.ValidatorRegistry[proposerIdx].Pubkey = priv.PublicKey().Marshal()
	buf := make([]byte, 32)
	binary.LittleEndian.PutUint64(buf, 0)
	domain := helpers.DomainVersion(s, 0, params.BeaconConfig().DomainRandao)
	epochSignature := priv.Sign(buf, domain)

	// Set up transfer object for block
	transfers := []*pb.Transfer{
		{
			Slot:      s.Slot,
			Sender:    3,
			Recipient: 4,
			Fee:       params.BeaconConfig().MinDepositAmount,
			Amount:    params.BeaconConfig().MinDepositAmount,
			Pubkey:    []byte("A"),
		},
	}
	buf = []byte{params.BeaconConfig().BLSWithdrawalPrefixByte}
	pubKey := []byte("A")
	hashed := hashutil.Hash(pubKey)
	buf = append(buf, hashed[:]...)
	s.ValidatorRegistry[3].WithdrawalCredentials = buf

	// Set up attestations obj for block.
	encoded, err := ssz.TreeHash(s.CurrentCrosslinks[0])
	if err != nil {
		b.Fatal(err)
	}

	attestations := make([]*pb.Attestation, 128)
	for i := 0; i < len(attestations); i++ {
		attestations[i] = &pb.Attestation{
			Data: &pb.AttestationData{
				SourceRoot: []byte("tron-sucks"),
				Crosslink: &pb.Crosslink{
					Shard:      uint64(i),
					ParentRoot: encoded[:],
					DataRoot:   params.BeaconConfig().ZeroHash[:],
				},
			},
			AggregationBitfield: []byte{0xC0, 0xC0, 0xC0, 0xC0, 0xC0, 0xC0, 0xC0,
				0xC0, 0xC0, 0xC0, 0xC0, 0xC0, 0xC0, 0xC0, 0xC0, 0xC0},
			CustodyBitfield: []byte{},
		}
	}

	blk := &pb.BeaconBlock{
		Slot: s.Slot + 1,
		Body: &pb.BeaconBlockBody{
			Eth1Data: &pb.Eth1Data{
				DepositRoot: root[:],
				BlockRoot:   root[:],
			},
			RandaoReveal:      epochSignature.Marshal(),
			Attestations:      attestations,
			ProposerSlashings: proposerSlashings,
			AttesterSlashings: attesterSlashings,
			Transfers:         transfers,
		},
	}

	// Precache the shuffled indices
	for i := uint64(0); i < shardCount; i++ {
		if _, err := helpers.CrosslinkCommitteeAtEpoch(s, 0, i); err != nil {
			b.Fatal(err)
		}
	}

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		_, err := state.ProcessBlock(context.Background(), s, blk, c)
		if err != nil {
			b.Fatal(err)
		}
		// Reset state fields to process block again
		s.ValidatorRegistry[1].Slashed = false
		s.ValidatorRegistry[2].Slashed = false
		s.Balances[3] += 2 * params.BeaconConfig().MinDepositAmount
	}
}
