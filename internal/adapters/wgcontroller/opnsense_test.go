package wgcontroller

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/h44z/wg-portal/internal/domain"
)

// Enumeration must survive records this portal did not create. A tunnel with no
// address is valid in OPNsense, and CidrsFromString reports the empty string as
// an error -- if that propagated, one such tunnel would fail GetInterfaces,
// which the startup importer treats as fatal for every backend.
func TestParseCidrsTolerant(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  []string
	}{
		{"empty is not an error", "", nil},
		{"whitespace only", "   ", nil},
		{"single", "10.99.0.1/24", []string{"10.99.0.1/24"}},
		{"multiple", "10.99.0.1/24,10.99.1.1/24", []string{"10.99.0.1/24", "10.99.1.1/24"}},
		{"spaces around separators", " 10.99.0.1/24 , 10.99.1.1/24 ", []string{"10.99.0.1/24", "10.99.1.1/24"}},
		{"ipv6", "fd00::1/64", []string{"fd00::1/64"}},
		{"garbage is dropped, the rest survives", "garbage,10.99.0.1/24", []string{"10.99.0.1/24"}},
		{"all garbage yields nothing rather than an error", "garbage,nonsense", nil},
		{"trailing separator", "10.99.0.1/24,", []string{"10.99.0.1/24"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseCidrsTolerant(tt.value, "test", "owner")
			if tt.want == nil {
				assert.Empty(t, got)
				return
			}
			assert.Equal(t, tt.want, splitCidrs(got))
		})
	}
}

func splitCidrs(cidrs []domain.Cidr) []string {
	out := make([]string, 0, len(cidrs))
	for _, c := range cidrs {
		out = append(out, c.String())
	}
	return out
}

// An IPv6 endpoint must survive a read/write round-trip. Formatting it as
// "host:port" would produce "2001:db8::1:51820", which still parses as an IPv6
// address with the port silently absorbed into it.
func TestEndpointRoundTrip(t *testing.T) {
	tests := []struct {
		name     string
		host     string
		port     string
		endpoint string
	}{
		{"ipv4 with port", "203.0.113.5", "51820", "203.0.113.5:51820"},
		{"ipv6 with port", "2001:db8::1", "51820", "[2001:db8::1]:51820"},
		{"hostname with port", "vpn.example.org", "51820", "vpn.example.org:51820"},
		{"ipv4 no port", "203.0.113.5", "", "203.0.113.5"},
		{"ipv6 no port", "2001:db8::1", "", "2001:db8::1"},
		{"empty", "", "", ""},
		{"port without host is meaningless", "", "51820", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			joined := joinEndpoint(tt.host, tt.port)
			assert.Equal(t, tt.endpoint, joined, "join")

			if joined == "" {
				return
			}
			host, port := splitEndpoint(joined)
			assert.Equal(t, tt.host, host, "host survives the round-trip")
			assert.Equal(t, tt.port, port, "port survives the round-trip")
		})
	}
}

// splitEndpoint also has to cope with values it did not produce itself.
func TestSplitEndpointTolerance(t *testing.T) {
	host, port := splitEndpoint("  203.0.113.5:51820  ")
	assert.Equal(t, "203.0.113.5", host)
	assert.Equal(t, "51820", port)

	// An unbracketed IPv6 literal has no recoverable port.
	host, port = splitEndpoint("2001:db8::1")
	assert.Equal(t, "2001:db8::1", host)
	assert.Empty(t, port)

	host, port = splitEndpoint("")
	assert.Empty(t, host)
	assert.Empty(t, port)
}

// Optional numeric fields are sent as empty rather than omitted, so that
// clearing a value on the portal actually clears it on the firewall instead of
// leaving the previous value in place.
func TestOptionalPositiveInt(t *testing.T) {
	assert.Equal(t, "1420", optionalPositiveInt(1420))
	assert.Equal(t, "", optionalPositiveInt(0))
	assert.Equal(t, "", optionalPositiveInt(-1))
}

func TestInstanceForDeviceName(t *testing.T) {
	tests := map[string]string{
		"wg0":       "0",
		"wg1":       "1",
		"wg42":      "42",
		"wg":        "",
		"wgx":       "",
		"vpn-admin": "",
		"":          "",
		"WG0":       "",
		"wg 1":      "",
	}
	for id, want := range tests {
		assert.Equal(t, want, instanceForDeviceName(domain.InterfaceIdentifier(id)), "id %q", id)
	}
}

func TestDeviceNameForInstance(t *testing.T) {
	assert.Equal(t, "wg0", deviceNameForInstance("0"))
	assert.Equal(t, "wg7", deviceNameForInstance("7"))
	assert.Equal(t, "", deviceNameForInstance(""), "no instance means no derivable device name")
}

// OPNsense validates tunnel and peer names as 1-64 characters of alphanumerics,
// dash and underscore, and rejects anything else rather than coercing it.
func TestOpnsenseName(t *testing.T) {
	// A real WireGuard public key: base64, so it contains "+", "/" and "=".
	const publicKey = "cZRjnW9Si7uw6EMCLPjaYULJTz6PB93KAzvqaPDDZmA="

	tests := []struct {
		name      string
		preferred string
		fallback  string
		want      string
	}{
		{"plain name passes through", "laptop", publicKey, "laptop"},
		{"spaces become dashes", "bob staff laptop", publicKey, "bob-staff-laptop"},
		{"dash and underscore are kept", "bob_staff-laptop", publicKey, "bob_staff-laptop"},
		{"runs of bad characters collapse", "a  ///  b", publicKey, "a-b"},
		{"leading and trailing junk is trimmed", "  !laptop!  ", publicKey, "laptop"},
		{"empty preferred falls back", "", publicKey, "cZRjnW9Si7uw6EMCLPjaYULJTz6PB93KAzvqaPDDZmA"},
		{"unusable preferred falls back", "!!!", publicKey, "cZRjnW9Si7uw6EMCLPjaYULJTz6PB93KAzvqaPDDZmA"},
		{"both unusable yields a constant", "!!!", "///", "wg-portal"},
		{"unicode is folded out", "büro-läptop", publicKey, "b-ro-l-ptop"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, opnsenseName(tt.preferred, tt.fallback))
		})
	}
}

// Whatever goes in, the result must always satisfy OPNsense's validator.
func TestOpnsenseNameAlwaysValid(t *testing.T) {
	inputs := []string{
		"", "   ", "!!!", strings.Repeat("x", 200),
		strings.Repeat("a b/c+d=", 30),
		"cZRjnW9Si7uw6EMCLPjaYULJTz6PB93KAzvqaPDDZmA=",
		"日本語", "-leading", "trailing-", "--collapse--",
	}

	for _, in := range inputs {
		got := opnsenseName(in, "")
		assert.NotEmpty(t, got, "input %q produced an empty name", in)
		assert.LessOrEqual(t, len(got), 64, "input %q produced %d characters", in, len(got))
		for _, r := range got {
			valid := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
				(r >= '0' && r <= '9') || r == '_' || r == '-'
			assert.True(t, valid, "input %q produced disallowed character %q in %q", in, r, got)
		}
	}
}
