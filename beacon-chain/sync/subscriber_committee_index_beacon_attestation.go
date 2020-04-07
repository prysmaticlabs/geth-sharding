package sync

import (
	"context"
	"errors"
	"fmt"

	"github.com/gogo/protobuf/proto"
	eth "github.com/prysmaticlabs/ethereumapis/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/beacon-chain/cache"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/feed"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/feed/operation"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	"github.com/prysmaticlabs/prysm/shared/params"
)

func (r *Service) committeeIndexBeaconAttestationSubscriber(ctx context.Context, msg proto.Message) error {
	a, ok := msg.(*eth.Attestation)
	if !ok {
		return fmt.Errorf("message was not type *eth.Attestation, type=%T", msg)
	}

	if a.Data == nil {
		return errors.New("nil attestation")
	}
	r.setSeenCommitteeIndicesSlot(a.Data.Slot, a.Data.CommitteeIndex, a.AggregationBits)

	if exists, _ := r.attPool.HasAggregatedAttestation(a); exists {
		return nil
	}

	// Broadcast the unaggregated attestation on a feed to notify other services in the beacon node
	// of a received unaggregated attestation.
	r.attestationNotifier.OperationFeed().Send(&feed.Event{
		Type: operation.UnaggregatedAttReceived,
		Data: &operation.UnAggregatedAttReceivedData{
			Attestation: a,
		},
	})

	return r.attPool.SaveUnaggregatedAttestation(a)
}

func (r *Service) committeesCount() int {
	activeValidatorIndices, err := r.chain.HeadValidatorsIndices(helpers.SlotToEpoch(r.chain.HeadSlot()))
	if err != nil {
		panic(err)
	}
	return int(helpers.SlotCommitteeCount(uint64(len(activeValidatorIndices))))
}

func (r *Service) aggregatorCommitteeIndices(currentSlot uint64) []uint64 {
	endEpoch := helpers.SlotToEpoch(currentSlot) + 1
	endSlot := endEpoch * params.BeaconConfig().SlotsPerEpoch
	commIds := []uint64{}
	for i := currentSlot; i <= endSlot; i++ {
		commIds = append(commIds, cache.CommitteeIDs.GetAggregatorCommitteeIDs(i)...)
	}
	return commIds
}

func (r *Service) attesterCommitteeIndices(currentSlot uint64) []uint64 {
	endEpoch := helpers.SlotToEpoch(currentSlot) + 1
	endSlot := endEpoch * params.BeaconConfig().SlotsPerEpoch
	commIds := []uint64{}
	for i := currentSlot; i <= endSlot; i++ {
		commIds = append(commIds, cache.CommitteeIDs.GetAttesterCommitteeIDs(i)...)
	}
	return commIds
}
