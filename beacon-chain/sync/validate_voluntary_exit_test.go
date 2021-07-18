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

func setupValidExit(t *testing.T) (*ethpb.SignedVoluntaryExit, iface.BeaconState) {
	exit := &ethpb.SignedVoluntaryExit{
		Exit: &ethpb.VoluntaryExit{
			ValidatorIndex: 0,
			Epoch:          1 + params.BeaconConfig().ShardCommitteePeriod,
		},
	}
	registry := []*ethpb.Validator{
		{
			ExitEpoch:       params.BeaconConfig().FarFutureEpoch,
			ActivationEpoch: 0,
		},
	}
	state, err := v1.InitializeFromProto(&pb.BeaconState{
		Validators: registry,
		Fork: &pb.Fork{
			CurrentVersion:  params.BeaconConfig().GenesisForkVersion,
			PreviousVersion: params.BeaconConfig().GenesisForkVersion,
		},
		Slot: params.BeaconConfig().SlotsPerEpoch * 5,
	})
	require.NoError(t, err)
	err = state.SetSlot(state.Slot() + params.BeaconConfig().SlotsPerEpoch.Mul(uint64(params.BeaconConfig().ShardCommitteePeriod)))
	require.NoError(t, err)

	priv, err := bls.RandKey()
	require.NoError(t, err)
	exit.Signature, err = helpers.ComputeDomainAndSign(state, helpers.CurrentEpoch(state), exit.Exit, params.BeaconConfig().DomainVoluntaryExit, priv)
	require.NoError(t, err)

	val, err := state.ValidatorAtIndex(0)
	require.NoError(t, err)
	val.PublicKey = priv.PublicKey().Marshal()
	require.NoError(t, state.UpdateValidatorAtIndex(0, val))

	b := make([]byte, 32)
	_, err = rand.Read(b)
	require.NoError(t, err)

	return exit, state
}

func TestValidateVoluntaryExit_ValidExit(t *testing.T) {
	p := p2ptest.NewTestP2P(t)
	ctx := context.Background()

	exit, s := setupValidExit(t)

	c, err := lru.New(10)
	require.NoError(t, err)
	r := &Service{
		cfg: &Config{
			P2P: p,
			Chain: &mock.ChainService{
				State:   s,
				Genesis: time.Now(),
			},
			InitialSync: &mockSync.Sync{IsSyncing: false},
		},
		seenExitCache: c,
	}

	buf := new(bytes.Buffer)
	_, err = p.Encoding().EncodeGossip(buf, exit)
	require.NoError(t, err)
	topic := p2p.GossipTypeMapping[reflect.TypeOf(exit)]
	d, err := r.currentForkDigest()
	assert.NoError(t, err)
	topic = r.addDigestToTopic(topic, d)
	m := &pubsub.Message{
		Message: &pubsubpb.Message{
			Data:  buf.Bytes(),
			Topic: &topic,
		},
	}
	valid := r.validateVoluntaryExit(ctx, "", m) == pubsub.ValidationAccept
	assert.Equal(t, true, valid, "Failed validation")
	assert.NotNil(t, m.ValidatorData, "Decoded message was not set on the message validator data")
}

func TestValidateVoluntaryExit_InvalidExitSlot(t *testing.T) {
	p := p2ptest.NewTestP2P(t)
	ctx := context.Background()

	exit, s := setupValidExit(t)
	// Set state slot to 1 to cause exit object fail to verify.
	require.NoError(t, s.SetSlot(1))
	c, err := lru.New(10)
	require.NoError(t, err)
	r := &Service{
		cfg: &Config{
			P2P: p,
			Chain: &mock.ChainService{
				State: s,
			},
			InitialSync: &mockSync.Sync{IsSyncing: false},
		},
		seenExitCache: c,
	}

	buf := new(bytes.Buffer)
	_, err = p.Encoding().EncodeGossip(buf, exit)
	require.NoError(t, err)
	topic := p2p.GossipTypeMapping[reflect.TypeOf(exit)]
	m := &pubsub.Message{
		Message: &pubsubpb.Message{
			Data:  buf.Bytes(),
			Topic: &topic,
		},
	}
	valid := r.validateVoluntaryExit(ctx, "", m) == pubsub.ValidationAccept
	assert.Equal(t, false, valid, "passed validation")
}

func TestValidateVoluntaryExit_ValidExit_Syncing(t *testing.T) {
	p := p2ptest.NewTestP2P(t)
	ctx := context.Background()

	exit, s := setupValidExit(t)

	r := &Service{
		cfg: &Config{
			P2P: p,
			Chain: &mock.ChainService{
				State: s,
			},
			InitialSync: &mockSync.Sync{IsSyncing: true},
		},
	}
	buf := new(bytes.Buffer)
	_, err := p.Encoding().EncodeGossip(buf, exit)
	require.NoError(t, err)
	topic := p2p.GossipTypeMapping[reflect.TypeOf(exit)]
	m := &pubsub.Message{
		Message: &pubsubpb.Message{
			Data:  buf.Bytes(),
			Topic: &topic,
		},
	}
	valid := r.validateVoluntaryExit(ctx, "", m) == pubsub.ValidationAccept
	assert.Equal(t, false, valid, "Validation should have failed")
}
