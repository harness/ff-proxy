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


FROM fanout/pushpin:1.41.0-1 as pushpin

USER root
COPY docker-entrypoint.sh /usr/local/bin/
RUN chmod +x /usr/local/bin/docker-entrypoint.sh
USER 65543

# Define default entrypoint and command
ENTRYPOINT ["docker-entrypoint.sh"]
CMD ["pushpin", "--merge-output"]


############################
# STEP 3 add relay proxy build to pushpin image
############################
FROM pushpin

USER root

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
