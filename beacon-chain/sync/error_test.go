package sync

import (
	"bytes"
	"testing"

	p2ptest "github.com/prysmaticlabs/prysm/beacon-chain/p2p/testing"
)

func TestRegularSync_generateErrorResponse(t *testing.T) {
	r := &Service{
		p2p: p2ptest.NewTestP2P(t),
	}
	data, err := r.generateErrorResponse(responseCodeServerError, "something bad happened")
	if err != nil {
		t.Fatal(err)
	}

	buf := bytes.NewBuffer(data)
	b := make([]byte, 1)
	if _, err := buf.Read(b); err != nil {
		t.Fatal(err)
	}
	if b[0] != responseCodeServerError {
		t.Errorf("The first byte was not the status code. Got %#x wanted %#x", b, responseCodeServerError)
	}
	msg := make([]byte, 0)
	if err := r.p2p.Encoding().DecodeWithLength(buf, &msg); err != nil {
		t.Fatal(err)
	}
	if string(msg) != "something bad happened" {
		t.Errorf("Received the wrong message: %v", msg)
	}
}
