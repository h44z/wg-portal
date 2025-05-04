package domain

import (
	"testing"
	"time"
)

func TestPeerStatus_IsConnected(t *testing.T) {
	now := time.Now()
	past := now.Add(-3 * time.Minute)
	recent := now.Add(-1 * time.Minute)

	tests := []struct {
		name   string
		status PeerStatus
		want   bool
	}{
		{
			name: "Pingable and recent handshake",
			status: PeerStatus{
				IsPingable:    true,
				LastHandshake: &recent,
			},
			want: true,
		},
		{
			name: "Not pingable but recent handshake",
			status: PeerStatus{
				IsPingable:    false,
				LastHandshake: &recent,
			},
			want: true,
		},
		{
			name: "Pingable but old handshake",
			status: PeerStatus{
				IsPingable:    true,
				LastHandshake: &past,
			},
			want: true,
		},
		{
			name: "Not pingable and old handshake",
			status: PeerStatus{
				IsPingable:    false,
				LastHandshake: &past,
			},
			want: false,
		},
		{
			name: "Pingable and no handshake",
			status: PeerStatus{
				IsPingable:    true,
				LastHandshake: nil,
			},
			want: true,
		},
		{
			name: "Not pingable and no handshake",
			status: PeerStatus{
				IsPingable:    false,
				LastHandshake: nil,
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.status.IsConnected(); got != tt.want {
				t.Errorf("IsConnected() = %v, want %v", got, tt.want)
			}
		})
	}
}
