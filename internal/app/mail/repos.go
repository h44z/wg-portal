package mail

import (
	"context"
	"github.com/h44z/wg-portal/internal/domain"
	"io"
)

type Mailer interface {
	Send(ctx context.Context, subject, body string, to []string, options *domain.MailOptions) error
}

type ConfigFileManager interface {
	GetInterfaceConfig(ctx context.Context, id domain.InterfaceIdentifier) (io.Reader, error)
	GetPeerConfig(ctx context.Context, id domain.PeerIdentifier) (io.Reader, error)
	GetPeerConfigQrCode(ctx context.Context, id domain.PeerIdentifier) (io.Reader, error)
}

type UserDatabaseRepo interface {
	GetUser(ctx context.Context, id domain.UserIdentifier) (*domain.User, error)
}

type WireguardDatabaseRepo interface {
	GetInterfaceAndPeers(ctx context.Context, id domain.InterfaceIdentifier) (*domain.Interface, []domain.Peer, error)
	GetPeer(ctx context.Context, id domain.PeerIdentifier) (*domain.Peer, error)
	GetInterface(ctx context.Context, id domain.InterfaceIdentifier) (*domain.Interface, error)
}
