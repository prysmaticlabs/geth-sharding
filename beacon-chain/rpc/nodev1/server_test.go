package nodev1

import (
	ethpb "github.com/prysmaticlabs/prysm/proto/eth/v1"
)

var _ ethpb.BeaconNodeServer = (*Server)(nil)
