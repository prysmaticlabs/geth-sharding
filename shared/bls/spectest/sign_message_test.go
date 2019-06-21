package spectest

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"testing"

	"github.com/ghodss/yaml"
	"github.com/prysmaticlabs/prysm/shared/bls"
)

// Note about yaml formatting: The domain value is written upstream as
// hexadecimal integer. This is one case where we want to keep hexadecimal value
// in the yaml. If a tool was run to convert hexadecimal strings to Base64, the
// domain values need to be reverted to stay as hexadecimal strings with a
// struct field type of `uint64`.
func TestSignMessageYaml(t *testing.T) {
	file, err := ioutil.ReadFile("sign_msg_formatted.yaml")
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	test := &SignMessageTest{}
	if err := yaml.Unmarshal(file, test); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	for i, tt := range test.TestCases {
		t.Run(fmt.Sprintf("Test %d", i), func(t *testing.T) {
			sk, err := bls.SecretKeyFromBytes(tt.Input.Privkey)
			if err != nil {
				t.Fatalf("Cannot unmarshal input to secret key: %v", err)
			}

			sig := sk.Sign(tt.Input.Message, tt.Input.Domain)
			if !bytes.Equal(tt.Output, sig.Marshal()) {
				t.Logf("Domain=%d", tt.Input.Domain)
				t.Fatalf("Signature does not match the expected output. "+
					"Expected %#x but received %#x", tt.Output, sig.Marshal())
			}
			t.Logf("Success. Domain=%d", tt.Input.Domain)
		})
	}
}
