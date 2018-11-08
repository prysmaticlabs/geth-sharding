package bytes

import (
	"bytes"
	"testing"
)

func TestBytes1(t *testing.T) {
	tests := []struct {
		a uint64
		b []byte
	}{
		{0, []byte{0}},
		{1, []byte{1}},
		{2, []byte{2}},
		{253, []byte{253}},
		{254, []byte{254}},
		{255, []byte{255}},
	}
	for _, tt := range tests {
		b := Bytes1(tt.a)
		if !bytes.Equal(b, tt.b) {
			t.Errorf("Bytes1(%d) = %v, want = %d", tt.a, b, tt.b)
		}
	}
}

func TestBytes2(t *testing.T) {
	tests := []struct {
		a uint64
		b []byte
	}{
		{0, []byte{0, 0}},
		{1, []byte{0, 1}},
		{255, []byte{0, 255}},
		{256, []byte{1, 0}},
		{65534, []byte{255, 254}},
		{65535, []byte{255, 255}},
	}
	for _, tt := range tests {
		b := Bytes2(tt.a)
		if !bytes.Equal(b, tt.b) {
			t.Errorf("Bytes2(%d) = %v, want = %d", tt.a, b, tt.b)
		}
	}
}

func TestBytes3(t *testing.T) {
	tests := []struct {
		a uint64
		b []byte
	}{
		{0, []byte{0, 0, 0}},
		{255, []byte{0, 0, 255}},
		{256, []byte{0, 1, 0}},
		{65535, []byte{0, 255, 255}},
		{65536, []byte{1, 0, 0}},
		{16777215, []byte{255, 255, 255}},
	}
	for _, tt := range tests {
		b := Bytes3(tt.a)
		if !bytes.Equal(b, tt.b) {
			t.Errorf("Bytes3(%d) = %v, want = %d", tt.a, b, tt.b)
		}
	}
}

func TestBytes4(t *testing.T) {
	tests := []struct {
		a uint64
		b []byte
	}{
		{0, []byte{0, 0, 0, 0}},
		{256, []byte{0, 0, 1, 0}},
		{65536, []byte{0, 1, 0, 0}},
		{16777216, []byte{1, 0, 0, 0}},
		{16777217, []byte{1, 0, 0, 1}},
		{4294967295, []byte{255, 255, 255, 255}},
	}
	for _, tt := range tests {
		b := Bytes4(tt.a)
		if !bytes.Equal(b, tt.b) {
			t.Errorf("Bytes4(%d) = %v, want = %d", tt.a, b, tt.b)
		}
	}
}

func TestBytes8(t *testing.T) {
	tests := []struct {
		a uint64
		b []byte
	}{
		{0, []byte{0, 0, 0, 0, 0, 0, 0, 0}},
		{16777216, []byte{0, 0, 0, 0, 1, 0, 0, 0}},
		{4294967296, []byte{0, 0, 0, 1, 0, 0, 0, 0}},
		{4294967297, []byte{0, 0, 0, 1, 0, 0, 0, 1}},
		{9223372036854775806, []byte{127, 255, 255, 255, 255, 255, 255, 254}},
		{9223372036854775807, []byte{127, 255, 255, 255, 255, 255, 255, 255}},
	}
	for _, tt := range tests {
		b := Bytes8(tt.a)
		if !bytes.Equal(b, tt.b) {
			t.Errorf("Bytes8(%d) = %v, want = %d", tt.a, b, tt.b)
		}
	}
}
