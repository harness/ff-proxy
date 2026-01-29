# ff-proxy Helm Chart

Helm chart to deploy a Harness Feature Flags v2r Relay Proxy

## Install

Install the v2 proxy:

The minimum configuration needed is:
- A [proxy key](https://developer.harness.io/docs/feature-flags/relay-proxy/relay_proxy_v2/#creating-a-proxy-key)
- An auth secret (a random string, used to encrypt connection between your SDK (applications) and the proxy)
- The address of a redis server

```
helm upgrade -i ff-proxy --namespace ff-proxy --create-namespace .\
  --set proxyKey=xxxx-xxx-xxx-xxxx \
  --set authSecret=xxxx-xxx-xxx-xxxx \
  --set redis.address=redis:6379
```

After install/upgrade the helm notes will display information on how to retrieve the proxy URL for your configuration for use with the SDK.

```
Use http://127.0.0.1:8080 as your relay proxy URL
```

## Uninstall

To remove the proxy run:
```
helm uninstall --namespace ff-proxy ff-proxy
```

### Configuration

Please read the [v2 Proxy documentation](https://developer.harness.io/docs/feature-flags/relay-proxy/relay_proxy_v2) for a detailed explanation of all configuration options.

Then see `values.yaml` for an extensive list of both proxy and Kubernetes configurations available.

By default the proxy will deploy with one writer and one read replica.

#### Redis TLS/mTLS Authentication

The chart supports Redis mTLS (mutual TLS) authentication. **TLS is disabled by default** and must be explicitly enabled by clients.

**To enable Redis mTLS:**

1. Create a Kubernetes Secret containing your certificates:
   ```bash
   kubectl create secret generic redis-tls-secret \
     --from-file=ca.crt=/path/to/ca.crt \
     --from-file=client.crt=/path/to/client.crt \
     --from-file=client.key=/path/to/client.key \
     -n <namespace>
   ```

2. Enable TLS in your values file or via `--set` flags:
   ```bash
   helm upgrade -i ff-proxy --namespace ff-proxy . \
     --set proxyKey=xxxx-xxx-xxx-xxxx \
     --set authSecret=xxxx-xxx-xxx-xxxx \
     --set redis.address=rediss://redis.example.com:6380 \
     --set redis.tls.enabled=true \
     --set redis.tls.secret.name=redis-tls-secret
   ```

   Or create a custom values file (`my-values.yaml`):
   ```yaml
   redis:
     address: "rediss://redis.example.com:6380"
     tls:
       enabled: true
       secret:
         name: "redis-tls-secret"
   ```

   Then deploy:
   ```bash
   helm upgrade -i ff-proxy --namespace ff-proxy . -f my-values.yaml
   ```

**Note:** The default `redis.tls.enabled: false` ensures backward compatibility. Clients who don't need TLS don't need to do anything - it will work as before with password authentication.
