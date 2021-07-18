package p2p

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p-core/peer"
	pubsubpb "github.com/libp2p/go-libp2p-pubsub/pb"
	mock "github.com/prysmaticlabs/prysm/beacon-chain/blockchain/testing"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/feed"
	statefeed "github.com/prysmaticlabs/prysm/beacon-chain/core/feed/state"
	"github.com/prysmaticlabs/prysm/beacon-chain/p2p/encoder"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/prysmaticlabs/prysm/shared/p2putils"
	"github.com/prysmaticlabs/prysm/shared/testutil/assert"
	"github.com/prysmaticlabs/prysm/shared/timeutils"
	"github.com/stretchr/testify/require"
)

func TestService_CanSubscribe(t *testing.T) {
	currentFork := [4]byte{0x01, 0x02, 0x03, 0x04}
	validProtocolSuffix := "/" + encoder.ProtocolSuffixSSZSnappy
	genesisTime := time.Now()
	valRoot := [32]byte{}
	digest, err := p2putils.CreateForkDigest(genesisTime, valRoot[:])
	assert.NoError(t, err)
	type test struct {
		name  string
		topic string
		want  bool
	}
	tests := []test{
		{
			name:  "block topic on current fork",
			topic: fmt.Sprintf(BlockSubnetTopicFormat, digest) + validProtocolSuffix,
			want:  true,
		},
		{
			name:  "block topic on unknown fork",
			topic: fmt.Sprintf(BlockSubnetTopicFormat, [4]byte{0xFF, 0xEE, 0x56, 0x21}) + validProtocolSuffix,
			want:  false,
		},
		{
			name:  "block topic missing protocol suffix",
			topic: fmt.Sprintf(BlockSubnetTopicFormat, currentFork),
			want:  false,
		},
		{
			name:  "block topic wrong protocol suffix",
			topic: fmt.Sprintf(BlockSubnetTopicFormat, currentFork) + "/foobar",
			want:  false,
		},
		{
			name:  "erroneous topic",
			topic: "hey, want to foobar?",
			want:  false,
		},
		{
			name:  "erroneous topic that has the correct amount of slashes",
			topic: "hey, want to foobar?////",
			want:  false,
		},
		{
			name:  "bad prefix",
			topic: fmt.Sprintf("/eth3/%x/foobar", digest) + validProtocolSuffix,
			want:  false,
		},
		{
			name:  "topic not in gossip mapping",
			topic: fmt.Sprintf("/eth2/%x/foobar", digest) + validProtocolSuffix,
			want:  false,
		},
		{
			name:  "att subnet topic on current fork",
			topic: fmt.Sprintf(AttestationSubnetTopicFormat, digest, 55 /*subnet*/) + validProtocolSuffix,
			want:  true,
		},
		{
			name:  "att subnet topic on unknown fork",
			topic: fmt.Sprintf(AttestationSubnetTopicFormat, [4]byte{0xCC, 0xBB, 0xAA, 0xA1} /*fork digest*/, 54 /*subnet*/) + validProtocolSuffix,
			want:  false,
		},
	}

	// Ensure all gossip topic mappings pass validation.
	for _, topic := range AllTopics() {
		formatting := []interface{}{digest}

		// Special case for attestation subnets which have a second formatting placeholder.
		if topic == AttestationSubnetTopicFormat || topic == SyncCommitteeSubnetTopicFormat {
			formatting = append(formatting, 0 /* some subnet ID */)
		}

		tt := test{
			name:  topic,
			topic: fmt.Sprintf(topic, formatting...) + validProtocolSuffix,
			want:  true,
		}
		tests = append(tests, tt)
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Service{
				genesisValidatorsRoot: valRoot[:],
				genesisTime:           genesisTime,
			}
			if got := s.CanSubscribe(tt.topic); got != tt.want {
				t.Errorf("CanSubscribe(%s) = %v, want %v", tt.topic, got, tt.want)
			}
		})
	}
}

func TestService_CanSubscribe_uninitialized(t *testing.T) {
	s := &Service{}
	require.False(t, s.CanSubscribe("foo"))
}

func Test_scanfcheck(t *testing.T) {
	type args struct {
		input  string
		format string
	}
	tests := []struct {
		name    string
		args    args
		want    int
		wantErr bool
	}{
		{
			name: "no formatting, exact match",
			args: args{
				input:  "/foo/bar/zzzzzzzzzzzz/1234567",
				format: "/foo/bar/zzzzzzzzzzzz/1234567",
			},
			want:    0,
			wantErr: false,
		},
		{
			name: "no formatting, mismatch",
			args: args{
				input:  "/foo/bar/zzzzzzzzzzzz/1234567",
				format: "/bar/foo/yyyyyy/7654321",
			},
			want:    0,
			wantErr: true,
		},
		{
			name: "formatting, match",
			args: args{
				input:  "/foo/bar/abcdef/topic_11",
				format: "/foo/bar/%x/topic_%d",
			},
			want:    2,
			wantErr: false,
		},
		{
			name: "formatting, incompatible bytes",
			args: args{
				input:  "/foo/bar/zzzzzz/topic_11",
				format: "/foo/bar/%x/topic_%d",
			},
			want:    0,
			wantErr: true,
		},
		{ // Note: This method only supports integer compatible formatting values.
			name: "formatting, string match",
			args: args{
				input:  "/foo/bar/zzzzzz/topic_11",
				format: "/foo/bar/%s/topic_%d",
			},
			want:    0,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := scanfcheck(tt.args.input, tt.args.format)
			if (err != nil) != tt.wantErr {
				t.Errorf("scanfcheck() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("scanfcheck() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGossipTopicMapping_scanfcheck_GossipTopicFormattingSanityCheck(t *testing.T) {
	// scanfcheck only supports integer based substitutions at the moment. Any others will
	// inaccurately fail validation.
	for _, topic := range AllTopics() {
		t.Run(topic, func(t *testing.T) {
			for i, c := range topic {
				if string(c) == "%" {
					next := string(topic[i+1])
					if next != "d" && next != "x" {
						t.Errorf("Topic %s has formatting incompatiable with scanfcheck. Only %%d and %%x are supported", topic)
					}
				}
			}
		})
	}
}

func TestService_FilterIncomingSubscriptions(t *testing.T) {
	validProtocolSuffix := "/" + encoder.ProtocolSuffixSSZSnappy
	genesisTime := time.Now()
	valRoot := [32]byte{}
	digest, err := p2putils.CreateForkDigest(genesisTime, valRoot[:])
	assert.NoError(t, err)
	type args struct {
		id   peer.ID
		subs []*pubsubpb.RPC_SubOpts
	}
	tests := []struct {
		name    string
		args    args
		want    []*pubsubpb.RPC_SubOpts
		wantErr bool
	}{
		{
			name: "too many topics",
			args: args{
				subs: make([]*pubsubpb.RPC_SubOpts, pubsubSubscriptionRequestLimit+1),
			},
			wantErr: true,
		},
		{
			name: "exactly topic limit",
			args: args{
				subs: make([]*pubsubpb.RPC_SubOpts, pubsubSubscriptionRequestLimit),
			},
			wantErr: false,
			want:    nil, // No topics matched filters.
		},
		{
			name: "blocks topic",
			args: args{
				subs: []*pubsubpb.RPC_SubOpts{
					{
						Subscribe: func() *bool {
							b := true
							return &b
						}(),
						Topicid: func() *string {
							s := fmt.Sprintf(BlockSubnetTopicFormat, digest) + validProtocolSuffix
							return &s
						}(),
					},
				},
			},
			wantErr: false,
			want: []*pubsubpb.RPC_SubOpts{
				{
					Subscribe: func() *bool {
						b := true
						return &b
					}(),
					Topicid: func() *string {
						s := fmt.Sprintf(BlockSubnetTopicFormat, digest) + validProtocolSuffix
						return &s
					}(),
				},
			},
		},
		{
			name: "blocks topic duplicated",
			args: args{
				subs: []*pubsubpb.RPC_SubOpts{
					{
						Subscribe: func() *bool {
							b := true
							return &b
						}(),
						Topicid: func() *string {
							s := fmt.Sprintf(BlockSubnetTopicFormat, digest) + validProtocolSuffix
							return &s
						}(),
					},
					{
						Subscribe: func() *bool {
							b := true
							return &b
						}(),
						Topicid: func() *string {
							s := fmt.Sprintf(BlockSubnetTopicFormat, digest) + validProtocolSuffix
							return &s
						}(),
					},
				},
			},
			wantErr: false,
			want: []*pubsubpb.RPC_SubOpts{ // Duplicated topics are only present once after filtering.
				{
					Subscribe: func() *bool {
						b := true
						return &b
					}(),
					Topicid: func() *string {
						s := fmt.Sprintf(BlockSubnetTopicFormat, digest) + validProtocolSuffix
						return &s
					}(),
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Service{
				genesisValidatorsRoot: valRoot[:],
				genesisTime:           genesisTime,
			}
			got, err := s.FilterIncomingSubscriptions(tt.args.id, tt.args.subs)
			if (err != nil) != tt.wantErr {
				t.Errorf("FilterIncomingSubscriptions() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("FilterIncomingSubscriptions() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestService_MonitorsStateForkUpdates(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	notifier := &mock.MockStateNotifier{}
	s, err := NewService(ctx, &Config{
		StateNotifier: notifier,
	})
	require.NoError(t, err)

	require.False(t, s.isInitialized())

	go s.awaitStateInitialized()

	for n := 0; n == 0; {
		if ctx.Err() != nil {
			t.Fatal(ctx.Err())
		}
		n = notifier.StateFeed().Send(&feed.Event{
			Type: statefeed.Initialized,
			Data: &statefeed.InitializedData{
				StartTime:             timeutils.Now(),
				GenesisValidatorsRoot: bytesutil.PadTo([]byte("genesis"), 32),
			},
		})
	}

	time.Sleep(50 * time.Millisecond)

	require.True(t, s.isInitialized())
}
