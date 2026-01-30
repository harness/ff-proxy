 # Redis mTLS Authentication Example

This example demonstrates how to run ff-proxy with Redis using mutual TLS (mTLS) authentication. This setup provides secure, encrypted communication between ff-proxy and Redis with certificate-based client authentication.

## 🔒 What is mTLS?

Mutual TLS (mTLS) is an authentication method where both the client and server verify each other's identity using certificates. Unlike regular TLS (which only verifies the server), mTLS requires both parties to present valid certificates. This provides:
- **Encryption**: All data between ff-proxy and Redis is encrypted
- **Mutual Authentication**: Both Redis and ff-proxy verify each other's identity via certificates
- **Strong Authorization**: Only clients with valid certificates signed by the trusted CA can connect

## 📋 Prerequisites

- Docker and Docker Compose installed
- OpenSSL (for generating certificates)
- A valid Harness Feature Flags API key (proxy key)

## 🚀 Quick Start

Follow these steps to run the mTLS example:

### 1. Navigate to the Example Directory

```bash
cd examples/redis_mtls
```

### 2. Run Setup Script

Run the setup script to automatically generate `redis.conf` and mTLS certificates:

```bash
./setup-mtls-testing.sh
```

This script will:
- **Create `redis.conf`** with mTLS configuration (port 0, TLS on 6380, client certificate authentication required)
- **Generate all required certificates** (CA, server, and client certificates)
- **Validate** that all files are present

**Note**: 
- The script is idempotent - it won't overwrite existing files unless you use `--force`
- `redis.conf` is automatically created - you don't need to create it manually
- Certificates are self-signed and suitable for development/testing only. For production, use certificates from a trusted CA.

### 3. Configure Your Proxy Key

Edit `docker-compose.yml` and replace `<your-proxy-key-here>` with your actual Harness Feature Flags API key:

```yaml
- PROXY_KEY=<your-proxy-key-here>
```

### 4. Start the Services

```bash
docker compose up --build
```

This will start:
- **Redis with mTLS**: Running on port `6380` (TLS-enabled, client certificates required)
- **Primary ff-proxy**: Running on port `7001`
- **Replica ff-proxy**: Running on port `7002`

### 5. Verify mTLS is Working

**Step 5a: Verify Redis is running with mTLS enabled:**

```bash
# Check Redis logs - should show TLS port 6380
docker-compose logs redis-mtls | grep -E "(port|tls|Ready)"

# Expected output:
# Running mode=standalone, port=6380.
# Ready to accept connections tls
```

**If you see `port=6379` or `connections tcp` instead, Redis is not loading the config file.**
See the [Troubleshooting](#-redis-starting-on-wrong-port-6379-instead-of-6380) section.

**Step 5b: Verify mTLS enforcement (connection without certs must fail):**

```bash
docker run --rm --network redis_mtls_ff-proxy-network redis:7-alpine \
  redis-cli -h redis-mtls -p 6380 ping
# Expected: Connection fails (proves mTLS is enforced)
```

**Step 5c: Verify mTLS connection (connection with certs must succeed):**

```bash
docker run --rm --network redis_mtls_ff-proxy-network \
  -v $(pwd)/certs:/certs:ro \
  redis:7-alpine \
  redis-cli --tls \
    --cert /certs/client.crt \
    --key /certs/client.key \
    --cacert /certs/ca.crt \
    -h redis-mtls -p 6380 ping
# Expected: "PONG" (proves mTLS works)
```

**Step 5d: Verify ff-proxy can connect to Redis:**

```bash
# Check primary logs
docker-compose logs primary | grep -i redis

# Check replica logs
docker-compose logs replica | grep -i redis

# You should see successful connection messages
```

**Step 5e: Test the API:**

```bash
# Health check
curl http://localhost:7001/health

# Client authentication (replace with your SDK key)
curl -X GET "http://localhost:7001/client/auth" \
  -H "Authorization: Bearer <your-sdk-key>"
```

### 6. Cleanup (when done)

```bash
docker-compose down -v
rm -rf certs/
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
| `REDIS_ADDRESS` | `rediss://redis-mtls:6380` | Redis server address with TLS port (rediss:// protocol required for TLS) |
| `REDIS_MTLS_CA_CERT` | `/certs/ca.crt` | Path to CA certificate (required for mTLS) |
| `REDIS_MTLS_CLIENT_CERT` | `/certs/client.crt` | Path to client certificate (required for mTLS) |
| `REDIS_MTLS_CLIENT_KEY` | `/certs/client.key` | Path to client private key (required for mTLS) |

**Note:** When all three mTLS certificate paths are provided, ff-proxy automatically uses mTLS authentication. No additional flags or mode settings are required.

**Local vs Production:**
- **This Example (Local)**: Certificates are mounted via Docker volumes from `./certs` directory
- **Production (Helm/K8s)**: Certificates come from Kubernetes Secrets mounted at `/etc/redis/tls` (configurable). See [Redis Cache Documentation](../../docs/redis_cache.md) for Helm deployment details.

## 📂 Directory Structure

```
examples/redis_mtls/
├── README.md                      # This file (Docker Compose setup)
├── redis-mtls-k8s-setup-guide.md  # Kubernetes/Helm setup guide
├── docker-compose.yml             # Docker Compose configuration
├── generate-certs.sh              # Certificate generator script
├── setup-mtls-testing.sh          # Full setup script (generates certs + redis.conf)
├── redis.conf                     # Redis mTLS configuration
├── Makefile                       # Convenience commands
└── certs/                         # Generated certificates (gitignored)
    ├── ca.crt               # Certificate Authority certificate
    ├── ca.key               # Certificate Authority private key
    ├── server.crt           # Redis server certificate
    ├── server.key           # Redis server private key
    ├── client.crt           # ff-proxy client certificate
    └── client.key           # ff-proxy client private key
```

## 🧪 Verifying mTLS is Enforced

### How to Know It's mTLS (Not Just TLS)

**Mutual TLS (mTLS) requires BOTH server and client certificates.** Regular TLS only requires a server certificate. To prove this example uses mTLS, you must verify:

1. **Connection WITHOUT client certificates MUST fail** - This proves Redis requires client certificates
2. **Connection WITH client certificates MUST succeed** - This proves the certificates work

If both conditions are true, you have true mTLS. If connections without client certificates succeed, you only have TLS (not mTLS).

### Test 1: Negative Test (Must Fail)

**Purpose:** Prove that Redis rejects connections without client certificates.

```bash
docker run --rm --network redis_mtls_ff-proxy-network redis:7-alpine \
  redis-cli -h redis-mtls -p 6380 ping
```

**Expected Result:**
```
Error: Connection reset by peer
```

**What this proves:**
- Redis is enforcing client certificate authentication
- Clients without valid certificates cannot connect
- This is the key difference between TLS and mTLS

**If this test succeeds (connects without certs):**
- Redis is NOT enforcing mTLS
- Check that `tls-auth-clients yes` is in `redis.conf`
- Verify Redis loaded the config file correctly

### Test 2: Positive Test (Must Succeed)

**Purpose:** Prove that valid client certificates allow connection.

```bash
docker run --rm --network redis_mtls_ff-proxy-network \
  -v $(pwd)/certs:/certs:ro \
  redis:7-alpine \
  redis-cli --tls \
    --cert /certs/client.crt \
    --key /certs/client.key \
    --cacert /certs/ca.crt \
    -h redis-mtls -p 6380 ping
```

**Expected Result:**
```
PONG
```

**What this proves:**
- Client certificates are valid and properly signed
- Redis accepts the client certificate
- mTLS connection is working correctly

### Test 3: Additional Operations (Optional)

Verify that you can perform Redis operations over mTLS:

```bash
docker run --rm --network redis_mtls_ff-proxy-network \
  -v $(pwd)/certs:/certs:ro \
  redis:7-alpine \
  sh -c "
    redis-cli --tls --cert /certs/client.crt --key /certs/client.key --cacert /certs/ca.crt -h redis-mtls -p 6380 SET testkey testvalue &&
    redis-cli --tls --cert /certs/client.crt --key /certs/client.key --cacert /certs/ca.crt -h redis-mtls -p 6380 GET testkey
  "
```

**Expected Result:**
```
OK
testvalue
```

### Summary: Both Tests Must Pass

- ✅ **Test 1 fails** = mTLS is enforced (good!)
- ✅ **Test 2 succeeds** = mTLS is working (good!)
- ❌ **Test 1 succeeds** = mTLS is NOT enforced (configuration error)
- ❌ **Test 2 fails** = Certificate or configuration issue

## 🛠️ Troubleshooting

### Redis Connection Failures

If ff-proxy cannot connect to Redis, verify the following:

1. **Check that `redis.conf` exists:**
   ```bash
   ls -la redis.conf
   # Should show the file exists
   ```

2. **Verify `redis.conf` has mTLS settings:**
   ```bash
   grep -E "port 0|tls-port 6380|tls-auth-clients yes" redis.conf
   # Should show:
   # port 0
   # tls-port 6380
   # tls-auth-clients yes
   ```

3. **Check that certificates exist:**
   ```bash
   ls -la certs/
   # Should show: ca.crt, ca.key, server.crt, server.key, client.crt, client.key
   ```

4. **Verify Redis is using TLS port:**
   ```bash
   docker-compose logs redis-mtls | grep -E "port|tls"
   # Should show: "port=6380" and "Ready to accept connections tls"
   ```

5. **Confirm clients connect to `redis:6380` (not 6379):**
   - Check `docker-compose.yml` - `REDIS_ADDRESS` should be `rediss://redis-mtls:6380` (rediss:// protocol required for TLS)
   - Redis plaintext is disabled (port 0) and TLS is on 6380
   - Verify mTLS environment variables are set: `REDIS_MTLS_CA_CERT`, `REDIS_MTLS_CLIENT_CERT`, `REDIS_MTLS_CLIENT_KEY`

If files are missing, run:
```bash
./setup-mtls-testing.sh
```

### Incomplete mTLS Configuration

**Error:**
```
invalid redis config: incomplete mTLS configuration: all three certificate paths are required when any are set. Missing: REDIS_MTLS_CLIENT_CERT, REDIS_MTLS_CLIENT_KEY
```

**Cause:**
ff-proxy requires all three mTLS certificate paths to be set when any are provided. This prevents silent downgrades to password authentication.

**Solution:**
Ensure all three environment variables are set in your `docker-compose.yml`:
```yaml
- REDIS_MTLS_CA_CERT=/certs/ca.crt
- REDIS_MTLS_CLIENT_CERT=/certs/client.crt
- REDIS_MTLS_CLIENT_KEY=/certs/client.key
```

**Verification:**
```bash
# Check environment variables in container
docker-compose exec primary env | grep REDIS_MTLS

# Should show all three:
# REDIS_MTLS_CA_CERT=/certs/ca.crt
# REDIS_MTLS_CLIENT_CERT=/certs/client.crt
# REDIS_MTLS_CLIENT_KEY=/certs/client.key
```

If you don't want to use mTLS, remove all three environment variables (or set them to empty strings).

### Certificate Errors

If you see certificate verification errors:

```bash
# Regenerate all certificates
./setup-mtls-testing.sh --force
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
./setup-mtls-testing.sh --force
docker-compose restart
```

### Certificate Path or Mount Issues

**Symptoms:**
- ff-proxy fails to connect: "failed to read CA certificate" or "certificate not found"
- Redis health check fails
- Container logs show certificate errors

**Verification Steps:**

1. **Check certificates exist locally:**
   ```bash
   ls -la certs/
   # Should show: ca.crt, ca.key, server.crt, server.key, client.crt, client.key
   ```

2. **Verify certificates are mounted in containers:**
   ```bash
   # Check Redis container
   docker-compose exec redis-mtls ls -la /certs/
   
   # Check ff-proxy container
   docker-compose exec primary ls -la /certs/
   ```

3. **Verify environment variables point to correct paths:**
   ```bash
   docker-compose exec primary env | grep REDIS_MTLS
   # Should show:
   # REDIS_MTLS_CA_CERT=/certs/ca.crt
   # REDIS_MTLS_CLIENT_CERT=/certs/client.crt
   # REDIS_MTLS_CLIENT_KEY=/certs/client.key
   ```

4. **Check volume mounts in docker-compose.yml:**
   ```bash
   grep -A 2 "volumes:" docker-compose.yml
   # Should show: - ./certs:/certs:ro
   ```

**Common Fixes:**
- If certs are missing: Run `./setup-mtls-testing.sh`
- If paths don't match: Ensure `docker-compose.yml` uses `/certs/` (container path) and `./certs` (host path)
- If permissions are wrong: Run `chmod 644 certs/*.crt && chmod 600 certs/*.key`


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

### Stop Services

To stop all services but keep data:

```bash
docker-compose down
```

### Complete Cleanup

To stop services, remove volumes, and delete certificates:

```bash
# Stop and remove containers and volumes
docker-compose down -v

# Remove generated certificates and config
rm -rf certs/ redis.conf
```

### Start Fresh

To start completely from scratch after cleanup:

```bash
# Regenerate everything
./setup-mtls-testing.sh

# Start services
docker-compose up -d
```

## 📝 Notes

- The example uses self-signed certificates suitable for development/testing only
- Certificate files are excluded from git via `.gitignore`
- Redis runs on port `6380` (not the default `6379`) to indicate TLS usage
- Both primary and replica proxies use the same client certificates
- The CA certificate must be present for both server and client verification
- ff-proxy supports **mTLS only** for Redis authentication - all three certificate paths must be provided

