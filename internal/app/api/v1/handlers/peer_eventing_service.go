package handlers

import (
	"context"

	"github.com/fedor-git/wg-portal-2/internal/app"
	"github.com/fedor-git/wg-portal-2/internal/domain"
)

type eventingPeerService struct {
    inner PeerService
    bus   app.EventPublisher
}

func NewEventingPeerService(inner PeerService, bus app.EventPublisher) PeerService {
    return &eventingPeerService{inner: inner, bus: bus}
}

// ---------- read-only ----------
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
    // вхідний sync нічого не фан-аутить сам по собі — і це ок
    return s.inner.SyncAllPeersFromDB(ctx)
}

// ---------- мутації + fanout ----------
func (s *eventingPeerService) Create(ctx context.Context, p *domain.Peer) (*domain.Peer, error) {
    out, err := s.inner.Create(ctx, p)
    if err != nil { return nil, err }
    s.bumpFanout(ctx, "peer.save", out)          // 1 аргумент
    s.bumpFanout(ctx, "peers.updated", "v1:create")
    return out, nil
}

func (s *eventingPeerService) Update(ctx context.Context, id domain.PeerIdentifier, p *domain.Peer) (*domain.Peer, error) {
    out, err := s.inner.Update(ctx, id, p)
    if err != nil { return nil, err }
    s.bumpFanout(ctx, "peer.save", out)          // 1 аргумент
    s.bumpFanout(ctx, "peers.updated", "v1:update")
    return out, nil
}

func (s *eventingPeerService) Delete(ctx context.Context, id domain.PeerIdentifier) error {
    if err := s.inner.Delete(ctx, id); err != nil { return err }
    s.bumpFanout(ctx, "peer.delete", id)         // 1 аргумент
    s.bumpFanout(ctx, "peers.updated", "v1:delete")
    return nil
}

// ---- helpers ----
func (s *eventingPeerService) bumpFanout(ctx context.Context, topic string, arg any) {
    if s.bus == nil || topic == "" { return }
    if app.NoFanout(ctx) { return }    // важливо: не ехо
    s.bus.Publish(topic, arg)          // рівно 1 аргумент
}
