package simulator

import (
	"context"
	"fmt"

	types "github.com/prysmaticlabs/eth2-types"
	ethpb "github.com/prysmaticlabs/ethereumapis/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	"github.com/prysmaticlabs/prysm/beacon-chain/state"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	"github.com/prysmaticlabs/prysm/shared/bls"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/rand"
	log "github.com/sirupsen/logrus"
)

func (s *Simulator) generateBlockHeadersForSlot(
	ctx context.Context, slot types.Slot,
) ([]*ethpb.SignedBeaconBlockHeader, []*ethpb.ProposerSlashing, error) {
	blocks := make([]*ethpb.SignedBeaconBlockHeader, 0)
	slashings := make([]*ethpb.ProposerSlashing, 0)
	proposer := rand.NewGenerator().Uint64() % s.srvConfig.Params.NumValidators

	parentRoot := [32]byte{}
	beaconState, err := s.srvConfig.StateGen.StateByRoot(ctx, parentRoot)
	if err != nil {
		return nil, nil, err
	}
	fmt.Println(beaconState)
	block := &ethpb.SignedBeaconBlockHeader{
		Header: &ethpb.BeaconBlockHeader{
			Slot:          slot,
			ProposerIndex: types.ValidatorIndex(proposer),
			ParentRoot:    bytesutil.PadTo([]byte{}, 32),
			StateRoot:     bytesutil.PadTo([]byte{}, 32),
			BodyRoot:      bytesutil.PadTo([]byte("good block"), 32),
		},
	}
	sig, err := s.signBlockHeader(beaconState, block)
	if err != nil {
		return nil, nil, err
	}
	block.Signature = sig.Marshal()

	blocks = append(blocks, block)
	if rand.NewGenerator().Float64() < s.srvConfig.Params.ProposerSlashingProbab {
		log.WithField("proposerIndex", proposer).Infof("Slashable block made")
		slashableBlock := &ethpb.SignedBeaconBlockHeader{
			Header: &ethpb.BeaconBlockHeader{
				Slot:          slot,
				ProposerIndex: types.ValidatorIndex(proposer),
				ParentRoot:    bytesutil.PadTo([]byte{}, 32),
				StateRoot:     bytesutil.PadTo([]byte{}, 32),
				BodyRoot:      bytesutil.PadTo([]byte("bad block"), 32),
			},
			Signature: sig.Marshal(),
		}
		blocks = append(blocks, slashableBlock)
		slashings = append(slashings, &ethpb.ProposerSlashing{
			Header_1: block,
			Header_2: slashableBlock,
		})
	}
	return blocks, slashings, nil
}

func (s *Simulator) signBlockHeader(
	beaconState *state.BeaconState,
	header *ethpb.SignedBeaconBlockHeader,
) (bls.Signature, error) {
	log.Warn(beaconState.Fork())
	log.Warn(beaconState.GenesisValidatorRoot())
	domain, err := helpers.Domain(
		beaconState.Fork(),
		0,
		params.BeaconConfig().DomainBeaconProposer,
		beaconState.GenesisValidatorRoot(),
	)
	if err != nil {
		return nil, err
	}
	htr, err := header.Header.HashTreeRoot()
	if err != nil {
		return nil, err
	}
	container := &pb.SigningData{
		ObjectRoot: htr[:],
		Domain:     domain,
	}
	signingRoot, err := container.HashTreeRoot()
	if err != nil {
		return nil, err
	}
	validatorPrivKey := s.srvConfig.PrivateKeysByValidatorIndex[header.Header.ProposerIndex]
	return validatorPrivKey.Sign(signingRoot[:]), nil
}
