package stategen

import (
	"context"
	"testing"

	types "github.com/prysmaticlabs/eth2-types"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/blocks"
	"github.com/prysmaticlabs/prysm/beacon-chain/db"
	testDB "github.com/prysmaticlabs/prysm/beacon-chain/db/testing"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	ethpb "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/proto/eth/v1alpha1/wrapper"
	"github.com/prysmaticlabs/prysm/proto/interfaces"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/testutil"
	"github.com/prysmaticlabs/prysm/shared/testutil/assert"
	"github.com/prysmaticlabs/prysm/shared/testutil/require"
	"github.com/prysmaticlabs/prysm/shared/version"
	"google.golang.org/protobuf/proto"
)

func TestReplayBlocks_AllSkipSlots(t *testing.T) {
	beaconDB := testDB.SetupDB(t)

	beaconState, _ := testutil.DeterministicGenesisState(t, 32)
	genesisBlock := blocks.NewGenesisBlock([]byte{})
	bodyRoot, err := genesisBlock.Block.HashTreeRoot()
	require.NoError(t, err)
	err = beaconState.SetLatestBlockHeader(&ethpb.BeaconBlockHeader{
		Slot:       genesisBlock.Block.Slot,
		ParentRoot: genesisBlock.Block.ParentRoot,
		StateRoot:  params.BeaconConfig().ZeroHash[:],
		BodyRoot:   bodyRoot[:],
	})
	require.NoError(t, err)
	require.NoError(t, beaconState.SetSlashings(make([]uint64, params.BeaconConfig().EpochsPerSlashingsVector)))
	cp := beaconState.CurrentJustifiedCheckpoint()
	mockRoot := [32]byte{}
	copy(mockRoot[:], "hello-world")
	cp.Root = mockRoot[:]
	require.NoError(t, beaconState.SetCurrentJustifiedCheckpoint(cp))
	require.NoError(t, beaconState.AppendCurrentEpochAttestations(&pb.PendingAttestation{}))

	service := New(beaconDB)
	targetSlot := params.BeaconConfig().SlotsPerEpoch - 1
	newState, err := service.ReplayBlocks(context.Background(), beaconState, []interfaces.SignedBeaconBlock{}, targetSlot)
	require.NoError(t, err)
	assert.Equal(t, targetSlot, newState.Slot(), "Did not advance slots")
}

func TestReplayBlocks_SameSlot(t *testing.T) {
	beaconDB := testDB.SetupDB(t)

	beaconState, _ := testutil.DeterministicGenesisState(t, 32)
	genesisBlock := blocks.NewGenesisBlock([]byte{})
	bodyRoot, err := genesisBlock.Block.HashTreeRoot()
	require.NoError(t, err)
	err = beaconState.SetLatestBlockHeader(&ethpb.BeaconBlockHeader{
		Slot:       genesisBlock.Block.Slot,
		ParentRoot: genesisBlock.Block.ParentRoot,
		StateRoot:  params.BeaconConfig().ZeroHash[:],
		BodyRoot:   bodyRoot[:],
	})
	require.NoError(t, err)
	require.NoError(t, beaconState.SetSlashings(make([]uint64, params.BeaconConfig().EpochsPerSlashingsVector)))
	cp := beaconState.CurrentJustifiedCheckpoint()
	mockRoot := [32]byte{}
	copy(mockRoot[:], "hello-world")
	cp.Root = mockRoot[:]
	require.NoError(t, beaconState.SetCurrentJustifiedCheckpoint(cp))
	require.NoError(t, beaconState.AppendCurrentEpochAttestations(&pb.PendingAttestation{}))

	service := New(beaconDB)
	targetSlot := beaconState.Slot()
	newState, err := service.ReplayBlocks(context.Background(), beaconState, []interfaces.SignedBeaconBlock{}, targetSlot)
	require.NoError(t, err)
	assert.Equal(t, targetSlot, newState.Slot(), "Did not advance slots")
}

func TestReplayBlocks_LowerSlotBlock(t *testing.T) {
	beaconDB := testDB.SetupDB(t)

	beaconState, _ := testutil.DeterministicGenesisState(t, 32)
	require.NoError(t, beaconState.SetSlot(1))
	genesisBlock := blocks.NewGenesisBlock([]byte{})
	bodyRoot, err := genesisBlock.Block.HashTreeRoot()
	require.NoError(t, err)
	err = beaconState.SetLatestBlockHeader(&ethpb.BeaconBlockHeader{
		Slot:       genesisBlock.Block.Slot,
		ParentRoot: genesisBlock.Block.ParentRoot,
		StateRoot:  params.BeaconConfig().ZeroHash[:],
		BodyRoot:   bodyRoot[:],
	})
	require.NoError(t, err)
	require.NoError(t, beaconState.SetSlashings(make([]uint64, params.BeaconConfig().EpochsPerSlashingsVector)))
	cp := beaconState.CurrentJustifiedCheckpoint()
	mockRoot := [32]byte{}
	copy(mockRoot[:], "hello-world")
	cp.Root = mockRoot[:]
	require.NoError(t, beaconState.SetCurrentJustifiedCheckpoint(cp))
	require.NoError(t, beaconState.AppendCurrentEpochAttestations(&pb.PendingAttestation{}))

	service := New(beaconDB)
	targetSlot := beaconState.Slot()
	b := testutil.NewBeaconBlock()
	b.Block.Slot = beaconState.Slot() - 1
	newState, err := service.ReplayBlocks(context.Background(), beaconState, []interfaces.SignedBeaconBlock{wrapper.WrappedPhase0SignedBeaconBlock(b)}, targetSlot)
	require.NoError(t, err)
	assert.Equal(t, targetSlot, newState.Slot(), "Did not advance slots")
}

func TestReplayBlocks_ThroughForkBoundary(t *testing.T) {
	params.SetupTestConfigCleanup(t)
	bCfg := params.BeaconConfig()
	bCfg.AltairForkEpoch = 1
	bCfg.ForkVersionSchedule[bytesutil.ToBytes4(bCfg.AltairForkVersion)] = 1
	params.OverrideBeaconConfig(bCfg)

	beaconState, _ := testutil.DeterministicGenesisState(t, 32)
	genesisBlock := blocks.NewGenesisBlock([]byte{})
	bodyRoot, err := genesisBlock.Block.HashTreeRoot()
	require.NoError(t, err)
	err = beaconState.SetLatestBlockHeader(&ethpb.BeaconBlockHeader{
		Slot:       genesisBlock.Block.Slot,
		ParentRoot: genesisBlock.Block.ParentRoot,
		StateRoot:  params.BeaconConfig().ZeroHash[:],
		BodyRoot:   bodyRoot[:],
	})
	require.NoError(t, err)

	service := New(testDB.SetupDB(t))
	targetSlot := params.BeaconConfig().SlotsPerEpoch
	newState, err := service.ReplayBlocks(context.Background(), beaconState, []interfaces.SignedBeaconBlock{}, targetSlot)
	require.NoError(t, err)

	// Verify state is version Altair.
	assert.Equal(t, version.Altair, newState.Version())
}

func TestLoadBlocks_FirstBranch(t *testing.T) {
	beaconDB := testDB.SetupDB(t)
	ctx := context.Background()
	s := &State{
		beaconDB: beaconDB,
	}

	roots, savedBlocks, err := tree1(t, beaconDB, bytesutil.PadTo([]byte{'A'}, 32))
	require.NoError(t, err)

	filteredBlocks, err := s.LoadBlocks(ctx, 0, 8, roots[len(roots)-1])
	require.NoError(t, err)

	wanted := []*ethpb.SignedBeaconBlock{
		savedBlocks[8],
		savedBlocks[6],
		savedBlocks[4],
		savedBlocks[2],
		savedBlocks[1],
		savedBlocks[0],
	}

	for i, block := range wanted {
		if !proto.Equal(block, filteredBlocks[i].Proto()) {
			t.Error("Did not get wanted blocks")
		}
	}
}

func TestLoadBlocks_SecondBranch(t *testing.T) {
	beaconDB := testDB.SetupDB(t)
	ctx := context.Background()
	s := &State{
		beaconDB: beaconDB,
	}

	roots, savedBlocks, err := tree1(t, beaconDB, bytesutil.PadTo([]byte{'A'}, 32))
	require.NoError(t, err)

	filteredBlocks, err := s.LoadBlocks(ctx, 0, 5, roots[5])
	require.NoError(t, err)

	wanted := []*ethpb.SignedBeaconBlock{
		savedBlocks[5],
		savedBlocks[3],
		savedBlocks[1],
		savedBlocks[0],
	}

	for i, block := range wanted {
		if !proto.Equal(block, filteredBlocks[i].Proto()) {
			t.Error("Did not get wanted blocks")
		}
	}
}

func TestLoadBlocks_ThirdBranch(t *testing.T) {
	beaconDB := testDB.SetupDB(t)
	ctx := context.Background()
	s := &State{
		beaconDB: beaconDB,
	}

	roots, savedBlocks, err := tree1(t, beaconDB, bytesutil.PadTo([]byte{'A'}, 32))
	require.NoError(t, err)

	filteredBlocks, err := s.LoadBlocks(ctx, 0, 7, roots[7])
	require.NoError(t, err)

	wanted := []*ethpb.SignedBeaconBlock{
		savedBlocks[7],
		savedBlocks[6],
		savedBlocks[4],
		savedBlocks[2],
		savedBlocks[1],
		savedBlocks[0],
	}

	for i, block := range wanted {
		if !proto.Equal(block, filteredBlocks[i].Proto()) {
			t.Error("Did not get wanted blocks")
		}
	}
}

func TestLoadBlocks_SameSlots(t *testing.T) {
	beaconDB := testDB.SetupDB(t)
	ctx := context.Background()
	s := &State{
		beaconDB: beaconDB,
	}

	roots, savedBlocks, err := tree2(t, beaconDB, bytesutil.PadTo([]byte{'A'}, 32))
	require.NoError(t, err)

	filteredBlocks, err := s.LoadBlocks(ctx, 0, 3, roots[6])
	require.NoError(t, err)

	wanted := []*ethpb.SignedBeaconBlock{
		savedBlocks[6],
		savedBlocks[5],
		savedBlocks[1],
		savedBlocks[0],
	}

	for i, block := range wanted {
		if !proto.Equal(block, filteredBlocks[i].Proto()) {
			t.Error("Did not get wanted blocks")
		}
	}
}

func TestLoadBlocks_SameEndSlots(t *testing.T) {
	beaconDB := testDB.SetupDB(t)
	ctx := context.Background()
	s := &State{
		beaconDB: beaconDB,
	}

	roots, savedBlocks, err := tree3(t, beaconDB, bytesutil.PadTo([]byte{'A'}, 32))
	require.NoError(t, err)

	filteredBlocks, err := s.LoadBlocks(ctx, 0, 2, roots[2])
	require.NoError(t, err)

	wanted := []*ethpb.SignedBeaconBlock{
		savedBlocks[2],
		savedBlocks[1],
		savedBlocks[0],
	}

	for i, block := range wanted {
		if !proto.Equal(block, filteredBlocks[i].Proto()) {
			t.Error("Did not get wanted blocks")
		}
	}
}

func TestLoadBlocks_SameEndSlotsWith2blocks(t *testing.T) {
	beaconDB := testDB.SetupDB(t)
	ctx := context.Background()
	s := &State{
		beaconDB: beaconDB,
	}

	roots, savedBlocks, err := tree4(t, beaconDB, bytesutil.PadTo([]byte{'A'}, 32))
	require.NoError(t, err)

	filteredBlocks, err := s.LoadBlocks(ctx, 0, 2, roots[1])
	require.NoError(t, err)

	wanted := []*ethpb.SignedBeaconBlock{
		savedBlocks[1],
		savedBlocks[0],
	}

	for i, block := range wanted {
		if !proto.Equal(block, filteredBlocks[i].Proto()) {
			t.Error("Did not get wanted blocks")
		}
	}
}

func TestLoadBlocks_BadStart(t *testing.T) {
	beaconDB := testDB.SetupDB(t)
	ctx := context.Background()
	s := &State{
		beaconDB: beaconDB,
	}

	roots, _, err := tree1(t, beaconDB, bytesutil.PadTo([]byte{'A'}, 32))
	require.NoError(t, err)
	_, err = s.LoadBlocks(ctx, 0, 5, roots[8])
	assert.ErrorContains(t, "end block roots don't match", err)
}

func TestLastSavedBlock_Genesis(t *testing.T) {
	beaconDB := testDB.SetupDB(t)
	ctx := context.Background()
	s := &State{
		beaconDB:      beaconDB,
		finalizedInfo: &finalizedInfo{slot: 128},
	}

	gBlk := testutil.NewBeaconBlock()
	gRoot, err := gBlk.Block.HashTreeRoot()
	require.NoError(t, err)
	require.NoError(t, s.beaconDB.SaveBlock(ctx, wrapper.WrappedPhase0SignedBeaconBlock(gBlk)))
	require.NoError(t, s.beaconDB.SaveGenesisBlockRoot(ctx, gRoot))

	savedRoot, savedSlot, err := s.lastSavedBlock(ctx, 0)
	require.NoError(t, err)
	assert.Equal(t, types.Slot(0), savedSlot, "Did not save genesis slot")
	assert.Equal(t, savedRoot, savedRoot, "Did not save genesis root")
}

func TestLastSavedBlock_CanGet(t *testing.T) {
	beaconDB := testDB.SetupDB(t)
	ctx := context.Background()
	s := &State{
		beaconDB:      beaconDB,
		finalizedInfo: &finalizedInfo{slot: 128},
	}

	b1 := testutil.NewBeaconBlock()
	b1.Block.Slot = s.finalizedInfo.slot + 5
	require.NoError(t, s.beaconDB.SaveBlock(ctx, wrapper.WrappedPhase0SignedBeaconBlock(b1)))
	b2 := testutil.NewBeaconBlock()
	b2.Block.Slot = s.finalizedInfo.slot + 10
	require.NoError(t, s.beaconDB.SaveBlock(ctx, wrapper.WrappedPhase0SignedBeaconBlock(b2)))
	b3 := testutil.NewBeaconBlock()
	b3.Block.Slot = s.finalizedInfo.slot + 20
	require.NoError(t, s.beaconDB.SaveBlock(ctx, wrapper.WrappedPhase0SignedBeaconBlock(b3)))

	savedRoot, savedSlot, err := s.lastSavedBlock(ctx, s.finalizedInfo.slot+100)
	require.NoError(t, err)
	assert.Equal(t, s.finalizedInfo.slot+20, savedSlot)
	wantedRoot, err := b3.Block.HashTreeRoot()
	require.NoError(t, err)
	assert.Equal(t, wantedRoot, savedRoot, "Did not save correct root")
}

func TestLastSavedBlock_NoSavedBlock(t *testing.T) {
	beaconDB := testDB.SetupDB(t)
	ctx := context.Background()
	s := &State{
		beaconDB:      beaconDB,
		finalizedInfo: &finalizedInfo{slot: 128},
	}

	root, slot, err := s.lastSavedBlock(ctx, s.finalizedInfo.slot+1)
	require.NoError(t, err)
	if slot != 0 && root != [32]byte{} {
		t.Error("Did not get wanted block")
	}
}

func TestLastSavedState_Genesis(t *testing.T) {
	beaconDB := testDB.SetupDB(t)
	ctx := context.Background()
	s := &State{
		beaconDB:      beaconDB,
		finalizedInfo: &finalizedInfo{slot: 128},
	}

	gBlk := testutil.NewBeaconBlock()
	gState, err := testutil.NewBeaconState()
	require.NoError(t, err)
	gRoot, err := gBlk.Block.HashTreeRoot()
	require.NoError(t, err)
	require.NoError(t, s.beaconDB.SaveBlock(ctx, wrapper.WrappedPhase0SignedBeaconBlock(gBlk)))
	require.NoError(t, s.beaconDB.SaveGenesisBlockRoot(ctx, gRoot))
	require.NoError(t, s.beaconDB.SaveState(ctx, gState, gRoot))

	savedState, err := s.lastSavedState(ctx, 0)
	require.NoError(t, err)
	require.DeepSSZEqual(t, gState.InnerStateUnsafe(), savedState.InnerStateUnsafe())
}

func TestLastSavedState_CanGet(t *testing.T) {
	beaconDB := testDB.SetupDB(t)
	ctx := context.Background()
	s := &State{
		beaconDB:      beaconDB,
		finalizedInfo: &finalizedInfo{slot: 128},
	}

	b1 := testutil.NewBeaconBlock()
	b1.Block.Slot = s.finalizedInfo.slot + 5
	require.NoError(t, s.beaconDB.SaveBlock(ctx, wrapper.WrappedPhase0SignedBeaconBlock(b1)))
	b2 := testutil.NewBeaconBlock()
	b2.Block.Slot = s.finalizedInfo.slot + 10
	require.NoError(t, s.beaconDB.SaveBlock(ctx, wrapper.WrappedPhase0SignedBeaconBlock(b2)))
	b2Root, err := b2.Block.HashTreeRoot()
	require.NoError(t, err)
	st, err := testutil.NewBeaconState()
	require.NoError(t, err)
	require.NoError(t, st.SetSlot(s.finalizedInfo.slot+10))

	require.NoError(t, s.beaconDB.SaveState(ctx, st, b2Root))
	b3 := testutil.NewBeaconBlock()
	b3.Block.Slot = s.finalizedInfo.slot + 20
	require.NoError(t, s.beaconDB.SaveBlock(ctx, wrapper.WrappedPhase0SignedBeaconBlock(b3)))

	savedState, err := s.lastSavedState(ctx, s.finalizedInfo.slot+100)
	require.NoError(t, err)
	require.DeepSSZEqual(t, st.InnerStateUnsafe(), savedState.InnerStateUnsafe())
}

func TestLastSavedState_NoSavedBlockState(t *testing.T) {
	beaconDB := testDB.SetupDB(t)
	ctx := context.Background()
	s := &State{
		beaconDB:      beaconDB,
		finalizedInfo: &finalizedInfo{slot: 128},
	}

	b1 := testutil.NewBeaconBlock()
	b1.Block.Slot = 127
	require.NoError(t, s.beaconDB.SaveBlock(ctx, wrapper.WrappedPhase0SignedBeaconBlock(b1)))

	_, err := s.lastSavedState(ctx, s.finalizedInfo.slot+1)
	assert.ErrorContains(t, errUnknownState.Error(), err)
}

// tree1 constructs the following tree:
// B0 - B1 - - B3 -- B5
//        \- B2 -- B4 -- B6 ----- B8
//                         \- B7
func tree1(t *testing.T, beaconDB db.Database, genesisRoot []byte) ([][32]byte, []*ethpb.SignedBeaconBlock, error) {
	b0 := testutil.NewBeaconBlock()
	b0.Block.Slot = 0
	b0.Block.ParentRoot = genesisRoot
	r0, err := b0.Block.HashTreeRoot()
	if err != nil {
		return nil, nil, err
	}
	b1 := testutil.NewBeaconBlock()
	b1.Block.Slot = 1
	b1.Block.ParentRoot = r0[:]
	r1, err := b1.Block.HashTreeRoot()
	if err != nil {
		return nil, nil, err
	}
	b2 := testutil.NewBeaconBlock()
	b2.Block.Slot = 2
	b2.Block.ParentRoot = r1[:]
	r2, err := b2.Block.HashTreeRoot()
	if err != nil {
		return nil, nil, err
	}
	b3 := testutil.NewBeaconBlock()
	b3.Block.Slot = 3
	b3.Block.ParentRoot = r1[:]
	r3, err := b3.Block.HashTreeRoot()
	if err != nil {
		return nil, nil, err
	}
	b4 := testutil.NewBeaconBlock()
	b4.Block.Slot = 4
	b4.Block.ParentRoot = r2[:]
	r4, err := b4.Block.HashTreeRoot()
	if err != nil {
		return nil, nil, err
	}
	b5 := testutil.NewBeaconBlock()
	b5.Block.Slot = 5
	b5.Block.ParentRoot = r3[:]
	r5, err := b5.Block.HashTreeRoot()
	if err != nil {
		return nil, nil, err
	}
	b6 := testutil.NewBeaconBlock()
	b6.Block.Slot = 6
	b6.Block.ParentRoot = r4[:]
	r6, err := b6.Block.HashTreeRoot()
	if err != nil {
		return nil, nil, err
	}
	b7 := testutil.NewBeaconBlock()
	b7.Block.Slot = 7
	b7.Block.ParentRoot = r6[:]
	r7, err := b7.Block.HashTreeRoot()
	if err != nil {
		return nil, nil, err
	}
	b8 := testutil.NewBeaconBlock()
	b8.Block.Slot = 8
	b8.Block.ParentRoot = r6[:]
	r8, err := b8.Block.HashTreeRoot()
	if err != nil {
		return nil, nil, err
	}
	st, err := testutil.NewBeaconState()
	require.NoError(t, err)

	returnedBlocks := make([]*ethpb.SignedBeaconBlock, 0)
	for _, b := range []*ethpb.SignedBeaconBlock{b0, b1, b2, b3, b4, b5, b6, b7, b8} {
		beaconBlock := testutil.NewBeaconBlock()
		beaconBlock.Block.Slot = b.Block.Slot
		beaconBlock.Block.ParentRoot = bytesutil.PadTo(b.Block.ParentRoot, 32)
		if err := beaconDB.SaveBlock(context.Background(), wrapper.WrappedPhase0SignedBeaconBlock(beaconBlock)); err != nil {
			return nil, nil, err
		}
		if err := beaconDB.SaveState(context.Background(), st.Copy(), bytesutil.ToBytes32(beaconBlock.Block.ParentRoot)); err != nil {
			return nil, nil, err
		}
		returnedBlocks = append(returnedBlocks, beaconBlock)
	}
	return [][32]byte{r0, r1, r2, r3, r4, r5, r6, r7, r8}, returnedBlocks, nil
}

// tree2 constructs the following tree:
// B0 - B1
//        \- B2
//        \- B2
//        \- B2
//        \- B2 -- B3
func tree2(t *testing.T, beaconDB db.Database, genesisRoot []byte) ([][32]byte, []*ethpb.SignedBeaconBlock, error) {
	b0 := testutil.NewBeaconBlock()
	b0.Block.Slot = 0
	b0.Block.ParentRoot = genesisRoot
	r0, err := b0.Block.HashTreeRoot()
	if err != nil {
		return nil, nil, err
	}
	b1 := testutil.NewBeaconBlock()
	b1.Block.Slot = 1
	b1.Block.ParentRoot = r0[:]
	r1, err := b1.Block.HashTreeRoot()
	if err != nil {
		return nil, nil, err
	}
	b21 := testutil.NewBeaconBlock()
	b21.Block.Slot = 2
	b21.Block.ParentRoot = r1[:]
	b21.Block.StateRoot = bytesutil.PadTo([]byte{'A'}, 32)
	r21, err := b21.Block.HashTreeRoot()
	if err != nil {
		return nil, nil, err
	}
	b22 := testutil.NewBeaconBlock()
	b22.Block.Slot = 2
	b22.Block.ParentRoot = r1[:]
	b22.Block.StateRoot = bytesutil.PadTo([]byte{'B'}, 32)
	r22, err := b22.Block.HashTreeRoot()
	if err != nil {
		return nil, nil, err
	}
	b23 := testutil.NewBeaconBlock()
	b23.Block.Slot = 2
	b23.Block.ParentRoot = r1[:]
	b23.Block.StateRoot = bytesutil.PadTo([]byte{'C'}, 32)
	r23, err := b23.Block.HashTreeRoot()
	if err != nil {
		return nil, nil, err
	}
	b24 := testutil.NewBeaconBlock()
	b24.Block.Slot = 2
	b24.Block.ParentRoot = r1[:]
	b24.Block.StateRoot = bytesutil.PadTo([]byte{'D'}, 32)
	r24, err := b24.Block.HashTreeRoot()
	if err != nil {
		return nil, nil, err
	}
	b3 := testutil.NewBeaconBlock()
	b3.Block.Slot = 3
	b3.Block.ParentRoot = r24[:]
	r3, err := b3.Block.HashTreeRoot()
	if err != nil {
		return nil, nil, err
	}
	st, err := testutil.NewBeaconState()
	require.NoError(t, err)

	returnedBlocks := make([]*ethpb.SignedBeaconBlock, 0)
	for _, b := range []*ethpb.SignedBeaconBlock{b0, b1, b21, b22, b23, b24, b3} {
		beaconBlock := testutil.NewBeaconBlock()
		beaconBlock.Block.Slot = b.Block.Slot
		beaconBlock.Block.ParentRoot = bytesutil.PadTo(b.Block.ParentRoot, 32)
		beaconBlock.Block.StateRoot = bytesutil.PadTo(b.Block.StateRoot, 32)
		if err := beaconDB.SaveBlock(context.Background(), wrapper.WrappedPhase0SignedBeaconBlock(beaconBlock)); err != nil {
			return nil, nil, err
		}
		if err := beaconDB.SaveState(context.Background(), st.Copy(), bytesutil.ToBytes32(beaconBlock.Block.ParentRoot)); err != nil {
			return nil, nil, err
		}
		returnedBlocks = append(returnedBlocks, beaconBlock)
	}
	return [][32]byte{r0, r1, r21, r22, r23, r24, r3}, returnedBlocks, nil
}

// tree3 constructs the following tree:
// B0 - B1
//        \- B2
//        \- B2
//        \- B2
//        \- B2
func tree3(t *testing.T, beaconDB db.Database, genesisRoot []byte) ([][32]byte, []*ethpb.SignedBeaconBlock, error) {
	b0 := testutil.NewBeaconBlock()
	b0.Block.Slot = 0
	b0.Block.ParentRoot = genesisRoot
	r0, err := b0.Block.HashTreeRoot()
	if err != nil {
		return nil, nil, err
	}
	b1 := testutil.NewBeaconBlock()
	b1.Block.Slot = 1
	b1.Block.ParentRoot = r0[:]
	r1, err := b1.Block.HashTreeRoot()
	if err != nil {
		return nil, nil, err
	}
	b21 := testutil.NewBeaconBlock()
	b21.Block.Slot = 2
	b21.Block.ParentRoot = r1[:]
	b21.Block.StateRoot = bytesutil.PadTo([]byte{'A'}, 32)
	r21, err := b21.Block.HashTreeRoot()
	if err != nil {
		return nil, nil, err
	}
	b22 := testutil.NewBeaconBlock()
	b22.Block.Slot = 2
	b22.Block.ParentRoot = r1[:]
	b22.Block.StateRoot = bytesutil.PadTo([]byte{'B'}, 32)
	r22, err := b22.Block.HashTreeRoot()
	if err != nil {
		return nil, nil, err
	}
	b23 := testutil.NewBeaconBlock()
	b23.Block.Slot = 2
	b23.Block.ParentRoot = r1[:]
	b23.Block.StateRoot = bytesutil.PadTo([]byte{'C'}, 32)
	r23, err := b23.Block.HashTreeRoot()
	if err != nil {
		return nil, nil, err
	}
	b24 := testutil.NewBeaconBlock()
	b24.Block.Slot = 2
	b24.Block.ParentRoot = r1[:]
	b24.Block.StateRoot = bytesutil.PadTo([]byte{'D'}, 32)
	r24, err := b24.Block.HashTreeRoot()
	if err != nil {
		return nil, nil, err
	}
	st, err := testutil.NewBeaconState()
	require.NoError(t, err)

	returnedBlocks := make([]*ethpb.SignedBeaconBlock, 0)
	for _, b := range []*ethpb.SignedBeaconBlock{b0, b1, b21, b22, b23, b24} {
		beaconBlock := testutil.NewBeaconBlock()
		beaconBlock.Block.Slot = b.Block.Slot
		beaconBlock.Block.ParentRoot = bytesutil.PadTo(b.Block.ParentRoot, 32)
		beaconBlock.Block.StateRoot = bytesutil.PadTo(b.Block.StateRoot, 32)
		if err := beaconDB.SaveBlock(context.Background(), wrapper.WrappedPhase0SignedBeaconBlock(beaconBlock)); err != nil {
			return nil, nil, err
		}
		if err := beaconDB.SaveState(context.Background(), st.Copy(), bytesutil.ToBytes32(beaconBlock.Block.ParentRoot)); err != nil {
			return nil, nil, err
		}
		returnedBlocks = append(returnedBlocks, beaconBlock)
	}

	return [][32]byte{r0, r1, r21, r22, r23, r24}, returnedBlocks, nil
}

// tree4 constructs the following tree:
// B0
//   \- B2
//   \- B2
//   \- B2
//   \- B2
func tree4(t *testing.T, beaconDB db.Database, genesisRoot []byte) ([][32]byte, []*ethpb.SignedBeaconBlock, error) {
	b0 := testutil.NewBeaconBlock()
	b0.Block.Slot = 0
	b0.Block.ParentRoot = genesisRoot
	r0, err := b0.Block.HashTreeRoot()
	if err != nil {
		return nil, nil, err
	}
	b21 := testutil.NewBeaconBlock()
	b21.Block.Slot = 2
	b21.Block.ParentRoot = r0[:]
	b21.Block.StateRoot = bytesutil.PadTo([]byte{'A'}, 32)
	r21, err := b21.Block.HashTreeRoot()
	if err != nil {
		return nil, nil, err
	}
	b22 := testutil.NewBeaconBlock()
	b22.Block.Slot = 2
	b22.Block.ParentRoot = r0[:]
	b22.Block.StateRoot = bytesutil.PadTo([]byte{'B'}, 32)
	r22, err := b22.Block.HashTreeRoot()
	if err != nil {
		return nil, nil, err
	}
	b23 := testutil.NewBeaconBlock()
	b23.Block.Slot = 2
	b23.Block.ParentRoot = r0[:]
	b23.Block.StateRoot = bytesutil.PadTo([]byte{'C'}, 32)
	r23, err := b23.Block.HashTreeRoot()
	if err != nil {
		return nil, nil, err
	}
	b24 := testutil.NewBeaconBlock()
	b24.Block.Slot = 2
	b24.Block.ParentRoot = r0[:]
	b24.Block.StateRoot = bytesutil.PadTo([]byte{'D'}, 32)
	r24, err := b24.Block.HashTreeRoot()
	if err != nil {
		return nil, nil, err
	}
	st, err := testutil.NewBeaconState()
	require.NoError(t, err)

	returnedBlocks := make([]*ethpb.SignedBeaconBlock, 0)
	for _, b := range []*ethpb.SignedBeaconBlock{b0, b21, b22, b23, b24} {
		beaconBlock := testutil.NewBeaconBlock()
		beaconBlock.Block.Slot = b.Block.Slot
		beaconBlock.Block.ParentRoot = bytesutil.PadTo(b.Block.ParentRoot, 32)
		beaconBlock.Block.StateRoot = bytesutil.PadTo(b.Block.StateRoot, 32)
		if err := beaconDB.SaveBlock(context.Background(), wrapper.WrappedPhase0SignedBeaconBlock(beaconBlock)); err != nil {
			return nil, nil, err
		}
		if err := beaconDB.SaveState(context.Background(), st.Copy(), bytesutil.ToBytes32(beaconBlock.Block.ParentRoot)); err != nil {
			return nil, nil, err
		}
		returnedBlocks = append(returnedBlocks, beaconBlock)
	}

	return [][32]byte{r0, r21, r22, r23, r24}, returnedBlocks, nil
}

func TestLoadFinalizedBlocks(t *testing.T) {
	beaconDB := testDB.SetupDB(t)
	ctx := context.Background()
	s := &State{
		beaconDB: beaconDB,
	}
	gBlock := testutil.NewBeaconBlock()
	gRoot, err := gBlock.Block.HashTreeRoot()
	require.NoError(t, err)
	require.NoError(t, beaconDB.SaveBlock(ctx, wrapper.WrappedPhase0SignedBeaconBlock(gBlock)))
	require.NoError(t, beaconDB.SaveGenesisBlockRoot(ctx, [32]byte{}))
	roots, _, err := tree1(t, beaconDB, gRoot[:])
	require.NoError(t, err)

	filteredBlocks, err := s.loadFinalizedBlocks(ctx, 0, 8)
	require.NoError(t, err)
	require.Equal(t, 0, len(filteredBlocks))
	require.NoError(t, beaconDB.SaveStateSummary(ctx, &pb.StateSummary{Root: roots[8][:]}))

	require.NoError(t, s.beaconDB.SaveFinalizedCheckpoint(ctx, &ethpb.Checkpoint{Root: roots[8][:]}))
	filteredBlocks, err = s.loadFinalizedBlocks(ctx, 0, 8)
	require.NoError(t, err)
	require.Equal(t, 10, len(filteredBlocks))
}
