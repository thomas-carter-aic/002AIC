# Health Check Service

The Health Check Service is a critical component of the 002AIC platform that monitors the health and availability of all 36 microservices in the system. It provides real-time health monitoring, service discovery, and system status reporting.

## Features

- **Service Registration**: Dynamic registration of services for health monitoring
- **Automated Health Checks**: Configurable health check intervals and timeouts
- **System Overview**: Real-time system health dashboard
- **Critical Service Monitoring**: Special handling for critical services
- **Metrics Collection**: Prometheus metrics for observability
- **Historical Data**: Health check history and trends
- **Redis Caching**: Fast access to health status data
- **PostgreSQL Storage**: Persistent service configuration and history

## Architecture

The service monitors all 36 microservices in the 002AIC platform:

### Core Platform Services (8)
- Configuration Service, Service Discovery, User Management, Data Management
- Analytics, Runtime Management, Notification, Audit Services

### AI/ML Services (12)
- Agent Orchestration, Model Training, Model Tuning, Model Deployment
- Data Integration, Pipeline Execution, Monitoring, File Storage
- Search, Model Registry, Recommendation Engine Services

### Business Services (5)
- Billing, Workflow, Integration, Support, Marketplace Services

### Infrastructure Services (11)
- API Gateway, Event Streaming, Content Management, Security
- Backup, Logging, Feature Flag, Caching, Queue, Metrics Services

## API Endpoints

### Health Check
```
GET /health
```
Returns the health status of the Health Check Service itself.

### Service Management
```
POST /v1/services          # Register a new service
GET /v1/services           # Get all services and their status
POST /v1/services/:id/check # Perform health check on specific service
```

### System Overview
```
GET /v1/system/overview    # Get system-wide health overview
```

### Metrics
```
GET /metrics               # Prometheus metrics
```

## Configuration

Environment variables:

- `PORT`: Service port (default: 8080)
- `DATABASE_URL`: PostgreSQL connection string
- `REDIS_URL`: Redis connection string
- `CHECK_INTERVAL`: Health check interval in seconds (default: 30)
- `TIMEOUT`: Health check timeout in milliseconds (default: 5000)
- `RETRIES`: Number of retries for failed checks (default: 3)
- `ENVIRONMENT`: Deployment environment

## Service Registration

Services can register themselves for health monitoring:

```json
{
  "name": "my-service",
  "url": "http://my-service:8080/health",
  "critical": true,
  "interval": 30,
  "timeout": 5000,
  "metadata": {
    "version": "1.0.0",
    "team": "platform"
  }
}
```

## Health Check Response Format

```json
{
  "service_id": "uuid",
  "service_name": "my-service",
  "url": "http://my-service:8080/health",
  "status": "healthy",
  "response_time": 150,
  "http_status": 200,
  "timestamp": "2024-01-01T00:00:00.000Z"
}
```

## System Status Levels

- **Healthy**: All critical services are operational
- **Degraded**: Some non-critical services are down
- **Critical**: One or more critical services are down

## Metrics

The service exposes Prometheus metrics:

- `health_checks_total`: Total health checks performed
- `service_status`: Current service health status (1=healthy, 0=unhealthy)
- `health_check_response_time_seconds`: Health check response times

## Database Schema

### Services Table
```sql
CREATE TABLE services (
    id UUID PRIMARY KEY,
    name VARCHAR(255) NOT NULL UNIQUE,
    url VARCHAR(500) NOT NULL,
    critical BOOLEAN DEFAULT FALSE,
    interval_seconds INTEGER DEFAULT 30,
    timeout_ms INTEGER DEFAULT 5000,
    metadata JSONB DEFAULT '{}',
    registered_at TIMESTAMP DEFAULT NOW(),
    last_check TIMESTAMP
);
```

### Health Check History Table
```sql
CREATE TABLE health_check_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    service_id UUID NOT NULL REFERENCES services(id),
    status VARCHAR(20) NOT NULL,
    response_time_ms INTEGER,
    http_status INTEGER,
    error_message TEXT,
    checked_at TIMESTAMP DEFAULT NOW()
);
```

## Running the Service

### Development
```bash
npm install
npm run dev
```

### Production
```bash
npm install --production
npm start
```

### Docker
```bash
docker build -t health-check-service .
docker run -p 8080:8080 health-check-service
```

### Kubernetes
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: health-check-service
spec:
  replicas: 2
  selector:
    matchLabels:
      app: health-check-service
  template:
    metadata:
      labels:
        app: health-check-service
    spec:
      containers:
      - name: health-check-service
        image: health-check-service:latest
        ports:
        - containerPort: 8080
        env:
        - name: DATABASE_URL
          valueFrom:
            secretKeyRef:
              name: database-secret
              key: url
        - name: REDIS_URL
          valueFrom:
            secretKeyRef:
              name: redis-secret
              key: url
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5
```

## Monitoring and Alerting

The Health Check Service integrates with:

- **Prometheus**: Metrics collection
- **Grafana**: Health dashboards
- **AlertManager**: Critical service alerts
- **PagerDuty**: Incident management

## Security

- **Authentication**: JWT token validation for API endpoints
- **Authorization**: Role-based access control
- **Encryption**: TLS 1.3 for all communications
- **Audit Logging**: All health check activities logged

## High Availability

- **Multi-instance**: Run multiple instances for redundancy
- **Load Balancing**: Distribute health check load
- **Failover**: Automatic failover to backup instances
- **Data Replication**: PostgreSQL and Redis clustering

## Performance

- **Concurrent Checks**: Parallel health check execution
- **Caching**: Redis caching for fast status retrieval
- **Connection Pooling**: Efficient database connections
- **Metrics**: Sub-second health check response times

## Contributing

1. Fork the repository
2. Create a feature branch
3. Add tests for new functionality
4. Submit a pull request

## License

MIT License - see LICENSE file for details.
