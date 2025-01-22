Starting from v2, each [release](https://github.com/h44z/wg-portal/releases) includes compiled binaries for supported platforms.
These binary versions can be manually downloaded and installed.

## Download

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

## Install

```shell
sudo mkdir -p /opt/wg-portal
sudo install wg-portal /opt/wg-portal/
```

## Unreleased

Unreleased versions could be downloaded from
[GitHub Workflow](https://github.com/h44z/wg-portal/actions/workflows/docker-publish.yml?query=branch%3Amaster) artifacs also.
