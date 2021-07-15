package blockutil

import (
	"github.com/pkg/errors"
	ethpb "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/proto/interfaces"
)

// SignedBeaconBlockHeaderFromBlock function to retrieve signed block header from block.
func SignedBeaconBlockHeaderFromBlock(block *ethpb.SignedBeaconBlock) (*ethpb.SignedBeaconBlockHeader, error) {
	if block.Block == nil || block.Block.Body == nil {
		return nil, errors.New("nil block")
	}

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

// SignedBeaconBlockHeaderFromBlockInterface function to retrieve signed block header from block.
func SignedBeaconBlockHeaderFromBlockInterface(block interfaces.SignedBeaconBlock) (*ethpb.SignedBeaconBlockHeader, error) {
	if block.Block().IsNil() || block.Block().Body().IsNil() {
		return nil, errors.New("nil block")
	}

	bodyRoot, err := block.Block().Body().HashTreeRoot()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get body root of block")
	}
	return &ethpb.SignedBeaconBlockHeader{
		Header: &ethpb.BeaconBlockHeader{
			Slot:          block.Block().Slot(),
			ProposerIndex: block.Block().ProposerIndex(),
			ParentRoot:    block.Block().ParentRoot(),
			StateRoot:     block.Block().StateRoot(),
			BodyRoot:      bodyRoot[:],
		},
		Signature: block.Signature(),
	}, nil
}

// BeaconBlockHeaderFromBlock function to retrieve block header from block.f
func BeaconBlockHeaderFromBlock(block *ethpb.BeaconBlock) (*ethpb.BeaconBlockHeader, error) {
	if block.Body == nil {
		return nil, errors.New("nil block body")
	}

	bodyRoot, err := block.Body.HashTreeRoot()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get body root of block")
	}
	return &ethpb.BeaconBlockHeader{
		Slot:          block.Slot,
		ProposerIndex: block.ProposerIndex,
		ParentRoot:    block.ParentRoot,
		StateRoot:     block.StateRoot,
		BodyRoot:      bodyRoot[:],
	}, nil
}

// BeaconBlockHeaderFromBlockInterface function to retrieve block header from block.
func BeaconBlockHeaderFromBlockInterface(block interfaces.BeaconBlock) (*ethpb.BeaconBlockHeader, error) {
	if block.Body().IsNil() {
		return nil, errors.New("nil block body")
	}

	bodyRoot, err := block.Body().HashTreeRoot()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get body root of block")
	}
	return &ethpb.BeaconBlockHeader{
		Slot:          block.Slot(),
		ProposerIndex: block.ProposerIndex(),
		ParentRoot:    block.ParentRoot(),
		StateRoot:     block.StateRoot(),
		BodyRoot:      bodyRoot[:],
	}, nil
}
