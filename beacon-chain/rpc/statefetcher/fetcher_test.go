package statefetcher

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common/hexutil"
	types "github.com/prysmaticlabs/eth2-types"
	eth "github.com/prysmaticlabs/ethereumapis/eth/v1alpha1"
	chainMock "github.com/prysmaticlabs/prysm/beacon-chain/blockchain/testing"
	testDB "github.com/prysmaticlabs/prysm/beacon-chain/db/testing"
	"github.com/prysmaticlabs/prysm/beacon-chain/state/stategen"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/prysmaticlabs/prysm/shared/interfaces"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/testutil"
	"github.com/prysmaticlabs/prysm/shared/testutil/assert"
	"github.com/prysmaticlabs/prysm/shared/testutil/require"
)

func TestGetState(t *testing.T) {
	ctx := context.Background()

	headSlot := types.Slot(123)
	fillSlot := func(state *pb.BeaconState) error {
		state.Slot = headSlot
		return nil
	}
	state, err := testutil.NewBeaconState(testutil.FillRootsNaturalOpt, fillSlot)
	require.NoError(t, err)
	stateRoot, err := state.HashTreeRoot(ctx)
	require.NoError(t, err)

	t.Run("Head", func(t *testing.T) {
		p := StateProvider{
			ChainInfoFetcher: &chainMock.ChainService{State: state},
		}

		s, err := p.State(ctx, []byte("head"))
		require.NoError(t, err)
		sRoot, err := s.HashTreeRoot(ctx)
		require.NoError(t, err)
		assert.DeepEqual(t, stateRoot, sRoot)
	})

	t.Run("Genesis", func(t *testing.T) {
		params.SetupTestConfigCleanup(t)
		cfg := params.BeaconConfig()
		cfg.ConfigName = "test"
		params.OverrideBeaconConfig(cfg)

		db := testDB.SetupDB(t)
		b := testutil.NewBeaconBlock()
		b.Block.StateRoot = bytesutil.PadTo([]byte("foo"), 32)
		require.NoError(t, db.SaveBlock(ctx, interfaces.WrappedPhase0SignedBeaconBlock(b)))
		r, err := b.Block.HashTreeRoot()
		require.NoError(t, err)

		state, err := testutil.NewBeaconState(func(state *pb.BeaconState) error {
			state.BlockRoots[0] = r[:]
			return nil
		})
		require.NoError(t, err)
		stateRoot, err := state.HashTreeRoot(ctx)
		require.NoError(t, err)

		require.NoError(t, db.SaveStateSummary(ctx, &pb.StateSummary{Root: r[:]}))
		require.NoError(t, db.SaveGenesisBlockRoot(ctx, r))
		require.NoError(t, db.SaveState(ctx, state, r))

		p := StateProvider{
			BeaconDB: db,
		}

		s, err := p.State(ctx, []byte("genesis"))
		require.NoError(t, err)
		sRoot, err := s.HashTreeRoot(ctx)
		require.NoError(t, err)
		assert.DeepEqual(t, stateRoot, sRoot)
	})

	t.Run("Finalized", func(t *testing.T) {
		stateGen := stategen.NewMockService()
		stateGen.StatesByRoot[stateRoot] = state

		p := StateProvider{
			ChainInfoFetcher: &chainMock.ChainService{
				FinalizedCheckPoint: &eth.Checkpoint{
					Root: stateRoot[:],
				},
			},
			StateGenService: stateGen,
		}

		s, err := p.State(ctx, []byte("finalized"))
		require.NoError(t, err)
		sRoot, err := s.HashTreeRoot(ctx)
		require.NoError(t, err)
		assert.Equal(t, stateRoot, sRoot)
	})

	t.Run("Justified", func(t *testing.T) {
		stateGen := stategen.NewMockService()
		stateGen.StatesByRoot[stateRoot] = state

		p := StateProvider{
			ChainInfoFetcher: &chainMock.ChainService{
				CurrentJustifiedCheckPoint: &eth.Checkpoint{
					Root: stateRoot[:],
				},
			},
			StateGenService: stateGen,
		}

		s, err := p.State(ctx, []byte("justified"))
		require.NoError(t, err)
		sRoot, err := s.HashTreeRoot(ctx)
		require.NoError(t, err)
		assert.DeepEqual(t, stateRoot, sRoot)
	})

	t.Run("Hex root", func(t *testing.T) {
		stateId, err := hexutil.Decode("0x" + strings.Repeat("0", 63) + "1")
		require.NoError(t, err)
		stateGen := stategen.NewMockService()
		stateGen.StatesByRoot[bytesutil.ToBytes32(stateId)] = state

		p := StateProvider{
			ChainInfoFetcher: &chainMock.ChainService{State: state},
			StateGenService:  stateGen,
		}

		s, err := p.State(ctx, stateId)
		require.NoError(t, err)
		sRoot, err := s.HashTreeRoot(ctx)
		require.NoError(t, err)
		assert.DeepEqual(t, stateRoot, sRoot)
	})

	t.Run("Hex root not found", func(t *testing.T) {
		p := StateProvider{
			ChainInfoFetcher: &chainMock.ChainService{State: state},
		}
		stateId, err := hexutil.Decode("0x" + strings.Repeat("f", 64))
		require.NoError(t, err)
		_, err = p.State(ctx, stateId)
		require.ErrorContains(t, "state not found in the last 8192 state roots", err)
	})

	t.Run("Slot", func(t *testing.T) {
		stateGen := stategen.NewMockService()
		stateGen.StatesBySlot[headSlot] = state

		p := StateProvider{
			GenesisTimeFetcher: &chainMock.ChainService{Slot: &headSlot},
			StateGenService:    stateGen,
		}

		s, err := p.State(ctx, []byte(strconv.FormatUint(uint64(headSlot), 10)))
		require.NoError(t, err)
		sRoot, err := s.HashTreeRoot(ctx)
		require.NoError(t, err)
		assert.Equal(t, stateRoot, sRoot)
	})

	t.Run("Slot too big", func(t *testing.T) {
		p := StateProvider{
			GenesisTimeFetcher: &chainMock.ChainService{
				Genesis: time.Now(),
			},
		}
		_, err := p.State(ctx, []byte(strconv.FormatUint(1, 10)))
		assert.ErrorContains(t, "slot cannot be in the future", err)
	})

	t.Run("Invalid state", func(t *testing.T) {
		p := StateProvider{}
		_, err := p.State(ctx, []byte("foo"))
		require.ErrorContains(t, "could not parse state ID", err)
	})
}

func TestGetStateRoot(t *testing.T) {
	ctx := context.Background()

	headSlot := types.Slot(123)
	fillSlot := func(state *pb.BeaconState) error {
		state.Slot = headSlot
		return nil
	}
	state, err := testutil.NewBeaconState(testutil.FillRootsNaturalOpt, fillSlot)
	require.NoError(t, err)
	stateRoot, err := state.HashTreeRoot(ctx)
	require.NoError(t, err)

	t.Run("Head", func(t *testing.T) {
		b := testutil.NewBeaconBlock()
		b.Block.StateRoot = stateRoot[:]
		p := StateProvider{
			ChainInfoFetcher: &chainMock.ChainService{
				State: state,
				Block: interfaces.WrappedPhase0SignedBeaconBlock(b),
			},
		}

		s, err := p.StateRoot(ctx, []byte("head"))
		require.NoError(t, err)
		assert.DeepEqual(t, stateRoot[:], s)
	})

	t.Run("Genesis", func(t *testing.T) {
		db := testDB.SetupDB(t)
		b := testutil.NewBeaconBlock()
		require.NoError(t, db.SaveBlock(ctx, interfaces.WrappedPhase0SignedBeaconBlock(b)))
		r, err := b.Block.HashTreeRoot()
		require.NoError(t, err)

		state, err := testutil.NewBeaconState(func(state *pb.BeaconState) error {
			state.BlockRoots[0] = r[:]
			return nil
		})
		require.NoError(t, err)

		require.NoError(t, db.SaveStateSummary(ctx, &pb.StateSummary{Root: r[:]}))
		require.NoError(t, db.SaveGenesisBlockRoot(ctx, r))
		require.NoError(t, db.SaveState(ctx, state, r))

		p := StateProvider{
			BeaconDB: db,
		}

		s, err := p.StateRoot(ctx, []byte("genesis"))
		require.NoError(t, err)
		genesisBlock, err := db.GenesisBlock(ctx)
		require.NoError(t, err)
		assert.DeepEqual(t, genesisBlock.Block().StateRoot(), s)
	})

	t.Run("Finalized", func(t *testing.T) {
		db := testDB.SetupDB(t)
		genesis := bytesutil.ToBytes32([]byte("genesis"))
		require.NoError(t, db.SaveGenesisBlockRoot(ctx, genesis))
		blk := testutil.NewBeaconBlock()
		blk.Block.ParentRoot = genesis[:]
		blk.Block.Slot = 40
		root, err := blk.Block.HashTreeRoot()
		require.NoError(t, err)
		cp := &eth.Checkpoint{
			Epoch: 5,
			Root:  root[:],
		}
		// a valid chain is required to save finalized checkpoint.
		require.NoError(t, db.SaveBlock(ctx, interfaces.WrappedPhase0SignedBeaconBlock(blk)))
		st, err := testutil.NewBeaconState()
		require.NoError(t, err)
		require.NoError(t, st.SetSlot(1))
		// a state is required to save checkpoint
		require.NoError(t, db.SaveState(ctx, st, root))
		require.NoError(t, db.SaveFinalizedCheckpoint(ctx, cp))

		p := StateProvider{
			BeaconDB: db,
		}

		s, err := p.StateRoot(ctx, []byte("finalized"))
		require.NoError(t, err)
		assert.DeepEqual(t, blk.Block.StateRoot, s)
	})

	t.Run("Justified", func(t *testing.T) {
		db := testDB.SetupDB(t)
		genesis := bytesutil.ToBytes32([]byte("genesis"))
		require.NoError(t, db.SaveGenesisBlockRoot(ctx, genesis))
		blk := testutil.NewBeaconBlock()
		blk.Block.ParentRoot = genesis[:]
		blk.Block.Slot = 40
		root, err := blk.Block.HashTreeRoot()
		require.NoError(t, err)
		cp := &eth.Checkpoint{
			Epoch: 5,
			Root:  root[:],
		}
		// a valid chain is required to save finalized checkpoint.
		require.NoError(t, db.SaveBlock(ctx, interfaces.WrappedPhase0SignedBeaconBlock(blk)))
		st, err := testutil.NewBeaconState()
		require.NoError(t, err)
		require.NoError(t, st.SetSlot(1))
		// a state is required to save checkpoint
		require.NoError(t, db.SaveState(ctx, st, root))
		require.NoError(t, db.SaveJustifiedCheckpoint(ctx, cp))

		p := StateProvider{
			BeaconDB: db,
		}

		s, err := p.StateRoot(ctx, []byte("justified"))
		require.NoError(t, err)
		assert.DeepEqual(t, blk.Block.StateRoot, s)
	})

	t.Run("Hex root", func(t *testing.T) {
		stateId, err := hexutil.Decode("0x" + strings.Repeat("0", 63) + "1")
		require.NoError(t, err)

		p := StateProvider{
			ChainInfoFetcher: &chainMock.ChainService{State: state},
		}

		s, err := p.StateRoot(ctx, stateId)
		require.NoError(t, err)
		assert.DeepEqual(t, stateId, s)
	})

	t.Run("Hex root not found", func(t *testing.T) {
		p := StateProvider{
			ChainInfoFetcher: &chainMock.ChainService{State: state},
		}
		stateId, err := hexutil.Decode("0x" + strings.Repeat("f", 64))
		require.NoError(t, err)
		_, err = p.StateRoot(ctx, stateId)
		require.ErrorContains(t, "state root not found in the last 8192 state roots", err)
	})

	t.Run("Slot", func(t *testing.T) {
		db := testDB.SetupDB(t)
		genesis := bytesutil.ToBytes32([]byte("genesis"))
		require.NoError(t, db.SaveGenesisBlockRoot(ctx, genesis))
		blk := testutil.NewBeaconBlock()
		blk.Block.ParentRoot = genesis[:]
		blk.Block.Slot = 40
		root, err := blk.Block.HashTreeRoot()
		require.NoError(t, err)
		require.NoError(t, db.SaveBlock(ctx, interfaces.WrappedPhase0SignedBeaconBlock(blk)))
		st, err := testutil.NewBeaconState()
		require.NoError(t, err)
		require.NoError(t, st.SetSlot(1))
		// a state is required to save checkpoint
		require.NoError(t, db.SaveState(ctx, st, root))

		slot := types.Slot(40)
		p := StateProvider{
			GenesisTimeFetcher: &chainMock.ChainService{Slot: &slot},
			BeaconDB:           db,
		}

		s, err := p.StateRoot(ctx, []byte(strconv.FormatUint(uint64(slot), 10)))
		require.NoError(t, err)
		assert.DeepEqual(t, blk.Block.StateRoot, s)
	})

	t.Run("Slot too big", func(t *testing.T) {
		p := StateProvider{
			GenesisTimeFetcher: &chainMock.ChainService{
				Genesis: time.Now(),
			},
		}
		_, err := p.StateRoot(ctx, []byte(strconv.FormatUint(1, 10)))
		assert.ErrorContains(t, "slot cannot be in the future", err)
	})

	t.Run("Invalid state", func(t *testing.T) {
		p := StateProvider{}
		_, err := p.StateRoot(ctx, []byte("foo"))
		require.ErrorContains(t, "could not parse state ID", err)
	})
}

func TestNewStateNotFoundError(t *testing.T) {
	e := NewStateNotFoundError(100)
	assert.Equal(t, "state not found in the last 100 state roots", e.message)
}
