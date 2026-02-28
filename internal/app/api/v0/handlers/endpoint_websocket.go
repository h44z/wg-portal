package handlers

import (
	"context"
	"net/http"
	"strings"
	"sync"

	"github.com/go-pkgz/routegroup"
	"github.com/gorilla/websocket"

	"github.com/h44z/wg-portal/internal/app"
	"github.com/h44z/wg-portal/internal/config"
	"github.com/h44z/wg-portal/internal/domain"
)

type WebsocketEventBus interface {
	Subscribe(topic string, fn any) error
	Unsubscribe(topic string, fn any) error
}

type WebsocketEndpoint struct {
	authenticator Authenticator
	bus           WebsocketEventBus

	upgrader websocket.Upgrader
}

func NewWebsocketEndpoint(cfg *config.Config, auth Authenticator, bus WebsocketEventBus) *WebsocketEndpoint {
	return &WebsocketEndpoint{
		authenticator: auth,
		bus:           bus,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				origin := r.Header.Get("Origin")
				return strings.HasPrefix(origin, cfg.Web.ExternalUrl)
			},
		},
	}
}

func (e WebsocketEndpoint) GetName() string {
	return "WebsocketEndpoint"
}

func (e WebsocketEndpoint) RegisterRoutes(g *routegroup.Bundle) {
	g.With(e.authenticator.LoggedIn()).HandleFunc("GET /ws", e.handleWebsocket())
}

// wsMessage represents a message sent over websocket to the frontend
type wsMessage struct {
	Type string `json:"type"` // either "peer_stats" or "interface_stats"
	Data any    `json:"data"` // domain.TrafficDelta
}

func (e WebsocketEndpoint) handleWebsocket() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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
			_ = writeJSON(wsMessage{Type: "peer_stats", Data: status})
		}
		interfaceStatsHandler := func(status domain.TrafficDelta) {
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
