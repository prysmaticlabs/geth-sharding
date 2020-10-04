package cache

import (
	"os"
	"testing"

	"github.com/prysmaticlabs/prysm/shared/featureconfig"
)

func TestMain(m *testing.M) {
	resetCfg := featureconfig.InitWithReset(&featureconfig.Flags{EnableEth1DataVoteCache: true})
	defer resetCfg()
	code := m.Run()
	// os.Exit will prevent defer from being called
	resetCfg()
	os.Exit(code)
}
