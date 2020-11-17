package scorers

import (
	"context"
	"math"
	"time"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/prysmaticlabs/prysm/beacon-chain/p2p/peers/peerdata"
)

var _ Scorer = (*Service)(nil)

// ScoreRoundingFactor defines how many digits to keep in decimal part.
// This parameter is used in math.Round(score*ScoreRoundingFactor) / ScoreRoundingFactor.
const ScoreRoundingFactor = 10000

// Scorer defines minimum set of methods every peer scorer must expose.
type Scorer interface {
	Score(pid peer.ID) float64
	IsBadPeer(pid peer.ID) bool
	BadPeers() []peer.ID
}

// Service manages peer scorers that are used to calculate overall peer score.
type Service struct {
	store   *peerdata.Store
	scorers struct {
		badResponsesScorer  *BadResponsesScorer
		blockProviderScorer *BlockProviderScorer
	}
	weights     map[Scorer]float64
	totalWeight float64
}

// Config holds configuration parameters for scoring service.
type Config struct {
	BadResponsesScorerConfig  *BadResponsesScorerConfig
	BlockProviderScorerConfig *BlockProviderScorerConfig
}

// NewService provides fully initialized peer scoring service.
func NewService(ctx context.Context, store *peerdata.Store, config *Config) *Service {
	s := &Service{
		store:   store,
		weights: make(map[Scorer]float64),
	}

	// Register scorers.
	s.scorers.badResponsesScorer = newBadResponsesScorer(store, config.BadResponsesScorerConfig)
	s.setScorerWeight(s.scorers.badResponsesScorer, 1.0)
	s.scorers.blockProviderScorer = newBlockProviderScorer(store, config.BlockProviderScorerConfig)
	s.setScorerWeight(s.scorers.blockProviderScorer, 1.0)

	// Start background tasks.
	go s.loop(ctx)

	return s
}

// BadResponsesScorer exposes bad responses scoring service.
func (s *Service) BadResponsesScorer() *BadResponsesScorer {
	return s.scorers.badResponsesScorer
}

// BlockProviderScorer exposes block provider scoring service.
func (s *Service) BlockProviderScorer() *BlockProviderScorer {
	return s.scorers.blockProviderScorer
}

// ActiveScorersCount returns number of scorers that can affect score (have non-zero weight).
func (s *Service) ActiveScorersCount() int {
	cnt := 0
	for _, w := range s.weights {
		if w > 0 {
			cnt++
		}
	}
	return cnt
}

// Score returns calculated peer score across all tracked metrics.
func (s *Service) Score(pid peer.ID) float64 {
	s.store.RLock()
	defer s.store.RUnlock()

	score := float64(0)
	if _, ok := s.store.PeerData(pid); !ok {
		return 0
	}
	score += s.scorers.badResponsesScorer.score(pid) * s.scorerWeight(s.scorers.badResponsesScorer)
	score += s.scorers.blockProviderScorer.score(pid) * s.scorerWeight(s.scorers.blockProviderScorer)
	return math.Round(score*ScoreRoundingFactor) / ScoreRoundingFactor
}

// IsBadPeer traverses all the scorers to see if any of them classifies peer as bad.
func (s *Service) IsBadPeer(pid peer.ID) bool {
	s.store.RLock()
	defer s.store.RUnlock()
	return s.isBadPeer(pid)
}

// isBadPeer is a lock-free version of isBadPeer.
func (s *Service) isBadPeer(pid peer.ID) bool {
	return s.scorers.badResponsesScorer.isBadPeer(pid)
}

// BadPeers returns the peers that are considered bad by any of registered scorers.
func (s *Service) BadPeers() []peer.ID {
	s.store.RLock()
	defer s.store.RUnlock()

	badPeers := make([]peer.ID, 0)
	for pid := range s.store.Peers() {
		if s.isBadPeer(pid) {
			badPeers = append(badPeers, pid)
		}
	}
	return badPeers
}

// loop handles background tasks.
func (s *Service) loop(ctx context.Context) {
	decayBadResponsesStats := time.NewTicker(s.scorers.badResponsesScorer.Params().DecayInterval)
	defer decayBadResponsesStats.Stop()
	decayBlockProviderStats := time.NewTicker(s.scorers.blockProviderScorer.Params().DecayInterval)
	defer decayBlockProviderStats.Stop()

	for {
		select {
		case <-decayBadResponsesStats.C:
			s.scorers.badResponsesScorer.Decay()
		case <-decayBlockProviderStats.C:
			s.scorers.blockProviderScorer.Decay()
		case <-ctx.Done():
			return
		}
	}
}

// setScorerWeight adds scorer to map of known scorers.
func (s *Service) setScorerWeight(scorer Scorer, weight float64) {
	s.weights[scorer] = weight
	s.totalWeight += s.weights[scorer]
}

// scorerWeight calculates contribution percentage of a given scorer in total score.
func (s *Service) scorerWeight(scorer Scorer) float64 {
	return s.weights[scorer] / s.totalWeight
}
