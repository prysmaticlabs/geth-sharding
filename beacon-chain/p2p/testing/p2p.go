// Package testing includes useful utilities for mocking
// a beacon node's p2p service for unit tests.
package testing

import (
	"bytes"
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/p2p/enr"
	ssz "github.com/ferranbt/fastssz"
	bhost "github.com/libp2p/go-libp2p-blankhost"
	core "github.com/libp2p/go-libp2p-core"
	"github.com/libp2p/go-libp2p-core/control"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	swarmt "github.com/libp2p/go-libp2p-swarm/testing"
	"github.com/multiformats/go-multiaddr"
	"github.com/prysmaticlabs/prysm/beacon-chain/p2p/encoder"
	"github.com/prysmaticlabs/prysm/beacon-chain/p2p/peers"
	"github.com/prysmaticlabs/prysm/beacon-chain/p2p/peers/scorers"
	"github.com/prysmaticlabs/prysm/proto/beacon/p2p"
	ethpb "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	prysmv2 "github.com/prysmaticlabs/prysm/proto/prysm/v2"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/proto"
)

// TestP2P represents a p2p implementation that can be used for testing.
type TestP2P struct {
	t               *testing.T
	BHost           host.Host
	pubsub          *pubsub.PubSub
	joinedTopics    map[string]*pubsub.Topic
	BroadcastCalled bool
	DelaySend       bool
	Digest          [4]byte
	peers           *peers.Status
	LocalMetadata   p2p.Metadata
}

// NewTestP2P initializes a new p2p test service.
func NewTestP2P(t *testing.T) *TestP2P {
	ctx := context.Background()
	h := bhost.NewBlankHost(swarmt.GenSwarm(t, ctx))
	ps, err := pubsub.NewFloodSub(ctx, h,
		pubsub.WithMessageSigning(false),
		pubsub.WithStrictSignatureVerification(false),
	)
	if err != nil {
		t.Fatal(err)
	}

	peerStatuses := peers.NewStatus(context.Background(), &peers.StatusConfig{
		PeerLimit: 30,
		ScorerParams: &scorers.Config{
			BadResponsesScorerConfig: &scorers.BadResponsesScorerConfig{
				Threshold: 5,
			},
		},
	})
	return &TestP2P{
		t:            t,
		BHost:        h,
		pubsub:       ps,
		joinedTopics: map[string]*pubsub.Topic{},
		peers:        peerStatuses,
	}
}

// Connect two test peers together.
func (p *TestP2P) Connect(b *TestP2P) {
	if err := connect(p.BHost, b.BHost); err != nil {
		p.t.Fatal(err)
	}
}

func connect(a, b host.Host) error {
	pinfo := b.Peerstore().PeerInfo(b.ID())
	return a.Connect(context.Background(), pinfo)
}

// ReceiveRPC simulates an incoming RPC.
func (p *TestP2P) ReceiveRPC(topic string, msg proto.Message) {
	h := bhost.NewBlankHost(swarmt.GenSwarm(p.t, context.Background()))
	if err := connect(h, p.BHost); err != nil {
		p.t.Fatalf("Failed to connect two peers for RPC: %v", err)
	}
	s, err := h.NewStream(context.Background(), p.BHost.ID(), protocol.ID(topic+p.Encoding().ProtocolSuffix()))
	if err != nil {
		p.t.Fatalf("Failed to open stream %v", err)
	}
	defer func() {
		if err := s.Close(); err != nil {
			p.t.Log(err)
		}
	}()

	castedMsg, ok := msg.(ssz.Marshaler)
	if !ok {
		p.t.Fatalf("%T doesnt support ssz marshaler", msg)
	}
	n, err := p.Encoding().EncodeWithMaxLength(s, castedMsg)
	if err != nil {
		_err := s.Reset()
		_ = _err
		p.t.Fatalf("Failed to encode message: %v", err)
	}

	p.t.Logf("Wrote %d bytes", n)
}

// ReceivePubSub simulates an incoming message over pubsub on a given topic.
func (p *TestP2P) ReceivePubSub(topic string, msg proto.Message) {
	h := bhost.NewBlankHost(swarmt.GenSwarm(p.t, context.Background()))
	ps, err := pubsub.NewFloodSub(context.Background(), h,
		pubsub.WithMessageSigning(false),
		pubsub.WithStrictSignatureVerification(false),
	)
	if err != nil {
		p.t.Fatalf("Failed to create flood sub: %v", err)
	}
	if err := connect(h, p.BHost); err != nil {
		p.t.Fatalf("Failed to connect two peers for RPC: %v", err)
	}

	// PubSub requires some delay after connecting for the (*PubSub).processLoop method to
	// pick up the newly connected peer.
	time.Sleep(time.Millisecond * 100)

	castedMsg, ok := msg.(ssz.Marshaler)
	if !ok {
		p.t.Fatalf("%T doesnt support ssz marshaler", msg)
	}
	buf := new(bytes.Buffer)
	if _, err := p.Encoding().EncodeGossip(buf, castedMsg); err != nil {
		p.t.Fatalf("Failed to encode message: %v", err)
	}
	digest, err := p.ForkDigest()
	if err != nil {
		p.t.Fatal(err)
	}
	topicHandle, err := ps.Join(fmt.Sprintf(topic, digest) + p.Encoding().ProtocolSuffix())
	if err != nil {
		p.t.Fatal(err)
	}
	if err := topicHandle.Publish(context.TODO(), buf.Bytes()); err != nil {
		p.t.Fatalf("Failed to publish message; %v", err)
	}
}

// Broadcast a message.
func (p *TestP2P) Broadcast(_ context.Context, _ proto.Message) error {
	p.BroadcastCalled = true
	return nil
}

// BroadcastAttestation broadcasts an attestation.
func (p *TestP2P) BroadcastAttestation(_ context.Context, _ uint64, _ *ethpb.Attestation) error {
	p.BroadcastCalled = true
	return nil
}

// BroadcastSyncCommitteeMessage broadcasts a sync committee message.
func (p *TestP2P) BroadcastSyncCommitteeMessage(_ context.Context, _ uint64, _ *prysmv2.SyncCommitteeMessage) error {
	p.BroadcastCalled = true
	return nil
}

// SetStreamHandler for RPC.
func (p *TestP2P) SetStreamHandler(topic string, handler network.StreamHandler) {
	p.BHost.SetStreamHandler(protocol.ID(topic), handler)
}

// JoinTopic will join PubSub topic, if not already joined.
func (p *TestP2P) JoinTopic(topic string, opts ...pubsub.TopicOpt) (*pubsub.Topic, error) {
	if _, ok := p.joinedTopics[topic]; !ok {
		joinedTopic, err := p.pubsub.Join(topic, opts...)
		if err != nil {
			return nil, err
		}
		p.joinedTopics[topic] = joinedTopic
	}

	return p.joinedTopics[topic], nil
}

// PublishToTopic publishes message to previously joined topic.
func (p *TestP2P) PublishToTopic(ctx context.Context, topic string, data []byte, opts ...pubsub.PubOpt) error {
	joinedTopic, err := p.JoinTopic(topic)
	if err != nil {
		return err
	}
	return joinedTopic.Publish(ctx, data, opts...)
}

// SubscribeToTopic joins (if necessary) and subscribes to PubSub topic.
func (p *TestP2P) SubscribeToTopic(topic string, opts ...pubsub.SubOpt) (*pubsub.Subscription, error) {
	joinedTopic, err := p.JoinTopic(topic)
	if err != nil {
		return nil, err
	}
	return joinedTopic.Subscribe(opts...)
}

// LeaveTopic closes topic and removes corresponding handler from list of joined topics.
// This method will return error if there are outstanding event handlers or subscriptions.
func (p *TestP2P) LeaveTopic(topic string) error {
	if t, ok := p.joinedTopics[topic]; ok {
		if err := t.Close(); err != nil {
			return err
		}
		delete(p.joinedTopics, topic)
	}
	return nil
}

// Encoding returns ssz encoding.
func (p *TestP2P) Encoding() encoder.NetworkEncoding {
	return &encoder.SszNetworkEncoder{}
}

// PubSub returns reference underlying floodsub. This test library uses floodsub
// to ensure all connected peers receive the message.
func (p *TestP2P) PubSub() *pubsub.PubSub {
	return p.pubsub
}

// Disconnect from a peer.
func (p *TestP2P) Disconnect(pid peer.ID) error {
	return p.BHost.Network().ClosePeer(pid)
}

// PeerID returns the Peer ID of the local peer.
func (p *TestP2P) PeerID() peer.ID {
	return p.BHost.ID()
}

// Host returns the libp2p host of the
// local peer.
func (p *TestP2P) Host() host.Host {
	return p.BHost
}

// ENR returns the enr of the local peer.
func (p *TestP2P) ENR() *enr.Record {
	return new(enr.Record)
}

// DiscoveryAddresses --
func (p *TestP2P) DiscoveryAddresses() ([]multiaddr.Multiaddr, error) {
	return nil, nil
}

// AddConnectionHandler handles the connection with a newly connected peer.
func (p *TestP2P) AddConnectionHandler(f, _ func(ctx context.Context, id peer.ID) error) {
	p.BHost.Network().Notify(&network.NotifyBundle{
		ConnectedF: func(net network.Network, conn network.Conn) {
			// Must be handled in a goroutine as this callback cannot be blocking.
			go func() {
				p.peers.Add(new(enr.Record), conn.RemotePeer(), conn.RemoteMultiaddr(), conn.Stat().Direction)
				ctx := context.Background()

				p.peers.SetConnectionState(conn.RemotePeer(), peers.PeerConnecting)
				if err := f(ctx, conn.RemotePeer()); err != nil {
					logrus.WithError(err).Error("Could not send succesful hello rpc request")
					if err := p.Disconnect(conn.RemotePeer()); err != nil {
						logrus.WithError(err).Errorf("Unable to close peer %s", conn.RemotePeer())
					}
					p.peers.SetConnectionState(conn.RemotePeer(), peers.PeerDisconnected)
					return
				}
				p.peers.SetConnectionState(conn.RemotePeer(), peers.PeerConnected)
			}()
		},
	})
}

// AddDisconnectionHandler --
func (p *TestP2P) AddDisconnectionHandler(f func(ctx context.Context, id peer.ID) error) {
	p.BHost.Network().Notify(&network.NotifyBundle{
		DisconnectedF: func(net network.Network, conn network.Conn) {
			// Must be handled in a goroutine as this callback cannot be blocking.
			go func() {
				p.peers.SetConnectionState(conn.RemotePeer(), peers.PeerDisconnecting)
				if err := f(context.Background(), conn.RemotePeer()); err != nil {
					logrus.WithError(err).Debug("Unable to invoke callback")
				}
				p.peers.SetConnectionState(conn.RemotePeer(), peers.PeerDisconnected)
			}()
		},
	})
}

// Send a message to a specific peer.
func (p *TestP2P) Send(ctx context.Context, msg interface{}, topic string, pid peer.ID) (network.Stream, error) {
	t := topic
	if t == "" {
		return nil, fmt.Errorf("protocol doesnt exist for proto message: %v", msg)
	}
	stream, err := p.BHost.NewStream(ctx, pid, core.ProtocolID(t+p.Encoding().ProtocolSuffix()))
	if err != nil {
		return nil, err
	}

	if topic != "/eth2/beacon_chain/req/metadata/1" && topic != "/eth2/beacon_chain/req/metadata/2" {
		castedMsg, ok := msg.(ssz.Marshaler)
		if !ok {
			p.t.Fatalf("%T doesnt support ssz marshaler", msg)
		}
		if _, err := p.Encoding().EncodeWithMaxLength(stream, castedMsg); err != nil {
			_err := stream.Reset()
			_ = _err
			return nil, err
		}
	}

	// Close stream for writing.
	if err := stream.CloseWrite(); err != nil {
		_err := stream.Reset()
		_ = _err
		return nil, err
	}
	// Delay returning the stream for testing purposes
	if p.DelaySend {
		time.Sleep(1 * time.Second)
	}

	return stream, nil
}

// Started always returns true.
func (p *TestP2P) Started() bool {
	return true
}

// Peers returns the peer status.
func (p *TestP2P) Peers() *peers.Status {
	return p.peers
}

// FindPeersWithSubnet mocks the p2p func.
func (p *TestP2P) FindPeersWithSubnet(_ context.Context, _ string, _, _ uint64) (bool, error) {
	return false, nil
}

// RefreshENR mocks the p2p func.
func (p *TestP2P) RefreshENR() {}

// ForkDigest mocks the p2p func.
func (p *TestP2P) ForkDigest() ([4]byte, error) {
	return p.Digest, nil
}

// Metadata mocks the peer's metadata.
func (p *TestP2P) Metadata() p2p.Metadata {
	return p.LocalMetadata.Copy()
}

// MetadataSeq mocks metadata sequence number.
func (p *TestP2P) MetadataSeq() uint64 {
	return p.LocalMetadata.SequenceNumber()
}

// AddPingMethod mocks the p2p func.
func (p *TestP2P) AddPingMethod(_ func(ctx context.Context, id peer.ID) error) {
	// no-op
}

// InterceptPeerDial .
func (p *TestP2P) InterceptPeerDial(peer.ID) (allow bool) {
	return true
}

// InterceptAddrDial .
func (p *TestP2P) InterceptAddrDial(peer.ID, multiaddr.Multiaddr) (allow bool) {
	return true
}

// InterceptAccept .
func (p *TestP2P) InterceptAccept(_ network.ConnMultiaddrs) (allow bool) {
	return true
}

// InterceptSecured .
func (p *TestP2P) InterceptSecured(network.Direction, peer.ID, network.ConnMultiaddrs) (allow bool) {
	return true
}

// InterceptUpgraded .
func (p *TestP2P) InterceptUpgraded(network.Conn) (allow bool, reason control.DisconnectReason) {
	return true, 0
}
