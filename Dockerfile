# Dockerfile References: https://docs.docker.com/engine/reference/builder/
# This dockerfile uses a multi-stage build system to reduce the image footprint.

######-
# Start from the latest golang base image as builder image (only used to compile the code)
######-
FROM golang:1.15 as builder

RUN mkdir /build

# Copy the source from the current directory to the Working Directory inside the container
ADD . /build/

# Set the Current Working Directory inside the container
WORKDIR /build

# Workaround for failing travis-ci builds
RUN rm -rf ~/go; rm -rf go.sum

# Build the Go app
RUN go clean -modcache; go mod tidy; make build

######-
# Here starts the main image
######-
FROM debian:buster

# Setup timezone
ENV TZ=Europe/Vienna

# GOSS for container health checks
ENV GOSS_VERSION v0.3.14
RUN apt-get update && apt-get upgrade -y && \
        apt-get install --no-install-recommends -y moreutils ca-certificates curl && \
        rm -rf /var/cache/apt /var/lib/apt/lists/*; \
        curl -L https://github.com/aelsabbahy/goss/releases/download/$GOSS_VERSION/goss-linux-amd64 -o /usr/local/bin/goss && \
        chmod +rx /usr/local/bin/goss && \
        goss --version

COPY --from=builder /build/dist/wg-portal /app/wgportal
COPY --from=builder /build/dist/assets /app/assets
COPY --from=builder /build/scripts /app/

# Set the Current Working Directory inside the container
WORKDIR /app

# Command to run the executable
CMD [ "/app/wgportal" ]

HEALTHCHECK --interval=1m --timeout=10s \
    CMD /app/docker-healthcheck.sh
