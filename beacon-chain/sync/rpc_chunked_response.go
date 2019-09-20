package sync

import (
	"errors"
	"io"

	libp2pcore "github.com/libp2p/go-libp2p-core"
	"github.com/prysmaticlabs/prysm/beacon-chain/p2p"
	eth "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
)

// chunkWriter writes the given message as a chunked response to the given network
// stream.
// response_chunk ::= | <result> | <encoding-dependent-header> | <encoded-payload>
func (r *RegularSync) chunkWriter(stream libp2pcore.Stream, msg interface{}) error {
	setStreamWriteDeadline(stream, defaultWriteDuration)
	if _, err := stream.Write([]byte{responseCodeSuccess}); err != nil {
		return err
	}
	_, err := r.p2p.Encoding().EncodeWithMaxLength(stream, msg, maxChunkSize)
	return err
}

// HandleChunkedBlocks handles each response chunk that is sent by the
// peer and converts it into a beacon block.
func HandleChunkedBlock(stream libp2pcore.Stream, p2p p2p.P2P) (*eth.BeaconBlock, error) {
	blk := &eth.BeaconBlock{}
	if err := readResponseChunk(stream, p2p, blk); err != nil {
		return nil, err
	}
	return blk, nil
}

// readResponseChunk reads the response from the stream and decodes it into the
// provided message type.
func readResponseChunk(stream libp2pcore.Stream, p2p p2p.P2P, to interface{}) error {
	setStreamReadDeadline(stream, 10 /* seconds */)
	code, errMsg, err := ReadStatusCode(stream, p2p.Encoding())
	if err == io.EOF {
		return errors.New("reached the end of the stream")
	}
	if err != nil {
		return err
	}

	if code != 0 {
		return errors.New(errMsg)
	}
	return p2p.Encoding().DecodeWithMaxLength(stream, to, maxChunkSize)
}
