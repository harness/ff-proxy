# Redis mTLS Authentication Example

This example demonstrates how to run ff-proxy with Redis using mutual TLS (mTLS) authentication. This setup provides secure, encrypted communication between ff-proxy and Redis with certificate-based client authentication.

## 🔒 What is mTLS?

Mutual TLS (mTLS) is an authentication method where both the client and server verify each other's identity using certificates. This provides:
- **Encryption**: All data between ff-proxy and Redis is encrypted
- **Authentication**: Redis verifies the client's identity via certificates
- **Authorization**: Only clients with valid certificates can connect

## 📋 Prerequisites

- Docker and Docker Compose installed
- OpenSSL (for generating certificates)
- A valid Harness Feature Flags API key (proxy key)

## 🚀 Quick Start

### 1. Generate TLS Certificates

Run the provided script to generate all required certificates:

```bash
./generate-certs.sh
```

This creates:
- `certs/ca.crt` & `certs/ca.key` - Certificate Authority
- `certs/server.crt` & `certs/server.key` - Redis server certificates
- `certs/client.crt` & `certs/client.key` - ff-proxy client certificates

**Note**: These are self-signed certificates for development/testing only. For production, use certificates from a trusted CA.

### 2. Configure Your Proxy Key

Edit `docker-compose.yml` and replace `<your-proxy-key-here>` with your actual Harness Feature Flags API key:

```yaml
- PROXY_KEY=<your-proxy-key-here>
```

### 3. Start the Services

```bash
docker-compose up
```

This will start:
- **Redis with mTLS**: Running on port `6380` (TLS-enabled)
- **Primary ff-proxy**: Running on port `7001`
- **Replica ff-proxy**: Running on port `7002`

### 4. Verify the Setup

**First, verify Redis is running with TLS enabled:**

```bash
# Check Redis logs - should show TLS port 6380
docker-compose logs redis-mtls | grep -E "(port|tls|Ready)"

# Expected output:
# Running mode=standalone, port=6380.
# Ready to accept connections tls
```

**If you see `port=6379` or `connections tcp` instead, Redis is not loading the config file.**
See the [Troubleshooting](#-redis-starting-on-wrong-port-6379-instead-of-6380) section.

**Then check that ff-proxy can connect to Redis:**

```bash
# Check primary logs
docker-compose logs primary

# Check replica logs
docker-compose logs replica

# You should see successful connection messages
```

Test the API:

```bash
# Health check
curl http://localhost:7001/health

# Client authentication (replace with your SDK key)
curl -X GET "http://localhost:7001/client/auth" \
  -H "Authorization: Bearer <your-sdk-key>"
```

## 🔧 Configuration Details

### Redis mTLS Configuration

The example includes a `redis.conf` file with the following mTLS configuration:

```conf
# Disable non-TLS connections
port 0

# Enable TLS on port 6380
tls-port 6380

# Server certificate and key
tls-cert-file /certs/server.crt
tls-key-file /certs/server.key

# CA certificate for client verification
tls-ca-cert-file /certs/ca.crt

# Require client certificates (mTLS)
tls-auth-clients yes

# TLS protocol versions
tls-protocols "TLSv1.2 TLSv1.3"

# Additional security settings
tls-prefer-server-ciphers yes

# Logging
loglevel notice
```

**Key Configuration Explained:**

| Setting | Value | Purpose |
|---------|-------|---------|
| `port` | `0` | Disables non-TLS connections entirely |
| `tls-port` | `6380` | Enables TLS on a dedicated port |
| `tls-cert-file` | `/certs/server.crt` | Redis server's certificate for proving identity |
| `tls-key-file` | `/certs/server.key` | Redis server's private key |
| `tls-ca-cert-file` | `/certs/ca.crt` | CA certificate to verify client certificates |
| `tls-auth-clients` | `yes` | **Enforces mTLS** - requires client certificates |
| `tls-protocols` | `"TLSv1.2 TLSv1.3"` | Only allows secure TLS versions |
| `tls-prefer-server-ciphers` | `yes` | Server chooses cipher suite (better security) |
| `loglevel` | `notice` | Appropriate logging level for production |

### ff-proxy mTLS Environment Variables

| Variable | Value | Description |
|----------|-------|-------------|
| `REDIS_ADDRESS` | `redis-mtls:6380` | Redis server address with TLS port |
| `REDIS_TLS_ENABLED` | `true` | Enable TLS for Redis connection |
| `REDIS_TLS_MODE` | `mtls` | Use mutual TLS authentication |
| `REDIS_TLS_CA_CERT` | `/certs/ca.crt` | Path to CA certificate |
| `REDIS_TLS_CLIENT_CERT` | `/certs/client.crt` | Path to client certificate |
| `REDIS_TLS_CLIENT_KEY` | `/certs/client.key` | Path to client private key |

## 🔐 TLS Modes

ff-proxy supports two TLS modes:

### 1. **TLS Mode** (`REDIS_TLS_MODE=tls`)
- Server authentication only
- Redis verifies its identity to the client
- Client certificate not required
- Use when Redis has TLS but doesn't require client certificates

### 2. **mTLS Mode** (`REDIS_TLS_MODE=mtls`)
- Mutual authentication (this example)
- Both Redis and client verify each other's identity
- Client certificate required
- Highest security level

## 📂 Directory Structure

```
examples/redis_mtls/
├── README.md                 # This file
├── docker-compose.yml        # Docker Compose configuration
├── generate-certs.sh         # Certificate generation script
├── redis.conf                # Redis mTLS configuration
├── Makefile                  # Convenience commands
└── certs/                    # Generated certificates (gitignored)
    ├── ca.crt               # Certificate Authority certificate
    ├── ca.key               # Certificate Authority private key
    ├── server.crt           # Redis server certificate
    ├── server.key           # Redis server private key
    ├── client.crt           # ff-proxy client certificate
    └── client.key           # ff-proxy client private key
```

## 🧪 Testing Different Scenarios

### Test Connection Without Certificates

Try connecting without certificates to verify mTLS is enforced:

```bash
docker run --rm --network redis_mtls_default redis:7-alpine \
  redis-cli -h redis-mtls -p 6380 ping
# Should fail: "Error: Connection reset by peer"
```

### Test Connection With Certificates

Connect with valid certificates:

```bash
docker run --rm --network redis_mtls_default \
  -v $(pwd)/certs:/certs:ro \
  redis:7-alpine \
  redis-cli --tls \
    --cert /certs/client.crt \
    --key /certs/client.key \
    --cacert /certs/ca.crt \
    -h redis-mtls -p 6380 ping
# Should succeed: "PONG"
```

## 🛠️ Troubleshooting

### Certificate Errors

If you see certificate verification errors:

```bash
# Regenerate all certificates
rm -rf certs/
./generate-certs.sh
docker-compose down
docker-compose up
```

### Redis Starting on Wrong Port (6379 instead of 6380)

**Symptoms:**
- Redis logs show: `Running mode=standalone, port=6379.`
- Redis logs show: `Ready to accept connections tcp` (instead of `tls`)
- Health check fails: `dependency failed to start: container ff-proxy-redis-mtls is unhealthy`

**Cause:**
Redis is not loading the `redis.conf` file, so it's using default configuration (port 6379, no TLS).

**Solution:**

1. **Verify redis.conf exists:**
   ```bash
   ls -la redis.conf
   # Should show the file exists
   ```

2. **Check if redis.conf is mounted correctly:**
   ```bash
   docker-compose exec redis-mtls ls -la /tmp/redis.conf
   # Should show the file exists in the container
   ```

3. **Verify redis.conf contents in container:**
   ```bash
   docker-compose exec redis-mtls cat /tmp/redis.conf
   # Should show your TLS configuration (port 0, tls-port 6380, etc.)
   ```

4. **If the file is missing or wrong, restart with clean state:**
   ```bash
   docker-compose down -v
   docker-compose up -d redis-mtls
   docker-compose logs redis-mtls
   # Should now show: "Running mode=standalone, port=6380." and "Ready to accept connections tls"
   ```

5. **If still not working, check file permissions:**
   ```bash
   chmod 644 redis.conf
   docker-compose restart redis-mtls
   ```

**Expected Output (when working correctly):**
```
ff-proxy-redis-mtls  | Running mode=standalone, port=6380.
ff-proxy-redis-mtls  | Ready to accept connections tls
```

### Connection Refused

If ff-proxy can't connect to Redis:

1. Check Redis is running: `docker-compose logs redis-mtls`
2. Verify certificates are mounted: `docker-compose exec primary ls -la /certs`
3. Check Redis TLS configuration: `docker-compose exec redis-mtls cat /tmp/redis.conf`
4. Verify Redis is listening on TLS port: Look for `Ready to accept connections tls` in logs

### Certificate Expiry

The generated certificates expire in 365 days. Regenerate them before expiry:

```bash
./generate-certs.sh
docker-compose restart
```

## 🔄 Switching Between TLS and mTLS

To switch from mTLS to regular TLS:

### Option 1: Modify redis.conf

Edit `redis.conf` and change:
```conf
# FROM (mTLS - requires client certificates):
tls-auth-clients yes

# TO (TLS only - server authentication only):
tls-auth-clients no
```

### Option 2: Modify docker-compose.yml

Change the ff-proxy environment variable:
```yaml
# FROM:
- REDIS_TLS_MODE=mtls

# TO:
- REDIS_TLS_MODE=tls
```

Then restart:
```bash
docker-compose restart
```

### Creating a Custom redis.conf

If you need to customize the Redis configuration, create your own `redis.conf`:

```bash
cat > redis.conf <<'EOF'
# Disable non-TLS connections
port 0

# Enable TLS on port 6380
tls-port 6380

# Server certificate and key
tls-cert-file /certs/server.crt
tls-key-file /certs/server.key

# CA certificate for client verification
tls-ca-cert-file /certs/ca.crt

# Require client certificates (mTLS)
tls-auth-clients yes

# TLS protocol versions
tls-protocols "TLSv1.2 TLSv1.3"

# Additional security settings
tls-prefer-server-ciphers yes

# Logging
loglevel notice

# Optional: Add your custom settings below
# maxmemory 256mb
# maxmemory-policy allkeys-lru
# save 900 1
# save 300 10
EOF
```

## 📖 Additional Resources

- [Harness FF Proxy Documentation](../../README.md)
- [Redis TLS Documentation](https://redis.io/docs/manual/security/encryption/)
- [TLS Configuration Guide](../../docs/tls.md)
- [Redis Cache Documentation](../../docs/redis_cache.md)

## ⚠️ Production Considerations

For production deployments:

1. **Use Proper Certificates**: Get certificates from a trusted CA (Let's Encrypt, DigiCert, etc.)
2. **Secure Certificate Storage**: Use secrets management (Kubernetes Secrets, HashiCorp Vault, AWS Secrets Manager)
3. **Certificate Rotation**: Implement automated certificate rotation
4. **Monitor Expiry**: Set up alerts for certificate expiration
5. **Network Security**: Use firewalls and network policies to restrict access
6. **Audit Logging**: Enable audit logs for certificate usage
7. **Backup Certificates**: Securely backup certificates and keys

## 🧹 Cleanup

To stop and remove all containers and volumes:

```bash
docker-compose down -v
```

To remove generated certificates:

```bash
rm -rf certs/
```

## 📝 Notes

- The example uses self-signed certificates suitable for development/testing only
- Certificate files are excluded from git via `.gitignore`
- Redis runs on port `6380` (not the default `6379`) to indicate TLS usage
- Both primary and replica proxies use the same client certificates
- The CA certificate must be present for both server and client verification

