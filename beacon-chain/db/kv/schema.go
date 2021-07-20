package kv

// The schema will define how to store and retrieve data from the db.
// we can prefix or suffix certain values such as `block` with attributes
// for prefix-wide scans across the underlying BoltDB buckets when filtering data.
// For example, we might store attestations as shard + attestation_root -> attestation, making
// it easy to scan for keys that have a certain shard number as a prefix and return those
// corresponding attestations.
var (
	attestationsBucket      = []byte("attestations")
	blocksBucket            = []byte("blocks")
	stateBucket             = []byte("state")
	stateSummaryBucket      = []byte("state-summary")
	proposerSlashingsBucket = []byte("proposer-slashings")
	attesterSlashingsBucket = []byte("attester-slashings")
	voluntaryExitsBucket    = []byte("voluntary-exits")
	chainMetadataBucket     = []byte("chain-metadata")
	checkpointBucket        = []byte("check-point")
	powchainBucket          = []byte("powchain")
	stateValidatorsBucket   = []byte("state-validators")

	// Deprecated: This bucket was migrated in PR 6461. Do not use, except for migrations.
	slotsHasObjectBucket = []byte("slots-has-objects")
	// Deprecated: This bucket was migrated in PR 6461. Do not use, except for migrations.
	archivedRootBucket = []byte("archived-index-root")

	// Key indices buckets.
	blockParentRootIndicesBucket        = []byte("block-parent-root-indices")
	blockSlotIndicesBucket              = []byte("block-slot-indices")
	stateSlotIndicesBucket              = []byte("state-slot-indices")
	attestationHeadBlockRootBucket      = []byte("attestation-head-block-root-indices")
	attestationSourceRootIndicesBucket  = []byte("attestation-source-root-indices")
	attestationSourceEpochIndicesBucket = []byte("attestation-source-epoch-indices")
	attestationTargetRootIndicesBucket  = []byte("attestation-target-root-indices")
	attestationTargetEpochIndicesBucket = []byte("attestation-target-epoch-indices")
	finalizedBlockRootsIndexBucket      = []byte("finalized-block-roots-index")
	blockRootValidatorHashesBucket      = []byte("block-root-validator-hashes")

	// Specific item keys.
	headBlockRootKey          = []byte("head-root")
	genesisBlockRootKey       = []byte("genesis-root")
	depositContractAddressKey = []byte("deposit-contract")
	justifiedCheckpointKey    = []byte("justified-checkpoint")
	finalizedCheckpointKey    = []byte("finalized-checkpoint")
	powchainDataKey           = []byte("powchain-data")

	// Deprecated: This index key was migrated in PR 6461. Do not use, except for migrations.
	lastArchivedIndexKey = []byte("last-archived")
	// Deprecated: This index key was migrated in PR 6461. Do not use, except for migrations.
	savedStateSlotsKey = []byte("saved-state-slots")

	// New state management service compatibility bucket.
	newStateServiceCompatibleBucket = []byte("new-state-compatible")

	// Migrations
	migrationsBucket = []byte("migrations")
)
