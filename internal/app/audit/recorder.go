package audit

import (
	"context"
	"fmt"
	"github.com/h44z/wg-portal/internal/app"
	"github.com/h44z/wg-portal/internal/config"
	"github.com/h44z/wg-portal/internal/domain"
	"github.com/sirupsen/logrus"
	evbus "github.com/vardius/message-bus"
	"time"
)

type Recorder struct {
	cfg *config.Config
	bus evbus.MessageBus

	db DatabaseRepo
}

func NewAuditRecorder(cfg *config.Config, bus evbus.MessageBus, db DatabaseRepo) (*Recorder, error) {
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

			logrus.Tracef("registered %d audit message within the last hour", 0) // TODO: implement
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
		logrus.Errorf("failed to create audit entry for handleAuthLoginEvent: %v", err)
		return
	}
}
