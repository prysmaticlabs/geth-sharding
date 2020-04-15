package beacon

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gogo/protobuf/proto"
	ptypes "github.com/gogo/protobuf/types"
	"github.com/golang/mock/gomock"
	ethpb "github.com/prysmaticlabs/ethereumapis/eth/v1alpha1"
	"github.com/prysmaticlabs/go-bitfield"
	"github.com/prysmaticlabs/go-ssz"
	mock "github.com/prysmaticlabs/prysm/beacon-chain/blockchain/testing"
	"github.com/prysmaticlabs/prysm/beacon-chain/cache"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/feed"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/feed/operation"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	dbTest "github.com/prysmaticlabs/prysm/beacon-chain/db/testing"
	"github.com/prysmaticlabs/prysm/beacon-chain/flags"
	"github.com/prysmaticlabs/prysm/beacon-chain/operations/attestations"
	mockRPC "github.com/prysmaticlabs/prysm/beacon-chain/rpc/testing"
	stateTrie "github.com/prysmaticlabs/prysm/beacon-chain/state"
	"github.com/prysmaticlabs/prysm/beacon-chain/state/stategen"
	pbp2p "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	"github.com/prysmaticlabs/prysm/shared/attestationutil"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/testutil"
)

func TestServer_ListAttestations_NoResults(t *testing.T) {
	db := dbTest.SetupDB(t)
	defer dbTest.TeardownDB(t, db)

	ctx := context.Background()
	st, err := stateTrie.InitializeFromProto(&pbp2p.BeaconState{
		Slot: 0,
	})
	if err != nil {
		t.Fatal(err)
	}
	bs := &Server{
		BeaconDB: db,
		HeadFetcher: &mock.ChainService{
			State: st,
		},
	}
	wanted := &ethpb.ListAttestationsResponse{
		Attestations:  make([]*ethpb.Attestation, 0),
		TotalSize:     int32(0),
		NextPageToken: strconv.Itoa(0),
	}
	res, err := bs.ListAttestations(ctx, &ethpb.ListAttestationsRequest{
		QueryFilter: &ethpb.ListAttestationsRequest_GenesisEpoch{GenesisEpoch: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !proto.Equal(wanted, res) {
		t.Errorf("Wanted %v, received %v", wanted, res)
	}
}

func TestServer_ListAttestations_Genesis(t *testing.T) {
	db := dbTest.SetupDB(t)
	defer dbTest.TeardownDB(t, db)

	ctx := context.Background()
	st, err := stateTrie.InitializeFromProto(&pbp2p.BeaconState{
		Slot: 0,
	})
	if err != nil {
		t.Fatal(err)
	}
	bs := &Server{
		BeaconDB: db,
		HeadFetcher: &mock.ChainService{
			State: st,
		},
	}

	// Should throw an error if no genesis data is found.
	if _, err := bs.ListAttestations(ctx, &ethpb.ListAttestationsRequest{
		QueryFilter: &ethpb.ListAttestationsRequest_GenesisEpoch{
			GenesisEpoch: true,
		},
	}); err != nil && !strings.Contains(err.Error(), "Could not find genesis") {
		t.Fatal(err)
	}
	att := &ethpb.Attestation{Data: &ethpb.AttestationData{Slot: 2, CommitteeIndex: 1}}

	parentRoot := [32]byte{1, 2, 3}
	blk := &ethpb.SignedBeaconBlock{Block: &ethpb.BeaconBlock{
		Slot:       0,
		ParentRoot: parentRoot[:],
		Body: &ethpb.BeaconBlockBody{
			Attestations: []*ethpb.Attestation{att},
		},
	},
	}
	root, err := ssz.HashTreeRoot(blk.Block)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.SaveBlock(ctx, blk); err != nil {
		t.Fatal(err)
	}
	if err := db.SaveGenesisBlockRoot(ctx, root); err != nil {
		t.Fatal(err)
	}
	wanted := &ethpb.ListAttestationsResponse{
		Attestations:  []*ethpb.Attestation{att},
		NextPageToken: "",
		TotalSize:     1,
	}

	res, err := bs.ListAttestations(ctx, &ethpb.ListAttestationsRequest{
		QueryFilter: &ethpb.ListAttestationsRequest_GenesisEpoch{
			GenesisEpoch: true,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !proto.Equal(wanted, res) {
		t.Errorf("Wanted %v, received %v", wanted, res)
	}

	// Should throw an error if there is more than 1 block
	// for the genesis slot.
	if err := db.SaveBlock(ctx, blk); err != nil {
		t.Fatal(err)
	}
	if _, err := bs.ListAttestations(ctx, &ethpb.ListAttestationsRequest{
		QueryFilter: &ethpb.ListAttestationsRequest_GenesisEpoch{
			GenesisEpoch: true,
		},
	}); err != nil && !strings.Contains(err.Error(), "Found more than 1") {
		t.Fatal(err)
	}
}

func TestServer_ListAttestations_NoPagination(t *testing.T) {
	db := dbTest.SetupDB(t)
	defer dbTest.TeardownDB(t, db)
	ctx := context.Background()

	count := uint64(8)
	atts := make([]*ethpb.Attestation, 0, count)
	for i := uint64(0); i < count; i++ {
		blockExample := &ethpb.SignedBeaconBlock{
			Block: &ethpb.BeaconBlock{
				Slot: i,
				Body: &ethpb.BeaconBlockBody{
					Attestations: []*ethpb.Attestation{
						{
							Data: &ethpb.AttestationData{
								BeaconBlockRoot: []byte("root"),
								Slot:            i,
							},
							AggregationBits: bitfield.Bitlist{0b11},
						},
					},
				},
			},
		}
		if err := db.SaveBlock(ctx, blockExample); err != nil {
			t.Fatal(err)
		}
		atts = append(atts, blockExample.Block.Body.Attestations...)
	}

	bs := &Server{
		BeaconDB: db,
	}

	received, err := bs.ListAttestations(ctx, &ethpb.ListAttestationsRequest{
		QueryFilter: &ethpb.ListAttestationsRequest_GenesisEpoch{
			GenesisEpoch: true,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(atts, received.Attestations) {
		t.Fatalf("incorrect attestations response: wanted \n%v, received \n%v", atts, received.Attestations)
	}
}

func TestServer_ListAttestations_FiltersCorrectly(t *testing.T) {
	db := dbTest.SetupDB(t)
	defer dbTest.TeardownDB(t, db)
	ctx := context.Background()

	someRoot := []byte{1, 2, 3}
	sourceRoot := []byte{4, 5, 6}
	sourceEpoch := uint64(5)
	targetRoot := []byte{7, 8, 9}
	targetEpoch := uint64(7)

	blocks := []*ethpb.SignedBeaconBlock{
		{
			Block: &ethpb.BeaconBlock{
				Slot: 4,
				Body: &ethpb.BeaconBlockBody{
					Attestations: []*ethpb.Attestation{
						{
							Data: &ethpb.AttestationData{
								BeaconBlockRoot: someRoot,
								Source: &ethpb.Checkpoint{
									Root:  sourceRoot,
									Epoch: sourceEpoch,
								},
								Target: &ethpb.Checkpoint{
									Root:  targetRoot,
									Epoch: targetEpoch,
								},
								Slot: 3,
							},
							AggregationBits: bitfield.Bitlist{0b11},
						},
					},
				},
			},
		},
		{
			Block: &ethpb.BeaconBlock{
				Slot: 5 + params.BeaconConfig().SlotsPerEpoch,
				Body: &ethpb.BeaconBlockBody{
					Attestations: []*ethpb.Attestation{
						{
							Data: &ethpb.AttestationData{
								BeaconBlockRoot: someRoot,
								Source: &ethpb.Checkpoint{
									Root:  sourceRoot,
									Epoch: sourceEpoch,
								},
								Target: &ethpb.Checkpoint{
									Root:  targetRoot,
									Epoch: targetEpoch,
								},
								Slot: 4 + params.BeaconConfig().SlotsPerEpoch,
							},
							AggregationBits: bitfield.Bitlist{0b11},
						},
					},
				},
			},
		},
		{
			Block: &ethpb.BeaconBlock{
				Slot: 5,
				Body: &ethpb.BeaconBlockBody{
					Attestations: []*ethpb.Attestation{
						{
							Data: &ethpb.AttestationData{
								BeaconBlockRoot: someRoot,
								Source: &ethpb.Checkpoint{
									Root:  sourceRoot,
									Epoch: sourceEpoch,
								},
								Target: &ethpb.Checkpoint{
									Root:  targetRoot,
									Epoch: targetEpoch,
								},
								Slot: 4,
							},
							AggregationBits: bitfield.Bitlist{0b11},
						},
					},
				},
			},
		},
	}

	if err := db.SaveBlocks(ctx, blocks); err != nil {
		t.Fatal(err)
	}

	bs := &Server{
		BeaconDB: db,
	}

	received, err := bs.ListAttestations(ctx, &ethpb.ListAttestationsRequest{
		QueryFilter: &ethpb.ListAttestationsRequest_Epoch{Epoch: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(received.Attestations) != 1 {
		t.Errorf("Wanted 1 matching attestations for epoch %d, received %d", 1, len(received.Attestations))
	}
	received, err = bs.ListAttestations(ctx, &ethpb.ListAttestationsRequest{
		QueryFilter: &ethpb.ListAttestationsRequest_GenesisEpoch{GenesisEpoch: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(received.Attestations) != 2 {
		t.Errorf("Wanted 2 matching attestations for epoch %d, received %d", 0, len(received.Attestations))
	}
}

func TestServer_ListAttestations_Pagination_CustomPageParameters(t *testing.T) {
	db := dbTest.SetupDB(t)
	defer dbTest.TeardownDB(t, db)
	ctx := context.Background()

	count := params.BeaconConfig().SlotsPerEpoch * 4
	atts := make([]*ethpb.Attestation, 0, count)
	for i := uint64(0); i < params.BeaconConfig().SlotsPerEpoch; i++ {
		for s := uint64(0); s < 4; s++ {
			blockExample := &ethpb.SignedBeaconBlock{
				Block: &ethpb.BeaconBlock{
					Slot: i,
					Body: &ethpb.BeaconBlockBody{
						Attestations: []*ethpb.Attestation{
							{
								Data: &ethpb.AttestationData{
									CommitteeIndex: s,
									Slot:           i,
								},
								AggregationBits: bitfield.Bitlist{0b11},
							},
						},
					},
				},
			}
			if err := db.SaveBlock(ctx, blockExample); err != nil {
				t.Fatal(err)
			}
			atts = append(atts, blockExample.Block.Body.Attestations...)
		}
	}
	sort.Sort(sortableAttestations(atts))

	bs := &Server{
		BeaconDB: db,
	}

	tests := []struct {
		name string
		req  *ethpb.ListAttestationsRequest
		res  *ethpb.ListAttestationsResponse
	}{
		{
			name: "1st of 3 pages",
			req: &ethpb.ListAttestationsRequest{
				QueryFilter: &ethpb.ListAttestationsRequest_GenesisEpoch{
					GenesisEpoch: true,
				},
				PageToken: strconv.Itoa(1),
				PageSize:  3,
			},
			res: &ethpb.ListAttestationsResponse{
				Attestations: []*ethpb.Attestation{
					atts[3],
					atts[4],
					atts[5],
				},
				NextPageToken: strconv.Itoa(2),
				TotalSize:     int32(count),
			},
		},
		{
			name: "10 of size 1",
			req: &ethpb.ListAttestationsRequest{
				QueryFilter: &ethpb.ListAttestationsRequest_GenesisEpoch{
					GenesisEpoch: true,
				},
				PageToken: strconv.Itoa(10),
				PageSize:  1,
			},
			res: &ethpb.ListAttestationsResponse{
				Attestations: []*ethpb.Attestation{
					atts[10],
				},
				NextPageToken: strconv.Itoa(11),
				TotalSize:     int32(count),
			},
		},
		{
			name: "2 of size 8",
			req: &ethpb.ListAttestationsRequest{
				QueryFilter: &ethpb.ListAttestationsRequest_GenesisEpoch{
					GenesisEpoch: true,
				},
				PageToken: strconv.Itoa(2),
				PageSize:  8,
			},
			res: &ethpb.ListAttestationsResponse{
				Attestations: []*ethpb.Attestation{
					atts[16],
					atts[17],
					atts[18],
					atts[19],
					atts[20],
					atts[21],
					atts[22],
					atts[23],
				},
				NextPageToken: strconv.Itoa(3),
				TotalSize:     int32(count)},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			res, err := bs.ListAttestations(ctx, test.req)
			if err != nil {
				t.Fatal(err)
			}
			if !proto.Equal(res, test.res) {
				t.Errorf("Incorrect attestations response, wanted \n%v, received \n%v", test.res, res)
			}
		})
	}
}

func TestServer_ListAttestations_Pagination_OutOfRange(t *testing.T) {
	db := dbTest.SetupDB(t)
	defer dbTest.TeardownDB(t, db)
	ctx := context.Background()

	count := uint64(1)
	atts := make([]*ethpb.Attestation, 0, count)
	for i := uint64(0); i < count; i++ {
		blockExample := &ethpb.SignedBeaconBlock{
			Block: &ethpb.BeaconBlock{
				Body: &ethpb.BeaconBlockBody{
					Attestations: []*ethpb.Attestation{
						{
							Data: &ethpb.AttestationData{
								BeaconBlockRoot: []byte("root"),
								Slot:            i,
							},
							AggregationBits: bitfield.Bitlist{0b11},
						},
					},
				},
			},
		}
		if err := db.SaveBlock(ctx, blockExample); err != nil {
			t.Fatal(err)
		}
		atts = append(atts, blockExample.Block.Body.Attestations...)
	}

	bs := &Server{
		BeaconDB: db,
	}

	req := &ethpb.ListAttestationsRequest{
		QueryFilter: &ethpb.ListAttestationsRequest_Epoch{
			Epoch: 0,
		},
		PageToken: strconv.Itoa(1),
		PageSize:  100,
	}
	wanted := fmt.Sprintf("page start %d >= list %d", req.PageSize, len(atts))
	if _, err := bs.ListAttestations(ctx, req); !strings.Contains(err.Error(), wanted) {
		t.Errorf("Expected error %v, received %v", wanted, err)
	}
}

func TestServer_ListAttestations_Pagination_ExceedsMaxPageSize(t *testing.T) {
	ctx := context.Background()
	bs := &Server{}
	exceedsMax := int32(flags.Get().MaxPageSize + 1)

	wanted := fmt.Sprintf("Requested page size %d can not be greater than max size %d", exceedsMax, flags.Get().MaxPageSize)
	req := &ethpb.ListAttestationsRequest{PageToken: strconv.Itoa(0), PageSize: exceedsMax}
	if _, err := bs.ListAttestations(ctx, req); !strings.Contains(err.Error(), wanted) {
		t.Errorf("Expected error %v, received %v", wanted, err)
	}
}

func TestServer_ListAttestations_Pagination_DefaultPageSize(t *testing.T) {
	db := dbTest.SetupDB(t)
	defer dbTest.TeardownDB(t, db)
	ctx := context.Background()

	count := uint64(params.BeaconConfig().DefaultPageSize)
	atts := make([]*ethpb.Attestation, 0, count)
	for i := uint64(0); i < count; i++ {
		blockExample := &ethpb.SignedBeaconBlock{
			Block: &ethpb.BeaconBlock{
				Body: &ethpb.BeaconBlockBody{
					Attestations: []*ethpb.Attestation{
						{
							Data: &ethpb.AttestationData{
								BeaconBlockRoot: []byte("root"),
								Slot:            i,
							},
							AggregationBits: bitfield.Bitlist{0b11},
						},
					},
				},
			},
		}
		if err := db.SaveBlock(ctx, blockExample); err != nil {
			t.Fatal(err)
		}
		atts = append(atts, blockExample.Block.Body.Attestations...)
	}

	bs := &Server{
		BeaconDB: db,
	}

	req := &ethpb.ListAttestationsRequest{
		QueryFilter: &ethpb.ListAttestationsRequest_GenesisEpoch{
			GenesisEpoch: true,
		},
	}
	res, err := bs.ListAttestations(ctx, req)
	if err != nil {
		t.Fatal(err)
	}

	i := 0
	j := params.BeaconConfig().DefaultPageSize
	if !reflect.DeepEqual(res.Attestations, atts[i:j]) {
		t.Log(res.Attestations, atts[i:j])
		t.Error("Incorrect attestations response")
	}
}

func TestServer_ListIndexedAttestations_GenesisEpoch(t *testing.T) {
	db := dbTest.SetupDB(t)
	defer dbTest.TeardownDB(t, db)
	helpers.ClearCache()
	ctx := context.Background()

	params.OverrideBeaconConfig(params.MainnetConfig())
	defer params.OverrideBeaconConfig(params.MinimalSpecConfig())
	count := params.BeaconConfig().SlotsPerEpoch
	atts := make([]*ethpb.Attestation, 0, count)
	for i := uint64(0); i < count; i++ {
		blockExample := &ethpb.SignedBeaconBlock{
			Block: &ethpb.BeaconBlock{
				Body: &ethpb.BeaconBlockBody{
					Attestations: []*ethpb.Attestation{
						{
							Data: &ethpb.AttestationData{
								BeaconBlockRoot: []byte("root"),
								Slot:            i,
								CommitteeIndex:  0,
							},
							AggregationBits: bitfield.Bitlist{0b11},
						},
					},
				},
			},
		}
		if err := db.SaveBlock(ctx, blockExample); err != nil {
			t.Fatal(err)
		}
		atts = append(atts, blockExample.Block.Body.Attestations...)
	}

	// We setup 128 validators.
	numValidators := 128
	state := setupActiveValidators(t, db, numValidators)

	activeIndices, err := helpers.ActiveValidatorIndices(state, 0)
	if err != nil {
		t.Fatal(err)
	}
	epoch := uint64(0)
	attesterSeed, err := helpers.Seed(state, epoch, params.BeaconConfig().DomainBeaconAttester)
	if err != nil {
		t.Fatal(err)
	}
	committees, err := computeCommittees(helpers.StartSlot(epoch), activeIndices, attesterSeed)
	if err != nil {
		t.Fatal(err)
	}
	// Next up we convert the test attestations to indexed form:
	indexedAtts := make([]*ethpb.IndexedAttestation, len(atts), len(atts))
	for i := 0; i < len(indexedAtts); i++ {
		att := atts[i]
		committee := committees[att.Data.Slot].Committees[att.Data.CommitteeIndex]
		idxAtt := attestationutil.ConvertToIndexed(ctx, atts[i], committee.ValidatorIndices)
		indexedAtts[i] = idxAtt
	}

	summaryCache := cache.NewStateSummaryCache()
	bs := &Server{
		BeaconDB: db,
		GenesisTimeFetcher: &mock.ChainService{
			Genesis: time.Now(),
		},
		StateGen: stategen.New(db, summaryCache),
	}
	root := bytesutil.ToBytes32([]byte("root"))
	if err := db.SaveState(ctx, state, root); err != nil {
		t.Fatal(err)
	}
	stateRoot, err := state.HashTreeRoot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	summaryCache.Put(root, &pbp2p.StateSummary{
		Slot: 0,
		Root: stateRoot[:],
	})
	res, err := bs.ListIndexedAttestations(ctx, &ethpb.ListIndexedAttestationsRequest{
		QueryFilter: &ethpb.ListIndexedAttestationsRequest_GenesisEpoch{
			GenesisEpoch: true,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(indexedAtts, res.IndexedAttestations) {
		t.Fatalf(
			"Incorrect list indexed attestations response: wanted %v, received %v",
			indexedAtts,
			res.IndexedAttestations,
		)
	}
}

func TestServer_ListIndexedAttestations_ArchivedEpoch(t *testing.T) {
	db := dbTest.SetupDB(t)
	defer dbTest.TeardownDB(t, db)
	helpers.ClearCache()
	ctx := context.Background()

	count := params.BeaconConfig().SlotsPerEpoch
	atts := make([]*ethpb.Attestation, 0, count)
	startSlot := helpers.StartSlot(50)
	epoch := uint64(50)
	for i := startSlot; i < count; i++ {
		blockExample := &ethpb.SignedBeaconBlock{
			Block: &ethpb.BeaconBlock{
				Body: &ethpb.BeaconBlockBody{
					Attestations: []*ethpb.Attestation{
						{
							Data: &ethpb.AttestationData{
								BeaconBlockRoot: []byte("root"),
								Slot:            i,
								CommitteeIndex:  0,
								Target: &ethpb.Checkpoint{
									Epoch: epoch,
									Root:  make([]byte, 32),
								},
							},
							AggregationBits: bitfield.Bitlist{0b11},
						},
					},
				},
			},
		}
		if err := db.SaveBlock(ctx, blockExample); err != nil {
			t.Fatal(err)
		}
		atts = append(atts, blockExample.Block.Body.Attestations...)
	}

	// We setup 128 validators.
	numValidators := 128
	state := setupActiveValidators(t, db, numValidators)
	if err := state.SetSlot(startSlot); err != nil {
		t.Fatal(err)
	}

	activeIndices, err := helpers.ActiveValidatorIndices(state, epoch)
	if err != nil {
		t.Fatal(err)
	}
	attesterSeed, err := helpers.Seed(state, epoch, params.BeaconConfig().DomainBeaconAttester)
	if err != nil {
		t.Fatal(err)
	}
	committees, err := computeCommittees(epoch, activeIndices, attesterSeed)
	if err != nil {
		t.Fatal(err)
	}

	// Next up we convert the test attestations to indexed form:
	indexedAtts := make([]*ethpb.IndexedAttestation, len(atts), len(atts))
	for i := 0; i < len(indexedAtts); i++ {
		att := atts[i]
		committee := committees[att.Data.Slot].Committees[att.Data.CommitteeIndex]
		idxAtt := attestationutil.ConvertToIndexed(ctx, atts[i], committee.ValidatorIndices)
		indexedAtts[i] = idxAtt
	}

	bs := &Server{
		BeaconDB: db,
		GenesisTimeFetcher: &mock.ChainService{
			Genesis: time.Now(),
		},
	}
	if err := db.SaveState(ctx, state, bytesutil.ToBytes32([]byte("root"))); err != nil {
		t.Fatal(err)
	}
	res, err := bs.ListIndexedAttestations(ctx, &ethpb.ListIndexedAttestationsRequest{
		QueryFilter: &ethpb.ListIndexedAttestationsRequest_Epoch{
			Epoch: epoch,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(indexedAtts, res.IndexedAttestations) {
		t.Fatalf(
			"Incorrect list indexed attestations response: wanted %v, received %v",
			indexedAtts,
			res.IndexedAttestations,
		)
	}
}

func TestServer_AttestationPool_Pagination_ExceedsMaxPageSize(t *testing.T) {
	ctx := context.Background()
	bs := &Server{}
	exceedsMax := int32(flags.Get().MaxPageSize + 1)

	wanted := fmt.Sprintf("Requested page size %d can not be greater than max size %d", exceedsMax, flags.Get().MaxPageSize)
	req := &ethpb.AttestationPoolRequest{PageToken: strconv.Itoa(0), PageSize: exceedsMax}
	if _, err := bs.AttestationPool(ctx, req); err != nil && !strings.Contains(err.Error(), wanted) {
		t.Errorf("Expected error %v, received %v", wanted, err)
	}
}

func TestServer_AttestationPool_Pagination_OutOfRange(t *testing.T) {
	ctx := context.Background()
	bs := &Server{
		AttestationsPool: attestations.NewPool(),
	}

	atts := []*ethpb.Attestation{
		{Data: &ethpb.AttestationData{Slot: 1}, AggregationBits: bitfield.Bitlist{0b1101}},
		{Data: &ethpb.AttestationData{Slot: 2}, AggregationBits: bitfield.Bitlist{0b1101}},
		{Data: &ethpb.AttestationData{Slot: 3}, AggregationBits: bitfield.Bitlist{0b1101}},
	}
	if err := bs.AttestationsPool.SaveAggregatedAttestations(atts); err != nil {
		t.Fatal(err)
	}

	req := &ethpb.AttestationPoolRequest{
		PageToken: strconv.Itoa(1),
		PageSize:  100,
	}
	wanted := fmt.Sprintf("page start %d >= list %d", req.PageSize, len(atts))
	if _, err := bs.AttestationPool(ctx, req); err != nil && !strings.Contains(err.Error(), wanted) {
		t.Errorf("Expected error %v, received %v", wanted, err)
	}
}

func TestServer_AttestationPool_Pagination_DefaultPageSize(t *testing.T) {
	ctx := context.Background()
	bs := &Server{
		AttestationsPool: attestations.NewPool(),
	}

	atts := make([]*ethpb.Attestation, params.BeaconConfig().DefaultPageSize+1)
	for i := 0; i < len(atts); i++ {
		atts[i] = &ethpb.Attestation{
			Data:            &ethpb.AttestationData{Slot: uint64(i)},
			AggregationBits: bitfield.Bitlist{0b1101},
		}
	}
	if err := bs.AttestationsPool.SaveAggregatedAttestations(atts); err != nil {
		t.Fatal(err)
	}

	req := &ethpb.AttestationPoolRequest{}
	res, err := bs.AttestationPool(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Attestations) != params.BeaconConfig().DefaultPageSize {
		t.Errorf(
			"Wanted %d attestations in response, received %d",
			params.BeaconConfig().DefaultPageSize,
			len(res.Attestations),
		)
	}
	if int(res.TotalSize) != params.BeaconConfig().DefaultPageSize+1 {
		t.Errorf("Wanted total size %d, received %d", params.BeaconConfig().DefaultPageSize+1, res.TotalSize)
	}
}

func TestServer_AttestationPool_Pagination_CustomPageSize(t *testing.T) {
	ctx := context.Background()
	bs := &Server{
		AttestationsPool: attestations.NewPool(),
	}

	numAtts := 100
	atts := make([]*ethpb.Attestation, numAtts)
	for i := 0; i < len(atts); i++ {
		atts[i] = &ethpb.Attestation{
			Data:            &ethpb.AttestationData{Slot: uint64(i)},
			AggregationBits: bitfield.Bitlist{0b1101},
		}
	}
	if err := bs.AttestationsPool.SaveAggregatedAttestations(atts); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		req *ethpb.AttestationPoolRequest
		res *ethpb.AttestationPoolResponse
	}{
		{
			req: &ethpb.AttestationPoolRequest{
				PageToken: strconv.Itoa(1),
				PageSize:  3,
			},
			res: &ethpb.AttestationPoolResponse{
				NextPageToken: "2",
				TotalSize:     int32(numAtts),
			},
		},
		{
			req: &ethpb.AttestationPoolRequest{
				PageToken: strconv.Itoa(3),
				PageSize:  30,
			},
			res: &ethpb.AttestationPoolResponse{
				NextPageToken: "",
				TotalSize:     int32(numAtts),
			},
		},
		{
			req: &ethpb.AttestationPoolRequest{
				PageToken: strconv.Itoa(0),
				PageSize:  int32(numAtts),
			},
			res: &ethpb.AttestationPoolResponse{
				NextPageToken: "1",
				TotalSize:     int32(numAtts),
			},
		},
	}
	for _, tt := range tests {
		res, err := bs.AttestationPool(ctx, tt.req)
		if err != nil {
			t.Fatal(err)
		}
		if res.TotalSize != tt.res.TotalSize {
			t.Errorf("Wanted total size %d, received %d", tt.res.TotalSize, res.TotalSize)
		}
		if res.NextPageToken != tt.res.NextPageToken {
			t.Errorf("Wanted next page token %s, received %s", tt.res.NextPageToken, res.NextPageToken)
		}
	}
}

func TestServer_StreamIndexedAttestations_ContextCanceled(t *testing.T) {
	db := dbTest.SetupDB(t)
	defer dbTest.TeardownDB(t, db)
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	chainService := &mock.ChainService{}
	server := &Server{
		Ctx:                 ctx,
		AttestationNotifier: chainService.OperationNotifier(),
	}

	exitRoutine := make(chan bool)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockStream := mockRPC.NewMockBeaconChain_StreamIndexedAttestationsServer(ctrl)
	mockStream.EXPECT().Context().Return(ctx).AnyTimes()
	go func(tt *testing.T) {
		if err := server.StreamIndexedAttestations(
			&ptypes.Empty{},
			mockStream,
		); err != nil && !strings.Contains(err.Error(), "Context canceled") {
			tt.Errorf("Expected context canceled error got: %v", err)
		}
		<-exitRoutine
	}(t)
	cancel()
	exitRoutine <- true
}

func TestServer_StreamIndexedAttestations_OK(t *testing.T) {
	params.UseMainnetConfig()
	db := dbTest.SetupDB(t)
	defer dbTest.TeardownDB(t, db)
	exitRoutine := make(chan bool)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	ctx := context.Background()

	numValidators := 64
	headState, privKeys := testutil.DeterministicGenesisState(t, uint64(numValidators))
	b := &ethpb.SignedBeaconBlock{Block: &ethpb.BeaconBlock{}}
	if err := db.SaveBlock(ctx, b); err != nil {
		t.Fatal(err)
	}
	gRoot, err := ssz.HashTreeRoot(b.Block)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.SaveGenesisBlockRoot(ctx, gRoot); err != nil {
		t.Fatal(err)
	}
	if err := db.SaveState(ctx, headState, gRoot); err != nil {
		t.Fatal(err)
	}

	activeIndices, err := helpers.ActiveValidatorIndices(headState, 0)
	if err != nil {
		t.Fatal(err)
	}
	epoch := uint64(0)
	attesterSeed, err := helpers.Seed(headState, epoch, params.BeaconConfig().DomainBeaconAttester)
	if err != nil {
		t.Fatal(err)
	}
	committees, err := computeCommittees(helpers.StartSlot(epoch), activeIndices, attesterSeed)
	if err != nil {
		t.Fatal(err)
	}

	count := params.BeaconConfig().SlotsPerEpoch
	// We generate attestations for each validator per slot per epoch.
	atts := make([]*ethpb.Attestation, 0, count)
	for i := uint64(0); i < count; i++ {
		comms := committees[i].Committees
		for j := 0; j < numValidators; j++ {
			attExample := &ethpb.Attestation{
				Data: &ethpb.AttestationData{
					BeaconBlockRoot: bytesutil.PadTo([]byte("root"), 32),
					Slot:            i,
					Target: &ethpb.Checkpoint{
						Epoch: 0,
						Root:  make([]byte, 32),
					},
				},
			}
			encoded, err := helpers.ComputeSigningRoot(attExample.Data, []byte{})
			if err != nil {
				t.Fatal(err)
			}
			sig := privKeys[j].Sign(encoded[:])
			attExample.Signature = sig.Marshal()

			var indexInCommittee uint64
			var committeeIndex uint64
			var committeeLength int
			var found bool
			for comIndex, item := range comms {
				for n, idx := range item.ValidatorIndices {
					if uint64(j) == idx {
						indexInCommittee = uint64(n)
						committeeIndex = uint64(comIndex)
						committeeLength = len(item.ValidatorIndices)
						found = true
						break
					}
				}
			}
			if !found {
				continue
			}
			attExample.Data.CommitteeIndex = committeeIndex
			aggregationBitfield := bitfield.NewBitlist(uint64(committeeLength))
			aggregationBitfield.SetBitAt(indexInCommittee, true)
			attExample.AggregationBits = aggregationBitfield
			atts = append(atts, attExample)
		}
	}

	aggAtts, err := helpers.AggregateAttestations(atts)
	if err != nil {
		t.Fatal(err)
	}

	// Next up we convert the test attestations to indexed form.
	indexedAtts := make([]*ethpb.IndexedAttestation, len(aggAtts), len(aggAtts))
	for i := 0; i < len(indexedAtts); i++ {
		att := aggAtts[i]
		committee := committees[att.Data.Slot].Committees[att.Data.CommitteeIndex]
		idxAtt := attestationutil.ConvertToIndexed(ctx, att, committee.ValidatorIndices)
		indexedAtts[i] = idxAtt
	}

	chainService := &mock.ChainService{}
	server := &Server{
		BeaconDB: db,
		Ctx:      context.Background(),
		HeadFetcher: &mock.ChainService{
			State: headState,
		},
		GenesisTimeFetcher: &mock.ChainService{
			Genesis: time.Now(),
		},
		AttestationNotifier:         chainService.OperationNotifier(),
		CollectedAttestationsBuffer: make(chan []*ethpb.Attestation, 1),
		StateGen:                    stategen.New(db, cache.NewStateSummaryCache()),
	}

	mockStream := mockRPC.NewMockBeaconChain_StreamIndexedAttestationsServer(ctrl)
	for i := 0; i < len(indexedAtts); i++ {
		if i == len(indexedAtts)-1 {
			mockStream.EXPECT().Send(indexedAtts[i]).Do(func(arg0 interface{}) {
				exitRoutine <- true
			})
		} else {
			mockStream.EXPECT().Send(indexedAtts[i])
		}
	}
	mockStream.EXPECT().Context().Return(ctx).AnyTimes()

	go func(tt *testing.T) {
		if err := server.StreamIndexedAttestations(&ptypes.Empty{}, mockStream); err != nil {
			tt.Errorf("Could not call RPC method: %v", err)
		}
	}(t)

	server.CollectedAttestationsBuffer <- atts
	<-exitRoutine
}

func TestServer_StreamAttestations_ContextCanceled(t *testing.T) {
	db := dbTest.SetupDB(t)
	defer dbTest.TeardownDB(t, db)
	ctx := context.Background()

	ctx, cancel := context.WithCancel(ctx)
	chainService := &mock.ChainService{}
	server := &Server{
		Ctx:                 ctx,
		AttestationNotifier: chainService.OperationNotifier(),
	}

	exitRoutine := make(chan bool)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockStream := mockRPC.NewMockBeaconChain_StreamAttestationsServer(ctrl)
	mockStream.EXPECT().Context().Return(ctx)
	go func(tt *testing.T) {
		if err := server.StreamAttestations(
			&ptypes.Empty{},
			mockStream,
		); !strings.Contains(err.Error(), "Context canceled") {
			tt.Errorf("Expected context canceled error got: %v", err)
		}
		<-exitRoutine
	}(t)
	cancel()
	exitRoutine <- true
}

func TestServer_StreamAttestations_OnSlotTick(t *testing.T) {
	db := dbTest.SetupDB(t)
	defer dbTest.TeardownDB(t, db)
	exitRoutine := make(chan bool)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	ctx := context.Background()
	chainService := &mock.ChainService{}
	server := &Server{
		Ctx:                 ctx,
		AttestationNotifier: chainService.OperationNotifier(),
	}

	atts := []*ethpb.Attestation{
		{Data: &ethpb.AttestationData{Slot: 1}, AggregationBits: bitfield.Bitlist{0b1101}},
		{Data: &ethpb.AttestationData{Slot: 2}, AggregationBits: bitfield.Bitlist{0b1101}},
		{Data: &ethpb.AttestationData{Slot: 3}, AggregationBits: bitfield.Bitlist{0b1101}},
	}

	mockStream := mockRPC.NewMockBeaconChain_StreamAttestationsServer(ctrl)
	mockStream.EXPECT().Send(atts[0])
	mockStream.EXPECT().Send(atts[1])
	mockStream.EXPECT().Send(atts[2]).Do(func(arg0 interface{}) {
		exitRoutine <- true
	})
	mockStream.EXPECT().Context().Return(ctx).AnyTimes()

	go func(tt *testing.T) {
		if err := server.StreamAttestations(&ptypes.Empty{}, mockStream); err != nil {
			tt.Errorf("Could not call RPC method: %v", err)
		}
	}(t)
	for i := 0; i < len(atts); i++ {
		// Send in a loop to ensure it is delivered (busy wait for the service to subscribe to the state feed).
		for sent := 0; sent == 0; {
			sent = server.AttestationNotifier.OperationFeed().Send(&feed.Event{
				Type: operation.UnaggregatedAttReceived,
				Data: &operation.UnAggregatedAttReceivedData{Attestation: atts[i]},
			})
		}
	}
	<-exitRoutine
}
