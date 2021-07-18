// Package beacon defines a gRPC beacon service implementation, providing
// useful endpoints for checking fetching chain-specific data such as
// blocks, committees, validators, assignments, and more.
package beacon

import (
	"context"

	beaconv1alpha1 "github.com/prysmaticlabs/prysm/beacon-chain/rpc/prysm/v1alpha1/beacon"
)

// Server defines a server implementation of the gRPC Beacon Chain service,
// providing RPC endpoints to access data relevant to the Ethereum beacon chain.
type Server struct {
	Ctx      context.Context
	V1Server *beaconv1alpha1.Server
}
