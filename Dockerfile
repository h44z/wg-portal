# Dockerfile References: https://docs.docker.com/engine/reference/builder/
# This dockerfile uses a multi-stage build system to reduce the image footprint.

######-
# Start from the latest golang base image as builder image (only used to compile the code)
######-
FROM golang:1.16 as builder

ARG BUILD_IDENTIFIER
ENV ENV_BUILD_IDENTIFIER=$BUILD_IDENTIFIER

ARG BUILD_VERSION
ENV ENV_BUILD_VERSION=$BUILD_VERSION

RUN mkdir /build

# Copy the source from the current directory to the Working Directory inside the container
ADD . /build/

# Set the Current Working Directory inside the container
WORKDIR /build

# Workaround for failing travis-ci builds
RUN rm -rf ~/go; rm -rf go.sum

# Download dependencies
RUN curl -L https://git.prolicht.digital/pub/healthcheck/-/releases/v1.0.1/downloads/binaries/hc -o /build/hc; \
    chmod +rx /build/hc; \
    echo "Building version: $ENV_BUILD_IDENTIFIER-$ENV_BUILD_VERSION"

# Build the Go app
RUN go clean -modcache; go mod tidy; make build-docker

######-
# Here starts the main image
######-
FROM scratch

# Setup timezone
ENV TZ=Europe/Vienna

# Import linux stuff from builder.
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /etc/passwd /etc/passwd
COPY --from=builder /etc/group /etc/group

# Import healthcheck binary
COPY --from=builder /build/hc /app/hc

# Copy binaries
COPY --from=builder /build/dist/wgportal /app/wgportal

# Set the Current Working Directory inside the container
WORKDIR /app

# Command to run the executable
CMD [ "/app/wgportal" ]

HEALTHCHECK --interval=30s --timeout=5s --start-period=30s --retries=3 CMD [ "/app/hc", "http://localhost:11223/health" ]
