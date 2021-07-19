// Used for converting keys.yaml files from eth2.0-pm for interop testing.
// See: https://github.com/ethereum/eth2.0-pm/tree/master/interop/mocked_start
//
// This code can be discarded after interop testing.
package main

import (
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"github.com/prysmaticlabs/prysm/tools/unencrypted-keys-gen/keygen"
	"gopkg.in/yaml.v2"
)

// KeyPair with hex encoded data.
type KeyPair struct {
	Priv string `yaml:"privkey"`
	Pub  string `yaml:"pubkey"`
}

// KeyPairs represent the data format in the upstream yaml.
type KeyPairs []KeyPair

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: convert-keys path/to/keys.yaml path/to/output.json")
		return
	}
	inFile := os.Args[1]

	in, err := ioutil.ReadFile(inFile)
	if err != nil {
		log.Fatalf("Failed to read file %s: %v", inFile, err)
	}
	data := make(KeyPairs, 0)
	if err := yaml.UnmarshalStrict(in, &data); err != nil {
		log.Fatalf("Failed to unmarshal yaml: %v", err)
	}

	out := &keygen.UnencryptedKeysContainer{}
	for _, key := range data {
		pk, err := hex.DecodeString(key.Priv[2:])
		if err != nil {
			log.Fatalf("Failed to decode hex string %s: %v", key.Priv, err)
		}

		out.Keys = append(out.Keys, &keygen.UnencryptedKeys{
			ValidatorKey:  pk,
			WithdrawalKey: pk,
		})
	}

	outFile, err := os.Create(os.Args[2])
	if err != nil {
		log.Fatalf("Failed to create file at %s: %v", os.Args[2], err)
	}
	cleanup := func() {
		if err := outFile.Close(); err != nil {
			panic(err)
		}
	}
	defer cleanup()
	if err := keygen.SaveUnencryptedKeysToFile(outFile, out); err != nil {
		// log.Fatalf will prevent defer from being called
		cleanup()
		log.Fatalf("Failed to save %v", err)
	}
	log.Printf("Wrote %s\n", os.Args[2])
}
