# FF-Proxy

[![Docs](https://img.shields.io/badge/docs-confluence-blue.svg?style=flat)](https://harness.atlassian.net/wiki/spaces/FFM/pages/2003665145/Relay+Proxy)
[![Slack](https://img.shields.io/badge/slack-ff--team-orange.svg?style=flat?label=ff-team)](https://harness.slack.com/archives/C02AN03D478)

## Quick Start
The proxy can run in one of two modes

1) Offline, where you download flag configuration and load it into the proxy, completely disconnected from Harness
2) Online, where the proxy will connect to harness to fetch feature flag configuration, and automatically fetch updates as flags are modified.

To get started take a look at

[Running in Offline mode](./docs/get_started_offline.md)

[Running in Online mode](./docs/get_started_online.md)

## Getting Started

These instructions are to help you get a copy of the ff-proxy server running on your local machine for development purposes.

### Prerequisites

To build and run the project locally you need to have

- golang >=1.16
- make

### Clone and initialise the repo

```
git clone --recursive https://github.com/harness/ff-proxy.git
git submodule init
git submodule update
```

### Build

The build target will generate the required code and build a binary named ff-proxy

```
make build
```

### Run the proxy

You can run the proxy by executing the binary like so and configure it by passing various flag values

```
./ff-proxy -help
Usage of ./ff-proxy:
    -account-identifier string
        account identifier to load remote config for (default "zEaak-FLS425IEO7OLzMUg")
    -admin-service string
        the url of the admin service (default "https://qa.harness.io/gateway/cf")
    -auth-secret string
        the secret used for signing auth tokens (default "secret")
    -bypass-auth
        bypasses authentication
    -debug
        enables debug logging
    -host string
        host of the proxy service (default "localhost")
    -offline
        enables side loading of data from config dir
    -org-identifier string
        org identifier to load remote config for (default "featureflagorg")
    -port int
        port that the proxy service is exposed on (default 7000)
    -service-token string
        token to use with the ff service
```

You can run the proxy in offline mode by passing the `-offline` flag. When the proxy is running in this mode it will only use configuration for environments from the `./config` directory,

```
./ff-proxy -offline
```

If you're running the proxy in online mode you will need to provide a valid service token that allows the proxy to retrieve config from the ff-server. Currently the only config that it retrives from ff-server is Auth config but there is work planned to make it retreive FeatureConfig, Targets and Segments from ff-server.

```
./ff-proxy -service-token <token>
```

### Developing the Proxy

When developing the proxy you can run it independently in offline mode which will load config for all of the environments under `./config`.

The structure of the config repo is as follows

```
|____test
| |____env-1234
| | |____feature_config.json
| | |____targets.json
| | |____segments.json
|____env-94ef7361-1f2d-40af-9b2c-c1145d537e5a
| |____feature_config.json
| |____targets.json
| |____segments.json
```

All config used for testing is kept under the `./config/test` directory and any config that's used if offline mode is kep in a directory under `./config` that must be prefixed with `-env`. So if you wanted to add config for a new environment that gets loaded in when you run the proxy in offline mode it would just be a case of creating a new `env-<id>` directory and adding the specific config files e.g.

```
.
|____test
| |____env-1234
| | |____.targets.json.swp
| | |____feature_config.json
| | |____targets.json
| | |____segments.json
|____env-94ef7361-1f2d-40af-9b2c-c1145d537e5a
| |____feature_config.json
| |____targets.json
| |____segments.json
|____env-af727e7a-0094-4d4e-b3a7-58db398af3a6
| |____feature_config.json
| |____targets.json
| |____segments.json
```

The contents of each individual config file comes from the following endpoints
- `feature_config.json` - GET /client/env/<env>/feature-configs
- `segments.json` - GET /client/env/<env>/target-segments
- `targets.json` - GET /admin/targets

### Running the Proxy from docker
The docker image can be built by running ```make image```.
To start the proxy you can execute the following command
```docker run -it -v $(PWD)/config:/config -p7000:7000 ff-proxy:latest --offline```

This will start the proxy in offline mode, mounting the local configuration.  


### Testing the Proxy

Like with running the proxy, there is also some default config that's loaded in for testing purposes which is located in `./config/test` and you can add/alter this config the same way as you would for the offline config. This test config is primarily used at the minute for running e2e type tests agains the proxy where we populate the cache with the test config, spin up an http server and check that we get the correct status codes and response bodies for each request.
