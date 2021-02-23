package migration

import (
	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
	ethpb "github.com/prysmaticlabs/ethereumapis/eth/v1"
	ethpb_alpha "github.com/prysmaticlabs/ethereumapis/eth/v1alpha1"
)

// V1Alpha1BlockToV1BlockHeader converts a v1alpha1 SignedBeaconBlock proto to a v1 SignedBeaconBlockHeader proto.
func V1Alpha1BlockToV1BlockHeader(block *ethpb_alpha.SignedBeaconBlock) (*ethpb.SignedBeaconBlockHeader, error) {
	bodyRoot, err := block.Block.Body.HashTreeRoot()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get body root of block")
	}
	return &ethpb.SignedBeaconBlockHeader{
		Header: &ethpb.BeaconBlockHeader{
			Slot:          block.Block.Slot,
			ProposerIndex: block.Block.ProposerIndex,
			ParentRoot:    block.Block.ParentRoot,
			StateRoot:     block.Block.StateRoot,
			BodyRoot:      bodyRoot[:],
		},
		Signature: block.Signature,
	}, nil
}

// V1Alpha1BlockToV1Block converts a v1alpha1 SignedBeaconBlock proto to a v1 proto.
func V1Alpha1ToV1Block(alphaBlk *ethpb_alpha.SignedBeaconBlock) (*ethpb.SignedBeaconBlock, error) {
	marshaledBlk, err := alphaBlk.Marshal()
	if err != nil {
		return nil, errors.Wrap(err, "could not marshal block")
	}
	v1Block := &ethpb.SignedBeaconBlock{}
	if err := proto.Unmarshal(marshaledBlk, v1Block); err != nil {
		return nil, errors.Wrap(err, "could not unmarshal block")
	}
	return v1Block, nil
}

// V1ToV1Alpha1Block converts a v1 SignedBeaconBlock proto to a v1alpha1 proto.
func V1ToV1Alpha1Block(alphaBlk *ethpb.SignedBeaconBlock) (*ethpb_alpha.SignedBeaconBlock, error) {
	marshaledBlk, err := alphaBlk.Marshal()
	if err != nil {
		return nil, errors.Wrap(err, "could not marshal block")
	}
	v1alpha1Block := &ethpb_alpha.SignedBeaconBlock{}
	if err := proto.Unmarshal(marshaledBlk, v1alpha1Block); err != nil {
		return nil, errors.Wrap(err, "could not unmarshal block")
	}
	return v1alpha1Block, nil
}

// V1Alpha1IndexedAttToV1 converts a v1alpha1 indexed attestation to v1.
func V1Alpha1IndexedAttToV1(v1alpha1Att *ethpb_alpha.IndexedAttestation) *ethpb.IndexedAttestation {
	if v1alpha1Att == nil {
		return &ethpb.IndexedAttestation{}
	}
	return &ethpb.IndexedAttestation{
		AttestingIndices: v1alpha1Att.AttestingIndices,
		Data:             V1Alpha1AttDataToV1(v1alpha1Att.Data),
		Signature:        v1alpha1Att.Signature,
	}
}

// V1Alpha1AttDataToV1 converts a v1alpha1 attestation data to v1.
func V1Alpha1AttDataToV1(v1alpha1AttData *ethpb_alpha.AttestationData) *ethpb.AttestationData {
	if v1alpha1AttData == nil || v1alpha1AttData.Source == nil || v1alpha1AttData.Target == nil {
		return &ethpb.AttestationData{}
	}
	return &ethpb.AttestationData{
		Slot:            v1alpha1AttData.Slot,
		CommitteeIndex:  v1alpha1AttData.CommitteeIndex,
		BeaconBlockRoot: v1alpha1AttData.BeaconBlockRoot,
		Source: &ethpb.Checkpoint{
			Root:  v1alpha1AttData.Source.Root,
			Epoch: v1alpha1AttData.Source.Epoch,
		},
		Target: &ethpb.Checkpoint{
			Root:  v1alpha1AttData.Target.Root,
			Epoch: v1alpha1AttData.Target.Epoch,
		},
	}
}

// V1Alpha1AttSlashingToV1 converts a v1alpha1 attester slashing to v1.
func V1Alpha1AttSlashingToV1(v1alpha1Slashing *ethpb_alpha.AttesterSlashing) *ethpb.AttesterSlashing {
	if v1alpha1Slashing == nil {
		return &ethpb.AttesterSlashing{}
	}
	return &ethpb.AttesterSlashing{
		Attestation_1: V1Alpha1IndexedAttToV1(v1alpha1Slashing.Attestation_1),
		Attestation_2: V1Alpha1IndexedAttToV1(v1alpha1Slashing.Attestation_2),
	}
}
