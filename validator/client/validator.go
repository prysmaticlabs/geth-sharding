// Package client represents the functionality to act as a validator.
package client

import (
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"time"

	ptypes "github.com/gogo/protobuf/types"
	"github.com/pkg/errors"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/rpc/v1"
	"github.com/prysmaticlabs/prysm/shared/keystore"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/slotutil"
	"github.com/sirupsen/logrus"
	"go.opencensus.io/trace"
)

type validator struct {
	genesisTime          uint64
	ticker               *slotutil.SlotTicker
	assignments          *pb.AssignmentResponse
	proposerClient       pb.ProposerServiceClient
	validatorClient      pb.ValidatorServiceClient
	attesterClient       pb.AttesterServiceClient
	keys                 map[string]*keystore.Key
	pubkeys              [][]byte
	prevBalance          map[[48]byte]uint64
	logValidatorBalances bool
}

// Done cleans up the validator.
func (v *validator) Done() {
	v.ticker.Done()
}

// WaitForChainStart checks whether the beacon node has started its runtime. That is,
// it calls to the beacon node which then verifies the ETH1.0 deposit contract logs to check
// for the ChainStart log to have been emitted. If so, it starts a ticker based on the ChainStart
// unix timestamp which will be used to keep track of time within the validator client.
func (v *validator) WaitForChainStart(ctx context.Context) error {
	ctx, span := trace.StartSpan(ctx, "validator.WaitForChainStart")
	defer span.End()
	// First, check if the beacon chain has started.
	stream, err := v.validatorClient.WaitForChainStart(ctx, &ptypes.Empty{})
	if err != nil {
		return errors.Wrap(err, "could not setup beacon chain ChainStart streaming client")
	}
	for {
		log.Info("Waiting for beacon chain start log from the ETH 1.0 deposit contract")
		chainStartRes, err := stream.Recv()
		// If the stream is closed, we stop the loop.
		if err == io.EOF {
			break
		}
		// If context is canceled we stop the loop.
		if ctx.Err() == context.Canceled {
			return errors.Wrap(ctx.Err(), "context has been canceled so shutting down the loop")
		}
		if err != nil {
			return errors.Wrap(err, "could not receive ChainStart from stream")
		}
		v.genesisTime = chainStartRes.GenesisTime
		break
	}
	// Once the ChainStart log is received, we update the genesis time of the validator client
	// and begin a slot ticker used to track the current slot the beacon node is in.
	v.ticker = slotutil.GetSlotTicker(time.Unix(int64(v.genesisTime), 0), params.BeaconConfig().SecondsPerSlot)
	log.WithField("genesisTime", time.Unix(int64(v.genesisTime), 0)).Info("Beacon chain started")
	return nil
}

// WaitForActivation checks whether the validator pubkey is in the active
// validator set. If not, this operation will block until an activation message is
// received.
func (v *validator) WaitForActivation(ctx context.Context) error {
	ctx, span := trace.StartSpan(ctx, "validator.WaitForActivation")
	defer span.End()
	req := &pb.ValidatorActivationRequest{
		PublicKeys: v.pubkeys,
	}
	stream, err := v.validatorClient.WaitForActivation(ctx, req)
	if err != nil {
		return errors.Wrap(err, "could not setup validator WaitForActivation streaming client")
	}
	var validatorActivatedRecords [][]byte
	for {
		res, err := stream.Recv()
		// If the stream is closed, we stop the loop.
		if err == io.EOF {
			break
		}
		// If context is canceled we stop the loop.
		if ctx.Err() == context.Canceled {
			return errors.Wrap(ctx.Err(), "context has been canceled so shutting down the loop")
		}
		if err != nil {
			return errors.Wrap(err, "could not receive validator activation from stream")
		}
		log.Info("Waiting for validator to be activated in the beacon chain")
		activatedKeys := v.checkAndLogValidatorStatus(res.Statuses)

		if len(activatedKeys) > 0 {
			validatorActivatedRecords = activatedKeys
			break
		}
	}
	for _, pk := range validatorActivatedRecords {
		pubKey := fmt.Sprintf("%#x", pk[:8])
		log.WithField("pubKey", pubKey).Info("Validator activated")
	}
	v.ticker = slotutil.GetSlotTicker(time.Unix(int64(v.genesisTime), 0), params.BeaconConfig().SecondsPerSlot)

	return nil
}

func (v *validator) checkAndLogValidatorStatus(validatorStatuses []*pb.ValidatorActivationResponse_Status) [][]byte {
	var activatedKeys [][]byte
	for _, status := range validatorStatuses {
		pubKey := fmt.Sprintf("%#x", status.PublicKey[:8])
		log := log.WithFields(logrus.Fields{
			"pubKey": pubKey,
			"status": status.Status.Status.String(),
		})
		if status.Status.Status == pb.ValidatorStatus_ACTIVE {
			activatedKeys = append(activatedKeys, status.PublicKey)
			continue
		}
		if status.Status.Status == pb.ValidatorStatus_EXITED {
			log.Info("Validator exited")
			continue
		}
		if status.Status.DepositInclusionSlot == 0 {
			log.Info("Validator not included in state")
			continue
		}
		if status.Status.ActivationEpoch == params.BeaconConfig().FarFutureEpoch {
			log.WithFields(logrus.Fields{
				"depositInclusionSlot":      status.Status.DepositInclusionSlot,
				"positionInActivationQueue": status.Status.PositionInActivationQueue,
			}).Info("Waiting to be activated")
			continue
		}
		log.WithFields(logrus.Fields{
			"depositInclusionSlot":      status.Status.DepositInclusionSlot,
			"activationEpoch":           status.Status.ActivationEpoch,
			"positionInActivationQueue": status.Status.PositionInActivationQueue,
		}).Info("Validator status")
	}
	return activatedKeys
}

// CanonicalHeadSlot returns the slot of canonical block currently found in the
// beacon chain via RPC.
func (v *validator) CanonicalHeadSlot(ctx context.Context) (uint64, error) {
	ctx, span := trace.StartSpan(ctx, "validator.CanonicalHeadSlot")
	defer span.End()
	head, err := v.validatorClient.CanonicalHead(ctx, &ptypes.Empty{})
	if err != nil {
		return 0, err
	}
	return head.Slot, nil
}

// NextSlot emits the next slot number at the start time of that slot.
func (v *validator) NextSlot() <-chan uint64 {
	return v.ticker.C()
}

// SlotDeadline is the start time of the next slot.
func (v *validator) SlotDeadline(slot uint64) time.Time {
	secs := (slot + 1) * params.BeaconConfig().SecondsPerSlot
	return time.Unix(int64(v.genesisTime), 0 /*ns*/).Add(time.Duration(secs) * time.Second)
}

// UpdateAssignments checks the slot number to determine if the validator's
// list of upcoming assignments needs to be updated. For example, at the
// beginning of a new epoch.
func (v *validator) UpdateAssignments(ctx context.Context, slot uint64) error {
	if slot%params.BeaconConfig().SlotsPerEpoch != 0 && v.assignments != nil {
		// Do nothing if not epoch start AND assignments already exist.
		return nil
	}
	ctx, span := trace.StartSpan(ctx, "validator.UpdateAssignments")
	defer span.End()

	req := &pb.AssignmentRequest{
		EpochStart: slot / params.BeaconConfig().SlotsPerEpoch,
		PublicKeys: v.pubkeys,
	}

	resp, err := v.validatorClient.CommitteeAssignment(ctx, req)
	if err != nil {
		v.assignments = nil // Clear assignments so we know to retry the request.
		log.Error(err)
		return err
	}

	v.assignments = resp
	// Only log the full assignments output on epoch start to be less verbose.
	if slot%params.BeaconConfig().SlotsPerEpoch == 0 {
		for _, assignment := range v.assignments.ValidatorAssignment {
			var proposerSlot uint64
			var attesterSlot uint64
			pubKey := fmt.Sprintf("%#x", assignment.PublicKey[:8])

			lFields := logrus.Fields{
				"pubKey": pubKey,
				"epoch":  slot / params.BeaconConfig().SlotsPerEpoch,
			}
			if assignment.Status != pb.ValidatorStatus_ACTIVE {
				lFields["status"] = assignment.Status
				log.WithFields(lFields).Info("New assignment")
				continue
			} else if assignment.IsProposer {
				proposerSlot = assignment.Slot
				attesterSlot = assignment.Slot
			} else {
				attesterSlot = assignment.Slot
			}
			lFields["attesterSlot"] = attesterSlot
			lFields["proposerSlot"] = "N/A"

			if assignment.IsProposer {
				lFields["proposerSlot"] = proposerSlot
			}
			log.WithFields(lFields).Info("New assignment")
		}
	}

	return nil
}

// RolesAt slot returns the validator roles at the given slot. Returns nil if the
// validator is known to not have a roles at the at slot. Returns UNKNOWN if the
// validator assignments are unknown. Otherwise returns a valid ValidatorRole map.
func (v *validator) RolesAt(slot uint64) map[string]pb.ValidatorRole {
	rolesAt := make(map[string]pb.ValidatorRole)
	for _, assignment := range v.assignments.ValidatorAssignment {
		var role pb.ValidatorRole
		if assignment == nil {
			role = pb.ValidatorRole_UNKNOWN
		}
		if assignment.Slot == slot {
			// Note: A proposer also attests to the slot.
			if assignment.IsProposer {
				role = pb.ValidatorRole_PROPOSER
			} else {
				role = pb.ValidatorRole_ATTESTER
			}
		} else {
			role = pb.ValidatorRole_UNKNOWN
		}
		rolesAt[hex.EncodeToString(assignment.PublicKey)] = role
	}
	return rolesAt
}
