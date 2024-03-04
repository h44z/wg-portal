# Dockerfile References: https://docs.docker.com/engine/reference/builder/
# This dockerfile uses a multi-stage build system to reduce the image footprint.

######
# Build frontend
######
FROM --platform=${BUILDPLATFORM} node:lts-alpine as frontend
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
FROM --platform=${BUILDPLATFORM} golang:1.21 as builder
# Set the working directory
WORKDIR /build
# Download dependencies
COPY go.mod go.sum ./
RUN go mod download
# Copy the sources to the working directory
COPY . .
# Copy the frontend build result
COPY --from=frontend /build/dist/ ./internal/app/api/core/frontend-dist/
# Set the build version and identifier from arguments
ARG BUILD_IDENTIFIER BUILD_VERSION
ENV ENV_BUILD_IDENTIFIER=${BUILD_IDENTIFIER}
ENV ENV_BUILD_VERSION=${BUILD_VERSION}

# Split to cross-platform build
ARG TARGETARCH
ENV GOARCH=${TARGETARCH}
# Build the Go app
RUN echo "Building version '$ENV_BUILD_IDENTIFIER-$ENV_BUILD_VERSION' for architecture $TARGETARCH"
RUN make build

######
# Final image
######
FROM alpine:3.19
# Install OS-level dependencies
RUN apk add --no-cache bash openresolv
# Setup timezone
ENV TZ=Europe/Vienna
# Copy binaries
COPY --from=builder /build/dist/wg-portal /app/wg-portal
# Set the Current Working Directory inside the container
WORKDIR /app
# by default, the web-portal is reachable on port 8888
EXPOSE 8888/tcp
# the database and config file can be mounted from the host
VOLUME [ "/app/data", "/app/config" ]
# Command to run the executable
ENTRYPOINT [ "/app/wg-portal" ]
