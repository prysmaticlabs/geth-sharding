package evaluators

import (
	"context"

	"github.com/pkg/errors"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	"github.com/prysmaticlabs/prysm/endtoend/policies"
	"github.com/prysmaticlabs/prysm/endtoend/types"
	ethpb "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	prysmv2 "github.com/prysmaticlabs/prysm/proto/prysm/v2"
	wrapperv2 "github.com/prysmaticlabs/prysm/proto/prysm/v2/wrapper"
	"github.com/prysmaticlabs/prysm/shared/params"
	"google.golang.org/grpc"
)

// ForkTransition ensures that the hard fork has occurred successfully.
var ForkTransition = types.Evaluator{
	Name:       "fork_transition_%d",
	Policy:     policies.OnEpoch(params.AltairE2EForkEpoch),
	Evaluation: forkOccurs,
}

func forkOccurs(conns ...*grpc.ClientConn) error {
	conn := conns[0]
	client := prysmv2.NewBeaconNodeValidatorAltairClient(conn)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stream, err := client.StreamBlocks(ctx, &ethpb.StreamBlocksRequest{VerifiedOnly: true})
	if err != nil {
		return errors.Wrap(err, "failed to get stream")
	}
	fSlot, err := helpers.StartSlot(params.AltairE2EForkEpoch)
	if err != nil {
		return err
	}
	if ctx.Err() == context.Canceled {
		return errors.New("context canceled prematurely")
	}
	res, err := stream.Recv()
	if err != nil {
		return err
	}
	if res == nil || res.Block == nil {
		return errors.New("nil block returned by beacon node")
	}
	if res.GetPhase0Block() == nil && res.GetAltairBlock() == nil {
		return errors.New("nil block returned by beacon node")
	}
	if res.GetPhase0Block() != nil {
		return errors.New("phase 0 block returned after altair fork has occurred")
	}
	blk := wrapperv2.WrappedAltairSignedBeaconBlock(res.GetAltairBlock())
	if blk == nil || blk.IsNil() {
		return errors.New("nil altair block received from stream")
	}
	if blk.Block().Slot() < fSlot {
		return errors.Errorf("wanted a block >= %d but received %d", fSlot, blk.Block().Slot())
	}
	return nil
}
