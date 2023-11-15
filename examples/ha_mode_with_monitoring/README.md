# HA Mode With Monitoring

This example will spin up a Primary Proxy and a Read Replica Proxy along with Prometheus and Grafana.

## Configure Proxy

If you haven't already, create a Proxy Key in Harness and set it as the `PROXY_KEY` environment variable in the docker-compose file.

## Running

To start the Proxy, Prometheus & Grafana run `docker-compose up` from this directory in your terminal.

## Configuring an SDK to use the Proxy

See [Configure SDKs to work with the Relay Proxy](https://developer.harness.io/docs/feature-flags/relay-proxy/deploy-relay-proxy#configure-sdks-to-work-with-the-relay-proxy)

## Monitoring the Proxy with Grafana

TBD
