package p2p

import (
	p2ppb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
)

const (
	// RPCStatusTopic defines the topic for the status rpc method.
	RPCStatusTopic = "/eth2/beacon_chain/req/status/1"
	// RPCGoodByeTopic defines the topic for the goodbye rpc method.
	RPCGoodByeTopic = "/eth2/beacon_chain/req/goodbye/1"
	// RPCBlocksByRangeTopic defines the topic for the blocks by range rpc method.
	RPCBlocksByRangeTopic = "/eth2/beacon_chain/req/beacon_blocks_by_range/1"
	// RPCBlocksByRootTopic defines the topic for the blocks by root rpc method.
	RPCBlocksByRootTopic = "/eth2/beacon_chain/req/beacon_blocks_by_root/1"
	// RPCPingTopic defines the topic for the ping rpc method.
	RPCPingTopic = "/eth2/beacon_chain/req/ping/1"
	// RPCMetaDataTopic defines the topic for the metadata rpc method.
	RPCMetaDataTopic = "/eth2/beacon_chain/req/metadata/1"
)

// RPCTopicMappings represent the protocol ID to protobuf message type map for easy
// lookup. These mappings should be used for outbound sending only. Peers may respond
// with a different message type as defined by the p2p protocol.
var RPCTopicMappings = map[string]interface{}{
	RPCStatusTopic:        &p2ppb.Status{},
	RPCGoodByeTopic:       new(uint64),
	RPCBlocksByRangeTopic: &p2ppb.BeaconBlocksByRangeRequest{},
	RPCBlocksByRootTopic:  [][32]byte{},
	RPCPingTopic:          new(uint64),
	RPCMetaDataTopic:      new(interface{}),
}
