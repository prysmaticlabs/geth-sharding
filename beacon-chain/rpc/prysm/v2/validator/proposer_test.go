package validator

import (
	"context"
	"testing"

	types "github.com/prysmaticlabs/eth2-types"
	"github.com/prysmaticlabs/go-bitfield"
	mock "github.com/prysmaticlabs/prysm/beacon-chain/blockchain/testing"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/altair"
	b "github.com/prysmaticlabs/prysm/beacon-chain/core/blocks"
	dbutil "github.com/prysmaticlabs/prysm/beacon-chain/db/testing"
	"github.com/prysmaticlabs/prysm/beacon-chain/operations/attestations"
	"github.com/prysmaticlabs/prysm/beacon-chain/operations/slashings"
	"github.com/prysmaticlabs/prysm/beacon-chain/operations/synccommittee"
	"github.com/prysmaticlabs/prysm/beacon-chain/operations/voluntaryexits"
	mockp2p "github.com/prysmaticlabs/prysm/beacon-chain/p2p/testing"
	mockPOW "github.com/prysmaticlabs/prysm/beacon-chain/powchain/testing"
	validatorv1alpha1 "github.com/prysmaticlabs/prysm/beacon-chain/rpc/prysm/v1alpha1/validator"
	"github.com/prysmaticlabs/prysm/beacon-chain/state/stategen"
	mockSync "github.com/prysmaticlabs/prysm/beacon-chain/sync/initial-sync/testing"
	ethpb "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	prysmv2 "github.com/prysmaticlabs/prysm/proto/prysm/v2"
	"github.com/prysmaticlabs/prysm/proto/prysm/v2/wrapper"
	"github.com/prysmaticlabs/prysm/shared/bls"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/testutil"
	"github.com/prysmaticlabs/prysm/shared/testutil/assert"
	"github.com/prysmaticlabs/prysm/shared/testutil/require"
)

func TestProposer_ProposeBlock_OK(t *testing.T) {
	db := dbutil.SetupDB(t)
	ctx := context.Background()
	params.SetupTestConfigCleanup(t)
	params.OverrideBeaconConfig(params.MainnetConfig())

	genesis := testutil.NewBeaconBlockAltair()
	require.NoError(
		t,
		db.SaveBlock(
			ctx,
			wrapper.WrappedAltairSignedBeaconBlock(genesis),
		),
		"Could not save genesis block",
	)

	numDeposits := uint64(64)
	beaconState, _ := testutil.DeterministicGenesisStateAltair(t, numDeposits)
	bsRoot, err := beaconState.HashTreeRoot(ctx)
	require.NoError(t, err)
	genesisRoot, err := genesis.Block.HashTreeRoot()
	require.NoError(t, err)
	require.NoError(t, db.SaveState(ctx, beaconState, genesisRoot), "Could not save genesis state")

	c := &mock.ChainService{Root: bsRoot[:], State: beaconState}
	V1Server := &validatorv1alpha1.Server{
		ChainStartFetcher: &mockPOW.POWChain{},
		Eth1InfoFetcher:   &mockPOW.POWChain{},
		Eth1BlockFetcher:  &mockPOW.POWChain{},
		BlockReceiver:     c,
		HeadFetcher:       c,
		BlockNotifier:     c.BlockNotifier(),
		P2P:               mockp2p.NewTestP2P(t),
	}
	proposerServer := &Server{
		V1Server: V1Server,
	}
	req := testutil.NewBeaconBlockAltair()
	req.Block.Slot = 5
	req.Block.ParentRoot = bsRoot[:]
	require.NoError(t, db.SaveBlock(ctx, wrapper.WrappedAltairSignedBeaconBlock(req)))
	_, err = proposerServer.ProposeBlock(context.Background(), req)
	assert.NoError(t, err, "Could not propose block correctly")
}

func TestProposer_GetBlock_OK(t *testing.T) {
	db := dbutil.SetupDB(t)
	ctx := context.Background()

	params.SetupTestConfigCleanup(t)
	params.OverrideBeaconConfig(params.MainnetConfig())
	beaconState, privKeys := testutil.DeterministicGenesisStateAltair(t, 64)
	committee, err := altair.NextSyncCommittee(beaconState)
	require.NoError(t, err)
	require.NoError(t, beaconState.SetCurrentSyncCommittee(committee))

	stateRoot, err := beaconState.HashTreeRoot(ctx)
	require.NoError(t, err, "Could not hash genesis state")

	genesis := b.NewGenesisBlock(stateRoot[:])
	genAltair := &prysmv2.SignedBeaconBlock{
		Block: &prysmv2.BeaconBlock{
			Slot:       genesis.Block.Slot,
			ParentRoot: genesis.Block.ParentRoot,
			StateRoot:  genesis.Block.StateRoot,
			Body: &prysmv2.BeaconBlockBody{
				RandaoReveal:  genesis.Block.Body.RandaoReveal,
				Graffiti:      genesis.Block.Body.Graffiti,
				Eth1Data:      genesis.Block.Body.Eth1Data,
				SyncAggregate: &prysmv2.SyncAggregate{SyncCommitteeBits: bitfield.NewBitvector512(), SyncCommitteeSignature: make([]byte, 96)},
			},
		},
		Signature: genesis.Signature,
	}
	require.NoError(t, db.SaveBlock(ctx, wrapper.WrappedAltairSignedBeaconBlock(genAltair)), "Could not save genesis block")

	parentRoot, err := genAltair.Block.HashTreeRoot()
	require.NoError(t, err, "Could not get signing root")
	require.NoError(t, db.SaveState(ctx, beaconState, parentRoot), "Could not save genesis state")
	require.NoError(t, db.SaveHeadBlockRoot(ctx, parentRoot), "Could not save genesis state")

	require.NoError(t, err, "Could not get signing root")
	require.NoError(t, db.SaveState(ctx, beaconState, parentRoot), "Could not save genesis state")
	require.NoError(t, db.SaveHeadBlockRoot(ctx, parentRoot), "Could not save genesis state")

	V1Server := &validatorv1alpha1.Server{
		HeadFetcher:       &mock.ChainService{State: beaconState, Root: parentRoot[:]},
		SyncChecker:       &mockSync.Sync{IsSyncing: false},
		BlockReceiver:     &mock.ChainService{},
		ChainStartFetcher: &mockPOW.POWChain{},
		Eth1InfoFetcher:   &mockPOW.POWChain{},
		Eth1BlockFetcher:  &mockPOW.POWChain{},
		MockEth1Votes:     true,
		AttPool:           attestations.NewPool(),
		SlashingsPool:     slashings.NewPool(),
		ExitPool:          voluntaryexits.NewPool(),
		StateGen:          stategen.New(db),
	}
	proposerServer := &Server{V1Server: V1Server, SyncCommitteePool: synccommittee.NewStore()}

	randaoReveal, err := testutil.RandaoReveal(beaconState, 0, privKeys)
	require.NoError(t, err)

	graffiti := bytesutil.ToBytes32([]byte("eth2"))
	req := &ethpb.BlockRequest{
		Slot:         1,
		RandaoReveal: randaoReveal,
		Graffiti:     graffiti[:],
	}

	proposerSlashings := make([]*ethpb.ProposerSlashing, params.BeaconConfig().MaxProposerSlashings)
	for i := types.ValidatorIndex(0); uint64(i) < params.BeaconConfig().MaxProposerSlashings; i++ {
		proposerSlashing, err := testutil.GenerateProposerSlashingForValidator(
			beaconState,
			privKeys[i],
			i, /* validator index */
		)
		require.NoError(t, err)
		proposerSlashings[i] = proposerSlashing
		err = proposerServer.V1Server.SlashingsPool.InsertProposerSlashing(context.Background(), beaconState, proposerSlashing)
		require.NoError(t, err)
	}

	attSlashings := make([]*ethpb.AttesterSlashing, params.BeaconConfig().MaxAttesterSlashings)
	for i := uint64(0); i < params.BeaconConfig().MaxAttesterSlashings; i++ {
		attesterSlashing, err := testutil.GenerateAttesterSlashingForValidator(
			beaconState,
			privKeys[i+params.BeaconConfig().MaxProposerSlashings],
			types.ValidatorIndex(i+params.BeaconConfig().MaxProposerSlashings), /* validator index */
		)
		require.NoError(t, err)
		attSlashings[i] = attesterSlashing
		err = proposerServer.V1Server.SlashingsPool.InsertAttesterSlashing(context.Background(), beaconState, attesterSlashing)
		require.NoError(t, err)
	}
	block, err := proposerServer.GetBlock(ctx, req)
	require.NoError(t, err)

	assert.Equal(t, req.Slot, block.Slot, "Expected block to have slot of 1")
	assert.DeepEqual(t, parentRoot[:], block.ParentRoot, "Expected block to have correct parent root")
	assert.DeepEqual(t, randaoReveal, block.Body.RandaoReveal, "Expected block to have correct randao reveal")
	assert.DeepEqual(t, req.Graffiti, block.Body.Graffiti, "Expected block to have correct Graffiti")
	assert.Equal(t, params.BeaconConfig().MaxProposerSlashings, uint64(len(block.Body.ProposerSlashings)))
	assert.DeepEqual(t, proposerSlashings, block.Body.ProposerSlashings)
	assert.Equal(t, params.BeaconConfig().MaxAttesterSlashings, uint64(len(block.Body.AttesterSlashings)))
	assert.DeepEqual(t, attSlashings, block.Body.AttesterSlashings)
}

func TestProposer_GetSyncAggregate_OK(t *testing.T) {
	V1Server := &validatorv1alpha1.Server{
		SyncChecker: &mockSync.Sync{IsSyncing: false},
	}
	proposerServer := &Server{V1Server: V1Server, SyncCommitteePool: synccommittee.NewStore()}

	r := params.BeaconConfig().ZeroHash
	conts := []*prysmv2.SyncCommitteeContribution{
		{Slot: 1, SubcommitteeIndex: 0, Signature: bls.NewAggregateSignature().Marshal(), AggregationBits: []byte{0b0001}, BlockRoot: r[:]},
		{Slot: 1, SubcommitteeIndex: 0, Signature: bls.NewAggregateSignature().Marshal(), AggregationBits: []byte{0b1001}, BlockRoot: r[:]},
		{Slot: 1, SubcommitteeIndex: 0, Signature: bls.NewAggregateSignature().Marshal(), AggregationBits: []byte{0b1110}, BlockRoot: r[:]},
		{Slot: 1, SubcommitteeIndex: 1, Signature: bls.NewAggregateSignature().Marshal(), AggregationBits: []byte{0b0001}, BlockRoot: r[:]},
		{Slot: 1, SubcommitteeIndex: 1, Signature: bls.NewAggregateSignature().Marshal(), AggregationBits: []byte{0b1001}, BlockRoot: r[:]},
		{Slot: 1, SubcommitteeIndex: 1, Signature: bls.NewAggregateSignature().Marshal(), AggregationBits: []byte{0b1110}, BlockRoot: r[:]},
		{Slot: 1, SubcommitteeIndex: 2, Signature: bls.NewAggregateSignature().Marshal(), AggregationBits: []byte{0b0001}, BlockRoot: r[:]},
		{Slot: 1, SubcommitteeIndex: 2, Signature: bls.NewAggregateSignature().Marshal(), AggregationBits: []byte{0b1001}, BlockRoot: r[:]},
		{Slot: 1, SubcommitteeIndex: 2, Signature: bls.NewAggregateSignature().Marshal(), AggregationBits: []byte{0b1110}, BlockRoot: r[:]},
		{Slot: 1, SubcommitteeIndex: 3, Signature: bls.NewAggregateSignature().Marshal(), AggregationBits: []byte{0b0001}, BlockRoot: r[:]},
		{Slot: 1, SubcommitteeIndex: 3, Signature: bls.NewAggregateSignature().Marshal(), AggregationBits: []byte{0b1001}, BlockRoot: r[:]},
		{Slot: 1, SubcommitteeIndex: 3, Signature: bls.NewAggregateSignature().Marshal(), AggregationBits: []byte{0b1110}, BlockRoot: r[:]},
		{Slot: 2, SubcommitteeIndex: 0, Signature: bls.NewAggregateSignature().Marshal(), AggregationBits: []byte{0b10101010}, BlockRoot: r[:]},
		{Slot: 2, SubcommitteeIndex: 1, Signature: bls.NewAggregateSignature().Marshal(), AggregationBits: []byte{0b10101010}, BlockRoot: r[:]},
		{Slot: 2, SubcommitteeIndex: 2, Signature: bls.NewAggregateSignature().Marshal(), AggregationBits: []byte{0b10101010}, BlockRoot: r[:]},
		{Slot: 2, SubcommitteeIndex: 3, Signature: bls.NewAggregateSignature().Marshal(), AggregationBits: []byte{0b10101010}, BlockRoot: r[:]},
	}

	for _, cont := range conts {
		require.NoError(t, proposerServer.SyncCommitteePool.SaveSyncCommitteeContribution(cont))
	}

	aggregate, err := proposerServer.getSyncAggregate(context.Background(), 1, bytesutil.ToBytes32(conts[0].BlockRoot))
	require.NoError(t, err)
	require.DeepEqual(t, bitfield.Bitvector512{0xf, 0xf, 0xf, 0xf}, aggregate.SyncCommitteeBits)

	aggregate, err = proposerServer.getSyncAggregate(context.Background(), 2, bytesutil.ToBytes32(conts[0].BlockRoot))
	require.NoError(t, err)
	require.DeepEqual(t, bitfield.Bitvector512{0xaa, 0xaa, 0xaa, 0xaa}, aggregate.SyncCommitteeBits)

	aggregate, err = proposerServer.getSyncAggregate(context.Background(), 3, bytesutil.ToBytes32(conts[0].BlockRoot))
	require.NoError(t, err)
	require.DeepEqual(t, bitfield.NewBitvector512(), aggregate.SyncCommitteeBits)
}
