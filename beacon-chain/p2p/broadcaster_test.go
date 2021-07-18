package p2p

import (
	"context"
	"fmt"
	"net"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/p2p/discover"
	"github.com/libp2p/go-libp2p-core/host"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/prysmaticlabs/go-bitfield"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	"github.com/prysmaticlabs/prysm/beacon-chain/p2p/peers"
	"github.com/prysmaticlabs/prysm/beacon-chain/p2p/peers/scorers"
	p2ptest "github.com/prysmaticlabs/prysm/beacon-chain/p2p/testing"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	"github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1/wrapper"
	eth "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	v2 "github.com/prysmaticlabs/prysm/proto/prysm/v2"
	testpb "github.com/prysmaticlabs/prysm/proto/testing"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/prysmaticlabs/prysm/shared/testutil"
	"github.com/prysmaticlabs/prysm/shared/testutil/assert"
	"github.com/prysmaticlabs/prysm/shared/testutil/require"
	"google.golang.org/protobuf/proto"
)

func TestService_Broadcast(t *testing.T) {
	p1 := p2ptest.NewTestP2P(t)
	p2 := p2ptest.NewTestP2P(t)
	p1.Connect(p2)
	if len(p1.BHost.Network().Peers()) == 0 {
		t.Fatal("No peers")
	}

	p := &Service{
		host:                  p1.BHost,
		pubsub:                p1.PubSub(),
		joinedTopics:          map[string]*pubsub.Topic{},
		cfg:                   &Config{},
		genesisTime:           time.Now(),
		genesisValidatorsRoot: bytesutil.PadTo([]byte{'A'}, 32),
	}

	msg := &pb.Fork{
		Epoch:           55,
		CurrentVersion:  []byte("fooo"),
		PreviousVersion: []byte("barr"),
	}

	topic := "/eth2/%x/testing"
	// Set a test gossip mapping for testpb.TestSimpleMessage.
	GossipTypeMapping[reflect.TypeOf(msg)] = topic
	digest, err := p.currentForkDigest()
	require.NoError(t, err)
	topic = fmt.Sprintf(topic, digest)

	// External peer subscribes to the topic.
	topic += p.Encoding().ProtocolSuffix()
	sub, err := p2.SubscribeToTopic(topic)
	require.NoError(t, err)

	time.Sleep(50 * time.Millisecond) // libp2p fails without this delay...

	// Async listen for the pubsub, must be before the broadcast.
	var wg sync.WaitGroup
	wg.Add(1)
	go func(tt *testing.T) {
		defer wg.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()

		incomingMessage, err := sub.Next(ctx)
		require.NoError(t, err)

		result := &pb.Fork{}
		require.NoError(t, p.Encoding().DecodeGossip(incomingMessage.Data, result))
		if !proto.Equal(result, msg) {
			tt.Errorf("Did not receive expected message, got %+v, wanted %+v", result, msg)
		}
	}(t)

	// Broadcast to peers and wait.
	require.NoError(t, p.Broadcast(context.Background(), msg))
	if testutil.WaitTimeout(&wg, 1*time.Second) {
		t.Error("Failed to receive pubsub within 1s")
	}
}

func TestService_Broadcast_ReturnsErr_TopicNotMapped(t *testing.T) {
	p := Service{
		genesisTime:           time.Now(),
		genesisValidatorsRoot: bytesutil.PadTo([]byte{'A'}, 32),
	}
	assert.ErrorContains(t, ErrMessageNotMapped.Error(), p.Broadcast(context.Background(), &testpb.AddressBook{}))
}

func TestService_Attestation_Subnet(t *testing.T) {
	if gtm := GossipTypeMapping[reflect.TypeOf(&eth.Attestation{})]; gtm != AttestationSubnetTopicFormat {
		t.Errorf("Constant is out of date. Wanted %s, got %s", AttestationSubnetTopicFormat, gtm)
	}

	tests := []struct {
		att   *eth.Attestation
		topic string
	}{
		{
			att: &eth.Attestation{
				Data: &eth.AttestationData{
					CommitteeIndex: 0,
					Slot:           2,
				},
			},
			topic: "/eth2/00000000/beacon_attestation_2",
		},
		{
			att: &eth.Attestation{
				Data: &eth.AttestationData{
					CommitteeIndex: 11,
					Slot:           10,
				},
			},
			topic: "/eth2/00000000/beacon_attestation_21",
		},
		{
			att: &eth.Attestation{
				Data: &eth.AttestationData{
					CommitteeIndex: 55,
					Slot:           529,
				},
			},
			topic: "/eth2/00000000/beacon_attestation_8",
		},
	}
	for _, tt := range tests {
		subnet := helpers.ComputeSubnetFromCommitteeAndSlot(100, tt.att.Data.CommitteeIndex, tt.att.Data.Slot)
		assert.Equal(t, tt.topic, attestationToTopic(subnet, [4]byte{} /* fork digest */), "Wrong topic")
	}
}

func TestService_BroadcastAttestation(t *testing.T) {
	p1 := p2ptest.NewTestP2P(t)
	p2 := p2ptest.NewTestP2P(t)
	p1.Connect(p2)
	if len(p1.BHost.Network().Peers()) == 0 {
		t.Fatal("No peers")
	}

	p := &Service{
		host:                  p1.BHost,
		pubsub:                p1.PubSub(),
		joinedTopics:          map[string]*pubsub.Topic{},
		cfg:                   &Config{},
		genesisTime:           time.Now(),
		genesisValidatorsRoot: bytesutil.PadTo([]byte{'A'}, 32),
		subnetsLock:           make(map[uint64]*sync.RWMutex),
		subnetsLockLock:       sync.Mutex{},
		peers: peers.NewStatus(context.Background(), &peers.StatusConfig{
			ScorerParams: &scorers.Config{},
		}),
	}

	msg := testutil.HydrateAttestation(&eth.Attestation{AggregationBits: bitfield.NewBitlist(7)})
	subnet := uint64(5)

	topic := AttestationSubnetTopicFormat
	GossipTypeMapping[reflect.TypeOf(msg)] = topic
	digest, err := p.currentForkDigest()
	require.NoError(t, err)
	topic = fmt.Sprintf(topic, digest, subnet)

	// External peer subscribes to the topic.
	topic += p.Encoding().ProtocolSuffix()
	sub, err := p2.SubscribeToTopic(topic)
	require.NoError(t, err)

	time.Sleep(50 * time.Millisecond) // libp2p fails without this delay...

	// Async listen for the pubsub, must be before the broadcast.
	var wg sync.WaitGroup
	wg.Add(1)
	go func(tt *testing.T) {
		defer wg.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()

		incomingMessage, err := sub.Next(ctx)
		require.NoError(t, err)

		result := &eth.Attestation{}
		require.NoError(t, p.Encoding().DecodeGossip(incomingMessage.Data, result))
		if !proto.Equal(result, msg) {
			tt.Errorf("Did not receive expected message, got %+v, wanted %+v", result, msg)
		}
	}(t)

	// Broadcast to peers and wait.
	require.NoError(t, p.BroadcastAttestation(context.Background(), subnet, msg))
	if testutil.WaitTimeout(&wg, 1*time.Second) {
		t.Error("Failed to receive pubsub within 1s")
	}
}

func TestService_BroadcastAttestationWithDiscoveryAttempts(t *testing.T) {
	// Setup bootnode.
	cfg := &Config{}
	port := 2000
	cfg.UDPPort = uint(port)
	_, pkey := createAddrAndPrivKey(t)
	ipAddr := net.ParseIP("127.0.0.1")
	genesisTime := time.Now()
	genesisValidatorsRoot := make([]byte, 32)
	s := &Service{
		cfg:                   cfg,
		genesisTime:           genesisTime,
		genesisValidatorsRoot: genesisValidatorsRoot,
	}
	bootListener, err := s.createListener(ipAddr, pkey)
	require.NoError(t, err)
	defer bootListener.Close()

	// Use shorter period for testing.
	currentPeriod := pollingPeriod
	pollingPeriod = 1 * time.Second
	defer func() {
		pollingPeriod = currentPeriod
	}()

	bootNode := bootListener.Self()
	subnet := uint64(5)

	var listeners []*discover.UDPv5
	var hosts []host.Host
	// setup other nodes.
	cfg = &Config{
		BootstrapNodeAddr:   []string{bootNode.String()},
		Discv5BootStrapAddr: []string{bootNode.String()},
		MaxPeers:            30,
	}
	// Setup 2 different hosts
	for i := 1; i <= 2; i++ {
		h, pkey, ipAddr := createHost(t, port+i)
		cfg.UDPPort = uint(port + i)
		cfg.TCPPort = uint(port + i)
		s := &Service{
			cfg:                   cfg,
			genesisTime:           genesisTime,
			genesisValidatorsRoot: genesisValidatorsRoot,
		}
		listener, err := s.startDiscoveryV5(ipAddr, pkey)
		// Set for 2nd peer
		if i == 2 {
			s.dv5Listener = listener
			s.metaData = wrapper.WrappedMetadataV0(new(pb.MetaDataV0))
			bitV := bitfield.NewBitvector64()
			bitV.SetBitAt(subnet, true)
			s.updateSubnetRecordWithMetadata(bitV)
		}
		assert.NoError(t, err, "Could not start discovery for node")
		listeners = append(listeners, listener)
		hosts = append(hosts, h)
	}
	defer func() {
		// Close down all peers.
		for _, listener := range listeners {
			listener.Close()
		}
	}()

	// close peers upon exit of test
	defer func() {
		for _, h := range hosts {
			if err := h.Close(); err != nil {
				t.Log(err)
			}
		}
	}()

	ps1, err := pubsub.NewFloodSub(context.Background(), hosts[0],
		pubsub.WithMessageSigning(false),
		pubsub.WithStrictSignatureVerification(false),
	)
	require.NoError(t, err)

	ps2, err := pubsub.NewFloodSub(context.Background(), hosts[1],
		pubsub.WithMessageSigning(false),
		pubsub.WithStrictSignatureVerification(false),
	)
	require.NoError(t, err)
	p := &Service{
		host:                  hosts[0],
		ctx:                   context.Background(),
		pubsub:                ps1,
		dv5Listener:           listeners[0],
		joinedTopics:          map[string]*pubsub.Topic{},
		cfg:                   &Config{},
		genesisTime:           time.Now(),
		genesisValidatorsRoot: bytesutil.PadTo([]byte{'A'}, 32),
		subnetsLock:           make(map[uint64]*sync.RWMutex),
		subnetsLockLock:       sync.Mutex{},
		peers: peers.NewStatus(context.Background(), &peers.StatusConfig{
			ScorerParams: &scorers.Config{},
		}),
	}

	p2 := &Service{
		host:                  hosts[1],
		ctx:                   context.Background(),
		pubsub:                ps2,
		dv5Listener:           listeners[1],
		joinedTopics:          map[string]*pubsub.Topic{},
		cfg:                   &Config{},
		genesisTime:           time.Now(),
		genesisValidatorsRoot: bytesutil.PadTo([]byte{'A'}, 32),
		subnetsLock:           make(map[uint64]*sync.RWMutex),
		subnetsLockLock:       sync.Mutex{},
		peers: peers.NewStatus(context.Background(), &peers.StatusConfig{
			ScorerParams: &scorers.Config{},
		}),
	}

	msg := testutil.HydrateAttestation(&eth.Attestation{AggregationBits: bitfield.NewBitlist(7)})
	topic := AttestationSubnetTopicFormat
	GossipTypeMapping[reflect.TypeOf(msg)] = topic
	digest, err := p.currentForkDigest()
	require.NoError(t, err)
	topic = fmt.Sprintf(topic, digest, subnet)

	// External peer subscribes to the topic.
	topic += p.Encoding().ProtocolSuffix()
	// We dont use our internal subscribe method
	// due to using floodsub over here.
	tpHandle, err := p2.JoinTopic(topic)
	require.NoError(t, err)
	sub, err := tpHandle.Subscribe()
	require.NoError(t, err)

	time.Sleep(50 * time.Millisecond) // libp2p fails without this delay...

	// Async listen for the pubsub, must be before the broadcast.
	var wg sync.WaitGroup
	wg.Add(1)
	go func(tt *testing.T) {
		defer wg.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
		defer cancel()

		incomingMessage, err := sub.Next(ctx)
		require.NoError(t, err)

		result := &eth.Attestation{}
		require.NoError(t, p.Encoding().DecodeGossip(incomingMessage.Data, result))
		if !proto.Equal(result, msg) {
			tt.Errorf("Did not receive expected message, got %+v, wanted %+v", result, msg)
		}
	}(t)

	// Broadcast to peers and wait.
	require.NoError(t, p.BroadcastAttestation(context.Background(), subnet, msg))
	if testutil.WaitTimeout(&wg, 4*time.Second) {
		t.Error("Failed to receive pubsub within 4s")
	}
}

func TestService_BroadcastSyncCommittee(t *testing.T) {
	p1 := p2ptest.NewTestP2P(t)
	p2 := p2ptest.NewTestP2P(t)
	p1.Connect(p2)
	if len(p1.BHost.Network().Peers()) == 0 {
		t.Fatal("No peers")
	}

	p := &Service{
		host:                  p1.BHost,
		pubsub:                p1.PubSub(),
		joinedTopics:          map[string]*pubsub.Topic{},
		cfg:                   &Config{},
		genesisTime:           time.Now(),
		genesisValidatorsRoot: bytesutil.PadTo([]byte{'A'}, 32),
		subnetsLock:           make(map[uint64]*sync.RWMutex),
		subnetsLockLock:       sync.Mutex{},
		peers: peers.NewStatus(context.Background(), &peers.StatusConfig{
			ScorerParams: &scorers.Config{},
		}),
	}

	msg := testutil.HydrateSyncCommittee(&v2.SyncCommitteeMessage{})
	subnet := uint64(5)

	topic := SyncCommitteeSubnetTopicFormat
	GossipTypeMapping[reflect.TypeOf(msg)] = topic
	digest, err := p.currentForkDigest()
	require.NoError(t, err)
	topic = fmt.Sprintf(topic, digest, subnet)

	// External peer subscribes to the topic.
	topic += p.Encoding().ProtocolSuffix()
	sub, err := p2.SubscribeToTopic(topic)
	require.NoError(t, err)

	time.Sleep(50 * time.Millisecond) // libp2p fails without this delay...

	// Async listen for the pubsub, must be before the broadcast.
	var wg sync.WaitGroup
	wg.Add(1)
	go func(tt *testing.T) {
		defer wg.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()

		incomingMessage, err := sub.Next(ctx)
		require.NoError(t, err)

		result := &v2.SyncCommitteeMessage{}
		require.NoError(t, p.Encoding().DecodeGossip(incomingMessage.Data, result))
		if !proto.Equal(result, msg) {
			tt.Errorf("Did not receive expected message, got %+v, wanted %+v", result, msg)
		}
	}(t)

	// Broadcast to peers and wait.
	require.NoError(t, p.BroadcastSyncCommitteeMessage(context.Background(), subnet, msg))
	if testutil.WaitTimeout(&wg, 1*time.Second) {
		t.Error("Failed to receive pubsub within 1s")
	}
}
