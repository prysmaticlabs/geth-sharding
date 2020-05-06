package validator

import (
	"context"
	"math/rand"
	"time"

	ethpb "github.com/prysmaticlabs/ethereumapis/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/beacon-chain/cache"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/feed"
	statefeed "github.com/prysmaticlabs/prysm/beacon-chain/core/feed/state"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/state"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/roughtime"
	"github.com/prysmaticlabs/prysm/shared/slotutil"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// GetDuties returns the duties assigned to a list of validators specified
// in the request object.
func (vs *Server) GetDuties(ctx context.Context, req *ethpb.DutiesRequest) (*ethpb.DutiesResponse, error) {
	if vs.SyncChecker.Syncing() {
		return nil, status.Error(codes.Unavailable, "Syncing to latest head, not ready to respond")
	}
	return vs.duties(ctx, req)
}

// StreamDuties returns the duties assigned to a list of validators specified
// in the request object via a server-side stream. The stream sends out new assignments in case
// a chain re-org occurred.
func (vs *Server) StreamDuties(req *ethpb.DutiesRequest, stream ethpb.BeaconNodeValidator_StreamDutiesServer) error {
	if vs.SyncChecker.Syncing() {
		return status.Error(codes.Unavailable, "Syncing to latest head, not ready to respond")
	}

	// We compute duties the very first time the endpoint is called.
	res, err := vs.duties(stream.Context(), req)
	if err != nil {
		return status.Errorf(codes.Internal, "Could not compute validator duties: %v", err)
	}
	if err := stream.Send(res); err != nil {
		return status.Errorf(codes.Internal, "Could not send response over stream: %v", err)
	}

	// We start a for loop which ticks on every epoch or a chain reorg.
	stateChannel := make(chan *feed.Event, 1)
	stateSub := vs.StateNotifier.StateFeed().Subscribe(stateChannel)
	defer stateSub.Unsubscribe()

	secondsPerEpoch := params.BeaconConfig().SecondsPerSlot * params.BeaconConfig().SlotsPerEpoch
	epochTicker := slotutil.GetSlotTicker(vs.GenesisTimeFetcher.GenesisTime(), secondsPerEpoch)
	for {
		select {
		// Ticks every epoch to submit assignments to connected validator clients.
		case <-epochTicker.C():
			res, err := vs.duties(stream.Context(), req)
			if err != nil {
				return status.Errorf(codes.Internal, "Could not compute validator duties: %v", err)
			}
			if err := stream.Send(res); err != nil {
				return status.Errorf(codes.Internal, "Could not send response over stream: %v", err)
			}
		case ev := <-stateChannel:
			// If a reorg occurred, we recompute duties for the connected validator clients
			// and send another response over the server stream right away.
			if ev.Type == statefeed.Reorg {
				data, ok := ev.Data.(*statefeed.ReorgData)
				if !ok {
					return status.Errorf(codes.Internal, "Received incorrect data type over reorg feed: %v", data)
				}
				newSlotEpoch := helpers.SlotToEpoch(data.NewSlot)
				oldSlotEpoch := helpers.SlotToEpoch(data.OldSlot)
				// We only send out new duties if a reorg across epochs occurred, otherwise
				// validator shufflings would not have changed as a result of a reorg.
				if newSlotEpoch >= oldSlotEpoch {
					continue
				}
				res, err := vs.duties(stream.Context(), req)
				if err != nil {
					return status.Errorf(codes.Internal, "Could not compute validator duties: %v", err)
				}
				if err := stream.Send(res); err != nil {
					return status.Errorf(codes.Internal, "Could not send response over stream: %v", err)
				}
			}
		case <-stream.Context().Done():
			return status.Error(codes.Canceled, "Stream context canceled")
		case <-vs.Ctx.Done():
			return status.Error(codes.Canceled, "RPC context canceled")
		}
	}
}

// Compute the validator duties from the head state's corresponding epoch
// for validators public key / indices requested.
func (vs *Server) duties(ctx context.Context, req *ethpb.DutiesRequest) (*ethpb.DutiesResponse, error) {
	s, err := vs.HeadFetcher.HeadState(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Could not get head state: %v", err)
	}

	// Advance state with empty transitions up to the requested epoch start slot.
	if epochStartSlot := helpers.StartSlot(req.Epoch); s.Slot() < epochStartSlot {
		s, err = state.ProcessSlots(ctx, s, epochStartSlot)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "Could not process slots up to %d: %v", epochStartSlot, err)
		}
	}
	committeeAssignments, proposerIndexToSlots, err := helpers.CommitteeAssignments(s, req.Epoch)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Could not compute committee assignments: %v", err)
	}

	var committeeIDs []uint64
	validatorAssignments := make([]*ethpb.DutiesResponse_Duty, 0, len(req.PublicKeys))
	for _, pubKey := range req.PublicKeys {
		if ctx.Err() != nil {
			return nil, status.Errorf(codes.Aborted, "Could not continue fetching assignments: %v", ctx.Err())
		}
		// Default assignment.
		assignment := &ethpb.DutiesResponse_Duty{
			PublicKey: pubKey,
		}

		idx, ok := s.ValidatorIndexByPubkey(bytesutil.ToBytes48(pubKey))
		if ok {
			assignment.ValidatorIndex = idx
			assignment.Status = vs.assignmentStatus(idx, s)
			assignment.ProposerSlots = proposerIndexToSlots[idx]

			ca, ok := committeeAssignments[idx]
			if ok {
				assignment.Committee = ca.Committee
				assignment.AttesterSlot = ca.AttesterSlot
				assignment.CommitteeIndex = ca.CommitteeIndex
				committeeIDs = append(committeeIDs, ca.CommitteeIndex)
			}
		} else {
			// If the validator isn't in the beacon state, assume their status is unknown.
			assignment.Status = ethpb.ValidatorStatus_UNKNOWN_STATUS
		}
		validatorAssignments = append(validatorAssignments, assignment)
		// Assign relevant validator to subnet.
		assignValidatorToSubnet(pubKey, assignment.Status)
	}

	return &ethpb.DutiesResponse{
		Duties: validatorAssignments,
	}, nil
}

// assignValidatorToSubnet checks the status and pubkey of a particular validator
// to discern whether persistent subnets need to be registered for them.
func assignValidatorToSubnet(pubkey []byte, status ethpb.ValidatorStatus) {
	if status != ethpb.ValidatorStatus_ACTIVE && status != ethpb.ValidatorStatus_EXITING {
		return
	}

	_, ok, expTime := cache.CommitteeIDs.GetPersistentCommittees(pubkey)
	if ok && expTime.After(roughtime.Now()) {
		return
	}
	epochDuration := time.Duration(params.BeaconConfig().SlotsPerEpoch * params.BeaconConfig().SecondsPerSlot)
	assignedIdxs := []uint64{}
	for i := uint64(0); i < params.BeaconNetworkConfig().RandomSubnetsPerValidator; i++ {
		assignedIndex := rand.Intn(int(params.BeaconNetworkConfig().AttestationSubnetCount))
		assignedIdxs = append(assignedIdxs, uint64(assignedIndex))
	}
	assignedDuration := rand.Intn(int(params.BeaconNetworkConfig().EpochsPerRandomSubnetSubscription))
	assignedDuration += int(params.BeaconNetworkConfig().EpochsPerRandomSubnetSubscription)

	totalDuration := epochDuration * time.Duration(assignedDuration)
	cache.CommitteeIDs.AddPersistentCommittee(pubkey, assignedIdxs, totalDuration*time.Second)
}
