############################
# STEP 1 build executable binary
############################
FROM golang:1.17 as builder

ARG GITHUB_ACCESS_TOKEN

RUN git config --global url."https://${GITHUB_ACCESS_TOKEN}:x-oauth-basic@github.com/".insteadOf "https://github.com/"
RUN go env -w GOPRIVATE=github.com/

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
# STEP 2 build a small image
############################
FROM fanout/pushpin
COPY --from=builder /app/ff-proxy /app/ff-proxy
COPY --from=builder ./app/config/pushpin /etc/pushpin
COPY --from=builder ./app/start.sh /start.sh

RUN mkdir /log
RUN mkdir /pushpin
RUN mkdir /pushpin/run
RUN mkdir /pushpin/log
RUN chmod -R 0500 /app/ff-proxy 
RUN chmod -R 0755 /log /pushpin
RUN chown -R nobody:nogroup /app/ff-proxy /log /pushpin 

# Seem to need to be root in order to get pushpin running
USER nobody

EXPOSE 7000
CMD ["./start.sh"]
