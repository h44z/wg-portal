**WireGuard Portal** is a simple, web based configuration portal for [WireGuard](https://wireguard.com).
The portal uses the WireGuard [wgctrl](https://github.com/WireGuard/wgctrl-go) library to manage existing VPN
interfaces. This allows for seamless activation or deactivation of new users, without disturbing existing VPN
connections.

The configuration portal supports using a database (SQLite, MySQL, MsSQL or Postgres), OAuth or LDAP 
(Active Directory or OpenLDAP) as a user source for authentication and profile data.

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

## Quick-Start

The easiest way to get started is to use the provided [Docker image](./getting-started/docker.md).

