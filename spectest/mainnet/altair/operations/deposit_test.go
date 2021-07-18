package operations

import (
	"testing"

	"github.com/prysmaticlabs/prysm/spectest/shared/altair/operations"
)

func TestMainnet_Altair_Operations_Deposit(t *testing.T) {
	operations.RunDepositTest(t, "mainnet")
}
