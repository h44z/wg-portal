package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	evbus "github.com/vardius/message-bus"

	"github.com/h44z/wg-portal/internal/app"
	"github.com/h44z/wg-portal/internal/config"
	"github.com/h44z/wg-portal/internal/domain"
)

// region test-helper

type websocketTestPeerService struct {
	peers map[domain.PeerIdentifier]*domain.Peer
}

func (s websocketTestPeerService) GetPeer(ctx context.Context, id domain.PeerIdentifier) (*domain.Peer, error) {
	peer, ok := s.peers[id]
	if !ok {
		return nil, errors.New("peer not found")
	}

	return peer, nil
}

func newTestWebsocketConnection(
	t *testing.T,
	bus evbus.MessageBus,
	userInfo *domain.ContextUserInfo,
	peers map[domain.PeerIdentifier]*domain.Peer,
) (*websocket.Conn, func()) {
	t.Helper()

	cfg := &config.Config{}
	endpoint := NewWebsocketEndpoint(cfg, nil, bus, websocketTestPeerService{peers: peers})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r = r.WithContext(domain.SetUserInfo(r.Context(), userInfo))
		endpoint.handleWebsocket()(w, r)
	}))
	cfg.Web.ExternalUrl = server.URL

	wsURL := "ws" + server.URL[len("http"):]
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, http.Header{"Origin": []string{server.URL}})
	if err != nil {
		server.Close()
		t.Fatalf("failed to dial websocket: %v", err)
	}

	cleanup := func() {
		conn.Close()
		server.Close()
	}

	return conn, cleanup
}

func assertWebsocketMessage(t *testing.T, conn *websocket.Conn, messageType string, entityId string) {
	t.Helper()

	if err := conn.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatalf("failed to set read deadline: %v", err)
	}

	var message wsMessage
	if err := conn.ReadJSON(&message); err != nil {
		t.Fatalf("failed to read websocket message: %v", err)
	}

	if message.Type != messageType {
		t.Fatalf("unexpected message type: got %q, want %q", message.Type, messageType)
	}

	data, ok := message.Data.(map[string]any)
	if !ok {
		t.Fatalf("unexpected message data type: %T", message.Data)
	}
	if data["EntityId"] != entityId {
		t.Fatalf("unexpected entity id: got %v, want %q", data["EntityId"], entityId)
	}
}

func assertNoWebsocketMessage(t *testing.T, conn *websocket.Conn) {
	t.Helper()

	if err := conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond)); err != nil {
		t.Fatalf("failed to set read deadline: %v", err)
	}

	var message wsMessage
	if err := conn.ReadJSON(&message); err == nil {
		t.Fatalf("unexpected websocket message: %+v", message)
	}
}

// endregion test-helper

func TestWebsocketEndpointAllowsOwnPeerStatsForNonAdmin(t *testing.T) {
	bus := evbus.New(10)
	conn, cleanup := newTestWebsocketConnection(t, bus, &domain.ContextUserInfo{Id: "user-a"},
		map[domain.PeerIdentifier]*domain.Peer{
			"own-peer": {Identifier: "own-peer", UserIdentifier: "user-a"},
		})
	defer cleanup()

	bus.Publish(app.TopicPeerStatsUpdated, domain.TrafficDelta{EntityId: "own-peer", BytesReceivedPerSecond: 1})
	assertWebsocketMessage(t, conn, "peer_stats", "own-peer")
}

func TestWebsocketEndpointCleansExpiredPeerUserIdentifierCache(t *testing.T) {
	now := time.Now()
	endpoint := &WebsocketEndpoint{
		ownershipCache: map[domain.PeerIdentifier]peerUserIdentifierCacheEntry{
			"expired-peer": {
				userIdentifier: "user-a",
				expiresAt:      now.Add(-time.Second),
			},
			"active-peer": {
				userIdentifier: "user-b",
				expiresAt:      now.Add(time.Second),
			},
		},
	}

	endpoint.cleanupOwnerCache(now)

	if _, ok := endpoint.ownershipCache["expired-peer"]; ok {
		t.Fatal("expired peer cache entry was not removed")
	}
	if _, ok := endpoint.ownershipCache["active-peer"]; !ok {
		t.Fatal("active peer cache entry was removed")
	}
}

func TestWebsocketEndpointFiltersOtherPeerStatsForNonAdmin(t *testing.T) {
	bus := evbus.New(10)
	conn, cleanup := newTestWebsocketConnection(t, bus, &domain.ContextUserInfo{Id: "user-a"},
		map[domain.PeerIdentifier]*domain.Peer{
			"other-peer": {Identifier: "other-peer", UserIdentifier: "user-b"},
		})
	defer cleanup()

	bus.Publish(app.TopicPeerStatsUpdated, domain.TrafficDelta{EntityId: "other-peer", BytesReceivedPerSecond: 1})
	assertNoWebsocketMessage(t, conn)
}

func TestWebsocketEndpointFiltersUnknownPeerStatsForNonAdmin(t *testing.T) {
	bus := evbus.New(10)
	conn, cleanup := newTestWebsocketConnection(t, bus, &domain.ContextUserInfo{Id: "user-a"},
		map[domain.PeerIdentifier]*domain.Peer{
			"other-peer": {Identifier: "other-peer", UserIdentifier: ""},
		})
	defer cleanup()

	bus.Publish(app.TopicPeerStatsUpdated, domain.TrafficDelta{EntityId: "other-peer", BytesReceivedPerSecond: 1})
	assertNoWebsocketMessage(t, conn)
}

func TestWebsocketEndpointFiltersUnknownPeerStatsForNonAdmin2(t *testing.T) {
	bus := evbus.New(10)
	conn, cleanup := newTestWebsocketConnection(t, bus, &domain.ContextUserInfo{Id: "user-a"}, nil)
	defer cleanup()

	bus.Publish(app.TopicPeerStatsUpdated, domain.TrafficDelta{EntityId: "unknown-peer", BytesReceivedPerSecond: 1})
	assertNoWebsocketMessage(t, conn)
}

func TestWebsocketEndpointFiltersInterfaceStatsForNonAdmin(t *testing.T) {
	bus := evbus.New(10)
	conn, cleanup := newTestWebsocketConnection(t, bus, &domain.ContextUserInfo{Id: "user-a"}, nil)
	defer cleanup()

	bus.Publish(app.TopicInterfaceStatsUpdated, domain.TrafficDelta{EntityId: "wg0", BytesReceivedPerSecond: 1})
	assertNoWebsocketMessage(t, conn)
}

func TestWebsocketEndpointAllowsAllStatsForAdmin(t *testing.T) {
	bus := evbus.New(10)
	conn, cleanup := newTestWebsocketConnection(t, bus, &domain.ContextUserInfo{Id: "admin", IsAdmin: true}, nil)
	defer cleanup()

	bus.Publish(app.TopicPeerStatsUpdated, domain.TrafficDelta{EntityId: "other-peer", BytesReceivedPerSecond: 1})
	assertWebsocketMessage(t, conn, "peer_stats", "other-peer")

	bus.Publish(app.TopicInterfaceStatsUpdated, domain.TrafficDelta{EntityId: "wg0", BytesReceivedPerSecond: 1})
	assertWebsocketMessage(t, conn, "interface_stats", "wg0")
}

func Test_matchOrigin(t *testing.T) {
	tests := []struct {
		name            string
		externalBaseUrl string
		origin          string
		want            bool
	}{
		{
			name:            "matching origin",
			externalBaseUrl: "https://example.com",
			origin:          "https://example.com",
			want:            true,
		},
		{
			name:            "matching origin with path",
			externalBaseUrl: "https://example.com/app1",
			origin:          "https://example.com/app2",
			want:            true,
		},
		{
			name:            "non-matching origin with different host",
			externalBaseUrl: "https://example.com",
			origin:          "https://example.com.malicious.com",
			want:            false,
		},
		{
			name:            "non-matching origin with different scheme",
			externalBaseUrl: "https://example.com",
			origin:          "http://example.com",
			want:            false,
		},
		{
			name:            "invalid origin URL",
			externalBaseUrl: "https://example.com",
			origin:          "://invalid-url",
			want:            false,
		},
		{
			name:            "invalid externalBaseUrl",
			externalBaseUrl: "://invalid-url",
			origin:          "https://example.com",
			want:            false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchOrigin(tt.externalBaseUrl, tt.origin)
			if got != tt.want {
				t.Errorf("matchOrigin() = %v, want %v", got, tt.want)
			}
		})
	}
}
