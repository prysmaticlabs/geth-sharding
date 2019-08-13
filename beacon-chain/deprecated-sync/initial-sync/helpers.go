package initialsync

import (
	"context"
	"fmt"
	"runtime/debug"

	"github.com/gogo/protobuf/proto"
	"github.com/pkg/errors"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	ethpb "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/shared/deprecated-p2p"
	"github.com/sirupsen/logrus"
	"go.opencensus.io/trace"
)

const noMsgData = "message contains no data"

func (s *InitialSync) checkBlockValidity(ctx context.Context, block *ethpb.BeaconBlock) error {
	ctx, span := trace.StartSpan(ctx, "beacon-chain.sync.initial-sync.checkBlockValidity")
	defer span.End()
	beaconState, err := s.db.HeadState(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get beacon state")
	}

	if block.Slot < helpers.StartSlot(beaconState.FinalizedCheckpoint.Epoch) {
		return errors.New("discarding received block with a slot number smaller than the last finalized slot")
	}
	// Attestation from proposer not verified as, other nodes only store blocks not proposer
	// attestations.
	return nil
}

// safelyHandleMessage will recover and log any panic that occurs from the
// function argument.
func safelyHandleMessage(fn func(p2p.Message) error, msg p2p.Message) {
	defer func() {
		if r := recover(); r != nil {
			printedMsg := noMsgData
			if msg.Data != nil {
				printedMsg = proto.MarshalTextString(msg.Data)
			}
			log.WithFields(logrus.Fields{
				"r":   r,
				"msg": printedMsg,
			}).Error("Panicked when handling p2p message! Recovering...")

			debug.PrintStack()

			if msg.Ctx == nil {
				return
			}
			if span := trace.FromContext(msg.Ctx); span != nil {
				span.SetStatus(trace.Status{
					Code:    trace.StatusCodeInternal,
					Message: fmt.Sprintf("Panic: %v", r),
				})
			}
		}
	}()

	// Fingers crossed that it doesn't panic...
	if err := fn(msg); err != nil {
		log.WithError(err).Error("Failed to process message")
	}
}
