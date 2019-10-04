package iputils_test

import (
	"regexp"
	"testing"

	"github.com/prysmaticlabs/prysm/shared/iputils"
)

func TestExternalIPv4(t *testing.T) {
	// Regular expression format for IPv4
	IPv4Format := `\.\d{1,3}\.\d{1,3}\b`
	test, err := iputils.ExternalIPv4()

	if err != nil {
		t.Errorf("Test check external ipv4 failed with %v", err)
	}

	valid := regexp.MustCompile(IPv4Format)

	if !valid.MatchString(test) {
		t.Errorf("Wanted: %v, got: %v", IPv4Format, test)
	}
}
