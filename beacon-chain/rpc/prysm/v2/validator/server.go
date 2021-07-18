// Package validator defines a gRPC validator service implementation, providing
// critical endpoints for validator clients to submit blocks/attestations to the
// beacon node, receive assignments, and more.
package validator

import (
	"context"

	"github.com/prysmaticlabs/prysm/beacon-chain/blockchain"
	"github.com/prysmaticlabs/prysm/beacon-chain/operations/synccommittee"
	"github.com/prysmaticlabs/prysm/beacon-chain/p2p"
	validatorv1alpha1 "github.com/prysmaticlabs/prysm/beacon-chain/rpc/prysm/v1alpha1/validator"
)

// Server defines a server implementation of the gRPC Validator service,
// providing RPC endpoints for obtaining validator assignments per epoch, the slots
// and committees in which particular validators need to perform their responsibilities,
// and more.
type Server struct {
	V1Server          *validatorv1alpha1.Server
	Ctx               context.Context
	HeadFetcher       blockchain.HeadFetcher
	P2P               p2p.Broadcaster
	SyncCommitteePool synccommittee.Pool
}
