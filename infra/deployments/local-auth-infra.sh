#!/bin/bash

# Local Auth Infrastructure Deployment Script
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
COMPOSE_DIR="$(dirname "$SCRIPT_DIR")/docker-compose"

echo "🚀 Starting Local Auth Infrastructure deployment..."

# Check if Docker and Docker Compose are available
if ! command -v docker &> /dev/null; then
    echo "❌ Docker is required but not installed"
    exit 1
fi

if ! command -v docker-compose &> /dev/null; then
    echo "❌ Docker Compose is required but not installed"
    exit 1
fi

echo "✅ Prerequisites check passed"

# Navigate to compose directory
cd "$COMPOSE_DIR"

# Stop any existing containers
echo "🛑 Stopping existing containers..."
docker-compose -f auth-infrastructure.yml down

# Start infrastructure services
echo "🏗️  Starting infrastructure services..."
docker-compose -f auth-infrastructure.yml up -d

echo "⏳ Waiting for services to be ready..."

# Wait for databases
echo "📊 Waiting for databases..."
sleep 30

# Wait for Keycloak
echo "🔐 Waiting for Keycloak..."
timeout=300
counter=0
while ! curl -f http://localhost:8080/health/ready &>/dev/null; do
    sleep 5
    counter=$((counter + 5))
    if [ $counter -ge $timeout ]; then
        echo "❌ Keycloak failed to start within $timeout seconds"
        docker-compose -f auth-infrastructure.yml logs keycloak
        exit 1
    fi
    echo "⏳ Still waiting for Keycloak... ($counter/$timeout seconds)"
done

echo "✅ Keycloak is ready!"

# Wait for Kong
echo "🦍 Waiting for Kong..."
timeout=120
counter=0
while ! curl -f http://localhost:8001 &>/dev/null; do
    sleep 5
    counter=$((counter + 5))
    if [ $counter -ge $timeout ]; then
        echo "❌ Kong failed to start within $timeout seconds"
        docker-compose -f auth-infrastructure.yml logs kong
        exit 1
    fi
    echo "⏳ Still waiting for Kong... ($counter/$timeout seconds)"
done

echo "✅ Kong is ready!"

echo ""
echo "🎉 Local Auth Infrastructure deployment completed successfully!"
echo ""
echo "🌐 Access URLs:"
echo "  - Keycloak Admin: http://localhost:8080/admin (admin/admin-password)"
echo "  - Kong Admin API: http://localhost:8001"
echo "  - Kong Manager: http://localhost:8002"
echo "  - Kong Proxy: http://localhost:8000"
echo "  - Jaeger UI: http://localhost:16686"
echo "  - Prometheus: http://localhost:9090"
echo "  - Grafana: http://localhost:3000 (admin/admin)"
echo ""
echo "📊 Database Connections:"
echo "  - Kong DB: localhost:5432 (kong/kong-password)"
echo "  - Keycloak DB: localhost:5433 (keycloak/keycloak-password)"
echo "  - AuthZ DB: localhost:5434 (authorization/authorization-password)"
echo "  - Redis: localhost:6379 (password: redis-password)"
echo ""
echo "🎯 Next Steps:"
echo "  1. Build and run the Go authorization service"
echo "  2. Configure Kong routes and plugins"
echo "  3. Test the authentication flow"
echo ""
echo "🧪 Test the infrastructure with:"
echo "  ./test-auth-infra.sh"
