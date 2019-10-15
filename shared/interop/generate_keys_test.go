package interop_test

import (
	"bytes"
	"io/ioutil"
	"testing"

	"github.com/bazelbuild/rules_go/go/tools/bazel"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/go-yaml/yaml"
	"github.com/prysmaticlabs/prysm/shared/bls"
	"github.com/prysmaticlabs/prysm/shared/interop"
	"github.com/prysmaticlabs/prysm/shared/mputil"
)

type TestCase struct {
	Privkey string `yaml:"privkey"`
}

type KeyTest struct {
	TestCases []*TestCase `yaml:"test_cases"`
}

func TestKeyGenerator(t *testing.T) {
	path, err := bazel.Runfile("keygen_test_vector.yaml")
	if err != nil {
		t.Fatal(err)
	}
	file, err := ioutil.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	testCases := &KeyTest{}
	if err := yaml.Unmarshal(file, testCases); err != nil {
		t.Fatal(err)
	}
	priv, _, err := interop.DeterministicallyGenerateKeys(0, 1000)
	if err != nil {
		t.Error(err)
	}
	// cross-check with the first 1000 keys generated from the python spec
	for i, key := range priv {
		hexKey := testCases.TestCases[i].Privkey
		nKey, err := hexutil.Decode("0x" + hexKey)
		if err != nil {
			t.Error(err)
			continue
		}
		if !bytes.Equal(key.Marshal(), nKey) {
			t.Errorf("key for index %d failed, wanted %v but got %v", i, nKey, key.Marshal())
		}
	}
}

func BenchmarkKeyGenerator(b *testing.B) {
	for i := 0; i < b.N; i++ {
		if _, _, err := interop.DeterministicallyGenerateKeys(0, 16384); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkKeyGeneratorScatter(b *testing.B) {
	type keys struct {
		secrets []*bls.SecretKey
		publics []*bls.PublicKey
	}

	for i := 0; i < b.N; i++ {
		workers, resultsCh, _, err := mputil.Scatter(16384, func(offset int, entries int) (*mputil.ScatterResults, error) {
			priv, pub, err := interop.DeterministicallyGenerateKeys(uint64(offset), uint64(entries))
			if err != nil {
				b.Fatal(err)
			}

			return mputil.NewScatterResults(offset, &keys{secrets: priv, publics: pub}), nil
		})
		if err != nil {
			b.Fatalf("Scatter failed: %v", err)
		}

		privKeys := make([]*bls.SecretKey, 16384)
		pubKeys := make([]*bls.PublicKey, 16384)
		for i := workers; i > 0; i-- {
			result := <-resultsCh
			copy(privKeys[result.Offset:], result.Extent.(*keys).secrets)
			copy(pubKeys[result.Offset:], result.Extent.(*keys).publics)
		}
	}
}
