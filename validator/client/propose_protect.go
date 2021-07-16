package client

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	ethpb "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/proto/eth/v1alpha1/wrapper"
	"github.com/prysmaticlabs/prysm/shared/blockutil"
	"github.com/prysmaticlabs/prysm/shared/featureconfig"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/sirupsen/logrus"
)

var failedBlockSignLocalErr = "attempted to sign a double proposal, block rejected by local protection"
var failedBlockSignExternalErr = "attempted a double proposal, block rejected by remote slashing protection"

func (v *validator) slashableProposalCheck(
	ctx context.Context, pubKey [48]byte, signedBlock *ethpb.SignedBeaconBlock, signingRoot [32]byte,
) error {
	fmtKey := fmt.Sprintf("%#x", pubKey[:])

	block := signedBlock.Block
	prevSigningRoot, proposalAtSlotExists, err := v.db.ProposalHistoryForSlot(ctx, pubKey, block.Slot)
	if err != nil {
		if v.emitAccountMetrics {
			ValidatorProposeFailVec.WithLabelValues(fmtKey).Inc()
		}
		return errors.Wrap(err, "failed to get proposal history")
	}

	lowestSignedProposalSlot, lowestProposalExists, err := v.db.LowestSignedProposal(ctx, pubKey)
	if err != nil {
		return err
	}

	// If a proposal exists in our history for the slot, we check the following:
	// If the signing root is empty (zero hash), then we consider it slashable. If signing root is not empty,
	// we check if it is different than the incoming block's signing root. If that is the case,
	// we consider that proposal slashable.
	signingRootIsDifferent := prevSigningRoot == params.BeaconConfig().ZeroHash || prevSigningRoot != signingRoot
	if proposalAtSlotExists && signingRootIsDifferent {
		if v.emitAccountMetrics {
			ValidatorProposeFailVec.WithLabelValues(fmtKey).Inc()
		}
		return errors.New(failedBlockSignLocalErr)
	}

	// Based on EIP3076, validator should refuse to sign any proposal with slot less
	// than or equal to the minimum signed proposal present in the DB for that public key.
	// In the case the slot of the incoming block is equal to the minimum signed proposal, we
	// then also check the signing root is different.
	if lowestProposalExists && signingRootIsDifferent && lowestSignedProposalSlot >= block.Slot {
		return fmt.Errorf(
			"could not sign block with slot <= lowest signed slot in db, lowest signed slot: %d >= block slot: %d",
			lowestSignedProposalSlot,
			block.Slot,
		)
	}
	if featureconfig.Get().OldRemoteSlasherProtection {
		blockHdr, err := blockutil.BeaconBlockHeaderFromBlock(block)
		if err != nil {
			return errors.Wrap(err, "failed to get block header from block")
		}
		if v.oldRemoteSlasher != nil && !v.oldRemoteSlasher.CheckBlockSafety(ctx, blockHdr) {
			if v.emitAccountMetrics {
				ValidatorProposeFailVecSlasher.WithLabelValues(fmtKey).Inc()
			}
			return errors.New(failedBlockSignExternalErr)
		}
		wrap := wrapper.WrappedPhase0SignedBeaconBlock(signedBlock)
		sbh, err := blockutil.SignedBeaconBlockHeaderFromBlock(wrap)
		if err != nil {
			return errors.Wrap(err, "failed to get block header from block")
		}
		if v.oldRemoteSlasher != nil {
			valid, err := v.oldRemoteSlasher.CommitBlock(ctx, sbh)
			if err != nil {
				return err
			}
			if !valid {
				if v.emitAccountMetrics {
					ValidatorProposeFailVecSlasher.WithLabelValues(fmtKey).Inc()
				}
				return fmt.Errorf(failedBlockSignExternalErr)
			}
		}
	}

	if featureconfig.Get().NewRemoteSlasherProtection {
		wrap := wrapper.WrappedPhase0SignedBeaconBlock(signedBlock)
		blockHdr, err := blockutil.SignedBeaconBlockHeaderFromBlock(wrap)
		if err != nil {
			return errors.Wrap(err, "failed to get block header from block")
		}
		slashing, err := v.slashingProtectionClient.IsSlashableBlock(ctx, blockHdr)
		if err != nil {
			return errors.Wrap(err, "could not check if block is slashable")
		}
		if slashing != nil && slashing.ProposerSlashing != nil {
			if v.emitAccountMetrics {
				ValidatorProposeFailVecSlasher.WithLabelValues(fmtKey).Inc()
			}
			return errors.New(failedBlockSignExternalErr)
		}
	}

	if err := v.db.SaveProposalHistoryForSlot(ctx, pubKey, block.Slot, signingRoot[:]); err != nil {
		if v.emitAccountMetrics {
			ValidatorProposeFailVec.WithLabelValues(fmtKey).Inc()
		}
		return errors.Wrap(err, "failed to save updated proposal history")
	}
	return nil
}

func blockLogFields(pubKey [48]byte, blk *ethpb.BeaconBlock, sig []byte) logrus.Fields {
	fields := logrus.Fields{
		"proposerPublicKey": fmt.Sprintf("%#x", pubKey),
		"proposerIndex":     blk.ProposerIndex,
		"blockSlot":         blk.Slot,
	}
	if sig != nil {
		fields["signature"] = fmt.Sprintf("%#x", sig)
	}
	return fields
}
