package wgcontroller

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/h44z/wg-portal/internal/config"
	"github.com/h44z/wg-portal/internal/domain"
	"github.com/h44z/wg-portal/internal/lowlevel"
)

// PfsenseController implements the InterfaceController interface for pfSense firewalls.
// It uses the pfSense REST API (https://pfrest.org/) to manage WireGuard interfaces and peers.
// API endpoint paths and field names should be verified against the Swagger documentation:
// https://pfrest.org/api-docs/

type PfsenseController struct {
	coreCfg *config.Config
	cfg     *config.BackendPfsense

	client *lowlevel.PfsenseApiClient

	// Add mutexes to prevent race conditions
	interfaceMutexes sync.Map   // map[domain.InterfaceIdentifier]*sync.Mutex
	peerMutexes      sync.Map   // map[domain.PeerIdentifier]*sync.Mutex
	coreMutex        sync.Mutex // for updating the core configuration such as routing table or DNS settings
}

func NewPfsenseController(coreCfg *config.Config, cfg *config.BackendPfsense) (*PfsenseController, error) {
	client, err := lowlevel.NewPfsenseApiClient(coreCfg, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create pfSense API client: %w", err)
	}

	return &PfsenseController{
		coreCfg: coreCfg,
		cfg:     cfg,

		client: client,

		interfaceMutexes: sync.Map{},
		peerMutexes:      sync.Map{},
		coreMutex:        sync.Mutex{},
	}, nil
}

func (c *PfsenseController) GetId() domain.InterfaceBackend {
	return domain.InterfaceBackend(c.cfg.Id)
}

// getInterfaceMutex returns a mutex for the given interface to prevent concurrent modifications
func (c *PfsenseController) getInterfaceMutex(id domain.InterfaceIdentifier) *sync.Mutex {
	mutex, _ := c.interfaceMutexes.LoadOrStore(id, &sync.Mutex{})
	return mutex.(*sync.Mutex)
}

// getPeerMutex returns a mutex for the given peer to prevent concurrent modifications
func (c *PfsenseController) getPeerMutex(id domain.PeerIdentifier) *sync.Mutex {
	mutex, _ := c.peerMutexes.LoadOrStore(id, &sync.Mutex{})
	return mutex.(*sync.Mutex)
}

// region wireguard-related

func (c *PfsenseController) GetInterfaces(ctx context.Context) ([]domain.PhysicalInterface, error) {
	// Query WireGuard tunnels from pfSense API
	// Using pfSense REST API v2 endpoints: GET /api/v2/vpn/wireguard/tunnels
	// Field names should be verified against Swagger docs: https://pfrest.org/api-docs/
	wgReply := c.client.Query(ctx, "/api/v2/vpn/wireguard/tunnels", &lowlevel.PfsenseRequestOptions{})
	if wgReply.Status != lowlevel.PfsenseApiStatusOk {
		return nil, fmt.Errorf("failed to query interfaces: %v", wgReply.Error)
	}

	// Parallelize loading of interface details to speed up overall latency.
	// Use a bounded semaphore to avoid overloading the pfSense device.
	maxConcurrent := c.cfg.GetConcurrency()
	sem := make(chan struct{}, maxConcurrent)

	interfaces := make([]domain.PhysicalInterface, 0, len(wgReply.Data))
	var mu sync.Mutex
	var wgWait sync.WaitGroup
	var firstErr error
	ctx2, cancel := context.WithCancel(ctx)
	defer cancel()

	for _, wgObj := range wgReply.Data {
		wgWait.Add(1)
		sem <- struct{}{} // block if more than maxConcurrent requests are processing
		go func(wg lowlevel.GenericJsonObject) {
			defer wgWait.Done()
			defer func() { <-sem }() // read from the semaphore and make space for the next entry
			if firstErr != nil {
				return
			}
			pi, err := c.loadInterfaceData(ctx2, wg)
			if err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = err
					cancel()
				}
				mu.Unlock()
				return
			}
			mu.Lock()
			interfaces = append(interfaces, *pi)
			mu.Unlock()
		}(wgObj)
	}

	wgWait.Wait()
	if firstErr != nil {
		return nil, firstErr
	}

	return interfaces, nil
}

func (c *PfsenseController) GetInterface(ctx context.Context, id domain.InterfaceIdentifier) (
	*domain.PhysicalInterface,
	error,
) {
	// First, get the tunnel ID by querying by name
	wgReply := c.client.Query(ctx, "/api/v2/vpn/wireguard/tunnels", &lowlevel.PfsenseRequestOptions{
		Filters: map[string]string{
			"name": string(id),
		},
	})
	if wgReply.Status != lowlevel.PfsenseApiStatusOk {
		return nil, fmt.Errorf("failed to query interface %s: %v", id, wgReply.Error)
	}

	if len(wgReply.Data) == 0 {
		return nil, fmt.Errorf("interface %s not found", id)
	}

	tunnelId := wgReply.Data[0].GetString("id")
	
	// Query the specific tunnel endpoint to get full details including addresses
	// Endpoint: GET /api/v2/vpn/wireguard/tunnel?id={id}
	if tunnelId != "" {
		tunnelReply := c.client.Get(ctx, "/api/v2/vpn/wireguard/tunnel", &lowlevel.PfsenseRequestOptions{
			Filters: map[string]string{
				"id": tunnelId,
			},
		})
		if tunnelReply.Status == lowlevel.PfsenseApiStatusOk && tunnelReply.Data != nil {
			// Use the detailed tunnel response which includes addresses
			return c.loadInterfaceData(ctx, tunnelReply.Data)
		}
		// Fall back to list response if detail query fails
		if c.cfg.Debug {
			slog.Debug("failed to query detailed tunnel info, using list response", "interface", id, "tunnel_id", tunnelId)
		}
	}

	return c.loadInterfaceData(ctx, wgReply.Data[0])
}

func (c *PfsenseController) loadInterfaceData(
	ctx context.Context,
	wireGuardObj lowlevel.GenericJsonObject,
) (*domain.PhysicalInterface, error) {
	deviceName := wireGuardObj.GetString("name")
	deviceId := wireGuardObj.GetString("id")

	// Extract addresses from the tunnel data
	// The tunnel response may include an "addresses" array when queried via /tunnel?id={id}
	addresses := c.extractAddresses(wireGuardObj, nil)

	// If addresses weren't found in the tunnel object and we have a tunnel ID,
	// query the specific tunnel endpoint to get full details including addresses
	// Endpoint: GET /api/v2/vpn/wireguard/tunnel?id={id}
	if len(addresses) == 0 && deviceId != "" {
		tunnelReply := c.client.Get(ctx, "/api/v2/vpn/wireguard/tunnel", &lowlevel.PfsenseRequestOptions{
			Filters: map[string]string{
				"id": deviceId,
			},
		})
		if tunnelReply.Status == lowlevel.PfsenseApiStatusOk && tunnelReply.Data != nil {
			// Extract addresses from the detailed tunnel response
			parsedAddrs := c.extractAddresses(tunnelReply.Data, nil)
			if len(parsedAddrs) > 0 {
				addresses = parsedAddrs
				if c.cfg.Debug {
					slog.Debug("loaded addresses from detailed tunnel query", "interface", deviceName, "count", len(addresses))
				}
			}
		}
	}

	interfaceModel, err := c.convertWireGuardInterface(wireGuardObj, nil, addresses)
	if err != nil {
		return nil, fmt.Errorf("interface convert failed for %s: %w", deviceName, err)
	}
	return &interfaceModel, nil
}

func (c *PfsenseController) extractAddresses(
	wgObj lowlevel.GenericJsonObject,
	ifaceObj lowlevel.GenericJsonObject,
) []domain.Cidr {
	addresses := make([]domain.Cidr, 0)

	// Try to get addresses from ifaceObj first
	if ifaceObj != nil {
		addrStr := ifaceObj.GetString("addresses")
		if addrStr != "" {
			// Addresses might be comma-separated or in an array
			addrs, _ := domain.CidrsFromString(addrStr)
			addresses = append(addresses, addrs...)
		}
	}

	// Try to get addresses from wgObj - check if it's an array first
	if len(addresses) == 0 {
		if addressesValue, ok := wgObj["addresses"]; ok && addressesValue != nil {
			if addressesArray, ok := addressesValue.([]any); ok {
				// Parse addresses array (from /tunnel?id={id} response)
				// Each object has "address" and "mask" fields
				for _, addrItem := range addressesArray {
					if addrObj, ok := addrItem.(map[string]any); ok {
						address := ""
						mask := 0
						
						// Extract address
						if addrVal, ok := addrObj["address"]; ok {
							if addrStr, ok := addrVal.(string); ok {
								address = addrStr
							} else {
								address = fmt.Sprintf("%v", addrVal)
							}
						}
						
						// Extract mask
						if maskVal, ok := addrObj["mask"]; ok {
							if maskInt, ok := maskVal.(int); ok {
								mask = maskInt
							} else if maskFloat, ok := maskVal.(float64); ok {
								mask = int(maskFloat)
							} else if maskStr, ok := maskVal.(string); ok {
								if maskInt, err := strconv.Atoi(maskStr); err == nil {
									mask = maskInt
								}
							}
						}
						
						// Convert to CIDR format
						if address != "" && mask > 0 {
							cidrStr := fmt.Sprintf("%s/%d", address, mask)
							if cidr, err := domain.CidrFromString(cidrStr); err == nil {
								addresses = append(addresses, cidr)
							}
						} else if address != "" {
							// Try parsing as CIDR string directly
							if cidr, err := domain.CidrFromString(address); err == nil {
								addresses = append(addresses, cidr)
							}
						}
					}
				}
			} else if addrStr, ok := addressesValue.(string); ok {
				// Fallback: try parsing as comma-separated string
				addrs, _ := domain.CidrsFromString(addrStr)
				addresses = append(addresses, addrs...)
			}
		} else {
			// Try as string field
			addrStr := wgObj.GetString("addresses")
			if addrStr != "" {
				addrs, _ := domain.CidrsFromString(addrStr)
				addresses = append(addresses, addrs...)
			}
		}
	}

	return addresses
}

// parseAddressArray parses an array of address objects from the pfSense API
// Each object has "address" and "mask" fields (similar to allowedips structure)
func (c *PfsenseController) parseAddressArray(addressArray []lowlevel.GenericJsonObject) []domain.Cidr {
	addresses := make([]domain.Cidr, 0, len(addressArray))
	
	for _, addrObj := range addressArray {
		address := addrObj.GetString("address")
		mask := addrObj.GetInt("mask")
		
		if address != "" && mask > 0 {
			cidrStr := fmt.Sprintf("%s/%d", address, mask)
			if cidr, err := domain.CidrFromString(cidrStr); err == nil {
				addresses = append(addresses, cidr)
			}
		} else if address != "" {
			// Try parsing as CIDR string directly
			if cidr, err := domain.CidrFromString(address); err == nil {
				addresses = append(addresses, cidr)
			}
		}
	}
	
	return addresses
}

func (c *PfsenseController) convertWireGuardInterface(
	wg, iface lowlevel.GenericJsonObject,
	addresses []domain.Cidr,
) (
	domain.PhysicalInterface,
	error,
) {
	// Map pfSense field names to our domain model
	// Field names should be verified against the Swagger UI: https://pfrest.org/api-docs/
	// The implementation attempts to handle both camelCase and kebab-case variations
	privateKey := wg.GetString("privatekey")
	if privateKey == "" {
		privateKey = wg.GetString("private-key")
	}
	publicKey := wg.GetString("publickey")
	if publicKey == "" {
		publicKey = wg.GetString("public-key")
	}

	listenPort := wg.GetInt("listenport")
	if listenPort == 0 {
		listenPort = wg.GetInt("listen-port")
	}

	mtu := wg.GetInt("mtu")
	running := wg.GetBool("running")
	disabled := wg.GetBool("disabled")

	// TODO: Interface statistics (rx/tx bytes) are not currently supported
	// by the pfSense REST API. This functionality is reserved for future implementation.
	var rxBytes, txBytes uint64

	pi := domain.PhysicalInterface{
		Identifier: domain.InterfaceIdentifier(wg.GetString("name")),
		KeyPair: domain.KeyPair{
			PrivateKey: privateKey,
			PublicKey:  publicKey,
		},
		ListenPort:    listenPort,
		Addresses:     addresses,
		Mtu:           mtu,
		FirewallMark:  0,
		DeviceUp:      running && !disabled,
		ImportSource:  domain.ControllerTypePfsense,
		DeviceType:    domain.ControllerTypePfsense,
		BytesUpload:   txBytes,
		BytesDownload: rxBytes,
	}

	// Extract description - pfSense API uses "descr" field
	description := wg.GetString("descr")
	if description == "" {
		description = wg.GetString("description")
	}
	if description == "" {
		description = wg.GetString("comment")
	}

	pi.SetExtras(domain.PfsenseInterfaceExtras{
		Id:       wg.GetString("id"),
		Comment:  description,
		Disabled: disabled,
	})

	return pi, nil
}

func (c *PfsenseController) GetPeers(ctx context.Context, deviceId domain.InterfaceIdentifier) (
	[]domain.PhysicalPeer,
	error,
) {
	// Query all peers and filter by interface client-side
	// Using pfSense REST API v2 endpoints (https://pfrest.org/)
	// The API uses query parameters like ?id=0 for specific items, but we need to filter
	// by interface (tun field), so we fetch all peers and filter client-side
	wgReply := c.client.Query(ctx, "/api/v2/vpn/wireguard/peers", &lowlevel.PfsenseRequestOptions{})
	if wgReply.Status != lowlevel.PfsenseApiStatusOk {
		return nil, fmt.Errorf("failed to query peers for %s: %v", deviceId, wgReply.Error)
	}

	if len(wgReply.Data) == 0 {
		return nil, nil
	}

	// Filter peers client-side by checking the "tun" field in each peer
	// pfSense peer responses use "tun" field to indicate which tunnel/interface the peer belongs to
	peers := make([]domain.PhysicalPeer, 0, len(wgReply.Data))
	for _, peer := range wgReply.Data {
		// Check if this peer belongs to the requested interface
		// pfSense uses "tun" field with the interface name (e.g., "tun_wg0")
		peerTun := peer.GetString("tun")
		if peerTun == "" {
			// Try alternative field names as fallback
			peerTun = peer.GetString("interface")
			if peerTun == "" {
				peerTun = peer.GetString("tunnel")
			}
		}
		
		// Only include peers that match the requested interface name
		if peerTun != string(deviceId) {
			if c.cfg.Debug {
				slog.Debug("skipping peer - interface mismatch",
					"peer", peer.GetString("name"),
					"peer_tun", peerTun,
					"requested_interface", deviceId,
					"peer_id", peer.GetString("id"))
			}
			continue
		}

		// Use peer data directly from the list response
		peerModel, err := c.convertWireGuardPeer(peer)
		if err != nil {
			return nil, fmt.Errorf("peer convert failed for %v: %w", peer.GetString("name"), err)
		}
		peers = append(peers, peerModel)
	}
	
	if c.cfg.Debug {
		slog.Debug("filtered peers for interface",
			"interface", deviceId,
			"total_peers_from_api", len(wgReply.Data),
			"filtered_peers", len(peers))
	}

	return peers, nil
}

func (c *PfsenseController) convertWireGuardPeer(peer lowlevel.GenericJsonObject) (
	domain.PhysicalPeer,
	error,
) {
	publicKey := peer.GetString("publickey")
	if publicKey == "" {
		publicKey = peer.GetString("public-key")
	}

	privateKey := peer.GetString("privatekey")
	if privateKey == "" {
		privateKey = peer.GetString("private-key")
	}

	presharedKey := peer.GetString("presharedkey")
	if presharedKey == "" {
		presharedKey = peer.GetString("preshared-key")
	}

	// pfSense returns allowedips as an array of objects with "address" and "mask" fields
	// Example: [{"address": "10.1.2.3", "mask": 32, ...}, ...]
	var allowedAddresses []domain.Cidr
	if allowedIPsValue, ok := peer["allowedips"]; ok {
		if allowedIPsArray, ok := allowedIPsValue.([]any); ok {
			// Parse array of objects
			for _, item := range allowedIPsArray {
				if itemObj, ok := item.(map[string]any); ok {
					address := ""
					mask := 0
					
					// Extract address
					if addrVal, ok := itemObj["address"]; ok {
						if addrStr, ok := addrVal.(string); ok {
							address = addrStr
						} else {
							address = fmt.Sprintf("%v", addrVal)
						}
					}
					
					// Extract mask
					if maskVal, ok := itemObj["mask"]; ok {
						if maskInt, ok := maskVal.(int); ok {
							mask = maskInt
						} else if maskFloat, ok := maskVal.(float64); ok {
							mask = int(maskFloat)
						} else if maskStr, ok := maskVal.(string); ok {
							if maskInt, err := strconv.Atoi(maskStr); err == nil {
								mask = maskInt
							}
						}
					}
					
					// Convert to CIDR format (e.g., "10.1.2.3/32")
					if address != "" && mask > 0 {
						cidrStr := fmt.Sprintf("%s/%d", address, mask)
						if cidr, err := domain.CidrFromString(cidrStr); err == nil {
							allowedAddresses = append(allowedAddresses, cidr)
						}
					}
				}
			}
		} else if allowedIPsStr, ok := allowedIPsValue.(string); ok {
			// Fallback: try parsing as comma-separated string
			allowedAddresses, _ = domain.CidrsFromString(allowedIPsStr)
		}
	}
	
	// Fallback to string parsing if array parsing didn't work
	if len(allowedAddresses) == 0 {
		allowedIPsStr := peer.GetString("allowedips")
		if allowedIPsStr == "" {
			allowedIPsStr = peer.GetString("allowed-ips")
		}
		if allowedIPsStr != "" {
			allowedAddresses, _ = domain.CidrsFromString(allowedIPsStr)
		}
	}

	endpoint := peer.GetString("endpoint")
	port := peer.GetString("port")
	
	// Combine endpoint and port if both are available
	if endpoint != "" && port != "" {
		// Check if endpoint already contains a port
		if !strings.Contains(endpoint, ":") {
			endpoint = fmt.Sprintf("%s:%s", endpoint, port)
		}
	} else if endpoint == "" && port != "" {
		// If only port is available, we can't construct a full endpoint
		// This might be used with the interface's listenport
	}

	keepAliveSeconds := 0
	keepAliveStr := peer.GetString("persistentkeepalive")
	if keepAliveStr == "" {
		keepAliveStr = peer.GetString("persistent-keepalive")
	}
	if keepAliveStr != "" {
		duration, err := time.ParseDuration(keepAliveStr)
		if err == nil {
			keepAliveSeconds = int(duration.Seconds())
		} else {
			// Try parsing as integer (seconds)
			if secs, err := strconv.Atoi(keepAliveStr); err == nil {
				keepAliveSeconds = secs
			}
		}
	}

	// TODO: Peer statistics (last handshake, rx/tx bytes) are not currently supported
	// by the pfSense REST API. This functionality is reserved for future implementation
	// when the API adds support for these fields.
	// See: https://github.com/jaredhendrickson13/pfsense-api/issues (issue opened by user)
	//
	// When supported, extract fields like:
	// - lastHandshake: peer.GetString("lasthandshake") or peer.GetString("last-handshake")
	// - rxBytes: peer.GetInt("rxbytes") or peer.GetInt("rx-bytes")
	// - txBytes: peer.GetInt("txbytes") or peer.GetInt("tx-bytes")
	lastHandshakeTime := time.Time{}
	rxBytes := uint64(0)
	txBytes := uint64(0)

	peerModel := domain.PhysicalPeer{
		Identifier: domain.PeerIdentifier(publicKey),
		Endpoint:   endpoint,
		AllowedIPs: allowedAddresses,
		KeyPair: domain.KeyPair{
			PublicKey:  publicKey,
			PrivateKey: privateKey,
		},
		PresharedKey:        domain.PreSharedKey(presharedKey),
		PersistentKeepalive: keepAliveSeconds,
		LastHandshake:       lastHandshakeTime,
		ProtocolVersion:     0, // pfSense may not expose protocol version
		BytesUpload:         txBytes,
		BytesDownload:       rxBytes,
		ImportSource:        domain.ControllerTypePfsense,
	}

	// Extract description/name - pfSense API uses "descr" field
	description := peer.GetString("descr")
	if description == "" {
		description = peer.GetString("description")
	}
	if description == "" {
		description = peer.GetString("comment")
	}

	// Extract name - pfSense API may use "name" or "descr"
	name := peer.GetString("name")
	if name == "" {
		name = peer.GetString("descr")
	}
	if name == "" {
		name = description // fallback to description if name is not available
	}

	peerModel.SetExtras(domain.PfsensePeerExtras{
		Id:              peer.GetString("id"),
		Name:            name,
		Comment:         description,
		Disabled:        peer.GetBool("disabled"),
		ClientEndpoint:  "", // pfSense may handle this differently
		ClientAddress:   "", // pfSense may handle this differently
		ClientDns:       "", // pfSense may handle this differently
		ClientKeepalive: 0,  // pfSense may handle this differently
	})

	return peerModel, nil
}

func (c *PfsenseController) SaveInterface(
	ctx context.Context,
	id domain.InterfaceIdentifier,
	updateFunc func(pi *domain.PhysicalInterface) (*domain.PhysicalInterface, error),
) error {
	// Lock the interface to prevent concurrent modifications
	mutex := c.getInterfaceMutex(id)
	mutex.Lock()
	defer mutex.Unlock()

	physicalInterface, err := c.getOrCreateInterface(ctx, id)
	if err != nil {
		return err
	}

	deviceId := ""
	if physicalInterface.GetExtras() != nil {
		if extras, ok := physicalInterface.GetExtras().(domain.PfsenseInterfaceExtras); ok {
			deviceId = extras.Id
		}
	}

	if updateFunc != nil {
		physicalInterface, err = updateFunc(physicalInterface)
		if err != nil {
			return err
		}
		if deviceId != "" {
			// Ensure the ID is preserved
			if extras, ok := physicalInterface.GetExtras().(domain.PfsenseInterfaceExtras); ok {
				extras.Id = deviceId
				physicalInterface.SetExtras(extras)
			}
		}
	}

	if err := c.updateInterface(ctx, physicalInterface); err != nil {
		return err
	}

	return nil
}

func (c *PfsenseController) getOrCreateInterface(
	ctx context.Context,
	id domain.InterfaceIdentifier,
) (*domain.PhysicalInterface, error) {
	wgReply := c.client.Query(ctx, "/api/v2/vpn/wireguard/tunnels", &lowlevel.PfsenseRequestOptions{
		Filters: map[string]string{
			"name": string(id),
		},
	})
	if wgReply.Status == lowlevel.PfsenseApiStatusOk && len(wgReply.Data) > 0 {
		return c.loadInterfaceData(ctx, wgReply.Data[0])
	}

	// create a new tunnel if it does not exist
	// Actual endpoint: POST /api/v2/vpn/wireguard/tunnel (singular)
	createReply := c.client.Create(ctx, "/api/v2/vpn/wireguard/tunnel", lowlevel.GenericJsonObject{
		"name": string(id),
	})
	if createReply.Status == lowlevel.PfsenseApiStatusOk {
		return c.loadInterfaceData(ctx, createReply.Data)
	}

	return nil, fmt.Errorf("failed to create interface %s: %v", id, createReply.Error)
}

func (c *PfsenseController) updateInterface(ctx context.Context, pi *domain.PhysicalInterface) error {
	extras := pi.GetExtras().(domain.PfsenseInterfaceExtras)
	interfaceId := extras.Id

	payload := lowlevel.GenericJsonObject{
		"name":        string(pi.Identifier),
		"description": extras.Comment,
		"mtu":         strconv.Itoa(pi.Mtu),
		"listenport":  strconv.Itoa(pi.ListenPort),
		"privatekey":  pi.KeyPair.PrivateKey,
		"disabled":    strconv.FormatBool(!pi.DeviceUp),
	}

	// Add addresses if present
	if len(pi.Addresses) > 0 {
		addresses := make([]string, 0, len(pi.Addresses))
		for _, addr := range pi.Addresses {
			addresses = append(addresses, addr.String())
		}
		payload["addresses"] = strings.Join(addresses, ",")
	}

	// Actual endpoint: PATCH /api/v2/vpn/wireguard/tunnel?id={id}
	wgReply := c.client.Update(ctx, "/api/v2/vpn/wireguard/tunnel?id="+interfaceId, payload)
	if wgReply.Status != lowlevel.PfsenseApiStatusOk {
		return fmt.Errorf("failed to update interface %s: %v", pi.Identifier, wgReply.Error)
	}

	return nil
}

func (c *PfsenseController) DeleteInterface(ctx context.Context, id domain.InterfaceIdentifier) error {
	// Lock the interface to prevent concurrent modifications
	mutex := c.getInterfaceMutex(id)
	mutex.Lock()
	defer mutex.Unlock()

	// Find the tunnel ID
	wgReply := c.client.Query(ctx, "/api/v2/vpn/wireguard/tunnels", &lowlevel.PfsenseRequestOptions{
		Filters: map[string]string{
			"name": string(id),
		},
	})
	if wgReply.Status != lowlevel.PfsenseApiStatusOk {
		return fmt.Errorf("unable to find WireGuard tunnel %s: %v", id, wgReply.Error)
	}
	if len(wgReply.Data) == 0 {
		return nil // tunnel does not exist, nothing to delete
	}

	interfaceId := wgReply.Data[0].GetString("id")
	// Actual endpoint: DELETE /api/v2/vpn/wireguard/tunnel?id={id}
	deleteReply := c.client.Delete(ctx, "/api/v2/vpn/wireguard/tunnel?id="+interfaceId)
	if deleteReply.Status != lowlevel.PfsenseApiStatusOk {
		return fmt.Errorf("failed to delete WireGuard interface %s: %v", id, deleteReply.Error)
	}

	return nil
}

func (c *PfsenseController) SavePeer(
	ctx context.Context,
	deviceId domain.InterfaceIdentifier,
	id domain.PeerIdentifier,
	updateFunc func(pp *domain.PhysicalPeer) (*domain.PhysicalPeer, error),
) error {
	// Lock the peer to prevent concurrent modifications
	mutex := c.getPeerMutex(id)
	mutex.Lock()
	defer mutex.Unlock()

	physicalPeer, err := c.getOrCreatePeer(ctx, deviceId, id)
	if err != nil {
		return err
	}

	peerId := ""
	if physicalPeer.GetExtras() != nil {
		if extras, ok := physicalPeer.GetExtras().(domain.PfsensePeerExtras); ok {
			peerId = extras.Id
		}
	}

	physicalPeer, err = updateFunc(physicalPeer)
	if err != nil {
		return err
	}
	if peerId != "" {
		// Ensure the ID is preserved
		if extras, ok := physicalPeer.GetExtras().(domain.PfsensePeerExtras); ok {
			extras.Id = peerId
			physicalPeer.SetExtras(extras)
		}
	}

	if err := c.updatePeer(ctx, deviceId, physicalPeer); err != nil {
		return err
	}

	return nil
}

func (c *PfsenseController) getOrCreatePeer(
	ctx context.Context,
	deviceId domain.InterfaceIdentifier,
	id domain.PeerIdentifier,
) (*domain.PhysicalPeer, error) {
	// Query for peer by publickey and interface (tun field)
	// The API uses query parameters like ?publickey=...&tun=...
	wgReply := c.client.Query(ctx, "/api/v2/vpn/wireguard/peers", &lowlevel.PfsenseRequestOptions{
		Filters: map[string]string{
			"publickey": string(id),
			"tun":        string(deviceId), // Use "tun" field name as that's what the API uses
		},
	})
	if wgReply.Status == lowlevel.PfsenseApiStatusOk && len(wgReply.Data) > 0 {
		slog.Debug("found existing pfSense peer", "peer", id, "interface", deviceId)
		existingPeer, err := c.convertWireGuardPeer(wgReply.Data[0])
		if err != nil {
			return nil, err
		}
		return &existingPeer, nil
	}

	// create a new peer if it does not exist
	// Actual endpoint: POST /api/v2/vpn/wireguard/peer (singular)
	slog.Debug("creating new pfSense peer", "peer", id, "interface", deviceId)
	createReply := c.client.Create(ctx, "/api/v2/vpn/wireguard/peer", lowlevel.GenericJsonObject{
		"name":       fmt.Sprintf("wg-%s", id[0:8]),
		"interface": string(deviceId),
		"publickey": string(id),
		"allowedips": "0.0.0.0/0", // Use 0.0.0.0/0 as default, will be updated by updatePeer
	})
	if createReply.Status == lowlevel.PfsenseApiStatusOk {
		newPeer, err := c.convertWireGuardPeer(createReply.Data)
		if err != nil {
			return nil, err
		}
		slog.Debug("successfully created pfSense peer", "peer", id, "interface", deviceId)
		return &newPeer, nil
	}

	return nil, fmt.Errorf("failed to create peer %s for interface %s: %v", id, deviceId, createReply.Error)
}

func (c *PfsenseController) updatePeer(
	ctx context.Context,
	deviceId domain.InterfaceIdentifier,
	pp *domain.PhysicalPeer,
) error {
	extras := pp.GetExtras().(domain.PfsensePeerExtras)
	peerId := extras.Id

	allowedIPsStr := domain.CidrsToString(pp.AllowedIPs)

	slog.Debug("updating pfSense peer",
		"peer", pp.Identifier,
		"interface", deviceId,
		"allowed-ips", allowedIPsStr,
		"allowed-ips-count", len(pp.AllowedIPs),
		"disabled", extras.Disabled)

	payload := lowlevel.GenericJsonObject{
		"name":                 extras.Name,
		"description":          extras.Comment,
		"presharedkey":         string(pp.PresharedKey),
		"publickey":            pp.KeyPair.PublicKey,
		"privatekey":           pp.KeyPair.PrivateKey,
		"persistentkeepalive":  strconv.Itoa(pp.PersistentKeepalive),
		"disabled":             strconv.FormatBool(extras.Disabled),
		"allowedips":           allowedIPsStr,
	}

	if pp.Endpoint != "" {
		payload["endpoint"] = pp.Endpoint
	}

	// Actual endpoint: PATCH /api/v2/vpn/wireguard/peer?id={id}
	wgReply := c.client.Update(ctx, "/api/v2/vpn/wireguard/peer?id="+peerId, payload)
	if wgReply.Status != lowlevel.PfsenseApiStatusOk {
		return fmt.Errorf("failed to update peer %s on interface %s: %v", pp.Identifier, deviceId, wgReply.Error)
	}

	if extras.Disabled {
		slog.Debug("successfully disabled pfSense peer", "peer", pp.Identifier, "interface", deviceId)
	} else {
		slog.Debug("successfully updated pfSense peer", "peer", pp.Identifier, "interface", deviceId)
	}

	return nil
}

func (c *PfsenseController) DeletePeer(
	ctx context.Context,
	deviceId domain.InterfaceIdentifier,
	id domain.PeerIdentifier,
) error {
	// Lock the peer to prevent concurrent modifications
	mutex := c.getPeerMutex(id)
	mutex.Lock()
	defer mutex.Unlock()

	// Query for peer by publickey and interface (tun field)
	// The API uses query parameters like ?publickey=...&tun=...
	wgReply := c.client.Query(ctx, "/api/v2/vpn/wireguard/peers", &lowlevel.PfsenseRequestOptions{
		Filters: map[string]string{
			"publickey": string(id),
			"tun":        string(deviceId), // Use "tun" field name as that's what the API uses
		},
	})
	if wgReply.Status != lowlevel.PfsenseApiStatusOk {
		return fmt.Errorf("unable to find WireGuard peer %s for interface %s: %v", id, deviceId, wgReply.Error)
	}
	if len(wgReply.Data) == 0 {
		return nil // peer does not exist, nothing to delete
	}

	peerId := wgReply.Data[0].GetString("id")
	// Actual endpoint: DELETE /api/v2/vpn/wireguard/peer?id={id}
	deleteReply := c.client.Delete(ctx, "/api/v2/vpn/wireguard/peer?id="+peerId)
	if deleteReply.Status != lowlevel.PfsenseApiStatusOk {
		return fmt.Errorf("failed to delete WireGuard peer %s for interface %s: %v", id, deviceId, deleteReply.Error)
	}

	return nil
}

// endregion wireguard-related

// region wg-quick-related

func (c *PfsenseController) ExecuteInterfaceHook(
	_ context.Context,
	_ domain.InterfaceIdentifier,
	_ string,
) error {
	// TODO implement me
	slog.Error("interface hooks are not yet supported for pfSense backends, please open an issue on GitHub")
	return nil
}

func (c *PfsenseController) SetDNS(
	ctx context.Context,
	_ domain.InterfaceIdentifier,
	dnsStr, _ string,
) error {
	// Lock the interface to prevent concurrent modifications
	c.coreMutex.Lock()
	defer c.coreMutex.Unlock()

	// pfSense DNS configuration is typically managed at the system level
	// This may need to be implemented based on pfSense API capabilities
	slog.Warn("DNS setting is not yet fully supported for pfSense backends")
	return nil
}

func (c *PfsenseController) UnsetDNS(
	ctx context.Context,
	_ domain.InterfaceIdentifier,
	dnsStr, _ string,
) error {
	// Lock the interface to prevent concurrent modifications
	c.coreMutex.Lock()
	defer c.coreMutex.Unlock()

	// pfSense DNS configuration is typically managed at the system level
	slog.Warn("DNS unsetting is not yet fully supported for pfSense backends")
	return nil
}

// endregion wg-quick-related

// region routing-related

func (c *PfsenseController) SetRoutes(_ context.Context, info domain.RoutingTableInfo) error {
	// pfSense routing is typically managed through the firewall rules and routing tables
	// This may need to be implemented based on pfSense API capabilities
	slog.Warn("route setting is not yet fully supported for pfSense backends")
	return nil
}

func (c *PfsenseController) RemoveRoutes(_ context.Context, info domain.RoutingTableInfo) error {
	// pfSense routing is typically managed through the firewall rules and routing tables
	slog.Warn("route removal is not yet fully supported for pfSense backends")
	return nil
}

// endregion routing-related

// region statistics-related

func (c *PfsenseController) PingAddresses(
	ctx context.Context,
	addr string,
) (*domain.PingerResult, error) {
	// Use pfSense API to ping if available, otherwise return error
	// This may need to be implemented based on pfSense API capabilities
	return nil, fmt.Errorf("ping functionality is not yet implemented for pfSense backends")
}

// endregion statistics-related

