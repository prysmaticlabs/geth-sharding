package v1

import (
	types "github.com/prysmaticlabs/eth2-types"
	ethpb "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
)

// ReadOnlyValidator returns a wrapper that only allows fields from a validator
// to be read, and prevents any modification of internal validator fields.
type ReadOnlyValidator struct {
	validator *ethpb.Validator
}

// NewValidator initializes the read only wrapper for validator.
func NewValidator(v *ethpb.Validator) *ReadOnlyValidator {
	return &ReadOnlyValidator{
		validator: v,
	}
}

// EffectiveBalance returns the effective balance of the
// read only validator.
func (v ReadOnlyValidator) EffectiveBalance() uint64 {
	if v.IsNil() {
		return 0
	}
	return v.validator.EffectiveBalance
}

// ActivationEligibilityEpoch returns the activation eligibility epoch of the
// read only validator.
func (v ReadOnlyValidator) ActivationEligibilityEpoch() types.Epoch {
	if v.IsNil() {
		return 0
	}
	return v.validator.ActivationEligibilityEpoch
}

// ActivationEpoch returns the activation epoch of the
// read only validator.
func (v ReadOnlyValidator) ActivationEpoch() types.Epoch {
	if v.IsNil() {
		return 0
	}
	return v.validator.ActivationEpoch
}

// WithdrawableEpoch returns the withdrawable epoch of the
// read only validator.
func (v ReadOnlyValidator) WithdrawableEpoch() types.Epoch {
	if v.IsNil() {
		return 0
	}
	return v.validator.WithdrawableEpoch
}

// ExitEpoch returns the exit epoch of the
// read only validator.
func (v ReadOnlyValidator) ExitEpoch() types.Epoch {
	if v.IsNil() {
		return 0
	}
	return v.validator.ExitEpoch
}

// PublicKey returns the public key of the
// read only validator.
func (v ReadOnlyValidator) PublicKey() [48]byte {
	if v.IsNil() {
		return [48]byte{}
	}
	var pubkey [48]byte
	copy(pubkey[:], v.validator.PublicKey)
	return pubkey
}

// WithdrawalCredentials returns the withdrawal credentials of the
// read only validator.
func (v ReadOnlyValidator) WithdrawalCredentials() []byte {
	creds := make([]byte, len(v.validator.WithdrawalCredentials))
	copy(creds, v.validator.WithdrawalCredentials)
	return creds
}

// Slashed returns the read only validator is slashed.
func (v ReadOnlyValidator) Slashed() bool {
	if v.IsNil() {
		return false
	}
	return v.validator.Slashed
}

// IsNil returns true if the validator is nil.
func (v ReadOnlyValidator) IsNil() bool {
	return v.validator == nil
}
