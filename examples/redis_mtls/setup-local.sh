#!/bin/bash

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# Flags
FORCE=false
if [[ "${1:-}" == "--force" ]]; then
    FORCE=true
fi

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}  Redis mTLS Local Setup${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""

# Step 1: Create redis.conf
REDIS_CONF="redis.conf"
if [[ -f "$REDIS_CONF" && "$FORCE" != "true" ]]; then
    echo -e "${YELLOW}✓ redis.conf already exists (use --force to overwrite)${NC}"
else
    echo -e "${GREEN}Creating redis.conf with TLS-only configuration...${NC}"
    cat > "$REDIS_CONF" <<'EOF'
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
EOF
    echo -e "${GREEN}✓ redis.conf created${NC}"
fi

# Step 2: Generate certificates
CERT_DIR="./certs"
REQUIRED_CERTS=("ca.crt" "ca.key" "server.crt" "server.key" "client.crt" "client.key")
CERTS_EXIST=true

if [[ -d "$CERT_DIR" ]]; then
    for cert in "${REQUIRED_CERTS[@]}"; do
        if [[ ! -f "$CERT_DIR/$cert" ]]; then
            CERTS_EXIST=false
            break
        fi
    done
else
    CERTS_EXIST=false
fi

if [[ "$CERTS_EXIST" == "true" && "$FORCE" != "true" ]]; then
    echo -e "${YELLOW}✓ Certificates already exist (use --force to regenerate)${NC}"
else
    echo -e "${GREEN}Generating TLS certificates...${NC}"
    if [[ ! -f "generate-certs.sh" ]]; then
        echo -e "${RED}Error: generate-certs.sh not found${NC}"
        exit 1
    fi
    chmod +x generate-certs.sh
    ./generate-certs.sh
    echo -e "${GREEN}✓ Certificates generated${NC}"
fi

# Step 3: Validate certificates exist
echo ""
echo -e "${GREEN}Validating setup...${NC}"
MISSING_FILES=()

if [[ ! -f "$REDIS_CONF" ]]; then
    MISSING_FILES+=("$REDIS_CONF")
fi

for cert in "${REQUIRED_CERTS[@]}"; do
    if [[ ! -f "$CERT_DIR/$cert" ]]; then
        MISSING_FILES+=("$CERT_DIR/$cert")
    fi
done

if [[ ${#MISSING_FILES[@]} -gt 0 ]]; then
    echo -e "${RED}Error: The following required files are missing:${NC}"
    for file in "${MISSING_FILES[@]}"; do
        echo -e "${RED}  - $file${NC}"
    done
    exit 1
fi

echo -e "${GREEN}✓ All required files present${NC}"
echo ""

# Step 4: Print next steps
echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}  Setup Complete!${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""
echo -e "${GREEN}Next step:${NC}"
echo -e "${YELLOW}  docker compose up --build${NC}"
echo ""

