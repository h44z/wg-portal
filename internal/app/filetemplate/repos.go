package filetemplate

import (
	"context"
	"github.com/h44z/wg-portal/internal/domain"
)

type UserDatabaseRepo interface {
	GetUser(ctx context.Context, id domain.UserIdentifier) (*domain.User, error)
}

type WireguardDatabaseRepo interface {
	GetInterfaceAndPeers(ctx context.Context, id domain.InterfaceIdentifier) (*domain.Interface, []domain.Peer, error)
	GetPeer(ctx context.Context, id domain.PeerIdentifier) (*domain.Peer, error)
	GetInterface(ctx context.Context, id domain.InterfaceIdentifier) (*domain.Interface, error)
}
