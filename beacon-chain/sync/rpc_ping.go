package sync

import (
	"context"
	"errors"
	"fmt"
	"strings"

	libp2pcore "github.com/libp2p/go-libp2p-core"
	"github.com/libp2p/go-libp2p-core/peer"
	types "github.com/prysmaticlabs/eth2-types"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	"github.com/prysmaticlabs/prysm/beacon-chain/p2p"
	p2ptypes "github.com/prysmaticlabs/prysm/beacon-chain/p2p/types"
	"github.com/prysmaticlabs/prysm/shared/timeutils"
)

// pingHandler reads the incoming ping rpc message from the peer.
func (s *Service) pingHandler(_ context.Context, msg interface{}, stream libp2pcore.Stream) error {
	SetRPCStreamDeadlines(stream)

	m, ok := msg.(*types.SSZUint64)
	if !ok {
		return fmt.Errorf("wrong message type for ping, got %T, wanted *uint64", msg)
	}
	if err := s.rateLimiter.validateRequest(stream, 1); err != nil {
		return err
	}
	s.rateLimiter.add(stream, 1)
	valid, err := s.validateSequenceNum(*m, stream.Conn().RemotePeer())
	if err != nil {
		// Descore peer for giving us a bad sequence number.
		if errors.Is(err, p2ptypes.ErrInvalidSequenceNum) {
			s.cfg.P2P.Peers().Scorers().BadResponsesScorer().Increment(stream.Conn().RemotePeer())
			s.writeErrorResponseToStream(responseCodeInvalidRequest, p2ptypes.ErrInvalidSequenceNum.Error(), stream)
		}
		return err
	}
	if _, err := stream.Write([]byte{responseCodeSuccess}); err != nil {
		return err
	}
	sq := types.SSZUint64(s.cfg.P2P.MetadataSeq())
	if _, err := s.cfg.P2P.Encoding().EncodeWithMaxLength(stream, &sq); err != nil {
		return err
	}

	closeStream(stream, log)

	if valid {
		// If the sequence number was valid we're done.
		return nil
	}

	// The sequence number was not valid.  Start our own ping back to the peer.
	go func() {
		// New context so the calling function doesn't cancel on us.
		ctx, cancel := context.WithTimeout(context.Background(), ttfbTimeout)
		defer cancel()
		md, err := s.sendMetaDataRequest(ctx, stream.Conn().RemotePeer())
		if err != nil {
			// We cannot compare errors directly as the stream muxer error
			// type isn't compatible with the error we have, so a direct
			// equality checks fails.
			if !strings.Contains(err.Error(), p2ptypes.ErrIODeadline.Error()) {
				log.WithField("peer", stream.Conn().RemotePeer()).WithError(err).Debug("Could not send metadata request")
			}
			return
		}
		// update metadata if there is no error
		s.cfg.P2P.Peers().SetMetadata(stream.Conn().RemotePeer(), md)
	}()

	return nil
}

func (s *Service) sendPingRequest(ctx context.Context, id peer.ID) error {
	ctx, cancel := context.WithTimeout(ctx, respTimeout)
	defer cancel()

	metadataSeq := types.SSZUint64(s.cfg.P2P.MetadataSeq())
	topic, err := p2p.TopicFromMessage(p2p.PingMessageName, helpers.SlotToEpoch(s.cfg.Chain.CurrentSlot()))
	if err != nil {
		return err
	}
	stream, err := s.cfg.P2P.Send(ctx, &metadataSeq, topic, id)
	if err != nil {
		return err
	}
	currentTime := timeutils.Now()
	defer closeStream(stream, log)

	code, errMsg, err := ReadStatusCode(stream, s.cfg.P2P.Encoding())
	if err != nil {
		return err
	}
	// Records the latency of the ping request for that peer.
	s.cfg.P2P.Host().Peerstore().RecordLatency(id, timeutils.Now().Sub(currentTime))

	if code != 0 {
		s.cfg.P2P.Peers().Scorers().BadResponsesScorer().Increment(stream.Conn().RemotePeer())
		return errors.New(errMsg)
	}
	msg := new(types.SSZUint64)
	if err := s.cfg.P2P.Encoding().DecodeWithMaxLength(stream, msg); err != nil {
		return err
	}
	valid, err := s.validateSequenceNum(*msg, stream.Conn().RemotePeer())
	if err != nil {
		// Descore peer for giving us a bad sequence number.
		if errors.Is(err, p2ptypes.ErrInvalidSequenceNum) {
			s.cfg.P2P.Peers().Scorers().BadResponsesScorer().Increment(stream.Conn().RemotePeer())
		}
		return err
	}
	if valid {
		return nil
	}
	md, err := s.sendMetaDataRequest(ctx, stream.Conn().RemotePeer())
	if err != nil {
		// do not increment bad responses, as its
		// already done in the request method.
		return err
	}
	s.cfg.P2P.Peers().SetMetadata(stream.Conn().RemotePeer(), md)
	return nil
}

// validates the peer's sequence number.
func (s *Service) validateSequenceNum(seq types.SSZUint64, id peer.ID) (bool, error) {
	md, err := s.cfg.P2P.Peers().Metadata(id)
	if err != nil {
		return false, err
	}
	if md == nil || md.IsNil() {
		return false, nil
	}
	// Return error on invalid sequence number.
	if md.SequenceNumber() > uint64(seq) {
		return false, p2ptypes.ErrInvalidSequenceNum
	}
	return md.SequenceNumber() == uint64(seq), nil
}
