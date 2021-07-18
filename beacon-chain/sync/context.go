package sync

import (
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/pkg/errors"
	"github.com/prysmaticlabs/prysm/beacon-chain/blockchain"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	"github.com/prysmaticlabs/prysm/beacon-chain/p2p"
)

// Specifies the fixed size context length.
const digestLength = 4

// writes peer's current context for the expected payload to the stream.
func writeContextToStream(objCtx []byte, stream network.Stream, chain blockchain.ChainInfoFetcher) error {
	rpcCtx, err := rpcContext(stream, chain)
	if err != nil {
		return err
	}
	// Exit early if there is an empty context.
	if len(rpcCtx) == 0 {
		return nil
	}
	// Always choose the object's context when writing to the stream.
	if objCtx != nil {
		rpcCtx = objCtx
	}
	_, err = stream.Write(rpcCtx)
	return err
}

// reads any attached context-bytes to the payload.
func readContextFromStream(stream network.Stream, chain blockchain.ChainInfoFetcher) ([]byte, error) {
	rpcCtx, err := rpcContext(stream, chain)
	if err != nil {
		return nil, err
	}
	if len(rpcCtx) == 0 {
		return []byte{}, nil
	}
	// Read context (fork-digest) from stream
	b := make([]byte, digestLength)
	if _, err := stream.Read(b); err != nil {
		return nil, err
	}
	return b, nil
}

// retrieve expected context depending on rpc topic schema version.
func rpcContext(stream network.Stream, chain blockchain.ChainInfoFetcher) ([]byte, error) {
	_, _, version, err := p2p.TopicDeconstructor(string(stream.Protocol()))
	if err != nil {
		return nil, err
	}
	switch version {
	case p2p.SchemaVersionV1:
		// Return empty context for a v1 method.
		return []byte{}, nil
	case p2p.SchemaVersionV2:
		currFork := chain.CurrentFork()
		genVersion := chain.GenesisValidatorRoot()
		digest, err := helpers.ComputeForkDigest(currFork.CurrentVersion, genVersion[:])
		if err != nil {
			return nil, err
		}
		return digest[:], nil
	default:
		return nil, errors.New("invalid version of %s registered for topic: %s")
	}
}

// Validates that the rpc topic matches the provided version.
func validateVersion(version string, stream network.Stream) error {
	_, _, streamVersion, err := p2p.TopicDeconstructor(string(stream.Protocol()))
	if err != nil {
		return err
	}
	if streamVersion != version {
		return errors.Errorf("stream version of %s doesn't match provided version %s", streamVersion, version)
	}
	return nil
}
