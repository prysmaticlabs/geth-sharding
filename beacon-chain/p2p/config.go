package p2p

import (
	"github.com/prysmaticlabs/prysm/beacon-chain/db"
)

// Config for the p2p service. These parameters are set from application level flags
// to initialize the p2p service.
type Config struct {
	BeaconDB              db.Database
	NoDiscovery           bool
	StaticPeers           []string
	BootstrapNodeAddr     []string
	KademliaBootStrapAddr []string
	Discv5BootStrapAddr   []string
	RelayNodeAddr         string
	LocalIP               string
	HostAddress           string
	HostDNS               string
	PrivateKey            string
	DataDir               string
	TCPPort               uint
	UDPPort               uint
	MaxPeers              uint
	WhitelistCIDR         string
	EnableUPnP            bool
	EnableDiscv5          bool
	Encoding              string
}
