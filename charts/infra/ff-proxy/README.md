# ff-proxy Helm Chart

Helm chart to deploy a Harness Feature Flags v2r Relay Proxy

## Install

Configure the helm repository:
```
helm repo add feature-flag-relay-proxy https://rssnyder.github.io/feature-flag-relay-proxy
```

Update the repository:
```
helm repo update feature-flag-relay-proxy
```

Install the v2 proxy:
```
helm upgrade -i ff-proxy --namespace ff-proxy --create-namespace \
  feature-flag-relay-proxy/ff-proxy \
  --set proxyKey=xxxx-xxx-xxx-xxxx \
  --set authSecret=xxxx-xxx-xxx-xxxx \
  --set redisAddress=redis:6379
```

After install/upgrade the helm notes will display information on how to retrieve the proxy URL for your configuration for use with the SDK.

## Uninstall

To remove the proxy run:
```
helm uninstall --namespace ff-proxy ff-proxy
```

### Configuration

Please read the [v2 Proxy documentation](https://developer.harness.io/docs/feature-flags/relay-proxy/relay_proxy_v2) for a detailed explanation of all configuration options.

Then see `values.yaml` for an extensive list of both proxy and Kubernetes configurations available.

By default the proxy will deploy with one writer and one read replica.
