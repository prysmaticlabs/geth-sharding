// Package operation contains types for block operation-specific events fired
// during the runtime of a beacon node such as attestations, voluntary
// exits, and slashings.
package operation

import (
	ethpb "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	prysmv2 "github.com/prysmaticlabs/prysm/proto/prysm/v2"
)

const (
	// UnaggregatedAttReceived is sent after an unaggregated attestation object has been received
	// from the outside world. (eg. in RPC or sync)
	UnaggregatedAttReceived = iota + 1

	// AggregatedAttReceived is sent after an aggregated attestation object has been received
	// from the outside world. (eg. in sync)
	AggregatedAttReceived

	// ExitReceived is sent after an voluntary exit object has been received from the outside world (eg in RPC or sync)
	ExitReceived

	// SyncCommMessageReceived is sent after a sync committee message object has been received
	// from the outside world. (eg. in sync)
	SyncCommMessageReceived

	// SyncContributionReceived is sent after a sync contribution object has been received
	// from the outside world. (eg. in sync)
	SyncContributionReceived
)

// UnAggregatedAttReceivedData is the data sent with UnaggregatedAttReceived events.
type UnAggregatedAttReceivedData struct {
	// Attestation is the unaggregated attestation object.
	Attestation *ethpb.Attestation
}

// AggregatedAttReceivedData is the data sent with AggregatedAttReceived events.
type AggregatedAttReceivedData struct {
	// Attestation is the aggregated attestation object.
	Attestation *ethpb.AggregateAttestationAndProof
}

// ExitReceivedData is the data sent with ExitReceived events.
type ExitReceivedData struct {
	// Exit is the voluntary exit object.
	Exit *ethpb.SignedVoluntaryExit
}

// SyncCommReceivedData is the data sent with SyncCommMessageReceived events.
type SyncCommReceivedData struct {
	Message *prysmv2.SyncCommitteeMessage
}

// SyncContributionReceivedData is the data sent with SyncContributionReceived events.
type SyncContributionReceivedData struct {
	Contribution *prysmv2.ContributionAndProof
}
