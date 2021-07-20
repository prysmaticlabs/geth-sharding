package client

import (
	"context"
	"fmt"

	emptypb "github.com/golang/protobuf/ptypes/empty"
	types "github.com/prysmaticlabs/eth2-types"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/altair"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	prysmv2 "github.com/prysmaticlabs/prysm/proto/prysm/v2"
	validatorpb "github.com/prysmaticlabs/prysm/proto/validator/accounts/v2"
	"github.com/prysmaticlabs/prysm/shared/bls"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/traceutil"
	"github.com/sirupsen/logrus"
	"go.opencensus.io/trace"
)

// SubmitSyncCommitteeMessage submits the sync committee message to the beacon chain.
func (v *validator) SubmitSyncCommitteeMessage(ctx context.Context, slot types.Slot, pubKey [48]byte) {
	ctx, span := trace.StartSpan(ctx, "validator.SubmitSyncCommitteeMessage")
	defer span.End()
	span.AddAttributes(trace.StringAttribute("validator", fmt.Sprintf("%#x", pubKey)))

	v.waitOneThirdOrValidBlock(ctx, slot)

	res, err := v.validatorClientV2.GetSyncMessageBlockRoot(ctx, &emptypb.Empty{})
	if err != nil {
		log.WithError(err).Error("Could not request sync message block root to sign")
		traceutil.AnnotateError(span, err)
		return
	}

	duty, err := v.duty(pubKey)
	if err != nil {
		log.WithError(err).Error("Could not fetch validator assignment")
		return
	}

	d, err := v.domainData(ctx, helpers.SlotToEpoch(slot), params.BeaconConfig().DomainSyncCommittee[:])
	if err != nil {
		log.WithError(err).Error("Could not get sync committee domain data")
		return
	}
	sszRoot := types.SSZBytes(res.Root)
	r, err := helpers.ComputeSigningRoot(&sszRoot, d.SignatureDomain)
	if err != nil {
		log.WithError(err).Error("Could not get sync committee message signing root")
		return
	}
	sig, err := v.keyManager.Sign(ctx, &validatorpb.SignRequest{
		PublicKey:       pubKey[:],
		SigningRoot:     r[:],
		SignatureDomain: d.SignatureDomain,
	})
	if err != nil {
		log.WithError(err).Error("Could not sign sync committee message")
		return
	}

	msg := &prysmv2.SyncCommitteeMessage{
		Slot:           slot,
		BlockRoot:      res.Root,
		ValidatorIndex: duty.ValidatorIndex,
		Signature:      sig.Marshal(),
	}
	if _, err := v.validatorClientV2.SubmitSyncMessage(ctx, msg); err != nil {
		log.WithError(err).Error("Could not submit sync committee message")
		return
	}

	log.WithFields(logrus.Fields{
		"slot":           msg.Slot,
		"blockRoot":      fmt.Sprintf("%#x", bytesutil.Trunc(msg.BlockRoot)),
		"validatorIndex": msg.ValidatorIndex,
	}).Info("Submitted new sync message")
}

// SubmitSignedContributionAndProof submits the signed sync committee contribution and proof to the beacon chain.
func (v *validator) SubmitSignedContributionAndProof(ctx context.Context, slot types.Slot, pubKey [48]byte) {
	ctx, span := trace.StartSpan(ctx, "validator.SubmitSignedContributionAndProof")
	defer span.End()
	span.AddAttributes(trace.StringAttribute("validator", fmt.Sprintf("%#x", pubKey)))

	duty, err := v.duty(pubKey)
	if err != nil {
		log.Errorf("Could not fetch validator assignment: %v", err)
		return
	}

	indexRes, err := v.validatorClientV2.GetSyncSubcommitteeIndex(ctx, &prysmv2.SyncSubcommitteeIndexRequest{
		PublicKey: pubKey[:],
		Slot:      slot,
	})
	if err != nil {
		log.Errorf("Could not get sync subcommittee index: %v", err)
		return
	}
	if len(indexRes.Indices) == 0 {
		log.Debug("Empty subcommittee index list, do nothing")
		return
	}

	selectionProofs := make([][]byte, len(indexRes.Indices))
	for i, index := range indexRes.Indices {
		subCommitteeSize := params.BeaconConfig().SyncCommitteeSize / params.BeaconConfig().SyncCommitteeSubnetCount
		subnet := uint64(index) / subCommitteeSize
		selectionProof, err := v.signSyncSelectionData(ctx, pubKey, subnet, slot)
		if err != nil {
			log.Errorf("Could not sign selection data: %v", err)
			return
		}
		selectionProofs[i] = selectionProof
	}

	v.waitToSlotTwoThirds(ctx, slot)

	for i, comIdx := range indexRes.Indices {
		if !altair.IsSyncCommitteeAggregator(selectionProofs[i]) {
			continue
		}
		subCommitteeSize := params.BeaconConfig().SyncCommitteeSize / params.BeaconConfig().SyncCommitteeSubnetCount
		subnet := uint64(comIdx) / subCommitteeSize
		contribution, err := v.validatorClientV2.GetSyncCommitteeContribution(ctx, &prysmv2.SyncCommitteeContributionRequest{
			Slot:      slot,
			PublicKey: pubKey[:],
			SubnetId:  subnet,
		})
		if err != nil {
			log.Errorf("Could not get sync committee contribution: %v", err)
			return
		}

		contributionAndProof := &prysmv2.ContributionAndProof{
			AggregatorIndex: duty.ValidatorIndex,
			Contribution:    contribution,
			SelectionProof:  selectionProofs[i],
		}
		sig, err := v.signContributionAndProof(ctx, pubKey, contributionAndProof)
		if err != nil {
			log.Errorf("Could not sign contribution and proof: %v", err)
			return
		}

		if _, err := v.validatorClientV2.SubmitSignedContributionAndProof(ctx, &prysmv2.SignedContributionAndProof{
			Message:   contributionAndProof,
			Signature: sig,
		}); err != nil {
			log.Errorf("Could not submit signed contribution and proof: %v", err)
			return
		}

		log.WithFields(logrus.Fields{
			"slot":              contributionAndProof.Contribution.Slot,
			"blockRoot":         fmt.Sprintf("%#x", bytesutil.Trunc(contributionAndProof.Contribution.BlockRoot)),
			"subcommitteeIndex": contributionAndProof.Contribution.SubcommitteeIndex,
			"aggregatorIndex":   contributionAndProof.AggregatorIndex,
			"bitsCount":         contributionAndProof.Contribution.AggregationBits.Count(),
		}).Info("Submitted new sync contribution and proof")
	}
}

// Signs input slot with domain sync committee selection proof. This is used to create the signature for sync committee selection.
func (v *validator) signSyncSelectionData(ctx context.Context, pubKey [48]byte, index uint64, slot types.Slot) (signature []byte, err error) {
	domain, err := v.domainData(ctx, helpers.SlotToEpoch(slot), params.BeaconConfig().DomainSyncCommitteeSelectionProof[:])
	if err != nil {
		return nil, err
	}

	data := &pb.SyncAggregatorSelectionData{
		Slot:              slot,
		SubcommitteeIndex: index,
	}
	root, err := helpers.ComputeSigningRoot(data, domain.SignatureDomain)
	if err != nil {
		return nil, err
	}
	sig, err := v.keyManager.Sign(ctx, &validatorpb.SignRequest{
		PublicKey:       pubKey[:],
		SigningRoot:     root[:],
		SignatureDomain: domain.SignatureDomain,
	})
	if err != nil {
		return nil, err
	}

	return sig.Marshal(), nil
}

// This returns the signature of validator signing over sync committee contribution and proof object.
func (v *validator) signContributionAndProof(ctx context.Context, pubKey [48]byte, c *prysmv2.ContributionAndProof) ([]byte, error) {
	d, err := v.domainData(ctx, helpers.SlotToEpoch(c.Contribution.Slot), params.BeaconConfig().DomainContributionAndProof[:])
	if err != nil {
		return nil, err
	}
	var sig bls.Signature
	root, err := helpers.ComputeSigningRoot(c, d.SignatureDomain)
	if err != nil {
		return nil, err
	}
	sig, err = v.keyManager.Sign(ctx, &validatorpb.SignRequest{
		PublicKey:       pubKey[:],
		SigningRoot:     root[:],
		SignatureDomain: d.SignatureDomain,
	})
	if err != nil {
		return nil, err
	}

	return sig.Marshal(), nil
}
