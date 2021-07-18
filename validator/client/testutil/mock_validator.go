package testutil

import (
	"bytes"
	"context"
	"time"

	types "github.com/prysmaticlabs/eth2-types"
	ethpb "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/shared/timeutils"
	"github.com/prysmaticlabs/prysm/validator/client/iface"
	"github.com/prysmaticlabs/prysm/validator/keymanager"
)

var _ iface.Validator = (*FakeValidator)(nil)

// FakeValidator for mocking.
type FakeValidator struct {
	DoneCalled                        bool
	WaitForWalletInitializationCalled bool
	SlasherReadyCalled                bool
	NextSlotCalled                    bool
	UpdateDutiesCalled                bool
	UpdateProtectionsCalled           bool
	RoleAtCalled                      bool
	AttestToBlockHeadCalled           bool
	ProposeBlockCalled                bool
	LogValidatorGainsAndLossesCalled  bool
	SaveProtectionsCalled             bool
	DeleteProtectionCalled            bool
	SlotDeadlineCalled                bool
	HandleKeyReloadCalled             bool
	WaitForChainStartCalled           int
	WaitForSyncCalled                 int
	WaitForActivationCalled           int
	CanonicalHeadSlotCalled           int
	ReceiveBlocksCalled               int
	RetryTillSuccess                  int
	ProposeBlockArg1                  uint64
	AttestToBlockHeadArg1             uint64
	RoleAtArg1                        uint64
	UpdateDutiesArg1                  uint64
	NextSlotRet                       <-chan types.Slot
	PublicKey                         string
	UpdateDutiesRet                   error
	RolesAtRet                        []iface.ValidatorRole
	Balances                          map[[48]byte]uint64
	IndexToPubkeyMap                  map[uint64][48]byte
	PubkeyToIndexMap                  map[[48]byte]uint64
	PubkeysToStatusesMap              map[[48]byte]ethpb.ValidatorStatus
	Keymanager                        keymanager.IKeymanager
}

type ctxKey string

// AllValidatorsAreExitedCtxKey represents the metadata context key used for exits.
var AllValidatorsAreExitedCtxKey = ctxKey("exited")

// Done for mocking.
func (fv *FakeValidator) Done() {
	fv.DoneCalled = true
}

// WaitForWalletInitialization for mocking.
func (fv *FakeValidator) WaitForWalletInitialization(_ context.Context) error {
	fv.WaitForWalletInitializationCalled = true
	return nil
}

// WaitForChainStart for mocking.
func (fv *FakeValidator) WaitForChainStart(_ context.Context) error {
	fv.WaitForChainStartCalled++
	if fv.RetryTillSuccess >= fv.WaitForChainStartCalled {
		return iface.ErrConnectionIssue
	}
	return nil
}

// WaitForActivation for mocking.
func (fv *FakeValidator) WaitForActivation(_ context.Context, _ chan [][48]byte) error {
	fv.WaitForActivationCalled++
	if fv.RetryTillSuccess >= fv.WaitForActivationCalled {
		return iface.ErrConnectionIssue
	}
	return nil
}

// WaitForSync for mocking.
func (fv *FakeValidator) WaitForSync(_ context.Context) error {
	fv.WaitForSyncCalled++
	if fv.RetryTillSuccess >= fv.WaitForSyncCalled {
		return iface.ErrConnectionIssue
	}
	return nil
}

// SlasherReady for mocking.
func (fv *FakeValidator) SlasherReady(_ context.Context) error {
	fv.SlasherReadyCalled = true
	return nil
}

// CanonicalHeadSlot for mocking.
func (fv *FakeValidator) CanonicalHeadSlot(_ context.Context) (types.Slot, error) {
	fv.CanonicalHeadSlotCalled++
	if fv.RetryTillSuccess > fv.CanonicalHeadSlotCalled {
		return 0, iface.ErrConnectionIssue
	}
	return 0, nil
}

// SlotDeadline for mocking.
func (fv *FakeValidator) SlotDeadline(_ types.Slot) time.Time {
	fv.SlotDeadlineCalled = true
	return timeutils.Now()
}

// NextSlot for mocking.
func (fv *FakeValidator) NextSlot() <-chan types.Slot {
	fv.NextSlotCalled = true
	return fv.NextSlotRet
}

// UpdateDuties for mocking.
func (fv *FakeValidator) UpdateDuties(_ context.Context, slot types.Slot) error {
	fv.UpdateDutiesCalled = true
	fv.UpdateDutiesArg1 = uint64(slot)
	return fv.UpdateDutiesRet
}

// UpdateProtections for mocking.
func (fv *FakeValidator) UpdateProtections(_ context.Context, _ uint64) error {
	fv.UpdateProtectionsCalled = true
	return nil
}

// LogValidatorGainsAndLosses for mocking.
func (fv *FakeValidator) LogValidatorGainsAndLosses(_ context.Context, _ types.Slot) error {
	fv.LogValidatorGainsAndLossesCalled = true
	return nil
}

// ResetAttesterProtectionData for mocking.
func (fv *FakeValidator) ResetAttesterProtectionData() {
	fv.DeleteProtectionCalled = true
}

// RolesAt for mocking.
func (fv *FakeValidator) RolesAt(_ context.Context, slot types.Slot) (map[[48]byte][]iface.ValidatorRole, error) {
	fv.RoleAtCalled = true
	fv.RoleAtArg1 = uint64(slot)
	vr := make(map[[48]byte][]iface.ValidatorRole)
	vr[[48]byte{1}] = fv.RolesAtRet
	return vr, nil
}

// SubmitAttestation for mocking.
func (fv *FakeValidator) SubmitAttestation(_ context.Context, slot types.Slot, _ [48]byte) {
	fv.AttestToBlockHeadCalled = true
	fv.AttestToBlockHeadArg1 = uint64(slot)
}

// ProposeBlock for mocking.
func (fv *FakeValidator) ProposeBlock(_ context.Context, slot types.Slot, _ [48]byte) {
	fv.ProposeBlockCalled = true
	fv.ProposeBlockArg1 = uint64(slot)
}

// SubmitAggregateAndProof for mocking.
func (fv *FakeValidator) SubmitAggregateAndProof(_ context.Context, _ types.Slot, _ [48]byte) {}

// SubmitSyncCommitteeMessage for mocking.
func (fv *FakeValidator) SubmitSyncCommitteeMessage(_ context.Context, _ types.Slot, _ [48]byte) {}

// LogAttestationsSubmitted for mocking.
func (fv *FakeValidator) LogAttestationsSubmitted() {}

// LogNextDutyTimeLeft for mocking.
func (fv *FakeValidator) LogNextDutyTimeLeft(slot types.Slot) error {
	return nil
}

// UpdateDomainDataCaches for mocking.
func (fv *FakeValidator) UpdateDomainDataCaches(context.Context, types.Slot) {}

// BalancesByPubkeys for mocking.
func (fv *FakeValidator) BalancesByPubkeys(_ context.Context) map[[48]byte]uint64 {
	return fv.Balances
}

// IndicesToPubkeys for mocking.
func (fv *FakeValidator) IndicesToPubkeys(_ context.Context) map[uint64][48]byte {
	return fv.IndexToPubkeyMap
}

// PubkeysToIndices for mocking.
func (fv *FakeValidator) PubkeysToIndices(_ context.Context) map[[48]byte]uint64 {
	return fv.PubkeyToIndexMap
}

// PubkeysToStatuses for mocking.
func (fv *FakeValidator) PubkeysToStatuses(_ context.Context) map[[48]byte]ethpb.ValidatorStatus {
	return fv.PubkeysToStatusesMap
}

// AllValidatorsAreExited for mocking
func (fv *FakeValidator) AllValidatorsAreExited(ctx context.Context) (bool, error) {
	if ctx.Value(AllValidatorsAreExitedCtxKey) == nil {
		return false, nil
	}
	return ctx.Value(AllValidatorsAreExitedCtxKey).(bool), nil
}

// GetKeymanager for mocking
func (fv *FakeValidator) GetKeymanager() keymanager.IKeymanager {
	return fv.Keymanager
}

// CheckDoppelGanger for mocking
func (fv *FakeValidator) CheckDoppelGanger(ctx context.Context) error {
	return nil
}

// ReceiveBlocks for mocking
func (fv *FakeValidator) ReceiveBlocks(ctx context.Context, connectionErrorChannel chan<- error) {
	fv.ReceiveBlocksCalled++
	if fv.RetryTillSuccess > fv.ReceiveBlocksCalled {
		connectionErrorChannel <- iface.ErrConnectionIssue
	}
}

// HandleKeyReload for mocking
func (fv *FakeValidator) HandleKeyReload(_ context.Context, newKeys [][48]byte) (anyActive bool, err error) {
	fv.HandleKeyReloadCalled = true
	for _, key := range newKeys {
		if bytes.Equal(key[:], ActiveKey[:]) {
			return true, nil
		}
	}
	return false, nil
}

// SubmitSignedContributionAndProof for mocking
func (fv *FakeValidator) SubmitSignedContributionAndProof(ctx context.Context, slot types.Slot, pubKey [48]byte) {

}
