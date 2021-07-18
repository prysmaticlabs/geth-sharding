package sync

import (
	"bytes"
	"context"
	"crypto/rand"
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
	"github.com/prysmaticlabs/prysm/beacon-chain/p2p"
	p2ptest "github.com/prysmaticlabs/prysm/beacon-chain/p2p/testing"
	iface "github.com/prysmaticlabs/prysm/beacon-chain/state/interface"
	v1 "github.com/prysmaticlabs/prysm/beacon-chain/state/v1"
	mockSync "github.com/prysmaticlabs/prysm/beacon-chain/sync/initial-sync/testing"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	ethpb "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/shared/bls"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/testutil/assert"
	"github.com/prysmaticlabs/prysm/shared/testutil/require"
)

func setupValidProposerSlashing(t *testing.T) (*ethpb.ProposerSlashing, iface.BeaconState) {
	validators := make([]*ethpb.Validator, 100)
	for i := 0; i < len(validators); i++ {
		validators[i] = &ethpb.Validator{
			EffectiveBalance:  params.BeaconConfig().MaxEffectiveBalance,
			Slashed:           false,
			ExitEpoch:         params.BeaconConfig().FarFutureEpoch,
			WithdrawableEpoch: params.BeaconConfig().FarFutureEpoch,
			ActivationEpoch:   0,
		}
	}
	validatorBalances := make([]uint64, len(validators))
	for i := 0; i < len(validatorBalances); i++ {
		validatorBalances[i] = params.BeaconConfig().MaxEffectiveBalance
	}

	currentSlot := types.Slot(0)
	state, err := v1.InitializeFromProto(&pb.BeaconState{
		Validators: validators,
		Slot:       currentSlot,
		Balances:   validatorBalances,
		Fork: &pb.Fork{
			CurrentVersion:  params.BeaconConfig().GenesisForkVersion,
			PreviousVersion: params.BeaconConfig().GenesisForkVersion,
			Epoch:           0,
		},
		Slashings:   make([]uint64, params.BeaconConfig().EpochsPerSlashingsVector),
		RandaoMixes: make([][]byte, params.BeaconConfig().EpochsPerHistoricalVector),

		StateRoots:        make([][]byte, params.BeaconConfig().SlotsPerHistoricalRoot),
		BlockRoots:        make([][]byte, params.BeaconConfig().SlotsPerHistoricalRoot),
		LatestBlockHeader: &ethpb.BeaconBlockHeader{},
	})
	require.NoError(t, err)

	privKey, err := bls.RandKey()
	require.NoError(t, err)
	someRoot := [32]byte{1, 2, 3}
	someRoot2 := [32]byte{4, 5, 6}
	header1 := &ethpb.SignedBeaconBlockHeader{
		Header: &ethpb.BeaconBlockHeader{
			ProposerIndex: 1,
			Slot:          0,
			ParentRoot:    someRoot[:],
			StateRoot:     someRoot[:],
			BodyRoot:      someRoot[:],
		},
	}
	header1.Signature, err = helpers.ComputeDomainAndSign(state, helpers.CurrentEpoch(state), header1.Header, params.BeaconConfig().DomainBeaconProposer, privKey)
	require.NoError(t, err)

	header2 := &ethpb.SignedBeaconBlockHeader{
		Header: &ethpb.BeaconBlockHeader{
			ProposerIndex: 1,
			Slot:          0,
			ParentRoot:    someRoot2[:],
			StateRoot:     someRoot2[:],
			BodyRoot:      someRoot2[:],
		},
	}
	header2.Signature, err = helpers.ComputeDomainAndSign(state, helpers.CurrentEpoch(state), header2.Header, params.BeaconConfig().DomainBeaconProposer, privKey)
	require.NoError(t, err)

	slashing := &ethpb.ProposerSlashing{
		Header_1: header1,
		Header_2: header2,
	}
	val, err := state.ValidatorAtIndex(1)
	require.NoError(t, err)
	val.PublicKey = privKey.PublicKey().Marshal()
	require.NoError(t, state.UpdateValidatorAtIndex(1, val))

	b := make([]byte, 32)
	_, err = rand.Read(b)
	require.NoError(t, err)

	return slashing, state
}

func TestValidateProposerSlashing_ValidSlashing(t *testing.T) {
	p := p2ptest.NewTestP2P(t)
	ctx := context.Background()

	slashing, s := setupValidProposerSlashing(t)

	c, err := lru.New(10)
	require.NoError(t, err)
	r := &Service{
		cfg: &Config{
			P2P:         p,
			Chain:       &mock.ChainService{State: s, Genesis: time.Now()},
			InitialSync: &mockSync.Sync{IsSyncing: false},
		},
		seenProposerSlashingCache: c,
	}

	buf := new(bytes.Buffer)
	_, err = p.Encoding().EncodeGossip(buf, slashing)
	require.NoError(t, err)
	topic := p2p.GossipTypeMapping[reflect.TypeOf(slashing)]
	d, err := r.currentForkDigest()
	assert.NoError(t, err)
	topic = r.addDigestToTopic(topic, d)
	m := &pubsub.Message{
		Message: &pubsubpb.Message{
			Data:  buf.Bytes(),
			Topic: &topic,
		},
	}

	valid := r.validateProposerSlashing(ctx, "", m) == pubsub.ValidationAccept
	assert.Equal(t, true, valid, "Failed validation")
	assert.NotNil(t, m.ValidatorData, "Decoded message was not set on the message validator data")
}

func TestValidateProposerSlashing_ContextTimeout(t *testing.T) {
	p := p2ptest.NewTestP2P(t)

	slashing, state := setupValidProposerSlashing(t)
	slashing.Header_1.Header.Slot = 100000000
	err := state.SetJustificationBits(bitfield.Bitvector4{0x0F}) // 0b1111
	require.NoError(t, err)
	err = state.SetPreviousJustifiedCheckpoint(&ethpb.Checkpoint{Epoch: 0, Root: []byte{}})
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	c, err := lru.New(10)
	require.NoError(t, err)
	r := &Service{
		cfg: &Config{
			P2P:         p,
			Chain:       &mock.ChainService{State: state},
			InitialSync: &mockSync.Sync{IsSyncing: false},
		},
		seenProposerSlashingCache: c,
	}

	buf := new(bytes.Buffer)
	_, err = p.Encoding().EncodeGossip(buf, slashing)
	require.NoError(t, err)
	topic := p2p.GossipTypeMapping[reflect.TypeOf(slashing)]
	m := &pubsub.Message{
		Message: &pubsubpb.Message{
			Data:  buf.Bytes(),
			Topic: &topic,
		},
	}
	valid := r.validateProposerSlashing(ctx, "", m) == pubsub.ValidationAccept
	assert.Equal(t, false, valid, "Slashing from the far distant future should have timed out and returned false")
}

func TestValidateProposerSlashing_Syncing(t *testing.T) {
	p := p2ptest.NewTestP2P(t)
	ctx := context.Background()

	slashing, s := setupValidProposerSlashing(t)

	r := &Service{
		cfg: &Config{
			P2P:         p,
			Chain:       &mock.ChainService{State: s},
			InitialSync: &mockSync.Sync{IsSyncing: true},
		},
	}

	buf := new(bytes.Buffer)
	_, err := p.Encoding().EncodeGossip(buf, slashing)
	require.NoError(t, err)
	topic := p2p.GossipTypeMapping[reflect.TypeOf(slashing)]
	m := &pubsub.Message{
		Message: &pubsubpb.Message{
			Data:  buf.Bytes(),
			Topic: &topic,
		},
	}
	valid := r.validateProposerSlashing(ctx, "", m) == pubsub.ValidationAccept
	assert.Equal(t, false, valid, "Did not fail validation")
}
