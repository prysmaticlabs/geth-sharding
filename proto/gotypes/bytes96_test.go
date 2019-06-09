package gotypes_test

import (
	"bytes"
	"testing"

	"github.com/gogo/protobuf/proto"

	"github.com/prysmaticlabs/prysm/proto/gotypes"
)

//var _ = proto.Merger(gotypes.Bytes96{})
var _ = proto.Marshaler(gotypes.Bytes96{})

//var _ = proto.Unmarshaler(gotypes.Bytes96{})
var _ = proto.Sizer(gotypes.Bytes96{})

func TestBytes96(t *testing.T) {
	input := make([]byte, 96)
	copy(input, []byte("Foobar!"))

	b := gotypes.Bytes96{}
	if err := b.Unmarshal(input); err != nil {
		t.Fatalf("Failed to unmarshal input. err = %v", err)
	}

	output := make([]byte, 96)
	n, err := b.MarshalTo(output)
	if n != 96 {
		t.Errorf("Unexpected n. Wanted 96, got %d", n)
	}
	if err != nil {
		t.Errorf("Unexpected error = %v", err)
	}
	if !bytes.Equal(input, output) {
		t.Errorf("Input != output bytes. Input=%v. Output=%v", input, output)
	}

	output, err = b.Marshal()
	if err != nil {
		t.Errorf("Unexpected error = %v", err)
	}
	if !bytes.Equal(input, output) {
		t.Errorf("Input != output bytes. Input=%v. Output=%v", input, output)
	}
}
