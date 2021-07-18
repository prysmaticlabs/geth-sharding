package sanity

import (
	"testing"

	"github.com/prysmaticlabs/prysm/spectest/shared/altair/sanity"
)

func TestMinimal_Altair_Sanity_Slots(t *testing.T) {
	sanity.RunSlotProcessingTests(t, "minimal")
}
