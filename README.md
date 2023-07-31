# WireGuard Portal (V2 - alpha testing)

[![Build Status](https://travis-ci.com/h44z/wg-portal.svg?token=q4pSqaqT58Jzpxdx62xk&branch=master)](https://travis-ci.com/h44z/wg-portal)
[![License: MIT](https://img.shields.io/badge/license-MIT-green.svg)](https://opensource.org/licenses/MIT)
![GitHub last commit](https://img.shields.io/github/last-commit/h44z/wg-portal)
[![Go Report Card](https://goreportcard.com/badge/github.com/h44z/wg-portal)](https://goreportcard.com/report/github.com/h44z/wg-portal)
![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/h44z/wg-portal)
![GitHub code size in bytes](https://img.shields.io/github/languages/code-size/h44z/wg-portal)
[![Docker Pulls](https://img.shields.io/docker/pulls/h44z/wg-portal.svg)](https://hub.docker.com/r/h44z/wg-portal/)

> :warning: **IMPORTANT** Version 2 is currently under development and may contain bugs. It is currently not advised to use this version
in production. Use version [1.0.17](https://github.com/h44z/wg-portal/releases) instead.

A simple, web based configuration portal for [WireGuard](https://wireguard.com).
The portal uses the WireGuard [wgctrl](https://github.com/WireGuard/wgctrl-go) library to manage existing VPN
interfaces. This allows for seamless activation or deactivation of new users, without disturbing existing VPN
connections.

The configuration portal supports using a database (SQLite, MySQL, MsSQL or Postgres), OAuth or LDAP (Active Directory or OpenLDAP) as a user source for authentication and profile data.


## Features
 * Self-hosted - the whole application is a single binary
 * Responsive web UI written in Vue.JS
 * Automatically select IP from the network pool assigned to client
 * QR-Code for convenient mobile client configuration
 * Sent email to client with QR-code and client config
 * Enable / Disable clients seamlessly
 * Generation of wg-quick configuration file (`wgX.conf`) if required
 * User authentication (database, OAuth or LDAP) 
 * IPv6 ready
 * Docker ready
 * Can be used with existing WireGuard setups
 * Support for multiple WireGuard interfaces
 * Peer Expiry Feature
 * Handle route and DNS settings like wg-quick does
 * ~~REST API for management and client deployment~~ (coming soon)

![Screenshot](screenshot.png)


## Configuration
You can configure WireGuard Portal using a yaml configuration file.
The filepath of the yaml configuration file defaults to **config.yml** in the working directory of the executable.
It is possible to override the configuration filepath using the environment variable **WG_PORTAL_CONFIG**.
For example: `WG_PORTAL_CONFIG=/home/test/config.yml ./wg-portal-amd64`.

### Configuration Options
The following configuration options are available:

| configuration key         | parent key | default_value                              | description                                                                                                                          |
|---------------------------|------------|--------------------------------------------|--------------------------------------------------------------------------------------------------------------------------------------|
| admin_user                | core       | admin@wgportal.local                       | The administrator user. This user will be created as default admin if it does not yet exist.                                         |
| admin_password            | core       | wgportal                                   | The administrator password. If unchanged, a random password will be set on first startup.                                            |
| editable_keys             | core       | true                                       | Allow to edit key-pairs in the UI.                                                                                                   |
| create_default_peer       | core       | false                                      | If an LDAP user logs in for the first time, a new WireGuard peer will be created on the WG_DEFAULT_DEVICE if this option is enabled. |
| self_provisioning_allowed | core       | false                                      | Allow registered users to automatically create peers via their profile page.                                                         |
| import_existing           | core       | true                                       | Import existing WireGuard interfaces and peers into WireGuard Portal.                                                                |
| restore_state             | core       | true                                       | Restore the WireGuard interface state after WireGuard Portal has started.                                                            |
| log_level                 | advanced   | warn                                       | The loglevel, can be one of: trace, debug, info, warn, error.                                                                        |
| log_pretty                | advanced   | false                                      | Uses pretty, colorized log messages.                                                                                                 |
| log_json                  | advanced   | false                                      | Logs in JSON format.                                                                                                                 |
| ldap_sync_interval        | advanced   | 15m                                        | The time interval after which users will be synchronized from LDAP.                                                                  |
| start_listen_port         | advanced   | 51820                                      | The first port number that will be used as listening port for new interfaces.                                                        |
| start_cidr_v4             | advanced   | 10.11.12.0/24                              | The first IPv4 subnet that will be used for new interfaces.                                                                          |
| start_cidr_v6             | advanced   | fdfd:d3ad:c0de:1234::0/64                  | The first IPv6 subnet that will be used for new interfaces.                                                                          |
| use_ip_v6                 | advanced   | true                                       | Enable IPv6 support.                                                                                                                 |
| config_storage_path       | advanced   |                                            | If a wg-quick style configuration should be stored to the filesystem, specify a storage directory.                                   |
| expiry_check_interval     | advanced   | 15m                                        | The interval after which existing peers will be checked if they expired.                                                             |
| rule_prio_offset          | advanced   | 20000                                      | The default offset for ip route rule priorities.                                                                                     |
| route_table_offset        | advanced   | 20000                                      | The default offset for ip route table id's.                                                                                          |
| use_ping_checks           | statistics | true                                       | If enabled, peers will be pinged periodically to check if they are still connected.                                                  |
| ping_check_workers        | statistics | 10                                         | Number of parallel ping checks that will be executed.                                                                                |
| ping_unprivileged         | statistics | false                                      | If set to false, the ping checks will run without root permissions (BETA).                                                           |
| ping_check_interval       | statistics | 1m                                         | The interval time between two ping check runs.                                                                                       |
| data_collection_interval  | statistics | 10m                                        | The interval between the data collection cycles.                                                                                     |
| collect_interface_data    | statistics | true                                       | A flag to enable interface data collection like bytes sent and received.                                                             |
| collect_peer_data         | statistics | true                                       | A flag to enable peer data collection like bytes sent and received, last handshake and remote endpoint address.                      |
| collect_audit_data        | statistics | true                                       | If enabled, some events, like portal logins, will be logged to the database.                                                         |
| host                      | mail       | 127.0.0.1                                  | The mail-server address.                                                                                                             |
| port                      | mail       | 25                                         | The mail-server SMTP port.                                                                                                           |
| encryption                | mail       | none                                       | SMTP encryption type, allowed values: none, tls, starttls.                                                                           |
| cert_validation           | mail       | false                                      | Validate the mail server certificate (if encryption tls is used).                                                                    |
| username                  | mail       |                                            | The SMTP user name.                                                                                                                  |
| password                  | mail       |                                            | The SMTP password.                                                                                                                   |
| auth_type                 | mail       | plain                                      | SMTP authentication type, allowed values: plain, login, crammd5.                                                                     |
| from                      | mail       | Wireguard Portal <noreply@wireguard.local> | The address that is used to send mails.                                                                                              |
| link_only                 | mail       | false                                      | Only send links to WireGuard Portal instead of the full configuration.                                                               |
| callback_url_prefix       | auth       | /api/v0                                    | OAuth callback URL prefix. The full callback URL will look like: https://wg.portal.local/callback_url_prefix/provider_name/callback  |
| oidc                      | auth       | Empty Array - no providers configured      | A list of OpenID Connect providers. See auth/oidc properties to setup a new provider.                                                |
| oauth                     | auth       | Empty Array - no providers configured      | A list of plain OAuth providers. See auth/oauth properties to setup a new provider.                                                  |
| ldap                      | auth       | Empty Array - no providers configured      | A list of LDAP providers. See auth/ldap properties to setup a new provider.                                                          |
| provider_name             | auth/oidc  |                                            | A unique provider name. This name must be unique throughout all authentication providers (even other types).                         |
| display_name              | auth/oidc  |                                            | The display name is shown at the login page (the login button).                                                                      |
| base_url                  | auth/oidc  |                                            | The base_url is the URL identifier for the service. For example: "https://accounts.google.com".                                      |
| client_id                 | auth/oidc  |                                            | The OAuth client id.                                                                                                                 |
| client_secret             | auth/oidc  |                                            | The OAuth client secret.                                                                                                             |
| extra_scopes              | auth/oidc  |                                            | Extra scopes that should be used in the OpenID Connect authentication flow.                                                          |
| field_map                 | auth/oidc  |                                            | Mapping of user fields. Internal fields: user_identifier, email, firstname, lastname, phone, department and is_admin.                |
| registration_enabled      | auth/oidc  |                                            | If registration is enabled, new user accounts will created in WireGuard Portal.                                                      |
| provider_name             | auth/oauth |                                            | A unique provider name. This name must be unique throughout all authentication providers (even other types).                         |
| display_name              | auth/oauth |                                            | The display name is shown at the login page (the login button).                                                                      |
| base_url                  | auth/oauth |                                            | The base_url is the URL identifier for the service. For example: "https://accounts.google.com".                                      |
| client_id                 | auth/oauth |                                            | The OAuth client id.                                                                                                                 |
| client_secret             | auth/oauth |                                            | The OAuth client secret.                                                                                                             |
| auth_url                  | auth/oauth |                                            | The URL for the authentication endpoint.                                                                                             |
| token_url                 | auth/oauth |                                            | The URL for the token endpoint.                                                                                                      |
| redirect_url              | auth/oauth |                                            | The redirect URL.                                                                                                                    |
| user_info_url             | auth/oauth |                                            | The URL for the user information endpoint.                                                                                           |
| scopes                    | auth/oauth |                                            | OAuth scopes.                                                                                                                        |
| field_map                 | auth/oauth |                                            | Mapping of user fields. Internal fields: user_identifier, email, firstname, lastname, phone, department and is_admin.                |
| registration_enabled      | auth/oauth |                                            | If registration is enabled, new user accounts will created in WireGuard Portal.                                                      |
| url                       | auth/ldap  |                                            | The LDAP server url. For example: ldap://srv-ad01.company.local:389	                                                                 |
| start_tls                 | auth/ldap  |                                            | Use STARTTLS to encrypt LDAP requests.                                                                                               |
| cert_validation           | auth/ldap  |                                            | Validate the LDAP server certificate.                                                                                                |
| tls_certificate_path      | auth/ldap  |                                            | A path to the TLS certificate.                                                                                                       |
| tls_key_path              | auth/ldap  |                                            | A path to the TLS key.                                                                                                               |
| base_dn                   | auth/ldap  |                                            | The base DN for searching users. For example: DC=COMPANY,DC=LOCAL	                                                                   |
| bind_user                 | auth/ldap  |                                            | The bind user. For example: company\\ldap_wireguard	                                                                                 |
| bind_pass                 | auth/ldap  |                                            | The bind password.                                                                                                                   |
| field_map                 | auth/ldap  |                                            | Mapping of user fields. Internal fields: user_identifier, email, firstname, lastname, phone, department and memberof.                |
| login_filter              | auth/ldap  |                                            | LDAP filters for users that should be allowed to log in. {{login_identifier}} will be replaced with the login username.              |
| admin_group               | auth/ldap  |                                            | Users in this group are marked as administrators.                                                                                    |
| synchronize               | auth/ldap  |                                            | Periodically synchronize users (name, department, phone, status, ...) to the WireGuard Portal database.                              |
| disable_missing           | auth/ldap  |                                            | If synchronization is enabled, missing LDAP users will be disabled in WireGuard Portal.                                              |
| sync_filter               | auth/ldap  |                                            | LDAP filters for users that should be synchronized to WireGuard Portal.                                                              |
| registration_enabled      | auth/ldap  |                                            | If registration is enabled, new user accounts will created in WireGuard Portal.                                                      |
| debug                     | database   | false                                      | Debug database statements (log each statement).                                                                                      |
| slow_query_threshold      | database   |                                            | A threshold for slow database queries. If the threshold is exceeded, a warning message will be logged.                               |
| type                      | database   | sqlite                                     | The database type. Allowed values: sqlite, mssql, mysql or postgres.                                                                 |
| dsn                       | database   | sqlite.db                                  | The database DSN. For example: user:pass@tcp(1.2.3.4:3306)/dbname?charset=utf8mb4&parseTime=True&loc=Local                           |
| request_logging           | web        | false                                      | Log all HTTP requests.                                                                                                               |
| external_url              | web        | http://localhost:8888                      | The URL where a client can access WireGuard Portal.                                                                                  |
| listening_address         | web        | :8888                                      | The listening port of the web server.                                                                                                |
| session_identifier        | web        | wgPortalSession                            | The session identifier for the web frontend.                                                                                         |
| session_secret            | web        | very_secret                                | The session secret for the web frontend.                                                                                             |
| csrf_secret               | web        | extremely_secret                           | The CSRF secret.                                                                                                                     |
| site_title                | web        | WireGuard Portal                           | The title that is shown in the web frontend.                                                                                         |
| site_company_name         | web        | WireGuard Portal                           | The company name that is shown at the bottom of the web frontend.                                                                    |


## Upgrading from V1

> :warning: Before upgrading from V1, make sure that you have a backup of your currently working configuration files and database!

To start the upgrade process, start the wg-portal binary with the **-migrateFrom** parameter. 
The configuration (config.yml) for WireGuard Portal must be updated and valid before starting the upgrade.

To upgrade from a previous SQLite database, start wg-portal like:

```shell
./wg-portal-amd64 -migrateFrom=old_wg_portal.db
```

You can also specify the database type using the parameter **-migrateFromType**, supported types: mysql, mssql, postgres or sqlite.
For example:

```shell
./wg-portal-amd64 -migrateFromType=mysql -migrateFrom=user:pass@tcp(1.2.3.4:3306)/dbname?charset=utf8mb4&parseTime=True&loc=Local
```

The upgrade will transform the old, existing database and store the values in the new database specified in config.yml.
Ensure that the new database does not contain any data!


## V2 TODOs
 * Public REST API
 * Translations
 * Documentation
 * Audit UI


## What is out of scope
 * Automatic generation or application of any `iptables` or `nftables` rules.
 * Support for operating systems other than linux.
 * Automatic import of private keys of an existing WireGuard setup.


## Application stack

 * [wgctrl-go](https://github.com/WireGuard/wgctrl-go) and [netlink](https://github.com/vishvananda/netlink) for interface handling
 * [Gin](https://github.com/gin-gonic/gin), HTTP web framework written in Go
 * [Bootstrap](https://getbootstrap.com/), for the HTML templates
 * [Vue.JS](https://vuejs.org/), for the frontend


## License

 * MIT License. [MIT](LICENSE.txt) or https://opensource.org/licenses/MIT
