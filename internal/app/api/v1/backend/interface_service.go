package backend

import (
	"context"
	"fmt"
	"slices"

	"github.com/h44z/wg-portal/internal/config"
	"github.com/h44z/wg-portal/internal/domain"
)

type InterfaceServiceInterfaceManagerRepo interface {
	GetAllInterfacesAndPeers(ctx context.Context) ([]domain.Interface, [][]domain.Peer, error)
	GetInterfaceAndPeers(ctx context.Context, id domain.InterfaceIdentifier) (*domain.Interface, []domain.Peer, error)
	PrepareInterface(ctx context.Context) (*domain.Interface, error)
	CreateInterface(ctx context.Context, in *domain.Interface) (*domain.Interface, error)
	UpdateInterface(ctx context.Context, in *domain.Interface) (*domain.Interface, []domain.Peer, error)
	DeleteInterface(ctx context.Context, id domain.InterfaceIdentifier) error
}

type InterfaceService struct {
	cfg *config.Config

	interfaces InterfaceServiceInterfaceManagerRepo
	users      PeerServiceUserManagerRepo
}

func NewInterfaceService(cfg *config.Config, interfaces InterfaceServiceInterfaceManagerRepo) *InterfaceService {
	return &InterfaceService{
		cfg:        cfg,
		interfaces: interfaces,
	}
}

func (s InterfaceService) GetAll(ctx context.Context) ([]domain.Interface, [][]domain.Peer, error) {
	if err := domain.ValidateAdminAccessRights(ctx); err != nil {
		return nil, nil, err
	}

	interfaces, interfacePeers, err := s.interfaces.GetAllInterfacesAndPeers(ctx)
	if err != nil {
		return nil, nil, err
	}

	return interfaces, interfacePeers, nil
}

func (s InterfaceService) GetById(ctx context.Context, id domain.InterfaceIdentifier) (
	*domain.Interface,
	[]domain.Peer,
	error,
) {
	if err := domain.ValidateAdminAccessRights(ctx); err != nil {
		return nil, nil, err
	}

	interfaceData, interfacePeers, err := s.interfaces.GetInterfaceAndPeers(ctx, id)
	if err != nil {
		return nil, nil, err
	}

	return interfaceData, interfacePeers, nil
}

func (s InterfaceService) Prepare(ctx context.Context) (*domain.Interface, error) {
	if err := domain.ValidateAdminAccessRights(ctx); err != nil {
		return nil, err
	}

	interfaceData, err := s.interfaces.PrepareInterface(ctx)
	if err != nil {
		return nil, err
	}

	return interfaceData, nil
}

func (s InterfaceService) Create(ctx context.Context, iface *domain.Interface) (*domain.Interface, error) {
	if err := domain.ValidateAdminAccessRights(ctx); err != nil {
		return nil, err
	}

	if err := s.validateBackend(iface.Backend); err != nil {
		return nil, err
	}

	createdInterface, err := s.interfaces.CreateInterface(ctx, iface)
	if err != nil {
		return nil, err
	}

	return createdInterface, nil
}

func (s InterfaceService) Update(ctx context.Context, id domain.InterfaceIdentifier, iface *domain.Interface) (
	*domain.Interface,
	[]domain.Peer,
	error,
) {
	if err := domain.ValidateAdminAccessRights(ctx); err != nil {
		return nil, nil, err
	}

	if iface.Identifier != id {
		return nil, nil, fmt.Errorf("interface id mismatch: %s != %s: %w",
			iface.Identifier, id, domain.ErrInvalidData)
	}

	existingInterface, _, err := s.interfaces.GetInterfaceAndPeers(ctx, id)
	if err != nil {
		return nil, nil, err
	}

	if err := s.validateBackend(iface.Backend); err != nil {
		return nil, nil, err
	}
	// These fields are managed by the server and cannot be changed through v1.
	iface.IsDynamic = existingInterface.IsDynamic
	iface.DriverType = existingInterface.DriverType
	iface.LdapAllowedUsers = existingInterface.LdapAllowedUsers

	updatedInterface, updatedPeers, err := s.interfaces.UpdateInterface(ctx, iface)
	if err != nil {
		return nil, nil, err
	}

	return updatedInterface, updatedPeers, nil
}

// validateBackend validates against configured IDs so a typo cannot silently use
// the controller manager's fallback to the local backend.
func (s InterfaceService) validateBackend(backend domain.InterfaceBackend) error {
	id := string(backend)

	// Empty is the legacy representation of the local backend in domain objects.
	if id == "" || id == config.LocalBackendName ||
		slices.ContainsFunc(s.cfg.Backend.Mikrotik, func(b config.BackendMikrotik) bool { return b.Id == id }) ||
		slices.ContainsFunc(s.cfg.Backend.Pfsense, func(b config.BackendPfsense) bool { return b.Id == id }) ||
		slices.ContainsFunc(s.cfg.Backend.Opnsense, func(b config.BackendOpnsense) bool { return b.Id == id }) {
		return nil
	}

	return fmt.Errorf("unknown interface backend %q: %w", id, domain.ErrInvalidData)
}

func (s InterfaceService) Delete(ctx context.Context, id domain.InterfaceIdentifier) error {
	if err := domain.ValidateAdminAccessRights(ctx); err != nil {
		return err
	}

	err := s.interfaces.DeleteInterface(ctx, id)
	if err != nil {
		return err
	}

	return nil
}
