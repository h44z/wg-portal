package adapters

import (
	"context"
	"net/http"
	"time"

	"github.com/h44z/wg-portal/internal"
	"github.com/h44z/wg-portal/internal/config"
	"github.com/h44z/wg-portal/internal/domain"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
)

type MetricsServer struct {
	*http.Server
	db *SqlRepo

	ifaceInfo                *prometheus.GaugeVec
	ifaceReceivedBytesTotal  *prometheus.GaugeVec
	ifaceSendBytesTotal      *prometheus.GaugeVec
	peerInfo                 *prometheus.GaugeVec
	peerIsConnected          *prometheus.GaugeVec
	peerLastHandshakeSeconds *prometheus.GaugeVec
	peerReceivedBytesTotal   *prometheus.GaugeVec
	peerSendBytesTotal       *prometheus.GaugeVec
}

// Wireguard metrics labels
var (
	labels      = []string{"interface"}
	ifaceLabels = []string{}
	peerLabels  = []string{"addresses", "id", "name"}
)

// NewMetricsServer returns a new prometheus server
func NewMetricsServer(cfg *config.Config, db *SqlRepo) *MetricsServer {
	reg := prometheus.NewRegistry()

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{Registry: reg}))

	return &MetricsServer{
		Server: &http.Server{
			Addr:    cfg.Statistics.ListeningAddress,
			Handler: mux,
		},
		db: db,

		ifaceInfo: promauto.With(reg).NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "wireguard_interface_info",
				Help: "Interface info.",
			}, append(labels, ifaceLabels...),
		),
		ifaceReceivedBytesTotal: promauto.With(reg).NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "wireguard_interface_received_bytes_total",
				Help: "Bytes received througth the interface.",
			}, append(labels, ifaceLabels...),
		),
		ifaceSendBytesTotal: promauto.With(reg).NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "wireguard_interface_sent_bytes_total",
				Help: "Bytes sent through the interface.",
			}, append(labels, ifaceLabels...),
		),

		peerInfo: promauto.With(reg).NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "wireguard_peer_info",
				Help: "Peer info.",
			}, append(labels, peerLabels...),
		),
		peerIsConnected: promauto.With(reg).NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "wireguard_peer_up",
				Help: "Peer connection state (boolean: 1/0).",
			}, append(labels, peerLabels...),
		),
		peerLastHandshakeSeconds: promauto.With(reg).NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "wireguard_peer_last_handshake_seconds",
				Help: "Seconds from the last handshake with the peer.",
			}, append(labels, peerLabels...),
		),
		peerReceivedBytesTotal: promauto.With(reg).NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "wireguard_peer_received_bytes_total",
				Help: "Bytes received from the peer.",
			}, append(labels, peerLabels...),
		),
		peerSendBytesTotal: promauto.With(reg).NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "wireguard_peer_sent_bytes_total",
				Help: "Bytes sent to the peer.",
			}, append(labels, peerLabels...),
		),
	}
}

// Run starts the metrics server
func (m *MetricsServer) Run(ctx context.Context) {
	// Run the metrics server in a goroutine
	go func() {
		if err := m.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logrus.Errorf("metrics service on %s exited: %v", m.Addr, err)
		}
	}()

	logrus.Infof("started metrics service on %s", m.Addr)

	// Wait for the context to be done
	<-ctx.Done()

	// Create a context with timeout for the shutdown process
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Attempt to gracefully shutdown the metrics server
	if err := m.Shutdown(shutdownCtx); err != nil {
		logrus.Errorf("metrics service on %s shutdown failed: %v", m.Addr, err)
	} else {
		logrus.Infof("metrics service on %s shutdown gracefully", m.Addr)
	}
}

// UpdateInterfaceMetrics updates the metrics for the given interface
func (m *MetricsServer) UpdateInterfaceMetrics(status domain.InterfaceStatus) {
	labels := []string{string(status.InterfaceId)}
	m.ifaceInfo.WithLabelValues(labels...).Set(1)
	m.ifaceReceivedBytesTotal.WithLabelValues(labels...).Set(float64(status.BytesReceived))
	m.ifaceSendBytesTotal.WithLabelValues(labels...).Set(float64(status.BytesTransmitted))
}

// UpdatePeerMetrics updates the metrics for the given peer
func (m *MetricsServer) UpdatePeerMetrics(ctx context.Context, status domain.PeerStatus) {
	// Fetch peer data from the database
	peer, err := m.db.GetPeer(ctx, status.PeerId)
	if err != nil {
		logrus.Warnf("failed to fetch peer data for labels %s: %v", status.PeerId, err)
		return
	}

	labels := []string{
		string(peer.InterfaceIdentifier),
		string(peer.Interface.AddressStr()),
		string(status.PeerId),
		string(peer.DisplayName),
	}

	m.peerInfo.WithLabelValues(labels...).Set(1)
	if status.LastHandshake != nil {
		m.peerLastHandshakeSeconds.WithLabelValues(labels...).Set(float64(status.LastHandshake.Unix()))
	}
	m.peerReceivedBytesTotal.WithLabelValues(labels...).Set(float64(status.BytesReceived))
	m.peerSendBytesTotal.WithLabelValues(labels...).Set(float64(status.BytesTransmitted))
	m.peerIsConnected.WithLabelValues(labels...).Set(internal.BoolToFloat64(status.IsConnected()))
}
