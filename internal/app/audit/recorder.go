package audit

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/h44z/wg-portal/internal/app"
	"github.com/h44z/wg-portal/internal/config"
	"github.com/h44z/wg-portal/internal/domain"
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

	if err := r.bus.Subscribe(app.TopicAuthLogin, r.authLoginEvent); err != nil {
		return fmt.Errorf("failed to subscribe to %s: %w", app.TopicAuthLogin, err)
	}

	return nil
}

func (r *Recorder) authLoginEvent(userIdentifier domain.UserIdentifier) {
	err := r.db.SaveAuditEntry(context.Background(), &domain.AuditEntry{
		CreatedAt: time.Time{},
		Severity:  domain.AuditSeverityLevelLow,
		Origin:    "authLoginEvent",
		Message:   fmt.Sprintf("user %s logged in", userIdentifier),
	})
	if err != nil {
		slog.Error("failed to create audit entry for handleAuthLoginEvent", "error", err)
		return
	}
}
