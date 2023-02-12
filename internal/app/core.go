package app

import (
	"context"
	"flag"
	"github.com/h44z/wg-portal/internal/config"
	"github.com/h44z/wg-portal/internal/domain"
	evbus "github.com/vardius/message-bus"
	"gorm.io/gorm"
)

// region global-repositories

type wireGuardRepo interface {
	GetInterfaces(_ context.Context) ([]domain.PhysicalInterface, error)
	GetInterface(_ context.Context, id domain.InterfaceIdentifier) (*domain.PhysicalInterface, error)
	GetPeers(_ context.Context, deviceId domain.InterfaceIdentifier) ([]domain.PhysicalPeer, error)
	GetPeer(_ context.Context, deviceId domain.InterfaceIdentifier, id domain.PeerIdentifier) (*domain.PhysicalPeer, error)
	SaveInterface(_ context.Context, id domain.InterfaceIdentifier, updateFunc func(pi *domain.PhysicalInterface) (*domain.PhysicalInterface, error)) error
	DeleteInterface(_ context.Context, id domain.InterfaceIdentifier) error
	SavePeer(_ context.Context, deviceId domain.InterfaceIdentifier, id domain.PeerIdentifier, updateFunc func(pp *domain.PhysicalPeer) (*domain.PhysicalPeer, error)) error
	DeletePeer(_ context.Context, deviceId domain.InterfaceIdentifier, id domain.PeerIdentifier) error
}

type dbRepo interface {
	GetInterface(ctx context.Context, id domain.InterfaceIdentifier) (*domain.Interface, error)
	GetInterfaceAndPeers(ctx context.Context, id domain.InterfaceIdentifier) (*domain.Interface, []domain.Peer, error)
	GetAllInterfaces(ctx context.Context) ([]domain.Interface, error)
	FindInterfaces(ctx context.Context, search string) ([]domain.Interface, error)
	SaveInterface(ctx context.Context, id domain.InterfaceIdentifier, updateFunc func(in *domain.Interface) (*domain.Interface, error)) error
	DeleteInterface(ctx context.Context, id domain.InterfaceIdentifier) error
	GetInterfaceIps(ctx context.Context) (map[domain.InterfaceIdentifier][]domain.Cidr, error)
	GetInterfacePeers(ctx context.Context, id domain.InterfaceIdentifier) ([]domain.Peer, error)
	FindInterfacePeers(ctx context.Context, id domain.InterfaceIdentifier, search string) ([]domain.Peer, error)
	GetUserPeers(ctx context.Context, id domain.UserIdentifier) ([]domain.Peer, error)
	FindUserPeers(ctx context.Context, id domain.UserIdentifier, search string) ([]domain.Peer, error)
	SavePeer(ctx context.Context, id domain.PeerIdentifier, updateFunc func(in *domain.Peer) (*domain.Peer, error)) error
	DeletePeer(ctx context.Context, id domain.PeerIdentifier) error
	GetUser(ctx context.Context, id domain.UserIdentifier) (*domain.User, error)
	GetAllUsers(ctx context.Context) ([]domain.User, error)
	FindUsers(ctx context.Context, search string) ([]domain.User, error)
	SaveUser(ctx context.Context, id domain.UserIdentifier, updateFunc func(u *domain.User) (*domain.User, error)) error
	DeleteUser(ctx context.Context, id domain.UserIdentifier) error
}

// endregion global-repositories

type App struct {
	Config *config.Config
	bus    evbus.MessageBus
	db     dbRepo
	wg     wireGuardRepo

	Authenticator *authenticator
	Users         *userManager
	WireGuard     *wireGuardManager
}

func New(cfg *config.Config, bus evbus.MessageBus, db dbRepo, wg wireGuardRepo) (*App, error) {
	users, err := newUserManager(cfg, bus, db, db)
	if err != nil {
		return nil, err
	}

	auth, err := newAuthenticator(&cfg.Auth, bus, users)
	if err != nil {
		return nil, err
	}

	wireGuard, err := newWireGuardManager(cfg, bus, wg, db)
	if err != nil {
		return nil, err
	}

	a := &App{
		Config: cfg,
		bus:    bus,
		db:     db,
		wg:     wg,

		Authenticator: auth,
		Users:         users,
		WireGuard:     wireGuard,
	}

	if a.Config.Core.ImportExisting {
		err := a.WireGuard.ImportNewInterfaces(context.Background())
		if err != nil {
			return nil, err
		}
	}

	if a.Config.Core.RestoreState {
		err := a.WireGuard.RestoreInterfaceState(context.Background(), true)
		if err != nil {
			return nil, err
		}
	}

	return a, nil
}

func HandleProgramArgs(cfg *config.Config, db *gorm.DB, wg wireGuardRepo) (exit bool, err error) {
	migrationSource := flag.String("migrateFrom", "", "path to v1 database file or DSN")
	migrationDbType := flag.String("migrateFromType", string(config.DatabaseSQLite), "old database type, either mysql, mssql, postgres or sqlite")
	flag.Parse()

	if *migrationSource != "" {
		err = migrateFromV1(cfg, db, wg, *migrationSource, *migrationDbType)
		exit = true
	}

	return
}
