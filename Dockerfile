############################
# STEP 1 build executable binary
############################
FROM golang:1.23.12 as builder

WORKDIR /app

ARG gitTag
ENV GIT_TAG $gitTag

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
FROM debian:bookworm-slim as certs
RUN apt-get update && apt-get install -y ca-certificates
RUN mkdir /tmp/certs && cp -r /etc/ssl/certs/* /tmp/certs

############################
# STEP 2: Final runtime image
############################
FROM fanout/pushpin:1.41.0

# Switch to root only for setup
USER root

# Copy entrypoint and binaries
COPY docker-entrypoint.sh /usr/local/bin/
COPY --from=builder /app/ff-proxy /app/ff-proxy
COPY --from=builder /app/config/pushpin /etc/pushpin
COPY --from=builder /app/start.sh /start.sh

# Copy CA certificates
COPY --from=certs /tmp/certs /etc/ssl/certs

# Prepare directories + set permissions in a single layer
RUN chmod +x /usr/local/bin/docker-entrypoint.sh \
 && mkdir -p /log /pushpin/run /pushpin/log \
 && chmod 0500 /app/ff-proxy \
 && chmod -R 0755 /usr/lib/pushpin /etc/pushpin \
 && chmod -R 0775 /log /pushpin \
 && chown -R 65534:65534 /app/ff-proxy /log /pushpin /usr/lib/pushpin /etc/pushpin

# Switch back to nobody user for runtime
USER 65534:65534

# Expose port and set entrypoint
EXPOSE 7000
ENTRYPOINT ["docker-entrypoint.sh"]
CMD ["./start.sh"]
