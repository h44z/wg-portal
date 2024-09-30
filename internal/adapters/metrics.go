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

	ifaceIsDisabled          *prometheus.GaugeVec
	ifaceReceivedBytesTotal  *prometheus.GaugeVec
	ifaceSendBytesTotal      *prometheus.GaugeVec
	peerIsConnected          *prometheus.GaugeVec
	peerLastHandshakeSeconds *prometheus.GaugeVec
	peerReceivedBytesTotal   *prometheus.GaugeVec
	peerSendBytesTotal       *prometheus.GaugeVec
}

// Wireguard metrics labels
var (
	ifaceLabels = []string{"interface"}
	peerLabels  = []string{"interface", "addresses", "id", "name"}
)

// NewMetricsServer returns a new prometheus server
func NewMetricsServer(cfg *config.Config) *MetricsServer {
	reg := prometheus.NewRegistry()

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{Registry: reg}))

	return &MetricsServer{
		Server: &http.Server{
			Addr:    cfg.Statistics.ListeningAddress,
			Handler: mux,
		},

		ifaceIsDisabled: promauto.With(reg).NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "wireguard_interface_up",
				Help: "Iterface state (boolean: 1/0).",
			}, ifaceLabels,
		),

		ifaceReceivedBytesTotal: promauto.With(reg).NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "wireguard_interface_received_bytes_total",
				Help: "Bytes received througth the interface.",
			}, ifaceLabels,
		),
		ifaceSendBytesTotal: promauto.With(reg).NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "wireguard_interface_sent_bytes_total",
				Help: "Bytes sent through the interface.",
			}, ifaceLabels,
		),

		peerIsConnected: promauto.With(reg).NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "wireguard_peer_up",
				Help: "Peer connection state (boolean: 1/0).",
			}, peerLabels,
		),
		peerLastHandshakeSeconds: promauto.With(reg).NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "wireguard_peer_last_handshake_seconds",
				Help: "Seconds from the last handshake with the peer.",
			}, peerLabels,
		),
		peerReceivedBytesTotal: promauto.With(reg).NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "wireguard_peer_received_bytes_total",
				Help: "Bytes received from the peer.",
			}, peerLabels,
		),
		peerSendBytesTotal: promauto.With(reg).NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "wireguard_peer_sent_bytes_total",
				Help: "Bytes sent to the peer.",
			}, peerLabels,
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
func (m *MetricsServer) UpdateInterfaceMetrics(iface *domain.Interface, status domain.InterfaceStatus) {
	labels := []string{string(status.InterfaceId)}
	m.ifaceIsDisabled.WithLabelValues(labels...).Set(internal.BoolToFloat64(iface.IsDisabled()))
	m.ifaceReceivedBytesTotal.WithLabelValues(labels...).Set(float64(status.BytesReceived))
	m.ifaceSendBytesTotal.WithLabelValues(labels...).Set(float64(status.BytesTransmitted))
}

// UpdatePeerMetrics updates the metrics for the given peer
func (m *MetricsServer) UpdatePeerMetrics(peer *domain.Peer, status domain.PeerStatus) {
	labels := []string{
		string(peer.InterfaceIdentifier),
		string(peer.Interface.AddressStr()),
		string(status.PeerId),
		string(peer.DisplayName),
	}

	if status.LastHandshake != nil {
		m.peerLastHandshakeSeconds.WithLabelValues(labels...).Set(float64(status.LastHandshake.Unix()))
	}
	m.peerReceivedBytesTotal.WithLabelValues(labels...).Set(float64(status.BytesReceived))
	m.peerSendBytesTotal.WithLabelValues(labels...).Set(float64(status.BytesTransmitted))
	m.peerIsConnected.WithLabelValues(labels...).Set(internal.BoolToFloat64(status.IsConnected()))
}
