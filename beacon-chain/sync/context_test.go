package sync

import (
	"context"
	"sync"
	"testing"
	"time"

	core "github.com/libp2p/go-libp2p-core"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/prysmaticlabs/prysm/beacon-chain/p2p"
	p2ptest "github.com/prysmaticlabs/prysm/beacon-chain/p2p/testing"
	"github.com/prysmaticlabs/prysm/shared/testutil"
	"github.com/prysmaticlabs/prysm/shared/testutil/assert"
)

func TestContextWrite_NoWrites(t *testing.T) {
	p1 := p2ptest.NewTestP2P(t)
	nPeer := p2ptest.NewTestP2P(t)
	p1.Connect(nPeer)

	wg := new(sync.WaitGroup)
	prID := p2p.RPCPingTopicV1
	wg.Add(1)
	nPeer.BHost.SetStreamHandler(core.ProtocolID(prID), func(stream network.Stream) {
		wg.Done()
		// no-op
	})
	strm, err := p1.BHost.NewStream(context.Background(), nPeer.PeerID(), p2p.RPCPingTopicV1)
	assert.NoError(t, err)

	// Nothing will be written to the stream
	assert.NoError(t, writeContextToStream(nil, strm, nil))
	if testutil.WaitTimeout(wg, 1*time.Second) {
		t.Fatal("Did not receive stream within 1 sec")
	}
}

func TestContextRead_NoReads(t *testing.T) {
	p1 := p2ptest.NewTestP2P(t)
	nPeer := p2ptest.NewTestP2P(t)
	p1.Connect(nPeer)

	wg := new(sync.WaitGroup)
	prID := p2p.RPCPingTopicV1
	wg.Add(1)
	wantedData := []byte{'A', 'B', 'C', 'D'}
	nPeer.BHost.SetStreamHandler(core.ProtocolID(prID), func(stream network.Stream) {
		// No Context will be read from it
		dt, err := readContextFromStream(stream, nil)
		assert.NoError(t, err)
		assert.Equal(t, 0, len(dt))

		// Ensure sent over data hasn't been modified.
		buf := make([]byte, len(wantedData))
		n, err := stream.Read(buf)
		assert.NoError(t, err)
		assert.Equal(t, len(wantedData), n)
		assert.DeepEqual(t, wantedData, buf)

		wg.Done()
	})
	strm, err := p1.BHost.NewStream(context.Background(), nPeer.PeerID(), p2p.RPCPingTopicV1)
	assert.NoError(t, err)

	n, err := strm.Write(wantedData)
	assert.NoError(t, err)
	assert.Equal(t, len(wantedData), n)
	if testutil.WaitTimeout(wg, 1*time.Second) {
		t.Fatal("Did not receive stream within 1 sec")
	}
}
