package sync

import (
	"context"

	"github.com/gogo/protobuf/proto"
	"github.com/karlseguin/ccache"
	"github.com/pkg/errors"
	ethpb "github.com/prysmaticlabs/ethereumapis/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/blocks"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/state"
	"github.com/prysmaticlabs/prysm/beacon-chain/p2p"
	"github.com/prysmaticlabs/prysm/shared/hashutil"
	"go.opencensus.io/trace"
)

// seenProposerSlashings represents a cache of all the seen slashings
var seenProposerSlashings = ccache.New(ccache.Configure())

func propSlashingCacheKey(slashing *ethpb.ProposerSlashing) (string, error) {
	hash, err := hashutil.HashProto(slashing)
	if err != nil {
		return "", err
	}
	return string(hash[:]), nil
}

// Clients who receive a proposer slashing on this topic MUST validate the conditions within VerifyProposerSlashing before
// forwarding it across the network.
func (r *RegularSync) validateProposerSlashing(ctx context.Context, msg proto.Message, p p2p.Broadcaster, fromSelf bool) (bool, error) {
	// The head state will be too far away to validate any slashing.
	if r.initialSync.Syncing() {
		return false, nil
	}

	ctx, span := trace.StartSpan(ctx, "sync.validateBeaconBlockPubSub")
	defer span.End()

	slashing, ok := msg.(*ethpb.ProposerSlashing)
	if !ok {
		return false, nil
	}
	cacheKey, err := propSlashingCacheKey(slashing)
	if err != nil {
		return false, errors.Wrapf(err, "could not hash proposer slashing")
	}

	invalidKey := invalid + cacheKey
	if seenProposerSlashings.Get(invalidKey) != nil {
		return false, errors.New("previously seen invalid proposer slashing received")
	}
	if seenProposerSlashings.Get(cacheKey) != nil {
		return false, nil
	}

	// Retrieve head state, advance state to the epoch slot used specified in slashing message.
	s, err := r.chain.HeadState(ctx)
	if err != nil {
		return false, errors.Wrap(err, "Could not get head state")
	}
	slashSlot := slashing.Header_1.Slot
	if s.Slot < slashSlot {
		if ctx.Err() != nil {
			return false, errors.Wrapf(ctx.Err(),
				"Failed to advance state to slot %d to process proposer slashing", slashSlot)
		}
		var err error
		s, err = state.ProcessSlots(ctx, s, slashSlot)
		if err != nil {
			return false, errors.Wrapf(err, "Failed to advance state to slot %d", slashSlot)
		}
	}

	if err := blocks.VerifyProposerSlashing(s, slashing); err != nil {
		seenProposerSlashings.Set(invalidKey, true /*value*/, oneYear /*TTL*/)
		return false, errors.Wrap(err, "Received invalid proposer slashing")
	}
	seenProposerSlashings.Set(cacheKey, true /*value*/, oneYear /*TTL*/)

	if fromSelf {
		return false, nil
	}

	if err := p.Broadcast(ctx, slashing); err != nil {
		log.WithError(err).Error("Failed to propagate proposer slashing")
	}
	return true, nil
}
