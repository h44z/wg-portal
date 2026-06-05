package handlers

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/go-pkgz/routegroup"
	"github.com/gorilla/websocket"

	"github.com/h44z/wg-portal/internal/app"
	"github.com/h44z/wg-portal/internal/config"
	"github.com/h44z/wg-portal/internal/domain"
)

const (
	websocketPeerUserIdentifierCacheTTL             = 90 * time.Second
	websocketPeerUserIdentifierCacheCleanupInterval = websocketPeerUserIdentifierCacheTTL * 2
)

type WebsocketEventBus interface {
	Subscribe(topic string, fn any) error
	Unsubscribe(topic string, fn any) error
}

type WebsocketPeerService interface {
	GetPeer(ctx context.Context, id domain.PeerIdentifier) (*domain.Peer, error)
}

type WebsocketEndpoint struct {
	authenticator Authenticator
	bus           WebsocketEventBus
	peerService   WebsocketPeerService

	upgrader websocket.Upgrader

	ownershipCache    map[domain.PeerIdentifier]peerUserIdentifierCacheEntry
	ownershipCacheMux sync.Mutex
}

func NewWebsocketEndpoint(
	cfg *config.Config,
	auth Authenticator,
	bus WebsocketEventBus,
	peerService WebsocketPeerService,
) *WebsocketEndpoint {
	return &WebsocketEndpoint{
		authenticator: auth,
		bus:           bus,
		peerService:   peerService,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return matchOrigin(cfg.Web.ExternalUrl, r.Header.Get("Origin"))
			},
		},
		ownershipCache:    make(map[domain.PeerIdentifier]peerUserIdentifierCacheEntry),
		ownershipCacheMux: sync.Mutex{},
	}
}

func (e *WebsocketEndpoint) GetName() string {
	return "WebsocketEndpoint"
}

func (e *WebsocketEndpoint) RegisterRoutes(g *routegroup.Bundle) {
	g.With(e.authenticator.LoggedIn()).HandleFunc("GET /ws", e.handleWebsocket())
}

// StartBackgroundJobs starts background jobs like the expired peers check.
// This method is non-blocking.
func (e *WebsocketEndpoint) StartBackgroundJobs(ctx context.Context) {
	go e.startOwnerCacheCleanup(ctx)
}

// wsMessage represents a message sent over websocket to the frontend
type wsMessage struct {
	Type string `json:"type"` // either "peer_stats" or "interface_stats"
	Data any    `json:"data"` // domain.TrafficDelta
}

// peerUserIdentifierCacheEntry is a cache entry object that reduces database load when checking peer ownership.
type peerUserIdentifierCacheEntry struct {
	userIdentifier domain.UserIdentifier
	expiresAt      time.Time
}

func (e *WebsocketEndpoint) handleWebsocket() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userInfo := domain.GetUserInfo(r.Context())

		conn, err := e.upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		ctx, cancel := context.WithCancel(r.Context())
		defer cancel()

		writeMutex := sync.Mutex{}
		writeJSON := func(msg wsMessage) error {
			writeMutex.Lock()
			defer writeMutex.Unlock()
			return conn.WriteJSON(msg)
		}

		peerStatsHandler := func(status domain.TrafficDelta) {
			if !userInfo.IsAdmin {
				// lookup peer user-info to validate ownership
				peerUserIdentifier, err := e.getPeerUserIdentifier(ctx, domain.PeerIdentifier(status.EntityId))
				if err != nil {
					return
				}

				if peerUserIdentifier == "" {
					return // if peer is not assigned to any user, dont send stats
				}

				if peerUserIdentifier != userInfo.Id {
					return // only expose stats for own peers
				}
			}

			_ = writeJSON(wsMessage{Type: "peer_stats", Data: status})
		}
		interfaceStatsHandler := func(status domain.TrafficDelta) {
			if !userInfo.IsAdmin {
				return // interface stats will only be exposed to admins
			}

			_ = writeJSON(wsMessage{Type: "interface_stats", Data: status})
		}

		_ = e.bus.Subscribe(app.TopicPeerStatsUpdated, peerStatsHandler)
		defer e.bus.Unsubscribe(app.TopicPeerStatsUpdated, peerStatsHandler)
		_ = e.bus.Subscribe(app.TopicInterfaceStatsUpdated, interfaceStatsHandler)
		defer e.bus.Unsubscribe(app.TopicInterfaceStatsUpdated, interfaceStatsHandler)

		// Keep connection open until client disconnects or context is cancelled
		go func() {
			for {
				if _, _, err := conn.ReadMessage(); err != nil {
					cancel()
					return
				}
			}
		}()

		<-ctx.Done()
	}
}

func (e *WebsocketEndpoint) getPeerUserIdentifier(
	ctx context.Context,
	peerIdentifier domain.PeerIdentifier,
) (domain.UserIdentifier, error) {
	now := time.Now()

	e.ownershipCacheMux.Lock()
	entry, ok := e.ownershipCache[peerIdentifier]
	if ok && now.Before(entry.expiresAt) {
		e.ownershipCacheMux.Unlock()
		return entry.userIdentifier, nil
	}
	e.ownershipCacheMux.Unlock()

	peer, err := e.peerService.GetPeer(ctx, peerIdentifier)
	if err != nil {
		return "", err
	}

	e.ownershipCacheMux.Lock()
	defer e.ownershipCacheMux.Unlock()
	e.ownershipCache[peerIdentifier] = peerUserIdentifierCacheEntry{
		userIdentifier: peer.UserIdentifier,
		expiresAt:      now.Add(websocketPeerUserIdentifierCacheTTL),
	}

	return peer.UserIdentifier, nil
}

func (e *WebsocketEndpoint) startOwnerCacheCleanup(ctx context.Context) {
	ticker := time.NewTicker(websocketPeerUserIdentifierCacheCleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			e.cleanupOwnerCache(now)
		}
	}
}

func (e *WebsocketEndpoint) cleanupOwnerCache(now time.Time) {
	e.ownershipCacheMux.Lock()
	defer e.ownershipCacheMux.Unlock()

	for peerIdentifier, entry := range e.ownershipCache {
		if !now.Before(entry.expiresAt) {
			delete(e.ownershipCache, peerIdentifier)
		}
	}
}

func matchOrigin(externalBaseUrl, origin string) bool {
	originURL, err := url.Parse(origin)
	if err != nil {
		return false
	}

	externalURL, err := url.Parse(externalBaseUrl)
	if err != nil {
		return false
	}

	return originURL.Scheme == externalURL.Scheme &&
		strings.EqualFold(originURL.Host, externalURL.Host)
}
