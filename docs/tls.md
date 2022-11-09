# Enabling TLS

The Relay Proxy does not currently natively support running with TLS enabled (coming soon).

The recommended way to connect to the Relay Proxy using TLS is to place a reverse proxy such as nginx in front of the Relay Proxy. Then all connected sdks should make requests to the reverse proxy url instead of hitting the Relay Proxy directly.

![TLS Setup](images/TLS.png?raw=true)

A sample docker compose for this architecture is included in our [examples folder](../examples/tls_reverse_proxy/README.md).