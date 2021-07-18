package beacon

import (
	"bytes"
	"context"

	"github.com/prysmaticlabs/prysm/beacon-chain/rpc/statefetcher"
	iface "github.com/prysmaticlabs/prysm/beacon-chain/state/interface"
	ethpb "github.com/prysmaticlabs/prysm/proto/eth/v1"
	eth "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/shared/params"
	"go.opencensus.io/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// GetGenesis retrieves details of the chain's genesis which can be used to identify chain.
func (bs *Server) GetGenesis(ctx context.Context, _ *emptypb.Empty) (*ethpb.GenesisResponse, error) {
	ctx, span := trace.StartSpan(ctx, "beacon.GetGenesis")
	defer span.End()

	genesisTime := bs.GenesisTimeFetcher.GenesisTime()
	if genesisTime.IsZero() {
		return nil, status.Errorf(codes.NotFound, "Chain genesis info is not yet known")
	}
	validatorRoot := bs.ChainInfoFetcher.GenesisValidatorRoot()
	if bytes.Equal(validatorRoot[:], params.BeaconConfig().ZeroHash[:]) {
		return nil, status.Errorf(codes.NotFound, "Chain genesis info is not yet known")
	}
	forkVersion := params.BeaconConfig().GenesisForkVersion

	return &ethpb.GenesisResponse{
		Data: &ethpb.GenesisResponse_Genesis{
			GenesisTime: &timestamppb.Timestamp{
				Seconds: genesisTime.Unix(),
				Nanos:   0,
			},
			GenesisValidatorsRoot: validatorRoot[:],
			GenesisForkVersion:    forkVersion,
		},
	}, nil
}

// GetStateRoot calculates HashTreeRoot for state with given 'stateId'. If stateId is root, same value will be returned.
func (bs *Server) GetStateRoot(ctx context.Context, req *ethpb.StateRequest) (*ethpb.StateRootResponse, error) {
	ctx, span := trace.StartSpan(ctx, "beacon.GetStateRoot")
	defer span.End()

	var (
		root []byte
		err  error
	)

	root, err = bs.StateFetcher.StateRoot(ctx, req.StateId)
	if err != nil {
		if rootNotFoundErr, ok := err.(*statefetcher.StateRootNotFoundError); ok {
			return nil, status.Errorf(codes.NotFound, "State root not found: %v", rootNotFoundErr)
		} else if parseErr, ok := err.(*statefetcher.StateIdParseError); ok {
			return nil, status.Errorf(codes.InvalidArgument, "Invalid state ID: %v", parseErr)
		}
		return nil, status.Errorf(codes.Internal, "Could not get state root: %v", err)
	}

	return &ethpb.StateRootResponse{
		Data: &ethpb.StateRootResponse_StateRoot{
			Root: root,
		},
	}, nil
}

// GetStateFork returns Fork object for state with given 'stateId'.
func (bs *Server) GetStateFork(ctx context.Context, req *ethpb.StateRequest) (*ethpb.StateForkResponse, error) {
	ctx, span := trace.StartSpan(ctx, "beacon.GetStateFork")
	defer span.End()

	var (
		state iface.BeaconState
		err   error
	)

	state, err = bs.StateFetcher.State(ctx, req.StateId)
	if err != nil {
		if stateNotFoundErr, ok := err.(*statefetcher.StateNotFoundError); ok {
			return nil, status.Errorf(codes.NotFound, "State not found: %v", stateNotFoundErr)
		} else if parseErr, ok := err.(*statefetcher.StateIdParseError); ok {
			return nil, status.Errorf(codes.InvalidArgument, "Invalid state ID: %v", parseErr)
		}
		return nil, status.Errorf(codes.Internal, "Could not get state: %v", err)
	}

	fork := state.Fork()
	return &ethpb.StateForkResponse{
		Data: &ethpb.Fork{
			PreviousVersion: fork.PreviousVersion,
			CurrentVersion:  fork.CurrentVersion,
			Epoch:           fork.Epoch,
		},
	}, nil
}

// GetFinalityCheckpoints returns finality checkpoints for state with given 'stateId'. In case finality is
// not yet achieved, checkpoint should return epoch 0 and ZERO_HASH as root.
func (bs *Server) GetFinalityCheckpoints(ctx context.Context, req *ethpb.StateRequest) (*ethpb.StateFinalityCheckpointResponse, error) {
	ctx, span := trace.StartSpan(ctx, "beacon.GetFinalityCheckpoints")
	defer span.End()

	var (
		state iface.BeaconState
		err   error
	)

	state, err = bs.StateFetcher.State(ctx, req.StateId)
	if err != nil {
		if stateNotFoundErr, ok := err.(*statefetcher.StateNotFoundError); ok {
			return nil, status.Errorf(codes.NotFound, "State not found: %v", stateNotFoundErr)
		} else if parseErr, ok := err.(*statefetcher.StateIdParseError); ok {
			return nil, status.Errorf(codes.InvalidArgument, "Invalid state ID: %v", parseErr)
		}
		return nil, status.Errorf(codes.Internal, "Could not get state: %v", err)
	}

	return &ethpb.StateFinalityCheckpointResponse{
		Data: &ethpb.StateFinalityCheckpointResponse_StateFinalityCheckpoint{
			PreviousJustified: checkpoint(state.PreviousJustifiedCheckpoint()),
			CurrentJustified:  checkpoint(state.CurrentJustifiedCheckpoint()),
			Finalized:         checkpoint(state.FinalizedCheckpoint()),
		},
	}, nil
}

func checkpoint(sourceCheckpoint *eth.Checkpoint) *ethpb.Checkpoint {
	if sourceCheckpoint != nil {
		return &ethpb.Checkpoint{
			Epoch: sourceCheckpoint.Epoch,
			Root:  sourceCheckpoint.Root,
		}
	}
	return &ethpb.Checkpoint{
		Epoch: 0,
		Root:  params.BeaconConfig().ZeroHash[:],
	}
}
