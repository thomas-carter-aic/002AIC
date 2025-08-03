#!/bin/bash

# Local Hybrid Auth Deployment Script
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
COMPOSE_DIR="$(dirname "$SCRIPT_DIR")/docker-compose"

echo "🚀 Starting Local Hybrid Auth deployment..."

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
docker-compose -f hybrid-auth-local.yml down

# Build and start services
echo "🏗️  Building and starting services..."
docker-compose -f hybrid-auth-local.yml up -d --build

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
        exit 1
    fi
    echo "⏳ Still waiting for Keycloak... ($counter/$timeout seconds)"
done

echo "✅ Keycloak is ready!"

# Wait for Authorization Service
echo "🛡️  Waiting for Authorization Service..."
timeout=120
counter=0
while ! curl -f http://localhost:8081/health &>/dev/null; do
    sleep 5
    counter=$((counter + 5))
    if [ $counter -ge $timeout ]; then
        echo "❌ Authorization Service failed to start within $timeout seconds"
        exit 1
    fi
    echo "⏳ Still waiting for Authorization Service... ($counter/$timeout seconds)"
done

echo "✅ Authorization Service is ready!"

echo ""
echo "🎉 Local Hybrid Auth deployment completed successfully!"
echo ""
echo "🌐 Access URLs:"
echo "  - Keycloak Admin: http://localhost:8080/admin (admin/admin-password)"
echo "  - Authorization Service: http://localhost:8081"
echo "  - Kong Admin API: http://localhost:8001"
echo "  - Kong Manager: http://localhost:8002"
echo "  - Kong Proxy: http://localhost:8000"
echo "  - Jaeger UI: http://localhost:16686"
echo "  - Prometheus: http://localhost:9090"
echo "  - Grafana: http://localhost:3000 (admin/admin)"
echo ""
echo "👤 Test Users:"
echo "  - admin@002aic.local / admin123!"
echo "  - test@002aic.local / testpass123!"
echo "  - scientist@002aic.local / science123!"
echo ""
echo "🧪 Test the setup with:"
echo "  ./test-local-auth.sh"
