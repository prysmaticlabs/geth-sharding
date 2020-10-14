package p2p

import (
	"fmt"
	"strings"

	"github.com/libp2p/go-libp2p-core/peer"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	pubsubpb "github.com/libp2p/go-libp2p-pubsub/pb"
	"github.com/prysmaticlabs/prysm/beacon-chain/p2p/encoder"
)

var _ pubsub.SubscriptionFilter = (*Service)(nil)

const pubsubSubscriptionRequestLimit = 100

// CanSubscribe returns true if the topic is of interest and we could subscribe to it.
func (s *Service) CanSubscribe(topic string) bool {
	if !s.isInitialized() {
		return false
	}
	parts := strings.Split(topic, "/")
	if len(parts) != 5 {
		return false
	}
	// The topic must start with a slash, which means the first part will be empty.
	if parts[0] != "" {
		return false
	}
	if parts[1] != "eth2" {
		return false
	}
	fd, err := s.forkDigest()
	if err != nil {
		log.WithError(err).Error("Could not determine fork digest")
		return false
	}
	if parts[2] != fmt.Sprintf("%x", fd) {
		return false
	}
	if parts[4] != encoder.ProtocolSuffixSSZSnappy {
		return false
	}

	// Check the incoming topic matches any topic mapping.
	for gt := range GossipTopicMappings {
		if _, err := scanfcheck(strings.Join(parts[0:4], "/"), gt); err == nil {
			return true
		}
	}

	return false
}

// FilterIncomingSubscriptions is invoked for all RPCs containing subscription notifications.
// This method returns only the topics of interest and may return an error if the subscription
// request contains too many topics.
func (sf *Service) FilterIncomingSubscriptions(_ peer.ID, subs []*pubsubpb.RPC_SubOpts) ([]*pubsubpb.RPC_SubOpts, error) {
	if len(subs) > pubsubSubscriptionRequestLimit {
		return nil, pubsub.ErrTooManySubscriptions
	}

	return pubsub.FilterSubscriptions(subs, sf.CanSubscribe), nil
}


// scanfcheck uses fmt.Sscanf to check that a given string matches expected format. This method
// returns the number of formatting substitutions matched and error if the string does not match
// the expected format. Note: this method only accepts integer compatible formatting substitutions
// such as %d or %x.
func scanfcheck(input, format string) (int, error) {
	var t int
	// Sscanf requires argument pointers with the appropriate type to load the value from the input.
	// This method only checks that the input conforms to the format, the arguments are not used and
	// therefore we can reuse the same integer pointer.
	var cnt = strings.Count(format, "%")
	var args = []interface{}{}
	for i := 0; i < cnt; i++ {
		args = append(args, &t)
	}
	return fmt.Sscanf(input, format, args...)
}
