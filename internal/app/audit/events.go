package audit

import "github.com/fedor-git/wg-portal-2/internal/domain"

type AuthEvent struct {
	Username string
	Error    string
}

type InterfaceEvent struct {
	Interface domain.Interface
	Action    string
}

type PeerEvent struct {
	Peer   domain.Peer
	Action string
}
