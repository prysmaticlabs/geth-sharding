package client

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	ethpb "github.com/prysmaticlabs/ethereumapis/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/roughtime"
	"github.com/prysmaticlabs/prysm/shared/slotutil"
	"go.opencensus.io/trace"
)

var (
	validatorAggSuccessVec = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "validator",
			Name:      "successful_aggregations",
		},
		[]string{
			// validator pubkey
			"pubkey",
		},
	)
	validatorAggFailVec = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "validator",
			Name:      "failed_aggregations",
		},
		[]string{
			// validator pubkey
			"pubkey",
		},
	)
)

// SubmitAggregateAndProof submits the validator's signed slot signature to the beacon node
// via gRPC. Beacon node will verify the slot signature and determine if the validator is also
// an aggregator. If yes, then beacon node will broadcast aggregated signature and
// proof on the validator's behalf.
func (v *validator) SubmitAggregateAndProof(ctx context.Context, slot uint64, pubKey [48]byte) {
	ctx, span := trace.StartSpan(ctx, "validator.SubmitAggregateAndProof")
	defer span.End()

	span.AddAttributes(trace.StringAttribute("validator", fmt.Sprintf("%#x", pubKey)))
	fmtKey := fmt.Sprintf("%#x", pubKey[:])

	duty, err := v.duty(pubKey)
	if err != nil {
		log.Errorf("Could not fetch validator assignment: %v", err)
		if v.emitAccountMetrics {
			validatorAggFailVec.WithLabelValues(fmtKey).Inc()
		}
		return
	}

	// Avoid sending beacon node duplicated aggregation requests.
	k := validatorSubscribeKey(slot, duty.CommitteeIndex)
	v.aggregatedSlotCommitteeIDCacheLock.Lock()
	defer v.aggregatedSlotCommitteeIDCacheLock.Unlock()
	if v.aggregatedSlotCommitteeIDCache.Contains(k) {
		return
	}
	v.aggregatedSlotCommitteeIDCache.Add(k, true)

	slotSig, err := v.signSlot(ctx, pubKey, slot)
	if err != nil {
		log.Errorf("Could not sign slot: %v", err)
		if v.emitAccountMetrics {
			validatorAggFailVec.WithLabelValues(fmtKey).Inc()
		}
		return
	}

	// As specified in spec, an aggregator should wait until two thirds of the way through slot
	// to broadcast the best aggregate to the global aggregate channel.
	// https://github.com/ethereum/eth2.0-specs/blob/v0.9.3/specs/validator/0_beacon-chain-validator.md#broadcast-aggregate
	v.waitToSlotTwoThirds(ctx, slot)

	res, err := v.validatorClient.SubmitAggregateSelectionProof(ctx, &ethpb.AggregateSelectionRequest{
		Slot:           slot,
		CommitteeIndex: duty.CommitteeIndex,
		PublicKey:      pubKey[:],
		SlotSignature:  slotSig,
	})
	if err != nil {
		log.Errorf("Could not submit slot signature to beacon node: %v", err)
		if v.emitAccountMetrics {
			validatorAggFailVec.WithLabelValues(fmtKey).Inc()
		}
		return
	}

	d, err := v.domainData(ctx, helpers.SlotToEpoch(res.AggregateAndProof.Aggregate.Data.Slot), params.BeaconConfig().DomainAggregateAndProof[:])
	if err != nil {
		log.Errorf("Could not get domain data to sign aggregate and proof: %v", err)
		return
	}
	signedRoot, err := helpers.ComputeSigningRoot(res.AggregateAndProof, d.SignatureDomain)
	if err != nil {
		log.Errorf("Could not compute sign root for aggregate and proof: %v", err)
		return
	}

	_, err = v.validatorClient.SubmitSignedAggregateSelectionProof(ctx, &ethpb.SignedAggregateSubmitRequest{
		SignedAggregateAndProof: &ethpb.SignedAggregateAttestationAndProof{
			Message:   res.AggregateAndProof,
			Signature: signedRoot[:],
		},
	})
	if err != nil {
		log.Errorf("Could not submit signed aggregate and proof to beacon node: %v", err)
		if v.emitAccountMetrics {
			validatorAggFailVec.WithLabelValues(fmtKey).Inc()
		}
		return
	}

	if err := v.addIndicesToLog(duty); err != nil {
		log.Errorf("Could not add aggregator indices to logs: %v", err)
		if v.emitAccountMetrics {
			validatorAggFailVec.WithLabelValues(fmtKey).Inc()
		}
		return
	}
	if v.emitAccountMetrics {
		validatorAggSuccessVec.WithLabelValues(fmtKey).Inc()
	}

}

// This implements selection logic outlined in:
// https://github.com/ethereum/eth2.0-specs/blob/v0.9.3/specs/validator/0_beacon-chain-validator.md#aggregation-selection
func (v *validator) signSlot(ctx context.Context, pubKey [48]byte, slot uint64) ([]byte, error) {
	domain, err := v.domainData(ctx, helpers.SlotToEpoch(slot), params.BeaconConfig().DomainSelectionProof[:])
	if err != nil {
		return nil, err
	}

	sig, err := v.signObject(pubKey, slot, domain.SignatureDomain)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to sign slot")
	}

	return sig.Marshal(), nil
}

// waitToSlotTwoThirds waits until two third through the current slot period
// such that any attestations from this slot have time to reach the beacon node
// before creating the aggregated attestation.
func (v *validator) waitToSlotTwoThirds(ctx context.Context, slot uint64) {
	_, span := trace.StartSpan(ctx, "validator.waitToSlotTwoThirds")
	defer span.End()

	twoThird := params.BeaconConfig().SecondsPerSlot * 2 / 3
	delay := time.Duration(twoThird) * time.Second

	startTime := slotutil.SlotStartTime(v.genesisTime, slot)
	finalTime := startTime.Add(delay)
	time.Sleep(roughtime.Until(finalTime))
}

func (v *validator) addIndicesToLog(duty *ethpb.DutiesResponse_Duty) error {
	v.attLogsLock.Lock()
	defer v.attLogsLock.Unlock()

	for _, log := range v.attLogs {
		if duty.CommitteeIndex == log.data.CommitteeIndex {
			log.aggregatorIndices = append(log.aggregatorIndices, duty.ValidatorIndex)
		}
	}

	return nil
}
