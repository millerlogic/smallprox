package smallprox

import (
	"strings"
	"testing"
)

func TestLoadHosts(t *testing.T) {
	hosts, err := LoadHosts(loadHostsTest)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Hosts: %s", hosts)
	// Contains:
	for _, host := range []string{"foo", "Foo", "bar", "Bar", "localhost", "puter", "puter.lan"} {
		if !ContainsHost(hosts, host) {
			t.Errorf("Expected to find host %s", host)
		}
	}
	// Does not contain:
	for _, host := range []string{"foo.bar", "puter.lanX", "computer.lan",
		"::1", "1", "127.0.0.1", "127", "", "skip", "#", "#skip", "etc", "format"} {
		if ContainsHost(hosts, host) {
			t.Errorf("Expected NOT to find host %s", host)
		}
	}
}

var loadHostsTest = strings.NewReader(`
#skip
foo
Bar

# /etc/hosts format:
127.0.0.1       localhost
127.0.0.2       puter.lan puter
::1             localhost ip6-localhost ip6-loopback
fe00::0         ip6-localnet
`)
