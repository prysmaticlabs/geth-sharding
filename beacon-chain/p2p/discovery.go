package p2p

import (
	"crypto/ecdsa"
	"errors"
	"fmt"
	"net"

	"github.com/ethereum/go-ethereum/p2p/discv5"
	ma "github.com/multiformats/go-multiaddr"

	_ "go.uber.org/automaxprocs"
)

func createListener(ipAddr net.IP, port int, privKey *ecdsa.PrivateKey) *discv5.Network {
	udpAddr := &net.UDPAddr{
		IP:   ipAddr,
		Port: port,
	}
	conn, err := net.ListenUDP("udp4", udpAddr)
	if err != nil {
		log.Fatal(err)
	}

	network, err := discv5.ListenUDP(privKey, conn, "", nil)
	if err != nil {
		log.Fatal(err)
	}
	return network
}

func startDiscoveryV5(addr net.IP, privKey *ecdsa.PrivateKey, cfg *Config) (*discv5.Network, error) {
	listener := createListener(addr, int(cfg.UDPPort), privKey)
	nodeID, err := discv5.HexID(cfg.BootstrapNodeAddr)
	if err != nil {
		return nil, err
	}

	bootNode := listener.Resolve(nodeID)
	if bootNode == nil {
		return nil, errors.New("bootnode could not be found")
	}
	if err := listener.SetFallbackNodes([]*discv5.Node{bootNode}); err != nil {
		return nil, err
	}
	node := listener.Self()
	log.Infof("Started Discovery: %s", node.ID)

	return listener, nil
}

func convertToMultiAddr(nodes []*discv5.Node) []ma.Multiaddr {
	var multiAddrs []ma.Multiaddr
	for _, node := range nodes {
		ip4 := node.IP.To4()
		if ip4 == nil {
			log.Error("node doesn't have an ip4 address")
			continue
		}
		if node.TCP < 1024 {
			log.Errorf("Invalid port, the tcp port of the node is a reserved port: %d", node.TCP)
		}
		multiAddrString := fmt.Sprintf("/ip4/%s/tcp/%d/discv5/%s", ip4.String(), node.TCP, node.ID.String())
		multiAddr, err := ma.NewMultiaddr(multiAddrString)
		if err != nil {
			log.Errorf("could not get multiaddr:%v", err)
			continue
		}
		multiAddrs = append(multiAddrs, multiAddr)
	}
	return multiAddrs
}
