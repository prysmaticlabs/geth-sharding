package operations

import (
	"testing"

	"github.com/prysmaticlabs/prysm/spectest/shared/phase0/operations"
)

func TestMinimal_Phase0_Operations_BlockHeader(t *testing.T) {
	operations.RunBlockHeaderTest(t, "minimal")
}
