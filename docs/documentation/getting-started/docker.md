## Image Usage

The preferred way to start WireGuard Portal as Docker container is to use Docker Compose.

A sample docker-compose.yml:

```yaml
--8<-- "docker-compose.yml::17"
```

By default, the webserver is listening on port **8888**.

Volumes for `/app/data` and `/app/config` should be used ensure data persistence across container restarts.

## Image Versioning

All images are hosted on Docker Hub at [https://hub.docker.com/r/wgportal/wg-portal](https://hub.docker.com/r/wgportal/wg-portal).
There are three types of tags in the repository:

#### Semantic versioned tags

For example, `1.0.19`.

These are official releases of WireGuard Portal. They correspond to the GitHub tags that we make, and you can see the release notes for them here: [https://github.com/h44z/wg-portal/releases](https://github.com/h44z/wg-portal/releases).

Once these tags show up in this repository, they will never change.

For production deployments of WireGuard Portal, we strongly recommend using one of these tags, e.g. **wgportal/wg-portal:1.0.19**, instead of the latest or canary tags.

If you only want to stay at the same major or major+minor version, use either `v[MAJOR]` or `[MAJOR].[MINOR]` tags. For example `v1` or `1.0`.

Version **1** is currently **stable**, version **2** is in **development**.

#### latest

This is the most recent build to master! It changes a lot and is very unstable.

We recommend that you don't use it except for development purposes.

#### Branch tags

For each commit in the master and the stable branch, a corresponding Docker image is build. These images use the `master` or `stable` tags.

## Configuration

You can configure WireGuard Portal using a yaml configuration file.
The filepath of the yaml configuration file defaults to `/app/config/config.yml`.
It is possible to override the configuration filepath using the environment variable **WG_PORTAL_CONFIG**.

By default, WireGuard Portal uses a SQLite database. The database is stored in `/app/data/sqlite.db`.

You should mount those directories as a volume:

- /app/data
- /app/config

A detailed description of the configuration options can be found [here](../configuration/overview.md).

## Running WireGuard inside Docker

Modern Linux distributions ship with a kernel that supports WireGuard out of the box.
This means that you can run WireGuard directly on the host system without the need for a Docker container.
WireGuard Portal can then manage the WireGuard interfaces directly on the host.

If you still want to run WireGuard inside a Docker container, you can use the following example docker-compose.yml:

```yaml
services:
  wg-portal:
    image: wgportal/wg-portal:latest
    container_name: wg-portal
    restart: unless-stopped
    logging:
      options:
        max-size: "10m"
        max-file: "3"
    cap_add:
      - NET_ADMIN
    network_mode: "service:wireguard" # So we ensure to stay on the same network as the wireguard container.
    volumes:
      - ./wg/etc:/etc/wireguard
      - ./wg/data:/app/data
      - ./wg/config:/app/config

  wireguard:
      image: lscr.io/linuxserver/wireguard:latest
      container_name: wireguard
      restart: unless-stopped
      cap_add:
        - NET_ADMIN
      ports:
        - "51820:51820/udp" # WireGuard port, needs to match the port in wg-portal interface config
        - "127.0.0.1:8888:8888" # Noticed that the port of the web UI is exposed in the wireguard container.
      volumes:
        - ./wg/etc:/config/wg_confs # We share the configuration (wgx.conf) between wg-portal and wireguard
      sysctls:
        - net.ipv4.conf.all.src_valid_mark=1
```

For this to work, you need to have at least the following configuration set in your WireGuard Portal config:

```yaml
core:
  # The WireGuard container uses wg-quick to manage the WireGuard interfaces - this conflicts with WireGuard Portal during startup.
  # To avoid this, we need to set the restore_state option to false so that wg-quick can create the interfaces.
  restore_state: false
  # Usually, there are no existing interfaces in the WireGuard container, so we can set this to false.
  import_existing: false
advanced:
  # WireGuard Portal needs to export the WireGuard configuration as wg-quick config files so that the WireGuard container can use them.
  config_storage_path: /etc/wireguard/
```

Also make sure that you restart the WireGuard container after you create or delete an interface in WireGuard Portal.