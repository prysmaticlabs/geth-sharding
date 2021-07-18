package sanity

import (
	"testing"

	"github.com/prysmaticlabs/prysm/spectest/shared/altair/sanity"
)

func TestMinimal_Altair_Sanity_Blocks(t *testing.T) {
	sanity.RunBlockProcessingTest(t, "minimal")
}
