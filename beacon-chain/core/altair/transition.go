package altair

import (
	"context"

	"github.com/pkg/errors"
	e "github.com/prysmaticlabs/prysm/beacon-chain/core/epoch"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/epoch/precompute"
	iface "github.com/prysmaticlabs/prysm/beacon-chain/state/interface"
	"go.opencensus.io/trace"
)

// ProcessEpoch describes the per epoch operations that are performed on the beacon state.
// It's optimized by pre computing validator attested info and epoch total/attested balances upfront.
//
// Spec code:
// def process_epoch(state: BeaconState) -> None:
//    process_justification_and_finalization(state)  # [Modified in Altair]
//    process_inactivity_updates(state)  # [New in Altair]
//    process_rewards_and_penalties(state)  # [Modified in Altair]
//    process_registry_updates(state)
//    process_slashings(state)  # [Modified in Altair]
//    process_eth1_data_reset(state)
//    process_effective_balance_updates(state)
//    process_slashings_reset(state)
//    process_randao_mixes_reset(state)
//    process_historical_roots_update(state)
//    process_participation_flag_updates(state)  # [New in Altair]
//    process_sync_committee_updates(state)  # [New in Altair]
func ProcessEpoch(ctx context.Context, state iface.BeaconStateAltair) (iface.BeaconStateAltair, error) {
	ctx, span := trace.StartSpan(ctx, "altair.ProcessEpoch")
	defer span.End()

	if state == nil || state.IsNil() {
		return nil, errors.New("nil state")
	}
	vp, bp, err := InitializeEpochValidators(ctx, state)
	if err != nil {
		return nil, err
	}

	// New in Altair.
	vp, bp, err = ProcessEpochParticipation(ctx, state, bp, vp)
	if err != nil {
		return nil, err
	}

	state, err = precompute.ProcessJustificationAndFinalizationPreCompute(state, bp)
	if err != nil {
		return nil, errors.Wrap(err, "could not process justification")
	}

	// New in Altair.
	state, vp, err = ProcessInactivityScores(ctx, state, vp)
	if err != nil {
		return nil, errors.Wrap(err, "could not process inactivity updates")
	}

	// New in Altair.
	state, err = ProcessRewardsAndPenaltiesPrecompute(state, bp, vp)
	if err != nil {
		return nil, errors.Wrap(err, "could not process rewards and penalties")
	}

	state, err = e.ProcessRegistryUpdates(state)
	if err != nil {
		return nil, errors.Wrap(err, "could not process registry updates")
	}

	// Modified in Altair.
	state, err = ProcessSlashings(state)
	if err != nil {
		return nil, err
	}

	state, err = e.ProcessEth1DataReset(state)
	if err != nil {
		return nil, err
	}
	state, err = e.ProcessEffectiveBalanceUpdates(state)
	if err != nil {
		return nil, err
	}
	state, err = e.ProcessSlashingsReset(state)
	if err != nil {
		return nil, err
	}
	state, err = e.ProcessRandaoMixesReset(state)
	if err != nil {
		return nil, err
	}
	state, err = e.ProcessHistoricalRootsUpdate(state)
	if err != nil {
		return nil, err
	}

	// New in Altair.
	state, err = ProcessParticipationFlagUpdates(state)
	if err != nil {
		return nil, err
	}

	// New in Altair.
	state, err = ProcessSyncCommitteeUpdates(state)
	if err != nil {
		return nil, err
	}

	return state, nil
}
