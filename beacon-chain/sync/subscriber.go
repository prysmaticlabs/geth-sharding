package sync

import (
	"context"
	"fmt"
	"reflect"
	"runtime/debug"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/libp2p/go-libp2p-core/peer"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	pb "github.com/prysmaticlabs/ethereumapis/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/feed"
	statefeed "github.com/prysmaticlabs/prysm/beacon-chain/core/feed/state"
	"github.com/prysmaticlabs/prysm/beacon-chain/p2p"
	"github.com/prysmaticlabs/prysm/shared/featureconfig"
	"github.com/prysmaticlabs/prysm/shared/messagehandler"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/roughtime"
	"github.com/prysmaticlabs/prysm/shared/traceutil"
	"go.opencensus.io/trace"
)

const pubsubMessageTimeout = 30 * time.Second

// subHandler represents handler for a given subscription.
type subHandler func(context.Context, proto.Message) error

// noopValidator is a no-op that only decodes the message, but does not check its contents.
func (r *Service) noopValidator(ctx context.Context, _ peer.ID, msg *pubsub.Message) bool {
	m, err := r.decodePubsubMessage(msg)
	if err != nil {
		log.WithError(err).Error("Failed to decode message")
		return false
	}
	msg.ValidatorData = m
	return true
}

// Register PubSub subscribers
func (r *Service) registerSubscribers() {
	go func() {
		// Wait until chain start.
		stateChannel := make(chan *feed.Event, 1)
		stateSub := r.stateNotifier.StateFeed().Subscribe(stateChannel)
		defer stateSub.Unsubscribe()
		for r.chainStarted == false {
			select {
			case event := <-stateChannel:
				if event.Type == statefeed.Initialized {
					data, ok := event.Data.(*statefeed.InitializedData)
					if !ok {
						log.Error("Event feed data is not type *statefeed.InitializedData")
						return
					}
					log.WithField("starttime", data.StartTime).Debug("Received state initialized event")
					if data.StartTime.After(roughtime.Now()) {
						stateSub.Unsubscribe()
						time.Sleep(roughtime.Until(data.StartTime))
					}
					r.chainStarted = true
				}
			case <-r.ctx.Done():
				log.Debug("Context closed, exiting goroutine")
				return
			case err := <-stateSub.Err():
				log.WithError(err).Error("Subscription to state notifier failed")
				return
			}
		}
	}()
	r.subscribe(
		"/eth2/beacon_block",
		r.validateBeaconBlockPubSub,
		r.beaconBlockSubscriber,
	)
	r.subscribe(
		"/eth2/beacon_aggregate_and_proof",
		r.validateAggregateAndProof,
		r.beaconAggregateProofSubscriber,
	)
	r.subscribe(
		"/eth2/voluntary_exit",
		r.validateVoluntaryExit,
		r.voluntaryExitSubscriber,
	)
	r.subscribe(
		"/eth2/proposer_slashing",
		r.validateProposerSlashing,
		r.proposerSlashingSubscriber,
	)
	r.subscribe(
		"/eth2/attester_slashing",
		r.validateAttesterSlashing,
		r.attesterSlashingSubscriber,
	)
	if featureconfig.Get().EnableDynamicCommitteeSubnets {
		r.subscribeDynamicWithSubnets(
			"/eth2/committee_index%d_beacon_attestation",
			r.committeeIndices,                          /* determineSubsLen */
			r.validateCommitteeIndexBeaconAttestation,   /* validator */
			r.committeeIndexBeaconAttestationSubscriber, /* message handler */
		)
	} else {
		r.subscribeDynamic(
			"/eth2/committee_index%d_beacon_attestation",
			r.committeesCount,                           /* determineSubsLen */
			r.validateCommitteeIndexBeaconAttestation,   /* validator */
			r.committeeIndexBeaconAttestationSubscriber, /* message handler */
		)
	}
}

// subscribe to a given topic with a given validator and subscription handler.
// The base protobuf message is used to initialize new messages for decoding.
func (r *Service) subscribe(topic string, validator pubsub.Validator, handle subHandler) *pubsub.Subscription {
	base := p2p.GossipTopicMappings[topic]
	if base == nil {
		panic(fmt.Sprintf("%s is not mapped to any message in GossipTopicMappings", topic))
	}
	return r.subscribeWithBase(base, topic, validator, handle)
}

func (r *Service) subscribeWithBase(base proto.Message, topic string, validator pubsub.Validator, handle subHandler) *pubsub.Subscription {
	topic += r.p2p.Encoding().ProtocolSuffix()
	log := log.WithField("topic", topic)

	if err := r.p2p.PubSub().RegisterTopicValidator(wrapAndReportValidation(topic, validator)); err != nil {
		log.WithError(err).Error("Failed to register validator")
	}

	sub, err := r.p2p.PubSub().Subscribe(topic)
	if err != nil {
		// Any error subscribing to a PubSub topic would be the result of a misconfiguration of
		// libp2p PubSub library. This should not happen at normal runtime, unless the config
		// changes to a fatal configuration.
		panic(err)
	}

	// Pipeline decodes the incoming subscription data, runs the validation, and handles the
	// message.
	pipeline := func(msg *pubsub.Message) {
		ctx, _ := context.WithTimeout(context.Background(), pubsubMessageTimeout)
		ctx, span := trace.StartSpan(ctx, "sync.pubsub")
		defer span.End()

		defer func() {
			if r := recover(); r != nil {
				traceutil.AnnotateError(span, fmt.Errorf("panic occurred: %v", r))
				log.WithField("error", r).Error("Panic occurred")
				debug.PrintStack()
			}
		}()

		span.AddAttributes(trace.StringAttribute("topic", topic))

		if msg.ValidatorData == nil {
			log.Error("Received nil message on pubsub")
			messageFailedProcessingCounter.WithLabelValues(topic).Inc()
			return
		}

		if err := handle(ctx, msg.ValidatorData.(proto.Message)); err != nil {
			traceutil.AnnotateError(span, err)
			log.WithError(err).Error("Failed to handle p2p pubsub")
			messageFailedProcessingCounter.WithLabelValues(topic).Inc()
			return
		}
	}

	// The main message loop for receiving incoming messages from this subscription.
	messageLoop := func() {
		for {
			msg, err := sub.Next(r.ctx)
			if err != nil {
				// This should only happen when the context is cancelled or subscription is cancelled.
				log.WithError(err).Error("Subscription next failed")
				return
			}

			if msg.ReceivedFrom == r.p2p.PeerID() {
				continue
			}

			go pipeline(msg)
		}
	}

	go messageLoop()
	return sub
}

// Wrap the pubsub validator with a metric monitoring function. This function increments the
// appropriate counter if the particular message fails to validate.
func wrapAndReportValidation(topic string, v pubsub.Validator) (string, pubsub.Validator) {
	return topic, func(ctx context.Context, pid peer.ID, msg *pubsub.Message) bool {
		defer messagehandler.HandlePanic(ctx, msg)
		ctx, _ = context.WithTimeout(ctx, pubsubMessageTimeout)
		messageReceivedCounter.WithLabelValues(topic).Inc()
		b := v(ctx, pid, msg)
		if !b {
			messageFailedValidationCounter.WithLabelValues(topic).Inc()
		}
		return b
	}
}

// subscribe to a dynamically changing list of subnets. This method expects a fmt compatible
// string for the topic name and the list of subnets for subscribed topics that should be
// maintained.
func (r *Service) subscribeDynamicWithSubnets(
	topicFormat string,
	determineSubIndices func() []uint64,
	validate pubsub.Validator,
	handle subHandler,
) {
	base := p2p.GossipTopicMappings[topicFormat]
	if base == nil {
		panic(fmt.Sprintf("%s is not mapped to any message in GossipTopicMappings", topicFormat))
	}

	subscriptions := make(map[uint64]*pubsub.Subscription, params.BeaconConfig().MaxCommitteesPerSlot)

	stateChannel := make(chan *feed.Event, 1)
	stateSub := r.stateNotifier.StateFeed().Subscribe(stateChannel)
	go func() {
		for {
			select {
			case <-r.ctx.Done():
				stateSub.Unsubscribe()
				return
			case <-stateChannel:
				if r.chainStarted && r.initialSync.Syncing() {
					continue
				}
				// Update desired topic indices.
				wantedSubs := determineSubIndices()
				// Resize as appropriate.
				for k, v := range subscriptions {
					var wanted bool
					for _, idx := range wantedSubs {
						if k == idx {
							wanted = true
							break
						}
					}
					if !wanted && v != nil {
						v.Cancel()
						if err := r.p2p.PubSub().UnregisterTopicValidator(fmt.Sprintf(topicFormat, k)); err != nil {
							log.WithError(err).Error("Failed to unregister topic validator")
						}
						delete(subscriptions, k)
					}
				}
				for _, idx := range wantedSubs {
					if _, exists := subscriptions[idx]; !exists {
						// do not subscribe if we have no peers in the same
						// subnet
						topic := p2p.GossipTypeMapping[reflect.TypeOf(&pb.Attestation{})]
						subnetTopic := fmt.Sprintf(topic, idx)
						numOfPeers := r.p2p.PubSub().ListPeers(subnetTopic)
						if len(r.p2p.Peers().SubscribedToSubnet(idx)) == 0 && len(numOfPeers) == 0 {
							log.Debugf("No peers found subscribed to attestation gossip subnet with "+
								"committee index %d. Searching network for peers subscribed to the subnet.", idx)
							go func(idx uint64) {
								peerExists, err := r.p2p.FindPeersWithSubnet(idx)
								if err != nil {
									log.Errorf("Could not search for peers: %v", err)
									return
								}
								// do not subscribe if we couldn't find a connected peer.
								if !peerExists {
									return
								}
								subscriptions[idx] = r.subscribeWithBase(base, subnetTopic, validate, handle)
							}(idx)
							continue
						}
						subscriptions[idx] = r.subscribeWithBase(base, subnetTopic, validate, handle)
					}
				}
			}
		}
	}()
}

// subscribe to a dynamically increasing index of topics. This method expects a fmt compatible
// string for the topic name and a maxID to represent the number of subscribed topics that should be
// maintained. As the state feed emits a newly updated state, the maxID function will be called to
// determine the appropriate number of topics. This method supports only sequential number ranges
// for topics.
func (r *Service) subscribeDynamic(topicFormat string, determineSubsLen func() int, validate pubsub.Validator, handle subHandler) {
	base := p2p.GossipTopicMappings[topicFormat]
	if base == nil {
		panic(fmt.Sprintf("%s is not mapped to any message in GossipTopicMappings", topicFormat))
	}

	var subscriptions []*pubsub.Subscription

	stateChannel := make(chan *feed.Event, 1)
	stateSub := r.stateNotifier.StateFeed().Subscribe(stateChannel)
	go func() {
		for {
			select {
			case <-r.ctx.Done():
				stateSub.Unsubscribe()
				return
			case <-stateChannel:
				if r.chainStarted && r.initialSync.Syncing() {
					continue
				}
				// Update topic count.
				wantedSubs := determineSubsLen()
				// Resize as appropriate.
				if len(subscriptions) > wantedSubs { // Reduce topics
					var cancelSubs []*pubsub.Subscription
					subscriptions, cancelSubs = subscriptions[:wantedSubs-1], subscriptions[wantedSubs:]
					for i, sub := range cancelSubs {
						sub.Cancel()
						if err := r.p2p.PubSub().UnregisterTopicValidator(fmt.Sprintf(topicFormat, i+wantedSubs)); err != nil {
							log.WithError(err).Error("Failed to unregister topic validator")
						}
					}
				} else if len(subscriptions) < wantedSubs { // Increase topics
					for i := len(subscriptions); i < wantedSubs; i++ {
						sub := r.subscribeWithBase(base, fmt.Sprintf(topicFormat, i), validate, handle)
						subscriptions = append(subscriptions, sub)
					}
				}
			}
		}
	}()
}
