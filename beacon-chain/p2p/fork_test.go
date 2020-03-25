package p2p

import (
	"bytes"
	"math/rand"
	"os"
	"path"
	"strconv"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/p2p/discover"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/sirupsen/logrus"
	logTest "github.com/sirupsen/logrus/hooks/test"

	mock "github.com/prysmaticlabs/prysm/beacon-chain/blockchain/testing"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/testutil"
)

func TestStartDiscv5_DifferentForkDigests(t *testing.T) {
	port := 2000
	ipAddr, pkey := createAddrAndPrivKey(t)
	genesisTime := time.Now()
	genesisValidatorsRoot := make([]byte, 32)
	s := &Service{
		cfg:                   &Config{UDPPort: uint(port)},
		genesisTime:           genesisTime,
		genesisValidatorsRoot: genesisValidatorsRoot,
	}
	bootListener := s.createListener(ipAddr, pkey)
	defer bootListener.Close()

	bootNode := bootListener.Self()
	cfg := &Config{
		Discv5BootStrapAddr: []string{bootNode.String()},
		Encoding:            "ssz",
		UDPPort:             uint(port),
	}

	var listeners []*discover.UDPv5
	for i := 1; i <= 5; i++ {
		port = 3000 + i
		cfg.UDPPort = uint(port)
		ipAddr, pkey := createAddrAndPrivKey(t)

		// We give every peer a different genesis validators root, which
		// will cause each peer to have a different ForkDigest, preventing
		// them from connecting according to our discovery rules for eth2.
		root := make([]byte, 32)
		copy(root, strconv.Itoa(port))
		s = &Service{
			cfg:                   cfg,
			genesisTime:           genesisTime,
			genesisValidatorsRoot: root,
		}
		listener, err := s.startDiscoveryV5(ipAddr, pkey)
		if err != nil {
			t.Errorf("Could not start discovery for node: %v", err)
		}
		listeners = append(listeners, listener)
	}

	// Wait for the nodes to have their local routing tables to be populated with the other nodes
	time.Sleep(discoveryWaitTime)

	lastListener := listeners[len(listeners)-1]
	nodes := lastListener.Lookup(bootNode.ID())
	if len(nodes) < 4 {
		t.Errorf("The node's local table doesn't have the expected number of nodes. "+
			"Expected more than or equal to %d but got %d", 4, len(nodes))
	}

	// Now, we start a new p2p service. It should have no peers aside from the
	// bootnode given all nodes provided by discv5 will have different fork digests.
	cfg.UDPPort = 14000
	cfg.TCPPort = 14001
	mockChainService := &mock.ChainService{}
	cfg.StateNotifier = mockChainService.StateNotifier()
	s, err := NewService(cfg)
	if err != nil {
		t.Fatal(err)
	}
	s.genesisTime = genesisTime
	s.genesisValidatorsRoot = make([]byte, 32)
	s.dv5Listener = lastListener
	multiAddrs := s.processPeers(nodes)

	// We should not have valid peers if the fork digest mismatched.
	if len(multiAddrs) != 0 {
		t.Errorf("Expected 0 valid peers, got %d", len(multiAddrs))
	}

	// Close down all peers.
	for _, listener := range listeners {
		listener.Close()
	}
}

func TestStartDiscv5_SameForkDigests_DifferentNextForkData(t *testing.T) {
	hook := logTest.NewGlobal()
	logrus.SetLevel(logrus.DebugLevel)
	port := 2000
	ipAddr, pkey := createAddrAndPrivKey(t)
	genesisTime := time.Now()
	genesisValidatorsRoot := make([]byte, 32)
	s := &Service{
		cfg:                   &Config{UDPPort: uint(port)},
		genesisTime:           genesisTime,
		genesisValidatorsRoot: genesisValidatorsRoot,
	}
	bootListener := s.createListener(ipAddr, pkey)
	defer bootListener.Close()

	bootNode := bootListener.Self()
	cfg := &Config{
		Discv5BootStrapAddr: []string{bootNode.String()},
		Encoding:            "ssz",
		UDPPort:             uint(port),
	}

	originalBeaconConfig := params.BeaconConfig()

	var listeners []*discover.UDPv5
	for i := 1; i <= 5; i++ {
		port = 3000 + i
		cfg.UDPPort = uint(port)
		ipAddr, pkey := createAddrAndPrivKey(t)

		c := params.BeaconConfig()
		nextForkEpoch := uint64(i)
		c.NextForkEpoch = nextForkEpoch
		params.OverrideBeaconConfig(c)

		// We give every peer a different genesis validators root, which
		// will cause each peer to have a different ForkDigest, preventing
		// them from connecting according to our discovery rules for eth2.
		s = &Service{
			cfg:                   cfg,
			genesisTime:           genesisTime,
			genesisValidatorsRoot: genesisValidatorsRoot,
		}
		listener, err := s.startDiscoveryV5(ipAddr, pkey)
		if err != nil {
			t.Errorf("Could not start discovery for node: %v", err)
		}
		listeners = append(listeners, listener)
	}

	// Wait for the nodes to have their local routing tables to be populated with the other nodes
	time.Sleep(discoveryWaitTime)

	lastListener := listeners[len(listeners)-1]
	nodes := lastListener.Lookup(bootNode.ID())
	if len(nodes) < 4 {
		t.Errorf("The node's local table doesn't have the expected number of nodes. "+
			"Expected more than or equal to %d but got %d", 4, len(nodes))
	}

	// Now, we start a new p2p service. It should have no peers aside from the
	// bootnode given all nodes provided by discv5 will have different fork digests.
	cfg.UDPPort = 14000
	cfg.TCPPort = 14001
	mockChainService := &mock.ChainService{}
	cfg.StateNotifier = mockChainService.StateNotifier()
	params.OverrideBeaconConfig(originalBeaconConfig)
	s, err := NewService(cfg)
	if err != nil {
		t.Fatal(err)
	}
	s.genesisTime = genesisTime
	s.genesisValidatorsRoot = make([]byte, 32)
	s.dv5Listener = lastListener
	multiAddrs := s.processPeers(nodes)
	if len(multiAddrs) == 0 {
		t.Error("Expected to have valid peers, got 0")
	}

	// Close down all peers.
	for _, listener := range listeners {
		listener.Close()
	}

	testutil.AssertLogsContain(t, hook, "Peer matches fork digest but has different next fork epoch")
}

func TestDiscv5_AddRetrieveForkEntryENR(t *testing.T) {
	c := params.BeaconConfig()
	c.ForkVersionSchedule = map[uint64][]byte{
		0: params.BeaconConfig().GenesisForkVersion,
		1: {0, 0, 0, 1},
		2: {0, 0, 0, 2},
		3: {0, 0, 0, 3},
	}
	nextForkEpoch := uint64(2)
	nextForkVersion := []byte{0, 0, 0, 2}
	c.NextForkEpoch = nextForkEpoch
	c.NextForkVersion = nextForkVersion
	params.OverrideBeaconConfig(c)

	// We simulate being in epoch 1.
	secondsPerEpoch := params.BeaconConfig().SlotsPerEpoch * params.BeaconConfig().SecondsPerSlot
	additionalBuffer := 2 * time.Second
	durationPerEpoch := (time.Duration(secondsPerEpoch) * time.Second) + additionalBuffer
	genesisTime := time.Now().Add(-durationPerEpoch)

	// In epoch 1 of current time, the fork version should be
	// {0, 0, 0, 1} according to the configuration override above.
	temp := testutil.TempDir()
	randNum := rand.Int()
	tempPath := path.Join(temp, strconv.Itoa(randNum))
	if err := os.Mkdir(tempPath, 0700); err != nil {
		t.Fatal(err)
	}
	pkey, err := privKey(&Config{Encoding: "ssz", DataDir: tempPath})
	if err != nil {
		t.Fatalf("Could not get private key: %v", err)
	}
	db, err := enode.OpenDB("")
	if err != nil {
		t.Fatal(err)
	}
	localNode := enode.NewLocalNode(db, pkey)

	genesisValidatorsRoot := make([]byte, 32)
	localNode, err = addForkEntry(localNode, genesisTime, genesisValidatorsRoot)
	if err != nil {
		t.Fatal(err)
	}

	want, err := helpers.ComputeForkDigest([]byte{0, 0, 0, 1}, genesisValidatorsRoot)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := retrieveForkEntry(localNode.Node().Record())
	if err != nil {
		t.Fatal(err)
	}
	if resp.CurrentForkDigest != want {
		t.Errorf("Wanted fork digest: %v, received %v", want, resp.CurrentForkDigest)
	}
	if !bytes.Equal(resp.NextForkVersion[:], nextForkVersion) {
		t.Errorf("Wanted next fork version: %v, received %v", nextForkVersion, resp.NextForkVersion)
	}
	if resp.NextForkEpoch != nextForkEpoch {
		t.Errorf("Wanted next for epoch: %d, received: %d", nextForkEpoch, resp.NextForkEpoch)
	}
}
