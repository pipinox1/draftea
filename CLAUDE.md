# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Development Commands

### Build and Run
```bash
# Build all services
make build
go build -o bin/payments-service ./cmd/payments-service
go build -o bin/wallet-service ./cmd/wallet-service

# Run tests
make test
go test -v ./...

# Development setup
make dev-setup
go mod tidy

# Database setup
psql -U postgres -d payment_system -f database/setup.sql
```

### Docker Operations
```bash
# Start all services (PostgreSQL + LocalStack + Services)
docker-compose up -d

# Start with optional UI components (pgAdmin + LocalStack UI)
docker-compose --profile ui up -d

# View logs
docker-compose logs -f
make docker-logs

# Stop services
docker-compose down
make docker-down

# Remove all volumes (reset everything)
docker-compose down -v
```

### Health and Testing
```bash
# Health checks
make health

# API testing
make test-payment
make test-wallet

# Integration testing
make integration-test
```

## Architecture Overview

This is a microservices payment system built in Go implementing:

- **Hexagonal Architecture**: Domain, Application, Infrastructure layers
- **Saga Choreography**: Event-driven coordination without central orchestrator
- **CQRS**: Command Query Responsibility Segregation

### Services Structure

Each service follows the same layered architecture:
```
service-name/
├── domain/         # Business logic and entities
├── application/    # Use cases and event handlers
├── infrastructure/ # Database repositories and external adapters
└── handlers/       # HTTP endpoints
```

### Event Flow Architecture

The system uses **Saga Choreography** where services react to events:

1. **Payment Initiated** → Payment Service publishes event
2. **Wallet Debit Requested** → Payment Service requests debit
3. **Wallet Debited** → Wallet Service confirms debit
4. **Gateway Processing** → External gateway processes payment
5. **Payment Completed** → Final success state

**Compensation Flow** for failures:
- Gateway fails → Wallet Credit Requested → Wallet Credited → Payment Failed

### Messaging Systems

**SQS/SNS Messaging**: Uses LocalStack for local development
   - File: `docker-compose.yml`
   - Environment: AWS SDK with LocalStack endpoint
   - LocalStack UI: http://localhost:8082 (with --profile ui)
   - pgAdmin: http://localhost:8083 (with --profile ui)

## Key Technologies

- **Go 1.23**: Main language
- **PostgreSQL**: Event store and persistence (VARCHAR-based UUIDs, no extensions required)
- **AWS SQS/SNS**: Messaging (via LocalStack locally)
- **Chi Router**: HTTP routing
- **SQLx**: Database operations
- **UUID**: Application-generated entity identification (github.com/google/uuid, stored as VARCHAR(36))

## Database Schema

Core tables:
- `event_stream`: Immutable event storage with aggregate versioning
- `snapshots`: Performance optimization for aggregate reconstruction
- `payments`: Payment aggregate storage
- `wallets`: Wallet aggregate storage
- `wallet_transactions`: Internal wallet transactions (debit/credit)
- `wallet_movements`: Business movements (income/expense)

All ID fields use VARCHAR(36) to store application-generated UUIDs. No PostgreSQL uuid-ossp extension required.

## Development Patterns

### Repository Pattern
Each domain has its own repository interface in the domain layer with PostgreSQL implementation in infrastructure.

### Event Handlers
Application layer contains event handlers that process domain events and coordinate with other services.

## Environment Configuration

Required environment variables:
- `DATABASE_URL`: PostgreSQL connection string
- `AWS_*`: AWS credentials for SQS/SNS
- `SNS_TOPIC_ARN`: Event publication topic
- `SQS_QUEUE_URL`: Event consumption queue
- `PORT`: Service port (8080 for payments, 8081 for wallet)

## Testing and Validation

The system includes test wallets with predefined UUIDs and balances for development testing:
- Wallet ID: `550e8400-e29b-41d4-a716-446655440001`
- User ID: `550e8400-e29b-41d4-a716-446655440010`

## Service Endpoints

- **Payment Service**: localhost:8080
- **Wallet Service**: localhost:8081
- **LocalStack**: localhost:4566 (AWS services)
- **pgAdmin**: localhost:8083 (with profile: ui)