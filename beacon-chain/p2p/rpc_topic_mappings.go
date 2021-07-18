package p2p

import (
	"reflect"

	"github.com/pkg/errors"
	types "github.com/prysmaticlabs/eth2-types"
	p2ptypes "github.com/prysmaticlabs/prysm/beacon-chain/p2p/types"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	"github.com/prysmaticlabs/prysm/shared/params"
)

// SchemaVersionV1 specifies the schema version for our rpc protocol ID.
const SchemaVersionV1 = "/1"

// SchemaVersionV2 specifies the next schema version for our rpc protocol ID.
const SchemaVersionV2 = "/2"

// Specifies the protocol prefix for all our Req/Resp topics.
const protocolPrefix = "/eth2/beacon_chain/req"

// StatusMessageName specifies the name for the status message topic.
const StatusMessageName = "/status"

// GoodbyeMessageName specifies the name for the goodbye message topic.
const GoodbyeMessageName = "/goodbye"

// BeaconBlocksByRangeMessageName specifies the name for the beacon blocks by range message topic.
const BeaconBlocksByRangeMessageName = "/beacon_blocks_by_range"

// BeaconBlocksByRootsMessageName specifies the name for the beacon blocks by root message topic.
const BeaconBlocksByRootsMessageName = "/beacon_blocks_by_root"

// PingMessageName Specifies the name for the ping message topic.
const PingMessageName = "/ping"

// MetadataMessageName specifies the name for the metadata message topic.
const MetadataMessageName = "/metadata"

const (
	// V1 RPC Topics
	// RPCStatusTopicV1 defines the v1 topic for the status rpc method.
	RPCStatusTopicV1 = protocolPrefix + StatusMessageName + SchemaVersionV1
	// RPCGoodByeTopicV1 defines the v1 topic for the goodbye rpc method.
	RPCGoodByeTopicV1 = protocolPrefix + GoodbyeMessageName + SchemaVersionV1
	// RPCBlocksByRangeTopicV1 defines v1 the topic for the blocks by range rpc method.
	RPCBlocksByRangeTopicV1 = protocolPrefix + BeaconBlocksByRangeMessageName + SchemaVersionV1
	// RPCBlocksByRootTopicV1 defines the v1 topic for the blocks by root rpc method.
	RPCBlocksByRootTopicV1 = protocolPrefix + BeaconBlocksByRootsMessageName + SchemaVersionV1
	// RPCPingTopicV1 defines the v1 topic for the ping rpc method.
	RPCPingTopicV1 = protocolPrefix + PingMessageName + SchemaVersionV1
	// RPCMetaDataTopicV1 defines the v1 topic for the metadata rpc method.
	RPCMetaDataTopicV1 = protocolPrefix + MetadataMessageName + SchemaVersionV1

	// V2 RPC Topics
	// RPCBlocksByRangeTopicV2 defines v2 the topic for the blocks by range rpc method.
	RPCBlocksByRangeTopicV2 = protocolPrefix + BeaconBlocksByRangeMessageName + SchemaVersionV2
	// RPCBlocksByRootTopicV2 defines the v2 topic for the blocks by root rpc method.
	RPCBlocksByRootTopicV2 = protocolPrefix + BeaconBlocksByRootsMessageName + SchemaVersionV2
	// RPCMetaDataTopicV2 defines the v2 topic for the metadata rpc method.
	RPCMetaDataTopicV2 = protocolPrefix + MetadataMessageName + SchemaVersionV2
)

// RPC errors for topic parsing.
const (
	invalidRPCMessageType = "provided message type doesn't have a registered mapping"
)

// RPCTopicMappings map the base message type to the rpc request.
var RPCTopicMappings = map[string]interface{}{
	// RPC Status Message
	RPCStatusTopicV1: new(pb.Status),
	// RPC Goodbye Message
	RPCGoodByeTopicV1: new(types.SSZUint64),
	// RPC Block By Range Message
	RPCBlocksByRangeTopicV1: new(pb.BeaconBlocksByRangeRequest),
	RPCBlocksByRangeTopicV2: new(pb.BeaconBlocksByRangeRequest),
	// RPC Block By Root Message
	RPCBlocksByRootTopicV1: new(p2ptypes.BeaconBlockByRootsReq),
	RPCBlocksByRootTopicV2: new(p2ptypes.BeaconBlockByRootsReq),
	// RPC Ping Message
	RPCPingTopicV1: new(types.SSZUint64),
	// RPC Metadata Message
	RPCMetaDataTopicV1: new(interface{}),
	RPCMetaDataTopicV2: new(interface{}),
}

// Maps all registered protocol prefixes.
var protocolMapping = map[string]bool{
	protocolPrefix: true,
}

// Maps all the protocol message names for the different rpc
// topics.
var messageMapping = map[string]bool{
	StatusMessageName:              true,
	GoodbyeMessageName:             true,
	BeaconBlocksByRangeMessageName: true,
	BeaconBlocksByRootsMessageName: true,
	PingMessageName:                true,
	MetadataMessageName:            true,
}

// Maps all the RPC messages which are to updated in altair.
var altairMapping = map[string]bool{
	BeaconBlocksByRangeMessageName: true,
	BeaconBlocksByRootsMessageName: true,
	MetadataMessageName:            true,
}

var versionMapping = map[string]bool{
	SchemaVersionV1: true,
	SchemaVersionV2: true,
}

// VerifyTopicMapping verifies that the topic and its accompanying
// message type is correct.
func VerifyTopicMapping(topic string, msg interface{}) error {
	msgType, ok := RPCTopicMappings[topic]
	if !ok {
		return errors.New("rpc topic is not registered currently")
	}
	receivedType := reflect.TypeOf(msg)
	registeredType := reflect.TypeOf(msgType)
	typeMatches := registeredType.AssignableTo(receivedType)

	if !typeMatches {
		return errors.Errorf("accompanying message type is incorrect for topic: wanted %v  but got %v",
			registeredType.String(), receivedType.String())
	}
	return nil
}

// TopicDeconstructor splits the provided topic to its logical sub-sections.
// It is assumed all input topics will follow the specific schema:
// /protocol-prefix/message-name/schema-version/...
// For the purposes of deconstruction, only the first 3 components are
// relevant.
func TopicDeconstructor(topic string) (string, string, string, error) {
	origTopic := topic
	protPrefix := ""
	message := ""
	version := ""

	// Iterate through all the relevant mappings to find the relevant prefixes,messages
	// and version for this topic.
	for k := range protocolMapping {
		keyLen := len(k)
		if keyLen > len(topic) {
			continue
		}
		if topic[:keyLen] == k {
			protPrefix = k
			topic = topic[keyLen:]
		}
	}

	if protPrefix == "" {
		return "", "", "", errors.Errorf("unable to find a valid protocol prefix for %s", origTopic)
	}

	for k := range messageMapping {
		keyLen := len(k)
		if keyLen > len(topic) {
			continue
		}
		if topic[:keyLen] == k {
			message = k
			topic = topic[keyLen:]
		}
	}

	if message == "" {
		return "", "", "", errors.Errorf("unable to find a valid message for %s", origTopic)
	}

	for k := range versionMapping {
		keyLen := len(k)
		if keyLen > len(topic) {
			continue
		}
		if topic[:keyLen] == k {
			version = k
			topic = topic[keyLen:]
		}
	}

	if version == "" {
		return "", "", "", errors.Errorf("unable to find a valid schema version for %s", origTopic)
	}

	return protPrefix, message, version, nil
}

// RPCTopic is a type used to denote and represent a req/resp topic.
type RPCTopic string

// ProtocolPrefix returns the protocol prefix of the rpc topic.
func (r RPCTopic) ProtocolPrefix() string {
	prefix, _, _, err := TopicDeconstructor(string(r))
	if err != nil {
		return ""
	}
	return prefix
}

// MessageType returns the message type of the rpc topic.
func (r RPCTopic) MessageType() string {
	_, message, _, err := TopicDeconstructor(string(r))
	if err != nil {
		return ""
	}
	return message
}

// Version returns the schema version of the rpc topic.
func (r RPCTopic) Version() string {
	_, _, version, err := TopicDeconstructor(string(r))
	if err != nil {
		return ""
	}
	return version
}

// TopicFromMessage constructs the rpc topic from the provided message
// type and epoch.
func TopicFromMessage(msg string, epoch types.Epoch) (string, error) {
	if !messageMapping[msg] {
		return "", errors.Errorf("%s: %s", invalidRPCMessageType, msg)
	}
	version := SchemaVersionV1
	isAltair := epoch >= params.BeaconConfig().AltairForkEpoch
	if isAltair && altairMapping[msg] {
		version = SchemaVersionV2
	}
	return protocolPrefix + msg + version, nil
}
