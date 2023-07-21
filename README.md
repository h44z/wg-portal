# WireGuard Portal

[![Build Status](https://travis-ci.com/h44z/wg-portal.svg?token=q4pSqaqT58Jzpxdx62xk&branch=master)](https://travis-ci.com/h44z/wg-portal)
[![License: MIT](https://img.shields.io/badge/license-MIT-green.svg)](https://opensource.org/licenses/MIT)
![GitHub last commit](https://img.shields.io/github/last-commit/h44z/wg-portal)
[![Go Report Card](https://goreportcard.com/badge/github.com/h44z/wg-portal)](https://goreportcard.com/report/github.com/h44z/wg-portal)
![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/h44z/wg-portal)
![GitHub code size in bytes](https://img.shields.io/github/languages/code-size/h44z/wg-portal)
[![Docker Pulls](https://img.shields.io/docker/pulls/h44z/wg-portal.svg)](https://hub.docker.com/r/h44z/wg-portal/)

A simple, web based configuration portal for [WireGuard](https://wireguard.com).
The portal uses the WireGuard [wgctrl](https://github.com/WireGuard/wgctrl-go) library to manage existing VPN
interfaces. This allows for seamless activation or deactivation of new users, without disturbing existing VPN
connections.

The configuration portal supports using a database (SQLite, MySQL, MsSQL or Postgres), OAuth or LDAP (Active Directory or OpenLDAP) as a user source for authentication and profile data.

## Features
 * Self-hosted and web based
 * Automatically select IP from the network pool assigned to client
 * QR-Code for convenient mobile client configuration
 * Sent email to client with QR-code and client config
 * Enable / Disable clients seamlessly
 * Generation of `wgX.conf` if required
 * IPv6 ready
 * User authentication (database, OAuth or LDAP)
 * Dockerized
 * Responsive web UI written in Vue.JS
 * One single binary
 * Can be used with existing WireGuard setups
 * Support for multiple WireGuard interfaces
 * Peer Expiry Feature
 * REST API for management and client deployment (coming soon)

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
| ldap_sync_interval        | advanced   | 15m                                        |                                                                                                                                      |
| start_listen_port         | advanced   | 51820                                      |                                                                                                                                      |
| start_cidr_v4             | advanced   | 10.11.12.0/24                              |                                                                                                                                      |
| start_cidr_v6             | advanced   | fdfd:d3ad:c0de:1234::0/64                  |                                                                                                                                      |
| use_ip_v6                 | advanced   | true                                       |                                                                                                                                      |
| config_storage_path       | advanced   |                                            |                                                                                                                                      |
| expiry_check_interval     | advanced   | 15m                                        |                                                                                                                                      |
| use_ping_checks           | statistics | true                                       |                                                                                                                                      |
| ping_check_workers        | statistics | 10                                         |                                                                                                                                      |
| ping_unprivileged         | statistics | false                                      |                                                                                                                                      |
| ping_check_interval       | statistics | 1m                                         |                                                                                                                                      |
| data_collection_interval  | statistics | 10m                                        |                                                                                                                                      |
| collect_interface_data    | statistics | true                                       |                                                                                                                                      |
| collect_peer_data         | statistics | true                                       |                                                                                                                                      |
| collect_audit_data        | statistics | true                                       |                                                                                                                                      |
| host                      | mail       | 127.0.0.1                                  |                                                                                                                                      |
| port                      | mail       | 25                                         |                                                                                                                                      |
| encryption                | mail       | none                                       |                                                                                                                                      |
| cert_validation           | mail       | false                                      |                                                                                                                                      |
| username                  | mail       |                                            |                                                                                                                                      |
| password                  | mail       |                                            |                                                                                                                                      |
| auth_type                 | mail       | plain                                      |                                                                                                                                      |
| from                      | mail       | Wireguard Portal <noreply@wireguard.local> |                                                                                                                                      |
| link_only                 | mail       | false                                      |                                                                                                                                      |
| callback_url_prefix       | auth       | /api/v0                                    |                                                                                                                                      |
| oidc                      | auth       | Empty Array - no providers configured      |                                                                                                                                      |
| oauth                     | auth       | Empty Array - no providers configured      |                                                                                                                                      |
| ldap                      | auth       | Empty Array - no providers configured      |                                                                                                                                      |
| provider_name             | auth/oidc  |                                            |                                                                                                                                      |
| display_name              | auth/oidc  |                                            |                                                                                                                                      |
| base_url                  | auth/oidc  |                                            |                                                                                                                                      |
| client_id                 | auth/oidc  |                                            |                                                                                                                                      |
| client_secret             | auth/oidc  |                                            |                                                                                                                                      |
| extra_scopes              | auth/oidc  |                                            |                                                                                                                                      |
| field_map                 | auth/oidc  |                                            |                                                                                                                                      |
| registration_enabled      | auth/oidc  |                                            |                                                                                                                                      |
| provider_name             | auth/oidc  |                                            |                                                                                                                                      |
| display_name              | auth/oauth |                                            |                                                                                                                                      |
| base_url                  | auth/oauth |                                            |                                                                                                                                      |
| client_id                 | auth/oauth |                                            |                                                                                                                                      |
| client_secret             | auth/oauth |                                            |                                                                                                                                      |
| auth_url                  | auth/oauth |                                            |                                                                                                                                      |
| token_url                 | auth/oauth |                                            |                                                                                                                                      |
| redirect_url              | auth/oauth |                                            |                                                                                                                                      |
| user_info_url             | auth/oauth |                                            |                                                                                                                                      |
| scopes                    | auth/oauth |                                            |                                                                                                                                      |
| field_map                 | auth/oauth |                                            |                                                                                                                                      |
| registration_enabled      | auth/oauth |                                            |                                                                                                                                      |
| url                       | auth/ldap  |                                            |                                                                                                                                      |
| start_tls                 | auth/ldap  |                                            |                                                                                                                                      |
| cert_validation           | auth/ldap  |                                            |                                                                                                                                      |
| tls_certificate_path      | auth/ldap  |                                            |                                                                                                                                      |
| tls_key_path              | auth/ldap  |                                            |                                                                                                                                      |
| base_dn                   | auth/ldap  |                                            |                                                                                                                                      |
| bind_user                 | auth/ldap  |                                            |                                                                                                                                      |
| bind_pass                 | auth/ldap  |                                            |                                                                                                                                      |
| field_map                 | auth/ldap  |                                            |                                                                                                                                      |
| login_filter              | auth/ldap  |                                            |                                                                                                                                      |
| admin_group               | auth/ldap  |                                            |                                                                                                                                      |
| synchronize               | auth/ldap  |                                            |                                                                                                                                      |
| disable_missing           | auth/ldap  |                                            |                                                                                                                                      |
| sync_filter               | auth/ldap  |                                            |                                                                                                                                      |
| registration_enabled      | auth/ldap  |                                            |                                                                                                                                      |
| debug                     | database   | false                                      |                                                                                                                                      |
| slow_query_threshold      | database   |                                            |                                                                                                                                      |
| type                      | database   | sqlite                                     |                                                                                                                                      |
| dsn                       | database   | sqlite.db                                  |                                                                                                                                      |
| request_logging           | web        | false                                      |                                                                                                                                      |
| external_url              | web        | http://localhost:8888                      |                                                                                                                                      |
| listening_address         | web        | :8888                                      |                                                                                                                                      |
| session_identifier        | web        | wgPortalSession                            |                                                                                                                                      |
| session_secret            | web        | very_secret                                |                                                                                                                                      |
| csrf_secret               | web        | extremely_secret                           |                                                                                                                                      |
| site_title                | web        | WireGuard Portal                           |                                                                                                                                      |
| site_company_name         | web        | WireGuard Portal                           |                                                                                                                                      |

## What is out of scope
 * Generation or application of any `iptables` or `nftables` rules.
 * Setting up or changing IP-addresses of the WireGuard interface on operating systems other than linux.
 * Importing private keys of an existing WireGuard setup.

## Application stack

 * [Gin, HTTP web framework written in Go](https://github.com/gin-gonic/gin)
 * [Bootstrap, for the HTML templates](https://getbootstrap.com/)
 * [Vue.JS, for the frontend](hhttps://vuejs.org/)

## License

 * MIT License. [MIT](LICENSE.txt) or https://opensource.org/licenses/MIT
