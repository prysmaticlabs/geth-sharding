package sync

import (
	"context"

	libp2pcore "github.com/libp2p/go-libp2p-core"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/pkg/errors"
	"github.com/prysmaticlabs/prysm/beacon-chain/blockchain"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	"github.com/prysmaticlabs/prysm/beacon-chain/p2p"
	"github.com/prysmaticlabs/prysm/beacon-chain/p2p/types"
	p2p2 "github.com/prysmaticlabs/prysm/proto/beacon/p2p"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/prysmaticlabs/prysm/shared/p2putils"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/version"
)

// metaDataHandler reads the incoming metadata rpc request from the peer.
func (s *Service) metaDataHandler(_ context.Context, _ interface{}, stream libp2pcore.Stream) error {
	SetRPCStreamDeadlines(stream)

	if err := s.rateLimiter.validateRequest(stream, 1); err != nil {
		return err
	}
	s.rateLimiter.add(stream, 1)

	if s.cfg.P2P.Metadata() == nil || s.cfg.P2P.Metadata().IsNil() {
		nilErr := errors.New("nil metadata stored for host")
		resp, err := s.generateErrorResponse(responseCodeServerError, types.ErrGeneric.Error())
		if err != nil {
			log.WithError(err).Debug("Could not generate a response error")
		} else if _, err := stream.Write(resp); err != nil {
			log.WithError(err).Debug("Could not write to stream")
		}
		return nilErr
	}
	topicVersion := ""
	switch s.cfg.P2P.Metadata().Version() {
	case version.Phase0:
		topicVersion = p2p.SchemaVersionV1
	case version.Altair:
		topicVersion = p2p.SchemaVersionV2
	}
	if err := validateVersion(topicVersion, stream); err != nil {
		resp, genErr := s.generateErrorResponse(responseCodeServerError, types.ErrGeneric.Error())
		if genErr != nil {
			log.WithError(genErr).Debug("Could not generate a response error")
		} else if _, wErr := stream.Write(resp); wErr != nil {
			log.WithError(wErr).Debug("Could not write to stream")
		}
		return err
	}
	if _, err := stream.Write([]byte{responseCodeSuccess}); err != nil {
		return err
	}
	_, err := s.cfg.P2P.Encoding().EncodeWithMaxLength(stream, s.cfg.P2P.Metadata())
	if err != nil {
		return err
	}
	closeStream(stream, log)
	return nil
}

func (s *Service) sendMetaDataRequest(ctx context.Context, id peer.ID) (p2p2.Metadata, error) {
	ctx, cancel := context.WithTimeout(ctx, respTimeout)
	defer cancel()

	topic, err := p2p.TopicFromMessage(p2p.MetadataMessageName, helpers.SlotToEpoch(s.cfg.Chain.CurrentSlot()))
	if err != nil {
		return nil, err
	}
	stream, err := s.cfg.P2P.Send(ctx, new(interface{}), topic, id)
	if err != nil {
		return nil, err
	}
	defer closeStream(stream, log)
	code, errMsg, err := ReadStatusCode(stream, s.cfg.P2P.Encoding())
	if err != nil {
		return nil, err
	}
	if code != 0 {
		s.cfg.P2P.Peers().Scorers().BadResponsesScorer().Increment(stream.Conn().RemotePeer())
		return nil, errors.New(errMsg)
	}
	valRoot := s.cfg.Chain.GenesisValidatorRoot()
	rpcCtx, err := p2putils.ForkDigestFromEpoch(helpers.SlotToEpoch(s.cfg.Chain.CurrentSlot()), valRoot[:])
	if err != nil {
		return nil, err
	}
	msg, err := extractMetaDataType(rpcCtx[:], s.cfg.Chain)
	if err != nil {
		return nil, err
	}
	// Defensive check to ensure valid objects are being sent.
	topicVersion := ""
	switch msg.Version() {
	case version.Phase0:
		topicVersion = p2p.SchemaVersionV1
	case version.Altair:
		topicVersion = p2p.SchemaVersionV2
	}
	if err := validateVersion(topicVersion, stream); err != nil {
		return nil, err
	}
	if err := s.cfg.P2P.Encoding().DecodeWithMaxLength(stream, msg); err != nil {
		return nil, err
	}
	return msg, nil
}

func extractMetaDataType(digest []byte, chain blockchain.ChainInfoFetcher) (p2p2.Metadata, error) {
	if len(digest) == 0 {
		mdFunc, ok := types.MetaDataMap[bytesutil.ToBytes4(params.BeaconConfig().GenesisForkVersion)]
		if !ok {
			return nil, errors.New("no metadata type exists for the genesis fork version.")
		}
		return mdFunc(), nil
	}
	if len(digest) != digestLength {
		return nil, errors.Errorf("invalid digest returned, wanted a length of %d but received %d", digestLength, len(digest))
	}
	vRoot := chain.GenesisValidatorRoot()
	for k, mdFunc := range types.MetaDataMap {
		rDigest, err := helpers.ComputeForkDigest(k[:], vRoot[:])
		if err != nil {
			return nil, err
		}
		if rDigest == bytesutil.ToBytes4(digest) {
			return mdFunc(), nil
		}
	}
	return nil, errors.New("no valid digest matched")
}
