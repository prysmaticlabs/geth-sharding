// Package cmd defines the command line flags for the shared utlities.
package cmd

import (
	"github.com/ethereum/go-ethereum/node"
	"github.com/urfave/cli"
)

var (
	// VerbosityFlag defines the logrus configuration.
	VerbosityFlag = cli.StringFlag{
		Name:  "verbosity",
		Usage: "Logging verbosity (debug, info=default, warn, error, fatal, panic)",
		Value: "info",
	}
	// IPCPathFlag defines the filename of a pipe within the datadir.
	IPCPathFlag = DirectoryFlag{
		Name:  "ipcpath",
		Usage: "Filename for IPC socket/pipe within the datadir (explicit paths escape it)",
	}
	// DataDirFlag defines a path on disk.
	DataDirFlag = DirectoryFlag{
		Name:  "datadir",
		Usage: "Data directory for the databases and keystore",
		Value: DirectoryString{node.DefaultDataDir()},
	}
	// NetworkIDFlag defines the specific network identifier.
	NetworkIDFlag = cli.Uint64Flag{
		Name:  "networkid",
		Usage: "Network identifier (integer, 1=Frontier, 2=Morden (disused), 3=Ropsten, 4=Rinkeby)",
		Value: 1,
	}
	// PasswordFileFlag defines the path to the user's account password file.
	PasswordFileFlag = cli.StringFlag{
		Name:  "password",
		Usage: "Password file to use for non-interactive password input",
		Value: "",
	}
	// RPCProviderFlag defines a http endpoint flag to connect to mainchain.
	RPCProviderFlag = cli.StringFlag{
		Name:  "rpc",
		Usage: "HTTP-RPC server end point to use to connect to mainchain.",
		Value: "http://localhost:8545/",
	}
	// EnableTracingFlag defines a flag to enable p2p message tracing.
	EnableTracingFlag = cli.BoolFlag{
		Name:  "enable-tracing",
		Usage: "Enable request tracing.",
	}
	// TracingEndpointFlag flag defines the http endpoint for serving traces to Jaeger.
	TracingEndpointFlag = cli.StringFlag{
		Name:  "tracing-endpoint",
		Usage: "Tracing endpoint defines where beacon chain traces are exposed to Jaeger.",
		Value: "http://127.0.0.1:14268",
	}
	// TraceSampleFractionFlag defines a flag to indicate what fraction of p2p
	// messages are sampled for tracing.
	TraceSampleFractionFlag = cli.Float64Flag{
		Name:  "trace-sample-fraction",
		Usage: "Indicate what fraction of p2p messages are sampled for tracing.",
		Value: 0.20,
	}
	// DisableMonitoringFlag defines a flag to disable the metrics collection.
	DisableMonitoringFlag = cli.BoolFlag{
		Name:  "disable-monitoring",
		Usage: "Disable monitoring service.",
	}
	// MonitoringPortFlag defines the http port used to serve prometheus metrics.
	MonitoringPortFlag = cli.Int64Flag{
		Name:  "monitoring-port",
		Usage: "Port used to listening and respond metrics for prometheus.",
		Value: 8080,
	}
	// KeystorePasswordFlag defines the password that will unlock the keystore file.
	KeystorePasswordFlag = cli.StringFlag{
		Name:  "keystore-password",
		Usage: "Keystore password is used to unlock the keystore so that the users decrypted keys can be used.",
	}
	// BootstrapNode tells the beacon node which bootstrap node to connect to
	BootstrapNode = cli.StringFlag{
		Name:  "bootstrap-node",
		Usage: "The address of bootstrap node. Beacon node will connect for peer discovery via DHT",
	}

	// RelayNode tells the beacon node which relay node to connect to.
	RelayNode = cli.StringFlag{
		Name: "relay-node",
		Usage: "The address of relay node. The beacon node will connect to the " +
			"relay node and advertise their address via the relay node to other peers",
	}

	// P2PPort defines the port to be used by libp2p.
	P2PPort = cli.IntFlag{
		Name:  "p2p-port",
		Usage: "The port used by libp2p.",
		Value: 12000,
	}
)
