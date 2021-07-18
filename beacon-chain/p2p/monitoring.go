package p2p

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	p2pPeerCount = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "p2p_peer_count",
		Help: "The number of peers in a given state.",
	},
		[]string{"state"})
	repeatPeerConnections = promauto.NewCounter(prometheus.CounterOpts{
		Name: "p2p_repeat_attempts",
		Help: "The number of repeat attempts the connection handler is triggered for a peer.",
	})
	savedAttestationBroadcasts = promauto.NewCounter(prometheus.CounterOpts{
		Name: "p2p_attestation_subnet_recovered_broadcasts",
		Help: "The number of attestations that were attempted to be broadcast with no peers on " +
			"the subnet. The beacon node increments this counter when the broadcast is blocked " +
			"until a subnet peer can be found.",
	})
	attestationBroadcastAttempts = promauto.NewCounter(prometheus.CounterOpts{
		Name: "p2p_attestation_subnet_attempted_broadcasts",
		Help: "The number of attestations that were attempted to be broadcast.",
	})
	savedSyncCommitteeBroadcasts = promauto.NewCounter(prometheus.CounterOpts{
		Name: "p2p_sync_committee_subnet_recovered_broadcasts",
		Help: "The number of sync committee messages that were attempted to be broadcast with no peers on " +
			"the subnet. The beacon node increments this counter when the broadcast is blocked " +
			"until a subnet peer can be found.",
	})
	syncCommitteeBroadcastAttempts = promauto.NewCounter(prometheus.CounterOpts{
		Name: "p2p_sync_committee_subnet_attempted_broadcasts",
		Help: "The number of sync committee that were attempted to be broadcast.",
	})
)

func (s *Service) updateMetrics() {
	p2pPeerCount.WithLabelValues("Connected").Set(float64(len(s.peers.Connected())))
	p2pPeerCount.WithLabelValues("Disconnected").Set(float64(len(s.peers.Disconnected())))
	p2pPeerCount.WithLabelValues("Connecting").Set(float64(len(s.peers.Connecting())))
	p2pPeerCount.WithLabelValues("Disconnecting").Set(float64(len(s.peers.Disconnecting())))
	p2pPeerCount.WithLabelValues("Bad").Set(float64(len(s.peers.Bad())))
}
