package evaluators

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	"github.com/prysmaticlabs/prysm/endtoend/policies"
	"github.com/prysmaticlabs/prysm/endtoend/types"
	eth "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	prysmv2 "github.com/prysmaticlabs/prysm/proto/prysm/v2"
	"github.com/prysmaticlabs/prysm/shared/params"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
)

var expectedParticipation = 0.95 // 95% participation to make room for minor issues.

var expectedSyncParticipation = 0.90 // 90% participation for sync committee members.

// ValidatorsAreActive ensures the expected amount of validators are active.
var ValidatorsAreActive = types.Evaluator{
	Name:       "validators_active_epoch_%d",
	Policy:     policies.AllEpochs,
	Evaluation: validatorsAreActive,
}

// ValidatorsParticipating ensures the expected amount of validators are active.
var ValidatorsParticipating = types.Evaluator{
	Name:       "validators_participating_epoch_%d",
	Policy:     policies.AfterNthEpoch(2),
	Evaluation: validatorsParticipating,
}

// ValidatorSyncParticipation ensures the expected amount of sync committee participants
// are active.
var ValidatorSyncParticipation = types.Evaluator{
	Name:       "validator_sync_participation_%d",
	Policy:     policies.AfterNthEpoch(params.AltairE2EForkEpoch - 1),
	Evaluation: validatorsSyncParticipation,
}

func validatorsAreActive(conns ...*grpc.ClientConn) error {
	conn := conns[0]
	client := eth.NewBeaconChainClient(conn)
	// Balances actually fluctuate but we just want to check initial balance.
	validatorRequest := &eth.ListValidatorsRequest{
		PageSize: int32(params.BeaconConfig().MinGenesisActiveValidatorCount),
		Active:   true,
	}
	validators, err := client.ListValidators(context.Background(), validatorRequest)
	if err != nil {
		return errors.Wrap(err, "failed to get validators")
	}

	expectedCount := params.BeaconConfig().MinGenesisActiveValidatorCount
	receivedCount := uint64(len(validators.ValidatorList))
	if expectedCount != receivedCount {
		return fmt.Errorf("expected validator count to be %d, recevied %d", expectedCount, receivedCount)
	}

	effBalanceLowCount := 0
	exitEpochWrongCount := 0
	withdrawEpochWrongCount := 0
	for _, item := range validators.ValidatorList {
		if valExited && item.Index == exitedIndex {
			continue
		}
		if item.Validator.EffectiveBalance < params.BeaconConfig().MaxEffectiveBalance {
			effBalanceLowCount++
		}
		if item.Validator.ExitEpoch != params.BeaconConfig().FarFutureEpoch {
			exitEpochWrongCount++
		}
		if item.Validator.WithdrawableEpoch != params.BeaconConfig().FarFutureEpoch {
			withdrawEpochWrongCount++
		}
	}

	if effBalanceLowCount > 0 {
		return fmt.Errorf(
			"%d validators did not have genesis validator effective balance of %d",
			effBalanceLowCount,
			params.BeaconConfig().MaxEffectiveBalance,
		)
	} else if exitEpochWrongCount > 0 {
		return fmt.Errorf("%d validators did not have genesis validator exit epoch of far future epoch", exitEpochWrongCount)
	} else if withdrawEpochWrongCount > 0 {
		return fmt.Errorf("%d validators did not have genesis validator withdrawable epoch of far future epoch", withdrawEpochWrongCount)
	}

	return nil
}

// validatorsParticipating ensures the validators have an acceptable participation rate.
func validatorsParticipating(conns ...*grpc.ClientConn) error {
	conn := conns[0]
	client := eth.NewBeaconChainClient(conn)
	validatorRequest := &eth.GetValidatorParticipationRequest{}
	participation, err := client.GetValidatorParticipation(context.Background(), validatorRequest)
	if err != nil {
		return errors.Wrap(err, "failed to get validator participation")
	}

	partRate := participation.Participation.GlobalParticipationRate
	expected := float32(expectedParticipation)
	if partRate < expected {
		return fmt.Errorf(
			"validator participation was below for epoch %d, expected %f, received: %f",
			participation.Epoch,
			expected,
			partRate,
		)
	}
	return nil
}

// validatorsSyncParticipation ensures the validators have an acceptable participation rate for
// sync committee assignments.
func validatorsSyncParticipation(conns ...*grpc.ClientConn) error {
	conn := conns[0]
	client := eth.NewNodeClient(conn)
	altairClient := prysmv2.NewBeaconChainAltairClient(conn)
	genesis, err := client.GetGenesis(context.Background(), &emptypb.Empty{})
	if err != nil {
		return errors.Wrap(err, "failed to get genesis data")
	}
	currSlot := helpers.CurrentSlot(uint64(genesis.GenesisTime.AsTime().Unix()))
	currEpoch := helpers.SlotToEpoch(currSlot)
	lowestBound := currEpoch - 1

	// TODO: Fix Sync Participation in fork epoch.
	if currEpoch == params.AltairE2EForkEpoch {
		return nil
	}
	// TODO: Fix Sync Participation in fork epoch to allow
	// blocks in the fork epoch from being evaluated.
	if lowestBound == params.AltairE2EForkEpoch {
		lowestBound++
	}

	if lowestBound < params.AltairE2EForkEpoch {
		lowestBound = params.AltairE2EForkEpoch
	}
	blockCtrs, err := altairClient.ListBlocks(context.Background(), &eth.ListBlocksRequest{QueryFilter: &eth.ListBlocksRequest_Epoch{Epoch: lowestBound}})
	if err != nil {
		return errors.Wrap(err, "failed to get validator participation")
	}
	for _, ctr := range blockCtrs.BlockContainers {
		if ctr.GetAltairBlock() == nil {
			return errors.Errorf("Altair block type doesn't exist for block at epoch %d", lowestBound)
		}
		blk := ctr.GetAltairBlock()
		if blk.Block == nil || blk.Block.Body == nil || blk.Block.Body.SyncAggregate == nil {
			return errors.New("nil block provided")
		}
		syncAgg := blk.Block.Body.SyncAggregate
		threshold := uint64(float64(syncAgg.SyncCommitteeBits.Len()) * expectedSyncParticipation)
		if syncAgg.SyncCommitteeBits.Count() < threshold {
			return errors.Errorf("In block of slot %d ,the aggregate bitvector with length of %d only got a count of %d", blk.Block.Slot, threshold, syncAgg.SyncCommitteeBits.Count())
		}
	}
	if lowestBound == currEpoch {
		return nil
	}
	blockCtrs, err = altairClient.ListBlocks(context.Background(), &eth.ListBlocksRequest{QueryFilter: &eth.ListBlocksRequest_Epoch{Epoch: currEpoch}})
	if err != nil {
		return errors.Wrap(err, "failed to get validator participation")
	}
	for _, ctr := range blockCtrs.BlockContainers {
		if ctr.GetAltairBlock() == nil {
			return errors.Errorf("Altair block type doesn't exist for block at epoch %d", lowestBound)
		}
		blk := ctr.GetAltairBlock()
		if blk.Block == nil || blk.Block.Body == nil || blk.Block.Body.SyncAggregate == nil {
			return errors.New("nil block provided")
		}
		syncAgg := blk.Block.Body.SyncAggregate
		threshold := uint64(float64(syncAgg.SyncCommitteeBits.Len()) * expectedSyncParticipation)
		if syncAgg.SyncCommitteeBits.Count() < threshold {
			return errors.Errorf("In block of slot %d ,the aggregate bitvector with length of %d only got a count of %d", blk.Block.Slot, threshold, syncAgg.SyncCommitteeBits.Count())
		}
	}
	return nil
}
