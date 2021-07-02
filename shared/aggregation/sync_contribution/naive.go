package sync_contribution

import (
	v2 "github.com/prysmaticlabs/prysm/proto/prysm/v2"
	"github.com/prysmaticlabs/prysm/shared/aggregation"
	"github.com/prysmaticlabs/prysm/shared/bls"
	"github.com/prysmaticlabs/prysm/shared/copyutil"
)

// naiveSyncContributionAggregation aggregates naively, without any complex algorithms or optimizations.
// Note: this is currently a naive implementation to the order of O(mn^2).
func naiveSyncContributionAggregation(contributions []*v2.SyncCommitteeContribution) ([]*v2.SyncCommitteeContribution, error) {
	if len(contributions) <= 1 {
		return contributions, nil
	}

	// Naive aggregation. O(n^2) time.
	for i, a := range contributions {
		if i >= len(contributions) {
			break
		}
		for j := i + 1; j < len(contributions); j++ {
			b := contributions[j]
			if a.AggregationBits.Len() == b.AggregationBits.Len() && !a.AggregationBits.Overlaps(b.AggregationBits) {
				var err error
				a, err = aggregate(a, b)
				if err != nil {
					return nil, err
				}
				// Delete b
				contributions = append(contributions[:j], contributions[j+1:]...)
				j--
				contributions[i] = a
			}
		}
	}

	// Naive deduplication of identical contributions. O(n^2) time.
	for i, a := range contributions {
		for j := i + 1; j < len(contributions); j++ {
			b := contributions[j]

			if a.AggregationBits.Len() != b.AggregationBits.Len() {
				continue
			}

			if a.AggregationBits.Contains(b.AggregationBits) {
				// If b is fully contained in a, then b can be removed.
				contributions = append(contributions[:j], contributions[j+1:]...)
				j--
			} else if b.AggregationBits.Contains(a.AggregationBits) {
				// if a is fully contained in b, then a can be removed.
				contributions = append(contributions[:i], contributions[i+1:]...)
				break // Stop the inner loop, advance a.
			}
		}
	}

	return contributions, nil
}

// aggregates pair of sync contributions c1 and c2 together.
func aggregate(c1, c2 *v2.SyncCommitteeContribution) (*v2.SyncCommitteeContribution, error) {
	if c1.AggregationBits.Overlaps(c2.AggregationBits) {
		return nil, aggregation.ErrBitsOverlap
	}

	baseContribution := copyutil.CopySyncCommitteeContribution(c1)
	newContribution := copyutil.CopySyncCommitteeContribution(c2)
	if newContribution.AggregationBits.Count() > baseContribution.AggregationBits.Count() {
		baseContribution, newContribution = newContribution, baseContribution
	}

	if baseContribution.AggregationBits.Contains(newContribution.AggregationBits) {
		return baseContribution, nil
	}

	newBits := baseContribution.AggregationBits.Or(newContribution.AggregationBits)
	newSig, err := bls.SignatureFromBytes(newContribution.Signature)
	if err != nil {
		return nil, err
	}
	baseSig, err := bls.SignatureFromBytes(baseContribution.Signature)
	if err != nil {
		return nil, err
	}

	aggregatedSig := bls.AggregateSignatures([]bls.Signature{baseSig, newSig})
	baseContribution.Signature = aggregatedSig.Marshal()
	baseContribution.AggregationBits = newBits

	return baseContribution, nil
}
