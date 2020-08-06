package main

import (
	"crypto/rand"
	"flag"
	"fmt"
	"io/ioutil"
	"math/big"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/prysmaticlabs/prysm/shared/bls"
	"github.com/prysmaticlabs/prysm/shared/testutil"
	"github.com/prysmaticlabs/prysm/shared/testutil/assert"
	"github.com/prysmaticlabs/prysm/shared/testutil/require"
	"github.com/prysmaticlabs/prysm/validator/flags"
	v2keymanager "github.com/prysmaticlabs/prysm/validator/keymanager/v2"
	"github.com/urfave/cli/v2"
	keystorev4 "github.com/wealdtech/go-eth2-wallet-encryptor-keystorev4"
)

func setupCli(
	tb testing.TB,
	passwordFilePath string,
) *cli.Context {
	app := cli.App{}
	set := flag.NewFlagSet("test", 0)
	set.String(flags.AccountPasswordFileFlag.Name, passwordFilePath, "")
	assert.NoError(tb, set.Set(flags.AccountPasswordFileFlag.Name, passwordFilePath))
	return cli.NewContext(&app, set, nil)
}

func createRandomKeystore(t testing.TB, password string) *v2keymanager.Keystore {
	encryptor := keystorev4.New()
	id, err := uuid.NewRandom()
	require.NoError(t, err)
	validatingKey := bls.RandKey()
	pubKey := validatingKey.PublicKey().Marshal()
	cryptoFields, err := encryptor.Encrypt(validatingKey.Marshal(), password)
	require.NoError(t, err)
	return &v2keymanager.Keystore{
		Crypto:  cryptoFields,
		Pubkey:  fmt.Sprintf("%x", pubKey),
		ID:      id.String(),
		Version: encryptor.Version(),
		Name:    encryptor.Name(),
	}
}

func TestDecrypt(t *testing.T) {
	randPath, err := rand.Int(rand.Reader, big.NewInt(1000000))
	require.NoError(t, err)
	passwordFileDir := filepath.Join(testutil.TempDir(), fmt.Sprintf("/%d", randPath), "passwordFile")
	require.NoError(t, os.MkdirAll(passwordFileDir, os.ModePerm))
	passwordFilePath := filepath.Join(passwordFileDir, "password.txt")
	t.Cleanup(func() {
		require.NoError(t, os.RemoveAll(passwordFileDir), "Failed to remove directory")
	})
	password := "secretPassw0rd$1999"
	require.NoError(t, ioutil.WriteFile(passwordFilePath, []byte(password), os.ModePerm))

	// Create several keystores and attempt to import them.
	numAccounts := 5
	keystores := make([]*v2keymanager.Keystore, numAccounts)
	for i := 0; i < numAccounts; i++ {
		keystores[i] = createRandomKeystore(t, password)
	}
	cliCtx := setupCli(t, passwordFilePath)
	require.NoError(t, decrypt(cliCtx))

	// Ensure the single, all-encompassing accounts keystore was written
	// to the wallet and ensure we can decrypt it using the EIP-2335 standard.
	//var encodedKeystore []byte
	//for k, v := range wallet.Files[AccountsPath] {
	//	if strings.Contains(k, "keystore") {
	//		encodedKeystore = v
	//	}
	//}
	//require.NotNil(t, encodedKeystore, "could not find keystore file")
	//keystoreFile := &v2keymanager.Keystore{}
	//require.NoError(t, json.Unmarshal(encodedKeystore, keystoreFile))
	//
	//// We decrypt the crypto fields of the accounts keystore.
	//decryptor := keystorev4.New()
	//encodedAccounts, err := decryptor.Decrypt(keystoreFile.Crypto, password)
	//require.NoError(t, err, "Could not decrypt validator accounts")
	//store := &AccountStore{}
	//require.NoError(t, json.Unmarshal(encodedAccounts, store))
	//
	//// We should have successfully imported all accounts
	//// from external sources into a single AccountsStore
	//// struct preserved within a single keystore file.
	//assert.Equal(t, numAccounts, len(store.PublicKeys))
	//assert.Equal(t, numAccounts, len(store.PrivateKeys))
}
