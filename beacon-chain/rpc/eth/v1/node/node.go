package node

import (
	"context"
	"fmt"
	"net/http"
	"runtime"
	"strconv"
	"strings"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/pkg/errors"
	"github.com/prysmaticlabs/prysm/beacon-chain/p2p"
	"github.com/prysmaticlabs/prysm/beacon-chain/p2p/peers"
	"github.com/prysmaticlabs/prysm/beacon-chain/p2p/peers/peerdata"
	ethpb "github.com/prysmaticlabs/prysm/proto/eth/v1"
	ethpb_alpha "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/proto/migration"
	"github.com/prysmaticlabs/prysm/shared/grpcutils"
	"github.com/prysmaticlabs/prysm/shared/version"
	"go.opencensus.io/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

var (
	stateConnecting    = ethpb.ConnectionState_CONNECTING.String()
	stateConnected     = ethpb.ConnectionState_CONNECTED.String()
	stateDisconnecting = ethpb.ConnectionState_DISCONNECTING.String()
	stateDisconnected  = ethpb.ConnectionState_DISCONNECTED.String()
	directionInbound   = ethpb.PeerDirection_INBOUND.String()
	directionOutbound  = ethpb.PeerDirection_OUTBOUND.String()
)

// GetIdentity retrieves data about the node's network presence.
func (ns *Server) GetIdentity(ctx context.Context, _ *emptypb.Empty) (*ethpb.IdentityResponse, error) {
	ctx, span := trace.StartSpan(ctx, "nodeV1.GetIdentity")
	defer span.End()

	peerId := ns.PeerManager.PeerID().Pretty()

	serializedEnr, err := p2p.SerializeENR(ns.PeerManager.ENR())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Could not obtain enr: %v", err)
	}
	enr := "enr:" + serializedEnr

	sourcep2p := ns.PeerManager.Host().Addrs()
	p2pAddresses := make([]string, len(sourcep2p))
	for i := range sourcep2p {
		p2pAddresses[i] = sourcep2p[i].String() + "/p2p/" + peerId
	}

	sourceDisc, err := ns.PeerManager.DiscoveryAddresses()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Could not obtain discovery address: %v", err)
	}
	discoveryAddresses := make([]string, len(sourceDisc))
	for i := range sourceDisc {
		discoveryAddresses[i] = sourceDisc[i].String()
	}

	metadata := &ethpb.Metadata{
		SeqNumber: ns.MetadataProvider.MetadataSeq(),
		Attnets:   ns.MetadataProvider.Metadata().AttnetsBitfield(),
	}

	return &ethpb.IdentityResponse{
		Data: &ethpb.Identity{
			PeerId:             peerId,
			Enr:                enr,
			P2PAddresses:       p2pAddresses,
			DiscoveryAddresses: discoveryAddresses,
			Metadata:           metadata,
		},
	}, nil
}

// GetPeer retrieves data about the given peer.
func (ns *Server) GetPeer(ctx context.Context, req *ethpb.PeerRequest) (*ethpb.PeerResponse, error) {
	ctx, span := trace.StartSpan(ctx, "node.GetPeer")
	defer span.End()

	peerStatus := ns.PeersFetcher.Peers()
	id, err := peer.Decode(req.PeerId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid peer ID: %v", err)
	}
	enr, err := peerStatus.ENR(id)
	if err != nil {
		if errors.Is(err, peerdata.ErrPeerUnknown) {
			return nil, status.Error(codes.NotFound, "Peer not found")
		}
		return nil, status.Errorf(codes.Internal, "Could not obtain ENR: %v", err)
	}
	serializedEnr, err := p2p.SerializeENR(enr)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Could not obtain ENR: %v", err)
	}
	p2pAddress, err := peerStatus.Address(id)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Could not obtain address: %v", err)
	}
	state, err := peerStatus.ConnectionState(id)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Could not obtain connection state: %v", err)
	}
	direction, err := peerStatus.Direction(id)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Could not obtain direction: %v", err)
	}
	if ethpb_alpha.PeerDirection(direction) == ethpb_alpha.PeerDirection_UNKNOWN {
		return nil, status.Error(codes.NotFound, "Peer not found")
	}

	v1ConnState := migration.V1Alpha1ConnectionStateToV1(ethpb_alpha.ConnectionState(state))
	v1PeerDirection, err := migration.V1Alpha1PeerDirectionToV1(ethpb_alpha.PeerDirection(direction))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Could not handle peer direction: %v", err)
	}
	return &ethpb.PeerResponse{
		Data: &ethpb.Peer{
			PeerId:             req.PeerId,
			Enr:                "enr:" + serializedEnr,
			LastSeenP2PAddress: p2pAddress.String(),
			State:              v1ConnState,
			Direction:          v1PeerDirection,
		},
	}, nil
}

// ListPeers retrieves data about the node's network peers.
func (ns *Server) ListPeers(ctx context.Context, req *ethpb.PeersRequest) (*ethpb.PeersResponse, error) {
	ctx, span := trace.StartSpan(ctx, "node.ListPeers")
	defer span.End()

	peerStatus := ns.PeersFetcher.Peers()
	emptyStateFilter, emptyDirectionFilter := ns.handleEmptyFilters(req)

	if emptyStateFilter && emptyDirectionFilter {
		allIds := peerStatus.All()
		allPeers := make([]*ethpb.Peer, 0, len(allIds))
		for _, id := range allIds {
			p, err := peerInfo(peerStatus, id)
			if err != nil {
				return nil, status.Errorf(codes.Internal, "Could not get peer info: %v", err)
			}
			if p == nil {
				continue
			}
			allPeers = append(allPeers, p)
		}
		return &ethpb.PeersResponse{Data: allPeers}, nil
	}

	var stateIds []peer.ID
	if emptyStateFilter {
		stateIds = peerStatus.All()
	} else {
		for _, stateFilter := range req.State {
			normalized := strings.ToUpper(stateFilter.String())
			if normalized == stateConnecting {
				ids := peerStatus.Connecting()
				stateIds = append(stateIds, ids...)
				continue
			}
			if normalized == stateConnected {
				ids := peerStatus.Connected()
				stateIds = append(stateIds, ids...)
				continue
			}
			if normalized == stateDisconnecting {
				ids := peerStatus.Disconnecting()
				stateIds = append(stateIds, ids...)
				continue
			}
			if normalized == stateDisconnected {
				ids := peerStatus.Disconnected()
				stateIds = append(stateIds, ids...)
				continue
			}
		}
	}

	var directionIds []peer.ID
	if emptyDirectionFilter {
		directionIds = peerStatus.All()
	} else {
		for _, directionFilter := range req.Direction {
			normalized := strings.ToUpper(directionFilter.String())
			if normalized == directionInbound {
				ids := peerStatus.Inbound()
				directionIds = append(directionIds, ids...)
				continue
			}
			if normalized == directionOutbound {
				ids := peerStatus.Outbound()
				directionIds = append(directionIds, ids...)
				continue
			}
		}
	}

	var filteredIds []peer.ID
	for _, stateId := range stateIds {
		for _, directionId := range directionIds {
			if stateId.Pretty() == directionId.Pretty() {
				filteredIds = append(filteredIds, stateId)
				break
			}
		}
	}
	filteredPeers := make([]*ethpb.Peer, 0, len(filteredIds))
	for _, id := range filteredIds {
		p, err := peerInfo(peerStatus, id)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "Could not get peer info: %v", err)
		}
		if p == nil {
			continue
		}
		filteredPeers = append(filteredPeers, p)
	}

	return &ethpb.PeersResponse{Data: filteredPeers}, nil
}

// PeerCount retrieves retrieves number of known peers.
func (ns *Server) PeerCount(ctx context.Context, _ *emptypb.Empty) (*ethpb.PeerCountResponse, error) {
	ctx, span := trace.StartSpan(ctx, "node.PeerCount")
	defer span.End()

	peerStatus := ns.PeersFetcher.Peers()

	return &ethpb.PeerCountResponse{
		Data: &ethpb.PeerCountResponse_PeerCount{
			Disconnected:  uint64(len(peerStatus.Disconnected())),
			Connecting:    uint64(len(peerStatus.Connecting())),
			Connected:     uint64(len(peerStatus.Connected())),
			Disconnecting: uint64(len(peerStatus.Disconnecting())),
		},
	}, nil
}

// GetVersion requests that the beacon node identify information about its implementation in a
// format similar to a HTTP User-Agent field.
func (ns *Server) GetVersion(ctx context.Context, _ *emptypb.Empty) (*ethpb.VersionResponse, error) {
	ctx, span := trace.StartSpan(ctx, "node.Version")
	defer span.End()

	v := fmt.Sprintf("Prysm/%s (%s %s)", version.SemanticVersion(), runtime.GOOS, runtime.GOARCH)
	return &ethpb.VersionResponse{
		Data: &ethpb.Version{
			Version: v,
		},
	}, nil
}

// GetSyncStatus requests the beacon node to describe if it's currently syncing or not, and
// if it is, what block it is up to.
func (ns *Server) GetSyncStatus(ctx context.Context, _ *emptypb.Empty) (*ethpb.SyncingResponse, error) {
	ctx, span := trace.StartSpan(ctx, "node.GetSyncStatus")
	defer span.End()

	headSlot := ns.HeadFetcher.HeadSlot()
	return &ethpb.SyncingResponse{
		Data: &ethpb.SyncInfo{
			HeadSlot:     headSlot,
			SyncDistance: ns.GenesisTimeFetcher.CurrentSlot() - headSlot,
			IsSyncing:    ns.SyncChecker.Syncing(),
		},
	}, nil
}

// GetHealth returns node health status in http status codes. Useful for load balancers.
// Response Usage:
//    "200":
//      description: Node is ready
//    "206":
//      description: Node is syncing but can serve incomplete data
//    "503":
//      description: Node not initialized or having issues
func (ns *Server) GetHealth(ctx context.Context, _ *emptypb.Empty) (*emptypb.Empty, error) {
	ctx, span := trace.StartSpan(ctx, "node.GetHealth")
	defer span.End()

	if ns.SyncChecker.Synced() {
		return &emptypb.Empty{}, nil
	}
	if ns.SyncChecker.Syncing() || ns.SyncChecker.Initialized() {
		if err := grpc.SetHeader(ctx, metadata.Pairs(grpcutils.HttpCodeMetadataKey, strconv.Itoa(http.StatusPartialContent))); err != nil {
			// We return a positive result because failing to set a non-gRPC related header should not cause the gRPC call to fail.
			return &emptypb.Empty{}, nil
		}
		return &emptypb.Empty{}, nil
	}
	return &emptypb.Empty{}, status.Error(codes.Internal, "Node not initialized or having issues")
}

func (ns *Server) handleEmptyFilters(req *ethpb.PeersRequest) (emptyState, emptyDirection bool) {
	emptyState = true
	for _, stateFilter := range req.State {
		normalized := strings.ToUpper(stateFilter.String())
		filterValid := normalized == stateConnecting || normalized == stateConnected ||
			normalized == stateDisconnecting || normalized == stateDisconnected
		if filterValid {
			emptyState = false
			break
		}
	}

	emptyDirection = true
	for _, directionFilter := range req.Direction {
		normalized := strings.ToUpper(directionFilter.String())
		filterValid := normalized == directionInbound || normalized == directionOutbound
		if filterValid {
			emptyDirection = false
			break
		}
	}

	return emptyState, emptyDirection
}

func peerInfo(peerStatus *peers.Status, id peer.ID) (*ethpb.Peer, error) {
	enr, err := peerStatus.ENR(id)
	if err != nil {
		return nil, errors.Wrap(err, "could not obtain ENR")
	}
	var serializedEnr string
	if enr != nil {
		serializedEnr, err = p2p.SerializeENR(enr)
		if err != nil {
			return nil, errors.Wrap(err, "could not serialize ENR")
		}
	}
	address, err := peerStatus.Address(id)
	if err != nil {
		return nil, errors.Wrap(err, "could not obtain address")
	}
	connectionState, err := peerStatus.ConnectionState(id)
	if err != nil {
		return nil, errors.Wrap(err, "could not obtain connection state")
	}
	direction, err := peerStatus.Direction(id)
	if err != nil {
		return nil, errors.Wrap(err, "could not obtain direction")
	}
	if ethpb_alpha.PeerDirection(direction) == ethpb_alpha.PeerDirection_UNKNOWN {
		return nil, nil
	}
	v1ConnState := migration.V1Alpha1ConnectionStateToV1(ethpb_alpha.ConnectionState(connectionState))
	v1PeerDirection, err := migration.V1Alpha1PeerDirectionToV1(ethpb_alpha.PeerDirection(direction))
	if err != nil {
		return nil, errors.Wrapf(err, "could not handle peer direction")
	}
	p := ethpb.Peer{
		PeerId:    id.Pretty(),
		State:     v1ConnState,
		Direction: v1PeerDirection,
	}
	if address != nil {
		p.LastSeenP2PAddress = address.String()
	}
	if serializedEnr != "" {
		p.Enr = "enr:" + serializedEnr
	}

	return &p, nil
}
