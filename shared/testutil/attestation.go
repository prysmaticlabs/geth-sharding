package testutil

import (
	"context"
	"errors"
	"fmt"
	"math"

	types "github.com/prysmaticlabs/eth2-types"
	"github.com/prysmaticlabs/go-bitfield"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/state"
	iface "github.com/prysmaticlabs/prysm/beacon-chain/state/interface"
	"github.com/prysmaticlabs/prysm/beacon-chain/state/v1"
	"github.com/prysmaticlabs/prysm/beacon-chain/state/v2"
	ethpb "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/shared/bls"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/rand"
	"github.com/prysmaticlabs/prysm/shared/version"
	log "github.com/sirupsen/logrus"
)

// NewAttestation creates an attestation block with minimum marshalable fields.
func NewAttestation() *ethpb.Attestation {
	return &ethpb.Attestation{
		AggregationBits: bitfield.Bitlist{0b1101},
		Data: &ethpb.AttestationData{
			BeaconBlockRoot: make([]byte, 32),
			Source: &ethpb.Checkpoint{
				Root: make([]byte, 32),
			},
			Target: &ethpb.Checkpoint{
				Root: make([]byte, 32),
			},
		},
		Signature: make([]byte, 96),
	}
}

// GenerateAttestations creates attestations that are entirely valid, for all
// the committees of the current state slot. This function expects attestations
// requested to be cleanly divisible by committees per slot. If there is 1 committee
// in the slot, and numToGen is set to 4, then it will return 4 attestations
// for the same data with their aggregation bits split uniformly.
//
// If you request 4 attestations, but there are 8 committees, you will get 4 fully aggregated attestations.
func GenerateAttestations(
	bState iface.BeaconState, privs []bls.SecretKey, numToGen uint64, slot types.Slot, randomRoot bool,
) ([]*ethpb.Attestation, error) {
	var attestations []*ethpb.Attestation
	generateHeadState := false
	bState = bState.Copy()
	if slot > bState.Slot() {
		// Going back a slot here so there's no inclusion delay issues.
		slot--
		generateHeadState = true
	}
	currentEpoch := helpers.SlotToEpoch(slot)

	targetRoot := make([]byte, 32)
	var headRoot []byte
	var err error
	// Only calculate head state if its an attestation for the current slot or future slot.
	if generateHeadState || slot == bState.Slot() {
		var headState iface.BeaconState
		switch bState.Version() {
		case version.Phase0:
			pbState, err := v1.ProtobufBeaconState(bState.CloneInnerState())
			if err != nil {
				return nil, err
			}
			genState, err := v1.InitializeFromProtoUnsafe(pbState)
			if err != nil {
				return nil, err
			}
			headState = iface.BeaconState(genState)
		case version.Altair:
			pbState, err := v2.ProtobufBeaconState(bState.CloneInnerState())
			if err != nil {
				return nil, err
			}
			genState, err := v2.InitializeFromProtoUnsafe(pbState)
			if err != nil {
				return nil, err
			}
			headState = iface.BeaconState(genState)
		default:
			return nil, errors.New("state type isn't supported")
		}

		headState, err = state.ProcessSlots(context.Background(), headState, slot+1)
		if err != nil {
			return nil, err
		}
		headRoot, err = helpers.BlockRootAtSlot(headState, slot)
		if err != nil {
			return nil, err
		}
		targetRoot, err = helpers.BlockRoot(headState, currentEpoch)
		if err != nil {
			return nil, err
		}
	} else {
		headRoot, err = helpers.BlockRootAtSlot(bState, slot)
		if err != nil {
			return nil, err
		}
	}
	if randomRoot {
		randGen := rand.NewDeterministicGenerator()
		b := make([]byte, 32)
		_, err := randGen.Read(b)
		if err != nil {
			return nil, err
		}
		headRoot = b
	}

	activeValidatorCount, err := helpers.ActiveValidatorCount(bState, currentEpoch)
	if err != nil {
		return nil, err
	}
	committeesPerSlot := helpers.SlotCommitteeCount(activeValidatorCount)

	if numToGen < committeesPerSlot {
		log.Printf(
			"Warning: %d attestations requested is less than %d committees in current slot, not all validators will be attesting.",
			numToGen,
			committeesPerSlot,
		)
	} else if numToGen > committeesPerSlot {
		log.Printf(
			"Warning: %d attestations requested are more than %d committees in current slot, attestations will not be perfectly efficient.",
			numToGen,
			committeesPerSlot,
		)
	}

	attsPerCommittee := math.Max(float64(numToGen/committeesPerSlot), 1)
	if math.Trunc(attsPerCommittee) != attsPerCommittee {
		return nil, fmt.Errorf(
			"requested attestations %d must be easily divisible by committees in slot %d, calculated %f",
			numToGen,
			committeesPerSlot,
			attsPerCommittee,
		)
	}

	domain, err := helpers.Domain(bState.Fork(), currentEpoch, params.BeaconConfig().DomainBeaconAttester, bState.GenesisValidatorRoot())
	if err != nil {
		return nil, err
	}
	for c := types.CommitteeIndex(0); uint64(c) < committeesPerSlot && uint64(c) < numToGen; c++ {
		committee, err := helpers.BeaconCommitteeFromState(bState, slot, c)
		if err != nil {
			return nil, err
		}

		attData := &ethpb.AttestationData{
			Slot:            slot,
			CommitteeIndex:  c,
			BeaconBlockRoot: headRoot,
			Source:          bState.CurrentJustifiedCheckpoint(),
			Target: &ethpb.Checkpoint{
				Epoch: currentEpoch,
				Root:  targetRoot,
			},
		}

		dataRoot, err := helpers.ComputeSigningRoot(attData, domain)
		if err != nil {
			return nil, err
		}

		committeeSize := uint64(len(committee))
		bitsPerAtt := committeeSize / uint64(attsPerCommittee)
		for i := uint64(0); i < committeeSize; i += bitsPerAtt {
			aggregationBits := bitfield.NewBitlist(committeeSize)
			var sigs []bls.Signature
			for b := i; b < i+bitsPerAtt; b++ {
				aggregationBits.SetBitAt(b, true)
				sigs = append(sigs, privs[committee[b]].Sign(dataRoot[:]))
			}

			// bls.AggregateSignatures will return nil if sigs is 0.
			if len(sigs) == 0 {
				continue
			}

			att := &ethpb.Attestation{
				Data:            attData,
				AggregationBits: aggregationBits,
				Signature:       bls.AggregateSignatures(sigs).Marshal(),
			}
			attestations = append(attestations, att)
		}
	}
	return attestations, nil
}

// HydrateAttestation hydrates an attestation object with correct field length sizes
// to comply with fssz marshalling and unmarshalling rules.
func HydrateAttestation(a *ethpb.Attestation) *ethpb.Attestation {
	if a.Signature == nil {
		a.Signature = make([]byte, 96)
	}
	if a.AggregationBits == nil {
		a.AggregationBits = make([]byte, 1)
	}
	if a.Data == nil {
		a.Data = &ethpb.AttestationData{}
	}
	a.Data = HydrateAttestationData(a.Data)
	return a
}

// HydrateAttestationData hydrates an attestation data object with correct field length sizes
// to comply with fssz marshalling and unmarshalling rules.
func HydrateAttestationData(d *ethpb.AttestationData) *ethpb.AttestationData {
	if d.BeaconBlockRoot == nil {
		d.BeaconBlockRoot = make([]byte, 32)
	}
	if d.Target == nil {
		d.Target = &ethpb.Checkpoint{}
	}
	if d.Target.Root == nil {
		d.Target.Root = make([]byte, 32)
	}
	if d.Source == nil {
		d.Source = &ethpb.Checkpoint{}
	}
	if d.Source.Root == nil {
		d.Source.Root = make([]byte, 32)
	}
	return d
}

// HydrateIndexedAttestation hydrates an indexed attestation with correct field length sizes
// to comply with fssz marshalling and unmarshalling rules.
func HydrateIndexedAttestation(a *ethpb.IndexedAttestation) *ethpb.IndexedAttestation {
	if a.Signature == nil {
		a.Signature = make([]byte, 96)
	}
	if a.Data == nil {
		a.Data = &ethpb.AttestationData{}
	}
	a.Data = HydrateAttestationData(a.Data)
	return a
}
