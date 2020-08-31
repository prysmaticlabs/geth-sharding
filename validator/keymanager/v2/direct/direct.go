package direct

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/logrusorgru/aurora"
	"github.com/pkg/errors"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	validatorpb "github.com/prysmaticlabs/prysm/proto/validator/accounts/v2"
	"github.com/prysmaticlabs/prysm/shared/bls"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/prysmaticlabs/prysm/shared/depositutil"
	"github.com/prysmaticlabs/prysm/shared/interop"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/petnames"
	"github.com/prysmaticlabs/prysm/shared/promptutil"
	"github.com/prysmaticlabs/prysm/validator/accounts/v2/iface"
	v2keymanager "github.com/prysmaticlabs/prysm/validator/keymanager/v2"
	"github.com/sirupsen/logrus"
	keystorev4 "github.com/wealdtech/go-eth2-wallet-encryptor-keystorev4"
)

var log = logrus.WithField("prefix", "direct-keymanager-v2")

const (
	// KeystoreFileNameFormat exposes the filename the keystore should be formatted in.
	KeystoreFileNameFormat = "keystore-%d.json"
	// AccountsPath where all direct keymanager keystores are kept.
	AccountsPath             = "accounts"
	accountsKeystoreFileName = "all-accounts.keystore.json"
	eipVersion               = "EIP-2335"
)

// KeymanagerOpts for a direct keymanager.
type KeymanagerOpts struct {
	EIPVersion string `json:"direct_eip_version"`
}

// Keymanager implementation for direct keystores utilizing EIP-2335.
type Keymanager struct {
	wallet        iface.Wallet
	opts          *KeymanagerOpts
	keysCache     map[[48]byte]bls.SecretKey
	accountsStore *AccountStore
	lock          sync.RWMutex
}

// AccountStore --
type AccountStore struct {
	PrivateKeys [][]byte `json:"private_keys"`
	PublicKeys  [][]byte `json:"public_keys"`
}

// DefaultKeymanagerOpts for a direct keymanager implementation.
func DefaultKeymanagerOpts() *KeymanagerOpts {
	return &KeymanagerOpts{
		EIPVersion: eipVersion,
	}
}

// SetupConfig includes configuration values for initializing
// a keymanager, such as passwords, the wallet, and more.
type SetupConfig struct {
	Wallet              iface.Wallet
	Opts                *KeymanagerOpts
	SkipMnemonicConfirm bool
	Mnemonic            string
}

// NewKeymanager instantiates a new direct keymanager from configuration options.
func NewKeymanager(ctx context.Context, cfg *SetupConfig) (*Keymanager, error) {
	k := &Keymanager{
		wallet:        cfg.Wallet,
		opts:          cfg.Opts,
		keysCache:     make(map[[48]byte]bls.SecretKey),
		accountsStore: &AccountStore{},
	}

	// If the wallet has the capability of unlocking accounts using
	// passphrases, then we initialize a cache of public key -> secret keys
	// used to retrieve secrets keys for the accounts via password unlock.
	// This cache is needed to process Sign requests using a public key.
	if err := k.initializeSecretKeysCache(ctx); err != nil {
		return nil, errors.Wrap(err, "could not initialize keys cache")
	}
	return k, nil
}

// NewInteropKeymanager instantiates a new direct keymanager with the deterministically generated interop keys.
func NewInteropKeymanager(ctx context.Context, offset uint64, numValidatorKeys uint64) (*Keymanager, error) {
	k := &Keymanager{
		keysCache: make(map[[48]byte]bls.SecretKey),
	}
	if numValidatorKeys == 0 {
		return k, nil
	}

	secretKeys, publicKeys, err := interop.DeterministicallyGenerateKeys(offset, numValidatorKeys)
	if err != nil {
		return nil, errors.Wrap(err, "could not generate interop keys")
	}

	for i := 0; i < len(publicKeys); i++ {
		k.keysCache[bytesutil.ToBytes48(publicKeys[i].Marshal())] = secretKeys[i]
	}
	return k, nil
}

// UnmarshalOptionsFile attempts to JSON unmarshal a direct keymanager
// options file into a struct.
func UnmarshalOptionsFile(r io.ReadCloser) (*KeymanagerOpts, error) {
	enc, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := r.Close(); err != nil {
			log.Errorf("Could not close keymanager config file: %v", err)
		}
	}()
	opts := &KeymanagerOpts{}
	if err := json.Unmarshal(enc, opts); err != nil {
		return nil, err
	}
	return opts, nil
}

// MarshalOptionsFile returns a marshaled options file for a keymanager.
func MarshalOptionsFile(ctx context.Context, opts *KeymanagerOpts) ([]byte, error) {
	return json.MarshalIndent(opts, "", "\t")
}

// KeymanagerOpts for the direct keymanager.
func (dr *Keymanager) KeymanagerOpts() *KeymanagerOpts {
	return dr.opts
}

// String pretty-print of a direct keymanager options.
func (opts *KeymanagerOpts) String() string {
	au := aurora.NewAurora(true)
	var b strings.Builder
	strAddr := fmt.Sprintf("%s: %s\n", au.BrightMagenta("EIP Version"), opts.EIPVersion)
	if _, err := b.WriteString(strAddr); err != nil {
		log.Error(err)
		return ""
	}
	return b.String()
}

// ValidatingAccountNames for a direct keymanager.
func (dr *Keymanager) ValidatingAccountNames() ([]string, error) {
	names := make([]string, len(dr.keysCache))
	index := 0
	for pubKey := range dr.keysCache {
		names[index] = petnames.DeterministicName(pubKey[:], "-")
		index++
	}
	return names, nil
}

// CreateAccount for a direct keymanager implementation. This utilizes
// the EIP-2335 keystore standard for BLS12-381 keystores. It
// stores the generated keystore.json file in the wallet and additionally
// generates withdrawal credentials. At the end, it logs
// the raw deposit data hex string for users to copy.
func (dr *Keymanager) CreateAccount(ctx context.Context) (string, error) {
	// Create a petname for an account from its public key and write its password to disk.
	validatingKey := bls.RandKey()
	accountName := petnames.DeterministicName(validatingKey.PublicKey().Marshal(), "-")
	dr.accountsStore.PrivateKeys = append(dr.accountsStore.PrivateKeys, validatingKey.Marshal())
	dr.accountsStore.PublicKeys = append(dr.accountsStore.PublicKeys, validatingKey.PublicKey().Marshal())
	newStore, err := dr.createAccountsKeystore(ctx, dr.accountsStore.PrivateKeys, dr.accountsStore.PublicKeys)
	if err != nil {
		return "", errors.Wrap(err, "could not create accounts keystore")
	}

	// Generate a withdrawal key and confirm user
	// acknowledgement of a 256-bit entropy mnemonic phrase.
	withdrawalKey := bls.RandKey()
	log.Info(
		"Write down the private key, as it is your unique " +
			"withdrawal private key for eth2",
	)
	fmt.Printf(`
==========================Withdrawal Key===========================

%#x

===================================================================
	`, withdrawalKey.Marshal())
	fmt.Println(" ")

	// Upon confirmation of the withdrawal key, proceed to display
	// and write associated deposit data to disk.
	tx, data, err := depositutil.GenerateDepositTransaction(validatingKey, withdrawalKey)
	if err != nil {
		return "", errors.Wrap(err, "could not generate deposit transaction data")
	}
	domain, err := helpers.ComputeDomain(
		params.BeaconConfig().DomainDeposit,
		nil, /*forkVersion*/
		nil, /*genesisValidatorsRoot*/
	)
	if err := depositutil.VerifyDepositSignature(data, domain); err != nil {
		return "", errors.Wrap(err, "failed to verify deposit signature, please make sure your account was created properly")
	}

	// Log the deposit transaction data to the user.
	fmt.Printf(`
==================Eth1 Deposit Transaction Data=================
%#x
================Verified for the %s network================`, tx.Data(), params.BeaconConfig().NetworkName)
	fmt.Println("")

	// Write the encoded keystore.
	encoded, err := json.MarshalIndent(newStore, "", "\t")
	if err != nil {
		return "", err
	}
	if err := dr.wallet.WriteFileAtPath(ctx, AccountsPath, accountsKeystoreFileName, encoded); err != nil {
		return "", errors.Wrap(err, "could not write keystore file for accounts")
	}

	log.WithFields(logrus.Fields{
		"name": accountName,
	}).Info("Successfully created new validator account")
	dr.lock.Lock()
	dr.keysCache[bytesutil.ToBytes48(validatingKey.PublicKey().Marshal())] = validatingKey
	dr.lock.Unlock()
	return accountName, nil
}

// DeleteAccounts takes in public keys and removes the accounts entirely. This includes their disk keystore and cached keystore.
func (dr *Keymanager) DeleteAccounts(ctx context.Context, publicKeys [][]byte) error {
	for _, publicKey := range publicKeys {
		var index int
		var found bool
		for i, pubKey := range dr.accountsStore.PublicKeys {
			if bytes.Equal(pubKey, publicKey) {
				index = i
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("could not find public key %#x", publicKey)
		}
		deletedPublicKey := dr.accountsStore.PublicKeys[index]
		accountName := petnames.DeterministicName(deletedPublicKey, "-")
		dr.accountsStore.PrivateKeys = append(dr.accountsStore.PrivateKeys[:index], dr.accountsStore.PrivateKeys[index+1:]...)
		dr.accountsStore.PublicKeys = append(dr.accountsStore.PublicKeys[:index], dr.accountsStore.PublicKeys[index+1:]...)

		newStore, err := dr.createAccountsKeystore(ctx, dr.accountsStore.PrivateKeys, dr.accountsStore.PublicKeys)
		if err != nil {
			return errors.Wrap(err, "could not rewrite accounts keystore")
		}

		// Write the encoded keystore.
		encoded, err := json.MarshalIndent(newStore, "", "\t")
		if err != nil {
			return err
		}
		if err := dr.wallet.WriteFileAtPath(ctx, AccountsPath, accountsKeystoreFileName, encoded); err != nil {
			return errors.Wrap(err, "could not write keystore file for accounts")
		}

		log.WithFields(logrus.Fields{
			"name":      accountName,
			"publicKey": fmt.Sprintf("%#x", bytesutil.Trunc(deletedPublicKey)),
		}).Info("Successfully deleted validator account")
		dr.lock.Lock()
		delete(dr.keysCache, bytesutil.ToBytes48(deletedPublicKey))
		dr.lock.Unlock()
	}
	return nil
}

// FetchValidatingPublicKeys fetches the list of public keys from the direct account keystores.
func (dr *Keymanager) FetchValidatingPublicKeys(ctx context.Context) ([][48]byte, error) {
	accountNames, err := dr.ValidatingAccountNames()
	if err != nil {
		return nil, err
	}

	// Return the public keys from the cache if they match the
	// number of accounts from the wallet.
	publicKeys := make([][48]byte, len(accountNames))
	dr.lock.Lock()
	defer dr.lock.Unlock()
	if dr.keysCache != nil && len(dr.keysCache) == len(accountNames) {
		var i int
		for k := range dr.keysCache {
			publicKeys[i] = k
			i++
		}
		return publicKeys, nil
	}
	return nil, nil
}

// Sign signs a message using a validator key.
func (dr *Keymanager) Sign(ctx context.Context, req *validatorpb.SignRequest) (bls.Signature, error) {
	rawPubKey := req.PublicKey
	if rawPubKey == nil {
		return nil, errors.New("nil public key in request")
	}
	dr.lock.RLock()
	defer dr.lock.RUnlock()
	secretKey, ok := dr.keysCache[bytesutil.ToBytes48(rawPubKey)]
	if !ok {
		return nil, errors.New("no signing key found in keys cache")
	}
	return secretKey.Sign(req.SigningRoot), nil
}

func (dr *Keymanager) initializeSecretKeysCache(ctx context.Context) error {
	encoded, err := dr.wallet.ReadFileAtPath(context.Background(), AccountsPath, accountsKeystoreFileName)
	if err != nil && strings.Contains(err.Error(), "no files found") {
		// If there are no keys to initialize at all, just exit.
		return nil
	} else if err != nil {
		return errors.Wrapf(err, "could not read keystore file for accounts %s", accountsKeystoreFileName)
	}
	keystoreFile := &v2keymanager.Keystore{}
	if err := json.Unmarshal(encoded, keystoreFile); err != nil {
		return errors.Wrapf(err, "could not decode keystore file for accounts %s", accountsKeystoreFileName)
	}
	// We extract the validator signing private key from the keystore
	// by utilizing the password and initialize a new BLS secret key from
	// its raw bytes.
	password := dr.wallet.Password()
	decryptor := keystorev4.New()
	enc, err := decryptor.Decrypt(keystoreFile.Crypto, password)
	if err != nil && strings.Contains(err.Error(), "invalid checksum") {
		// If the password fails for an individual account, we ask the user to input
		// that individual account's password until it succeeds.
		enc, password, err = dr.askUntilPasswordConfirms(decryptor, keystoreFile)
		if err != nil {
			return errors.Wrap(err, "could not confirm password via prompt")
		}
	} else if err != nil {
		return errors.Wrap(err, "could not decrypt keystore")
	}

	store := &AccountStore{}
	if err := json.Unmarshal(enc, store); err != nil {
		return err
	}
	if len(store.PublicKeys) != len(store.PrivateKeys) {
		return errors.New("unequal number of public keys and private keys")
	}
	if len(store.PublicKeys) == 0 {
		return nil
	}
	dr.lock.Lock()
	defer dr.lock.Unlock()
	for i := 0; i < len(store.PublicKeys); i++ {
		privKey, err := bls.SecretKeyFromBytes(store.PrivateKeys[i])
		if err != nil {
			return err
		}
		dr.keysCache[bytesutil.ToBytes48(store.PublicKeys[i])] = privKey
	}
	dr.accountsStore = store
	return err
}

func (dr *Keymanager) createAccountsKeystore(
	ctx context.Context,
	privateKeys [][]byte,
	publicKeys [][]byte,
) (*v2keymanager.Keystore, error) {
	au := aurora.NewAurora(true)
	encryptor := keystorev4.New()
	id, err := uuid.NewRandom()
	if err != nil {
		return nil, err
	}
	if len(privateKeys) != len(publicKeys) {
		return nil, fmt.Errorf(
			"number of private keys and public keys is not equal: %d != %d", len(privateKeys), len(publicKeys),
		)
	}
	if dr.accountsStore == nil {
		dr.accountsStore = &AccountStore{
			PrivateKeys: privateKeys,
			PublicKeys:  publicKeys,
		}
	} else {
		existingPubKeys := make(map[string]bool)
		existingPrivKeys := make(map[string]bool)
		for i := 0; i < len(dr.accountsStore.PrivateKeys); i++ {
			existingPrivKeys[string(dr.accountsStore.PrivateKeys[i])] = true
			existingPubKeys[string(dr.accountsStore.PublicKeys[i])] = true
		}
		// We append to the accounts store keys only
		// if the private/secret key do not already exist, to prevent duplicates.
		for i := 0; i < len(privateKeys); i++ {
			sk := privateKeys[i]
			pk := publicKeys[i]
			_, privKeyExists := existingPrivKeys[string(sk)]
			_, pubKeyExists := existingPubKeys[string(pk)]
			if privKeyExists || pubKeyExists {
				fmt.Printf("Public key %#x already exists\n", au.BrightMagenta(bytesutil.Trunc(pk)))
				continue
			}
			dr.accountsStore.PublicKeys = append(dr.accountsStore.PublicKeys, pk)
			dr.accountsStore.PrivateKeys = append(dr.accountsStore.PrivateKeys, sk)
		}
	}
	encodedStore, err := json.MarshalIndent(dr.accountsStore, "", "\t")
	if err != nil {
		return nil, err
	}
	cryptoFields, err := encryptor.Encrypt(encodedStore, dr.wallet.Password())
	if err != nil {
		return nil, errors.Wrap(err, "could not encrypt accounts")
	}
	return &v2keymanager.Keystore{
		Crypto:  cryptoFields,
		ID:      id.String(),
		Version: encryptor.Version(),
		Name:    encryptor.Name(),
	}, nil
}

func (dr *Keymanager) askUntilPasswordConfirms(
	decryptor *keystorev4.Encryptor, keystore *v2keymanager.Keystore,
) ([]byte, string, error) {
	au := aurora.NewAurora(true)
	// Loop asking for the password until the user enters it correctly.
	var secretKey []byte
	var password string
	var err error
	publicKey, err := hex.DecodeString(keystore.Pubkey)
	if err != nil {
		return nil, "", errors.Wrap(err, "could not decode public key")
	}
	formattedPublicKey := fmt.Sprintf("%#x", bytesutil.Trunc(publicKey))
	for {
		password, err = promptutil.PasswordPrompt(
			fmt.Sprintf("\nPlease try again, could not use password to import account %s", au.BrightGreen(formattedPublicKey)),
			promptutil.NotEmpty,
		)
		if err != nil {
			return nil, "", fmt.Errorf("could not read account password: %v", err)
		}
		secretKey, err = decryptor.Decrypt(keystore.Crypto, password)
		if err != nil && strings.Contains(err.Error(), "invalid checksum") {
			fmt.Print(au.Red("Incorrect password entered, please try again"))
			continue
		}
		if err != nil {
			return nil, "", err
		}
		break
	}
	return secretKey, password, nil
}
