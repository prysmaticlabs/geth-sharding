package epoch

import (
	"github.com/pkg/errors"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	ethpb "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
)

// ComputeValidatorParticipation by matching validator attestations during the epoch,
// computing the attesting balance, and how much attested compared to the total balances.
func ComputeValidatorParticipation(state *pb.BeaconState) (*ethpb.ValidatorParticipation, error) {
	currentEpoch := helpers.SlotToEpoch(state.Slot)
	finalized := currentEpoch == state.FinalizedCheckpoint.Epoch

	atts, err := MatchAttestations(state, currentEpoch)
	if err != nil {
		return nil, errors.Wrap(err, "could not retrieve head attestations")
	}
	attestedBalances, err := AttestingBalance(state, atts.Target)
	if err != nil {
		return nil, errors.Wrap(err, "could not retrieve attested balances")
	}
	totalBalances, err := helpers.TotalActiveBalance(state)
	if err != nil {
		return nil, errors.Wrap(err, "could not retrieve total balances")
	}
	return &ethpb.ValidatorParticipation{
		Epoch:                   currentEpoch,
		Finalized:               finalized,
		GlobalParticipationRate: float32(attestedBalances) / float32(totalBalances),
		VotedEther:              attestedBalances,
		EligibleEther:           totalBalances,
	}, nil
}
