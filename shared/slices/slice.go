package slices

// Intersection of two uint32 slices with time
// complexity of approximately O(n) leveraging a map to
// check for element existence off by a constant factor
// of underlying map efficiency.
func Intersection(a []uint32, b []uint32) []uint32 {
	set := make([]uint32, 0)
	m := make(map[uint32]bool)

	for i := 0; i < len(a); i++ {
		m[a[i]] = true
	}
	for i := 0; i < len(b); i++ {
		if _, found := m[b[i]]; found {
			set = append(set, b[i])
		}
	}
	return set
}

// Union of two uint32 slices with time
// complexity of approximately O(n) leveraging a map to
// check for element existence off by a constant factor
// of underlying map efficiency.
func Union(a []uint32, b []uint32) []uint32 {
	set := make([]uint32, 0)
	m := make(map[uint32]bool)

	for i := 0; i < len(a); i++ {
		m[a[i]] = true
		set = append(set, a[i])
	}
	for i := 0; i < len(b); i++ {
		if _, found := m[b[i]]; !found {
			set = append(set, b[i])
		}
	}
	return set
}

// Not returns the uint32 in slice a that are
// not in slice b with time complexity of approximately
// O(n) leveraging a map to check for element existence
// off by a constant factor of underlying map efficiency.
func Not(a []uint32, b []uint32) []uint32 {
	set := make([]uint32, 0)
	m := make(map[uint32]bool)

	for i := 0; i < len(a); i++ {
		m[a[i]] = true
	}
	for i := 0; i < len(b); i++ {
		if _, found := m[b[i]]; !found {
			set = append(set, b[i])
		}
	}
	return set
}

// IsIn returns true if a is in b and False otherwise.
func IsIn(a uint32, b []uint32) bool {
	for _, v := range b {
		if a == v {
			return true
		}
	}
	return false
}

// IntersectionUint64 of two uint64 slices with time
// complexity of approximately O(n) leveraging a map to
// check for element existence off by a constant factor
// of underlying map efficiency.
func IntersectionUint64(a []uint64, b []uint64) []uint64 {
	set := make([]uint64, 0)
	m := make(map[uint64]bool)

	for i := 0; i < len(a); i++ {
		m[a[i]] = true
	}
	for i := 0; i < len(b); i++ {
		if _, found := m[b[i]]; found {
			set = append(set, b[i])
		}
	}
	return set
}

// UnionUint64 of two uint64 slices with time
// complexity of approximately O(n) leveraging a map to
// check for element existence off by a constant factor
// of underlying map efficiency.
func UnionUint64(a []uint64, b []uint64) []uint64 {
	set := make([]uint64, 0)
	m := make(map[uint64]bool)

	for i := 0; i < len(a); i++ {
		m[a[i]] = true
		set = append(set, a[i])
	}
	for i := 0; i < len(b); i++ {
		if _, found := m[b[i]]; !found {
			set = append(set, b[i])
		}
	}
	return set
}

// NotUint64 returns the uint64 in slice a that are
// not in slice b with time complexity of approximately
// O(n) leveraging a map to check for element existence
// off by a constant factor of underlying map efficiency.
func NotUint64(a []uint64, b []uint64) []uint64 {
	set := make([]uint64, 0)
	m := make(map[uint64]bool)

	for i := 0; i < len(a); i++ {
		m[a[i]] = true
	}
	for i := 0; i < len(b); i++ {
		if _, found := m[b[i]]; !found {
			set = append(set, b[i])
		}
	}
	return set
}

// IsInUint64 returns true if a is in b and False otherwise.
func IsInUint64(a uint64, b []uint64) bool {
	for _, v := range b {
		if a == v {
			return true
		}
	}
	return false
}

// IntersectionInt32 of two int32 slices with time
// complexity of approximately O(n) leveraging a map to
// check for element existence off by a constant factor
// of underlying map efficiency.
func IntersectionInt32(a []int32, b []int32) []int32 {
	set := make([]int32, 0)
	m := make(map[int32]bool)

	for i := 0; i < len(a); i++ {
		m[a[i]] = true
	}
	for i := 0; i < len(b); i++ {
		if _, found := m[b[i]]; found {
			set = append(set, b[i])
		}
	}
	return set
}

// UnionInt32 of two int32 slices with time
// complexity of approximately O(n) leveraging a map to
// check for element existence off by a constant factor
// of underlying map efficiency.
func UnionInt32(a []int32, b []int32) []int32 {
	set := make([]int32, 0)
	m := make(map[int32]bool)

	for i := 0; i < len(a); i++ {
		m[a[i]] = true
		set = append(set, a[i])
	}
	for i := 0; i < len(b); i++ {
		if _, found := m[b[i]]; !found {
			set = append(set, b[i])
		}
	}
	return set
}

// NotInt32 returns the int32 in slice a that are
// not in slice b with time complexity of approximately
// O(n) leveraging a map to check for element existence
// off by a constant factor of underlying map efficiency.
func NotInt32(a []int32, b []int32) []int32 {
	set := make([]int32, 0)
	m := make(map[int32]bool)

	for i := 0; i < len(a); i++ {
		m[a[i]] = true
	}
	for i := 0; i < len(b); i++ {
		if _, found := m[b[i]]; !found {
			set = append(set, b[i])
		}
	}
	return set
}

// IsInInt32 returns true if a is in b and False otherwise.
func IsInInt32(a int32, b []int32) bool {
	for _, v := range b {
		if a == v {
			return true
		}
	}
	return false
}

// IntersectionInt64 of two int64 slices with time
// complexity of approximately O(n) leveraging a map to
// check for element existence off by a constant factor
// of underlying map efficiency.
func IntersectionInt64(a []int64, b []int64) []int64 {
	set := make([]int64, 0)
	m := make(map[int64]bool)

	for i := 0; i < len(a); i++ {
		m[a[i]] = true
	}
	for i := 0; i < len(b); i++ {
		if _, found := m[b[i]]; found {
			set = append(set, b[i])
		}
	}
	return set
}

// UnionInt64 of two int64 slices with time
// complexity of approximately O(n) leveraging a map to
// check for element existence off by a constant factor
// of underlying map efficiency.
func UnionInt64(a []int64, b []int64) []int64 {
	set := make([]int64, 0)
	m := make(map[int64]bool)

	for i := 0; i < len(a); i++ {
		m[a[i]] = true
		set = append(set, a[i])
	}
	for i := 0; i < len(b); i++ {
		if _, found := m[b[i]]; !found {
			set = append(set, b[i])
		}
	}
	return set
}

// NotInt64 returns the int64 in slice a that are
// not in slice b with time complexity of approximately
// O(n) leveraging a map to check for element existence
// off by a constant factor of underlying map efficiency.
func NotInt64(a []int64, b []int64) []int64 {
	set := make([]int64, 0)
	m := make(map[int64]bool)

	for i := 0; i < len(a); i++ {
		m[a[i]] = true
	}
	for i := 0; i < len(b); i++ {
		if _, found := m[b[i]]; !found {
			set = append(set, b[i])
		}
	}
	return set
}

// IsInInt64 returns true if a is in b and False otherwise.
func IsInInt64(a int64, b []int64) bool {
	for _, v := range b {
		if a == v {
			return true
		}
	}
	return false
}

// ByteIntersection returns a new set with elements that are common in
// both sets a and b.
func ByteIntersection(a []byte, b []byte) []byte {
	set := make([]byte, 0)
	m := make(map[byte]bool)

	for i := 0; i < len(a); i++ {
		m[a[i]] = true
	}
	for i := 0; i < len(b); i++ {
		if _, found := m[b[i]]; found {
			set = append(set, b[i])
		}
	}
	return set
}

// ByteUnion returns a new set with elements that are common in
// both sets a and b.
func ByteUnion(a []byte, b []byte) []byte {
	set := make([]byte, 0)
	m := make(map[byte]bool)

	for i := 0; i < len(a); i++ {
		m[a[i]] = true
		set = append(set, a[i])
	}
	for i := 0; i < len(b); i++ {
		if _, found := m[b[i]]; !found {
			set = append(set, b[i])
		}
	}
	return set
}

// ByteNot returns a new set with elements that are common in
// both sets a and b.
func ByteNot(a []byte, b []byte) []byte {
	set := make([]byte, 0)
	m := make(map[byte]bool)

	for i := 0; i < len(a); i++ {
		m[a[i]] = true
	}
	for i := 0; i < len(b); i++ {
		if _, found := m[b[i]]; !found {
			set = append(set, b[i])
		}
	}
	return set
}

// ByteIsIn returns true if a is in b and False otherwise.
func ByteIsIn(a byte, b []byte) bool {
	for _, v := range b {
		if a == v {
			return true
		}
	}
	return false
}
