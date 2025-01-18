## Image Usage

The preferred way to start WireGuard Portal as Docker container is to use Docker Compose.

A sample docker-compose.yml:

```yaml
version: '3.6'
services:
  wg-portal:
    image: wgportal/wg-portal:latest
    restart: unless-stopped
    cap_add:
      - NET_ADMIN
    network_mode: "host"
    ports:
      - "8888:8888"
    volumes:
      - /etc/wireguard:/etc/wireguard
      - ./data:/app/data
      - ./config:/app/config
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
