package rpc

import (
	"context"
	"fmt"

	ptypes "github.com/gogo/protobuf/types"
	"github.com/prysmaticlabs/prysm/beacon-chain/db"
	pbp2p "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/rpc/v1"
)

// BeaconServer defines a server implementation of the gRPC Beacon service,
// providing RPC endpoints for obtaining the canonical beacon chain head,
// fetching latest observed attestations, and more.
type BeaconServer struct {
	beaconDB            *db.BeaconDB
	ctx                 context.Context
	attestationService  attestationService
	incomingAttestation chan *pbp2p.Attestation
	canonicalStateChan  chan *pbp2p.BeaconState
}

// CanonicalHead of the current beacon chain. This method is requested on-demand
// by a validator when it is their time to propose or attest.
func (bs *BeaconServer) CanonicalHead(ctx context.Context, req *ptypes.Empty) (*pbp2p.BeaconBlock, error) {
	block, err := bs.beaconDB.ChainHead()
	if err != nil {
		return nil, fmt.Errorf("could not get canonical head block: %v", err)
	}
	return block, nil
}

// LatestAttestation streams the latest processed attestations to the rpc clients.
func (bs *BeaconServer) LatestAttestation(req *ptypes.Empty, stream pb.BeaconService_LatestAttestationServer) error {
	sub := bs.attestationService.IncomingAttestationFeed().Subscribe(bs.incomingAttestation)
	defer sub.Unsubscribe()
	for {
		select {
		case attestation := <-bs.incomingAttestation:
			log.Info("Sending attestation to RPC clients")
			if err := stream.Send(attestation); err != nil {
				return err
			}
		case <-sub.Err():
			log.Debug("Subscriber closed, exiting goroutine")
			return nil
		case <-bs.ctx.Done():
			log.Debug("RPC context closed, exiting goroutine")
			return nil
		}
	}
}
