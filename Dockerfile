############################
# STEP 1 build executable binary
############################
FROM golang:1.22 as builder

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
# STEP 2 build pushpin 22.04 image - source https://github.com/fanout/docker-pushpin/blob/master/Dockerfile
# TODO - this will rarely change - publish as an image we can consume
############################
# Pull the base image
FROM ubuntu:24.04 as pushpin

# Add private APT repository
RUN \
  apt-get update && \
  apt-get install -y apt-transport-https software-properties-common && \
  echo deb https://fanout.jfrog.io/artifactory/debian fanout-jammy main \
    | tee /etc/apt/sources.list.d/fanout.list && \
  apt-key adv --keyserver hkp://keyserver.ubuntu.com:80 --recv-keys \
    7D0343148157C3DF

ENV PUSHPIN_VERSION 1.37.0-1~jammy

# Install Pushpin
RUN \
  apt-get update && \
  apt-get install -y pushpin=$PUSHPIN_VERSION curl binutils

# Fix CVEs
RUN \
  apt-get upgrade -y perl openssl nghttp2

# Required for the image to work on Centos7 with 3.10 kernel
RUN \
    strip --remove-section=.note.ABI-tag /usr/lib/x86_64-linux-gnu/libQt5Core.so.5

# Cleanup
RUN \
  apt-get clean && \
  rm -fr /var/lib/apt/lists/* && \
  rm -fr /tmp/*

# Add entrypoint script
COPY docker-entrypoint.sh /usr/local/bin/
# give permission to run entrypoint script
RUN chmod +x /usr/local/bin/docker-entrypoint.sh

# Define default entrypoint and command
ENTRYPOINT ["docker-entrypoint.sh"]
CMD ["pushpin", "--merge-output"]


############################
# STEP 3 add relay proxy build to pushpin image
############################
FROM pushpin
COPY --from=builder /app/ff-proxy /app/ff-proxy
COPY --from=builder ./app/config/pushpin /etc/pushpin
COPY --from=builder ./app/start.sh /start.sh

RUN mkdir /log
RUN mkdir /pushpin
RUN mkdir /pushpin/run
RUN mkdir /pushpin/log
RUN chmod -R 0500 /app/ff-proxy /usr/lib/pushpin /etc/pushpin
RUN chmod -R 0755 /log /pushpin /usr/lib/pushpin /etc/pushpin
RUN chown -R 65534:65534 /app/ff-proxy /log /pushpin /usr/lib/pushpin /etc/pushpin

# Setting this to 65534 which hould be the nodbody user
USER 65534

# Expose default port pushpin listens on
EXPOSE 7000
CMD ["./start.sh"]
