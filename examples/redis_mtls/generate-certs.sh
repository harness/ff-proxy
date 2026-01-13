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

# Certificate directory
CERT_DIR="./certs"

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}  Redis mTLS Certificate Generator${NC}"
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

# Clean up old certificates
if [ "$(ls -A $CERT_DIR)" ]; then
    echo -e "${YELLOW}Removing old certificates...${NC}"
    rm -f "$CERT_DIR"/*
fi

echo ""
echo -e "${GREEN}Step 1: Generating Certificate Authority (CA)${NC}"
echo "-------------------------------------------"

# Generate CA private key
openssl genrsa -out "$CERT_DIR/ca.key" 4096

# Generate CA certificate
openssl req -new -x509 -days $CERT_VALIDITY -key "$CERT_DIR/ca.key" \
    -out "$CERT_DIR/ca.crt" \
    -subj "/C=US/ST=California/L=San Francisco/O=FF-Proxy/OU=Development/CN=FF-Proxy-CA"

echo -e "${GREEN}✓ CA certificate generated${NC}"
echo ""

echo -e "${GREEN}Step 2: Generating Redis Server Certificate${NC}"
echo "-------------------------------------------"

# Generate server private key
openssl genrsa -out "$CERT_DIR/server.key" 4096

# Generate server certificate signing request (CSR)
openssl req -new -key "$CERT_DIR/server.key" \
    -out "$CERT_DIR/server.csr" \
    -subj "/C=US/ST=California/L=San Francisco/O=FF-Proxy/OU=Redis/CN=redis-mtls"

# Create server certificate extensions file
cat > "$CERT_DIR/server.ext" <<EOF
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
openssl x509 -req -in "$CERT_DIR/server.csr" \
    -CA "$CERT_DIR/ca.crt" \
    -CAkey "$CERT_DIR/ca.key" \
    -CAcreateserial \
    -out "$CERT_DIR/server.crt" \
    -days $CERT_VALIDITY \
    -sha256 \
    -extfile "$CERT_DIR/server.ext"

echo -e "${GREEN}✓ Server certificate generated${NC}"
echo ""

echo -e "${GREEN}Step 3: Generating ff-proxy Client Certificate${NC}"
echo "-------------------------------------------"

# Generate client private key
openssl genrsa -out "$CERT_DIR/client.key" 4096

# Generate client certificate signing request (CSR)
openssl req -new -key "$CERT_DIR/client.key" \
    -out "$CERT_DIR/client.csr" \
    -subj "/C=US/ST=California/L=San Francisco/O=FF-Proxy/OU=Client/CN=ff-proxy-client"

# Create client certificate extensions file
cat > "$CERT_DIR/client.ext" <<EOF
authorityKeyIdentifier=keyid,issuer
basicConstraints=CA:FALSE
keyUsage = digitalSignature, nonRepudiation, keyEncipherment, dataEncipherment
extendedKeyUsage = clientAuth
EOF

# Sign client certificate with CA
openssl x509 -req -in "$CERT_DIR/client.csr" \
    -CA "$CERT_DIR/ca.crt" \
    -CAkey "$CERT_DIR/ca.key" \
    -CAcreateserial \
    -out "$CERT_DIR/client.crt" \
    -days $CERT_VALIDITY \
    -sha256 \
    -extfile "$CERT_DIR/client.ext"

echo -e "${GREEN}✓ Client certificate generated${NC}"
echo ""

# Clean up temporary files
echo -e "${YELLOW}Cleaning up temporary files...${NC}"
rm -f "$CERT_DIR"/*.csr "$CERT_DIR"/*.ext "$CERT_DIR"/*.srl

# Set appropriate permissions
chmod 600 "$CERT_DIR"/*.key
chmod 644 "$CERT_DIR"/*.crt

echo ""
echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}  Certificate Generation Complete!${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""
echo "Generated certificates:"
echo ""
echo "  Certificate Authority:"
echo "    - $CERT_DIR/ca.crt (CA certificate)"
echo "    - $CERT_DIR/ca.key (CA private key)"
echo ""
echo "  Redis Server:"
echo "    - $CERT_DIR/server.crt (Server certificate)"
echo "    - $CERT_DIR/server.key (Server private key)"
echo ""
echo "  ff-proxy Client:"
echo "    - $CERT_DIR/client.crt (Client certificate)"
echo "    - $CERT_DIR/client.key (Client private key)"
echo ""
echo -e "${YELLOW}⚠️  Important:${NC}"
echo "  - These certificates are valid for $CERT_VALIDITY days"
echo "  - They are self-signed and suitable for development/testing only"
echo "  - For production, use certificates from a trusted CA"
echo ""
echo -e "${BLUE}Next steps:${NC}"
echo "  1. Update PROXY_KEY in docker-compose.yml"
echo "  2. Run: docker-compose up"
echo ""

