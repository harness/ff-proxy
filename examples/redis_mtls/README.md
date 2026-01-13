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

Check that ff-proxy can connect to Redis:

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

The Redis server is configured with:

```conf
port 0                              # Disable non-TLS port
tls-port 6380                       # Enable TLS on port 6380
tls-cert-file /certs/server.crt     # Server certificate
tls-key-file /certs/server.key      # Server private key
tls-ca-cert-file /certs/ca.crt      # CA certificate
tls-auth-clients yes                # Require client certificates (mTLS)
tls-protocols "TLSv1.2 TLSv1.3"    # Supported TLS versions
```

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
├── redis.conf                # Redis configuration (optional)
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

### Connection Refused

If ff-proxy can't connect to Redis:

1. Check Redis is running: `docker-compose logs redis-mtls`
2. Verify certificates are mounted: `docker-compose exec primary ls -la /certs`
3. Check Redis TLS configuration: `docker-compose exec redis-mtls cat /tmp/redis.conf`

### Certificate Expiry

The generated certificates expire in 365 days. Regenerate them before expiry:

```bash
./generate-certs.sh
docker-compose restart
```

## 🔄 Switching Between TLS and mTLS

To switch from mTLS to regular TLS:

1. In `docker-compose.yml`, change `REDIS_TLS_MODE=mtls` to `REDIS_TLS_MODE=tls`
2. In Redis command, change `tls-auth-clients yes` to `tls-auth-clients no`
3. Restart: `docker-compose restart`

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

