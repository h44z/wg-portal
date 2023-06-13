package domain

import "time"

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

type InterfaceStatus struct {
	InterfaceId InterfaceIdentifier `gorm:"primaryKey;column:identifier"`
	UpdatedAt   time.Time           `gorm:"column:updated_at"`

	BytesReceived    uint64 `gorm:"column:received"`
	BytesTransmitted uint64 `gorm:"column:transmitted"`
}
