package testing

import (
	"context"

	"github.com/ethereum/go-ethereum/p2p/enr"
	"github.com/libp2p/go-libp2p-core/control"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/multiformats/go-multiaddr"
	"github.com/prysmaticlabs/prysm/proto/beacon/p2p"
	ethpb "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	prysmv2 "github.com/prysmaticlabs/prysm/proto/prysm/v2"
	"google.golang.org/protobuf/proto"

	"github.com/prysmaticlabs/prysm/beacon-chain/p2p/encoder"
	"github.com/prysmaticlabs/prysm/beacon-chain/p2p/peers"
)

// FakeP2P stack
type FakeP2P struct {
}

// NewFuzzTestP2P - Create a new fake p2p stack.
func NewFuzzTestP2P() *FakeP2P {
	return &FakeP2P{}
}

// Encoding -- fake.
func (p *FakeP2P) Encoding() encoder.NetworkEncoding {
	return &encoder.SszNetworkEncoder{}
}

// AddConnectionHandler -- fake.
func (p *FakeP2P) AddConnectionHandler(_, _ func(ctx context.Context, id peer.ID) error) {

}

// AddDisconnectionHandler -- fake.
func (p *FakeP2P) AddDisconnectionHandler(_ func(ctx context.Context, id peer.ID) error) {
}

// AddPingMethod -- fake.
func (p *FakeP2P) AddPingMethod(_ func(ctx context.Context, id peer.ID) error) {

}

// PeerID -- fake.
func (p *FakeP2P) PeerID() peer.ID {
	return "fake"
}

// ENR returns the enr of the local peer.
func (p *FakeP2P) ENR() *enr.Record {
	return new(enr.Record)
}

// DiscoveryAddresses -- fake
func (p *FakeP2P) DiscoveryAddresses() ([]multiaddr.Multiaddr, error) {
	return nil, nil
}

// FindPeersWithSubnet mocks the p2p func.
func (p *FakeP2P) FindPeersWithSubnet(_ context.Context, _ string, _, _ uint64) (bool, error) {
	return false, nil
}

// RefreshENR mocks the p2p func.
func (p *FakeP2P) RefreshENR() {}

// LeaveTopic -- fake.
func (p *FakeP2P) LeaveTopic(_ string) error {
	return nil
}

// Metadata -- fake.
func (p *FakeP2P) Metadata() p2p.Metadata {
	return nil
}

// Peers -- fake.
func (p *FakeP2P) Peers() *peers.Status {
	return nil
}

// PublishToTopic -- fake.
func (p *FakeP2P) PublishToTopic(_ context.Context, _ string, _ []byte, _ ...pubsub.PubOpt) error {
	return nil
}

// Send -- fake.
func (p *FakeP2P) Send(_ context.Context, _ interface{}, _ string, _ peer.ID) (network.Stream, error) {
	return nil, nil
}

// PubSub -- fake.
func (p *FakeP2P) PubSub() *pubsub.PubSub {
	return nil
}

// MetadataSeq -- fake.
func (p *FakeP2P) MetadataSeq() uint64 {
	return 0
}

// SetStreamHandler -- fake.
func (p *FakeP2P) SetStreamHandler(_ string, _ network.StreamHandler) {

}

// SubscribeToTopic -- fake.
func (p *FakeP2P) SubscribeToTopic(_ string, _ ...pubsub.SubOpt) (*pubsub.Subscription, error) {
	return nil, nil
}

// JoinTopic -- fake.
func (p *FakeP2P) JoinTopic(_ string, _ ...pubsub.TopicOpt) (*pubsub.Topic, error) {
	return nil, nil
}

// Host -- fake.
func (p *FakeP2P) Host() host.Host {
	return nil
}

// Disconnect -- fake.
func (p *FakeP2P) Disconnect(_ peer.ID) error {
	return nil
}

// Broadcast -- fake.
func (p *FakeP2P) Broadcast(_ context.Context, _ proto.Message) error {
	return nil
}

// BroadcastAttestation -- fake.
func (p *FakeP2P) BroadcastAttestation(_ context.Context, _ uint64, _ *ethpb.Attestation) error {
	return nil
}

// BroadcastSyncCommitteeMessage -- fake.
func (b *FakeP2P) BroadcastSyncCommitteeMessage(_ context.Context, _ uint64, _ *prysmv2.SyncCommitteeMessage) error {
	return nil
}

// InterceptPeerDial -- fake.
func (p *FakeP2P) InterceptPeerDial(peer.ID) (allow bool) {
	return true
}

// InterceptAddrDial -- fake.
func (p *FakeP2P) InterceptAddrDial(peer.ID, multiaddr.Multiaddr) (allow bool) {
	return true
}

// InterceptAccept -- fake.
func (p *FakeP2P) InterceptAccept(_ network.ConnMultiaddrs) (allow bool) {
	return true
}

// InterceptSecured -- fake.
func (p *FakeP2P) InterceptSecured(network.Direction, peer.ID, network.ConnMultiaddrs) (allow bool) {
	return true
}

// InterceptUpgraded -- fake.
func (p *FakeP2P) InterceptUpgraded(network.Conn) (allow bool, reason control.DisconnectReason) {
	return true, 0
}
