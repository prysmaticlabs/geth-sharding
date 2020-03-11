package slashings

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/gogo/protobuf/proto"
	ethpb "github.com/prysmaticlabs/ethereumapis/eth/v1alpha1"
	beaconstate "github.com/prysmaticlabs/prysm/beacon-chain/state"
	p2ppb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/testutil"
)

func proposerSlashingForValIdx(valIdx uint64) *ethpb.ProposerSlashing {
	return &ethpb.ProposerSlashing{
		ProposerIndex: valIdx,
	}
}

func generateNProposerSlashings(n uint64) []*ethpb.ProposerSlashing {
	proposerSlashings := make([]*ethpb.ProposerSlashing, n)
	for i := uint64(0); i < n; i++ {
		proposerSlashings[i] = proposerSlashingForValIdx(i)
	}
	return proposerSlashings
}

func TestPool_InsertProposerSlashing(t *testing.T) {
	type fields struct {
		wantErr  bool
		err      string
		pending  []*ethpb.ProposerSlashing
		included map[uint64]bool
	}
	type args struct {
		slashing *ethpb.ProposerSlashing
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   []*ethpb.ProposerSlashing
	}{
		{
			name: "Empty list",
			fields: fields{
				pending:  make([]*ethpb.ProposerSlashing, 0),
				included: make(map[uint64]bool),
			},
			args: args{
				slashing: proposerSlashingForValIdx(0),
			},
			want: generateNProposerSlashings(1),
		},
		{
			name: "Duplicate identical slashing",
			fields: fields{
				pending:  generateNProposerSlashings(1),
				included: make(map[uint64]bool),
				wantErr:  true,
				err:      "slashing object already exists in pending proposer slashings",
			},
			args: args{
				slashing: proposerSlashingForValIdx(0),
			},
			want: generateNProposerSlashings(1),
		},
		{
			name: "Slashing for exited validator",
			fields: fields{
				pending:  []*ethpb.ProposerSlashing{},
				included: make(map[uint64]bool),
				wantErr:  true,
				err:      "cannot be slashed",
			},
			args: args{
				slashing: proposerSlashingForValIdx(2),
			},
			want: []*ethpb.ProposerSlashing{},
		},
		{
			name: "Slashing for future exited validator",
			fields: fields{
				pending:  []*ethpb.ProposerSlashing{},
				included: make(map[uint64]bool),
			},
			args: args{
				slashing: proposerSlashingForValIdx(4),
			},
			want: []*ethpb.ProposerSlashing{
				proposerSlashingForValIdx(4),
			},
		},
		{
			name: "Slashing for slashed validator",
			fields: fields{
				pending:  []*ethpb.ProposerSlashing{},
				included: make(map[uint64]bool),
				wantErr:  true,
				err:      "cannot be slashed",
			},
			args: args{
				slashing: proposerSlashingForValIdx(5),
			},
			want: []*ethpb.ProposerSlashing{},
		},
		{
			name: "Already included",
			fields: fields{
				pending: []*ethpb.ProposerSlashing{},
				included: map[uint64]bool{
					1: true,
				},
				wantErr: true,
				err:     "cannot be slashed",
			},
			args: args{
				slashing: proposerSlashingForValIdx(1),
			},
			want: []*ethpb.ProposerSlashing{},
		},
		{
			name: "Maintains sorted order",
			fields: fields{
				pending: []*ethpb.ProposerSlashing{
					proposerSlashingForValIdx(0),
					proposerSlashingForValIdx(4),
				},
				included: make(map[uint64]bool),
			},
			args: args{
				slashing: proposerSlashingForValIdx(1),
			},
			want: []*ethpb.ProposerSlashing{
				proposerSlashingForValIdx(0),
				proposerSlashingForValIdx(1),
				proposerSlashingForValIdx(4),
			},
		},
	}
	validators := []*ethpb.Validator{
		{ // 0
			ExitEpoch: params.BeaconConfig().FarFutureEpoch,
		},
		{ // 1
			ExitEpoch: params.BeaconConfig().FarFutureEpoch,
		},
		{ // 2 - Already exited.
			ExitEpoch: 15,
		},
		{ // 3
			ExitEpoch: params.BeaconConfig().FarFutureEpoch,
		},
		{ // 4 - Will be exited.
			ExitEpoch: 17,
		},
		{ // 5 - Slashed.
			ExitEpoch: params.BeaconConfig().FarFutureEpoch,
			Slashed:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Pool{
				pendingProposerSlashing: tt.fields.pending,
				included:                tt.fields.included,
			}
			beaconState, err := beaconstate.InitializeFromProtoUnsafe(&p2ppb.BeaconState{
				Slot:       16 * params.BeaconConfig().SlotsPerEpoch,
				Validators: validators,
			})
			if err != nil {
				t.Fatal(err)
			}
			err = p.InsertProposerSlashing(beaconState, tt.args.slashing)
			if err != nil && tt.fields.wantErr && !strings.Contains(err.Error(), tt.fields.err) {
				t.Fatalf("Wanted err: %v, received %v", tt.fields.err, err)
			}
			if !tt.fields.wantErr && err != nil {
				t.Fatalf("Did not expect error: %v", err)
			}
			if len(p.pendingProposerSlashing) != len(tt.want) {
				t.Fatalf("Mismatched lengths of pending list. Got %d, wanted %d.", len(p.pendingProposerSlashing), len(tt.want))
			}
			for i := range p.pendingAttesterSlashing {
				if p.pendingProposerSlashing[i].ProposerIndex != tt.want[i].ProposerIndex {
					t.Errorf(
						"Pending proposer to slash at index %d does not match expected. Got=%v wanted=%v",
						i,
						p.pendingProposerSlashing[i].ProposerIndex,
						tt.want[i].ProposerIndex,
					)
				}
				if !proto.Equal(p.pendingProposerSlashing[i], tt.want[i]) {
					t.Errorf("Proposer slashing at index %d does not match expected. Got=%v wanted=%v", i, p.pendingProposerSlashing[i], tt.want[i])
				}
			}
		})
	}
}

func TestPool_MarkIncludedProposerSlashing(t *testing.T) {
	type fields struct {
		pending  []*ethpb.ProposerSlashing
		included map[uint64]bool
	}
	type args struct {
		slashing *ethpb.ProposerSlashing
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   fields
	}{
		{
			name: "Included, does not exist in pending",
			fields: fields{
				pending: []*ethpb.ProposerSlashing{
					proposerSlashingForValIdx(1),
				},
				included: make(map[uint64]bool),
			},
			args: args{
				slashing: proposerSlashingForValIdx(3),
			},
			want: fields{
				pending: []*ethpb.ProposerSlashing{
					proposerSlashingForValIdx(1),
				},
				included: map[uint64]bool{
					3: true,
				},
			},
		},
		{
			name: "Removes from pending list",
			fields: fields{
				pending: []*ethpb.ProposerSlashing{
					proposerSlashingForValIdx(1),
					proposerSlashingForValIdx(2),
					proposerSlashingForValIdx(3),
				},
				included: map[uint64]bool{
					0: true,
				},
			},
			args: args{
				slashing: proposerSlashingForValIdx(2),
			},
			want: fields{
				pending: []*ethpb.ProposerSlashing{
					proposerSlashingForValIdx(1),
					proposerSlashingForValIdx(3),
				},
				included: map[uint64]bool{
					0: true,
					2: true,
				},
			},
		},
		{
			name: "Removes from pending long list",
			fields: fields{
				pending: []*ethpb.ProposerSlashing{
					proposerSlashingForValIdx(1),
					proposerSlashingForValIdx(2),
					proposerSlashingForValIdx(3),
					proposerSlashingForValIdx(4),
					proposerSlashingForValIdx(5),
					proposerSlashingForValIdx(6),
					proposerSlashingForValIdx(7),
					proposerSlashingForValIdx(8),
					proposerSlashingForValIdx(9),
					proposerSlashingForValIdx(10),
				},
				included: map[uint64]bool{
					0: true,
				},
			},
			args: args{
				slashing: proposerSlashingForValIdx(7),
			},
			want: fields{
				pending: []*ethpb.ProposerSlashing{
					proposerSlashingForValIdx(1),
					proposerSlashingForValIdx(2),
					proposerSlashingForValIdx(3),
					proposerSlashingForValIdx(4),
					proposerSlashingForValIdx(5),
					proposerSlashingForValIdx(6),
					proposerSlashingForValIdx(8),
					proposerSlashingForValIdx(9),
					proposerSlashingForValIdx(10),
				},
				included: map[uint64]bool{
					0: true,
					7: true,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Pool{
				pendingProposerSlashing: tt.fields.pending,
				included:                tt.fields.included,
			}
			p.MarkIncludedProposerSlashing(tt.args.slashing)
			if len(p.pendingProposerSlashing) != len(tt.want.pending) {
				t.Fatalf(
					"Mismatched lengths of pending list. Got %d, wanted %d.",
					len(p.pendingProposerSlashing),
					len(tt.want.pending),
				)
			}
			for i := range p.pendingProposerSlashing {
				if !proto.Equal(p.pendingProposerSlashing[i], tt.want.pending[i]) {
					t.Errorf(
						"Pending proposer slashing at index %d does not match expected. Got=%v wanted=%v",
						i,
						p.pendingProposerSlashing[i],
						tt.want.pending[i],
					)
				}
			}
			if !reflect.DeepEqual(p.included, tt.want.included) {
				t.Errorf("Included map is not as expected. Got=%v wanted=%v", p.included, tt.want.included)
			}
		})
	}
}

func TestPool_PendingProposerSlashings(t *testing.T) {
	type fields struct {
		pending []*ethpb.ProposerSlashing
	}
	beaconState, privKeys := testutil.DeterministicGenesisState(t, 64)
	slashings := make([]*ethpb.ProposerSlashing, 20)
	for i := 0; i < len(slashings); i++ {
		sl, err := testutil.GenerateProposerSlashingForValidator(beaconState, privKeys[i], uint64(i))
		if err != nil {
			t.Fatal(err)
		}
		slashings[i] = sl
	}
	tests := []struct {
		name   string
		fields fields
		want   []*ethpb.ProposerSlashing
	}{
		{
			name: "Empty list",
			fields: fields{
				pending: []*ethpb.ProposerSlashing{},
			},
			want: []*ethpb.ProposerSlashing{},
		},
		{
			name: "All eligible",
			fields: fields{
				pending: slashings[:params.BeaconConfig().MaxProposerSlashings],
			},
			want: slashings[:params.BeaconConfig().MaxProposerSlashings],
		},
		{
			name: "Multiple indices",
			fields: fields{
				pending: slashings[3:6],
			},
			want: slashings[3:6],
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Pool{
				pendingProposerSlashing: tt.fields.pending,
			}
			if got := p.PendingProposerSlashings(
				context.Background(),
				beaconState,
			); !reflect.DeepEqual(tt.want, got) {
				t.Errorf("Unexpected return from PendingProposerSlashings, wanted %v, received %v", tt.want, got)
			}
		})
	}
}

func TestPool_PendingProposerSlashings_SigFailsVerify_ClearPool(t *testing.T) {
	conf := params.BeaconConfig()
	conf.MaxAttesterSlashings = 2
	params.OverrideBeaconConfig(conf)
	beaconState, privKeys := testutil.DeterministicGenesisState(t, 64)
	slashings := make([]*ethpb.ProposerSlashing, 2)
	for i := 0; i < 2; i++ {
		sl, err := testutil.GenerateProposerSlashingForValidator(beaconState, privKeys[i], uint64(i))
		if err != nil {
			t.Fatal(err)
		}
		slashings[i] = sl
	}
	// We mess up the signature of the second slashing.
	badSig := make([]byte, 96)
	copy(badSig, "muahaha")
	slashings[1].Header_1.Signature = badSig
	p := &Pool{
		pendingProposerSlashing: slashings,
	}
	// We only want a single slashing to remain.
	want := slashings[0:1]
	if got := p.PendingProposerSlashings(
		context.Background(),
		beaconState,
	); !reflect.DeepEqual(want, got) {
		t.Errorf("Unexpected return from PendingProposerSlashings, wanted %v, received %v", want, got)
	}
	// We expect to only have 1 pending proposer slashing in the pool.
	if len(p.pendingProposerSlashing) != 1 {
		t.Error("Expected failed proposer slashing to have been cleared from pool")
	}
}
