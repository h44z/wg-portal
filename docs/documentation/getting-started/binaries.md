Starting from v2, each [release](https://github.com/h44z/wg-portal/releases) includes compiled binaries for supported platforms.
These binary versions can be manually downloaded and installed.

## Download

Make sure that you download the correct binary for your architecture. The available binaries are:

- `wg-portal_linux_amd64` - Linux x86_64
- `wg-portal_linux_arm64` - Linux ARM 64-bit
- `wg-portal_linux_arm_v7` - Linux ARM 32-bit

### Released versions

To download a specific version, replace `${WG_PORTAL_VERSION}` with the desired version (or set an environment variable). 
All official release versions can be found on the [GitHub Releases Page](https://github.com/h44z/wg-portal/releases).

With `curl`:

```shell
curl -L -o wg-portal https://github.com/h44z/wg-portal/releases/download/${WG_PORTAL_VERSION}/wg-portal_linux_amd64 
```

With `wget`:

```shell
wget -O wg-portal https://github.com/h44z/wg-portal/releases/download/${WG_PORTAL_VERSION}/wg-portal_linux_amd64
```

with `gh cli`:

```shell
gh release download ${WG_PORTAL_VERSION} --repo h44z/wg-portal --output wg-portal --pattern '*amd64'
```

The downloaded file will be named `wg-portal` and can be moved to a directory of your choice, see [Install](#install) for more information.

### Unreleased versions (master branch builds)

Unreleased versions can be fetched directly from the artifacts section of the [GitHub Workflow](https://github.com/h44z/wg-portal/actions/workflows/docker-publish.yml?query=branch%3Amaster).


## Install

The following command can be used to install the downloaded binary (`wg-portal`) to `/opt/wg-portal/wg-portal`. It ensures that the binary is executable.

```shell
sudo mkdir -p /opt/wg-portal
sudo install wg-portal /opt/wg-portal/
```

To handle tasks such as restarting the service or configuring automatic startup, it is recommended to use a process manager like [systemd](https://systemd.io/). 
Refer to [Systemd Service Setup](#systemd-service-setup) for instructions.

## Systemd Service Setup

> **Note:** To run WireGuard Portal as systemd service, you need to download the binary for your architecture beforehand.
> 
> The following examples assume that you downloaded the binary to `/opt/wg-portal/wg-portal`. 
> The configuration file is expected to be located at `/opt/wg-portal/config.yml`.

To run WireGuard Portal as a systemd service, you can create a service unit file. The easiest way to do this is by using `systemctl edit`:

```shell
sudo systemctl edit --force --full wg-portal.service
```

Paste the following content into the editor and adjust the variables to your needs:

```ini
[Unit]
Description=WireGuard Portal
ConditionPathExists=/opt/wg-portal/wg-portal
After=network.target

[Service]
Type=simple
User=root
Group=root
AmbientCapabilities=CAP_NET_ADMIN CAP_NET_RAW

Restart=on-failure
RestartSec=10

WorkingDirectory=/opt/wg-portal
Environment=WG_PORTAL_CONFIG=/opt/wg-portal/config.yml
ExecStart=/opt/wg-portal/wg-portal

[Install]
WantedBy=multi-user.target
```

Alternatively, you can create or modify the file manually in `/etc/systemd/system/wg-portal.service`. 
For systemd to pick up the changes, you need to reload the daemon:

```shell
sudo systemctl daemon-reload
```

After creating the service file, you can enable and start the service:

```shell
sudo systemctl enable --now wg-portal.service
```

To check status and log output, use: `sudo systemctl status wg-portal.service` or `sudo journalctl -u wg-portal.service`.
