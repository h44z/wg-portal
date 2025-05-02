## Image Usage

The WireGuard Portal Docker image is available on both [Docker Hub](https://hub.docker.com/r/wgportal/wg-portal) and [GitHub Container Registry](https://github.com/h44z/wg-portal/pkgs/container/wg-portal).
It is built on the official Alpine Linux base image and comes pre-packaged with all necessary WireGuard dependencies.

This container allows you to establish WireGuard VPN connections without relying on a host system that supports WireGuard or using the `linuxserver/wireguard` Docker image.

The recommended method for deploying WireGuard Portal is via Docker Compose for ease of configuration and management.

A sample docker-compose.yml:

```yaml
--8<-- "docker-compose.yml::17"
```

By default, the webserver is listening on port **8888**.

Volumes for `/app/data` and `/app/config` should be used ensure data persistence across container restarts.

## WireGuard Interface Handling

WireGuard Portal supports managing WireGuard interfaces through three distinct deployment methods, providing flexibility based on your system architecture and operational preferences:

 - **Directly on the host system**: 
   WireGuard Portal can control WireGuard interfaces natively on the host, without using containers. 
   This setup is ideal for environments where direct access to system networking is preferred.
   To use this method, you need to set the network mode to `host` in your docker-compose.yml file.
   ```yaml
   services:
     wg-portal:
       ...
       network_mode: "host"
       ...
   ```

 - **Within the WireGuard Portal Docker container**: 
   WireGuard interfaces can be managed directly from within the WireGuard Portal container itself.
   This is the recommended approach when running WireGuard Portal via Docker, as it encapsulates all functionality in a single, portable container without requiring a separate WireGuard host or image.
   The sample docker-compose.yml file provided above is configured for this method.

 - **Via a separate Docker container**: 
   WireGuard Portal can interface with and control WireGuard running in another Docker container, such as the [linuxserver/wireguard](https://docs.linuxserver.io/images/docker-wireguard/) image.
   This method is useful in setups that already use `linuxserver/wireguard` or where you want to isolate the VPN backend from the portal frontend.
   For this, you need to set the network mode to `service:wireguard` in your docker-compose.yml file, `wireguard` is the service name of your WireGuard container.
   ```yaml
   services:
     wg-portal:
       image: wgportal/wg-portal:latest
       container_name: wg-portal
       ...
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
         - "8888:8888/tcp" # Noticed that the port of the web UI is exposed in the wireguard container.
       volumes:
         - ./wg/etc:/config/wg_confs # We share the configuration (wgx.conf) between wg-portal and wireguard
       sysctls:
         - net.ipv4.conf.all.src_valid_mark=1
   ```
   As the `linuxserver/wireguard` image uses _wg-quick_ to manage the interfaces, you need to have at least the following configuration set for WireGuard Portal:
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

## Image Versioning

All images are hosted on Docker Hub at [https://hub.docker.com/r/wgportal/wg-portal](https://hub.docker.com/r/wgportal/wg-portal) or in the [GitHub Container Registry](https://github.com/h44z/wg-portal/pkgs/container/wg-portal).
There are three types of tags in the repository:

#### Semantic versioned tags

For example, `2.0.0-rc.1` or `v2.0.0-rc.1`.

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

You can configure WireGuard Portal using a YAML configuration file.
The filepath of the YAML configuration file defaults to `/app/config/config.yml`.
It is possible to override the configuration filepath using the environment variable **WG_PORTAL_CONFIG**.

By default, WireGuard Portal uses an SQLite database. The database is stored in `/app/data/sqlite.db`.

You should mount those directories as a volume:

- `/app/data`
- `/app/config`

A detailed description of the configuration options can be found [here](../configuration/overview.md).

If you want to access configuration files in wg-quick format, you can mount the `/etc/wireguard` directory to a location of your choice.
Also enable the `config_storage_path` option in the configuration file:
```yaml
advanced:
  config_storage_path: /etc/wireguard
```
