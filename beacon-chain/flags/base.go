package flags

import (
	"github.com/urfave/cli"
)

var (
	// NoCustomConfigFlag determines whether to launch a beacon chain using real parameters or demo parameters.
	NoCustomConfigFlag = cli.BoolFlag{
		Name:  "no-custom-config",
		Usage: "Run the beacon chain with the real parameters from phase 0.",
	}
	// HTTPWeb3ProviderFlag provides an HTTP access endpoint to an ETH 1.0 RPC.
	HTTPWeb3ProviderFlag = cli.StringFlag{
		Name:  "http-web3provider",
		Usage: "A mainchain web3 provider string http endpoint",
		Value: "https://goerli.prylabs.net",
	}
	// Web3ProviderFlag defines a flag for a mainchain RPC endpoint.
	Web3ProviderFlag = cli.StringFlag{
		Name:  "web3provider",
		Usage: "A mainchain web3 provider string endpoint. Can either be an IPC file string or a WebSocket endpoint. Cannot be an HTTP endpoint.",
		Value: "wss://goerli.prylabs.net/websocket",
	}
	// DepositContractFlag defines a flag for the deposit contract address.
	DepositContractFlag = cli.StringFlag{
		Name:  "deposit-contract",
		Usage: "Deposit contract address. Beacon chain node will listen logs coming from the deposit contract to determine when validator is eligible to participate.",
	}
	// RPCHost defines the host on which the RPC server should listen.
	RPCHost = cli.StringFlag{
		Name:  "rpc-host",
		Usage: "Host on which the RPC server should listen",
		Value: "0.0.0.0",
	}
	// RPCPort defines a beacon node RPC port to open.
	RPCPort = cli.IntFlag{
		Name:  "rpc-port",
		Usage: "RPC port exposed by a beacon node",
		Value: 4000,
	}
	// RPCMaxPageSize defines the maximum numbers per page returned in RPC responses from this
	// beacon node (default: 500).
	RPCMaxPageSize = cli.IntFlag{
		Name:  "rpc-max-page-size",
		Usage: "Max number of items returned per page in RPC responses for paginated endpoints (default: 500)",
		Value: 500,
	}
	// CertFlag defines a flag for the node's TLS certificate.
	CertFlag = cli.StringFlag{
		Name:  "tls-cert",
		Usage: "Certificate for secure gRPC. Pass this and the tls-key flag in order to use gRPC securely.",
	}
	// KeyFlag defines a flag for the node's TLS key.
	KeyFlag = cli.StringFlag{
		Name:  "tls-key",
		Usage: "Key for secure gRPC. Pass this and the tls-cert flag in order to use gRPC securely.",
	}
	// GRPCGatewayPort enables a gRPC gateway to be exposed for Prysm.
	GRPCGatewayPort = cli.IntFlag{
		Name:  "grpc-gateway-port",
		Usage: "Enable gRPC gateway for JSON requests",
	}
	// MinSyncPeers specifies the required number of successful peer handshakes in order
	// to start syncing with external peers.
	MinSyncPeers = cli.IntFlag{
		Name:  "min-sync-peers",
		Usage: "The required number of valid peers to connect with before syncing.",
		Value: 3,
	}
	// ContractDeploymentBlock is the block in which the eth1 deposit contract was deployed.
	ContractDeploymentBlock = cli.IntFlag{
		Name:  "contract-deployment-block",
		Usage: "The eth1 block in which the deposit contract was deployed.",
		Value: 1960177,
	}
	// SetGCPercent is the percentage of current live allocations at which the garbage collector is to run.
	SetGCPercent = cli.IntFlag{
		Name:  "gc-percent",
		Usage: "The percentage of freshly allocated data to live data on which the gc will be run again.",
		Value: 100,
	}
	// UnsafeSync starts the beacon node from the previously saved head state and syncs from there.
	UnsafeSync = cli.BoolFlag{
		Name:  "unsafe-sync",
		Usage: "Starts the beacon node with the previously saved head state instead of finalized state.",
	}
	// SlasherCertFlag defines a flag for the slasher TLS certificate.
	SlasherCertFlag = cli.StringFlag{
		Name:  "slasher-tls-cert",
		Usage: "Certificate for secure slasher gRPC connection. Pass this in order to use slasher gRPC securely.",
	}
	// SlasherProviderFlag defines a flag for a slasher RPC provider.
	SlasherProviderFlag = cli.StringFlag{
		Name:  "slasher-provider",
		Usage: "A slasher provider string endpoint. Can either be an grpc server endpoint.",
		Value: "127.0.0.1:5000",
	}
)
