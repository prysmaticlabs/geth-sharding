package p2p

import (
	"context"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/p2p/discv5"
	"github.com/gogo/protobuf/proto"
	libp2p "github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/host"
	network "github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/pkg/errors"
	"github.com/prysmaticlabs/prysm/beacon-chain/p2p/encoder"
	"github.com/prysmaticlabs/prysm/shared"
	deprecatedp2p "github.com/prysmaticlabs/prysm/shared/deprecated-p2p"
	"github.com/prysmaticlabs/prysm/shared/event"
)

var _ = shared.Service(&Service{})
var pollingPeriod = 1 * time.Second

// Service for managing peer to peer (p2p) networking.
type Service struct {
	ctx         context.Context
	cancel      context.CancelFunc
	started     bool
	cfg         *Config
	startupErr  error
	dv5Listener Listener
	host        host.Host
	pubsub      *pubsub.PubSub
}

// NewService initializes a new p2p service compatible with shared.Service interface. No
// connections are made until the Start function is called during the service registry startup.
func NewService(cfg *Config) (*Service, error) {
	ctx, cancel := context.WithCancel(context.Background())
	return &Service{
		ctx:    ctx,
		cancel: cancel,
		cfg:    cfg,
	}, nil
}

// Start the p2p service.
func (s *Service) Start() {
	if s.started {
		log.Error("Attempted to start p2p service when it was already started")
		return
	}

	ipAddr := ipAddr(s.cfg)
	privKey, err := privKey(s.cfg)
	if err != nil {
		s.startupErr = err
		log.WithError(err).Error("Failed to generate p2p private key")
		return
	}

	// TODO(3147): Add host options
	opts := buildOptions(s.cfg, ipAddr, privKey)
	h, err := libp2p.New(s.ctx, opts...)
	if err != nil {
		s.startupErr = err
		log.WithError(err).Error("Failed to create p2p host")
		return
	}
	s.host = h

	// TODO(3147): Add gossip sub options
	// Gossipsub registration is done before we add in any new peers
	// due to libp2p's gossipsub implementation not taking into
	// account previously added peers when creating the gossipsub
	// object.
	gs, err := pubsub.NewGossipSub(s.ctx, s.host)
	if err != nil {
		s.startupErr = err

		log.WithError(err).Error("Failed to start pubsub")
		return
	}
	s.pubsub = gs

	if s.cfg.BootstrapNodeAddr != "" && !s.cfg.NoDiscovery {
		listener, err := startDiscoveryV5(ipAddr, privKey, s.cfg)
		if err != nil {
			log.WithError(err).Error("Failed to start discovery")
			s.startupErr = err
			return
		}
		s.dv5Listener = listener

		go s.listenForNewNodes()
	}

	s.started = true

	if len(s.cfg.StaticPeers) > 0 {
		addrs, err := manyMultiAddrsFromString(s.cfg.StaticPeers)
		if err != nil {
			log.Errorf("Could not connect to static peer: %v", err)
		}
		s.connectWithAllPeers(addrs)
	}

	registerMetrics(s)
	multiAddrs := s.host.Network().ListenAddresses()
	logIP4Addr(s.host.ID(), multiAddrs...)
}

// Stop the p2p service and terminate all peer connections.
func (s *Service) Stop() error {
	s.started = false
	if s.dv5Listener != nil {
		s.dv5Listener.Close()
	}
	return nil
}

// Status of the p2p service. Will return an error if the service is considered unhealthy to
// indicate that this node should not serve traffic until the issue has been resolved.
func (s *Service) Status() error {
	if !s.started {
		return errors.New("not running")
	}
	return nil
}

// Started returns true if the p2p service has successfully started.
func (s *Service) Started() bool {
	return s.started
}

// Encoding returns the configured networking encoding.
func (s *Service) Encoding() encoder.NetworkEncoding {
	encoding := s.cfg.Encoding
	switch encoding {
	case encoder.SSZ:
		return &encoder.SszNetworkEncoder{}
	case encoder.SSZSnappy:
		return &encoder.SszNetworkEncoder{UseSnappyCompression: true}
	default:
		panic("Invalid Network Encoding Flag Provided")
	}
}

// PubSub returns the p2p pubsub framework.
func (s *Service) PubSub() *pubsub.PubSub {
	return s.pubsub
}

// SetStreamHandler sets the protocol handler on the p2p host multiplexer.
// This method is a pass through to libp2pcore.Host.SetStreamHandler.
func (s *Service) SetStreamHandler(topic string, handler network.StreamHandler) {
	s.host.SetStreamHandler(protocol.ID(topic), handler)
}

// PeerID returns the Peer ID of the local peer.
func (s *Service) PeerID() peer.ID {
	return s.host.ID()
}

// Disconnect from a peer.
func (s *Service) Disconnect(pid peer.ID) error {
	return s.host.Network().ClosePeer(pid)
}

// listen for new nodes watches for new nodes in the network and adds them to the peerstore.
func (s *Service) listenForNewNodes() {
	node, err := discv5.ParseNode(s.cfg.BootstrapNodeAddr)
	if err != nil {
		log.Fatalf("could not parse bootstrap address: %v", err)
	}
	nodeID := node.ID
	ticker := time.NewTicker(pollingPeriod)
	for {
		select {
		case <-ticker.C:
			nodes := s.dv5Listener.Lookup(nodeID)
			multiAddresses := convertToMultiAddr(nodes)
			s.connectWithAllPeers(multiAddresses)
			// store furthest node as the next to lookup
			nodeID = nodes[len(nodes)-1].ID
		case <-s.ctx.Done():
			log.Debug("p2p context is closed, exiting routine")
			break

		}
	}
}

func (s *Service) connectWithAllPeers(multiAddrs []ma.Multiaddr) {
	addrInfos, err := peer.AddrInfosFromP2pAddrs(multiAddrs...)
	if err != nil {
		log.Errorf("Could not convert to peer address info's from multiaddresses: %v", err)
		return
	}
	for _, info := range addrInfos {
		if info.ID == s.host.ID() {
			continue
		}
		if err := s.host.Connect(s.ctx, info); err != nil {
			log.Errorf("Could not connect with peer: %v", err)
		}
	}
}

func logIP4Addr(id peer.ID, addrs ...ma.Multiaddr) {
	var correctAddr ma.Multiaddr
	for _, addr := range addrs {
		if strings.Contains(addr.String(), "/ip4/") {
			correctAddr = addr
			break
		}
	}
	log.Infof("Node's listening multiaddr is %s", correctAddr.String()+"/p2p/"+id.String())
}

// Subscribe to some topic.
// TODO(3147): Remove
// DEPRECATED: Do not use.
func (s *Service) Subscribe(_ proto.Message, _ chan deprecatedp2p.Message) event.Subscription {
	return nil
}
