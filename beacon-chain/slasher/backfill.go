package slasher

import (
	"context"
	"time"

	"github.com/pkg/errors"
	types "github.com/prysmaticlabs/eth2-types"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	"github.com/prysmaticlabs/prysm/beacon-chain/db/filters"
	slashertypes "github.com/prysmaticlabs/prysm/beacon-chain/slasher/types"
	iface "github.com/prysmaticlabs/prysm/beacon-chain/state/interface"
	"github.com/prysmaticlabs/prysm/proto/interfaces"
	"github.com/prysmaticlabs/prysm/shared/attestationutil"
	"github.com/prysmaticlabs/prysm/shared/blockutil"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/prysmaticlabs/prysm/shared/slotutil"
)

// Backfills data for slasher if necessary after initial sync, and blocks
// the main slasher thread until the backfill procedure is complete.
func (s *Service) waitForDataBackfill(wssPeriod types.Epoch) {
	// The lowest epoch we need to backfill for slasher is based on the
	// head epoch minus the weak subjectivity period.
	headSlot := s.serviceCfg.HeadStateFetcher.HeadSlot()
	headEpoch := helpers.SlotToEpoch(headSlot)
	lowestEpoch := headEpoch
	if lowestEpoch <= wssPeriod {
		lowestEpoch = 0
	} else {
		lowestEpoch = lowestEpoch - wssPeriod
	}

	log.Infof("Beginning slasher data backfill from epoch %d to %d", lowestEpoch, headEpoch)
	start := time.Now()
	if err := s.backfill(lowestEpoch, headEpoch); err != nil {
		log.Error(err)
		return
	}
	log.Infof("Finished backfilling range with time elapsed %v", time.Since(start))
	lowestEpoch = headEpoch

	for {
		// If we have no difference between the max epoch we have detected for
		// slasher and the current epoch on the clock, then we can exit the loop.
		currentEpoch := slotutil.EpochsSinceGenesis(s.genesisTime)
		diff := currentEpoch
		if diff >= lowestEpoch {
			diff = diff - lowestEpoch
		}
		if diff == 0 {
			break
		}

		// We set the max epoch for slasher to the current epoch on the clock for backfilling.
		maxEpoch := currentEpoch

		log.Infof("Beginning slasher data backfill from epoch %d to %d", lowestEpoch, maxEpoch)
		start := time.Now()
		if err := s.backfill(lowestEpoch, maxEpoch); err != nil {
			log.Error(err)
			return
		}
		log.Infof("Finished backfilling range with time elapsed %v", time.Since(start))

		// After backfilling, we set the lowest epoch for backfilling to be the
		// max epoch we have completed backfill to.
		lowestEpoch = maxEpoch
	}
}

// Backfill slasher's historical database from blocks in a range of epochs.
// The max range between start and end is approximately 4096 epochs,
// so we perform backfilling in chunks of a set size to reduce impact
// on disk reads and writes during the procedure.
func (s *Service) backfill(start, end types.Epoch) error {
	f := filters.NewFilter().SetStartEpoch(start).SetEndEpoch(end)
	blocks, roots, err := s.serviceCfg.BeaconDatabase.Blocks(s.ctx, f)
	if err != nil {
		return err
	}
	headers := make([]*slashertypes.SignedBlockHeaderWrapper, 0, len(blocks))
	atts := make([]*slashertypes.IndexedAttestationWrapper, 0)
	for i, block := range blocks {
		header, err := blockutil.SignedBeaconBlockHeaderFromBlock(block)
		if err != nil {
			return err
		}
		headers = append(headers, &slashertypes.SignedBlockHeaderWrapper{
			SignedBeaconBlockHeader: header,
			SigningRoot:             roots[i],
		})
		preState, err := s.getBlockPreState(s.ctx, block.Block())
		if err != nil {
			return err
		}
		for _, att := range block.Block().Body().Attestations() {
			committee, err := helpers.BeaconCommitteeFromState(preState, att.Data.Slot, att.Data.CommitteeIndex)
			if err != nil {
				return err
			}
			indexedAtt, err := attestationutil.ConvertToIndexed(s.ctx, att, committee)
			if err != nil {
				return err
			}
			signingRoot, err := indexedAtt.Data.HashTreeRoot()
			if err != nil {
				return err
			}
			atts = append(atts, &slashertypes.IndexedAttestationWrapper{
				IndexedAttestation: indexedAtt,
				SigningRoot:        signingRoot,
			})
		}
	}
	log.Debugf("Running slashing detection on %d blocks", len(headers))
	propSlashings, err := s.detectProposerSlashings(s.ctx, headers)
	if err != nil {
		return err
	}
	log.Debugf("Found %d proposer slashing(s)", len(propSlashings))
	if err := s.processProposerSlashings(s.ctx, propSlashings); err != nil {
		return err
	}
	log.Debugf("Running slashing detection on %d attestation(s)", len(atts))
	attSlashings, err := s.checkSlashableAttestations(s.ctx, atts)
	if err != nil {
		return err
	}
	log.Debugf("Found %d attester slashing(s)", len(attSlashings))
	return s.processAttesterSlashings(s.ctx, attSlashings)
}

func (s *Service) getBlockPreState(ctx context.Context, b interfaces.BeaconBlock) (iface.BeaconState, error) {
	preState, err := s.serviceCfg.StateGen.StateByRoot(ctx, bytesutil.ToBytes32(b.ParentRoot()))
	if err != nil {
		return nil, errors.Wrapf(err, "could not get pre state for slot %d", b.Slot())
	}
	if preState == nil || preState.IsNil() {
		return nil, errors.Wrapf(err, "nil pre state for slot %d", b.Slot())
	}
	return preState, nil
}
