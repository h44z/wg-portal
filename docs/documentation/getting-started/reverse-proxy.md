## Reverse Proxy for HTTPS

For production deployments, always serve the WireGuard Portal over HTTPS. You have two options to secure your connection:


### Reverse Proxy

Let a front‐end proxy handle HTTPS for you. This also frees you from managing certificates manually and is therefore the preferred option.
You can use Nginx, Traefik, Caddy or any other proxy. 

Below is an example using a Docker Compose stack with [Traefik](https://traefik.io/traefik/). 
It exposes the WireGuard Portal on `https://wg.domain.com` and redirects initial HTTP traffic to HTTPS.

```yaml
services:
  reverse-proxy:
    image: traefik:v3.3
    restart: unless-stopped
    command:
      #- '--log.level=DEBUG'
      - '--providers.docker.endpoint=unix:///var/run/docker.sock'
      - '--providers.docker.exposedbydefault=false'
      - '--entrypoints.web.address=:80'
      - '--entrypoints.websecure.address=:443'
      - '--entrypoints.websecure.http3'
      - '--certificatesresolvers.letsencryptresolver.acme.httpchallenge=true'
      - '--certificatesresolvers.letsencryptresolver.acme.httpchallenge.entrypoint=web'
      - '--certificatesresolvers.letsencryptresolver.acme.email=your.email@domain.com'
      - '--certificatesresolvers.letsencryptresolver.acme.storage=/letsencrypt/acme.json'
      #- '--certificatesresolvers.letsencryptresolver.acme.caserver=https://acme-staging-v02.api.letsencrypt.org/directory'  # just for testing
    ports:
      - 80:80 # for HTTP
      - 443:443/tcp  # for HTTPS
      - 443:443/udp  # for HTTP/3
    volumes:
      - acme-certs:/letsencrypt
      - /var/run/docker.sock:/var/run/docker.sock:ro
    labels:
      - 'traefik.enable=true'
      # HTTP Catchall for redirecting HTTP -> HTTPS
      - 'traefik.http.routers.dashboard-catchall.rule=Host(`wg.domain.com`) && PathPrefix(`/`)'
      - 'traefik.http.routers.dashboard-catchall.entrypoints=web'
      - 'traefik.http.routers.dashboard-catchall.middlewares=redirect-to-https'
      - 'traefik.http.middlewares.redirect-to-https.redirectscheme.scheme=https'

  wg-portal:
    image: wgportal/wg-portal:v2
    container_name: wg-portal
    restart: unless-stopped
    logging:
      options:
        max-size: "10m"
        max-file: "3"
    cap_add:
      - NET_ADMIN
    ports:
      # host port : container port
      # WireGuard port, needs to match the port in wg-portal interface config (add one port mapping for each interface)
      - "51820:51820/udp"
      # Web UI port (only available on localhost, Traefik will handle the HTTPS)
      - "127.0.0.1:8888:8888/tcp"
    sysctls:
      - net.ipv4.conf.all.src_valid_mark=1
    volumes:
      # host path : container path
      - ./wg/data:/app/data
      - ./wg/config:/app/config
    labels:
      - 'traefik.enable=true'
      - 'traefik.http.routers.wgportal.rule=Host(`wg.domain.com`)'
      - 'traefik.http.routers.wgportal.entrypoints=websecure'
      - 'traefik.http.routers.wgportal.tls.certresolver=letsencryptresolver'
      - 'traefik.http.routers.wgportal.service=wgportal'
      - 'traefik.http.services.wgportal.loadbalancer.server.port=8888'

volumes:
  acme-certs:
```

The WireGuard Portal configuration must be updated accordingly so that the correct external URL is set for the web interface:

```yaml
web:
  external_url: https://wg.domain.com
```

### Built-in TLS

If you prefer to let WireGuard Portal handle TLS itself, you can use the built-in TLS support.
In your `config.yaml`, under the `web` section, point to your certificate and key files:

```yaml
web:
  cert_file: /path/to/your/fullchain.pem
  key_file:  /path/to/your/privkey.pem
```

The web server will then use these files to serve HTTPS traffic directly instead of HTTP.