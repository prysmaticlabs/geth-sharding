// Package params defines important constants that are essential to the beacon chain.
package params

const (
	// AttesterReward determines how much ETH attesters get for performing their duty.
	AttesterReward = 1
	// CycleLength is the beacon chain cycle length in slots.
	CycleLength = 64
	// ShardCount is a fixed number.
	ShardCount = 1024
	// DefaultBalance of a validator in ETH.
	DefaultBalance = 32
	// MaxValidators in the protocol.
	MaxValidators = 4194304
	// SlotDuration in seconds.
	SlotDuration = 8
	// Cofactor is used cutoff algorithm to select slot and shard cutoffs.
	Cofactor = 19
	// MinCommiteeSize is the minimal number of validator needs to be in a committee.
	MinCommiteeSize = 128
	// DefaultEndDynasty is the upper bound of dynasty. We use it to track queued and exited validators.
	DefaultEndDynasty = 9999999999999999999
	// BootstrappedValidatorsCount is the number of validators we seed the first crystallized
	// state with. This number has yet to be decided by research and is arbitrary for now.
	BootstrappedValidatorsCount = 1000
	// MinDynastyLength is the slots needed before dynasty transition happens.
	MinDynastyLength = 256
	// EtherDenomination is the denomination of ether in wei.
	EtherDenomination = 1e18
	// BaseRewardQuotient is where 1/BaseRewardQuotient is the per-slot interest rate which will,
	// compound to an annual rate of 3.88% for 10 million eth staked.
	BaseRewardQuotient = 32768
	// SqrtDropTime is a constant set to reflect the amount of time it will take for the quadratic leak to
	// cut nonparticipating validators’ deposits by 39.4%.
	SqrtDropTime = 1048576
)
