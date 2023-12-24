package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/h44z/wg-portal/internal/domain"
	"net/http"
	"strconv"
	"strings"
	"time"
)

var MikrotikDeviceType = "mikrotik"

// WgMikrotikRepo implements all low-level WireGuard interactions using the Mikrotik REST API.
// It uses the API endpoints described in https://help.mikrotik.com/docs/display/ROS/REST+API
type WgMikrotikRepo struct {
	apiClient *http.Client
	baseUrl   string
	user      string
	pass      string
}

func NewWgMikrotikRepo(baseUrl, user, pass string) *WgMikrotikRepo {
	apiClient := &http.Client{
		Timeout: 30 * time.Second,
	}

	return &WgMikrotikRepo{
		apiClient: apiClient,
		baseUrl:   baseUrl,
		user:      user,
		pass:      pass,
	}
}

func (w *WgMikrotikRepo) getFullUrl(endpoint string) string {
	return w.baseUrl + endpoint
}

func (w *WgMikrotikRepo) getRequest(ctx context.Context, method, endpoint string) (*http.Request, error) {
	url := w.getFullUrl(endpoint)
	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build REST request: %w", err)
	}
	req.SetBasicAuth(w.user, w.pass)
	req.Header.Set("Accept", "application/json")

	return req, nil
}

func closeHttpResponse(response *http.Response) {
	if response != nil && response.Body != nil {
		_ = response.Body.Close()
	}
}

func parseResponseError(response *http.Response) map[string]string {
	var restData map[string]string
	_ = json.NewDecoder(response.Body).Decode(&restData)
	return restData
}

func (w *WgMikrotikRepo) fetchList(ctx context.Context, endpoint string) ([]map[string]string, error) {
	req, err := w.getRequest(ctx, http.MethodGet, endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to build REST request for endpoint %s: %w", endpoint, err)
	}

	response, err := w.apiClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute REST request %s: %w", req.URL.String(), err)
	}
	defer closeHttpResponse(response)

	if response.StatusCode != http.StatusOK {
		errData := parseResponseError(response)
		return nil, fmt.Errorf("REST request %s returned status %d: %v", req.URL.String(), response.StatusCode, errData)
	}

	var restData []map[string]string // mikrotik API always returns values as strings
	err = json.NewDecoder(response.Body).Decode(&restData)
	if err != nil {
		return nil, fmt.Errorf("failed to decode REST response: %w", err)
	}

	return restData, nil
}

func (w *WgMikrotikRepo) fetchObject(ctx context.Context, endpoint string) (map[string]string, error) {
	req, err := w.getRequest(ctx, http.MethodGet, endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to build REST request for endpoint %s: %w", endpoint, err)
	}

	response, err := w.apiClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute REST request %s: %w", req.URL.String(), err)
	}
	defer closeHttpResponse(response)

	if response.StatusCode != http.StatusOK {
		errData := parseResponseError(response)
		return nil, fmt.Errorf("REST request %s returned status %d: %v", req.URL.String(), response.StatusCode, errData)
	}

	var restData map[string]string // mikrotik API always returns values as strings
	err = json.NewDecoder(response.Body).Decode(&restData)
	if err != nil {
		return nil, fmt.Errorf("failed to decode REST response: %w", err)
	}

	return restData, nil
}

func (w *WgMikrotikRepo) deleteObject(ctx context.Context, endpoint string) error {
	req, err := w.getRequest(ctx, http.MethodDelete, endpoint)
	if err != nil {
		return fmt.Errorf("failed to build REST request for endpoint %s: %w", endpoint, err)
	}

	response, err := w.apiClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute REST request %s: %w", req.URL.String(), err)
	}
	defer closeHttpResponse(response)

	if response.StatusCode != http.StatusNoContent {
		errData := parseResponseError(response)
		return fmt.Errorf("REST request %s returned status %d: %v", req.URL.String(), response.StatusCode, errData)
	}

	return nil
}

func (w *WgMikrotikRepo) createObject(ctx context.Context, endpoint string, data map[string]string) (map[string]string, error) {
	panic("implement me")
}

func (w *WgMikrotikRepo) GetInterfaces(ctx context.Context) ([]domain.PhysicalInterface, error) {
	restInterfaces, err := w.fetchList(ctx, "/interface/wireguard")
	if err != nil {
		return nil, fmt.Errorf("failed to get interfaces: %w", err)
	}

	restIPv4, err := w.fetchList(ctx, "/ip/address")
	if err != nil {
		return nil, fmt.Errorf("failed to get IPv4 addresses: %w", err)
	}

	restIPv6, err := w.fetchList(ctx, "/ipv6/address")
	if err != nil {
		return nil, fmt.Errorf("failed to get IPv6 addresses: %w", err)
	}

	var interfaces []domain.PhysicalInterface
	for _, restInterface := range restInterfaces {
		iface, err := w.parseInterfaceData(restInterface, restIPv4, restIPv6)
		if err != nil {
			continue
		}

		interfaces = append(interfaces, iface)
	}

	return interfaces, nil
}

func (w *WgMikrotikRepo) GetInterface(ctx context.Context, id domain.InterfaceIdentifier) (*domain.PhysicalInterface, error) {
	restInterface, err := w.fetchObject(ctx, "/interface/wireguard/"+string(id))
	if err != nil {
		return nil, fmt.Errorf("failed to get interface %s: %w", id, err)
	}

	restIPv4, err := w.fetchList(ctx, "/ip/address")
	if err != nil {
		return nil, fmt.Errorf("failed to get IPv4 addresses: %w", err)
	}

	restIPv6, err := w.fetchList(ctx, "/ipv6/address")
	if err != nil {
		return nil, fmt.Errorf("failed to get IPv6 addresses: %w", err)
	}

	iface, err := w.parseInterfaceData(restInterface, restIPv4, restIPv6)
	if err != nil {
		return nil, fmt.Errorf("failed to parse interface data: %w", err)
	}

	return &iface, nil
}

func (w *WgMikrotikRepo) parseInterfaceData(restInterface map[string]string, restIPv4, restIPv6 []map[string]string) (domain.PhysicalInterface, error) {
	mtu, err := strconv.Atoi(restInterface["mtu"])
	if err != nil {
		mtu = 0 // ignore invalid mtu value, use default
	}
	listenPort, err := strconv.Atoi(restInterface["listen-port"])
	if err != nil {
		mtu = 0 // ignore invalid mtu value, use default
	}
	deviceDisabled, err := strconv.ParseBool(restInterface["disabled"])
	if err != nil {
		deviceDisabled = true // ignore invalid device-up value, use default
	}
	deviceRunning, err := strconv.ParseBool(restInterface["running"])
	if err != nil {
		deviceRunning = false // ignore invalid device-up value, use default
	}

	var addresses []domain.Cidr
	for _, addr := range restIPv4 {
		if addr["interface"] == restInterface["name"] {
			cidr, err := domain.CidrFromString(addr["address"])
			if err != nil {
				continue
			}
			addresses = append(addresses, cidr)
		}
	}
	for _, addr := range restIPv6 {
		if addr["interface"] == restInterface["name"] {
			if strings.HasPrefix(addr["address"], "fe80:") {
				continue // ignore link-local addresses
			}
			cidr, err := domain.CidrFromString(addr["address"])
			if err != nil {
				continue
			}
			addresses = append(addresses, cidr)
		}
	}

	iface := domain.PhysicalInterface{
		Identifier: domain.InterfaceIdentifier(restInterface["name"]),
		KeyPair: domain.KeyPair{
			PrivateKey: restInterface["private-key"],
			PublicKey:  restInterface["public-key"],
		},
		ListenPort:    listenPort,
		Addresses:     addresses,
		Mtu:           mtu,
		FirewallMark:  0,
		DeviceUp:      !deviceDisabled && deviceRunning,
		ImportSource:  "",
		DeviceType:    MikrotikDeviceType,
		BytesUpload:   0,
		BytesDownload: 0,
	}
	return iface, nil
}

func (w *WgMikrotikRepo) GetPeers(ctx context.Context, deviceId domain.InterfaceIdentifier) ([]domain.PhysicalPeer, error) {
	restPeers, err := w.fetchList(ctx, "/interface/wireguard/peers?interface="+string(deviceId))
	if err != nil {
		return nil, fmt.Errorf("failed to get peers for %s: %w", deviceId, err)
	}

	var peers []domain.PhysicalPeer
	for _, restPeer := range restPeers {
		peer, err := w.parsePeerData(restPeer)
		if err != nil {
			continue
		}

		peers = append(peers, peer)
	}

	return peers, nil
}

func (w *WgMikrotikRepo) GetPeer(ctx context.Context, deviceId domain.InterfaceIdentifier, id domain.PeerIdentifier) (*domain.PhysicalPeer, error) {
	restPeers, err := w.fetchList(ctx, "/interface/wireguard/peers?interface="+string(deviceId)+"&public-key="+string(id))
	if err != nil {
		return nil, fmt.Errorf("failed to get peers for %s: %w", deviceId, err)
	}
	if len(restPeers) != 1 {
		return nil, fmt.Errorf("failed to get peer %s on device %s: got %d entries", id, deviceId, len(restPeers))
	}
	restPeer := restPeers[0]

	peer, err := w.parsePeerData(restPeer)
	if err != nil {
		return nil, fmt.Errorf("failed to parse peer data: %w", err)
	}

	return &peer, nil
}

func (w *WgMikrotikRepo) parsePeerData(restPeer map[string]string) (domain.PhysicalPeer, error) {
	endpoint := restPeer["current-endpoint-address"]
	if restPeer["current-endpoint-port"] != "" && restPeer["current-endpoint-port"] != "0" {
		endpoint = endpoint + ":" + restPeer["current-endpoint-port"]
	}

	keepAlive, _ := time.ParseDuration(restPeer["persistent-keepalive"])
	lastHandshake, _ := time.ParseDuration(restPeer["last-handshake"])
	var lastHandshakeTime time.Time
	if lastHandshake > 0 {
		lastHandshakeTime = time.Now().Add(-lastHandshake)
	}

	rxBytes, _ := strconv.ParseUint(restPeer["rx"], 10, 64)
	txBytes, _ := strconv.ParseUint(restPeer["tx"], 10, 64)

	peerDisabled, err := strconv.ParseBool(restPeer["disabled"])
	if err != nil {
		peerDisabled = true // ignore invalid device-up value, use default
	}

	if peerDisabled {
		return domain.PhysicalPeer{}, fmt.Errorf("peer is disabled")
	}

	allowedIPs, _ := domain.CidrsFromString(restPeer["allowed-address"])

	peer := domain.PhysicalPeer{
		Identifier: domain.PeerIdentifier(restPeer["public-key"]),
		Endpoint:   endpoint,
		AllowedIPs: allowedIPs,
		KeyPair: domain.KeyPair{
			PrivateKey: restPeer["private-key"],
			PublicKey:  restPeer["public-key"],
		},
		PresharedKey:        domain.PreSharedKey(restPeer["preshared-key"]),
		PersistentKeepalive: int(keepAlive.Seconds()),
		LastHandshake:       lastHandshakeTime,
		ProtocolVersion:     0,
		BytesUpload:         rxBytes,
		BytesDownload:       txBytes,
	}

	return peer, nil
}

func (w *WgMikrotikRepo) SaveInterface(_ context.Context, id domain.InterfaceIdentifier, updateFunc func(pi *domain.PhysicalInterface) (*domain.PhysicalInterface, error)) error {
	//TODO implement me
	panic("implement me")
}

func (w *WgMikrotikRepo) DeleteInterface(ctx context.Context, id domain.InterfaceIdentifier) error {
	restIPv4, err := w.fetchList(ctx, "/ip/address?interface="+string(id))
	if err != nil {
		return fmt.Errorf("failed to get IPv4 addresses: %w", err)
	}
	for _, addr := range restIPv4 {
		err = w.deleteObject(ctx, "/ip/address/"+addr[".id"])
		if err != nil {
			return fmt.Errorf("failed to delete IPv4 address %s: %w", addr["address"], err)
		}
	}

	restIPv6, err := w.fetchList(ctx, "/ipv6/address?interface="+string(id))
	if err != nil {
		return fmt.Errorf("failed to get IPv6 addresses: %w", err)
	}
	for _, addr := range restIPv6 {
		err = w.deleteObject(ctx, "/ipv6/address/"+addr[".id"])
		if err != nil {
			return fmt.Errorf("failed to delete IPv6 address %s: %w", addr["address"], err)
		}
	}

	restPeers, err := w.fetchList(ctx, "/interface/wireguard/peers?interface="+string(id))
	if err != nil {
		return fmt.Errorf("failed to get peers for %s: %w", id, err)
	}
	for _, restPeer := range restPeers {
		err = w.DeletePeer(ctx, id, domain.PeerIdentifier(restPeer["public-key"]))
		if err != nil {
			return fmt.Errorf("failed to delete peer %s: %w", restPeer["public-key"], err)
		}
	}

	restInterface, err := w.fetchObject(ctx, "/interface/wireguard/"+string(id)+"?.proplist=.id")
	if err != nil {
		return fmt.Errorf("failed to get interface %s: %w", id, err)
	}

	err = w.deleteObject(ctx, "/interface/wireguard/"+restInterface[".id"])
	if err != nil {
		return fmt.Errorf("failed to delete interface %s: %w", id, err)
	}

	return nil
}

func (w *WgMikrotikRepo) SavePeer(_ context.Context, deviceId domain.InterfaceIdentifier, id domain.PeerIdentifier, updateFunc func(pp *domain.PhysicalPeer) (*domain.PhysicalPeer, error)) error {
	//TODO implement me
	panic("implement me")
}

func (w *WgMikrotikRepo) DeletePeer(_ context.Context, deviceId domain.InterfaceIdentifier, id domain.PeerIdentifier) error {
	restPeers, err := w.fetchList(context.Background(), "/interface/wireguard/peers?interface="+string(deviceId)+"&public-key="+string(id)+"&.proplist=.id")
	if err != nil {
		return fmt.Errorf("failed to get peer %s on device %s: %w", id, deviceId, err)
	}

	if len(restPeers) != 1 {
		return fmt.Errorf("failed to get peer %s on device %s: got %d entries", id, deviceId, len(restPeers))
	}

	restPeer := restPeers[0]

	err = w.deleteObject(context.Background(), "/interface/wireguard/peers/"+restPeer[".id"])
	if err != nil {
		return fmt.Errorf("failed to delete peer %s on device %s: %w", id, deviceId, err)
	}

	return nil
}
