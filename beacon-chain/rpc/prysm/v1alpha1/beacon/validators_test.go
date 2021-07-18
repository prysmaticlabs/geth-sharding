package beacon

import (
	"context"
	"encoding/binary"
	"fmt"
	"sort"
	"strconv"
	"testing"
	"time"

	types "github.com/prysmaticlabs/eth2-types"
	"github.com/prysmaticlabs/go-bitfield"
	mock "github.com/prysmaticlabs/prysm/beacon-chain/blockchain/testing"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/epoch/precompute"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	"github.com/prysmaticlabs/prysm/beacon-chain/db"
	dbTest "github.com/prysmaticlabs/prysm/beacon-chain/db/testing"
	iface "github.com/prysmaticlabs/prysm/beacon-chain/state/interface"
	"github.com/prysmaticlabs/prysm/beacon-chain/state/stategen"
	v1 "github.com/prysmaticlabs/prysm/beacon-chain/state/v1"
	mockSync "github.com/prysmaticlabs/prysm/beacon-chain/sync/initial-sync/testing"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	ethpb "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/proto/eth/v1alpha1/wrapper"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/prysmaticlabs/prysm/shared/cmd"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/testutil"
	"github.com/prysmaticlabs/prysm/shared/testutil/assert"
	"github.com/prysmaticlabs/prysm/shared/testutil/require"
	"github.com/prysmaticlabs/prysm/shared/timeutils"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"
)

const (
	errNoEpochInfoError = "Cannot retrieve information about an epoch in the future"
)

func TestServer_GetValidatorActiveSetChanges_CannotRequestFutureEpoch(t *testing.T) {
	beaconDB := dbTest.SetupDB(t)
	ctx := context.Background()
	st, err := testutil.NewBeaconState()
	require.NoError(t, err)
	require.NoError(t, st.SetSlot(0))
	bs := &Server{
		GenesisTimeFetcher: &mock.ChainService{},
		HeadFetcher: &mock.ChainService{
			State: st,
		},
		BeaconDB: beaconDB,
	}

	wanted := errNoEpochInfoError
	_, err = bs.GetValidatorActiveSetChanges(
		ctx,
		&ethpb.GetValidatorActiveSetChangesRequest{
			QueryFilter: &ethpb.GetValidatorActiveSetChangesRequest_Epoch{
				Epoch: helpers.SlotToEpoch(bs.GenesisTimeFetcher.CurrentSlot()) + 1,
			},
		},
	)
	assert.ErrorContains(t, wanted, err)
}

func TestServer_ListValidatorBalances_CannotRequestFutureEpoch(t *testing.T) {
	beaconDB := dbTest.SetupDB(t)
	ctx := context.Background()

	st, err := testutil.NewBeaconState()
	require.NoError(t, err)
	require.NoError(t, st.SetSlot(0))
	bs := &Server{
		BeaconDB: beaconDB,
		HeadFetcher: &mock.ChainService{
			State: st,
		},
		GenesisTimeFetcher: &mock.ChainService{},
	}

	wanted := errNoEpochInfoError
	_, err = bs.ListValidatorBalances(
		ctx,
		&ethpb.ListValidatorBalancesRequest{
			QueryFilter: &ethpb.ListValidatorBalancesRequest_Epoch{
				Epoch: helpers.SlotToEpoch(bs.GenesisTimeFetcher.CurrentSlot()) + 1,
			},
		},
	)
	assert.ErrorContains(t, wanted, err)
}

func TestServer_ListValidatorBalances_NoResults(t *testing.T) {
	beaconDB := dbTest.SetupDB(t)

	ctx := context.Background()
	st, err := testutil.NewBeaconState()
	require.NoError(t, err)
	require.NoError(t, st.SetSlot(0))
	bs := &Server{
		GenesisTimeFetcher: &mock.ChainService{},
		StateGen:           stategen.New(beaconDB),
	}

	headState, err := testutil.NewBeaconState()
	require.NoError(t, err)
	b := testutil.NewBeaconBlock()
	require.NoError(t, beaconDB.SaveBlock(ctx, wrapper.WrappedPhase0SignedBeaconBlock(b)))
	gRoot, err := b.Block.HashTreeRoot()
	require.NoError(t, err)
	require.NoError(t, beaconDB.SaveGenesisBlockRoot(ctx, gRoot))
	require.NoError(t, beaconDB.SaveState(ctx, headState, gRoot))

	wanted := &ethpb.ValidatorBalances{
		Balances:      make([]*ethpb.ValidatorBalances_Balance, 0),
		TotalSize:     int32(0),
		NextPageToken: strconv.Itoa(0),
	}
	res, err := bs.ListValidatorBalances(
		ctx,
		&ethpb.ListValidatorBalancesRequest{
			QueryFilter: &ethpb.ListValidatorBalancesRequest_Epoch{
				Epoch: 0,
			},
		},
	)
	require.NoError(t, err)
	if !proto.Equal(wanted, res) {
		t.Errorf("Wanted %v, received %v", wanted, res)
	}
}

func TestServer_ListValidatorBalances_DefaultResponse_NoArchive(t *testing.T) {
	beaconDB := dbTest.SetupDB(t)
	ctx := context.Background()

	numItems := 100
	validators := make([]*ethpb.Validator, numItems)
	balances := make([]uint64, numItems)
	balancesResponse := make([]*ethpb.ValidatorBalances_Balance, numItems)
	for i := 0; i < numItems; i++ {
		validators[i] = &ethpb.Validator{
			PublicKey:             pubKey(uint64(i)),
			WithdrawalCredentials: make([]byte, 32),
		}
		balances[i] = params.BeaconConfig().MaxEffectiveBalance
		balancesResponse[i] = &ethpb.ValidatorBalances_Balance{
			PublicKey: pubKey(uint64(i)),
			Index:     types.ValidatorIndex(i),
			Balance:   params.BeaconConfig().MaxEffectiveBalance,
			Status:    "EXITED",
		}
	}
	st, err := testutil.NewBeaconState()
	require.NoError(t, err)
	require.NoError(t, st.SetSlot(0))
	require.NoError(t, st.SetValidators(validators))
	require.NoError(t, st.SetBalances(balances))
	b := testutil.NewBeaconBlock()
	require.NoError(t, beaconDB.SaveBlock(ctx, wrapper.WrappedPhase0SignedBeaconBlock(b)))
	gRoot, err := b.Block.HashTreeRoot()
	require.NoError(t, err)
	require.NoError(t, beaconDB.SaveGenesisBlockRoot(ctx, gRoot))
	require.NoError(t, beaconDB.SaveState(ctx, st, gRoot))
	bs := &Server{
		GenesisTimeFetcher: &mock.ChainService{},
		StateGen:           stategen.New(beaconDB),
		HeadFetcher: &mock.ChainService{
			State: st,
		},
	}
	res, err := bs.ListValidatorBalances(
		ctx,
		&ethpb.ListValidatorBalancesRequest{
			QueryFilter: &ethpb.ListValidatorBalancesRequest_Epoch{Epoch: 0},
		},
	)
	require.NoError(t, err)
	assert.DeepEqual(t, balancesResponse, res.Balances)
}

func TestServer_ListValidatorBalances_PaginationOutOfRange(t *testing.T) {
	beaconDB := dbTest.SetupDB(t)
	ctx := context.Background()

	_, _, headState := setupValidators(t, beaconDB, 100)
	b := testutil.NewBeaconBlock()
	gRoot, err := b.Block.HashTreeRoot()
	require.NoError(t, err)
	require.NoError(t, beaconDB.SaveGenesisBlockRoot(ctx, gRoot))
	require.NoError(t, beaconDB.SaveState(ctx, headState, gRoot))

	bs := &Server{
		GenesisTimeFetcher: &mock.ChainService{},
		StateGen:           stategen.New(beaconDB),
		HeadFetcher: &mock.ChainService{
			State: headState,
		},
	}

	wanted := fmt.Sprintf("page start %d >= list %d", 200, len(headState.Balances()))
	_, err = bs.ListValidatorBalances(context.Background(), &ethpb.ListValidatorBalancesRequest{
		PageToken:   strconv.Itoa(2),
		PageSize:    100,
		QueryFilter: &ethpb.ListValidatorBalancesRequest_Epoch{Epoch: 0},
	})
	assert.ErrorContains(t, wanted, err)
}

func TestServer_ListValidatorBalances_ExceedsMaxPageSize(t *testing.T) {
	bs := &Server{}
	exceedsMax := int32(cmd.Get().MaxRPCPageSize + 1)

	wanted := fmt.Sprintf(
		"Requested page size %d can not be greater than max size %d",
		exceedsMax,
		cmd.Get().MaxRPCPageSize,
	)
	req := &ethpb.ListValidatorBalancesRequest{PageSize: exceedsMax}
	_, err := bs.ListValidatorBalances(context.Background(), req)
	assert.ErrorContains(t, wanted, err)
}

func pubKey(i uint64) []byte {
	pubKey := make([]byte, params.BeaconConfig().BLSPubkeyLength)
	binary.LittleEndian.PutUint64(pubKey, i)
	return pubKey
}

func TestServer_ListValidatorBalances_Pagination_Default(t *testing.T) {
	beaconDB := dbTest.SetupDB(t)
	ctx := context.Background()

	_, _, headState := setupValidators(t, beaconDB, 100)
	b := testutil.NewBeaconBlock()
	gRoot, err := b.Block.HashTreeRoot()
	require.NoError(t, err)
	require.NoError(t, beaconDB.SaveGenesisBlockRoot(ctx, gRoot))
	require.NoError(t, beaconDB.SaveState(ctx, headState, gRoot))

	bs := &Server{
		GenesisTimeFetcher: &mock.ChainService{},
		StateGen:           stategen.New(beaconDB),
		HeadFetcher: &mock.ChainService{
			State: headState,
		},
	}

	tests := []struct {
		req *ethpb.ListValidatorBalancesRequest
		res *ethpb.ValidatorBalances
	}{
		{req: &ethpb.ListValidatorBalancesRequest{PublicKeys: [][]byte{pubKey(99)}, QueryFilter: &ethpb.ListValidatorBalancesRequest_Epoch{Epoch: 0}},
			res: &ethpb.ValidatorBalances{
				Balances: []*ethpb.ValidatorBalances_Balance{
					{Index: 99, PublicKey: pubKey(99), Balance: 99, Status: "EXITED"},
				},
				NextPageToken: "",
				TotalSize:     1,
			},
		},
		{req: &ethpb.ListValidatorBalancesRequest{Indices: []types.ValidatorIndex{1, 2, 3}, QueryFilter: &ethpb.ListValidatorBalancesRequest_Epoch{Epoch: 0}},
			res: &ethpb.ValidatorBalances{
				Balances: []*ethpb.ValidatorBalances_Balance{
					{Index: 1, PublicKey: pubKey(1), Balance: 1, Status: "EXITED"},
					{Index: 2, PublicKey: pubKey(2), Balance: 2, Status: "EXITED"},
					{Index: 3, PublicKey: pubKey(3), Balance: 3, Status: "EXITED"},
				},
				NextPageToken: "",
				TotalSize:     3,
			},
		},
		{req: &ethpb.ListValidatorBalancesRequest{PublicKeys: [][]byte{pubKey(10), pubKey(11), pubKey(12)}, QueryFilter: &ethpb.ListValidatorBalancesRequest_Epoch{Epoch: 0}},
			res: &ethpb.ValidatorBalances{
				Balances: []*ethpb.ValidatorBalances_Balance{
					{Index: 10, PublicKey: pubKey(10), Balance: 10, Status: "EXITED"},
					{Index: 11, PublicKey: pubKey(11), Balance: 11, Status: "EXITED"},
					{Index: 12, PublicKey: pubKey(12), Balance: 12, Status: "EXITED"},
				},
				NextPageToken: "",
				TotalSize:     3,
			}},
		{req: &ethpb.ListValidatorBalancesRequest{PublicKeys: [][]byte{pubKey(2), pubKey(3)}, Indices: []types.ValidatorIndex{3, 4}, QueryFilter: &ethpb.ListValidatorBalancesRequest_Epoch{Epoch: 0}}, // Duplication
			res: &ethpb.ValidatorBalances{
				Balances: []*ethpb.ValidatorBalances_Balance{
					{Index: 2, PublicKey: pubKey(2), Balance: 2, Status: "EXITED"},
					{Index: 3, PublicKey: pubKey(3), Balance: 3, Status: "EXITED"},
					{Index: 4, PublicKey: pubKey(4), Balance: 4, Status: "EXITED"},
				},
				NextPageToken: "",
				TotalSize:     3,
			}},
		{req: &ethpb.ListValidatorBalancesRequest{PublicKeys: [][]byte{{}}, Indices: []types.ValidatorIndex{3, 4}, QueryFilter: &ethpb.ListValidatorBalancesRequest_Epoch{Epoch: 0}}, // Public key has a blank value
			res: &ethpb.ValidatorBalances{
				Balances: []*ethpb.ValidatorBalances_Balance{
					{Index: 3, PublicKey: pubKey(3), Balance: 3, Status: "EXITED"},
					{Index: 4, PublicKey: pubKey(4), Balance: 4, Status: "EXITED"},
				},
				NextPageToken: "",
				TotalSize:     2,
			}},
	}
	for _, test := range tests {
		res, err := bs.ListValidatorBalances(context.Background(), test.req)
		require.NoError(t, err)
		if !proto.Equal(res, test.res) {
			t.Errorf("Expected %v, received %v", test.res, res)
		}
	}
}

func TestServer_ListValidatorBalances_Pagination_CustomPageSizes(t *testing.T) {
	beaconDB := dbTest.SetupDB(t)
	ctx := context.Background()

	count := 1000
	_, _, headState := setupValidators(t, beaconDB, count)
	b := testutil.NewBeaconBlock()
	gRoot, err := b.Block.HashTreeRoot()
	require.NoError(t, err)
	require.NoError(t, beaconDB.SaveGenesisBlockRoot(ctx, gRoot))
	require.NoError(t, beaconDB.SaveState(ctx, headState, gRoot))

	bs := &Server{
		GenesisTimeFetcher: &mock.ChainService{},
		StateGen:           stategen.New(beaconDB),
		HeadFetcher: &mock.ChainService{
			State: headState,
		},
	}

	tests := []struct {
		req *ethpb.ListValidatorBalancesRequest
		res *ethpb.ValidatorBalances
	}{
		{req: &ethpb.ListValidatorBalancesRequest{PageToken: strconv.Itoa(1), PageSize: 3, QueryFilter: &ethpb.ListValidatorBalancesRequest_Epoch{Epoch: 0}},
			res: &ethpb.ValidatorBalances{
				Balances: []*ethpb.ValidatorBalances_Balance{
					{PublicKey: pubKey(3), Index: 3, Balance: uint64(3), Status: "EXITED"},
					{PublicKey: pubKey(4), Index: 4, Balance: uint64(4), Status: "EXITED"},
					{PublicKey: pubKey(5), Index: 5, Balance: uint64(5), Status: "EXITED"}},
				NextPageToken: strconv.Itoa(2),
				TotalSize:     int32(count)}},
		{req: &ethpb.ListValidatorBalancesRequest{PageToken: strconv.Itoa(10), PageSize: 5, QueryFilter: &ethpb.ListValidatorBalancesRequest_Epoch{Epoch: 0}},
			res: &ethpb.ValidatorBalances{
				Balances: []*ethpb.ValidatorBalances_Balance{
					{PublicKey: pubKey(50), Index: 50, Balance: uint64(50), Status: "EXITED"},
					{PublicKey: pubKey(51), Index: 51, Balance: uint64(51), Status: "EXITED"},
					{PublicKey: pubKey(52), Index: 52, Balance: uint64(52), Status: "EXITED"},
					{PublicKey: pubKey(53), Index: 53, Balance: uint64(53), Status: "EXITED"},
					{PublicKey: pubKey(54), Index: 54, Balance: uint64(54), Status: "EXITED"}},
				NextPageToken: strconv.Itoa(11),
				TotalSize:     int32(count)}},
		{req: &ethpb.ListValidatorBalancesRequest{PageToken: strconv.Itoa(33), PageSize: 3, QueryFilter: &ethpb.ListValidatorBalancesRequest_Epoch{Epoch: 0}},
			res: &ethpb.ValidatorBalances{
				Balances: []*ethpb.ValidatorBalances_Balance{
					{PublicKey: pubKey(99), Index: 99, Balance: uint64(99), Status: "EXITED"},
					{PublicKey: pubKey(100), Index: 100, Balance: uint64(100), Status: "EXITED"},
					{PublicKey: pubKey(101), Index: 101, Balance: uint64(101), Status: "EXITED"},
				},
				NextPageToken: "34",
				TotalSize:     int32(count)}},
		{req: &ethpb.ListValidatorBalancesRequest{PageSize: 2, QueryFilter: &ethpb.ListValidatorBalancesRequest_Epoch{Epoch: 0}},
			res: &ethpb.ValidatorBalances{
				Balances: []*ethpb.ValidatorBalances_Balance{
					{PublicKey: pubKey(0), Index: 0, Balance: uint64(0), Status: "EXITED"},
					{PublicKey: pubKey(1), Index: 1, Balance: uint64(1), Status: "EXITED"}},
				NextPageToken: strconv.Itoa(1),
				TotalSize:     int32(count)}},
	}
	for _, test := range tests {
		res, err := bs.ListValidatorBalances(context.Background(), test.req)
		require.NoError(t, err)
		if !proto.Equal(res, test.res) {
			t.Errorf("Expected %v, received %v", test.res, res)
		}
	}
}

func TestServer_ListValidatorBalances_OutOfRange(t *testing.T) {
	beaconDB := dbTest.SetupDB(t)

	ctx := context.Background()
	_, _, headState := setupValidators(t, beaconDB, 1)
	b := testutil.NewBeaconBlock()
	gRoot, err := b.Block.HashTreeRoot()
	require.NoError(t, err)
	require.NoError(t, beaconDB.SaveGenesisBlockRoot(ctx, gRoot))
	require.NoError(t, beaconDB.SaveState(ctx, headState, gRoot))

	bs := &Server{
		GenesisTimeFetcher: &mock.ChainService{},
		StateGen:           stategen.New(beaconDB),
		HeadFetcher: &mock.ChainService{
			State: headState,
		},
	}

	req := &ethpb.ListValidatorBalancesRequest{Indices: []types.ValidatorIndex{types.ValidatorIndex(1)}, QueryFilter: &ethpb.ListValidatorBalancesRequest_Epoch{Epoch: 0}}
	wanted := "Validator index 1 >= balance list 1"
	_, err = bs.ListValidatorBalances(context.Background(), req)
	assert.ErrorContains(t, wanted, err)
}

func TestServer_ListValidators_CannotRequestFutureEpoch(t *testing.T) {
	beaconDB := dbTest.SetupDB(t)
	ctx := context.Background()

	st, err := testutil.NewBeaconState()
	require.NoError(t, err)
	require.NoError(t, st.SetSlot(0))
	bs := &Server{
		BeaconDB: beaconDB,
		GenesisTimeFetcher: &mock.ChainService{
			// We are in epoch 0.
			Genesis: time.Now(),
		},
		HeadFetcher: &mock.ChainService{
			State: st,
		},
	}

	wanted := errNoEpochInfoError
	_, err = bs.ListValidators(
		ctx,
		&ethpb.ListValidatorsRequest{
			QueryFilter: &ethpb.ListValidatorsRequest_Epoch{
				Epoch: 1,
			},
		},
	)
	assert.ErrorContains(t, wanted, err)
}

func TestServer_ListValidators_NoResults(t *testing.T) {
	beaconDB := dbTest.SetupDB(t)

	ctx := context.Background()
	st, err := testutil.NewBeaconState()
	require.NoError(t, err)
	require.NoError(t, st.SetSlot(0))
	gRoot := [32]byte{'g'}
	require.NoError(t, beaconDB.SaveGenesisBlockRoot(ctx, gRoot))
	require.NoError(t, beaconDB.SaveState(ctx, st, gRoot))
	bs := &Server{
		BeaconDB: beaconDB,
		GenesisTimeFetcher: &mock.ChainService{
			// We are in epoch 0.
			Genesis: time.Now(),
		},
		HeadFetcher: &mock.ChainService{
			State: st,
		},
		StateGen: stategen.New(beaconDB),
	}
	wanted := &ethpb.Validators{
		ValidatorList: make([]*ethpb.Validators_ValidatorContainer, 0),
		TotalSize:     int32(0),
		NextPageToken: strconv.Itoa(0),
	}
	res, err := bs.ListValidators(
		ctx,
		&ethpb.ListValidatorsRequest{
			QueryFilter: &ethpb.ListValidatorsRequest_Epoch{
				Epoch: 0,
			},
		},
	)
	require.NoError(t, err)
	if !proto.Equal(wanted, res) {
		t.Errorf("Wanted %v, received %v", wanted, res)
	}
}

func TestServer_ListValidators_OnlyActiveValidators(t *testing.T) {
	ctx := context.Background()
	beaconDB := dbTest.SetupDB(t)
	count := 100
	balances := make([]uint64, count)
	validators := make([]*ethpb.Validator, count)
	activeValidators := make([]*ethpb.Validators_ValidatorContainer, 0)
	for i := 0; i < count; i++ {
		pubKey := pubKey(uint64(i))
		balances[i] = params.BeaconConfig().MaxEffectiveBalance

		// We mark even validators as active, and odd validators as inactive.
		if i%2 == 0 {
			val := &ethpb.Validator{
				PublicKey:             pubKey,
				WithdrawalCredentials: make([]byte, 32),
				ActivationEpoch:       0,
				ExitEpoch:             params.BeaconConfig().FarFutureEpoch,
			}
			validators[i] = val
			activeValidators = append(activeValidators, &ethpb.Validators_ValidatorContainer{
				Index:     types.ValidatorIndex(i),
				Validator: val,
			})
		} else {
			validators[i] = &ethpb.Validator{
				PublicKey:             pubKey,
				WithdrawalCredentials: make([]byte, 32),
				ActivationEpoch:       0,
				ExitEpoch:             0,
			}
		}
	}
	st, err := testutil.NewBeaconState()
	require.NoError(t, err)
	require.NoError(t, st.SetValidators(validators))
	require.NoError(t, st.SetBalances(balances))

	bs := &Server{
		HeadFetcher: &mock.ChainService{
			State: st,
		},
		GenesisTimeFetcher: &mock.ChainService{
			// We are in epoch 0.
			Genesis: time.Now(),
		},
		StateGen: stategen.New(beaconDB),
	}

	b := testutil.NewBeaconBlock()
	require.NoError(t, beaconDB.SaveBlock(ctx, wrapper.WrappedPhase0SignedBeaconBlock(b)))
	gRoot, err := b.Block.HashTreeRoot()
	require.NoError(t, err)
	require.NoError(t, beaconDB.SaveGenesisBlockRoot(ctx, gRoot))
	require.NoError(t, beaconDB.SaveState(ctx, st, gRoot))

	received, err := bs.ListValidators(ctx, &ethpb.ListValidatorsRequest{
		Active: true,
	})
	require.NoError(t, err)
	assert.DeepEqual(t, activeValidators, received.ValidatorList)
}

func TestServer_ListValidators_InactiveInTheMiddle(t *testing.T) {
	ctx := context.Background()
	beaconDB := dbTest.SetupDB(t)
	count := 100
	balances := make([]uint64, count)
	validators := make([]*ethpb.Validator, count)
	activeValidators := make([]*ethpb.Validators_ValidatorContainer, 0)
	for i := 0; i < count; i++ {
		pubKey := pubKey(uint64(i))
		balances[i] = params.BeaconConfig().MaxEffectiveBalance

		// We mark even validators as active, and odd validators as inactive.
		if i%2 == 0 {
			val := &ethpb.Validator{
				PublicKey:             pubKey,
				WithdrawalCredentials: make([]byte, 32),
				ActivationEpoch:       0,
				ExitEpoch:             params.BeaconConfig().FarFutureEpoch,
			}
			validators[i] = val
			activeValidators = append(activeValidators, &ethpb.Validators_ValidatorContainer{
				Index:     types.ValidatorIndex(i),
				Validator: val,
			})
		} else {
			validators[i] = &ethpb.Validator{
				PublicKey:             pubKey,
				WithdrawalCredentials: make([]byte, 32),
				ActivationEpoch:       0,
				ExitEpoch:             0,
			}
		}
	}

	// Set first validator to be inactive.
	validators[0].ActivationEpoch = params.BeaconConfig().FarFutureEpoch
	activeValidators[0].Validator.ActivationEpoch = params.BeaconConfig().FarFutureEpoch

	st, err := testutil.NewBeaconState()
	require.NoError(t, err)
	require.NoError(t, st.SetValidators(validators))
	require.NoError(t, st.SetBalances(balances))

	bs := &Server{
		HeadFetcher: &mock.ChainService{
			State: st,
		},
		GenesisTimeFetcher: &mock.ChainService{
			// We are in epoch 0.
			Genesis: time.Now(),
		},
		StateGen: stategen.New(beaconDB),
	}

	b := testutil.NewBeaconBlock()
	require.NoError(t, beaconDB.SaveBlock(ctx, wrapper.WrappedPhase0SignedBeaconBlock(b)))
	gRoot, err := b.Block.HashTreeRoot()
	require.NoError(t, err)
	require.NoError(t, beaconDB.SaveGenesisBlockRoot(ctx, gRoot))
	require.NoError(t, beaconDB.SaveState(ctx, st, gRoot))

	received, err := bs.ListValidators(ctx, &ethpb.ListValidatorsRequest{
		Active: true,
	})
	require.NoError(t, err)

	require.Equal(t, count/2-1, len(received.ValidatorList))
	require.Equal(t, count/2-1, int(received.TotalSize))
}

func TestServer_ListValidatorBalances_UnknownValidatorInResponse(t *testing.T) {
	beaconDB := dbTest.SetupDB(t)
	ctx := context.Background()

	_, _, headState := setupValidators(t, beaconDB, 4)
	b := testutil.NewBeaconBlock()
	gRoot, err := b.Block.HashTreeRoot()
	require.NoError(t, err)
	require.NoError(t, beaconDB.SaveGenesisBlockRoot(ctx, gRoot))
	require.NoError(t, beaconDB.SaveState(ctx, headState, gRoot))

	bs := &Server{
		GenesisTimeFetcher: &mock.ChainService{},
		StateGen:           stategen.New(beaconDB),
		HeadFetcher: &mock.ChainService{
			State: headState,
		},
	}

	nonExistentPubKey := [32]byte{8}
	req := &ethpb.ListValidatorBalancesRequest{
		PublicKeys: [][]byte{
			pubKey(1),
			pubKey(2),
			nonExistentPubKey[:],
		},
		QueryFilter: &ethpb.ListValidatorBalancesRequest_Epoch{Epoch: 0},
	}

	wanted := &ethpb.ValidatorBalances{
		Balances: []*ethpb.ValidatorBalances_Balance{
			{Status: "UNKNOWN"},
			{Index: 1, PublicKey: pubKey(1), Balance: 1, Status: "EXITED"},
			{Index: 2, PublicKey: pubKey(2), Balance: 2, Status: "EXITED"},
		},
		NextPageToken: "",
		TotalSize:     3,
	}
	res, err := bs.ListValidatorBalances(context.Background(), req)
	require.NoError(t, err)
	if !proto.Equal(res, wanted) {
		t.Errorf("Expected %v, received %v", wanted, res)
	}
}

func TestServer_ListValidators_NoPagination(t *testing.T) {
	beaconDB := dbTest.SetupDB(t)

	validators, _, headState := setupValidators(t, beaconDB, 100)
	want := make([]*ethpb.Validators_ValidatorContainer, len(validators))
	for i := 0; i < len(validators); i++ {
		want[i] = &ethpb.Validators_ValidatorContainer{
			Index:     types.ValidatorIndex(i),
			Validator: validators[i],
		}
	}

	bs := &Server{
		HeadFetcher: &mock.ChainService{
			State: headState,
		},
		GenesisTimeFetcher: &mock.ChainService{
			// We are in epoch 0.
			Genesis: time.Now(),
		},
		FinalizationFetcher: &mock.ChainService{
			FinalizedCheckPoint: &ethpb.Checkpoint{
				Epoch: 0,
			},
		},
		StateGen: stategen.New(beaconDB),
	}

	received, err := bs.ListValidators(context.Background(), &ethpb.ListValidatorsRequest{})
	require.NoError(t, err)
	assert.DeepSSZEqual(t, want, received.ValidatorList, "Incorrect respond of validators")
}

func TestServer_ListValidators_StategenNotUsed(t *testing.T) {
	beaconDB := dbTest.SetupDB(t)

	validators, _, headState := setupValidators(t, beaconDB, 100)
	want := make([]*ethpb.Validators_ValidatorContainer, len(validators))
	for i := 0; i < len(validators); i++ {
		want[i] = &ethpb.Validators_ValidatorContainer{
			Index:     types.ValidatorIndex(i),
			Validator: validators[i],
		}
	}

	bs := &Server{
		HeadFetcher: &mock.ChainService{
			State: headState,
		},
		GenesisTimeFetcher: &mock.ChainService{
			// We are in epoch 0.
			Genesis: time.Now(),
		},
	}

	received, err := bs.ListValidators(context.Background(), &ethpb.ListValidatorsRequest{})
	require.NoError(t, err)
	assert.DeepEqual(t, want, received.ValidatorList, "Incorrect respond of validators")
}

func TestServer_ListValidators_IndicesPubKeys(t *testing.T) {
	beaconDB := dbTest.SetupDB(t)

	validators, _, headState := setupValidators(t, beaconDB, 100)
	indicesWanted := []types.ValidatorIndex{2, 7, 11, 17}
	pubkeyIndicesWanted := []types.ValidatorIndex{3, 5, 9, 15}
	allIndicesWanted := append(indicesWanted, pubkeyIndicesWanted...)
	want := make([]*ethpb.Validators_ValidatorContainer, len(allIndicesWanted))
	for i, idx := range allIndicesWanted {
		want[i] = &ethpb.Validators_ValidatorContainer{
			Index:     idx,
			Validator: validators[idx],
		}
	}
	sort.Slice(want, func(i int, j int) bool {
		return want[i].Index < want[j].Index
	})

	bs := &Server{
		HeadFetcher: &mock.ChainService{
			State: headState,
		},
		FinalizationFetcher: &mock.ChainService{
			FinalizedCheckPoint: &ethpb.Checkpoint{
				Epoch: 0,
			},
		},
		GenesisTimeFetcher: &mock.ChainService{
			// We are in epoch 0.
			Genesis: time.Now(),
		},
		StateGen: stategen.New(beaconDB),
	}

	pubKeysWanted := make([][]byte, len(pubkeyIndicesWanted))
	for i, indice := range pubkeyIndicesWanted {
		pubKeysWanted[i] = pubKey(uint64(indice))
	}
	req := &ethpb.ListValidatorsRequest{
		Indices:    indicesWanted,
		PublicKeys: pubKeysWanted,
	}
	received, err := bs.ListValidators(context.Background(), req)
	require.NoError(t, err)
	assert.DeepEqual(t, want, received.ValidatorList, "Incorrect respond of validators")
}

func TestServer_ListValidators_Pagination(t *testing.T) {
	beaconDB := dbTest.SetupDB(t)

	count := 100
	_, _, headState := setupValidators(t, beaconDB, count)

	bs := &Server{
		BeaconDB: beaconDB,
		HeadFetcher: &mock.ChainService{
			State: headState,
		},
		FinalizationFetcher: &mock.ChainService{
			FinalizedCheckPoint: &ethpb.Checkpoint{
				Epoch: 0,
			},
		},
		GenesisTimeFetcher: &mock.ChainService{
			// We are in epoch 0.
			Genesis: time.Now(),
		},
		StateGen: stategen.New(beaconDB),
	}

	tests := []struct {
		req *ethpb.ListValidatorsRequest
		res *ethpb.Validators
	}{
		{req: &ethpb.ListValidatorsRequest{PageToken: strconv.Itoa(1), PageSize: 3},
			res: &ethpb.Validators{
				ValidatorList: []*ethpb.Validators_ValidatorContainer{
					{
						Validator: &ethpb.Validator{
							PublicKey:             pubKey(3),
							WithdrawalCredentials: make([]byte, 32),
						},
						Index: 3,
					},
					{
						Validator: &ethpb.Validator{
							PublicKey:             pubKey(4),
							WithdrawalCredentials: make([]byte, 32),
						},
						Index: 4,
					},
					{
						Validator: &ethpb.Validator{
							PublicKey:             pubKey(5),
							WithdrawalCredentials: make([]byte, 32),
						},
						Index: 5,
					},
				},
				NextPageToken: strconv.Itoa(2),
				TotalSize:     int32(count)}},
		{req: &ethpb.ListValidatorsRequest{PageToken: strconv.Itoa(10), PageSize: 5},
			res: &ethpb.Validators{
				ValidatorList: []*ethpb.Validators_ValidatorContainer{
					{
						Validator: &ethpb.Validator{
							PublicKey:             pubKey(50),
							WithdrawalCredentials: make([]byte, 32),
						},
						Index: 50,
					},
					{
						Validator: &ethpb.Validator{
							PublicKey:             pubKey(51),
							WithdrawalCredentials: make([]byte, 32),
						},
						Index: 51,
					},
					{
						Validator: &ethpb.Validator{
							PublicKey:             pubKey(52),
							WithdrawalCredentials: make([]byte, 32),
						},
						Index: 52,
					},
					{
						Validator: &ethpb.Validator{
							PublicKey:             pubKey(53),
							WithdrawalCredentials: make([]byte, 32),
						},
						Index: 53,
					},
					{
						Validator: &ethpb.Validator{
							PublicKey:             pubKey(54),
							WithdrawalCredentials: make([]byte, 32),
						},
						Index: 54,
					},
				},
				NextPageToken: strconv.Itoa(11),
				TotalSize:     int32(count)}},
		{req: &ethpb.ListValidatorsRequest{PageToken: strconv.Itoa(33), PageSize: 3},
			res: &ethpb.Validators{
				ValidatorList: []*ethpb.Validators_ValidatorContainer{
					{
						Validator: &ethpb.Validator{
							PublicKey:             pubKey(99),
							WithdrawalCredentials: make([]byte, 32),
						},
						Index: 99,
					},
				},
				NextPageToken: "",
				TotalSize:     int32(count)}},
		{req: &ethpb.ListValidatorsRequest{PageSize: 2},
			res: &ethpb.Validators{
				ValidatorList: []*ethpb.Validators_ValidatorContainer{
					{
						Validator: &ethpb.Validator{
							PublicKey:             pubKey(0),
							WithdrawalCredentials: make([]byte, 32),
						},
						Index: 0,
					},
					{
						Validator: &ethpb.Validator{
							PublicKey:             pubKey(1),
							WithdrawalCredentials: make([]byte, 32),
						},
						Index: 1,
					},
				},
				NextPageToken: strconv.Itoa(1),
				TotalSize:     int32(count)}},
	}
	for _, test := range tests {
		res, err := bs.ListValidators(context.Background(), test.req)
		require.NoError(t, err)
		if !proto.Equal(res, test.res) {
			t.Errorf("Incorrect validator response, wanted %v, received %v", test.res, res)
		}
	}
}

func TestServer_ListValidators_PaginationOutOfRange(t *testing.T) {
	beaconDB := dbTest.SetupDB(t)

	count := 1
	validators, _, headState := setupValidators(t, beaconDB, count)

	bs := &Server{
		HeadFetcher: &mock.ChainService{
			State: headState,
		},
		FinalizationFetcher: &mock.ChainService{
			FinalizedCheckPoint: &ethpb.Checkpoint{
				Epoch: 0,
			},
		},
		GenesisTimeFetcher: &mock.ChainService{
			// We are in epoch 0.
			Genesis: time.Now(),
		},
		StateGen: stategen.New(beaconDB),
	}

	req := &ethpb.ListValidatorsRequest{PageToken: strconv.Itoa(1), PageSize: 100}
	wanted := fmt.Sprintf("page start %d >= list %d", req.PageSize, len(validators))
	_, err := bs.ListValidators(context.Background(), req)
	assert.ErrorContains(t, wanted, err)
}

func TestServer_ListValidators_ExceedsMaxPageSize(t *testing.T) {
	bs := &Server{}
	exceedsMax := int32(cmd.Get().MaxRPCPageSize + 1)

	wanted := fmt.Sprintf("Requested page size %d can not be greater than max size %d", exceedsMax, cmd.Get().MaxRPCPageSize)
	req := &ethpb.ListValidatorsRequest{PageToken: strconv.Itoa(0), PageSize: exceedsMax}
	_, err := bs.ListValidators(context.Background(), req)
	assert.ErrorContains(t, wanted, err)
}

func TestServer_ListValidators_DefaultPageSize(t *testing.T) {
	beaconDB := dbTest.SetupDB(t)

	validators, _, headState := setupValidators(t, beaconDB, 1000)
	want := make([]*ethpb.Validators_ValidatorContainer, len(validators))
	for i := 0; i < len(validators); i++ {
		want[i] = &ethpb.Validators_ValidatorContainer{
			Index:     types.ValidatorIndex(i),
			Validator: validators[i],
		}
	}

	bs := &Server{
		HeadFetcher: &mock.ChainService{
			State: headState,
		},
		FinalizationFetcher: &mock.ChainService{
			FinalizedCheckPoint: &ethpb.Checkpoint{
				Epoch: 0,
			},
		},
		GenesisTimeFetcher: &mock.ChainService{
			// We are in epoch 0.
			Genesis: time.Now(),
		},
		StateGen: stategen.New(beaconDB),
	}

	req := &ethpb.ListValidatorsRequest{}
	res, err := bs.ListValidators(context.Background(), req)
	require.NoError(t, err)

	i := 0
	j := params.BeaconConfig().DefaultPageSize
	assert.DeepEqual(t, want[i:j], res.ValidatorList, "Incorrect respond of validators")
}

func TestServer_ListValidators_FromOldEpoch(t *testing.T) {
	beaconDB := dbTest.SetupDB(t)
	ctx := context.Background()

	numEpochs := types.Epoch(30)
	validators := make([]*ethpb.Validator, numEpochs)
	for i := types.Epoch(0); i < numEpochs; i++ {
		validators[i] = &ethpb.Validator{
			ActivationEpoch:       i,
			PublicKey:             make([]byte, 48),
			WithdrawalCredentials: make([]byte, 32),
		}
	}
	want := make([]*ethpb.Validators_ValidatorContainer, len(validators))
	for i := 0; i < len(validators); i++ {
		want[i] = &ethpb.Validators_ValidatorContainer{
			Index:     types.ValidatorIndex(i),
			Validator: validators[i],
		}
	}

	st, err := testutil.NewBeaconState()
	require.NoError(t, err)
	require.NoError(t, st.SetSlot(20*params.BeaconConfig().SlotsPerEpoch))
	require.NoError(t, st.SetValidators(validators))
	b := testutil.NewBeaconBlock()
	require.NoError(t, beaconDB.SaveBlock(ctx, wrapper.WrappedPhase0SignedBeaconBlock(b)))
	gRoot, err := b.Block.HashTreeRoot()
	require.NoError(t, err)
	require.NoError(t, beaconDB.SaveState(ctx, st, gRoot))
	require.NoError(t, beaconDB.SaveGenesisBlockRoot(ctx, gRoot))
	secondsPerEpoch := params.BeaconConfig().SecondsPerSlot * uint64(params.BeaconConfig().SlotsPerEpoch)
	bs := &Server{
		HeadFetcher: &mock.ChainService{
			State: st,
		},
		GenesisTimeFetcher: &mock.ChainService{
			// We are in epoch 30
			Genesis: time.Now().Add(time.Duration(-1*int64(30*secondsPerEpoch)) * time.Second),
		},
		StateGen: stategen.New(beaconDB),
	}

	req := &ethpb.ListValidatorsRequest{
		QueryFilter: &ethpb.ListValidatorsRequest_Genesis{
			Genesis: true,
		},
	}
	res, err := bs.ListValidators(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, 30, len(res.ValidatorList))

	req = &ethpb.ListValidatorsRequest{
		QueryFilter: &ethpb.ListValidatorsRequest_Epoch{
			Epoch: 20,
		},
	}
	res, err = bs.ListValidators(context.Background(), req)
	require.NoError(t, err)
	assert.DeepEqual(t, want, res.ValidatorList, "Incorrect number of validators")
}

func TestServer_ListValidators_ProcessHeadStateSlots(t *testing.T) {
	params.UseMinimalConfig()

	beaconDB := dbTest.SetupDB(t)
	ctx := context.Background()

	headSlot := types.Slot(32)
	numValidators := params.BeaconConfig().MinGenesisActiveValidatorCount
	validators := make([]*ethpb.Validator, numValidators)
	balances := make([]uint64, numValidators)
	for i := uint64(0); i < numValidators; i++ {
		validators[i] = &ethpb.Validator{
			ActivationEpoch:       0,
			PublicKey:             make([]byte, 48),
			WithdrawalCredentials: make([]byte, 32),
			EffectiveBalance:      params.BeaconConfig().MaxEffectiveBalance,
		}
		balances[i] = params.BeaconConfig().MaxEffectiveBalance
	}
	want := make([]*ethpb.Validators_ValidatorContainer, len(validators))
	for i := 0; i < len(validators); i++ {
		want[i] = &ethpb.Validators_ValidatorContainer{
			Index:     types.ValidatorIndex(i),
			Validator: validators[i],
		}
	}

	st, err := testutil.NewBeaconState()
	require.NoError(t, err)
	require.NoError(t, st.SetSlot(headSlot))
	require.NoError(t, st.SetValidators(validators))
	require.NoError(t, st.SetBalances(balances))
	b := testutil.NewBeaconBlock()
	require.NoError(t, beaconDB.SaveBlock(ctx, wrapper.WrappedPhase0SignedBeaconBlock(b)))
	gRoot, err := b.Block.HashTreeRoot()
	require.NoError(t, err)
	require.NoError(t, beaconDB.SaveState(ctx, st, gRoot))
	require.NoError(t, beaconDB.SaveGenesisBlockRoot(ctx, gRoot))
	secondsPerEpoch := params.BeaconConfig().SecondsPerSlot * uint64(params.BeaconConfig().SlotsPerEpoch)
	bs := &Server{
		HeadFetcher: &mock.ChainService{
			State: st,
		},
		GenesisTimeFetcher: &mock.ChainService{
			Genesis: time.Now().Add(time.Duration(-1*int64(secondsPerEpoch)) * time.Second),
		},
		StateGen: stategen.New(beaconDB),
	}

	req := &ethpb.ListValidatorsRequest{
		QueryFilter: &ethpb.ListValidatorsRequest_Epoch{
			Epoch: 1,
		},
	}
	res, err := bs.ListValidators(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, len(want), len(res.ValidatorList), "Incorrect number of validators")
	for i := 0; i < len(res.ValidatorList); i++ {
		assert.DeepEqual(t, want[i], res.ValidatorList[i])
	}
}

func TestServer_GetValidator(t *testing.T) {
	count := types.Epoch(30)
	validators := make([]*ethpb.Validator, count)
	for i := types.Epoch(0); i < count; i++ {
		validators[i] = &ethpb.Validator{
			ActivationEpoch:       i,
			PublicKey:             pubKey(uint64(i)),
			WithdrawalCredentials: make([]byte, 32),
		}
	}

	st, err := testutil.NewBeaconState()
	require.NoError(t, err)
	require.NoError(t, st.SetValidators(validators))

	bs := &Server{
		HeadFetcher: &mock.ChainService{
			State: st,
		},
	}

	tests := []struct {
		req       *ethpb.GetValidatorRequest
		res       *ethpb.Validator
		wantedErr string
	}{
		{
			req: &ethpb.GetValidatorRequest{
				QueryFilter: &ethpb.GetValidatorRequest_Index{
					Index: 0,
				},
			},
			res: validators[0],
		},
		{
			req: &ethpb.GetValidatorRequest{
				QueryFilter: &ethpb.GetValidatorRequest_Index{
					Index: types.ValidatorIndex(count - 1),
				},
			},
			res: validators[count-1],
		},
		{
			req: &ethpb.GetValidatorRequest{
				QueryFilter: &ethpb.GetValidatorRequest_PublicKey{
					PublicKey: pubKey(5),
				},
			},
			res: validators[5],
		},
		{
			req: &ethpb.GetValidatorRequest{
				QueryFilter: &ethpb.GetValidatorRequest_PublicKey{
					PublicKey: []byte("bad-keyxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"),
				},
			},
			res:       nil,
			wantedErr: "No validator matched filter criteria",
		},
		{
			req: &ethpb.GetValidatorRequest{
				QueryFilter: &ethpb.GetValidatorRequest_Index{
					Index: types.ValidatorIndex(len(validators)),
				},
			},
			res:       nil,
			wantedErr: fmt.Sprintf("there are only %d validators", len(validators)),
		},
	}

	for _, test := range tests {
		res, err := bs.GetValidator(context.Background(), test.req)
		if test.wantedErr != "" {
			require.ErrorContains(t, test.wantedErr, err)
		} else {
			require.NoError(t, err)
		}
		assert.DeepEqual(t, test.res, res)
	}
}

func TestServer_GetValidatorActiveSetChanges(t *testing.T) {
	beaconDB := dbTest.SetupDB(t)

	ctx := context.Background()
	validators := make([]*ethpb.Validator, 8)
	headState, err := testutil.NewBeaconState()
	require.NoError(t, err)
	require.NoError(t, headState.SetSlot(0))
	require.NoError(t, headState.SetValidators(validators))
	for i := 0; i < len(validators); i++ {
		activationEpoch := params.BeaconConfig().FarFutureEpoch
		withdrawableEpoch := params.BeaconConfig().FarFutureEpoch
		exitEpoch := params.BeaconConfig().FarFutureEpoch
		slashed := false
		balance := params.BeaconConfig().MaxEffectiveBalance
		// Mark indices divisible by two as activated.
		if i%2 == 0 {
			activationEpoch = 0
		} else if i%3 == 0 {
			// Mark indices divisible by 3 as slashed.
			withdrawableEpoch = params.BeaconConfig().EpochsPerSlashingsVector
			slashed = true
		} else if i%5 == 0 {
			// Mark indices divisible by 5 as exited.
			exitEpoch = 0
			withdrawableEpoch = params.BeaconConfig().MinValidatorWithdrawabilityDelay
		} else if i%7 == 0 {
			// Mark indices divisible by 7 as ejected.
			exitEpoch = 0
			withdrawableEpoch = params.BeaconConfig().MinValidatorWithdrawabilityDelay
			balance = params.BeaconConfig().EjectionBalance
		}
		err := headState.UpdateValidatorAtIndex(types.ValidatorIndex(i), &ethpb.Validator{
			ActivationEpoch:       activationEpoch,
			PublicKey:             pubKey(uint64(i)),
			EffectiveBalance:      balance,
			WithdrawalCredentials: make([]byte, 32),
			WithdrawableEpoch:     withdrawableEpoch,
			Slashed:               slashed,
			ExitEpoch:             exitEpoch,
		})
		require.NoError(t, err)
	}
	b := testutil.NewBeaconBlock()
	require.NoError(t, beaconDB.SaveBlock(ctx, wrapper.WrappedPhase0SignedBeaconBlock(b)))

	gRoot, err := b.Block.HashTreeRoot()
	require.NoError(t, err)
	require.NoError(t, beaconDB.SaveGenesisBlockRoot(ctx, gRoot))
	require.NoError(t, beaconDB.SaveState(ctx, headState, gRoot))

	bs := &Server{
		FinalizationFetcher: &mock.ChainService{
			FinalizedCheckPoint: &ethpb.Checkpoint{Epoch: 0, Root: make([]byte, 32)},
		},
		GenesisTimeFetcher: &mock.ChainService{},
		StateGen:           stategen.New(beaconDB),
	}
	res, err := bs.GetValidatorActiveSetChanges(ctx, &ethpb.GetValidatorActiveSetChangesRequest{
		QueryFilter: &ethpb.GetValidatorActiveSetChangesRequest_Genesis{Genesis: true},
	})
	require.NoError(t, err)
	wantedActive := [][]byte{
		pubKey(0),
		pubKey(2),
		pubKey(4),
		pubKey(6),
	}
	wantedActiveIndices := []types.ValidatorIndex{0, 2, 4, 6}
	wantedExited := [][]byte{
		pubKey(5),
	}
	wantedExitedIndices := []types.ValidatorIndex{5}
	wantedSlashed := [][]byte{
		pubKey(3),
	}
	wantedSlashedIndices := []types.ValidatorIndex{3}
	wantedEjected := [][]byte{
		pubKey(7),
	}
	wantedEjectedIndices := []types.ValidatorIndex{7}
	wanted := &ethpb.ActiveSetChanges{
		Epoch:               0,
		ActivatedPublicKeys: wantedActive,
		ActivatedIndices:    wantedActiveIndices,
		ExitedPublicKeys:    wantedExited,
		ExitedIndices:       wantedExitedIndices,
		SlashedPublicKeys:   wantedSlashed,
		SlashedIndices:      wantedSlashedIndices,
		EjectedPublicKeys:   wantedEjected,
		EjectedIndices:      wantedEjectedIndices,
	}
	if !proto.Equal(wanted, res) {
		t.Errorf("Wanted \n%v, received \n%v", wanted, res)
	}
}

func TestServer_GetValidatorQueue_PendingActivation(t *testing.T) {
	headState, err := v1.InitializeFromProto(&pb.BeaconState{
		Validators: []*ethpb.Validator{
			{
				ActivationEpoch:            helpers.ActivationExitEpoch(0),
				ActivationEligibilityEpoch: 3,
				PublicKey:                  pubKey(3),
				WithdrawalCredentials:      make([]byte, 32),
			},
			{
				ActivationEpoch:            helpers.ActivationExitEpoch(0),
				ActivationEligibilityEpoch: 2,
				PublicKey:                  pubKey(2),
				WithdrawalCredentials:      make([]byte, 32),
			},
			{
				ActivationEpoch:            helpers.ActivationExitEpoch(0),
				ActivationEligibilityEpoch: 1,
				PublicKey:                  pubKey(1),
				WithdrawalCredentials:      make([]byte, 32),
			},
		},
		FinalizedCheckpoint: &ethpb.Checkpoint{
			Epoch: 0,
		},
	})
	require.NoError(t, err)
	bs := &Server{
		HeadFetcher: &mock.ChainService{
			State: headState,
		},
	}
	res, err := bs.GetValidatorQueue(context.Background(), &emptypb.Empty{})
	require.NoError(t, err)
	// We verify the keys are properly sorted by the validators' activation eligibility epoch.
	wanted := [][]byte{
		pubKey(1),
		pubKey(2),
		pubKey(3),
	}
	activeValidatorCount, err := helpers.ActiveValidatorCount(headState, helpers.CurrentEpoch(headState))
	require.NoError(t, err)
	wantChurn, err := helpers.ValidatorChurnLimit(activeValidatorCount)
	require.NoError(t, err)
	assert.Equal(t, wantChurn, res.ChurnLimit)
	assert.DeepEqual(t, wanted, res.ActivationPublicKeys)
	wantedActiveIndices := []types.ValidatorIndex{2, 1, 0}
	assert.DeepEqual(t, wantedActiveIndices, res.ActivationValidatorIndices)
}

func TestServer_GetValidatorQueue_ExitedValidatorLeavesQueue(t *testing.T) {
	validators := []*ethpb.Validator{
		{
			ActivationEpoch:   0,
			ExitEpoch:         params.BeaconConfig().FarFutureEpoch,
			WithdrawableEpoch: params.BeaconConfig().FarFutureEpoch,
			PublicKey:         bytesutil.PadTo([]byte("1"), 48),
		},
		{
			ActivationEpoch:   0,
			ExitEpoch:         4,
			WithdrawableEpoch: 6,
			PublicKey:         bytesutil.PadTo([]byte("2"), 48),
		},
	}

	headState, err := testutil.NewBeaconState()
	require.NoError(t, err)
	require.NoError(t, headState.SetValidators(validators))
	require.NoError(t, headState.SetFinalizedCheckpoint(&ethpb.Checkpoint{Epoch: 0, Root: make([]byte, 32)}))
	bs := &Server{
		HeadFetcher: &mock.ChainService{
			State: headState,
		},
	}

	// First we check if validator with index 1 is in the exit queue.
	res, err := bs.GetValidatorQueue(context.Background(), &emptypb.Empty{})
	require.NoError(t, err)
	wanted := [][]byte{
		bytesutil.PadTo([]byte("2"), 48),
	}
	activeValidatorCount, err := helpers.ActiveValidatorCount(headState, helpers.CurrentEpoch(headState))
	require.NoError(t, err)
	wantChurn, err := helpers.ValidatorChurnLimit(activeValidatorCount)
	require.NoError(t, err)
	assert.Equal(t, wantChurn, res.ChurnLimit)
	assert.DeepEqual(t, wanted, res.ExitPublicKeys)
	wantedExitIndices := []types.ValidatorIndex{1}
	assert.DeepEqual(t, wantedExitIndices, res.ExitValidatorIndices)

	// Now, we move the state.slot past the exit epoch of the validator, and now
	// the validator should no longer exist in the queue.
	require.NoError(t, headState.SetSlot(params.BeaconConfig().SlotsPerEpoch.Mul(uint64(validators[1].ExitEpoch+1))))
	res, err = bs.GetValidatorQueue(context.Background(), &emptypb.Empty{})
	require.NoError(t, err)
	assert.Equal(t, 0, len(res.ExitPublicKeys))
}

func TestServer_GetValidatorQueue_PendingExit(t *testing.T) {
	headState, err := v1.InitializeFromProto(&pb.BeaconState{
		Validators: []*ethpb.Validator{
			{
				ActivationEpoch:       0,
				ExitEpoch:             4,
				WithdrawableEpoch:     3,
				PublicKey:             pubKey(3),
				WithdrawalCredentials: make([]byte, 32),
			},
			{
				ActivationEpoch:       0,
				ExitEpoch:             4,
				WithdrawableEpoch:     2,
				PublicKey:             pubKey(2),
				WithdrawalCredentials: make([]byte, 32),
			},
			{
				ActivationEpoch:       0,
				ExitEpoch:             4,
				WithdrawableEpoch:     1,
				PublicKey:             pubKey(1),
				WithdrawalCredentials: make([]byte, 32),
			},
		},
		FinalizedCheckpoint: &ethpb.Checkpoint{
			Epoch: 0,
		},
	})
	require.NoError(t, err)
	bs := &Server{
		HeadFetcher: &mock.ChainService{
			State: headState,
		},
	}
	res, err := bs.GetValidatorQueue(context.Background(), &emptypb.Empty{})
	require.NoError(t, err)
	// We verify the keys are properly sorted by the validators' withdrawable epoch.
	wanted := [][]byte{
		pubKey(1),
		pubKey(2),
		pubKey(3),
	}
	activeValidatorCount, err := helpers.ActiveValidatorCount(headState, helpers.CurrentEpoch(headState))
	require.NoError(t, err)
	wantChurn, err := helpers.ValidatorChurnLimit(activeValidatorCount)
	require.NoError(t, err)
	assert.Equal(t, wantChurn, res.ChurnLimit)
	assert.DeepEqual(t, wanted, res.ExitPublicKeys)
}

func TestServer_GetValidatorParticipation_CannotRequestFutureEpoch(t *testing.T) {
	beaconDB := dbTest.SetupDB(t)

	ctx := context.Background()
	headState, err := testutil.NewBeaconState()
	require.NoError(t, err)
	require.NoError(t, headState.SetSlot(0))
	bs := &Server{
		BeaconDB: beaconDB,
		HeadFetcher: &mock.ChainService{
			State: headState,
		},
		GenesisTimeFetcher: &mock.ChainService{},
		StateGen:           stategen.New(beaconDB),
	}

	wanted := "Cannot retrieve information about an epoch"
	_, err = bs.GetValidatorParticipation(
		ctx,
		&ethpb.GetValidatorParticipationRequest{
			QueryFilter: &ethpb.GetValidatorParticipationRequest_Epoch{
				Epoch: helpers.SlotToEpoch(bs.GenesisTimeFetcher.CurrentSlot()) + 1,
			},
		},
	)
	assert.ErrorContains(t, wanted, err)
}

func TestServer_GetValidatorParticipation_UnknownState(t *testing.T) {
	beaconDB := dbTest.SetupDB(t)

	ctx := context.Background()
	headState, err := testutil.NewBeaconState()
	require.NoError(t, err)
	require.NoError(t, headState.SetSlot(0))
	epoch := types.Epoch(50)
	slots := params.BeaconConfig().SlotsPerEpoch.Mul(uint64(epoch))
	mockStateGen := &stategen.MockStateManager{
		StatesBySlot: map[types.Slot]iface.BeaconState{
			0: (*v1.BeaconState)(nil),
		},
	}
	bs := &Server{
		BeaconDB: beaconDB,
		HeadFetcher: &mock.ChainService{
			State: headState,
		},
		GenesisTimeFetcher: &mock.ChainService{
			Genesis: time.Now().Add(time.Duration(-1*int64(slots)) * time.Second),
		},
		StateGen: mockStateGen,
	}

	wanted := "Could not set up pre compute instance: failed to initialize precompute: nil inner state"
	_, err = bs.GetValidatorParticipation(
		ctx,
		&ethpb.GetValidatorParticipationRequest{
			QueryFilter: &ethpb.GetValidatorParticipationRequest_Epoch{
				Epoch: 1,
			},
		},
	)
	assert.ErrorContains(t, wanted, err)
}

func TestServer_GetValidatorParticipation_CurrentAndPrevEpoch(t *testing.T) {
	helpers.ClearCache()
	beaconDB := dbTest.SetupDB(t)

	ctx := context.Background()
	validatorCount := uint64(32)

	validators := make([]*ethpb.Validator, validatorCount)
	balances := make([]uint64, validatorCount)
	for i := 0; i < len(validators); i++ {
		validators[i] = &ethpb.Validator{
			PublicKey:             bytesutil.ToBytes(uint64(i), 48),
			WithdrawalCredentials: make([]byte, 32),
			ExitEpoch:             params.BeaconConfig().FarFutureEpoch,
			EffectiveBalance:      params.BeaconConfig().MaxEffectiveBalance,
		}
		balances[i] = params.BeaconConfig().MaxEffectiveBalance
	}

	atts := []*pb.PendingAttestation{{
		Data:            testutil.HydrateAttestationData(&ethpb.AttestationData{}),
		InclusionDelay:  1,
		AggregationBits: bitfield.NewBitlist(validatorCount / uint64(params.BeaconConfig().SlotsPerEpoch)),
	}}
	headState, err := testutil.NewBeaconState()
	require.NoError(t, err)
	require.NoError(t, headState.SetSlot(2*params.BeaconConfig().SlotsPerEpoch-1))
	require.NoError(t, headState.SetValidators(validators))
	require.NoError(t, headState.SetBalances(balances))
	require.NoError(t, headState.AppendCurrentEpochAttestations(atts[0]))
	require.NoError(t, headState.AppendPreviousEpochAttestations(atts[0]))

	b := testutil.NewBeaconBlock()
	b.Block.Slot = 16
	require.NoError(t, beaconDB.SaveBlock(ctx, wrapper.WrappedPhase0SignedBeaconBlock(b)))
	bRoot, err := b.Block.HashTreeRoot()
	require.NoError(t, beaconDB.SaveStateSummary(ctx, &pb.StateSummary{Root: bRoot[:]}))
	require.NoError(t, beaconDB.SaveStateSummary(ctx, &pb.StateSummary{Root: params.BeaconConfig().ZeroHash[:]}))
	require.NoError(t, beaconDB.SaveGenesisBlockRoot(ctx, bRoot))
	require.NoError(t, err)
	require.NoError(t, beaconDB.SaveState(ctx, headState, bRoot))

	m := &mock.ChainService{State: headState}
	offset := int64(params.BeaconConfig().SlotsPerEpoch.Mul(params.BeaconConfig().SecondsPerSlot))
	bs := &Server{
		BeaconDB:    beaconDB,
		HeadFetcher: m,
		StateGen:    stategen.New(beaconDB),
		GenesisTimeFetcher: &mock.ChainService{
			Genesis: timeutils.Now().Add(time.Duration(-1*offset) * time.Second),
		},
		CanonicalFetcher: &mock.ChainService{
			CanonicalRoots: map[[32]byte]bool{
				bRoot: true,
			},
		},
		FinalizationFetcher: &mock.ChainService{FinalizedCheckPoint: &ethpb.Checkpoint{Epoch: 100}},
	}

	res, err := bs.GetValidatorParticipation(ctx, &ethpb.GetValidatorParticipationRequest{QueryFilter: &ethpb.GetValidatorParticipationRequest_Epoch{Epoch: 1}})
	require.NoError(t, err)

	wanted := &ethpb.ValidatorParticipation{
		GlobalParticipationRate:          float32(params.BeaconConfig().EffectiveBalanceIncrement) / float32(validatorCount*params.BeaconConfig().MaxEffectiveBalance),
		VotedEther:                       params.BeaconConfig().EffectiveBalanceIncrement,
		EligibleEther:                    validatorCount * params.BeaconConfig().MaxEffectiveBalance,
		CurrentEpochActiveGwei:           validatorCount * params.BeaconConfig().MaxEffectiveBalance,
		CurrentEpochAttestingGwei:        params.BeaconConfig().EffectiveBalanceIncrement,
		CurrentEpochTargetAttestingGwei:  params.BeaconConfig().EffectiveBalanceIncrement,
		PreviousEpochActiveGwei:          validatorCount * params.BeaconConfig().MaxEffectiveBalance,
		PreviousEpochAttestingGwei:       params.BeaconConfig().EffectiveBalanceIncrement,
		PreviousEpochTargetAttestingGwei: params.BeaconConfig().EffectiveBalanceIncrement,
		PreviousEpochHeadAttestingGwei:   params.BeaconConfig().EffectiveBalanceIncrement,
	}
	assert.DeepEqual(t, true, res.Finalized, "Incorrect validator participation respond")
	assert.DeepEqual(t, wanted, res.Participation, "Incorrect validator participation respond")
}

func TestServer_GetValidatorParticipation_OrphanedUntilGenesis(t *testing.T) {
	helpers.ClearCache()
	params.UseMainnetConfig()

	beaconDB := dbTest.SetupDB(t)
	ctx := context.Background()
	validatorCount := uint64(100)

	validators := make([]*ethpb.Validator, validatorCount)
	balances := make([]uint64, validatorCount)
	for i := 0; i < len(validators); i++ {
		validators[i] = &ethpb.Validator{
			PublicKey:             bytesutil.ToBytes(uint64(i), 48),
			WithdrawalCredentials: make([]byte, 32),
			ExitEpoch:             params.BeaconConfig().FarFutureEpoch,
			EffectiveBalance:      params.BeaconConfig().MaxEffectiveBalance,
		}
		balances[i] = params.BeaconConfig().MaxEffectiveBalance
	}

	atts := []*pb.PendingAttestation{{
		Data:            testutil.HydrateAttestationData(&ethpb.AttestationData{}),
		InclusionDelay:  1,
		AggregationBits: bitfield.NewBitlist(validatorCount / uint64(params.BeaconConfig().SlotsPerEpoch)),
	}}
	headState, err := testutil.NewBeaconState()
	require.NoError(t, err)
	require.NoError(t, headState.SetSlot(2*params.BeaconConfig().SlotsPerEpoch-1))
	require.NoError(t, headState.SetValidators(validators))
	require.NoError(t, headState.SetBalances(balances))
	require.NoError(t, headState.AppendCurrentEpochAttestations(atts[0]))
	require.NoError(t, headState.AppendPreviousEpochAttestations(atts[0]))

	b := testutil.NewBeaconBlock()
	require.NoError(t, beaconDB.SaveBlock(ctx, wrapper.WrappedPhase0SignedBeaconBlock(b)))
	bRoot, err := b.Block.HashTreeRoot()
	require.NoError(t, beaconDB.SaveStateSummary(ctx, &pb.StateSummary{Root: bRoot[:]}))
	require.NoError(t, beaconDB.SaveStateSummary(ctx, &pb.StateSummary{Root: params.BeaconConfig().ZeroHash[:]}))
	require.NoError(t, beaconDB.SaveGenesisBlockRoot(ctx, bRoot))
	require.NoError(t, err)
	require.NoError(t, beaconDB.SaveState(ctx, headState, bRoot))

	m := &mock.ChainService{State: headState}
	offset := int64(params.BeaconConfig().SlotsPerEpoch.Mul(params.BeaconConfig().SecondsPerSlot))
	bs := &Server{
		BeaconDB:    beaconDB,
		HeadFetcher: m,
		StateGen:    stategen.New(beaconDB),
		GenesisTimeFetcher: &mock.ChainService{
			Genesis: timeutils.Now().Add(time.Duration(-1*offset) * time.Second),
		},
		CanonicalFetcher: &mock.ChainService{
			CanonicalRoots: map[[32]byte]bool{
				bRoot: true,
			},
		},
		FinalizationFetcher: &mock.ChainService{FinalizedCheckPoint: &ethpb.Checkpoint{Epoch: 100}},
	}

	res, err := bs.GetValidatorParticipation(ctx, &ethpb.GetValidatorParticipationRequest{QueryFilter: &ethpb.GetValidatorParticipationRequest_Epoch{Epoch: 1}})
	require.NoError(t, err)

	wanted := &ethpb.ValidatorParticipation{
		GlobalParticipationRate:          float32(params.BeaconConfig().EffectiveBalanceIncrement) / float32(validatorCount*params.BeaconConfig().MaxEffectiveBalance),
		VotedEther:                       params.BeaconConfig().EffectiveBalanceIncrement,
		EligibleEther:                    validatorCount * params.BeaconConfig().MaxEffectiveBalance,
		CurrentEpochActiveGwei:           validatorCount * params.BeaconConfig().MaxEffectiveBalance,
		CurrentEpochAttestingGwei:        params.BeaconConfig().EffectiveBalanceIncrement,
		CurrentEpochTargetAttestingGwei:  params.BeaconConfig().EffectiveBalanceIncrement,
		PreviousEpochActiveGwei:          validatorCount * params.BeaconConfig().MaxEffectiveBalance,
		PreviousEpochAttestingGwei:       params.BeaconConfig().EffectiveBalanceIncrement,
		PreviousEpochTargetAttestingGwei: params.BeaconConfig().EffectiveBalanceIncrement,
		PreviousEpochHeadAttestingGwei:   params.BeaconConfig().EffectiveBalanceIncrement,
	}
	assert.DeepEqual(t, true, res.Finalized, "Incorrect validator participation respond")
	assert.DeepEqual(t, wanted, res.Participation, "Incorrect validator participation respond")
}

func TestGetValidatorPerformance_Syncing(t *testing.T) {
	ctx := context.Background()

	bs := &Server{
		SyncChecker: &mockSync.Sync{IsSyncing: true},
	}

	wanted := "Syncing to latest head, not ready to respond"
	_, err := bs.GetValidatorPerformance(ctx, nil)
	assert.ErrorContains(t, wanted, err)
}

func TestGetValidatorPerformance_OK(t *testing.T) {
	helpers.ClearCache()
	params.UseMinimalConfig()
	defer params.UseMainnetConfig()

	ctx := context.Background()
	epoch := types.Epoch(1)
	headState, err := testutil.NewBeaconState()
	require.NoError(t, err)
	require.NoError(t, headState.SetSlot(params.BeaconConfig().SlotsPerEpoch.Mul(uint64(epoch+1))))
	atts := make([]*pb.PendingAttestation, 3)
	for i := 0; i < len(atts); i++ {
		atts[i] = &pb.PendingAttestation{
			Data: &ethpb.AttestationData{
				Target: &ethpb.Checkpoint{Root: make([]byte, 32)},
				Source: &ethpb.Checkpoint{Root: make([]byte, 32)},
			},
			AggregationBits: bitfield.Bitlist{},
			InclusionDelay:  1,
		}
		require.NoError(t, headState.AppendPreviousEpochAttestations(atts[i]))
	}
	defaultBal := params.BeaconConfig().MaxEffectiveBalance
	extraBal := params.BeaconConfig().MaxEffectiveBalance + params.BeaconConfig().GweiPerEth
	balances := []uint64{defaultBal, extraBal, extraBal + params.BeaconConfig().GweiPerEth}
	require.NoError(t, headState.SetBalances(balances))
	publicKey1 := bytesutil.ToBytes48([]byte{1})
	publicKey2 := bytesutil.ToBytes48([]byte{2})
	publicKey3 := bytesutil.ToBytes48([]byte{3})
	validators := []*ethpb.Validator{
		{
			PublicKey:       publicKey1[:],
			ActivationEpoch: 5,
			ExitEpoch:       params.BeaconConfig().FarFutureEpoch,
		},
		{
			PublicKey:        publicKey2[:],
			EffectiveBalance: defaultBal,
			ActivationEpoch:  0,
			ExitEpoch:        params.BeaconConfig().FarFutureEpoch,
		},
		{
			PublicKey:        publicKey3[:],
			EffectiveBalance: defaultBal,
			ActivationEpoch:  0,
			ExitEpoch:        params.BeaconConfig().FarFutureEpoch,
		},
	}
	require.NoError(t, headState.SetValidators(validators))
	require.NoError(t, headState.SetBalances([]uint64{100, 101, 102}))
	offset := int64(headState.Slot().Mul(params.BeaconConfig().SecondsPerSlot))
	bs := &Server{
		HeadFetcher: &mock.ChainService{
			State: headState,
		},
		GenesisTimeFetcher: &mock.ChainService{Genesis: time.Now().Add(time.Duration(-1*offset) * time.Second)},
		SyncChecker:        &mockSync.Sync{IsSyncing: false},
	}
	farFuture := params.BeaconConfig().FarFutureSlot
	want := &ethpb.ValidatorPerformanceResponse{
		PublicKeys:                    [][]byte{publicKey2[:], publicKey3[:]},
		CurrentEffectiveBalances:      []uint64{params.BeaconConfig().MaxEffectiveBalance, params.BeaconConfig().MaxEffectiveBalance},
		InclusionSlots:                []types.Slot{farFuture, farFuture},
		InclusionDistances:            []types.Slot{farFuture, farFuture},
		CorrectlyVotedSource:          []bool{false, false},
		CorrectlyVotedTarget:          []bool{false, false},
		CorrectlyVotedHead:            []bool{false, false},
		BalancesBeforeEpochTransition: []uint64{101, 102},
		BalancesAfterEpochTransition:  []uint64{0, 0},
		MissingValidators:             [][]byte{publicKey1[:]},
	}

	res, err := bs.GetValidatorPerformance(ctx, &ethpb.ValidatorPerformanceRequest{
		PublicKeys: [][]byte{publicKey1[:], publicKey3[:], publicKey2[:]},
	})
	require.NoError(t, err)
	if !proto.Equal(want, res) {
		t.Errorf("Wanted %v\nReceived %v", want, res)
	}
}

func TestGetValidatorPerformance_Indices(t *testing.T) {
	ctx := context.Background()
	epoch := types.Epoch(1)
	defaultBal := params.BeaconConfig().MaxEffectiveBalance
	extraBal := params.BeaconConfig().MaxEffectiveBalance + params.BeaconConfig().GweiPerEth
	headState, err := testutil.NewBeaconState()
	require.NoError(t, err)
	require.NoError(t, headState.SetSlot(params.BeaconConfig().SlotsPerEpoch.Mul(uint64(epoch+1))))
	balances := []uint64{defaultBal, extraBal, extraBal + params.BeaconConfig().GweiPerEth}
	require.NoError(t, headState.SetBalances(balances))
	publicKey1 := bytesutil.ToBytes48([]byte{1})
	publicKey2 := bytesutil.ToBytes48([]byte{2})
	publicKey3 := bytesutil.ToBytes48([]byte{3})
	validators := []*ethpb.Validator{
		{
			PublicKey:       publicKey1[:],
			ActivationEpoch: 5,
			ExitEpoch:       params.BeaconConfig().FarFutureEpoch,
		},
		{
			PublicKey:        publicKey2[:],
			EffectiveBalance: defaultBal,
			ActivationEpoch:  0,
			ExitEpoch:        params.BeaconConfig().FarFutureEpoch,
		},
		{
			PublicKey:        publicKey3[:],
			EffectiveBalance: defaultBal,
			ActivationEpoch:  0,
			ExitEpoch:        params.BeaconConfig().FarFutureEpoch,
		},
	}
	require.NoError(t, headState.SetValidators(validators))
	offset := int64(headState.Slot().Mul(params.BeaconConfig().SecondsPerSlot))
	bs := &Server{
		HeadFetcher: &mock.ChainService{
			// 10 epochs into the future.
			State: headState,
		},
		SyncChecker:        &mockSync.Sync{IsSyncing: false},
		GenesisTimeFetcher: &mock.ChainService{Genesis: time.Now().Add(time.Duration(-1*offset) * time.Second)},
	}
	c := headState.Copy()
	vp, bp, err := precompute.New(ctx, c)
	require.NoError(t, err)
	vp, bp, err = precompute.ProcessAttestations(ctx, c, vp, bp)
	require.NoError(t, err)
	_, err = precompute.ProcessRewardsAndPenaltiesPrecompute(c, bp, vp, precompute.AttestationsDelta, precompute.ProposersDelta)
	require.NoError(t, err)
	farFuture := params.BeaconConfig().FarFutureSlot
	want := &ethpb.ValidatorPerformanceResponse{
		PublicKeys:                    [][]byte{publicKey2[:], publicKey3[:]},
		CurrentEffectiveBalances:      []uint64{params.BeaconConfig().MaxEffectiveBalance, params.BeaconConfig().MaxEffectiveBalance},
		InclusionSlots:                []types.Slot{farFuture, farFuture},
		InclusionDistances:            []types.Slot{farFuture, farFuture},
		CorrectlyVotedSource:          []bool{false, false},
		CorrectlyVotedTarget:          []bool{false, false},
		CorrectlyVotedHead:            []bool{false, false},
		BalancesBeforeEpochTransition: []uint64{extraBal, extraBal + params.BeaconConfig().GweiPerEth},
		BalancesAfterEpochTransition:  []uint64{vp[1].AfterEpochTransitionBalance, vp[2].AfterEpochTransitionBalance},
		MissingValidators:             [][]byte{publicKey1[:]},
	}

	res, err := bs.GetValidatorPerformance(ctx, &ethpb.ValidatorPerformanceRequest{
		Indices: []types.ValidatorIndex{2, 1, 0},
	})
	require.NoError(t, err)
	if !proto.Equal(want, res) {
		t.Errorf("Wanted %v\nReceived %v", want, res)
	}
}

func TestGetValidatorPerformance_IndicesPubkeys(t *testing.T) {
	ctx := context.Background()
	epoch := types.Epoch(1)
	defaultBal := params.BeaconConfig().MaxEffectiveBalance
	extraBal := params.BeaconConfig().MaxEffectiveBalance + params.BeaconConfig().GweiPerEth
	headState, err := testutil.NewBeaconState()
	require.NoError(t, err)
	require.NoError(t, headState.SetSlot(params.BeaconConfig().SlotsPerEpoch.Mul(uint64(epoch+1))))
	balances := []uint64{defaultBal, extraBal, extraBal + params.BeaconConfig().GweiPerEth}
	require.NoError(t, headState.SetBalances(balances))
	publicKey1 := bytesutil.ToBytes48([]byte{1})
	publicKey2 := bytesutil.ToBytes48([]byte{2})
	publicKey3 := bytesutil.ToBytes48([]byte{3})
	validators := []*ethpb.Validator{
		{
			PublicKey:       publicKey1[:],
			ActivationEpoch: 5,
			ExitEpoch:       params.BeaconConfig().FarFutureEpoch,
		},
		{
			PublicKey:        publicKey2[:],
			EffectiveBalance: defaultBal,
			ActivationEpoch:  0,
			ExitEpoch:        params.BeaconConfig().FarFutureEpoch,
		},
		{
			PublicKey:        publicKey3[:],
			EffectiveBalance: defaultBal,
			ActivationEpoch:  0,
			ExitEpoch:        params.BeaconConfig().FarFutureEpoch,
		},
	}
	require.NoError(t, headState.SetValidators(validators))

	offset := int64(headState.Slot().Mul(params.BeaconConfig().SecondsPerSlot))
	bs := &Server{
		HeadFetcher: &mock.ChainService{
			// 10 epochs into the future.
			State: headState,
		},
		SyncChecker:        &mockSync.Sync{IsSyncing: false},
		GenesisTimeFetcher: &mock.ChainService{Genesis: time.Now().Add(time.Duration(-1*offset) * time.Second)},
	}
	c := headState.Copy()
	vp, bp, err := precompute.New(ctx, c)
	require.NoError(t, err)
	vp, bp, err = precompute.ProcessAttestations(ctx, c, vp, bp)
	require.NoError(t, err)
	_, err = precompute.ProcessRewardsAndPenaltiesPrecompute(c, bp, vp, precompute.AttestationsDelta, precompute.ProposersDelta)
	require.NoError(t, err)
	farFuture := params.BeaconConfig().FarFutureSlot
	want := &ethpb.ValidatorPerformanceResponse{
		PublicKeys:                    [][]byte{publicKey2[:], publicKey3[:]},
		CurrentEffectiveBalances:      []uint64{params.BeaconConfig().MaxEffectiveBalance, params.BeaconConfig().MaxEffectiveBalance},
		InclusionSlots:                []types.Slot{farFuture, farFuture},
		InclusionDistances:            []types.Slot{farFuture, farFuture},
		CorrectlyVotedSource:          []bool{false, false},
		CorrectlyVotedTarget:          []bool{false, false},
		CorrectlyVotedHead:            []bool{false, false},
		BalancesBeforeEpochTransition: []uint64{extraBal, extraBal + params.BeaconConfig().GweiPerEth},
		BalancesAfterEpochTransition:  []uint64{vp[1].AfterEpochTransitionBalance, vp[2].AfterEpochTransitionBalance},
		MissingValidators:             [][]byte{publicKey1[:]},
	}
	// Index 2 and publicKey3 points to the same validator.
	// Should not return duplicates.
	res, err := bs.GetValidatorPerformance(ctx, &ethpb.ValidatorPerformanceRequest{
		PublicKeys: [][]byte{publicKey1[:], publicKey3[:]}, Indices: []types.ValidatorIndex{1, 2},
	})
	require.NoError(t, err)
	if !proto.Equal(want, res) {
		t.Errorf("Wanted %v\nReceived %v", want, res)
	}
}

func BenchmarkListValidatorBalances(b *testing.B) {
	b.StopTimer()
	beaconDB := dbTest.SetupDB(b)
	ctx := context.Background()

	count := 1000
	_, _, headState := setupValidators(b, beaconDB, count)
	bs := &Server{
		HeadFetcher: &mock.ChainService{
			State: headState,
		},
	}

	req := &ethpb.ListValidatorBalancesRequest{PageSize: 100}
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		_, err := bs.ListValidatorBalances(ctx, req)
		require.NoError(b, err)
	}
}

func setupValidators(t testing.TB, _ db.Database, count int) ([]*ethpb.Validator, []uint64, iface.BeaconState) {
	balances := make([]uint64, count)
	validators := make([]*ethpb.Validator, 0, count)
	for i := 0; i < count; i++ {
		pubKey := pubKey(uint64(i))
		balances[i] = uint64(i)
		validators = append(validators, &ethpb.Validator{
			PublicKey:             pubKey,
			WithdrawalCredentials: make([]byte, 32),
		})
	}
	s, err := testutil.NewBeaconState()
	require.NoError(t, err)
	require.NoError(t, s.SetValidators(validators))
	require.NoError(t, s.SetBalances(balances))
	return validators, balances, s
}

func TestServer_GetIndividualVotes_RequestFutureSlot(t *testing.T) {
	ds := &Server{GenesisTimeFetcher: &mock.ChainService{}}
	req := &ethpb.IndividualVotesRequest{
		Epoch: helpers.SlotToEpoch(ds.GenesisTimeFetcher.CurrentSlot()) + 1,
	}
	wanted := errNoEpochInfoError
	_, err := ds.GetIndividualVotes(context.Background(), req)
	assert.ErrorContains(t, wanted, err)
}

func TestServer_GetIndividualVotes_ValidatorsDontExist(t *testing.T) {
	params.UseMinimalConfig()
	defer params.UseMainnetConfig()

	beaconDB := dbTest.SetupDB(t)
	ctx := context.Background()

	validators := uint64(64)
	stateWithValidators, _ := testutil.DeterministicGenesisState(t, validators)
	beaconState, err := testutil.NewBeaconState()
	require.NoError(t, err)
	require.NoError(t, beaconState.SetValidators(stateWithValidators.Validators()))
	require.NoError(t, beaconState.SetSlot(params.BeaconConfig().SlotsPerEpoch))

	b := testutil.NewBeaconBlock()
	b.Block.Slot = params.BeaconConfig().SlotsPerEpoch
	require.NoError(t, beaconDB.SaveBlock(ctx, wrapper.WrappedPhase0SignedBeaconBlock(b)))
	gRoot, err := b.Block.HashTreeRoot()
	require.NoError(t, err)
	gen := stategen.New(beaconDB)
	require.NoError(t, gen.SaveState(ctx, gRoot, beaconState))
	require.NoError(t, beaconDB.SaveState(ctx, beaconState, gRoot))
	require.NoError(t, beaconDB.SaveGenesisBlockRoot(ctx, gRoot))
	bs := &Server{
		StateGen:           gen,
		GenesisTimeFetcher: &mock.ChainService{},
	}

	// Test non exist public key.
	res, err := bs.GetIndividualVotes(ctx, &ethpb.IndividualVotesRequest{
		PublicKeys: [][]byte{{'a'}},
		Epoch:      0,
	})
	require.NoError(t, err)
	wanted := &ethpb.IndividualVotesRespond{
		IndividualVotes: []*ethpb.IndividualVotesRespond_IndividualVote{
			{PublicKey: []byte{'a'}, ValidatorIndex: types.ValidatorIndex(^uint64(0))},
		},
	}
	assert.DeepEqual(t, wanted, res, "Unexpected response")

	// Test non-existent validator index.
	res, err = bs.GetIndividualVotes(ctx, &ethpb.IndividualVotesRequest{
		Indices: []types.ValidatorIndex{100},
		Epoch:   0,
	})
	require.NoError(t, err)
	wanted = &ethpb.IndividualVotesRespond{
		IndividualVotes: []*ethpb.IndividualVotesRespond_IndividualVote{
			{ValidatorIndex: 100},
		},
	}
	assert.DeepEqual(t, wanted, res, "Unexpected response")

	// Test both.
	res, err = bs.GetIndividualVotes(ctx, &ethpb.IndividualVotesRequest{
		PublicKeys: [][]byte{{'a'}, {'b'}},
		Indices:    []types.ValidatorIndex{100, 101},
		Epoch:      0,
	})
	require.NoError(t, err)
	wanted = &ethpb.IndividualVotesRespond{
		IndividualVotes: []*ethpb.IndividualVotesRespond_IndividualVote{
			{PublicKey: []byte{'a'}, ValidatorIndex: types.ValidatorIndex(^uint64(0))},
			{PublicKey: []byte{'b'}, ValidatorIndex: types.ValidatorIndex(^uint64(0))},
			{ValidatorIndex: 100},
			{ValidatorIndex: 101},
		},
	}
	assert.DeepEqual(t, wanted, res, "Unexpected response")
}

func TestServer_GetIndividualVotes_Working(t *testing.T) {
	helpers.ClearCache()

	params.UseMinimalConfig()
	defer params.UseMainnetConfig()
	beaconDB := dbTest.SetupDB(t)
	ctx := context.Background()

	validators := uint64(32)
	stateWithValidators, _ := testutil.DeterministicGenesisState(t, validators)
	beaconState, err := testutil.NewBeaconState()
	require.NoError(t, err)
	require.NoError(t, beaconState.SetValidators(stateWithValidators.Validators()))
	require.NoError(t, beaconState.SetSlot(params.BeaconConfig().SlotsPerEpoch))

	bf := bitfield.NewBitlist(validators / uint64(params.BeaconConfig().SlotsPerEpoch))
	att1 := testutil.NewAttestation()
	att1.AggregationBits = bf
	att2 := testutil.NewAttestation()
	att2.AggregationBits = bf
	rt := [32]byte{'A'}
	att1.Data.Target.Root = rt[:]
	att1.Data.BeaconBlockRoot = rt[:]
	br := beaconState.BlockRoots()
	newRt := [32]byte{'B'}
	br[0] = newRt[:]
	require.NoError(t, beaconState.SetBlockRoots(br))
	att2.Data.Target.Root = rt[:]
	att2.Data.BeaconBlockRoot = newRt[:]
	err = beaconState.AppendPreviousEpochAttestations(&pb.PendingAttestation{
		Data: att1.Data, AggregationBits: bf, InclusionDelay: 1,
	})
	require.NoError(t, err)
	err = beaconState.AppendCurrentEpochAttestations(&pb.PendingAttestation{
		Data: att2.Data, AggregationBits: bf, InclusionDelay: 1,
	})
	require.NoError(t, err)

	b := testutil.NewBeaconBlock()
	b.Block.Slot = params.BeaconConfig().SlotsPerEpoch
	require.NoError(t, beaconDB.SaveBlock(ctx, wrapper.WrappedPhase0SignedBeaconBlock(b)))
	gRoot, err := b.Block.HashTreeRoot()
	require.NoError(t, err)
	gen := stategen.New(beaconDB)
	require.NoError(t, gen.SaveState(ctx, gRoot, beaconState))
	require.NoError(t, beaconDB.SaveState(ctx, beaconState, gRoot))
	require.NoError(t, beaconDB.SaveGenesisBlockRoot(ctx, gRoot))
	bs := &Server{
		StateGen:           gen,
		GenesisTimeFetcher: &mock.ChainService{},
	}

	res, err := bs.GetIndividualVotes(ctx, &ethpb.IndividualVotesRequest{
		Indices: []types.ValidatorIndex{0, 1},
		Epoch:   0,
	})
	require.NoError(t, err)
	wanted := &ethpb.IndividualVotesRespond{
		IndividualVotes: []*ethpb.IndividualVotesRespond_IndividualVote{
			{
				ValidatorIndex:                   0,
				PublicKey:                        beaconState.Validators()[0].PublicKey,
				IsActiveInCurrentEpoch:           true,
				IsActiveInPreviousEpoch:          true,
				CurrentEpochEffectiveBalanceGwei: params.BeaconConfig().MaxEffectiveBalance,
				InclusionSlot:                    params.BeaconConfig().FarFutureSlot,
				InclusionDistance:                params.BeaconConfig().FarFutureSlot,
			},
			{
				ValidatorIndex:                   1,
				PublicKey:                        beaconState.Validators()[1].PublicKey,
				IsActiveInCurrentEpoch:           true,
				IsActiveInPreviousEpoch:          true,
				CurrentEpochEffectiveBalanceGwei: params.BeaconConfig().MaxEffectiveBalance,
				InclusionSlot:                    params.BeaconConfig().FarFutureSlot,
				InclusionDistance:                params.BeaconConfig().FarFutureSlot,
			},
		},
	}
	assert.DeepEqual(t, wanted, res, "Unexpected response")
}

func Test_validatorStatus(t *testing.T) {
	tests := []struct {
		name      string
		validator *ethpb.Validator
		epoch     types.Epoch
		want      ethpb.ValidatorStatus
	}{
		{
			name:      "Unknown",
			validator: nil,
			epoch:     0,
			want:      ethpb.ValidatorStatus_UNKNOWN_STATUS,
		},
		{
			name: "Deposited",
			validator: &ethpb.Validator{
				ActivationEligibilityEpoch: 1,
			},
			epoch: 0,
			want:  ethpb.ValidatorStatus_DEPOSITED,
		},
		{
			name: "Pending",
			validator: &ethpb.Validator{
				ActivationEligibilityEpoch: 0,
				ActivationEpoch:            1,
			},
			epoch: 0,
			want:  ethpb.ValidatorStatus_PENDING,
		},
		{
			name: "Active",
			validator: &ethpb.Validator{
				ActivationEligibilityEpoch: 0,
				ActivationEpoch:            0,
				ExitEpoch:                  params.BeaconConfig().FarFutureEpoch,
			},
			epoch: 0,
			want:  ethpb.ValidatorStatus_ACTIVE,
		},
		{
			name: "Slashed",
			validator: &ethpb.Validator{
				ActivationEligibilityEpoch: 0,
				ActivationEpoch:            0,
				ExitEpoch:                  5,
				Slashed:                    true,
			},
			epoch: 4,
			want:  ethpb.ValidatorStatus_SLASHING,
		},
		{
			name: "Exiting",
			validator: &ethpb.Validator{
				ActivationEligibilityEpoch: 0,
				ActivationEpoch:            0,
				ExitEpoch:                  5,
				Slashed:                    false,
			},
			epoch: 4,
			want:  ethpb.ValidatorStatus_EXITING,
		},
		{
			name: "Exiting",
			validator: &ethpb.Validator{
				ActivationEligibilityEpoch: 0,
				ActivationEpoch:            0,
				ExitEpoch:                  3,
				Slashed:                    false,
			},
			epoch: 4,
			want:  ethpb.ValidatorStatus_EXITED,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := validatorStatus(tt.validator, tt.epoch); got != tt.want {
				t.Errorf("validatorStatus() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestServer_isSlotCanonical(t *testing.T) {
	beaconDB := dbTest.SetupDB(t)
	ctx := context.Background()
	var roots [][32]byte
	cRoots := map[[32]byte]bool{}
	for i := 1; i < 100; i++ {
		b := testutil.NewBeaconBlock()
		b.Block.Slot = types.Slot(i)
		require.NoError(t, beaconDB.SaveBlock(ctx, wrapper.WrappedPhase0SignedBeaconBlock(b)))
		br, err := b.Block.HashTreeRoot()
		require.NoError(t, err)
		if i%2 == 0 {
			cRoots[br] = true
		}
		roots = append(roots, br)
	}

	bs := &Server{
		BeaconDB: beaconDB,
		CanonicalFetcher: &mock.ChainService{
			CanonicalRoots: cRoots,
		},
	}

	for i := range roots {
		slot := types.Slot(i + 1)
		c, err := bs.isSlotCanonical(ctx, slot)
		require.NoError(t, err)
		if slot%2 == 0 {
			require.Equal(t, true, c)
		} else {
			require.Equal(t, false, c)
		}
	}
}

func TestServer_isSlotCanonical_MultipleBlocks(t *testing.T) {
	beaconDB := dbTest.SetupDB(t)
	ctx := context.Background()
	var roots [][32]byte
	cRoots := map[[32]byte]bool{}
	for i := 1; i < 100; i++ {
		b := testutil.NewBeaconBlock()
		b.Block.Slot = types.Slot(i)
		require.NoError(t, beaconDB.SaveBlock(ctx, wrapper.WrappedPhase0SignedBeaconBlock(b)))
		br, err := b.Block.HashTreeRoot()
		require.NoError(t, err)
		if i%2 == 0 {
			cRoots[br] = true
			// Save a block in the same slot
			b = testutil.NewBeaconBlock()
			b.Block.Slot = types.Slot(i)
			b.Block.ProposerIndex = 100
			require.NoError(t, beaconDB.SaveBlock(ctx, wrapper.WrappedPhase0SignedBeaconBlock(b)))
		}
		roots = append(roots, br)
	}

	bs := &Server{
		BeaconDB: beaconDB,
		CanonicalFetcher: &mock.ChainService{
			CanonicalRoots: cRoots,
		},
	}

	for i := range roots {
		slot := types.Slot(i + 1)
		c, err := bs.isSlotCanonical(ctx, slot)
		require.NoError(t, err)
		if slot%2 == 0 {
			require.Equal(t, true, c)
		} else {
			require.Equal(t, false, c)
		}
	}
}

func TestServer_isSlotCanonicalForSlot0(t *testing.T) {
	ctx := context.Background()
	bs := &Server{}
	c, err := bs.isSlotCanonical(ctx, 0)
	require.NoError(t, err)
	require.Equal(t, true, c)
}
