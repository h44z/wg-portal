package webhooks

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/h44z/wg-portal/internal/app"
	"github.com/h44z/wg-portal/internal/config"
	"github.com/h44z/wg-portal/internal/domain"
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
	m.handleGenericEvent(WebhookEventCreate, user)
}

func (m Manager) handleUserUpdateEvent(user domain.User) {
	m.handleGenericEvent(WebhookEventUpdate, user)
}

func (m Manager) handleUserDeleteEvent(user domain.User) {
	m.handleGenericEvent(WebhookEventDelete, user)
}

func (m Manager) handlePeerCreateEvent(peer domain.Peer) {
	m.handleGenericEvent(WebhookEventCreate, peer)
}

func (m Manager) handlePeerUpdateEvent(peer domain.Peer) {
	m.handleGenericEvent(WebhookEventUpdate, peer)
}

func (m Manager) handlePeerDeleteEvent(peer domain.Peer) {
	m.handleGenericEvent(WebhookEventDelete, peer)
}

func (m Manager) handleInterfaceCreateEvent(iface domain.Interface) {
	m.handleGenericEvent(WebhookEventCreate, iface)
}

func (m Manager) handleInterfaceUpdateEvent(iface domain.Interface) {
	m.handleGenericEvent(WebhookEventUpdate, iface)
}

func (m Manager) handleInterfaceDeleteEvent(iface domain.Interface) {
	m.handleGenericEvent(WebhookEventDelete, iface)
}

func (m Manager) handlePeerStateChangeEvent(peerStatus domain.PeerStatus) {
	if peerStatus.IsConnected {
		m.handleGenericEvent(WebhookEventConnect, peerStatus)
	} else {
		m.handleGenericEvent(WebhookEventDisconnect, peerStatus)
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
	case domain.User:
		d.Entity = WebhookEntityUser
		d.Identifier = string(v.Identifier)
	case domain.Peer:
		d.Entity = WebhookEntityPeer
		d.Identifier = string(v.Identifier)
	case domain.Interface:
		d.Entity = WebhookEntityInterface
		d.Identifier = string(v.Identifier)
	case domain.PeerStatus:
		d.Entity = WebhookEntityPeer
		d.Identifier = string(v.PeerId)
	default:
		return nil, fmt.Errorf("unsupported payload type: %T", v)
	}

	return d, nil
}
