package models

import (
	"time"

	"github.com/h44z/wg-portal/internal/domain"
)

// PeerMetrics represents a peer metrics model for webhooks.
// For details about the fields, see the domain.PeerStatus and domain.Peer structs.
type PeerMetrics struct {
	Status PeerStatus `json:"Status"`
	Peer   Peer       `json:"Peer"`
}

// PeerStatus represents the status of a peer for webhooks.
// For details about the fields, see the domain.PeerStatus struct.
type PeerStatus struct {
	UpdatedAt time.Time `json:"UpdatedAt"`

	IsConnected bool `json:"IsConnected"`

	IsPingable bool       `json:"IsPingable"`
	LastPing   *time.Time `json:"LastPing,omitempty"`

	BytesReceived    uint64 `json:"BytesReceived"`
	BytesTransmitted uint64 `json:"BytesTransmitted"`

	Endpoint         string     `json:"Endpoint"`
	LastHandshake    *time.Time `json:"LastHandshake,omitempty"`
	LastSessionStart *time.Time `json:"LastSessionStart,omitempty"`
}

// NewPeerMetrics creates a new PeerMetrics model from the domain.PeerStatus and domain.Peer models.
func NewPeerMetrics(status domain.PeerStatus, peer domain.Peer) PeerMetrics {
	return PeerMetrics{
		Status: PeerStatus{
			UpdatedAt:        status.UpdatedAt,
			IsConnected:      status.IsConnected,
			IsPingable:       status.IsPingable,
			LastPing:         status.LastPing,
			BytesReceived:    status.BytesReceived,
			BytesTransmitted: status.BytesTransmitted,
			Endpoint:         status.Endpoint,
			LastHandshake:    status.LastHandshake,
			LastSessionStart: status.LastSessionStart,
		},
		Peer: NewPeer(peer),
	}
}
