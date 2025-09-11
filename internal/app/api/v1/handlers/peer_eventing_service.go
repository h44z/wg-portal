package handlers

import (
	"context"

	"github.com/fedor-git/wg-portal-2/internal/app" // твої топіки і EventPublisher
	"github.com/fedor-git/wg-portal-2/internal/domain"
)

// eventingPeerService декорує будь-яку реалізацію PeerService
// та надсилає івенти після Create/Update/Delete.
type eventingPeerService struct {
	inner PeerService
	bus   app.EventPublisher
}

// Конструктор: повертає саме PeerService, тож NewPeerEndpoint прийме як є.
func NewEventingPeerService(inner PeerService, bus app.EventPublisher) PeerService {
	return &eventingPeerService{inner: inner, bus: bus}
}

// ---------- read-only як є ----------

func (s *eventingPeerService) GetForInterface(ctx context.Context, id domain.InterfaceIdentifier) ([]domain.Peer, error) {
	return s.inner.GetForInterface(ctx, id)
}
func (s *eventingPeerService) GetForUser(ctx context.Context, id domain.UserIdentifier) ([]domain.Peer, error) {
	return s.inner.GetForUser(ctx, id)
}
func (s *eventingPeerService) GetById(ctx context.Context, id domain.PeerIdentifier) (*domain.Peer, error) {
	return s.inner.GetById(ctx, id)
}
func (s *eventingPeerService) Prepare(ctx context.Context, id domain.InterfaceIdentifier) (*domain.Peer, error) {
	return s.inner.Prepare(ctx, id)
}
func (s *eventingPeerService) SyncAllPeersFromDB(ctx context.Context) (int, error) {
	return s.inner.SyncAllPeersFromDB(ctx)
}

// ---------- мутації + події ----------

func (s *eventingPeerService) Create(ctx context.Context, p *domain.Peer) (*domain.Peer, error) {
    out, err := s.inner.Create(ctx, p)
    if err != nil { return nil, err }
    s.publish(app.TopicPeerCreated)
    s.publish(app.TopicPeerUpdated)
    return out, nil
}

func (s *eventingPeerService) Update(ctx context.Context, id domain.PeerIdentifier, p *domain.Peer) (*domain.Peer, error) {
    out, err := s.inner.Update(ctx, id, p)
    if err != nil { return nil, err }
    s.publish(app.TopicPeerUpdated)
    return out, nil
}

func (s *eventingPeerService) Delete(ctx context.Context, id domain.PeerIdentifier) error {
    if err := s.inner.Delete(ctx, id); err != nil { return err }
    s.publish(app.TopicPeerDeleted)
    s.publish(app.TopicPeerUpdated)
    return nil
}

func (s *eventingPeerService) publish(topic string, args ...any) {
    if s.bus == nil || topic == "" { return }
    s.bus.Publish(topic, args...)
}