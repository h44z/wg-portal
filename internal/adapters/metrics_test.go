package adapters

import (
	"testing"

	"github.com/h44z/wg-portal/internal/domain"
	"github.com/prometheus/client_golang/prometheus"
)

func TestDeletePeerMetricsRemovesAllPeerSeries(t *testing.T) {
	labels := []string{"interface", "addresses", "id", "name", "user"}
	m := &MetricsServer{
		peerIsConnected:          prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "peer_up_test"}, labels),
		peerLastHandshakeSeconds: prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "peer_handshake_test"}, labels),
		peerReceivedBytesTotal:   prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "peer_received_test"}, labels),
		peerSendBytesTotal:       prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "peer_sent_test"}, labels),
	}
	addr, err := domain.CidrFromString("10.0.0.2/32")
	if err != nil {
		t.Fatal(err)
	}
	peer := &domain.Peer{
		Identifier:          "peer-id",
		InterfaceIdentifier: "wg0",
		DisplayName:         "peer",
		UserIdentifier:      "user",
		Interface:           domain.PeerInterfaceConfig{Addresses: []domain.Cidr{addr}},
	}
	m.UpdatePeerMetrics(peer, domain.PeerStatus{PeerId: peer.Identifier, IsConnected: true})
	m.DeletePeerMetrics(*peer)

	for name, collector := range map[string]prometheus.Collector{
		"up": m.peerIsConnected, "handshake": m.peerLastHandshakeSeconds,
		"received": m.peerReceivedBytesTotal, "sent": m.peerSendBytesTotal,
	} {
		ch := make(chan prometheus.Metric, 1)
		collector.Collect(ch)
		close(ch)
		if _, ok := <-ch; ok {
			t.Errorf("%s metric remained after peer deletion", name)
		}
	}
}

func TestDeletePeerMetricsLeavesRetainedPeerSeries(t *testing.T) {
	labels := []string{"interface", "addresses", "id", "name", "user"}
	m := &MetricsServer{
		peerIsConnected:          prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "peer_up_retained_test"}, labels),
		peerLastHandshakeSeconds: prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "peer_handshake_retained_test"}, labels),
		peerReceivedBytesTotal:   prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "peer_received_retained_test"}, labels),
		peerSendBytesTotal:       prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "peer_sent_retained_test"}, labels),
	}
	deletedAddr, err := domain.CidrFromString("10.0.0.2/32")
	if err != nil {
		t.Fatal(err)
	}
	retainedAddr, err := domain.CidrFromString("10.0.0.3/32")
	if err != nil {
		t.Fatal(err)
	}
	deleted := &domain.Peer{Identifier: "deleted", InterfaceIdentifier: "wg0", DisplayName: "deleted", UserIdentifier: "user", Interface: domain.PeerInterfaceConfig{Addresses: []domain.Cidr{deletedAddr}}}
	retained := &domain.Peer{Identifier: "retained", InterfaceIdentifier: "wg0", DisplayName: "retained", UserIdentifier: "user", Interface: domain.PeerInterfaceConfig{Addresses: []domain.Cidr{retainedAddr}}}
	m.UpdatePeerMetrics(deleted, domain.PeerStatus{PeerId: deleted.Identifier, IsConnected: false})
	m.UpdatePeerMetrics(retained, domain.PeerStatus{PeerId: retained.Identifier, IsConnected: false})
	m.DeletePeerMetrics(*deleted)

	ch := make(chan prometheus.Metric, 2)
	m.peerIsConnected.Collect(ch)
	close(ch)
	count := 0
	for range ch {
		count++
	}
	if count != 1 {
		t.Fatalf("expected one retained peer series, got %d", count)
	}
}
