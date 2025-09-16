# Payment System Makefile

.PHONY: help build run test clean docker-up docker-down docker-logs migrate

# Default target
help:
	@echo "Available commands:"
	@echo "  build       - Build all services"
	@echo "  run         - Run services locally"
	@echo "  test        - Run tests"
	@echo "  clean       - Clean build artifacts"
	@echo "  docker-up   - Start all services with Docker"
	@echo "  docker-down - Stop all Docker services"
	@echo "  docker-logs - View Docker logs"
	@echo "  migrate     - Run database migrations"

# Build all services
build:
	@echo "Building payments service..."
	go build -o bin/payments-service ./cmd/payments-service
	@echo "Building wallet service..."
	go build -o bin/wallet-service ./cmd/wallet-service

# Run services locally (requires PostgreSQL and Kafka running)
run-payments:
	@echo "Starting payments service..."
	./bin/payments-service

run-wallet:
	@echo "Starting wallet service..."
	./bin/wallet-service

# Run tests
test:
	go test -v ./...

# Clean build artifacts
clean:
	rm -rf bin/
	docker-compose down --volumes --remove-orphans

# Docker commands
docker-up:
	@echo "Starting all services with Docker..."
	docker-compose up -d
	@echo "Services started! Check health with 'make docker-logs'"

docker-down:
	@echo "Stopping all Docker services..."
	docker-compose down

docker-logs:
	docker-compose logs -f

docker-build:
	@echo "Building Docker images..."
	docker-compose build

# Database migration (manual)
migrate:
	@echo "Applying database migrations..."
	docker-compose exec postgres psql -U postgres -d payment_system -f /docker-entrypoint-initdb.d/001_initial_schema.sql

# Development helpers
dev-setup:
	@echo "Setting up development environment..."
	go mod tidy
	@echo "Development environment ready!"

# API testing
test-payment:
	@echo "Testing payment creation..."
	curl -X POST http://localhost:8080/payments \
		-H "Content-Type: application/json" \
		-d '{"user_id":"550e8400-e29b-41d4-a716-446655440010","amount":5000,"currency":"USD","payment_method":{"type":"wallet","wallet_id":"550e8400-e29b-41d4-a716-446655440001"},"description":"Test payment"}'

test-wallet:
	@echo "Testing wallet balance..."
	curl http://localhost:8081/wallets/550e8400-e29b-41d4-a716-446655440001

# Health checks
health:
	@echo "Checking service health..."
	@echo "Payments service:"
	curl -s http://localhost:8080/health || echo "❌ Payments service not available"
	@echo ""
	@echo "Wallet service:"
	curl -s http://localhost:8081/health || echo "❌ Wallet service not available"

# Monitor Kafka topics
kafka-topics:
	docker-compose exec kafka kafka-topics --bootstrap-server localhost:9092 --list

kafka-events:
	docker-compose exec kafka kafka-console-consumer --bootstrap-server localhost:9092 --topic payment-events --from-beginning

# Database access
db-connect:
	docker-compose exec postgres psql -U postgres -d payment_system

# Full development cycle
dev: clean build docker-up
	@echo "Development environment is ready!"
	@echo "- Payments Service: http://localhost:8080"
	@echo "- Wallet Service: http://localhost:8081"
	@echo "- Kafka UI: http://localhost:8082"
	@echo "- pgAdmin: http://localhost:8083"

# Production-like testing
integration-test: docker-up
	@echo "Waiting for services to be ready..."
	sleep 10
	@echo "Running integration tests..."
	make test-wallet
	make test-payment
	make health