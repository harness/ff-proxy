############################
# STEP 1 build executable binary
############################
FROM golang:1.18 as builder

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
# STEP 2 build pushpin 20.04 image - source https://github.com/fanout/docker-pushpin/blob/master/Dockerfile
# TODO - this will rarely change - publish as an image we can consume
############################
# Pull the base image
FROM ubuntu:20.04 as pushpin

# Add private APT repository
RUN \
  apt-get update && \
  apt-get install -y apt-transport-https software-properties-common && \
  echo deb https://fanout.jfrog.io/artifactory/debian fanout-bionic main \
    | tee /etc/apt/sources.list.d/fanout.list && \
  apt-key adv --keyserver hkp://keyserver.ubuntu.com:80 --recv-keys \
    EA01C1E777F95324

ENV PUSHPIN_VERSION 1.33.1-1~bionic1

# Install Pushpin
RUN \
  apt-get update && \
  apt-get install -y pushpin=$PUSHPIN_VERSION

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

# Expose ports.
# - 7999: HTTP port to forward on to the app
# - 5560: ZMQ PULL for receiving messages
# - 5561: HTTP port for receiving messages and commands
# - 5562: ZMQ SUB for receiving messages
# - 5563: ZMQ REP for receiving commands
EXPOSE 7999
EXPOSE 5560
EXPOSE 5561
EXPOSE 5562
EXPOSE 5563


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

EXPOSE 7000
CMD ["./start.sh"]
