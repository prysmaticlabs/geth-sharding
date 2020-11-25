package kv

var (
	// Genesis information bucket key.
	genesisInfoBucket = []byte("genesis-info-bucket")
	// Genesis validators root key.
	genesisValidatorsRootKey = []byte("genesis-val-root")

	// Key to the lowest signed proposal in a validator bucket.
	lowestSignedProposalKey  = []byte("lowest-signed-proposal")
	highestSignedProposalKey = []byte("highest-signed-proposal")

	// Validator slashing protection from double proposals.
	historicProposalsBucket = []byte("proposal-history-bucket")
	// Validator slashing protection from double proposals.
	newhistoricProposalsBucket = []byte("proposal-history-bucket-interchange")
	// Validator slashing protection from slashable attestations.
	historicAttestationsBucket = []byte("attestation-history-bucket")
	// New Validator slashing protection from slashable attestations.
	newHistoricAttestationsBucket = []byte("attestation-history-bucket-interchange")
)
