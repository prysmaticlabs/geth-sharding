package sync

import (
	"bytes"
	"context"
	"reflect"
	"testing"
	"time"

	lru "github.com/hashicorp/golang-lru"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	pubsubpb "github.com/libp2p/go-libp2p-pubsub/pb"
	types "github.com/prysmaticlabs/eth2-types"
	"github.com/prysmaticlabs/go-bitfield"
	mock "github.com/prysmaticlabs/prysm/beacon-chain/blockchain/testing"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	dbtest "github.com/prysmaticlabs/prysm/beacon-chain/db/testing"
	"github.com/prysmaticlabs/prysm/beacon-chain/operations/attestations"
	"github.com/prysmaticlabs/prysm/beacon-chain/p2p"
	p2ptest "github.com/prysmaticlabs/prysm/beacon-chain/p2p/testing"
	mockSync "github.com/prysmaticlabs/prysm/beacon-chain/sync/initial-sync/testing"
	ethpb "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/proto/eth/v1alpha1/wrapper"
	"github.com/prysmaticlabs/prysm/shared/attestationutil"
	"github.com/prysmaticlabs/prysm/shared/bls"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/testutil"
	"github.com/prysmaticlabs/prysm/shared/testutil/assert"
	"github.com/prysmaticlabs/prysm/shared/testutil/require"
)

func TestVerifyIndexInCommittee_CanVerify(t *testing.T) {
	ctx := context.Background()
	params.UseMinimalConfig()
	defer params.UseMainnetConfig()

	validators := uint64(32)
	s, _ := testutil.DeterministicGenesisState(t, validators)
	require.NoError(t, s.SetSlot(params.BeaconConfig().SlotsPerEpoch))

	bf := bitfield.NewBitlist(validators / uint64(params.BeaconConfig().SlotsPerEpoch))
	bf.SetBitAt(0, true)
	att := &ethpb.Attestation{Data: &ethpb.AttestationData{
		Target: &ethpb.Checkpoint{Epoch: 0}},
		AggregationBits: bf}

	committee, err := helpers.BeaconCommitteeFromState(s, att.Data.Slot, att.Data.CommitteeIndex)
	assert.NoError(t, err)
	indices, err := attestationutil.AttestingIndices(att.AggregationBits, committee)
	require.NoError(t, err)
	require.NoError(t, validateIndexInCommittee(ctx, s, att, types.ValidatorIndex(indices[0])))

	wanted := "validator index 1000 is not within the committee"
	assert.ErrorContains(t, wanted, validateIndexInCommittee(ctx, s, att, 1000))
}

func TestVerifyIndexInCommittee_ExistsInBeaconCommittee(t *testing.T) {
	ctx := context.Background()
	params.UseMinimalConfig()
	defer params.UseMainnetConfig()

	validators := uint64(64)
	s, _ := testutil.DeterministicGenesisState(t, validators)
	require.NoError(t, s.SetSlot(params.BeaconConfig().SlotsPerEpoch))

	bf := []byte{0xff}
	att := &ethpb.Attestation{Data: &ethpb.AttestationData{
		Target: &ethpb.Checkpoint{Epoch: 0}},
		AggregationBits: bf}

	committee, err := helpers.BeaconCommitteeFromState(s, att.Data.Slot, att.Data.CommitteeIndex)
	require.NoError(t, err)

	require.NoError(t, validateIndexInCommittee(ctx, s, att, committee[0]))

	wanted := "validator index 1000 is not within the committee"
	assert.ErrorContains(t, wanted, validateIndexInCommittee(ctx, s, att, 1000))
}

func TestVerifySelection_NotAnAggregator(t *testing.T) {
	ctx := context.Background()
	params.UseMinimalConfig()
	defer params.UseMainnetConfig()
	validators := uint64(2048)
	beaconState, privKeys := testutil.DeterministicGenesisState(t, validators)

	sig := privKeys[0].Sign([]byte{'A'})
	data := testutil.HydrateAttestationData(&ethpb.AttestationData{})

	_, err := validateSelectionIndex(ctx, beaconState, data, 0, sig.Marshal())
	wanted := "validator is not an aggregator for slot"
	assert.ErrorContains(t, wanted, err)
}

func TestValidateAggregateAndProof_NoBlock(t *testing.T) {
	db := dbtest.SetupDB(t)
	p := p2ptest.NewTestP2P(t)

	att := testutil.HydrateAttestation(&ethpb.Attestation{
		Data: &ethpb.AttestationData{
			Source: &ethpb.Checkpoint{Root: bytesutil.PadTo([]byte("hello-world"), 32)},
			Target: &ethpb.Checkpoint{Root: bytesutil.PadTo([]byte("hello-world"), 32)},
		},
	})

	aggregateAndProof := &ethpb.AggregateAttestationAndProof{
		SelectionProof:  bytesutil.PadTo([]byte{'A'}, 96),
		Aggregate:       att,
		AggregatorIndex: 0,
	}
	signedAggregateAndProof := &ethpb.SignedAggregateAttestationAndProof{Message: aggregateAndProof, Signature: make([]byte, 96)}

	c, err := lru.New(10)
	require.NoError(t, err)
	r := &Service{
		cfg: &Config{
			P2P:         p,
			DB:          db,
			InitialSync: &mockSync.Sync{IsSyncing: false},
			AttPool:     attestations.NewPool(),
			Chain:       &mock.ChainService{},
		},
		blkRootToPendingAtts: make(map[[32]byte][]*ethpb.SignedAggregateAttestationAndProof),
		seenAttestationCache: c,
	}
	err = r.initCaches()
	require.NoError(t, err)

	buf := new(bytes.Buffer)
	_, err = p.Encoding().EncodeGossip(buf, signedAggregateAndProof)
	require.NoError(t, err)

	topic := p2p.GossipTypeMapping[reflect.TypeOf(signedAggregateAndProof)]
	msg := &pubsub.Message{
		Message: &pubsubpb.Message{
			Data:  buf.Bytes(),
			Topic: &topic,
		},
	}

	if r.validateAggregateAndProof(context.Background(), "", msg) == pubsub.ValidationAccept {
		t.Error("Expected validate to fail")
	}
}

func TestValidateAggregateAndProof_NotWithinSlotRange(t *testing.T) {
	db := dbtest.SetupDB(t)
	p := p2ptest.NewTestP2P(t)

	validators := uint64(256)
	beaconState, _ := testutil.DeterministicGenesisState(t, validators)

	b := testutil.NewBeaconBlock()
	require.NoError(t, db.SaveBlock(context.Background(), wrapper.WrappedPhase0SignedBeaconBlock(b)))
	root, err := b.Block.HashTreeRoot()
	require.NoError(t, err)
	s, err := testutil.NewBeaconState()
	require.NoError(t, err)
	require.NoError(t, db.SaveState(context.Background(), s, root))

	aggBits := bitfield.NewBitlist(3)
	aggBits.SetBitAt(0, true)
	att := &ethpb.Attestation{
		Data: &ethpb.AttestationData{
			Slot:            1,
			BeaconBlockRoot: root[:],
			Source:          &ethpb.Checkpoint{Epoch: 0, Root: bytesutil.PadTo([]byte("hello-world"), 32)},
			Target:          &ethpb.Checkpoint{Epoch: 0, Root: bytesutil.PadTo([]byte("hello-world"), 32)},
		},
		AggregationBits: aggBits,
		Signature:       make([]byte, 96),
	}

	aggregateAndProof := &ethpb.AggregateAttestationAndProof{
		Aggregate:      att,
		SelectionProof: make([]byte, 96),
	}
	signedAggregateAndProof := &ethpb.SignedAggregateAttestationAndProof{Message: aggregateAndProof, Signature: make([]byte, 96)}

	require.NoError(t, beaconState.SetGenesisTime(uint64(time.Now().Unix())))

	c, err := lru.New(10)
	require.NoError(t, err)
	r := &Service{
		cfg: &Config{
			P2P:         p,
			DB:          db,
			InitialSync: &mockSync.Sync{IsSyncing: false},
			Chain: &mock.ChainService{
				Genesis: time.Now(),
				State:   beaconState,
			},
			AttPool:           attestations.NewPool(),
			OperationNotifier: (&mock.ChainService{}).OperationNotifier(),
		},
		seenAttestationCache: c,
	}
	err = r.initCaches()
	require.NoError(t, err)

	buf := new(bytes.Buffer)
	_, err = p.Encoding().EncodeGossip(buf, signedAggregateAndProof)
	require.NoError(t, err)

	topic := p2p.GossipTypeMapping[reflect.TypeOf(signedAggregateAndProof)]
	msg := &pubsub.Message{
		Message: &pubsubpb.Message{
			Data:  buf.Bytes(),
			Topic: &topic,
		},
	}

	if r.validateAggregateAndProof(context.Background(), "", msg) == pubsub.ValidationAccept {
		t.Error("Expected validate to fail")
	}

	att.Data.Slot = 1<<32 - 1

	buf = new(bytes.Buffer)
	_, err = p.Encoding().EncodeGossip(buf, signedAggregateAndProof)
	require.NoError(t, err)

	msg = &pubsub.Message{
		Message: &pubsubpb.Message{
			Data:  buf.Bytes(),
			Topic: &topic,
		},
	}
	if r.validateAggregateAndProof(context.Background(), "", msg) == pubsub.ValidationAccept {
		t.Error("Expected validate to fail")
	}
}

func TestValidateAggregateAndProof_ExistedInPool(t *testing.T) {
	db := dbtest.SetupDB(t)
	p := p2ptest.NewTestP2P(t)

	validators := uint64(256)
	beaconState, _ := testutil.DeterministicGenesisState(t, validators)

	b := testutil.NewBeaconBlock()
	require.NoError(t, db.SaveBlock(context.Background(), wrapper.WrappedPhase0SignedBeaconBlock(b)))
	root, err := b.Block.HashTreeRoot()
	require.NoError(t, err)

	aggBits := bitfield.NewBitlist(3)
	aggBits.SetBitAt(0, true)
	att := &ethpb.Attestation{
		Data: &ethpb.AttestationData{
			Slot:            1,
			BeaconBlockRoot: root[:],
			Source:          &ethpb.Checkpoint{Epoch: 0, Root: bytesutil.PadTo([]byte("hello-world"), 32)},
			Target:          &ethpb.Checkpoint{Epoch: 0, Root: bytesutil.PadTo([]byte("hello-world"), 32)},
		},
		AggregationBits: aggBits,
		Signature:       make([]byte, 96),
	}

	aggregateAndProof := &ethpb.AggregateAttestationAndProof{
		Aggregate:      att,
		SelectionProof: make([]byte, 96),
	}
	signedAggregateAndProof := &ethpb.SignedAggregateAttestationAndProof{Message: aggregateAndProof, Signature: make([]byte, 96)}

	require.NoError(t, beaconState.SetGenesisTime(uint64(time.Now().Unix())))
	c, err := lru.New(10)
	require.NoError(t, err)
	r := &Service{
		cfg: &Config{
			AttPool:     attestations.NewPool(),
			P2P:         p,
			DB:          db,
			InitialSync: &mockSync.Sync{IsSyncing: false},
			Chain: &mock.ChainService{Genesis: time.Now(),
				State: beaconState},
			OperationNotifier: (&mock.ChainService{}).OperationNotifier(),
		},
		seenAttestationCache: c,
		blkRootToPendingAtts: make(map[[32]byte][]*ethpb.SignedAggregateAttestationAndProof),
	}
	err = r.initCaches()
	require.NoError(t, err)

	buf := new(bytes.Buffer)
	_, err = p.Encoding().EncodeGossip(buf, signedAggregateAndProof)
	require.NoError(t, err)

	topic := p2p.GossipTypeMapping[reflect.TypeOf(signedAggregateAndProof)]
	msg := &pubsub.Message{
		Message: &pubsubpb.Message{
			Data:  buf.Bytes(),
			Topic: &topic,
		},
	}

	require.NoError(t, r.cfg.AttPool.SaveBlockAttestation(att))
	if r.validateAggregateAndProof(context.Background(), "", msg) == pubsub.ValidationAccept {
		t.Error("Expected validate to fail")
	}
}

func TestValidateAggregateAndProof_CanValidate(t *testing.T) {
	db := dbtest.SetupDB(t)
	p := p2ptest.NewTestP2P(t)

	validators := uint64(256)
	beaconState, privKeys := testutil.DeterministicGenesisState(t, validators)

	b := testutil.NewBeaconBlock()
	require.NoError(t, db.SaveBlock(context.Background(), wrapper.WrappedPhase0SignedBeaconBlock(b)))
	root, err := b.Block.HashTreeRoot()
	require.NoError(t, err)
	s, err := testutil.NewBeaconState()
	require.NoError(t, err)
	require.NoError(t, db.SaveState(context.Background(), s, root))

	aggBits := bitfield.NewBitlist(validators / uint64(params.BeaconConfig().SlotsPerEpoch))
	aggBits.SetBitAt(0, true)
	att := &ethpb.Attestation{
		Data: &ethpb.AttestationData{
			BeaconBlockRoot: root[:],
			Source:          &ethpb.Checkpoint{Epoch: 0, Root: bytesutil.PadTo([]byte("hello-world"), 32)},
			Target:          &ethpb.Checkpoint{Epoch: 0, Root: root[:]},
		},
		AggregationBits: aggBits,
	}

	committee, err := helpers.BeaconCommitteeFromState(beaconState, att.Data.Slot, att.Data.CommitteeIndex)
	assert.NoError(t, err)
	attestingIndices, err := attestationutil.AttestingIndices(att.AggregationBits, committee)
	require.NoError(t, err)
	assert.NoError(t, err)
	attesterDomain, err := helpers.Domain(beaconState.Fork(), 0, params.BeaconConfig().DomainBeaconAttester, beaconState.GenesisValidatorRoot())
	assert.NoError(t, err)
	hashTreeRoot, err := helpers.ComputeSigningRoot(att.Data, attesterDomain)
	assert.NoError(t, err)
	sigs := make([]bls.Signature, len(attestingIndices))
	for i, indice := range attestingIndices {
		sig := privKeys[indice].Sign(hashTreeRoot[:])
		sigs[i] = sig
	}
	att.Signature = bls.AggregateSignatures(sigs).Marshal()
	ai := committee[0]
	sszUint := types.SSZUint64(att.Data.Slot)
	sig, err := helpers.ComputeDomainAndSign(beaconState, 0, &sszUint, params.BeaconConfig().DomainSelectionProof, privKeys[ai])
	require.NoError(t, err)
	aggregateAndProof := &ethpb.AggregateAttestationAndProof{
		SelectionProof:  sig,
		Aggregate:       att,
		AggregatorIndex: ai,
	}
	signedAggregateAndProof := &ethpb.SignedAggregateAttestationAndProof{Message: aggregateAndProof}
	signedAggregateAndProof.Signature, err = helpers.ComputeDomainAndSign(beaconState, 0, signedAggregateAndProof.Message, params.BeaconConfig().DomainAggregateAndProof, privKeys[ai])
	require.NoError(t, err)

	require.NoError(t, beaconState.SetGenesisTime(uint64(time.Now().Unix())))
	c, err := lru.New(10)
	require.NoError(t, err)
	r := &Service{
		cfg: &Config{
			P2P:         p,
			DB:          db,
			InitialSync: &mockSync.Sync{IsSyncing: false},
			Chain: &mock.ChainService{Genesis: time.Now(),
				State:            beaconState,
				ValidAttestation: true,
				FinalizedCheckPoint: &ethpb.Checkpoint{
					Epoch: 0,
					Root:  att.Data.BeaconBlockRoot,
				}},
			AttPool:           attestations.NewPool(),
			OperationNotifier: (&mock.ChainService{}).OperationNotifier(),
		},
		seenAttestationCache: c,
	}
	err = r.initCaches()
	require.NoError(t, err)

	buf := new(bytes.Buffer)
	_, err = p.Encoding().EncodeGossip(buf, signedAggregateAndProof)
	require.NoError(t, err)

	topic := p2p.GossipTypeMapping[reflect.TypeOf(signedAggregateAndProof)]
	d, err := r.currentForkDigest()
	assert.NoError(t, err)
	topic = r.addDigestToTopic(topic, d)
	msg := &pubsub.Message{
		Message: &pubsubpb.Message{
			Data:  buf.Bytes(),
			Topic: &topic,
		},
	}

	assert.Equal(t, pubsub.ValidationAccept, r.validateAggregateAndProof(context.Background(), "", msg), "Validated status is false")
	assert.NotNil(t, msg.ValidatorData, "Did not set validator data")
}

func TestVerifyIndexInCommittee_SeenAggregatorEpoch(t *testing.T) {
	db := dbtest.SetupDB(t)
	p := p2ptest.NewTestP2P(t)

	validators := uint64(256)
	beaconState, privKeys := testutil.DeterministicGenesisState(t, validators)

	b := testutil.NewBeaconBlock()
	require.NoError(t, db.SaveBlock(context.Background(), wrapper.WrappedPhase0SignedBeaconBlock(b)))
	root, err := b.Block.HashTreeRoot()
	require.NoError(t, err)
	s, err := testutil.NewBeaconState()
	require.NoError(t, err)
	require.NoError(t, db.SaveState(context.Background(), s, root))

	aggBits := bitfield.NewBitlist(validators / uint64(params.BeaconConfig().SlotsPerEpoch))
	aggBits.SetBitAt(0, true)
	att := &ethpb.Attestation{
		Data: &ethpb.AttestationData{
			BeaconBlockRoot: root[:],
			Source:          &ethpb.Checkpoint{Epoch: 0, Root: bytesutil.PadTo([]byte("hello-world"), 32)},
			Target:          &ethpb.Checkpoint{Epoch: 0, Root: root[:]},
		},
		AggregationBits: aggBits,
	}

	committee, err := helpers.BeaconCommitteeFromState(beaconState, att.Data.Slot, att.Data.CommitteeIndex)
	require.NoError(t, err)
	attestingIndices, err := attestationutil.AttestingIndices(att.AggregationBits, committee)
	require.NoError(t, err)
	attesterDomain, err := helpers.Domain(beaconState.Fork(), 0, params.BeaconConfig().DomainBeaconAttester, beaconState.GenesisValidatorRoot())
	require.NoError(t, err)
	hashTreeRoot, err := helpers.ComputeSigningRoot(att.Data, attesterDomain)
	assert.NoError(t, err)
	sigs := make([]bls.Signature, len(attestingIndices))
	for i, indice := range attestingIndices {
		sig := privKeys[indice].Sign(hashTreeRoot[:])
		sigs[i] = sig
	}
	att.Signature = bls.AggregateSignatures(sigs).Marshal()
	ai := committee[0]
	sszUint := types.SSZUint64(att.Data.Slot)
	sig, err := helpers.ComputeDomainAndSign(beaconState, 0, &sszUint, params.BeaconConfig().DomainSelectionProof, privKeys[ai])
	require.NoError(t, err)
	aggregateAndProof := &ethpb.AggregateAttestationAndProof{
		SelectionProof:  sig,
		Aggregate:       att,
		AggregatorIndex: ai,
	}
	signedAggregateAndProof := &ethpb.SignedAggregateAttestationAndProof{Message: aggregateAndProof}
	signedAggregateAndProof.Signature, err = helpers.ComputeDomainAndSign(beaconState, 0, signedAggregateAndProof.Message, params.BeaconConfig().DomainAggregateAndProof, privKeys[ai])
	require.NoError(t, err)
	require.NoError(t, beaconState.SetGenesisTime(uint64(time.Now().Unix())))

	c, err := lru.New(10)
	require.NoError(t, err)
	r := &Service{
		cfg: &Config{
			P2P:         p,
			DB:          db,
			InitialSync: &mockSync.Sync{IsSyncing: false},
			Chain: &mock.ChainService{Genesis: time.Now(),
				ValidatorsRoot:   [32]byte{'A'},
				State:            beaconState,
				ValidAttestation: true,
				FinalizedCheckPoint: &ethpb.Checkpoint{
					Epoch: 0,
					Root:  signedAggregateAndProof.Message.Aggregate.Data.BeaconBlockRoot,
				}},

			AttPool:           attestations.NewPool(),
			OperationNotifier: (&mock.ChainService{}).OperationNotifier(),
		},
		seenAttestationCache: c,
	}
	err = r.initCaches()
	require.NoError(t, err)

	buf := new(bytes.Buffer)
	_, err = p.Encoding().EncodeGossip(buf, signedAggregateAndProof)
	require.NoError(t, err)

	topic := p2p.GossipTypeMapping[reflect.TypeOf(signedAggregateAndProof)]
	d, err := r.currentForkDigest()
	assert.NoError(t, err)
	topic = r.addDigestToTopic(topic, d)
	msg := &pubsub.Message{
		Message: &pubsubpb.Message{
			Data:  buf.Bytes(),
			Topic: &topic,
		},
	}

	require.Equal(t, pubsub.ValidationAccept, r.validateAggregateAndProof(context.Background(), "", msg), "Validated status is false")

	// Should fail with another attestation in the same epoch.
	signedAggregateAndProof.Message.Aggregate.Data.Slot++
	buf = new(bytes.Buffer)
	_, err = p.Encoding().EncodeGossip(buf, signedAggregateAndProof)
	require.NoError(t, err)
	msg = &pubsub.Message{
		Message: &pubsubpb.Message{
			Data:  buf.Bytes(),
			Topic: &topic,
		},
	}

	time.Sleep(10 * time.Millisecond) // Wait for cached value to pass through buffers.
	if r.validateAggregateAndProof(context.Background(), "", msg) == pubsub.ValidationAccept {
		t.Fatal("Validated status is true")
	}
}

func TestValidateAggregateAndProof_BadBlock(t *testing.T) {

	db := dbtest.SetupDB(t)
	p := p2ptest.NewTestP2P(t)

	validators := uint64(256)
	beaconState, privKeys := testutil.DeterministicGenesisState(t, validators)

	b := testutil.NewBeaconBlock()
	root, err := b.Block.HashTreeRoot()
	require.NoError(t, err)
	s, err := testutil.NewBeaconState()
	require.NoError(t, err)
	require.NoError(t, db.SaveState(context.Background(), s, root))

	aggBits := bitfield.NewBitlist(validators / uint64(params.BeaconConfig().SlotsPerEpoch))
	aggBits.SetBitAt(0, true)
	att := &ethpb.Attestation{
		Data: &ethpb.AttestationData{
			BeaconBlockRoot: root[:],
			Source:          &ethpb.Checkpoint{Epoch: 0, Root: bytesutil.PadTo([]byte("hello-world"), 32)},
			Target:          &ethpb.Checkpoint{Epoch: 0, Root: root[:]},
		},
		AggregationBits: aggBits,
	}

	committee, err := helpers.BeaconCommitteeFromState(beaconState, att.Data.Slot, att.Data.CommitteeIndex)
	assert.NoError(t, err)
	attestingIndices, err := attestationutil.AttestingIndices(att.AggregationBits, committee)
	require.NoError(t, err)
	assert.NoError(t, err)
	attesterDomain, err := helpers.Domain(beaconState.Fork(), 0, params.BeaconConfig().DomainBeaconAttester, beaconState.GenesisValidatorRoot())
	assert.NoError(t, err)
	hashTreeRoot, err := helpers.ComputeSigningRoot(att.Data, attesterDomain)
	assert.NoError(t, err)
	sigs := make([]bls.Signature, len(attestingIndices))
	for i, indice := range attestingIndices {
		sig := privKeys[indice].Sign(hashTreeRoot[:])
		sigs[i] = sig
	}
	att.Signature = bls.AggregateSignatures(sigs).Marshal()
	ai := committee[0]
	sszUint := types.SSZUint64(att.Data.Slot)
	sig, err := helpers.ComputeDomainAndSign(beaconState, 0, &sszUint, params.BeaconConfig().DomainSelectionProof, privKeys[ai])
	require.NoError(t, err)

	aggregateAndProof := &ethpb.AggregateAttestationAndProof{
		SelectionProof:  sig,
		Aggregate:       att,
		AggregatorIndex: ai,
	}
	signedAggregateAndProof := &ethpb.SignedAggregateAttestationAndProof{Message: aggregateAndProof}
	signedAggregateAndProof.Signature, err = helpers.ComputeDomainAndSign(beaconState, 0, signedAggregateAndProof.Message, params.BeaconConfig().DomainAggregateAndProof, privKeys[ai])
	require.NoError(t, err)

	require.NoError(t, beaconState.SetGenesisTime(uint64(time.Now().Unix())))
	c, err := lru.New(10)
	require.NoError(t, err)
	r := &Service{
		cfg: &Config{
			P2P:         p,
			DB:          db,
			InitialSync: &mockSync.Sync{IsSyncing: false},
			Chain: &mock.ChainService{Genesis: time.Now(),
				State:            beaconState,
				ValidAttestation: true,
				FinalizedCheckPoint: &ethpb.Checkpoint{
					Epoch: 0,
				}},
			AttPool:           attestations.NewPool(),
			OperationNotifier: (&mock.ChainService{}).OperationNotifier(),
		},
		seenAttestationCache: c,
	}
	err = r.initCaches()
	require.NoError(t, err)
	// Set beacon block as bad.
	r.setBadBlock(context.Background(), root)
	buf := new(bytes.Buffer)
	_, err = p.Encoding().EncodeGossip(buf, signedAggregateAndProof)
	require.NoError(t, err)

	topic := p2p.GossipTypeMapping[reflect.TypeOf(signedAggregateAndProof)]
	msg := &pubsub.Message{
		Message: &pubsubpb.Message{
			Data:  buf.Bytes(),
			Topic: &topic,
		},
	}

	assert.Equal(t, pubsub.ValidationReject, r.validateAggregateAndProof(context.Background(), "", msg), "Validated status is true")
}

func TestValidateAggregateAndProof_RejectWhenAttEpochDoesntEqualTargetEpoch(t *testing.T) {
	db := dbtest.SetupDB(t)
	p := p2ptest.NewTestP2P(t)

	validators := uint64(256)
	beaconState, privKeys := testutil.DeterministicGenesisState(t, validators)

	b := testutil.NewBeaconBlock()
	require.NoError(t, db.SaveBlock(context.Background(), wrapper.WrappedPhase0SignedBeaconBlock(b)))
	root, err := b.Block.HashTreeRoot()
	require.NoError(t, err)
	s, err := testutil.NewBeaconState()
	require.NoError(t, err)
	require.NoError(t, db.SaveState(context.Background(), s, root))

	aggBits := bitfield.NewBitlist(validators / uint64(params.BeaconConfig().SlotsPerEpoch))
	aggBits.SetBitAt(0, true)
	att := &ethpb.Attestation{
		Data: &ethpb.AttestationData{
			BeaconBlockRoot: root[:],
			Source:          &ethpb.Checkpoint{Epoch: 0, Root: bytesutil.PadTo([]byte("hello-world"), 32)},
			Target:          &ethpb.Checkpoint{Epoch: 1, Root: root[:]},
		},
		AggregationBits: aggBits,
	}

	committee, err := helpers.BeaconCommitteeFromState(beaconState, att.Data.Slot, att.Data.CommitteeIndex)
	assert.NoError(t, err)
	attestingIndices, err := attestationutil.AttestingIndices(att.AggregationBits, committee)
	require.NoError(t, err)
	assert.NoError(t, err)
	attesterDomain, err := helpers.Domain(beaconState.Fork(), 0, params.BeaconConfig().DomainBeaconAttester, beaconState.GenesisValidatorRoot())
	assert.NoError(t, err)
	hashTreeRoot, err := helpers.ComputeSigningRoot(att.Data, attesterDomain)
	assert.NoError(t, err)
	sigs := make([]bls.Signature, len(attestingIndices))
	for i, indice := range attestingIndices {
		sig := privKeys[indice].Sign(hashTreeRoot[:])
		sigs[i] = sig
	}
	att.Signature = bls.AggregateSignatures(sigs).Marshal()
	ai := committee[0]
	sszUint := types.SSZUint64(att.Data.Slot)
	sig, err := helpers.ComputeDomainAndSign(beaconState, 0, &sszUint, params.BeaconConfig().DomainSelectionProof, privKeys[ai])
	require.NoError(t, err)
	aggregateAndProof := &ethpb.AggregateAttestationAndProof{
		SelectionProof:  sig,
		Aggregate:       att,
		AggregatorIndex: ai,
	}
	signedAggregateAndProof := &ethpb.SignedAggregateAttestationAndProof{Message: aggregateAndProof}
	signedAggregateAndProof.Signature, err = helpers.ComputeDomainAndSign(beaconState, 0, signedAggregateAndProof.Message, params.BeaconConfig().DomainAggregateAndProof, privKeys[ai])
	require.NoError(t, err)

	require.NoError(t, beaconState.SetGenesisTime(uint64(time.Now().Unix())))
	c, err := lru.New(10)
	require.NoError(t, err)
	r := &Service{
		cfg: &Config{
			P2P:         p,
			DB:          db,
			InitialSync: &mockSync.Sync{IsSyncing: false},
			Chain: &mock.ChainService{Genesis: time.Now(),
				State:            beaconState,
				ValidAttestation: true,
				FinalizedCheckPoint: &ethpb.Checkpoint{
					Epoch: 0,
					Root:  att.Data.BeaconBlockRoot,
				}},
			AttPool:           attestations.NewPool(),
			OperationNotifier: (&mock.ChainService{}).OperationNotifier(),
		},
		seenAttestationCache: c,
	}
	err = r.initCaches()
	require.NoError(t, err)

	buf := new(bytes.Buffer)
	_, err = p.Encoding().EncodeGossip(buf, signedAggregateAndProof)
	require.NoError(t, err)

	topic := p2p.GossipTypeMapping[reflect.TypeOf(signedAggregateAndProof)]
	msg := &pubsub.Message{
		Message: &pubsubpb.Message{
			Data:  buf.Bytes(),
			Topic: &topic,
		},
	}

	assert.Equal(t, pubsub.ValidationReject, r.validateAggregateAndProof(context.Background(), "", msg))
}
