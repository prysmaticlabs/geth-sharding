package p2p

import (
	"crypto/ecdsa"
	"crypto/rand"
	"net"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/p2p/discv5"
	"github.com/prysmaticlabs/prysm/shared/iputils"
	"github.com/prysmaticlabs/prysm/shared/testutil"
	logTest "github.com/sirupsen/logrus/hooks/test"
)

func createAddrAndPrivKey(t *testing.T) (net.IP, *ecdsa.PrivateKey) {
	ip, err := iputils.ExternalIPv4()
	if err != nil {
		t.Fatalf("Could not get ip: %v", err)
	}
	ipAddr := net.ParseIP(ip)
	pkey, err := privKey("")
	if err != nil {
		t.Fatalf("Could not get private key: %v", err)
	}
	return ipAddr, pkey
}

func TestCreateListener(t *testing.T) {
	port := 1024
	ipAddr, pkey := createAddrAndPrivKey(t)
	listener := createListener(ipAddr, port, pkey)
	defer listener.Close()

	if !listener.Self().IP.Equal(ipAddr) {
		t.Errorf("Ip address is not the expected type, wanted %s but got %s", ipAddr.String(), listener.Self().IP.String())
	}

	if port != int(listener.Self().UDP) {
		t.Errorf("In correct port number, wanted %d but got %d", port, listener.Self().UDP)
	}
	pubkey, err := listener.Self().ID.Pubkey()
	if err != nil {
		t.Error(err)
	}
	XisSame := pkey.PublicKey.X.Cmp(pubkey.X) == 0
	YisSame := pkey.PublicKey.Y.Cmp(pubkey.Y) == 0

	if !(XisSame && YisSame) {
		t.Error("Pubkey is different from what was used to create the listener")
	}
}

func TestStartDiscV5_DiscoverAllPeers(t *testing.T) {
	port := 2000
	ipAddr, pkey := createAddrAndPrivKey(t)
	bootListener := createListener(ipAddr, port, pkey)
	defer bootListener.Close()

	bootNode := bootListener.Self()

	cfg := &Config{
		BootstrapNodeAddr: bootNode.String(),
	}

	var listeners []*discv5.Network
	for i := 1; i <= 10; i++ {
		port = 2000 + i
		cfg.UDPPort = uint(port)
		ipAddr, pkey := createAddrAndPrivKey(t)
		listener, err := startDiscoveryV5(ipAddr, pkey, cfg)
		if err != nil {
			t.Errorf("Could not start discovery for node: %v", err)
		}
		listeners = append(listeners, listener)
	}

	// Wait for the nodes to have their local routing tables to be populated with the other nodes
	time.Sleep(100 * time.Millisecond)

	lastListener := listeners[len(listeners)-1]
	nodes := lastListener.Lookup(bootNode.ID)
	if len(nodes) != 11 {
		t.Errorf("The node's local table doesn't have the expected number of nodes. "+
			"Expected %d but got %d", 11, len(nodes))
	}

	// Close all ports
	for _, listener := range listeners {
		listener.Close()
	}
}

func TestMultiAddrsConversion_InvalidIPAddr(t *testing.T) {
	hook := logTest.NewGlobal()
	ipAddr := net.IPv6zero
	pkey, err := ecdsa.GenerateKey(crypto.S256(), rand.Reader)
	if err != nil {
		t.Fatalf("Could not generate key %v", err)
	}
	nodeID := discv5.PubkeyID(&pkey.PublicKey)
	node := discv5.NewNode(nodeID, ipAddr, 0, 0)
	_ = convertToMultiAddr([]*discv5.Node{node})

	testutil.AssertLogsContain(t, hook, "Node doesn't have an ip4 address")
}

func TestMultiAddrConversion_OK(t *testing.T) {
	hook := logTest.NewGlobal()
	port := 1024
	ipAddr, pkey := createAddrAndPrivKey(t)
	listener := createListener(ipAddr, port, pkey)

	_ = convertToMultiAddr([]*discv5.Node{listener.Self()})
	testutil.AssertLogsDoNotContain(t, hook, "Node doesn't have an ip4 address")
	testutil.AssertLogsDoNotContain(t, hook, "Invalid port, the tcp port of the node is a reserved port")
	testutil.AssertLogsDoNotContain(t, hook, "Could not get multiaddr")
}
