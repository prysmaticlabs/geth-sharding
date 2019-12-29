package client

// Validator client proposer functions.
import (
	"context"
	"encoding/binary"
	"fmt"
	"github.com/prysmaticlabs/prysm/validator/db"

	"github.com/pkg/errors"
	ethpb "github.com/prysmaticlabs/ethereumapis/eth/v1alpha1"
	"github.com/prysmaticlabs/go-ssz"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/sirupsen/logrus"
	"go.opencensus.io/trace"
)

// ProposeBlock A new beacon block for a given slot. This method collects the
// previous beacon block, any pending deposits, and ETH1 data from the beacon
// chain node to construct the new block. The new block is then processed with
// the state root computation, and finally signed by the validator before being
// sent back to the beacon node for broadcasting.
func (v *validator) ProposeBlock(ctx context.Context, slot uint64, pubKey [48]byte) {
	if slot == 0 {
		log.Info("Assigned to genesis slot, skipping proposal")
		return
	}
	ctx, span := trace.StartSpan(ctx, "validator.ProposeBlock")
	defer span.End()

	span.AddAttributes(trace.StringAttribute("validator", fmt.Sprintf("%#x", pubKey)))
	log := log.WithField("pubKey", fmt.Sprintf("%#x", bytesutil.Trunc(pubKey[:])))

	// Sign randao reveal, it's used to request block from beacon node
	epoch := slot / params.BeaconConfig().SlotsPerEpoch
	randaoReveal, err := v.signRandaoReveal(ctx, pubKey, epoch)
	if err != nil {
		log.WithError(err).Error("Failed to sign randao reveal")
		return
	}

	// Request block from beacon node
	b, err := v.validatorClient.GetBlock(ctx, &ethpb.BlockRequest{
		Slot:         slot,
		RandaoReveal: randaoReveal,
		Graffiti:     v.graffiti,
	})
	if err != nil {
		log.WithError(err).Error("Failed to request block from beacon node")
		return
	}

	history, err := v.db.ProposalHistory(pubKey[:])
	if err != nil {
		log.WithError(err).Error("Failed to get proposal history")
		return
	}

	if db.HasProposedForEpoch(history, epoch) {
		log.Info("Tried to sign a double proposal, rejected")
		return
	}

	log.Info("Validator has not proposed for this epoch")

	// Sign returned block from beacon node
	sig, err := v.signBlock(ctx, pubKey, epoch, b)
	if err != nil {
		log.WithError(err).Error("Failed to sign block")
		return
	}
	b.Signature = sig

	db.SetProposedForEpoch(history, epoch)
	if err := v.db.SaveProposalHistory(pubKey[:], history); err != nil {
		log.WithError(err).Error("Failed to save updated proposal history")
		return
	}

	// Propose and broadcast block via beacon node
	blkResp, err := v.validatorClient.ProposeBlock(ctx, b)
	if err != nil {
		log.WithError(err).Error("Failed to propose block")
		return
	}

	span.AddAttributes(
		trace.StringAttribute("blockRoot", fmt.Sprintf("%#x", blkResp.BlockRoot)),
		trace.Int64Attribute("numDeposits", int64(len(b.Body.Deposits))),
		trace.Int64Attribute("numAttestations", int64(len(b.Body.Attestations))),
	)

	log.WithField("signature", fmt.Sprintf("%#x", b.Signature)).Debug("block signature")
	blkRoot := fmt.Sprintf("%#x", bytesutil.Trunc(blkResp.BlockRoot))
	log.WithFields(logrus.Fields{
		"slot":            b.Slot,
		"blockRoot":       blkRoot,
		"numAttestations": len(b.Body.Attestations),
		"numDeposits":     len(b.Body.Deposits),
	}).Info("Submitted new block")
}

// ProposeExit --
func (v *validator) ProposeExit(ctx context.Context, exit *ethpb.VoluntaryExit) error {
	return errors.New("unimplemented")
}

// Sign randao reveal with randao domain and private key.
func (v *validator) signRandaoReveal(ctx context.Context, pubKey [48]byte, epoch uint64) ([]byte, error) {
	domain, err := v.validatorClient.DomainData(ctx, &ethpb.DomainRequest{
		Epoch:  epoch,
		Domain: params.BeaconConfig().DomainRandao,
	})
	if err != nil {
		return nil, errors.Wrap(err, "could not get domain data")
	}
	buf := make([]byte, 32)
	binary.LittleEndian.PutUint64(buf, epoch)
	randaoReveal := v.keys[pubKey].SecretKey.Sign(buf, domain.SignatureDomain)
	return randaoReveal.Marshal(), nil
}

// Sign block with proposer domain and private key.
func (v *validator) signBlock(ctx context.Context, pubKey [48]byte, epoch uint64, b *ethpb.BeaconBlock) ([]byte, error) {
	domain, err := v.validatorClient.DomainData(ctx, &ethpb.DomainRequest{
		Epoch:  epoch,
		Domain: params.BeaconConfig().DomainBeaconProposer,
	})
	if err != nil {
		return nil, errors.Wrap(err, "could not get domain data")
	}
	root, err := ssz.SigningRoot(b)
	if err != nil {
		return nil, errors.Wrap(err, "could not get signing root")
	}
	sig := v.keys[pubKey].SecretKey.Sign(root[:], domain.SignatureDomain)
	return sig.Marshal(), nil
}
