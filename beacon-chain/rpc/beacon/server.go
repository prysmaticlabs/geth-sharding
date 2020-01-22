package beacon

import (
	"context"
	"time"

	ethpb "github.com/prysmaticlabs/ethereumapis/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/beacon-chain/blockchain"
	statefeed "github.com/prysmaticlabs/prysm/beacon-chain/core/feed/state"
	"github.com/prysmaticlabs/prysm/beacon-chain/db"
	"github.com/prysmaticlabs/prysm/beacon-chain/operations/attestations"
	"github.com/prysmaticlabs/prysm/beacon-chain/powchain"
	pbp2p "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	"github.com/prysmaticlabs/prysm/shared/slotutil"
)

// Server defines a server implementation of the gRPC Beacon Chain service,
// providing RPC endpoints to access data relevant to the Ethereum 2.0 phase 0
// beacon chain.
type Server struct {
	BeaconDB             db.ReadOnlyDatabase
	Ctx                  context.Context
	ChainStartFetcher    powchain.ChainStartFetcher
	HeadFetcher          blockchain.HeadFetcher
	FinalizationFetcher  blockchain.FinalizationFetcher
	ParticipationFetcher blockchain.ParticipationFetcher
	StateNotifier        statefeed.Notifier
	Pool                 attestations.Pool
	IncomingAttestation  chan *ethpb.Attestation
	CanonicalStateChan   chan *pbp2p.BeaconState
	ChainStartChan       chan time.Time
	SlotTicker           slotutil.Ticker
}
