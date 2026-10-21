package netutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseClientIP(t *testing.T) {
	cases := map[string]string{
		"192.168.1.5":          "192.168.1.5",
		"192.168.1.5:443":      "192.168.1.5",
		"::ffff:192.168.1.5":   "192.168.1.5",
		"[::ffff:10.0.0.1]:80": "10.0.0.1",
		"fe80::1%eth0":         "fe80::1",
		"[fd00::1]:8080":       "fd00::1",
		" 10.1.2.3 ":           "10.1.2.3",
	}
	for in, want := range cases {
		ip := ParseClientIP(in)
		if assert.NotNil(t, ip, in) {
			assert.Equal(t, want, ip.String(), in)
		}
	}
	for _, bad := range []string{"", "not-an-ip", "999.1.1.1", "192.168.1"} {
		assert.Nil(t, ParseClientIP(bad), bad)
	}
	// Mapped addresses must normalise to 4 bytes so IPv4 CIDRs match them.
	assert.Len(t, ParseClientIP("::ffff:192.168.1.5"), 4)
}

func TestHostURL(t *testing.T) {
	assert.Equal(t, "http://192.168.1.45", HostURL("http", "192.168.1.45", 80))
	assert.Equal(t, "http://192.168.1.45", HostURL("http", "192.168.1.45", 0))
	assert.Equal(t, "http://192.168.1.45:8080", HostURL("http", "192.168.1.45", 8080))
	assert.Equal(t, "http://[fd00::1]:8080", HostURL("http", "fd00::1", 8080))
	assert.Equal(t, "http://[fd00::1]", HostURL("http", "fd00::1", 80))
}

func TestHostURLHTTPS(t *testing.T) {
	assert.Equal(t, "https://192.168.1.45", HostURL("https", "192.168.1.45", 443))
	assert.Equal(t, "https://192.168.1.45:8443", HostURL("https", "192.168.1.45", 8443))
}
