package blocks_test

import (
	"context"
	"fmt"
	"testing"

	types "github.com/prysmaticlabs/eth2-types"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/blocks"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	v "github.com/prysmaticlabs/prysm/beacon-chain/core/validators"
	iface "github.com/prysmaticlabs/prysm/beacon-chain/state/interface"
	v1 "github.com/prysmaticlabs/prysm/beacon-chain/state/v1"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	ethpb "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/shared/bls"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/testutil"
	"github.com/prysmaticlabs/prysm/shared/testutil/assert"
	"github.com/prysmaticlabs/prysm/shared/testutil/require"
)

func TestProcessProposerSlashings_UnmatchedHeaderSlots(t *testing.T) {

	beaconState, _ := testutil.DeterministicGenesisState(t, 20)
	currentSlot := types.Slot(0)
	slashings := []*ethpb.ProposerSlashing{
		{
			Header_1: &ethpb.SignedBeaconBlockHeader{
				Header: &ethpb.BeaconBlockHeader{
					ProposerIndex: 1,
					Slot:          params.BeaconConfig().SlotsPerEpoch + 1,
				},
			},
			Header_2: &ethpb.SignedBeaconBlockHeader{
				Header: &ethpb.BeaconBlockHeader{
					ProposerIndex: 1,
					Slot:          0,
				},
			},
		},
	}
	require.NoError(t, beaconState.SetSlot(currentSlot))

	b := testutil.NewBeaconBlock()
	b.Block = &ethpb.BeaconBlock{
		Body: &ethpb.BeaconBlockBody{
			ProposerSlashings: slashings,
		},
	}
	want := "mismatched header slots"
	_, err := blocks.ProcessProposerSlashings(context.Background(), beaconState, b.Block.Body.ProposerSlashings, v.SlashValidator)
	assert.ErrorContains(t, want, err)
}

func TestProcessProposerSlashings_SameHeaders(t *testing.T) {

	beaconState, _ := testutil.DeterministicGenesisState(t, 2)
	currentSlot := types.Slot(0)
	slashings := []*ethpb.ProposerSlashing{
		{
			Header_1: &ethpb.SignedBeaconBlockHeader{
				Header: &ethpb.BeaconBlockHeader{
					ProposerIndex: 1,
					Slot:          0,
				},
			},
			Header_2: &ethpb.SignedBeaconBlockHeader{
				Header: &ethpb.BeaconBlockHeader{
					ProposerIndex: 1,
					Slot:          0,
				},
			},
		},
	}

	require.NoError(t, beaconState.SetSlot(currentSlot))
	b := testutil.NewBeaconBlock()
	b.Block = &ethpb.BeaconBlock{
		Body: &ethpb.BeaconBlockBody{
			ProposerSlashings: slashings,
		},
	}
	want := "expected slashing headers to differ"
	_, err := blocks.ProcessProposerSlashings(context.Background(), beaconState, b.Block.Body.ProposerSlashings, v.SlashValidator)
	assert.ErrorContains(t, want, err)
}

func TestProcessProposerSlashings_ValidatorNotSlashable(t *testing.T) {
	registry := []*ethpb.Validator{
		{
			PublicKey:         []byte("key"),
			Slashed:           true,
			ActivationEpoch:   0,
			WithdrawableEpoch: 0,
		},
	}
	currentSlot := types.Slot(0)
	slashings := []*ethpb.ProposerSlashing{
		{
			Header_1: &ethpb.SignedBeaconBlockHeader{
				Header: &ethpb.BeaconBlockHeader{
					ProposerIndex: 0,
					Slot:          0,
					BodyRoot:      []byte("foo"),
				},
				Signature: bytesutil.PadTo([]byte("A"), 96),
			},
			Header_2: &ethpb.SignedBeaconBlockHeader{
				Header: &ethpb.BeaconBlockHeader{
					ProposerIndex: 0,
					Slot:          0,
					BodyRoot:      []byte("bar"),
				},
				Signature: bytesutil.PadTo([]byte("B"), 96),
			},
		},
	}

	beaconState, err := v1.InitializeFromProto(&pb.BeaconState{
		Validators: registry,
		Slot:       currentSlot,
	})
	require.NoError(t, err)
	b := testutil.NewBeaconBlock()
	b.Block = &ethpb.BeaconBlock{
		Body: &ethpb.BeaconBlockBody{
			ProposerSlashings: slashings,
		},
	}
	want := fmt.Sprintf(
		"validator with key %#x is not slashable",
		bytesutil.ToBytes48(beaconState.Validators()[0].PublicKey),
	)
	_, err = blocks.ProcessProposerSlashings(context.Background(), beaconState, b.Block.Body.ProposerSlashings, v.SlashValidator)
	assert.ErrorContains(t, want, err)
}

func TestProcessProposerSlashings_AppliesCorrectStatus(t *testing.T) {
	// We test the case when data is correct and verify the validator
	// registry has been updated.
	beaconState, privKeys := testutil.DeterministicGenesisState(t, 100)
	proposerIdx := types.ValidatorIndex(1)

	header1 := &ethpb.SignedBeaconBlockHeader{
		Header: testutil.HydrateBeaconHeader(&ethpb.BeaconBlockHeader{
			ProposerIndex: proposerIdx,
			StateRoot:     bytesutil.PadTo([]byte("A"), 32),
		}),
	}
	var err error
	header1.Signature, err = helpers.ComputeDomainAndSign(beaconState, 0, header1.Header, params.BeaconConfig().DomainBeaconProposer, privKeys[proposerIdx])
	require.NoError(t, err)

	header2 := testutil.HydrateSignedBeaconHeader(&ethpb.SignedBeaconBlockHeader{
		Header: &ethpb.BeaconBlockHeader{
			ProposerIndex: proposerIdx,
			StateRoot:     bytesutil.PadTo([]byte("B"), 32),
		},
	})
	header2.Signature, err = helpers.ComputeDomainAndSign(beaconState, 0, header2.Header, params.BeaconConfig().DomainBeaconProposer, privKeys[proposerIdx])
	require.NoError(t, err)

	slashings := []*ethpb.ProposerSlashing{
		{
			Header_1: header1,
			Header_2: header2,
		},
	}

	block := testutil.NewBeaconBlock()
	block.Block.Body.ProposerSlashings = slashings

	newState, err := blocks.ProcessProposerSlashings(context.Background(), beaconState, block.Block.Body.ProposerSlashings, v.SlashValidator)
	require.NoError(t, err)

	newStateVals := newState.Validators()
	if newStateVals[1].ExitEpoch != beaconState.Validators()[1].ExitEpoch {
		t.Errorf("Proposer with index 1 did not correctly exit,"+"wanted slot:%d, got:%d",
			newStateVals[1].ExitEpoch, beaconState.Validators()[1].ExitEpoch)
	}
}

func TestVerifyProposerSlashing(t *testing.T) {
	type args struct {
		beaconState iface.BeaconState
		slashing    *ethpb.ProposerSlashing
	}

	beaconState, sks := testutil.DeterministicGenesisState(t, 2)
	currentSlot := types.Slot(0)
	require.NoError(t, beaconState.SetSlot(currentSlot))
	rand1, err := bls.RandKey()
	require.NoError(t, err)
	sig1 := rand1.Sign([]byte("foo")).Marshal()

	rand2, err := bls.RandKey()
	require.NoError(t, err)
	sig2 := rand2.Sign([]byte("bar")).Marshal()

	tests := []struct {
		name    string
		args    args
		wantErr string
	}{
		{
			name: "same header, same slot as state",
			args: args{
				slashing: &ethpb.ProposerSlashing{
					Header_1: testutil.HydrateSignedBeaconHeader(&ethpb.SignedBeaconBlockHeader{
						Header: &ethpb.BeaconBlockHeader{
							ProposerIndex: 1,
							Slot:          currentSlot,
						},
					}),
					Header_2: testutil.HydrateSignedBeaconHeader(&ethpb.SignedBeaconBlockHeader{
						Header: &ethpb.BeaconBlockHeader{
							ProposerIndex: 1,
							Slot:          currentSlot,
						},
					}),
				},
				beaconState: beaconState,
			},
			wantErr: "expected slashing headers to differ",
		},
		{ // Regression test for https://github.com/sigp/beacon-fuzz/issues/74
			name: "same header, different signatures",
			args: args{
				slashing: &ethpb.ProposerSlashing{
					Header_1: testutil.HydrateSignedBeaconHeader(&ethpb.SignedBeaconBlockHeader{
						Header: &ethpb.BeaconBlockHeader{
							ProposerIndex: 1,
						},
						Signature: sig1,
					}),
					Header_2: testutil.HydrateSignedBeaconHeader(&ethpb.SignedBeaconBlockHeader{
						Header: &ethpb.BeaconBlockHeader{
							ProposerIndex: 1,
						},
						Signature: sig2,
					}),
				},
				beaconState: beaconState,
			},
			wantErr: "expected slashing headers to differ",
		},
		{
			name: "slashing in future epoch",
			args: args{
				slashing: &ethpb.ProposerSlashing{
					Header_1: &ethpb.SignedBeaconBlockHeader{
						Header: &ethpb.BeaconBlockHeader{
							ProposerIndex: 1,
							Slot:          65,
							StateRoot:     bytesutil.PadTo([]byte{}, 32),
							BodyRoot:      bytesutil.PadTo([]byte{}, 32),
							ParentRoot:    bytesutil.PadTo([]byte("foo"), 32),
						},
					},
					Header_2: &ethpb.SignedBeaconBlockHeader{
						Header: &ethpb.BeaconBlockHeader{
							ProposerIndex: 1,
							Slot:          65,
							StateRoot:     bytesutil.PadTo([]byte{}, 32),
							BodyRoot:      bytesutil.PadTo([]byte{}, 32),
							ParentRoot:    bytesutil.PadTo([]byte("bar"), 32),
						},
					},
				},
				beaconState: beaconState,
			},
			wantErr: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			sk := sks[tt.args.slashing.Header_1.Header.ProposerIndex]
			d, err := helpers.Domain(tt.args.beaconState.Fork(), helpers.SlotToEpoch(tt.args.slashing.Header_1.Header.Slot), params.BeaconConfig().DomainBeaconProposer, tt.args.beaconState.GenesisValidatorRoot())
			require.NoError(t, err)
			if tt.args.slashing.Header_1.Signature == nil {
				sr, err := helpers.ComputeSigningRoot(tt.args.slashing.Header_1.Header, d)
				require.NoError(t, err)
				tt.args.slashing.Header_1.Signature = sk.Sign(sr[:]).Marshal()
			}
			if tt.args.slashing.Header_2.Signature == nil {
				sr, err := helpers.ComputeSigningRoot(tt.args.slashing.Header_2.Header, d)
				require.NoError(t, err)
				tt.args.slashing.Header_2.Signature = sk.Sign(sr[:]).Marshal()
			}
			if err := blocks.VerifyProposerSlashing(tt.args.beaconState, tt.args.slashing); (err != nil || tt.wantErr != "") && err.Error() != tt.wantErr {
				t.Errorf("VerifyProposerSlashing() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
