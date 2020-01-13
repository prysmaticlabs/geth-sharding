package precompute

// Validator stores the pre computation of individual validator's attesting records these records
// consist of attestation votes, block inclusion record. Pre computing and storing such record
// is essential for process epoch optimizations.
type Validator struct {
	// IsSlashed is true if the validator has been slashed.
	IsSlashed bool
	// IsWithdrawableCurrentEpoch is true if the validator can withdraw current epoch.
	IsWithdrawableCurrentEpoch bool
	// IsActiveCurrentEpoch is true if the validator was active current epoch.
	IsActiveCurrentEpoch bool
	// IsActivePrevEpoch is true if the validator was active prev epoch.
	IsActivePrevEpoch bool
	// IsCurrentEpochAttester is true if the validator attested current epoch.
	IsCurrentEpochAttester bool
	// IsCurrentEpochTargetAttester is true if the validator attested current epoch target.
	IsCurrentEpochTargetAttester bool
	// IsPrevEpochAttester is true if the validator attested previous epoch.
	IsPrevEpochAttester bool
	// IsPrevEpochTargetAttester is true if the validator attested previous epoch target.
	IsPrevEpochTargetAttester bool
	// IsHeadAttester is true if the validator attested head.
	IsPrevEpochHeadAttester bool

	// CurrentEpochEffectiveBalance is how much effective balance this validator validator has current epoch.
	CurrentEpochEffectiveBalance uint64
	// InclusionSlot is the slot of when the attestation gets included in the chain.
	InclusionSlot uint64
	// InclusionDistance is the distance between the assigned slot and this validator's attestation was included in block.
	InclusionDistance uint64
	// ProposerIndex is the index of proposer at slot where this validator's attestation was included.
	ProposerIndex uint64
}

// Balance stores the pre computation of the total participated balances for a given epoch
// Pre computing and storing such record is essential for process epoch optimizations.
type Balance struct {
	// CurrentEpoch is the total effective balance of all active validators during current epoch.
	CurrentEpoch uint64
	// PrevEpoch is the total effective balance of all active validators during prev epoch.
	PrevEpoch uint64
	// CurrentEpochAttesters is the total effective balance of all validators who attested during current epoch.
	CurrentEpochAttesters uint64
	// CurrentEpochTargetAttesters is the total effective balance of all validators who attested
	// for epoch boundary block during current epoch.
	CurrentEpochTargetAttesters uint64
	// PrevEpochAttesters is the total effective balance of all validators who attested during prev epoch.
	PrevEpochAttesters uint64
	// PrevEpochTargetAttesters is the total effective balance of all validators who attested
	// for epoch boundary block during prev epoch.
	PrevEpochTargetAttesters uint64
	// PrevEpochHeadAttesters is the total effective balance of all validators who attested
	// correctly for head block during prev epoch.
	PrevEpochHeadAttesters uint64
}
