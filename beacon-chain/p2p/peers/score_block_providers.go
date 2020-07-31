package peers

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/prysmaticlabs/prysm/beacon-chain/flags"
)

const (
	// DefaultBlockProviderStartScore defines initial score before any stats update takes place.
	// By setting this to positive value, peers are given a chance to be used for the first time.
	DefaultBlockProviderStartScore = 0.1
	// DefaultBlockProviderProcessedBatchWeight is a default weight of a processed batch of blocks.
	DefaultBlockProviderProcessedBatchWeight = 0.02
	// DefaultBlockProviderDecayInterval defines how often to call a decaying routine.
	DefaultBlockProviderDecayInterval = 5 * time.Minute
)

// BlockProviderScorer represents block provider scoring service.
type BlockProviderScorer struct {
	ctx    context.Context
	config *BlockProviderScorerConfig
	store  *peerDataStore
	// totalProcessedBlocks is a sum of all processed blocks, across all peers.
	totalProcessedBlocks uint64
}

// BlockProviderScorerConfig holds configuration parameters for block providers scoring service.
type BlockProviderScorerConfig struct {
	// StartScore defines initial score from which peer starts. Set to positive to give peers an
	// opportunity to be selected for block fetching (allows new peers to start participating,
	// when there are already scored peers).
	StartScore float64
	// ProcessedBatchWeight defines a reward for a single processed batch of blocks.
	ProcessedBatchWeight float64
	// DecayInterval defines how often requested/returned/processed stats should be decayed.
	DecayInterval time.Duration
	// Decay specifies number of blocks subtracted from stats on each decay step.
	Decay uint64
}

// newBlockProviderScorer creates block provider scoring service.
func newBlockProviderScorer(
	ctx context.Context, store *peerDataStore, config *BlockProviderScorerConfig) *BlockProviderScorer {
	if config == nil {
		config = &BlockProviderScorerConfig{}
	}
	scorer := &BlockProviderScorer{
		ctx:    ctx,
		config: config,
		store:  store,
	}
	if scorer.config.StartScore == 0.0 {
		scorer.config.StartScore = DefaultBlockProviderStartScore
	}
	if scorer.config.ProcessedBatchWeight == 0.0 {
		scorer.config.ProcessedBatchWeight = DefaultBlockProviderProcessedBatchWeight
	}
	if scorer.config.DecayInterval == 0 {
		scorer.config.DecayInterval = DefaultBlockProviderDecayInterval
	}
	if scorer.config.Decay == 0 {
		scorer.config.Decay = uint64(flags.Get().BlockBatchLimit)
	}
	return scorer
}

// Score calculates and returns total score based on returned and processed blocks.
func (s *BlockProviderScorer) Score(pid peer.ID) float64 {
	s.store.RLock()
	defer s.store.RUnlock()
	return s.score(pid)
}

// score is a lock-free version of Score.
func (s *BlockProviderScorer) score(pid peer.ID) float64 {
	score := s.Params().StartScore
	peerData, ok := s.store.peers[pid]
	if ok && peerData.processedBlocks > 0 {
		processedBatches := float64(peerData.processedBlocks / uint64(flags.Get().BlockBatchLimit))
		score += processedBatches * s.config.ProcessedBatchWeight
	} else {
		// Boost peers that have never been selected.
		return s.MaxScore()
	}
	return math.Round(score*ScoreRoundingFactor) / ScoreRoundingFactor
}

// Params exposes scorer's parameters.
func (s *BlockProviderScorer) Params() *BlockProviderScorerConfig {
	return s.config
}

// IncrementProcessedBlocks increments the number of blocks that have been successfully processed.
func (s *BlockProviderScorer) IncrementProcessedBlocks(pid peer.ID, cnt uint64) {
	s.store.Lock()
	defer s.store.Unlock()

	if _, ok := s.store.peers[pid]; !ok {
		s.store.peers[pid] = &peerData{}
	}
	s.store.peers[pid].processedBlocks += cnt
	s.totalProcessedBlocks += cnt
}

// ProcessedBlocks returns number of peer returned blocks that are successfully processed.
func (s *BlockProviderScorer) ProcessedBlocks(pid peer.ID) uint64 {
	s.store.RLock()
	defer s.store.RUnlock()
	return s.processedBlocks(pid)
}

// processedBlocks is a lock-free version of ProcessedBlocks.
func (s *BlockProviderScorer) processedBlocks(pid peer.ID) uint64 {
	if peerData, ok := s.store.peers[pid]; ok {
		return peerData.processedBlocks
	}
	return 0
}

// Decay updates block provider counters by decaying them.
// This urges peers to keep up the performance to get a high score (and allows new peers to contest previously high
// scoring ones).
func (s *BlockProviderScorer) Decay() {
	s.store.Lock()
	defer s.store.Unlock()

	for _, peerData := range s.store.peers {
		if peerData.processedBlocks > s.config.Decay {
			peerData.processedBlocks -= s.config.Decay
			s.totalProcessedBlocks -= s.config.Decay
		}
	}
}

// Sorted returns list of block providers sorted by score in descending order.
func (s *BlockProviderScorer) Sorted(pids []peer.ID) []peer.ID {
	s.store.Lock()
	defer s.store.Unlock()

	if len(pids) == 0 {
		return pids
	}
	scores := make(map[peer.ID]float64, len(pids))
	peers := make([]peer.ID, len(pids))
	for i, pid := range pids {
		scores[pid] = s.score(pid)
		peers[i] = pid
	}
	sort.Slice(peers, func(i, j int) bool {
		return scores[peers[i]] > scores[peers[j]]
	})
	return peers
}

// BlockProviderScorePretty returns full scoring information about a given peer.
func (s *BlockProviderScorer) BlockProviderScorePretty(pid peer.ID) string {
	s.store.Lock()
	defer s.store.Unlock()
	score := s.score(pid)
	return fmt.Sprintf("[%0.2f%%, raw: %v,  blocks: %d]", (score/s.MaxScore())*100, score, s.processedBlocks(pid))
}

// MaxScore exposes maximum score attainable by peers.
func (s *BlockProviderScorer) MaxScore() float64 {
	totalProcessedBatches := float64(s.totalProcessedBlocks / uint64(flags.Get().BlockBatchLimit))
	score := s.Params().StartScore + totalProcessedBatches*s.config.ProcessedBatchWeight
	return math.Round(score*ScoreRoundingFactor) / ScoreRoundingFactor
}
