package bytesutil_test

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/prysmaticlabs/prysm/shared/bytesutil"
)

func TestToBytes(t *testing.T) {
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
		{0, []byte{0, 0}},
		{1, []byte{1, 0}},
		{255, []byte{255, 0}},
		{256, []byte{0, 1}},
		{65534, []byte{254, 255}},
		{65535, []byte{255, 255}},
		{0, []byte{0, 0, 0}},
		{255, []byte{255, 0, 0}},
		{256, []byte{0, 1, 0}},
		{65535, []byte{255, 255, 0}},
		{65536, []byte{0, 0, 1}},
		{16777215, []byte{255, 255, 255}},
		{0, []byte{0, 0, 0, 0}},
		{256, []byte{0, 1, 0, 0}},
		{65536, []byte{0, 0, 1, 0}},
		{16777216, []byte{0, 0, 0, 1}},
		{16777217, []byte{1, 0, 0, 1}},
		{4294967295, []byte{255, 255, 255, 255}},
		{0, []byte{0, 0, 0, 0, 0, 0, 0, 0}},
		{16777216, []byte{0, 0, 0, 1, 0, 0, 0, 0}},
		{4294967296, []byte{0, 0, 0, 0, 1, 0, 0, 0}},
		{4294967297, []byte{1, 0, 0, 0, 1, 0, 0, 0}},
		{9223372036854775806, []byte{254, 255, 255, 255, 255, 255, 255, 127}},
		{9223372036854775807, []byte{255, 255, 255, 255, 255, 255, 255, 127}},
	}
	for _, tt := range tests {
		b := bytesutil.ToBytes(tt.a, len(tt.b))
		if !bytes.Equal(b, tt.b) {
			t.Errorf("Bytes1(%d) = %v, want = %d", tt.a, b, tt.b)
		}
	}
}

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
		b := bytesutil.Bytes1(tt.a)
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
		{1, []byte{1, 0}},
		{255, []byte{255, 0}},
		{256, []byte{0, 1}},
		{65534, []byte{254, 255}},
		{65535, []byte{255, 255}},
	}
	for _, tt := range tests {
		b := bytesutil.Bytes2(tt.a)
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
		{255, []byte{255, 0, 0}},
		{256, []byte{0, 1, 0}},
		{65535, []byte{255, 255, 0}},
		{65536, []byte{0, 0, 1}},
		{16777215, []byte{255, 255, 255}},
	}
	for _, tt := range tests {
		b := bytesutil.Bytes3(tt.a)
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
		{256, []byte{0, 1, 0, 0}},
		{65536, []byte{0, 0, 1, 0}},
		{16777216, []byte{0, 0, 0, 1}},
		{16777217, []byte{1, 0, 0, 1}},
		{4294967295, []byte{255, 255, 255, 255}},
	}
	for _, tt := range tests {
		b := bytesutil.Bytes4(tt.a)
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
		{16777216, []byte{0, 0, 0, 1, 0, 0, 0, 0}},
		{4294967296, []byte{0, 0, 0, 0, 1, 0, 0, 0}},
		{4294967297, []byte{1, 0, 0, 0, 1, 0, 0, 0}},
		{9223372036854775806, []byte{254, 255, 255, 255, 255, 255, 255, 127}},
		{9223372036854775807, []byte{255, 255, 255, 255, 255, 255, 255, 127}},
	}
	for _, tt := range tests {
		b := bytesutil.Bytes8(tt.a)
		if !bytes.Equal(b, tt.b) {
			t.Errorf("Bytes8(%d) = %v, want = %d", tt.a, b, tt.b)
		}
	}
}

func TestFromBool(t *testing.T) {
	tests := []byte{
		0,
		1,
	}
	for _, tt := range tests {
		b := bytesutil.ToBool(tt)
		c := bytesutil.FromBool(b)
		if c != tt {
			t.Errorf("Wanted %d but got %d", tt, c)
		}
	}
}

func TestFromBytes2(t *testing.T) {
	tests := []uint64{
		0,
		1776,
		96726,
		(1 << 16) - 1,
	}
	for _, tt := range tests {
		b := bytesutil.ToBytes(tt, 2)
		c := bytesutil.FromBytes2(b)
		if c != uint16(tt) {
			t.Errorf("Wanted %d but got %d", tt, c)
		}
	}
}

func TestFromBytes4(t *testing.T) {
	tests := []uint64{
		0,
		1776,
		96726,
		4290997,
		4294967295, //2^32 - 1
		4294967200,
		3894948296,
	}
	for _, tt := range tests {
		b := bytesutil.ToBytes(tt, 4)
		c := bytesutil.FromBytes4(b)
		if c != tt {
			t.Errorf("Wanted %d but got %d", tt, c)
		}
	}
}

func TestFromBytes8(t *testing.T) {
	tests := []uint64{
		0,
		1776,
		96726,
		4290997,
		922376854775806,
		42893720984775807,
		18446744073709551615,
	}
	for _, tt := range tests {
		b := bytesutil.ToBytes(tt, 8)
		c := bytesutil.FromBytes8(b)
		if c != tt {
			t.Errorf("Wanted %d but got %d", tt, c)
		}
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		a []byte
		b []byte
	}{
		{[]byte{'A', 'C', 'D', 'E', 'F', 'G', 'H', 'I', 'J', 'K', 'L', 'M', 'N', 'O'},
			[]byte{'A', 'C', 'D', 'E', 'F', 'G'}},
		{[]byte{'A', 'C', 'D', 'E', 'F'},
			[]byte{'A', 'C', 'D', 'E', 'F'}},
		{[]byte{}, []byte{}},
	}
	for _, tt := range tests {
		b := bytesutil.Trunc(tt.a)
		if !bytes.Equal(b, tt.b) {
			t.Errorf("Trunc(%d) = %v, want = %d", tt.a, b, tt.b)
		}
	}
}

func TestReverse(t *testing.T) {
	tests := []struct {
		input  [][32]byte
		output [][32]byte
	}{
		{[][32]byte{{'A'}, {'B'}, {'C'}, {'D'}, {'E'}, {'F'}, {'G'}, {'H'}},
			[][32]byte{{'H'}, {'G'}, {'F'}, {'E'}, {'D'}, {'C'}, {'B'}, {'A'}}},
		{[][32]byte{{1}, {2}, {3}, {4}},
			[][32]byte{{4}, {3}, {2}, {1}}},
		{[][32]byte{}, [][32]byte{}},
	}
	for _, tt := range tests {
		b := bytesutil.ReverseBytes32Slice(tt.input)
		if !reflect.DeepEqual(b, tt.output) {
			t.Errorf("Reverse(%d) = %v, want = %d", tt.input, b, tt.output)
		}
	}
}

func TestSetBit(t *testing.T) {
	tests := []struct {
		a []byte
		b int
		c []byte
	}{
		{[]byte{0b00000000}, 1, []byte{0b00000010}},
		{[]byte{0b00000010}, 7, []byte{0b10000010}},
		{[]byte{0b10000010}, 9, []byte{0b10000010, 0b00000010}},
		{[]byte{0b10000010}, 27, []byte{0b10000010, 0b00000000, 0b00000000, 0b00001000}},
		{[]byte{0b10000010, 0b00000000}, 8, []byte{0b10000010, 0b00000001}},
		{[]byte{0b10000010, 0b00000000}, 31, []byte{0b10000010, 0b00000000, 0b00000000, 0b10000000}},
	}
	for _, tt := range tests {
		if !bytes.Equal(bytesutil.SetBit(tt.a, tt.b), tt.c) {
			t.Errorf("Setbit(%d) = %v, want = %d", tt.b, tt.c, bytesutil.SetBit(tt.a, tt.b))
		}
	}
}

func TestClearBit(t *testing.T) {
	tests := []struct {
		a []byte
		b int
		c []byte
	}{
		{[]byte{0b00000000}, 1, []byte{0b00000000}},
		{[]byte{0b00000010}, 1, []byte{0b00000000}},
		{[]byte{0b10000010}, 1, []byte{0b10000000}},
		{[]byte{0b10000010}, 8, []byte{0b10000010}},
		{[]byte{0b10000010, 0b00001111}, 7, []byte{0b00000010, 0b00001111}},
		{[]byte{0b10000010, 0b00001111}, 10, []byte{0b10000010, 0b00001011}},
	}
	for _, tt := range tests {
		if !bytes.Equal(bytesutil.ClearBit(tt.a, tt.b), tt.c) {
			t.Errorf("ClearBit(%d) = %v, want = %d", tt.b, tt.c, bytesutil.ClearBit(tt.a, tt.b))
		}
	}
}

func TestMakeEmptyBitfields(t *testing.T) {
	tests := []struct {
		a int
		b int
	}{
		{0, 1},
		{1, 1},
		{2, 1},
		{7, 1},
		{8, 2},
		{15, 2},
		{16, 3},
		{100, 13},
		{104, 14},
	}
	for _, tt := range tests {
		if len(bytesutil.MakeEmptyBitlists(tt.a)) != tt.b {
			t.Errorf("MakeEmptyBitlists(%d) = %v, want = %d", tt.a, len(bytesutil.MakeEmptyBitlists(tt.a)), tt.b)
		}
	}
}

func TestHighestBitIndex(t *testing.T) {
	tests := []struct {
		a     []byte
		b     int
		error bool
	}{
		{nil, 0, true},
		{[]byte{}, 0, true},
		{[]byte{0b00000001}, 1, false},
		{[]byte{0b10100101}, 8, false},
		{[]byte{0x00, 0x00}, 0, false},
		{[]byte{0xff, 0xa0}, 16, false},
		{[]byte{12, 34, 56, 78}, 31, false},
		{[]byte{255, 255, 255, 255}, 32, false},
	}
	for _, tt := range tests {
		i, err := bytesutil.HighestBitIndex(tt.a)
		if !tt.error {
			if err != nil {
				t.Fatal(err)
			}
			if i != tt.b {
				t.Errorf("HighestBitIndex(%d) = %v, want = %d", tt.a, i, tt.b)
			}
		} else {
			if err.Error() != "input list can't be empty or nil" {
				t.Error("Did not get wanted error")
			}
		}
	}
}

func TestHighestBitIndexBelow(t *testing.T) {
	tests := []struct {
		a     []byte
		b     int
		c     int
		error bool
	}{
		{nil, 0, 0, true},
		{[]byte{}, 0, 0, true},
		{[]byte{0b00010001}, 0, 0, false},
		{[]byte{0b00010001}, 1, 1, false},
		{[]byte{0b00010001}, 2, 1, false},
		{[]byte{0b00010001}, 4, 1, false},
		{[]byte{0b00010001}, 5, 5, false},
		{[]byte{0b00010001}, 8, 5, false},
		{[]byte{0b00010001, 0b00000000}, 0, 0, false},
		{[]byte{0b00010001, 0b00000000}, 1, 1, false},
		{[]byte{0b00010001, 0b00000000}, 2, 1, false},
		{[]byte{0b00010001, 0b00000000}, 4, 1, false},
		{[]byte{0b00010001, 0b00000000}, 5, 5, false},
		{[]byte{0b00010001, 0b00000000}, 8, 5, false},
		{[]byte{0b00010001, 0b00000000}, 15, 5, false},
		{[]byte{0b00010001, 0b00000000}, 16, 5, false},
		{[]byte{0b00010001, 0b00100010}, 8, 5, false},
		{[]byte{0b00010001, 0b00100010}, 9, 5, false},
		{[]byte{0b00010001, 0b00100010}, 10, 10, false},
		{[]byte{0b00010001, 0b00100010}, 11, 10, false},
		{[]byte{0b00010001, 0b00100010}, 14, 14, false},
		{[]byte{0b00010001, 0b00100010}, 15, 14, false},
		{[]byte{0b00010001, 0b00100010}, 24, 14, false},
		{[]byte{0b00010001, 0b00100010, 0b10000000}, 23, 14, false},
		{[]byte{0b00010001, 0b00100010, 0b10000000}, 24, 24, false},
		{[]byte{0b00000000, 0b00000001, 0b00000011}, 17, 17, false},
		{[]byte{0b00000000, 0b00000001, 0b00000011}, 18, 18, false},
		{[]byte{12, 34, 56, 78}, 1000, 31, false},
		{[]byte{255, 255, 255, 255}, 1000, 32, false},
	}
	for _, tt := range tests {
		i, err := bytesutil.HighestBitIndexAt(tt.a, tt.b)
		if !tt.error {
			if err != nil {
				t.Fatal(err)
			}
			if i != tt.c {
				t.Errorf("HighestBitIndexAt(%0.8b, %d) = %v, want = %d", tt.a, tt.b, i, tt.c)
			}
		} else {
			if err.Error() != "input list can't be empty or nil" {
				t.Error("Did not get wanted error")
			}
		}
	}
}
