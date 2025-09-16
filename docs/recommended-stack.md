# Recommended Technology Stack

## Infrastructure and Orchestration
- **Kubernetes** for container infrastructure
- **ArgoCD** for deployment visibility and management

## Databases

### Payments
- **DynamoDB** as primary database
    - Primary Key: Payment ID
    - Sort Key: User
- **DocumentDB** for advanced field indexing when needed

### Movements
- **PostgreSQL** leveraging ACID properties for atomic balance and movement updates

## API Gateway and Middleware
- **Kong** as API Gateway
- Middleware configuration to route incoming traffic to authentication service

## Asynchronous Messaging
- **SQS/SNS** for inter-service communication
- **Custom idempotency service** to ensure message uniqueness on reception
- **Redis** as idempotency service backend
- **Dead Letter Queue** per entity for post-processing failed messages

## Observability
- **OpenTelemetry (OTEL)** for application metrics and distributed tracing

## Scalability Strategies
- **Retry with exponential backoff** for handling temporary failures
- **Circuit breaker** on movements service to prevent failure cascades and protect system availability