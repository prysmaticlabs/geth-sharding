package sync

import (
	"bytes"
	"errors"
	"io"

	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"

	"github.com/prysmaticlabs/prysm/beacon-chain/p2p/encoder"
)

const genericError = "internal service error"
const rateLimitedError = "rate limited"
const stepError = "invalid range or step"

var errWrongForkDigestVersion = errors.New("wrong fork digest version")
var errInvalidEpoch = errors.New("invalid epoch")
var errInvalidFinalizedRoot = errors.New("invalid finalized root")
var errGeneric = errors.New(genericError)

var responseCodeSuccess = byte(0x00)
var responseCodeInvalidRequest = byte(0x01)
var responseCodeServerError = byte(0x02)

func (r *Service) generateErrorResponse(code byte, reason string) ([]byte, error) {
	buf := bytes.NewBuffer([]byte{code})
	resp := &pb.ErrorResponse{
		Message: []byte(reason),
	}
	if _, err := r.p2p.Encoding().EncodeWithLength(buf, resp); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// ReadStatusCode response from a RPC stream.
func ReadStatusCode(stream io.Reader, encoding encoder.NetworkEncoding) (uint8, string, error) {
	b := make([]byte, 1)
	_, err := stream.Read(b)
	if err != nil {
		return 0, "", err
	}

	if b[0] == responseCodeSuccess {
		return 0, "", nil
	}

	msg := &pb.ErrorResponse{
		Message: []byte{},
	}
	if err := encoding.DecodeWithLength(stream, msg); err != nil {
		return 0, "", err
	}

	return b[0], string(msg.Message), nil
}
