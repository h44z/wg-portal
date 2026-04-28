This page provides an overview of **all available configuration options** for WireGuard Portal.

You can supply these configurations in a **YAML** file when starting the Portal.
The path of the configuration file defaults to `config/config.yaml` (or `config/config.yml`) in the working directory of the executable.  
It is possible to override the configuration filepath using the environment variable `WG_PORTAL_CONFIG`.
For example: `WG_PORTAL_CONFIG=/etc/wg-portal/config.yaml ./wg-portal`.  
Also, environment variable substitution in the config file is supported. Refer to the [syntax](https://github.com/a8m/envsubst?tab=readme-ov-file#docs).

Configuration examples are available on the [Examples](./examples.md) page.

<details>
<summary>Default configuration</summary>

```yaml
core:
  admin_user: admin@wgportal.local
  admin_password: wgportal-default
  admin_api_token: ""
  disable_admin_user: false
  editable_keys: true
  create_default_peer: false
  create_default_peer_on_creation: false
  re_enable_peer_after_user_enable: true
  delete_peer_after_user_deleted: false
  self_provisioning_allowed: false
  import_existing: true
  restore_state: true
  peer:
    rotation_interval: 0
    expiry_action: disable
    expiry_notification_enabled: true
    expiry_notification_intervals:
      - 168h
      - 72h
      - 24h
    notification_cleanup_after: 720h
  
backend:
  default: local
  rekey_timeout_interval: 125s
  local_resolvconf_prefix: tun.

advanced:
  log_level: info
  log_pretty: false
  log_json: false
  start_listen_port: 51820
  start_cidr_v4: 10.11.12.0/24
  start_cidr_v6: fdfd:d3ad:c0de:1234::0/64
  use_ip_v6: true
  config_storage_path: ""
  expiry_check_interval: 15m
  rule_prio_offset: 20000
  route_table_offset: 20000
  api_admin_only: true
  limit_additional_user_peers: 0

database:
  debug: false
  slow_query_threshold: "0"
  type: sqlite
  dsn: data/sqlite.db
  encryption_passphrase: ""

statistics:
  use_ping_checks: true
  ping_check_workers: 10
  ping_unprivileged: false
  ping_check_interval: 1m
  data_collection_interval: 1m
  collect_interface_data: true
  collect_peer_data: true
  collect_audit_data: true
  listening_address: :8787

mail:
  host: 127.0.0.1
  port: 25
  encryption: none
  cert_validation: true
  username: ""
  password: ""
  auth_type: plain
  from: Wireguard Portal <noreply@wireguard.local>
  link_only: false
  allow_peer_email: false
  templates_path: ""

auth:
  oidc: []
  oauth: []
  ldap: []
  webauthn:
    enabled: true
  min_password_length: 16
  hide_login_form: false

web:
  listening_address: :8888
  external_url: http://localhost:8888
  base_path: ""
  site_company_name: WireGuard Portal
  site_title: WireGuard Portal
  session_identifier: wgPortalSession
  session_secret: very_secret
  csrf_secret: extremely_secret
  request_logging: false
  expose_host_info: false
  cert_file: ""
  key_File: ""
  frontend_filepath: ""

webhook:
  url: ""
  authentication: ""
  timeout: 10s
```

</details>


Below you will find sections like
[`core`](#core),
[`backend`](#backend),
[`advanced`](#advanced),
[`database`](#database),
[`statistics`](#statistics),
[`mail`](#mail),
[`auth`](#auth),
[`web`](#web) and
[`webhook`](#webhook).  
Each section describes the individual configuration keys, their default values, and a brief explanation of their purpose.

---

## Core

These are the primary configuration options that control fundamental WireGuard Portal behavior.
More advanced options are found in the subsequent `Advanced` section.

### `admin_user`
- **Default:** `admin@wgportal.local`
- **Environment Variable:** `WG_PORTAL_CORE_ADMIN_USER`
- **Description:** The administrator user. This user will be created as a default admin if it does not yet exist.

### `admin_password`
- **Default:** `wgportal-default`
- **Environment Variable:** `WG_PORTAL_CORE_ADMIN_PASSWORD`
- **Description:** The administrator password. The default password should be changed immediately!
- **Important:** The password should be strong and secure. The minimum password length is specified in [auth.min_password_length](#min_password_length). By default, it is 16 characters.

### `disable_admin_user`
- **Default:** `false`
- **Environment Variable:** `WG_PORTAL_CORE_DISABLE_ADMIN_USER`
- **Description:** If `true`, no admin user is created. This is useful if you plan to manage users exclusively through external authentication providers such as LDAP or OAuth.

### `admin_api_token`
- **Default:** *(empty)*
- **Environment Variable:** `WG_PORTAL_CORE_ADMIN_API_TOKEN`
- **Description:** An API token for the admin user. If a token is provided, the REST API can be accessed using this token. If empty, the API is initially disabled for the admin user.

### `editable_keys`
- **Default:** `true`
- **Environment Variable:** `WG_PORTAL_CORE_EDITABLE_KEYS`
- **Description:** Allow editing of WireGuard key-pairs directly in the UI.

### `create_default_peer` (deprecated)
- **Default:** `false`
- **Environment Variable:** `WG_PORTAL_CORE_CREATE_DEFAULT_PEER`
- **Description:** **DEPRECATED** in favor of [create_default_peer_on_login](#create_default_peer_on_login). If set to `true`, this option is equivalent to enabling `create_default_peer_on_login`. It will be removed in a future release (2.4).

### `create_default_peer_on_creation` (deprecated)
- **Default:** `false`
- **Environment Variable:** `WG_PORTAL_CORE_CREATE_DEFAULT_PEER_ON_CREATION`
- **Description:** **DEPRECATED** in favor of [create_default_peer_on_user_creation](#create_default_peer_on_user_creation) and [create_default_peer_on_interface_creation](#create_default_peer_on_interface_creation). If set to `true`, both of those options are enabled. It will be removed in a future release (2.4).

### `create_default_peer_on_login`
- **Default:** `false`
- **Environment Variable:** `WG_PORTAL_CORE_CREATE_DEFAULT_PEER`
- **Description:** If a user logs in for the first time with no existing peers, automatically create a new WireGuard peer for all server interfaces where the "Create default peer" flag is set.
- **Important:** This option is only effective for interfaces where the "Create default peer" flag is set (via the UI).

### `create_default_peer_on_user_creation`
- **Default:** `false`
- **Environment Variable:** `WG_PORTAL_CORE_CREATE_DEFAULT_PEER_ON_USER_CREATION`
- **Description:** If a new user is created (e.g., through LDAP sync or registration) and has no peers, automatically create a new WireGuard peer for all server interfaces where the "Create default peer" flag is set.
- **Important:** This option is only effective for interfaces where the "Create default peer" flag is set (via the UI).

### `create_default_peer_on_interface_creation`
- **Default:** `false`
- **Environment Variable:** `WG_PORTAL_CORE_CREATE_DEFAULT_PEER_ON_INTERFACE_CREATION`
- **Description:** When a new server interface is created with the "Create default peer" flag set, automatically create a default WireGuard peer on that interface for every existing user who does not yet have a peer on it.
- **Important:** This option is only effective for interfaces where the "Create default peer" flag is set (via the UI).

### `re_enable_peer_after_user_enable`
- **Default:** `true`
- **Environment Variable:** `WG_PORTAL_CORE_RE_ENABLE_PEER_AFTER_USER_ENABLE`
- **Description:** Re-enable all peers that were previously disabled if the associated user is re-enabled.

### `delete_peer_after_user_deleted`
- **Default:** `false`
- **Environment Variable:** `WG_PORTAL_CORE_DELETE_PEER_AFTER_USER_DELETED`
- **Description:** If a user is deleted, remove all linked peers. Otherwise, peers remain but are disabled.

### `self_provisioning_allowed`
- **Default:** `false`
- **Environment Variable:** `WG_PORTAL_CORE_SELF_PROVISIONING_ALLOWED`
- **Description:** Allow registered (non-admin) users to self-provision peers from their profile page.

### `import_existing`
- **Default:** `true`
- **Environment Variable:** `WG_PORTAL_CORE_IMPORT_EXISTING`
- **Description:** On startup, import existing WireGuard interfaces and peers into WireGuard Portal.

### `restore_state`
- **Default:** `true`
- **Environment Variable:** `WG_PORTAL_CORE_RESTORE_STATE`
- **Description:** Restore the WireGuard interface states (up/down) that existed before WireGuard Portal started.

### Peer

The `peer` sub-section groups peer lifecycle and expiry notification settings.

#### `rotation_interval`
- **Default:** `0` (disabled)
- **Environment Variable:** `WG_PORTAL_CORE_PEER_ROTATION_INTERVAL`
- **Description:** The maximum lifetime of a peer. When set to a non-zero duration, every newly created peer automatically receives an expiry date equal to its creation time plus this interval. When the peer expires, the action defined by `expiry_action` is applied. A value of `0` disables automatic expiry assignment entirely.
  Format uses `s`, `m`, `h` for seconds, minutes, hours, see [time.ParseDuration](https://golang.org/pkg/time/#ParseDuration).
  **Example:** `rotation_interval: 8760h` sets a one-year maximum peer lifetime.

#### `expiry_action`
- **Default:** `disable`
- **Environment Variable:** `WG_PORTAL_CORE_PEER_EXPIRY_ACTION`
- **Description:** The action taken when a peer reaches its expiry date. Valid values are:
  - `disable` — the peer is disabled and its `DisabledReason` is set to a human-readable string that includes the expiry timestamp (e.g. `expired on 2026-04-06T15:04:05Z`).
  - `delete` — the peer record is permanently removed from storage.

#### `auto_recreate_on_expiry`
- **Default:** `false`
- **Environment Variable:** `WG_PORTAL_CORE_PEER_AUTO_RECREATE_ON_EXPIRY`
- **Description:** If `true`, a fresh replacement peer is automatically created for the same user and interface after the expired peer is disabled or deleted (according to `expiry_action`). The new peer receives fresh keys and a new expiry date based on `rotation_interval`. The peer must be linked to a user for auto-recreation to take effect. When expiry notifications are enabled, the warning emails will include a notice that a new peer will be generated and the user will need to download the new configuration from the portal.

#### `recreate_on_expiry_suffix`
- **Default:** `" (recreated)"`
- **Environment Variable:** `WG_PORTAL_CORE_PEER_RECREATE_ON_EXPIRY_SUFFIX`
- **Description:** A string appended to the display name of auto-recreated peers to distinguish them from the original. The suffix is only added once — if the display name already ends with this suffix, it is not duplicated. Set to an empty string to disable the suffix.
  **Suggested variants:** `" (recreated)"`, `" (renewed)"`, `" (rotated)"`, `" (auto)"`, `" -R"`

#### `purge_expired_after`
- **Default:** `720h` (30 days)
- **Environment Variable:** `WG_PORTAL_CORE_PEER_PURGE_EXPIRED_AFTER`
- **Description:** When `expiry_action` is `disable`, disabled expired peers accumulate in the database. This setting automatically deletes them after the specified duration has passed since their expiry date. A value of `0` disables purging entirely.
  Format uses `s`, `m`, `h` for seconds, minutes, hours, see [time.ParseDuration](https://golang.org/pkg/time/#ParseDuration).

#### `expiry_notification_enabled`
- **Default:** `true`
- **Environment Variable:** `WG_PORTAL_CORE_PEER_EXPIRY_NOTIFICATION_ENABLED`
- **Description:** If `true`, the notification manager sends expiry warning emails to users before their peer expires, according to the intervals defined in `expiry_notification_intervals`. Set to `false` to disable all expiry warning emails.

#### `expiry_notification_intervals`
- **Default:** `[168h, 72h, 24h]` (7 days, 3 days, 1 day)
- **Environment Variable:** `WG_PORTAL_CORE_PEER_EXPIRY_NOTIFICATION_INTERVALS` (comma-separated, e.g. `168h,72h,24h`)
- **Description:** An ordered list of durations that define how far before expiry a warning email is sent. For each interval, at most one email is sent per peer. If the list is empty, no warning emails are sent regardless of the `expiry_notification_enabled` flag.
  Format uses `s`, `m`, `h` for seconds, minutes, hours, see [time.ParseDuration](https://golang.org/pkg/time/#ParseDuration).

#### `notification_cleanup_after`
- **Default:** `720h` (30 days)
- **Environment Variable:** `WG_PORTAL_CORE_PEER_NOTIFICATION_CLEANUP_AFTER`
- **Description:** How long sent-notification records are kept in storage. Records older than this duration are pruned at the end of each notification check cycle. This prevents unbounded growth of the notification history table.
  Format uses `s`, `m`, `h` for seconds, minutes, hours, see [time.ParseDuration](https://golang.org/pkg/time/#ParseDuration).

---

## Backend

Configuration options for the WireGuard backend, which manages the WireGuard interfaces and peers.
The current MikroTik backend is in **BETA** and may not support all features.

### `default`
- **Default:** `local`
- **Description:** The default backend to use for managing WireGuard interfaces. 
  Valid options are: `local`, or other backend id's configured in the `mikrotik` section.

### `rekey_timeout_interval`
- **Default:** `180s`
- **Environment Variable:** `WG_PORTAL_BACKEND_REKEY_TIMEOUT_INTERVAL`
- **Description:** The interval after which a WireGuard peer is considered disconnected if no handshake updates are received. 
  This corresponds to the WireGuard rekey timeout setting of 120 seconds plus a 60-second buffer to account for latency or retry handling.
  Uses Go duration format (e.g., `10s`, `1m`). If omitted, a default of 180 seconds is used.

### `local_resolvconf_prefix`
- **Default:** `tun.`
- **Environment Variable:** `WG_PORTAL_BACKEND_LOCAL_RESOLVCONF_PREFIX`
- **Description:** Interface name prefix for WireGuard interfaces on the local system which is used to configure DNS servers with *resolvconf*. 
  It depends on the *resolvconf* implementation you are using, most use a prefix of `tun.`, but some have an empty prefix (e.g., systemd).

### `ignored_local_interfaces`
- **Default:** *(empty)*
- **Environment Variable:** `WG_PORTAL_BACKEND_IGNORED_LOCAL_INTERFACES`
  (comma-separated values)
- **Description:** A list of interface names to exclude when enumerating local interfaces.
  This is useful if you want to prevent certain interfaces from being imported from the local system.

### Mikrotik

The `mikrotik` array contains a list of MikroTik backend definitions. Each entry describes how to connect to a MikroTik RouterOS instance that hosts WireGuard interfaces.

Below are the properties for each entry inside `backend.mikrotik`:

#### `id`
- **Default:** *(empty)*
- **Description:** A unique identifier for this backend. 
  This value can be referenced by `backend.default` to use this backend as default.
  The identifier must be unique across all backends and must not use the reserved keyword `local`.

#### `display_name`
- **Default:** *(empty)*
- **Description:** A human-friendly display name for this backend. If omitted, the `id` will be used as the display name.

#### `api_url`
- **Default:** *(empty)*
- **Description:** Base URL of the MikroTik REST API, including scheme and path, e.g., `https://10.10.10.10:8729/rest`.

#### `api_user`
- **Default:** *(empty)*
- **Description:** Username for authenticating against the MikroTik API.
  Ensure that the user has sufficient permissions to manage WireGuard interfaces and peers.

#### `api_password`
- **Default:** *(empty)*
- **Description:** Password for the specified API user.

#### `api_verify_tls`
- **Default:** `false`
- **Description:** Whether to verify the TLS certificate of the MikroTik API endpoint. Set to `false` to allow self-signed certificates (not recommended for production).

#### `api_timeout`
- **Default:** `30s`
- **Description:** Timeout for API requests to the MikroTik device. Uses Go duration format (e.g., `10s`, `1m`). If omitted, a default of 30 seconds is used.

#### `concurrency`
- **Default:** `5`
- **Description:** Maximum number of concurrent API requests the backend will issue when enumerating interfaces and their details. If `0` or negative, a sane default of `5` is used.

#### `ignored_interfaces`
- **Default:** *(empty)*
- **Description:** A list of interface names to exclude during interface enumeration.
  This is useful if you want to prevent specific interfaces from being imported from the MikroTik device.

#### `debug`
- **Default:** `false`
- **Description:** Enable verbose debug logging for the MikroTik backend.

For more details on configuring the MikroTik backend, see the [Backends](../usage/backends.md) documentation.

---

## Advanced

Additional or more specialized configuration options for logging and interface creation details.

### `log_level`
- **Default:** `info`
- **Environment Variable:** `WG_PORTAL_ADVANCED_LOG_LEVEL`
- **Description:** The log level used by the application. Valid options are: `trace`, `debug`, `info`, `warn`, `error`.

### `log_pretty`
- **Default:** `false`
- **Environment Variable:** `WG_PORTAL_ADVANCED_LOG_PRETTY`
- **Description:** If `true`, log messages are colorized and formatted for readability (pretty-print).

### `log_json`
- **Default:** `false`
- **Environment Variable:** `WG_PORTAL_ADVANCED_LOG_JSON`
- **Description:** If `true`, log messages are structured in JSON format.

### `start_listen_port`
- **Default:** `51820`
- **Environment Variable:** `WG_PORTAL_ADVANCED_START_LISTEN_PORT`
- **Description:** The first port to use when automatically creating new WireGuard interfaces.

### `start_cidr_v4`
- **Default:** `10.11.12.0/24`
- **Environment Variable:** `WG_PORTAL_ADVANCED_START_CIDR_V4`
- **Description:** The initial IPv4 subnet to use when automatically creating new WireGuard interfaces.

### `start_cidr_v6`
- **Default:** `fdfd:d3ad:c0de:1234::0/64`
- **Environment Variable:** `WG_PORTAL_ADVANCED_START_CIDR_V6`
- **Description:** The initial IPv6 subnet to use when automatically creating new WireGuard interfaces.

### `use_ip_v6`
- **Default:** `true`
- **Environment Variable:** `WG_PORTAL_ADVANCED_USE_IP_V6`
- **Description:** Enable or disable IPv6 support.

### `config_storage_path`
- **Default:** *(empty)*
- **Environment Variable:** `WG_PORTAL_ADVANCED_CONFIG_STORAGE_PATH`
- **Description:** Path to a directory where `wg-quick` style configuration files will be stored (if you need local filesystem configs).

### `expiry_check_interval`
- **Default:** `15m`
- **Environment Variable:** `WG_PORTAL_ADVANCED_EXPIRY_CHECK_INTERVAL`
- **Description:** Interval after which existing peers are checked if they are expired. Format uses `s`, `m`, `h` for seconds, minutes, hours, see [time.ParseDuration](https://golang.org/pkg/time/#ParseDuration). Note: `d` (days) is not supported — use `24h` for one day.

### `rule_prio_offset`
- **Default:** `20000`
- **Environment Variable:** `WG_PORTAL_ADVANCED_RULE_PRIO_OFFSET`
- **Description:** Offset for IP route rule priorities when configuring routing.

### `route_table_offset`
- **Default:** `20000`
- **Environment Variable:** `WG_PORTAL_ADVANCED_ROUTE_TABLE_OFFSET`
- **Description:** Offset for IP route table IDs when configuring routing.

### `api_admin_only`
- **Default:** `true`
- **Environment Variable:** `WG_PORTAL_ADVANCED_API_ADMIN_ONLY`
- **Description:** If `true`, the public REST API is accessible only to admin users. The API docs live at [`/api/v1/doc.html`](../rest-api/api-doc.md).

### `limit_additional_user_peers`
- **Default:** `0`
- **Environment Variable:** `WG_PORTAL_ADVANCED_LIMIT_ADDITIONAL_USER_PEERS`
- **Description:** Limit additional peers a normal user can create. `0` means unlimited.

---

## Database

Configuration for the underlying database used by WireGuard Portal. 
Supported databases include SQLite, MySQL, Microsoft SQL Server, and Postgres.

If sensitive values (like private keys) should be stored in an encrypted format, set the `encryption_passphrase` option.

### `debug`
- **Default:** `false`
- **Environment Variable:** `WG_PORTAL_DATABASE_DEBUG`
- **Description:** If `true`, logs all database statements (verbose).

### `slow_query_threshold`
- **Default:** "0"
- **Environment Variable:** `WG_PORTAL_DATABASE_SLOW_QUERY_THRESHOLD`
- **Description:** A time threshold (e.g., `100ms`) above which queries are considered slow and logged as warnings. If zero, slow query logging is disabled. Format uses `s`, `ms` for seconds, milliseconds, see [time.ParseDuration](https://golang.org/pkg/time/#ParseDuration). The value must be a string.

### `type`
- **Default:** `sqlite`
- **Environment Variable:** `WG_PORTAL_DATABASE_TYPE`
- **Description:** The database type. Valid options: `sqlite`, `mssql`, `mysql`, `postgres`.

### `dsn`
- **Default:** `data/sqlite.db`
- **Environment Variable:** `WG_PORTAL_DATABASE_DSN`
- **Description:** The Data Source Name (DSN) for connecting to the database.  
  For example:
  ```text
  user:pass@tcp(1.2.3.4:3306)/dbname?charset=utf8mb4&parseTime=True&loc=Local
  ```

### `encryption_passphrase`
- **Default:** *(empty)*
- **Environment Variable:** `WG_PORTAL_DATABASE_ENCRYPTION_PASSPHRASE`
- **Description:** Passphrase for encrypting sensitive values such as private keys in the database. Encryption is only applied if this passphrase is set.
  **Important:** Once you enable encryption by setting this passphrase, you cannot disable it or change it afterward. 
  New or updated records will be encrypted; existing data remains in plaintext until it’s next modified.

---

## Statistics

Controls how WireGuard Portal collects and reports usage statistics, including ping checks and Prometheus metrics.

### `use_ping_checks`
- **Default:** `true`
- **Environment Variable:** `WG_PORTAL_STATISTICS_USE_PING_CHECKS`
- **Description:** Enable periodic ping checks to verify that peers remain responsive.

### `ping_check_workers`
- **Default:** `10`
- **Environment Variable:** `WG_PORTAL_STATISTICS_PING_CHECK_WORKERS`
- **Description:** Number of parallel worker processes for ping checks.

### `ping_unprivileged`
- **Default:** `false`
- **Environment Variable:** `WG_PORTAL_STATISTICS_PING_UNPRIVILEGED`
- **Description:** If `false`, ping checks run without root privileges. This is currently considered BETA.

### `ping_check_interval`
- **Default:** `1m`
- **Environment Variable:** `WG_PORTAL_STATISTICS_PING_CHECK_INTERVAL`
- **Description:** Interval between consecutive ping checks for all peers. Format uses `s`, `m`, `h` for seconds, minutes, hours, see [time.ParseDuration](https://golang.org/pkg/time/#ParseDuration). Note: `d` (days) is not supported — use `24h` for one day.

### `data_collection_interval`
- **Default:** `1m`
- **Environment Variable:** `WG_PORTAL_STATISTICS_DATA_COLLECTION_INTERVAL`
- **Description:** Interval between data collection cycles (bytes sent/received, handshake times, etc.). Format uses `s`, `m`, `h` for seconds, minutes, hours, see [time.ParseDuration](https://golang.org/pkg/time/#ParseDuration). Note: `d` (days) is not supported — use `24h` for one day.

### `collect_interface_data`
- **Default:** `true`
- **Environment Variable:** `WG_PORTAL_STATISTICS_COLLECT_INTERFACE_DATA`
- **Description:** If `true`, collects interface-level data (bytes in/out) for monitoring and statistics.

### `collect_peer_data`
- **Default:** `true`
- **Environment Variable:** `WG_PORTAL_STATISTICS_COLLECT_PEER_DATA`
- **Description:** If `true`, collects peer-level data (bytes, last handshake, endpoint, etc.).

### `collect_audit_data`
- **Default:** `true`
- **Environment Variable:** `WG_PORTAL_STATISTICS_COLLECT_AUDIT_DATA`
- **Description:** If `true`, logs certain portal events (such as user logins) to the database.

### `listening_address`
- **Default:** `:8787`
- **Environment Variable:** `WG_PORTAL_STATISTICS_LISTENING_ADDRESS`
- **Description:** Address and port for the integrated Prometheus metric server (e.g., `:8787` or `127.0.0.1:8787`).

---

## Mail

Options for configuring email notifications or sending peer configurations via email. 
By default, emails will only be sent to peers that have a valid user record linked. 
To send emails to all peers that have a valid email-address as user-identifier, set `allow_peer_email` to `true`.

### `host`
- **Default:** `127.0.0.1`
- **Environment Variable:** `WG_PORTAL_MAIL_HOST`
- **Description:** Hostname or IP of the SMTP server.

### `port`
- **Default:** `25`
- **Environment Variable:** `WG_PORTAL_MAIL_PORT`
- **Description:** Port number for the SMTP server.

### `encryption`
- **Default:** `none`
- **Environment Variable:** `WG_PORTAL_MAIL_ENCRYPTION`
- **Description:** SMTP encryption type. Valid values: `none`, `tls`, `starttls`.

### `cert_validation`
- **Default:** `true`
- **Environment Variable:** `WG_PORTAL_MAIL_CERT_VALIDATION`
- **Description:** If `true`, validate the SMTP server certificate (relevant if `encryption` = `tls`).

### `username`
- **Default:** *(empty)*
- **Environment Variable:** `WG_PORTAL_MAIL_USERNAME`
- **Description:** Optional SMTP username for authentication.

### `password`
- **Default:** *(empty)*
- **Environment Variable:** `WG_PORTAL_MAIL_PASSWORD`
- **Description:** Optional SMTP password for authentication.

### `auth_type`
- **Default:** `plain`
- **Environment Variable:** `WG_PORTAL_MAIL_AUTH_TYPE`
- **Description:** SMTP authentication type. Valid values: `plain`, `login`, `crammd5`.

### `from`
- **Default:** `Wireguard Portal <noreply@wireguard.local>`
- **Environment Variable:** `WG_PORTAL_MAIL_FROM`
- **Description:** The default "From" address when sending emails.

### `link_only`
- **Default:** `false`
- **Environment Variable:** `WG_PORTAL_MAIL_LINK_ONLY`
- **Description:** If `true`, emails only contain a link to WireGuard Portal, rather than attaching the full configuration.

### `allow_peer_email`
- **Default:** `false`
- **Environment Variable:** `WG_PORTAL_MAIL_ALLOW_PEER_EMAIL`
- **Description:** If `true`, and a peer has no valid user record linked, but the user-identifier of the peer is a valid email address, emails will be sent to that email address.
  If false, and the peer has no valid user record linked, emails will not be sent.
  If a peer has linked a valid user, the email address is always taken from the user record.

### `templates_path`
- **Default:** *(empty)*
- **Environment Variable:** `WG_PORTAL_MAIL_TEMPLATES_PATH`
- **Description:** Path to the email template files that override embedded templates. Check [usage documentation](../usage/mail-templates.md) for an example.`

---

## Auth

WireGuard Portal supports multiple authentication strategies, including **OpenID Connect** (`oidc`), **OAuth** (`oauth`), **Passkeys** (`webauthn`) and **LDAP** (`ldap`).
Each can have multiple providers configured. Below are the relevant keys.

Some core authentication options are shared across all providers, while others are specific to each provider type.

### `min_password_length`
- **Default:** `16`
- **Environment Variable:** `WG_PORTAL_AUTH_MIN_PASSWORD_LENGTH`
- **Description:** Minimum password length for local authentication. This is not enforced for LDAP authentication.
  The default admin password strength is also enforced by this setting.
- **Important:** The password should be strong and secure. It is recommended to use a password with at least 16 characters, including uppercase and lowercase letters, numbers, and special characters.

### `hide_login_form`
- **Default:** `false`
- **Environment Variable:** `WG_PORTAL_AUTH_HIDE_LOGIN_FORM`
- **Description:** If `true`, the login form is hidden and only the OIDC, OAuth, LDAP, or WebAuthn providers are shown. This is useful if you want to enforce a specific authentication method.
  If no social login providers are configured, the login form is always shown, regardless of this setting.
- **Important:** You can still access the login form by adding the `?all` query parameter to the login URL (e.g. https://wg.portal/#/login?all). 

---

### OIDC

The `oidc` array contains a list of OpenID Connect providers. 
Below are the properties for each OIDC provider entry inside `auth.oidc`:

#### `provider_name`
- **Default:** *(empty)*
- **Description:** A **unique** name for this provider. Must not conflict with other providers.

#### `display_name`
- **Default:** *(empty)*
- **Description:** A user-friendly name shown on the login page (e.g., "Login with Google").

#### `base_url`
- **Default:** *(empty)*
- **Description:** The OIDC provider’s base URL (e.g., `https://accounts.google.com`).

#### `client_id`
- **Default:** *(empty)*
- **Description:** The OAuth client ID from the OIDC provider.

#### `client_secret`
- **Default:** *(empty)*
- **Description:** The OAuth client secret from the OIDC provider.

#### `extra_scopes`
- **Default:** *(empty)*
- **Description:** A list of additional OIDC scopes (e.g., `profile`, `email`).

#### `allowed_domains`
- **Default:** *(empty)*
- **Description:** A list of allowlisted domains. Only users with email addresses in these domains can log in or register. This is useful for restricting access to specific organizations or groups.

#### `allowed_user_groups`
- **Default:** *(empty)*
- **Description:** A list of allowlisted user groups. If configured, at least one entry in the mapped `user_groups` claim must match one of these values.

#### `field_map`
- **Default:** *(empty)*
- **Description:** Maps OIDC claims to WireGuard Portal user fields. 
  - Available fields: `user_identifier`, `email`, `firstname`, `lastname`, `phone`, `department`, `is_admin`, `user_groups`.

    | **Field**         | **Typical OIDC Claim**            | **Explanation**                                                                                                                                                                                         |
    |-------------------|-----------------------------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
    | `user_identifier` | `sub` or `preferred_username`     | A unique identifier for the user. Often the OIDC `sub` claim is used because it’s guaranteed to be unique for the user within the IdP. Some providers also support `preferred_username` if it’s unique. |
    | `email`           | `email`                           | The user’s email address as provided by the IdP. Not always verified, depending on IdP settings.                                                                                                        |
    | `firstname`       | `given_name`                      | The user’s first name, typically provided by the IdP in the `given_name` claim.                                                                                                                         |
    | `lastname`        | `family_name`                     | The user’s last (family) name, typically provided by the IdP in the `family_name` claim.                                                                                                                |
    | `phone`           | `phone_number`                    | The user’s phone number. This may require additional scopes/permissions from the IdP to access.                                                                                                         |
    | `department`      | Custom claim (e.g., `department`) | If the IdP can provide organizational data, it may store it in a custom claim. Adjust accordingly (e.g., `department`, `org`, or another attribute).                                                    |
    | `is_admin`        | Custom claim or derived role      | If the IdP returns a role or admin flag, you can map that to `is_admin`. Often this is managed through custom claims or group membership.                                                               |
    | `user_groups`     | `groups` or another custom claim  | A list of group memberships for the user. Some IdPs provide `groups` out of the box; others require custom claims or directory lookups.                                                                 |

#### `admin_mapping`
- **Default:** *(empty)*
- **Description:** WgPortal can grant a user admin rights by matching the value of the `is_admin` claim against a regular expression. Alternatively, a regular expression can be used to check if a user is member of a specific group listed in the `user_group` claim. The regular expressions are defined in `admin_value_regex` and `admin_group_regex`.
    - `admin_value_regex`: A regular expression to match the `is_admin` claim. By default, this expression matches the string "true" (`^true$`).
    - `admin_group_regex`: A regular expression to match the `user_groups` claim. Each entry in the `user_groups` claim is checked against this regex.

#### `registration_enabled`
- **Default:** `false`
- **Description:** If `true`, a new user will be created in WireGuard Portal if not already present.

#### `log_user_info`
- **Default:** `false`
- **Description:** If `true`, OIDC user data is logged at the trace level upon login (for debugging).

#### `log_sensitive_info`
- **Default:** `false`
- **Description:** If `true`, sensitive OIDC user data, such as tokens and raw responses, will be logged at the trace level upon login (for debugging).
- **Important:** Keep this setting disabled in production environments! Remove logs once you finished debugging authentication issues.

#### `logout_idp_session`
- **Default:** `true`
- **Description:** If `true` (default), WireGuard Portal will redirect the user to the OIDC provider's `end_session_endpoint` after local logout, terminating the session at the IdP as well. Set to `false` to only invalidate the local WireGuard Portal session without touching the IdP session.

---

### OAuth

The `oauth` array contains a list of plain OAuth2 providers.
Below are the properties for each OAuth provider entry inside `auth.oauth`:

#### `provider_name`
- **Default:** *(empty)*
- **Description:** A **unique** name for this provider. Must not conflict with other providers.

#### `display_name`
- **Default:** *(empty)*
- **Description:** A user-friendly name shown on the login page.

#### `client_id`
- **Default:** *(empty)*
- **Description:** The OAuth client ID for the provider.

#### `client_secret`
- **Default:** *(empty)*
- **Description:** The OAuth client secret for the provider.

#### `auth_url`
- **Default:** *(empty)*
- **Description:** URL of the authentication endpoint.

#### `token_url`
- **Default:** *(empty)*
- **Description:** URL of the token endpoint.

#### `user_info_url`
- **Default:** *(empty)*
- **Description:** URL of the user information endpoint.

#### `scopes`
- **Default:** *(empty)*
- **Description:** A list of OAuth scopes.

#### `allowed_domains`
- **Default:** *(empty)*
- **Description:** A list of allowlisted domains. Only users with email addresses in these domains can log in or register. This is useful for restricting access to specific organizations or groups.

#### `allowed_user_groups`
- **Default:** *(empty)*
- **Description:** A list of allowlisted user groups. If configured, at least one entry in the mapped `user_groups` claim must match one of these values.

#### `field_map`
- **Default:** *(empty)*
- **Description:** Maps OAuth attributes to WireGuard Portal fields.
  - Available fields: `user_identifier`, `email`, `firstname`, `lastname`, `phone`, `department`, `is_admin`, `user_groups`.

    | **Field**         | **Typical Claim**                 | **Explanation**                                                                                                                                                                                         |
    |-------------------|-----------------------------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
    | `user_identifier` | `sub` or `preferred_username`     | A unique identifier for the user. Often the OIDC `sub` claim is used because it’s guaranteed to be unique for the user within the IdP. Some providers also support `preferred_username` if it’s unique. |
    | `email`           | `email`                           | The user’s email address as provided by the IdP. Not always verified, depending on IdP settings.                                                                                                        |
    | `firstname`       | `given_name`                      | The user’s first name, typically provided by the IdP in the `given_name` claim.                                                                                                                         |
    | `lastname`        | `family_name`                     | The user’s last (family) name, typically provided by the IdP in the `family_name` claim.                                                                                                                |
    | `phone`           | `phone_number`                    | The user’s phone number. This may require additional scopes/permissions from the IdP to access.                                                                                                         |
    | `department`      | Custom claim (e.g., `department`) | If the IdP can provide organizational data, it may store it in a custom claim. Adjust accordingly (e.g., `department`, `org`, or another attribute).                                                    |
    | `is_admin`        | Custom claim or derived role      | If the IdP returns a role or admin flag, you can map that to `is_admin`. Often this is managed through custom claims or group membership.                                                               |
    | `user_groups`     | `groups` or another custom claim  | A list of group memberships for the user. Some IdPs provide `groups` out of the box; others require custom claims or directory lookups.                                                                 |

#### `admin_mapping`
- **Default:** *(empty)*
- **Description:** WgPortal can grant a user admin rights by matching the value of the `is_admin` claim against a regular expression. Alternatively, a regular expression can be used to check if a user is member of a specific group listed in the `user_group` claim. The regular expressions are defined in `admin_value_regex` and `admin_group_regex`.
  - `admin_value_regex`: A regular expression to match the `is_admin` claim. By default, this expression matches the string "true" (`^true$`).
  - `admin_group_regex`: A regular expression to match the `user_groups` claim. Each entry in the `user_groups` claim is checked against this regex.

#### `registration_enabled`
- **Default:** `false`
- **Description:** If `true`, new users are created automatically on successful login.

#### `log_user_info`
- **Default:** `false`
- **Description:** If `true`, logs user info at the trace level upon login.

#### `log_sensitive_info`
- **Default:** `false`
- **Description:** If `true`, sensitive OIDC user data, such as tokens and raw responses, will be logged at the trace level upon login (for debugging).
- **Important:** Keep this setting disabled in production environments! Remove logs once you finished debugging authentication issues.

---

### LDAP

The `ldap` array contains a list of LDAP authentication providers.
Below are the properties for each LDAP provider entry inside `auth.ldap`:

#### `provider_name`
- **Default:** *(empty)*
- **Description:** A **unique** name for this provider. Must not conflict with other providers.

#### `url`
- **Default:** *(empty)*
- **Description:** The LDAP server URL (e.g., `ldap://srv-ad01.company.local:389`).

#### `start_tls`
- **Default:** `false`
- **Description:** If `true`, use STARTTLS to secure the LDAP connection.

#### `cert_validation`
- **Default:** `false`
- **Description:** If `true`, validate the LDAP server’s TLS certificate.

#### `tls_certificate_path`
- **Default:** *(empty)*
- **Description:** Path to a TLS certificate if needed for LDAP connections.

#### `tls_key_path`
- **Default:** *(empty)*
- **Description:** Path to the corresponding TLS certificate key.

#### `base_dn`
- **Default:** *(empty)*
- **Description:** The base DN for user searches (e.g., `DC=COMPANY,DC=LOCAL`).

#### `bind_user`
- **Default:** *(empty)*
- **Description:** The bind user for LDAP (e.g., `company\\ldap_wireguard` or `ldap_wireguard@company.local`).

#### `bind_pass`
- **Default:** *(empty)*
- **Description:** The bind password for LDAP authentication.

#### `field_map`
- **Default:** *(empty)*
- **Description:** Maps LDAP attributes to WireGuard Portal fields.
    - Available fields: `user_identifier`, `email`, `firstname`, `lastname`, `phone`, `department`, `memberof`.
  
      | **WireGuard Portal Field** | **Typical LDAP Attribute** | **Short Description**                                        |
      |----------------------------|----------------------------|--------------------------------------------------------------|
      | user_identifier            | sAMAccountName / uid       | Uniquely identifies the user within the LDAP directory.      |
      | email                      | mail / userPrincipalName   | Stores the user's primary email address.                     |
      | firstname                  | givenName                  | Contains the user's first (given) name.                      |
      | lastname                   | sn                         | Contains the user's last (surname) name.                     |
      | phone                      | telephoneNumber / mobile   | Holds the user's phone or mobile number.                     |
      | department                 | departmentNumber / ou      | Specifies the department or organizational unit of the user. |
      | memberof                   | memberOf                   | Lists the groups and roles to which the user belongs.        |

#### `login_filter`
- **Default:** *(empty)*
- **Description:** An LDAP filter to restrict which users can log in. Use `{{login_identifier}}` to insert the username.
  For example:
  ```text
  (&(objectClass=organizationalPerson)(mail={{login_identifier}})(!userAccountControl:1.2.840.113556.1.4.803:=2))
  ```
- **Important**: The `login_filter` must always be a valid LDAP filter. It should at most return one user. 
  If the filter returns multiple or no users, the login will fail.

#### `interface_filter`
- **Default:** *(empty)*
- **Description:** A map of LDAP filters to restrict access to specific WireGuard interfaces. The map keys are the interface identifiers (e.g., `wg0`), and the values are LDAP filters. Only users matching the filter will be allowed to provision peers for the respective interface.
  For example:
  ```yaml
  interface_filter:
    wg0: "(memberOf=CN=VPNUsers,OU=Groups,DC=COMPANY,DC=LOCAL)"
    wg1: "(description=special-access)"
  ```

#### `admin_group`
- **Default:** *(empty)*
- **Description:** A specific LDAP group whose members are considered administrators in WireGuard Portal.
  For example:
  ```text
  CN=WireGuardAdmins,OU=Some-OU,DC=YOURDOMAIN,DC=LOCAL
  ```

#### `sync_interval`
- **Default:** *(empty)*
- **Description:** How frequently (in duration, e.g. `30m`) to synchronize users from LDAP. Empty or `0` disables sync. Format uses `s`, `m`, `h`, `d` for seconds, minutes, hours, days, see [time.ParseDuration](https://golang.org/pkg/time/#ParseDuration).
  Only users that match the `sync_filter` are synchronized, if `disable_missing` is `true`, users not found in LDAP are disabled.

#### `sync_filter`
- **Default:** *(empty)*
- **Description:** An LDAP filter to select which users get synchronized into WireGuard Portal.
  For example:
  ```text
  (&(objectClass=organizationalPerson)(!userAccountControl:1.2.840.113556.1.4.803:=2)(mail=*))
  ```

#### `sync_log_user_info`
- **Default:** `false`
- **Description:** If `true`, logs LDAP user data at the trace level during synchronization.

#### `disable_missing`
- **Default:** `false`
- **Description:** If `true`, any user **not** found in LDAP (during sync) is disabled in WireGuard Portal.

#### `auto_re_enable`
- **Default:** `false`
- **Description:** If `true`, users that where disabled because they were missing (see `disable_missing`) will be re-enabled once they are found again.

#### `registration_enabled`
- **Default:** `false`
- **Description:** If `true`, new user accounts are created in WireGuard Portal upon first login.

#### `log_user_info`
- **Default:** `false`
- **Description:** If `true`, logs LDAP user data at the trace level upon login.

---

### WebAuthn (Passkeys)

The `webauthn` section contains configuration options for WebAuthn authentication (passkeys).

#### `enabled`
- **Default:** `true`
- **Environment Variable:** `WG_PORTAL_AUTH_WEBAUTHN_ENABLED`
- **Description:** If `true`, Passkey authentication is enabled. If `false`, WebAuthn is disabled.
  Users are encouraged to use Passkeys for secure authentication instead of passwords. 
  If a passkey is registered, the password login is still available as a fallback. Ensure that the password is strong and secure.

## Web

The web section contains configuration options for the web server, including the listening address, session management, and CSRF protection.
It is important to specify a valid `external_url` for the web server, especially if you are using a reverse proxy. 
Without a valid `external_url`, the login process may fail due to CSRF protection.

### `listening_address`
- **Default:** `:8888`
- **Environment Variable:** `WG_PORTAL_WEB_LISTENING_ADDRESS`
- **Description:** The listening address and port for the web server (e.g., `:8888` to bind on all interfaces or `127.0.0.1:8888` to bind only on the loopback interface).
  Ensure that access to WireGuard Portal is protected against unauthorized access, especially if binding to all interfaces.

### `external_url`
- **Default:** `http://localhost:8888`
- **Environment Variable:** `WG_PORTAL_WEB_EXTERNAL_URL`
- **Description:** The URL where a client can access WireGuard Portal. This URL is used for generating links in emails and for performing OAUTH redirects.
  The external URL must not contain a path component or trailing slash. If you want to serve WireGuard Portal on a subpath, use the `base_path` setting.
  **Important:** If you are using a reverse proxy, set this to the external URL of the reverse proxy, otherwise login will fail. If you access the portal via IP address, set this to the IP address of the server.

### `base_path`
- **Default:** *(empty)*
- **Environment Variable:** `WG_PORTAL_WEB_BASE_PATH`
- **Description:** The base path for the web server (e.g., `/wgportal`). 
  By default (meaning an empty value), the portal will be served from the root path `/`.

### `site_company_name`
- **Default:** `WireGuard Portal`
- **Environment Variable:** `WG_PORTAL_WEB_SITE_COMPANY_NAME`
- **Description:** The company name that is shown at the bottom of the web frontend.

### `site_title`
- **Default:** `WireGuard Portal`
- **Environment Variable:** `WG_PORTAL_WEB_SITE_TITLE`
- **Description:** The title that is shown in the web frontend.

### `session_identifier`
- **Default:** `wgPortalSession`
- **Environment Variable:** `WG_PORTAL_WEB_SESSION_IDENTIFIER`
- **Description:** The session identifier for the web frontend.

### `session_secret`
- **Default:** `very_secret`
- **Environment Variable:** `WG_PORTAL_WEB_SESSION_SECRET`
- **Description:** The session secret for the web frontend.

### `csrf_secret`
- **Default:** `extremely_secret`
- **Environment Variable:** `WG_PORTAL_WEB_CSRF_SECRET`
- **Description:** The CSRF secret.

### `request_logging`
- **Default:** `false`
- **Environment Variable:** `WG_PORTAL_WEB_REQUEST_LOGGING`
- **Description:** Log all HTTP requests.

### `expose_host_info`
- **Default:** `false`
- **Environment Variable:** `WG_PORTAL_WEB_EXPOSE_HOST_INFO`
- **Description:** Expose the hostname and version of the WireGuard Portal server in an HTTP header. This is useful for debugging but may expose sensitive information.

### `cert_file`
- **Default:** *(empty)*
- **Environment Variable:** `WG_PORTAL_WEB_CERT_FILE`
- **Description:** (Optional) Path to the TLS certificate file.

### `key_file`
- **Default:** *(empty)*
- **Environment Variable:** `WG_PORTAL_WEB_KEY_FILE`
- **Description:** (Optional) Path to the TLS certificate key file.

### `frontend_filepath`
- **Default:** *(empty)*
- **Environment Variable:** `WG_PORTAL_WEB_FRONTEND_FILEPATH`
- **Description:** Optional base directory from which the web frontend is served. Check out the [building](../getting-started/sources.md) documentation for more information on how to compile the frontend assets.
  - If the directory contains at least one file (recursively), these files are served at `/app`, overriding the embedded frontend assets.
  - If the directory is empty or does not exist on startup, the embedded frontend is copied into this directory automatically and then served.
  - If left empty, the embedded frontend is served and no files are written to disk.

---

## Webhook

The webhook section allows you to configure a webhook that is called on certain events in WireGuard Portal.
Further details can be found in the [usage documentation](../usage/webhooks.md).

### `url`
- **Default:** *(empty)*
- **Environment Variable:** `WG_PORTAL_WEBHOOK_URL`
- **Description:** The POST endpoint to which the webhook is sent. The URL must be reachable from the WireGuard Portal server. If the URL is empty, the webhook is disabled.

### `authentication`
- **Default:** *(empty)*
- **Environment Variable:** `WG_PORTAL_WEBHOOK_AUTHENTICATION`
- **Description:** The Authorization header for the webhook endpoint. The value is send as-is in the header. For example: `Bearer <token>`.

### `timeout`
- **Default:** `10s`
- **Environment Variable:** `WG_PORTAL_WEBHOOK_TIMEOUT`
- **Description:** The timeout for the webhook request. If the request takes longer than this, it is aborted.
