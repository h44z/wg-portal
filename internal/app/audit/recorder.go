package audit

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/fedor-git/wg-portal-2/internal/app"
	"github.com/fedor-git/wg-portal-2/internal/config"
	"github.com/fedor-git/wg-portal-2/internal/domain"
)

// region dependencies

type DatabaseRepo interface {
	// SaveAuditEntry saves an audit entry to the database
	SaveAuditEntry(ctx context.Context, entry *domain.AuditEntry) error
}

type EventBus interface {
	// Subscribe subscribes to a topic
	Subscribe(topic string, fn interface{}) error
}

// endregion dependencies

// Recorder is responsible for recording audit events to the database.
type Recorder struct {
	cfg *config.Config
	bus EventBus

	db DatabaseRepo
}

// NewAuditRecorder creates a new audit recorder instance.
func NewAuditRecorder(cfg *config.Config, bus EventBus, db DatabaseRepo) (*Recorder, error) {
	r := &Recorder{
		cfg: cfg,
		bus: bus,

		db: db,
	}

	err := r.connectToMessageBus()
	if err != nil {
		return nil, fmt.Errorf("failed to setup message bus: %w", err)
	}

	return r, nil
}

// StartBackgroundJobs starts background jobs for the audit recorder.
// This method is non-blocking and returns immediately.
func (r *Recorder) StartBackgroundJobs(ctx context.Context) {
	if !r.cfg.Statistics.CollectAuditData {
		return // noting to do
	}

	go func() {
		running := true
		for running {
			select {
			case <-ctx.Done():
				running = false
				continue
			case <-time.After(1 * time.Hour):
				// select blocks until one of the cases evaluate to true
			}

			slog.Debug("audit status", "registered_messages", 0) // TODO: implement
		}
	}()
}

func (r *Recorder) connectToMessageBus() error {
	if !r.cfg.Statistics.CollectAuditData {
		return nil // noting to do
	}

	if err := r.bus.Subscribe(app.TopicAuditLoginSuccess, r.handleAuthEvent); err != nil {
		return fmt.Errorf("failed to subscribe to %s: %w", app.TopicAuditLoginSuccess, err)
	}
	if err := r.bus.Subscribe(app.TopicAuditLoginFailed, r.handleAuthEvent); err != nil {
		return fmt.Errorf("failed to subscribe to %s: %w", app.TopicAuditLoginFailed, err)
	}
	if err := r.bus.Subscribe(app.TopicAuditInterfaceChanged, r.handleInterfaceEvent); err != nil {
		return fmt.Errorf("failed to subscribe to %s: %w", app.TopicAuditInterfaceChanged, err)
	}
	if err := r.bus.Subscribe(app.TopicAuditPeerChanged, r.handlePeerEvent); err != nil {
		return fmt.Errorf("failed to subscribe to %s: %w", app.TopicAuditPeerChanged, err)
	}

	return nil
}

func (r *Recorder) handleAuthEvent(event domain.AuditEventWrapper[AuthEvent]) {
	err := r.db.SaveAuditEntry(context.Background(), r.authEventToAuditEntry(event))
	if err != nil {
		slog.Error("failed to create audit entry for auth event", "error", err)
		return
	}
}

func (r *Recorder) handleInterfaceEvent(event domain.AuditEventWrapper[InterfaceEvent]) {
	err := r.db.SaveAuditEntry(context.Background(), r.interfaceEventToAuditEntry(event))
	if err != nil {
		slog.Error("failed to create audit entry for interface event", "error", err)
		return
	}
}

func (r *Recorder) handlePeerEvent(event domain.AuditEventWrapper[PeerEvent]) {
	err := r.db.SaveAuditEntry(context.Background(), r.peerEventToAuditEntry(event))
	if err != nil {
		slog.Error("failed to create audit entry for peer event", "error", err)
		return
	}
}

func (r *Recorder) authEventToAuditEntry(event domain.AuditEventWrapper[AuthEvent]) *domain.AuditEntry {
	contextUser := domain.GetUserInfo(event.Ctx)
	e := domain.AuditEntry{
		CreatedAt:   time.Now(),
		Severity:    domain.AuditSeverityLevelLow,
		ContextUser: contextUser.UserId(),
		Origin:      fmt.Sprintf("auth: %s", event.Source),
		Message:     fmt.Sprintf("%s logged in", event.Event.Username),
	}

	if event.Event.Error != "" {
		e.Severity = domain.AuditSeverityLevelHigh
		e.Message = fmt.Sprintf("%s failed to login: %s", event.Event.Username, event.Event.Error)
	}

	return &e
}

func (r *Recorder) interfaceEventToAuditEntry(event domain.AuditEventWrapper[InterfaceEvent]) *domain.AuditEntry {
	contextUser := domain.GetUserInfo(event.Ctx)
	e := domain.AuditEntry{
		CreatedAt:   time.Now(),
		Severity:    domain.AuditSeverityLevelLow,
		ContextUser: contextUser.UserId(),
		Origin:      fmt.Sprintf("interface: %s", event.Event.Action),
	}

	switch event.Event.Action {
	case "save":
		e.Message = fmt.Sprintf("%s updated", event.Event.Interface.Identifier)
	default:
		e.Message = fmt.Sprintf("%s: unknown action", event.Event.Interface.Identifier)
	}

	return &e
}

func (r *Recorder) peerEventToAuditEntry(event domain.AuditEventWrapper[PeerEvent]) *domain.AuditEntry {
	contextUser := domain.GetUserInfo(event.Ctx)
	e := domain.AuditEntry{
		CreatedAt:   time.Now(),
		Severity:    domain.AuditSeverityLevelLow,
		ContextUser: contextUser.UserId(),
		Origin:      fmt.Sprintf("peer: %s", event.Event.Action),
	}

	switch event.Event.Action {
	case "save":
		e.Message = fmt.Sprintf("%s updated", event.Event.Peer.Identifier)
	default:
		e.Message = fmt.Sprintf("%s: unknown action", event.Event.Peer.Identifier)
	}

	return &e
}
