package p2p

import (
	"crypto/ecdsa"
	"crypto/rand"
	"fmt"
	"net"
	"os"

	"github.com/btcsuite/btcd/btcec"
	curve "github.com/ethereum/go-ethereum/crypto"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/prysmaticlabs/prysm/shared/iputils"
)

// buildOptions for the libp2p host.
func buildOptions(cfg *Config) ([]libp2p.Option, net.IP, *ecdsa.PrivateKey) {
	ip, err := iputils.ExternalIPv4()
	if err != nil {
		log.Fatalf("Could not get IPv4 address: %v", err)
	}
	listen, err := ma.NewMultiaddr(fmt.Sprintf("/ip4/%s/tcp/%d", ip, cfg.Port))
	if err != nil {
		log.Fatalf("Failed to p2p listen: %v", err)
	}
	privateKey, err := privKey(cfg.PrivateKey)
	if err != nil {
		log.Fatalf("Could not create private key %v", err)
	}
	options := []libp2p.Option{
		libp2p.ListenAddrs(listen),
		privKeyOption(privateKey),
	}
	if cfg.EnableUPnP {
		options = append(options, libp2p.NATPortMap()) //Allow to use UPnP
	}
	// add discv5 to list of protocols in libp2p.
	if err := addDiscv5protocol(); err != nil {
		log.Fatalf("Could not set add discv5 to libp2p protocols: %v", err)
	}
	return options, net.ParseIP(ip), privateKey
}

func privKey(prvKey string) (*ecdsa.PrivateKey, error) {
	if prvKey == "" {
		priv, _, err := crypto.GenerateSecp256k1Key(rand.Reader)
		if err != nil {
			return nil, err
		}
		convertedKey := convertFromInterface(priv)
		return convertedKey, nil
	}
	if _, err := os.Stat(prvKey); os.IsNotExist(err) {
		log.WithField("private key file", prvKey).Warn("Could not read private key, file is missing or unreadable")
		return nil, err
	}
	priv, err := curve.LoadECDSA(prvKey)
	if err != nil {
		log.WithError(err).Error("Error reading private key from file")
		return nil, err
	}
	return priv, nil
}

// Adds a private key to the libp2p option if the option was provided.
// If the private key file is missing or cannot be read, or if the
// private key contents cannot be marshaled, an exception is thrown.
func privKeyOption(privkey *ecdsa.PrivateKey) libp2p.Option {
	return func(cfg *libp2p.Config) error {
		convertedKey := convertToInterface(privkey)
		id, err := peer.IDFromPrivateKey(convertedKey)
		if err != nil {
			return err
		}
		log.WithField("peer id", id.Pretty()).Info("Private key generated. Announcing peer id")
		return cfg.Apply(libp2p.Identity(convertedKey))
	}
}

func convertFromInterface(privkey crypto.PrivKey) *ecdsa.PrivateKey {
	typeAssertedKey := (*ecdsa.PrivateKey)((*btcec.PrivateKey)(privkey.(*crypto.Secp256k1PrivateKey)))
	return typeAssertedKey
}

func convertToInterface(privkey *ecdsa.PrivateKey) crypto.PrivKey {
	typeAssertedKey := crypto.PrivKey((*crypto.Secp256k1PrivateKey)((*btcec.PrivateKey)(privkey)))
	return typeAssertedKey
}
