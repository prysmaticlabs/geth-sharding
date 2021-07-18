package iface

import (
	"context"
	"errors"
	"time"

	types "github.com/prysmaticlabs/eth2-types"
	"github.com/prysmaticlabs/prysm/validator/keymanager"
)

// ErrConnectionIssue represents a connection problem.
var ErrConnectionIssue = errors.New("could not connect")

// ValidatorRole defines the validator role.
type ValidatorRole int8

const (
	// RoleUnknown means that the role of the validator cannot be determined.
	RoleUnknown ValidatorRole = iota
	// RoleAttester means that the validator should submit an attestation.
	RoleAttester
	// RoleProposer means that the validator should propose a block.
	RoleProposer
	// RoleAggregator means that the validator should submit an aggregation and proof.
	RoleAggregator
	// RoleSyncCommittee means that the validator should submit a sync committee message.
	RoleSyncCommittee
	// RoleSyncCommitteeAggregator means the valiator should aggregate sync committee messages and submit a sync committee contribution.
	RoleSyncCommitteeAggregator
)

// Validator interface defines the primary methods of a validator client.
type Validator interface {
	Done()
	WaitForChainStart(ctx context.Context) error
	WaitForSync(ctx context.Context) error
	WaitForActivation(ctx context.Context, accountsChangedChan chan [][48]byte) error
	SlasherReady(ctx context.Context) error
	CanonicalHeadSlot(ctx context.Context) (types.Slot, error)
	NextSlot() <-chan types.Slot
	SlotDeadline(slot types.Slot) time.Time
	LogValidatorGainsAndLosses(ctx context.Context, slot types.Slot) error
	UpdateDuties(ctx context.Context, slot types.Slot) error
	RolesAt(ctx context.Context, slot types.Slot) (map[[48]byte][]ValidatorRole, error) // validator pubKey -> roles
	SubmitAttestation(ctx context.Context, slot types.Slot, pubKey [48]byte)
	ProposeBlock(ctx context.Context, slot types.Slot, pubKey [48]byte)
	SubmitAggregateAndProof(ctx context.Context, slot types.Slot, pubKey [48]byte)
	SubmitSyncCommitteeMessage(ctx context.Context, slot types.Slot, pubKey [48]byte)
	SubmitSignedContributionAndProof(ctx context.Context, slot types.Slot, pubKey [48]byte)
	LogAttestationsSubmitted()
	LogNextDutyTimeLeft(slot types.Slot) error
	UpdateDomainDataCaches(ctx context.Context, slot types.Slot)
	WaitForWalletInitialization(ctx context.Context) error
	AllValidatorsAreExited(ctx context.Context) (bool, error)
	GetKeymanager() keymanager.IKeymanager
	ReceiveBlocks(ctx context.Context, connectionErrorChannel chan<- error)
	HandleKeyReload(ctx context.Context, newKeys [][48]byte) (bool, error)
	CheckDoppelGanger(ctx context.Context) error
}
