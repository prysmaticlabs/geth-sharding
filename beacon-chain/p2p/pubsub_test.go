package p2p

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/golang/snappy"
	pubsubpb "github.com/libp2p/go-libp2p-pubsub/pb"
	"github.com/pkg/errors"
	mock "github.com/prysmaticlabs/prysm/beacon-chain/blockchain/testing"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	"github.com/prysmaticlabs/prysm/beacon-chain/p2p/encoder"
	testp2p "github.com/prysmaticlabs/prysm/beacon-chain/p2p/testing"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/prysmaticlabs/prysm/shared/hashutil"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/testutil/assert"
	"github.com/prysmaticlabs/prysm/shared/testutil/require"
)

func TestService_PublishToTopicConcurrentMapWrite(t *testing.T) {
	s, err := NewService(context.Background(), &Config{
		StateNotifier: &mock.MockStateNotifier{},
	})
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	go s.awaitStateInitialized()
	fd := initializeStateWithForkDigest(ctx, t, s.stateNotifier.StateFeed())

	if !s.isInitialized() {
		t.Fatal("service was not initialized")
	}

	// Set up two connected test hosts.
	p0 := testp2p.NewTestP2P(t)
	p1 := testp2p.NewTestP2P(t)
	p0.Connect(p1)
	s.host = p0.BHost
	s.pubsub = p0.PubSub()

	topic := fmt.Sprintf(BlockSubnetTopicFormat, fd) + "/" + encoder.ProtocolSuffixSSZSnappy

	// Establish the remote peer to be subscribed to the outgoing topic.
	_, err = p1.SubscribeToTopic(topic)
	require.NoError(t, err)

	wg := sync.WaitGroup{}
	wg.Add(10)
	for i := 0; i < 10; i++ {
		go func(i int) {
			assert.NoError(t, s.PublishToTopic(ctx, topic, []byte{}))
			wg.Done()
		}(i)
	}
	wg.Wait()
}

func TestMessageIDFunction_HashesCorrectly(t *testing.T) {
	s := &Service{
		cfg: &Config{
			TCPPort: 0,
			UDPPort: 0,
		},
		genesisTime:           time.Now(),
		genesisValidatorsRoot: bytesutil.PadTo([]byte{'A'}, 32),
	}
	d, err := s.currentForkDigest()
	assert.NoError(t, err)
	tpc := fmt.Sprintf(BlockSubnetTopicFormat, d)
	invalidSnappy := [32]byte{'J', 'U', 'N', 'K'}
	pMsg := &pubsubpb.Message{Data: invalidSnappy[:], Topic: &tpc}
	hashedData := hashutil.Hash(append(params.BeaconNetworkConfig().MessageDomainInvalidSnappy[:], pMsg.Data...))
	msgID := string(hashedData[:20])
	assert.Equal(t, msgID, s.msgIDFunction(pMsg), "Got incorrect msg id")

	validObj := [32]byte{'v', 'a', 'l', 'i', 'd'}
	enc := snappy.Encode(nil, validObj[:])
	nMsg := &pubsubpb.Message{Data: enc, Topic: &tpc}
	hashedData = hashutil.Hash(append(params.BeaconNetworkConfig().MessageDomainValidSnappy[:], validObj[:]...))
	msgID = string(hashedData[:20])
	assert.Equal(t, msgID, s.msgIDFunction(nMsg), "Got incorrect msg id")
}

func TestMessageIDFunction_HashesCorrectlyAltair(t *testing.T) {
	s := &Service{
		cfg: &Config{
			TCPPort: 0,
			UDPPort: 0,
		},
		genesisTime:           time.Now(),
		genesisValidatorsRoot: bytesutil.PadTo([]byte{'A'}, 32),
	}
	d, err := helpers.ComputeForkDigest(params.BeaconConfig().AltairForkVersion, s.genesisValidatorsRoot)
	assert.NoError(t, err)
	tpc := fmt.Sprintf(BlockSubnetTopicFormat, d)
	topicLen := uint64(len(tpc))
	topicLenBytes := bytesutil.Uint64ToBytesLittleEndian(topicLen)
	invalidSnappy := [32]byte{'J', 'U', 'N', 'K'}
	pMsg := &pubsubpb.Message{Data: invalidSnappy[:], Topic: &tpc}
	// Create object to hash
	combinedObj := append(params.BeaconNetworkConfig().MessageDomainInvalidSnappy[:], topicLenBytes...)
	combinedObj = append(combinedObj, tpc...)
	combinedObj = append(combinedObj, pMsg.Data...)
	hashedData := hashutil.Hash(combinedObj)
	msgID := string(hashedData[:20])
	assert.Equal(t, msgID, s.msgIDFunction(pMsg), "Got incorrect msg id")

	validObj := [32]byte{'v', 'a', 'l', 'i', 'd'}
	enc := snappy.Encode(nil, validObj[:])
	nMsg := &pubsubpb.Message{Data: enc, Topic: &tpc}
	// Create object to hash
	combinedObj = append(params.BeaconNetworkConfig().MessageDomainValidSnappy[:], topicLenBytes...)
	combinedObj = append(combinedObj, tpc...)
	combinedObj = append(combinedObj, validObj[:]...)
	hashedData = hashutil.Hash(combinedObj)
	msgID = string(hashedData[:20])
	assert.Equal(t, msgID, s.msgIDFunction(nMsg), "Got incorrect msg id")
}

func TestExtractGossipDigest(t *testing.T) {
	tests := []struct {
		name    string
		topic   string
		want    [4]byte
		wantErr bool
		error   error
	}{
		{
			name:    "too short topic",
			topic:   "/eth2/",
			want:    [4]byte{},
			wantErr: true,
			error:   errors.New("invalid topic format"),
		},
		{
			name:    "invalid digest in topic",
			topic:   "/eth2/zzxxyyaa/beacon_block" + "/" + encoder.ProtocolSuffixSSZSnappy,
			want:    [4]byte{},
			wantErr: true,
			error:   errors.New("encoding/hex: invalid byte"),
		},
		{
			name:    "short digest",
			topic:   fmt.Sprintf(BlockSubnetTopicFormat, []byte{0xb5, 0x30, 0x3f}) + "/" + encoder.ProtocolSuffixSSZSnappy,
			want:    [4]byte{},
			wantErr: true,
			error:   errors.New("invalid digest length wanted"),
		},
		{
			name:    "valid topic",
			topic:   fmt.Sprintf(BlockSubnetTopicFormat, []byte{0xb5, 0x30, 0x3f, 0x2a}) + "/" + encoder.ProtocolSuffixSSZSnappy,
			want:    [4]byte{0xb5, 0x30, 0x3f, 0x2a},
			wantErr: false,
			error:   nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExtractGossipDigest(tt.topic)
			assert.Equal(t, err != nil, tt.wantErr)
			if tt.wantErr {
				assert.ErrorContains(t, tt.error.Error(), err)
			}
			assert.DeepEqual(t, tt.want, got)
		})
	}
}
