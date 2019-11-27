package precompute_test

import (
	"context"
	"reflect"
	"testing"

	ethpb "github.com/prysmaticlabs/ethereumapis/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/epoch/precompute"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	"github.com/prysmaticlabs/prysm/shared/params"
)

func TestNew(t *testing.T) {
	ffe := params.BeaconConfig().FarFutureEpoch
	s := &pb.BeaconState{
		Slot: params.BeaconConfig().SlotsPerEpoch,
		// Validator 0 is slashed
		// Validator 1 is withdrawable
		// Validator 2 is active prev epoch and current epoch
		// Validator 3 is active prev epoch
		Validators: []*ethpb.Validator{
			{Slashed: true, WithdrawableEpoch: ffe, EffectiveBalance: 100},
			{EffectiveBalance: 100},
			{WithdrawableEpoch: ffe, ExitEpoch: ffe, EffectiveBalance: 100},
			{WithdrawableEpoch: ffe, ExitEpoch: 1, EffectiveBalance: 100},
		},
	}
	e := params.BeaconConfig().FarFutureEpoch
	v, b := precompute.New(context.Background(), s)
	if !reflect.DeepEqual(v[0], &precompute.Validator{IsSlashed: true, CurrentEpochEffectiveBalance: 100,
		InclusionDistance: e, InclusionSlot: e}) {
		t.Error("Incorrect validator 0 status")
	}
	if !reflect.DeepEqual(v[1], &precompute.Validator{IsWithdrawableCurrentEpoch: true, CurrentEpochEffectiveBalance: 100,
		InclusionDistance: e, InclusionSlot: e}) {
		t.Error("Incorrect validator 1 status")
	}
	if !reflect.DeepEqual(v[2], &precompute.Validator{IsActiveCurrentEpoch: true, IsActivePrevEpoch: true,
		CurrentEpochEffectiveBalance: 100, InclusionDistance: e, InclusionSlot: e}) {
		t.Error("Incorrect validator 2 status")
	}
	if !reflect.DeepEqual(v[3], &precompute.Validator{IsActivePrevEpoch: true, CurrentEpochEffectiveBalance: 100,
		InclusionDistance: e, InclusionSlot: e}) {
		t.Error("Incorrect validator 3 status")
	}

	wantedBalances := &precompute.Balance{
		CurrentEpoch: 100,
		PrevEpoch:    200,
	}
	if !reflect.DeepEqual(b, wantedBalances) {
		t.Error("Incorrect wanted balance")
	}
}
