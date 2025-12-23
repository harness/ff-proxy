############################
# STEP 1 build executable binary
############################
FROM golang:1.25.5 AS builder

WORKDIR /app

# Fetch dependencies.
COPY go.mod .
COPY go.sum .
COPY Makefile .
RUN make dep
COPY . .

# Generate Code and Build
RUN make build

############################
# STEP 2: Grab CA certificates
############################
FROM debian:bookworm-slim AS certs
RUN apt-get update && \
    apt-get upgrade -y && \
    apt-get install -y ca-certificates && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*
RUN mkdir /tmp/certs && cp -r /etc/ssl/certs/* /tmp/certs

############################
# STEP 3: Build pushpin from source (matching fanout/pushpin:1.41.0)
# Adapted from https://github.com/fanout/docker-pushpin
# Using ubuntu:24.04 LTS for better security patches and support until 2029
############################
FROM ubuntu:24.04 AS pushpin-builder

# Install build dependencies and update all packages
RUN apt-get update && \
    apt-get upgrade -y && \
    apt-get install -y bzip2 pkg-config make g++ rustc cargo libssl-dev qt6-base-dev libzmq3-dev libboost-dev && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /build

ARG PUSHPIN_VERSION=1.41.0

# Download and extract pushpin source
ADD https://github.com/fastly/pushpin/releases/download/v${PUSHPIN_VERSION}/pushpin-${PUSHPIN_VERSION}.tar.bz2 .

RUN tar xf pushpin-${PUSHPIN_VERSION}.tar.bz2 && mv pushpin-${PUSHPIN_VERSION} pushpin

WORKDIR /build/pushpin

# Build pushpin
RUN make RELEASE=1 PREFIX=/usr CONFIGDIR=/etc
RUN make RELEASE=1 PREFIX=/usr CONFIGDIR=/etc check
RUN make RELEASE=1 PREFIX=/usr CONFIGDIR=/etc INSTALL_ROOT=/build/out install

############################
# STEP 4: Create final image with pushpin and ff-proxy
############################
FROM ubuntu:24.04

# Install runtime dependencies for pushpin and update all packages to fix vulnerabilities
RUN apt-get update && \
    apt-get upgrade -y && \
    apt-get install -y --no-install-recommends libqt6core6 libqt6network6 libzmq5 && \
    apt-get -y autoremove && \
    apt-get -y clean && \
    rm -rf /var/lib/apt/lists/*

# Copy pushpin from builder (matching original base image)
COPY --from=pushpin-builder /build/out/ /

# Copy entrypoint script (from original base image)
COPY docker-entrypoint.sh /usr/local/bin/

# Copy ff-proxy and config
COPY --from=builder /app/ff-proxy /app/ff-proxy
COPY --from=builder /app/config/pushpin /etc/pushpin
COPY --from=builder /app/start.sh /start.sh

# Copy CA certificates
COPY --from=certs /tmp/certs /etc/ssl/certs

# Prepare directories + set permissions
# Use existing nobody user (UID 65534)
RUN chmod +x /usr/local/bin/docker-entrypoint.sh \
 && mkdir -p /var/run/pushpin /log /pushpin/run /pushpin/log \
 && chmod 0500 /app/ff-proxy \
 && chown -R 65534:65534 /etc/pushpin /var/run/pushpin /log /pushpin \
 && chown 65534:65534 /app/ff-proxy

# Use nobody user for runtime
USER 65534:65534

ENV LANG=C.UTF-8

# Expose ports (matching original base image)
EXPOSE 7999
EXPOSE 5560
EXPOSE 5561
EXPOSE 5562
EXPOSE 5563
EXPOSE 7000

ENTRYPOINT ["docker-entrypoint.sh"]
CMD ["./start.sh"]
