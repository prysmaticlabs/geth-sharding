package voluntaryexits

import (
	"context"
	"reflect"
	"testing"

	types "github.com/prysmaticlabs/eth2-types"
	v1 "github.com/prysmaticlabs/prysm/beacon-chain/state/v1"
	p2ppb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	ethpb "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/testutil/require"
	"google.golang.org/protobuf/proto"
)

func TestPool_InsertVoluntaryExit(t *testing.T) {
	type fields struct {
		pending []*ethpb.SignedVoluntaryExit
	}
	type args struct {
		exit *ethpb.SignedVoluntaryExit
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   []*ethpb.SignedVoluntaryExit
	}{
		{
			name: "Prevent inserting nil exit",
			fields: fields{
				pending: make([]*ethpb.SignedVoluntaryExit, 0),
			},
			args: args{
				exit: nil,
			},
			want: []*ethpb.SignedVoluntaryExit{},
		},
		{
			name: "Prevent inserting malformed exit",
			fields: fields{
				pending: make([]*ethpb.SignedVoluntaryExit, 0),
			},
			args: args{
				exit: &ethpb.SignedVoluntaryExit{
					Exit: nil,
				},
			},
			want: []*ethpb.SignedVoluntaryExit{},
		},
		{
			name: "Empty list",
			fields: fields{
				pending: make([]*ethpb.SignedVoluntaryExit, 0),
			},
			args: args{
				exit: &ethpb.SignedVoluntaryExit{
					Exit: &ethpb.VoluntaryExit{
						Epoch:          12,
						ValidatorIndex: 1,
					},
				},
			},
			want: []*ethpb.SignedVoluntaryExit{
				{
					Exit: &ethpb.VoluntaryExit{
						Epoch:          12,
						ValidatorIndex: 1,
					},
				},
			},
		},
		{
			name: "Duplicate identical exit",
			fields: fields{
				pending: []*ethpb.SignedVoluntaryExit{
					{
						Exit: &ethpb.VoluntaryExit{
							Epoch:          12,
							ValidatorIndex: 1,
						},
					},
				},
			},
			args: args{
				exit: &ethpb.SignedVoluntaryExit{
					Exit: &ethpb.VoluntaryExit{
						Epoch:          12,
						ValidatorIndex: 1,
					},
				},
			},
			want: []*ethpb.SignedVoluntaryExit{
				{
					Exit: &ethpb.VoluntaryExit{
						Epoch:          12,
						ValidatorIndex: 1,
					},
				},
			},
		},
		{
			name: "Duplicate exit in pending list",
			fields: fields{
				pending: []*ethpb.SignedVoluntaryExit{
					{
						Exit: &ethpb.VoluntaryExit{
							Epoch:          12,
							ValidatorIndex: 1,
						},
					},
				},
			},
			args: args{
				exit: &ethpb.SignedVoluntaryExit{
					Exit: &ethpb.VoluntaryExit{
						Epoch:          12,
						ValidatorIndex: 1,
					},
				},
			},
			want: []*ethpb.SignedVoluntaryExit{
				{
					Exit: &ethpb.VoluntaryExit{
						Epoch:          12,
						ValidatorIndex: 1,
					},
				},
			},
		},
		{
			name: "Duplicate validator index",
			fields: fields{
				pending: []*ethpb.SignedVoluntaryExit{
					{
						Exit: &ethpb.VoluntaryExit{
							Epoch:          12,
							ValidatorIndex: 1,
						},
					},
				},
			},
			args: args{
				exit: &ethpb.SignedVoluntaryExit{
					Exit: &ethpb.VoluntaryExit{
						Epoch:          20,
						ValidatorIndex: 1,
					},
				},
			},
			want: []*ethpb.SignedVoluntaryExit{
				{
					Exit: &ethpb.VoluntaryExit{
						Epoch:          12,
						ValidatorIndex: 1,
					},
				},
			},
		},
		{
			name: "Duplicate received with more favorable exit epoch",
			fields: fields{
				pending: []*ethpb.SignedVoluntaryExit{
					{
						Exit: &ethpb.VoluntaryExit{
							Epoch:          12,
							ValidatorIndex: 1,
						},
					},
				},
			},
			args: args{
				exit: &ethpb.SignedVoluntaryExit{
					Exit: &ethpb.VoluntaryExit{
						Epoch:          4,
						ValidatorIndex: 1,
					},
				},
			},
			want: []*ethpb.SignedVoluntaryExit{
				{
					Exit: &ethpb.VoluntaryExit{
						Epoch:          4,
						ValidatorIndex: 1,
					},
				},
			},
		},
		{
			name: "Exit for already exited validator",
			fields: fields{
				pending: []*ethpb.SignedVoluntaryExit{},
			},
			args: args{
				exit: &ethpb.SignedVoluntaryExit{
					Exit: &ethpb.VoluntaryExit{
						Epoch:          12,
						ValidatorIndex: 2,
					},
				},
			},
			want: []*ethpb.SignedVoluntaryExit{},
		},
		{
			name: "Maintains sorted order",
			fields: fields{
				pending: []*ethpb.SignedVoluntaryExit{
					{
						Exit: &ethpb.VoluntaryExit{
							Epoch:          12,
							ValidatorIndex: 0,
						},
					},
					{
						Exit: &ethpb.VoluntaryExit{
							Epoch:          12,
							ValidatorIndex: 2,
						},
					},
				},
			},
			args: args{
				exit: &ethpb.SignedVoluntaryExit{
					Exit: &ethpb.VoluntaryExit{
						Epoch:          10,
						ValidatorIndex: 1,
					},
				},
			},
			want: []*ethpb.SignedVoluntaryExit{
				{
					Exit: &ethpb.VoluntaryExit{
						Epoch:          12,
						ValidatorIndex: 0,
					},
				},
				{
					Exit: &ethpb.VoluntaryExit{
						Epoch:          10,
						ValidatorIndex: 1,
					},
				},
				{
					Exit: &ethpb.VoluntaryExit{
						Epoch:          12,
						ValidatorIndex: 2,
					},
				},
			},
		},
	}
	ctx := context.Background()
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Pool{
				pending: tt.fields.pending,
			}
			s, err := v1.InitializeFromProtoUnsafe(&p2ppb.BeaconState{Validators: validators})
			require.NoError(t, err)
			p.InsertVoluntaryExit(ctx, s, tt.args.exit)
			if len(p.pending) != len(tt.want) {
				t.Fatalf("Mismatched lengths of pending list. Got %d, wanted %d.", len(p.pending), len(tt.want))
			}
			for i := range p.pending {
				if !proto.Equal(p.pending[i], tt.want[i]) {
					t.Errorf("Pending exit at index %d does not match expected. Got=%v wanted=%v", i, p.pending[i], tt.want[i])
				}
			}
		})
	}
}

func TestPool_MarkIncluded(t *testing.T) {
	type fields struct {
		pending []*ethpb.SignedVoluntaryExit
	}
	type args struct {
		exit *ethpb.SignedVoluntaryExit
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   fields
	}{
		{
			name: "Removes from pending list",
			fields: fields{
				pending: []*ethpb.SignedVoluntaryExit{
					{
						Exit: &ethpb.VoluntaryExit{ValidatorIndex: 1},
					},
					{
						Exit: &ethpb.VoluntaryExit{ValidatorIndex: 2},
					},
					{
						Exit: &ethpb.VoluntaryExit{ValidatorIndex: 3},
					},
				},
			},
			args: args{
				exit: &ethpb.SignedVoluntaryExit{
					Exit: &ethpb.VoluntaryExit{ValidatorIndex: 2},
				},
			},
			want: fields{
				pending: []*ethpb.SignedVoluntaryExit{
					{
						Exit: &ethpb.VoluntaryExit{ValidatorIndex: 1},
					},
					{
						Exit: &ethpb.VoluntaryExit{ValidatorIndex: 3},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Pool{
				pending: tt.fields.pending,
			}
			p.MarkIncluded(tt.args.exit)
			if len(p.pending) != len(tt.want.pending) {
				t.Fatalf("Mismatched lengths of pending list. Got %d, wanted %d.", len(p.pending), len(tt.want.pending))
			}
			for i := range p.pending {
				if !proto.Equal(p.pending[i], tt.want.pending[i]) {
					t.Errorf("Pending exit at index %d does not match expected. Got=%v wanted=%v", i, p.pending[i], tt.want.pending[i])
				}
			}
		})
	}
}

func TestPool_PendingExits(t *testing.T) {
	type fields struct {
		pending []*ethpb.SignedVoluntaryExit
		noLimit bool
	}
	type args struct {
		slot types.Slot
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   []*ethpb.SignedVoluntaryExit
	}{
		{
			name: "Empty list",
			fields: fields{
				pending: []*ethpb.SignedVoluntaryExit{},
			},
			args: args{
				slot: 100000,
			},
			want: []*ethpb.SignedVoluntaryExit{},
		},
		{
			name: "All eligible",
			fields: fields{
				pending: []*ethpb.SignedVoluntaryExit{
					{Exit: &ethpb.VoluntaryExit{Epoch: 0}},
					{Exit: &ethpb.VoluntaryExit{Epoch: 1}},
					{Exit: &ethpb.VoluntaryExit{Epoch: 2}},
					{Exit: &ethpb.VoluntaryExit{Epoch: 3}},
					{Exit: &ethpb.VoluntaryExit{Epoch: 4}},
				},
			},
			args: args{
				slot: 1000000,
			},
			want: []*ethpb.SignedVoluntaryExit{
				{Exit: &ethpb.VoluntaryExit{Epoch: 0}},
				{Exit: &ethpb.VoluntaryExit{Epoch: 1}},
				{Exit: &ethpb.VoluntaryExit{Epoch: 2}},
				{Exit: &ethpb.VoluntaryExit{Epoch: 3}},
				{Exit: &ethpb.VoluntaryExit{Epoch: 4}},
			},
		},
		{
			name: "All eligible, above max",
			fields: fields{
				noLimit: true,
				pending: []*ethpb.SignedVoluntaryExit{
					{Exit: &ethpb.VoluntaryExit{Epoch: 0}},
					{Exit: &ethpb.VoluntaryExit{Epoch: 1}},
					{Exit: &ethpb.VoluntaryExit{Epoch: 2}},
					{Exit: &ethpb.VoluntaryExit{Epoch: 3}},
					{Exit: &ethpb.VoluntaryExit{Epoch: 4}},
					{Exit: &ethpb.VoluntaryExit{Epoch: 5}},
					{Exit: &ethpb.VoluntaryExit{Epoch: 6}},
					{Exit: &ethpb.VoluntaryExit{Epoch: 7}},
					{Exit: &ethpb.VoluntaryExit{Epoch: 8}},
					{Exit: &ethpb.VoluntaryExit{Epoch: 9}},
					{Exit: &ethpb.VoluntaryExit{Epoch: 10}},
					{Exit: &ethpb.VoluntaryExit{Epoch: 11}},
					{Exit: &ethpb.VoluntaryExit{Epoch: 12}},
					{Exit: &ethpb.VoluntaryExit{Epoch: 13}},
					{Exit: &ethpb.VoluntaryExit{Epoch: 14}},
					{Exit: &ethpb.VoluntaryExit{Epoch: 15}},
					{Exit: &ethpb.VoluntaryExit{Epoch: 16}},
					{Exit: &ethpb.VoluntaryExit{Epoch: 17}},
					{Exit: &ethpb.VoluntaryExit{Epoch: 18}},
					{Exit: &ethpb.VoluntaryExit{Epoch: 19}},
				},
			},
			args: args{
				slot: 1000000,
			},
			want: []*ethpb.SignedVoluntaryExit{
				{Exit: &ethpb.VoluntaryExit{Epoch: 0}},
				{Exit: &ethpb.VoluntaryExit{Epoch: 1}},
				{Exit: &ethpb.VoluntaryExit{Epoch: 2}},
				{Exit: &ethpb.VoluntaryExit{Epoch: 3}},
				{Exit: &ethpb.VoluntaryExit{Epoch: 4}},
				{Exit: &ethpb.VoluntaryExit{Epoch: 5}},
				{Exit: &ethpb.VoluntaryExit{Epoch: 6}},
				{Exit: &ethpb.VoluntaryExit{Epoch: 7}},
				{Exit: &ethpb.VoluntaryExit{Epoch: 8}},
				{Exit: &ethpb.VoluntaryExit{Epoch: 9}},
				{Exit: &ethpb.VoluntaryExit{Epoch: 10}},
				{Exit: &ethpb.VoluntaryExit{Epoch: 11}},
				{Exit: &ethpb.VoluntaryExit{Epoch: 12}},
				{Exit: &ethpb.VoluntaryExit{Epoch: 13}},
				{Exit: &ethpb.VoluntaryExit{Epoch: 14}},
				{Exit: &ethpb.VoluntaryExit{Epoch: 15}},
				{Exit: &ethpb.VoluntaryExit{Epoch: 16}},
				{Exit: &ethpb.VoluntaryExit{Epoch: 17}},
				{Exit: &ethpb.VoluntaryExit{Epoch: 18}},
				{Exit: &ethpb.VoluntaryExit{Epoch: 19}},
			},
		},
		{
			name: "All eligible, block max",
			fields: fields{
				pending: []*ethpb.SignedVoluntaryExit{
					{Exit: &ethpb.VoluntaryExit{Epoch: 0}},
					{Exit: &ethpb.VoluntaryExit{Epoch: 1}},
					{Exit: &ethpb.VoluntaryExit{Epoch: 2}},
					{Exit: &ethpb.VoluntaryExit{Epoch: 3}},
					{Exit: &ethpb.VoluntaryExit{Epoch: 4}},
					{Exit: &ethpb.VoluntaryExit{Epoch: 5}},
					{Exit: &ethpb.VoluntaryExit{Epoch: 6}},
					{Exit: &ethpb.VoluntaryExit{Epoch: 7}},
					{Exit: &ethpb.VoluntaryExit{Epoch: 8}},
					{Exit: &ethpb.VoluntaryExit{Epoch: 9}},
					{Exit: &ethpb.VoluntaryExit{Epoch: 10}},
					{Exit: &ethpb.VoluntaryExit{Epoch: 11}},
					{Exit: &ethpb.VoluntaryExit{Epoch: 12}},
					{Exit: &ethpb.VoluntaryExit{Epoch: 13}},
					{Exit: &ethpb.VoluntaryExit{Epoch: 14}},
					{Exit: &ethpb.VoluntaryExit{Epoch: 15}},
					{Exit: &ethpb.VoluntaryExit{Epoch: 16}},
					{Exit: &ethpb.VoluntaryExit{Epoch: 17}},
					{Exit: &ethpb.VoluntaryExit{Epoch: 18}},
					{Exit: &ethpb.VoluntaryExit{Epoch: 19}},
				},
			},
			args: args{
				slot: 1000000,
			},
			want: []*ethpb.SignedVoluntaryExit{
				{Exit: &ethpb.VoluntaryExit{Epoch: 0}},
				{Exit: &ethpb.VoluntaryExit{Epoch: 1}},
				{Exit: &ethpb.VoluntaryExit{Epoch: 2}},
				{Exit: &ethpb.VoluntaryExit{Epoch: 3}},
				{Exit: &ethpb.VoluntaryExit{Epoch: 4}},
				{Exit: &ethpb.VoluntaryExit{Epoch: 5}},
				{Exit: &ethpb.VoluntaryExit{Epoch: 6}},
				{Exit: &ethpb.VoluntaryExit{Epoch: 7}},
				{Exit: &ethpb.VoluntaryExit{Epoch: 8}},
				{Exit: &ethpb.VoluntaryExit{Epoch: 9}},
				{Exit: &ethpb.VoluntaryExit{Epoch: 10}},
				{Exit: &ethpb.VoluntaryExit{Epoch: 11}},
				{Exit: &ethpb.VoluntaryExit{Epoch: 12}},
				{Exit: &ethpb.VoluntaryExit{Epoch: 13}},
				{Exit: &ethpb.VoluntaryExit{Epoch: 14}},
				{Exit: &ethpb.VoluntaryExit{Epoch: 15}},
			},
		},
		{
			name: "Some eligible",
			fields: fields{
				pending: []*ethpb.SignedVoluntaryExit{
					{Exit: &ethpb.VoluntaryExit{Epoch: 0}},
					{Exit: &ethpb.VoluntaryExit{Epoch: 3}},
					{Exit: &ethpb.VoluntaryExit{Epoch: 4}},
					{Exit: &ethpb.VoluntaryExit{Epoch: 2}},
					{Exit: &ethpb.VoluntaryExit{Epoch: 1}},
				},
			},
			args: args{
				slot: 2 * params.BeaconConfig().SlotsPerEpoch,
			},
			want: []*ethpb.SignedVoluntaryExit{
				{Exit: &ethpb.VoluntaryExit{Epoch: 0}},
				{Exit: &ethpb.VoluntaryExit{Epoch: 2}},
				{Exit: &ethpb.VoluntaryExit{Epoch: 1}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Pool{
				pending: tt.fields.pending,
			}
			s, err := v1.InitializeFromProtoUnsafe(&p2ppb.BeaconState{Validators: []*ethpb.Validator{{ExitEpoch: params.BeaconConfig().FarFutureEpoch}}})
			require.NoError(t, err)
			if got := p.PendingExits(s, tt.args.slot, tt.fields.noLimit); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("PendingExits() = %v, want %v", got, tt.want)
			}
		})
	}
}
