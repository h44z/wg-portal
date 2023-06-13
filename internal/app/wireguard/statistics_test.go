package wireguard

import (
	"github.com/h44z/wg-portal/internal/domain"
	"reflect"
	"testing"
	"time"
)

func Test_getSessionStartTime(t *testing.T) {
	now := time.Now()
	nowMinus1 := now.Add(-1 * time.Minute)
	nowMinus3 := now.Add(-3 * time.Minute)
	nowMinus5 := now.Add(-5 * time.Minute)

	type args struct {
		oldStats       domain.PeerStatus
		newReceived    uint64
		newTransmitted uint64
		lastHandshake  *time.Time
	}
	tests := []struct {
		name string
		args args
		want *time.Time
	}{
		{
			name: "not connected",
			args: args{
				newReceived:    0,
				newTransmitted: 0,
				lastHandshake:  nil,
			},
			want: nil,
		},
		{
			name: "freshly connected",
			args: args{
				oldStats:       domain.PeerStatus{LastSessionStart: &nowMinus1},
				newReceived:    100,
				newTransmitted: 100,
				lastHandshake:  &now,
			},
			want: &now,
		},
		{
			name: "freshly connected (no prev session)",
			args: args{
				oldStats:       domain.PeerStatus{LastSessionStart: nil},
				newReceived:    100,
				newTransmitted: 100,
				lastHandshake:  &now,
			},
			want: &now,
		},
		{
			name: "still connected",
			args: args{
				oldStats:       domain.PeerStatus{LastSessionStart: &nowMinus1, BytesReceived: 10, BytesTransmitted: 10},
				newReceived:    100,
				newTransmitted: 100,
				lastHandshake:  &now,
			},
			want: &nowMinus1,
		},
		{
			name: "no longer connected",
			args: args{
				oldStats:       domain.PeerStatus{LastSessionStart: &nowMinus5, BytesReceived: 100, BytesTransmitted: 100},
				newReceived:    100,
				newTransmitted: 100,
				lastHandshake:  &nowMinus3,
			},
			want: &nowMinus5,
		},
		{
			name: "reconnect (recv, hs outdated)",
			args: args{
				oldStats:       domain.PeerStatus{LastHandshake: &nowMinus5, BytesReceived: 100, BytesTransmitted: 100},
				newReceived:    10,
				newTransmitted: 100,
				lastHandshake:  &nowMinus1,
			},
			want: &nowMinus1,
		},
		{
			name: "reconnect (recv)",
			args: args{
				oldStats:       domain.PeerStatus{LastHandshake: &nowMinus1, BytesReceived: 100, BytesTransmitted: 100},
				newReceived:    10,
				newTransmitted: 100,
				lastHandshake:  &now,
			},
			want: &now,
		},
		{
			name: "reconnect (sent, hs outdated)",
			args: args{
				oldStats:       domain.PeerStatus{LastHandshake: &nowMinus5, BytesReceived: 100, BytesTransmitted: 100},
				newReceived:    100,
				newTransmitted: 10,
				lastHandshake:  &nowMinus1,
			},
			want: &nowMinus1,
		},
		{
			name: "reconnect (sent)",
			args: args{
				oldStats:       domain.PeerStatus{LastSessionStart: &nowMinus1, BytesReceived: 100, BytesTransmitted: 100},
				newReceived:    100,
				newTransmitted: 10,
				lastHandshake:  &now,
			},
			want: &now,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getSessionStartTime(tt.args.oldStats, tt.args.newReceived, tt.args.newTransmitted, tt.args.lastHandshake); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getSessionStartTime() = %v, want %v", got, tt.want)
			}
		})
	}
}
