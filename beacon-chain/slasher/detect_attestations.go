package slasher

import (
	"context"

	types "github.com/prysmaticlabs/eth2-types"
)

// Process queued attestations every time an epoch ticker fires. We retrieve
// these attestations from a queue, then group them all by validator chunk index.
// This grouping will allow us to perform detection on batches of attestations
// per validator chunk index which can be done concurrently.
func (s *Service) processQueuedAttestations(ctx context.Context, epochTicker <-chan uint64) {
	for {
		select {
		case currentEpoch := <-epochTicker:
			s.queueLock.Lock()
			atts := s.attestationQueue
			s.attestationQueue = make([]*compactAttestation, 0)
			s.queueLock.Unlock()
			log.Infof("Epoch %d reached, processing %d queued atts for slashing detection", currentEpoch, len(atts))
			groupedAtts := s.groupByValidatorChunkIndex(atts)
			for validatorChunkIdx, attsBatch := range groupedAtts {
				s.detectAttestationBatch(attsBatch, validatorChunkIdx, types.Epoch(currentEpoch))
			}
		case <-ctx.Done():
			return
		}
	}
}

// Given a list of attestations all corresponding to a validator chunk index as well
// as the current epoch in time, we perform slashing detection over the batch.
// TODO(#8331): Implement.
func (s *Service) detectAttestationBatch(
	atts []*compactAttestation, validatorChunkIndex uint64, currentEpoch types.Epoch,
) {

}

// Group a list of attestations into batches by validator chunk index.
// This way, we can detect on the batch of attestations for each validator chunk index
// concurrently, and also allowing us to effectively use a single 2D chunk
// for slashing detection through this logical grouping.
func (s *Service) groupByValidatorChunkIndex(
	attestations []*compactAttestation,
) map[uint64][]*compactAttestation {
	groupedAttestations := make(map[uint64][]*compactAttestation)
	for _, att := range attestations {
		validatorChunkIndices := make(map[uint64]bool)
		for _, validatorIdx := range att.attestingIndices {
			validatorChunkIndex := s.params.validatorChunkIndex(types.ValidatorIndex(validatorIdx))
			validatorChunkIndices[validatorChunkIndex] = true
		}
		for validatorChunkIndex := range validatorChunkIndices {
			groupedAttestations[validatorChunkIndex] = append(
				groupedAttestations[validatorChunkIndex],
				att,
			)
		}
	}
	return groupedAttestations
}
