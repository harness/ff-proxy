# Redis Cache

You can optionally configure the Relay Proxy to store flag data in redis. See [configuration](./configuration.md) for details on setting this up.



### What happens if network connection is lost?
If connection is lost to Harness servers the Relay Proxy will continue to serve the cached values to connected sdks.

It will continue to attempt to reconnect and once connection is established will sync down the latest data then continue as normal.

### What happens if changes are made on SaaS while connection is lost?
Once connection is reestablished with SaaS all flag data is pulled down again. This means even if changes were made on SaaS e.g. flag toggles during the outage they will be picked up once connection is reestablished.

### What happens if no network connection is available on startup?
If the Relay Proxy has previously store flag data in redis then even with no connection to Harness servers it will startup successfully using the cached data and serve connected sdks.

This will essentially launch the Relay Proxy in offline mode, as it will not regain connection once internet connection is restored.