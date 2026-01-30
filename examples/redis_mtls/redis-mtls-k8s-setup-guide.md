# Redis mTLS Authentication Guide for FF-Proxy

This guide provides step-by-step instructions for configuring FF-Proxy to connect to Redis using mutual TLS (mTLS) authentication.

---

## Table of Contents

1. [Overview](#overview)
2. [Prerequisites](#prerequisites)
3. [Certificate Requirements](#certificate-requirements)
4. [Step 1: Prepare Certificates](#step-1-prepare-certificates)
5. [Step 2: Create Kubernetes Secrets](#step-2-create-kubernetes-secrets)
6. [Step 3: Configure Helm Values](#step-3-configure-helm-values)
7. [Step 4: Deploy FF-Proxy](#step-4-deploy-ff-proxy)
8. [Step 5: Verify the Deployment](#step-5-verify-the-deployment)
9. [Configuration Reference](#configuration-reference)
10. [Troubleshooting](#troubleshooting)

---

## Overview

### What is mTLS?

Mutual TLS (mTLS) is an authentication method where both the client (FF-Proxy) and server (Redis) verify each other's identity using certificates. This provides:

- **Encryption**: All data between FF-Proxy and Redis is encrypted
- **Mutual Authentication**: Both parties verify each other's identity
- **Strong Security**: Only clients with valid certificates can connect

### How FF-Proxy Detects mTLS

FF-Proxy automatically enables mTLS when all three certificate environment variables are set:
- `REDIS_MTLS_CA_CERT` - Path to CA certificate
- `REDIS_MTLS_CLIENT_CERT` - Path to client certificate
- `REDIS_MTLS_CLIENT_KEY` - Path to client private key

The Helm chart handles setting these variables when you enable TLS.

---

## Prerequisites

Before starting, ensure you have:

1. **Kubernetes cluster** with kubectl access
2. **Helm 3.x** installed
3. **Redis server** configured with mTLS enabled
4. **Certificates** (CA cert, client cert, client key) signed by the same CA as Redis
5. **FF-Proxy Helm chart** (version with mTLS support)

### Redis Server Requirements

Your Redis server must be configured with:
- TLS enabled on a specific port (commonly 6380)
- Client certificate authentication enabled (`tls-auth-clients yes`)
- CA certificate to verify client certificates

Example Redis configuration (`redis.conf`):
```conf
port 0
tls-port 6380
tls-cert-file /path/to/server.crt
tls-key-file /path/to/server.key
tls-ca-cert-file /path/to/ca.crt
tls-auth-clients yes
tls-protocols "TLSv1.2 TLSv1.3"
```

---

## Certificate Requirements

You need three certificate files for FF-Proxy:

| File | Description | Format |
|------|-------------|--------|
| `ca.crt` | CA certificate that signed both server and client certs | PEM |
| `client.crt` | Client certificate for FF-Proxy | PEM |
| `client.key` | Client private key for FF-Proxy | PEM |

### Important Notes

- All certificates must be in PEM format
- The client certificate must be signed by the same CA that Redis trusts
- The CA certificate must be the same one used to sign the Redis server certificate
- Private keys should have restricted permissions (600)

---

## Step 1: Prepare Certificates

### Option A: Use the Certificate Generator Script (Recommended)

We provide a script that generates all required certificates with correct filenames matching the Helm chart defaults.

> **📜 Script**: See [Appendix: Certificate Generation Script](#appendix-certificate-generation-script) at the bottom of this document.

**Usage:**

```bash
# Make script executable (one-time)
chmod +x ./generate-certs.sh

# Run from ff-proxy base directory
./generate-certs.sh
```

**Generated files:**

```
certs/
├── ca.crt        # CA certificate (for FF-Proxy and Redis)
├── ca.key        # CA private key (keep secure)
├── server.crt    # Redis server certificate
├── server.key    # Redis server private key
├── client.crt    # FF-Proxy client certificate
└── client.key    # FF-Proxy client private key
```

> **Note**: The filenames (`ca.crt`, `client.crt`, `client.key`) match the Helm chart defaults, so you don't need to override them.

---

### Option B: Use Existing Certificates

If you already have certificates from your organization's PKI or a certificate authority:

1. Ensure you have these files:
   - `ca.crt` - CA certificate
   - `client.crt` - Client certificate for FF-Proxy
   - `client.key` - Client private key

2. Verify the certificates:
```bash
# Verify client cert is signed by the CA
openssl verify -CAfile ca.crt client.crt

# Check certificate details
openssl x509 -in client.crt -noout -subject -issuer -dates

# Verify key matches certificate
openssl x509 -noout -modulus -in client.crt | openssl md5
openssl rsa -noout -modulus -in client.key | openssl md5
# Both MD5 hashes should match
```

---

## Step 2: Create Kubernetes Secret

The Kubernetes secret containing certificates must exist **before** deploying FF-Proxy. Create it using your standard deployment process.

### Secret Manifest

Include this in your deployment manifests (GitOps, ArgoCD, etc.):

```yaml
# redis-tls-secret.yaml
apiVersion: v1
kind: Secret
metadata:
  name: redis-tls-secret
  namespace: <your-namespace>
type: Opaque
data:
  ca.crt: <BASE64_ENCODED_CA_CERT>
  client.crt: <BASE64_ENCODED_CLIENT_CERT>
  client.key: <BASE64_ENCODED_CLIENT_KEY>
```

### Generate Base64 Values

```bash
cat ./certs/ca.crt | base64 | tr -d '\n' && echo ""
cat ./certs/client.crt | base64 | tr -d '\n' && echo ""
cat ./certs/client.key | base64 | tr -d '\n' && echo ""
```

### Secret Structure

| Key in Secret | Description |
|---------------|-------------|
| `ca.crt` | CA certificate (base64 encoded) |
| `client.crt` | Client certificate (base64 encoded) |
| `client.key` | Client private key (base64 encoded) |

---

## Step 3: Configure Helm Values

Create an **override values file** to enable mTLS. This file overrides the chart's default values.

> **📁 Example File**: Download [`redis-mtls-values.yaml`](examples/redis-mtls-values.yaml) as a starting template.

### mTLS Override (redis-mtls-values.yaml)

```yaml
# redis-mtls-values.yaml
# Use with: helm upgrade --install ff-proxy harness/ff-proxy -f redis-mtls-values.yaml

global:
  database:
    redis:
      protocol: "rediss"            # Use "rediss" for TLS connections
      hosts:
        - redis:6380                # Your Redis host:port
      tls:
        enabled: true
        secret:
          name: "redis-tls-secret"  # Your K8s secret name
          caCert: "ca.crt"          # Key name for CA cert in your secret
          clientCert: "client.crt"  # Key name for client cert in your secret
          clientKey: "client.key"   # Key name for client key in your secret
        mountPath: "/etc/redis/tls"
        serverName: "redis"         # Optional: SNI server name
```

> **📁 Example File**: Download [`redis-mtls-values.yaml`](examples/redis-mtls-values.yaml)

### Important Notes

1. **Redis Address Protocol**: Use `rediss://` (with double 's') for TLS connections, not `redis://`
2. **enabled: true**: This is REQUIRED - without it, mTLS will not be configured
3. **Secret Key Names**: Must exactly match the keys in your Kubernetes secret

---

## Step 4: Deploy FF-Proxy

```bash
helm upgrade --install ff-proxy harness/ff-proxy \
  --namespace <your-namespace> \
  -f redis-mtls-values.yaml
```

---

## Step 5: Verify the Deployment

### 1. Check Pod Status

```bash
NAMESPACE=ff-proxy

# Get pod names
kubectl get pods -n $NAMESPACE

# Check if pods are running
kubectl get pods -n $NAMESPACE -o wide
```

### 2. Verify Certificate Mounts

```bash
# Get the writer pod name
POD=$(kubectl get pods -n $NAMESPACE -l app.kubernetes.io/component=ff-proxy-writer -o jsonpath='{.items[0].metadata.name}')

# Check certificates are mounted
kubectl exec -n $NAMESPACE $POD -- ls -la /etc/redis/tls/
# Expected output:
# ca.crt
# client.crt
# client.key

# Verify certificate content (first few lines)
kubectl exec -n $NAMESPACE $POD -- head -2 /etc/redis/tls/ca.crt
# Should show: -----BEGIN CERTIFICATE-----
```

### 3. Check Environment Variables

```bash
# Verify mTLS environment variables are set
kubectl exec -n $NAMESPACE $POD -- env | grep REDIS
# Expected output:
# REDIS_ADDRESS=rediss://your-redis-host:6380
# REDIS_MTLS_CA_CERT=/etc/redis/tls/ca.crt
# REDIS_MTLS_CLIENT_CERT=/etc/redis/tls/client.crt
# REDIS_MTLS_CLIENT_KEY=/etc/redis/tls/client.key
```

### 4. Check Logs for Successful Connection

```bash
# Check logs for Redis connection
kubectl logs -n $NAMESPACE $POD --tail=50 | grep -i redis

# Look for: "Redis connection successful" with authMode="mtls"
```

### 5. Health Check

```bash
# Port forward to access health endpoint
kubectl port-forward -n $NAMESPACE $POD 7000:7000 &

# Wait for port forward to establish
sleep 2

# Check health endpoint
curl -s http://localhost:7000/health | jq .

# Expected output:
# {
#   "configStatus": {"state": "SYNCED", ...},
#   "streamStatus": {"state": "CONNECTED", ...},
#   "cacheStatus": "healthy"
# }

# Stop port forward
pkill -f "kubectl port-forward"
```

### Verification Checklist

- [ ] Pods are in `Running` state
- [ ] Certificates are mounted at `/etc/redis/tls/`
- [ ] Environment variables `REDIS_MTLS_*` are set
- [ ] Logs show successful Redis connection with `authMode="mtls"`
- [ ] Health endpoint returns `"cacheStatus": "healthy"`

---

## Configuration Reference

### Helm Values

| Value | Type | Default | Description |
|-------|------|---------|-------------|
| `global.database.redis.protocol` | string | `"redis"` | Protocol (`redis` or `rediss` for TLS) |
| `global.database.redis.hosts` | list | `[]` | Redis host:port list |
| `global.database.redis.tls.enabled` | bool | `false` | Enable Redis mTLS authentication |
| `global.database.redis.tls.secret.name` | string | `""` | Name of Kubernetes secret with certs |
| `global.database.redis.tls.secret.caCert` | string | `"ca.crt"` | Key name for CA certificate in secret |
| `global.database.redis.tls.secret.clientCert` | string | `"client.crt"` | Key name for client certificate |
| `global.database.redis.tls.secret.clientKey` | string | `"client.key"` | Key name for client private key |
| `global.database.redis.tls.mountPath` | string | `"/etc/redis/tls"` | Mount path for certificates |
| `global.database.redis.tls.insecureSkipVerify` | bool | `false` | Skip server cert verification |
| `global.database.redis.tls.serverName` | string | `""` | Server name for TLS SNI |

### Environment Variables (Set Automatically)

| Variable | Example Value | Description |
|----------|---------------|-------------|
| `REDIS_ADDRESS` | `rediss://redis:6380` | Redis connection string |
| `REDIS_MTLS_CA_CERT` | `/etc/redis/tls/ca.crt` | Path to CA certificate |
| `REDIS_MTLS_CLIENT_CERT` | `/etc/redis/tls/client.crt` | Path to client certificate |
| `REDIS_MTLS_CLIENT_KEY` | `/etc/redis/tls/client.key` | Path to client private key |

---

## Troubleshooting

### Common Issues

#### 1. Pod CrashLoopBackOff - Certificate Not Found

**Error in logs:**
```
failed to create redis client: CA certificate not found: /etc/redis/tls/ca.crt
```

**Causes:**
- Secret not created or in wrong namespace
- Secret key names don't match Helm values
- `redis.tls.enabled` not set to `true`

**Solution:**
```bash
# Verify secret exists
kubectl get secret redis-tls-secret -n $NAMESPACE

# Check secret keys
kubectl get secret redis-tls-secret -n $NAMESPACE -o json | jq -r '.data | keys[]'

# Ensure keys match your values.yaml
# caCert, clientCert, clientKey values must match secret keys
```

#### 2. Connection Refused / TLS Handshake Failed

**Error in logs:**
```
failed to connect to redis: tls: failed to verify certificate
```

**Causes:**
- Client certificate not signed by Redis CA
- Wrong CA certificate
- Redis not configured for mTLS

**Solution:**
```bash
# Verify client cert is signed by CA
openssl verify -CAfile ca.crt client.crt
# Should output: client.crt: OK

# Check certificate chain
openssl x509 -in client.crt -noout -issuer
openssl x509 -in ca.crt -noout -subject
# Issuer of client.crt should match Subject of ca.crt
```

#### 3. Unknown Certificate Authority

**Error in logs:**
```
remote error: tls: unknown certificate authority
```

**Causes:**
- Redis doesn't trust the CA that signed the client certificate
- CA certificate mismatch between FF-Proxy and Redis

**Solution:**
- Ensure the same CA certificate is used on both Redis server and FF-Proxy
- Verify Redis `tls-ca-cert-file` points to the correct CA

#### 4. Environment Variables Not Set

**Symptom:** Pod runs but uses password auth instead of mTLS

**Causes:**
- `redis.tls.enabled: false` (or not set)
- Incorrect Helm values file

**Solution:**
```bash
# Check environment variables
kubectl exec -n $NAMESPACE $POD -- env | grep REDIS_MTLS
# If empty, redis.tls.enabled is not true

# Verify Helm values
helm get values ff-proxy -n $NAMESPACE
```

### Debug Logging

Enable debug logging for detailed TLS information:

```yaml
# In values.yaml
logLevel: DEBUG

# Or via environment variable
writer:
  custom_envs:
    - name: LOG_LEVEL
      value: "DEBUG"
```

With debug logging, you'll see:
- Certificate loading details
- TLS configuration
- Connection attempts
- Certificate chain verification

### Useful Commands

```bash
# Get all FF-Proxy resources
kubectl get all -n $NAMESPACE -l app.kubernetes.io/name=ff-proxy

# Describe pod for events
kubectl describe pod $POD -n $NAMESPACE

# Get previous logs (if pod restarted)
kubectl logs $POD -n $NAMESPACE --previous

# Check ConfigMap for env vars
kubectl get configmap -n $NAMESPACE -o yaml | grep -A 20 "REDIS"

# Test connectivity from pod to Redis
kubectl exec -n $NAMESPACE $POD -- nc -zv redis-host 6380
```

---

## Quick Reference

### 1. Create Secret Manifest

```yaml
# redis-tls-secret.yaml
apiVersion: v1
kind: Secret
metadata:
  name: redis-tls-secret
  namespace: <your-namespace>
type: Opaque
data:
  ca.crt: <BASE64_CA_CERT>
  client.crt: <BASE64_CLIENT_CERT>
  client.key: <BASE64_CLIENT_KEY>
```

### 2. Create Override Values

```yaml
# redis-mtls-values.yaml
global:
  database:
    redis:
      protocol: "rediss"
      hosts:
        - redis:6380
      tls:
        enabled: true
        secret:
          name: "redis-tls-secret"
          caCert: "ca.crt"
          clientCert: "client.crt"
          clientKey: "client.key"
        mountPath: "/etc/redis/tls"
        serverName: "redis"
```

### 3. Deploy

```bash
# Apply secret first (through your deployment process)
# Then deploy FF-Proxy with override
helm upgrade --install ff-proxy harness/ff-proxy \
  -n ff-proxy \
  -f redis-mtls-values.yaml
```

### Complete Setup Script (Development/Testing)

```bash
#!/bin/bash
# FF-Proxy Redis mTLS Quick Setup

NAMESPACE="ff-proxy"

# 1. Generate certificates (run from ff-proxy base directory)
chmod +x ./generate-certs.sh
./generate-certs.sh

# 2. Create namespace
kubectl create namespace $NAMESPACE --dry-run=client -o yaml | kubectl apply -f -

# 3. Create Kubernetes secret (for dev/testing only)
kubectl create secret generic redis-tls-secret \
  --from-file=ca.crt=./certs/ca.crt \
  --from-file=client.crt=./certs/client.crt \
  --from-file=client.key=./certs/client.key \
  -n $NAMESPACE --dry-run=client -o yaml | kubectl apply -f -

# 4. Deploy with override values file
helm upgrade --install ff-proxy harness/ff-proxy \
  -n $NAMESPACE \
  -f redis-mtls-values.yaml

# 5. Wait and verify
kubectl wait --for=condition=ready pod \
  -l app.kubernetes.io/name=ff-proxy \
  -n $NAMESPACE --timeout=120s

kubectl logs -n $NAMESPACE -l app.kubernetes.io/component=ff-proxy-writer | grep -i "redis\|mtls"
```

### Verify mTLS is Working

```bash
# Check environment variables are set
POD=$(kubectl get pods -n ff-proxy -l app.kubernetes.io/component=ff-proxy-writer -o jsonpath='{.items[0].metadata.name}')
kubectl exec -n ff-proxy $POD -- env | grep REDIS_MTLS

# Expected output:
# REDIS_MTLS_CA_CERT=/etc/redis/tls/ca.crt
# REDIS_MTLS_CLIENT_CERT=/etc/redis/tls/client.crt
# REDIS_MTLS_CLIENT_KEY=/etc/redis/tls/client.key
```

---

## Support

If you encounter issues not covered in this guide:

1. Enable debug logging and collect logs
2. Verify certificate validity and chain
3. Check Redis server TLS configuration
4. Contact Harness support with logs and configuration details

---

## Appendix: Certificate Generation Script

The certificate generation script is available in this folder as `generate-certs.sh`.

### Usage

```bash
# Make script executable (one-time)
chmod +x ./generate-certs.sh

# Run from ff-proxy base directory
./generate-certs.sh

# Generated files will be in ./certs/
```

### Generated Files

```
certs/
├── ca.crt        # CA certificate
├── ca.key        # CA private key
├── server.crt    # Redis server certificate
├── server.key    # Redis server private key
├── client.crt    # FF-Proxy client certificate
└── client.key    # FF-Proxy client private key
```

### Generate Base64 for Kubernetes Secret

```bash
cat ./certs/ca.crt | base64 | tr -d '\n' && echo ""
cat ./certs/client.crt | base64 | tr -d '\n' && echo ""
cat ./certs/client.key | base64 | tr -d '\n' && echo ""
```

---

*Document Version: 1.0*  
*Last Updated: January 2026*

