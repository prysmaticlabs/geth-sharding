// Package p2p defines the network protocol implementation for eth2
// used by beacon nodes, including peer discovery using discv5, gossip-sub
// using libp2p, and handing peer lifecycles + handshakes.
package p2p

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/dgraph-io/ristretto"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/enr"
	"github.com/gogo/protobuf/proto"
	ds "github.com/ipfs/go-datastore"
	dsync "github.com/ipfs/go-datastore/sync"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
	kaddht "github.com/libp2p/go-libp2p-kad-dht"
	dhtopts "github.com/libp2p/go-libp2p-kad-dht/opts"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	rhost "github.com/libp2p/go-libp2p/p2p/host/routed"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/pkg/errors"
	"github.com/prysmaticlabs/go-bitfield"
	"github.com/prysmaticlabs/prysm/beacon-chain/cache"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/feed"
	statefeed "github.com/prysmaticlabs/prysm/beacon-chain/core/feed/state"
	"github.com/prysmaticlabs/prysm/beacon-chain/p2p/encoder"
	"github.com/prysmaticlabs/prysm/beacon-chain/p2p/peers"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	"github.com/prysmaticlabs/prysm/shared"
	"github.com/prysmaticlabs/prysm/shared/runutil"
	"github.com/prysmaticlabs/prysm/shared/slotutil"
	"github.com/sirupsen/logrus"
)

var _ = shared.Service(&Service{})

// Check local table every 15 seconds for newly added peers.
var pollingPeriod = 15 * time.Second

// Refresh rate of ENR set at twice per slot.
var refreshRate = slotutil.DivideSlotBy(2)

// search limit for number of peers in discovery v5.
const searchLimit = 100

// lookup limit whenever looking up for random nodes.
const lookupLimit = 15

const prysmProtocolPrefix = "/prysm/0.0.0"

// maxBadResponses is the maximum number of bad responses from a peer before we stop talking to it.
const maxBadResponses = 3

const (
	pubsubFlood  = "flood"
	pubsubGossip = "gossip"
	pubsubRandom = "random"
)

// Service for managing peer to peer (p2p) networking.
type Service struct {
	started               bool
	isPreGenesis          bool
	pingMethod            func(ctx context.Context, id peer.ID) error
	cancel                context.CancelFunc
	cfg                   *Config
	peers                 *peers.Status
	dht                   *kaddht.IpfsDHT
	privKey               *ecdsa.PrivateKey
	exclusionList         *ristretto.Cache
	metaData              *pb.MetaData
	pubsub                *pubsub.PubSub
	dv5Listener           Listener
	startupErr            error
	stateNotifier         statefeed.Notifier
	ctx                   context.Context
	host                  host.Host
	genesisTime           time.Time
	genesisValidatorsRoot []byte
}

// NewService initializes a new p2p service compatible with shared.Service interface. No
// connections are made until the Start function is called during the service registry startup.
func NewService(cfg *Config) (*Service, error) {
	var err error
	ctx, cancel := context.WithCancel(context.Background())
	cache, err := ristretto.NewCache(&ristretto.Config{
		NumCounters: 1000,
		MaxCost:     1000,
		BufferItems: 64,
	})
	if err != nil {
		return nil, err
	}

	s := &Service{
		ctx:           ctx,
		stateNotifier: cfg.StateNotifier,
		cancel:        cancel,
		cfg:           cfg,
		exclusionList: cache,
		isPreGenesis:  true,
	}

	dv5Nodes, kadDHTNodes := parseBootStrapAddrs(s.cfg.BootstrapNodeAddr)

	cfg.Discv5BootStrapAddr = dv5Nodes
	cfg.KademliaBootStrapAddr = kadDHTNodes

	ipAddr := ipAddr()
	s.privKey, err = privKey(s.cfg)
	if err != nil {
		log.WithError(err).Error("Failed to generate p2p private key")
		return nil, err
	}
	s.metaData, err = metaDataFromConfig(s.cfg)
	if err != nil {
		log.WithError(err).Error("Failed to create peer metadata")
		return nil, err
	}

	opts := buildOptions(s.cfg, ipAddr, s.privKey)
	h, err := libp2p.New(s.ctx, opts...)
	if err != nil {
		log.WithError(err).Error("Failed to create p2p host")
		return nil, err
	}

	if len(cfg.KademliaBootStrapAddr) != 0 && !cfg.NoDiscovery {
		dopts := []dhtopts.Option{
			dhtopts.Datastore(dsync.MutexWrap(ds.NewMapDatastore())),
			dhtopts.Protocols(
				prysmProtocolPrefix + "/dht",
			),
		}

		s.dht, err = kaddht.New(ctx, h, dopts...)
		if err != nil {
			return nil, err
		}
		// Wrap host with a routed host so that peers can be looked up in the
		// distributed hash table by their peer ID.
		h = rhost.Wrap(h, s.dht)
	}
	s.host = h

	// TODO(3147): Add gossip sub options
	// Gossipsub registration is done before we add in any new peers
	// due to libp2p's gossipsub implementation not taking into
	// account previously added peers when creating the gossipsub
	// object.
	psOpts := []pubsub.Option{
		pubsub.WithMessageSigning(false),
		pubsub.WithStrictSignatureVerification(false),
		pubsub.WithMessageIdFn(msgIDFunction),
	}

	var gs *pubsub.PubSub
	if cfg.PubSub == "" {
		cfg.PubSub = pubsubGossip
	}
	if cfg.PubSub == pubsubFlood {
		gs, err = pubsub.NewFloodSub(s.ctx, s.host, psOpts...)
	} else if cfg.PubSub == pubsubGossip {
		gs, err = pubsub.NewGossipSub(s.ctx, s.host, psOpts...)
	} else if cfg.PubSub == pubsubRandom {
		gs, err = pubsub.NewRandomSub(s.ctx, s.host, psOpts...)
	} else {
		return nil, fmt.Errorf("unknown pubsub type %s", cfg.PubSub)
	}
	if err != nil {
		log.WithError(err).Error("Failed to start pubsub")
		return nil, err
	}
	s.pubsub = gs

	s.peers = peers.NewStatus(maxBadResponses)

	return s, nil
}

// Start the p2p service.
func (s *Service) Start() {
	if s.started {
		log.Error("Attempted to start p2p service when it was already started")
		return
	}

	// Waits until the state is initialized via an event feed.
	// Used for fork-related data when connecting peers.
	s.awaitStateInitialized()
	s.isPreGenesis = false

	var peersToWatch []string
	if s.cfg.RelayNodeAddr != "" {
		peersToWatch = append(peersToWatch, s.cfg.RelayNodeAddr)
		if err := dialRelayNode(s.ctx, s.host, s.cfg.RelayNodeAddr); err != nil {
			log.WithError(err).Errorf("Could not dial relay node")
		}
		peer, err := MakePeer(s.cfg.RelayNodeAddr)
		if err != nil {
			log.WithError(err).Errorf("Could not create peer")
		}
		s.host.ConnManager().Protect(peer.ID, "relay")
	}

	if !s.cfg.NoDiscovery && !s.cfg.DisableDiscv5 {
		ipAddr := ipAddr()
		listener, err := s.startDiscoveryV5(
			ipAddr,
			s.privKey,
		)
		if err != nil {
			log.WithError(err).Error("Failed to start discovery")
			s.startupErr = err
			return
		}
		err = s.connectToBootnodes()
		if err != nil {
			log.WithError(err).Error("Could not add bootnode to the exclusion list")
			s.startupErr = err
			return
		}
		s.dv5Listener = listener
		go s.listenForNewNodes()
	}

	if len(s.cfg.KademliaBootStrapAddr) != 0 && !s.cfg.NoDiscovery {
		for _, addr := range s.cfg.KademliaBootStrapAddr {
			peersToWatch = append(peersToWatch, addr)
			err := startDHTDiscovery(s.host, addr)
			if err != nil {
				log.WithError(err).Error("Could not connect to bootnode")
				s.startupErr = err
				return
			}
			if err := s.addKadDHTNodesToExclusionList(addr); err != nil {
				s.startupErr = err
				return
			}
			peer, err := MakePeer(addr)
			if err != nil {
				log.WithError(err).Errorf("Could not create peer")
			}
			s.host.ConnManager().Protect(peer.ID, "bootnode")
		}
		bcfg := kaddht.DefaultBootstrapConfig
		bcfg.Period = 30 * time.Second
		if err := s.dht.BootstrapWithConfig(s.ctx, bcfg); err != nil {
			log.WithError(err).Error("Failed to bootstrap DHT")
		}
	}

	s.started = true

	if len(s.cfg.StaticPeers) > 0 {
		addrs, err := peersFromStringAddrs(s.cfg.StaticPeers)
		if err != nil {
			log.Errorf("Could not connect to static peer: %v", err)
		}
		s.connectWithAllPeers(addrs)
	}

	// Periodic functions.
	runutil.RunEvery(s.ctx, 5*time.Second, func() {
		ensurePeerConnections(s.ctx, s.host, peersToWatch...)
	})
	runutil.RunEvery(s.ctx, time.Hour, s.Peers().Decay)
	runutil.RunEvery(s.ctx, 10*time.Second, s.updateMetrics)
	runutil.RunEvery(s.ctx, refreshRate, func() {
		s.RefreshENR()
	})

	multiAddrs := s.host.Network().ListenAddresses()
	logIPAddr(s.host.ID(), multiAddrs...)

	p2pHostAddress := s.cfg.HostAddress
	p2pTCPPort := s.cfg.TCPPort

	if p2pHostAddress != "" {
		logExternalIPAddr(s.host.ID(), p2pHostAddress, p2pTCPPort)
	}

	p2pHostDNS := s.cfg.HostDNS
	if p2pHostDNS != "" {
		logExternalDNSAddr(s.host.ID(), p2pHostDNS, p2pTCPPort)
	}
}

// Stop the p2p service and terminate all peer connections.
func (s *Service) Stop() error {
	defer s.cancel()
	s.started = false
	if s.dv5Listener != nil {
		s.dv5Listener.Close()
	}
	return nil
}

// Status of the p2p service. Will return an error if the service is considered unhealthy to
// indicate that this node should not serve traffic until the issue has been resolved.
func (s *Service) Status() error {
	if s.isPreGenesis {
		return nil
	}
	if !s.started {
		return errors.New("not running")
	}
	if s.startupErr != nil {
		return s.startupErr
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

// Connect to a specific peer.
func (s *Service) Connect(pi peer.AddrInfo) error {
	return s.host.Connect(s.ctx, pi)
}

// Peers returns the peer status interface.
func (s *Service) Peers() *peers.Status {
	return s.peers
}

// Metadata returns a copy of the peer's metadata.
func (s *Service) Metadata() *pb.MetaData {
	return proto.Clone(s.metaData).(*pb.MetaData)
}

// MetadataSeq returns the metadata sequence number.
func (s *Service) MetadataSeq() uint64 {
	return s.metaData.SeqNumber
}

// RefreshENR uses an epoch to refresh the enr entry for our node
// with the tracked committee id's for the epoch, allowing our node
// to be dynamically discoverable by others given our tracked committee id's.
func (s *Service) RefreshENR() {
	// return early if discv5 isnt running
	if s.dv5Listener == nil {
		return
	}
	bitV := bitfield.NewBitvector64()
	committees := cache.CommitteeIDs.GetAllCommittees()
	for _, idx := range committees {
		bitV.SetBitAt(idx, true)
	}
	currentBitV, err := retrieveBitvector(s.dv5Listener.Self().Record())
	if err != nil {
		log.Errorf("Could not retrieve bitfield: %v", err)
		return
	}
	if bytes.Equal(bitV, currentBitV) {
		// return early if bitfield hasn't changed
		return
	}
	s.updateSubnetRecordWithMetadata(bitV)
	// ping all peers to inform them of new metadata
	s.pingPeers()
}

// FindPeersWithSubnet performs a network search for peers
// subscribed to a particular subnet. Then we try to connect
// with those peers.
func (s *Service) FindPeersWithSubnet(index uint64) (bool, error) {
	nodes := make([]*enode.Node, searchLimit)
	if s.dv5Listener == nil {
		// return if discovery isn't set
		return false, nil
	}
	num := s.dv5Listener.ReadRandomNodes(nodes)
	exists := false
	for _, node := range nodes[:num] {
		if node.IP() == nil {
			continue
		}
		// do not look for nodes with no tcp port set
		if err := node.Record().Load(enr.WithEntry("tcp", new(enr.TCP))); err != nil {
			if !enr.IsNotFound(err) {
				log.WithError(err).Error("Could not retrieve tcp port")
			}
			continue
		}
		subnets, err := retrieveAttSubnets(node.Record())
		if err != nil {
			log.Errorf("could not retrieve subnets: %v", err)
			continue
		}
		for _, comIdx := range subnets {
			if comIdx == index {
				multiAddr, err := convertToSingleMultiAddr(node)
				if err != nil {
					return false, err
				}
				info, err := peer.AddrInfoFromP2pAddr(multiAddr)
				if err != nil {
					return false, err
				}
				if s.peers.IsActive(info.ID) {
					exists = true
					continue
				}
				if s.host.Network().Connectedness(info.ID) == network.Connected {
					exists = true
					continue
				}
				s.peers.Add(node.Record(), info.ID, multiAddr, network.DirUnknown)
				if err := s.connectWithPeer(*info); err != nil {
					log.WithError(err).Tracef("Could not connect with peer %s", info.String())
					continue
				}
				exists = true
			}
		}
	}
	return exists, nil
}

// AddPingMethod adds the metadata ping rpc method to the p2p service, so that it can
// be used to refresh ENR.
func (s *Service) AddPingMethod(reqFunc func(ctx context.Context, id peer.ID) error) {
	s.pingMethod = reqFunc
}

func (s *Service) pingPeers() {
	if s.pingMethod == nil {
		return
	}
	for _, pid := range s.peers.Connected() {
		go func(id peer.ID) {
			if err := s.pingMethod(s.ctx, id); err != nil {
				log.WithField("peer", id).WithError(err).Error("Failed to ping peer")
			}
		}(pid)
	}
}

// Waits for the beacon state to be initialized, important
// for initializing the p2p service as p2p needs to be aware
// of genesis information for peering.
func (s *Service) awaitStateInitialized() {
	stateChannel := make(chan *feed.Event, 1)
	stateSub := s.stateNotifier.StateFeed().Subscribe(stateChannel)
	defer stateSub.Unsubscribe()
	for {
		select {
		case event := <-stateChannel:
			if event.Type == statefeed.Initialized {
				data, ok := event.Data.(*statefeed.InitializedData)
				if !ok {
					log.Fatalf("Received wrong data over state initialized feed: %v", data)
				}
				s.genesisTime = data.StartTime
				s.genesisValidatorsRoot = data.GenesisValidatorsRoot
				return
			}
		}
	}
}

// listen for new nodes watches for new nodes in the network and adds them to the peerstore.
func (s *Service) listenForNewNodes() {
	runutil.RunEvery(s.ctx, pollingPeriod, func() {
		nodes := s.dv5Listener.LookupRandom()
		multiAddresses := s.processPeers(nodes)
		// do not process a large amount than required peers.
		if len(multiAddresses) > lookupLimit {
			multiAddresses = multiAddresses[:lookupLimit]
		}
		s.connectWithAllPeers(multiAddresses)
	})
}

func (s *Service) connectWithAllPeers(multiAddrs []ma.Multiaddr) {
	addrInfos, err := peer.AddrInfosFromP2pAddrs(multiAddrs...)
	if err != nil {
		log.Errorf("Could not convert to peer address info's from multiaddresses: %v", err)
		return
	}
	for _, info := range addrInfos {
		// make each dial non-blocking
		go func(info peer.AddrInfo) {
			if err := s.connectWithPeer(info); err != nil {
				log.WithError(err).Tracef("Could not connect with peer %s", info.String())
			}
		}(info)
	}
}

func (s *Service) connectWithPeer(info peer.AddrInfo) error {
	if len(s.Peers().Active()) >= int(s.cfg.MaxPeers) {
		log.WithFields(logrus.Fields{"peer": info.ID.String(),
			"reason": "at peer limit"}).Trace("Not dialing peer")
		return nil
	}
	if info.ID == s.host.ID() {
		return nil
	}
	if s.Peers().IsBad(info.ID) {
		return nil
	}
	if err := s.host.Connect(s.ctx, info); err != nil {
		s.Peers().IncrementBadResponses(info.ID)
		return err
	}
	return nil
}

// process new peers that come in from our dht.
func (s *Service) processPeers(nodes []*enode.Node) []ma.Multiaddr {
	var multiAddrs []ma.Multiaddr
	for _, node := range nodes {
		// ignore nodes with no ip address stored.
		if node.IP() == nil {
			continue
		}
		// do not dial nodes with their tcp ports not set
		if err := node.Record().Load(enr.WithEntry("tcp", new(enr.TCP))); err != nil {
			if !enr.IsNotFound(err) {
				log.WithError(err).Error("Could not retrieve tcp port")
			}
			continue
		}
		multiAddr, err := convertToSingleMultiAddr(node)
		if err != nil {
			log.WithError(err).Error("Could not convert to multiAddr")
			continue
		}
		peerData, err := peer.AddrInfoFromP2pAddr(multiAddr)
		if err != nil {
			log.WithError(err).Error("Could not get peer id")
			continue
		}
		if s.peers.IsBad(peerData.ID) {
			continue
		}
		if s.peers.IsActive(peerData.ID) {
			continue
		}
		if s.host.Network().Connectedness(peerData.ID) == network.Connected {
			continue
		}
		nodeENR := node.Record()
		// Decide whether or not to connect to peer that does not
		// match the proper fork ENR data with our local node.
		if s.genesisValidatorsRoot != nil {
			if err := s.compareForkENR(nodeENR); err != nil {
				log.WithError(err).Debug("Fork ENR mismatches between peer and local node")
				continue
			}
		}

		// Add peer to peer handler.
		s.peers.Add(nodeENR, peerData.ID, multiAddr, network.DirUnknown)
		multiAddrs = append(multiAddrs, multiAddr)
	}
	return multiAddrs
}

func (s *Service) connectToBootnodes() error {
	nodes := make([]*enode.Node, 0, len(s.cfg.Discv5BootStrapAddr))
	for _, addr := range s.cfg.Discv5BootStrapAddr {
		bootNode, err := enode.Parse(enode.ValidSchemes, addr)
		if err != nil {
			return err
		}
		// do not dial bootnodes with their tcp ports not set
		if err := bootNode.Record().Load(enr.WithEntry("tcp", new(enr.TCP))); err != nil {
			if !enr.IsNotFound(err) {
				log.WithError(err).Error("Could not retrieve tcp port")
			}
			continue
		}
		nodes = append(nodes, bootNode)
	}
	multiAddresses := convertToMultiAddr(nodes)
	s.connectWithAllPeers(multiAddresses)
	return nil
}

func (s *Service) addKadDHTNodesToExclusionList(addr string) error {
	multiAddr, err := ma.NewMultiaddr(addr)
	if err != nil {
		return errors.Wrap(err, "could not get multiaddr")
	}
	addrInfo, err := peer.AddrInfoFromP2pAddr(multiAddr)
	if err != nil {
		return err
	}
	// bootnode is never dialled, so ttl is tentatively 1 year
	s.exclusionList.Set(addrInfo.ID.String(), true, 1)
	return nil
}

// Updates the service's discv5 listener record's attestation subnet
// with a new value for a bitfield of subnets tracked. It also updates
// the node's metadata by increasing the sequence number and the
// subnets tracked by the node.
func (s *Service) updateSubnetRecordWithMetadata(bitV bitfield.Bitvector64) {
	entry := enr.WithEntry(attSubnetEnrKey, &bitV)
	s.dv5Listener.LocalNode().Set(entry)
	s.metaData = &pb.MetaData{
		SeqNumber: s.metaData.SeqNumber + 1,
		Attnets:   bitV,
	}
}

func logIPAddr(id peer.ID, addrs ...ma.Multiaddr) {
	var correctAddr ma.Multiaddr
	for _, addr := range addrs {
		if strings.Contains(addr.String(), "/ip4/") || strings.Contains(addr.String(), "/ip6/") {
			correctAddr = addr
			break
		}
	}
	if correctAddr != nil {
		log.WithField(
			"multiAddr",
			correctAddr.String()+"/p2p/"+id.String(),
		).Info("Node started p2p server")
	}
}

func logExternalIPAddr(id peer.ID, addr string, port uint) {
	if addr != "" {
		multiAddr, err := multiAddressBuilder(addr, port)
		if err != nil {
			log.Errorf("Could not create multiaddress: %v", err)
			return
		}
		log.WithField(
			"multiAddr",
			multiAddr.String()+"/p2p/"+id.String(),
		).Info("Node started external p2p server")
	}
}

func logExternalDNSAddr(id peer.ID, addr string, port uint) {
	if addr != "" {
		p := strconv.FormatUint(uint64(port), 10)

		log.WithField(
			"multiAddr",
			"/dns4/"+addr+"/tcp/"+p+"/p2p/"+id.String(),
		).Info("Node started external p2p server")
	}
}
