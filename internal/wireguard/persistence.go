package wireguard

import (
	"github.com/h44z/wg-portal/internal/persistence"
)

type store interface {
	GetAvailableInterfaces() ([]persistence.InterfaceIdentifier, error)

	GetAllInterfaces(interfaceIdentifiers ...persistence.InterfaceIdentifier) (map[persistence.InterfaceConfig][]persistence.PeerConfig, error)
	GetInterface(identifier persistence.InterfaceIdentifier) (persistence.InterfaceConfig, []persistence.PeerConfig, error)

	SaveInterface(cfg persistence.InterfaceConfig, peers []persistence.PeerConfig) error
	SavePeer(peer persistence.PeerConfig, interfaceIdentifier persistence.InterfaceIdentifier) error

	DeleteInterface(identifier persistence.InterfaceIdentifier) error
	DeletePeer(peer persistence.PeerIdentifier, interfaceIdentifier persistence.InterfaceIdentifier) error
}
