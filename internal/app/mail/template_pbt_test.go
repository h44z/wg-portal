package mail

// Feature: peer-rotation-interval, Property 9: rendered email contains required content

import (
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"pgregory.net/rapid"

	"github.com/h44z/wg-portal/internal/domain"
)

// TestProperty9_ExpiryNotificationMail_ContainsRequiredContent verifies that
// for any peer with a known ExpiresAt and a linked user, the rendered expiry
// notification email (both text and HTML) contains:
//   - the peer's display name
//   - the expiry date formatted as "2006-01-02 15:04:05 UTC"
//   - the number of days remaining (as a string)
//   - the portal URL
//
// Minimum 100 iterations (rapid default is 100).
func TestProperty9_ExpiryNotificationMail_ContainsRequiredContent(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate a random portal URL and portal name.
		portalURL := "https://" + rapid.StringMatching(`[a-z]{4,10}\.[a-z]{2,4}`).Draw(t, "portalDomain")
		portalName := rapid.StringMatching(`[A-Za-z]{4,12}`).Draw(t, "portalName")

		// Generate a random peer display name (non-empty, printable ASCII).
		displayName := rapid.StringMatching(`[A-Za-z0-9 _-]{3,20}`).Draw(t, "displayName")

		// Generate a random ExpiresAt between 1 and 365 days in the future.
		daysAhead := rapid.IntRange(1, 365).Draw(t, "daysAhead")
		expiresAt := time.Now().UTC().Add(time.Duration(daysAhead) * 24 * time.Hour)

		// Generate a random daysLeft value (1–365).
		daysLeft := rapid.IntRange(1, 365).Draw(t, "daysLeft")

		// Build a minimal peer.
		peer := &domain.Peer{
			Identifier:  domain.PeerIdentifier("peer-" + displayName),
			DisplayName: displayName,
		}

		// Build a minimal user.
		user := &domain.User{
			Identifier: domain.UserIdentifier("user-test"),
			Email:      "test@example.com",
		}

		// Create a TemplateHandler using embedded templates (empty basePath).
		handler, err := newTemplateHandler(portalURL, portalName, "")
		if err != nil {
			t.Fatalf("failed to create TemplateHandler: %v", err)
		}

		// Render both templates.
		txtReader, htmlReader, err := handler.GetExpiryNotificationMail(user, peer, expiresAt, daysLeft, false)
		if err != nil {
			t.Fatalf("GetExpiryNotificationMail returned error: %v", err)
		}

		txtBytes, err := io.ReadAll(txtReader)
		if err != nil {
			t.Fatalf("failed to read text output: %v", err)
		}
		htmlBytes, err := io.ReadAll(htmlReader)
		if err != nil {
			t.Fatalf("failed to read HTML output: %v", err)
		}

		txtOutput := string(txtBytes)
		htmlOutput := string(htmlBytes)

		// The date format used in the templates.
		expectedDate := expiresAt.UTC().Format("2006-01-02 15:04:05 UTC")
		expectedDays := fmt.Sprintf("%d", daysLeft)

		// Assert text template contains all required fields.
		assertContains(t, "text", txtOutput, "peer display name", displayName)
		assertContains(t, "text", txtOutput, "expiry date", expectedDate)
		assertContains(t, "text", txtOutput, "days remaining", expectedDays)
		assertContains(t, "text", txtOutput, "portal URL", portalURL)

		// Assert HTML template contains all required fields.
		assertContains(t, "HTML", htmlOutput, "peer display name", displayName)
		assertContains(t, "HTML", htmlOutput, "expiry date", expectedDate)
		assertContains(t, "HTML", htmlOutput, "days remaining", expectedDays)
		assertContains(t, "HTML", htmlOutput, "portal URL", portalURL)
	})
}

// assertContains is a helper that fails the test if output does not contain the expected substring.
func assertContains(t *rapid.T, templateType, output, fieldName, expected string) {
	t.Helper()
	if !strings.Contains(output, expected) {
		t.Fatalf("Property 9: %s template missing %s: expected to find %q in output:\n%s",
			templateType, fieldName, expected, output)
	}
}
