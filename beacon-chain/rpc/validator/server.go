package validator

import (
	"context"
	"time"

	ptypes "github.com/gogo/protobuf/types"
	ethpb "github.com/prysmaticlabs/ethereumapis/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/beacon-chain/blockchain"
	"github.com/prysmaticlabs/prysm/beacon-chain/cache"
	"github.com/prysmaticlabs/prysm/beacon-chain/cache/depositcache"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/feed"
	opfeed "github.com/prysmaticlabs/prysm/beacon-chain/core/feed/operation"
	statefeed "github.com/prysmaticlabs/prysm/beacon-chain/core/feed/state"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	"github.com/prysmaticlabs/prysm/beacon-chain/db"
	"github.com/prysmaticlabs/prysm/beacon-chain/operations"
	"github.com/prysmaticlabs/prysm/beacon-chain/operations/attestations"
	"github.com/prysmaticlabs/prysm/beacon-chain/p2p"
	"github.com/prysmaticlabs/prysm/beacon-chain/powchain"
	"github.com/prysmaticlabs/prysm/beacon-chain/sync"
	pbp2p "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/rpc/v1"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var log logrus.FieldLogger

func init() {
	log = logrus.WithField("prefix", "rpc/validator")
}

// Server defines a server implementation of the gRPC Validator service,
// providing RPC endpoints for obtaining validator assignments per epoch, the slots
// and committees in which particular validators need to perform their responsibilities,
// and more.
type Server struct {
	Ctx                    context.Context
	BeaconDB               db.Database
	AttestationCache       *cache.AttestationCache
	HeadFetcher            blockchain.HeadFetcher
	ForkFetcher            blockchain.ForkFetcher
	CanonicalStateChan     chan *pbp2p.BeaconState
	BlockFetcher           powchain.POWBlockFetcher
	DepositFetcher         depositcache.DepositFetcher
	ChainStartFetcher      powchain.ChainStartFetcher
	Eth1InfoFetcher        powchain.ChainInfoFetcher
	SyncChecker            sync.Checker
	StateNotifier          statefeed.Notifier
	OperationsHandler      operations.Handler
	P2P                    p2p.Broadcaster
	Pool                   operations.Pool
	AttPool                attestations.Pool
	BlockReceiver          blockchain.BlockReceiver
	MockEth1Votes          bool
	Eth1BlockFetcher       powchain.POWBlockFetcher
	PendingDepositsFetcher depositcache.PendingDepositsFetcher
	OperationNotifier      opfeed.Notifier
	GenesisTimeFetcher     blockchain.GenesisTimeFetcher
}

// WaitForActivation checks if a validator public key exists in the active validator registry of the current
// beacon state, if not, then it creates a stream which listens for canonical states which contain
// the validator with the public key as an active validator record.
func (vs *Server) WaitForActivation(req *ethpb.ValidatorActivationRequest, stream ethpb.BeaconNodeValidator_WaitForActivationServer) error {
	activeValidatorExists, validatorStatuses, err := vs.multipleValidatorStatus(stream.Context(), req.PublicKeys)
	if err != nil {
		return status.Errorf(codes.Internal, "Could not fetch validator status: %v", err)
	}
	res := &ethpb.ValidatorActivationResponse{
		Statuses: validatorStatuses,
	}
	if activeValidatorExists {
		return stream.Send(res)
	}
	if err := stream.Send(res); err != nil {
		return status.Errorf(codes.Internal, "Could not send response over stream: %v", err)
	}

	for {
		select {
		case <-time.After(6 * time.Second):
			activeValidatorExists, validatorStatuses, err := vs.multipleValidatorStatus(stream.Context(), req.PublicKeys)
			if err != nil {
				return status.Errorf(codes.Internal, "Could not fetch validator status: %v", err)
			}
			res := &ethpb.ValidatorActivationResponse{
				Statuses: validatorStatuses,
			}
			if activeValidatorExists {
				return stream.Send(res)
			}
			if err := stream.Send(res); err != nil {
				return status.Errorf(codes.Internal, "Could not send response over stream: %v", err)
			}
		case <-stream.Context().Done():
			return status.Error(codes.Canceled, "Stream context canceled")
		case <-vs.Ctx.Done():
			return status.Error(codes.Canceled, "RPC context canceled")
		}
	}
}

// ValidatorIndex is called by a validator to get its index location in the beacon state.
func (vs *Server) ValidatorIndex(ctx context.Context, req *ethpb.ValidatorIndexRequest) (*ethpb.ValidatorIndexResponse, error) {
	index, ok, err := vs.BeaconDB.ValidatorIndex(ctx, bytesutil.ToBytes48(req.PublicKey))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Could not fetch validator index: %v", err)
	}
	if !ok {
		return nil, status.Errorf(codes.Internal, "Could not find validator index for public key %#x not found", req.PublicKey)
	}

	return &ethpb.ValidatorIndexResponse{Index: index}, nil
}

// ExitedValidators queries validator statuses for a give list of validators
// and returns a filtered list of validator keys that are exited.
func (vs *Server) ExitedValidators(
	ctx context.Context,
	req *pb.ExitedValidatorsRequest) (*pb.ExitedValidatorsResponse, error) {

	_, statuses, err := vs.multipleValidatorStatus(ctx, req.PublicKeys)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Could not retrieve validator statuses: %v", err)
	}

	exitedKeys := make([][]byte, 0)
	for _, st := range statuses {
		s := st.Status.Status
		if s == ethpb.ValidatorStatus_EXITED ||
			s == ethpb.ValidatorStatus_EXITED_SLASHED ||
			s == ethpb.ValidatorStatus_INITIATED_EXIT {
			exitedKeys = append(exitedKeys, st.PublicKey)
		}
	}

	resp := &pb.ExitedValidatorsResponse{
		PublicKeys: exitedKeys,
	}

	return resp, nil
}

// DomainData fetches the current domain version information from the beacon state.
func (vs *Server) DomainData(ctx context.Context, request *ethpb.DomainRequest) (*ethpb.DomainResponse, error) {
	fork := vs.ForkFetcher.CurrentFork()
	dv := helpers.Domain(fork, request.Epoch, request.Domain)
	return &ethpb.DomainResponse{
		SignatureDomain: dv,
	}, nil
}

// CanonicalHead of the current beacon chain. This method is requested on-demand
// by a validator when it is their time to propose or attest.
func (vs *Server) CanonicalHead(ctx context.Context, req *ptypes.Empty) (*ethpb.BeaconBlock, error) {
	return vs.HeadFetcher.HeadBlock(), nil
}

// WaitForChainStart queries the logs of the Deposit Contract in order to verify the beacon chain
// has started its runtime and validators begin their responsibilities. If it has not, it then
// subscribes to an event stream triggered by the powchain service whenever the ChainStart log does
// occur in the Deposit Contract on ETH 1.0.
func (vs *Server) WaitForChainStart(req *ptypes.Empty, stream ethpb.BeaconNodeValidator_WaitForChainStartServer) error {
	head, err := vs.BeaconDB.HeadState(context.Background())
	if err != nil {
		return status.Errorf(codes.Internal, "Could not retrieve head state: %v", err)
	}
	if head != nil {
		res := &ethpb.ChainStartResponse{
			Started:     true,
			GenesisTime: head.GenesisTime,
		}
		return stream.Send(res)
	}

	stateChannel := make(chan *feed.Event, 1)
	stateSub := vs.StateNotifier.StateFeed().Subscribe(stateChannel)
	defer stateSub.Unsubscribe()
	for {
		select {
		case event := <-stateChannel:
			if event.Type == statefeed.ChainStarted {
				data := event.Data.(*statefeed.ChainStartedData)
				log.WithField("starttime", data.StartTime).Debug("Received chain started event")
				log.Info("Sending genesis time notification to connected validator clients")
				res := &ethpb.ChainStartResponse{
					Started:     true,
					GenesisTime: uint64(data.StartTime.Unix()),
				}
				return stream.Send(res)
			}
		case <-stateSub.Err():
			return status.Error(codes.Aborted, "Subscriber closed, exiting goroutine")
		case <-vs.Ctx.Done():
			return status.Error(codes.Canceled, "Context canceled")
		}
	}
}
