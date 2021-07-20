package validator

import (
	"bytes"
	"context"

	"github.com/pkg/errors"
	types "github.com/prysmaticlabs/eth2-types"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	prysmv2 "github.com/prysmaticlabs/prysm/proto/prysm/v2"
	"github.com/prysmaticlabs/prysm/shared/bls"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/prysmaticlabs/prysm/shared/params"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

// GetSyncMessageBlockRoot retrieves the sync committee block root of the beacon chain.
func (vs *Server) GetSyncMessageBlockRoot(
	ctx context.Context, _ *emptypb.Empty,
) (*prysmv2.SyncMessageBlockRootResponse, error) {
	r, err := vs.HeadFetcher.HeadRoot(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Could not retrieve head root: %v", err)
	}

	return &prysmv2.SyncMessageBlockRootResponse{
		Root: r,
	}, nil
}

// SubmitSyncMessage submits the sync committee message to the network.
// It also saves the sync committee message into the pending pool for block inclusion.
func (vs *Server) SubmitSyncMessage(ctx context.Context, msg *prysmv2.SyncCommitteeMessage) (*emptypb.Empty, error) {
	errs, ctx := errgroup.WithContext(ctx)

	idxResp, err := vs.syncSubcommitteeIndex(ctx, msg.ValidatorIndex, msg.Slot)
	if err != nil {
		return &emptypb.Empty{}, err
	}
	// Broadcasting and saving message into the pool in parallel. As one fail should not affect another.
	// This broadcasts for all subnets.
	for _, id := range idxResp.Indices {
		subCommitteeSize := params.BeaconConfig().SyncCommitteeSize / params.BeaconConfig().SyncCommitteeSubnetCount
		subnet := uint64(id) / subCommitteeSize
		errs.Go(func() error {
			return vs.P2P.BroadcastSyncCommitteeMessage(ctx, subnet, msg)
		})
	}

	if err := vs.SyncCommitteePool.SaveSyncCommitteeMessage(msg); err != nil {
		return &emptypb.Empty{}, err
	}

	// Wait for p2p broadcast to complete and return the first error (if any)
	err = errs.Wait()
	return &emptypb.Empty{}, err
}

// GetSyncSubcommitteeIndex is called by a sync committee participant to get
// its subcommittee index for sync message aggregation duty.
func (vs *Server) GetSyncSubcommitteeIndex(
	ctx context.Context, req *prysmv2.SyncSubcommitteeIndexRequest,
) (*prysmv2.SyncSubcommitteeIndexResponse, error) {
	index, exists := vs.HeadFetcher.HeadPublicKeyToValidatorIndex(ctx, bytesutil.ToBytes48(req.PublicKey))
	if !exists {
		return nil, errors.New("public key does not exist in state")
	}
	indices, err := vs.syncSubcommitteeIndex(ctx, index, req.Slot)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Could not get sync subcommittee index: %v", err)
	}
	return indices, nil
}

// syncSubcommitteeIndex returns a list of subcommittee index of a validator and slot for sync message aggregation duty.
func (vs *Server) syncSubcommitteeIndex(
	ctx context.Context, index types.ValidatorIndex, slot types.Slot,
) (*prysmv2.SyncSubcommitteeIndexResponse, error) {

	nextSlotEpoch := helpers.SlotToEpoch(slot + 1)
	currentEpoch := helpers.SlotToEpoch(slot)

	switch {
	case helpers.SyncCommitteePeriod(nextSlotEpoch) == helpers.SyncCommitteePeriod(currentEpoch):
		indices, err := vs.HeadFetcher.HeadCurrentSyncCommitteeIndices(ctx, index, slot)
		if err != nil {
			return nil, err
		}
		return &prysmv2.SyncSubcommitteeIndexResponse{
			Indices: indices,
		}, nil
	// At sync committee period boundary, validator should sample the next epoch sync committee.
	case helpers.SyncCommitteePeriod(nextSlotEpoch) == helpers.SyncCommitteePeriod(currentEpoch)+1:
		indices, err := vs.HeadFetcher.HeadNextSyncCommitteeIndices(ctx, index, slot)
		if err != nil {
			return nil, err
		}
		return &prysmv2.SyncSubcommitteeIndexResponse{
			Indices: indices,
		}, nil
	default:
		// Impossible condition.
		return nil, errors.New("could get calculate sync subcommittee based on the period")
	}
}

// GetSyncCommitteeContribution is called by a sync committee aggregator
// to retrieve sync committee contribution object.
func (vs *Server) GetSyncCommitteeContribution(
	ctx context.Context, req *prysmv2.SyncCommitteeContributionRequest,
) (*prysmv2.SyncCommitteeContribution, error) {
	msgs, err := vs.SyncCommitteePool.SyncCommitteeMessages(req.Slot)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Could not get sync subcommittee messages: %v", err)
	}
	headRoot, err := vs.HeadFetcher.HeadRoot(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Could not get head root: %v", err)
	}

	subCommitteeSize := params.BeaconConfig().SyncCommitteeSize / params.BeaconConfig().SyncCommitteeSubnetCount
	sigs := make([]bls.Signature, 0, subCommitteeSize)
	bits := prysmv2.NewSyncCommitteeAggregationBits()
	for _, msg := range msgs {
		if bytes.Equal(headRoot, msg.BlockRoot) {
			idxResp, err := vs.syncSubcommitteeIndex(ctx, msg.ValidatorIndex, req.Slot)
			if err != nil {
				return nil, status.Errorf(codes.Internal, "Could not get sync subcommittee index: %v", err)
			}
			for _, index := range idxResp.Indices {
				i := uint64(index)
				subnetIndex := i / subCommitteeSize
				if subnetIndex == req.SubnetId {
					bits.SetBitAt(i%subCommitteeSize, true)
					sig, err := bls.SignatureFromBytes(msg.Signature)
					if err != nil {
						return nil, status.Errorf(
							codes.Internal,
							"Could not get bls signature from bytes: %v",
							err,
						)
					}
					sigs = append(sigs, sig)
				}
			}
		}
	}
	aggregatedSig := make([]byte, 96)
	aggregatedSig[0] = 0xC0
	if len(sigs) != 0 {
		aggregatedSig = bls.AggregateSignatures(sigs).Marshal()
	}
	contribution := &prysmv2.SyncCommitteeContribution{
		Slot:              req.Slot,
		BlockRoot:         headRoot,
		SubcommitteeIndex: req.SubnetId,
		AggregationBits:   bits,
		Signature:         aggregatedSig,
	}

	return contribution, nil
}

// SubmitSignedContributionAndProof is called by a sync committee aggregator
// to submit signed contribution and proof object.
func (vs *Server) SubmitSignedContributionAndProof(
	ctx context.Context, s *prysmv2.SignedContributionAndProof,
) (*emptypb.Empty, error) {
	errs, ctx := errgroup.WithContext(ctx)

	// Broadcasting and saving contribution into the pool in parallel. As one fail should not affect another.
	errs.Go(func() error {
		return vs.P2P.Broadcast(ctx, s)
	})

	if err := vs.SyncCommitteePool.SaveSyncCommitteeContribution(s.Message.Contribution); err != nil {
		return nil, err
	}

	// Wait for p2p broadcast to complete and return the first error (if any)
	err := errs.Wait()
	return &emptypb.Empty{}, err
}
