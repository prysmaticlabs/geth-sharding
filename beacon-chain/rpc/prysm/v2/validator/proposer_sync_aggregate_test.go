package validator

import (
	"bytes"
	"sort"
	"testing"

	"github.com/prysmaticlabs/go-bitfield"
	v2 "github.com/prysmaticlabs/prysm/proto/prysm/v2"
	"github.com/prysmaticlabs/prysm/shared/testutil/assert"
)

func TestProposerSyncContributions_FilterByBlockRoot(t *testing.T) {
	rootA := [32]byte{'a'}
	rootB := [32]byte{'b'}
	tests := []struct {
		name string
		cs   proposerSyncContributions
		want proposerSyncContributions
	}{
		{
			name: "empty list",
			cs:   proposerSyncContributions{},
			want: proposerSyncContributions{},
		},
		{
			name: "single item, not found",
			cs: proposerSyncContributions{
				&v2.SyncCommitteeContribution{AggregationBits: bitfield.NewBitvector128()},
			},
			want: proposerSyncContributions{},
		},
		{
			name: "single item with filter, found",
			cs: proposerSyncContributions{
				&v2.SyncCommitteeContribution{BlockRoot: rootA[:], Slot: 0},
				&v2.SyncCommitteeContribution{BlockRoot: rootB[:], Slot: 1},
			},
			want: proposerSyncContributions{
				&v2.SyncCommitteeContribution{BlockRoot: rootA[:]},
			},
		},
		{
			name: "multiple items with filter, found",
			cs: proposerSyncContributions{
				&v2.SyncCommitteeContribution{BlockRoot: rootA[:], Slot: 0},
				&v2.SyncCommitteeContribution{BlockRoot: rootB[:], Slot: 1},
				&v2.SyncCommitteeContribution{BlockRoot: rootA[:], Slot: 2},
				&v2.SyncCommitteeContribution{BlockRoot: rootB[:], Slot: 3},
			},
			want: proposerSyncContributions{
				&v2.SyncCommitteeContribution{BlockRoot: rootA[:], Slot: 0},
				&v2.SyncCommitteeContribution{BlockRoot: rootA[:], Slot: 2},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cs := tt.cs.filterByBlockRoot(rootA)
			assert.DeepEqual(t, tt.want, cs)
		})
	}
}

func TestProposerSyncContributions_FilterBySubcommitteeID(t *testing.T) {
	rootA := [32]byte{'a'}
	rootB := [32]byte{'b'}
	tests := []struct {
		name string
		cs   proposerSyncContributions
		want proposerSyncContributions
	}{
		{
			name: "empty list",
			cs:   proposerSyncContributions{},
			want: proposerSyncContributions{},
		},
		{
			name: "single item, not found",
			cs: proposerSyncContributions{
				&v2.SyncCommitteeContribution{AggregationBits: bitfield.NewBitvector128(), SubcommitteeIndex: 1},
			},
			want: proposerSyncContributions{},
		},
		{
			name: "single item with filter",
			cs: proposerSyncContributions{
				&v2.SyncCommitteeContribution{BlockRoot: rootA[:], SubcommitteeIndex: 0},
				&v2.SyncCommitteeContribution{BlockRoot: rootB[:], SubcommitteeIndex: 1},
			},
			want: proposerSyncContributions{
				&v2.SyncCommitteeContribution{BlockRoot: rootA[:]},
			},
		},
		{
			name: "multiple items with filter",
			cs: proposerSyncContributions{
				&v2.SyncCommitteeContribution{BlockRoot: rootA[:], SubcommitteeIndex: 0},
				&v2.SyncCommitteeContribution{BlockRoot: rootB[:], SubcommitteeIndex: 1},
				&v2.SyncCommitteeContribution{BlockRoot: rootB[:], SubcommitteeIndex: 0},
				&v2.SyncCommitteeContribution{BlockRoot: rootB[:], SubcommitteeIndex: 2},
			},
			want: proposerSyncContributions{
				&v2.SyncCommitteeContribution{BlockRoot: rootA[:], SubcommitteeIndex: 0},
				&v2.SyncCommitteeContribution{BlockRoot: rootB[:], SubcommitteeIndex: 0},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cs := tt.cs.filterBySubIndex(0)
			assert.DeepEqual(t, tt.want, cs)
		})
	}
}

func TestProposerSyncContributions_Dedup(t *testing.T) {
	tests := []struct {
		name string
		cs   proposerSyncContributions
		want proposerSyncContributions
	}{
		{
			name: "nil list",
			cs:   nil,
			want: proposerSyncContributions(nil),
		},
		{
			name: "empty list",
			cs:   proposerSyncContributions{},
			want: proposerSyncContributions{},
		},
		{
			name: "single item",
			cs: proposerSyncContributions{
				&v2.SyncCommitteeContribution{AggregationBits: bitfield.NewBitvector128()},
			},
			want: proposerSyncContributions{
				&v2.SyncCommitteeContribution{AggregationBits: bitfield.NewBitvector128()},
			},
		},
		{
			name: "two items no duplicates",
			cs: proposerSyncContributions{
				&v2.SyncCommitteeContribution{AggregationBits: bitfield.Bitvector128{0b10111110, 0x01}},
				&v2.SyncCommitteeContribution{AggregationBits: bitfield.Bitvector128{0b01111111, 0x01}},
			},
			want: proposerSyncContributions{
				&v2.SyncCommitteeContribution{AggregationBits: bitfield.Bitvector128{0b01111111, 0x01}},
				&v2.SyncCommitteeContribution{AggregationBits: bitfield.Bitvector128{0b10111110, 0x01}},
			},
		},
		{
			name: "two items with duplicates",
			cs: proposerSyncContributions{
				&v2.SyncCommitteeContribution{AggregationBits: bitfield.Bitvector128{0xba, 0x01}},
				&v2.SyncCommitteeContribution{AggregationBits: bitfield.Bitvector128{0xba, 0x01}},
			},
			want: proposerSyncContributions{
				&v2.SyncCommitteeContribution{AggregationBits: bitfield.Bitvector128{0xba, 0x01}},
			},
		},
		{
			name: "sorted no duplicates",
			cs: proposerSyncContributions{
				&v2.SyncCommitteeContribution{AggregationBits: bitfield.Bitvector128{0b11001111, 0b1}},
				&v2.SyncCommitteeContribution{AggregationBits: bitfield.Bitvector128{0b01101101, 0b1}},
				&v2.SyncCommitteeContribution{AggregationBits: bitfield.Bitvector128{0b00101011, 0b1}},
				&v2.SyncCommitteeContribution{AggregationBits: bitfield.Bitvector128{0b10100000, 0b1}},
				&v2.SyncCommitteeContribution{AggregationBits: bitfield.Bitvector128{0b00010000, 0b1}},
			},
			want: proposerSyncContributions{
				&v2.SyncCommitteeContribution{AggregationBits: bitfield.Bitvector128{0b11001111, 0b1}},
				&v2.SyncCommitteeContribution{AggregationBits: bitfield.Bitvector128{0b01101101, 0b1}},
				&v2.SyncCommitteeContribution{AggregationBits: bitfield.Bitvector128{0b00101011, 0b1}},
				&v2.SyncCommitteeContribution{AggregationBits: bitfield.Bitvector128{0b10100000, 0b1}},
				&v2.SyncCommitteeContribution{AggregationBits: bitfield.Bitvector128{0b00010000, 0b1}},
			},
		},
		{
			name: "sorted with duplicates",
			cs: proposerSyncContributions{
				&v2.SyncCommitteeContribution{AggregationBits: bitfield.Bitvector128{0b11001111, 0b1}},
				&v2.SyncCommitteeContribution{AggregationBits: bitfield.Bitvector128{0b01101101, 0b1}},
				&v2.SyncCommitteeContribution{AggregationBits: bitfield.Bitvector128{0b01101101, 0b1}},
				&v2.SyncCommitteeContribution{AggregationBits: bitfield.Bitvector128{0b01101101, 0b1}},
				&v2.SyncCommitteeContribution{AggregationBits: bitfield.Bitvector128{0b00001111, 0b1}},
				&v2.SyncCommitteeContribution{AggregationBits: bitfield.Bitvector128{0b00000011, 0b1}},
				&v2.SyncCommitteeContribution{AggregationBits: bitfield.Bitvector128{0b00000011, 0b1}},
				&v2.SyncCommitteeContribution{AggregationBits: bitfield.Bitvector128{0b00000001, 0b1}},
			},
			want: proposerSyncContributions{
				&v2.SyncCommitteeContribution{AggregationBits: bitfield.Bitvector128{0b11001111, 0b1}},
				&v2.SyncCommitteeContribution{AggregationBits: bitfield.Bitvector128{0b01101101, 0b1}},
			},
		},
		{
			name: "all equal",
			cs: proposerSyncContributions{
				&v2.SyncCommitteeContribution{AggregationBits: bitfield.Bitvector128{0b00000011, 0b1}},
				&v2.SyncCommitteeContribution{AggregationBits: bitfield.Bitvector128{0b00000011, 0b1}},
				&v2.SyncCommitteeContribution{AggregationBits: bitfield.Bitvector128{0b00000011, 0b1}},
				&v2.SyncCommitteeContribution{AggregationBits: bitfield.Bitvector128{0b00000011, 0b1}},
				&v2.SyncCommitteeContribution{AggregationBits: bitfield.Bitvector128{0b00000011, 0b1}},
				&v2.SyncCommitteeContribution{AggregationBits: bitfield.Bitvector128{0b00000011, 0b1}},
			},
			want: proposerSyncContributions{
				&v2.SyncCommitteeContribution{AggregationBits: bitfield.Bitvector128{0b00000011, 0b1}},
			},
		},
		{
			name: "unsorted no duplicates",
			cs: proposerSyncContributions{
				&v2.SyncCommitteeContribution{AggregationBits: bitfield.Bitvector128{0b01101101, 0b1}},
				&v2.SyncCommitteeContribution{AggregationBits: bitfield.Bitvector128{0b00100010, 0b1}},
				&v2.SyncCommitteeContribution{AggregationBits: bitfield.Bitvector128{0b10100101, 0b1}},
				&v2.SyncCommitteeContribution{AggregationBits: bitfield.Bitvector128{0b00010000, 0b1}},
				&v2.SyncCommitteeContribution{AggregationBits: bitfield.Bitvector128{0b11001111, 0b1}},
			},
			want: proposerSyncContributions{
				&v2.SyncCommitteeContribution{AggregationBits: bitfield.Bitvector128{0b11001111, 0b1}},
				&v2.SyncCommitteeContribution{AggregationBits: bitfield.Bitvector128{0b01101101, 0b1}},
				&v2.SyncCommitteeContribution{AggregationBits: bitfield.Bitvector128{0b10100101, 0b1}},
				&v2.SyncCommitteeContribution{AggregationBits: bitfield.Bitvector128{0b00100010, 0b1}},
				&v2.SyncCommitteeContribution{AggregationBits: bitfield.Bitvector128{0b00010000, 0b1}},
			},
		},
		{
			name: "unsorted with duplicates",
			cs: proposerSyncContributions{
				&v2.SyncCommitteeContribution{AggregationBits: bitfield.Bitvector128{0b00001111, 0b1}},
				&v2.SyncCommitteeContribution{AggregationBits: bitfield.Bitvector128{0b11001111, 0b1}},
				&v2.SyncCommitteeContribution{AggregationBits: bitfield.Bitvector128{0b10100101, 0b1}},
				&v2.SyncCommitteeContribution{AggregationBits: bitfield.Bitvector128{0b10100101, 0b1}},
				&v2.SyncCommitteeContribution{AggregationBits: bitfield.Bitvector128{0b00000001, 0b1}},
				&v2.SyncCommitteeContribution{AggregationBits: bitfield.Bitvector128{0b00000011, 0b1}},
				&v2.SyncCommitteeContribution{AggregationBits: bitfield.Bitvector128{0b11001111, 0b1}},
				&v2.SyncCommitteeContribution{AggregationBits: bitfield.Bitvector128{0b01101101, 0b1}},
				&v2.SyncCommitteeContribution{AggregationBits: bitfield.Bitvector128{0b00000001, 0b1}},
			},
			want: proposerSyncContributions{
				&v2.SyncCommitteeContribution{AggregationBits: bitfield.Bitvector128{0b11001111, 0b1}},
				&v2.SyncCommitteeContribution{AggregationBits: bitfield.Bitvector128{0b01101101, 0b1}},
				&v2.SyncCommitteeContribution{AggregationBits: bitfield.Bitvector128{0b10100101, 0b1}},
			},
		},
		{
			name: "no proper subset (same root)",
			cs: proposerSyncContributions{
				&v2.SyncCommitteeContribution{AggregationBits: bitfield.Bitvector128{0b00000101, 0b1}},
				&v2.SyncCommitteeContribution{AggregationBits: bitfield.Bitvector128{0b00000011, 0b1}},
				&v2.SyncCommitteeContribution{AggregationBits: bitfield.Bitvector128{0b10000001, 0b1}},
				&v2.SyncCommitteeContribution{AggregationBits: bitfield.Bitvector128{0b00011001, 0b1}},
			},
			want: proposerSyncContributions{
				&v2.SyncCommitteeContribution{AggregationBits: bitfield.Bitvector128{0b00011001, 0b1}},
				&v2.SyncCommitteeContribution{AggregationBits: bitfield.Bitvector128{0b00000011, 0b1}},
				&v2.SyncCommitteeContribution{AggregationBits: bitfield.Bitvector128{0b00000101, 0b1}},
				&v2.SyncCommitteeContribution{AggregationBits: bitfield.Bitvector128{0b10000001, 0b1}},
			},
		},
		{
			name: "proper subset (same root)",
			cs: proposerSyncContributions{
				&v2.SyncCommitteeContribution{AggregationBits: bitfield.Bitvector128{0b00001111, 0b1}},
				&v2.SyncCommitteeContribution{AggregationBits: bitfield.Bitvector128{0b11001111, 0b1}},
				&v2.SyncCommitteeContribution{AggregationBits: bitfield.Bitvector128{0b00001111, 0b1}},
				&v2.SyncCommitteeContribution{AggregationBits: bitfield.Bitvector128{0b00001111, 0b1}},
				&v2.SyncCommitteeContribution{AggregationBits: bitfield.Bitvector128{0b00000001, 0b1}},
				&v2.SyncCommitteeContribution{AggregationBits: bitfield.Bitvector128{0b00000011, 0b1}},
				&v2.SyncCommitteeContribution{AggregationBits: bitfield.Bitvector128{0b11001111, 0b1}},
				&v2.SyncCommitteeContribution{AggregationBits: bitfield.Bitvector128{0b00000001, 0b1}},
				&v2.SyncCommitteeContribution{AggregationBits: bitfield.Bitvector128{0b01101101, 0b1}},
			},
			want: proposerSyncContributions{
				&v2.SyncCommitteeContribution{AggregationBits: bitfield.Bitvector128{0b11001111, 0b1}},
				&v2.SyncCommitteeContribution{AggregationBits: bitfield.Bitvector128{0b01101101, 0b1}},
			},
		},
		{
			name: "no proper subset (different index)",
			cs: proposerSyncContributions{
				&v2.SyncCommitteeContribution{AggregationBits: bitfield.Bitvector128{0b00000101, 0b1}},
				&v2.SyncCommitteeContribution{AggregationBits: bitfield.Bitvector128{0b00000011, 0b1}},
				&v2.SyncCommitteeContribution{SubcommitteeIndex: 1, AggregationBits: bitfield.Bitvector128{0b10000001, 0b1}},
				&v2.SyncCommitteeContribution{SubcommitteeIndex: 1, AggregationBits: bitfield.Bitvector128{0b00011001, 0b1}},
			},
			want: proposerSyncContributions{
				&v2.SyncCommitteeContribution{SubcommitteeIndex: 1, AggregationBits: bitfield.Bitvector128{0b00011001, 0b1}},
				&v2.SyncCommitteeContribution{SubcommitteeIndex: 1, AggregationBits: bitfield.Bitvector128{0b10000001, 0b1}},
				&v2.SyncCommitteeContribution{AggregationBits: bitfield.Bitvector128{0b00000011, 0b1}},
				&v2.SyncCommitteeContribution{AggregationBits: bitfield.Bitvector128{0b00000101, 0b1}},
			},
		},
		{
			name: "proper subset (different index 1)",
			cs: proposerSyncContributions{
				&v2.SyncCommitteeContribution{SubcommitteeIndex: 1, AggregationBits: bitfield.Bitvector128{0b00001111, 0b1}},
				&v2.SyncCommitteeContribution{SubcommitteeIndex: 1, AggregationBits: bitfield.Bitvector128{0b11001111, 0b1}},
				&v2.SyncCommitteeContribution{SubcommitteeIndex: 1, AggregationBits: bitfield.Bitvector128{0b00001111, 0b1}},
				&v2.SyncCommitteeContribution{SubcommitteeIndex: 1, AggregationBits: bitfield.Bitvector128{0b00001111, 0b1}},
				&v2.SyncCommitteeContribution{AggregationBits: bitfield.Bitvector128{0b00000001, 0b1}},
				&v2.SyncCommitteeContribution{SubcommitteeIndex: 1, AggregationBits: bitfield.Bitvector128{0b00000011, 0b1}},
				&v2.SyncCommitteeContribution{SubcommitteeIndex: 1, AggregationBits: bitfield.Bitvector128{0b11001111, 0b1}},
				&v2.SyncCommitteeContribution{AggregationBits: bitfield.Bitvector128{0b00000001, 0b1}},
				&v2.SyncCommitteeContribution{AggregationBits: bitfield.Bitvector128{0b01101101, 0b1}},
			},
			want: proposerSyncContributions{
				&v2.SyncCommitteeContribution{SubcommitteeIndex: 1, AggregationBits: bitfield.Bitvector128{0b11001111, 0b1}},
				&v2.SyncCommitteeContribution{AggregationBits: bitfield.Bitvector128{0b01101101, 0b1}},
			},
		},
		{
			name: "proper subset (different index 2)",
			cs: proposerSyncContributions{
				&v2.SyncCommitteeContribution{AggregationBits: bitfield.Bitvector128{0b00001111, 0b1}},
				&v2.SyncCommitteeContribution{AggregationBits: bitfield.Bitvector128{0b11001111, 0b1}},
				&v2.SyncCommitteeContribution{SubcommitteeIndex: 1, AggregationBits: bitfield.Bitvector128{0b00001111, 0b1}},
				&v2.SyncCommitteeContribution{SubcommitteeIndex: 1, AggregationBits: bitfield.Bitvector128{0b11001111, 0b1}},
			},
			want: proposerSyncContributions{
				&v2.SyncCommitteeContribution{SubcommitteeIndex: 1, AggregationBits: bitfield.Bitvector128{0b11001111, 0b1}},
				&v2.SyncCommitteeContribution{AggregationBits: bitfield.Bitvector128{0b11001111, 0b1}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cs, err := tt.cs.dedup()
			if err != nil {
				t.Error(err)
			}
			sort.Slice(cs, func(i, j int) bool {
				if cs[i].AggregationBits.Count() == cs[j].AggregationBits.Count() {
					if cs[i].SubcommitteeIndex == cs[j].SubcommitteeIndex {
						return bytes.Compare(cs[i].AggregationBits, cs[j].AggregationBits) <= 0
					}
					return cs[i].SubcommitteeIndex > cs[j].SubcommitteeIndex
				}
				return cs[i].AggregationBits.Count() > cs[j].AggregationBits.Count()
			})
			assert.DeepEqual(t, tt.want, cs)
		})
	}
}

func TestProposerSyncContributions_MostProfitable(t *testing.T) {
	tests := []struct {
		name string
		cs   proposerSyncContributions
		want *v2.SyncCommitteeContribution
	}{
		{
			name: "Same item",
			cs: proposerSyncContributions{
				&v2.SyncCommitteeContribution{AggregationBits: bitfield.Bitvector128{0b01}},
				&v2.SyncCommitteeContribution{AggregationBits: bitfield.Bitvector128{0b01}},
			},
			want: &v2.SyncCommitteeContribution{AggregationBits: bitfield.Bitvector128{0b01}},
		},
		{
			name: "Same item again",
			cs: proposerSyncContributions{
				&v2.SyncCommitteeContribution{AggregationBits: bitfield.Bitvector128{0b01}},
				&v2.SyncCommitteeContribution{AggregationBits: bitfield.Bitvector128{0b10}},
			},
			want: &v2.SyncCommitteeContribution{AggregationBits: bitfield.Bitvector128{0b01}},
		},
		{
			name: "most profitable at the start",
			cs: proposerSyncContributions{
				&v2.SyncCommitteeContribution{AggregationBits: bitfield.Bitvector128{0b0101}},
				&v2.SyncCommitteeContribution{AggregationBits: bitfield.Bitvector128{0b0100}},
			},
			want: &v2.SyncCommitteeContribution{AggregationBits: bitfield.Bitvector128{0b0101}},
		},
		{
			name: "most profitable at the end",
			cs: proposerSyncContributions{
				&v2.SyncCommitteeContribution{AggregationBits: bitfield.Bitvector128{0b0101}},
				&v2.SyncCommitteeContribution{AggregationBits: bitfield.Bitvector128{0b0111}},
			},
			want: &v2.SyncCommitteeContribution{AggregationBits: bitfield.Bitvector128{0b0111}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cs := tt.cs.mostProfitable()
			assert.DeepEqual(t, tt.want, cs)
		})
	}
}
