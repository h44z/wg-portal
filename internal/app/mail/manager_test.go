package mail

import (
	"io"
	"strings"
	"testing"

	"github.com/h44z/wg-portal/internal/config"
	"github.com/h44z/wg-portal/internal/domain"
)

func Test_base64UrlEncode_isReversibleWithHandlerDecode(t *testing.T) {
	inputs := []string{
		"peer-identifier",
		"aGVsbG8=",    // ensure padding characters are handled
		"abc/def+ghi", // ensure + and / are handled
		"wgTestKey1234567890==",
	}

	for _, in := range inputs {
		encoded := domain.Base64UrlEncode(in)

		// The URL-safe variant must not contain characters that are unsafe in URLs.
		if strings.ContainsAny(encoded, "+/=") {
			t.Fatalf("encoded value %q still contains unsafe characters", encoded)
		}

		decoded := domain.Base64UrlDecode(encoded)
		if decoded != in {
			t.Fatalf("round trip failed: got %q, want %q (encoded: %q)", decoded, in, encoded)
		}
	}
}

func Test_getPeerConfigDownloadLink(t *testing.T) {
	tests := []struct {
		name        string
		externalUrl string
		basePath    string
		peerId      domain.PeerIdentifier
		style       string
		want        string
	}{
		{
			name:        "no base path",
			externalUrl: "https://wg.example.com",
			basePath:    "",
			peerId:      "peer1",
			style:       "wgquick",
			want:        "https://wg.example.com/app/#/peer/config/" + domain.Base64UrlEncode("peer1") + "?style=wgquick",
		},
		{
			name:        "with base path",
			externalUrl: "https://wg.example.com",
			basePath:    "/wg",
			peerId:      "peer1",
			style:       "",
			want:        "https://wg.example.com/wg/app/#/peer/config/" + domain.Base64UrlEncode("peer1"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{}
			cfg.Web.ExternalUrl = tt.externalUrl
			cfg.Web.BasePath = tt.basePath
			m := Manager{cfg: cfg}

			got := m.getPeerConfigDownloadLink(tt.peerId, tt.style)
			if got != tt.want {
				t.Fatalf("getPeerConfigDownloadLink() = %q, want %q", got, tt.want)
			}
		})
	}
}

func Test_GetConfigMail_containsLink(t *testing.T) {
	handler, err := newTemplateHandler("https://wg.example.com", "WireGuard Portal", "")
	if err != nil {
		t.Fatalf("failed to create template handler: %v", err)
	}

	link := "https://wg.example.com/app/#/peer/config/abc?style=wgquick"
	txtReader, htmlReader, err := handler.GetConfigMail(&domain.User{Firstname: "John", Lastname: "Doe"}, link)
	if err != nil {
		t.Fatalf("failed to render link mail: %v", err)
	}

	txt, _ := io.ReadAll(txtReader)
	html, _ := io.ReadAll(htmlReader)

	if !strings.Contains(string(txt), link) {
		t.Errorf("text link mail does not contain the generated link.\n%s", string(txt))
	}
	if !strings.Contains(string(html), link) {
		t.Errorf("html link mail does not contain the generated link.\n%s", string(html))
	}

	// The link mail must not reference the placeholder that was used before the fix.
	if strings.Contains(string(txt), "deep link TBD") || strings.Contains(string(html), "deep link TBD") {
		t.Errorf("link mail still contains the placeholder link")
	}
}
