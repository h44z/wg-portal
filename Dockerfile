# Dockerfile References: https://docs.docker.com/engine/reference/builder/
# This dockerfile uses a multi-stage build system to reduce the image footprint.

######-
# Start from the latest golang base image as builder image (only used to compile the code)
######-
FROM golang:1.21 as builder

ARG BUILD_IDENTIFIER
ENV ENV_BUILD_IDENTIFIER=$BUILD_IDENTIFIER

ARG BUILD_VERSION
ENV ENV_BUILD_VERSION=$BUILD_VERSION

# populated by BuildKit
ARG TARGETPLATFORM
ENV ENV_TARGETPLATFORM=$TARGETPLATFORM

RUN mkdir /build

# Copy the source from the current directory to the Working Directory inside the container
ADD . /build/

# Set the Current Working Directory inside the container
WORKDIR /build

# Build the Go app
RUN echo "Building version '$ENV_BUILD_IDENTIFIER-$ENV_BUILD_VERSION' for platform $ENV_TARGETPLATFORM"; make build

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