package kv

import "bytes"

// In order for an encoding to be Altair compatible, it must be prefixed with altair key.
func hasAltairKey(enc []byte) bool {
	if len(altairKey) >= len(enc) {
		return false
	}
	return bytes.Equal(enc[:len(altairKey)], altairKey)
}
