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
# STEP 2 add relay proxy build to pushpin image
############################
FROM harness/proxy-pushpin
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
