package wgcontroller

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/h44z/wg-portal/internal/config"
	"github.com/h44z/wg-portal/internal/domain"
	"github.com/h44z/wg-portal/internal/lowlevel"
)

// OpnsenseController implements the InterfaceController interface for OPNsense
// firewalls, using the WireGuard API that ships in OPNsense core
// (https://docs.opnsense.org/development/api/plugins/wireguard.html).
//
// Unlike the pfSense backend, which depends on the third-party pfSense-API
// package, no add-on is required: a stock install answers /api/wireguard/*.
//
// Terminology mapping, which is the main thing to keep straight while reading
// this file:
//
//	OPNsense "server" == a WireGuard tunnel      == domain.PhysicalInterface
//	OPNsense "client" == a peer on that tunnel   == domain.PhysicalPeer
//
// A server's device name (wg0, wg1, ...) is derived by OPNsense from its
// `instance` number and reported back in the `interface` field. That device
// name is what wg-portal uses as the interface identifier, so that an interface
// imported from OPNsense looks the same as one managed by the local backend.
// The human-readable `name` field becomes the display name.
//
// Reads use the searchXxx endpoints rather than getXxx: search returns every
// record with select fields already flattened to comma-joined scalars, so a
// full enumeration costs a fixed number of calls and needs no per-record
// fan-out. getXxx, by contrast, returns select fields as maps and is only used
// where a full record must be round-tripped through a write.

const (
	// OPNsense stages WireGuard changes and only applies them when the service
	// is reconfigured. Verified against a live 26.7 instance: reconfiguring
	// while a peer is connected neither drops nor rekeys the existing session,
	// so it is safe to call after every mutation.
	opnsenseReconfigureEndpoint = "/api/wireguard/service/reconfigure"

	// Bootgrid-style search endpoints paginate. Ask for everything explicitly
	// rather than relying on the default page size: a silently truncated list
	// would read as "these peers do not exist" and provoke duplicate creates.
	opnsenseSearchAllParams = "?current=1&rowCount=-1"
)

// Compile-time proof that the controller satisfies the backend contract. The
// wg-quick and routing interfaces live in the app layer and cannot be asserted
// here without an import cycle; they are covered in the controller manager test.
var _ domain.InterfaceController = (*OpnsenseController)(nil)

type OpnsenseController struct {
	coreCfg *config.Config
	cfg     *config.BackendOpnsense

	client *lowlevel.OpnsenseApiClient

	interfaceMutexes sync.Map   // map[domain.InterfaceIdentifier]*sync.Mutex
	peerMutexes      sync.Map   // map[domain.PeerIdentifier]*sync.Mutex
	coreMutex        sync.Mutex // for updating core configuration such as routes or DNS
}

func NewOpnsenseController(coreCfg *config.Config, cfg *config.BackendOpnsense) (*OpnsenseController, error) {
	client, err := lowlevel.NewOpnsenseApiClient(coreCfg, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create OPNsense API client: %w", err)
	}

	return &OpnsenseController{
		coreCfg: coreCfg,
		cfg:     cfg,

		client: client,

		interfaceMutexes: sync.Map{},
		peerMutexes:      sync.Map{},
		coreMutex:        sync.Mutex{},
	}, nil
}

func (c *OpnsenseController) GetId() domain.InterfaceBackend {
	return domain.InterfaceBackend(c.cfg.Id)
}

// getInterfaceMutex returns a mutex for the given interface to prevent concurrent modifications
func (c *OpnsenseController) getInterfaceMutex(id domain.InterfaceIdentifier) *sync.Mutex {
	mutex, _ := c.interfaceMutexes.LoadOrStore(id, &sync.Mutex{})
	return mutex.(*sync.Mutex)
}

// getPeerMutex returns a mutex for the given peer to prevent concurrent modifications
func (c *OpnsenseController) getPeerMutex(id domain.PeerIdentifier) *sync.Mutex {
	mutex, _ := c.peerMutexes.LoadOrStore(id, &sync.Mutex{})
	return mutex.(*sync.Mutex)
}

// region helpers

// opnsenseName renders a name OPNsense will accept for a tunnel or a peer.
//
// Both models validate the name as 1-64 characters of alphanumerics, dash and
// underscore only, and reject anything else outright rather than coercing it:
//
//	{"client.name":"Should be a string between 1 and 64 characters.
//	  Allowed characters are alphanumeric characters, dash and underscores."}
//
// Two ordinary inputs fall foul of that. A wg-portal display name may contain
// spaces ("bob staff laptop"), and the fallback when no display name is set is
// the peer's identifier -- a WireGuard public key, which is base64 and so
// contains "+", "/" and "=". Both must be folded into the allowed set.
func opnsenseName(preferred, fallback string) string {
	if name := sanitizeOpnsenseName(preferred); name != "" {
		return name
	}
	if name := sanitizeOpnsenseName(fallback); name != "" {
		return name
	}
	return "wg-portal"
}

func sanitizeOpnsenseName(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	lastDash := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_', r == '-':
			b.WriteRune(r)
			lastDash = false
		default:
			// Collapse any run of disallowed characters into a single dash,
			// and never start with one.
			if !lastDash && b.Len() > 0 {
				b.WriteRune('-')
				lastDash = true
			}
		}
	}

	out := strings.Trim(b.String(), "-")
	if len(out) > 64 {
		out = strings.Trim(out[:64], "-")
	}
	return out
}

// parseCidrsTolerant parses a comma-separated CIDR list, dropping entries it
// cannot read instead of failing.
//
// Enumeration runs over every record the firewall holds, including ones this
// portal did not create. One malformed or absent address must not take out the
// whole listing: GetInterfaces errors propagate into the startup importer,
// which treats them as fatal for all backends.
func parseCidrsTolerant(value, what, owner string) []domain.Cidr {
	if strings.TrimSpace(value) == "" {
		return nil
	}

	parsed := make([]domain.Cidr, 0, 1)
	for _, part := range strings.Split(value, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		cidr, err := domain.CidrFromString(part)
		if err != nil {
			slog.Warn("ignoring unparsable OPNsense address",
				"kind", what, "owner", owner, "value", part, "error", err)
			continue
		}
		parsed = append(parsed, cidr)
	}
	if len(parsed) == 0 {
		return nil
	}
	return parsed
}

// optionalPositiveInt renders an optional numeric field: a positive value as
// its decimal string, anything else as empty.
//
// Sending the empty string rather than omitting the key is deliberate. These
// models patch by key, so an omitted field keeps whatever the firewall already
// held, which would make "the operator cleared the MTU" indistinguishable from
// "the operator did not mention the MTU".
func optionalPositiveInt(value int) string {
	if value > 0 {
		return strconv.Itoa(value)
	}
	return ""
}

// opnsenseBool renders a Go bool in the "0"/"1" string form the API expects.
// OPNsense never accepts JSON booleans on these models.
func opnsenseBool(value bool) string {
	if value {
		return "1"
	}
	return "0"
}

// deviceNameForInstance mirrors OPNsense's own naming: instance 0 is wg0.
func deviceNameForInstance(instance string) string {
	if instance == "" {
		return ""
	}
	return "wg" + instance
}

// instanceForDeviceName is the inverse, used when creating a tunnel for an
// interface identifier that wg-portal chose (e.g. "wg2" -> instance "2").
// Returns "" when the name does not follow the wgN convention, in which case
// the caller must let OPNsense allocate an instance.
func instanceForDeviceName(id domain.InterfaceIdentifier) string {
	name := string(id)
	if !strings.HasPrefix(name, "wg") {
		return ""
	}
	suffix := strings.TrimPrefix(name, "wg")
	if suffix == "" {
		return ""
	}
	if _, err := strconv.Atoi(suffix); err != nil {
		return ""
	}
	return suffix
}

// opnsenseStats holds the runtime counters exposed by service/show, which are
// not part of the configuration models.
type opnsenseStats struct {
	lastHandshake time.Time
	bytesReceived uint64
	bytesSent     uint64
	endpoint      string
	up            bool
}

// loadStats builds lookup tables from `wg show`-equivalent output. Peers are
// keyed by device name and public key because a public key may legitimately
// appear on more than one tunnel.
func (c *OpnsenseController) loadStats(ctx context.Context) (
	map[string]opnsenseStats, // by device name (wg0), for interfaces
	map[string]opnsenseStats, // by "wg0/<pubkey>", for peers
) {
	interfaceStats := make(map[string]opnsenseStats)
	peerStats := make(map[string]opnsenseStats)

	reply := c.client.Search(ctx, "/api/wireguard/service/show")
	if reply.Status != lowlevel.OpnsenseApiStatusOk {
		// Statistics are decoration: a tunnel that is otherwise readable should
		// not disappear because the service is stopped or the call failed.
		slog.Debug("could not load OPNsense WireGuard statistics",
			"backend", c.cfg.Id, "error", reply.Error.String())
		return interfaceStats, peerStats
	}

	for _, row := range reply.Data.Rows {
		device := row.GetString("if")
		if device == "" {
			continue
		}
		switch row.GetString("type") {
		case "interface":
			interfaceStats[device] = opnsenseStats{
				up:       row.GetString("status") == "up",
				endpoint: row.GetString("endpoint"),
			}
		case "peer":
			publicKey := row.GetString("public-key")
			if publicKey == "" {
				continue
			}
			stats := opnsenseStats{
				bytesReceived: uint64(row.GetInt("transfer-rx")),
				bytesSent:     uint64(row.GetInt("transfer-tx")),
				endpoint:      row.GetString("endpoint"),
			}
			if epoch := row.GetInt("latest-handshake"); epoch > 0 {
				stats.lastHandshake = time.Unix(int64(epoch), 0)
			}
			peerStats[device+"/"+publicKey] = stats
		}
	}

	return interfaceStats, peerStats
}

// applyChanges commits staged WireGuard configuration.
func (c *OpnsenseController) applyChanges(ctx context.Context, what string) error {
	reply := c.client.Post(ctx, opnsenseReconfigureEndpoint, nil)
	if reply.Status != lowlevel.OpnsenseApiStatusOk {
		return fmt.Errorf("failed to apply WireGuard changes after %s: %v", what, reply.Error)
	}
	return nil
}

// ensureWireGuardEnabled turns on the global WireGuard switch. A tunnel that is
// configured while the service is disabled stays down with no error anywhere,
// which is a confusing failure to debug, so bringing an interface up implies
// enabling the service. This only ever enables; it never disables.
func (c *OpnsenseController) ensureWireGuardEnabled(ctx context.Context) error {
	reply := c.client.Get(ctx, "/api/wireguard/general/get")
	if reply.Status != lowlevel.OpnsenseApiStatusOk {
		return fmt.Errorf("failed to read WireGuard general settings: %v", reply.Error)
	}

	general, ok := reply.Data["general"].(map[string]any)
	if !ok {
		return fmt.Errorf("unexpected WireGuard general settings payload")
	}
	if lowlevel.GenericJsonObject(general).GetBool("enabled") {
		return nil
	}

	slog.Info("enabling the OPNsense WireGuard service", "backend", c.cfg.Id)
	setReply := c.client.Post(ctx, "/api/wireguard/general/set", lowlevel.GenericJsonObject{
		"general": lowlevel.GenericJsonObject{"enabled": "1"},
	})
	if setReply.Status != lowlevel.OpnsenseApiStatusOk {
		return fmt.Errorf("failed to enable the WireGuard service: %v", setReply.Error)
	}
	return nil
}

// findServerRow returns the searchServer row for the given interface, or nil
// when no such tunnel exists.
func (c *OpnsenseController) findServerRow(
	ctx context.Context,
	id domain.InterfaceIdentifier,
) (lowlevel.GenericJsonObject, error) {
	reply := c.client.Search(ctx, "/api/wireguard/server/searchServer"+opnsenseSearchAllParams)
	if reply.Status != lowlevel.OpnsenseApiStatusOk {
		return nil, fmt.Errorf("failed to query interfaces: %v", reply.Error)
	}

	for _, row := range reply.Data.Rows {
		if serverIdentifier(row) == id {
			return row, nil
		}
	}
	return nil, nil
}

// serverIdentifier derives the wg-portal interface identifier from a server
// row. OPNsense fills `interface` once the tunnel has been applied; before that
// only `instance` is set, so fall back to deriving the device name.
func serverIdentifier(row lowlevel.GenericJsonObject) domain.InterfaceIdentifier {
	if device := row.GetString("interface"); device != "" {
		return domain.InterfaceIdentifier(device)
	}
	return domain.InterfaceIdentifier(deviceNameForInstance(row.GetString("instance")))
}

// endregion helpers

// region wireguard-related

func (c *OpnsenseController) GetInterfaces(ctx context.Context) ([]domain.PhysicalInterface, error) {
	reply := c.client.Search(ctx, "/api/wireguard/server/searchServer"+opnsenseSearchAllParams)
	if reply.Status != lowlevel.OpnsenseApiStatusOk {
		return nil, fmt.Errorf("failed to query interfaces: %v", reply.Error)
	}

	interfaceStats, _ := c.loadStats(ctx)

	interfaces := make([]domain.PhysicalInterface, 0, len(reply.Data.Rows))
	for _, row := range reply.Data.Rows {
		physicalInterface, err := c.convertServer(row, interfaceStats)
		if err != nil {
			return nil, fmt.Errorf("interface convert failed for %s: %w", row.GetString("name"), err)
		}
		interfaces = append(interfaces, *physicalInterface)
	}

	return interfaces, nil
}

func (c *OpnsenseController) GetInterface(ctx context.Context, id domain.InterfaceIdentifier) (
	*domain.PhysicalInterface,
	error,
) {
	row, err := c.findServerRow(ctx, id)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, fmt.Errorf("interface %s not found", id)
	}

	interfaceStats, _ := c.loadStats(ctx)
	return c.convertServer(row, interfaceStats)
}

func (c *OpnsenseController) convertServer(
	row lowlevel.GenericJsonObject,
	interfaceStats map[string]opnsenseStats,
) (*domain.PhysicalInterface, error) {
	identifier := serverIdentifier(row)

	// searchServer already flattens select fields, so tunneladdress arrives as
	// a comma-separated CIDR list rather than the map getServer would return.
	//
	// A tunnel with no address is valid in OPNsense, and CidrsFromString reports
	// the empty string as an error. Enumeration must not fail because of it: a
	// single unreadable tunnel would otherwise abort GetInterfaces, which the
	// startup importer turns into a fatal error for *every* backend, not just
	// this one. Tolerate it the way pfsense.go and mikrotik.go do.
	addresses := parseCidrsTolerant(row.GetString("tunneladdress"),
		"tunnel addresses", string(identifier))

	enabled := row.GetBool("enabled")
	stats := interfaceStats[string(identifier)]

	physicalInterface := &domain.PhysicalInterface{
		Identifier: identifier,
		KeyPair: domain.KeyPair{
			PrivateKey: row.GetString("privkey"),
			PublicKey:  row.GetString("pubkey"),
		},
		ListenPort:   row.GetInt("port"),
		Addresses:    addresses,
		Mtu:          row.GetInt("mtu"),
		FirewallMark: 0, // OPNsense does not expose fwmark on the tunnel model
		DeviceUp:     enabled && stats.up,
		ImportSource: domain.ControllerTypeOpnsense,
		DeviceType:   domain.ControllerTypeOpnsense,
		// Byte counters are per-peer in `wg show`; the tunnel itself has none.
		BytesUpload:   0,
		BytesDownload: 0,
	}

	physicalInterface.SetExtras(domain.OpnsenseInterfaceExtras{
		Uuid:     row.GetString("uuid"),
		Instance: row.GetString("instance"),
		Comment:  row.GetString("name"),
		Disabled: !enabled,
	})

	return physicalInterface, nil
}

func (c *OpnsenseController) GetPeers(ctx context.Context, deviceId domain.InterfaceIdentifier) (
	[]domain.PhysicalPeer,
	error,
) {
	serverRow, err := c.findServerRow(ctx, deviceId)
	if err != nil {
		return nil, err
	}
	if serverRow == nil {
		return nil, fmt.Errorf("interface %s not found", deviceId)
	}
	serverUuid := serverRow.GetString("uuid")

	reply := c.client.Search(ctx, "/api/wireguard/client/searchClient"+opnsenseSearchAllParams)
	if reply.Status != lowlevel.OpnsenseApiStatusOk {
		return nil, fmt.Errorf("failed to query peers for %s: %v", deviceId, reply.Error)
	}

	_, peerStats := c.loadStats(ctx)

	peers := make([]domain.PhysicalPeer, 0, len(reply.Data.Rows))
	for _, row := range reply.Data.Rows {
		// A client carries the set of tunnels it is attached to; filter to the
		// requested one. Membership is a comma-joined UUID list after search
		// flattening.
		if !clientBelongsToServer(row, serverUuid) {
			continue
		}

		peer, err := c.convertClient(row, deviceId, peerStats)
		if err != nil {
			return nil, fmt.Errorf("peer convert failed for %s: %w", row.GetString("name"), err)
		}
		peers = append(peers, *peer)
	}

	return peers, nil
}

func clientBelongsToServer(row lowlevel.GenericJsonObject, serverUuid string) bool {
	if serverUuid == "" {
		return false
	}
	for _, uuid := range strings.Split(row.GetString("servers"), ",") {
		if strings.TrimSpace(uuid) == serverUuid {
			return true
		}
	}
	return false
}

func (c *OpnsenseController) convertClient(
	row lowlevel.GenericJsonObject,
	deviceId domain.InterfaceIdentifier,
	peerStats map[string]opnsenseStats,
) (*domain.PhysicalPeer, error) {
	publicKey := row.GetString("pubkey")

	allowedIPs := parseCidrsTolerant(row.GetString("tunneladdress"), "allowed addresses", publicKey)

	// serveraddress/serverport describe a remote endpoint this client dials,
	// i.e. the case where the OPNsense box is the one initiating.
	endpoint := joinEndpoint(row.GetString("serveraddress"), row.GetString("serverport"))

	enabled := row.GetBool("enabled")
	stats := peerStats[string(deviceId)+"/"+publicKey]

	peer := &domain.PhysicalPeer{
		Identifier: domain.PeerIdentifier(publicKey),
		Endpoint:   endpoint,
		AllowedIPs: allowedIPs,
		KeyPair: domain.KeyPair{
			PublicKey: publicKey,
			// OPNsense stores only the public key for a peer; the private key
			// stays with the client device.
			PrivateKey: "",
		},
		PresharedKey:        domain.PreSharedKey(row.GetString("psk")),
		PersistentKeepalive: row.GetInt("keepalive"),
		LastHandshake:       stats.lastHandshake,
		ProtocolVersion:     0,
		// Counters are named from the firewall's point of view but reported from
		// the peer's, matching local.go:203-204: what the firewall received is
		// what the peer uploaded.
		BytesUpload:   stats.bytesReceived,
		BytesDownload: stats.bytesSent,
		ImportSource:  domain.ControllerTypeOpnsense,
	}

	peer.SetExtras(domain.OpnsensePeerExtras{
		Uuid:            row.GetString("uuid"),
		Name:            row.GetString("name"),
		Comment:         row.GetString("name"),
		Disabled:        !enabled,
		ClientEndpoint:  endpoint,
		ClientAddress:   row.GetString("tunneladdress"),
		ClientDns:       "",
		ClientKeepalive: row.GetInt("keepalive"),
	})

	return peer, nil
}

func (c *OpnsenseController) SaveInterface(
	ctx context.Context,
	id domain.InterfaceIdentifier,
	updateFunc func(pi *domain.PhysicalInterface) (*domain.PhysicalInterface, error),
) error {
	mutex := c.getInterfaceMutex(id)
	mutex.Lock()
	defer mutex.Unlock()

	row, err := c.findServerRow(ctx, id)
	if err != nil {
		return err
	}

	var physicalInterface *domain.PhysicalInterface
	if row != nil {
		physicalInterface, err = c.convertServer(row, nil)
		if err != nil {
			return err
		}
	} else {
		physicalInterface = &domain.PhysicalInterface{
			Identifier:   id,
			ImportSource: domain.ControllerTypeOpnsense,
			DeviceType:   domain.ControllerTypeOpnsense,
		}
		physicalInterface.SetExtras(domain.OpnsenseInterfaceExtras{
			Instance: instanceForDeviceName(id),
		})
	}

	if updateFunc != nil {
		physicalInterface, err = updateFunc(physicalInterface)
		if err != nil {
			return err
		}
	}

	return c.createOrUpdateInterface(ctx, physicalInterface)
}

func (c *OpnsenseController) createOrUpdateInterface(ctx context.Context, pi *domain.PhysicalInterface) error {
	extras, ok := pi.GetExtras().(domain.OpnsenseInterfaceExtras)
	if !ok {
		return fmt.Errorf("interface %s is missing OPNsense extras", pi.Identifier)
	}

	// The identity of a tunnel on this backend is its OPNsense `instance`, which
	// determines the device name (instance 2 -> wg2) that wg-portal uses as the
	// interface identifier. An identifier that does not follow that convention
	// has no instance to map to, so OPNsense would allocate its own and the
	// resulting tunnel could never be found again: every subsequent save would
	// create yet another tunnel and every delete would be a silent no-op.
	// Refuse it up front rather than corrupting the firewall's configuration.
	instance := extras.Instance
	if instance == "" {
		instance = instanceForDeviceName(pi.Identifier)
	}
	if instance == "" && extras.Uuid == "" {
		return fmt.Errorf(
			"cannot create interface %q on an OPNsense backend: the identifier must be of the form wgN "+
				"(for example wg0), because OPNsense derives the device name from the tunnel instance number",
			pi.Identifier)
	}

	// OPNsense requires a non-empty name in a restricted character set; the
	// wg-portal display name is free-form and optional.
	name := opnsenseName(extras.Comment, string(pi.Identifier))

	// Every field wg-portal owns is sent on every write, including when it is
	// empty. Omitting zero values would make a cleared MTU or listen port
	// unrepresentable: the previous value would simply persist on the firewall.
	server := lowlevel.GenericJsonObject{
		"enabled":       opnsenseBool(!extras.Disabled),
		"name":          name,
		"pubkey":        pi.KeyPair.PublicKey,
		"privkey":       pi.KeyPair.PrivateKey,
		"tunneladdress": domain.CidrsToString(pi.Addresses),
		"port":          optionalPositiveInt(pi.ListenPort),
		"mtu":           optionalPositiveInt(pi.Mtu),
	}
	if instance != "" {
		server["instance"] = instance
	}

	if extras.Uuid == "" {
		slog.Debug("creating new OPNsense tunnel",
			"interface", pi.Identifier, "addresses", domain.CidrsToString(pi.Addresses))

		reply := c.client.Post(ctx, "/api/wireguard/server/addServer",
			lowlevel.GenericJsonObject{"server": server})
		if reply.Status != lowlevel.OpnsenseApiStatusOk {
			return fmt.Errorf("failed to create interface %s: %v", pi.Identifier, reply.Error)
		}
		if newUuid := reply.Data.GetString("uuid"); newUuid != "" {
			extras.Uuid = newUuid
			pi.SetExtras(extras)
		}
	} else {
		slog.Debug("updating OPNsense tunnel", "interface", pi.Identifier, "uuid", extras.Uuid)

		reply := c.client.Post(ctx, "/api/wireguard/server/setServer/"+url.PathEscape(extras.Uuid),
			lowlevel.GenericJsonObject{"server": server})
		if reply.Status != lowlevel.OpnsenseApiStatusOk {
			return fmt.Errorf("failed to update interface %s: %v", pi.Identifier, reply.Error)
		}
	}

	if pi.DeviceUp || !extras.Disabled {
		if err := c.ensureWireGuardEnabled(ctx); err != nil {
			return err
		}
	}

	return c.applyChanges(ctx, fmt.Sprintf("saving interface %s", pi.Identifier))
}

func (c *OpnsenseController) DeleteInterface(ctx context.Context, id domain.InterfaceIdentifier) error {
	mutex := c.getInterfaceMutex(id)
	mutex.Lock()
	defer mutex.Unlock()

	row, err := c.findServerRow(ctx, id)
	if err != nil {
		return err
	}
	if row == nil {
		return nil // tunnel does not exist, nothing to delete
	}

	uuid := row.GetString("uuid")
	reply := c.client.Post(ctx, "/api/wireguard/server/delServer/"+url.PathEscape(uuid), nil)
	if reply.Status != lowlevel.OpnsenseApiStatusOk {
		return fmt.Errorf("failed to delete interface %s: %v", id, reply.Error)
	}

	return c.applyChanges(ctx, fmt.Sprintf("deleting interface %s", id))
}

func (c *OpnsenseController) SavePeer(
	ctx context.Context,
	deviceId domain.InterfaceIdentifier,
	id domain.PeerIdentifier,
	updateFunc func(pp *domain.PhysicalPeer) (*domain.PhysicalPeer, error),
) error {
	mutex := c.getPeerMutex(id)
	mutex.Lock()
	defer mutex.Unlock()

	serverRow, err := c.findServerRow(ctx, deviceId)
	if err != nil {
		return err
	}
	if serverRow == nil {
		return fmt.Errorf("interface %s not found", deviceId)
	}
	serverUuid := serverRow.GetString("uuid")

	clientRow, err := c.findClientRow(ctx, id)
	if err != nil {
		return err
	}

	var physicalPeer *domain.PhysicalPeer
	if clientRow != nil {
		physicalPeer, err = c.convertClient(clientRow, deviceId, nil)
		if err != nil {
			return err
		}
	} else {
		physicalPeer = &domain.PhysicalPeer{
			Identifier:   id,
			KeyPair:      domain.KeyPair{PublicKey: string(id)},
			ImportSource: domain.ControllerTypeOpnsense,
		}
		physicalPeer.SetExtras(domain.OpnsensePeerExtras{})
	}

	if updateFunc != nil {
		physicalPeer, err = updateFunc(physicalPeer)
		if err != nil {
			return err
		}
	}

	return c.createOrUpdatePeer(ctx, deviceId, serverUuid, clientRow, physicalPeer)
}

// findClientRow locates a peer by public key. OPNsense has no filtered lookup
// on this controller, so the full list is fetched and matched locally.
func (c *OpnsenseController) findClientRow(
	ctx context.Context,
	id domain.PeerIdentifier,
) (lowlevel.GenericJsonObject, error) {
	reply := c.client.Search(ctx, "/api/wireguard/client/searchClient"+opnsenseSearchAllParams)
	if reply.Status != lowlevel.OpnsenseApiStatusOk {
		return nil, fmt.Errorf("failed to query peers: %v", reply.Error)
	}

	for _, row := range reply.Data.Rows {
		if row.GetString("pubkey") == string(id) {
			return row, nil
		}
	}
	return nil, nil
}

func (c *OpnsenseController) createOrUpdatePeer(
	ctx context.Context,
	deviceId domain.InterfaceIdentifier,
	serverUuid string,
	existingRow lowlevel.GenericJsonObject,
	pp *domain.PhysicalPeer,
) error {
	extras, ok := pp.GetExtras().(domain.OpnsensePeerExtras)
	if !ok {
		return fmt.Errorf("peer %s is missing OPNsense extras", pp.Identifier)
	}

	name := opnsenseName(extras.Name, string(pp.Identifier))

	// Attaching a peer to a tunnel is done from the client side: setting
	// `servers` here is what populates the tunnel's `peers` list. Preserve any
	// other tunnels this peer is already attached to rather than detaching it
	// from them.
	servers := map[string]struct{}{serverUuid: {}}
	if existingRow != nil {
		for _, uuid := range strings.Split(existingRow.GetString("servers"), ",") {
			if uuid = strings.TrimSpace(uuid); uuid != "" {
				servers[uuid] = struct{}{}
			}
		}
	}
	serverList := make([]string, 0, len(servers))
	for uuid := range servers {
		serverList = append(serverList, uuid)
	}
	// Sort for the same reason FlattenForWrite does: Go map iteration order is
	// randomised, so an unsorted join would send a different `servers` value on
	// every save and churn the firewall's configuration with no real change.
	sort.Strings(serverList)

	// As with the tunnel, send every field wg-portal owns on every write so that
	// clearing one actually clears it on the firewall.
	address, port := splitEndpoint(pp.Endpoint)
	client := lowlevel.GenericJsonObject{
		"enabled":       opnsenseBool(!extras.Disabled),
		"name":          name,
		"pubkey":        pp.KeyPair.PublicKey,
		"psk":           string(pp.PresharedKey),
		"tunneladdress": domain.CidrsToString(pp.AllowedIPs),
		"servers":       strings.Join(serverList, ","),
		"keepalive":     optionalPositiveInt(pp.PersistentKeepalive),
		"serveraddress": address,
		"serverport":    port,
	}

	if extras.Uuid == "" {
		slog.Debug("creating new OPNsense peer",
			"peer", pp.Identifier, "interface", deviceId,
			"allowed-ips", domain.CidrsToString(pp.AllowedIPs))

		reply := c.client.Post(ctx, "/api/wireguard/client/addClient",
			lowlevel.GenericJsonObject{"client": client})
		if reply.Status != lowlevel.OpnsenseApiStatusOk {
			return fmt.Errorf("failed to create peer %s for interface %s: %v",
				pp.Identifier, deviceId, reply.Error)
		}
		if newUuid := reply.Data.GetString("uuid"); newUuid != "" {
			extras.Uuid = newUuid
			pp.SetExtras(extras)
		}
	} else {
		slog.Debug("updating OPNsense peer",
			"peer", pp.Identifier, "interface", deviceId, "uuid", extras.Uuid,
			"disabled", extras.Disabled)

		reply := c.client.Post(ctx, "/api/wireguard/client/setClient/"+url.PathEscape(extras.Uuid),
			lowlevel.GenericJsonObject{"client": client})
		if reply.Status != lowlevel.OpnsenseApiStatusOk {
			return fmt.Errorf("failed to update peer %s on interface %s: %v",
				pp.Identifier, deviceId, reply.Error)
		}
	}

	return c.applyChanges(ctx, fmt.Sprintf("saving peer %s", pp.Identifier))
}

// joinEndpoint combines a host and port, bracketing IPv6 literals.
//
// Formatting this as "host:port" is wrong for IPv6: "2001:db8::1" and "51820"
// would become "2001:db8::1:51820", which still parses as an IPv6 address with
// the port silently absorbed into it. splitEndpoint would then hand back the
// whole string as the host with no port, so the value is corrupted on every
// read/write round-trip.
func joinEndpoint(host, port string) string {
	host = strings.TrimSpace(host)
	port = strings.TrimSpace(port)

	switch {
	case host == "":
		return ""
	case port == "":
		return host
	default:
		return net.JoinHostPort(host, port)
	}
}

// splitEndpoint is the inverse of joinEndpoint, tolerating a bare host with no
// port and an unbracketed IPv6 literal.
func splitEndpoint(endpoint string) (string, string) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return "", ""
	}

	if host, port, err := net.SplitHostPort(endpoint); err == nil {
		return host, port
	}

	// No port present, or an unbracketed IPv6 literal. Either way there is no
	// port to recover; strip any brackets so the host round-trips cleanly.
	return strings.Trim(endpoint, "[]"), ""
}

func (c *OpnsenseController) DeletePeer(
	ctx context.Context,
	deviceId domain.InterfaceIdentifier,
	id domain.PeerIdentifier,
) error {
	mutex := c.getPeerMutex(id)
	mutex.Lock()
	defer mutex.Unlock()

	row, err := c.findClientRow(ctx, id)
	if err != nil {
		return err
	}
	if row == nil {
		return nil // peer does not exist, nothing to delete
	}

	// An OPNsense client is a single record that may be attached to several
	// tunnels at once, and createOrUpdatePeer deliberately preserves those
	// attachments. Deleting the record outright would therefore detach the peer
	// from every other tunnel as a side effect of removing it from this one.
	// Detach from this tunnel instead, and only delete once nothing references
	// it.
	serverRow, err := c.findServerRow(ctx, deviceId)
	if err != nil {
		return err
	}

	uuid := row.GetString("uuid")
	remaining := make([]string, 0)
	if serverRow != nil {
		serverUuid := serverRow.GetString("uuid")
		for _, attached := range strings.Split(row.GetString("servers"), ",") {
			if attached = strings.TrimSpace(attached); attached != "" && attached != serverUuid {
				remaining = append(remaining, attached)
			}
		}
	}

	if len(remaining) > 0 {
		sort.Strings(remaining)
		slog.Debug("detaching OPNsense peer from one tunnel, it remains on others",
			"peer", id, "interface", deviceId, "remaining", len(remaining))

		client := lowlevel.FlattenForWrite(row)
		client["servers"] = strings.Join(remaining, ",")
		delete(client, "uuid") // identity travels in the path, not the body
		// search rows carry "%"-prefixed display renderings (e.g. "%servers":
		// "tier-staff") alongside the real fields; they are not writable.
		for key := range client {
			if strings.HasPrefix(key, "%") {
				delete(client, key)
			}
		}

		reply := c.client.Post(ctx, "/api/wireguard/client/setClient/"+url.PathEscape(uuid),
			lowlevel.GenericJsonObject{"client": client})
		if reply.Status != lowlevel.OpnsenseApiStatusOk {
			return fmt.Errorf("failed to detach peer %s from interface %s: %v", id, deviceId, reply.Error)
		}
	} else {
		reply := c.client.Post(ctx, "/api/wireguard/client/delClient/"+url.PathEscape(uuid), nil)
		if reply.Status != lowlevel.OpnsenseApiStatusOk {
			return fmt.Errorf("failed to delete peer %s for interface %s: %v", id, deviceId, reply.Error)
		}
	}

	return c.applyChanges(ctx, fmt.Sprintf("deleting peer %s", id))
}

// endregion wireguard-related

// region wg-quick-related

func (c *OpnsenseController) ExecuteInterfaceHook(
	_ context.Context,
	id domain.InterfaceIdentifier,
	_ string,
) error {
	// Hooks would have to run as shell commands on the firewall; the WireGuard
	// API offers no equivalent and running arbitrary commands is out of scope
	// for this backend.
	slog.Error("interface hooks are not supported for OPNsense backends, please open an issue on GitHub",
		"interface", id)
	return nil
}

func (c *OpnsenseController) SetDNS(
	_ context.Context,
	id domain.InterfaceIdentifier,
	_, _ string,
) error {
	c.coreMutex.Lock()
	defer c.coreMutex.Unlock()

	// DNS pushed to clients is a property of the tunnel (`peer_dns`) rather
	// than something applied to the firewall's own resolver, so this hook has
	// no sensible OPNsense equivalent.
	slog.Warn("DNS setting is not supported for OPNsense backends", "interface", id)
	return nil
}

func (c *OpnsenseController) UnsetDNS(
	_ context.Context,
	id domain.InterfaceIdentifier,
	_, _ string,
) error {
	c.coreMutex.Lock()
	defer c.coreMutex.Unlock()

	slog.Warn("DNS unsetting is not supported for OPNsense backends", "interface", id)
	return nil
}

// endregion wg-quick-related

// region routing-related

func (c *OpnsenseController) SetRoutes(_ context.Context, info domain.RoutingTableInfo) error {
	// OPNsense derives routes from the tunnel's allowed addresses unless
	// `disableroutes` is set; wg-portal does not need to manage them directly.
	slog.Debug("route setting is handled by OPNsense itself", "interface", info.Interface.Identifier)
	return nil
}

func (c *OpnsenseController) RemoveRoutes(_ context.Context, info domain.RoutingTableInfo) error {
	slog.Debug("route removal is handled by OPNsense itself", "interface", info.Interface.Identifier)
	return nil
}

// endregion routing-related

// region statistics-related

func (c *OpnsenseController) PingAddresses(
	_ context.Context,
	_ string,
) (*domain.PingerResult, error) {
	return nil, fmt.Errorf("ping functionality is not implemented for OPNsense backends")
}

// endregion statistics-related
