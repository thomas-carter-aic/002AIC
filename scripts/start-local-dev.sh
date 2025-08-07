#!/bin/bash

# Nexus Platform - Local Development Environment Startup Script
# This script starts the complete local development environment

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
PROJECT_ROOT="/home/oss/002AIC"
COMPOSE_DIR="$PROJECT_ROOT/infra/docker-compose"
SERVICES_DIR="$PROJECT_ROOT/apps/backend-services"

# Function to print colored output
print_status() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Function to check if a service is healthy
check_service_health() {
    local service_name=$1
    local health_url=$2
    local max_attempts=30
    local attempt=1

    print_status "Checking health of $service_name..."
    
    while [ $attempt -le $max_attempts ]; do
        if curl -f -s "$health_url" > /dev/null 2>&1; then
            print_success "$service_name is healthy"
            return 0
        fi
        
        echo -n "."
        sleep 2
        attempt=$((attempt + 1))
    done
    
    print_error "$service_name failed to become healthy"
    return 1
}

# Function to wait for database
wait_for_postgres() {
    print_status "Waiting for PostgreSQL to be ready..."
    
    local max_attempts=30
    local attempt=1
    
    while [ $attempt -le $max_attempts ]; do
        if docker exec docker-compose-postgres-1 pg_isready -U nexus > /dev/null 2>&1; then
            print_success "PostgreSQL is ready"
            return 0
        fi
        
        echo -n "."
        sleep 2
        attempt=$((attempt + 1))
    done
    
    print_error "PostgreSQL failed to become ready"
    return 1
}

# Main execution
main() {
    print_status "Starting Nexus Platform Local Development Environment..."
    
    # Change to project root
    cd "$PROJECT_ROOT"
    
    # Create external network if it doesn't exist
    if ! docker network ls | grep -q "nexus-net"; then
        print_status "Creating nexus-net network..."
        docker network create nexus-net
        print_success "Network created"
    fi
    
    # Step 1: Start infrastructure services (Kong, Kuma, monitoring)
    print_status "Starting infrastructure services..."
    cd "$COMPOSE_DIR"
    docker-compose -f kong-kuma-local.yml up -d
    
    # Wait for Kong to be ready
    print_status "Waiting for Kong API Gateway..."
    sleep 10
    check_service_health "Kong Admin API" "http://localhost:8001/status"
    
    # Step 2: Start core databases and messaging
    print_status "Starting core services (databases, Kafka)..."
    docker-compose -f nexus-core.yml up -d postgres mongodb redis zookeeper kafka
    
    # Wait for databases
    wait_for_postgres
    
    # Step 3: Build and start microservices
    print_status "Building and starting microservices..."
    
    # Build core services
    core_services=("authorization-service" "api-gateway-service" "user-management-service" "configuration-service" "discovery-service" "health-check-service")
    
    for service in "${core_services[@]}"; do
        print_status "Building $service..."
        cd "$SERVICES_DIR/$service"
        
        # Build the service based on its language
        if [ -f "go.mod" ]; then
            # Go service
            go mod tidy
            go build -o bin/server ./cmd/server
        elif [ -f "package.json" ]; then
            # Node.js service
            npm install
            npm run build
        elif [ -f "requirements.txt" ]; then
            # Python service
            pip install -r requirements.txt
        fi
    done
    
    # Start microservices
    cd "$COMPOSE_DIR"
    docker-compose -f nexus-core.yml up -d
    
    # Step 4: Health checks
    print_status "Performing health checks..."
    
    # Check core services
    services_to_check=(
        "Kong Admin:http://localhost:8001/status"
        "Kong Proxy:http://localhost:8000"
        "Kuma Control Plane:http://localhost:5681"
        "Prometheus:http://localhost:9090/-/healthy"
        "Grafana:http://localhost:3000/api/health"
    )
    
    for service_check in "${services_to_check[@]}"; do
        IFS=':' read -r service_name service_url <<< "$service_check"
        check_service_health "$service_name" "$service_url" || true
    done
    
    # Step 5: Configure Kong routes
    print_status "Configuring Kong API Gateway routes..."
    
    # Create services and routes for microservices
    services_config=(
        "authorization-service:http://authorization-service:8080:/auth"
        "api-gateway-service:http://api-gateway-service:8081:/gateway"
        "user-management-service:http://user-management-service:8082:/users"
        "configuration-service:http://configuration-service:8083:/config"
        "discovery-service:http://discovery-service:8084:/discovery"
        "health-check-service:http://health-check-service:8085:/health"
    )
    
    for service_config in "${services_config[@]}"; do
        IFS=':' read -r service_name service_url service_path <<< "$service_config"
        
        # Create Kong service
        curl -i -X POST http://localhost:8001/services/ \
            --data "name=$service_name" \
            --data "url=$service_url" > /dev/null 2>&1 || true
        
        # Create Kong route
        curl -i -X POST http://localhost:8001/services/$service_name/routes \
            --data "paths[]=$service_path" > /dev/null 2>&1 || true
    done
    
    print_success "Kong routes configured"
    
    # Step 6: Display status
    print_status "Nexus Platform Local Development Environment Status:"
    echo ""
    echo "🌐 API Gateway (Kong):"
    echo "   - Proxy: http://localhost:8000"
    echo "   - Admin API: http://localhost:8001"
    echo "   - Admin GUI: http://localhost:8002"
    echo ""
    echo "🕸️  Service Mesh (Kuma):"
    echo "   - Control Plane: http://localhost:5681"
    echo ""
    echo "📊 Monitoring:"
    echo "   - Prometheus: http://localhost:9090"
    echo "   - Grafana: http://localhost:3000 (admin/admin)"
    echo "   - Jaeger: http://localhost:16686"
    echo ""
    echo "🗄️  Databases:"
    echo "   - PostgreSQL: localhost:5433 (nexus/nexus-password)"
    echo "   - MongoDB: localhost:17017 (nexus/nexus-password)"
    echo "   - Redis: localhost:16379 (password: nexus-password)"
    echo ""
    echo "📡 Messaging:"
    echo "   - Kafka: localhost:29092"
    echo ""
    echo "🔧 Core Services:"
    echo "   - Authorization: http://localhost:8080"
    echo "   - API Gateway: http://localhost:8081"
    echo "   - User Management: http://localhost:8082"
    echo "   - Configuration: http://localhost:8083"
    echo "   - Discovery: http://localhost:8084"
    echo "   - Health Check: http://localhost:8085"
    echo ""
    echo "🚀 API Endpoints (via Kong):"
    echo "   - Auth: http://localhost:8000/auth"
    echo "   - Users: http://localhost:8000/users"
    echo "   - Config: http://localhost:8000/config"
    echo "   - Discovery: http://localhost:8000/discovery"
    echo "   - Health: http://localhost:8000/health"
    echo ""
    
    print_success "Nexus Platform is ready for development!"
    print_status "Run './scripts/stop-local-dev.sh' to stop all services"
}

# Trap to handle script interruption
trap 'print_warning "Script interrupted"; exit 1' INT TERM

# Run main function
main "$@"
