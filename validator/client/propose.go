package client

// Validator client proposer functions.
import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"
	types "github.com/prysmaticlabs/eth2-types"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	ethpb "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	wrapperv1 "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1/wrapper"
	protoInterfaces "github.com/prysmaticlabs/prysm/proto/interfaces"
	prysmv2 "github.com/prysmaticlabs/prysm/proto/prysm/v2"
	wrapperv2 "github.com/prysmaticlabs/prysm/proto/prysm/v2/wrapper"
	validatorpb "github.com/prysmaticlabs/prysm/proto/validator/accounts/v2"
	"github.com/prysmaticlabs/prysm/shared/bls"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/prysmaticlabs/prysm/shared/mputil"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/rand"
	"github.com/prysmaticlabs/prysm/shared/timeutils"
	"github.com/prysmaticlabs/prysm/shared/version"
	"github.com/prysmaticlabs/prysm/validator/client/iface"
	"github.com/sirupsen/logrus"
	"go.opencensus.io/trace"
	"google.golang.org/protobuf/types/known/emptypb"
)

type signingFunc func(context.Context, *validatorpb.SignRequest) (bls.Signature, error)

const domainDataErr = "could not get domain data"
const signingRootErr = "could not get signing root"
const signExitErr = "could not sign voluntary exit proposal"

// ProposeBlock proposes a new beacon block for a given slot. This method collects the
// previous beacon block, any pending deposits, and ETH1 data from the beacon
// chain node to construct the new block. The new block is then processed with
// the state root computation, and finally signed by the validator before being
// sent back to the beacon node for broadcasting.
func (v *validator) ProposeBlock(ctx context.Context, slot types.Slot, pubKey [48]byte) {
	currEpoch := helpers.SlotToEpoch(slot)
	switch {
	case currEpoch >= params.BeaconConfig().AltairForkEpoch:
		v.proposeBlockV2(ctx, slot, pubKey)
	default:
		v.proposeBlock(ctx, slot, pubKey)
	}
}

func (v *validator) proposeBlock(ctx context.Context, slot types.Slot, pubKey [48]byte) {
	if slot == 0 {
		log.Debug("Assigned to genesis slot, skipping proposal")
		return
	}
	lock := mputil.NewMultilock(fmt.Sprint(iface.RoleProposer), string(pubKey[:]))
	lock.Lock()
	defer lock.Unlock()
	ctx, span := trace.StartSpan(ctx, "validator.proposeBlock")
	defer span.End()
	fmtKey := fmt.Sprintf("%#x", pubKey[:])

	span.AddAttributes(trace.StringAttribute("validator", fmt.Sprintf("%#x", pubKey)))
	log := log.WithField("pubKey", fmt.Sprintf("%#x", bytesutil.Trunc(pubKey[:])))

	// Sign randao reveal, it's used to request block from beacon node
	epoch := types.Epoch(slot / params.BeaconConfig().SlotsPerEpoch)
	randaoReveal, err := v.signRandaoReveal(ctx, pubKey, epoch)
	if err != nil {
		log.WithError(err).Error("Failed to sign randao reveal")
		if v.emitAccountMetrics {
			ValidatorProposeFailVec.WithLabelValues(fmtKey).Inc()
		}
		return
	}

	g, err := v.getGraffiti(ctx, pubKey)
	if err != nil {
		// Graffiti is not a critical enough to fail block production and cause
		// validator to miss block reward. When failed, validator should continue
		// to produce the block.
		log.WithError(err).Warn("Could not get graffiti")
	}

	// Request block from beacon node
	b, err := v.validatorClient.GetBlock(ctx, &ethpb.BlockRequest{
		Slot:         slot,
		RandaoReveal: randaoReveal,
		Graffiti:     g,
	})
	if err != nil {
		log.WithField("blockSlot", slot).WithError(err).Error("Failed to request block from beacon node")
		if v.emitAccountMetrics {
			ValidatorProposeFailVec.WithLabelValues(fmtKey).Inc()
		}
		return
	}

	// Sign returned block from beacon node
	sig, domain, err := v.signBlock(ctx, pubKey, epoch, wrapperv1.WrappedPhase0BeaconBlock(b))
	if err != nil {
		log.WithError(err).Error("Failed to sign block")
		if v.emitAccountMetrics {
			ValidatorProposeFailVec.WithLabelValues(fmtKey).Inc()
		}
		return
	}
	blk := &ethpb.SignedBeaconBlock{
		Block:     b,
		Signature: sig,
	}

	signingRoot, err := helpers.ComputeSigningRoot(b, domain.SignatureDomain)
	if err != nil {
		if v.emitAccountMetrics {
			ValidatorProposeFailVec.WithLabelValues(fmtKey).Inc()
		}
		log.WithError(err).Error("Failed to compute signing root for block")
		return
	}

	if err := v.preBlockSignValidations(ctx, pubKey, wrapperv1.WrappedPhase0BeaconBlock(b), signingRoot); err != nil {
		log.WithFields(
			blockLogFields(pubKey, wrapperv1.WrappedPhase0BeaconBlock(b), nil),
		).WithError(err).Error("Failed block slashing protection check")
		return
	}

	if err := v.postBlockSignUpdate(ctx, pubKey, wrapperv1.WrappedPhase0SignedBeaconBlock(blk), signingRoot); err != nil {
		log.WithFields(
			blockLogFields(pubKey, wrapperv1.WrappedPhase0BeaconBlock(b), sig),
		).WithError(err).Error("Failed block slashing protection check")
		return
	}

	// Propose and broadcast block via beacon node
	blkResp, err := v.validatorClient.ProposeBlock(ctx, blk)
	if err != nil {
		log.WithError(err).Error("Failed to propose block")
		if v.emitAccountMetrics {
			ValidatorProposeFailVec.WithLabelValues(fmtKey).Inc()
		}
		return
	}

	span.AddAttributes(
		trace.StringAttribute("blockRoot", fmt.Sprintf("%#x", blkResp.BlockRoot)),
		trace.Int64Attribute("numDeposits", int64(len(b.Body.Deposits))),
		trace.Int64Attribute("numAttestations", int64(len(b.Body.Attestations))),
	)

	blkRoot := fmt.Sprintf("%#x", bytesutil.Trunc(blkResp.BlockRoot))
	log.WithFields(logrus.Fields{
		"slot":            b.Slot,
		"blockRoot":       blkRoot,
		"numAttestations": len(b.Body.Attestations),
		"numDeposits":     len(b.Body.Deposits),
		"graffiti":        string(b.Body.Graffiti),
	}).Info("Submitted new block")

	if v.emitAccountMetrics {
		ValidatorProposeSuccessVec.WithLabelValues(fmtKey).Inc()
	}
}

// This is a routine to propose altair compatible beacon blocks.
func (v *validator) proposeBlockV2(ctx context.Context, slot types.Slot, pubKey [48]byte) {
	if slot == 0 {
		log.Debug("Assigned to genesis slot, skipping proposal")
		return
	}
	lock := mputil.NewMultilock(fmt.Sprint(iface.RoleProposer), string(pubKey[:]))
	lock.Lock()
	defer lock.Unlock()
	ctx, span := trace.StartSpan(ctx, "validator.proposeBlockV2")
	defer span.End()
	fmtKey := fmt.Sprintf("%#x", pubKey[:])

	span.AddAttributes(trace.StringAttribute("validator", fmt.Sprintf("%#x", pubKey)))
	log := log.WithField("pubKey", fmt.Sprintf("%#x", bytesutil.Trunc(pubKey[:])))

	// Sign randao reveal, it's used to request block from beacon node
	epoch := types.Epoch(slot / params.BeaconConfig().SlotsPerEpoch)
	randaoReveal, err := v.signRandaoReveal(ctx, pubKey, epoch)
	if err != nil {
		log.WithError(err).Error("Failed to sign randao reveal")
		if v.emitAccountMetrics {
			ValidatorProposeFailVec.WithLabelValues(fmtKey).Inc()
		}
		return
	}

	g, err := v.getGraffiti(ctx, pubKey)
	if err != nil {
		// Graffiti is not a critical enough to fail block production and cause
		// validator to miss block reward. When failed, validator should continue
		// to produce the block.
		log.WithError(err).Warn("Could not get graffiti")
	}

	// Request block from beacon node
	b, err := v.validatorClientV2.GetBlock(ctx, &ethpb.BlockRequest{
		Slot:         slot,
		RandaoReveal: randaoReveal,
		Graffiti:     g,
	})
	if err != nil {
		log.WithField("blockSlot", slot).WithError(err).Error("Failed to request block from beacon node")
		if v.emitAccountMetrics {
			ValidatorProposeFailVec.WithLabelValues(fmtKey).Inc()
		}
		return
	}

	// Sign returned block from beacon node
	sig, domain, err := v.signBlock(ctx, pubKey, epoch, wrapperv2.WrappedAltairBeaconBlock(b))
	if err != nil {
		log.WithError(err).Error("Failed to sign block")
		if v.emitAccountMetrics {
			ValidatorProposeFailVec.WithLabelValues(fmtKey).Inc()
		}
		return
	}
	blk := &prysmv2.SignedBeaconBlock{
		Block:     b,
		Signature: sig,
	}

	signingRoot, err := helpers.ComputeSigningRoot(b, domain.SignatureDomain)
	if err != nil {
		if v.emitAccountMetrics {
			ValidatorProposeFailVec.WithLabelValues(fmtKey).Inc()
		}
		log.WithError(err).Error("Failed to compute signing root for block")
		return
	}

	if err := v.preBlockSignValidations(ctx, pubKey, wrapperv2.WrappedAltairBeaconBlock(b), signingRoot); err != nil {
		log.WithFields(
			blockLogFields(pubKey, wrapperv2.WrappedAltairBeaconBlock(b), nil),
		).WithError(err).Error("Failed block slashing protection check")
		return
	}

	if err := v.postBlockSignUpdate(ctx, pubKey, wrapperv2.WrappedAltairSignedBeaconBlock(blk), signingRoot); err != nil {
		log.WithFields(
			blockLogFields(pubKey, wrapperv2.WrappedAltairBeaconBlock(b), sig),
		).WithError(err).Error("Failed block slashing protection check")
		return
	}

	// Propose and broadcast block via beacon node
	blkResp, err := v.validatorClientV2.ProposeBlock(ctx, blk)
	if err != nil {
		log.WithError(err).Error("Failed to propose block")
		if v.emitAccountMetrics {
			ValidatorProposeFailVec.WithLabelValues(fmtKey).Inc()
		}
		return
	}

	span.AddAttributes(
		trace.StringAttribute("blockRoot", fmt.Sprintf("%#x", blkResp.BlockRoot)),
		trace.Int64Attribute("numDeposits", int64(len(b.Body.Deposits))),
		trace.Int64Attribute("numAttestations", int64(len(b.Body.Attestations))),
	)

	blkRoot := fmt.Sprintf("%#x", bytesutil.Trunc(blkResp.BlockRoot))
	log.WithFields(logrus.Fields{
		"slot":            b.Slot,
		"blockRoot":       blkRoot,
		"numAttestations": len(b.Body.Attestations),
		"numDeposits":     len(b.Body.Deposits),
		"graffiti":        string(b.Body.Graffiti),
		"fork":            "altair",
	}).Info("Submitted new block")

	if v.emitAccountMetrics {
		ValidatorProposeSuccessVec.WithLabelValues(fmtKey).Inc()
	}
}

// ProposeExit performs a voluntary exit on a validator.
// The exit is signed by the validator before being sent to the beacon node for broadcasting.
func ProposeExit(
	ctx context.Context,
	validatorClient ethpb.BeaconNodeValidatorClient,
	nodeClient ethpb.NodeClient,
	signer signingFunc,
	pubKey []byte,
) error {
	ctx, span := trace.StartSpan(ctx, "validator.ProposeExit")
	defer span.End()

	indexResponse, err := validatorClient.ValidatorIndex(ctx, &ethpb.ValidatorIndexRequest{PublicKey: pubKey})
	if err != nil {
		return errors.Wrap(err, "gRPC call to get validator index failed")
	}
	genesisResponse, err := nodeClient.GetGenesis(ctx, &emptypb.Empty{})
	if err != nil {
		return errors.Wrap(err, "gRPC call to get genesis time failed")
	}
	totalSecondsPassed := timeutils.Now().Unix() - genesisResponse.GenesisTime.Seconds
	currentEpoch := types.Epoch(uint64(totalSecondsPassed) / uint64(params.BeaconConfig().SlotsPerEpoch.Mul(params.BeaconConfig().SecondsPerSlot)))

	exit := &ethpb.VoluntaryExit{Epoch: currentEpoch, ValidatorIndex: indexResponse.Index}
	sig, err := signVoluntaryExit(ctx, validatorClient, signer, pubKey, exit)
	if err != nil {
		return errors.Wrap(err, "failed to sign voluntary exit")
	}

	signedExit := &ethpb.SignedVoluntaryExit{Exit: exit, Signature: sig}
	exitResp, err := validatorClient.ProposeExit(ctx, signedExit)
	if err != nil {
		return errors.Wrap(err, "failed to propose voluntary exit")
	}

	span.AddAttributes(
		trace.StringAttribute("exitRoot", fmt.Sprintf("%#x", exitResp.ExitRoot)),
	)

	return nil
}

// Sign randao reveal with randao domain and private key.
func (v *validator) signRandaoReveal(ctx context.Context, pubKey [48]byte, epoch types.Epoch) ([]byte, error) {
	domain, err := v.domainData(ctx, epoch, params.BeaconConfig().DomainRandao[:])
	if err != nil {
		return nil, errors.Wrap(err, domainDataErr)
	}
	if domain == nil {
		return nil, errors.New(domainDataErr)
	}

	var randaoReveal bls.Signature
	sszUint := types.SSZUint64(epoch)
	root, err := helpers.ComputeSigningRoot(&sszUint, domain.SignatureDomain)
	if err != nil {
		return nil, err
	}
	randaoReveal, err = v.keyManager.Sign(ctx, &validatorpb.SignRequest{
		PublicKey:       pubKey[:],
		SigningRoot:     root[:],
		SignatureDomain: domain.SignatureDomain,
		Object:          &validatorpb.SignRequest_Epoch{Epoch: epoch},
	})
	if err != nil {
		return nil, err
	}
	return randaoReveal.Marshal(), nil
}

// Sign block with proposer domain and private key.
func (v *validator) signBlock(ctx context.Context, pubKey [48]byte, epoch types.Epoch, b protoInterfaces.BeaconBlock) ([]byte, *ethpb.DomainResponse, error) {
	domain, err := v.domainData(ctx, epoch, params.BeaconConfig().DomainBeaconProposer[:])
	if err != nil {
		return nil, nil, errors.Wrap(err, domainDataErr)
	}
	if domain == nil {
		return nil, nil, errors.New(domainDataErr)
	}

	var sig bls.Signature
	switch b.Version() {
	case version.Altair:
		block, ok := b.Proto().(*prysmv2.BeaconBlock)
		if !ok {
			return nil, nil, errors.New("could not convert obj to beacon block altair")
		}
		blockRoot, err := helpers.ComputeSigningRoot(block, domain.SignatureDomain)
		if err != nil {
			return nil, nil, errors.Wrap(err, signingRootErr)
		}
		sig, err = v.keyManager.Sign(ctx, &validatorpb.SignRequest{
			PublicKey:       pubKey[:],
			SigningRoot:     blockRoot[:],
			SignatureDomain: domain.SignatureDomain,
			Object:          &validatorpb.SignRequest_BlockV2{BlockV2: block},
		})
		if err != nil {
			return nil, nil, errors.Wrap(err, "could not sign block proposal")
		}
		return sig.Marshal(), domain, nil
	case version.Phase0:
		block, ok := b.Proto().(*ethpb.BeaconBlock)
		if !ok {
			return nil, nil, errors.New("could not convert obj to beacon block phase 0")
		}
		blockRoot, err := helpers.ComputeSigningRoot(block, domain.SignatureDomain)
		if err != nil {
			return nil, nil, errors.Wrap(err, signingRootErr)
		}
		sig, err = v.keyManager.Sign(ctx, &validatorpb.SignRequest{
			PublicKey:       pubKey[:],
			SigningRoot:     blockRoot[:],
			SignatureDomain: domain.SignatureDomain,
			Object:          &validatorpb.SignRequest_Block{Block: block},
		})
		if err != nil {
			return nil, nil, errors.Wrap(err, "could not sign block proposal")
		}
		return sig.Marshal(), domain, nil
	default:
		return nil, nil, errors.New("unknown block type")
	}
}

// Sign voluntary exit with proposer domain and private key.
func signVoluntaryExit(
	ctx context.Context,
	validatorClient ethpb.BeaconNodeValidatorClient,
	signer signingFunc,
	pubKey []byte,
	exit *ethpb.VoluntaryExit,
) ([]byte, error) {
	req := &ethpb.DomainRequest{
		Epoch:  exit.Epoch,
		Domain: params.BeaconConfig().DomainVoluntaryExit[:],
	}

	domain, err := validatorClient.DomainData(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err, domainDataErr)
	}
	if domain == nil {
		return nil, errors.New(domainDataErr)
	}

	exitRoot, err := helpers.ComputeSigningRoot(exit, domain.SignatureDomain)
	if err != nil {
		return nil, errors.Wrap(err, signingRootErr)
	}

	sig, err := signer(ctx, &validatorpb.SignRequest{
		PublicKey:       pubKey,
		SigningRoot:     exitRoot[:],
		SignatureDomain: domain.SignatureDomain,
		Object:          &validatorpb.SignRequest_Exit{Exit: exit},
	})
	if err != nil {
		return nil, errors.Wrap(err, signExitErr)
	}
	return sig.Marshal(), nil
}

// Gets the graffiti from cli or file for the validator public key.
func (v *validator) getGraffiti(ctx context.Context, pubKey [48]byte) ([]byte, error) {
	// When specified, default graffiti from the command line takes the first priority.
	if len(v.graffiti) != 0 {
		return v.graffiti, nil
	}

	if v.graffitiStruct == nil {
		return nil, errors.New("graffitiStruct can't be nil")
	}

	// When specified, individual validator specified graffiti takes the second priority.
	idx, err := v.validatorClient.ValidatorIndex(ctx, &ethpb.ValidatorIndexRequest{PublicKey: pubKey[:]})
	if err != nil {
		return []byte{}, err
	}
	g, ok := v.graffitiStruct.Specific[idx.Index]
	if ok {
		return []byte(g), nil
	}

	// When specified, a graffiti from the ordered list in the file take third priority.
	if v.graffitiOrderedIndex < uint64(len(v.graffitiStruct.Ordered)) {
		graffiti := v.graffitiStruct.Ordered[v.graffitiOrderedIndex]
		v.graffitiOrderedIndex = v.graffitiOrderedIndex + 1
		err := v.db.SaveGraffitiOrderedIndex(ctx, v.graffitiOrderedIndex)
		if err != nil {
			return nil, errors.Wrap(err, "failed to update graffiti ordered index")
		}
		return []byte(graffiti), nil
	}

	// When specified, a graffiti from the random list in the file take fourth priority.
	if len(v.graffitiStruct.Random) != 0 {
		r := rand.NewGenerator()
		r.Seed(time.Now().Unix())
		i := r.Uint64() % uint64(len(v.graffitiStruct.Random))
		return []byte(v.graffitiStruct.Random[i]), nil
	}

	// Finally, default graffiti if specified in the file will be used.
	if v.graffitiStruct.Default != "" {
		return []byte(v.graffitiStruct.Default), nil
	}

	return []byte{}, nil
}
