# Nexus Platform - Development Scripts

This directory contains scripts to help with local development, testing, and deployment of the Nexus Platform.

## Available Scripts

### Local Development

#### `start-local-dev.sh`
Starts the complete local development environment including:
- Kong API Gateway
- Kuma Service Mesh
- PostgreSQL, MongoDB, Redis databases
- Apache Kafka for event streaming
- Prometheus & Grafana for monitoring
- Jaeger for distributed tracing
- All core microservices

**Usage:**
```bash
./scripts/start-local-dev.sh
```

**What it does:**
1. Creates Docker networks
2. Starts infrastructure services (Kong, Kuma, monitoring)
3. Starts databases and messaging (PostgreSQL, MongoDB, Redis, Kafka)
4. Builds and starts microservices
5. Configures Kong API Gateway routes
6. Displays service URLs and credentials

#### `stop-local-dev.sh`
Stops all local development services while preserving data volumes.

**Usage:**
```bash
./scripts/stop-local-dev.sh
```

#### `test-environment.sh`
Comprehensive test suite that verifies all services are running correctly.

**Usage:**
```bash
./scripts/test-environment.sh
```

**Tests performed:**
- Infrastructure service health checks
- Database connectivity tests
- Microservice health endpoints
- API Gateway routing verification

## Service URLs (After Starting)

### Infrastructure
- **Kong Proxy**: http://localhost:8000
- **Kong Admin**: http://localhost:8001
- **Kong Admin GUI**: http://localhost:8002
- **Kuma Control Plane**: http://localhost:5681

### Monitoring
- **Prometheus**: http://localhost:9090
- **Grafana**: http://localhost:3000 (admin/admin)
- **Jaeger**: http://localhost:16686

### Databases
- **PostgreSQL**: localhost:5433 (nexus/nexus-password)
- **MongoDB**: localhost:27017 (nexus/nexus-password)
- **Redis**: localhost:6379 (password: nexus-password)

### Messaging
- **Kafka**: localhost:29092

### Microservices (Direct Access)
- **Authorization Service**: http://localhost:8080
- **API Gateway Service**: http://localhost:8081
- **User Management Service**: http://localhost:8082
- **Configuration Service**: http://localhost:8083
- **Discovery Service**: http://localhost:8084
- **Health Check Service**: http://localhost:8085

### API Routes (via Kong Gateway)
- **Auth API**: http://localhost:8000/auth
- **Users API**: http://localhost:8000/users
- **Config API**: http://localhost:8000/config
- **Discovery API**: http://localhost:8000/discovery
- **Health API**: http://localhost:8000/health

## Development Workflow

### 1. Start Development Environment
```bash
./scripts/start-local-dev.sh
```

### 2. Verify Everything is Working
```bash
./scripts/test-environment.sh
```

### 3. Develop Your Services
- Services are automatically reloaded when you make changes
- View logs: `docker-compose logs -f [service-name]`
- Access databases directly using the credentials above

### 4. Test Your Changes
```bash
# Run unit tests
go test ./...  # For Go services
npm test       # For Node.js services

# Run integration tests
./scripts/test-environment.sh
```

### 5. Stop Environment When Done
```bash
./scripts/stop-local-dev.sh
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

2. **Services not starting:**
   ```bash
   # Check logs
   docker-compose logs [service-name]
   
   # Rebuild and restart
   docker-compose build [service-name]
   docker-compose up -d [service-name]
   ```

3. **Database connection issues:**
   ```bash
   # Check PostgreSQL
   docker exec nexus-postgres-1 pg_isready -U nexus
   
   # Check Redis
   docker exec nexus-redis-1 redis-cli -a nexus-password ping
   
   # Check MongoDB
   docker exec nexus-mongodb-1 mongosh --eval "db.adminCommand('ping')"
   ```

4. **Kong configuration issues:**
   ```bash
   # Check Kong status
   curl http://localhost:8001/status
   
   # List services and routes
   curl http://localhost:8001/services
   curl http://localhost:8001/routes
   ```

### Clean Reset

If you need to completely reset your environment:

```bash
# Stop all services
./scripts/stop-local-dev.sh

# Remove all containers and volumes
docker-compose -f infra/docker-compose/kong-kuma-local.yml down -v
docker-compose -f infra/docker-compose/nexus-core.yml down -v

# Remove network
docker network rm nexus-net

# Start fresh
./scripts/start-local-dev.sh
```

## Adding New Scripts

When adding new scripts to this directory:

1. Make them executable: `chmod +x script-name.sh`
2. Add proper error handling: `set -e`
3. Use colored output for better UX
4. Include help text and usage examples
5. Update this README with the new script

## Script Development Guidelines

- Use bash for shell scripts
- Include proper error handling (`set -e`)
- Use colored output for better user experience
- Include help text and usage examples
- Test scripts thoroughly before committing
- Follow the existing naming convention
