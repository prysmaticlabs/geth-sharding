package accounts

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"testing"

	validatorpb "github.com/prysmaticlabs/prysm/proto/validator/accounts/v2"
	"github.com/prysmaticlabs/prysm/shared/bls"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/prysmaticlabs/prysm/shared/petnames"
	"github.com/prysmaticlabs/prysm/shared/testutil/assert"
	"github.com/prysmaticlabs/prysm/shared/testutil/require"
	"github.com/prysmaticlabs/prysm/validator/accounts/wallet"
	"github.com/prysmaticlabs/prysm/validator/keymanager"
	"github.com/prysmaticlabs/prysm/validator/keymanager/derived"
	"github.com/prysmaticlabs/prysm/validator/keymanager/imported"
	"github.com/prysmaticlabs/prysm/validator/keymanager/remote"
)

type mockRemoteKeymanager struct {
	publicKeys [][48]byte
	opts       *remote.KeymanagerOpts
}

func (m *mockRemoteKeymanager) FetchValidatingPublicKeys(_ context.Context) ([][48]byte, error) {
	return m.publicKeys, nil
}

func (m *mockRemoteKeymanager) Sign(context.Context, *validatorpb.SignRequest) (bls.Signature, error) {
	return nil, nil
}

func TestListAccounts_DirectKeymanager(t *testing.T) {
	walletDir, passwordsDir, walletPasswordFile := setupWalletAndPasswordsDir(t)
	cliCtx := setupWalletCtx(t, &testWalletConfig{
		walletDir:          walletDir,
		passwordsDir:       passwordsDir,
		keymanagerKind:     keymanager.Imported,
		walletPasswordFile: walletPasswordFile,
	})
	w, err := CreateWalletWithKeymanager(cliCtx.Context, &CreateWalletConfig{
		WalletCfg: &wallet.Config{
			WalletDir:      walletDir,
			KeymanagerKind: keymanager.Imported,
			WalletPassword: "Passwordz0320$",
		},
	})
	require.NoError(t, err)
	keymanager, err := imported.NewKeymanager(
		cliCtx.Context,
		&imported.SetupConfig{
			Wallet: w,
			Opts:   imported.DefaultKeymanagerOpts(),
		},
	)
	require.NoError(t, err)

	numAccounts := 5
	for i := 0; i < numAccounts; i++ {
		_, _, err := keymanager.CreateAccount(cliCtx.Context)
		require.NoError(t, err)
	}
	rescueStdout := os.Stdout
	r, writer, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = writer

	// We call the list imported keymanager accounts function.
	require.NoError(t, listDirectKeymanagerAccounts(context.Background(), true /* show deposit data */, true /*show private keys */, keymanager))

	require.NoError(t, writer.Close())
	out, err := ioutil.ReadAll(r)
	require.NoError(t, err)
	os.Stdout = rescueStdout

	// Get stdout content and split to lines
	newLine := fmt.Sprintln()
	lines := strings.Split(string(out), newLine)

	// Expected output example:
	/*
		(keymanager kind) non-HD wallet

		Showing 5 validator accounts
		View the eth1 deposit transaction data for your accounts by running `validator accounts list --show-deposit-data

		Account 0 | fully-evolving-fawn
		[validating public key] 0xa6669aa0381c06470b9a6faf8abf4194ad5148a62e461cbef5a6bc4d292026f58b992c4cf40e50552d301cef19da75b9
		[validating private key] 0x50cabc13435fcbde9d240fe720aff84f8557a6c1c445211b904f1a9620668241
		If you imported your account coming from the eth2 launchpad, you will find your deposit_data.json in the eth2.0-deposit-cli's validator_keys folder


		Account 1 | preferably-mighty-heron
		[validating public key] 0xa7ea37fa2e2272762ffed8486f09b13cd56d76cf03a2a3e75bc36bd1719add84c20597671750be5bc1ccd3dadfebc30f
		[validating private key] 0x44563da0d11bc6a7219d18217cce8cdd064de3ebee5cdcf8d901c2fae7545116
		If you imported your account coming from the eth2 launchpad, you will find your deposit_data.json in the eth2.0-deposit-cli's validator_keys folder


		Account 2 | conversely-good-monitor
		[validating public key] 0xa4c63619fb8cb87f6dd1686c9255f99c68066797bf284488ecbab64b1926d33eefdf96d1ee89ae4a89e84e7fb019d5e5
		[validating private key] 0x4448d0ab17ecd73bbb636ddbfc89b181731f6cd88c33f2cecc0d04cba1a18447
		If you imported your account coming from the eth2 launchpad, you will find your deposit_data.json in the eth2.0-deposit-cli's validator_keys folder


		Account 3 | rarely-joint-mako
		[validating public key] 0x91dd8d5bfc22aea398740ebcea66ced159df8d3f1a066d7aba9f0bef4ed6d9687fc1fd1c87bd2b6d12b0788dfb6a7d20
		[validating private key] 0x4d1944bd7375185f70b3e70c68d9e6307f2009de3a4cf47ca5217443ddf81fc9
		If you imported your account coming from the eth2 launchpad, you will find your deposit_data.json in the eth2.0-deposit-cli's validator_keys folder


		Account 4 | mainly-useful-catfish
		[validating public key] 0x83c4d722a98b599e2666bbe35146ff44800256190bc662f2dd5efbc0c4c0d57e5d297487a4f9c21a932d3b1b40e8379f
		[validating private key] 0x284cd65030496bf82ee2d52963cd540a1abb2cc738b8164901bbe7e2df4d57bd
		If you imported your account coming from the eth2 launchpad, you will find your deposit_data.json in the eth2.0-deposit-cli's validator_keys folder



	*/

	// Expected output format definition
	const prologLength = 4
	const accountLength = 6
	const epilogLength = 2
	const nameOffset = 1
	const keyOffset = 2
	const privkeyOffset = 3

	// Require the output has correct number of lines
	lineCount := prologLength + accountLength*numAccounts + epilogLength
	require.Equal(t, lineCount, len(lines))

	// Assert the keymanager kind is printed on the first line.
	kindString := "non-HD"
	kindFound := strings.Contains(lines[0], kindString)
	assert.Equal(t, true, kindFound, "Keymanager Kind %s not found on the first line", kindString)

	// Get account names and require the correct count
	accountNames, err := keymanager.ValidatingAccountNames()
	require.NoError(t, err)
	require.Equal(t, numAccounts, len(accountNames))

	// Assert that account names are printed on the correct lines
	for i, accountName := range accountNames {
		lineNumber := prologLength + accountLength*i + nameOffset
		accountNameFound := strings.Contains(lines[lineNumber], accountName)
		assert.Equal(t, true, accountNameFound, "Account Name %s not found on line number %d", accountName, lineNumber)
	}

	// Get public keys and require the correct count
	pubKeys, err := keymanager.FetchValidatingPublicKeys(cliCtx.Context)
	require.NoError(t, err)
	require.Equal(t, numAccounts, len(pubKeys))

	// Assert that public keys are printed on the correct lines
	for i, key := range pubKeys {
		lineNumber := prologLength + accountLength*i + keyOffset
		keyString := fmt.Sprintf("%#x", key)
		keyFound := strings.Contains(lines[lineNumber], keyString)
		assert.Equal(t, true, keyFound, "Public Key %s not found on line number %d", keyString, lineNumber)
	}

	// Get private keys and require the correct count
	privKeys, err := keymanager.FetchValidatingPrivateKeys(cliCtx.Context)
	require.NoError(t, err)
	require.Equal(t, numAccounts, len(pubKeys))

	// Assert that private keys are printed on the correct lines
	for i, key := range privKeys {
		lineNumber := prologLength + accountLength*i + privkeyOffset
		keyString := fmt.Sprintf("%#x", key)
		keyFound := strings.Contains(lines[lineNumber], keyString)
		assert.Equal(t, true, keyFound, "Private Key %s not found on line number %d", keyString, lineNumber)
	}
}

func TestListAccounts_DerivedKeymanager(t *testing.T) {
	walletDir, passwordsDir, passwordFilePath := setupWalletAndPasswordsDir(t)
	cliCtx := setupWalletCtx(t, &testWalletConfig{
		walletDir:          walletDir,
		passwordsDir:       passwordsDir,
		keymanagerKind:     keymanager.Derived,
		walletPasswordFile: passwordFilePath,
	})
	w, err := CreateWalletWithKeymanager(cliCtx.Context, &CreateWalletConfig{
		WalletCfg: &wallet.Config{
			WalletDir:      walletDir,
			KeymanagerKind: keymanager.Derived,
			WalletPassword: "Passwordz0320$",
		},
	})
	require.NoError(t, err)

	keymanager, err := derived.NewKeymanager(
		cliCtx.Context,
		&derived.SetupConfig{
			Opts:                derived.DefaultKeymanagerOpts(),
			Wallet:              w,
			SkipMnemonicConfirm: true,
		},
	)
	require.NoError(t, err)

	numAccounts := 5
	depositDataForAccounts := make([][]byte, numAccounts)
	for i := 0; i < numAccounts; i++ {
		_, _, err := keymanager.CreateAccount(cliCtx.Context)
		require.NoError(t, err)
		enc, err := keymanager.DepositDataForAccount(uint64(i))
		require.NoError(t, err)
		depositDataForAccounts[i] = enc
	}

	rescueStdout := os.Stdout
	r, writer, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = writer

	// We call the list imported keymanager accounts function.
	require.NoError(t, listDerivedKeymanagerAccounts(cliCtx.Context, true /* show deposit data */, true /*show private keys */, keymanager))

	require.NoError(t, writer.Close())
	out, err := ioutil.ReadAll(r)
	require.NoError(t, err)
	os.Stdout = rescueStdout

	// Get stdout content and split to lines
	newLine := fmt.Sprintln()
	lines := strings.Split(string(out), newLine)

	// Expected output example:
	/*
		(keymanager kind) derived, (HD) hierarchical-deterministic
		(derivation format) m / purpose / coin_type / account_index / withdrawal_key / validating_key
		Showing 2 validator accounts

		Account 0 | uniquely-sunny-tarpon
		[withdrawal public key] 0xa5faa97252104b408340b5d8cae3fa01023fa4dc9e7c7b470821433cf3a2a18158410b7d8a6dcdcd176c6552c2526681
		[withdrawal private key] 0x5266fd1f13d7af74614fde4fed3b664bfd529bc4ad91118e3db73647b99546df
		[derivation path] m/12381/3600/0/0
		[validating public key] 0xa7292d8f8d1c1f3d42cacefd2fc4cd3b82651be37c1eb790bbd294a874829f4b7e1c167345dcc1966cc844132b38097e
		[validating private key] 0x590707187dae64b42b8d36a95f3d7e11313ddd8b8d871b09e478e08c9bc8740b
		[derivation path] m/12381/3600/0/0/0

		======================Eth1 Deposit Transaction Data=====================

		0x22895118000000000000000000000000000000000000000000000000000000000000008000000000000000000000000000000000000000000000000000000000000000e000000000000000000000000000000000000000000000000000000000000001205a9e92992d6a97ad113d217fa35cbe0659c662afe913ffd3a3ba61d7473be5630000000000000000000000000000000000000000000000000000000000000030a7292d8f8d1c1f3d42cacefd2fc4cd3b82651be37c1eb790bbd294a874829f4b7e1c167345dcc1966cc844132b38097e000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000020003b8f70706c37fb0b8dcbd95340889bad7d7f29121ea895052a8b216de95e480000000000000000000000000000000000000000000000000000000000000060b6727242b055448defbf54292c65e30ae28ca3aef8a07c8fe674abc0ca42a324be2e7592d3e45bba84ca364d7fe1f0ce073bf8b3692246395aa127cdbf93c64ae9ca48f85cb4b1e519f6821998181de1c7465b2bdcae4ddd0dbc2d02a56219d9

		===================================================================

		Account 1 | usually-obliging-pelican
		[withdrawal public key] 0xb91840d33bb87338bb28605cff837acd50e43a174a8a6d3893108fb91217fa428c12f1b2a25cf3c7aca75d418bcf0384
		[withdrawal private key] 0x72c5ffa7d08fb16cd35a9cb10494dfd49b46842ea1bcc1a4cf46b46680b66810
		[derivation path] m/12381/3600/1/0
		[validating public key] 0x8447f878b701dad4dfa5a884cebc4745b0e8f21340dc56c840826537764dcc54e2e68f80b8d4e5737180212a26211891
		[validating private key] 0x2cd5b1cddc9d96e50a16bea05d0953447655e3dd59fa1bfefad467c73d6c164a
		[derivation path] m/12381/3600/1/0/0

		======================Eth1 Deposit Transaction Data=====================

		0x22895118000000000000000000000000000000000000000000000000000000000000008000000000000000000000000000000000000000000000000000000000000000e000000000000000000000000000000000000000000000000000000000000001200a0b9079c33cc40d602a50f5c51f6db30b0f959fc6f58048d6d43319fea6c09000000000000000000000000000000000000000000000000000000000000000308447f878b701dad4dfa5a884cebc4745b0e8f21340dc56c840826537764dcc54e2e68f80b8d4e5737180212a2621189100000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000002000d6ac42bde23388e7428c1247364347c027c3507e461d68b851d506c60364cf0000000000000000000000000000000000000000000000000000000000000060801a2d432595164d7d88ae1695618db511d1507108573b8471098536b2b5a23f6711235f0a9c6fa65ac26cbd0f2d97e013e0c72ab6b5cff406c48d99ec0a2439aa9faa4557d20bb210d451519101616fa20b1ff2c67fae561cdff160fbc7dc98

		===================================================================


	*/

	// Expected output format definition
	const prologLength = 3
	const accountLength = 14
	const epilogLength = 1
	const nameOffset = 1
	const keyOffset = 5
	const validatingPrivateKeyOffset = 6
	const withdrawalPrivateKeyOffset = 3
	const depositOffset = 11

	// Require the output has correct number of lines
	lineCount := prologLength + accountLength*numAccounts + epilogLength
	require.Equal(t, lineCount, len(lines))

	// Assert the keymanager kind is printed on the first line.
	kindString := w.KeymanagerKind().String()
	kindFound := strings.Contains(lines[0], kindString)
	assert.Equal(t, true, kindFound, "Keymanager Kind %s not found on the first line", kindString)

	// Get account names and require the correct count
	accountNames, err := keymanager.ValidatingAccountNames(cliCtx.Context)
	require.NoError(t, err)
	require.Equal(t, numAccounts, len(accountNames))

	// Assert that account names are printed on the correct lines
	for i, accountName := range accountNames {
		lineNumber := prologLength + accountLength*i + nameOffset
		accountNameFound := strings.Contains(lines[lineNumber], accountName)
		assert.Equal(t, true, accountNameFound, "Account Name %s not found on line number %d", accountName, lineNumber)
	}

	// Get public keys and require the correct count
	pubKeys, err := keymanager.FetchValidatingPublicKeys(cliCtx.Context)
	require.NoError(t, err)
	require.Equal(t, numAccounts, len(pubKeys))

	// Assert that public keys are printed on the correct lines
	for i, key := range pubKeys {
		lineNumber := prologLength + accountLength*i + keyOffset
		keyString := fmt.Sprintf("%#x", key)
		keyFound := strings.Contains(lines[lineNumber], keyString)
		assert.Equal(t, true, keyFound, "Public Key %s not found on line number %d", keyString, lineNumber)
	}

	// Get validating private keys and require the correct count
	validatingPrivKeys, err := keymanager.FetchValidatingPrivateKeys(cliCtx.Context)
	require.NoError(t, err)
	require.Equal(t, numAccounts, len(pubKeys))

	// Assert that validating private keys are printed on the correct lines
	for i, key := range validatingPrivKeys {
		lineNumber := prologLength + accountLength*i + validatingPrivateKeyOffset
		keyString := fmt.Sprintf("%#x", key)
		keyFound := strings.Contains(lines[lineNumber], keyString)
		assert.Equal(t, true, keyFound, "Validating Private Key %s not found on line number %d", keyString, lineNumber)
	}

	// Get withdrawal private keys and require the correct count
	withdrawalPrivKeys, err := keymanager.FetchWithdrawalPrivateKeys(cliCtx.Context)
	require.NoError(t, err)
	require.Equal(t, numAccounts, len(pubKeys))

	// Assert that withdrawal private keys are printed on the correct lines
	for i, key := range withdrawalPrivKeys {
		lineNumber := prologLength + accountLength*i + withdrawalPrivateKeyOffset
		keyString := fmt.Sprintf("%#x", key)
		keyFound := strings.Contains(lines[lineNumber], keyString)
		assert.Equal(t, true, keyFound, "Withdrawal Private Key %s not found on line number %d", keyString, lineNumber)
	}

	// Assert that deposit data are printed on the correct lines
	for i, deposit := range depositDataForAccounts {
		lineNumber := prologLength + accountLength*i + depositOffset
		depositString := fmt.Sprintf("%#x", deposit)
		depositFound := strings.Contains(lines[lineNumber], depositString)
		assert.Equal(t, true, depositFound, "Deposit data %s not found on line number %d", depositString, lineNumber)
	}
}

func TestListAccounts_RemoteKeymanager(t *testing.T) {
	walletDir, _, _ := setupWalletAndPasswordsDir(t)
	cliCtx := setupWalletCtx(t, &testWalletConfig{
		walletDir:      walletDir,
		keymanagerKind: keymanager.Remote,
	})
	w, err := CreateWalletWithKeymanager(cliCtx.Context, &CreateWalletConfig{
		WalletCfg: &wallet.Config{
			WalletDir:      walletDir,
			KeymanagerKind: keymanager.Remote,
			WalletPassword: password,
		},
	})
	require.NoError(t, err)

	rescueStdout := os.Stdout
	r, writer, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = writer

	numAccounts := 3
	pubKeys := make([][48]byte, numAccounts)
	for i := 0; i < numAccounts; i++ {
		key := make([]byte, 48)
		copy(key, strconv.Itoa(i))
		pubKeys[i] = bytesutil.ToBytes48(key)
	}
	km := &mockRemoteKeymanager{
		publicKeys: pubKeys,
		opts: &remote.KeymanagerOpts{
			RemoteCertificate: &remote.CertificateConfig{
				ClientCertPath: "/tmp/client.crt",
				ClientKeyPath:  "/tmp/client.key",
				CACertPath:     "/tmp/ca.crt",
			},
			RemoteAddr: "localhost:4000",
		},
	}
	// We call the list remote keymanager accounts function.
	require.NoError(t, listRemoteKeymanagerAccounts(context.Background(), w, km, km.opts))

	require.NoError(t, writer.Close())
	out, err := ioutil.ReadAll(r)
	require.NoError(t, err)
	os.Stdout = rescueStdout

	// Get stdout content and split to lines
	newLine := fmt.Sprintln()
	lines := strings.Split(string(out), newLine)

	// Expected output example:
	/*
		(keymanager kind) remote signer
		(configuration file path) /tmp/79336/wallet/remote/keymanageropts.json

		Configuration options
		Remote gRPC address: localhost:4000
		Client cert path: /tmp/client.crt
		Client key path: /tmp/client.key
		CA cert path: /tmp/ca.crt

		Showing 3 validator accounts

		equally-primary-foal
		[validating public key] 0x300000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000


		rationally-charmed-werewolf
		[validating public key] 0x310000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000


	*/

	// Expected output format definition
	const prologLength = 10
	const configOffset = 4
	const configLength = 4
	const accountLength = 4
	const nameOffset = 1
	const keyOffset = 2
	const epilogLength = 1

	// Require the output has correct number of lines
	lineCount := prologLength + accountLength*numAccounts + epilogLength
	require.Equal(t, lineCount, len(lines))

	// Assert the keymanager kind is printed on the first line.
	kindString := w.KeymanagerKind().String()
	kindFound := strings.Contains(lines[0], kindString)
	assert.Equal(t, true, kindFound, "Keymanager Kind %s not found on the first line", kindString)

	// Assert that Configuration is printed in the right position
	configLines := lines[configOffset:(configOffset + configLength)]
	configExpected := km.opts.String()
	configActual := fmt.Sprintln(strings.Join(configLines, newLine))
	assert.Equal(t, configExpected, configActual, "Configuration not found at the expected position")

	// Assert that account names are printed on the correct lines
	for i := 0; i < numAccounts; i++ {
		lineNumber := prologLength + accountLength*i + nameOffset
		accountName := petnames.DeterministicName(pubKeys[i][:], "-")
		accountNameFound := strings.Contains(lines[lineNumber], accountName)
		assert.Equal(t, true, accountNameFound, "Account Name %s not found on line number %d", accountName, lineNumber)
	}

	// Assert that public keys are printed on the correct lines
	for i, key := range pubKeys {
		lineNumber := prologLength + accountLength*i + keyOffset
		keyString := fmt.Sprintf("%#x", key)
		keyFound := strings.Contains(lines[lineNumber], keyString)
		assert.Equal(t, true, keyFound, "Public Key %s not found on line number %d", keyString, lineNumber)
	}
}
