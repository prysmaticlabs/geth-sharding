package validator

import (
	"context"
	"sort"

	types "github.com/prysmaticlabs/eth2-types"
	"github.com/prysmaticlabs/go-bitfield"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/altair"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/blocks"
	iface "github.com/prysmaticlabs/prysm/beacon-chain/state/interface"
	ethpb "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/shared/aggregation"
	"github.com/prysmaticlabs/prysm/shared/featureconfig"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/version"
)

type proposerAtts []*ethpb.Attestation

// filter separates attestation list into two groups: valid and invalid attestations.
// The first group passes the all the required checks for attestation to be considered for proposing.
// And attestations from the second group should be deleted.
func (a proposerAtts) filter(ctx context.Context, state iface.BeaconState) (proposerAtts, proposerAtts) {
	validAtts := make([]*ethpb.Attestation, 0, len(a))
	invalidAtts := make([]*ethpb.Attestation, 0, len(a))
	var attestationProcessor func(context.Context, iface.BeaconState, *ethpb.Attestation) (iface.BeaconState, error)
	switch state.Version() {
	case version.Phase0:
		attestationProcessor = blocks.ProcessAttestationNoVerifySignature
	case version.Altair:
		// Use a wrapper here, as go needs strong typing for the function signature.
		attestationProcessor = func(ctx context.Context, state iface.BeaconState, attestation *ethpb.Attestation) (iface.BeaconState, error) {
			return altair.ProcessAttestationNoVerifySignature(ctx, state, attestation)
		}
	default:
		// Exit early if there is an unknown state type.
		return validAtts, invalidAtts
	}
	for _, att := range a {
		if _, err := attestationProcessor(ctx, state, att); err == nil {
			validAtts = append(validAtts, att)
			continue
		}
		invalidAtts = append(invalidAtts, att)
	}
	return validAtts, invalidAtts
}

// sortByProfitability orders attestations by highest slot and by highest aggregation bit count.
func (a proposerAtts) sortByProfitability() (proposerAtts, error) {
	if len(a) < 2 {
		return a, nil
	}
	if featureconfig.Get().ProposerAttsSelectionUsingMaxCover {
		return a.sortByProfitabilityUsingMaxCover()
	}
	sort.Slice(a, func(i, j int) bool {
		if a[i].Data.Slot == a[j].Data.Slot {
			return a[i].AggregationBits.Count() > a[j].AggregationBits.Count()
		}
		return a[i].Data.Slot > a[j].Data.Slot
	})
	return a, nil
}

// sortByProfitabilityUsingMaxCover orders attestations by highest slot and by highest aggregation bit count.
// Duplicate bits are counted only once, using max-cover algorithm.
func (a proposerAtts) sortByProfitabilityUsingMaxCover() (proposerAtts, error) {
	// Separate attestations by slot, as slot number takes higher precedence when sorting.
	var slots []types.Slot
	attsBySlot := map[types.Slot]proposerAtts{}
	for _, att := range a {
		if _, ok := attsBySlot[att.Data.Slot]; !ok {
			slots = append(slots, att.Data.Slot)
		}
		attsBySlot[att.Data.Slot] = append(attsBySlot[att.Data.Slot], att)
	}

	selectAtts := func(atts proposerAtts) (proposerAtts, error) {
		if len(atts) < 2 {
			return atts, nil
		}
		candidates := make([]*bitfield.Bitlist64, len(atts))
		for i := 0; i < len(atts); i++ {
			var err error
			candidates[i], err = atts[i].AggregationBits.ToBitlist64()
			if err != nil {
				return nil, err
			}
		}
		// Add selected candidates on top, those that are not selected - append at bottom.
		selectedKeys, _, err := aggregation.MaxCover(candidates, len(candidates), true /* allowOverlaps */)
		if err == nil {
			// Pick selected attestations first, leftover attestations will be appended at the end.
			// Both lists will be sorted by number of bits set.
			selectedAtts := make(proposerAtts, selectedKeys.Count())
			leftoverAtts := make(proposerAtts, selectedKeys.Not().Count())
			for i, key := range selectedKeys.BitIndices() {
				selectedAtts[i] = atts[key]
			}
			for i, key := range selectedKeys.Not().BitIndices() {
				leftoverAtts[i] = atts[key]
			}
			sort.Slice(selectedAtts, func(i, j int) bool {
				return selectedAtts[i].AggregationBits.Count() > selectedAtts[j].AggregationBits.Count()
			})
			sort.Slice(leftoverAtts, func(i, j int) bool {
				return leftoverAtts[i].AggregationBits.Count() > leftoverAtts[j].AggregationBits.Count()
			})
			return append(selectedAtts, leftoverAtts...), nil
		}
		return atts, nil
	}

	// Select attestations. Slots are sorted from higher to lower at this point. Within slots attestations
	// are sorted to maximize profitability (greedily selected, with previous attestations' bits
	// evaluated before including any new attestation).
	var sortedAtts proposerAtts
	sort.Slice(slots, func(i, j int) bool {
		return slots[i] > slots[j]
	})
	for _, slot := range slots {
		selected, err := selectAtts(attsBySlot[slot])
		if err != nil {
			return nil, err
		}
		sortedAtts = append(sortedAtts, selected...)
	}

	return sortedAtts, nil
}

// limitToMaxAttestations limits attestations to maximum attestations per block.
func (a proposerAtts) limitToMaxAttestations() proposerAtts {
	if uint64(len(a)) > params.BeaconConfig().MaxAttestations {
		return a[:params.BeaconConfig().MaxAttestations]
	}
	return a
}

// dedup removes duplicate attestations (ones with the same bits set on).
// Important: not only exact duplicates are removed, but proper subsets are removed too
// (their known bits are redundant and are already contained in their supersets).
func (a proposerAtts) dedup() (proposerAtts, error) {
	if len(a) < 2 {
		return a, nil
	}
	attsByDataRoot := make(map[[32]byte][]*ethpb.Attestation, len(a))
	for _, att := range a {
		attDataRoot, err := att.Data.HashTreeRoot()
		if err != nil {
			continue
		}
		attsByDataRoot[attDataRoot] = append(attsByDataRoot[attDataRoot], att)
	}

	uniqAtts := make([]*ethpb.Attestation, 0, len(a))
	for _, atts := range attsByDataRoot {
		for i := 0; i < len(atts); i++ {
			a := atts[i]
			for j := i + 1; j < len(atts); j++ {
				b := atts[j]
				if c, err := a.AggregationBits.Contains(b.AggregationBits); err != nil {
					return nil, err
				} else if c {
					// a contains b, b is redundant.
					atts[j] = atts[len(atts)-1]
					atts[len(atts)-1] = nil
					atts = atts[:len(atts)-1]
					j--
				} else if c, err := b.AggregationBits.Contains(a.AggregationBits); err != nil {
					return nil, err
				} else if c {
					// b contains a, a is redundant.
					atts[i] = atts[len(atts)-1]
					atts[len(atts)-1] = nil
					atts = atts[:len(atts)-1]
					i--
					break
				}
			}
		}
		uniqAtts = append(uniqAtts, atts...)
	}

	return uniqAtts, nil
}
