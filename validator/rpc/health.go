package rpc

import (
	"context"
	"time"

	ptypes "github.com/gogo/protobuf/types"
	"github.com/pkg/errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/prysmaticlabs/prysm/proto/validator/accounts/v2"
)

// GetBeaconNodeConnection retrieves the current beacon node connection
// information, as well as its sync status.
func (s *Server) GetBeaconNodeConnection(ctx context.Context, _ *ptypes.Empty) (*pb.NodeConnectionResponse, error) {
	syncStatus, err := s.syncChecker.Syncing(ctx)
	if err != nil || s.validatorService.Status() != nil {
		return &pb.NodeConnectionResponse{
			GenesisTime:        0,
			BeaconNodeEndpoint: s.nodeGatewayEndpoint,
			Connected:          false,
			Syncing:            false,
		}, nil
	}
	genesis, err := s.genesisFetcher.GenesisInfo(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.NodeConnectionResponse{
		GenesisTime:            uint64(time.Unix(genesis.GenesisTime.Seconds, 0).Unix()),
		DepositContractAddress: genesis.DepositContractAddress,
		BeaconNodeEndpoint:     s.nodeGatewayEndpoint,
		Connected:              true,
		Syncing:                syncStatus,
	}, nil
}

// GetLogsEndpoints for the beacon and validator client.
func (s *Server) GetLogsEndpoints(ctx context.Context, _ *ptypes.Empty) (*pb.LogsEndpointResponse, error) {
	return nil, status.Error(codes.Unimplemented, "unimplemented")
}

// StreamBeaconLogs from the beacon node via a gRPC server-side stream.
func (s *Server) StreamBeaconLogs(req *ptypes.Empty, stream pb.Health_StreamBeaconLogsServer) error {
	client, err := s.beaconNodeHealthClient.StreamBeaconLogs(s.ctx, req)
	if err != nil {
		return err
	}
	for {
		select {
		case <-s.ctx.Done():
			return status.Error(codes.Canceled, "Context canceled")
		case <-stream.Context().Done():
			return status.Error(codes.Canceled, "Context canceled")
		default:
			resp, err := client.Recv()
			if err != nil {
				return errors.Wrap(err, "could not receive beacon logs from stream")
			}
			if err := stream.Send(resp); err != nil {
				return status.Errorf(codes.Unavailable, "Could not send over stream: %v", err)
			}
		}
	}
}

// StreamValidatorLogs from the validator client via a gRPC server-side stream.
func (s *Server) StreamValidatorLogs(_ *ptypes.Empty, stream pb.Health_StreamValidatorLogsServer) error {
	ch := make(chan []byte, s.streamLogsBufferSize)
	defer close(ch)
	sub := s.logsStreamer.LogsFeed().Subscribe(ch)
	defer sub.Unsubscribe()

	recentLogs := s.logsStreamer.GetLastFewLogs()
	logStrings := make([]string, len(recentLogs))
	for i, log := range recentLogs {
		logStrings[i] = string(log)
	}
	if err := stream.Send(&pb.LogsResponse{
		Logs: logStrings,
	}); err != nil {
		return status.Errorf(codes.Unavailable, "Could not send over stream: %v", err)
	}
	for {
		select {
		case log := <-ch:
			resp := &pb.LogsResponse{
				Logs: []string{string(log)},
			}
			if err := stream.Send(resp); err != nil {
				return status.Errorf(codes.Unavailable, "Could not send over stream: %v", err)
			}
		case <-s.ctx.Done():
			return status.Error(codes.Canceled, "Context canceled")
		case <-stream.Context().Done():
			return status.Error(codes.Canceled, "Context canceled")
		}
	}
}
