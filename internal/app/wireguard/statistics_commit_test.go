package wireguard

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/h44z/wg-portal/internal/adapters"
	"github.com/h44z/wg-portal/internal/app"
	"github.com/h44z/wg-portal/internal/config"
	"github.com/h44z/wg-portal/internal/domain"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

// Only return interfaces once, so each test observes exactly one collection pass.
type statisticsTestRepo struct {
	*adapters.SqlRepo
	once sync.Once
}

func (r *statisticsTestRepo) GetAllInterfaces(ctx context.Context) (interfaces []domain.Interface, err error) {
	r.once.Do(func() { interfaces, err = r.SqlRepo.GetAllInterfaces(ctx) })
	return
}

type statisticsTestController struct{ domain.InterfaceController }

func (*statisticsTestController) GetInterface(_ context.Context, id domain.InterfaceIdentifier) (*domain.PhysicalInterface, error) {
	return &domain.PhysicalInterface{Identifier: id, BytesUpload: 42, BytesDownload: 24}, nil
}
func (*statisticsTestController) GetPeers(context.Context, domain.InterfaceIdentifier) ([]domain.PhysicalPeer, error) {
	return []domain.PhysicalPeer{{Identifier: "peer", BytesUpload: 42, BytesDownload: 24, LastHandshake: time.Now()}}, nil
}

type statisticsTestBus struct{ publish func(string, ...any) }

func (b statisticsTestBus) Publish(topic string, args ...any) { b.publish(topic, args...) }
func (statisticsTestBus) Subscribe(string, any) error         { return nil }

type statisticsTestMetrics struct{ statuses chan any }

func (m statisticsTestMetrics) UpdateInterfaceMetrics(s domain.InterfaceStatus) { m.statuses <- s }
func (m statisticsTestMetrics) UpdatePeerMetrics(_ *domain.Peer, s domain.PeerStatus) {
	m.statuses <- s
}
func (statisticsTestMetrics) DeletePeerMetrics(domain.Peer) {}

func newStatisticsTestCollector(t *testing.T) (*StatisticsCollector, *gorm.DB, statisticsTestMetrics) {
	t.Helper()
	schema.RegisterSerializer("encstr", app.NewGormEncryptedStringSerializer(""))
	cfg := &config.Config{}
	cfg.Database = config.DatabaseConfig{Type: config.DatabaseSQLite, DSN: filepath.Join(t.TempDir(), "statistics.db")}
	cfg.Statistics.DataCollectionInterval = time.Millisecond
	cfg.Backend.ReKeyTimeoutInterval = time.Minute
	db, err := adapters.NewDatabase(cfg.Database)
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.Equal(t, 1, sqlDB.Stats().MaxOpenConnections)
	repo, err := adapters.NewSqlRepository(db, cfg)
	require.NoError(t, err)
	require.NoError(t, db.Create(&domain.Interface{Identifier: "wg0"}).Error)
	require.NoError(t, db.Create(&domain.User{Identifier: "user"}).Error)
	require.NoError(t, db.Create(&domain.Peer{Identifier: "peer", UserIdentifier: "user", InterfaceIdentifier: "wg0"}).Error)
	old := time.Now().Add(-time.Minute)
	require.NoError(t, db.Create(&domain.PeerStatus{PeerId: "peer", UpdatedAt: old, BytesReceived: 2, BytesTransmitted: 1}).Error)
	require.NoError(t, db.Create(&domain.InterfaceStatus{InterfaceId: "wg0", UpdatedAt: old, BytesReceived: 2, BytesTransmitted: 1}).Error)
	metrics := statisticsTestMetrics{statuses: make(chan any, 4)}
	collector := &StatisticsCollector{
		cfg: cfg, db: &statisticsTestRepo{SqlRepo: repo}, ms: metrics,
		wg: &ControllerManager{controllers: map[domain.InterfaceBackend]backendInstance{
			config.LocalBackendName: {Implementation: &statisticsTestController{}},
		}},
	}
	return collector, db, metrics
}

func startStatisticsTestCollector(t *testing.T, collector *StatisticsCollector, kind string) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		if kind == "peer" {
			collector.collectPeerData(ctx)
		} else {
			collector.collectInterfaceData(ctx)
		}
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Error("collector did not stop")
		}
	})
}

func receiveStatisticsTest[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	select {
	case value := <-ch:
		return value
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for statistics collection")
	}
	var zero T
	return zero
}

// The subscriber reads through the real one-connection pool before Publish returns.
// Publishing inside the transaction cannot satisfy this read.
func TestStatisticsSubscribersReadCommittedStatus(t *testing.T) {
	for _, kind := range []string{"peer", "interface"} {
		t.Run(kind, func(t *testing.T) {
			collector, db, metrics := newStatisticsTestCollector(t)
			results := make(chan error, 1)
			state := make(chan domain.PeerStatus, 1)
			collector.bus = statisticsTestBus{publish: func(topic string, args ...any) {
				if topic == app.TopicPeerStateChanged {
					state <- args[0].(domain.PeerStatus)
					return
				}
				readCtx, cancel := context.WithTimeout(context.Background(), time.Second)
				defer cancel()
				if kind == "peer" {
					var status domain.PeerStatus
					err := db.WithContext(readCtx).First(&status, "identifier = ?", "peer").Error
					if err == nil && (status.BytesReceived != 42 || status.BytesTransmitted != 24 || !status.IsConnected) {
						err = errors.New("peer event preceded committed status")
					}
					results <- err
				} else {
					var status domain.InterfaceStatus
					err := db.WithContext(readCtx).First(&status, "identifier = ?", "wg0").Error
					if err == nil && (status.BytesReceived != 24 || status.BytesTransmitted != 42) {
						err = errors.New("interface event preceded committed status")
					}
					results <- err
				}
			}}
			startStatisticsTestCollector(t, collector, kind)
			require.NoError(t, receiveStatisticsTest(t, results))
			status := receiveStatisticsTest(t, metrics.statuses)
			if kind == "peer" {
				s := status.(domain.PeerStatus)
				require.EqualValues(t, 42, s.BytesReceived)
				require.EqualValues(t, 24, s.BytesTransmitted)
				require.True(t, s.IsConnected)
				require.Equal(t, s, receiveStatisticsTest(t, state))
			} else {
				s := status.(domain.InterfaceStatus)
				require.EqualValues(t, 24, s.BytesReceived)
				require.EqualValues(t, 42, s.BytesTransmitted)
			}
		})
	}
}

// Fail at the COMMIT boundary after the callback and real SQLite save succeed.
// Roll back the underlying transaction so the injected failure releases its connection.
type statisticsCommitFailurePool struct{ *sql.DB }

func (p statisticsCommitFailurePool) BeginTx(ctx context.Context, options *sql.TxOptions) (gorm.ConnPool, error) {
	tx, err := p.DB.BeginTx(ctx, options)
	if err != nil {
		return nil, err
	}
	return &statisticsCommitFailureTx{Tx: tx}, nil
}

type statisticsCommitFailureTx struct{ *sql.Tx }

func (tx statisticsCommitFailureTx) Commit() error {
	return errors.Join(errors.New("injected commit failure"), tx.Tx.Rollback())
}

func TestStatisticsRollbackDoesNotPublish(t *testing.T) {
	for _, kind := range []string{"peer", "interface"} {
		for _, failure := range []string{"save", "commit"} {
			t.Run(kind+"/"+failure, func(t *testing.T) {
				collector, db, metrics := newStatisticsTestCollector(t)
				if failure == "commit" {
					pool, err := db.DB()
					require.NoError(t, err)
					db.Statement.ConnPool = &statisticsCommitFailurePool{DB: pool}
				}
				failed := make(chan struct{}, 1)
				require.NoError(t, db.Callback().Update().Before("gorm:update").Register("test:reject-status", func(tx *gorm.DB) {
					if failure == "save" {
						tx.AddError(errors.New("injected status write failure"))
					}
					failed <- struct{}{}
				}))
				published := make(chan string, 4)
				collector.bus = statisticsTestBus{publish: func(topic string, _ ...any) { published <- topic }}
				startStatisticsTestCollector(t, collector, kind)
				receiveStatisticsTest(t, failed)
				// This read also waits for rollback to release the only connection.
				var peer domain.PeerStatus
				var iface domain.InterfaceStatus
				require.NoError(t, db.First(&peer, "identifier = ?", "peer").Error)
				require.NoError(t, db.First(&iface, "identifier = ?", "wg0").Error)
				require.EqualValues(t, 2, peer.BytesReceived)
				require.False(t, peer.IsConnected)
				require.EqualValues(t, 1, iface.BytesTransmitted)
				select {
				case topic := <-published:
					t.Errorf("rolled back update published %s", topic)
				case status := <-metrics.statuses:
					t.Errorf("rolled back update scheduled metrics: %+v", status)
				case <-time.After(100 * time.Millisecond):
				}
			})
		}
	}
}
