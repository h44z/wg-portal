package route

import (
	"context"

	"github.com/h44z/wg-portal/internal/domain"
)

type InterfaceAndPeerDatabaseRepo interface {
	GetAllInterfaces(ctx context.Context) ([]domain.Interface, error)
	GetInterfacePeers(ctx context.Context, id domain.InterfaceIdentifier) ([]domain.Peer, error)
}
