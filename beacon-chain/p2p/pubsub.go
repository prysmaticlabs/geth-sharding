package p2p

import (
	"context"
	"encoding/hex"
	"strings"
	"time"

	"github.com/libp2p/go-libp2p-core/peer"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	pubsub_pb "github.com/libp2p/go-libp2p-pubsub/pb"
	"github.com/pkg/errors"
	"github.com/prysmaticlabs/prysm/beacon-chain/p2p/encoder"
	"github.com/prysmaticlabs/prysm/cmd/beacon-chain/flags"
	pbrpc "github.com/prysmaticlabs/prysm/proto/beacon/rpc/v1"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/prysmaticlabs/prysm/shared/featureconfig"
	"github.com/prysmaticlabs/prysm/shared/hashutil"
	"github.com/prysmaticlabs/prysm/shared/p2putils"
	"github.com/prysmaticlabs/prysm/shared/params"
)

const (
	// overlay parameters
	gossipSubD   = 8  // topic stable mesh target count
	gossipSubDlo = 6  // topic stable mesh low watermark
	gossipSubDhi = 12 // topic stable mesh high watermark

	// gossip parameters
	gossipSubMcacheLen    = 6   // number of windows to retain full messages in cache for `IWANT` responses
	gossipSubMcacheGossip = 3   // number of windows to gossip about
	gossipSubSeenTTL      = 550 // number of heartbeat intervals to retain message IDs

	// fanout ttl
	gossipSubFanoutTTL = 60000000000 // TTL for fanout maps for topics we are not subscribed to but have published to, in nano seconds

	// heartbeat interval
	gossipSubHeartbeatInterval = 700 * time.Millisecond // frequency of heartbeat, milliseconds

	// misc
	randomSubD = 6 // random gossip target
)

var errInvalidTopic = errors.New("invalid topic format")

// Specifies the fixed size context length.
const digestLength = 4

// JoinTopic will join PubSub topic, if not already joined.
func (s *Service) JoinTopic(topic string, opts ...pubsub.TopicOpt) (*pubsub.Topic, error) {
	s.joinedTopicsLock.Lock()
	defer s.joinedTopicsLock.Unlock()

	if _, ok := s.joinedTopics[topic]; !ok {
		topicHandle, err := s.pubsub.Join(topic, opts...)
		if err != nil {
			return nil, err
		}
		s.joinedTopics[topic] = topicHandle
	}

	return s.joinedTopics[topic], nil
}

// LeaveTopic closes topic and removes corresponding handler from list of joined topics.
// This method will return error if there are outstanding event handlers or subscriptions.
func (s *Service) LeaveTopic(topic string) error {
	s.joinedTopicsLock.Lock()
	defer s.joinedTopicsLock.Unlock()

	if t, ok := s.joinedTopics[topic]; ok {
		if err := t.Close(); err != nil {
			return err
		}
		delete(s.joinedTopics, topic)
	}
	return nil
}

// PublishToTopic joins (if necessary) and publishes a message to a PubSub topic.
func (s *Service) PublishToTopic(ctx context.Context, topic string, data []byte, opts ...pubsub.PubOpt) error {
	topicHandle, err := s.JoinTopic(topic)
	if err != nil {
		return err
	}

	// Wait for at least 1 peer to be available to receive the published message.
	for {
		if len(topicHandle.ListPeers()) > 0 || flags.Get().MinimumSyncPeers == 0 {
			return topicHandle.Publish(ctx, data, opts...)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			time.Sleep(100 * time.Millisecond)
		}
	}
}

// SubscribeToTopic joins (if necessary) and subscribes to PubSub topic.
func (s *Service) SubscribeToTopic(topic string, opts ...pubsub.SubOpt) (*pubsub.Subscription, error) {
	s.awaitStateInitialized() // Genesis time and genesis validator root are required to subscribe.

	topicHandle, err := s.JoinTopic(topic)
	if err != nil {
		return nil, err
	}
	scoringParams, err := s.topicScoreParams(topic)
	if err != nil {
		return nil, err
	}

	if scoringParams != nil {
		if err := topicHandle.SetScoreParams(scoringParams); err != nil {
			return nil, err
		}
		logGossipParameters(topic, scoringParams)
	}
	return topicHandle.Subscribe(opts...)
}

// peerInspector will scrape all the relevant scoring data and add it to our
// peer handler.
func (s *Service) peerInspector(peerMap map[peer.ID]*pubsub.PeerScoreSnapshot) {
	// Iterate through all the connected peers and through any of their
	// relevant topics.
	for pid, snap := range peerMap {
		s.peers.Scorers().GossipScorer().SetGossipData(pid, snap.Score,
			snap.BehaviourPenalty, convertTopicScores(snap.Topics))
	}
}

// Content addressable ID function.
//
// Ethereum Beacon Chain spec defines the message ID as:
//    The `message-id` of a gossipsub message MUST be the following 20 byte value computed from the message data:
//    If `message.data` has a valid snappy decompression, set `message-id` to the first 20 bytes of the `SHA256` hash of
//    the concatenation of `MESSAGE_DOMAIN_VALID_SNAPPY` with the snappy decompressed message data,
//    i.e. `SHA256(MESSAGE_DOMAIN_VALID_SNAPPY + snappy_decompress(message.data))[:20]`.
//
//    Otherwise, set `message-id` to the first 20 bytes of the `SHA256` hash of
//    the concatenation of `MESSAGE_DOMAIN_INVALID_SNAPPY` with the raw message data,
//    i.e. `SHA256(MESSAGE_DOMAIN_INVALID_SNAPPY + message.data)[:20]`.
func (s *Service) msgIDFunction(pmsg *pubsub_pb.Message) string {
	digest, err := ExtractGossipDigest(*pmsg.Topic)
	if err != nil {
		// Impossible condition that should
		// never be hit.
		msg := make([]byte, 20)
		copy(msg, "invalid")
		return string(msg)
	}
	_, fEpoch, err := p2putils.RetrieveForkDataFromDigest(digest, s.genesisValidatorsRoot)
	if err != nil {
		// Impossible condition that should
		// never be hit.
		msg := make([]byte, 20)
		copy(msg, "invalid")
		return string(msg)
	}
	if fEpoch >= params.BeaconConfig().AltairForkEpoch {
		return s.altairMsgID(pmsg)
	}
	decodedData, err := encoder.DecodeSnappy(pmsg.Data, params.BeaconNetworkConfig().GossipMaxSize)
	if err != nil {
		combinedData := append(params.BeaconNetworkConfig().MessageDomainInvalidSnappy[:], pmsg.Data...)
		h := hashutil.Hash(combinedData)
		return string(h[:20])
	}
	combinedData := append(params.BeaconNetworkConfig().MessageDomainValidSnappy[:], decodedData...)
	h := hashutil.Hash(combinedData)
	return string(h[:20])
}

// Spec:
// The derivation of the message-id has changed starting with Altair to incorporate the message topic along with the message data.
// These are fields of the Message Protobuf, and interpreted as empty byte strings if missing. The message-id MUST be the following
// 20 byte value computed from the message:
//
// If message.data has a valid snappy decompression, set message-id to the first 20 bytes of the SHA256 hash of the concatenation of
// the following data: MESSAGE_DOMAIN_VALID_SNAPPY, the length of the topic byte string (encoded as little-endian uint64), the topic
// byte string, and the snappy decompressed message data: i.e. SHA256(MESSAGE_DOMAIN_VALID_SNAPPY + uint_to_bytes(uint64(len(message.topic)))
// + message.topic + snappy_decompress(message.data))[:20]. Otherwise, set message-id to the first 20 bytes of the SHA256 hash of the concatenation
// of the following data: MESSAGE_DOMAIN_INVALID_SNAPPY, the length of the topic byte string (encoded as little-endian uint64),
// the topic byte string, and the raw message data: i.e. SHA256(MESSAGE_DOMAIN_INVALID_SNAPPY + uint_to_bytes(uint64(len(message.topic))) + message.topic + message.data)[:20].
func (s *Service) altairMsgID(pmsg *pubsub_pb.Message) string {
	topic := *pmsg.Topic
	topicLen := uint64(len(topic))
	topicLenBytes := bytesutil.Uint64ToBytesLittleEndian(topicLen)

	decodedData, err := encoder.DecodeSnappy(pmsg.Data, params.BeaconNetworkConfig().GossipMaxSize)
	if err != nil {
		totalLength := len(params.BeaconNetworkConfig().MessageDomainInvalidSnappy) + len(topicLenBytes) + int(topicLen) + len(pmsg.Data)
		combinedData := make([]byte, 0, totalLength)
		combinedData = append(combinedData, params.BeaconNetworkConfig().MessageDomainInvalidSnappy[:]...)
		combinedData = append(combinedData, topicLenBytes...)
		combinedData = append(combinedData, topic...)
		combinedData = append(combinedData, pmsg.Data...)
		h := hashutil.Hash(combinedData)
		return string(h[:20])
	}
	totalLength := len(params.BeaconNetworkConfig().MessageDomainValidSnappy) + len(topicLenBytes) + int(topicLen) + len(decodedData)
	combinedData := make([]byte, 0, totalLength)
	combinedData = append(combinedData, params.BeaconNetworkConfig().MessageDomainValidSnappy[:]...)
	combinedData = append(combinedData, topicLenBytes...)
	combinedData = append(combinedData, topic...)
	combinedData = append(combinedData, decodedData...)
	h := hashutil.Hash(combinedData)
	return string(h[:20])
}

func setPubSubParameters() {
	pubsub.GossipSubDlo = gossipSubDlo
	pubsub.GossipSubD = gossipSubD
	pubsub.GossipSubHeartbeatInterval = gossipSubHeartbeatInterval
	pubsub.GossipSubHistoryLength = gossipSubMcacheLen
	pubsub.GossipSubHistoryGossip = gossipSubMcacheGossip
	pubsub.TimeCacheDuration = 550 * gossipSubHeartbeatInterval

	// Set a larger gossip history to ensure that slower
	// messages have a longer time to be propagated. This
	// comes with the tradeoff of larger memory usage and
	// size of the seen message cache.
	if featureconfig.Get().EnableLargerGossipHistory {
		pubsub.GossipSubHistoryLength = 12
		pubsub.GossipSubHistoryLength = 5
	}
}

// convert from libp2p's internal schema to a compatible prysm protobuf format.
func convertTopicScores(topicMap map[string]*pubsub.TopicScoreSnapshot) map[string]*pbrpc.TopicScoreSnapshot {
	newMap := make(map[string]*pbrpc.TopicScoreSnapshot, len(topicMap))
	for t, s := range topicMap {
		newMap[t] = &pbrpc.TopicScoreSnapshot{
			TimeInMesh:               uint64(s.TimeInMesh.Milliseconds()),
			FirstMessageDeliveries:   float32(s.FirstMessageDeliveries),
			MeshMessageDeliveries:    float32(s.MeshMessageDeliveries),
			InvalidMessageDeliveries: float32(s.InvalidMessageDeliveries),
		}
	}
	return newMap
}

// Extracts the relevant fork digest from the gossip topic.
func ExtractGossipDigest(topic string) ([4]byte, error) {
	splitParts := strings.Split(topic, "/")
	parts := []string{}
	for _, p := range splitParts {
		if p == "" {
			continue
		}
		parts = append(parts, p)
	}
	if len(parts) < 2 {
		return [4]byte{}, errors.Wrapf(errInvalidTopic, "it only has %d parts: %v", len(parts), parts)
	}
	strDigest := parts[1]
	digest, err := hex.DecodeString(strDigest)
	if err != nil {
		return [4]byte{}, err
	}
	if len(digest) != digestLength {
		return [4]byte{}, errors.Errorf("invalid digest length wanted %d but got %d", digestLength, len(digest))
	}
	return bytesutil.ToBytes4(digest), nil
}
