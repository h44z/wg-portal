package wgcontroller

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"time"

	"github.com/h44z/wg-portal/internal/config"
	"github.com/h44z/wg-portal/internal/domain"
	"github.com/h44z/wg-portal/internal/lowlevel"
)

type MikrotikController struct {
	coreCfg *config.Config
	cfg     *config.BackendMikrotik

	client *lowlevel.MikrotikApiClient
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
	}, nil
}

func (c MikrotikController) GetId() domain.InterfaceBackend {
	return domain.InterfaceBackend(c.cfg.Id)
}

// region wireguard-related

func (c MikrotikController) GetInterfaces(ctx context.Context) ([]domain.PhysicalInterface, error) {
	wgReply := c.client.Query(ctx, "/interface/wireguard", &lowlevel.MikrotikRequestOptions{
		PropList: []string{
			".id", "name", "public-key", "private-key", "listen-port", "mtu", "disabled", "running", "comment",
		},
	})
	if wgReply.Status != lowlevel.MikrotikApiStatusOk {
		return nil, fmt.Errorf("failed to query interfaces: %v", wgReply.Error)
	}

	interfaces := make([]domain.PhysicalInterface, 0, len(wgReply.Data))
	for _, wg := range wgReply.Data {
		physicalInterface, err := c.loadInterfaceData(ctx, wg)
		if err != nil {
			return nil, err
		}
		interfaces = append(interfaces, *physicalInterface)
	}

	return interfaces, nil
}

func (c MikrotikController) GetInterface(ctx context.Context, id domain.InterfaceIdentifier) (
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

func (c MikrotikController) loadInterfaceData(
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

func (c MikrotikController) loadIpAddresses(
	ctx context.Context,
	deviceName string,
) (ipv4 []lowlevel.GenericJsonObject, ipv6 []lowlevel.GenericJsonObject, err error) {
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
		return nil, nil, fmt.Errorf("failed to query IPv4 addresses for interface %s: %v", deviceName,
			addrV4Reply.Error)
	}

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
		return nil, nil, fmt.Errorf("failed to query IPv6 addresses for interface %s: %v", deviceName,
			addrV6Reply.Error)
	}

	return addrV4Reply.Data, addrV6Reply.Data, nil
}

func (c MikrotikController) convertIpAddresses(
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

func (c MikrotikController) convertWireGuardInterface(
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

func (c MikrotikController) GetPeers(ctx context.Context, deviceId domain.InterfaceIdentifier) (
	[]domain.PhysicalPeer,
	error,
) {
	wgReply := c.client.Query(ctx, "/interface/wireguard/peers", &lowlevel.MikrotikRequestOptions{
		PropList: []string{
			".id", "name", "allowed-address", "client-address", "client-endpoint", "client-keepalive", "comment",
			"current-endpoint-address", "current-endpoint-port", "last-handshake", "persistent-keepalive",
			"public-key", "private-key", "preshared-key", "mtu", "disabled", "rx", "tx", "responder",
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

func (c MikrotikController) convertWireGuardPeer(peer lowlevel.GenericJsonObject) (
	domain.PhysicalPeer,
	error,
) {
	keepAliveSeconds := 0
	duration, err := time.ParseDuration(peer.GetString("client-keepalive"))
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
		Name:           peer.GetString("name"),
		Comment:        peer.GetString("comment"),
		IsResponder:    peer.GetBool("responder"),
		ClientEndpoint: peer.GetString("client-endpoint"),
		ClientAddress:  peer.GetString("client-address"),
		Disabled:       peer.GetBool("disabled"),
	})

	return peerModel, nil
}

func (c MikrotikController) SaveInterface(
	ctx context.Context,
	id domain.InterfaceIdentifier,
	updateFunc func(pi *domain.PhysicalInterface) (*domain.PhysicalInterface, error),
) error {
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

func (c MikrotikController) getOrCreateInterface(
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

func (c MikrotikController) updateInterface(ctx context.Context, pi *domain.PhysicalInterface) error {
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

func (c MikrotikController) updateIpAddresses(
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

func (c MikrotikController) DeleteInterface(ctx context.Context, id domain.InterfaceIdentifier) error {
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
	if wgReply.Status != lowlevel.MikrotikApiStatusOk || len(wgReply.Data) == 0 {
		return fmt.Errorf("unable to find WireGuard interface %s: %v", id, wgReply.Error)
	}

	deviceId := wgReply.Data[0].GetString(".id")
	deleteReply := c.client.Delete(ctx, "/interface/wireguard/"+deviceId)
	if deleteReply.Status != lowlevel.MikrotikApiStatusOk {
		return fmt.Errorf("failed to delete WireGuard interface %s: %v", id, deleteReply.Error)
	}

	return nil
}

func (c MikrotikController) SavePeer(
	_ context.Context,
	deviceId domain.InterfaceIdentifier,
	id domain.PeerIdentifier,
	updateFunc func(pp *domain.PhysicalPeer) (*domain.PhysicalPeer, error),
) error {
	// TODO implement me
	return nil
}

func (c MikrotikController) DeletePeer(
	_ context.Context,
	deviceId domain.InterfaceIdentifier,
	id domain.PeerIdentifier,
) error {
	// TODO implement me
	return nil
}

// endregion wireguard-related

// region wg-quick-related

func (c MikrotikController) ExecuteInterfaceHook(id domain.InterfaceIdentifier, hookCmd string) error {
	// TODO implement me
	panic("implement me")
}

func (c MikrotikController) SetDNS(id domain.InterfaceIdentifier, dnsStr, dnsSearchStr string) error {
	// TODO implement me
	panic("implement me")
}

func (c MikrotikController) UnsetDNS(id domain.InterfaceIdentifier) error {
	// TODO implement me
	panic("implement me")
}

// endregion wg-quick-related

// region routing-related

func (c MikrotikController) SyncRouteRules(_ context.Context, rules []domain.RouteRule) error {
	// TODO implement me
	panic("implement me")
}

func (c MikrotikController) DeleteRouteRules(_ context.Context, rules []domain.RouteRule) error {
	// TODO implement me
	panic("implement me")
}

// endregion routing-related

// region statistics-related

func (c MikrotikController) PingAddresses(
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
