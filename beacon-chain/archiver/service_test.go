package archiver

import (
	"context"
	"fmt"
	"io/ioutil"
	"reflect"
	"testing"

	"github.com/gogo/protobuf/proto"
	ethpb "github.com/prysmaticlabs/ethereumapis/eth/v1alpha1"
	"github.com/prysmaticlabs/go-bitfield"
	mock "github.com/prysmaticlabs/prysm/beacon-chain/blockchain/testing"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/statefeed"
	"github.com/prysmaticlabs/prysm/beacon-chain/db"
	dbutil "github.com/prysmaticlabs/prysm/beacon-chain/db/testing"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/testutil"
	"github.com/sirupsen/logrus"
	logTest "github.com/sirupsen/logrus/hooks/test"
)

func init() {
	logrus.SetLevel(logrus.DebugLevel)
	logrus.SetOutput(ioutil.Discard)
	params.OverrideBeaconConfig(params.MinimalSpecConfig())
}

func TestArchiverService_ReceivesBlockProcessedEvent(t *testing.T) {
	hook := logTest.NewGlobal()
	svc, beaconDB := setupService(t)
	defer dbutil.TeardownDB(t, beaconDB)
	svc.headFetcher = &mock.ChainService{
		State: &pb.BeaconState{Slot: 1},
	}
	event := &statefeed.Event{
		Type: statefeed.BlockProcessed,
		Data: &statefeed.BlockProcessedData{
			BlockRoot: [32]byte{1, 2, 3},
			Verified:  true,
		},
	}
	triggerStateEvent(t, svc, event)
	testutil.AssertLogsContain(t, hook, fmt.Sprintf("%#x", event.Data.(*statefeed.BlockProcessedData).BlockRoot))
	testutil.AssertLogsContain(t, hook, "Received block processed event")
}

func TestArchiverService_OnlyArchiveAtEpochEnd(t *testing.T) {
	hook := logTest.NewGlobal()
	svc, beaconDB := setupService(t)
	defer dbutil.TeardownDB(t, beaconDB)
	// The head state is NOT an epoch end.
	svc.headFetcher = &mock.ChainService{
		State: &pb.BeaconState{Slot: params.BeaconConfig().SlotsPerEpoch - 2},
	}
	event := &statefeed.Event{
		Type: statefeed.BlockProcessed,
		Data: &statefeed.BlockProcessedData{
			BlockRoot: [32]byte{1, 2, 3},
			Verified:  true,
		},
	}
	triggerStateEvent(t, svc, event)

	// The context should have been canceled.
	if svc.ctx.Err() != context.Canceled {
		t.Error("context was not canceled")
	}
	testutil.AssertLogsContain(t, hook, "Received block processed event")
	// The service should ONLY log any archival logs if we receive a
	// head slot that is an epoch end.
	testutil.AssertLogsDoNotContain(t, hook, "Successfully archived")
}

func TestArchiverService_ArchivesEvenThroughSkipSlot(t *testing.T) {
	hook := logTest.NewGlobal()
	svc, beaconDB := setupService(t)
	validatorCount := uint64(100)
	headState := setupState(t, validatorCount)
	defer dbutil.TeardownDB(t, beaconDB)
	event := &statefeed.Event{
		Type: statefeed.BlockProcessed,
		Data: &statefeed.BlockProcessedData{
			BlockRoot: [32]byte{1, 2, 3},
			Verified:  true,
		},
	}

	exitRoutine := make(chan bool)
	go func() {
		svc.run(svc.ctx)
		<-exitRoutine
	}()

	// Send out an event every slot, skipping the end slot of the epoch.
	for i := uint64(0); i < params.BeaconConfig().SlotsPerEpoch+1; i++ {
		headState.Slot = i
		svc.headFetcher = &mock.ChainService{
			State: headState,
		}
		if helpers.IsEpochEnd(i) {
			continue
		}
		// Send in a loop to ensure it is delivered (busy wait for the service to subscribe to the state feed).
		for sent := 0; sent == 0; {
			sent = svc.stateNotifier.StateFeed().Send(event)
		}
	}
	if err := svc.Stop(); err != nil {
		t.Fatal(err)
	}
	exitRoutine <- true

	// The context should have been canceled.
	if svc.ctx.Err() != context.Canceled {
		t.Error("context was not canceled")
	}

	testutil.AssertLogsContain(t, hook, "Received block processed event")
	// Even though there was a skip slot, we should still be able to archive
	// upon the next block event afterwards.
	testutil.AssertLogsContain(t, hook, "Successfully archived")
}

func TestArchiverService_ComputesAndSavesParticipation(t *testing.T) {
	hook := logTest.NewGlobal()
	validatorCount := uint64(100)
	headState := setupState(t, validatorCount)
	svc, beaconDB := setupService(t)
	defer dbutil.TeardownDB(t, beaconDB)
	svc.headFetcher = &mock.ChainService{
		State: headState,
	}
	event := &statefeed.Event{
		Type: statefeed.BlockProcessed,
		Data: &statefeed.BlockProcessedData{
			BlockRoot: [32]byte{1, 2, 3},
			Verified:  true,
		},
	}
	triggerStateEvent(t, svc, event)

	attestedBalance := uint64(1)
	currentEpoch := helpers.CurrentEpoch(headState)
	wanted := &ethpb.ValidatorParticipation{
		VotedEther:              attestedBalance,
		EligibleEther:           validatorCount * params.BeaconConfig().MaxEffectiveBalance,
		GlobalParticipationRate: float32(attestedBalance) / float32(validatorCount*params.BeaconConfig().MaxEffectiveBalance),
	}

	retrieved, err := svc.beaconDB.ArchivedValidatorParticipation(svc.ctx, currentEpoch)
	if err != nil {
		t.Fatal(err)
	}

	if !proto.Equal(wanted, retrieved) {
		t.Errorf("Wanted participation for epoch %d %v, retrieved %v", currentEpoch-1, wanted, retrieved)
	}
	testutil.AssertLogsContain(t, hook, "Successfully archived")
}

func TestArchiverService_SavesIndicesAndBalances(t *testing.T) {
	hook := logTest.NewGlobal()
	validatorCount := uint64(100)
	headState := setupState(t, validatorCount)
	svc, beaconDB := setupService(t)
	defer dbutil.TeardownDB(t, beaconDB)
	svc.headFetcher = &mock.ChainService{
		State: headState,
	}
	event := &statefeed.Event{
		Type: statefeed.BlockProcessed,
		Data: &statefeed.BlockProcessedData{
			BlockRoot: [32]byte{1, 2, 3},
			Verified:  true,
		},
	}
	triggerStateEvent(t, svc, event)

	retrieved, err := svc.beaconDB.ArchivedBalances(svc.ctx, helpers.CurrentEpoch(headState))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(headState.Balances, retrieved) {
		t.Errorf(
			"Wanted balances for epoch %d %v, retrieved %v",
			helpers.CurrentEpoch(headState),
			headState.Balances,
			retrieved,
		)
	}
	testutil.AssertLogsContain(t, hook, "Successfully archived")
}

func TestArchiverService_SavesCommitteeInfo(t *testing.T) {
	hook := logTest.NewGlobal()
	validatorCount := uint64(100)
	headState := setupState(t, validatorCount)
	svc, beaconDB := setupService(t)
	defer dbutil.TeardownDB(t, beaconDB)
	svc.headFetcher = &mock.ChainService{
		State: headState,
	}
	event := &statefeed.Event{
		Type: statefeed.BlockProcessed,
		Data: &statefeed.BlockProcessedData{
			BlockRoot: [32]byte{1, 2, 3},
			Verified:  true,
		},
	}
	triggerStateEvent(t, svc, event)

	currentEpoch := helpers.CurrentEpoch(headState)
	proposerSeed, err := helpers.Seed(headState, currentEpoch, params.BeaconConfig().DomainBeaconProposer)
	if err != nil {
		t.Fatal(err)
	}
	attesterSeed, err := helpers.Seed(headState, currentEpoch, params.BeaconConfig().DomainBeaconAttester)
	if err != nil {
		t.Fatal(err)
	}
	wanted := &pb.ArchivedCommitteeInfo{
		ProposerSeed: proposerSeed[:],
		AttesterSeed: attesterSeed[:],
	}

	retrieved, err := svc.beaconDB.ArchivedCommitteeInfo(svc.ctx, helpers.CurrentEpoch(headState))
	if err != nil {
		t.Fatal(err)
	}
	if !proto.Equal(wanted, retrieved) {
		t.Errorf(
			"Wanted committee info for epoch %d %v, retrieved %v",
			helpers.CurrentEpoch(headState),
			wanted,
			retrieved,
		)
	}
	testutil.AssertLogsContain(t, hook, "Successfully archived")
}

func TestArchiverService_SavesActivatedValidatorChanges(t *testing.T) {
	hook := logTest.NewGlobal()
	validatorCount := uint64(100)
	headState := setupState(t, validatorCount)
	svc, beaconDB := setupService(t)
	defer dbutil.TeardownDB(t, beaconDB)
	svc.headFetcher = &mock.ChainService{
		State: headState,
	}
	prevEpoch := helpers.PrevEpoch(headState)
	delayedActEpoch := helpers.DelayedActivationExitEpoch(prevEpoch)
	headState.Validators[4].ActivationEpoch = delayedActEpoch
	headState.Validators[5].ActivationEpoch = delayedActEpoch
	event := &statefeed.Event{
		Type: statefeed.BlockProcessed,
		Data: &statefeed.BlockProcessedData{
			BlockRoot: [32]byte{1, 2, 3},
			Verified:  true,
		},
	}
	triggerStateEvent(t, svc, event)

	retrieved, err := beaconDB.ArchivedActiveValidatorChanges(svc.ctx, prevEpoch)
	if err != nil {
		t.Fatal(err)
	}
	if retrieved == nil {
		t.Fatal("Retrieved indices are nil")
	}
	if !reflect.DeepEqual(retrieved.Activated, []uint64{4, 5}) {
		t.Errorf("Wanted indices 4 5 activated, received %v", retrieved.Activated)
	}
	testutil.AssertLogsContain(t, hook, "Successfully archived")
}

func TestArchiverService_SavesSlashedValidatorChanges(t *testing.T) {
	hook := logTest.NewGlobal()
	validatorCount := uint64(100)
	headState := setupState(t, validatorCount)
	svc, beaconDB := setupService(t)
	defer dbutil.TeardownDB(t, beaconDB)
	svc.headFetcher = &mock.ChainService{
		State: headState,
	}
	prevEpoch := helpers.PrevEpoch(headState)
	headState.Validators[95].Slashed = true
	headState.Validators[96].Slashed = true
	event := &statefeed.Event{
		Type: statefeed.BlockProcessed,
		Data: &statefeed.BlockProcessedData{
			BlockRoot: [32]byte{1, 2, 3},
			Verified:  true,
		},
	}
	triggerStateEvent(t, svc, event)

	retrieved, err := beaconDB.ArchivedActiveValidatorChanges(svc.ctx, prevEpoch)
	if err != nil {
		t.Fatal(err)
	}
	if retrieved == nil {
		t.Fatal("Retrieved indices are nil")
	}
	if !reflect.DeepEqual(retrieved.Slashed, []uint64{95, 96}) {
		t.Errorf("Wanted indices 95, 96 slashed, received %v", retrieved.Slashed)
	}
	testutil.AssertLogsContain(t, hook, "Successfully archived")
}

func TestArchiverService_SavesExitedValidatorChanges(t *testing.T) {
	hook := logTest.NewGlobal()
	validatorCount := uint64(100)
	headState := setupState(t, validatorCount)
	svc, beaconDB := setupService(t)
	defer dbutil.TeardownDB(t, beaconDB)
	svc.headFetcher = &mock.ChainService{
		State: headState,
	}
	prevEpoch := helpers.PrevEpoch(headState)
	headState.Validators[95].ExitEpoch = prevEpoch
	headState.Validators[95].WithdrawableEpoch = prevEpoch + params.BeaconConfig().MinValidatorWithdrawabilityDelay
	event := &statefeed.Event{
		Type: statefeed.BlockProcessed,
		Data: &statefeed.BlockProcessedData{
			BlockRoot: [32]byte{1, 2, 3},
			Verified:  true,
		},
	}
	triggerStateEvent(t, svc, event)
	testutil.AssertLogsContain(t, hook, "Successfully archived")
	retrieved, err := beaconDB.ArchivedActiveValidatorChanges(svc.ctx, prevEpoch)
	if err != nil {
		t.Fatal(err)
	}
	if retrieved == nil {
		t.Fatal("Retrieved indices are nil")
	}
	if !reflect.DeepEqual(retrieved.Exited, []uint64{95}) {
		t.Errorf("Wanted indices 95 exited, received %v", retrieved.Exited)
	}
}

func setupState(t *testing.T, validatorCount uint64) *pb.BeaconState {
	validators := make([]*ethpb.Validator, validatorCount)
	balances := make([]uint64, validatorCount)
	for i := 0; i < len(validators); i++ {
		validators[i] = &ethpb.Validator{
			ExitEpoch:         params.BeaconConfig().FarFutureEpoch,
			WithdrawableEpoch: params.BeaconConfig().FarFutureEpoch,
			EffectiveBalance:  params.BeaconConfig().MaxEffectiveBalance,
		}
		balances[i] = params.BeaconConfig().MaxEffectiveBalance
	}

	atts := []*pb.PendingAttestation{{Data: &ethpb.AttestationData{Target: &ethpb.Checkpoint{}}}}

	// We initialize a head state that has attestations from participated
	// validators in a simulated fashion.
	return &pb.BeaconState{
		Slot:                       (2 * params.BeaconConfig().SlotsPerEpoch) - 1,
		Validators:                 validators,
		Balances:                   balances,
		BlockRoots:                 make([][]byte, 128),
		Slashings:                  []uint64{0, 1e9, 1e9},
		RandaoMixes:                make([][]byte, params.BeaconConfig().EpochsPerHistoricalVector),
		CurrentEpochAttestations:   atts,
		FinalizedCheckpoint:        &ethpb.Checkpoint{},
		JustificationBits:          bitfield.Bitvector4{0x00},
		CurrentJustifiedCheckpoint: &ethpb.Checkpoint{},
	}
}

func setupService(t *testing.T) (*Service, db.Database) {
	beaconDB := dbutil.SetupDB(t)
	ctx, cancel := context.WithCancel(context.Background())
	mockChainService := &mock.ChainService{}
	return &Service{
		beaconDB:      beaconDB,
		ctx:           ctx,
		cancel:        cancel,
		stateNotifier: mockChainService.StateNotifier(),
	}, beaconDB
}

func triggerStateEvent(t *testing.T, svc *Service, event *statefeed.Event) {
	exitRoutine := make(chan bool)
	go func() {
		svc.run(svc.ctx)
		<-exitRoutine
	}()

	// Send in a loop to ensure it is delivered (busy wait for the service to subscribe to the state feed).
	for sent := 0; sent == 0; {
		sent = svc.stateNotifier.StateFeed().Send(event)
	}
	if err := svc.Stop(); err != nil {
		t.Fatal(err)
	}
	exitRoutine <- true

	// The context should have been canceled.
	if svc.ctx.Err() != context.Canceled {
		t.Error("context was not canceled")
	}
}
