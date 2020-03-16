package evaluators

import (
	"google.golang.org/grpc"
)

// Evaluator defines the structure of the evaluators used to
// conduct the current beacon state during the E2E.
type Evaluator struct {
	Name       string
	Policy     func(currentEpoch uint64) bool
	Evaluation func(conn ...*grpc.ClientConn) error
}

// BeaconNodeInfo contains the info of ports and other required information
// needed to communicate with the beacon node it represents.
type BeaconNodeInfo struct {
	ProcessID   int
	DataDir     string
	RPCPort     uint64
	MonitorPort uint64
	GRPCPort    uint64
	MultiAddr   string
}
