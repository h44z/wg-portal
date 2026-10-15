package wireguard

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/go-pkgz/routegroup"
	"github.com/gorilla/websocket"
	"github.com/h44z/wg-portal/internal/adapters"
	"github.com/h44z/wg-portal/internal/app"
	"github.com/h44z/wg-portal/internal/app/api/v0/handlers"
	"github.com/h44z/wg-portal/internal/domain"
	"github.com/stretchr/testify/require"
	evbus "github.com/vardius/message-bus"
)

// Only authentication is replaced; the real endpoint performs the ownership
// check, SQLite query, caching, and WebSocket writes for a non-admin user.
type statisticsWebsocketAuth struct{ handlers.Authenticator }

func (statisticsWebsocketAuth) LoggedIn(...handlers.Scope) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(domain.SetUserInfo(r.Context(), &domain.ContextUserInfo{Id: "user"})))
		})
	}
}

type statisticsWebsocketBus struct {
	evbus.MessageBus
	ready chan struct{}
}

func (b statisticsWebsocketBus) Subscribe(topic string, fn any) error {
	err := b.MessageBus.Subscribe(topic, fn)
	// The endpoint subscribes to peer stats before interface stats.
	if err == nil && topic == app.TopicInterfaceStatsUpdated {
		close(b.ready)
	}
	return err
}

type statisticsOwnershipService struct {
	*adapters.SqlRepo
	entered chan struct{}
	release chan struct{}
	result  chan error
	once    *sync.Once
}

func (s statisticsOwnershipService) GetPeer(ctx context.Context, id domain.PeerIdentifier) (*domain.Peer, error) {
	s.once.Do(func() { close(s.entered) })
	select {
	case <-s.release:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	// Bound the old deadlock as well, so a failing regression cleans up all workers.
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	peer, err := s.SqlRepo.GetPeer(ctx, id)
	s.result <- err
	return peer, err
}

type statisticsBackpressureRepo struct {
	*statisticsTestRepo
	bus       evbus.MessageBus
	ownership statisticsOwnershipService
}

func (r statisticsBackpressureRepo) UpdatePeerStatus(ctx context.Context, id domain.PeerIdentifier,
	update func(*domain.PeerStatus) (*domain.PeerStatus, error)) error {
	return r.SqlRepo.UpdatePeerStatus(ctx, id, func(status *domain.PeerStatus) (*domain.PeerStatus, error) {
		// The transaction now owns SQLite's only connection. Start the real
		// ownership handler, park it, then fill its one-slot subscriber queue.
		event := domain.TrafficDelta{EntityId: string(id)}
		r.bus.Publish(app.TopicPeerStatsUpdated, event)
		select {
		case <-r.ownership.entered:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		r.bus.Publish(app.TopicPeerStatsUpdated, event)
		close(r.ownership.release)
		// The collector's third Publish must happen after this transaction returns.
		return update(status)
	})
}

func TestStatisticsBoundedBusWithNonAdminWebsocket(t *testing.T) {
	collector, db, metrics := newStatisticsTestCollector(t)
	repo := collector.db.(*statisticsTestRepo)
	bus := statisticsWebsocketBus{MessageBus: evbus.New(1), ready: make(chan struct{})}
	ownership := statisticsOwnershipService{SqlRepo: repo.SqlRepo,
		entered: make(chan struct{}), release: make(chan struct{}), result: make(chan error, 4), once: &sync.Once{}}
	collector.db = statisticsBackpressureRepo{statisticsTestRepo: repo, bus: bus, ownership: ownership}
	collector.bus = bus

	state := make(chan domain.PeerStatus, 1)
	require.NoError(t, bus.Subscribe(app.TopicPeerStateChanged, func(status domain.PeerStatus, _ domain.Peer) { state <- status }))
	defer bus.Close(app.TopicPeerStateChanged)
	mux := http.NewServeMux()
	endpointDone := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mux.ServeHTTP(w, r)
		close(endpointDone)
	}))
	t.Cleanup(server.Close)
	collector.cfg.Web.ExternalUrl = server.URL
	endpoint := handlers.NewWebsocketEndpoint(collector.cfg, statisticsWebsocketAuth{}, bus, ownership)
	endpoint.RegisterRoutes(routegroup.New(mux))
	conn, _, err := websocket.DefaultDialer.Dial("ws"+server.URL[len("http"):]+"/ws",
		http.Header{"Origin": []string{server.URL}})
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close(); receiveStatisticsTest(t, endpointDone) })
	receiveStatisticsTest(t, bus.ready)

	// A separate reader models login/API traffic competing for the same pool.
	readResult := make(chan error, 1)
	var readers sync.WaitGroup
	readers.Add(1)
	go func() {
		defer readers.Done()
		<-ownership.entered
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		var user domain.User
		readResult <- db.WithContext(ctx).First(&user, "identifier = ?", "user").Error
	}()
	t.Cleanup(readers.Wait)
	startStatisticsTestCollector(t, collector, "peer")
	require.NoError(t, receiveStatisticsTest(t, readResult), "concurrent API query must not be starved by a blocked Publish")
	require.NoError(t, receiveStatisticsTest(t, ownership.result), "ownership query must not wait on the publishing transaction")

	require.NoError(t, conn.SetReadDeadline(time.Now().Add(time.Second)))
	for range 3 {
		var message struct {
			Type string              `json:"type"`
			Data domain.TrafficDelta `json:"data"`
		}
		require.NoError(t, conn.ReadJSON(&message))
		require.Equal(t, "peer_stats", message.Type)
		require.Equal(t, "peer", message.Data.EntityId)
	}
	require.True(t, receiveStatisticsTest(t, state).IsConnected)
	receiveStatisticsTest(t, metrics.statuses)
	var status domain.PeerStatus
	require.NoError(t, db.First(&status, "identifier = ?", "peer").Error)
	require.EqualValues(t, 42, status.BytesReceived)
}
