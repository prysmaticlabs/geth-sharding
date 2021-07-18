package sync

import (
	"reflect"
	"strings"

	ssz "github.com/ferranbt/fastssz"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/pkg/errors"
	"github.com/prysmaticlabs/prysm/beacon-chain/p2p"
	eth "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	prysmv2 "github.com/prysmaticlabs/prysm/proto/prysm/v2"
	"google.golang.org/protobuf/proto"
)

var errNilPubsubMessage = errors.New("nil pubsub message")
var errInvalidTopic = errors.New("invalid topic format")

func (s *Service) decodePubsubMessage(msg *pubsub.Message) (ssz.Unmarshaler, error) {
	if msg == nil || msg.Topic == nil || *msg.Topic == "" {
		return nil, errNilPubsubMessage
	}
	topic := *msg.Topic
	fDigest, err := p2p.ExtractGossipDigest(topic)
	if err != nil {
		return nil, errors.Wrapf(err, "extraction failed for topic: %s", topic)
	}
	topic = strings.TrimSuffix(topic, s.cfg.P2P.Encoding().ProtocolSuffix())
	topic, err = s.replaceForkDigest(topic)
	if err != nil {
		return nil, err
	}
	// Specially handle subnet messages.
	switch {
	case strings.Contains(topic, p2p.GossipAttestationMessage):
		topic = p2p.GossipTypeMapping[reflect.TypeOf(&eth.Attestation{})]
		// Given that both sync message related subnets have the same message name, we have to
		// differentiate them below.
	case strings.Contains(topic, p2p.GossipSyncCommitteeMessage) && !strings.Contains(topic, p2p.SyncContributionAndProofSubnetTopicFormat):
		topic = p2p.GossipTypeMapping[reflect.TypeOf(&prysmv2.SyncCommitteeMessage{})]
	}

	base := p2p.GossipTopicMappings(topic, 0)
	if base == nil {
		return nil, p2p.ErrMessageNotMapped
	}
	m, ok := proto.Clone(base).(ssz.Unmarshaler)
	if !ok {
		return nil, errors.Errorf("message of %T does not support marshaller interface", base)
	}
	// Handle different message types across forks.
	if topic == p2p.BlockSubnetTopicFormat {
		m, err = extractBlockDataType(fDigest[:], s.cfg.Chain)
		if err != nil {
			return nil, err
		}
	}
	if err := s.cfg.P2P.Encoding().DecodeGossip(msg.Data, m); err != nil {
		return nil, err
	}
	return m, nil
}

// Replaces our fork digest with the formatter.
func (s *Service) replaceForkDigest(topic string) (string, error) {
	subStrings := strings.Split(topic, "/")
	if len(subStrings) != 4 {
		return "", errInvalidTopic
	}
	subStrings[2] = "%x"
	return strings.Join(subStrings, "/"), nil
}
