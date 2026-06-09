package adapters

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gorm.io/gorm"

	"github.com/fedor-git/wg-portal-2/internal"
	"github.com/fedor-git/wg-portal-2/internal/config"
	"github.com/fedor-git/wg-portal-2/internal/domain"
)

type HealthStatus struct {
	Health        bool      `json:"health"`
	DatabaseOk    bool      `json:"database_ok"`
	SyncOk        bool      `json:"sync_ok"`
	LastSyncError string    `json:"last_sync_error,omitempty"`
	LastCheckTime time.Time `json:"last_check_time"`
}

type MetricsServer struct {
	*http.Server

	DB  *gorm.DB       // Database connection
	cfg *config.Config // Configuration

	registry *prometheus.Registry

	ifaceReceivedBytesTotal  *prometheus.GaugeVec
	ifaceSendBytesTotal      *prometheus.GaugeVec
	peerIsConnected          *prometheus.GaugeVec
	peerLastHandshakeSeconds *prometheus.GaugeVec
	peerReceivedBytesTotal   *prometheus.GaugeVec
	peerSendBytesTotal       *prometheus.GaugeVec

	healthStatus *HealthStatus
	healthMutex  sync.RWMutex
}

// Wireguard metrics labels
var (
	ifaceLabels = []string{"interface"}
	peerLabels  = []string{"interface", "addresses", "id", "name"}
)

// NewMetricsServer returns a new prometheus server
func NewMetricsServer(cfg *config.Config, db *gorm.DB) *MetricsServer {
	// Create a new custom registry
	reg := prometheus.NewRegistry()

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))

	ms := &MetricsServer{
		Server: &http.Server{
			Addr:    cfg.Statistics.ListeningAddress,
			Handler: mux,
		},
		DB:       db,
		cfg:      cfg,
		registry: reg,

		healthStatus: &HealthStatus{
			Health:        true,
			DatabaseOk:    true,
			SyncOk:        true,
			LastCheckTime: time.Now(),
		},

		ifaceReceivedBytesTotal: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "wireguard_interface_received_bytes_total",
				Help: "Bytes received through the interface.",
			}, ifaceLabels,
		),
		ifaceSendBytesTotal: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "wireguard_interface_sent_bytes_total",
				Help: "Bytes sent through the interface.",
			}, ifaceLabels,
		),

		peerIsConnected: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "wireguard_peer_up",
				Help: "Peer connection state (boolean: 1/0).",
			}, peerLabels,
		),
		peerLastHandshakeSeconds: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "wireguard_peer_last_handshake_seconds",
				Help: "Seconds from the last handshake with the peer.",
			}, peerLabels,
		),
		peerReceivedBytesTotal: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "wireguard_peer_received_bytes_total",
				Help: "Bytes received from the peer.",
			}, peerLabels,
		),
		peerSendBytesTotal: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "wireguard_peer_sent_bytes_total",
				Help: "Bytes sent to the peer.",
			}, peerLabels,
		),
	}

	reg.MustRegister(
		ms.ifaceReceivedBytesTotal,
		ms.ifaceSendBytesTotal,
		ms.peerIsConnected,
	)

	if cfg.Statistics.ExportDetailedPeerMetrics {
		reg.MustRegister(
			ms.peerLastHandshakeSeconds,
			ms.peerReceivedBytesTotal,
			ms.peerSendBytesTotal,
		)
		slog.Info("Detailed peer metrics enabled (handshake, bytes received/transmitted)")
	} else {
		slog.Info("Detailed peer metrics disabled (only peer_up will be exported)")
	}

	// Add health check endpoint
	mux.HandleFunc("/health", ms.handleHealth)

	return ms
}

// Run starts the metrics server. The function blocks until the context is cancelled.
// Metrics endpoint is always available regardless of database or sync status.
// Use /health endpoint to check cluster health status.
func (m *MetricsServer) Run(ctx context.Context) {
	// Run the metrics server in a goroutine
	go func() {
		if err := m.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("metrics service exited", "address", m.Addr, "error", err)
		}
	}()

	slog.Info("started metrics service", "address", m.Addr)

	// Wait for the context to be done
	<-ctx.Done()

	// Create a context with timeout for the shutdown process
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Attempt to gracefully shut down the metrics server
	if err := m.Shutdown(shutdownCtx); err != nil {
		slog.Error("metrics service shutdown failed", "address", m.Addr, "error", err)
	} else {
		slog.Info("metrics service shutdown gracefully", "address", m.Addr)
	}
}

// monitorHealth checks if both DB and sync are down. If so, closes the server to signal unhealthiness.

// UpdateInterfaceMetrics updates the metrics for the given interface
func (m *MetricsServer) UpdateInterfaceMetrics(status domain.InterfaceStatus) {
	labels := []string{string(status.InterfaceId)}
	m.ifaceReceivedBytesTotal.WithLabelValues(labels...).Set(float64(status.BytesReceived))
	m.ifaceSendBytesTotal.WithLabelValues(labels...).Set(float64(status.BytesTransmitted))

	// Add debug logs for interface metrics registration
	slog.Debug("Registering interface metrics", "labels", labels, "interfaceID", status.InterfaceId)
	slog.Debug("Setting ifaceReceivedBytesTotal", "value", status.BytesReceived)
	slog.Debug("Setting ifaceSendBytesTotal", "value", status.BytesTransmitted)
}

// RegisterPeerMetrics registers metrics for a peer without removing existing ones.
// This should be called ONCE when a peer is created or on startup.
// It does NOT remove existing metrics, making it efficient for initial registration.
func (m *MetricsServer) RegisterPeerMetrics(peer *domain.Peer) {
	labels := []string{
		string(peer.InterfaceIdentifier),
		peer.CheckAliveAddress(),
		string(peer.Identifier),
		peer.DisplayName,
	}

	if labels[2] == "" {
		slog.Warn("Skip RegisterPeerMetrics: id label is empty", "peerID", peer.Identifier)
		return
	}

	// Initialize metrics if they don't already exist
	// WithLabelValues is idempotent - calling twice with same labels is safe
	m.peerIsConnected.WithLabelValues(labels...).Add(0)

	if m.cfg.Statistics.ExportDetailedPeerMetrics {
		m.peerLastHandshakeSeconds.WithLabelValues(labels...).Add(0)
		m.peerReceivedBytesTotal.WithLabelValues(labels...).Add(0)
		m.peerSendBytesTotal.WithLabelValues(labels...).Add(0)
	}
}

// UpdatePeerMetricsValues updates ONLY the values of metrics for the given peer.
// This should be called frequently (on every statistics collection).
// It does NOT re-register or remove metrics, making it very efficient.
// If labels have changed (name, address), call RegisterPeerMetrics to update them.
// NOTE: This function uses lazy registration - metrics are registered on first update if needed
func (m *MetricsServer) UpdatePeerMetricsValues(peer *domain.Peer, status domain.PeerStatus) {
	labels := []string{
		string(peer.InterfaceIdentifier),
		peer.CheckAliveAddress(),
		string(status.PeerId),
		peer.DisplayName,
	}

	if labels[2] == "" {
		slog.Warn("Skip UpdatePeerMetricsValues: id label is empty", "peerID", peer.Identifier)
		return
	}

	// Lazy registration: metrics are initialized on first use
	// This ensures metrics exist before we try to set values
	// WithLabelValues is idempotent - calling multiple times is safe

	// CRITICAL FIX: Recalculate IsConnected based on fresh LastHandshake data
	// This ensures metrics reflect actual peer status, not stale cached values
	// CalcConnected() uses: peer is online if handshake is within last 2 minutes OR IsPingable
	status.CalcConnected()
	m.peerIsConnected.WithLabelValues(labels...).Set(internal.BoolToFloat64(status.IsConnected))

	// ALWAYS update LastHandshake - it's the most important metric for lifecycle tracking
	// This metric is needed regardless of export_detailed_peer_metrics setting
	if status.LastHandshake != nil {
		m.peerLastHandshakeSeconds.WithLabelValues(labels...).Set(float64(status.LastHandshake.Unix()))
	}

	// Only export detailed metrics (bytes, etc) if configured
	// This reduces load when export_detailed_peer_metrics: false
	if m.cfg.Statistics.ExportDetailedPeerMetrics {
		m.peerReceivedBytesTotal.WithLabelValues(labels...).Set(float64(status.BytesReceived))
		m.peerSendBytesTotal.WithLabelValues(labels...).Set(float64(status.BytesTransmitted))
	}
}

// UpdatePeerMetrics is deprecated - use RegisterPeerMetrics() on startup and UpdatePeerMetricsValues() in loops instead
func (m *MetricsServer) UpdatePeerMetrics(peer *domain.Peer, status domain.PeerStatus) {
	// For backwards compatibility, delegate to the new split functions
	m.RegisterPeerMetrics(peer)
	m.UpdatePeerMetricsValues(peer, status)
}

// removePeerMetricsByIDInternal is an internal method to remove peer metrics without verbose logging
func (m *MetricsServer) removePeerMetricsByIDInternal(peerId string) {
	mfs, err := m.registry.Gather()
	if err != nil {
		return
	}

	metricMap := map[string]*prometheus.GaugeVec{
		"wireguard_peer_up": m.peerIsConnected,
	}

	if m.cfg.Statistics.ExportDetailedPeerMetrics {
		metricMap["wireguard_peer_last_handshake_seconds"] = m.peerLastHandshakeSeconds
		metricMap["wireguard_peer_received_bytes_total"] = m.peerReceivedBytesTotal
		metricMap["wireguard_peer_sent_bytes_total"] = m.peerSendBytesTotal
	}

	for _, mf := range mfs {
		name := mf.GetName()
		vec, ok := metricMap[name]
		if !ok {
			continue
		}
		for _, mtr := range mf.GetMetric() {
			var labelValues []string
			var found bool
			for _, label := range mtr.GetLabel() {
				if label.GetName() == "id" && label.GetValue() == peerId {
					found = true
				}
			}
			if found {
				// Restore label values in correct order
				for _, l := range peerLabels {
					val := ""
					for _, label := range mtr.GetLabel() {
						if label.GetName() == l {
							val = label.GetValue()
							break
						}
					}
					labelValues = append(labelValues, val)
				}
				vec.DeleteLabelValues(labelValues...)
			}
		}
	}
}

// Remove all peer metrics by id, regardless of other label values
func (m *MetricsServer) RemovePeerMetrics(peer *domain.Peer) {
	if peer == nil {
		slog.Warn("Attempted to remove metrics for a nil peer")
		return
	}

	peerId := string(peer.Identifier)
	slog.Debug("Starting removal of metrics for peer by id", "id", peerId, "name", peer.DisplayName)

	mfs, err := m.registry.Gather()
	if err != nil {
		slog.Warn("Failed to gather metrics for removal", "err", err)
		return
	}

	metricMap := map[string]*prometheus.GaugeVec{
		"wireguard_peer_up": m.peerIsConnected,
	}

	if m.cfg.Statistics.ExportDetailedPeerMetrics {
		metricMap["wireguard_peer_last_handshake_seconds"] = m.peerLastHandshakeSeconds
		metricMap["wireguard_peer_received_bytes_total"] = m.peerReceivedBytesTotal
		metricMap["wireguard_peer_sent_bytes_total"] = m.peerSendBytesTotal
	}

	for _, mf := range mfs {
		name := mf.GetName()
		vec, ok := metricMap[name]
		if !ok {
			continue
		}
		for _, mtr := range mf.GetMetric() {
			var labelValues []string
			var found bool
			for _, label := range mtr.GetLabel() {
				if label.GetName() == "id" && label.GetValue() == peerId {
					found = true
				}
			}
			if found {
				for _, l := range peerLabels {
					val := ""
					for _, label := range mtr.GetLabel() {
						if label.GetName() == l {
							val = label.GetValue()
							break
						}
					}
					labelValues = append(labelValues, val)
				}
				vec.DeleteLabelValues(labelValues...)
				slog.Debug("Removed metric by id", "metric", name, "id", peerId, "labels", labelValues)
			}
		}
	}

	slog.Info("Completed removal of metrics for peer by id", "id", peerId, "name", peer.DisplayName)
}

// Remove all peer metrics by id only (for when peer object is no longer available)
func (m *MetricsServer) RemovePeerMetricsByID(peerId string) {
	slog.Debug("Starting removal of metrics for peer by id", "id", peerId, "name", "unknown")

	mfs, err := m.registry.Gather()
	if err != nil {
		slog.Warn("Failed to gather metrics for removal", "err", err)
		return
	}

	metricMap := map[string]*prometheus.GaugeVec{
		"wireguard_peer_up": m.peerIsConnected,
	}

	if m.cfg.Statistics.ExportDetailedPeerMetrics {
		metricMap["wireguard_peer_last_handshake_seconds"] = m.peerLastHandshakeSeconds
		metricMap["wireguard_peer_received_bytes_total"] = m.peerReceivedBytesTotal
		metricMap["wireguard_peer_sent_bytes_total"] = m.peerSendBytesTotal
	}

	for _, mf := range mfs {
		name := mf.GetName()
		vec, ok := metricMap[name]
		if !ok {
			continue
		}
		for _, mtr := range mf.GetMetric() {
			var labelValues []string
			var found bool
			for _, label := range mtr.GetLabel() {
				if label.GetName() == "id" && label.GetValue() == peerId {
					found = true
				}
			}
			if found {
				for _, l := range peerLabels {
					val := ""
					for _, label := range mtr.GetLabel() {
						if label.GetName() == l {
							val = label.GetValue()
							break
						}
					}
					labelValues = append(labelValues, val)
				}
				vec.DeleteLabelValues(labelValues...)
				slog.Debug("Removed metric by id", "metric", name, "id", peerId, "labels", labelValues)
			}
		}
	}

	slog.Info("Completed removal of metrics for peer by id", "id", peerId, "name", "unknown")
}

// CleanupOrphanedPeerMetrics removes metrics for all peers that no longer exist in the database.
// This is called on startup to clean up leftover metrics from deleted peers.
// Returns the count of metrics cleaned up.
func (m *MetricsServer) CleanupOrphanedPeerMetrics(ctx context.Context, db *gorm.DB) (int, error) {
	type PeerStatus struct {
		PeerId string `gorm:"column:identifier"`
	}

	// Get all peer_status records that don't have corresponding peers
	var orphanedStatuses []PeerStatus
	result := db.WithContext(ctx).Raw(`
		SELECT ps.identifier FROM peer_statuses ps
		LEFT JOIN peers p ON ps.identifier = p.identifier
		WHERE p.identifier IS NULL
	`).Scan(&orphanedStatuses)

	if result.Error != nil {
		return 0, fmt.Errorf("failed to find orphaned peer statuses: %w", result.Error)
	}

	// Remove metrics for each orphaned peer
	for _, status := range orphanedStatuses {
		m.RemovePeerMetricsByID(status.PeerId)
	}

	return len(orphanedStatuses), nil
}

// SetSyncStatus updates the sync status for the health check
func (m *MetricsServer) SetSyncStatus(ok bool, errMsg string) {
	m.healthMutex.Lock()
	defer m.healthMutex.Unlock()

	m.healthStatus.SyncOk = ok
	if !ok {
		m.healthStatus.LastSyncError = errMsg
	} else {
		m.healthStatus.LastSyncError = ""
	}
	m.healthStatus.LastCheckTime = time.Now()
}

// GetHealth returns the current health status
func (m *MetricsServer) GetHealth() HealthStatus {
	m.healthMutex.RLock()
	defer m.healthMutex.RUnlock()

	// Check DB connection
	health := *m.healthStatus
	if m.DB.Exec("SELECT 1").Error != nil {
		health.DatabaseOk = false
	} else {
		health.DatabaseOk = true
	}

	// Overall health is OK only if both DB and sync are OK
	health.Health = health.DatabaseOk && health.SyncOk

	return health
}

// handleHealth handles the /health endpoint
func (m *MetricsServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	health := m.GetHealth()

	w.Header().Set("Content-Type", "application/json")

	if !health.Health {
		w.WriteHeader(http.StatusServiceUnavailable) // 503
	} else {
		w.WriteHeader(http.StatusOK) // 200
	}

	json.NewEncoder(w).Encode(health)
}
