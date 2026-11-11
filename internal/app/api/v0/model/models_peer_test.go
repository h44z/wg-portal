package model

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/h44z/wg-portal/internal/domain"
)

// The value returned by the API must be accepted again unchanged (saving a peer without touching the expiry date).
func TestPeer_ExpiresAt_RFC3339RoundTrip(t *testing.T) {
	expires := time.Date(2026, 4, 6, 15, 4, 5, 0, time.UTC)

	out, err := json.Marshal(NewPeer(&domain.Peer{ExpiresAt: &expires}))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var raw map[string]any
	if err := json.Unmarshal(out, &raw); err != nil {
		t.Fatalf("unmarshal raw: %v", err)
	}
	if got, want := raw["ExpiresAt"], "2026-04-06T15:04:05Z"; got != want {
		t.Fatalf("ExpiresAt = %v, want %v", got, want)
	}

	var in Peer
	if err := json.Unmarshal(out, &in); err != nil {
		t.Fatalf("unmarshal peer: %v", err)
	}
	dp := NewDomainPeer(&in)
	if dp.ExpiresAt == nil || !dp.ExpiresAt.Equal(expires) {
		t.Fatalf("domain ExpiresAt = %v, want %v", dp.ExpiresAt, expires)
	}
}

func TestExpiryDate_UnmarshalJSON(t *testing.T) {
	tests := map[string]struct {
		input   string
		want    *time.Time
		wantErr bool
	}{
		"rfc3339 utc":       {input: `"2026-04-06T15:04:05Z"`, want: ptr(time.Date(2026, 4, 6, 15, 4, 5, 0, time.UTC))},
		"rfc3339 offset":    {input: `"2026-04-06T18:04:05+03:00"`, want: ptr(time.Date(2026, 4, 6, 15, 4, 5, 0, time.UTC))},
		"rfc3339 midnight":  {input: `"2026-04-06T00:00:00Z"`, want: ptr(time.Date(2026, 4, 6, 0, 0, 0, 0, time.UTC))},
		"fractional second": {input: `"2026-04-06T15:04:05.123Z"`, want: ptr(time.Date(2026, 4, 6, 15, 4, 5, 123000000, time.UTC))},
		"null":              {input: `null`},
		"empty string":      {input: `""`},
		"date only":         {input: `"2026-04-06"`, wantErr: true},
		"garbage":           {input: `"tomorrow"`, wantErr: true},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			var d ExpiryDate
			err := json.Unmarshal([]byte(tt.input), &d)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %v", d.Time)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			switch {
			case tt.want == nil && d.Time != nil:
				t.Fatalf("expected no expiry, got %v", d.Time)
			case tt.want != nil && (d.Time == nil || !d.Time.Equal(*tt.want)):
				t.Fatalf("got %v, want %v", d.Time, tt.want)
			}
		})
	}
}

func TestExpiryDate_MarshalJSON_Null(t *testing.T) {
	out, err := json.Marshal(NewPeer(&domain.Peer{}))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(out, &raw); err != nil {
		t.Fatalf("unmarshal raw: %v", err)
	}
	if v, ok := raw["ExpiresAt"]; !ok || v != nil {
		t.Fatalf("ExpiresAt = %v (present: %v), want null", v, ok)
	}
}

func ptr[T any](v T) *T { return &v }
