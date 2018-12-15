package ssz

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"reflect"
)

// TODO(1068): Support more data types

const lengthBytes = 4

// Encode encodes val and output the result into w.
func Encode(w io.Writer, val interface{}) error {
	eb := &encbuf{}
	if err := eb.encode(val); err != nil {
		return err
	}
	return eb.toWriter(w)
}

// EncodeSize returns the target encoding size without doing the actual encoding.
// This is an optional pass. You don't need to call this before the encoding unless you
// want to know the output size first.
func EncodeSize(val interface{}) (uint32, error) {
	return encodeSize(val)
}

type encbuf struct {
	str []byte
}

func (w *encbuf) encode(val interface{}) error {
	rval := reflect.ValueOf(val)
	encDec, err := cachedEncoderDecoder(rval.Type())
	if err != nil {
		return newEncodeError(fmt.Sprint(err), rval.Type())
	}
	if err = encDec.encoder(rval, w); err != nil {
		return newEncodeError(fmt.Sprint(err), rval.Type())
	}
	return nil
}

func encodeSize(val interface{}) (uint32, error) {
	rval := reflect.ValueOf(val)
	encDec, err := cachedEncoderDecoder(rval.Type())
	if err != nil {
		return 0, newEncodeError(fmt.Sprint(err), rval.Type())
	}
	var size uint32
	if size, err = encDec.encodeSizer(rval); err != nil {
		return 0, newEncodeError(fmt.Sprint(err), rval.Type())
	}
	return size, nil

}

func (w *encbuf) toWriter(out io.Writer) error {
	if _, err := out.Write(w.str); err != nil {
		return err
	}
	return nil
}

func makeEncoder(typ reflect.Type) (encoder, encodeSizer, error) {
	kind := typ.Kind()
	switch {
	case kind == reflect.Bool:
		return encodeBool, func(reflect.Value) (uint32, error) { return 1, nil }, nil
	case kind == reflect.Uint8:
		return encodeUint8, func(reflect.Value) (uint32, error) { return 1, nil }, nil
	case kind == reflect.Uint16:
		return encodeUint16, func(reflect.Value) (uint32, error) { return 2, nil }, nil
	case kind == reflect.Slice && typ.Elem().Kind() == reflect.Uint8:
		return makeBytesEncoder()
	case kind == reflect.Slice:
		return makeSliceEncoder(typ)
	case kind == reflect.Struct:
		return makeStructEncoder(typ)
	default:
		return nil, nil, fmt.Errorf("type %v is not serializable", typ)
	}
}

func encodeBool(val reflect.Value, w *encbuf) error {
	if val.Bool() {
		w.str = append(w.str, uint8(1))
	} else {
		w.str = append(w.str, uint8(0))
	}
	return nil
}

func encodeUint8(val reflect.Value, w *encbuf) error {
	v := val.Uint()
	w.str = append(w.str, uint8(v))
	return nil
}

func encodeUint16(val reflect.Value, w *encbuf) error {
	v := val.Uint()
	b := make([]byte, 2)
	binary.BigEndian.PutUint16(b, uint16(v))
	w.str = append(w.str, b...)
	return nil
}

func makeBytesEncoder() (encoder, encodeSizer, error) {
	encoder := func(val reflect.Value, w *encbuf) error {
		b := val.Bytes()
		sizeEnc := make([]byte, lengthBytes)
		if len(val.Bytes()) >= 2<<32 {
			return errors.New("bytes oversize")
		}
		binary.BigEndian.PutUint32(sizeEnc, uint32(len(b)))
		w.str = append(w.str, sizeEnc...)
		w.str = append(w.str, val.Bytes()...)
		return nil
	}
	encodeSizer := func(val reflect.Value) (uint32, error) {
		if len(val.Bytes()) >= 2<<32 {
			return 0, errors.New("bytes oversize")
		}
		return uint32(len(val.Bytes())), nil
	}
	return encoder, encodeSizer, nil
}

func makeSliceEncoder(typ reflect.Type) (encoder, encodeSizer, error) {
	elemEncoderDecoder, err := cachedEncoderDecoderNoAcquireLock(typ.Elem())
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get encoder/decoder: %v", err)
	}
	encoder := func(val reflect.Value, w *encbuf) error {
		origBufSize := len(w.str)
		totalSizeEnc := make([]byte, lengthBytes)
		w.str = append(w.str, totalSizeEnc...)
		for i := 0; i < val.Len(); i++ {
			if err := elemEncoderDecoder.encoder(val.Index(i), w); err != nil {
				return fmt.Errorf("failed to encode element of slice: %v", err)
			}
		}
		totalSize := len(w.str) - lengthBytes - origBufSize
		if totalSize >= 2<<32 {
			return errors.New("slice oversize")
		}
		binary.BigEndian.PutUint32(totalSizeEnc, uint32(totalSize))
		copy(w.str[origBufSize:origBufSize+lengthBytes], totalSizeEnc)
		return nil
	}
	encodeSizer := func(val reflect.Value) (uint32, error) {
		if val.Len() == 0 {
			return lengthBytes, nil
		}
		elemSize, err := elemEncoderDecoder.encodeSizer(val.Index(0))
		if err != nil {
			return 0, errors.New("failed to get encode size of element of slice")
		}
		return lengthBytes + elemSize*uint32(val.Len()), nil
	}
	return encoder, encodeSizer, nil
}

func makeStructEncoder(typ reflect.Type) (encoder, encodeSizer, error) {
	fields, err := sortedStructFields(typ)
	if err != nil {
		return nil, nil, err
	}
	encoder := func(val reflect.Value, w *encbuf) error {
		origBufSize := len(w.str)
		totalSizeEnc := make([]byte, lengthBytes)
		w.str = append(w.str, totalSizeEnc...)
		for _, f := range fields {
			if err := f.encDec.encoder(val.Field(f.index), w); err != nil {
				return fmt.Errorf("failed to encode field of struct: %v", err)
			}
		}
		totalSize := len(w.str) - lengthBytes - origBufSize
		if totalSize >= 2<<32 {
			return errors.New("struct oversize")
		}
		binary.BigEndian.PutUint32(totalSizeEnc, uint32(totalSize))
		copy(w.str[origBufSize:origBufSize+lengthBytes], totalSizeEnc)
		return nil
	}
	encodeSizer := func(val reflect.Value) (uint32, error) {
		totalSize := uint32(0)
		for _, f := range fields {
			fieldSize, err := f.encDec.encodeSizer(val.Field(f.index))
			if err != nil {
				return 0, fmt.Errorf("failed to get encode size for field of struct: %v", err)
			}
			totalSize += fieldSize
		}
		return lengthBytes + totalSize, nil
	}
	return encoder, encodeSizer, nil
}

// encodeError is what gets reported to the encoder user in error case.
type encodeError struct {
	msg string
	typ reflect.Type
}

func newEncodeError(msg string, typ reflect.Type) *encodeError {
	return &encodeError{msg, typ}
}

func (err *encodeError) Error() string {
	return fmt.Sprintf("encode error: %s for input type %v", err.msg, err.typ)
}
