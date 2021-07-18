package altair

import (
	"context"

	"github.com/pkg/errors"
	types "github.com/prysmaticlabs/eth2-types"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/blocks"
	iface "github.com/prysmaticlabs/prysm/beacon-chain/state/interface"
	ethpb "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/prysmaticlabs/prysm/shared/params"
)

// ProcessDeposits processes validator deposits for beacon state Altair.
func ProcessDeposits(
	ctx context.Context,
	beaconState iface.BeaconStateAltair,
	deposits []*ethpb.Deposit,
) (iface.BeaconStateAltair, error) {
	batchVerified, err := blocks.BatchVerifyDepositsSignatures(ctx, deposits)
	if err != nil {
		return nil, err
	}

	for _, deposit := range deposits {
		if deposit == nil || deposit.Data == nil {
			return nil, errors.New("got a nil deposit in block")
		}
		beaconState, err = ProcessDeposit(ctx, beaconState, deposit, batchVerified)
		if err != nil {
			return nil, errors.Wrapf(err, "could not process deposit from %#x", bytesutil.Trunc(deposit.Data.PublicKey))
		}
	}
	return beaconState, nil
}

// ProcessDeposit processes validator deposit for beacon state Altair.
func ProcessDeposit(ctx context.Context, beaconState iface.BeaconStateAltair, deposit *ethpb.Deposit, verifySignature bool) (iface.BeaconStateAltair, error) {
	beaconState, err := blocks.ProcessDeposit(beaconState, deposit, verifySignature)
	if err != nil {
		return nil, err
	}

	// The last validator in the beacon state validator registry.
	v, err := beaconState.ValidatorAtIndexReadOnly(types.ValidatorIndex(beaconState.NumValidators() - 1))
	if err != nil {
		return nil, err
	}
	// We know a validator is brand new when its status epochs are all far future epoch.
	// In this case, we append 0 to inactivity score and participation bits.
	if v.ActivationEligibilityEpoch() == v.ActivationEpoch() &&
		v.ActivationEpoch() == v.ExitEpoch() &&
		v.ExitEpoch() == v.WithdrawableEpoch() &&
		v.WithdrawableEpoch() == params.BeaconConfig().FarFutureEpoch {
		if err := beaconState.AppendInactivityScore(0); err != nil {
			return nil, err
		}
		if err := beaconState.AppendPreviousParticipationBits(0); err != nil {
			return nil, err
		}
		if err := beaconState.AppendCurrentParticipationBits(0); err != nil {
			return nil, err
		}
	}

	return beaconState, nil
}
