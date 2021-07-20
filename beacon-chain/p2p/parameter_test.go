package p2p

import (
	"testing"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/prysmaticlabs/prysm/shared/testutil/assert"
)

func TestOverlayParameters(t *testing.T) {
	pms := pubsubGossipParam()
	assert.Equal(t, gossipSubD, pms.D, "gossipSubD")
	assert.Equal(t, gossipSubDlo, pms.Dlo, "gossipSubDlo")
	assert.Equal(t, gossipSubDhi, pms.Dhi, "gossipSubDhi")
}

func TestGossipParameters(t *testing.T) {
	setPubSubParameters()
	pms := pubsubGossipParam()
	assert.Equal(t, gossipSubMcacheLen, pms.HistoryLength, "gossipSubMcacheLen")
	assert.Equal(t, gossipSubMcacheGossip, pms.HistoryGossip, "gossipSubMcacheGossip")
	assert.Equal(t, gossipSubSeenTTL, int(pubsub.TimeCacheDuration.Milliseconds()/pms.HeartbeatInterval.Milliseconds()), "gossipSubSeenTtl")
}

func TestFanoutParameters(t *testing.T) {
	pms := pubsubGossipParam()
	if pms.FanoutTTL != gossipSubFanoutTTL {
		t.Errorf("gossipSubFanoutTTL, wanted: %d, got: %d", gossipSubFanoutTTL, pms.FanoutTTL)
	}
}

func TestHeartbeatParameters(t *testing.T) {
	pms := pubsubGossipParam()
	if pms.HeartbeatInterval != gossipSubHeartbeatInterval {
		t.Errorf("gossipSubHeartbeatInterval, wanted: %d, got: %d", gossipSubHeartbeatInterval, pms.HeartbeatInterval)
	}
}

func TestMiscParameters(t *testing.T) {
	setPubSubParameters()
	assert.Equal(t, randomSubD, pubsub.RandomSubD, "randomSubD")
}
