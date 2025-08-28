package backend

import (
	"context"
	"io"

	"github.com/fedor-git/wg-portal-2/internal/config"
	"github.com/fedor-git/wg-portal-2/internal/domain"
)

// region dependencies

type InterfaceServiceInterfaceManager interface {
	GetAllInterfacesAndPeers(ctx context.Context) ([]domain.Interface, [][]domain.Peer, error)
	GetInterfaceAndPeers(ctx context.Context, id domain.InterfaceIdentifier) (*domain.Interface, []domain.Peer, error)
	CreateInterface(ctx context.Context, in *domain.Interface) (*domain.Interface, error)
	UpdateInterface(ctx context.Context, in *domain.Interface) (*domain.Interface, []domain.Peer, error)
	DeleteInterface(ctx context.Context, id domain.InterfaceIdentifier) error
	PrepareInterface(ctx context.Context) (*domain.Interface, error)
	ApplyPeerDefaults(ctx context.Context, in *domain.Interface) error
}

type InterfaceServiceConfigFileManager interface {
	PersistInterfaceConfig(ctx context.Context, id domain.InterfaceIdentifier) error
	GetInterfaceConfig(ctx context.Context, id domain.InterfaceIdentifier) (io.Reader, error)
}

// endregion dependencies

type InterfaceService struct {
	cfg *config.Config

	interfaces InterfaceServiceInterfaceManager
	configFile InterfaceServiceConfigFileManager
}

func NewInterfaceService(
	cfg *config.Config,
	interfaces InterfaceServiceInterfaceManager,
	configFile InterfaceServiceConfigFileManager,
) *InterfaceService {
	return &InterfaceService{
		cfg:        cfg,
		interfaces: interfaces,
		configFile: configFile,
	}
}

func (i InterfaceService) GetInterfaceAndPeers(ctx context.Context, id domain.InterfaceIdentifier) (
	*domain.Interface,
	[]domain.Peer,
	error,
) {
	return i.interfaces.GetInterfaceAndPeers(ctx, id)
}

func (i InterfaceService) PrepareInterface(ctx context.Context) (*domain.Interface, error) {
	return i.interfaces.PrepareInterface(ctx)
}

func (i InterfaceService) CreateInterface(ctx context.Context, in *domain.Interface) (*domain.Interface, error) {
	return i.interfaces.CreateInterface(ctx, in)
}

func (i InterfaceService) UpdateInterface(ctx context.Context, in *domain.Interface) (
	*domain.Interface,
	[]domain.Peer,
	error,
) {
	return i.interfaces.UpdateInterface(ctx, in)
}

func (i InterfaceService) DeleteInterface(ctx context.Context, id domain.InterfaceIdentifier) error {
	return i.interfaces.DeleteInterface(ctx, id)
}

func (i InterfaceService) GetAllInterfacesAndPeers(ctx context.Context) ([]domain.Interface, [][]domain.Peer, error) {
	return i.interfaces.GetAllInterfacesAndPeers(ctx)
}

func (i InterfaceService) GetInterfaceConfig(ctx context.Context, id domain.InterfaceIdentifier) (io.Reader, error) {
	return i.configFile.GetInterfaceConfig(ctx, id)
}

func (i InterfaceService) PersistInterfaceConfig(ctx context.Context, id domain.InterfaceIdentifier) error {
	return i.configFile.PersistInterfaceConfig(ctx, id)
}

func (i InterfaceService) ApplyPeerDefaults(ctx context.Context, in *domain.Interface) error {
	return i.interfaces.ApplyPeerDefaults(ctx, in)
}
