Starting from v2, each [release](https://github.com/fedor-git/wg-portal-2/releases) includes compiled binaries for supported platforms.
These binary versions can be manually downloaded and installed.

## Download

Make sure that you download the correct binary for your architecture. The available binaries are:

- `wg-portal_linux_amd64` - Linux x86_64
- `wg-portal_linux_arm64` - Linux ARM 64-bit
- `wg-portal_linux_arm_v7` - Linux ARM 32-bit

With `curl`:

```shell
curl -L -o wg-portal https://github.com/fedor-git/wg-portal-2/releases/download/${WG_PORTAL_VERSION}/wg-portal_linux_amd64 
```

With `wget`:

```shell
wget -O wg-portal https://github.com/fedor-git/wg-portal-2/releases/download/${WG_PORTAL_VERSION}/wg-portal_linux_amd64
```

with `gh cli`:

```shell
gh release download ${WG_PORTAL_VERSION} --repo fedor-git/wg-portal-2 --output wg-portal --pattern '*amd64'
```



## Install

```shell
sudo mkdir -p /opt/wg-portal
sudo install wg-portal /opt/wg-portal/
```

## Unreleased versions (master branch builds)

Unreleased versions can be fetched directly from the artifacts section of the [GitHub Workflow](https://github.com/fedor-git/wg-portal-2/actions/workflows/docker-publish.yml?query=branch%3Amaster).

