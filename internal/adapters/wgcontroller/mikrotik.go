package wgcontroller

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/h44z/wg-portal/internal/config"
	"github.com/h44z/wg-portal/internal/domain"
	"github.com/h44z/wg-portal/internal/lowlevel"
)

const MikrotikRouteDistance = 5
const MikrotikDefaultRoutingTable = "main"

type MikrotikController struct {
	coreCfg *config.Config
	cfg     *config.BackendMikrotik

	client *lowlevel.MikrotikApiClient

	// Add mutexes to prevent race conditions
	interfaceMutexes sync.Map   // map[domain.InterfaceIdentifier]*sync.Mutex
	peerMutexes      sync.Map   // map[domain.PeerIdentifier]*sync.Mutex
	coreMutex        sync.Mutex // for updating the core configuration such as routing table or DNS settings
}

func NewMikrotikController(coreCfg *config.Config, cfg *config.BackendMikrotik) (*MikrotikController, error) {
	client, err := lowlevel.NewMikrotikApiClient(coreCfg, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create Mikrotik API client: %w", err)
	}

	return &MikrotikController{
		coreCfg: coreCfg,
		cfg:     cfg,

		client: client,

		interfaceMutexes: sync.Map{},
		peerMutexes:      sync.Map{},
		coreMutex:        sync.Mutex{},
	}, nil
}

func (c *MikrotikController) GetId() domain.InterfaceBackend {
	return domain.InterfaceBackend(c.cfg.Id)
}

// getInterfaceMutex returns a mutex for the given interface to prevent concurrent modifications
func (c *MikrotikController) getInterfaceMutex(id domain.InterfaceIdentifier) *sync.Mutex {
	mutex, _ := c.interfaceMutexes.LoadOrStore(id, &sync.Mutex{})
	return mutex.(*sync.Mutex)
}

// getPeerMutex returns a mutex for the given peer to prevent concurrent modifications
func (c *MikrotikController) getPeerMutex(id domain.PeerIdentifier) *sync.Mutex {
	mutex, _ := c.peerMutexes.LoadOrStore(id, &sync.Mutex{})
	return mutex.(*sync.Mutex)
}

// region wireguard-related

func (c *MikrotikController) GetInterfaces(ctx context.Context) ([]domain.PhysicalInterface, error) {
	wgReply := c.client.Query(ctx, "/interface/wireguard", &lowlevel.MikrotikRequestOptions{
		PropList: []string{
			".id", "name", "public-key", "private-key", "listen-port", "mtu", "disabled", "running", "comment",
		},
	})
	if wgReply.Status != lowlevel.MikrotikApiStatusOk {
		return nil, fmt.Errorf("failed to query interfaces: %v", wgReply.Error)
	}

	// Parallelize loading of interface details to speed up overall latency.
	// Use a bounded semaphore to avoid overloading the MikroTik device.
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

func (c *MikrotikController) GetInterface(ctx context.Context, id domain.InterfaceIdentifier) (
	*domain.PhysicalInterface,
	error,
) {
	wgReply := c.client.Query(ctx, "/interface/wireguard", &lowlevel.MikrotikRequestOptions{
		PropList: []string{
			".id", "name", "public-key", "private-key", "listen-port", "mtu", "disabled", "running",
		},
		Filters: map[string]string{
			"name": string(id),
		},
	})
	if wgReply.Status != lowlevel.MikrotikApiStatusOk {
		return nil, fmt.Errorf("failed to query interface %s: %v", id, wgReply.Error)
	}

	if len(wgReply.Data) == 0 {
		return nil, fmt.Errorf("interface %s not found", id)
	}

	return c.loadInterfaceData(ctx, wgReply.Data[0])
}

func (c *MikrotikController) loadInterfaceData(
	ctx context.Context,
	wireGuardObj lowlevel.GenericJsonObject,
) (*domain.PhysicalInterface, error) {
	deviceId := wireGuardObj.GetString(".id")
	deviceName := wireGuardObj.GetString("name")
	ifaceReply := c.client.Get(ctx, "/interface/"+deviceId, &lowlevel.MikrotikRequestOptions{
		PropList: []string{
			"name", "rx-byte", "tx-byte",
		},
	})
	if ifaceReply.Status != lowlevel.MikrotikApiStatusOk {
		return nil, fmt.Errorf("failed to query interface %s: %v", deviceId, ifaceReply.Error)
	}

	ipv4, ipv6, err := c.loadIpAddresses(ctx, deviceName)
	if err != nil {
		return nil, fmt.Errorf("failed to query IP addresses for interface %s: %v", deviceId, err)
	}
	addresses := c.convertIpAddresses(ipv4, ipv6)

	interfaceModel, err := c.convertWireGuardInterface(wireGuardObj, ifaceReply.Data, addresses)
	if err != nil {
		return nil, fmt.Errorf("interface convert failed for %s: %w", deviceName, err)
	}
	return &interfaceModel, nil
}

func (c *MikrotikController) loadIpAddresses(
	ctx context.Context,
	deviceName string,
) (ipv4 []lowlevel.GenericJsonObject, ipv6 []lowlevel.GenericJsonObject, err error) {
	// Query IPv4 and IPv6 addresses in parallel to reduce latency.
	var (
		v4    []lowlevel.GenericJsonObject
		v6    []lowlevel.GenericJsonObject
		v4Err error
		v6Err error
		wg    sync.WaitGroup
	)
	wg.Add(2)

	go func() {
		defer wg.Done()
		addrV4Reply := c.client.Query(ctx, "/ip/address", &lowlevel.MikrotikRequestOptions{
			PropList: []string{
				".id", "address", "network",
			},
			Filters: map[string]string{
				"interface": deviceName,
				"dynamic":   "false", // we only want static addresses
				"disabled":  "false", // we only want addresses that are not disabled
			},
		})
		if addrV4Reply.Status != lowlevel.MikrotikApiStatusOk {
			v4Err = fmt.Errorf("failed to query IPv4 addresses for interface %s: %v", deviceName, addrV4Reply.Error)
			return
		}
		v4 = addrV4Reply.Data
	}()

	go func() {
		defer wg.Done()
		addrV6Reply := c.client.Query(ctx, "/ipv6/address", &lowlevel.MikrotikRequestOptions{
			PropList: []string{
				".id", "address", "network",
			},
			Filters: map[string]string{
				"interface": deviceName,
				"dynamic":   "false", // we only want static addresses
				"disabled":  "false", // we only want addresses that are not disabled
			},
		})
		if addrV6Reply.Status != lowlevel.MikrotikApiStatusOk {
			v6Err = fmt.Errorf("failed to query IPv6 addresses for interface %s: %v", deviceName, addrV6Reply.Error)
			return
		}
		v6 = addrV6Reply.Data
	}()

	wg.Wait()
	if v4Err != nil {
		return nil, nil, v4Err
	}
	if v6Err != nil {
		return nil, nil, v6Err
	}

	return v4, v6, nil
}

func (c *MikrotikController) convertIpAddresses(
	ipv4, ipv6 []lowlevel.GenericJsonObject,
) []domain.Cidr {
	addresses := make([]domain.Cidr, 0, len(ipv4)+len(ipv6))
	for _, addr := range append(ipv4, ipv6...) {
		addrStr := addr.GetString("address")
		if addrStr == "" {
			continue
		}
		cidr, err := domain.CidrFromString(addrStr)
		if err != nil {
			continue
		}

		addresses = append(addresses, cidr)
	}

	return addresses
}

func (c *MikrotikController) convertWireGuardInterface(
	wg, iface lowlevel.GenericJsonObject,
	addresses []domain.Cidr,
) (
	domain.PhysicalInterface,
	error,
) {
	pi := domain.PhysicalInterface{
		Identifier: domain.InterfaceIdentifier(wg.GetString("name")),
		KeyPair: domain.KeyPair{
			PrivateKey: wg.GetString("private-key"),
			PublicKey:  wg.GetString("public-key"),
		},
		ListenPort:    wg.GetInt("listen-port"),
		Addresses:     addresses,
		Mtu:           wg.GetInt("mtu"),
		FirewallMark:  0,
		DeviceUp:      wg.GetBool("running"),
		ImportSource:  domain.ControllerTypeMikrotik,
		DeviceType:    domain.ControllerTypeMikrotik,
		BytesUpload:   uint64(iface.GetInt("tx-byte")),
		BytesDownload: uint64(iface.GetInt("rx-byte")),
	}

	pi.SetExtras(domain.MikrotikInterfaceExtras{
		Id:       wg.GetString(".id"),
		Comment:  wg.GetString("comment"),
		Disabled: wg.GetBool("disabled"),
	})

	return pi, nil
}

func (c *MikrotikController) GetPeers(ctx context.Context, deviceId domain.InterfaceIdentifier) (
	[]domain.PhysicalPeer,
	error,
) {
	wgReply := c.client.Query(ctx, "/interface/wireguard/peers", &lowlevel.MikrotikRequestOptions{
		PropList: []string{
			".id", "name", "allowed-address", "client-address", "client-endpoint", "client-keepalive", "comment",
			"current-endpoint-address", "current-endpoint-port", "last-handshake", "persistent-keepalive",
			"public-key", "private-key", "preshared-key", "mtu", "disabled", "rx", "tx", "responder", "client-dns",
		},
		Filters: map[string]string{
			"interface": string(deviceId),
		},
	})
	if wgReply.Status != lowlevel.MikrotikApiStatusOk {
		return nil, fmt.Errorf("failed to query peers for %s: %v", deviceId, wgReply.Error)
	}

	if len(wgReply.Data) == 0 {
		return nil, nil
	}

	peers := make([]domain.PhysicalPeer, 0, len(wgReply.Data))
	for _, peer := range wgReply.Data {
		peerModel, err := c.convertWireGuardPeer(peer)
		if err != nil {
			return nil, fmt.Errorf("peer convert failed for %v: %w", peer.GetString("name"), err)
		}
		peers = append(peers, peerModel)
	}

	return peers, nil
}

func (c *MikrotikController) convertWireGuardPeer(peer lowlevel.GenericJsonObject) (
	domain.PhysicalPeer,
	error,
) {
	keepAliveSeconds := 0
	duration, err := time.ParseDuration(peer.GetString("persistent-keepalive"))
	if err == nil {
		keepAliveSeconds = int(duration.Seconds())
	}

	currentEndpoint := ""
	if peer.GetString("current-endpoint-address") != "" && peer.GetString("current-endpoint-port") != "" {
		currentEndpoint = peer.GetString("current-endpoint-address") + ":" + peer.GetString("current-endpoint-port")
	}

	lastHandshakeTime := time.Time{}
	if peer.GetString("last-handshake") != "" {
		relDuration, err := time.ParseDuration(peer.GetString("last-handshake"))
		if err == nil {
			lastHandshakeTime = time.Now().Add(-relDuration)
		}
	}

	allowedAddresses, _ := domain.CidrsFromString(peer.GetString("allowed-address"))

	clientKeepAliveSeconds := 0
	duration, err = time.ParseDuration(peer.GetString("client-keepalive"))
	if err == nil {
		clientKeepAliveSeconds = int(duration.Seconds())
	}

	peerModel := domain.PhysicalPeer{
		Identifier: domain.PeerIdentifier(peer.GetString("public-key")),
		Endpoint:   currentEndpoint,
		AllowedIPs: allowedAddresses,
		KeyPair: domain.KeyPair{
			PublicKey:  peer.GetString("public-key"),
			PrivateKey: peer.GetString("private-key"),
		},
		PresharedKey:        domain.PreSharedKey(peer.GetString("preshared-key")),
		PersistentKeepalive: keepAliveSeconds,
		LastHandshake:       lastHandshakeTime,
		ProtocolVersion:     0, // Mikrotik does not support protocol versioning, so we set it to 0
		BytesUpload:         uint64(peer.GetInt("rx")),
		BytesDownload:       uint64(peer.GetInt("tx")),
		ImportSource:        domain.ControllerTypeMikrotik,
	}

	peerModel.SetExtras(domain.MikrotikPeerExtras{
		Id:              peer.GetString(".id"),
		Name:            peer.GetString("name"),
		Comment:         peer.GetString("comment"),
		IsResponder:     peer.GetBool("responder"),
		Disabled:        peer.GetBool("disabled"),
		ClientEndpoint:  peer.GetString("client-endpoint"),
		ClientAddress:   peer.GetString("client-address"),
		ClientDns:       peer.GetString("client-dns"),
		ClientKeepalive: clientKeepAliveSeconds,
	})

	return peerModel, nil
}

func (c *MikrotikController) SaveInterface(
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

	deviceId := physicalInterface.GetExtras().(domain.MikrotikInterfaceExtras).Id
	if updateFunc != nil {
		physicalInterface, err = updateFunc(physicalInterface)
		if err != nil {
			return err
		}
		newExtras := physicalInterface.GetExtras().(domain.MikrotikInterfaceExtras)
		newExtras.Id = deviceId // ensure the ID is not changed
		physicalInterface.SetExtras(newExtras)
	}

	if err := c.updateInterface(ctx, physicalInterface); err != nil {
		return err
	}

	return nil
}

func (c *MikrotikController) getOrCreateInterface(
	ctx context.Context,
	id domain.InterfaceIdentifier,
) (*domain.PhysicalInterface, error) {
	wgReply := c.client.Query(ctx, "/interface/wireguard", &lowlevel.MikrotikRequestOptions{
		PropList: []string{
			".id", "name", "public-key", "private-key", "listen-port", "mtu", "disabled", "running",
		},
		Filters: map[string]string{
			"name": string(id),
		},
	})
	if wgReply.Status == lowlevel.MikrotikApiStatusOk && len(wgReply.Data) > 0 {
		return c.loadInterfaceData(ctx, wgReply.Data[0])
	}

	// create a new interface if it does not exist
	createReply := c.client.Create(ctx, "/interface/wireguard", lowlevel.GenericJsonObject{
		"name": string(id),
	})
	if wgReply.Status == lowlevel.MikrotikApiStatusOk {
		return c.loadInterfaceData(ctx, createReply.Data)
	}

	return nil, fmt.Errorf("failed to create interface %s: %v", id, createReply.Error)
}

func (c *MikrotikController) updateInterface(ctx context.Context, pi *domain.PhysicalInterface) error {
	extras := pi.GetExtras().(domain.MikrotikInterfaceExtras)
	interfaceId := extras.Id
	wgReply := c.client.Update(ctx, "/interface/wireguard/"+interfaceId, lowlevel.GenericJsonObject{
		"name":        pi.Identifier,
		"comment":     extras.Comment,
		"mtu":         strconv.Itoa(pi.Mtu),
		"listen-port": strconv.Itoa(pi.ListenPort),
		"private-key": pi.KeyPair.PrivateKey,
		"disabled":    strconv.FormatBool(!pi.DeviceUp),
	})
	if wgReply.Status != lowlevel.MikrotikApiStatusOk {
		return fmt.Errorf("failed to update interface %s: %v", pi.Identifier, wgReply.Error)
	}

	// update the interface's addresses
	currentV4, currentV6, err := c.loadIpAddresses(ctx, string(pi.Identifier))
	if err != nil {
		return fmt.Errorf("failed to load current addresses for interface %s: %v", pi.Identifier, err)
	}
	currentAddresses := c.convertIpAddresses(currentV4, currentV6)

	// get all addresses that are currently not in the interface, only in pi
	newAddresses := make([]domain.Cidr, 0, len(pi.Addresses))
	for _, addr := range pi.Addresses {
		if slices.Contains(currentAddresses, addr) {
			continue
		}
		newAddresses = append(newAddresses, addr)
	}
	// get obsolete addresses that are in the interface, but not in pi
	obsoleteAddresses := make([]domain.Cidr, 0, len(currentAddresses))
	for _, addr := range currentAddresses {
		if slices.Contains(pi.Addresses, addr) {
			continue
		}
		obsoleteAddresses = append(obsoleteAddresses, addr)
	}

	// update the IP addresses for the interface
	if err := c.updateIpAddresses(ctx, string(pi.Identifier), currentV4, currentV6,
		newAddresses, obsoleteAddresses); err != nil {
		return fmt.Errorf("failed to update IP addresses for interface %s: %v", pi.Identifier, err)
	}

	return nil
}

func (c *MikrotikController) updateIpAddresses(
	ctx context.Context,
	deviceName string,
	currentV4, currentV6 []lowlevel.GenericJsonObject,
	new, obsolete []domain.Cidr,
) error {
	// first, delete all obsolete addresses
	for _, addr := range obsolete {
		// find ID of the address to delete
		if addr.IsV4() {
			for _, a := range currentV4 {
				if a.GetString("address") == addr.String() {
					// delete the address
					reply := c.client.Delete(ctx, "/ip/address/"+a.GetString(".id"))
					if reply.Status != lowlevel.MikrotikApiStatusOk {
						return fmt.Errorf("failed to delete obsolete IPv4 address %s: %v", addr, reply.Error)
					}
					break
				}
			}
		} else {
			for _, a := range currentV6 {
				if a.GetString("address") == addr.String() {
					// delete the address
					reply := c.client.Delete(ctx, "/ipv6/address/"+a.GetString(".id"))
					if reply.Status != lowlevel.MikrotikApiStatusOk {
						return fmt.Errorf("failed to delete obsolete IPv6 address %s: %v", addr, reply.Error)
					}
					break
				}
			}
		}
	}

	// then, add all new addresses
	for _, addr := range new {
		var createPath string
		if addr.IsV4() {
			createPath = "/ip/address"
		} else {
			createPath = "/ipv6/address"
		}

		// create the address
		reply := c.client.Create(ctx, createPath, lowlevel.GenericJsonObject{
			"address":   addr.String(),
			"interface": deviceName,
		})
		if reply.Status != lowlevel.MikrotikApiStatusOk {
			return fmt.Errorf("failed to create new address %s: %v", addr, reply.Error)
		}
	}

	return nil
}

func (c *MikrotikController) DeleteInterface(ctx context.Context, id domain.InterfaceIdentifier) error {
	// Lock the interface to prevent concurrent modifications
	mutex := c.getInterfaceMutex(id)
	mutex.Lock()
	defer mutex.Unlock()

	// delete the interface's addresses
	currentV4, currentV6, err := c.loadIpAddresses(ctx, string(id))
	if err != nil {
		return fmt.Errorf("failed to load current addresses for interface %s: %v", id, err)
	}
	for _, a := range currentV4 {
		// delete the address
		reply := c.client.Delete(ctx, "/ip/address/"+a.GetString(".id"))
		if reply.Status != lowlevel.MikrotikApiStatusOk {
			return fmt.Errorf("failed to delete IPv4 address %s: %v", a.GetString("address"), reply.Error)
		}
	}
	for _, a := range currentV6 {
		// delete the address
		reply := c.client.Delete(ctx, "/ipv6/address/"+a.GetString(".id"))
		if reply.Status != lowlevel.MikrotikApiStatusOk {
			return fmt.Errorf("failed to delete IPv6 address %s: %v", a.GetString("address"), reply.Error)
		}
	}

	// delete the WireGuard interface
	wgReply := c.client.Query(ctx, "/interface/wireguard", &lowlevel.MikrotikRequestOptions{
		PropList: []string{".id"},
		Filters: map[string]string{
			"name": string(id),
		},
	})
	if wgReply.Status != lowlevel.MikrotikApiStatusOk {
		return fmt.Errorf("unable to find WireGuard interface %s: %v", id, wgReply.Error)
	}
	if len(wgReply.Data) == 0 {
		return nil // interface does not exist, nothing to delete
	}

	interfaceId := wgReply.Data[0].GetString(".id")
	deleteReply := c.client.Delete(ctx, "/interface/wireguard/"+interfaceId)
	if deleteReply.Status != lowlevel.MikrotikApiStatusOk {
		return fmt.Errorf("failed to delete WireGuard interface %s: %v", id, deleteReply.Error)
	}

	return nil
}

func (c *MikrotikController) SavePeer(
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

	peerId := physicalPeer.GetExtras().(domain.MikrotikPeerExtras).Id
	physicalPeer, err = updateFunc(physicalPeer)
	if err != nil {
		return err
	}
	newExtras := physicalPeer.GetExtras().(domain.MikrotikPeerExtras)
	newExtras.Id = peerId // ensure the ID is not changed
	physicalPeer.SetExtras(newExtras)

	if err := c.updatePeer(ctx, deviceId, physicalPeer); err != nil {
		return err
	}

	return nil
}

func (c *MikrotikController) getOrCreatePeer(
	ctx context.Context,
	deviceId domain.InterfaceIdentifier,
	id domain.PeerIdentifier,
) (*domain.PhysicalPeer, error) {
	wgReply := c.client.Query(ctx, "/interface/wireguard/peers", &lowlevel.MikrotikRequestOptions{
		PropList: []string{
			".id", "name", "public-key", "private-key", "preshared-key", "persistent-keepalive", "client-address",
			"client-endpoint", "client-keepalive", "allowed-address", "client-dns", "comment", "disabled", "responder",
		},
		Filters: map[string]string{
			"public-key": string(id),
			"interface":  string(deviceId),
		},
	})
	if wgReply.Status == lowlevel.MikrotikApiStatusOk && len(wgReply.Data) > 0 {
		slog.Debug("found existing Mikrotik peer", "peer", id, "interface", deviceId)
		existingPeer, err := c.convertWireGuardPeer(wgReply.Data[0])
		if err != nil {
			return nil, err
		}
		return &existingPeer, nil
	}

	// create a new peer if it does not exist
	slog.Debug("creating new Mikrotik peer", "peer", id, "interface", deviceId)
	createReply := c.client.Create(ctx, "/interface/wireguard/peers", lowlevel.GenericJsonObject{
		"name":            fmt.Sprintf("tmp-wg-%s", id[0:8]),
		"interface":       string(deviceId),
		"public-key":      string(id),
		"allowed-address": "0.0.0.0/0", // Use 0.0.0.0/0 as default, will be updated by updatePeer
	})
	if createReply.Status == lowlevel.MikrotikApiStatusOk {
		newPeer, err := c.convertWireGuardPeer(createReply.Data)
		if err != nil {
			return nil, err
		}
		slog.Debug("successfully created Mikrotik peer", "peer", id, "interface", deviceId)
		return &newPeer, nil
	}

	return nil, fmt.Errorf("failed to create peer %s for interface %s: %v", id, deviceId, createReply.Error)
}

func (c *MikrotikController) updatePeer(
	ctx context.Context,
	deviceId domain.InterfaceIdentifier,
	pp *domain.PhysicalPeer,
) error {
	extras := pp.GetExtras().(domain.MikrotikPeerExtras)
	peerId := extras.Id

	endpoint := ""           // by default, we have no endpoint (the peer does not initiate a connection)
	endpointPort := "0"      // by default, we have no endpoint port (the peer does not initiate a connection)
	if !extras.IsResponder { // if the peer is not only a responder, it needs the endpoint to initiate a connection
		endpoint = pp.Endpoint
		endpointPort = "51820" // default port if not set
		if s := strings.Split(endpoint, ":"); len(s) == 2 {
			endpoint = s[0]
			endpointPort = s[1]
		}
	}

	allowedAddressStr := domain.CidrsToString(pp.AllowedIPs)
	slog.Debug("updating Mikrotik peer",
		"peer", pp.Identifier,
		"interface", deviceId,
		"allowed-address", allowedAddressStr,
		"allowed-ips-count", len(pp.AllowedIPs),
		"disabled", extras.Disabled)

	wgReply := c.client.Update(ctx, "/interface/wireguard/peers/"+peerId, lowlevel.GenericJsonObject{
		"name":                 extras.Name,
		"comment":              extras.Comment,
		"preshared-key":        pp.PresharedKey,
		"public-key":           pp.KeyPair.PublicKey,
		"private-key":          pp.KeyPair.PrivateKey,
		"persistent-keepalive": (time.Duration(pp.PersistentKeepalive) * time.Second).String(),
		"disabled":             strconv.FormatBool(extras.Disabled),
		"responder":            strconv.FormatBool(extras.IsResponder),
		"client-endpoint":      extras.ClientEndpoint,
		"client-address":       extras.ClientAddress,
		"client-keepalive":     (time.Duration(extras.ClientKeepalive) * time.Second).String(),
		"client-dns":           extras.ClientDns,
		"endpoint-address":     endpoint,
		"endpoint-port":        endpointPort,
		"allowed-address":      allowedAddressStr, // Add the missing allowed-address field
	})
	if wgReply.Status != lowlevel.MikrotikApiStatusOk {
		return fmt.Errorf("failed to update peer %s on interface %s: %v", pp.Identifier, deviceId, wgReply.Error)
	}

	if extras.Disabled {
		slog.Debug("successfully disabled Mikrotik peer", "peer", pp.Identifier, "interface", deviceId)
	} else {
		slog.Debug("successfully updated Mikrotik peer", "peer", pp.Identifier, "interface", deviceId)
	}

	return nil
}

func (c *MikrotikController) DeletePeer(
	ctx context.Context,
	deviceId domain.InterfaceIdentifier,
	id domain.PeerIdentifier,
) error {
	// Lock the peer to prevent concurrent modifications
	mutex := c.getPeerMutex(id)
	mutex.Lock()
	defer mutex.Unlock()

	wgReply := c.client.Query(ctx, "/interface/wireguard/peers", &lowlevel.MikrotikRequestOptions{
		PropList: []string{".id"},
		Filters: map[string]string{
			"public-key": string(id),
			"interface":  string(deviceId),
		},
	})
	if wgReply.Status != lowlevel.MikrotikApiStatusOk {
		return fmt.Errorf("unable to find WireGuard peer %s for interface %s: %v", id, deviceId, wgReply.Error)
	}
	if len(wgReply.Data) == 0 {
		return nil // peer does not exist, nothing to delete
	}

	peerId := wgReply.Data[0].GetString(".id")
	deleteReply := c.client.Delete(ctx, "/interface/wireguard/peers/"+peerId)
	if deleteReply.Status != lowlevel.MikrotikApiStatusOk {
		return fmt.Errorf("failed to delete WireGuard peer %s for interface %s: %v", id, deviceId, deleteReply.Error)
	}

	return nil
}

// endregion wireguard-related

// region wg-quick-related

func (c *MikrotikController) ExecuteInterfaceHook(
	_ context.Context,
	_ domain.InterfaceIdentifier,
	_ string,
) error {
	// TODO implement me
	slog.Error("interface hooks are not yet supported for Mikrotik backends, please open an issue on GitHub")
	return nil
}

func (c *MikrotikController) SetDNS(
	ctx context.Context,
	_ domain.InterfaceIdentifier,
	dnsStr, _ string,
) error {
	// Lock the interface to prevent concurrent modifications
	c.coreMutex.Lock()
	defer c.coreMutex.Unlock()

	// check if the server is already configured
	wgReply := c.client.Get(ctx, "/ip/dns", &lowlevel.MikrotikRequestOptions{
		PropList: []string{"servers"},
	})
	if wgReply.Status != lowlevel.MikrotikApiStatusOk {
		return fmt.Errorf("unable to find WireGuard dns settings: %v", wgReply.Error)
	}

	var existingServers []string
	existingServers = append(existingServers, strings.Split(wgReply.Data.GetString("servers"), ",")...)

	newServers := strings.Split(dnsStr, ",")

	mergedServers := slices.Clone(existingServers)
	for _, s := range newServers {
		if s == "" {
			continue
		}
		if !slices.Contains(mergedServers, s) {
			mergedServers = append(mergedServers, s)
		}
	}
	mergedServersStr := strings.Join(mergedServers, ",")

	reply := c.client.ExecList(ctx, "/ip/dns/set", lowlevel.GenericJsonObject{
		"servers": mergedServersStr,
	})
	if reply.Status != lowlevel.MikrotikApiStatusOk {
		return fmt.Errorf("failed to set DNS servers: %s: %v", mergedServersStr, reply.Error)
	}

	return nil
}

func (c *MikrotikController) UnsetDNS(
	ctx context.Context,
	_ domain.InterfaceIdentifier,
	dnsStr, _ string,
) error {
	// Lock the interface to prevent concurrent modifications
	c.coreMutex.Lock()
	defer c.coreMutex.Unlock()

	// retrieve current DNS settings
	wgReply := c.client.Get(ctx, "/ip/dns", &lowlevel.MikrotikRequestOptions{
		PropList: []string{"servers"},
	})
	if wgReply.Status != lowlevel.MikrotikApiStatusOk {
		return fmt.Errorf("unable to find WireGuard dns settings: %v", wgReply.Error)
	}

	var existingServers []string
	existingServers = append(existingServers, strings.Split(wgReply.Data.GetString("servers"), ",")...)

	oldServers := strings.Split(dnsStr, ",")

	mergedServers := make([]string, 0, len(existingServers))
	for _, s := range existingServers {
		if s == "" {
			continue
		}
		if !slices.Contains(oldServers, s) {
			mergedServers = append(mergedServers, s) // only keep the servers that are not in the old list
		}
	}
	mergedServersStr := strings.Join(mergedServers, ",")

	reply := c.client.ExecList(ctx, "/ip/dns/set", lowlevel.GenericJsonObject{
		"servers": mergedServersStr,
	})
	if reply.Status != lowlevel.MikrotikApiStatusOk {
		return fmt.Errorf("failed to set DNS servers: %s: %v", mergedServersStr, reply.Error)
	}

	return nil
}

// endregion wg-quick-related

// region routing-related

// SetRoutes sets the routes for the given interface. If no routes are provided, the function is a no-op.
func (c *MikrotikController) SetRoutes(ctx context.Context, info domain.RoutingTableInfo) error {
	interfaceId := info.Interface.Identifier
	slog.Debug("setting mikrotik routes", "interface", interfaceId, "table", info.TableStr, "cidrs", info.AllowedIps)

	// Mikrotik needs some time to apply the changes.
	// If we don't wait, the routes might get created multiple times as the dynamic routes are not yet available.
	time.Sleep(2 * time.Second)

	tableName, err := c.getOrCreateRoutingTables(ctx, info.Interface.Identifier, info.TableStr)
	if err != nil {
		return fmt.Errorf("failed to get or create routing table for %s: %v", interfaceId, err)
	}

	cidrsV4, cidrsV6 := domain.CidrsPerFamily(info.AllowedIps)

	err = c.setRoutesForFamily(ctx, interfaceId, false, tableName, cidrsV4)
	if err != nil {
		return fmt.Errorf("failed to set IPv4 routes for %s: %v", interfaceId, err)
	}

	err = c.setRoutesForFamily(ctx, interfaceId, true, tableName, cidrsV6)
	if err != nil {
		return fmt.Errorf("failed to set IPv6 routes for %s: %v", interfaceId, err)
	}

	return nil
}

func (c *MikrotikController) resolveRouteTableName(name string) string {
	name = strings.TrimSpace(name)

	var mikrotikTableName string
	switch strings.ToLower(name) {
	case "", "0":
		mikrotikTableName = MikrotikDefaultRoutingTable
	case MikrotikDefaultRoutingTable:
		return fmt.Sprintf("wgportal-%s",
			MikrotikDefaultRoutingTable) // if the Mikrotik Main table should be used, the table-name should be left empty or set to "0".
	default:
		mikrotikTableName = name
	}

	return mikrotikTableName
}

func (c *MikrotikController) getOrCreateRoutingTables(
	ctx context.Context,
	interfaceId domain.InterfaceIdentifier,
	table string,
) (string, error) {
	// retrieve current routing tables
	wgReply := c.client.Query(ctx, "/routing/table", &lowlevel.MikrotikRequestOptions{
		PropList: []string{
			".id", "dynamic", "fib", "name",
		},
	})
	if wgReply.Status != lowlevel.MikrotikApiStatusOk {
		return "", fmt.Errorf("unable to query routing tables: %v", wgReply.Error)
	}

	wantedTableName := c.resolveRouteTableName(table)

	// check if the table already exists
	for _, table := range wgReply.Data {
		if table.GetString("name") == wantedTableName {
			return wantedTableName, nil // already exists, nothing to do
		}
	}

	// create the table if it does not exist
	createReply := c.client.Create(ctx, "/routing/table", lowlevel.GenericJsonObject{
		"name":    wantedTableName,
		"comment": fmt.Sprintf("Routing Table for %s", interfaceId),
		"fib":     strconv.FormatBool(true),
	})
	if createReply.Status != lowlevel.MikrotikApiStatusOk {
		return "", fmt.Errorf("failed to create routing table %s: %v", wantedTableName, createReply.Error)
	}

	return wantedTableName, nil
}

func (c *MikrotikController) setRoutesForFamily(
	ctx context.Context,
	interfaceId domain.InterfaceIdentifier,
	ipV6 bool,
	table string,
	cidrs []domain.Cidr,
) error {
	apiPath := "/ip/route"
	if ipV6 {
		apiPath = "/ipv6/route"
	}

	// retrieve current routes
	wgReply := c.client.Query(ctx, apiPath, &lowlevel.MikrotikRequestOptions{
		PropList: []string{
			".id", "disabled", "inactive", "distance", "dst-address", "dynamic", "gateway", "immediate-gw",
			"routing-table", "scope", "target-scope", "client-dns", "comment", "disabled", "responder",
		},
		Filters: map[string]string{
			"gateway": string(interfaceId),
		},
	})
	if wgReply.Status != lowlevel.MikrotikApiStatusOk {
		return fmt.Errorf("unable to find WireGuard IP route settings (v6=%t): %v", ipV6, wgReply.Error)
	}

	// first create or update the routes
	for _, cidr := range cidrs {
		// check if the route already exists
		exists := false
		for _, route := range wgReply.Data {
			existingRoute, err := domain.CidrFromString(route.GetString("dst-address"))
			if err != nil {
				slog.Warn("failed to parse route destination address",
					"cidr", route.GetString("dst-address"), "error", err)
				continue
			}
			if existingRoute.EqualPrefix(cidr) && route.GetString("routing-table") == table {
				exists = true
				break
			}
		}
		if exists {
			continue // route already exists, nothing to do
		}

		// create the route
		reply := c.client.Create(ctx, apiPath, lowlevel.GenericJsonObject{
			"gateway":       string(interfaceId),
			"dst-address":   cidr.String(),
			"distance":      strconv.Itoa(MikrotikRouteDistance),
			"disabled":      strconv.FormatBool(false),
			"routing-table": table,
		})
		if reply.Status != lowlevel.MikrotikApiStatusOk {
			return fmt.Errorf("failed to create new route %s via %s: %v", cidr.String(), interfaceId, reply.Error)
		}
	}

	// finally, remove the routes that are not in the new list
	for _, route := range wgReply.Data {
		if route.GetBool("dynamic") {
			continue // dynamic routes are not managed by the controller, nothing to do
		}

		existingRoute, err := domain.CidrFromString(route.GetString("dst-address"))
		if err != nil {
			slog.Warn("failed to parse route destination address",
				"cidr", route.GetString("dst-address"), "error", err)
			continue
		}

		valid := false
		for _, cidr := range cidrs {
			if existingRoute.EqualPrefix(cidr) {
				valid = true
				break
			}
		}
		if valid {
			continue // route is still valid, nothing to do
		}

		// remove the route
		reply := c.client.Delete(ctx, apiPath+"/"+route.GetString(".id"))
		if reply.Status != lowlevel.MikrotikApiStatusOk {
			return fmt.Errorf("failed to remove outdated route %s: %v", existingRoute.String(), reply.Error)
		}
	}

	return nil
}

// RemoveRoutes removes the routes for the given interface. If no routes are provided, the function is a no-op.
func (c *MikrotikController) RemoveRoutes(ctx context.Context, info domain.RoutingTableInfo) error {
	interfaceId := info.Interface.Identifier
	slog.Debug("removing mikrotik routes", "interface", interfaceId, "table", info.TableStr, "cidrs", info.AllowedIps)

	tableName := c.resolveRouteTableName(info.TableStr)

	cidrsV4, cidrsV6 := domain.CidrsPerFamily(info.AllowedIps)

	err := c.removeRoutesForFamily(ctx, interfaceId, false, tableName, cidrsV4)
	if err != nil {
		return fmt.Errorf("failed to remove IPv4 routes for %s: %v", interfaceId, err)
	}

	err = c.removeRoutesForFamily(ctx, interfaceId, true, tableName, cidrsV6)
	if err != nil {
		return fmt.Errorf("failed to remove IPv6 routes for %s: %v", interfaceId, err)
	}

	err = c.removeRoutingTable(ctx, tableName)
	if err != nil {
		return fmt.Errorf("failed to remove routing table for %s: %v", interfaceId, err)
	}

	return nil
}

func (c *MikrotikController) removeRoutesForFamily(
	ctx context.Context,
	interfaceId domain.InterfaceIdentifier,
	ipV6 bool,
	table string,
	cidrs []domain.Cidr,
) error {
	apiPath := "/ip/route"
	if ipV6 {
		apiPath = "/ipv6/route"
	}

	// retrieve current routes
	wgReply := c.client.Query(ctx, apiPath, &lowlevel.MikrotikRequestOptions{
		PropList: []string{
			".id", "disabled", "inactive", "distance", "dst-address", "dynamic", "gateway", "immediate-gw",
			"routing-table", "scope", "target-scope", "client-dns", "comment", "disabled", "responder",
		},
		Filters: map[string]string{
			"gateway": string(interfaceId),
		},
	})
	if wgReply.Status != lowlevel.MikrotikApiStatusOk {
		return fmt.Errorf("unable to find WireGuard IP route settings (v6=%t): %v", ipV6, wgReply.Error)
	}

	// remove the routes from the list
	for _, route := range wgReply.Data {
		if route.GetBool("dynamic") {
			continue // dynamic routes are not managed by the controller, nothing to do
		}

		existingRoute, err := domain.CidrFromString(route.GetString("dst-address"))
		if err != nil {
			slog.Warn("failed to parse route destination address",
				"cidr", route.GetString("dst-address"), "error", err)
			continue
		}

		remove := false
		for _, cidr := range cidrs {
			if existingRoute.EqualPrefix(cidr) && route.GetString("routing-table") == table {
				remove = true
				break
			}
		}
		if !remove {
			continue // route is still valid, nothing to do
		}

		// remove the route
		reply := c.client.Delete(ctx, apiPath+"/"+route.GetString(".id"))
		if reply.Status != lowlevel.MikrotikApiStatusOk {
			return fmt.Errorf("failed to remove old route %s: %v", existingRoute.String(), reply.Error)
		}
	}

	return nil
}

func (c *MikrotikController) removeRoutingTable(
	ctx context.Context,
	table string,
) error {
	if table == MikrotikDefaultRoutingTable {
		return nil // we cannot remove the default table
	}

	// retrieve current routing tables
	wgReply := c.client.Query(ctx, "/routing/table", &lowlevel.MikrotikRequestOptions{
		PropList: []string{
			".id", "dynamic", "fib", "name",
		},
	})
	if wgReply.Status != lowlevel.MikrotikApiStatusOk {
		return fmt.Errorf("unable to query routing tables: %v", wgReply.Error)
	}

	for _, existingTable := range wgReply.Data {
		if existingTable.GetBool("dynamic") {
			continue // dynamic tables are not managed by the controller, nothing to do
		}
		if existingTable.GetString("name") != table {
			continue // not the table we want to remove
		}

		// remove the table
		reply := c.client.Delete(ctx, "/routing/table/"+existingTable.GetString(".id"))
		if reply.Status != lowlevel.MikrotikApiStatusOk {
			return fmt.Errorf("failed to remove routing table %s: %v", table, reply.Error)
		}
		return nil
	}

	return nil
}

// endregion routing-related

// region statistics-related

func (c *MikrotikController) PingAddresses(
	ctx context.Context,
	addr string,
) (*domain.PingerResult, error) {
	wgReply := c.client.ExecList(ctx, "/tool/ping",
		// limit to 1 packet with a max running time of 2 seconds
		lowlevel.GenericJsonObject{"address": addr, "count": 1, "interval": "00:00:02"},
	)

	if wgReply.Status != lowlevel.MikrotikApiStatusOk {
		return nil, fmt.Errorf("failed to ping %s: %v", addr, wgReply.Error)
	}

	var result domain.PingerResult
	for _, item := range wgReply.Data {
		result.PacketsRecv += item.GetInt("received")
		result.PacketsSent += item.GetInt("sent")

		rttStr := item.GetString("avg-rtt")
		if rttStr != "" {
			rtt, err := time.ParseDuration(rttStr)
			if err == nil {
				result.Rtts = append(result.Rtts, rtt)
			} else {
				// use a high value to indicate failure or timeout
				result.Rtts = append(result.Rtts, 999999*time.Millisecond)
			}
		}
	}

	return &result, nil
}

// endregion statistics-related
