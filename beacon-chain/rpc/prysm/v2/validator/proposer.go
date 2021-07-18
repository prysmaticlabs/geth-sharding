package validator

import (
	"context"

	types "github.com/prysmaticlabs/eth2-types"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/state/interop"
	ethpb "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	prysmv2 "github.com/prysmaticlabs/prysm/proto/prysm/v2"
	"github.com/prysmaticlabs/prysm/proto/prysm/v2/wrapper"
	"github.com/prysmaticlabs/prysm/shared/aggregation/sync_contribution"
	"github.com/prysmaticlabs/prysm/shared/bls"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/prysmaticlabs/prysm/shared/params"
	bytesutil2 "github.com/wealdtech/go-bytesutil"
	"go.opencensus.io/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// GetBlock is called by a proposer during its assigned slot to request a block to sign
// by passing in the slot and the signed randao reveal of the slot. This is used by a validator
// after the altair fork epoch has been encountered.
func (vs *Server) GetBlock(ctx context.Context, req *ethpb.BlockRequest) (*prysmv2.BeaconBlock, error) {
	ctx, span := trace.StartSpan(ctx, "ProposerServer.GetBlock")
	defer span.End()
	span.AddAttributes(trace.Int64Attribute("slot", int64(req.Slot)))

	blkData, err := vs.V1Server.BuildBlockData(ctx, req)
	if err != nil {
		return nil, err
	}

	// Use zero hash as stub for state root to compute later.
	stateRoot := params.BeaconConfig().ZeroHash[:]

	syncAggregate, err := vs.getSyncAggregate(ctx, req.Slot-1, bytesutil.ToBytes32(blkData.ParentRoot))
	if err != nil {
		return nil, err
	}

	blk := &prysmv2.BeaconBlock{
		Slot:          req.Slot,
		ParentRoot:    blkData.ParentRoot,
		StateRoot:     stateRoot,
		ProposerIndex: blkData.ProposerIdx,
		Body: &prysmv2.BeaconBlockBody{
			Eth1Data:          blkData.Eth1Data,
			Deposits:          blkData.Deposits,
			Attestations:      blkData.Attestations,
			RandaoReveal:      req.RandaoReveal,
			ProposerSlashings: blkData.ProposerSlashings,
			AttesterSlashings: blkData.AttesterSlashings,
			VoluntaryExits:    blkData.VoluntaryExits,
			Graffiti:          blkData.Graffiti[:],
			SyncAggregate:     syncAggregate,
		},
	}
	// Compute state root with the newly constructed block.
	stateRoot, err = vs.V1Server.ComputeStateRoot(
		ctx,
		wrapper.WrappedAltairSignedBeaconBlock(
			&prysmv2.SignedBeaconBlock{Block: blk, Signature: make([]byte, 96)},
		),
	)
	if err != nil {
		interop.WriteBlockToDisk(
			wrapper.WrappedAltairSignedBeaconBlock(
				&prysmv2.SignedBeaconBlock{Block: blk},
			), true, /*failed*/
		)
		return nil, status.Errorf(codes.Internal, "Could not compute state root: %v", err)
	}
	blk.StateRoot = stateRoot

	return blk, nil
}

// ProposeBlock is called by a proposer during its assigned slot to create a block in an attempt
// to get it processed by the beacon node as the canonical head.
func (vs *Server) ProposeBlock(ctx context.Context, rBlk *prysmv2.SignedBeaconBlock) (*ethpb.ProposeResponse, error) {
	blk := wrapper.WrappedAltairSignedBeaconBlock(rBlk)
	return vs.V1Server.ProposeBlockGeneric(ctx, blk)
}

// getSyncAggregate retrieves the sync contributions from the pool to construct the sync aggregate object.
// The contributions are filtered based on matching of the input root and slot then profitability.
func (vs *Server) getSyncAggregate(ctx context.Context, slot types.Slot, root [32]byte) (*prysmv2.SyncAggregate, error) {
	ctx, span := trace.StartSpan(ctx, "ProposerServer.GetSyncAggregate")
	defer span.End()

	// Contributions have to match the input root
	contributions, err := vs.SyncCommitteePool.SyncCommitteeContributions(slot)
	if err != nil {
		return nil, err
	}
	proposerContributions := proposerSyncContributions(contributions).filterByBlockRoot(root)

	// Each sync subcommittee is 128 bits and the sync committee is 512 bits(mainnet).
	bitsHolder := [][]byte{}
	for i := uint64(0); i < params.BeaconConfig().SyncCommitteeSubnetCount; i++ {
		bitsHolder = append(bitsHolder, prysmv2.NewSyncCommitteeAggregationBits())
	}
	sigsHolder := make([]bls.Signature, 0, params.BeaconConfig().SyncCommitteeSize/params.BeaconConfig().SyncCommitteeSubnetCount)

	for i := uint64(0); i < params.BeaconConfig().SyncCommitteeSubnetCount; i++ {
		cs := proposerContributions.filterBySubIndex(i)
		aggregates, err := sync_contribution.Aggregate(cs)
		if err != nil {
			return nil, err
		}

		// Retrieve the most profitable contribution
		deduped, err := proposerSyncContributions(aggregates).dedup()
		if err != nil {
			return nil, err
		}
		c := deduped.mostProfitable()
		if c == nil {
			continue
		}
		bitsHolder[i] = c.AggregationBits
		sig, err := bls.SignatureFromBytes(c.Signature)
		if err != nil {
			return nil, err
		}
		sigsHolder = append(sigsHolder, sig)
	}

	// Aggregate all the contribution bits and signatures.
	var syncBits []byte
	for _, b := range bitsHolder {
		syncBits = append(syncBits, b...)
	}
	syncSig := bls.AggregateSignatures(sigsHolder)
	var syncSigBytes [96]byte
	if syncSig == nil {
		syncSigBytes = [96]byte{0xC0} // Infinity signature if itself is nil.
	} else {
		syncSigBytes = bytesutil2.ToBytes96(syncSig.Marshal())
	}

	return &prysmv2.SyncAggregate{
		SyncCommitteeBits:      syncBits,
		SyncCommitteeSignature: syncSigBytes[:],
	}, nil
}
