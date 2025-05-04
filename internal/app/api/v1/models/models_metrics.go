package models

import (
	"time"

	"github.com/h44z/wg-portal/internal/domain"
)

// PeerMetrics represents the metrics of a WireGuard peer.
type PeerMetrics struct {
	// The unique identifier of the peer.
	PeerIdentifier string `json:"PeerIdentifier" example:"xTIBA5rboUvnH4htodjb6e697QjLERt1NAB4mZqp8Dg="`

	// If this field is set, the peer is pingable.
	IsPingable bool `json:"IsPingable" example:"true"`
	// The last time the peer responded to a ICMP ping request.
	LastPing *time.Time `json:"LastPing" example:"2021-01-01T12:00:00Z"`

	// The number of bytes received by the peer.
	BytesReceived uint64 `json:"BytesReceived" example:"123456789"`
	// The number of bytes transmitted by the peer.
	BytesTransmitted uint64 `json:"BytesTransmitted" example:"123456789"`

	// The last time the peer initiated a handshake.
	LastHandshake *time.Time `json:"LastHandshake" example:"2021-01-01T12:00:00Z"`
	// The current endpoint address of the peer.
	Endpoint string `json:"Endpoint" example:"12.34.56.78"`
	// The last time the peer initiated a session.
	LastSessionStart *time.Time `json:"LastSessionStart" example:"2021-01-01T12:00:00Z"`
}

func NewPeerMetrics(src *domain.PeerStatus) *PeerMetrics {
	return &PeerMetrics{
		PeerIdentifier:   string(src.PeerId),
		IsPingable:       src.IsPingable,
		LastPing:         src.LastPing,
		BytesReceived:    src.BytesReceived,
		BytesTransmitted: src.BytesTransmitted,
		LastHandshake:    src.LastHandshake,
		Endpoint:         src.Endpoint,
		LastSessionStart: src.LastSessionStart,
	}
}

// InterfaceMetrics represents the metrics of a WireGuard interface.
type InterfaceMetrics struct {
	// The unique identifier of the interface.
	InterfaceIdentifier string `json:"InterfaceIdentifier" example:"wg0"`

	// The number of bytes received by the interface.
	BytesReceived uint64 `json:"BytesReceived" example:"123456789"`
	// The number of bytes transmitted by the interface.
	BytesTransmitted uint64 `json:"BytesTransmitted" example:"123456789"`
}

func NewInterfaceMetrics(src *domain.InterfaceStatus) *InterfaceMetrics {
	return &InterfaceMetrics{
		InterfaceIdentifier: string(src.InterfaceId),
		BytesReceived:       src.BytesReceived,
		BytesTransmitted:    src.BytesTransmitted,
	}
}

// UserMetrics represents the metrics of a WireGuard user.
type UserMetrics struct {
	// The unique identifier of the user.
	UserIdentifier string `json:"UserIdentifier" example:"uid-1234567"`

	// PeerCount represents the number of peers linked to the user.
	PeerCount int `json:"PeerCount" example:"2"`

	// The total number of bytes received by the user. This is the sum of all bytes received by the peers linked to the user.
	BytesReceived uint64 `json:"BytesReceived" example:"123456789"`
	// The total number of bytes transmitted by the user. This is the sum of all bytes transmitted by the peers linked to the user.
	BytesTransmitted uint64 `json:"BytesTransmitted" example:"123456789"`

	// PeerMetrics represents the metrics of the peers linked to the user.
	PeerMetrics []PeerMetrics `json:"PeerMetrics"`
}

func NewUserMetrics(srcUser *domain.User, src []domain.PeerStatus) *UserMetrics {
	if srcUser == nil {
		return nil
	}

	um := &UserMetrics{
		UserIdentifier: string(srcUser.Identifier),
		PeerCount:      srcUser.LinkedPeerCount,
		PeerMetrics:    []PeerMetrics{},

		BytesReceived:    0,
		BytesTransmitted: 0,
	}

	peerMetrics := make([]PeerMetrics, len(src))
	for i, peer := range src {
		peerMetrics[i] = *NewPeerMetrics(&peer)

		um.BytesReceived += peer.BytesReceived
		um.BytesTransmitted += peer.BytesTransmitted
	}
	um.PeerMetrics = peerMetrics

	return um
}
