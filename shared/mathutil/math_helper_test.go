package mathutil

import (
	"testing"
)

func TestIntegerSquareRoot(t *testing.T) {
	tt := []struct {
		number uint64
		root   uint64
	}{
		{
			number: 20,
			root:   4,
		},
		{
			number: 200,
			root:   14,
		},
		{
			number: 1987,
			root:   44,
		},
		{
			number: 34989843,
			root:   5915,
		},
		{
			number: 97282,
			root:   311,
		},
	}

	for _, testVals := range tt {
		root := IntegerSquareRoot(testVals.number)
		if testVals.root != root {
			t.Fatalf("expected root and computed root are not equal %d, %d", testVals.root, root)
		}
	}
}

func TestCeilDiv8(t *testing.T) {
	tests := []struct {
		number int
		div8   int
	}{
		{
			number: 20,
			div8:   3,
		},
		{
			number: 200,
			div8:   25,
		},
		{
			number: 1987,
			div8:   249,
		},
		{
			number: 1,
			div8:   1,
		},
		{
			number: 97282,
			div8:   12161,
		},
	}

	for _, tt := range tests {
		div8 := CeilDiv8(tt.number)
		if tt.div8 != div8 {
			t.Fatalf("Div8 was not an expected value. Wanted: %d, got: %d", tt.div8, div8)
		}
	}
}
