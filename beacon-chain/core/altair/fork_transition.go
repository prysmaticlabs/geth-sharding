package altair

import (
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	iface "github.com/prysmaticlabs/prysm/beacon-chain/state/interface"
	statealtair "github.com/prysmaticlabs/prysm/beacon-chain/state/v2"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	"github.com/prysmaticlabs/prysm/shared/attestationutil"
	"github.com/prysmaticlabs/prysm/shared/params"
)

// UpgradeToAltair updates input state to return the version Altair state.
func UpgradeToAltair(state iface.BeaconState) (iface.BeaconStateAltair, error) {
	epoch := helpers.CurrentEpoch(state)

	s := &pb.BeaconStateAltair{
		GenesisTime:           state.GenesisTime(),
		GenesisValidatorsRoot: state.GenesisValidatorRoot(),
		Slot:                  state.Slot(),
		Fork: &pb.Fork{
			PreviousVersion: state.Fork().CurrentVersion,
			CurrentVersion:  params.BeaconConfig().AltairForkVersion,
			Epoch:           epoch,
		},
		LatestBlockHeader:           state.LatestBlockHeader(),
		BlockRoots:                  state.BlockRoots(),
		StateRoots:                  state.StateRoots(),
		HistoricalRoots:             state.HistoricalRoots(),
		Eth1Data:                    state.Eth1Data(),
		Eth1DataVotes:               state.Eth1DataVotes(),
		Eth1DepositIndex:            state.Eth1DepositIndex(),
		Validators:                  state.Validators(),
		Balances:                    state.Balances(),
		RandaoMixes:                 state.RandaoMixes(),
		Slashings:                   state.Slashings(),
		PreviousEpochParticipation:  make([]byte, state.NumValidators()),
		CurrentEpochParticipation:   make([]byte, state.NumValidators()),
		JustificationBits:           state.JustificationBits(),
		PreviousJustifiedCheckpoint: state.PreviousJustifiedCheckpoint(),
		CurrentJustifiedCheckpoint:  state.CurrentJustifiedCheckpoint(),
		FinalizedCheckpoint:         state.FinalizedCheckpoint(),
		InactivityScores:            make([]uint64, state.NumValidators()),
	}

	newState, err := statealtair.InitializeFromProto(s)
	if err != nil {
		return nil, err
	}

	prevEpochAtts, err := state.PreviousEpochAttestations()
	if err != nil {
		return nil, err
	}
	newState, err = TranslateParticipation(newState, prevEpochAtts)
	if err != nil {
		return nil, err
	}

	committee, err := NextSyncCommittee(newState)
	if err != nil {
		return nil, err
	}
	if err := newState.SetCurrentSyncCommittee(committee); err != nil {
		return nil, err
	}
	if err := newState.SetNextSyncCommittee(committee); err != nil {
		return nil, err
	}
	return newState, nil
}

// TranslateParticipation translates pending attestations into participation bits, then inserts the bits into beacon state.
// This is helper function t o convert phase 0 beacon state(pending attestations) to Altair beacon state(participation bits).
func TranslateParticipation(state *statealtair.BeaconState, atts []*pb.PendingAttestation) (*statealtair.BeaconState, error) {
	for _, att := range atts {
		epochParticipation, err := state.PreviousEpochParticipation()
		if err != nil {
			return nil, err
		}

		participatedFlags, err := attestationParticipationFlagIndices(state, att.Data, att.InclusionDelay)
		if err != nil {
			return nil, err
		}
		committee, err := helpers.BeaconCommitteeFromState(state, att.Data.Slot, att.Data.CommitteeIndex)
		if err != nil {
			return nil, err
		}
		indices, err := attestationutil.AttestingIndices(att.AggregationBits, committee)
		if err != nil {
			return nil, err
		}
		sourceFlagIndex := params.BeaconConfig().TimelySourceFlagIndex
		targetFlagIndex := params.BeaconConfig().TimelyTargetFlagIndex
		headFlagIndex := params.BeaconConfig().TimelyHeadFlagIndex
		for _, index := range indices {
			if participatedFlags[sourceFlagIndex] && !HasValidatorFlag(epochParticipation[index], sourceFlagIndex) {
				epochParticipation[index] = AddValidatorFlag(epochParticipation[index], sourceFlagIndex)
			}
			if participatedFlags[targetFlagIndex] && !HasValidatorFlag(epochParticipation[index], targetFlagIndex) {
				epochParticipation[index] = AddValidatorFlag(epochParticipation[index], targetFlagIndex)
			}
			if participatedFlags[headFlagIndex] && !HasValidatorFlag(epochParticipation[index], headFlagIndex) {
				epochParticipation[index] = AddValidatorFlag(epochParticipation[index], headFlagIndex)
			}
		}

		if err := state.SetPreviousParticipationBits(epochParticipation); err != nil {
			return nil, err
		}
	}
	return state, nil
}
