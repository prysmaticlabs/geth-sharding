package client

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"github.com/prysmaticlabs/prysm/proto/interfaces"
	"github.com/prysmaticlabs/prysm/shared/blockutil"
	"github.com/prysmaticlabs/prysm/shared/featureconfig"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/sirupsen/logrus"
)

var failedPreBlockSignLocalErr = "attempted to sign a double proposal, block rejected by local protection"
var failedPreBlockSignExternalErr = "attempted a double proposal, block rejected by remote slashing protection"
var failedPostBlockSignErr = "made a double proposal, considered slashable by remote slashing protection"

func (v *validator) preBlockSignValidations(
	ctx context.Context, pubKey [48]byte, block interfaces.BeaconBlock, signingRoot [32]byte,
) error {
	fmtKey := fmt.Sprintf("%#x", pubKey[:])

	prevSigningRoot, proposalAtSlotExists, err := v.db.ProposalHistoryForSlot(ctx, pubKey, block.Slot())
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
		return errors.New(failedPreBlockSignLocalErr)
	}

	// Based on EIP3076, validator should refuse to sign any proposal with slot less
	// than or equal to the minimum signed proposal present in the DB for that public key.
	// In the case the slot of the incoming block is equal to the minimum signed proposal, we
	// then also check the signing root is different.
	if lowestProposalExists && signingRootIsDifferent && lowestSignedProposalSlot >= block.Slot() {
		return fmt.Errorf(
			"could not sign block with slot <= lowest signed slot in db, lowest signed slot: %d >= block slot: %d",
			lowestSignedProposalSlot,
			block.Slot(),
		)
	}

	if featureconfig.Get().SlasherProtection && v.protector != nil {
		blockHdr, err := blockutil.BeaconBlockHeaderFromBlockInterface(block)
		if err != nil {
			return errors.Wrap(err, "failed to get block header from block")
		}
		if !v.protector.CheckBlockSafety(ctx, blockHdr) {
			if v.emitAccountMetrics {
				ValidatorProposeFailVecSlasher.WithLabelValues(fmtKey).Inc()
			}
			return errors.New(failedPreBlockSignExternalErr)
		}
	}

	return nil
}

func (v *validator) postBlockSignUpdate(
	ctx context.Context,
	pubKey [48]byte,
	block interfaces.SignedBeaconBlock,
	signingRoot [32]byte,
) error {
	fmtKey := fmt.Sprintf("%#x", pubKey[:])
	if featureconfig.Get().SlasherProtection && v.protector != nil {
		sbh, err := blockutil.SignedBeaconBlockHeaderFromBlockInterface(block)
		if err != nil {
			return errors.Wrap(err, "failed to get block header from block")
		}
		valid, err := v.protector.CommitBlock(ctx, sbh)
		if err != nil {
			return err
		}
		if !valid {
			if v.emitAccountMetrics {
				ValidatorProposeFailVecSlasher.WithLabelValues(fmtKey).Inc()
			}
			return fmt.Errorf(failedPostBlockSignErr)
		}
	}
	if err := v.db.SaveProposalHistoryForSlot(ctx, pubKey, block.Block().Slot(), signingRoot[:]); err != nil {
		if v.emitAccountMetrics {
			ValidatorProposeFailVec.WithLabelValues(fmtKey).Inc()
		}
		return errors.Wrap(err, "failed to save updated proposal history")
	}
	return nil
}

func blockLogFields(pubKey [48]byte, blk interfaces.BeaconBlock, sig []byte) logrus.Fields {
	fields := logrus.Fields{
		"proposerPublicKey": fmt.Sprintf("%#x", pubKey),
		"proposerIndex":     blk.ProposerIndex(),
		"blockSlot":         blk.Slot(),
	}
	if sig != nil {
		fields["signature"] = fmt.Sprintf("%#x", sig)
	}
	return fields
}
