package beaconv1

import (
	"context"
	"fmt"
	"strconv"

	"github.com/pkg/errors"
	types "github.com/prysmaticlabs/eth2-types"
	ethpb "github.com/prysmaticlabs/ethereumapis/eth/v1"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	iface "github.com/prysmaticlabs/prysm/beacon-chain/state/interface"
	"github.com/prysmaticlabs/prysm/proto/migration"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/prysmaticlabs/prysm/shared/params"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// GetValidator returns a validator specified by state and id or public key along with status and balance.
func (bs *Server) GetValidator(ctx context.Context, req *ethpb.StateValidatorRequest) (*ethpb.StateValidatorResponse, error) {
	state, err := bs.StateFetcher.State(ctx, req.StateId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Could not get state: %v", err)
	}
	if len(req.ValidatorId) == 0 {
		return nil, status.Error(codes.Internal, "Must request a validator id")
	}
	valContainer, err := valContainersByRequestIds(state, [][]byte{req.ValidatorId})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Could not get validator container: %v", err)
	}
	if len(valContainer) == 0 {
		return nil, status.Error(codes.NotFound, "Could not find validator")
	}
	return &ethpb.StateValidatorResponse{Data: valContainer[0]}, nil
}

// ListValidators returns filterable list of validators with their balance, status and index.
func (bs *Server) ListValidators(ctx context.Context, req *ethpb.StateValidatorsRequest) (*ethpb.StateValidatorsResponse, error) {
	state, err := bs.StateFetcher.State(ctx, req.StateId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Could not get state: %v", err)
	}

	valContainers, err := valContainersByRequestIds(state, req.Id)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Could not get validator container: %v", err)
	}

	if len(req.Status) == 0 {
		return &ethpb.StateValidatorsResponse{Data: valContainers}, nil
	}

	filterStatus := make(map[ethpb.ValidatorStatus]bool, len(req.Status))
	for _, ss := range req.Status {
		filterStatus[ss] = true
	}
	epoch := helpers.SlotToEpoch(state.Slot())
	filteredVals := make([]*ethpb.ValidatorContainer, 0, len(valContainers))
	for _, vc := range valContainers {
		valStatus, err := validatorStatus(vc.Validator, epoch)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "Could not get validator status: %v", err)
		}
		valSubStatus, err := validatorSubStatus(vc.Validator, epoch)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "Could not get validator sub status: %v", err)
		}
		if filterStatus[valStatus] || filterStatus[valSubStatus] {
			filteredVals = append(filteredVals, vc)
		}
	}
	return &ethpb.StateValidatorsResponse{Data: filteredVals}, nil
}

// ListValidatorBalances returns a filterable list of validator balances.
func (bs *Server) ListValidatorBalances(ctx context.Context, req *ethpb.ValidatorBalancesRequest) (*ethpb.ValidatorBalancesResponse, error) {
	state, err := bs.StateFetcher.State(ctx, req.StateId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Could not get state: %v", err)
	}

	valContainers, err := valContainersByRequestIds(state, req.Id)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Could not get validator: %v", err)
	}
	valBalances := make([]*ethpb.ValidatorBalance, len(valContainers))
	for i := 0; i < len(valContainers); i++ {
		valBalances[i] = &ethpb.ValidatorBalance{
			Index:   valContainers[i].Index,
			Balance: valContainers[i].Balance,
		}
	}
	return &ethpb.ValidatorBalancesResponse{Data: valBalances}, nil
}

// ListCommittees retrieves the committees for the given state at the given epoch.
// If the requested slot and index are defined, only those committees are returned.
func (bs *Server) ListCommittees(ctx context.Context, req *ethpb.StateCommitteesRequest) (*ethpb.StateCommitteesResponse, error) {
	state, err := bs.StateFetcher.State(ctx, req.StateId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Could not get state: %v", err)
	}

	epoch := helpers.SlotToEpoch(state.Slot())
	if req.Epoch != nil {
		epoch = *req.Epoch
	}
	activeCount, err := helpers.ActiveValidatorCount(state, epoch)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Could not get active validator count: %v", err)
	}

	startSlot, err := helpers.StartSlot(epoch)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Could not get epoch start slot: %v", err)
	}
	endSlot, err := helpers.EndSlot(epoch)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Could not get epoch end slot: %v", err)
	}
	committeesPerSlot := helpers.SlotCommitteeCount(activeCount)
	committees := make([]*ethpb.Committee, 0)
	for slot := startSlot; slot <= endSlot; slot++ {
		if req.Slot != nil && slot != *req.Slot {
			continue
		}
		for index := types.CommitteeIndex(0); index < types.CommitteeIndex(committeesPerSlot); index++ {
			if req.Index != nil && index != *req.Index {
				continue
			}
			committee, err := helpers.BeaconCommitteeFromState(state, slot, index)
			if err != nil {
				return nil, status.Errorf(codes.Internal, "Could not get committee: %v", err)
			}
			committeeContainer := &ethpb.Committee{
				Index:      index,
				Slot:       slot,
				Validators: committee,
			}
			committees = append(committees, committeeContainer)
		}
	}
	return &ethpb.StateCommitteesResponse{Data: committees}, nil
}

// This function returns the validator object based on the passed in ID. The validator ID could be its public key,
// or its index.
func valContainersByRequestIds(state iface.BeaconState, validatorIds [][]byte) ([]*ethpb.ValidatorContainer, error) {
	epoch := helpers.SlotToEpoch(state.Slot())
	allValidators := state.Validators()
	allBalances := state.Balances()
	var valContainers []*ethpb.ValidatorContainer
	if len(validatorIds) == 0 {
		valContainers = make([]*ethpb.ValidatorContainer, len(allValidators))
		for i, validator := range allValidators {
			v1Validator := migration.V1Alpha1ValidatorToV1(validator)
			subStatus, err := validatorSubStatus(v1Validator, epoch)
			if err != nil {
				return nil, fmt.Errorf("could not get validator sub status: %v", err)
			}
			valContainers[i] = &ethpb.ValidatorContainer{
				Index:     types.ValidatorIndex(i),
				Balance:   allBalances[i],
				Status:    subStatus,
				Validator: v1Validator,
			}
		}
	} else {
		valContainers = make([]*ethpb.ValidatorContainer, len(validatorIds))
		for i, validatorId := range validatorIds {
			var valIndex types.ValidatorIndex
			if len(validatorId) == params.BeaconConfig().BLSPubkeyLength {
				var ok bool
				valIndex, ok = state.ValidatorIndexByPubkey(bytesutil.ToBytes48(validatorId))
				if !ok {
					return nil, fmt.Errorf("could not find validator with public key: %#x", validatorId)
				}
			} else {
				index, err := strconv.ParseUint(string(validatorId), 10, 64)
				if err != nil {
					return nil, errors.Wrap(err, "could not decode validator id")
				}
				valIndex = types.ValidatorIndex(index)
			}
			v1Validator := migration.V1Alpha1ValidatorToV1(allValidators[valIndex])
			subStatus, err := validatorSubStatus(v1Validator, epoch)
			if err != nil {
				return nil, fmt.Errorf("could not get validator sub status: %v", err)
			}
			valContainers[i] = &ethpb.ValidatorContainer{
				Index:     valIndex,
				Balance:   allBalances[valIndex],
				Status:    subStatus,
				Validator: v1Validator,
			}
		}
	}
	return valContainers, nil
}

func validatorStatus(validator *ethpb.Validator, epoch types.Epoch) (ethpb.ValidatorStatus, error) {
	valStatus, err := validatorSubStatus(validator, epoch)
	if err != nil {
		return 0, errors.Wrap(err, "could not get sub status")
	}
	switch valStatus {
	case ethpb.ValidatorStatus_PENDING_INITIALIZED, ethpb.ValidatorStatus_PENDING_QUEUED:
		return ethpb.ValidatorStatus_PENDING, nil
	case ethpb.ValidatorStatus_ACTIVE_ONGOING, ethpb.ValidatorStatus_ACTIVE_SLASHED, ethpb.ValidatorStatus_ACTIVE_EXITING:
		return ethpb.ValidatorStatus_ACTIVE, nil
	case ethpb.ValidatorStatus_EXITED_UNSLASHED, ethpb.ValidatorStatus_EXITED_SLASHED:
		return ethpb.ValidatorStatus_EXITED, nil
	case ethpb.ValidatorStatus_WITHDRAWAL_POSSIBLE, ethpb.ValidatorStatus_WITHDRAWAL_DONE:
		return ethpb.ValidatorStatus_WITHDRAWAL, nil
	}
	return 0, errors.New("no valid status found")
}

func validatorSubStatus(validator *ethpb.Validator, epoch types.Epoch) (ethpb.ValidatorStatus, error) {
	if validator == nil {
		return 0, errors.New("validator is nil")
	}
	farFutureEpoch := params.BeaconConfig().FarFutureEpoch

	// Pending.
	if validator.ActivationEpoch > epoch {
		if validator.ActivationEligibilityEpoch == farFutureEpoch {
			return ethpb.ValidatorStatus_PENDING_INITIALIZED, nil
		} else if validator.ActivationEligibilityEpoch < farFutureEpoch && validator.ActivationEpoch > epoch {
			return ethpb.ValidatorStatus_PENDING_QUEUED, nil
		}
	}

	// Active.
	if validator.ActivationEpoch <= epoch && epoch < validator.ExitEpoch {
		if validator.ExitEpoch == farFutureEpoch {
			return ethpb.ValidatorStatus_ACTIVE_ONGOING, nil
		} else if validator.ExitEpoch < farFutureEpoch {
			if validator.Slashed {
				return ethpb.ValidatorStatus_ACTIVE_SLASHED, nil
			}
			return ethpb.ValidatorStatus_ACTIVE_EXITING, nil
		}
	}

	// Exited.
	if validator.ExitEpoch <= epoch && epoch < validator.WithdrawableEpoch {
		if validator.Slashed {
			return ethpb.ValidatorStatus_EXITED_SLASHED, nil
		}
		return ethpb.ValidatorStatus_EXITED_UNSLASHED, nil
	}

	if validator.WithdrawableEpoch <= epoch {
		if validator.EffectiveBalance != 0 {
			return ethpb.ValidatorStatus_WITHDRAWAL_POSSIBLE, nil
		} else {
			return ethpb.ValidatorStatus_WITHDRAWAL_DONE, nil
		}
	}

	return 0, errors.New("no valid sub status found")
}
