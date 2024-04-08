# HA Mode Proxy with Single Redis

This example spins up a Primary & Read replica Proxy that connect to a single redis instance. It also starts up Prometheus and Grafana to demonstrate what monitoring we have available for the Proxy.

## Configure Proxy

If you haven't already, create a Proxy Key in Harness and set it as the `PROXY_KEY` environment variable in the docker-compose file.

## Running

To start the Proxy, Prometheus & Grafana run `docker-compose up` from this directory in your terminal.

## Configuring an SDK to use the Proxy

See [Configure SDKs to work with the Relay Proxy](https://developer.harness.io/docs/feature-flags/relay-proxy/deploy-relay-proxy#configure-sdks-to-work-with-the-relay-proxy)

## Monitoring the Proxy with Grafana

- Open [http://localhost:3000](http://localhost:3000) in your browser
- To log in to a local grafana instance for the first time use `admin` for the username and `admin` for the password. Grafana will then ask you to create a new password for the `admin` user, you can set this to any value you want it to be.
- Once you've logged in you should be able to navigate to the Harness FF Proxy dashboard. If you've pointed an SDK at your Proxy you should be able to see some metrics start to appear here. 


https://github.com/harness/ff-proxy/assets/16992818/3168c7e0-a27b-4bca-b48f-92f69b27fd05

