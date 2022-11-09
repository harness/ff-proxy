# FF-Proxy

The Relay Proxy is a lightweight Go application that runs within your infrastructure and handles all streaming connections to the Harness platform. It fetches all of your flag, target, and target group data on startup, caches it, fetches any updates over time, and serves it to your connected downstream SDKs.

## Getting Started
To learn more about deploying the relay proxy see [Deploy the Relay Proxy](https://docs.harness.io/article/rae6uk12hk-deploy-relay-proxy).

## Why use the Relay Proxy?
To read more about use cases and advantages of the Relay Proxy see the [Why use the Relay Proxy?](https://docs.harness.io/article/q0kvq8nd2o-relay-proxy#why_use_the_relay_proxy).

You can also read more about the use cases, architecture and more in [our blog post](https://harness.io/blog/in-depth-feature-flags-relay-proxy).


## Configuration
To view the many configuration options available read [Configuration](./docs/configuration.md).

## TLS
To view how to securely connect sdks to the Relay Proxy with HTTPS enabled see [TLS](./docs/tls.md).

## Redis Cache
By default the Relay Proxy runs with an [In Memory Cache](./docs/in_memory_cache.md).

To run with persistent storage in redis read [Redis](./docs/redis_cache.md).

## Offline mode
If you'd like to run the Relay Proxy in airgapped environments or locations with poor internet reliability you can generate offline flag configuration and run the Relay Proxy in a fully offline mode. To learn more read [Offline Mode](./docs/offline.md).

## Windows
If you'd like to build and run the Relay Proxy on Windows see [Windows](./docs/windows.md)

## Contributing
See the [contribution guide](CONTRIBUTING.md).