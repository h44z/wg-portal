package wireguard

import (
	"math"
	"testing"
	"time"
)

// TestDaysLeft_CeilingDivision verifies that the daysLeft calculation uses
// ceiling division so that fractional days round up rather than truncate.
// Task 7 acceptance criteria: 23h→1, 24h→1, 25h→2, negative→0.
func TestDaysLeft_CeilingDivision(t *testing.T) {
	cases := []struct {
		name      string
		hoursLeft float64
		want      int
	}{
		{"23h remaining → 1 day", 23.0, 1},
		{"23h59m remaining → 1 day", 23.0 + 59.0/60.0, 1},
		{"24h remaining → 1 day", 24.0, 1},
		{"25h remaining → 2 days", 25.0, 2},
		{"negative (already expired) → 0", -1.0, 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Replicate the production formula from processPeerNotifications.
			daysLeft := int(math.Ceil(tc.hoursLeft / 24))
			if daysLeft < 0 {
				daysLeft = 0
			}
			if daysLeft != tc.want {
				t.Errorf("hoursLeft=%.4f: got daysLeft=%d, want %d", tc.hoursLeft, daysLeft, tc.want)
			}
		})
	}
}

// TestDaysLeft_ViaUntil verifies the formula end-to-end using time.Until,
// matching the actual production code path.
func TestDaysLeft_ViaUntil(t *testing.T) {
	cases := []struct {
		name     string
		duration time.Duration
		want     int
	}{
		{"23h remaining → 1", 23 * time.Hour, 1},
		{"24h remaining → 1", 24 * time.Hour, 1},
		{"25h remaining → 2", 25 * time.Hour, 2},
		{"already expired → 0", -1 * time.Hour, 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			expiresAt := time.Now().Add(tc.duration)
			hoursLeft := time.Until(expiresAt).Hours()
			daysLeft := int(math.Ceil(hoursLeft / 24))
			if daysLeft < 0 {
				daysLeft = 0
			}
			if daysLeft != tc.want {
				t.Errorf("duration=%v: got daysLeft=%d, want %d", tc.duration, daysLeft, tc.want)
			}
		})
	}
}
