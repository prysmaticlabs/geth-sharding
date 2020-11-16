package imported

import (
	"context"
	"errors"

	"github.com/prysmaticlabs/prysm/shared/bytesutil"
)

// DisableAccounts disables public keys from the user's wallet.
func (dr *Keymanager) DisableAccounts(ctx context.Context, pubKeys [][]byte) error {
	if pubKeys == nil || len(pubKeys) < 1 {
		return errors.New("no public keys specified to disable")
	}
	updatedDisabledPubKeys := make([][]byte, 0)
	existingDisabledPubKeys := make(map[[48]byte]bool, len(dr.disabledPublicKeys))
	for _, pk := range dr.disabledPublicKeys {
		existingDisabledPubKeys[bytesutil.ToBytes48(pk)] = true
	}
	for _, pk := range pubKeys {
		if _, ok := existingDisabledPubKeys[bytesutil.ToBytes48(pk)]; !ok {
			updatedDisabledPubKeys = append(updatedDisabledPubKeys, pk)
		}
	}
	dr.disabledPublicKeys = updatedDisabledPubKeys
	return nil
}

// EnableAccounts enables public keys from a user's wallet if they are disabled.
func (dr *Keymanager) EnableAccounts(ctx context.Context, pubKeys [][]byte) error {
	if pubKeys == nil || len(pubKeys) < 1 {
		return errors.New("no public keys specified to enable")
	}
	updatedDisabledPubKeys := make([][]byte, 0)
	setEnabledPubKeys := make(map[[48]byte]bool, len(pubKeys))
	for _, pk := range pubKeys {
		setEnabledPubKeys[bytesutil.ToBytes48(pk)] = true
	}
	for _, pk := range dr.disabledPublicKeys {
		if _, ok := setEnabledPubKeys[bytesutil.ToBytes48(pk)]; !ok {
			updatedDisabledPubKeys = append(updatedDisabledPubKeys, pk)
		}
	}
	dr.disabledPublicKeys = updatedDisabledPubKeys
	return nil
}
