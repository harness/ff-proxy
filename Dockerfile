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
FROM alpine:latest
RUN apk update && apk add --no-cache bash
COPY --from=builder /app/ff-proxy /app/ff-proxy
USER nobody:nogroup
ENTRYPOINT ["/app/ff-proxy"]
