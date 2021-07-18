package epoch_processing

import (
	"testing"

	"github.com/prysmaticlabs/prysm/shared/params"
)

func TestMain(m *testing.M) {
	prevConfig := params.BeaconConfig().Copy()
	defer params.OverrideBeaconConfig(prevConfig)
	c := params.BeaconConfig()
	c.MinGenesisActiveValidatorCount = 16384
	params.OverrideBeaconConfig(c)

	m.Run()
}
