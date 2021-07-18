package finality

import (
	"testing"

	"github.com/prysmaticlabs/prysm/spectest/shared/altair/finality"
)

func TestMainnet_Altair_Finality(t *testing.T) {
	finality.RunFinalityTest(t, "mainnet")
}
