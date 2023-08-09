# FF-Proxy
[![Go Report Card](https://goreportcard.com/badge/github.com/harness/ff-proxy)](https://goreportcard.com/report/github.com/harness/ff-proxy)

The Relay Proxy is a lightweight Go application that runs within your infrastructure and handles all streaming connections to the Harness platform. It fetches all of your flag, target, and target group data on startup, caches it, fetches any updates over time, and serves it to your connected downstream SDKs.

## Getting Started
To learn more about deploying the relay proxy see [Deploy the Relay Proxy](https://developer.harness.io/docs/feature-flags/ff-using-flags/relay-proxy/deploy-relay-proxy/).

## Why use the Relay Proxy?
To read more about use cases and advantages of the Relay Proxy see the [Why use the Relay Proxy?](https://developer.harness.io/docs/feature-flags/ff-using-flags/relay-proxy/#why-use-the-relay-proxy).

You can also read more about the use cases, architecture and more in [our blog post](https://harness.io/blog/in-depth-feature-flags-relay-proxy).


## Configuration
To view the many configuration options available read [Configuration](./docs/configuration.md).

## TLS
To view how to securely connect sdks to the Relay Proxy with HTTPS enabled see [TLS](./docs/tls.md).

## Redis Cache
By default the Relay Proxy runs with an [In Memory Cache](./docs/in_memory_cache.md).

To run with persistent storage in redis read [Redis](./docs/redis_cache.md).

## Offline Mode
If you'd like to run the Relay Proxy in airgapped environments or locations with poor internet reliability you can generate offline flag configuration and run the Relay Proxy in a fully offline mode. To learn more read [Offline Mode](./docs/offline.md).

## Load Balancing
For info on horizontal scaling Relay Proxies and a working example see [Load Balancing](./docs/load_balancing.md).

## Windows
If you'd like to build and run the Relay Proxy on Windows see [Windows](./docs/windows.md).

## Endpoints
For info on the external Harness SaaS endpoints the Relay Proxy communicates with see [Outbound Endpoints](./docs/outbound_endpoints.md).
For info on the Relay Proxy endpoints your sdks will connect to see [Inbound Endpoints](./docs/inbound_endpoints.md).

## Debugging
For help on debugging your Relay Proxy install see [Debugging](./docs/debugging.md).

## Contributing
See the [contribution guide](CONTRIBUTING.md).
