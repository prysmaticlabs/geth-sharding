package rpc

import (
	"context"
	"sort"
	"time"

	ptypes "github.com/gogo/protobuf/types"
	"github.com/prysmaticlabs/prysm/beacon-chain/db"
	"github.com/prysmaticlabs/prysm/beacon-chain/sync"
	ethpb "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/shared/version"
	"google.golang.org/grpc"
)

type serviceInfoFetcher interface {
	GetServiceInfo() map[string]grpc.ServiceInfo
}

// NodeServer defines a server implementation of the gRPC Node service,
// providing RPC endpoints for verifying a beacon node's sync status, genesis and
// version information, and services the node implements and runs.
type NodeServer struct {
	syncChecker    sync.SyncChecker
	serviceFetcher serviceInfoFetcher
	beaconDB       *db.BeaconDB
}

// GetSyncStatus checks the current network sync status of the node.
func (ns *NodeServer) GetSyncStatus(ctx context.Context, _ *ptypes.Empty) (*ethpb.SyncStatus, error) {
	return &ethpb.SyncStatus{
		Syncing: ns.syncChecker.Syncing(),
	}, nil
}

// GetGenesis fetches genesis chain information of Ethereum 2.0
func (ns *NodeServer) GetGenesis(ctx context.Context, _ *ptypes.Empty) (*ethpb.Genesis, error) {
	beaconState, err := ns.beaconDB.FinalizedState()
	if err != nil {
		// TODO: return grpc Error.
		return nil, err
	}
	address, err := ns.beaconDB.DepositContractAddress(ctx)
	if err != nil {
		// TODO: return grpc Error.
		return nil, err
	}
	genesisTimestamp := time.Unix(int64(beaconState.GenesisTime), 0)
	genesisProtoTimestamp, err := ptypes.TimestampProto(genesisTimestamp)
	if err != nil {
		// TODO: return grpc Error.
		return nil, err
	}
	return &ethpb.Genesis{
		DepositContractAddress: address,
		GenesisTime:            genesisProtoTimestamp,
	}, nil
}

// GetVersion checks the version information of the beacon node.
func (ns *NodeServer) GetVersion(ctx context.Context, _ *ptypes.Empty) (*ethpb.Version, error) {
	return &ethpb.Version{
		Version: version.GetVersion(),
	}, nil
}

// ListImplementedServices lists the services implemented and enabled by this node.
//
// Any service not present in this list may return UNIMPLEMENTED or
// PERMISSION_DENIED. The server may also support fetching services by grpc
// reflection.
func (ns *NodeServer) ListImplementedServices(ctx context.Context, _ *ptypes.Empty) (*ethpb.ImplementedServices, error) {
	serviceInfo := ns.serviceFetcher.GetServiceInfo()
	serviceNames := make([]string, 0, len(serviceInfo))
	for svc := range serviceInfo {
		serviceNames = append(serviceNames, svc)
	}
	sort.Strings(serviceNames)
	return &ethpb.ImplementedServices{
		Services: serviceNames,
	}, nil
}
