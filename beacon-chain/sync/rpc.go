package sync

import (
	"context"
	"errors"
	"time"

	"github.com/gogo/protobuf/proto"
	libp2pcore "github.com/libp2p/go-libp2p-core"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/prysmaticlabs/prysm/shared/roughtime"
)

// Time to first byte timeout. The maximum time to wait for first byte of
// request response (time-to-first-byte). The client is expected to give up if
// they don't receive the first byte within 5 seconds.
var ttfbTimeout = 5 * time.Second

// rpcHandler is responsible for handling and responding to any incoming message.
// This method may return an error to internal monitoring, but the error will
// not be relayed to the peer.
type rpcHandler func(context.Context, proto.Message, libp2pcore.Stream) error

// TODO(3147): Delete after all handlers implemented.
func notImplementedRPCHandler(_ context.Context, _ proto.Message, _ libp2pcore.Stream) error {
	return errors.New("not implemented")
}

// registerRPC for a given topic with an expected protobuf message type.
func (r *RegularSync) registerRPC(topic string, base proto.Message, handle rpcHandler) {
	topic += r.p2p.Encoding().ProtocolSuffix()
	log := log.WithField("topic", topic)
	r.p2p.SetStreamHandler(topic, func(stream network.Stream) {
		ctx, cancel := context.WithTimeout(r.ctx, ttfbTimeout)
		defer cancel()
		defer stream.Close()

		if err := stream.SetReadDeadline(roughtime.Now().Add(ttfbTimeout)); err != nil {
			log.WithError(err).Error("Could not set stream read deadline")
			return
		}

		// Clone the base message type so we have a newly initialized message as the decoding
		// destination.
		msg := proto.Clone(base)
		if err := r.p2p.Encoding().Decode(stream, msg); err != nil {
			log.WithError(err).Error("Failed to decode stream message")
			return
		}
		if err := handle(ctx, msg, stream); err != nil {
			// TODO(3147): Update metrics
			log.WithError(err).Error("Failed to handle p2p RPC")
		}
	})
}
