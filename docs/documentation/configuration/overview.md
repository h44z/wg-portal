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
  editable_keys: true
  create_default_peer: false
  create_default_peer_on_creation: false
  re_enable_peer_after_user_enable: true
  delete_peer_after_user_deleted: false
  self_provisioning_allowed: false
  import_existing: true
  restore_state: true

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

auth:
  oidc: []
  oauth: []
  ldap: []
  webauthn:
    enabled: true
  min_password_length: 16

web:
  listening_address: :8888
  external_url: http://localhost:8888
  site_company_name: WireGuard Portal
  site_title: WireGuard Portal
  session_identifier: wgPortalSession
  session_secret: very_secret
  csrf_secret: extremely_secret
  request_logging: false
  expose_host_info: false
  cert_file: ""
  key_File: ""

webhook:
  url: ""
  authentication: ""
  timeout: 10s
```

</details>


Below you will find sections like
[`core`](#core),
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
- **Description:** The administrator user. This user will be created as a default admin if it does not yet exist.

### `admin_password`
- **Default:** `wgportal-default`
- **Description:** The administrator password. The default password should be changed immediately!
- **Important:** The password should be strong and secure. The minimum password length is specified in [auth.min_password_length](#min_password_length). By default, it is 16 characters.

### `admin_api_token`
- **Default:** *(empty)*
- **Description:** An API token for the admin user. If a token is provided, the REST API can be accessed using this token. If empty, the API is initially disabled for the admin user.

### `editable_keys`
- **Default:** `true`
- **Description:** Allow editing of WireGuard key-pairs directly in the UI.

### `create_default_peer`
- **Default:** `false`
- **Description:** If a user logs in for the first time with no existing peers, automatically create a new WireGuard peer for **all** server interfaces.

### `create_default_peer_on_creation`
- **Default:** `false`
- **Description:** If an LDAP user is created (e.g., through LDAP sync) and has no peers, automatically create a new WireGuard peer for **all** server interfaces.

### `re_enable_peer_after_user_enable`
- **Default:** `true`
- **Description:** Re-enable all peers that were previously disabled if the associated user is re-enabled.

### `delete_peer_after_user_deleted`
- **Default:** `false`
- **Description:** If a user is deleted, remove all linked peers. Otherwise, peers remain but are disabled.

### `self_provisioning_allowed`
- **Default:** `false`
- **Description:** Allow registered (non-admin) users to self-provision peers from their profile page.

### `import_existing`
- **Default:** `true`
- **Description:** On startup, import existing WireGuard interfaces and peers into WireGuard Portal.

### `restore_state`
- **Default:** `true`
- **Description:** Restore the WireGuard interface states (up/down) that existed before WireGuard Portal started.

---

## Advanced

Additional or more specialized configuration options for logging and interface creation details.

### `log_level`
- **Default:** `info`
- **Description:** The log level used by the application. Valid options are: `trace`, `debug`, `info`, `warn`, `error`.

### `log_pretty`
- **Default:** `false`
- **Description:** If `true`, log messages are colorized and formatted for readability (pretty-print).

### `log_json`
- **Default:** `false`
- **Description:** If `true`, log messages are structured in JSON format.

### `start_listen_port`
- **Default:** `51820`
- **Description:** The first port to use when automatically creating new WireGuard interfaces.

### `start_cidr_v4`
- **Default:** `10.11.12.0/24`
- **Description:** The initial IPv4 subnet to use when automatically creating new WireGuard interfaces.

### `start_cidr_v6`
- **Default:** `fdfd:d3ad:c0de:1234::0/64`
- **Description:** The initial IPv6 subnet to use when automatically creating new WireGuard interfaces.

### `use_ip_v6`
- **Default:** `true`
- **Description:** Enable or disable IPv6 support.

### `config_storage_path`
- **Default:** *(empty)*
- **Description:** Path to a directory where `wg-quick` style configuration files will be stored (if you need local filesystem configs).

### `expiry_check_interval`
- **Default:** `15m`
- **Description:** Interval after which existing peers are checked if they are expired. Format uses `s`, `m`, `h`, `d` for seconds, minutes, hours, days, see [time.ParseDuration](https://golang.org/pkg/time/#ParseDuration).

### `rule_prio_offset`
- **Default:** `20000`
- **Description:** Offset for IP route rule priorities when configuring routing.

### `route_table_offset`
- **Default:** `20000`
- **Description:** Offset for IP route table IDs when configuring routing.

### `api_admin_only`
- **Default:** `true`
- **Description:** If `true`, the public REST API is accessible only to admin users. The API docs live at [`/api/v1/doc.html`](../rest-api/api-doc.md).

---

## Database

Configuration for the underlying database used by WireGuard Portal. 
Supported databases include SQLite, MySQL, Microsoft SQL Server, and Postgres.

If sensitive values (like private keys) should be stored in an encrypted format, set the `encryption_passphrase` option.

### `debug`
- **Default:** `false`
- **Description:** If `true`, logs all database statements (verbose).

### `slow_query_threshold`
- **Default:** "0"
- **Description:** A time threshold (e.g., `100ms`) above which queries are considered slow and logged as warnings. If zero, slow query logging is disabled. Format uses `s`, `ms` for seconds, milliseconds, see [time.ParseDuration](https://golang.org/pkg/time/#ParseDuration). The value must be a string.

### `type`
- **Default:** `sqlite`
- **Description:** The database type. Valid options: `sqlite`, `mssql`, `mysql`, `postgres`.

### `dsn`
- **Default:** `data/sqlite.db`
- **Description:** The Data Source Name (DSN) for connecting to the database.  
  For example:
  ```text
  user:pass@tcp(1.2.3.4:3306)/dbname?charset=utf8mb4&parseTime=True&loc=Local
  ```

### `encryption_passphrase`
- **Default:** *(empty)*
- **Description:** Passphrase for encrypting sensitive values such as private keys in the database. Encryption is only applied if this passphrase is set.
  **Important:** Once you enable encryption by setting this passphrase, you cannot disable it or change it afterward. 
  New or updated records will be encrypted; existing data remains in plaintext until it’s next modified.

---

## Statistics

Controls how WireGuard Portal collects and reports usage statistics, including ping checks and Prometheus metrics.

### `use_ping_checks`
- **Default:** `true`
- **Description:** Enable periodic ping checks to verify that peers remain responsive.

### `ping_check_workers`
- **Default:** `10`
- **Description:** Number of parallel worker processes for ping checks.

### `ping_unprivileged`
- **Default:** `false`
- **Description:** If `false`, ping checks run without root privileges. This is currently considered BETA.

### `ping_check_interval`
- **Default:** `1m`
- **Description:** Interval between consecutive ping checks for all peers. Format uses `s`, `m`, `h`, `d` for seconds, minutes, hours, days, see [time.ParseDuration](https://golang.org/pkg/time/#ParseDuration).

### `data_collection_interval`
- **Default:** `1m`
- **Description:** Interval between data collection cycles (bytes sent/received, handshake times, etc.). Format uses `s`, `m`, `h`, `d` for seconds, minutes, hours, days, see [time.ParseDuration](https://golang.org/pkg/time/#ParseDuration).

### `collect_interface_data`
- **Default:** `true`
- **Description:** If `true`, collects interface-level data (bytes in/out) for monitoring and statistics.

### `collect_peer_data`
- **Default:** `true`
- **Description:** If `true`, collects peer-level data (bytes, last handshake, endpoint, etc.).

### `collect_audit_data`
- **Default:** `true`
- **Description:** If `true`, logs certain portal events (such as user logins) to the database.

### `listening_address`
- **Default:** `:8787`
- **Description:** Address and port for the integrated Prometheus metric server (e.g., `:8787` or `127.0.0.1:8787`).

---

## Mail

Options for configuring email notifications or sending peer configurations via email.

### `host`
- **Default:** `127.0.0.1`
- **Description:** Hostname or IP of the SMTP server.

### `port`
- **Default:** `25`
- **Description:** Port number for the SMTP server.

### `encryption`
- **Default:** `none`
- **Description:** SMTP encryption type. Valid values: `none`, `tls`, `starttls`.

### `cert_validation`
- **Default:** `true`
- **Description:** If `true`, validate the SMTP server certificate (relevant if `encryption` = `tls`).

### `username`
- **Default:** *(empty)*
- **Description:** Optional SMTP username for authentication.

### `password`
- **Default:** *(empty)*
- **Description:** Optional SMTP password for authentication.

### `auth_type`
- **Default:** `plain`
- **Description:** SMTP authentication type. Valid values: `plain`, `login`, `crammd5`.

### `from`
- **Default:** `Wireguard Portal <noreply@wireguard.local>`
- **Description:** The default "From" address when sending emails.

### `link_only`
- **Default:** `false`
- **Description:** If `true`, emails only contain a link to WireGuard Portal, rather than attaching the full configuration.

---

## Auth

WireGuard Portal supports multiple authentication strategies, including **OpenID Connect** (`oidc`), **OAuth** (`oauth`), **Passkeys** (`webauthn`) and **LDAP** (`ldap`).
Each can have multiple providers configured. Below are the relevant keys.

Some core authentication options are shared across all providers, while others are specific to each provider type.

### `min_password_length`
- **Default:** `16`
- **Description:** Minimum password length for local authentication. This is not enforced for LDAP authentication.
  The default admin password strength is also enforced by this setting.
- **Important:** The password should be strong and secure. It is recommended to use a password with at least 16 characters, including uppercase and lowercase letters, numbers, and special characters.

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
- **Default:** *(empty)*
- **Description:** If `true`, a new user will be created in WireGuard Portal if not already present.

#### `log_user_info`
- **Default:** *(empty)*
- **Description:** If `true`, OIDC user data is logged at the trace level upon login (for debugging).

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
- **Default:** *(empty)*
- **Description:** If `true`, new users are created automatically on successful login.

#### `log_user_info`
- **Default:** *(empty)*
- **Description:** If `true`, logs user info at the trace level upon login.

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
- **Default:** *(empty)*
- **Description:** If `true`, use STARTTLS to secure the LDAP connection.

#### `cert_validation`
- **Default:** *(empty)*
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

#### `disable_missing`
- **Default:** *(empty)*
- **Description:** If `true`, any user **not** found in LDAP (during sync) is disabled in WireGuard Portal.

#### `auto_re_enable`
- **Default:** *(empty)*
- **Description:** If `true`, users that where disabled because they were missing (see `disable_missing`) will be re-enabled once they are found again.

#### `registration_enabled`
- **Default:** *(empty)*
- **Description:** If `true`, new user accounts are created in WireGuard Portal upon first login.

#### `log_user_info`
- **Default:** *(empty)*
- **Description:** If `true`, logs LDAP user data at the trace level upon login.

---

### WebAuthn (Passkeys)

The `webauthn` section contains configuration options for WebAuthn authentication (passkeys).

#### `enabled`
- **Default:** `true`
- **Description:** If `true`, Passkey authentication is enabled. If `false`, WebAuthn is disabled.
  Users are encouraged to use Passkeys for secure authentication instead of passwords. 
  If a passkey is registered, the password login is still available as a fallback. Ensure that the password is strong and secure.

## Web

The web section contains configuration options for the web server, including the listening address, session management, and CSRF protection.
It is important to specify a valid `external_url` for the web server, especially if you are using a reverse proxy. 
Without a valid `external_url`, the login process may fail due to CSRF protection.

### `listening_address`
- **Default:** `:8888`
- **Description:** The listening address and port for the web server (e.g., `:8888` to bind on all interfaces or `127.0.0.1:8888` to bind only on the loopback interface).
  Ensure that access to WireGuard Portal is protected against unauthorized access, especially if binding to all interfaces.

### `external_url`
- **Default:** `http://localhost:8888`
- **Description:** The URL where a client can access WireGuard Portal. This URL is used for generating links in emails and for performing OAUTH redirects.  
  **Important:** If you are using a reverse proxy, set this to the external URL of the reverse proxy, otherwise login will fail. If you access the portal via IP address, set this to the IP address of the server.

### `site_company_name`
- **Default:** `WireGuard Portal`
- **Description:** The company name that is shown at the bottom of the web frontend.

### `site_title`
- **Default:** `WireGuard Portal`
- **Description:** The title that is shown in the web frontend.

### `session_identifier`
- **Default:** `wgPortalSession`
- **Description:** The session identifier for the web frontend.

### `session_secret`
- **Default:** `very_secret`
- **Description:** The session secret for the web frontend.

### `csrf_secret`
- **Default:** `extremely_secret`
- **Description:** The CSRF secret.

### `request_logging`
- **Default:** `false`
- **Description:** Log all HTTP requests.

### `expose_host_info`
- **Default:** `false`
- **Description:** Expose the hostname and version of the WireGuard Portal server in an HTTP header. This is useful for debugging but may expose sensitive information.

### `cert_file`
- **Default:** *(empty)*
- **Description:** (Optional) Path to the TLS certificate file.

### `key_file`
- **Default:** *(empty)*
- **Description:** (Optional) Path to the TLS certificate key file.

---

## Webhook

The webhook section allows you to configure a webhook that is called on certain events in WireGuard Portal.
A JSON object is sent in a POST request to the webhook URL with the following structure:
```json
{
  "event": "peer_created",
  "entity": "peer",
  "identifier": "the-peer-identifier",
  "payload": {
    // The payload of the event, e.g. peer data.
    // Check the API documentation for the exact structure.
  }
}
```

### `url`
- **Default:** *(empty)*
- **Description:** The POST endpoint to which the webhook is sent. The URL must be reachable from the WireGuard Portal server. If the URL is empty, the webhook is disabled.

### `authentication`
- **Default:** *(empty)*
- **Description:** The Authorization header for the webhook endpoint. The value is send as-is in the header. For example: `Bearer <token>`.

### `timeout`
- **Default:** `10s`
- **Description:** The timeout for the webhook request. If the request takes longer than this, it is aborted.