package sync

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/kevinms/leakybucket-go"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/protocol"
	types "github.com/prysmaticlabs/eth2-types"
	mock "github.com/prysmaticlabs/prysm/beacon-chain/blockchain/testing"
	db "github.com/prysmaticlabs/prysm/beacon-chain/db/testing"
	p2ptest "github.com/prysmaticlabs/prysm/beacon-chain/p2p/testing"
	p2ptypes "github.com/prysmaticlabs/prysm/beacon-chain/p2p/types"
	"github.com/prysmaticlabs/prysm/shared/testutil"
	"github.com/prysmaticlabs/prysm/shared/testutil/assert"
	"github.com/prysmaticlabs/prysm/shared/testutil/require"
)

func TestGoodByeRPCHandler_Disconnects_With_Peer(t *testing.T) {
	p1 := p2ptest.NewTestP2P(t)
	p2 := p2ptest.NewTestP2P(t)
	p1.Connect(p2)
	assert.Equal(t, 1, len(p1.BHost.Network().Peers()), "Expected peers to be connected")

	// Set up a head state in the database with data we expect.
	d := db.SetupDB(t)
	r := &Service{
		cfg: &Config{
			DB:  d,
			P2P: p1,
		},
		rateLimiter: newRateLimiter(p1),
	}

	// Setup streams
	pcl := protocol.ID("/testing")
	topic := string(pcl)
	r.rateLimiter.limiterMap[topic] = leakybucket.NewCollector(1, 1, false)
	var wg sync.WaitGroup
	wg.Add(1)
	p2.BHost.SetStreamHandler(pcl, func(stream network.Stream) {
		defer wg.Done()
		expectResetStream(t, stream)
	})
	stream1, err := p1.BHost.NewStream(context.Background(), p2.BHost.ID(), pcl)
	require.NoError(t, err)
	failureCode := p2ptypes.GoodbyeCodeClientShutdown

	assert.NoError(t, r.goodbyeRPCHandler(context.Background(), &failureCode, stream1))

	if testutil.WaitTimeout(&wg, 1*time.Second) {
		t.Fatal("Did not receive stream within 1 sec")
	}

	conns := p1.BHost.Network().ConnsToPeer(p2.BHost.ID())
	if len(conns) > 0 {
		t.Error("Peer is still not disconnected despite sending a goodbye message")
	}
}

func TestGoodByeRPCHandler_BackOffPeer(t *testing.T) {
	p1 := p2ptest.NewTestP2P(t)
	p2 := p2ptest.NewTestP2P(t)
	p3 := p2ptest.NewTestP2P(t)

	p1.Connect(p2)
	p1.Connect(p3)
	assert.Equal(t, 2, len(p1.BHost.Network().Peers()), "Expected peers to be connected")

	// Set up a head state in the database with data we expect.
	d := db.SetupDB(t)
	r := &Service{
		cfg: &Config{
			DB:  d,
			P2P: p1,
		},
		rateLimiter: newRateLimiter(p1),
	}

	// Setup streams
	pcl := protocol.ID("/testing")
	topic := string(pcl)
	r.rateLimiter.limiterMap[topic] = leakybucket.NewCollector(1, 1, false)
	var wg sync.WaitGroup
	wg.Add(1)
	p2.BHost.SetStreamHandler(pcl, func(stream network.Stream) {
		defer wg.Done()
		expectResetStream(t, stream)
	})
	stream1, err := p1.BHost.NewStream(context.Background(), p2.BHost.ID(), pcl)
	require.NoError(t, err)
	failureCode := p2ptypes.GoodbyeCodeClientShutdown

	assert.NoError(t, r.goodbyeRPCHandler(context.Background(), &failureCode, stream1))

	if testutil.WaitTimeout(&wg, 1*time.Second) {
		t.Fatal("Did not receive stream within 1 sec")
	}

	conns := p1.BHost.Network().ConnsToPeer(p2.BHost.ID())
	if len(conns) > 0 {
		t.Error("Peer is still not disconnected despite sending a goodbye message")
	}
	valTime, err := p1.Peers().NextValidTime(p2.BHost.ID())
	require.NoError(t, err)
	expectedTime := time.Now().Add(backOffTime[failureCode])
	diff := expectedTime.Sub(valTime)
	// Add a little bit of allowance
	require.Equal(t, true, diff.Seconds() <= 1)

	wg.Add(1)
	p3.BHost.SetStreamHandler(pcl, func(stream network.Stream) {
		defer wg.Done()
		expectResetStream(t, stream)
	})

	stream2, err := p1.BHost.NewStream(context.Background(), p3.BHost.ID(), pcl)
	require.NoError(t, err)
	failureCode = p2ptypes.GoodbyeCodeBanned

	assert.NoError(t, r.goodbyeRPCHandler(context.Background(), &failureCode, stream2))

	if testutil.WaitTimeout(&wg, 1*time.Second) {
		t.Fatal("Did not receive stream within 1 sec")
	}

	conns = p1.BHost.Network().ConnsToPeer(p3.BHost.ID())
	if len(conns) > 0 {
		t.Error("Peer is still not disconnected despite sending a goodbye message")
	}
	valTime, err = p1.Peers().NextValidTime(p3.BHost.ID())
	require.NoError(t, err)
	expectedTime = time.Now().Add(backOffTime[failureCode])
	diff = expectedTime.Sub(valTime)
	// Add a little bit of allowance
	require.Equal(t, true, diff.Seconds() <= 1)
}

func TestSendGoodbye_SendsMessage(t *testing.T) {
	p1 := p2ptest.NewTestP2P(t)
	p2 := p2ptest.NewTestP2P(t)
	p1.Connect(p2)
	assert.Equal(t, 1, len(p1.BHost.Network().Peers()), "Expected peers to be connected")

	// Set up a head state in the database with data we expect.
	d := db.SetupDB(t)
	r := &Service{
		cfg: &Config{
			DB:    d,
			P2P:   p1,
			Chain: &mock.ChainService{ValidatorsRoot: [32]byte{}, Genesis: time.Now()},
		},
		rateLimiter: newRateLimiter(p1),
	}
	failureCode := p2ptypes.GoodbyeCodeClientShutdown

	// Setup streams
	pcl := protocol.ID("/eth2/beacon_chain/req/goodbye/1/ssz_snappy")
	topic := string(pcl)
	r.rateLimiter.limiterMap[topic] = leakybucket.NewCollector(1, 1, false)
	var wg sync.WaitGroup
	wg.Add(1)
	p2.BHost.SetStreamHandler(pcl, func(stream network.Stream) {
		defer wg.Done()
		out := new(types.SSZUint64)
		assert.NoError(t, r.cfg.P2P.Encoding().DecodeWithMaxLength(stream, out))
		assert.Equal(t, failureCode, *out)
		assert.NoError(t, stream.Close())
	})

	err := r.sendGoodByeMessage(context.Background(), failureCode, p2.BHost.ID())
	assert.NoError(t, err)

	if testutil.WaitTimeout(&wg, 1*time.Second) {
		t.Fatal("Did not receive stream within 1 sec")
	}

	conns := p1.BHost.Network().ConnsToPeer(p1.BHost.ID())
	if len(conns) > 0 {
		t.Error("Peer is still not disconnected despite sending a goodbye message")
	}
}

func TestSendGoodbye_DisconnectWithPeer(t *testing.T) {
	p1 := p2ptest.NewTestP2P(t)
	p2 := p2ptest.NewTestP2P(t)
	p1.Connect(p2)
	assert.Equal(t, 1, len(p1.BHost.Network().Peers()), "Expected peers to be connected")

	// Set up a head state in the database with data we expect.
	d := db.SetupDB(t)
	r := &Service{
		cfg: &Config{
			DB:    d,
			P2P:   p1,
			Chain: &mock.ChainService{Genesis: time.Now(), ValidatorsRoot: [32]byte{}},
		},
		rateLimiter: newRateLimiter(p1),
	}
	failureCode := p2ptypes.GoodbyeCodeClientShutdown

	// Setup streams
	pcl := protocol.ID("/eth2/beacon_chain/req/goodbye/1/ssz_snappy")
	topic := string(pcl)
	r.rateLimiter.limiterMap[topic] = leakybucket.NewCollector(1, 1, false)
	var wg sync.WaitGroup
	wg.Add(1)
	p2.BHost.SetStreamHandler(pcl, func(stream network.Stream) {
		defer wg.Done()
		out := new(types.SSZUint64)
		assert.NoError(t, r.cfg.P2P.Encoding().DecodeWithMaxLength(stream, out))
		assert.Equal(t, failureCode, *out)
		assert.NoError(t, stream.Close())
	})

	assert.NoError(t, r.sendGoodByeAndDisconnect(context.Background(), failureCode, p2.BHost.ID()))
	conns := p1.BHost.Network().ConnsToPeer(p2.BHost.ID())
	if len(conns) > 0 {
		t.Error("Peer is still not disconnected despite sending a goodbye message")
	}

	if testutil.WaitTimeout(&wg, 1*time.Second) {
		t.Fatal("Did not receive stream within 1 sec")
	}

}
