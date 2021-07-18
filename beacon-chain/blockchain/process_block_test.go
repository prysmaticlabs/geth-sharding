package blockchain

import (
	"context"
	"fmt"
	"math/big"
	"strconv"
	"testing"
	"time"

	"github.com/pkg/errors"
	types "github.com/prysmaticlabs/eth2-types"
	mock "github.com/prysmaticlabs/prysm/beacon-chain/blockchain/testing"
	"github.com/prysmaticlabs/prysm/beacon-chain/cache/depositcache"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/blocks"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/state"
	"github.com/prysmaticlabs/prysm/beacon-chain/db"
	testDB "github.com/prysmaticlabs/prysm/beacon-chain/db/testing"
	"github.com/prysmaticlabs/prysm/beacon-chain/forkchoice/protoarray"
	iface "github.com/prysmaticlabs/prysm/beacon-chain/state/interface"
	"github.com/prysmaticlabs/prysm/beacon-chain/state/stategen"
	v1 "github.com/prysmaticlabs/prysm/beacon-chain/state/v1"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	ethpb "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/proto/eth/v1alpha1/wrapper"
	"github.com/prysmaticlabs/prysm/proto/interfaces"
	"github.com/prysmaticlabs/prysm/shared/attestationutil"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/testutil"
	"github.com/prysmaticlabs/prysm/shared/testutil/assert"
	"github.com/prysmaticlabs/prysm/shared/testutil/require"
	"github.com/prysmaticlabs/prysm/shared/timeutils"
)

func TestStore_OnBlock(t *testing.T) {
	ctx := context.Background()
	beaconDB := testDB.SetupDB(t)

	cfg := &Config{
		BeaconDB:        beaconDB,
		StateGen:        stategen.New(beaconDB),
		ForkChoiceStore: protoarray.New(0, 0, [32]byte{}),
	}
	service, err := NewService(ctx, cfg)
	require.NoError(t, err)
	genesisStateRoot := [32]byte{}
	genesis := blocks.NewGenesisBlock(genesisStateRoot[:])
	assert.NoError(t, beaconDB.SaveBlock(ctx, wrapper.WrappedPhase0SignedBeaconBlock(genesis)))
	validGenesisRoot, err := genesis.Block.HashTreeRoot()
	require.NoError(t, err)
	st, err := testutil.NewBeaconState()
	require.NoError(t, err)
	require.NoError(t, service.cfg.BeaconDB.SaveState(ctx, st.Copy(), validGenesisRoot))
	roots, err := blockTree1(t, beaconDB, validGenesisRoot[:])
	require.NoError(t, err)
	random := testutil.NewBeaconBlock()
	random.Block.Slot = 1
	random.Block.ParentRoot = validGenesisRoot[:]
	assert.NoError(t, beaconDB.SaveBlock(ctx, wrapper.WrappedPhase0SignedBeaconBlock(random)))
	randomParentRoot, err := random.Block.HashTreeRoot()
	assert.NoError(t, err)
	require.NoError(t, service.cfg.BeaconDB.SaveStateSummary(ctx, &pb.StateSummary{Slot: st.Slot(), Root: randomParentRoot[:]}))
	require.NoError(t, service.cfg.BeaconDB.SaveState(ctx, st.Copy(), randomParentRoot))
	randomParentRoot2 := roots[1]
	require.NoError(t, service.cfg.BeaconDB.SaveStateSummary(ctx, &pb.StateSummary{Slot: st.Slot(), Root: randomParentRoot2}))
	require.NoError(t, service.cfg.BeaconDB.SaveState(ctx, st.Copy(), bytesutil.ToBytes32(randomParentRoot2)))

	tests := []struct {
		name          string
		blk           *ethpb.SignedBeaconBlock
		s             iface.BeaconState
		time          uint64
		wantErrString string
	}{
		{
			name:          "parent block root does not have a state",
			blk:           testutil.NewBeaconBlock(),
			s:             st.Copy(),
			wantErrString: "could not reconstruct parent state",
		},
		{
			name: "block is from the future",
			blk: func() *ethpb.SignedBeaconBlock {
				b := testutil.NewBeaconBlock()
				b.Block.ParentRoot = randomParentRoot2
				b.Block.Slot = params.BeaconConfig().FarFutureSlot
				return b
			}(),
			s:             st.Copy(),
			wantErrString: "is in the far distant future",
		},
		{
			name: "could not get finalized block",
			blk: func() *ethpb.SignedBeaconBlock {
				b := testutil.NewBeaconBlock()
				b.Block.ParentRoot = randomParentRoot[:]
				return b
			}(),
			s:             st.Copy(),
			wantErrString: "is not a descendent of the current finalized block",
		},
		{
			name: "same slot as finalized block",
			blk: func() *ethpb.SignedBeaconBlock {
				b := testutil.NewBeaconBlock()
				b.Block.Slot = 0
				b.Block.ParentRoot = randomParentRoot2
				return b
			}(),
			s:             st.Copy(),
			wantErrString: "block is equal or earlier than finalized block, slot 0 < slot 0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service.justifiedCheckpt = &ethpb.Checkpoint{Root: validGenesisRoot[:]}
			service.bestJustifiedCheckpt = &ethpb.Checkpoint{Root: validGenesisRoot[:]}
			service.finalizedCheckpt = &ethpb.Checkpoint{Root: validGenesisRoot[:]}
			service.prevFinalizedCheckpt = &ethpb.Checkpoint{Root: validGenesisRoot[:]}
			service.finalizedCheckpt.Root = roots[0]

			root, err := tt.blk.Block.HashTreeRoot()
			assert.NoError(t, err)
			err = service.onBlock(ctx, wrapper.WrappedPhase0SignedBeaconBlock(tt.blk), root)
			assert.ErrorContains(t, tt.wantErrString, err)
		})
	}
}

func TestStore_OnBlockBatch(t *testing.T) {
	ctx := context.Background()
	beaconDB := testDB.SetupDB(t)

	cfg := &Config{
		BeaconDB: beaconDB,
		StateGen: stategen.New(beaconDB),
	}
	service, err := NewService(ctx, cfg)
	require.NoError(t, err)

	genesisStateRoot := [32]byte{}
	genesis := blocks.NewGenesisBlock(genesisStateRoot[:])
	assert.NoError(t, beaconDB.SaveBlock(ctx, wrapper.WrappedPhase0SignedBeaconBlock(genesis)))
	gRoot, err := genesis.Block.HashTreeRoot()
	require.NoError(t, err)
	service.finalizedCheckpt = &ethpb.Checkpoint{
		Root: gRoot[:],
	}
	service.cfg.ForkChoiceStore = protoarray.New(0, 0, [32]byte{})
	service.saveInitSyncBlock(gRoot, wrapper.WrappedPhase0SignedBeaconBlock(genesis))

	st, keys := testutil.DeterministicGenesisState(t, 64)

	bState := st.Copy()

	var blks []interfaces.SignedBeaconBlock
	var blkRoots [][32]byte
	var firstState iface.BeaconState
	for i := 1; i < 10; i++ {
		b, err := testutil.GenerateFullBlock(bState, keys, testutil.DefaultBlockGenConfig(), types.Slot(i))
		require.NoError(t, err)
		bState, err = state.ExecuteStateTransition(ctx, bState, wrapper.WrappedPhase0SignedBeaconBlock(b))
		require.NoError(t, err)
		if i == 1 {
			firstState = bState.Copy()
		}
		root, err := b.Block.HashTreeRoot()
		require.NoError(t, err)
		service.saveInitSyncBlock(root, wrapper.WrappedPhase0SignedBeaconBlock(b))
		blks = append(blks, wrapper.WrappedPhase0SignedBeaconBlock(b))
		blkRoots = append(blkRoots, root)
	}

	rBlock, err := blks[0].PbPhase0Block()
	assert.NoError(t, err)
	rBlock.Block.ParentRoot = gRoot[:]
	require.NoError(t, beaconDB.SaveBlock(context.Background(), blks[0]))
	require.NoError(t, service.cfg.StateGen.SaveState(ctx, blkRoots[0], firstState))
	_, _, err = service.onBlockBatch(ctx, blks[1:], blkRoots[1:])
	require.NoError(t, err)
}

func TestRemoveStateSinceLastFinalized_EmptyStartSlot(t *testing.T) {
	ctx := context.Background()
	beaconDB := testDB.SetupDB(t)
	params.UseMinimalConfig()
	defer params.UseMainnetConfig()

	cfg := &Config{BeaconDB: beaconDB, ForkChoiceStore: protoarray.New(0, 0, [32]byte{})}
	service, err := NewService(ctx, cfg)
	require.NoError(t, err)
	service.genesisTime = time.Now()

	update, err := service.shouldUpdateCurrentJustified(ctx, &ethpb.Checkpoint{Root: make([]byte, 32)})
	require.NoError(t, err)
	assert.Equal(t, true, update, "Should be able to update justified")
	lastJustifiedBlk := testutil.NewBeaconBlock()
	lastJustifiedBlk.Block.ParentRoot = bytesutil.PadTo([]byte{'G'}, 32)
	lastJustifiedRoot, err := lastJustifiedBlk.Block.HashTreeRoot()
	require.NoError(t, err)
	newJustifiedBlk := testutil.NewBeaconBlock()
	newJustifiedBlk.Block.Slot = 1
	newJustifiedBlk.Block.ParentRoot = bytesutil.PadTo(lastJustifiedRoot[:], 32)
	newJustifiedRoot, err := newJustifiedBlk.Block.HashTreeRoot()
	require.NoError(t, err)
	require.NoError(t, service.cfg.BeaconDB.SaveBlock(ctx, wrapper.WrappedPhase0SignedBeaconBlock(newJustifiedBlk)))
	require.NoError(t, service.cfg.BeaconDB.SaveBlock(ctx, wrapper.WrappedPhase0SignedBeaconBlock(lastJustifiedBlk)))

	diff := params.BeaconConfig().SlotsPerEpoch.Sub(1).Mul(params.BeaconConfig().SecondsPerSlot)
	service.genesisTime = time.Unix(time.Now().Unix()-int64(diff), 0)
	service.justifiedCheckpt = &ethpb.Checkpoint{Root: lastJustifiedRoot[:]}
	update, err = service.shouldUpdateCurrentJustified(ctx, &ethpb.Checkpoint{Root: newJustifiedRoot[:]})
	require.NoError(t, err)
	assert.Equal(t, true, update, "Should be able to update justified")
}

func TestShouldUpdateJustified_ReturnFalse(t *testing.T) {
	ctx := context.Background()
	beaconDB := testDB.SetupDB(t)
	params.UseMinimalConfig()
	defer params.UseMainnetConfig()

	cfg := &Config{BeaconDB: beaconDB}
	service, err := NewService(ctx, cfg)
	require.NoError(t, err)
	lastJustifiedBlk := testutil.NewBeaconBlock()
	lastJustifiedBlk.Block.ParentRoot = bytesutil.PadTo([]byte{'G'}, 32)
	lastJustifiedRoot, err := lastJustifiedBlk.Block.HashTreeRoot()
	require.NoError(t, err)
	newJustifiedBlk := testutil.NewBeaconBlock()
	newJustifiedBlk.Block.ParentRoot = bytesutil.PadTo(lastJustifiedRoot[:], 32)
	newJustifiedRoot, err := newJustifiedBlk.Block.HashTreeRoot()
	require.NoError(t, err)
	require.NoError(t, service.cfg.BeaconDB.SaveBlock(ctx, wrapper.WrappedPhase0SignedBeaconBlock(newJustifiedBlk)))
	require.NoError(t, service.cfg.BeaconDB.SaveBlock(ctx, wrapper.WrappedPhase0SignedBeaconBlock(lastJustifiedBlk)))

	diff := params.BeaconConfig().SlotsPerEpoch.Sub(1).Mul(params.BeaconConfig().SecondsPerSlot)
	service.genesisTime = time.Unix(time.Now().Unix()-int64(diff), 0)
	service.justifiedCheckpt = &ethpb.Checkpoint{Root: lastJustifiedRoot[:]}

	update, err := service.shouldUpdateCurrentJustified(ctx, &ethpb.Checkpoint{Root: newJustifiedRoot[:]})
	require.NoError(t, err)
	assert.Equal(t, false, update, "Should not be able to update justified, received true")
}

func TestCachedPreState_CanGetFromStateSummary(t *testing.T) {
	ctx := context.Background()
	beaconDB := testDB.SetupDB(t)

	cfg := &Config{
		BeaconDB: beaconDB,
		StateGen: stategen.New(beaconDB),
	}
	service, err := NewService(ctx, cfg)
	require.NoError(t, err)

	s, err := v1.InitializeFromProto(&pb.BeaconState{Slot: 1, GenesisValidatorsRoot: params.BeaconConfig().ZeroHash[:]})
	require.NoError(t, err)

	genesisStateRoot := [32]byte{}
	genesis := blocks.NewGenesisBlock(genesisStateRoot[:])
	assert.NoError(t, beaconDB.SaveBlock(ctx, wrapper.WrappedPhase0SignedBeaconBlock(genesis)))
	gRoot, err := genesis.Block.HashTreeRoot()
	require.NoError(t, err)
	service.finalizedCheckpt = &ethpb.Checkpoint{
		Root: gRoot[:],
	}
	service.cfg.ForkChoiceStore = protoarray.New(0, 0, [32]byte{})
	service.saveInitSyncBlock(gRoot, wrapper.WrappedPhase0SignedBeaconBlock(genesis))

	b := testutil.NewBeaconBlock()
	b.Block.Slot = 1
	b.Block.ParentRoot = gRoot[:]
	require.NoError(t, service.cfg.BeaconDB.SaveStateSummary(ctx, &pb.StateSummary{Slot: 1, Root: gRoot[:]}))
	require.NoError(t, service.cfg.StateGen.SaveState(ctx, gRoot, s))
	require.NoError(t, service.verifyBlkPreState(ctx, wrapper.WrappedPhase0BeaconBlock(b.Block)))
}

func TestCachedPreState_CanGetFromDB(t *testing.T) {
	ctx := context.Background()
	beaconDB := testDB.SetupDB(t)

	cfg := &Config{
		BeaconDB: beaconDB,
		StateGen: stategen.New(beaconDB),
	}
	service, err := NewService(ctx, cfg)
	require.NoError(t, err)

	genesisStateRoot := [32]byte{}
	genesis := blocks.NewGenesisBlock(genesisStateRoot[:])
	assert.NoError(t, beaconDB.SaveBlock(ctx, wrapper.WrappedPhase0SignedBeaconBlock(genesis)))
	gRoot, err := genesis.Block.HashTreeRoot()
	require.NoError(t, err)
	service.finalizedCheckpt = &ethpb.Checkpoint{
		Root: gRoot[:],
	}
	service.cfg.ForkChoiceStore = protoarray.New(0, 0, [32]byte{})
	service.saveInitSyncBlock(gRoot, wrapper.WrappedPhase0SignedBeaconBlock(genesis))

	b := testutil.NewBeaconBlock()
	b.Block.Slot = 1
	service.finalizedCheckpt = &ethpb.Checkpoint{Root: gRoot[:]}
	err = service.verifyBlkPreState(ctx, wrapper.WrappedPhase0BeaconBlock(b.Block))
	wanted := "could not reconstruct parent state"
	assert.ErrorContains(t, wanted, err)

	b.Block.ParentRoot = gRoot[:]
	s, err := v1.InitializeFromProto(&pb.BeaconState{Slot: 1})
	require.NoError(t, err)
	require.NoError(t, service.cfg.BeaconDB.SaveStateSummary(ctx, &pb.StateSummary{Slot: 1, Root: gRoot[:]}))
	require.NoError(t, service.cfg.StateGen.SaveState(ctx, gRoot, s))
	require.NoError(t, service.verifyBlkPreState(ctx, wrapper.WrappedPhase0SignedBeaconBlock(b).Block()))
}

func TestUpdateJustified_CouldUpdateBest(t *testing.T) {
	ctx := context.Background()
	beaconDB := testDB.SetupDB(t)

	cfg := &Config{BeaconDB: beaconDB, StateGen: stategen.New(beaconDB)}
	service, err := NewService(ctx, cfg)
	require.NoError(t, err)

	signedBlock := testutil.NewBeaconBlock()
	require.NoError(t, beaconDB.SaveBlock(ctx, wrapper.WrappedPhase0SignedBeaconBlock(signedBlock)))
	r, err := signedBlock.Block.HashTreeRoot()
	require.NoError(t, err)
	service.justifiedCheckpt = &ethpb.Checkpoint{Root: []byte{'A'}}
	service.bestJustifiedCheckpt = &ethpb.Checkpoint{Root: []byte{'A'}}
	st, err := testutil.NewBeaconState()
	require.NoError(t, err)
	require.NoError(t, beaconDB.SaveState(ctx, st.Copy(), r))

	// Could update
	s, err := testutil.NewBeaconState()
	require.NoError(t, err)
	require.NoError(t, s.SetCurrentJustifiedCheckpoint(&ethpb.Checkpoint{Epoch: 1, Root: r[:]}))
	require.NoError(t, service.updateJustified(context.Background(), s))

	assert.Equal(t, s.CurrentJustifiedCheckpoint().Epoch, service.bestJustifiedCheckpt.Epoch, "Incorrect justified epoch in service")

	// Could not update
	service.bestJustifiedCheckpt.Epoch = 2
	require.NoError(t, service.updateJustified(context.Background(), s))

	assert.Equal(t, types.Epoch(2), service.bestJustifiedCheckpt.Epoch, "Incorrect justified epoch in service")
}

func TestFillForkChoiceMissingBlocks_CanSave(t *testing.T) {
	ctx := context.Background()
	beaconDB := testDB.SetupDB(t)

	cfg := &Config{BeaconDB: beaconDB}
	service, err := NewService(ctx, cfg)
	require.NoError(t, err)
	service.cfg.ForkChoiceStore = protoarray.New(0, 0, [32]byte{'A'})
	service.finalizedCheckpt = &ethpb.Checkpoint{Root: make([]byte, 32)}

	genesisStateRoot := [32]byte{}
	genesis := blocks.NewGenesisBlock(genesisStateRoot[:])
	require.NoError(t, beaconDB.SaveBlock(ctx, wrapper.WrappedPhase0SignedBeaconBlock(genesis)))
	validGenesisRoot, err := genesis.Block.HashTreeRoot()
	require.NoError(t, err)
	st, err := testutil.NewBeaconState()
	require.NoError(t, err)

	require.NoError(t, service.cfg.BeaconDB.SaveState(ctx, st.Copy(), validGenesisRoot))
	roots, err := blockTree1(t, beaconDB, validGenesisRoot[:])
	require.NoError(t, err)

	beaconState, _ := testutil.DeterministicGenesisState(t, 32)
	block := testutil.NewBeaconBlock()
	block.Block.Slot = 9
	block.Block.ParentRoot = roots[8]

	err = service.fillInForkChoiceMissingBlocks(
		context.Background(), wrapper.WrappedPhase0SignedBeaconBlock(block).Block(), beaconState.FinalizedCheckpoint(), beaconState.CurrentJustifiedCheckpoint())
	require.NoError(t, err)

	// 5 nodes from the block tree 1. B0 - B3 - B4 - B6 - B8
	assert.Equal(t, 5, len(service.cfg.ForkChoiceStore.Nodes()), "Miss match nodes")
	assert.Equal(t, true, service.cfg.ForkChoiceStore.HasNode(bytesutil.ToBytes32(roots[4])), "Didn't save node")
	assert.Equal(t, true, service.cfg.ForkChoiceStore.HasNode(bytesutil.ToBytes32(roots[6])), "Didn't save node")
	assert.Equal(t, true, service.cfg.ForkChoiceStore.HasNode(bytesutil.ToBytes32(roots[8])), "Didn't save node")
}

func TestFillForkChoiceMissingBlocks_RootsMatch(t *testing.T) {
	ctx := context.Background()
	beaconDB := testDB.SetupDB(t)

	cfg := &Config{BeaconDB: beaconDB}
	service, err := NewService(ctx, cfg)
	require.NoError(t, err)
	service.cfg.ForkChoiceStore = protoarray.New(0, 0, [32]byte{'A'})
	service.finalizedCheckpt = &ethpb.Checkpoint{Root: make([]byte, 32)}

	genesisStateRoot := [32]byte{}
	genesis := blocks.NewGenesisBlock(genesisStateRoot[:])
	require.NoError(t, beaconDB.SaveBlock(ctx, wrapper.WrappedPhase0SignedBeaconBlock(genesis)))
	validGenesisRoot, err := genesis.Block.HashTreeRoot()
	require.NoError(t, err)
	st, err := testutil.NewBeaconState()
	require.NoError(t, err)

	require.NoError(t, service.cfg.BeaconDB.SaveState(ctx, st.Copy(), validGenesisRoot))
	roots, err := blockTree1(t, beaconDB, validGenesisRoot[:])
	require.NoError(t, err)

	beaconState, _ := testutil.DeterministicGenesisState(t, 32)
	block := testutil.NewBeaconBlock()
	block.Block.Slot = 9
	block.Block.ParentRoot = roots[8]

	err = service.fillInForkChoiceMissingBlocks(
		context.Background(), wrapper.WrappedPhase0SignedBeaconBlock(block).Block(), beaconState.FinalizedCheckpoint(), beaconState.CurrentJustifiedCheckpoint())
	require.NoError(t, err)

	// 5 nodes from the block tree 1. B0 - B3 - B4 - B6 - B8
	assert.Equal(t, 5, len(service.cfg.ForkChoiceStore.Nodes()), "Miss match nodes")
	// Ensure all roots and their respective blocks exist.
	wantedRoots := [][]byte{roots[0], roots[3], roots[4], roots[6], roots[8]}
	for i, rt := range wantedRoots {
		assert.Equal(t, true, service.cfg.ForkChoiceStore.HasNode(bytesutil.ToBytes32(rt)), fmt.Sprintf("Didn't save node: %d", i))
		assert.Equal(t, true, service.cfg.BeaconDB.HasBlock(context.Background(), bytesutil.ToBytes32(rt)))
	}
}

func TestFillForkChoiceMissingBlocks_FilterFinalized(t *testing.T) {
	ctx := context.Background()
	beaconDB := testDB.SetupDB(t)

	cfg := &Config{BeaconDB: beaconDB}
	service, err := NewService(ctx, cfg)
	require.NoError(t, err)
	service.cfg.ForkChoiceStore = protoarray.New(0, 0, [32]byte{'A'})
	// Set finalized epoch to 1.
	service.finalizedCheckpt = &ethpb.Checkpoint{Epoch: 1}

	genesisStateRoot := [32]byte{}
	genesis := blocks.NewGenesisBlock(genesisStateRoot[:])
	assert.NoError(t, beaconDB.SaveBlock(ctx, wrapper.WrappedPhase0SignedBeaconBlock(genesis)))
	validGenesisRoot, err := genesis.Block.HashTreeRoot()
	assert.NoError(t, err)
	st, err := testutil.NewBeaconState()
	require.NoError(t, err)

	require.NoError(t, service.cfg.BeaconDB.SaveState(ctx, st.Copy(), validGenesisRoot))

	// Define a tree branch, slot 63 <- 64 <- 65
	b63 := testutil.NewBeaconBlock()
	b63.Block.Slot = 63
	require.NoError(t, service.cfg.BeaconDB.SaveBlock(ctx, wrapper.WrappedPhase0SignedBeaconBlock(b63)))
	r63, err := b63.Block.HashTreeRoot()
	require.NoError(t, err)
	b64 := testutil.NewBeaconBlock()
	b64.Block.Slot = 64
	b64.Block.ParentRoot = r63[:]
	require.NoError(t, service.cfg.BeaconDB.SaveBlock(ctx, wrapper.WrappedPhase0SignedBeaconBlock(b64)))
	r64, err := b64.Block.HashTreeRoot()
	require.NoError(t, err)
	b65 := testutil.NewBeaconBlock()
	b65.Block.Slot = 65
	b65.Block.ParentRoot = r64[:]
	require.NoError(t, service.cfg.BeaconDB.SaveBlock(ctx, wrapper.WrappedPhase0SignedBeaconBlock(b65)))

	beaconState, _ := testutil.DeterministicGenesisState(t, 32)
	err = service.fillInForkChoiceMissingBlocks(
		context.Background(), wrapper.WrappedPhase0SignedBeaconBlock(b65).Block(), beaconState.FinalizedCheckpoint(), beaconState.CurrentJustifiedCheckpoint())
	require.NoError(t, err)

	// There should be 2 nodes, block 65 and block 64.
	assert.Equal(t, 2, len(service.cfg.ForkChoiceStore.Nodes()), "Miss match nodes")

	// Block with slot 63 should be in fork choice because it's less than finalized epoch 1.
	assert.Equal(t, true, service.cfg.ForkChoiceStore.HasNode(r63), "Didn't save node")
}

// blockTree1 constructs the following tree:
//    /- B1
// B0           /- B5 - B7
//    \- B3 - B4 - B6 - B8
// (B1, and B3 are all from the same slots)
func blockTree1(t *testing.T, beaconDB db.Database, genesisRoot []byte) ([][]byte, error) {
	genesisRoot = bytesutil.PadTo(genesisRoot, 32)
	b0 := testutil.NewBeaconBlock()
	b0.Block.Slot = 0
	b0.Block.ParentRoot = genesisRoot
	r0, err := b0.Block.HashTreeRoot()
	if err != nil {
		return nil, err
	}
	b1 := testutil.NewBeaconBlock()
	b1.Block.Slot = 1
	b1.Block.ParentRoot = r0[:]
	r1, err := b1.Block.HashTreeRoot()
	if err != nil {
		return nil, err
	}
	b3 := testutil.NewBeaconBlock()
	b3.Block.Slot = 3
	b3.Block.ParentRoot = r0[:]
	r3, err := b3.Block.HashTreeRoot()
	if err != nil {
		return nil, err
	}
	b4 := testutil.NewBeaconBlock()
	b4.Block.Slot = 4
	b4.Block.ParentRoot = r3[:]
	r4, err := b4.Block.HashTreeRoot()
	if err != nil {
		return nil, err
	}
	b5 := testutil.NewBeaconBlock()
	b5.Block.Slot = 5
	b5.Block.ParentRoot = r4[:]
	r5, err := b5.Block.HashTreeRoot()
	if err != nil {
		return nil, err
	}
	b6 := testutil.NewBeaconBlock()
	b6.Block.Slot = 6
	b6.Block.ParentRoot = r4[:]
	r6, err := b6.Block.HashTreeRoot()
	if err != nil {
		return nil, err
	}
	b7 := testutil.NewBeaconBlock()
	b7.Block.Slot = 7
	b7.Block.ParentRoot = r5[:]
	r7, err := b7.Block.HashTreeRoot()
	if err != nil {
		return nil, err
	}
	b8 := testutil.NewBeaconBlock()
	b8.Block.Slot = 8
	b8.Block.ParentRoot = r6[:]
	r8, err := b8.Block.HashTreeRoot()
	if err != nil {
		return nil, err
	}
	st, err := testutil.NewBeaconState()
	require.NoError(t, err)

	for _, b := range []*ethpb.SignedBeaconBlock{b0, b1, b3, b4, b5, b6, b7, b8} {
		beaconBlock := testutil.NewBeaconBlock()
		beaconBlock.Block.Slot = b.Block.Slot
		beaconBlock.Block.ParentRoot = bytesutil.PadTo(b.Block.ParentRoot, 32)
		if err := beaconDB.SaveBlock(context.Background(), wrapper.WrappedPhase0SignedBeaconBlock(beaconBlock)); err != nil {
			return nil, err
		}
		if err := beaconDB.SaveState(context.Background(), st.Copy(), bytesutil.ToBytes32(beaconBlock.Block.ParentRoot)); err != nil {
			return nil, errors.Wrap(err, "could not save state")
		}
	}
	if err := beaconDB.SaveState(context.Background(), st.Copy(), r1); err != nil {
		return nil, err
	}
	if err := beaconDB.SaveState(context.Background(), st.Copy(), r7); err != nil {
		return nil, err
	}
	if err := beaconDB.SaveState(context.Background(), st.Copy(), r8); err != nil {
		return nil, err
	}
	return [][]byte{r0[:], r1[:], nil, r3[:], r4[:], r5[:], r6[:], r7[:], r8[:]}, nil
}

func TestCurrentSlot_HandlesOverflow(t *testing.T) {
	svc := Service{genesisTime: timeutils.Now().Add(1 * time.Hour)}

	slot := svc.CurrentSlot()
	require.Equal(t, types.Slot(0), slot, "Unexpected slot")
}
func TestAncestorByDB_CtxErr(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	service, err := NewService(ctx, &Config{})
	require.NoError(t, err)

	cancel()
	_, err = service.ancestorByDB(ctx, [32]byte{}, 0)
	require.ErrorContains(t, "context canceled", err)
}

func TestAncestor_HandleSkipSlot(t *testing.T) {
	ctx := context.Background()
	beaconDB := testDB.SetupDB(t)

	cfg := &Config{BeaconDB: beaconDB, ForkChoiceStore: protoarray.New(0, 0, [32]byte{})}
	service, err := NewService(ctx, cfg)
	require.NoError(t, err)

	b1 := testutil.NewBeaconBlock()
	b1.Block.Slot = 1
	b1.Block.ParentRoot = bytesutil.PadTo([]byte{'a'}, 32)
	r1, err := b1.Block.HashTreeRoot()
	require.NoError(t, err)
	b100 := testutil.NewBeaconBlock()
	b100.Block.Slot = 100
	b100.Block.ParentRoot = r1[:]
	r100, err := b100.Block.HashTreeRoot()
	require.NoError(t, err)
	b200 := testutil.NewBeaconBlock()
	b200.Block.Slot = 200
	b200.Block.ParentRoot = r100[:]
	r200, err := b200.Block.HashTreeRoot()
	require.NoError(t, err)
	for _, b := range []*ethpb.SignedBeaconBlock{b1, b100, b200} {
		beaconBlock := testutil.NewBeaconBlock()
		beaconBlock.Block.Slot = b.Block.Slot
		beaconBlock.Block.ParentRoot = bytesutil.PadTo(b.Block.ParentRoot, 32)
		require.NoError(t, beaconDB.SaveBlock(context.Background(), wrapper.WrappedPhase0SignedBeaconBlock(beaconBlock)))
	}

	// Slots 100 to 200 are skip slots. Requesting root at 150 will yield root at 100. The last physical block.
	r, err := service.ancestor(context.Background(), r200[:], 150)
	require.NoError(t, err)
	if bytesutil.ToBytes32(r) != r100 {
		t.Error("Did not get correct root")
	}

	// Slots 1 to 100 are skip slots. Requesting root at 50 will yield root at 1. The last physical block.
	r, err = service.ancestor(context.Background(), r200[:], 50)
	require.NoError(t, err)
	if bytesutil.ToBytes32(r) != r1 {
		t.Error("Did not get correct root")
	}
}

func TestAncestor_CanUseForkchoice(t *testing.T) {
	ctx := context.Background()
	cfg := &Config{ForkChoiceStore: protoarray.New(0, 0, [32]byte{})}
	service, err := NewService(ctx, cfg)
	require.NoError(t, err)

	b1 := testutil.NewBeaconBlock()
	b1.Block.Slot = 1
	b1.Block.ParentRoot = bytesutil.PadTo([]byte{'a'}, 32)
	r1, err := b1.Block.HashTreeRoot()
	require.NoError(t, err)
	b100 := testutil.NewBeaconBlock()
	b100.Block.Slot = 100
	b100.Block.ParentRoot = r1[:]
	r100, err := b100.Block.HashTreeRoot()
	require.NoError(t, err)
	b200 := testutil.NewBeaconBlock()
	b200.Block.Slot = 200
	b200.Block.ParentRoot = r100[:]
	r200, err := b200.Block.HashTreeRoot()
	require.NoError(t, err)
	for _, b := range []*ethpb.SignedBeaconBlock{b1, b100, b200} {
		beaconBlock := testutil.NewBeaconBlock()
		beaconBlock.Block.Slot = b.Block.Slot
		beaconBlock.Block.ParentRoot = bytesutil.PadTo(b.Block.ParentRoot, 32)
		r, err := b.Block.HashTreeRoot()
		require.NoError(t, err)
		require.NoError(t, service.cfg.ForkChoiceStore.ProcessBlock(context.Background(), b.Block.Slot, r, bytesutil.ToBytes32(b.Block.ParentRoot), [32]byte{}, 0, 0)) // Saves blocks to fork choice store.
	}

	r, err := service.ancestor(context.Background(), r200[:], 150)
	require.NoError(t, err)
	if bytesutil.ToBytes32(r) != r100 {
		t.Error("Did not get correct root")
	}
}

func TestAncestor_CanUseDB(t *testing.T) {
	ctx := context.Background()
	beaconDB := testDB.SetupDB(t)

	cfg := &Config{BeaconDB: beaconDB, ForkChoiceStore: protoarray.New(0, 0, [32]byte{})}
	service, err := NewService(ctx, cfg)
	require.NoError(t, err)

	b1 := testutil.NewBeaconBlock()
	b1.Block.Slot = 1
	b1.Block.ParentRoot = bytesutil.PadTo([]byte{'a'}, 32)
	r1, err := b1.Block.HashTreeRoot()
	require.NoError(t, err)
	b100 := testutil.NewBeaconBlock()
	b100.Block.Slot = 100
	b100.Block.ParentRoot = r1[:]
	r100, err := b100.Block.HashTreeRoot()
	require.NoError(t, err)
	b200 := testutil.NewBeaconBlock()
	b200.Block.Slot = 200
	b200.Block.ParentRoot = r100[:]
	r200, err := b200.Block.HashTreeRoot()
	require.NoError(t, err)
	for _, b := range []*ethpb.SignedBeaconBlock{b1, b100, b200} {
		beaconBlock := testutil.NewBeaconBlock()
		beaconBlock.Block.Slot = b.Block.Slot
		beaconBlock.Block.ParentRoot = bytesutil.PadTo(b.Block.ParentRoot, 32)
		require.NoError(t, beaconDB.SaveBlock(context.Background(), wrapper.WrappedPhase0SignedBeaconBlock(beaconBlock))) // Saves blocks to DB.
	}

	require.NoError(t, service.cfg.ForkChoiceStore.ProcessBlock(context.Background(), 200, r200, r200, [32]byte{}, 0, 0))

	r, err := service.ancestor(context.Background(), r200[:], 150)
	require.NoError(t, err)
	if bytesutil.ToBytes32(r) != r100 {
		t.Error("Did not get correct root")
	}
}

func TestEnsureRootNotZeroHashes(t *testing.T) {
	ctx := context.Background()
	cfg := &Config{}
	service, err := NewService(ctx, cfg)
	require.NoError(t, err)
	service.genesisRoot = [32]byte{'a'}

	r := service.ensureRootNotZeros(params.BeaconConfig().ZeroHash)
	assert.Equal(t, service.genesisRoot, r, "Did not get wanted justified root")
	root := [32]byte{'b'}
	r = service.ensureRootNotZeros(root)
	assert.Equal(t, root, r, "Did not get wanted justified root")
}

func TestFinalizedImpliesNewJustified(t *testing.T) {
	beaconDB := testDB.SetupDB(t)
	ctx := context.Background()
	type args struct {
		cachedCheckPoint        *ethpb.Checkpoint
		stateCheckPoint         *ethpb.Checkpoint
		diffFinalizedCheckPoint bool
	}
	tests := []struct {
		name string
		args args
		want *ethpb.Checkpoint
	}{
		{
			name: "Same justified, do nothing",
			args: args{
				cachedCheckPoint: &ethpb.Checkpoint{Epoch: 1, Root: []byte{'a'}},
				stateCheckPoint:  &ethpb.Checkpoint{Epoch: 1, Root: []byte{'a'}},
			},
			want: &ethpb.Checkpoint{Epoch: 1, Root: []byte{'a'}},
		},
		{
			name: "Different justified, higher epoch, cache new justified",
			args: args{
				cachedCheckPoint: &ethpb.Checkpoint{Epoch: 1, Root: []byte{'a'}},
				stateCheckPoint:  &ethpb.Checkpoint{Epoch: 2, Root: []byte{'b'}},
			},
			want: &ethpb.Checkpoint{Epoch: 2, Root: []byte{'b'}},
		},
		{
			name: "finalized has different justified, cache new justified",
			args: args{
				cachedCheckPoint:        &ethpb.Checkpoint{Epoch: 1, Root: []byte{'a'}},
				stateCheckPoint:         &ethpb.Checkpoint{Epoch: 1, Root: []byte{'b'}},
				diffFinalizedCheckPoint: true,
			},
			want: &ethpb.Checkpoint{Epoch: 1, Root: []byte{'b'}},
		},
	}
	for _, test := range tests {
		beaconState, err := testutil.NewBeaconState()
		require.NoError(t, err)
		require.NoError(t, beaconState.SetCurrentJustifiedCheckpoint(test.args.stateCheckPoint))
		service, err := NewService(ctx, &Config{BeaconDB: beaconDB, StateGen: stategen.New(beaconDB), ForkChoiceStore: protoarray.New(0, 0, [32]byte{})})
		require.NoError(t, err)
		service.justifiedCheckpt = test.args.cachedCheckPoint
		require.NoError(t, service.cfg.BeaconDB.SaveStateSummary(ctx, &pb.StateSummary{Root: bytesutil.PadTo(test.want.Root, 32)}))
		genesisState, err := testutil.NewBeaconState()
		require.NoError(t, err)
		require.NoError(t, service.cfg.BeaconDB.SaveState(ctx, genesisState, bytesutil.ToBytes32(test.want.Root)))

		if test.args.diffFinalizedCheckPoint {
			b1 := testutil.NewBeaconBlock()
			b1.Block.Slot = 1
			b1.Block.ParentRoot = bytesutil.PadTo([]byte{'a'}, 32)
			r1, err := b1.Block.HashTreeRoot()
			require.NoError(t, err)
			b100 := testutil.NewBeaconBlock()
			b100.Block.Slot = 100
			b100.Block.ParentRoot = r1[:]
			r100, err := b100.Block.HashTreeRoot()
			require.NoError(t, err)
			for _, b := range []*ethpb.SignedBeaconBlock{b1, b100} {
				beaconBlock := testutil.NewBeaconBlock()
				beaconBlock.Block.Slot = b.Block.Slot
				beaconBlock.Block.ParentRoot = bytesutil.PadTo(b.Block.ParentRoot, 32)
				require.NoError(t, service.cfg.BeaconDB.SaveBlock(context.Background(), wrapper.WrappedPhase0SignedBeaconBlock(beaconBlock)))
			}
			service.finalizedCheckpt = &ethpb.Checkpoint{Root: []byte{'c'}, Epoch: 1}
			service.justifiedCheckpt.Root = r100[:]
		}

		require.NoError(t, service.finalizedImpliesNewJustified(ctx, beaconState))
		assert.Equal(t, true, attestationutil.CheckPointIsEqual(test.want, service.justifiedCheckpt), "Did not get wanted check point")
	}
}

func TestVerifyBlkDescendant(t *testing.T) {
	beaconDB := testDB.SetupDB(t)
	ctx := context.Background()

	b := testutil.NewBeaconBlock()
	b.Block.Slot = 1
	r, err := b.Block.HashTreeRoot()
	require.NoError(t, err)
	require.NoError(t, beaconDB.SaveBlock(ctx, wrapper.WrappedPhase0SignedBeaconBlock(b)))

	b1 := testutil.NewBeaconBlock()
	b1.Block.Slot = 1
	b1.Block.Body.Graffiti = bytesutil.PadTo([]byte{'a'}, 32)
	r1, err := b1.Block.HashTreeRoot()
	require.NoError(t, err)
	require.NoError(t, beaconDB.SaveBlock(ctx, wrapper.WrappedPhase0SignedBeaconBlock(b1)))

	type args struct {
		parentRoot    [32]byte
		finalizedRoot [32]byte
	}
	tests := []struct {
		name      string
		args      args
		wantedErr string
	}{
		{
			name: "could not get finalized block in block service cache",
			args: args{
				finalizedRoot: [32]byte{'a'},
			},
			wantedErr: "nil finalized block",
		},
		{
			name: "could not get finalized block root in DB",
			args: args{
				finalizedRoot: r,
				parentRoot:    [32]byte{'a'},
			},
			wantedErr: "could not get finalized block root",
		},
		{
			name: "is not descendant",
			args: args{
				finalizedRoot: r1,
				parentRoot:    r,
			},
			wantedErr: "is not a descendent of the current finalized block slot",
		},
		{
			name: "is descendant",
			args: args{
				finalizedRoot: r,
				parentRoot:    r,
			},
		},
	}
	for _, tt := range tests {
		service, err := NewService(ctx, &Config{BeaconDB: beaconDB, StateGen: stategen.New(beaconDB), ForkChoiceStore: protoarray.New(0, 0, [32]byte{})})
		require.NoError(t, err)
		service.finalizedCheckpt = &ethpb.Checkpoint{
			Root: tt.args.finalizedRoot[:],
		}
		err = service.VerifyBlkDescendant(ctx, tt.args.parentRoot)
		if tt.wantedErr != "" {
			assert.ErrorContains(t, tt.wantedErr, err)
		} else if err != nil {
			assert.NoError(t, err)
		}
	}
}

func TestUpdateJustifiedInitSync(t *testing.T) {
	beaconDB := testDB.SetupDB(t)
	ctx := context.Background()
	cfg := &Config{BeaconDB: beaconDB}
	service, err := NewService(ctx, cfg)
	require.NoError(t, err)

	gBlk := testutil.NewBeaconBlock()
	gRoot, err := gBlk.Block.HashTreeRoot()
	require.NoError(t, err)
	require.NoError(t, service.cfg.BeaconDB.SaveBlock(ctx, wrapper.WrappedPhase0SignedBeaconBlock(gBlk)))
	require.NoError(t, service.cfg.BeaconDB.SaveGenesisBlockRoot(ctx, gRoot))
	require.NoError(t, service.cfg.BeaconDB.SaveStateSummary(ctx, &pb.StateSummary{Root: gRoot[:]}))
	beaconState, _ := testutil.DeterministicGenesisState(t, 32)
	require.NoError(t, service.cfg.BeaconDB.SaveState(ctx, beaconState, gRoot))
	service.genesisRoot = gRoot
	currentCp := &ethpb.Checkpoint{Epoch: 1}
	service.justifiedCheckpt = currentCp
	newCp := &ethpb.Checkpoint{Epoch: 2, Root: gRoot[:]}

	require.NoError(t, service.updateJustifiedInitSync(ctx, newCp))

	assert.DeepSSZEqual(t, currentCp, service.prevJustifiedCheckpt, "Incorrect previous justified checkpoint")
	assert.DeepSSZEqual(t, newCp, service.CurrentJustifiedCheckpt(), "Incorrect current justified checkpoint in cache")
	cp, err := service.cfg.BeaconDB.JustifiedCheckpoint(ctx)
	require.NoError(t, err)
	assert.DeepSSZEqual(t, newCp, cp, "Incorrect current justified checkpoint in db")
}

func TestHandleEpochBoundary_BadMetrics(t *testing.T) {
	ctx := context.Background()
	cfg := &Config{}
	service, err := NewService(ctx, cfg)
	require.NoError(t, err)

	s, err := testutil.NewBeaconState()
	require.NoError(t, err)
	require.NoError(t, s.SetSlot(1))
	service.head = &head{state: (*v1.BeaconState)(nil)}

	require.ErrorContains(t, "failed to initialize precompute: nil inner state", service.handleEpochBoundary(ctx, s))
}

func TestHandleEpochBoundary_UpdateFirstSlot(t *testing.T) {
	ctx := context.Background()
	cfg := &Config{}
	service, err := NewService(ctx, cfg)
	require.NoError(t, err)

	s, _ := testutil.DeterministicGenesisState(t, 1024)
	service.head = &head{state: s}
	require.NoError(t, s.SetSlot(2*params.BeaconConfig().SlotsPerEpoch))
	require.NoError(t, service.handleEpochBoundary(ctx, s))
	require.Equal(t, 3*params.BeaconConfig().SlotsPerEpoch, service.nextEpochBoundarySlot)
}

func TestOnBlock_CanFinalize(t *testing.T) {
	ctx := context.Background()
	beaconDB := testDB.SetupDB(t)
	depositCache, err := depositcache.New()
	require.NoError(t, err)
	cfg := &Config{
		BeaconDB:        beaconDB,
		StateGen:        stategen.New(beaconDB),
		ForkChoiceStore: protoarray.New(0, 0, [32]byte{}),
		DepositCache:    depositCache,
		StateNotifier:   &mock.MockStateNotifier{},
	}
	service, err := NewService(ctx, cfg)
	require.NoError(t, err)

	gs, keys := testutil.DeterministicGenesisState(t, 32)
	require.NoError(t, service.saveGenesisData(ctx, gs))
	gBlk, err := service.cfg.BeaconDB.GenesisBlock(ctx)
	require.NoError(t, err)
	gRoot, err := gBlk.Block().HashTreeRoot()
	require.NoError(t, err)
	service.finalizedCheckpt = &ethpb.Checkpoint{Root: gRoot[:]}

	testState := gs.Copy()
	for i := types.Slot(1); i <= 4*params.BeaconConfig().SlotsPerEpoch; i++ {
		blk, err := testutil.GenerateFullBlock(testState, keys, testutil.DefaultBlockGenConfig(), i)
		require.NoError(t, err)
		r, err := blk.Block.HashTreeRoot()
		require.NoError(t, err)
		require.NoError(t, service.onBlock(ctx, wrapper.WrappedPhase0SignedBeaconBlock(blk), r))
		testState, err = service.cfg.StateGen.StateByRoot(ctx, r)
		require.NoError(t, err)
	}
	require.Equal(t, types.Epoch(3), service.CurrentJustifiedCheckpt().Epoch)
	require.Equal(t, types.Epoch(2), service.FinalizedCheckpt().Epoch)
}

func TestInsertFinalizedDeposits(t *testing.T) {
	ctx := context.Background()
	beaconDB := testDB.SetupDB(t)
	depositCache, err := depositcache.New()
	require.NoError(t, err)
	cfg := &Config{
		BeaconDB:        beaconDB,
		StateGen:        stategen.New(beaconDB),
		ForkChoiceStore: protoarray.New(0, 0, [32]byte{}),
		DepositCache:    depositCache,
	}
	service, err := NewService(ctx, cfg)
	require.NoError(t, err)

	gs, _ := testutil.DeterministicGenesisState(t, 32)
	require.NoError(t, service.saveGenesisData(ctx, gs))
	gBlk, err := service.cfg.BeaconDB.GenesisBlock(ctx)
	require.NoError(t, err)
	gRoot, err := gBlk.Block().HashTreeRoot()
	require.NoError(t, err)
	service.finalizedCheckpt = &ethpb.Checkpoint{Root: gRoot[:]}
	gs = gs.Copy()
	assert.NoError(t, gs.SetEth1Data(&ethpb.Eth1Data{DepositCount: 10}))
	assert.NoError(t, service.cfg.StateGen.SaveState(ctx, [32]byte{'m', 'o', 'c', 'k'}, gs))
	zeroSig := [96]byte{}
	for i := uint64(0); i < uint64(4*params.BeaconConfig().SlotsPerEpoch); i++ {
		root := []byte(strconv.Itoa(int(i)))
		assert.NoError(t, depositCache.InsertDeposit(ctx, &ethpb.Deposit{Data: &ethpb.Deposit_Data{
			PublicKey:             bytesutil.FromBytes48([48]byte{}),
			WithdrawalCredentials: params.BeaconConfig().ZeroHash[:],
			Amount:                0,
			Signature:             zeroSig[:],
		}, Proof: [][]byte{root}}, 100+i, int64(i), bytesutil.ToBytes32(root)))
	}
	assert.NoError(t, service.insertFinalizedDeposits(ctx, [32]byte{'m', 'o', 'c', 'k'}))
	fDeposits := depositCache.FinalizedDeposits(ctx)
	assert.Equal(t, 9, int(fDeposits.MerkleTrieIndex), "Finalized deposits not inserted correctly")
	deps := depositCache.AllDeposits(ctx, big.NewInt(109))
	for _, d := range deps {
		assert.DeepEqual(t, [][]byte(nil), d.Proof, "Proofs are not empty")
	}
}
