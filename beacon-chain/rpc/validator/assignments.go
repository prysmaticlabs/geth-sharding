package validator

import (
	"context"

	ethpb "github.com/prysmaticlabs/ethereumapis/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/beacon-chain/cache"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/state"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/prysmaticlabs/prysm/shared/featureconfig"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// GetDuties returns the committee assignment response from a given validator public key.
// The committee assignment response contains the following fields for the current and previous epoch:
//	1.) The list of validators in the committee.
//	2.) The shard to which the committee is assigned.
//	3.) The slot at which the committee is assigned.
//	4.) The bool signaling if the validator is expected to propose a block at the assigned slot.
func (vs *Server) GetDuties(ctx context.Context, req *ethpb.DutiesRequest) (*ethpb.DutiesResponse, error) {
	if vs.SyncChecker.Syncing() {
		return nil, status.Error(codes.Unavailable, "Syncing to latest head, not ready to respond")
	}

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
	committeeAssignments, proposerIndexToSlot, err := helpers.CommitteeAssignments(s, req.Epoch)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Could not compute committee assignments: %v", err)
	}
	// Query the next epoch assignments for committee subnet subscriptions.
	nextCommitteeAssignments, _, err := helpers.CommitteeAssignments(s, req.Epoch+1)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Could not compute committee assignments: %v", err)
	}

	var committeeIDs []uint64
	var nextCommitteeIDs []uint64
	var validatorAssignments []*ethpb.DutiesResponse_Duty
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
			ca, ok := committeeAssignments[idx]
			if ok {
				assignment.Committee = ca.Committee
				assignment.Status = vs.assignmentStatus(idx, s)
				assignment.ValidatorIndex = idx
				assignment.PublicKey = pubKey
				assignment.AttesterSlot = ca.AttesterSlot
				assignment.ProposerSlot = proposerIndexToSlot[idx]
				assignment.CommitteeIndex = ca.CommitteeIndex
				committeeIDs = append(committeeIDs, ca.CommitteeIndex)
			}
			// Save the next epoch assignments.
			ca, ok = nextCommitteeAssignments[idx]
			if ok {
				nextCommitteeIDs = append(nextCommitteeIDs, ca.CommitteeIndex)
			}

		} else {
			vs := vs.validatorStatus(ctx, pubKey, s)
			assignment.Status = vs.Status
		}
		validatorAssignments = append(validatorAssignments, assignment)

	}

	if featureconfig.Get().EnableDynamicCommitteeSubnets {
		cache.CommitteeIDs.AddIDs(committeeIDs, req.Epoch)
		cache.CommitteeIDs.AddIDs(nextCommitteeIDs, req.Epoch+1)
	}

	return &ethpb.DutiesResponse{
		Duties: validatorAssignments,
	}, nil
}
