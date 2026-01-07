# Configuration

### Docker container
When running the docker container configuration must be passed as environment variables. This is most commonly done by passing a .env file when starting the container.

### Compiled exe's
When running a compiled exe you can run either pass flags or environment variables.

### Env file format
The format of the .env file is a standard `variable=value` format. See the examples directory for a  [sample .env file](../examples/tls_reverse_proxy/.env)

### Passing .env file
When running as a docker container the .env file can be passed like so:

`docker run -d -p 7000:7000 --env-file .env ff-proxy`

### Passing flags
When running the compiled exes flags can be passed like so:

`./ff-proxy.exe --admin-service-token=${TOKEN} --auth-secret=${SECRET} --account-identifier=${ACCOUNT_IDENTIFIER} --org-identifier=${ORG_IDENTIFIER} --api-keys=${API_KEYS}`

## Configuration options
### Required config
When running in online mode these config options are the minimal required config to run the relay proxy.

| Environment Variable | Flag                | Description                                                                                                                           | Type   | Default |
|----------------------|---------------------|---------------------------------------------------------------------------------------------------------------------------------------|--------|---------|
| ACCOUNT_IDENTIFIER   | account-identifier  | An account identifier to load remote config.                                                                                          | string |         |
| ADMIN_SERVICE_TOKEN  | admin-service-token | Token to use to communicate with the ff service                                                                                       | string |         |
| ORG_IDENTIFIER       | org-identifier      | An org identifier to load remote config.                                                                                              | string |         |
| API_KEYS             | api-keys            | API keys to connect with ff-server for each environment. Requires only one server api key per environment in a comma separated string | string |         |

### Redis cache
Config required to connect to redis. Note only `REDIS_ADDRESS` is required to connect, the others only need to be set if they differ from the defaults.

| Environment Variable | Flag           | Description                                                        | Type   | Default |
|----------------------|----------------|--------------------------------------------------------------------|--------|---------|
| REDIS_ADDRESS        | redis-address  | Redis host:port address. See below for info on connecting via TLS  | string |         |
| REDIS_USERNAME       | redis-username | (Optional) Redis username for ACL authentication                   | string |         |
| REDIS_PASSWORD       | redis-password | (Optional) Redis password                                         | string |         |
| REDIS_DB             | redis-db       | (Optional) Database to be selected after connecting to the server | int    | 0       |

**Connecting to Redis:** The Relay Proxy supports two Redis authentication modes:

1. **Password Authentication** (default): Username/password with optional TLS
2. **mTLS Authentication**: Mutual TLS with client certificates

For complete Redis configuration, certificate generation, and setup instructions, see [Redis Cache Configuration](./redis_cache.md). 

### Logging
Control log level

| Environment Variable | Flag  | Description            | Type    | Default |
|----------------------|-------|------------------------|---------|---------|
| LOG_LEVEL                | log-level | Controls the log level. Valid inputs are `INFO`, `DEBUG` & `ERROR`. | string | `INFO`   |

### Offline mode
These are the config options applicable for running in offline mode

| Environment Variable    | Flag                    | Description                                                                                                    | Type    | Default |
|-------------------------|-------------------------|----------------------------------------------------------------------------------------------------------------|---------|---------|
| GENERATE_OFFLINE_CONFIG | generate-offline-config | if set to true, the proxy produces an offline configuration in the mounted /config directory, then terminates. | boolean | false   |
| CONFIG_DIR              | config-dir              | Specify a path for the offline config directory. The default is /config.                                       | string  | /config |
| OFFLINE                 | offline                 | Enables side loading of data from the config directory.                                                        | boolean | false   |


### Port
Adjust the port the relay proxy runs on.

| Environment Variable | Flag | Description                                | Type | Default |
|----------------------|------|--------------------------------------------|------|---------|
| PORT                 | port | Port that the proxy service is exposed on. | int  | 7000    |

**Note**: When running the docker container PORT cannot be set to 8000 as it is reserved for an internal service.

### Connection mode between Relay Proxy and Harness SaaS
Some corporate networks may be highly restrictive on allowing sse connections. If you find that the Relay Proxy starts successfully but fails to receive any updates you may want to use these settings to force the Relay Proxy to poll for changes instead of streaming them.

| Environment Variable | Flag                 | Description                                                                                                                 | Type    | Default |
|----------------------|----------------------|-----------------------------------------------------------------------------------------------------------------------------|---------|---------|
| FLAG_STREAM_ENABLED  | flag-stream-enabled  | Should the proxy connect to Harness in streaming mode to get flag changes. Set to false if your network absorbs sse events. | boolean | true    |
| FLAG_POLL_INTERVAL   | flag-poll-interval   | How often in seconds the proxy should poll for flag updates (if stream not connected)                                       | int     | 1       |

### Adjust timings
Adjust how often certain actions are performed.

| Environment Variable | Flag                 | Description                                                                                 | Type | Default |
|----------------------|----------------------|---------------------------------------------------------------------------------------------|------|---------|
| TARGET_POLL_DURATION | target-poll-duration | How often in seconds the proxy polls feature flags for Target changes. Set to 0 to disable. | int  | 0       |
| METRIC_POST_DURATION | metric-post-duration | How often in seconds the proxy posts metrics to Harness. Set to 0 to disable.               | int  | 60      |
| HEARTBEAT_INTERVAL   | heartbeat-interval   | How often in seconds the proxy polls pings it's health function. Set to 0 to disable.       | int  | 60      |

### TLS (HTTP Server)
| Environment Variable | Flag        | Description                                                                 | Type   | Default |
|----------------------|-------------|-----------------------------------------------------------------------------|--------|---------|
| TLS_ENABLED          | tls-enabled | If true the proxy will use the tlsCert and tlsKey to run with https enabled | bool   | false   |
| TLS_CERT             | tls-cert    | Path to tls cert file. Required if tls enabled is true.                     | string |         |
| TLS_KEY              | tls-key     | Path to tls key file. Required if tls enabled is true.                      | string |         |

### Redis TLS/mTLS Configuration
Configuration for Redis TLS and mutual TLS (mTLS) authentication.

| Environment Variable | Flag                        | Description                                                                 | Type   | Default |
|----------------------|-----------------------------|-----------------------------------------------------------------------------|--------|---------|
| REDIS_TLS_ENABLED            | redis-tls-enabled            | Enable TLS/mTLS for Redis connections                                      | bool   | false   |
| REDIS_TLS_MODE               | redis-tls-mode               | TLS mode: 'mtls' for mutual TLS (only supported mode)                       | string | ""      |
| REDIS_TLS_CA_CERT            | redis-tls-ca-cert            | Path to CA certificate file. Required if TLS is enabled                     | string | ""      |
| REDIS_TLS_CLIENT_CERT        | redis-tls-client-cert        | Path to client certificate file. Required if TLS mode is 'mtls'             | string | ""      |
| REDIS_TLS_CLIENT_KEY         | redis-tls-client-key         | Path to client private key file. Required if TLS mode is 'mtls'              | string | ""      |
| REDIS_TLS_INSECURE_SKIP_VERIFY | redis-tls-insecure-skip-verify | Skip server certificate verification (not recommended for production)     | bool   | false   |
| REDIS_TLS_SERVER_NAME        | redis-tls-server-name        | Server name for TLS SNI. Use if Redis is behind a load balancer            | string | ""      |

**Examples:**

**Password Authentication (Default):**
```bash
# Plain connection
REDIS_ADDRESS=redis://localhost:6379
REDIS_PASSWORD=your_password

# TLS-encrypted connection (via rediss://)
REDIS_ADDRESS=rediss://localhost:6380
REDIS_PASSWORD=your_password
```

**mTLS (Mutual Authentication):**
```bash
REDIS_ADDRESS=rediss://redis.example.com:6380
REDIS_TLS_ENABLED=true
REDIS_TLS_MODE=mtls
REDIS_TLS_CA_CERT=/etc/redis/ca.crt
REDIS_TLS_CLIENT_CERT=/etc/redis/client.crt
REDIS_TLS_CLIENT_KEY=/etc/redis/client.key
```

**Password + mTLS (Both Required):**
```bash
REDIS_ADDRESS=rediss://redis.example.com:6380
REDIS_USERNAME=myuser
REDIS_PASSWORD=mypassword
REDIS_TLS_ENABLED=true
REDIS_TLS_MODE=mtls
REDIS_TLS_CA_CERT=/etc/redis/ca.crt
REDIS_TLS_CLIENT_CERT=/etc/redis/client.crt
REDIS_TLS_CLIENT_KEY=/etc/redis/client.key
```

**Important Notes:**
- **TLS is Disabled by Default**: `REDIS_TLS_ENABLED` defaults to `false` for backward compatibility
- **Opt-in**: TLS must be explicitly enabled - existing deployments continue working without changes
- **Certificate Paths**: Certificate paths are configurable (default: `/etc/redis/tls` in Helm deployments)
- **Mount Path**: In Helm deployments, the mount path can be customized via `redis.tls.mountPath`
- **Path Construction**: Certificate paths are automatically constructed as `mountPath + "/" + secretKey`

**Note:** By default, the Relay Proxy uses password-based authentication. TLS/mTLS is opt-in and does not affect existing deployments.

### Harness URLs
You may need to adjust these if you pass all your traffic through a filter or proxy rather than sending the requests directly. 

| Environment Variable | Flag           | Description                                 | Type   | Default                              |
|----------------------|----------------|---------------------------------------------|--------|--------------------------------------|
| ADMIN_SERVICE        | admin-service  | URL of the ff admin service                 | string | https://app.harness.io/gateway/cf    |
| CLIENT_SERVICE       | client-service | URL of the ff client service                | string | https://config.ff.harness.io/api/1.0 |
| METRIC_SERVICE       | metric-service | URL of the ff metric service                | string | https://events.ff.harness.io/api/1.0 |
| SDK_BASE_URL         | sdk-base-url   | URL for the embedded SDK to connect to      | string | https://config.ff.harness.io/api/1.0 |
| SDK_EVENTS_URL       | sdk-events-url | URL for the embedded SDK to send metrics to | string | https://events.ff.harness.io/api/1.0 |

### Auth
| Environment Variable | Flag        | Description                                                                  | Type    | Default |
|----------------------|-------------|------------------------------------------------------------------------------|---------|---------|
| BYPASS_AUTH          | bypass-auth | Bypasses authentication for connecting sdks                                  | boolean | false   |
| AUTH_SECRET          | auth-secret | The secret used for signing the authentication token generated by the Proxy. | string  | secret  |

### Development
Flags that can help when developing the proxy.

| Environment Variable | Flag                 | Description                    | Type    | Default |
|----------------------|----------------------|--------------------------------|---------|---------|
| PPROF                | pprof                | Enables pprof on port 6060     | boolean | false   |
| GCP_PROFILER_ENABLED | gcp-profiler-enabled | Enables the gcp cloud profiler | boolean | false   |

### Beta
Features that are in beta and need to be manually enabled

| Environment Variable | Flag                 | Description                                                                                                                                           | Type    | Default |
|----------------------|----------------------|-------------------------------------------------------------------------------------------------------------------------------------------------------|---------|---------|
| AND_RULES            | and-rules            | Enables support for AND rules feature. Note this also requires the FFM_11173_TARGET_GROUP_AND_OPERATOR to be enabled for your account on Harness SaaS | boolean | false   |
