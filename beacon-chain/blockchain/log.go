package blockchain

import (
	"encoding/hex"
	"fmt"
	"time"

	ethpb "github.com/prysmaticlabs/ethereumapis/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/timeutils"
	"github.com/sirupsen/logrus"
)

var log = logrus.WithField("prefix", "blockchain")

// logs state transition related data every slot.
func logStateTransitionData(b *ethpb.BeaconBlock) {
	log := log.WithField("slot", b.Slot)
	if len(b.Body.Attestations) > 0 {
		log = log.WithField("attestations", len(b.Body.Attestations))
	}
	if len(b.Body.Deposits) > 0 {
		log = log.WithField("deposits", len(b.Body.Deposits))
	}
	if len(b.Body.AttesterSlashings) > 0 {
		log = log.WithField("attesterSlashings", len(b.Body.AttesterSlashings))
	}
	if len(b.Body.ProposerSlashings) > 0 {
		log = log.WithField("proposerSlashings", len(b.Body.ProposerSlashings))
	}
	if len(b.Body.VoluntaryExits) > 0 {
		log = log.WithField("voluntaryExits", len(b.Body.VoluntaryExits))
	}
	log.Info("Finished applying state transition")
}

func logBlockSyncStatus(block *ethpb.BeaconBlock, blockRoot [32]byte, finalized *ethpb.Checkpoint, receivedTime time.Time, genesisTime uint64) error {
	startTime, err := helpers.SlotToTime(genesisTime, block.Slot)
	if err != nil {
		return err
	}
	log.WithFields(logrus.Fields{
		"slot":                      block.Slot,
		"slotInEpoch":               block.Slot % params.BeaconConfig().SlotsPerEpoch,
		"block":                     fmt.Sprintf("0x%s...", hex.EncodeToString(blockRoot[:])[:8]),
		"epoch":                     helpers.SlotToEpoch(block.Slot),
		"finalizedEpoch":            finalized.Epoch,
		"finalizedRoot":             fmt.Sprintf("0x%s...", hex.EncodeToString(finalized.Root)[:8]),
	}).Info("Synced new block")
	log.WithFields(logrus.Fields{
		"slot":                      block.Slot,
		"sinceSlotStartTime":        timeutils.Now().Sub(startTime),
		"chainServiceProcessedTime": timeutils.Now().Sub(receivedTime),
	}).Debug("Timings for synced block")
	return nil
}
