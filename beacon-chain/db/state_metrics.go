package db

import (
	"encoding/hex"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	"github.com/prysmaticlabs/prysm/shared/params"
)

var (
	validatorBalancesGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "state_validator_balances",
		Help: "Balances of validators, updated on epoch transition",
	}, []string{
		"validator",
	})
	validatorActivatedGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "state_validator_activated_epoch",
		Help: "Activated epoch of validators, updated on epoch transition",
	}, []string{
		"validatorIndex",
	})
	validatorExitedGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "state_validator_exited_epoch",
		Help: "Exited epoch of validators, updated on epoch transition",
	}, []string{
		"validatorIndex",
	})
	validatorSlashedGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "state_validator_slashed_epoch",
		Help: "Slashed epoch of validators, updated on epoch transition",
	}, []string{
		"validatorIndex",
	})
	lastSlotGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "state_last_slot",
		Help: "Last slot number of the processed state",
	})
	lastJustifiedEpochGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "state_last_justified_epoch",
		Help: "Last justified epoch of the processed state",
	})
	lastPrevJustifiedEpochGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "state_last_prev_justified_epoch",
		Help: "Last prev justified epoch of the processed state",
	})
	lastFinalizedEpochGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "state_last_finalized_epoch",
		Help: "Last finalized epoch of the processed state",
	})
	activeValidatorsGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "state_active_validators",
		Help: "Total number of active validators",
	})
)

func reportStateMetrics(state *pb.BeaconState) {
	currentEpoch := state.Slot / params.BeaconConfig().SlotsPerEpoch
	// Validator balances
	for i, bal := range state.Balances {
		validatorBalancesGauge.WithLabelValues(
			"0x" + hex.EncodeToString(state.Validators[i].Pubkey), // Validator
		).Set(float64(bal))
	}

	var active float64
	for i, v := range state.Validators {
		// Track individual Validator's activation epochs
		validatorActivatedGauge.WithLabelValues(
			strconv.Itoa(i), //Validator index
		).Set(float64(v.ActivationEpoch))
		// Track individual Validator's exited epochs
		validatorExitedGauge.WithLabelValues(
			strconv.Itoa(i), //Validator index
		).Set(float64(v.ExitEpoch))
		// Track individual Validator's slashed epochs
		if v.Slashed {
			validatorSlashedGauge.WithLabelValues(
				strconv.Itoa(i), //Validator index
			).Set(float64(v.WithdrawableEpoch - params.BeaconConfig().EpochsPerSlashingsVector))
		} else {
			validatorSlashedGauge.WithLabelValues(
				strconv.Itoa(i), //Validator index
			).Set(float64(params.BeaconConfig().FarFutureEpoch))
		}
		// Total number of active validators
		if v.ActivationEpoch <= currentEpoch && currentEpoch < v.ExitEpoch {
			active++
		}
	}
	activeValidatorsGauge.Set(active)

	// Slot number
	lastSlotGauge.Set(float64(state.Slot))

	// Last justified slot
	if state.CurrentJustifiedCheckpoint != nil {
		lastJustifiedEpochGauge.Set(float64(state.CurrentJustifiedCheckpoint.Epoch))
	}
	// Last previous justified slot
	if state.PreviousJustifiedCheckpoint != nil {
		lastPrevJustifiedEpochGauge.Set(float64(state.PreviousJustifiedCheckpoint.Epoch))
	}
	// Last finalized slot
	if state.FinalizedCheckpoint != nil {
		lastFinalizedEpochGauge.Set(float64(state.FinalizedCheckpoint.Epoch))
	}
}
