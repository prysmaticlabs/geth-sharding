package client

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	types "github.com/prysmaticlabs/eth2-types"
	"github.com/prysmaticlabs/go-bitfield"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	ethpb "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/proto/interfaces"
	validatorpb "github.com/prysmaticlabs/prysm/proto/validator/accounts/v2"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/prysmaticlabs/prysm/shared/featureconfig"
	"github.com/prysmaticlabs/prysm/shared/hashutil"
	"github.com/prysmaticlabs/prysm/shared/mputil"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/slotutil"
	"github.com/prysmaticlabs/prysm/shared/timeutils"
	"github.com/prysmaticlabs/prysm/shared/traceutil"
	"github.com/prysmaticlabs/prysm/validator/client/iface"
	"github.com/sirupsen/logrus"
	"go.opencensus.io/trace"
)

// SubmitAttestation completes the validator client's attester responsibility at a given slot.
// It fetches the latest beacon block head along with the latest canonical beacon state
// information in order to sign the block and include information about the validator's
// participation in voting on the block.
func (v *validator) SubmitAttestation(ctx context.Context, slot types.Slot, pubKey [48]byte) {
	ctx, span := trace.StartSpan(ctx, "validator.SubmitAttestation")
	defer span.End()
	span.AddAttributes(trace.StringAttribute("validator", fmt.Sprintf("%#x", pubKey)))

	v.waitOneThirdOrValidBlock(ctx, slot)

	var b strings.Builder
	if err := b.WriteByte(byte(iface.RoleAttester)); err != nil {
		log.WithError(err).Error("Could not write role byte for lock key")
		traceutil.AnnotateError(span, err)
		return
	}
	_, err := b.Write(pubKey[:])
	if err != nil {
		log.WithError(err).Error("Could not write pubkey bytes for lock key")
		traceutil.AnnotateError(span, err)
		return
	}
	lock := mputil.NewMultilock(b.String())
	lock.Lock()
	defer lock.Unlock()

	fmtKey := fmt.Sprintf("%#x", pubKey[:])
	log := log.WithField("pubKey", fmt.Sprintf("%#x", bytesutil.Trunc(pubKey[:]))).WithField("slot", slot)
	duty, err := v.duty(pubKey)
	if err != nil {
		log.WithError(err).Error("Could not fetch validator assignment")
		if v.emitAccountMetrics {
			ValidatorAttestFailVec.WithLabelValues(fmtKey).Inc()
		}
		traceutil.AnnotateError(span, err)
		return
	}
	if len(duty.Committee) == 0 {
		log.Debug("Empty committee for validator duty, not attesting")
		return
	}

	req := &ethpb.AttestationDataRequest{
		Slot:           slot,
		CommitteeIndex: duty.CommitteeIndex,
	}
	data, err := v.validatorClient.GetAttestationData(ctx, req)
	if err != nil {
		log.WithError(err).Error("Could not request attestation to sign at slot")
		if v.emitAccountMetrics {
			ValidatorAttestFailVec.WithLabelValues(fmtKey).Inc()
		}
		traceutil.AnnotateError(span, err)
		return
	}

	indexedAtt := &ethpb.IndexedAttestation{
		AttestingIndices: []uint64{uint64(duty.ValidatorIndex)},
		Data:             data,
	}

	_, signingRoot, err := v.getDomainAndSigningRoot(ctx, indexedAtt.Data)
	if err != nil {
		log.WithError(err).Error("Could not get domain and signing root from attestation")
		if v.emitAccountMetrics {
			ValidatorAttestFailVec.WithLabelValues(fmtKey).Inc()
		}
		traceutil.AnnotateError(span, err)
		return
	}

	sig, _, err := v.signAtt(ctx, pubKey, data)
	if err != nil {
		log.WithError(err).Error("Could not sign attestation")
		if v.emitAccountMetrics {
			ValidatorAttestFailVec.WithLabelValues(fmtKey).Inc()
		}
		traceutil.AnnotateError(span, err)
		return
	}

	var indexInCommittee uint64
	var found bool
	for i, vID := range duty.Committee {
		if vID == duty.ValidatorIndex {
			indexInCommittee = uint64(i)
			found = true
			break
		}
	}
	if !found {
		log.Errorf("Validator ID %d not found in committee of %v", duty.ValidatorIndex, duty.Committee)
		if v.emitAccountMetrics {
			ValidatorAttestFailVec.WithLabelValues(fmtKey).Inc()
		}
		return
	}

	aggregationBitfield := bitfield.NewBitlist(uint64(len(duty.Committee)))
	aggregationBitfield.SetBitAt(indexInCommittee, true)
	attestation := &ethpb.Attestation{
		Data:            data,
		AggregationBits: aggregationBitfield,
		Signature:       sig,
	}

	// Set the signature of the attestation and send it out to the beacon node.
	indexedAtt.Signature = sig
	if err := v.slashableAttestationCheck(ctx, indexedAtt, pubKey, signingRoot); err != nil {
		log.WithError(err).Error("Failed attestation slashing protection check")
		log.WithFields(
			attestationLogFields(pubKey, indexedAtt),
		).Debug("Attempted slashable attestation details")
		traceutil.AnnotateError(span, err)
		return
	}
	attResp, err := v.validatorClient.ProposeAttestation(ctx, attestation)
	if err != nil {
		log.WithError(err).Error("Could not submit attestation to beacon node")
		if v.emitAccountMetrics {
			ValidatorAttestFailVec.WithLabelValues(fmtKey).Inc()
		}
		traceutil.AnnotateError(span, err)
		return
	}

	if err := v.saveAttesterIndexToData(data, duty.ValidatorIndex); err != nil {
		log.WithError(err).Error("Could not save validator index for logging")
		if v.emitAccountMetrics {
			ValidatorAttestFailVec.WithLabelValues(fmtKey).Inc()
		}
		traceutil.AnnotateError(span, err)
		return
	}

	span.AddAttributes(
		trace.Int64Attribute("slot", int64(slot)),
		trace.StringAttribute("attestationHash", fmt.Sprintf("%#x", attResp.AttestationDataRoot)),
		trace.Int64Attribute("committeeIndex", int64(data.CommitteeIndex)),
		trace.StringAttribute("blockRoot", fmt.Sprintf("%#x", data.BeaconBlockRoot)),
		trace.Int64Attribute("justifiedEpoch", int64(data.Source.Epoch)),
		trace.Int64Attribute("targetEpoch", int64(data.Target.Epoch)),
		trace.StringAttribute("bitfield", fmt.Sprintf("%#x", aggregationBitfield)),
	)

	if v.emitAccountMetrics {
		ValidatorAttestSuccessVec.WithLabelValues(fmtKey).Inc()
		ValidatorAttestedSlotsGaugeVec.WithLabelValues(fmtKey).Set(float64(slot))
	}
}

// Given the validator public key, this gets the validator assignment.
func (v *validator) duty(pubKey [48]byte) (*ethpb.DutiesResponse_Duty, error) {
	if v.duties == nil {
		return nil, errors.New("no duties for validators")
	}

	for _, duty := range v.duties.Duties {
		if bytes.Equal(pubKey[:], duty.PublicKey) {
			return duty, nil
		}
	}

	return nil, fmt.Errorf("pubkey %#x not in duties", bytesutil.Trunc(pubKey[:]))
}

// Given validator's public key, this function returns the signature of an attestation data and its signing root.
func (v *validator) signAtt(ctx context.Context, pubKey [48]byte, data *ethpb.AttestationData) ([]byte, [32]byte, error) {
	domain, root, err := v.getDomainAndSigningRoot(ctx, data)
	if err != nil {
		return nil, [32]byte{}, err
	}

	sig, err := v.keyManager.Sign(ctx, &validatorpb.SignRequest{
		PublicKey:       pubKey[:],
		SigningRoot:     root[:],
		SignatureDomain: domain.SignatureDomain,
		Object:          &validatorpb.SignRequest_AttestationData{AttestationData: data},
	})
	if err != nil {
		return nil, [32]byte{}, err
	}

	return sig.Marshal(), root, nil
}

func (v *validator) getDomainAndSigningRoot(ctx context.Context, data *ethpb.AttestationData) (*ethpb.DomainResponse, [32]byte, error) {
	domain, err := v.domainData(ctx, data.Target.Epoch, params.BeaconConfig().DomainBeaconAttester[:])
	if err != nil {
		return nil, [32]byte{}, err
	}
	root, err := helpers.ComputeSigningRoot(data, domain.SignatureDomain)
	if err != nil {
		return nil, [32]byte{}, err
	}
	return domain, root, nil
}

// For logging, this saves the last submitted attester index to its attestation data. The purpose of this
// is to enhance attesting logs to be readable when multiple validator keys ran in a single client.
func (v *validator) saveAttesterIndexToData(data *ethpb.AttestationData, index types.ValidatorIndex) error {
	v.attLogsLock.Lock()
	defer v.attLogsLock.Unlock()

	h, err := hashutil.HashProto(data)
	if err != nil {
		return err
	}

	if v.attLogs[h] == nil {
		v.attLogs[h] = &attSubmitted{data, []types.ValidatorIndex{}, []types.ValidatorIndex{}}
	}
	v.attLogs[h] = &attSubmitted{data, append(v.attLogs[h].attesterIndices, index), []types.ValidatorIndex{}}

	return nil
}

// waitOneThirdOrValidBlock waits until (a) or (b) whichever comes first:
//   (a) the validator has received a valid block that is the same slot as input slot
//   (b) one-third of the slot has transpired (SECONDS_PER_SLOT / 3 seconds after the start of slot)
func (v *validator) waitOneThirdOrValidBlock(ctx context.Context, slot types.Slot) {
	ctx, span := trace.StartSpan(ctx, "validator.waitOneThirdOrValidBlock")
	defer span.End()

	// Don't need to wait if requested slot is the same as highest valid slot.
	if slot <= v.highestValidSlot {
		return
	}

	delay := slotutil.DivideSlotBy(3 /* a third of the slot duration */)
	startTime := slotutil.SlotStartTime(v.genesisTime, slot)
	finalTime := startTime.Add(delay)
	wait := timeutils.Until(finalTime)
	if wait <= 0 {
		return
	}
	t := time.NewTimer(wait)
	defer t.Stop()

	bChannel := make(chan interfaces.SignedBeaconBlock, 1)
	sub := v.blockFeed.Subscribe(bChannel)
	defer sub.Unsubscribe()

	for {
		select {
		case b := <-bChannel:
			if featureconfig.Get().AttestTimely {
				if slot <= b.Block().Slot() {
					return
				}
			}
		case <-ctx.Done():
			traceutil.AnnotateError(span, ctx.Err())
			return
		case <-sub.Err():
			log.Error("Subscriber closed, exiting goroutine")
			return
		case <-t.C:
			return
		}
	}
}

func attestationLogFields(pubKey [48]byte, indexedAtt *ethpb.IndexedAttestation) logrus.Fields {
	return logrus.Fields{
		"attesterPublicKey": fmt.Sprintf("%#x", pubKey),
		"attestationSlot":   indexedAtt.Data.Slot,
		"committeeIndex":    indexedAtt.Data.CommitteeIndex,
		"beaconBlockRoot":   fmt.Sprintf("%#x", indexedAtt.Data.BeaconBlockRoot),
		"sourceEpoch":       indexedAtt.Data.Source.Epoch,
		"sourceRoot":        fmt.Sprintf("%#x", indexedAtt.Data.Source.Root),
		"targetEpoch":       indexedAtt.Data.Target.Epoch,
		"targetRoot":        fmt.Sprintf("%#x", indexedAtt.Data.Target.Root),
		"signature":         fmt.Sprintf("%#x", indexedAtt.Signature),
	}
}
