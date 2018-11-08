// Package bytes defines helper methods for converting integers to byte slices.
package bytes

import (
	"encoding/binary"
)

// Bytes1 returns integer x to bytes in big-endian format, x.to_bytes(1, 'big').
func Bytes1(x uint64) []byte {
	bytes := make([]byte, 8)
	binary.BigEndian.PutUint64(bytes, x)
	return bytes[7:]
}

// Bytes2 returns integer x to bytes in big-endian format, x.to_bytes(2, 'big').
func Bytes2(x uint64) []byte {
	bytes := make([]byte, 8)
	binary.BigEndian.PutUint64(bytes, x)
	return bytes[6:]
}

// Bytes3 returns integer x to bytes in big-endian format, x.to_bytes(3, 'big').
func Bytes3(x uint64) []byte {
	bytes := make([]byte, 8)
	binary.BigEndian.PutUint64(bytes, x)
	return bytes[5:]
}

// Bytes4 returns integer x to bytes in big-endian format, x.to_bytes(4, 'big').
func Bytes4(x uint64) []byte {
	bytes := make([]byte, 8)
	binary.BigEndian.PutUint64(bytes, x)
	return bytes[4:]
}

// Bytes8 returns integer x to bytes in big-endian format, x.to_bytes(8, 'big').
func Bytes8(x uint64) []byte {
	bytes := make([]byte, 8)
	binary.BigEndian.PutUint64(bytes, x)
	return bytes
}
