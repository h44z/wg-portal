To build the application from source files, use the Makefile provided in the repository.

## Requirements

- [Git](https://git-scm.com/downloads)
- [Make](https://www.gnu.org/software/make/)
- [Go](https://go.dev/dl/): `>=1.24.0`
- [Node.js with npm](https://nodejs.org/en/download): `node>=18, npm>=9`

## Build

```shell
# Get source code
git clone https://github.com/h44z/wg-portal -b ${WG_PORTAL_VERSION:-master} --depth 1
cd wg-portal
# Build the frontend
make frontend
# Build the backend
make build
```

## Install

Compiled binary will be available in `./dist` directory. 

For installation instructions, check the [Binaries](./binaries.md) section.
