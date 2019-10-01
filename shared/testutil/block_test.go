package testutil

import (
	"context"
	"testing"

	"github.com/prysmaticlabs/prysm/beacon-chain/core/state"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/state/stateutils"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/prysmaticlabs/prysm/shared/params"
)

func TestGenerateFullBlock_PassesStateTransition(t *testing.T) {
	deposits, privs := SetupInitialDeposits(t, 128)
	eth1Data := GenerateEth1Data(t, deposits)
	beaconState, err := state.GenesisBeaconState(deposits, 0, eth1Data)
	if err != nil {
		t.Fatal(err)
	}
	conf := &BlockGenConfig{
		MaxProposerSlashings: 0,
		MaxAttesterSlashings: 0,
		MaxAttestations:      0,
		MaxDeposits:          0,
		MaxVoluntaryExits:    0,
	}
	block := GenerateFullBlock(t, beaconState, privs, conf)
	beaconState, err = state.ExecuteStateTransition(context.Background(), beaconState, block)
	if err != nil {
		t.Fatal(err)
	}
}

func TestGenerateFullBlock_ValidProposerSlashings(t *testing.T) {
	deposits, privs := SetupInitialDeposits(t, 128)
	eth1Data := GenerateEth1Data(t, deposits)
	beaconState, err := state.GenesisBeaconState(deposits, 0, eth1Data)
	if err != nil {
		t.Fatal(err)
	}
	conf := &BlockGenConfig{
		MaxProposerSlashings: 1,
		MaxAttesterSlashings: 0,
		MaxAttestations:      0,
		MaxDeposits:          0,
		MaxVoluntaryExits:    0,
	}
	block := GenerateFullBlock(t, beaconState, privs, conf)
	beaconState, err = state.ExecuteStateTransition(context.Background(), beaconState, block)
	if err != nil {
		t.Fatal(err)
	}

	slashableIndice := block.Body.ProposerSlashings[0].ProposerIndex
	if !beaconState.Validators[slashableIndice].Slashed {
		t.Fatal("expected validator to be slashed")
	}
}

func TestGenerateFullBlock_ValidAttesterSlashings(t *testing.T) {
	deposits, privs := SetupInitialDeposits(t, 128)
	eth1Data := GenerateEth1Data(t, deposits)
	beaconState, err := state.GenesisBeaconState(deposits, 0, eth1Data)
	if err != nil {
		t.Fatal(err)
	}
	conf := &BlockGenConfig{
		MaxProposerSlashings: 0,
		MaxAttesterSlashings: 1,
		MaxAttestations:      0,
		MaxDeposits:          0,
		MaxVoluntaryExits:    0,
	}
	block := GenerateFullBlock(t, beaconState, privs, conf)
	beaconState, err = state.ExecuteStateTransition(context.Background(), beaconState, block)
	if err != nil {
		t.Fatal(err)
	}

	slashableIndices := block.Body.AttesterSlashings[0].Attestation_1.CustodyBit_0Indices
	if !beaconState.Validators[slashableIndices[0]].Slashed {
		t.Fatal("expected validator to be slashed")
	}
}

func TestGenerateFullBlock_ValidAttestations(t *testing.T) {
	deposits, privs := SetupInitialDeposits(t, 128)
	eth1Data := GenerateEth1Data(t, deposits)
	beaconState, err := state.GenesisBeaconState(deposits, 0, eth1Data)
	if err != nil {
		t.Fatal(err)
	}
	// Moving the slot forward one due to ATTESTATION_INCLUSION_DELAY.
	beaconState.Slot++
	conf := &BlockGenConfig{
		MaxProposerSlashings: 0,
		MaxAttesterSlashings: 0,
		MaxAttestations:      2,
		MaxDeposits:          0,
		MaxVoluntaryExits:    0,
	}
	block := GenerateFullBlock(t, beaconState, privs, conf)
	beaconState, err = state.ExecuteStateTransition(context.Background(), beaconState, block)
	if err != nil {
		t.Fatal(err)
	}
	if len(beaconState.CurrentEpochAttestations) != 2 {
		t.Fatal("expected 2 attestations to be saved to the beacon state")
	}
}

func TestGenerateFullBlock_ValidDeposits(t *testing.T) {
	deposits, privs := SetupInitialDeposits(t, 128)
	eth1Data := GenerateEth1Data(t, deposits)
	beaconState, err := state.GenesisBeaconState(deposits, 0, eth1Data)
	if err != nil {
		t.Fatal(err)
	}
	deposits, privs = SetupInitialDeposits(t, 129)
	eth1Data = GenerateEth1Data(t, deposits)
	beaconState.Eth1Data = eth1Data
	conf := &BlockGenConfig{
		MaxProposerSlashings: 0,
		MaxAttesterSlashings: 0,
		MaxAttestations:      0,
		MaxDeposits:          1,
		MaxVoluntaryExits:    0,
	}
	block := GenerateFullBlock(t, beaconState, privs, conf)
	beaconState, err = state.ExecuteStateTransition(context.Background(), beaconState, block)
	if err != nil {
		t.Fatal(err)
	}

	depositedPubkey := block.Body.Deposits[0].Data.PublicKey
	valIndexMap := stateutils.ValidatorIndexMap(beaconState)
	index := valIndexMap[bytesutil.ToBytes48(depositedPubkey)]
	if beaconState.Validators[index].EffectiveBalance != params.BeaconConfig().MaxEffectiveBalance {
		t.Fatalf(
			"expected validator balance to be max effective balance, received %d",
			beaconState.Validators[index].EffectiveBalance,
		)
	}
}

func TestGenerateFullBlock_ValidVoluntaryExits(t *testing.T) {
	deposits, privs := SetupInitialDeposits(t, 128)
	eth1Data := GenerateEth1Data(t, deposits)
	beaconState, err := state.GenesisBeaconState(deposits, 0, eth1Data)
	if err != nil {
		t.Fatal(err)
	}
	// Moving the state 2048 epochs forward due to PERSISTENT_COMMITTEE_PERIOD.
	beaconState.Slot = 3 + params.BeaconConfig().PersistentCommitteePeriod*params.BeaconConfig().SlotsPerEpoch
	conf := &BlockGenConfig{
		MaxProposerSlashings: 0,
		MaxAttesterSlashings: 0,
		MaxAttestations:      0,
		MaxDeposits:          0,
		MaxVoluntaryExits:    1,
	}
	block := GenerateFullBlock(t, beaconState, privs, conf)
	beaconState, err = state.ExecuteStateTransition(context.Background(), beaconState, block)
	if err != nil {
		t.Fatal(err)
	}

	exitedIndice := block.Body.VoluntaryExits[0].ValidatorIndex
	if beaconState.Validators[exitedIndice].ExitEpoch == params.BeaconConfig().FarFutureEpoch {
		t.Fatal("expected exiting validator index to be marked as exiting")
	}
}
