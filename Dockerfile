# Dockerfile References: https://docs.docker.com/engine/reference/builder/
# This dockerfile uses a multi-stage build system to reduce the image footprint.

######
# Build frontend
######
FROM --platform=${BUILDPLATFORM} node:lts-alpine AS frontend
# Set the working directory
WORKDIR /build
# Download dependencies
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
# Set dist output directory
ENV DIST_OUT_DIR="dist"
# Copy the sources to the working directory
COPY frontend .
# Build the frontend
RUN npm run build

######
# Build backend
######
FROM --platform=${BUILDPLATFORM} golang:1.27-alpine AS builder
# Set the working directory
WORKDIR /build
# Download dependencies
COPY go.mod go.sum ./
RUN go mod download
# Copy the sources to the working directory
COPY ./cmd ./cmd
COPY ./internal ./internal
# Copy the frontend build result
COPY --from=frontend /build/dist/ ./internal/app/api/core/frontend-dist/
# Set the build version from arguments
ARG BUILD_VERSION
# Split to cross-platform build
ARG TARGETARCH
# Build the application
RUN CGO_ENABLED=0 GOARCH=${TARGETARCH} go build -o /build/dist/wg-portal \
  -ldflags "-w -s -extldflags '-static' -X 'github.com/h44z/wg-portal/internal.Version=${BUILD_VERSION}'" \
  -tags netgo \
  cmd/wg-portal/main.go

######
# Export binaries
######
FROM scratch AS binaries
COPY --from=builder /build/dist/wg-portal /

######
# Build the optional official AmneziaWG tools for the target architecture.
# These are diagnostic tools; the portal's native backend does not need them.
# The kernel module belongs on the host and is not built into this image.
######
FROM alpine:3.24 AS amneziawg-tools
RUN apk add --no-cache build-base linux-headers
ADD --checksum=sha256:22438f231d39ea27e4bdc69707ec3103357f3cecc9ad3eecba18a61e5b39c9b9 \
    https://codeload.github.com/amnezia-vpn/amneziawg-tools/tar.gz/ee0f0a9aa34ff0a0da4b3433b9512781cfe02843 /tmp/amneziawg-tools.tar.gz
RUN mkdir /build && tar -xzf /tmp/amneziawg-tools.tar.gz -C /build --strip-components=1 \
    && make -C /build/src \
    && make -C /build/src install DESTDIR=/out PREFIX=/usr WITH_WGQUICK=yes WITH_BASHCOMPLETION=no WITH_SYSTEMDUNITS=no

######
# Final image
######
FROM alpine:3.24 AS runtime
# Install OS-level dependencies
RUN apk add --no-cache bash curl iptables nftables openresolv wireguard-tools tzdata
# Setup timezone
ENV TZ=UTC
# Copy binaries
COPY --from=builder /build/dist/wg-portal /app/wg-portal
# Set the Current Working Directory inside the container
WORKDIR /app
# Expose default ports for metrics, web and wireguard
EXPOSE 8787/tcp
EXPOSE 8888/tcp
EXPOSE 51820/udp
# the database and config file can be mounted from the host
VOLUME [ "/app/data", "/app/config" ]
# Command to run the executable
ENTRYPOINT [ "/app/wg-portal" ]

# Optional image, built with --target amneziawg and published as *-amneziawg.
FROM runtime AS amneziawg
RUN apk add --no-cache iproute2
COPY --from=amneziawg-tools /out/usr/bin/awg /usr/bin/awg
COPY --from=amneziawg-tools /out/usr/bin/awg-quick /usr/bin/awg-quick
COPY --from=amneziawg-tools /build/COPYING /usr/share/licenses/amneziawg-tools/COPYING

# Keep the default image and its dependencies unchanged.
FROM runtime AS standard
