package client

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/golang/mock/gomock"
	ethpb "github.com/prysmaticlabs/ethereumapis/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/prysmaticlabs/prysm/shared/featureconfig"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/testutil/require"
	"github.com/prysmaticlabs/prysm/validator/db/kv"
	mockSlasher "github.com/prysmaticlabs/prysm/validator/testing"
)

func TestPreSignatureValidation(t *testing.T) {
	config := &featureconfig.Flags{
		SlasherProtection: true,
	}
	reset := featureconfig.InitWithReset(config)
	defer reset()
	validator, m, validatorKey, finish := setup(t)
	defer finish()
	pubKey := [48]byte{}
	copy(pubKey[:], validatorKey.PublicKey().Marshal())
	att := &ethpb.IndexedAttestation{
		AttestingIndices: []uint64{1, 2},
		Data: &ethpb.AttestationData{
			Slot:            5,
			CommitteeIndex:  2,
			BeaconBlockRoot: bytesutil.PadTo([]byte("great block"), 32),
			Source: &ethpb.Checkpoint{
				Epoch: 4,
				Root:  bytesutil.PadTo([]byte("good source"), 32),
			},
			Target: &ethpb.Checkpoint{
				Epoch: 10,
				Root:  bytesutil.PadTo([]byte("good target"), 32),
			},
		},
	}
	mockProtector := &mockSlasher.MockProtector{AllowAttestation: false}
	validator.protector = mockProtector
	m.validatorClient.EXPECT().DomainData(
		gomock.Any(), // ctx
		gomock.Any(), // epoch
	).Times(2).Return(&ethpb.DomainResponse{SignatureDomain: make([]byte, 32)}, nil /*err*/)
	err := validator.preAttSignValidations(context.Background(), att, pubKey)
	require.ErrorContains(t, failedPreAttSignExternalErr, err)
	mockProtector.AllowAttestation = true
	err = validator.preAttSignValidations(context.Background(), att, pubKey)
	require.NoError(t, err, "Expected allowed attestation not to throw error")
}

func TestPreSignatureValidation_NilLocal(t *testing.T) {
	config := &featureconfig.Flags{
		SlasherProtection: false,
	}
	reset := featureconfig.InitWithReset(config)
	defer reset()
	validator, m, _, finish := setup(t)
	defer finish()
	att := &ethpb.IndexedAttestation{
		AttestingIndices: []uint64{1, 2},
		Data: &ethpb.AttestationData{
			Slot:            5,
			CommitteeIndex:  2,
			BeaconBlockRoot: bytesutil.PadTo([]byte("great block"), 32),
			Source: &ethpb.Checkpoint{
				Epoch: 4,
				Root:  bytesutil.PadTo([]byte("good source"), 32),
			},
			Target: &ethpb.Checkpoint{
				Epoch: 10,
				Root:  bytesutil.PadTo([]byte("good target"), 32),
			},
		},
	}
	fakePubkey := bytesutil.ToBytes48([]byte("test"))
	m.validatorClient.EXPECT().DomainData(
		gomock.Any(), // ctx
		gomock.Any(), // epoch
	).Return(&ethpb.DomainResponse{SignatureDomain: make([]byte, 32)}, nil /*err*/)
	err := validator.preAttSignValidations(context.Background(), att, fakePubkey)
	require.NoError(t, err, "Expected allowed attestation not to throw error")
}

func TestPostSignatureUpdate(t *testing.T) {
	config := &featureconfig.Flags{
		SlasherProtection: true,
	}
	reset := featureconfig.InitWithReset(config)
	defer reset()
	validator, m, validatorKey, finish := setup(t)
	defer finish()
	ctx := context.Background()
	pubKey := [48]byte{}
	copy(pubKey[:], validatorKey.PublicKey().Marshal())
	att := &ethpb.IndexedAttestation{
		AttestingIndices: []uint64{1, 2},
		Data: &ethpb.AttestationData{
			Slot:            5,
			CommitteeIndex:  2,
			BeaconBlockRoot: bytesutil.PadTo([]byte("great block"), 32),
			Source: &ethpb.Checkpoint{
				Epoch: 4,
				Root:  bytesutil.PadTo([]byte("good source"), 32),
			},
			Target: &ethpb.Checkpoint{
				Epoch: 10,
				Root:  bytesutil.PadTo([]byte("good target"), 32),
			},
		},
	}
	mockProtector := &mockSlasher.MockProtector{AllowAttestation: false}
	validator.protector = mockProtector
	m.validatorClient.EXPECT().DomainData(
		gomock.Any(), // ctx
		gomock.Any(), // epoch2
	).Return(&ethpb.DomainResponse{SignatureDomain: make([]byte, 32)}, nil /*err*/)
	_, sr, err := validator.getDomainAndSigningRoot(ctx, att.Data)
	require.NoError(t, err)
	err = validator.postAttSignUpdate(context.Background(), att, pubKey, sr)
	require.ErrorContains(t, failedPostAttSignExternalErr, err, "Expected error on post signature update is detected as slashable")
	mockProtector.AllowAttestation = true
	err = validator.postAttSignUpdate(context.Background(), att, pubKey, sr)
	require.NoError(t, err, "Expected allowed attestation not to throw error")
}

func TestPostSignatureUpdate_NilLocal(t *testing.T) {
	config := &featureconfig.Flags{
		SlasherProtection: false,
	}
	reset := featureconfig.InitWithReset(config)
	defer reset()
	ctx := context.Background()
	validator, _, _, finish := setup(t)
	defer finish()
	att := &ethpb.IndexedAttestation{
		AttestingIndices: []uint64{1, 2},
		Data: &ethpb.AttestationData{
			Slot:            5,
			CommitteeIndex:  2,
			BeaconBlockRoot: []byte("great block"),
			Source: &ethpb.Checkpoint{
				Epoch: 4,
				Root:  []byte("good source"),
			},
			Target: &ethpb.Checkpoint{
				Epoch: 10,
				Root:  []byte("good target"),
			},
		},
	}
	sr := [32]byte{1}
	fakePubkey := bytesutil.ToBytes48([]byte("test"))
	err := validator.postAttSignUpdate(ctx, att, fakePubkey, sr)
	require.NoError(t, err, "Expected allowed attestation not to throw error")
}

func TestAttestationHistory_BlocksDoubleAttestation(t *testing.T) {
	ctx := context.Background()
	history := kv.NewAttestationHistoryArray(3)
	// Mark an attestation spanning epochs 0 to 3.
	newAttSource := uint64(0)
	newAttTarget := uint64(3)
	sr1 := [32]byte{1}
	history = markAttestationForTargetEpoch(ctx, history, newAttSource, newAttTarget, sr1)
	lew, err := history.GetLatestEpochWritten(ctx)
	require.NoError(t, err)
	require.Equal(t, newAttTarget, lew, "Unexpected latest epoch written")

	// Try an attestation that should be slashable (double att) spanning epochs 1 to 3.
	sr2 := [32]byte{2}
	newAttSource = uint64(1)
	newAttTarget = uint64(3)
	if !isNewAttSlashable(ctx, history, newAttSource, newAttTarget, sr2) {
		t.Fatalf("Expected attestation of source %d and target %d to be considered slashable", newAttSource, newAttTarget)
	}
}

func TestAttestationHistory_BlocksSurroundAttestationPostSignature(t *testing.T) {
	ctx := context.Background()
	att := &ethpb.IndexedAttestation{
		AttestingIndices: []uint64{1, 2},
		Data: &ethpb.AttestationData{
			Slot:            5,
			CommitteeIndex:  2,
			BeaconBlockRoot: []byte("great block"),
			Source: &ethpb.Checkpoint{
				Root: []byte("good source"),
			},
			Target: &ethpb.Checkpoint{
				Root: []byte("good target"),
			},
		},
	}

	v, _, validatorKey, finish := setup(t)
	defer finish()
	pubKey := [48]byte{}
	copy(pubKey[:], validatorKey.PublicKey().Marshal())
	passThrough := 0
	slashable := 0
	var wg sync.WaitGroup
	for i := uint64(0); i < 100; i++ {

		wg.Add(1)
		//Test surround and surrounded attestations.
		go func(i uint64) {
			sr := [32]byte{1}
			att.Data.Source.Epoch = 110 - i
			att.Data.Target.Epoch = 111 + i
			err := v.postAttSignUpdate(ctx, att, pubKey, sr)
			if err == nil {
				passThrough++
			} else {
				if strings.Contains(err.Error(), failedAttLocalProtectionErr) {
					slashable++
				}
				t.Logf("attestation source epoch %d", att.Data.Source.Epoch)
				t.Logf("attestation target epoch %d", att.Data.Target.Epoch)
			}
			wg.Done()
		}(i)
	}
	wg.Wait()
	require.Equal(t, 1, passThrough, "Expecting only one attestations to go through and all others to be found to be slashable")
	require.Equal(t, 99, slashable, "Expecting 99 attestations to be found as slashable")
}

func TestAttestationHistory_BlocksDoubleAttestationPostSignature(t *testing.T) {
	ctx := context.Background()
	att := &ethpb.IndexedAttestation{
		AttestingIndices: []uint64{1, 2},
		Data: &ethpb.AttestationData{
			Slot:            5,
			CommitteeIndex:  2,
			BeaconBlockRoot: []byte("great block"),
			Source: &ethpb.Checkpoint{
				Root: []byte("good source"),
			},
			Target: &ethpb.Checkpoint{
				Root: []byte("good target"),
			},
		},
	}

	v, _, validatorKey, finish := setup(t)
	defer finish()
	pubKey := [48]byte{}
	copy(pubKey[:], validatorKey.PublicKey().Marshal())
	passThrough := 0
	slashable := 0
	var wg sync.WaitGroup
	for i := uint64(0); i < 100; i++ {

		wg.Add(1)
		//Test double attestations.
		go func(i uint64) {
			sr := [32]byte{byte(i)}
			att.Data.Source.Epoch = 110 - i
			att.Data.Target.Epoch = 111
			err := v.postAttSignUpdate(ctx, att, pubKey, sr)
			if err == nil {
				passThrough++
			} else {
				if strings.Contains(err.Error(), failedAttLocalProtectionErr) {
					slashable++
				}
				t.Logf("attestation source epoch %d", att.Data.Source.Epoch)
				t.Logf("signing root %d", att.Data.Target.Epoch)
			}
			wg.Done()
		}(i)
	}
	wg.Wait()
	require.Equal(t, 1, passThrough, "Expecting only one attestations to go through and all others to be found to be slashable")
	require.Equal(t, 99, slashable, "Expecting 99 attestations to be found as slashable")

}

func TestAttestationHistory_Prunes(t *testing.T) {
	ctx := context.Background()
	wsPeriod := params.BeaconConfig().WeakSubjectivityPeriod

	signingRoot := [32]byte{1}
	signingRoot2 := [32]byte{2}
	signingRoot3 := [32]byte{3}
	signingRoot4 := [32]byte{4}
	history := kv.NewAttestationHistoryArray(0)

	// Try an attestation on totally unmarked history, should not be slashable.
	require.Equal(t, false, isNewAttSlashable(ctx, history, 0, wsPeriod+5, signingRoot), "Should not be slashable")

	// Mark attestations spanning epochs 0 to 3 and 6 to 9.
	prunedNewAttSource := uint64(0)
	prunedNewAttTarget := uint64(3)
	history = markAttestationForTargetEpoch(ctx, history, prunedNewAttSource, prunedNewAttTarget, signingRoot)
	newAttSource := prunedNewAttSource + 6
	newAttTarget := prunedNewAttTarget + 6
	history = markAttestationForTargetEpoch(ctx, history, newAttSource, newAttTarget, signingRoot2)
	lte, err := history.GetLatestEpochWritten(ctx)
	require.NoError(t, err)
	require.Equal(t, newAttTarget, lte, "Unexpected latest epoch")

	// Mark an attestation spanning epochs 54000 to 54003.
	farNewAttSource := newAttSource + wsPeriod
	farNewAttTarget := newAttTarget + wsPeriod
	history = markAttestationForTargetEpoch(ctx, history, farNewAttSource, farNewAttTarget, signingRoot3)
	lte, err = history.GetLatestEpochWritten(ctx)
	require.NoError(t, err)
	require.Equal(t, farNewAttTarget, lte, "Unexpected latest epoch")

	require.Equal(t, (*kv.HistoryData)(nil), safeTargetToSource(ctx, history, prunedNewAttTarget), "Unexpectedly marked attestation")
	require.Equal(t, farNewAttSource, safeTargetToSource(ctx, history, farNewAttTarget).Source, "Unexpectedly marked attestation")

	// Try an attestation from existing source to outside prune, should slash.
	if !isNewAttSlashable(ctx, history, newAttSource, farNewAttTarget, signingRoot4) {
		t.Fatalf("Expected attestation of source %d, target %d to be considered slashable", newAttSource, farNewAttTarget)
	}
	// Try an attestation from before existing target to outside prune, should slash.
	if !isNewAttSlashable(ctx, history, newAttTarget-1, farNewAttTarget, signingRoot4) {
		t.Fatalf("Expected attestation of source %d, target %d to be considered slashable", newAttTarget-1, farNewAttTarget)
	}
	// Try an attestation larger than pruning amount, should slash.
	if !isNewAttSlashable(ctx, history, 0, farNewAttTarget+5, signingRoot4) {
		t.Fatalf("Expected attestation of source 0, target %d to be considered slashable", farNewAttTarget+5)
	}
}

func TestAttestationHistory_BlocksSurroundedAttestation(t *testing.T) {
	ctx := context.Background()
	history := kv.NewAttestationHistoryArray(0)

	// Mark an attestation spanning epochs 0 to 3.
	signingRoot := [32]byte{1}
	newAttSource := uint64(0)
	newAttTarget := uint64(3)
	history = markAttestationForTargetEpoch(ctx, history, newAttSource, newAttTarget, signingRoot)
	lte, err := history.GetLatestEpochWritten(ctx)
	require.NoError(t, err)
	require.Equal(t, newAttTarget, lte)

	// Try an attestation that should be slashable (being surrounded) spanning epochs 1 to 2.
	newAttSource = uint64(1)
	newAttTarget = uint64(2)
	require.Equal(t, true, isNewAttSlashable(ctx, history, newAttSource, newAttTarget, signingRoot), "Expected slashable attestation")
}

func TestAttestationHistory_BlocksSurroundingAttestation(t *testing.T) {
	ctx := context.Background()
	history := kv.NewAttestationHistoryArray(0)
	signingRoot := [32]byte{1}

	// Mark an attestation spanning epochs 1 to 2.
	newAttSource := uint64(1)
	newAttTarget := uint64(2)
	history = markAttestationForTargetEpoch(ctx, history, newAttSource, newAttTarget, signingRoot)
	lte, err := history.GetLatestEpochWritten(ctx)
	require.NoError(t, err)
	require.Equal(t, newAttTarget, lte)
	ts, err := history.GetTargetData(ctx, newAttTarget)
	require.NoError(t, err)
	require.Equal(t, newAttSource, ts.Source)

	// Try an attestation that should be slashable (surrounding) spanning epochs 0 to 3.
	newAttSource = uint64(0)
	newAttTarget = uint64(3)
	require.Equal(t, true, isNewAttSlashable(ctx, history, newAttSource, newAttTarget, signingRoot))
}
