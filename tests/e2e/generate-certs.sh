#!/bin/bash

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Certificate validity (in days)
CERT_VALIDITY=365

# Certificate directory - relative to script location
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CERT_DIR="$SCRIPT_DIR/certs"

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}  Redis mTLS Certificate Generator${NC}"
echo -e "${BLUE}  (E2E Test Suite)${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""

# Check if OpenSSL is installed
if ! command -v openssl &> /dev/null; then
    echo -e "${RED}Error: OpenSSL is not installed.${NC}"
    echo "Please install OpenSSL and try again."
    exit 1
fi

# Create certs directory
echo -e "${YELLOW}Creating certificate directory...${NC}"
mkdir -p "$CERT_DIR"

# Clean up old Redis mTLS certificates (but preserve cert.crt if it exists separately)
if [ "$(ls -A $CERT_DIR 2>/dev/null)" ]; then
    echo -e "${YELLOW}Removing old Redis mTLS certificates...${NC}"
    rm -f "$CERT_DIR"/redis-*.crt "$CERT_DIR"/redis-*.key
    # Only remove cert.crt if it's a symlink to redis-ca.crt
    if [ -L "$CERT_DIR/cert.crt" ]; then
        rm -f "$CERT_DIR/cert.crt"
    fi
fi

echo ""
echo -e "${GREEN}Step 1: Generating Certificate Authority (CA)${NC}"
echo "-------------------------------------------"

# Generate CA private key
openssl genrsa -out "$CERT_DIR/redis-ca.key" 4096

# Generate CA certificate
openssl req -new -x509 -days $CERT_VALIDITY -key "$CERT_DIR/redis-ca.key" \
    -out "$CERT_DIR/redis-ca.crt" \
    -subj "/C=US/ST=California/L=San Francisco/O=FF-Proxy/OU=E2E-Testing/CN=FF-Proxy-CA"

echo -e "${GREEN}✓ CA certificate generated${NC}"
echo ""

echo -e "${GREEN}Step 2: Generating Redis Server Certificate${NC}"
echo "-------------------------------------------"

# Generate server private key
openssl genrsa -out "$CERT_DIR/redis-server.key" 4096

# Generate server certificate signing request (CSR)
openssl req -new -key "$CERT_DIR/redis-server.key" \
    -out "$CERT_DIR/redis-server.csr" \
    -subj "/C=US/ST=California/L=San Francisco/O=FF-Proxy/OU=Redis/CN=redis-mtls"

# Create server certificate extensions file
cat > "$CERT_DIR/redis-server.ext" <<EOF
authorityKeyIdentifier=keyid,issuer
basicConstraints=CA:FALSE
keyUsage = digitalSignature, nonRepudiation, keyEncipherment, dataEncipherment
subjectAltName = @alt_names

[alt_names]
DNS.1 = redis-mtls
DNS.2 = localhost
IP.1 = 127.0.0.1
EOF

# Sign server certificate with CA
openssl x509 -req -in "$CERT_DIR/redis-server.csr" \
    -CA "$CERT_DIR/redis-ca.crt" \
    -CAkey "$CERT_DIR/redis-ca.key" \
    -CAcreateserial \
    -out "$CERT_DIR/redis-server.crt" \
    -days $CERT_VALIDITY \
    -sha256 \
    -extfile "$CERT_DIR/redis-server.ext"

echo -e "${GREEN}✓ Server certificate generated${NC}"
echo ""

echo -e "${GREEN}Step 3: Generating ff-proxy Client Certificate${NC}"
echo "-------------------------------------------"

# Generate client private key
openssl genrsa -out "$CERT_DIR/redis-client.key" 4096

# Generate client certificate signing request (CSR)
openssl req -new -key "$CERT_DIR/redis-client.key" \
    -out "$CERT_DIR/redis-client.csr" \
    -subj "/C=US/ST=California/L=San Francisco/O=FF-Proxy/OU=Client/CN=ff-proxy-client"

# Create client certificate extensions file
cat > "$CERT_DIR/redis-client.ext" <<EOF
authorityKeyIdentifier=keyid,issuer
basicConstraints=CA:FALSE
keyUsage = digitalSignature, nonRepudiation, keyEncipherment, dataEncipherment
extendedKeyUsage = clientAuth
EOF

# Sign client certificate with CA
openssl x509 -req -in "$CERT_DIR/redis-client.csr" \
    -CA "$CERT_DIR/redis-ca.crt" \
    -CAkey "$CERT_DIR/redis-ca.key" \
    -CAcreateserial \
    -out "$CERT_DIR/redis-client.crt" \
    -days $CERT_VALIDITY \
    -sha256 \
    -extfile "$CERT_DIR/redis-client.ext"

echo -e "${GREEN}✓ Client certificate generated${NC}"
echo ""

# Clean up temporary files
echo -e "${YELLOW}Cleaning up temporary files...${NC}"
rm -f "$CERT_DIR"/redis-*.csr "$CERT_DIR"/redis-*.ext "$CERT_DIR"/*.srl

# Set appropriate permissions
chmod 600 "$CERT_DIR"/redis-*.key
chmod 644 "$CERT_DIR"/redis-*.crt

# Handle platform auth certificate requirement
# The E2E test helpers expect cert.crt for platform authentication
# Create a symlink from redis-ca.crt to cert.crt if cert.crt doesn't exist
if [ ! -f "$CERT_DIR/cert.crt" ]; then
    echo -e "${YELLOW}Creating symlink for platform auth certificate...${NC}"
    ln -sf redis-ca.crt "$CERT_DIR/cert.crt"
    echo -e "${GREEN}✓ Created symlink: cert.crt -> redis-ca.crt${NC}"
else
    echo -e "${YELLOW}⚠️  cert.crt already exists, skipping symlink creation${NC}"
    echo "  (This is expected if you have a separate platform auth certificate)"
fi

echo ""
echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}  Certificate Generation Complete!${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""
echo "Generated certificates in $CERT_DIR:"
echo ""
echo "  Certificate Authority:"
echo "    - redis-ca.crt (CA certificate)"
echo "    - redis-ca.key (CA private key)"
echo ""
echo "  Redis Server:"
echo "    - redis-server.crt (Server certificate)"
echo "    - redis-server.key (Server private key)"
echo ""
echo "  ff-proxy Client:"
echo "    - redis-client.crt (Client certificate)"
echo "    - redis-client.key (Client private key)"
echo ""
echo "  Platform Auth (symlink):"
echo "    - cert.crt -> redis-ca.crt (for E2E test helpers)"
echo ""
echo -e "${YELLOW}⚠️  Important:${NC}"
echo "  - These certificates are valid for $CERT_VALIDITY days"
echo "  - They are self-signed and suitable for E2E testing only"
echo "  - For production, use certificates from a trusted CA"
echo ""

