package keymanager

import (
	"context"
	"fmt"

	validatorpb "github.com/prysmaticlabs/prysm/proto/validator/accounts/v2"
	"github.com/prysmaticlabs/prysm/shared/bls"
	"github.com/prysmaticlabs/prysm/shared/event"
)

// IKeymanager defines a general keymanager interface for Prysm wallets.
type IKeymanager interface {
	// FetchValidatingPublicKeys fetches the list of active public keys that should be used to validate with.
	FetchValidatingPublicKeys(ctx context.Context) ([][48]byte, error)
	// FetchAllValidatingPublicKeys fetches the list of all public keys, including disabled ones.
	FetchAllValidatingPublicKeys(ctx context.Context) ([][48]byte, error)
	// Sign signs a message using a validator key.
	Sign(context.Context, *validatorpb.SignRequest) (bls.Signature, error)
	// SubscribeAccountChanges subscribes to changes made to the underlying keys.
	SubscribeAccountChanges(pubKeysChan chan [][48]byte) event.Subscription
}

// Keystore json file representation as a Go struct.
type Keystore struct {
	Crypto  map[string]interface{} `json:"crypto"`
	ID      string                 `json:"uuid"`
	Pubkey  string                 `json:"pubkey"`
	Version uint                   `json:"version"`
	Name    string                 `json:"name"`
}

// Kind defines an enum for either imported, derived, or remote-signing
// keystores for Prysm wallets.
type Kind int

const (
	// Imported keymanager defines an on-disk, encrypted keystore-capable store.
	Imported Kind = iota
	// Derived keymanager using a hierarchical-deterministic algorithm.
	Derived
	// Remote keymanager capable of remote-signing data.
	Remote
)

// String marshals a keymanager kind to a string value.
func (k Kind) String() string {
	switch k {
	case Derived:
		return "derived"
	case Imported:
		return "direct"
	case Remote:
		return "remote"
	default:
		return fmt.Sprintf("%d", int(k))
	}
}

// ParseKind from a raw string, returning a keymanager kind.
func ParseKind(k string) (Kind, error) {
	switch k {
	case "derived":
		return Derived, nil
	case "direct":
		return Imported, nil
	case "remote":
		return Remote, nil
	default:
		return 0, fmt.Errorf("%s is not an allowed keymanager", k)
	}
}
