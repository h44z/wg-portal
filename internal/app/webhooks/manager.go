package webhooks

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/fedor-git/wg-portal-2/internal/app"
	"github.com/fedor-git/wg-portal-2/internal/app/webhooks/models"
	"github.com/fedor-git/wg-portal-2/internal/config"
	"github.com/fedor-git/wg-portal-2/internal/domain"
)

// region dependencies

type EventBus interface {
	// Publish sends a message to the message bus.
	Publish(topic string, args ...any)
	// Subscribe subscribes to a topic
	Subscribe(topic string, fn interface{}) error
}

// endregion dependencies

type Manager struct {
	cfg *config.Config
	bus EventBus

	client *http.Client
}

// NewManager creates a new webhook manager instance.
func NewManager(cfg *config.Config, bus EventBus) (*Manager, error) {
	m := &Manager{
		cfg: cfg,
		bus: bus,
		client: &http.Client{
			Timeout: cfg.Webhook.Timeout,
		},
	}

	m.connectToMessageBus()

	return m, nil
}

// StartBackgroundJobs starts background jobs for the webhook manager.
// This method is non-blocking and returns immediately.
func (m Manager) StartBackgroundJobs(_ context.Context) {
	// this is a no-op for now
}

func (m Manager) connectToMessageBus() {
	if m.cfg.Webhook.Url == "" {
		slog.Info("[WEBHOOK] no webhook configured, skipping event-bus subscription")
		return
	}

	_ = m.bus.Subscribe(app.TopicUserCreated, m.handleUserCreateEvent)
	_ = m.bus.Subscribe(app.TopicUserUpdated, m.handleUserUpdateEvent)
	_ = m.bus.Subscribe(app.TopicUserDeleted, m.handleUserDeleteEvent)

	_ = m.bus.Subscribe(app.TopicPeerCreated, m.handlePeerCreateEvent)
	_ = m.bus.Subscribe(app.TopicPeerUpdated, m.handlePeerUpdateEvent)
	_ = m.bus.Subscribe(app.TopicPeerDeleted, m.handlePeerDeleteEvent)
	_ = m.bus.Subscribe(app.TopicPeerStateChanged, m.handlePeerStateChangeEvent)

	_ = m.bus.Subscribe(app.TopicInterfaceCreated, m.handleInterfaceCreateEvent)
	_ = m.bus.Subscribe(app.TopicInterfaceUpdated, m.handleInterfaceUpdateEvent)
	_ = m.bus.Subscribe(app.TopicInterfaceDeleted, m.handleInterfaceDeleteEvent)
}

func (m Manager) sendWebhook(ctx context.Context, data io.Reader) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.cfg.Webhook.Url, data)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	if m.cfg.Webhook.Authentication != "" {
		req.Header.Set("Authorization", m.cfg.Webhook.Authentication)
	}

	resp, err := m.client.Do(req)
	if err != nil {
		return err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			slog.Error("[WEBHOOK] failed to close response body", "error", err)
		}
	}(resp.Body)

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("webhook request failed with status: %s", resp.Status)
	}

	return nil
}

func (m Manager) handleUserCreateEvent(user domain.User) {
	m.handleGenericEvent(WebhookEventCreate, models.NewUser(user))
}

func (m Manager) handleUserUpdateEvent(user domain.User) {
	m.handleGenericEvent(WebhookEventUpdate, models.NewUser(user))
}

func (m Manager) handleUserDeleteEvent(user domain.User) {
	m.handleGenericEvent(WebhookEventDelete, models.NewUser(user))
}

func (m Manager) handlePeerCreateEvent(peer domain.Peer) {
	m.handleGenericEvent(WebhookEventCreate, models.NewPeer(peer))
}

func (m Manager) handlePeerUpdateEvent(peer domain.Peer) {
	m.handleGenericEvent(WebhookEventUpdate, models.NewPeer(peer))
}

func (m Manager) handlePeerDeleteEvent(peer domain.Peer) {
	m.handleGenericEvent(WebhookEventDelete, models.NewPeer(peer))
}

func (m Manager) handleInterfaceCreateEvent(iface domain.Interface) {
	m.handleGenericEvent(WebhookEventCreate, models.NewInterface(iface))
}

func (m Manager) handleInterfaceUpdateEvent(iface domain.Interface) {
	m.handleGenericEvent(WebhookEventUpdate, models.NewInterface(iface))
}

func (m Manager) handleInterfaceDeleteEvent(iface domain.Interface) {
	m.handleGenericEvent(WebhookEventDelete, models.NewInterface(iface))
}

func (m Manager) handlePeerStateChangeEvent(peerStatus domain.PeerStatus, peer domain.Peer) {
	if peerStatus.IsConnected {
		m.handleGenericEvent(WebhookEventConnect, models.NewPeerMetrics(peerStatus, peer))
	} else {
		m.handleGenericEvent(WebhookEventDisconnect, models.NewPeerMetrics(peerStatus, peer))
	}
}

func (m Manager) handleGenericEvent(action WebhookEvent, payload any) {
	eventData, err := m.createWebhookData(action, payload)
	if err != nil {
		slog.Error("[WEBHOOK] failed to create webhook data", "error", err, "action", action,
			"payload", fmt.Sprintf("%T", payload))
		return
	}

	eventJson, err := eventData.Serialize()
	if err != nil {
		slog.Error("[WEBHOOK] failed to serialize event data", "error", err, "action", action,
			"payload", fmt.Sprintf("%T", payload), "identifier", eventData.Identifier)
		return
	}

	err = m.sendWebhook(context.Background(), eventJson)
	if err != nil {
		slog.Error("[WEBHOOK] failed to execute webhook", "error", err, "action", action,
			"payload", fmt.Sprintf("%T", payload), "identifier", eventData.Identifier)
		return
	}

	slog.Info("[WEBHOOK] executed webhook", "action", action, "payload", fmt.Sprintf("%T", payload),
		"identifier", eventData.Identifier)
}

func (m Manager) createWebhookData(action WebhookEvent, payload any) (*WebhookData, error) {
	d := &WebhookData{
		Event:   action,
		Payload: payload,
	}

	switch v := payload.(type) {
	case models.User:
		d.Entity = WebhookEntityUser
		d.Identifier = v.Identifier
	case models.Peer:
		d.Entity = WebhookEntityPeer
		d.Identifier = v.Identifier
	case models.Interface:
		d.Entity = WebhookEntityInterface
		d.Identifier = v.Identifier
	case models.PeerMetrics:
		d.Entity = WebhookEntityPeerMetric
		d.Identifier = v.Peer.Identifier
	default:
		return nil, fmt.Errorf("unsupported payload type: %T", v)
	}

	return d, nil
}
