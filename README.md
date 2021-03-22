# WireGuard Portal

[![Build Status](https://travis-ci.com/h44z/wg-portal.svg?token=q4pSqaqT58Jzpxdx62xk&branch=master)](https://travis-ci.com/h44z/wg-portal)
[![License: MIT](https://img.shields.io/badge/license-MIT-green.svg)](https://opensource.org/licenses/MIT)
![GitHub last commit](https://img.shields.io/github/last-commit/h44z/wg-portal)
[![Go Report Card](https://goreportcard.com/badge/github.com/h44z/wg-portal)](https://goreportcard.com/report/github.com/h44z/wg-portal)
![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/h44z/wg-portal)
![GitHub code size in bytes](https://img.shields.io/github/languages/code-size/h44z/wg-portal)
[![Docker Pulls](https://img.shields.io/docker/pulls/h44z/wg-portal.svg)](https://hub.docker.com/r/h44z/wg-portal/)

A simple, web based configuration portal for [WireGuard](https://wireguard.com). 
The portal uses the WireGuard [wgctrl](https://github.com/WireGuard/wgctrl-go) library to manage the VPN 
interface. This allows for seamless activation or deactivation of new users, without disturbing existing VPN 
connections.

The configuration portal currently supports using SQLite, MySQL as a user source for authentication and profile data.
It also supports LDAP (Active Directory or OpenLDAP) as authentication provider.

## Features
 * Self-hosted and web based
 * Automatically select IP from the network pool assigned to client
 * QR-Code for convenient mobile client configuration
 * Sent email to client with QR-code and client config
 * Enable / Disable clients seamlessly
 * Generation of `wgX.conf` after any modification
 * IPv6 ready
 * User authentication (SQLite/MySQL and LDAP)
 * Dockerized
 * Responsive template
 * One single binary
 * Can be used with existing WireGuard setups
 * Support for multiple WireGuard interfaces
 
![Screenshot](screenshot.png)

## Setup

### Docker
The easiest way to run WireGuard Portal is to use the Docker image provided.

Docker Compose snippet with some sample configuration values:
```
version: '3.6'
services:
  wg-portal:
    image: h44z/wg-portal:latest
    container_name: wg-portal
    restart: unless-stopped
    cap_add:
      - NET_ADMIN
    network_mode: "host"
    volumes:
      - /etc/wireguard:/etc/wireguard
      - ./data:/app/data
    ports:
      - '8123:8123'
    environment:
      # WireGuard Settings
      - WG_DEVICES=wg0
      - WG_DEFAULT_DEVICE=wg0
      - WG_CONFIG_PATH=/etc/wireguard
      # Core Settings
      - EXTERNAL_URL=https://vpn.company.com
      - WEBSITE_TITLE=WireGuard VPN
      - COMPANY_NAME=Your Company Name
      - ADMIN_USER=admin@domain.com
      - ADMIN_PASS=supersecret
      # Mail Settings
      - MAIL_FROM=WireGuard VPN <noreply+wireguard@company.com>
      - EMAIL_HOST=10.10.10.10
      - EMAIL_PORT=25
      # LDAP Settings
      - LDAP_ENABLED=true
      - LDAP_URL=ldap://srv-ad01.company.local:389
      - LDAP_BASEDN=DC=COMPANY,DC=LOCAL
      - LDAP_USER=ldap_wireguard@company.local
      - LDAP_PASSWORD=supersecretldappassword
      - LDAP_ADMIN_GROUP=CN=WireGuardAdmins,OU=Users,DC=COMPANY,DC=LOCAL
```
Please note that mapping ```/etc/wireguard``` to ```/etc/wireguard``` inside the docker, will erase your host's current configuration.
If needed, please make sure to backup your files from ```/etc/wireguard```.
For a full list of configuration options take a look at the source file [internal/server/configuration.go](internal/server/configuration.go#L56).

### Standalone
For a standalone application, use the Makefile provided in the repository to build the application.

```
make

# To build for arm architecture as well use:
make build-cross-plat
```

The compiled binary will be located in the dist folder.
A detailed description for using this software with a raspberry pi can be found in the [README-RASPBERRYPI.md](README-RASPBERRYPI.md).

## What is out of scope

 * Generation or application of any `iptables` or `nftables` rules
 * Setting up or changing IP-addresses of the WireGuard interface on operating systems other than linux
 * Importing private keys of an existing WireGuard setup
 
## Application stack

 * [Gin, HTTP web framework written in Go](https://github.com/gin-gonic/gin)
 * [go-template, data-driven templates for generating textual output](https://golang.org/pkg/text/template/)
 * [Bootstrap, for the HTML templates](https://getbootstrap.com/)
 * [JQuery, for some nice JavaScript effects ;)](https://jquery.com/)

## License

 * MIT License. [MIT](LICENSE.txt) or https://opensource.org/licenses/MIT
 

This project was inspired by [wg-gen-web](https://github.com/vx3r/wg-gen-web).