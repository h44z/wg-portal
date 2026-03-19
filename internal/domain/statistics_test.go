package domain

import (
	"testing"
	"time"
)

func TestPeerStatus_IsConnected(t *testing.T) {
	now := time.Now()
	past := now.Add(-3 * time.Minute)
	recent := now.Add(-1 * time.Minute)
	defaultTimeout := 125 * time.Second // rekey interval of 120s + 5 seconds grace period
	past126 := now.Add(-1*defaultTimeout - 1*time.Second)
	past125 := now.Add(-1 * defaultTimeout)
	past124 := now.Add(-1*defaultTimeout + 1*time.Second)

	tests := []struct {
		name    string
		status  PeerStatus
		timeout time.Duration
		want    bool
	}{
		{
			name: "Pingable and recent handshake",
			status: PeerStatus{
				IsPingable:    true,
				LastHandshake: &recent,
			},
			timeout: defaultTimeout,
			want:    true,
		},
		{
			name: "Not pingable but recent handshake",
			status: PeerStatus{
				IsPingable:    false,
				LastHandshake: &recent,
			},
			timeout: defaultTimeout,
			want:    true,
		},
		{
			name: "Pingable but old handshake",
			status: PeerStatus{
				IsPingable:    true,
				LastHandshake: &past,
			},
			timeout: defaultTimeout,
			want:    true,
		},
		{
			name: "Not pingable and ok handshake (-124s)",
			status: PeerStatus{
				IsPingable:    false,
				LastHandshake: &past124,
			},
			timeout: defaultTimeout,
			want:    true,
		},
		{
			name: "Not pingable and old handshake (-125s)",
			status: PeerStatus{
				IsPingable:    false,
				LastHandshake: &past125,
			},
			timeout: defaultTimeout,
			want:    false,
		},
		{
			name: "Not pingable and old handshake (-126s)",
			status: PeerStatus{
				IsPingable:    false,
				LastHandshake: &past126,
			},
			timeout: defaultTimeout,
			want:    false,
		},
		{
			name: "Not pingable and old handshake (very old)",
			status: PeerStatus{
				IsPingable:    false,
				LastHandshake: &past,
			},
			timeout: defaultTimeout,
			want:    false,
		},
		{
			name: "Pingable and no handshake",
			status: PeerStatus{
				IsPingable:    true,
				LastHandshake: nil,
			},
			timeout: defaultTimeout,
			want:    true,
		},
		{
			name: "Not pingable and no handshake",
			status: PeerStatus{
				IsPingable:    false,
				LastHandshake: nil,
			},
			timeout: defaultTimeout,
			want:    false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.status.CalcConnected(tt.timeout)
			if got := tt.status.IsConnected; got != tt.want {
				t.Errorf("IsConnected = %v, want %v", got, tt.want)
			}
		})
	}
}
