# Outbound Endpoints
These are the endpoints requested by the Relay Proxy when it communicates with Harness SaaS. These are listed in the order they're used:
- Startup
- Start SDK per key
- Periodic requests/polls

The base url of these endpoints are configurable if you need to pass them through a filter or another proxy. See [configuration](./configuration.md) for details.

## Basic startup

This is the basic data fetched on startup. It authenticates each sdk key and parses the project/environment info from the jwt response. It then fetches the hashed api keys + optionally targets. This will page through this data so may make multiple requests.

* `POST https://config.ff.harness.io/api/1.0/client/auth` - authenticates api key

* `GET https://app.harness.io/gateway/cf/admin/apikey` - gets all hashed api keys for this environment. These are required so connected sdks can authenticate using any key from this environment. This pages through the api keys so may make multiple requests.

* `GET https://app.harness.io/gateway/cf/admin/targets` - fetches environment target data (optional - see TARGET_POLL_DURATION config option). This pages through the targets so may make multiple requests.

## Start SDKs

These requests run per each valid API  key configured. This authenticates, fetches flag/target group data and sets up the stream. These are required to start up correctly.

* `POST https://config.ff.harness.io/api/1.0/client/auth` - authenticates API  key

* `GET https://config.ff.harness.io/api/1.0/client/env/${ENV_ID}/feature-configs` - fetches flag data

* `GET https://config.ff.harness.io/api/1.0/client/env/${ENV_ID}/target-segments` - fetches target group data

* `GET https://config.ff.harness.io/api/1.0/client/env/${ENV_ID}/stream` - initialises long lived stream to listen for events

* `GET https://config.ff.harness.io/api/1.0/client/env/${ENV_ID}/feature-configs/${FLAG_NAME}` - fetches updated flag data after a flag stream event comes in

* `GET https://config.ff.harness.io/api/1.0/client/env/${ENV_ID}/target-segments/${GROUP_NAME}` - fetches updated target group data after a target group stream event comes in


## Periodic requests/polls

These requests happen either on demand or by various timers while the Relay Proxy is running.

* `GET https://app.harness.io/gateway/cf/admin/targets` - polls the latest environment target data (optional - see TARGET_POLL_DURATION config option). This pages through the targets so may make multiple requests.

* `POST https://events.ff.harness.io/api/1.0/metrics` - sends metrics (optional - see METRIC_POST_DURATION config option).

* `POST https://config.ff.harness.io/api/1.0/client/auth` - when a client authenticates with the Relay Proxy, Harness forwards this request to the remote server to register the target.

## Domains Requested
* https://app.harness.io/gateway/cf/admin/*

* https://config.ff.harness.io/api/1.0/client/*

* https://events.ff.harness.io/api/1.0/metrics

## Protocols
All requests to SaaS are made using HTTPS on port 443

The /stream request is a long lived SSE connection that receives messages over time and may need special network configuration to be allowed in corporate environments.

## Sequence diagram for proxy requests

Here is a sequence diagram describing the proxy calls:

![Call Flow](./images/call_flow.png "Call Flow")
