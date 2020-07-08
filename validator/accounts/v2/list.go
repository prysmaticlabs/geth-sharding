package v2

import (
	"context"
	"fmt"
	"path"

	"github.com/logrusorgru/aurora"
	"github.com/pkg/errors"
	"github.com/prysmaticlabs/prysm/validator/flags"
	v2keymanager "github.com/prysmaticlabs/prysm/validator/keymanager/v2"
	"github.com/urfave/cli/v2"
)

// ListAccounts displays all available validator accounts in a Prysm wallet.
func ListAccounts(cliCtx *cli.Context) error {
	walletDir := cliCtx.String(flags.WalletDirFlag.Name)
	if walletDir == flags.DefaultValidatorDir() {
		walletDir = path.Join(walletDir, WalletDefaultDirName)
	}
	passwordsDir := cliCtx.String(flags.WalletPasswordsDirFlag.Name)
	if passwordsDir == flags.DefaultValidatorDir() {
		passwordsDir = path.Join(passwordsDir, PasswordsDefaultDirName)
	}
	// Read the wallet from the specified path.
	ctx := context.Background()
	wallet, err := OpenWallet(ctx, &WalletConfig{
		PasswordsDir: passwordsDir,
		WalletDir:    walletDir,
	})
	if err == ErrNoWalletFound {
		log.Fatal("No wallet nor accounts found, create a new account with `validator accounts-v2 new`")
	} else if err != nil {
		log.Fatalf("Could not read wallet at specified path %s: %v", walletDir, err)
	}
	keymanager, err := wallet.ExistingKeyManager(ctx)
	if err != nil {
		log.Fatalf("Could not initialize keymanager: %v", err)
	}
	switch wallet.KeymanagerKind() {
	case v2keymanager.Direct:
		if err := listDirectKeymanagerAccounts(cliCtx, wallet, keymanager); err != nil {

		}
	default:
		log.Fatalf("Keymanager kind %s not yet supported", wallet.KeymanagerKind().String())
	}
	return nil
}

func listDirectKeymanagerAccounts(cliCtx *cli.Context, wallet *Wallet, keymanager v2keymanager.IKeymanager) error {
	// We initialize the wallet's keymanager.
	accountNames, err := wallet.AccountNames()
	if err != nil {
		return errors.Wrap(err, "could not fetch account names")
	}
	au := aurora.NewAurora(true)
	numAccounts := au.BrightYellow(len(accountNames))
	fmt.Println("")
	if len(accountNames) == 1 {
		fmt.Printf("Showing %d validator account\n", numAccounts)
	} else {
		fmt.Printf("Showing %d validator accounts\n", numAccounts)
	}
	fmt.Println(
		au.BrightRed("View the eth1 deposit transaction data for your accounts " +
			"by running `validator accounts-v2 list --show-deposit-data"),
	)
	dirPath := au.BrightCyan("(wallet dir)")
	fmt.Printf("%s %s\n", dirPath, wallet.AccountsDir())
	dirPath = au.BrightCyan("(passwords dir)")
	fmt.Printf("%s %s\n", dirPath, wallet.passwordsDir)
	fmt.Printf("Keymanager kind: %s\n", au.BrightGreen(wallet.KeymanagerKind().String()).Bold())

	showDepositData := cliCtx.Bool(flags.ShowDepositDataFlag.Name)
	pubKeys, err := keymanager.FetchValidatingPublicKeys(context.Background())
	if err != nil {
		return errors.Wrap(err, "could not fetch validating public keys")
	}
	for i := 0; i < len(accountNames); i++ {
		fmt.Println("")
		fmt.Printf("%s\n", au.BrightGreen(accountNames[i]).Bold())
		fmt.Printf("%s %#x\n", au.BrightMagenta("[public key]").Bold(), pubKeys[i])
		fmt.Printf("%s %s\n", au.BrightCyan("[created at]").Bold(), "July 07, 2020 2:32 PM")
		if !showDepositData {
			continue
		}
		enc, err := wallet.ReadFileForAccount(accountNames[i], "deposit_transaction.rlp")
		if err != nil {
			return errors.Wrapf(err, "could not read file for account: %s", "depo")
		}
		fmt.Printf(
			"%s %s\n",
			"(deposit tx file)",
			path.Join(wallet.AccountsDir(), accountNames[i], "deposit_transaction.rlp"),
		)
		fmt.Printf(`
======================Deposit Transaction Data=====================

%#x

===================================================================`, enc)
		fmt.Println("")
		fmt.Println(
			au.BrightRed("Enter the above deposit data into step 3 on https://prylabs.net/participate").Bold(),
		)
	}
	fmt.Println("")
}
