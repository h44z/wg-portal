package domain

import "time"

type PeerStatus struct {
	PeerId    PeerIdentifier `gorm:"primaryKey;column:identifier" json:"PeerId"`
	UpdatedAt time.Time      `gorm:"column:updated_at" json:"-"`

	IsConnected bool `gorm:"column:connected" json:"IsConnected"` // indicates if the peer is connected based on the last handshake or ping

	IsPingable bool       `gorm:"column:pingable" json:"IsPingable"`
	LastPing   *time.Time `gorm:"column:last_ping" json:"LastPing"`

	BytesReceived    uint64 `gorm:"column:received" json:"BytesReceived"`
	BytesTransmitted uint64 `gorm:"column:transmitted" json:"BytesTransmitted"`

	LastHandshake    *time.Time `gorm:"column:last_handshake" json:"LastHandshake"`
	Endpoint         string     `gorm:"column:endpoint" json:"Endpoint"`
	LastSessionStart *time.Time `gorm:"column:last_session_start" json:"LastSessionStart"`
}

func (s *PeerStatus) CalcConnected() {
	oldestHandshakeTime := time.Now().Add(-2 * time.Minute) // if a handshake is older than 2 minutes, the peer is no longer connected

	handshakeValid := false
	if s.LastHandshake != nil {
		handshakeValid = !s.LastHandshake.Before(oldestHandshakeTime)
	}

	s.IsConnected = s.IsPingable || handshakeValid
}

type InterfaceStatus struct {
	InterfaceId InterfaceIdentifier `gorm:"primaryKey;column:identifier"`
	UpdatedAt   time.Time           `gorm:"column:updated_at"`

	BytesReceived    uint64 `gorm:"column:received"`
	BytesTransmitted uint64 `gorm:"column:transmitted"`
}
