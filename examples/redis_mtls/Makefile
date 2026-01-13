.PHONY: help certs up down logs clean test verify

# Default target
help:
	@echo "Redis mTLS Example - Available Commands"
	@echo "========================================"
	@echo ""
	@echo "  make certs      - Generate TLS certificates"
	@echo "  make up         - Start all services"
	@echo "  make down       - Stop all services"
	@echo "  make logs       - View service logs"
	@echo "  make clean      - Remove certificates and volumes"
	@echo "  make test       - Test Redis mTLS connection"
	@echo "  make verify     - Verify ff-proxy health"
	@echo ""

# Generate certificates
certs:
	@echo "Generating mTLS certificates..."
	@chmod +x generate-certs.sh
	@./generate-certs.sh

# Start services
up: certs
	@echo "Starting services..."
	@docker-compose up -d
	@echo ""
	@echo "Services started!"
	@echo "  - Primary proxy: http://localhost:7001"
	@echo "  - Replica proxy: http://localhost:7002"
	@echo "  - Redis mTLS:    localhost:6380"
	@echo ""
	@echo "Run 'make logs' to view logs"
	@echo "Run 'make verify' to check health"

# Stop services
down:
	@echo "Stopping services..."
	@docker-compose down
	@echo "Services stopped."

# View logs
logs:
	@docker-compose logs -f

# Clean up everything
clean:
	@echo "Cleaning up..."
	@docker-compose down -v
	@rm -rf certs/
	@echo "Cleanup complete."

# Test Redis connection with mTLS
test:
	@echo "Testing Redis mTLS connection..."
	@docker run --rm --network redis_mtls_ff-proxy-network \
		-v $$(pwd)/certs:/certs:ro \
		redis:7-alpine \
		redis-cli --tls \
			--cert /certs/client.crt \
			--key /certs/client.key \
			--cacert /certs/ca.crt \
			-h redis-mtls -p 6380 ping || \
		(echo "❌ Connection failed. Make sure services are running with 'make up'" && exit 1)
	@echo "✅ Redis mTLS connection successful!"

# Verify ff-proxy health
verify:
	@echo "Checking primary proxy health..."
	@curl -s http://localhost:7001/health | grep -q "status" && \
		echo "✅ Primary proxy is healthy" || \
		echo "❌ Primary proxy health check failed"
	@echo ""
	@echo "Checking replica proxy health..."
	@curl -s http://localhost:7002/health | grep -q "status" && \
		echo "✅ Replica proxy is healthy" || \
		echo "❌ Replica proxy health check failed"

