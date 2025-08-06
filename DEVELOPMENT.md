# Nexus Platform - Development Guide

This guide provides comprehensive instructions for setting up and developing the Nexus Platform locally.

## Quick Start

### Prerequisites

- Docker & Docker Compose
- Go 1.21+
- Node.js 18+
- Python 3.11+
- kubectl (for Kubernetes deployment)
- Helm 3.x (for Kubernetes deployment)

### Local Development Setup

1. **Start the complete development environment:**
   ```bash
   ./scripts/start-local-dev.sh
   ```

2. **Stop the development environment:**
   ```bash
   ./scripts/stop-local-dev.sh
   ```

### Manual Setup (Alternative)

If you prefer to start services individually:

1. **Start infrastructure services:**
   ```bash
   cd infra/docker-compose
   docker-compose -f kong-kuma-local.yml up -d
   ```

2. **Start core services:**
   ```bash
   docker-compose -f nexus-core.yml up -d
   ```

## Architecture Overview

The Nexus Platform follows a microservices architecture with the following components:

### Infrastructure Layer
- **Kong API Gateway**: Routes and manages API traffic
- **Kuma Service Mesh**: Service-to-service communication
- **Apache Kafka**: Event streaming and messaging
- **PostgreSQL**: Primary relational database
- **MongoDB**: Document storage
- **Redis**: Caching and session storage

### Monitoring Stack
- **Prometheus**: Metrics collection
- **Grafana**: Visualization and dashboards
- **Jaeger**: Distributed tracing

### Core Microservices

| Service | Language | Port | Purpose |
|---------|----------|------|---------|
| Authorization Service | Go | 8080 | Authentication & authorization |
| API Gateway Service | Go/Node.js | 8081 | API routing and management |
| User Management Service | Go | 8082 | User CRUD operations |
| Configuration Service | Go | 8083 | Dynamic configuration |
| Discovery Service | Go | 8084 | Service discovery |
| Health Check Service | Node.js | 8085 | System health monitoring |

## Development Workflow

### Adding a New Microservice

1. **Create service directory:**
   ```bash
   mkdir -p apps/backend-services/my-new-service
   cd apps/backend-services/my-new-service
   ```

2. **Initialize based on language:**

   **For Go services:**
   ```bash
   go mod init github.com/002aic/my-new-service
   mkdir -p cmd/server internal/{handler,service,repository,config}
   ```

   **For Node.js services:**
   ```bash
   npm init -y
   npm install express cors helmet morgan winston
   ```

   **For Python services:**
   ```bash
   python -m venv venv
   source venv/bin/activate
   pip install fastapi uvicorn sqlalchemy redis kafka-python
   ```

3. **Create Dockerfile:**
   ```dockerfile
   FROM golang:1.21-alpine AS builder
   WORKDIR /app
   COPY go.mod go.sum ./
   RUN go mod download
   COPY . .
   RUN go build -o bin/server ./cmd/server

   FROM alpine:latest
   RUN apk --no-cache add ca-certificates
   WORKDIR /root/
   COPY --from=builder /app/bin/server .
   EXPOSE 8080
   CMD ["./server"]
   ```

4. **Add to docker-compose:**
   ```yaml
   my-new-service:
     build:
       context: ../../apps/backend-services/my-new-service
       dockerfile: Dockerfile
     environment:
       - PORT=8086
       - DB_HOST=postgres
       # ... other env vars
     ports:
       - "8086:8086"
     networks:
       - nexus-net
   ```

### Service Communication

Services communicate through:

1. **HTTP APIs** (via Kong Gateway)
2. **Event Streaming** (via Kafka)
3. **Service Mesh** (via Kuma)

#### Example: Publishing an Event

**Go (using Kafka):**
```go
import "github.com/segmentio/kafka-go"

func publishEvent(topic string, message []byte) error {
    writer := kafka.NewWriter(kafka.WriterConfig{
        Brokers: []string{"kafka:9092"},
        Topic:   topic,
    })
    defer writer.Close()
    
    return writer.WriteMessages(context.Background(),
        kafka.Message{Value: message},
    )
}
```

**Node.js (using KafkaJS):**
```javascript
const { Kafka } = require('kafkajs');

const kafka = Kafka({
    clientId: 'my-service',
    brokers: ['kafka:9092']
});

const producer = kafka.producer();

async function publishEvent(topic, message) {
    await producer.send({
        topic,
        messages: [{ value: JSON.stringify(message) }]
    });
}
```

### Database Migrations

Database migrations are handled per service:

1. **Create migration file:**
   ```bash
   mkdir -p apps/backend-services/my-service/migrations
   touch apps/backend-services/my-service/migrations/001_initial.sql
   ```

2. **Add to initialization script:**
   ```sql
   -- Add to infra/docker-compose/init-scripts/01-init-databases.sql
   CREATE DATABASE my_service_db;
   CREATE USER my_service WITH PASSWORD 'my_service_password';
   GRANT ALL PRIVILEGES ON DATABASE my_service_db TO my_service;
   ```

### Testing

#### Unit Tests
```bash
# Go services
go test ./...

# Node.js services
npm test

# Python services
pytest
```

#### Integration Tests
```bash
# Start test environment
docker-compose -f infra/docker-compose/test.yml up -d

# Run integration tests
go test -tags=integration ./test/integration/...
```

#### Load Testing
```bash
# Using k6
k6 run test/load/api-load-test.js
```

## API Documentation

### Authentication

All API requests require authentication via JWT tokens:

```bash
# Login to get token
curl -X POST http://localhost:8000/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email": "user@example.com", "password": "password"}'

# Use token in subsequent requests
curl -H "Authorization: Bearer <token>" \
  http://localhost:8000/users/profile
```

### Core Endpoints

#### Authentication Service (`/auth`)
- `POST /auth/login` - User login
- `POST /auth/logout` - User logout
- `POST /auth/refresh` - Refresh token
- `GET /auth/verify` - Verify token

#### User Management Service (`/users`)
- `GET /users/profile` - Get user profile
- `PUT /users/profile` - Update user profile
- `GET /users` - List users (admin)
- `POST /users` - Create user (admin)

#### Configuration Service (`/config`)
- `GET /config/{key}` - Get configuration value
- `PUT /config/{key}` - Update configuration value
- `GET /config/features` - Get feature flags

#### Health Check Service (`/health`)
- `GET /health` - Overall system health
- `GET /health/services` - Individual service health
- `GET /health/dependencies` - Dependency health

## Monitoring and Observability

### Metrics

Access Prometheus metrics at:
- Prometheus UI: http://localhost:9090
- Service metrics: http://localhost:8080/metrics

### Dashboards

Access Grafana dashboards at:
- Grafana UI: http://localhost:3000 (admin/admin)
- Pre-configured dashboards for each service

### Tracing

Access Jaeger tracing at:
- Jaeger UI: http://localhost:16686

### Logs

View service logs:
```bash
# All services
docker-compose logs -f

# Specific service
docker-compose logs -f authorization-service
```

## Troubleshooting

### Common Issues

1. **Port conflicts:**
   ```bash
   # Check what's using a port
   lsof -i :8080
   
   # Kill process using port
   kill -9 $(lsof -t -i:8080)
   ```

2. **Database connection issues:**
   ```bash
   # Check PostgreSQL status
   docker exec nexus-postgres-1 pg_isready -U nexus
   
   # Connect to database
   docker exec -it nexus-postgres-1 psql -U nexus -d nexus
   ```

3. **Service not starting:**
   ```bash
   # Check service logs
   docker-compose logs service-name
   
   # Rebuild service
   docker-compose build service-name
   docker-compose up -d service-name
   ```

4. **Kong configuration issues:**
   ```bash
   # Check Kong status
   curl http://localhost:8001/status
   
   # List Kong services
   curl http://localhost:8001/services
   
   # List Kong routes
   curl http://localhost:8001/routes
   ```

### Performance Optimization

1. **Database optimization:**
   - Add appropriate indexes
   - Use connection pooling
   - Monitor slow queries

2. **Caching:**
   - Use Redis for frequently accessed data
   - Implement application-level caching
   - Use Kong caching plugins

3. **Service mesh optimization:**
   - Configure appropriate timeouts
   - Implement circuit breakers
   - Use load balancing strategies

## Deployment

### Local Kubernetes (Development)

1. **Start local Kubernetes:**
   ```bash
   # Using kind
   kind create cluster --name nexus-dev
   
   # Using minikube
   minikube start --profile nexus-dev
   ```

2. **Deploy with Helm:**
   ```bash
   helm install nexus infra/helm/nexus/
   ```

### Production Deployment

See [Production Deployment Guide](documentation/ops/production-deployment.md) for detailed production deployment instructions.

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Update documentation
6. Submit a pull request

## Support

- 📖 [Documentation](documentation/)
- 💬 [Discussions](https://github.com/thomas-carter-aic/002AIC/discussions)
- 🐛 [Issues](https://github.com/thomas-carter-aic/002AIC/issues)
- 📧 [Email Support](mailto:support@appliedinnovationcorp.com)
