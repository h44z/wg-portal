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
FROM --platform=${BUILDPLATFORM} golang:1.24-alpine AS builder
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
# Final image
######
FROM alpine:3.19
# Install OS-level dependencies
RUN apk add --no-cache bash curl iptables nftables openresolv wireguard-tools
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
