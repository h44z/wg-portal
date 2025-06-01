package domain

import (
	"time"
)

type PeerStatus struct {
	PeerId    PeerIdentifier `gorm:"primaryKey;column:identifier"`
	UpdatedAt time.Time      `gorm:"column:updated_at"`

	IsPingable bool       `gorm:"column:pingable"`
	LastPing   *time.Time `gorm:"column:last_ping"`

	BytesReceived    uint64 `gorm:"column:received"`
	BytesTransmitted uint64 `gorm:"column:transmitted"`

	LastHandshake    *time.Time `gorm:"column:last_handshake"`
	Endpoint         string     `gorm:"column:endpoint"`
	LastSessionStart *time.Time `gorm:"column:last_session_start"`
}

func (s PeerStatus) IsConnected() bool {
	oldestHandshakeTime := time.Now().Add(-2 * time.Minute) // if a handshake is older than 2 minutes, the peer is no longer connected

	handshakeValid := false
	if s.LastHandshake != nil {
		handshakeValid = !s.LastHandshake.Before(oldestHandshakeTime)
	}

	return s.IsPingable || handshakeValid
}

type InterfaceStatus struct {
	InterfaceId InterfaceIdentifier `gorm:"primaryKey;column:identifier"`
	UpdatedAt   time.Time           `gorm:"column:updated_at"`

	BytesReceived    uint64 `gorm:"column:received"`
	BytesTransmitted uint64 `gorm:"column:transmitted"`
}

type PingerResult struct {
	PacketsRecv int
	PacketsSent int
	Rtts        []time.Duration
}

func (r PingerResult) IsPingable() bool {
	return r.PacketsRecv > 0 && r.PacketsSent > 0 && len(r.Rtts) > 0
}

func (r PingerResult) AverageRtt() time.Duration {
	if len(r.Rtts) == 0 {
		return 0
	}

	var total time.Duration
	for _, rtt := range r.Rtts {
		total += rtt
	}
	return total / time.Duration(len(r.Rtts))
}
