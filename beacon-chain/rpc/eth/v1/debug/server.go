// Package debug defines a gRPC beacon service implementation,
// following the official API standards https://ethereum.github.io/eth2.0-APIs/#/.
// This package includes the beacon and config endpoints.
package debug

import (
	"github.com/prysmaticlabs/prysm/beacon-chain/blockchain"
	"github.com/prysmaticlabs/prysm/beacon-chain/db"
	"github.com/prysmaticlabs/prysm/beacon-chain/rpc/statefetcher"
)

// Server defines a server implementation of the gRPC Beacon Chain service,
// providing RPC endpoints to access data relevant to the Ethereum Beacon Chain.
type Server struct {
	BeaconDB     db.ReadOnlyDatabase
	HeadFetcher  blockchain.HeadFetcher
	StateFetcher statefetcher.Fetcher
}
