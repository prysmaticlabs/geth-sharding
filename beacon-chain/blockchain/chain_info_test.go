package blockchain

import (
	"context"
	"testing"
	"time"

	types "github.com/prysmaticlabs/eth2-types"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	testDB "github.com/prysmaticlabs/prysm/beacon-chain/db/testing"
	"github.com/prysmaticlabs/prysm/beacon-chain/forkchoice/protoarray"
	v1 "github.com/prysmaticlabs/prysm/beacon-chain/state/v1"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	ethpb "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/proto/eth/v1alpha1/wrapper"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/testutil"
	"github.com/prysmaticlabs/prysm/shared/testutil/assert"
	"github.com/prysmaticlabs/prysm/shared/testutil/require"
	"google.golang.org/protobuf/proto"
)

// Ensure Service implements chain info interface.
var _ ChainInfoFetcher = (*Service)(nil)
var _ TimeFetcher = (*Service)(nil)
var _ ForkFetcher = (*Service)(nil)

func TestFinalizedCheckpt_Nil(t *testing.T) {
	beaconDB := testDB.SetupDB(t)
	c := setupBeaconChain(t, beaconDB)
	assert.DeepEqual(t, params.BeaconConfig().ZeroHash[:], c.FinalizedCheckpt().Root, "Incorrect pre chain start value")
}

func TestHeadRoot_Nil(t *testing.T) {
	beaconDB := testDB.SetupDB(t)
	c := setupBeaconChain(t, beaconDB)
	headRoot, err := c.HeadRoot(context.Background())
	require.NoError(t, err)
	assert.DeepEqual(t, params.BeaconConfig().ZeroHash[:], headRoot, "Incorrect pre chain start value")
}

func TestFinalizedCheckpt_CanRetrieve(t *testing.T) {
	beaconDB := testDB.SetupDB(t)

	cp := &ethpb.Checkpoint{Epoch: 5, Root: bytesutil.PadTo([]byte("foo"), 32)}
	c := setupBeaconChain(t, beaconDB)
	c.finalizedCheckpt = cp

	assert.Equal(t, cp.Epoch, c.FinalizedCheckpt().Epoch, "Unexpected finalized epoch")
}

func TestFinalizedCheckpt_GenesisRootOk(t *testing.T) {
	beaconDB := testDB.SetupDB(t)

	genesisRoot := [32]byte{'A'}
	cp := &ethpb.Checkpoint{Root: genesisRoot[:]}
	c := setupBeaconChain(t, beaconDB)
	c.finalizedCheckpt = cp
	c.genesisRoot = genesisRoot
	assert.DeepEqual(t, c.genesisRoot[:], c.FinalizedCheckpt().Root)
}

func TestCurrentJustifiedCheckpt_CanRetrieve(t *testing.T) {
	beaconDB := testDB.SetupDB(t)

	c := setupBeaconChain(t, beaconDB)
	assert.Equal(t, params.BeaconConfig().ZeroHash, bytesutil.ToBytes32(c.CurrentJustifiedCheckpt().Root), "Unexpected justified epoch")
	cp := &ethpb.Checkpoint{Epoch: 6, Root: bytesutil.PadTo([]byte("foo"), 32)}
	c.justifiedCheckpt = cp
	assert.Equal(t, cp.Epoch, c.CurrentJustifiedCheckpt().Epoch, "Unexpected justified epoch")
}

func TestJustifiedCheckpt_GenesisRootOk(t *testing.T) {
	beaconDB := testDB.SetupDB(t)

	c := setupBeaconChain(t, beaconDB)
	genesisRoot := [32]byte{'B'}
	cp := &ethpb.Checkpoint{Root: genesisRoot[:]}
	c.justifiedCheckpt = cp
	c.genesisRoot = genesisRoot
	assert.DeepEqual(t, c.genesisRoot[:], c.CurrentJustifiedCheckpt().Root)
}

func TestPreviousJustifiedCheckpt_CanRetrieve(t *testing.T) {
	beaconDB := testDB.SetupDB(t)

	cp := &ethpb.Checkpoint{Epoch: 7, Root: bytesutil.PadTo([]byte("foo"), 32)}
	c := setupBeaconChain(t, beaconDB)
	assert.Equal(t, params.BeaconConfig().ZeroHash, bytesutil.ToBytes32(c.CurrentJustifiedCheckpt().Root), "Unexpected justified epoch")
	c.prevJustifiedCheckpt = cp
	assert.Equal(t, cp.Epoch, c.PreviousJustifiedCheckpt().Epoch, "Unexpected previous justified epoch")
}

func TestPrevJustifiedCheckpt_GenesisRootOk(t *testing.T) {
	beaconDB := testDB.SetupDB(t)

	genesisRoot := [32]byte{'C'}
	cp := &ethpb.Checkpoint{Root: genesisRoot[:]}
	c := setupBeaconChain(t, beaconDB)
	c.prevJustifiedCheckpt = cp
	c.genesisRoot = genesisRoot
	assert.DeepEqual(t, c.genesisRoot[:], c.PreviousJustifiedCheckpt().Root)
}

func TestHeadSlot_CanRetrieve(t *testing.T) {
	c := &Service{}
	s, err := v1.InitializeFromProto(&pb.BeaconState{})
	require.NoError(t, err)
	c.head = &head{slot: 100, state: s}
	assert.Equal(t, types.Slot(100), c.HeadSlot())
}

func TestHeadRoot_CanRetrieve(t *testing.T) {
	c := &Service{}
	c.head = &head{root: [32]byte{'A'}}
	r, err := c.HeadRoot(context.Background())
	require.NoError(t, err)
	assert.Equal(t, [32]byte{'A'}, bytesutil.ToBytes32(r))
}

func TestHeadRoot_UseDB(t *testing.T) {
	beaconDB := testDB.SetupDB(t)
	c := &Service{cfg: &Config{BeaconDB: beaconDB}}
	c.head = &head{root: params.BeaconConfig().ZeroHash}
	b := testutil.NewBeaconBlock()
	br, err := b.Block.HashTreeRoot()
	require.NoError(t, err)
	require.NoError(t, beaconDB.SaveBlock(context.Background(), wrapper.WrappedPhase0SignedBeaconBlock(b)))
	require.NoError(t, beaconDB.SaveStateSummary(context.Background(), &pb.StateSummary{Root: br[:]}))
	require.NoError(t, beaconDB.SaveHeadBlockRoot(context.Background(), br))
	r, err := c.HeadRoot(context.Background())
	require.NoError(t, err)
	assert.Equal(t, br, bytesutil.ToBytes32(r))
}

func TestHeadBlock_CanRetrieve(t *testing.T) {
	b := testutil.NewBeaconBlock()
	b.Block.Slot = 1
	s, err := v1.InitializeFromProto(&pb.BeaconState{})
	require.NoError(t, err)
	c := &Service{}
	c.head = &head{block: wrapper.WrappedPhase0SignedBeaconBlock(b), state: s}

	recevied, err := c.HeadBlock(context.Background())
	require.NoError(t, err)
	assert.DeepEqual(t, b, recevied.Proto(), "Incorrect head block received")
}

func TestHeadState_CanRetrieve(t *testing.T) {
	s, err := v1.InitializeFromProto(&pb.BeaconState{Slot: 2, GenesisValidatorsRoot: params.BeaconConfig().ZeroHash[:]})
	require.NoError(t, err)
	c := &Service{}
	c.head = &head{state: s}
	headState, err := c.HeadState(context.Background())
	require.NoError(t, err)
	assert.DeepEqual(t, headState.InnerStateUnsafe(), s.InnerStateUnsafe(), "Incorrect head state received")
}

func TestGenesisTime_CanRetrieve(t *testing.T) {
	c := &Service{genesisTime: time.Unix(999, 0)}
	wanted := time.Unix(999, 0)
	assert.Equal(t, wanted, c.GenesisTime(), "Did not get wanted genesis time")
}

func TestCurrentFork_CanRetrieve(t *testing.T) {
	f := &pb.Fork{Epoch: 999}
	s, err := v1.InitializeFromProto(&pb.BeaconState{Fork: f})
	require.NoError(t, err)
	c := &Service{}
	c.head = &head{state: s}
	if !proto.Equal(c.CurrentFork(), f) {
		t.Error("Received incorrect fork version")
	}
}

func TestCurrentFork_NilHeadSTate(t *testing.T) {
	f := &pb.Fork{
		PreviousVersion: params.BeaconConfig().GenesisForkVersion,
		CurrentVersion:  params.BeaconConfig().GenesisForkVersion,
	}
	c := &Service{}
	if !proto.Equal(c.CurrentFork(), f) {
		t.Error("Received incorrect fork version")
	}
}

func TestGenesisValidatorRoot_CanRetrieve(t *testing.T) {
	// Should not panic if head state is nil.
	c := &Service{}
	assert.Equal(t, [32]byte{}, c.GenesisValidatorRoot(), "Did not get correct genesis validator root")

	s, err := v1.InitializeFromProto(&pb.BeaconState{GenesisValidatorsRoot: []byte{'a'}})
	require.NoError(t, err)
	c.head = &head{state: s}
	assert.Equal(t, [32]byte{'a'}, c.GenesisValidatorRoot(), "Did not get correct genesis validator root")
}

func TestHeadETH1Data_Nil(t *testing.T) {
	beaconDB := testDB.SetupDB(t)
	c := setupBeaconChain(t, beaconDB)
	assert.DeepEqual(t, &ethpb.Eth1Data{}, c.HeadETH1Data(), "Incorrect pre chain start value")
}

func TestHeadETH1Data_CanRetrieve(t *testing.T) {
	d := &ethpb.Eth1Data{DepositCount: 999}
	s, err := v1.InitializeFromProto(&pb.BeaconState{Eth1Data: d})
	require.NoError(t, err)
	c := &Service{}
	c.head = &head{state: s}
	if !proto.Equal(c.HeadETH1Data(), d) {
		t.Error("Received incorrect eth1 data")
	}
}

func TestIsCanonical_Ok(t *testing.T) {
	ctx := context.Background()
	beaconDB := testDB.SetupDB(t)
	c := setupBeaconChain(t, beaconDB)

	blk := testutil.NewBeaconBlock()
	blk.Block.Slot = 0
	root, err := blk.Block.HashTreeRoot()
	require.NoError(t, err)
	require.NoError(t, beaconDB.SaveBlock(ctx, wrapper.WrappedPhase0SignedBeaconBlock(blk)))
	require.NoError(t, beaconDB.SaveGenesisBlockRoot(ctx, root))
	can, err := c.IsCanonical(ctx, root)
	require.NoError(t, err)
	assert.Equal(t, true, can)

	can, err = c.IsCanonical(ctx, [32]byte{'a'})
	require.NoError(t, err)
	assert.Equal(t, false, can)
}

func TestService_HeadValidatorsIndices(t *testing.T) {
	s, _ := testutil.DeterministicGenesisState(t, 10)
	c := &Service{}

	c.head = &head{}
	indices, err := c.HeadValidatorsIndices(context.Background(), 0)
	require.NoError(t, err)
	require.Equal(t, 0, len(indices))

	c.head = &head{state: s}
	indices, err = c.HeadValidatorsIndices(context.Background(), 0)
	require.NoError(t, err)
	require.Equal(t, 10, len(indices))
}

func TestService_HeadSeed(t *testing.T) {
	s, _ := testutil.DeterministicGenesisState(t, 1)
	c := &Service{}
	seed, err := helpers.Seed(s, 0, params.BeaconConfig().DomainBeaconAttester)
	require.NoError(t, err)

	c.head = &head{}
	root, err := c.HeadSeed(context.Background(), 0)
	require.NoError(t, err)
	require.Equal(t, [32]byte{}, root)

	c.head = &head{state: s}
	root, err = c.HeadSeed(context.Background(), 0)
	require.NoError(t, err)
	require.DeepEqual(t, seed, root)
}

func TestService_HeadGenesisValidatorRoot(t *testing.T) {
	s, _ := testutil.DeterministicGenesisState(t, 1)
	c := &Service{}

	c.head = &head{}
	root := c.HeadGenesisValidatorRoot()
	require.Equal(t, [32]byte{}, root)

	c.head = &head{state: s}
	root = c.HeadGenesisValidatorRoot()
	require.DeepEqual(t, root[:], s.GenesisValidatorRoot())
}

func TestService_ProtoArrayStore(t *testing.T) {
	c := &Service{cfg: &Config{ForkChoiceStore: protoarray.New(0, 0, [32]byte{})}}
	p := c.ProtoArrayStore()
	require.Equal(t, 0, int(p.FinalizedEpoch()))
}

func TestService_ChainHeads(t *testing.T) {
	ctx := context.Background()
	c := &Service{cfg: &Config{ForkChoiceStore: protoarray.New(0, 0, [32]byte{})}}
	require.NoError(t, c.cfg.ForkChoiceStore.ProcessBlock(ctx, 100, [32]byte{'a'}, [32]byte{}, [32]byte{}, 0, 0))
	require.NoError(t, c.cfg.ForkChoiceStore.ProcessBlock(ctx, 101, [32]byte{'b'}, [32]byte{'a'}, [32]byte{}, 0, 0))
	require.NoError(t, c.cfg.ForkChoiceStore.ProcessBlock(ctx, 102, [32]byte{'c'}, [32]byte{'b'}, [32]byte{}, 0, 0))
	require.NoError(t, c.cfg.ForkChoiceStore.ProcessBlock(ctx, 103, [32]byte{'d'}, [32]byte{}, [32]byte{}, 0, 0))
	require.NoError(t, c.cfg.ForkChoiceStore.ProcessBlock(ctx, 104, [32]byte{'e'}, [32]byte{'b'}, [32]byte{}, 0, 0))

	roots, slots := c.ChainHeads()
	require.DeepEqual(t, [][32]byte{{'c'}, {'d'}, {'e'}}, roots)
	require.DeepEqual(t, []types.Slot{102, 103, 104}, slots)
}
