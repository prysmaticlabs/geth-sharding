// Package attester defines all relevant functionality for a Attester actor
// within a sharded Ethereum blockchain.
package attester

import (
	"bytes"
	"context"
	"io"

	"github.com/golang/protobuf/ptypes/empty"
	"github.com/prysmaticlabs/prysm/client/types"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/rpc/v1"
	"github.com/sirupsen/logrus"
)

var log = logrus.WithField("prefix", "attester")

// Attester holds functionality required to run a collation attester
// in a sharded system. Must satisfy the Service interface defined in
// sharding/service.go.
type Attester struct {
	ctx            context.Context
	cancel         context.CancelFunc
	clientService  types.RPCClient
	validatorIndex int
	assignedHeight uint64
}

// NewAttester creates a new attester instance.
func NewAttester(ctx context.Context, clientService types.RPCClient) *Attester {
	ctx, cancel := context.WithCancel(ctx)
	return &Attester{
		ctx:           ctx,
		cancel:        cancel,
		clientService: clientService,
	}
}

// Start the main routine for a attester.
func (at *Attester) Start() {
	log.Info("Starting attester service")
	rpcClient := at.clientService.BeaconServiceClient()
	go at.fetchBeaconBlocks(rpcClient)
	go at.fetchCrystallizedState(rpcClient)
}

// Stop the main loop for notarizing collations.
func (at *Attester) Stop() error {
	log.Info("Stopping attester service")
	return nil
}

func (at *Attester) fetchBeaconBlocks(client pb.BeaconServiceClient) {
	stream, err := client.LatestBeaconBlock(at.ctx, &empty.Empty{})
	if err != nil {
		log.Fatalf("Could not setup beacon chain block streaming client: %v", err)
	}
	for {
		block, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatalf("Could not receive latest beacon block from stream: %v", err)
		}
		log.WithField("slotNumber", block.GetSlotNumber()).Info("Latest beacon block slot number")

		// Based on the height determined from the latest crystallized state, check if
		// it matches the latest received beacon height. If so, the attester has to perform
		// its responsibilities.

		// TODO: determine height based on slot number and epoch.
		//
		// if &at.assignedHeight != nil && beaconData.GetHeight() == at.assignedHeight {
		// 	log.Info("Assigned attestation height reached, performing attestation responsibility")
		// }
	}
}

func (at *Attester) fetchCrystallizedState(client pb.BeaconServiceClient) {
	stream, err := client.LatestCrystallizedState(at.ctx, &empty.Empty{})
	if err != nil {
		log.Fatalf("Could not setup crystallized beacon state streaming client: %v", err)
	}
	for {
		crystallizedState, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatalf("Could not receive latest crystallized beacon state from stream: %v", err)
		}
		// After receiving the crystallized state, get the number of active validators
		// and this attester's index in the list.
		activeValidators := crystallizedState.GetActiveValidators()
		validatorCount := len(activeValidators)

		validatorIndexSet := false

		for i, val := range activeValidators {
			// TODO: Check the public key instead of withdrawal address. This will
			// use BLS.
			if bytes.Equal(val.GetWithdrawalAddress(), []byte{}) {
				at.validatorIndex = i
				validatorIndexSet = true
				break
			}
		}

		// If validator was not found in the validator set was not set, keep listening for
		// crystallized states.
		if !validatorIndexSet {
			continue
		}

		req := &pb.ShuffleRequest{
			ValidatorCount: uint64(validatorCount),
			ValidatorIndex: uint64(at.validatorIndex),
		}

		res, err := client.ShuffleValidators(at.ctx, req)
		if err != nil {
			log.Errorf("Could not shuffle validator list: %v", err)
			continue
		}
		// Based on the cutoff and assigned heights, determine the beacon block
		// height at which attester has to perform its responsibility.
		currentAssignedHeights := res.GetAssignedAttestationHeights()
		currentCutoffs := res.GetCutoffIndices()

		// The algorithm functions as follows:
		// Given a list of heights: [0 19 38 57 12 31 50] and
		// A list of cutoff indices: [0 142 285 428 571 714 857 1000]
		// if the validator index is between 0-142, it can attest at height 0, if it is
		// between 142-285, that validator can attest at height 19, etc.
		heightIndex := 0
		for i := 0; i < len(currentCutoffs)-1; i++ {
			lowCutoff := currentCutoffs[i]
			highCutoff := currentCutoffs[i+1]
			if (uint64(at.validatorIndex) >= lowCutoff) && (uint64(at.validatorIndex) <= highCutoff) {
				break
			}
			heightIndex++
		}
		at.assignedHeight = currentAssignedHeights[heightIndex]
	}
}
