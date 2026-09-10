package lowlevel

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/h44z/wg-portal/internal/config"
)

func testClient(t *testing.T) *OpnsenseApiClient {
	t.Helper()
	client, err := NewOpnsenseApiClient(&config.Config{}, &config.BackendOpnsense{
		BackendBase: config.BackendBase{Id: "test"},
		ApiUrl:      "https://fw.example.org",
		ApiKey:      "key",
		ApiSecret:   "secret",
	})
	require.NoError(t, err)
	return client
}

func TestNewOpnsenseApiClientRequiresCredentials(t *testing.T) {
	tests := map[string]config.BackendOpnsense{
		"no url":    {ApiKey: "k", ApiSecret: "s"},
		"no key":    {ApiUrl: "https://fw", ApiSecret: "s"},
		"no secret": {ApiUrl: "https://fw", ApiKey: "k"},
	}
	for name, cfg := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := NewOpnsenseApiClient(&config.Config{}, &cfg)
			assert.Error(t, err)
		})
	}
}

func TestGetFullPath(t *testing.T) {
	client := testClient(t)

	tests := []struct {
		name    string
		command string
		want    string
	}{
		{"plain path", "/api/wireguard/server/searchServer",
			"https://fw.example.org/api/wireguard/server/searchServer"},
		{"query is preserved, not path-escaped", "/api/wireguard/server/searchServer?current=1&rowCount=-1",
			"https://fw.example.org/api/wireguard/server/searchServer?current=1&rowCount=-1"},
		{"uuid segment", "/api/wireguard/client/delClient/3a94f76f-67dd-4d65-8fb9-e7bae0fc0f65",
			"https://fw.example.org/api/wireguard/client/delClient/3a94f76f-67dd-4d65-8fb9-e7bae0fc0f65"},
		{"escaped traversal stays literal", "/api/wireguard/client/delClient/..%2F..%2Fcore",
			"https://fw.example.org/api/wireguard/client/delClient/..%2F..%2Fcore"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := client.getFullPath(tt.command)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// url.JoinPath resolves ".." segments, which would let a value interpolated
// into the command -- a UUID read from a firewall response -- redirect an
// authenticated POST to an unrelated endpoint. Callers escape those values, but
// the client refuses traversal as well so one missed call site cannot do it.
func TestGetFullPathRefusesTraversal(t *testing.T) {
	client := testClient(t)

	for _, command := range []string{
		"/api/wireguard/client/delClient/../../core/firmware/poweroff",
		"/api/wireguard/client/delClient/x/../../../core/firmware/poweroff",
		"/api/wireguard/../core/firmware/poweroff",
	} {
		got, err := client.getFullPath(command)
		require.Error(t, err, "command %q must be refused", command)
		assert.Empty(t, got)
		assert.Contains(t, err.Error(), "traversal")
	}
}

func TestRedactOpnsenseSecrets(t *testing.T) {
	const priv = "GG9t5/5ancs3jARn0ZE2gfy/Dg0vxOmKVIGNhuYVlls="
	const psk = "CuuWzzwdLP4zfNxQnzZ0CVpKHR1K+4WKexVgPggZ5gg="

	rendered := redactOpnsenseSecrets(GenericJsonObject{
		"server": GenericJsonObject{
			"name":    "wg0",
			"privkey": priv,
			"pubkey":  "LWd/ZsnkjoT+YMHNgi6Qb66iGD4quml51734Mv1A40U=",
		},
	})
	assert.NotContains(t, rendered, priv, "the private key must not reach the log")
	assert.Contains(t, rendered, "REDACTED")
	assert.Contains(t, rendered, "wg0", "non-secret fields stay visible for debugging")
	assert.Contains(t, rendered, "privkey", "the field name stays visible")

	rendered = redactOpnsenseSecrets(GenericJsonObject{
		"client": GenericJsonObject{"name": "peer", "psk": psk},
	})
	assert.NotContains(t, rendered, psk)
	assert.Contains(t, rendered, "REDACTED")

	// An empty secret is not worth redacting; keep the payload readable.
	rendered = redactOpnsenseSecrets(GenericJsonObject{
		"client": GenericJsonObject{"name": "peer", "psk": ""},
	})
	assert.NotContains(t, rendered, "REDACTED")
}

func TestFlattenForWrite(t *testing.T) {
	// The read form as getServer returns it.
	read := GenericJsonObject{
		"enabled": "1",
		"name":    "wg0",
		"tunneladdress": map[string]any{
			"10.99.0.1/24": map[string]any{"value": "10.99.0.1/24", "selected": float64(1)},
		},
		"dns": map[string]any{
			"": map[string]any{"value": "", "selected": float64(1)},
		},
		"peers": map[string]any{
			"bbb": map[string]any{"value": "b", "selected": float64(1)},
			"aaa": map[string]any{"value": "a", "selected": float64(1)},
			"ccc": map[string]any{"value": "c", "selected": float64(0)},
		},
	}

	got := FlattenForWrite(read)

	assert.Equal(t, "1", got["enabled"], "scalars pass through untouched")
	assert.Equal(t, "wg0", got["name"])
	assert.Equal(t, "10.99.0.1/24", got["tunneladdress"], "a select map collapses to its selected key")
	assert.Equal(t, "", got["dns"], `the "" placeholder key is not a real value`)
	assert.Equal(t, "aaa,bbb", got["peers"],
		"only selected keys, joined in sorted order so writes are deterministic")
}

func TestIsSelectedAcceptsEveryShape(t *testing.T) {
	// The flag has been observed as a JSON number, a bool and a quoted string.
	for _, truthy := range []any{true, float64(1), 1, "1", "true", "True"} {
		assert.True(t, isSelected(truthy), "%v (%T) should be selected", truthy, truthy)
	}
	for _, falsy := range []any{false, float64(0), 0, "0", "", "no", nil} {
		assert.False(t, isSelected(falsy), "%v (%T) should not be selected", falsy, falsy)
	}
}

func TestSelectedKeysAndValue(t *testing.T) {
	obj := GenericJsonObject{
		"peers": map[string]any{
			"b": map[string]any{"selected": float64(1)},
			"a": map[string]any{"selected": float64(1)},
			"c": map[string]any{"selected": float64(0)},
		},
		"scalar": "not-a-select-map",
	}

	assert.Equal(t, []string{"a", "b"}, SelectedKeys(obj, "peers"))
	assert.Equal(t, "a", SelectedValue(obj, "peers"))
	assert.Nil(t, SelectedKeys(obj, "scalar"), "a scalar is not a select map")
	assert.Nil(t, SelectedKeys(obj, "absent"))
	assert.Empty(t, SelectedValue(obj, "absent"))
}

func TestCheckMutationResult(t *testing.T) {
	ok := func(body GenericJsonObject) OpnsenseApiResponse[GenericJsonObject] {
		return OpnsenseApiResponse[GenericJsonObject]{Status: OpnsenseApiStatusOk, Code: 200, Data: body}
	}

	// OPNsense reports validation failures with HTTP 200, so the body decides.
	for _, good := range []string{"saved", "deleted", "ok", ""} {
		got := checkMutationResult(ok(GenericJsonObject{"result": good}))
		assert.Equal(t, OpnsenseApiStatusOk, got.Status, "result %q should be a success", good)
	}

	failed := checkMutationResult(ok(GenericJsonObject{
		"result":      "failed",
		"validations": map[string]any{"client.name": "Should be a string between 1 and 64 characters."},
	}))
	require.Equal(t, OpnsenseApiStatusError, failed.Status)
	require.NotNil(t, failed.Error)
	assert.Contains(t, failed.Error.Details, "client.name",
		"the validation detail is what makes the failure actionable")

	// A response with no body at all must not be treated as a failure.
	assert.Equal(t, OpnsenseApiStatusOk,
		checkMutationResult(OpnsenseApiResponse[GenericJsonObject]{Status: OpnsenseApiStatusOk}).Status)
}

func TestOpnsenseApiErrorStringHandlesNil(t *testing.T) {
	var err *OpnsenseApiError
	assert.Equal(t, "no error", err.String())
	assert.True(t, strings.Contains((&OpnsenseApiError{Code: 1, Message: "m", Details: "d"}).String(), "m"))
}
