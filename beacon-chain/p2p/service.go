package p2p

import (
	"context"
	"time"

	"github.com/ethereum/go-ethereum/p2p/discv5"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/host"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/pkg/errors"
	"github.com/prysmaticlabs/prysm/shared"
)

var _ = shared.Service(&Service{})
var pollingPeriod time.Duration = 1
var standardttl time.Duration = 1e8

// Service for managing peer to peer (p2p) networking.
type Service struct {
	ctx    context.Context
	cancel context.CancelFunc

	started     bool
	cfg         *Config
	startupErr  error
	dv5Listener *discv5.Network
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
	s.started = true

	// TODO(3147): Add host options
	opts, ipAddr, privKey := buildOptions(s.cfg)
	h, err := libp2p.New(s.ctx, opts...)
	if err != nil {
		s.startupErr = err
		return
	}
	s.host = h
	listener, err := startDiscoveryV5(ipAddr, privKey, s.cfg)
	if err != nil {
		s.startupErr = err
		return
	}
	s.dv5Listener = listener

	go s.listenForNewNodes()

	// TODO(3147): Add gossip sub options
	gs, err := pubsub.NewGossipSub(s.ctx, s.host)
	if err != nil {
		s.startupErr = err
		return
	}
	s.pubsub = gs
}

// Stop the p2p service and terminate all peer connections.
func (s *Service) Stop() error {
	s.started = false
	s.dv5Listener.Close()
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

// listen for new nodes watches for new nodes in the network and adds them to the peerstore.
func (s *Service) listenForNewNodes() {
	nodeID, err := discv5.HexID(s.cfg.BootstrapNodeAddr)
	if err != nil {
		log.Fatalf("could not parse bootstrap address: %v", err)
	}
	ticker := time.NewTicker(pollingPeriod * time.Second)
	ttl := standardttl * time.Hour
	for {
		select {
		case <-ticker.C:
			nodes := s.dv5Listener.Lookup(nodeID)
			multiAddresses := convertToMultiAddr(nodes)
			s.host.Peerstore().AddAddrs(s.host.ID(), multiAddresses, ttl)
			// store furthest node as the next to lookup
			nodeID = nodes[len(nodes)-1].ID
		case <-s.ctx.Done():
			log.Debug("p2p context is closed, exiting routine")
			break

		}
	}
}
