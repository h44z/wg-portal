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

The configuration portal currently supports using SQLite and MySQL as a user source for authentication and profile data.
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
 * REST API for management and client deployment

![Screenshot](screenshot.png)

## Setup
Make sure that your host system has at least one WireGuard interface (for example wg0) available.
If you did not start up a WireGuard interface yet, take a look at [wg-quick](https://manpages.debian.org/unstable/wireguard-tools/wg-quick.8.en.html) in order to get started.

### Docker
The easiest way to run WireGuard Portal is to use the Docker image provided.

HINT: the *latest* tag always refers to the master branch and might contain unstable or incompatible code!

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
If needed, please make sure to back up your files from ```/etc/wireguard```.
For a full list of configuration options take a look at the source file [internal/server/configuration.go](internal/server/configuration.go#L56).

### Standalone
For a standalone application, use the Makefile provided in the repository to build the application. Go version 1.16 or higher has to be installed to build WireGuard Portal.

```
make

# To build for arm architecture as well use:
make build-cross-plat
```

The compiled binary will be located in the dist folder.
A detailed description for using this software with a raspberry pi can be found in the [README-RASPBERRYPI.md](README-RASPBERRYPI.md).

## Configuration
You can configure WireGuard Portal using either environment variables or a yaml configuration file.
The filepath of the yaml configuration file defaults to **config.yml** in the working directory of the executable.
It is possible to override the configuration filepath using the environment variable **CONFIG_FILE**.
For example: `CONFIG_FILE=/home/test/config.yml ./wg-portal-amd64`.

### Configuration Options
The following configuration options are available:

| environment                | yaml                    | yaml_parent | default_value                                   | description                                                                                |
|----------------------------|-------------------------|-------------|-------------------------------------------------|-------------------------------------------------------------------------------------------|
| LISTENING_ADDRESS          | listeningAddress        | core        | :8123                                           | The address on which the web server is listening. Optional IP address and port, e.g.: 127.0.0.1:8080.                                                    |
| EXTERNAL_URL               | externalUrl             | core        | http://localhost:8123                           | The external URL where the web server is reachable. This link is used in emails that are created by the WireGuard Portal.                                |
| WEBSITE_TITLE              | title                   | core        | WireGuard VPN                                   | The website title.                                                                                     |
| COMPANY_NAME               | company                 | core        | WireGuard Portal                                | The company name (for branding).                                                                                          |
| MAIL_FROM                  | mailFrom                | core        | WireGuard VPN <noreply@company.com>             | The email address from which emails are sent.                                                                                      |
| LOGO_URL                   | logoUrl                 | core        | /img/header-logo.png                            | The logo displayed in the page's header.                                                                                    |
| ADMIN_USER                 | adminUser               | core        | admin@wgportal.local                            | The administrator user. Must be a valid email address.                                                                                   |
| ADMIN_PASS                 | adminPass               | core        | wgportal                                        | The administrator password. If unchanged, a random password will be set on first startup.                                                              |
| EDITABLE_KEYS              | editableKeys            | core        | true                                            | Allow to edit key-pairs in the UI.                                                                                        |
| CREATE_DEFAULT_PEER        | createDefaultPeer       | core        | false                                           | If an LDAP user logs in for the first time, a new WireGuard peer will be created on the WG_DEFAULT_DEVICE if this option is enabled.                   |
| SELF_PROVISIONING          | selfProvisioning        | core        | false                                           | Allow registered users to automatically create peers via the RESTful API.                                                                               |
| WG_EXPORTER_FRIENDLY_NAMES | wgExporterFriendlyNames | core        | false                                           | Enable integration with [prometheus_wireguard_exporter friendly name](https://github.com/MindFlavor/prometheus_wireguard_exporter#friendly-tags). |
| LDAP_ENABLED               | ldapEnabled             | core        | false                                           | Enable or disable the LDAP backend.                                                                                   |
| SESSION_SECRET             | sessionSecret           | core        | secret                                          | Use a custom secret to encrypt session data.                                                                                      |
| DATABASE_TYPE              | typ                     | database    | sqlite                                          | Either mysql or sqlite.                                                                                    |
| DATABASE_HOST              | host                    | database    |                                                 | The mysql server address.                                                                                   |
| DATABASE_PORT              | port                    | database    |                                                 | The mysql server port.                                                                                      |
| DATABASE_NAME              | database                | database    | data/wg_portal.db                               | For sqlite database: the database file-path, otherwise the database name.                                                                             |
| DATABASE_USERNAME          | user                    | database    |                                                 | The mysql user.                                                                                      |
| DATABASE_PASSWORD          | password                | database    |                                                 | The mysql password.                                                                                  |
| EMAIL_HOST                 | host                    | email       | 127.0.0.1                                       | The email server address.                                                                                   |
| EMAIL_PORT                 | port                    | email       | 25                                              | The email server port.                                                                                      |
| EMAIL_TLS                  | tls                     | email       | false                                           | Use STARTTLS. DEPRECATED: use EMAIL_ENCRYPTION instead.                                                                                   |
| EMAIL_ENCRYPTION           | encryption              | email       | none                                            | Either none, tls or starttls.                                                                                  |
| EMAIL_CERT_VALIDATION      | certcheck               | email       | false                                           | Validate the email server certificate.                                                                               |
| EMAIL_USERNAME             | user                    | email       |                                                 | An optional username for SMTP authentication.                                                                            |
| EMAIL_PASSWORD             | pass                    | email       |                                                 | An optional password for SMTP authentication.                                                                            |
| EMAIL_AUTHTYPE             | auth                    | email       | plain                                           | Either plain, login or crammd5. If username and password are empty, this value is ignored.                                                              |
| WG_DEVICES                 | devices                 | wg          | wg0                                             | A comma separated list of WireGuard devices.                                                                                   |
| WG_DEFAULT_DEVICE          | defaultDevice           | wg          | wg0                                             | This device is used for auto-created peers (if CREATE_DEFAULT_PEER is enabled).                                                           |
| WG_CONFIG_PATH             | configDirectory         | wg          | /etc/wireguard                                  | If set, interface configuration updates will be written to this path, filename: <devicename>.conf.                                                    |
| MANAGE_IPS                 | manageIPAddresses       | wg          | true                                            | Handle IP address setup of interface, only available on linux.                                                                                     |
| LDAP_URL                   | url                     | ldap        | ldap://srv-ad01.company.local:389               | The LDAP server url.                                                                                       |
| LDAP_STARTTLS              | startTLS                | ldap        | true                                            | Use STARTTLS.                                                                                  |
| LDAP_CERT_VALIDATION       | certcheck               | ldap        | false                                           | Validate the LDAP server certificate.                                                                               |
| LDAP_BASEDN                | dn                      | ldap        | DC=COMPANY,DC=LOCAL                             | The base DN for searching users.                                                                                     |
| LDAP_USER                  | user                    | ldap        | company\\\\ldap_wireguard                       | The bind user.                                                                                      |
| LDAP_PASSWORD              | pass                    | ldap        | SuperSecret                                     | The bind password.                                                                                  |
| LDAP_LOGIN_FILTER          | loginFilter             | ldap        | (&(objectClass=organizationalPerson)(mail={{login_identifier}})(!userAccountControl:1.2.840.113556.1.4.803:=2)) | {{login_identifier}} will be replaced with the login email address.                      |
| LDAP_SYNC_FILTER           | syncFilter              | ldap        | (&(objectClass=organizationalPerson)(!userAccountControl:1.2.840.113556.1.4.803:=2)(mail=*))                    | The filter string for the LDAP synchronization service.                                  |
| LDAP_ADMIN_GROUP           | adminGroup              | ldap        | CN=WireGuardAdmins,OU=_O_IT,DC=COMPANY,DC=LOCAL | Users in this group are marked as administrators.                                                                            |
| LDAP_ATTR_EMAIL            | attrEmail               | ldap        | mail                                            | User email attribute.                                                                                 |
| LDAP_ATTR_FIRSTNAME        | attrFirstname           | ldap        | givenName                                       | User firstname attribute.                                                                                 |
| LDAP_ATTR_LASTNAME         | attrLastname            | ldap        | sn                                              | User lastname attribute.                                                                                 |
| LDAP_ATTR_PHONE            | attrPhone               | ldap        | telephoneNumber                                 | User phone number attribute.                                                                                 |
| LDAP_ATTR_GROUPS           | attrGroups              | ldap        | memberOf                                        | User groups attribute.                                                                                 |
| LDAP_CERT_CONN             | ldapCertConn            | ldap        | false                                           | Allow connection with certificate against LDAP server without user/password                            |
| LDAPTLS_CERT               | ldapTlsCert             | ldap        |                                                 | The LDAP cert's path                                                                                   |
| LDAPTLS_KEY                | ldapTlsKey              | ldap        |                                                 | The LDAP key's path                                                                                    |
| LOG_LEVEL                  |                         |             | debug                                           | Specify log level, one of: trace, debug, info, off.                                                                                       |
| LOG_JSON                   |                         |             | false                                           | Format log output as JSON.                                                                                      |
| LOG_COLOR                  |                         |             | true                                            | Colorize log output.                                                                                    |
| CONFIG_FILE                |                         |             | config.yml                                      | The config file path.                                                                                      |

### Sample yaml configuration
config.yml:
```yaml
core:
  listeningAddress: :8123
  externalUrl: https://wg-test.test.com
  adminUser: test@test.com
  adminPass: test
  editableKeys: true
  createDefaultPeer: false
  ldapEnabled: true
  mailFrom: WireGuard VPN <noreply@test.com>
ldap:
  url: ldap://10.10.10.10:389
  dn: DC=test,DC=test
  startTLS: false
  user: wireguard@test.test
  pass: test
  adminGroup: CN=WireGuardAdmins,CN=Users,DC=test,DC=test
database:
  typ: sqlite
  database: data/wg_portal.db
email:
  host: smtp.gmail.com
  port: 587
  tls: true
  user: test@gmail.com
  pass: topsecret
wg:
  devices:
    - wg0
    - wg1
  defaultDevice: wg0
  configDirectory: /etc/wireguard
  manageIPAddresses: true
```

### RESTful API
WireGuard Portal offers a RESTful API to interact with.
The API is documented using OpenAPI 2.0, the Swagger UI can be found
under the URL `http://<your wg-portal ip/domain>/swagger/index.html?displayOperationId=true`.

The [API's unittesting](tests/test_API.py) may serve as an example how to make use of the API with python3 & pyswagger.

## What is out of scope
 * Creating or removing WireGuard (wgX) interfaces.
 * Generation or application of any `iptables` or `nftables` rules.
 * Setting up or changing IP-addresses of the WireGuard interface on operating systems other than linux.
 * Importing private keys of an existing WireGuard setup.

## Application stack

 * [Gin, HTTP web framework written in Go](https://github.com/gin-gonic/gin)
 * [go-template, data-driven templates for generating textual output](https://golang.org/pkg/text/template/)
 * [Bootstrap, for the HTML templates](https://getbootstrap.com/)
 * [JQuery, for some nice JavaScript effects ;)](https://jquery.com/)

## License

 * MIT License. [MIT](LICENSE.txt) or https://opensource.org/licenses/MIT


This project was inspired by [wg-gen-web](https://github.com/vx3r/wg-gen-web).
