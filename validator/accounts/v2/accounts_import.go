package v2

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/logrusorgru/aurora"
	"github.com/pkg/errors"
	"github.com/prysmaticlabs/prysm/shared/petnames"
	"github.com/prysmaticlabs/prysm/validator/flags"
	v2keymanager "github.com/prysmaticlabs/prysm/validator/keymanager/v2"
	"github.com/prysmaticlabs/prysm/validator/keymanager/v2/direct"
	"github.com/urfave/cli/v2"
)

// ImportAccount uses the archived account made from ExportAccount to import an account and
// asks the users for account passwords.
func ImportAccount(cliCtx *cli.Context) error {
	walletDir, err := inputDirectory(cliCtx, walletDirPromptText, flags.WalletDirFlag)
	if err != nil && !errors.Is(err, ErrNoWalletFound) {
		return errors.Wrap(err, "could not parse wallet directory")
	}
	// Check if the user has a wallet at the specified path. If so, only let them continue if it is a non-HD wallet.
	walletExists, err := hasDir(walletDir)
	if err != nil {
		return errors.Wrap(err, "could not check if wallet exists")
	}
	if walletExists {
		keymanagerKind, err := readKeymanagerKindFromWalletPath(walletDir)
		if err != nil {
			return errors.Wrap(err, "could not read keymanager kind for existing wallet")
		}
		if keymanagerKind != v2keymanager.Direct {
			return fmt.Errorf(
				"importing non-HD accounts into a non-direct wallet is not allowed, given wallet path contains a %s wallet",
				keymanagerKind.String(),
			)
		}
	}
	passwordsDir, err := inputDirectory(cliCtx, passwordsDirPromptText, flags.WalletPasswordsDirFlag)
	if err != nil {
		return err
	}
	keysDir, err := inputDirectory(cliCtx, importKeysDirPromptText, flags.KeysDirFlag)
	if err != nil {
		return errors.Wrap(err, "could not parse keys directory")
	}

	accountsPath := filepath.Join(walletDir, v2keymanager.Direct.String())
	if err := os.MkdirAll(accountsPath, DirectoryPermissions); err != nil {
		return errors.Wrap(err, "could not create wallet directory")
	}
	if err := os.MkdirAll(passwordsDir, DirectoryPermissions); err != nil {
		return errors.Wrap(err, "could not create passwords directory")
	}

	wallet := &Wallet{
		accountsPath:   accountsPath,
		passwordsDir:   passwordsDir,
		keymanagerKind: v2keymanager.Direct,
	}

	var accountsImported []string
	ctx := context.Background()
	if err := filepath.Walk(keysDir, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}

		parentDir := filepath.Dir(path)
		matches, err := filepath.Glob(filepath.Join(parentDir, direct.KeystoreFileName))
		if err != nil {
			return err
		}

		var keystoreFileFound bool
		for _, match := range matches {
			if match == path {
				keystoreFileFound = true
			}
		}
		if !keystoreFileFound {
			return nil
		}

		accountName, err := wallet.importKeystore(ctx, path)
		if err != nil {
			return errors.Wrap(err, "could not import keystore")
		}
		if err := wallet.enterPasswordForAccount(cliCtx, accountName); err != nil {
			return errors.Wrap(err, "could not verify password for keystore")
		}
		accountsImported = append(accountsImported, accountName)
		return nil
	}); err != nil {
		return errors.Wrap(err, "could not walk files")
	}

	keymanager, err := wallet.InitializeKeymanager(context.Background(), true /* skip mnemonic confirm */)
	if err != nil {
		return errors.Wrap(err, "could not initialize keymanager")
	}
	km, ok := keymanager.(*direct.Keymanager)
	if !ok {
		return errors.New("can only export accounts for a non-HD wallet")
	}
	if err := logAccountsImported(wallet, km, accountsImported); err != nil {
		return errors.Wrap(err, "could not log accounts imported")
	}

	return nil
}

func (w *Wallet) importKeystore(ctx context.Context, keystoreFilePath string) (string, error) {
	keystoreBytes, err := ioutil.ReadFile(keystoreFilePath)
	if err != nil {
		return "", errors.Wrap(err, "could not read keystore file")
	}
	keystoreFile := &v2keymanager.Keystore{}
	if err := json.Unmarshal(keystoreBytes, keystoreFile); err != nil {
		return "", errors.Wrap(err, "could not decode keystore json")
	}
	pubKeyBytes, err := hex.DecodeString(keystoreFile.Pubkey)
	if err != nil {
		return "", errors.Wrap(err, "could not decode public key string in keystore")
	}
	accountName := petnames.DeterministicName(pubKeyBytes, "-")
	keystoreFileName := filepath.Base(keystoreFilePath)
	if err := w.WriteFileAtPath(ctx, accountName, keystoreFileName, keystoreBytes); err != nil {
		return "", errors.Wrap(err, "could not write keystore to account dir")
	}
	return accountName, nil
}

func logAccountsImported(wallet *Wallet, keymanager *direct.Keymanager, accountNames []string) error {
	au := aurora.NewAurora(true)

	numAccounts := au.BrightYellow(len(accountNames))
	fmt.Println("")
	if len(accountNames) == 1 {
		fmt.Printf("Imported %d validator account\n", numAccounts)
	} else {
		fmt.Printf("Imported %d validator accounts\n", numAccounts)
	}
	for _, accountName := range accountNames {
		fmt.Println("")
		fmt.Printf("%s\n", au.BrightGreen(accountName).Bold())

		publicKey, err := keymanager.PublicKeyForAccount(accountName)
		if err != nil {
			return errors.Wrap(err, "could not get public key")
		}
		fmt.Printf("%s %#x\n", au.BrightMagenta("[public key]").Bold(), publicKey)

		dirPath := au.BrightCyan("(wallet dir)")
		fmt.Printf("%s %s\n", dirPath, filepath.Join(wallet.AccountsDir(), accountName))
	}
	return nil
}
