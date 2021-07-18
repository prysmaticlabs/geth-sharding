package sanity

import (
	"testing"

	"github.com/prysmaticlabs/prysm/spectest/shared/altair/sanity"
)

func TestMainnet_Altair_Sanity_Blocks(t *testing.T) {
	sanity.RunBlockProcessingTest(t, "mainnet")
}
