package ssz

import (
	"bytes"
	"io"
	"reflect"
	"testing"
)

type exampleStruct1 struct {
	Field1 uint8
	Field2 []byte
}

func (e *exampleStruct1) EncodeSSZ(w io.Writer) error {
	// Need to pass value of struct for Encode function
	// Later we can enhance the ssz implementation to support passing pointer, if necessary
	return Encode(w, *e)
}

func (e *exampleStruct1) EncodeSSZSize() (uint32, error) {
	return EncodeSize(*e)
}

func (e *exampleStruct1) DecodeSSZ(r io.Reader) error {
	// Need to pass pointer of struct for Decode function
	return Decode(r, e)
}

func (e *exampleStruct1) TreeHashSSZ() ([32]byte, error) {
	return TreeHash(e)
}

type exampleStruct2 struct {
	Field1 uint8 // a volatile, or host-specific field that doesn't need to be exported
	Field2 []byte
}

// You can use a helper struct to only encode/decode custom fields of your struct
type exampleStruct2Export struct {
	Field2 []byte
}

func (e *exampleStruct2) EncodeSSZ(w io.Writer) error {
	return Encode(w, exampleStruct2Export{
		e.Field2,
	})
}

func (e *exampleStruct2) EncodeSSZSize() (uint32, error) {
	return EncodeSize(exampleStruct2Export{
		e.Field2,
	})
}

func (e *exampleStruct2) DecodeSSZ(r io.Reader) error {
	ee := new(exampleStruct2Export)
	if err := Decode(r, ee); err != nil {
		return err
	}
	e.Field2 = ee.Field2
	return nil
}

func (e *exampleStruct2) TreeHashSSZ() ([32]byte, error) {
	return TreeHash(exampleStruct2Export{
		e.Field2,
	})
}

func TestEncodeDecode_Struct1(t *testing.T) {
	var err error
	e1 := &exampleStruct1{
		Field1: 10,
		Field2: []byte{1, 2, 3, 4},
	}
	wBuf := new(bytes.Buffer)
	if err = e1.EncodeSSZ(wBuf); err != nil {
		t.Fatalf("failed to encode: %v", err)
	}
	encoding := wBuf.Bytes()

	e2 := new(exampleStruct1)
	rBuf := bytes.NewReader(encoding)
	if err = e2.DecodeSSZ(rBuf); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	if !reflect.DeepEqual(*e1, *e2) {
		t.Error("encode/decode algorithm don't match")
	}

	encodeSize := uint32(0)
	if encodeSize, err = e1.EncodeSSZSize(); err != nil {
		t.Errorf("failed to get encode size: %v", err)
	}
	if encodeSize != 13 {
		t.Error("wrong encode size calculation result")
	}

	hash, err := e1.TreeHashSSZ()
	if err != nil {
		t.Fatalf("failed to hash: %v", err)
	}
	if !bytes.Equal(hash[:], unhex("de89580c8ae5db17252b9be2335aecb3af8c75b443d024e9255f5cde7a770d57")) {
		t.Errorf("wrong hash result, got %#x", hash)
	}
}

func TestEncodeDecode_Struct2(t *testing.T) {
	var err error
	e1 := &exampleStruct2{
		Field1: 10,
		Field2: []byte{1, 2, 3, 4},
	}
	wBuf := new(bytes.Buffer)
	if err = e1.EncodeSSZ(wBuf); err != nil {
		t.Fatalf("failed to encode: %v", err)
	}
	encoding := wBuf.Bytes()

	e2 := new(exampleStruct2)
	rBuf := bytes.NewReader(encoding)
	if err = e2.DecodeSSZ(rBuf); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	if !reflect.DeepEqual(e1.Field2, e2.Field2) {
		t.Error("encode/decode algorithm don't match")
	}

	encodeSize := uint32(0)
	if encodeSize, err = e1.EncodeSSZSize(); err != nil {
		t.Errorf("failed to get encode size: %v", err)
	}
	if encodeSize != 12 {
		t.Error("wrong encode size calculation result")
	}

	hash, err := e1.TreeHashSSZ()
	if err != nil {
		t.Fatalf("failed to hash: %v", err)
	}
	if !bytes.Equal(hash[:], unhex("7056ebc39d3c9c99dff1d158cc965522ea504c015fad102c29dd88e6fcaee5d6")) {
		t.Errorf("wrong hash result, got %#x", hash)
	}
}
