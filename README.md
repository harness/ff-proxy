# FF-Proxy

[![Docs](https://img.shields.io/badge/docs-confluence-blue.svg?style=flat)](https://harness.atlassian.net/wiki/spaces/FFM/pages/2003665145/Relay+Proxy)
[![Slack](https://img.shields.io/badge/slack-ff--team-orange.svg?style=flat?label=ff-team)](https://harness.slack.com/archives/C02AN03D478)


## Getting Started

These instructions are to help you get a copy of the ff-proxy server running on your local machine

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
./ff-proxy
```

```
./ff-proxy -help
Usage of ./ff-proxy:
    -debug
        enables debug logging
    -host string
        host of the proxy service (default "localhost")
    -offline
        enables side loading of data from config dir
    -port int
        port that the proxy service is exposed on (default 7000)
```

Currently the proxy only works in offline mode so if you're running it locally for now you'll want to pass the `-offline` flag

```
./ff-proxy -offline
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

### Testing the Proxy

Like with running the proxy, there is also some default config that's loaded in for testing purposes which is located in `./config/test` and you can add/alter this config the same way as you would for the offline config. This test config is primarily used at the minute for running e2e type tests agains the proxy where we populate the cache with the test config, spin up an http server and check that we get the correct status codes and response bodies for each request.
