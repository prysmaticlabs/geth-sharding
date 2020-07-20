package v2

import (
	"context"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/prysmaticlabs/prysm/shared/testutil/require"

	"github.com/prysmaticlabs/prysm/shared/testutil"
	"github.com/prysmaticlabs/prysm/shared/testutil/assert"
	"github.com/prysmaticlabs/prysm/validator/flags"
	v2 "github.com/prysmaticlabs/prysm/validator/keymanager/v2"
	"github.com/urfave/cli/v2"
)

func setupWallet(t *testing.T, testDir string) *Wallet {
	walletDir := filepath.Join(testDir, walletDirName)
	passwordsDir := filepath.Join(testDir, passwordDirName)
	ctx := context.Background()

	app := cli.App{}
	set := flag.NewFlagSet("test", 0)
	set.String(flags.WalletPasswordsDirFlag.Name, passwordsDir, "")
	assert.NoError(t, set.Set(flags.WalletPasswordsDirFlag.Name, passwordsDir))
	cliCtx := cli.NewContext(&app, set, nil)
	assert.NoError(t, createDirectWallet(cliCtx, walletDir))
	cfg := &WalletConfig{
		WalletDir:      walletDir,
		PasswordsDir:   passwordsDir,
		KeymanagerKind: v2.Direct,
	}
	w, err := NewWallet(ctx, cfg)
	require.NoError(t, err)

	keymanager, err := w.InitializeKeymanager(ctx, true)
	require.NoError(t, err)

	_, err = keymanager.CreateAccount(ctx, password)
	require.NoError(t, err)
	return w
}

func TestZipAndUnzip(t *testing.T) {
	testDir := testutil.TempDir()
	walletDir := filepath.Join(testDir, walletDirName)
	passwordsDir := filepath.Join(testDir, passwordDirName)
	exportDir := filepath.Join(testDir, exportDirName)
	importDir := filepath.Join(testDir, importDirName)
	defer func() {
		assert.NoError(t, os.RemoveAll(walletDir))
		assert.NoError(t, os.RemoveAll(passwordsDir))
		assert.NoError(t, os.RemoveAll(exportDir))
		assert.NoError(t, os.RemoveAll(importDir))
	}()
	wallet := setupWallet(t, testDir)

	accounts, err := wallet.AccountNames()
	require.NoError(t, err)

	if len(accounts) == 0 {
		t.Fatal("Expected more accounts, received 0")
	}
	err = wallet.zipAccounts(accounts, exportDir)
	require.NoError(t, err)

	if _, err := os.Stat(filepath.Join(exportDir, archiveFilename)); os.IsNotExist(err) {
		t.Fatal("Expected file to exist")
	}

	importedAccounts, err := unzipArchiveToTarget(exportDir, importDir)
	require.NoError(t, err)

	allAccountsStr := strings.Join(accounts, " ")
	for _, importedAccount := range importedAccounts {
		if !strings.Contains(allAccountsStr, importedAccount) {
			t.Fatalf("Expected %s to be in %s", importedAccount, allAccountsStr)
		}
	}
}

func TestExport_Noninteractive(t *testing.T) {
	testDir := testutil.TempDir()
	walletDir := filepath.Join(testDir, walletDirName)
	passwordsDir := filepath.Join(testDir, passwordDirName)
	exportDir := filepath.Join(testDir, exportDirName)
	accounts := "all"
	defer func() {
		assert.NoError(t, os.RemoveAll(walletDir))
		assert.NoError(t, os.RemoveAll(passwordsDir))
		assert.NoError(t, os.RemoveAll(exportDir))
	}()
	setupWallet(t, testDir)
	app := cli.App{}
	set := flag.NewFlagSet("test", 0)
	set.String(flags.WalletDirFlag.Name, walletDir, "")
	set.String(flags.WalletPasswordsDirFlag.Name, passwordsDir, "")
	set.String(flags.BackupPathFlag.Name, exportDir, "")
	set.String(flags.AccountsFlag.Name, accounts, "")
	assert.NoError(t, set.Set(flags.WalletDirFlag.Name, walletDir))
	assert.NoError(t, set.Set(flags.WalletPasswordsDirFlag.Name, passwordsDir))
	assert.NoError(t, set.Set(flags.BackupPathFlag.Name, exportDir))
	assert.NoError(t, set.Set(flags.AccountsFlag.Name, accounts))
	cliCtx := cli.NewContext(&app, set, nil)

	require.NoError(t, ExportAccount(cliCtx))
	if _, err := os.Stat(filepath.Join(exportDir, archiveFilename)); os.IsNotExist(err) {
		t.Fatal("Expected file to exist")
	}
}
