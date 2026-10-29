############################
# STEP 1: Build executable binary
############################
FROM harness0.harness.io/oci/docker_artifacts/hhi/go:1.27.3-dev AS builder

# The curated base runs as a non-root user and ships without make/git
USER root
RUN apk add --no-cache make git

WORKDIR /app

ARG gitTag
ENV GIT_TAG=$gitTag

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
RUN apt-get update && apt-get install -y ca-certificates
RUN mkdir /tmp/certs && cp -r /etc/ssl/certs/* /tmp/certs

############################
# STEP 3: Build pushpin from source (matching fanout/pushpin:1.41.0)
# Adapted from https://github.com/fanout/docker-pushpin
# Built on HarnessOS (glibc, apk)
############################
FROM harness0.harness.io/oci/docker_artifacts/hhi/harnessos-alpine:1 AS pushpin-builder

# HarnessOS: apk equivalents of the ubuntu build deps (build-base for make/g++, rust-1.97 for rustc/cargo).
RUN apk add --no-cache bzip2 pkgconf build-base rust-1.97 openssl-dev qt6-qtbase-dev zeromq-dev boost-dev
# HarnessOS installs qmake under /usr/lib/qt6/bin, which is not on PATH; pushpin's build looks it up there.
ENV PATH="/usr/lib/qt6/bin:${PATH}"

WORKDIR /build

ARG PUSHPIN_VERSION=1.41.0

# Download and extract pushpin source
ADD https://github.com/fastly/pushpin/releases/download/v${PUSHPIN_VERSION}/pushpin-${PUSHPIN_VERSION}.tar.bz2 .

RUN tar xf pushpin-${PUSHPIN_VERSION}.tar.bz2 && mv pushpin-${PUSHPIN_VERSION} pushpin

WORKDIR /build/pushpin

RUN make RELEASE=1 PREFIX=/usr CONFIGDIR=/etc
RUN make RELEASE=1 PREFIX=/usr CONFIGDIR=/etc check
RUN make RELEASE=1 PREFIX=/usr CONFIGDIR=/etc INSTALL_ROOT=/build/out install

############################
# STEP 4: Create final image with pushpin and ff-proxy
############################
FROM harness0.harness.io/oci/docker_artifacts/hhi/harnessos-alpine:1

# HarnessOS: apk equivalents of the ubuntu runtime libraries. bash and procps (pkill -P) are used by
# docker-entrypoint.sh and start.sh and shipped with ubuntu.
RUN apk add --no-cache qt6-qtbase zeromq libsodium libtasn1 ca-certificates bash procps

# Copy pushpin from builder (matching original base image)
COPY --from=pushpin-builder /build/out/ /

# Copy entrypoint script
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
EXPOSE 7999 5560 5561 5562 5563 7000

ENTRYPOINT ["docker-entrypoint.sh"]
CMD ["./start.sh"]
