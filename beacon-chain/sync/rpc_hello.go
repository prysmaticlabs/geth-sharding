package sync

import (
	"bytes"
	"context"
	"time"

	"github.com/gogo/protobuf/proto"
	libp2pcore "github.com/libp2p/go-libp2p-core"
	"github.com/libp2p/go-libp2p-core/network"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	"github.com/prysmaticlabs/prysm/shared/params"
)

// registerRPC for a given topic with an expected protobuf message type.
func (r *RegularSync) sendRPCHelloRequest(ctx context.Context, topic string, stream network.Stream) error {
	topic += r.p2p.Encoding().ProtocolSuffix()
	setRPCStreamDeadlines(stream)

	ctx, cancel := context.WithTimeout(r.ctx, 10*time.Second)
	defer cancel()

	// return if hello already exists
	hello := r.helloTracker[stream.Conn().RemotePeer()]
	if hello != nil {
		return nil
	}

	r.helloTracker[stream.Conn().RemotePeer()] = nil

	resp := &pb.Hello{
		ForkVersion:    params.BeaconConfig().GenesisForkVersion,
		FinalizedRoot:  r.chain.FinalizedCheckpt().Root,
		FinalizedEpoch: r.chain.FinalizedCheckpt().Epoch,
		HeadRoot:       r.chain.HeadRoot(),
		HeadSlot:       r.chain.HeadSlot(),
	}

	if _, err := r.p2p.Encoding().Encode(stream, resp); err != nil {
		return err
	}
	// Close stream after finishing writing the request
	stream.Close()

	msg := &pb.Hello{}
	if err := r.p2p.Encoding().Decode(stream, msg); err != nil {
		return err
	}
	r.helloTracker[stream.Conn().RemotePeer()] = msg
	return r.validateHelloMessage(msg, stream)
}

// helloRPCHandler reads the incoming Hello RPC from the peer and responds with our version of a hello message.
// This handler will disconnect any peer that does not match our fork version.
func (r *RegularSync) helloRPCHandler(ctx context.Context, msg proto.Message, stream libp2pcore.Stream) error {
	defer stream.Close()
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	setRPCStreamDeadlines(stream)

	// return if hello already exists
	hello := r.helloTracker[stream.Conn().RemotePeer()]
	if hello != nil {
		return nil
	}

	r.helloTracker[stream.Conn().RemotePeer()] = nil

	log := log.WithField("rpc", "hello")
	m := msg.(*pb.Hello)

	r.helloTracker[stream.Conn().RemotePeer()] = m

	if err := r.validateHelloMessage(m, stream); err != nil {
		resp, err := r.generateErrorResponse(responseCodeInvalidRequest, errWrongForkVersion.Error())
		if err != nil {
			log.WithError(err).Error("Failed to generate a response error")
		} else {
			if _, err := stream.Write(resp); err != nil {
				log.WithError(err).Errorf("Failed to write to stream")
			}
		}
		stream.Close() // Close before disconnecting.
		// Add a short delay to allow the stream to flush before closing the connection.
		// There is still a chance that the peer won't receive the message.
		time.Sleep(50 * time.Millisecond)
		if err := r.p2p.Disconnect(stream.Conn().RemotePeer()); err != nil {
			log.WithError(err).Error("Failed to disconnect from peer")
		}
	}

	r.p2p.AddHandshake(stream.Conn().RemotePeer(), m)

	resp := &pb.Hello{
		ForkVersion:    params.BeaconConfig().GenesisForkVersion,
		FinalizedRoot:  r.chain.FinalizedCheckpt().Root,
		FinalizedEpoch: r.chain.FinalizedCheckpt().Epoch,
		HeadRoot:       r.chain.HeadRoot(),
		HeadSlot:       r.chain.HeadSlot(),
	}

	if _, err := stream.Write([]byte{responseCodeSuccess}); err != nil {
		log.WithError(err).Error("Failed to write to stream")
	}
	_, err := r.p2p.Encoding().Encode(stream, resp)

	return err
}

func (r *RegularSync) validateHelloMessage(msg *pb.Hello, stream network.Stream) error {
	if !bytes.Equal(params.BeaconConfig().GenesisForkVersion, msg.ForkVersion) {
		return errWrongForkVersion
	}
	return nil
}
