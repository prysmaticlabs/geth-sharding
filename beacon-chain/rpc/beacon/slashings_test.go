package beacon

import (
	"context"
	"strconv"
	"testing"

	"github.com/gogo/protobuf/proto"
	ethpb "github.com/prysmaticlabs/ethereumapis/eth/v1alpha1"
	pbp2p "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"

	mock "github.com/prysmaticlabs/prysm/beacon-chain/blockchain/testing"
	"github.com/prysmaticlabs/prysm/beacon-chain/operations/slashings"
	stateTrie "github.com/prysmaticlabs/prysm/beacon-chain/state"
	"github.com/prysmaticlabs/prysm/shared/params"
)

func TestServer_SubmitProposerSlashing(t *testing.T) {
	ctx := context.Background()
	vals := make([]*ethpb.Validator, 10)
	for i := 0; i < len(vals); i++ {
		key := make([]byte, 48)
		copy(key, strconv.Itoa(i))
		vals[i] = &ethpb.Validator{
			PublicKey:             key[:],
			WithdrawalCredentials: make([]byte, 32),
			EffectiveBalance:      params.BeaconConfig().MaxEffectiveBalance,
			Slashed:               false,
		}
	}

	// We mark the validator at index 5 as already slashed.
	vals[5].Slashed = true

	st, err := stateTrie.InitializeFromProto(&pbp2p.BeaconState{
		Slot:       0,
		Validators: vals,
	})
	if err != nil {
		t.Fatal(err)
	}
	bs := &Server{
		HeadFetcher: &mock.ChainService{
			State: st,
		},
		SlashingsPool: slashings.NewPool(),
	}

	// We want a proposer slashing for validator with index 2 to
	// be included in the pool.
	wanted := &ethpb.SubmitSlashingResponse{
		SlashedIndices: []uint64{2},
	}
	slashing := &ethpb.ProposerSlashing{
		ProposerIndex: 2,
		Header_1: &ethpb.SignedBeaconBlockHeader{
			Header: &ethpb.BeaconBlockHeader{
				Slot:       0,
				ParentRoot: nil,
				StateRoot:  nil,
				BodyRoot:   nil,
			},
			Signature: make([]byte, 96),
		},
		Header_2: &ethpb.SignedBeaconBlockHeader{
			Header: &ethpb.BeaconBlockHeader{
				Slot:       0,
				ParentRoot: nil,
				StateRoot:  nil,
				BodyRoot:   nil,
			},
			Signature: make([]byte, 96),
		},
	}
	res, err := bs.SubmitProposerSlashing(ctx, slashing)
	if err != nil {
		t.Fatal(err)
	}
	if !proto.Equal(wanted, res) {
		t.Errorf("Wanted %v, received %v", wanted, res)
	}

	// We do not want a proposer slashing for an already slashed validator
	// (the validator at index 5) to be included in the pool.
	slashing.ProposerIndex = 5
	if _, err := bs.SubmitProposerSlashing(ctx, slashing); err == nil {
		t.Error("Expected including a proposer slashing for an already slashed validator to fail")
	}
}

func TestServer_SubmitAttesterSlashing(t *testing.T) {
	ctx := context.Background()
	bs := &Server{
		SlashingsPool: slashings.NewPool(),
	}
	wanted := &ethpb.SubmitSlashingResponse{
		SlashedIndices: []uint64{0, 1, 2},
	}
	res, err := bs.SubmitAttesterSlashing(ctx, &ethpb.AttesterSlashing{})
	if err != nil {
		t.Fatal(err)
	}
	if !proto.Equal(wanted, res) {
		t.Errorf("Wanted %v, received %v", wanted, res)
	}
}
