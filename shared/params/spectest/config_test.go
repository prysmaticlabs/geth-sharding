package spectest

import (
	"testing"

	"github.com/prysmaticlabs/prysm/shared/params"
)

func TestConfig(t *testing.T) {
	err := SetConfig("minimal")
	if err != nil {
		t.Error(err)
	}
	if params.BeaconConfig().SlotsPerEpoch != 8 {
		t.Errorf("Expected minimal config to be set, but got %d slots per epoch", params.BeaconConfig().SlotsPerEpoch)
	}

	err = SetConfig("mainnet")
	if err != nil {
		t.Error(err)
	}
	if params.BeaconConfig().SlotsPerEpoch != 32 {
		t.Errorf("Expected mainnet config to be set, but got %d slots per epoch", params.BeaconConfig().SlotsPerEpoch)
	}
}
