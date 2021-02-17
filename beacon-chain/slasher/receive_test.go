package slasher

import (
	"context"
	"testing"

	types "github.com/prysmaticlabs/eth2-types"
	ethpb "github.com/prysmaticlabs/ethereumapis/eth/v1alpha1"
	dbtest "github.com/prysmaticlabs/prysm/beacon-chain/db/testing"
	slashertypes "github.com/prysmaticlabs/prysm/beacon-chain/slasher/types"
	"github.com/prysmaticlabs/prysm/shared/event"
	"github.com/prysmaticlabs/prysm/shared/testutil/assert"
	"github.com/prysmaticlabs/prysm/shared/testutil/require"
	logTest "github.com/sirupsen/logrus/hooks/test"
)

func Test_processQueuedAttestations_DetectsSurroundingVotes(t *testing.T) {
	hook := logTest.NewGlobal()
	beaconDB := dbtest.SetupDB(t)
	ctx, cancel := context.WithCancel(context.Background())
	s := &Service{
		serviceCfg: &ServiceConfig{
			Database: beaconDB,
		},
		params:           DefaultParams(),
		attestationQueue: make([]*slashertypes.CompactAttestation, 0),
	}
	currentEpochChan := make(chan types.Epoch)
	exitChan := make(chan struct{})
	go func() {
		s.processQueuedAttestations(ctx, currentEpochChan)
		exitChan <- struct{}{}
	}()
	s.attestationQueue = []*slashertypes.CompactAttestation{
		{
			AttestingIndices: []uint64{0, 1},
			Source:           1,
			Target:           2,
			SigningRoot:      [32]byte{1},
		},
		{
			AttestingIndices: []uint64{0, 1},
			Source:           0,
			Target:           3,
			SigningRoot:      [32]byte{1},
		},
	}
	currentEpoch := types.Epoch(4)
	currentEpochChan <- currentEpoch
	cancel()
	<-exitChan
	require.LogsContain(t, hook, "Slashable offenses found")
	require.LogsContain(t, hook, "Attester surrounding vote")
}

func Test_processQueuedAttestations_DetectsSurroundedVote(t *testing.T) {
	hook := logTest.NewGlobal()
	beaconDB := dbtest.SetupDB(t)
	ctx, cancel := context.WithCancel(context.Background())
	s := &Service{
		serviceCfg: &ServiceConfig{
			Database: beaconDB,
		},
		params:           DefaultParams(),
		attestationQueue: make([]*slashertypes.CompactAttestation, 0),
	}
	currentEpochChan := make(chan types.Epoch)
	exitChan := make(chan struct{})
	go func() {
		s.processQueuedAttestations(ctx, currentEpochChan)
		exitChan <- struct{}{}
	}()
	s.attestationQueue = []*slashertypes.CompactAttestation{
		{
			AttestingIndices: []uint64{0, 1},
			Source:           0,
			Target:           3,
			SigningRoot:      [32]byte{1},
		},
		{
			AttestingIndices: []uint64{0, 1},
			Source:           1,
			Target:           2,
			SigningRoot:      [32]byte{1},
		},
	}
	currentEpoch := types.Epoch(4)
	currentEpochChan <- currentEpoch
	cancel()
	<-exitChan
	require.LogsContain(t, hook, "Slashable offenses found")
	require.LogsContain(t, hook, "Attester surrounded vote")
}

func Test_processQueuedAttestations_NotSlashable(t *testing.T) {
	hook := logTest.NewGlobal()
	beaconDB := dbtest.SetupDB(t)
	ctx, cancel := context.WithCancel(context.Background())
	s := &Service{
		serviceCfg: &ServiceConfig{
			Database: beaconDB,
		},
		params:           DefaultParams(),
		attestationQueue: make([]*slashertypes.CompactAttestation, 0),
	}
	currentEpochChan := make(chan types.Epoch)
	exitChan := make(chan struct{})
	go func() {
		s.processQueuedAttestations(ctx, currentEpochChan)
		exitChan <- struct{}{}
	}()
	s.attestationQueue = []*slashertypes.CompactAttestation{
		{
			AttestingIndices: []uint64{0, 1},
			Source:           0,
			Target:           1,
			SigningRoot:      [32]byte{1},
		},
		{
			AttestingIndices: []uint64{0, 1},
			Source:           1,
			Target:           2,
			SigningRoot:      [32]byte{1},
		},
	}
	currentEpoch := types.Epoch(4)
	currentEpochChan <- currentEpoch
	cancel()
	<-exitChan
	require.LogsDoNotContain(t, hook, "Slashable offenses found")
}

func TestSlasher_receiveAttestations_OK(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	s := &Service{
		serviceCfg: &ServiceConfig{
			IndexedAttsFeed: new(event.Feed),
		},
		indexedAttsChan: make(chan *ethpb.IndexedAttestation),
	}
	exitChan := make(chan struct{})
	go func() {
		s.receiveAttestations(ctx)
		exitChan <- struct{}{}
	}()
	firstIndices := []uint64{1, 2, 3}
	secondIndices := []uint64{4, 5, 6}
	att1 := &ethpb.IndexedAttestation{
		AttestingIndices: firstIndices,
		Data: &ethpb.AttestationData{
			Source: &ethpb.Checkpoint{
				Epoch: 1,
			},
			Target: &ethpb.Checkpoint{
				Epoch: 2,
			},
		},
	}
	att2 := &ethpb.IndexedAttestation{
		AttestingIndices: secondIndices,
		Data: &ethpb.AttestationData{
			Source: &ethpb.Checkpoint{
				Epoch: 1,
			},
			Target: &ethpb.Checkpoint{
				Epoch: 2,
			},
		},
	}
	s.indexedAttsChan <- att1
	s.indexedAttsChan <- att2
	cancel()
	<-exitChan
	wanted := []*slashertypes.CompactAttestation{
		{
			AttestingIndices: att1.AttestingIndices,
			Source:           att1.Data.Source.Epoch,
			Target:           att1.Data.Target.Epoch,
		},
		{
			AttestingIndices: att2.AttestingIndices,
			Source:           att2.Data.Source.Epoch,
			Target:           att2.Data.Target.Epoch,
		},
	}
	require.DeepEqual(t, wanted, s.attestationQueue)
}

func TestSlasher_receiveAttestations_OnlyValidAttestations(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	s := &Service{
		serviceCfg: &ServiceConfig{
			IndexedAttsFeed: new(event.Feed),
		},
		indexedAttsChan: make(chan *ethpb.IndexedAttestation),
	}
	exitChan := make(chan struct{})
	go func() {
		s.receiveAttestations(ctx)
		exitChan <- struct{}{}
	}()
	firstIndices := []uint64{1, 2, 3}
	secondIndices := []uint64{4, 5, 6}
	// Add a valid attestation.
	validAtt := &ethpb.IndexedAttestation{
		AttestingIndices: firstIndices,
		Data: &ethpb.AttestationData{
			Source: &ethpb.Checkpoint{
				Epoch: 1,
			},
			Target: &ethpb.Checkpoint{
				Epoch: 2,
			},
		},
	}
	s.indexedAttsChan <- validAtt
	// Send an invalid, bad attestation which will not
	// pass integrity checks at it has invalid attestation data.
	s.indexedAttsChan <- &ethpb.IndexedAttestation{
		AttestingIndices: secondIndices,
	}
	cancel()
	<-exitChan
	// Expect only a single, valid attestation was added to the queue.
	require.Equal(t, 1, len(s.attestationQueue))
	wanted := []*slashertypes.CompactAttestation{
		{
			AttestingIndices: validAtt.AttestingIndices,
			Source:           validAtt.Data.Source.Epoch,
			Target:           validAtt.Data.Target.Epoch,
		},
	}
	require.DeepEqual(t, wanted, s.attestationQueue)
}

func TestSlasher_receiveBlocks_OK(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	s := &Service{
		serviceCfg: &ServiceConfig{
			BeaconBlocksFeed: new(event.Feed),
		},
		beaconBlocksChan: make(chan *ethpb.BeaconBlockHeader),
	}
	exitChan := make(chan struct{})
	go func() {
		s.receiveBlocks(ctx)
		exitChan <- struct{}{}
	}()
	block1 := &ethpb.BeaconBlockHeader{
		ProposerIndex: 1,
	}
	block2 := &ethpb.BeaconBlockHeader{
		ProposerIndex: 2,
	}
	s.beaconBlocksChan <- block1
	s.beaconBlocksChan <- block2
	cancel()
	<-exitChan
	wanted := []*slashertypes.CompactBeaconBlock{
		{
			ProposerIndex: block1.ProposerIndex,
		},
		{
			ProposerIndex: block2.ProposerIndex,
		},
	}
	require.DeepEqual(t, wanted, s.beaconBlocksQueue)
}

func TestService_processQueuedBlocks(t *testing.T) {
	hook := logTest.NewGlobal()
	beaconDB := dbtest.SetupDB(t)
	s := &Service{
		params: DefaultParams(),
		serviceCfg: &ServiceConfig{
			Database: beaconDB,
		},
		beaconBlocksQueue: []*slashertypes.CompactBeaconBlock{
			{
				ProposerIndex: 1,
			},
		},
	}
	ctx, cancel := context.WithCancel(context.Background())
	tickerChan := make(chan types.Epoch)
	exitChan := make(chan struct{})
	go func() {
		s.processQueuedBlocks(ctx, tickerChan)
		exitChan <- struct{}{}
	}()

	// Send a value over the ticker.
	tickerChan <- 0
	cancel()
	<-exitChan
	assert.LogsContain(t, hook, "Epoch reached, processing queued")
}
