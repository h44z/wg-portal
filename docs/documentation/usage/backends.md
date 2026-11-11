# Backends

WireGuard Portal can manage WireGuard interfaces and peers on different backends. 
Each backend represents a system where interfaces actually live. 
You can register multiple backends and choose which one to use per interface. 
A global default backend determines where newly created interfaces go (unless you explicitly choose another in the UI).

**Supported backends:**
- **Local** (default): Manages interfaces on the host running WireGuard Portal (Linux WireGuard via wgctrl). Use this when the portal should directly configure wg devices on the same server.
- **MikroTik** RouterOS (_beta_): Manages interfaces and peers on MikroTik devices via the RouterOS REST API. Use this to control WG interfaces on RouterOS v7+.
- **pfSense** (_alpha_): Manages interfaces and peers on pfSense firewalls via the pfSense REST API.
- **OPNsense** (_alpha_): Manages interfaces and peers on OPNsense firewalls via the WireGuard API built into OPNsense core. Unlike the pfSense backend, no add-on package is required on the firewall.

How backend selection works:
- The default backend is configured at `backend.default` (_local_ or the id of a defined MikroTik, pfSense or OPNsense backend). 
  New interfaces created in the UI will use this backend by default.
- Each interface stores its backend. You can select a different backend when creating a new interface.

## Configuring MikroTik backends (RouterOS v7+)

> :warning: The MikroTik backend is currently marked beta. While basic functionality is implemented, some advanced features are not yet implemented or contain bugs. Please test carefully before using in production.

The MikroTik backend uses the [REST API](https://help.mikrotik.com/docs/spaces/ROS/pages/47579162/REST+API) under a base URL ending with /rest. 
You can register one or more MikroTik devices as backends for a single WireGuard Portal instance.

### Prerequisites on MikroTik:
- RouterOS v7 with WireGuard support.
- REST API enabled and reachable over HTTP(S). A typical base URL is https://<router-address>:8729/rest or https://<router-address>/rest depending on your service setup.
- A dedicated RouterOS user with the following group permissions:
  - **api** (for logging in via REST API)
  - **rest-api** (for logging in via REST API)
  - **read** (to read interface and peer data)
  - **write** (to create/update interfaces and peers)
  - **test** (to perform ping checks)
  - **sensitive** (to read private keys)
- TLS certificate on the device is recommended. If you use a self-signed certificate during testing, set `api_verify_tls`: _false_ in wg-portal (not recommended for production).

Example WireGuard Portal configuration (config/config.yaml):

```yaml
backend:
  # default backend decides where new interfaces are created
  default: mikrotik-prod

  mikrotik:
    - id: mikrotik-prod              # unique id, not "local"
      display_name: RouterOS RB5009  # optional nice name
      api_url: https://10.10.10.10/rest
      api_user: wgportal
      api_password: a-super-secret-password
      api_verify_tls: true         # set to false only if using self-signed during testing
      api_timeout: 30s             # maximum request duration
      concurrency: 5               # limit parallel REST calls to device
      debug: false                 # verbose logging for this backend
```

### Known limitations:
- The MikroTik backend is still in beta. Some features may not work as expected.
- Not all WireGuard Portal features are supported yet (e.g., no support for interface hooks)

## Configuring pfSense backends

> :warning: The pfSense backend is currently **alpha**. Only basic interface and peer CRUD are supported. Traffic statistics (rx/tx, last handshake) are not exposed by the pfSense REST API and will show as empty.

The pfSense backend talks to the pfSense REST API (pfSense Plus / CE with the REST API package installed). Point the backend at the appliance hostname without appending `/api/v2` — the portal appends `/api/v2` automatically. wg-portal is developed for and tested against REST API v2.8.0.

### Prerequisites on pfSense:
- pfSense with the REST API package enabled (`System -> API`) and WireGuard configured.
- An API key with permissions for WireGuard endpoints. If you use a read-only key, set `core.restore_state: false` in `config.yml` to avoid write attempts at startup.
- HTTPS recommended; set `api_verify_tls: false` only for lab/self-signed setups.

Example WireGuard Portal configuration:

```yaml
backend:
  # default backend decides where new interfaces are created
  default: pfsense1

  pfsense:
    - id: pfsense1                 # unique id, not "local"
      display_name: Main pfSense   # optional nice name
      api_url: https://pfsense.example.com  # no trailing /api/v2
      api_key: your-api-key
      api_verify_tls: true
      api_timeout: 30s
      concurrency: 5
      debug: false
```

### Known limitations:
- Alpha quality: behavior and API coverage may change.
- Statistics (rx/tx bytes, last handshake) are not available from the pfSense REST API today.

## Configuring OPNsense backends

> :warning: The OPNsense backend is currently **alpha**. Interface and peer CRUD are supported, as are **per-peer** traffic statistics (rx/tx bytes and last handshake), which the pfSense backend cannot provide. Interface-level byte counters are not reported, because OPNsense exposes counters per peer only. Interface hooks, DNS push and ping are not supported.

The OPNsense backend talks to the WireGuard API that is part of **OPNsense core**. Unlike the pfSense backend, no add-on package needs to be installed — a stock OPNsense install already answers `/api/wireguard/*`.

Point `api_url` at the appliance root (for example `https://opnsense.example.com`); the portal appends the API paths itself.

### Prerequisites on OPNsense:
- OPNsense 24.1 or newer, where WireGuard is part of the base system. Developed and tested against 26.7.
- An API key/secret pair, created under `System -> Access -> Users -> <user> -> API keys`. OPNsense shows the secret only once, at creation time.
- The user needs permission for the WireGuard and firewall pages it should manage.
- HTTPS recommended; set `api_verify_tls: false` only for lab/self-signed setups.

Example WireGuard Portal configuration:

```yaml
backend:
  # default backend decides where new interfaces are created
  default: opnsense1

  opnsense:
    - id: opnsense1                  # unique id, not "local"
      display_name: Edge firewall    # optional nice name
      api_url: https://opnsense.example.com   # appliance root, no /api suffix
      api_key: your-api-key
      api_secret: your-api-secret
      api_verify_tls: true
      api_timeout: 30s
      debug: false
```

### Behaviour worth knowing:
- **Interface naming.** An OPNsense tunnel has both an instance number and a free-text name. WireGuard Portal uses the resulting device name (`wg0`, `wg1`, ...) as the interface identifier, so an interface imported from OPNsense looks the same as one managed by the `local` backend. The free-text name becomes the display name.
- **Changes are staged.** OPNsense applies WireGuard configuration only when the service is reconfigured, which the backend does after every change. Reconfiguring does **not** drop or rekey sessions of peers that are already connected, so adding a peer is safe against a live VPN.
- **The service is enabled automatically.** Bringing an interface up switches on the global WireGuard service if it is off, because a tunnel configured while the service is disabled stays down without reporting an error anywhere. The backend never disables the service.
- **Firewall rules are not managed.** A newly created tunnel has no pass rule, so peers will complete a handshake but carry no traffic until you add a rule on the `WireGuard (Group)` interface. This matches the pfSense backend, which also leaves firewall rules to the administrator.

### Known limitations:
- Alpha quality: behavior and API coverage may change.
- Interface hooks (`PreUp`/`PostUp`/...) are not supported; OPNsense has no API equivalent.
- DNS settings pushed to clients are a property of the tunnel in OPNsense and are not managed by the portal.
- `PingAddresses` is not implemented.
