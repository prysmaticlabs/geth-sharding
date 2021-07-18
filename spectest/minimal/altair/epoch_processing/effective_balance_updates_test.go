package epoch_processing

import (
	"testing"

	"github.com/prysmaticlabs/prysm/spectest/shared/altair/epoch_processing"
)

func TestMinimal_Altair_EpochProcessing_EffectiveBalanceUpdates(t *testing.T) {
	epoch_processing.RunEffectiveBalanceUpdatesTests(t, "minimal")
}
