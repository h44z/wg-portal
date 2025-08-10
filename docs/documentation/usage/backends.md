# Backends

WireGuard Portal can manage WireGuard interfaces and peers on different backends. 
Each backend represents a system where interfaces actually live. 
You can register multiple backends and choose which one to use per interface. 
A global default backend determines where newly created interfaces go (unless you explicitly choose another in the UI).

**Supported backends:**
- **Local** (default): Manages interfaces on the host running WireGuard Portal (Linux WireGuard via wgctrl). Use this when the portal should directly configure wg devices on the same server.
- **MikroTik** RouterOS (_beta_): Manages interfaces and peers on MikroTik devices via the RouterOS REST API. Use this to control WG interfaces on RouterOS v7+.

How backend selection works:
- The default backend is configured at `backend.default` (_local_ or the id of a defined MikroTik backend). 
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