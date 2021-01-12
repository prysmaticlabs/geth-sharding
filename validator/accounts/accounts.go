package accounts

import (
	"github.com/prysmaticlabs/prysm/validator/accounts/wallet"
	"github.com/prysmaticlabs/prysm/validator/keymanager"
)

var msgKeymanagerNotSupported = "keymanager kind not supported: %s"

var (
	// ErrCouldNotInitializeKeymanager informs about failed keymanager initialization
	ErrCouldNotInitializeKeymanager = "could not initialize keymanager"
)

// AccountsConfig specifies parameters to run to delete, enable, disable accounts.
type AccountsConfig struct {
	Wallet            *wallet.Wallet
	Keymanager        keymanager.IKeymanager
	DisablePublicKeys [][]byte
	EnablePublicKeys  [][]byte
	DeletePublicKeys  [][]byte
}
