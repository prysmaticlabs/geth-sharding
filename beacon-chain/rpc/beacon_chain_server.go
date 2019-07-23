package rpc

import (
	"context"
	"strconv"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"github.com/prysmaticlabs/prysm/beacon-chain/db"
	ethpb "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
)

// BeaconChainServer defines a server implementation of the gRPC Beacon Chain service,
// providing RPC endpoints to access data relevant to the Ethereum 2.0 phase 0
// beacon chain.
type BeaconChainServer struct {
	beaconDB *db.BeaconDB
}

// ListValidatorBalances retrieves the validator balances for a given set of public key at
// a specific epoch in time.
//
// TODO(#3045): Implement balances for a specific epoch. Current implementation returns latest balances,
// this is blocked by DB refactor.
func (bs *BeaconChainServer) ListValidatorBalances(
	ctx context.Context,
	req *ethpb.GetValidatorBalancesRequest) (*ethpb.ValidatorBalances, error) {

	res := make([]*ethpb.ValidatorBalances_Balance, 0, len(req.PublicKeys)+len(req.Indices))
	filtered := map[uint64]bool{} // track filtered validators to prevent duplication in the response.

	balances, err := bs.beaconDB.Balances(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "could not retrieve validator balances: %v", err)
	}
	validators, err := bs.beaconDB.Validators(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "could not retrieve validators: %v", err)
	}

	for _, pubKey := range req.PublicKeys {
		index, err := bs.beaconDB.ValidatorIndex(pubKey)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "could not retrieve validator index: %v", err)
		}
		filtered[index] = true

		if int(index) >= len(balances) {
			return nil, status.Errorf(codes.OutOfRange, "validator index %d >= balance list %d",
				index, len(balances))
		}

		res = append(res, &ethpb.ValidatorBalances_Balance{
			PublicKey: pubKey,
			Index:     index,
			Balance:   balances[index],
		})
	}

	for _, index := range req.Indices {
		if int(index) >= len(balances) {
			return nil, status.Errorf(codes.OutOfRange, "validator index %d >= balance list %d",
				index, len(balances))
		}

		if !filtered[index] {
			res = append(res, &ethpb.ValidatorBalances_Balance{
				PublicKey: validators[index].PublicKey,
				Index:     index,
				Balance:   balances[index],
			})
		}
	}
	return &ethpb.ValidatorBalances{Balances: res}, nil
}

// GetValidators retrieves the active validators with an optional historical epoch flag to
// to retrieve validator set in time.
//
// TODO(#3045): Implement validator set for a specific epoch. Current implementation returns latest set,
// this is blocked by DB refactor.
func (bs *BeaconChainServer) GetValidators(
	ctx context.Context,
	req *ethpb.GetValidatorsRequest) (*ethpb.Validators, error) {

	validators, err := bs.beaconDB.Validators(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "could not retrieve validators: %v", err)
	}

	pageSize := int(req.PageSize)

	start, err := strconv.Atoi(req.PageToken)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "could not get page token: %v", err)
	}
	end := start + pageSize
	nextPage := end + 1
	if end > len(validators) {
		end = len(validators)
		pageSize = end - start
		nextPage = 0
	}

	res := &ethpb.Validators{
			Validators: validators[start:end],
			TotalSize: int32(pageSize),
			NextPageToken: string(nextPage),
		}
	return res, nil
}
