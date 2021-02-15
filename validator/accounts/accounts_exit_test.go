package accounts

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/gogo/protobuf/types"
	"github.com/golang/mock/gomock"
	ethpb "github.com/prysmaticlabs/ethereumapis/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/shared/mock"
	"github.com/prysmaticlabs/prysm/shared/testutil/assert"
	"github.com/prysmaticlabs/prysm/shared/testutil/require"
	"github.com/prysmaticlabs/prysm/validator/accounts/wallet"
	"github.com/prysmaticlabs/prysm/validator/keymanager"
	"github.com/prysmaticlabs/prysm/validator/keymanager/imported"
	"github.com/sirupsen/logrus/hooks/test"
	"google.golang.org/grpc/metadata"
)

func TestExitAccountsCli_OK(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockValidatorClient := mock.NewMockBeaconNodeValidatorClient(ctrl)
	mockNodeClient := mock.NewMockNodeClient(ctrl)

	mockValidatorClient.EXPECT().
		ValidatorIndex(gomock.Any(), gomock.Any()).
		Return(&ethpb.ValidatorIndexResponse{Index: 1}, nil)

	// Any time in the past will suffice
	genesisTime := &types.Timestamp{
		Seconds: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC).Unix(),
	}

	mockNodeClient.EXPECT().
		GetGenesis(gomock.Any(), gomock.Any()).
		Return(&ethpb.Genesis{GenesisTime: genesisTime}, nil)

	mockValidatorClient.EXPECT().
		DomainData(gomock.Any(), gomock.Any()).
		Return(&ethpb.DomainResponse{SignatureDomain: make([]byte, 32)}, nil)

	mockValidatorClient.EXPECT().
		ProposeExit(gomock.Any(), gomock.AssignableToTypeOf(&ethpb.SignedVoluntaryExit{})).
		Return(&ethpb.ProposeExitResponse{}, nil)

	walletDir, _, passwordFilePath := setupWalletAndPasswordsDir(t)
	// Write a directory where we will import keys from.
	keysDir := filepath.Join(t.TempDir(), "keysDir")
	require.NoError(t, os.MkdirAll(keysDir, os.ModePerm))

	// Create keystore file in the keys directory we can then import from in our wallet.
	keystore, _ := createKeystore(t, keysDir)
	time.Sleep(time.Second)

	// We initialize a wallet with a imported keymanager.
	cliCtx := setupWalletCtx(t, &testWalletConfig{
		// Wallet configuration flags.
		walletDir:           walletDir,
		keymanagerKind:      keymanager.Imported,
		walletPasswordFile:  passwordFilePath,
		accountPasswordFile: passwordFilePath,
		// Flag required for ImportAccounts to work.
		keysDir: keysDir,
		// Flag required for ExitAccounts to work.
		voluntaryExitPublicKeys: keystore.Pubkey,
	})
	_, err := CreateWalletWithKeymanager(cliCtx.Context, &CreateWalletConfig{
		WalletCfg: &wallet.Config{
			WalletDir:      walletDir,
			KeymanagerKind: keymanager.Imported,
			WalletPassword: password,
		},
	})
	require.NoError(t, err)
	require.NoError(t, ImportAccountsCli(cliCtx))

	validatingPublicKeys, keymanager, err := prepareWallet(cliCtx)
	require.NoError(t, err)
	require.NotNil(t, validatingPublicKeys)
	require.NotNil(t, keymanager)

	// Prepare user input for final confirmation step
	var stdin bytes.Buffer
	stdin.Write([]byte(exitPassphrase))
	rawPubKeys, formattedPubKeys, err := interact(cliCtx, &stdin, validatingPublicKeys)
	require.NoError(t, err)
	require.NotNil(t, rawPubKeys)
	require.NotNil(t, formattedPubKeys)

	cfg := performExitCfg{
		mockValidatorClient,
		mockNodeClient,
		keymanager,
		rawPubKeys,
		formattedPubKeys,
	}
	rawExitedKeys, formattedExitedKeys, err := performExit(cliCtx, cfg)
	require.NoError(t, err)
	require.Equal(t, 1, len(rawExitedKeys))
	assert.DeepEqual(t, rawPubKeys[0], rawExitedKeys[0])
	require.Equal(t, 1, len(formattedExitedKeys))
	assert.Equal(t, "0x"+keystore.Pubkey[:12], formattedExitedKeys[0])
}

func TestExitAccountsCli_OK_AllPublicKeys(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockValidatorClient := mock.NewMockBeaconNodeValidatorClient(ctrl)
	mockNodeClient := mock.NewMockNodeClient(ctrl)

	mockValidatorClient.EXPECT().
		ValidatorIndex(gomock.Any(), gomock.Any()).
		Return(&ethpb.ValidatorIndexResponse{Index: 0}, nil)

	mockValidatorClient.EXPECT().
		ValidatorIndex(gomock.Any(), gomock.Any()).
		Return(&ethpb.ValidatorIndexResponse{Index: 1}, nil)

	// Any time in the past will suffice
	genesisTime := &types.Timestamp{
		Seconds: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC).Unix(),
	}

	mockNodeClient.EXPECT().
		GetGenesis(gomock.Any(), gomock.Any()).
		Times(2).
		Return(&ethpb.Genesis{GenesisTime: genesisTime}, nil)

	mockValidatorClient.EXPECT().
		DomainData(gomock.Any(), gomock.Any()).
		Times(2).
		Return(&ethpb.DomainResponse{SignatureDomain: make([]byte, 32)}, nil)

	mockValidatorClient.EXPECT().
		ProposeExit(gomock.Any(), gomock.AssignableToTypeOf(&ethpb.SignedVoluntaryExit{})).
		Times(2).
		Return(&ethpb.ProposeExitResponse{}, nil)

	walletDir, _, passwordFilePath := setupWalletAndPasswordsDir(t)
	// Write a directory where we will import keys from.
	keysDir := filepath.Join(t.TempDir(), "keysDir")
	require.NoError(t, os.MkdirAll(keysDir, os.ModePerm))

	// Create keystore file in the keys directory we can then import from in our wallet.
	keystore1, _ := createKeystore(t, keysDir)
	time.Sleep(time.Second)
	keystore2, _ := createKeystore(t, keysDir)
	time.Sleep(time.Second)

	// We initialize a wallet with a imported keymanager.
	cliCtx := setupWalletCtx(t, &testWalletConfig{
		// Wallet configuration flags.
		walletDir:           walletDir,
		keymanagerKind:      keymanager.Imported,
		walletPasswordFile:  passwordFilePath,
		accountPasswordFile: passwordFilePath,
		// Flag required for ImportAccounts to work.
		keysDir: keysDir,
		// Exit all public keys.
		exitAll: true,
	})
	_, err := CreateWalletWithKeymanager(cliCtx.Context, &CreateWalletConfig{
		WalletCfg: &wallet.Config{
			WalletDir:      walletDir,
			KeymanagerKind: keymanager.Imported,
			WalletPassword: password,
		},
	})
	require.NoError(t, err)
	require.NoError(t, ImportAccountsCli(cliCtx))

	validatingPublicKeys, keymanager, err := prepareWallet(cliCtx)
	require.NoError(t, err)
	require.NotNil(t, validatingPublicKeys)
	require.NotNil(t, keymanager)

	// Prepare user input for final confirmation step
	var stdin bytes.Buffer
	stdin.Write([]byte(exitPassphrase))
	rawPubKeys, formattedPubKeys, err := interact(cliCtx, &stdin, validatingPublicKeys)
	require.NoError(t, err)
	require.NotNil(t, rawPubKeys)
	require.NotNil(t, formattedPubKeys)

	cfg := performExitCfg{
		mockValidatorClient,
		mockNodeClient,
		keymanager,
		rawPubKeys,
		formattedPubKeys,
	}
	rawExitedKeys, formattedExitedKeys, err := performExit(cliCtx, cfg)
	require.NoError(t, err)
	require.Equal(t, 2, len(rawExitedKeys))
	assert.DeepEqual(t, rawPubKeys, rawExitedKeys)
	require.Equal(t, 2, len(formattedExitedKeys))
	wantedFormatted := []string{
		"0x" + keystore1.Pubkey[:12],
		"0x" + keystore2.Pubkey[:12],
	}
	sort.Strings(wantedFormatted)
	sort.Strings(formattedExitedKeys)
	require.DeepEqual(t, wantedFormatted, formattedExitedKeys)
}

func TestPrepareWallet_EmptyWalletReturnsError(t *testing.T) {
	imported.ResetCaches()
	walletDir, _, passwordFilePath := setupWalletAndPasswordsDir(t)
	cliCtx := setupWalletCtx(t, &testWalletConfig{
		walletDir:           walletDir,
		keymanagerKind:      keymanager.Imported,
		walletPasswordFile:  passwordFilePath,
		accountPasswordFile: passwordFilePath,
	})
	_, err := CreateWalletWithKeymanager(cliCtx.Context, &CreateWalletConfig{
		WalletCfg: &wallet.Config{
			WalletDir:      walletDir,
			KeymanagerKind: keymanager.Imported,
			WalletPassword: password,
		},
	})
	require.NoError(t, err)
	_, _, err = prepareWallet(cliCtx)
	assert.ErrorContains(t, "wallet is empty", err)
}

func TestPrepareClients_AddsGRPCHeaders(t *testing.T) {
	imported.ResetCaches()
	walletDir, _, passwordFilePath := setupWalletAndPasswordsDir(t)
	cliCtx := setupWalletCtx(t, &testWalletConfig{
		walletDir:           walletDir,
		keymanagerKind:      keymanager.Imported,
		walletPasswordFile:  passwordFilePath,
		accountPasswordFile: passwordFilePath,
		grpcHeaders:         "Authorization=Basic some-token,Some-Other-Header=some-value",
	})
	_, err := CreateWalletWithKeymanager(cliCtx.Context, &CreateWalletConfig{
		WalletCfg: &wallet.Config{
			WalletDir:      walletDir,
			KeymanagerKind: keymanager.Imported,
			WalletPassword: password,
		},
	})
	require.NoError(t, err)
	_, _, err = prepareClients(cliCtx)
	require.NoError(t, err)
	md, _ := metadata.FromOutgoingContext(cliCtx.Context)
	assert.Equal(t, "Basic some-token", md.Get("Authorization")[0])
	assert.Equal(t, "some-value", md.Get("Some-Other-Header")[0])
}

func TestDisplayExitInfo(t *testing.T) {
	logHook := test.NewGlobal()
	key := []byte("0x123456")
	displayExitInfo([][]byte{key}, []string{string(key)})
	assert.LogsContain(t, logHook, "https://beaconcha.in/validator/3078313233343536")
}

func TestDisplayExitInfo_NoKeys(t *testing.T) {
	logHook := test.NewGlobal()
	displayExitInfo([][]byte{}, []string{})
	assert.LogsContain(t, logHook, "No successful voluntary exits")
}
