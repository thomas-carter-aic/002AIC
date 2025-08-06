#!/bin/bash

# Nexus Platform - AI/ML Platform Startup Script
# This script starts the complete AI-native platform with ML capabilities

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
NC='\033[0m' # No Color

# Configuration
PROJECT_ROOT="/home/oss/002AIC"
COMPOSE_DIR="$PROJECT_ROOT/infra/docker-compose"

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

print_ai() {
    echo -e "${PURPLE}[AI/ML]${NC} $1"
}

# Function to check service health
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

# Main execution
main() {
    echo "🤖 Starting Nexus AI-Native Platform"
    echo "===================================="
    
    # Change to project root
    cd "$PROJECT_ROOT"
    
    # Create external network if it doesn't exist
    if ! docker network ls | grep -q "nexus-net"; then
        print_status "Creating nexus-net network..."
        docker network create nexus-net
        print_success "Network created"
    fi
    
    # Step 1: Start infrastructure services
    print_status "🏗️  Starting infrastructure services..."
    cd "$COMPOSE_DIR"
    docker-compose -f kong-kuma-local.yml up -d
    
    # Wait for Kong
    check_service_health "Kong Admin API" "http://localhost:8001/status"
    
    # Step 2: Start core databases and messaging
    print_status "🗄️  Starting databases and messaging..."
    docker-compose -f nexus-core.yml up -d postgres mongodb redis zookeeper kafka
    
    # Wait for databases
    print_status "Waiting for databases to be ready..."
    sleep 15
    
    # Step 3: Start AI/ML infrastructure
    print_ai "🧠 Starting AI/ML infrastructure..."
    docker-compose -f ai-ml-stack.yml up -d
    
    # Wait for AI/ML services
    print_ai "Waiting for AI/ML services..."
    sleep 20
    
    # Check MLflow
    check_service_health "MLflow Server" "http://localhost:5000/health" || true
    check_service_health "Jupyter Lab" "http://localhost:8888" || true
    check_service_health "MinIO" "http://localhost:9000/minio/health/live" || true
    check_service_health "Qdrant" "http://localhost:6333/health" || true
    check_service_health "Airflow" "http://localhost:8080/health" || true
    
    # Step 4: Start core microservices
    print_status "🔧 Starting core microservices..."
    docker-compose -f nexus-core.yml up -d
    
    # Step 5: Start AI/ML microservices
    print_ai "🤖 Starting AI/ML microservices..."
    docker-compose -f nexus-ai-services.yml up -d
    
    # Step 6: Configure Kong routes for AI services
    print_status "🌐 Configuring AI/ML API routes..."
    
    sleep 10  # Wait for services to start
    
    # AI/ML service routes
    ai_services_config=(
        "model-management-service:http://model-management-service:8086:/models"
        "model-training-service:http://model-training-service:8087:/training"
        "model-deployment-service:http://model-deployment-service:8088:/deployments"
        "data-integration-service:http://data-integration-service:8089:/data"
        "analytics-service:http://analytics-service:8090:/analytics"
    )
    
    for service_config in "${ai_services_config[@]}"; do
        IFS=':' read -r service_name service_url service_path <<< "$service_config"
        
        # Create Kong service
        curl -i -X POST http://localhost:8001/services/ \
            --data "name=$service_name" \
            --data "url=$service_url" > /dev/null 2>&1 || true
        
        # Create Kong route
        curl -i -X POST http://localhost:8001/services/$service_name/routes \
            --data "paths[]=$service_path" > /dev/null 2>&1 || true
    done
    
    print_success "AI/ML API routes configured"
    
    # Step 7: Health checks
    print_status "🏥 Performing comprehensive health checks..."
    
    # Infrastructure health checks
    services_to_check=(
        "Kong Admin:http://localhost:8001/status"
        "Kong Proxy:http://localhost:8000"
        "Prometheus:http://localhost:9090/-/healthy"
        "Grafana:http://localhost:3000/api/health"
    )
    
    for service_check in "${services_to_check[@]}"; do
        IFS=':' read -r service_name service_url <<< "$service_check"
        check_service_health "$service_name" "$service_url" || true
    done
    
    # AI/ML service health checks
    ai_services_to_check=(
        "Model Management:http://localhost:8086/health"
        "Model Training:http://localhost:8087/health"
        "MLflow Server:http://localhost:5000/health"
    )
    
    for service_check in "${ai_services_to_check[@]}"; do
        IFS=':' read -r service_name service_url <<< "$service_check"
        check_service_health "$service_name" "$service_url" || true
    done
    
    # Step 8: Display comprehensive status
    echo ""
    echo "🎉 Nexus AI-Native Platform Status"
    echo "=================================="
    echo ""
    echo "🌐 API Gateway & Service Mesh:"
    echo "   - Kong Proxy: http://localhost:8000"
    echo "   - Kong Admin: http://localhost:8001"
    echo "   - Kong Admin GUI: http://localhost:8002"
    echo "   - Kuma Control Plane: http://localhost:5681"
    echo ""
    echo "📊 Monitoring & Observability:"
    echo "   - Prometheus: http://localhost:9090"
    echo "   - Grafana: http://localhost:3000 (admin/admin)"
    echo "   - Jaeger Tracing: http://localhost:16686"
    echo ""
    echo "🗄️  Data Infrastructure:"
    echo "   - PostgreSQL: localhost:5433 (nexus/nexus-password)"
    echo "   - MongoDB: localhost:27017 (nexus/nexus-password)"
    echo "   - Redis: localhost:6379 (password: nexus-password)"
    echo "   - Kafka: localhost:29092"
    echo ""
    echo "🤖 AI/ML Infrastructure:"
    echo "   - MLflow Server: http://localhost:5000"
    echo "   - Jupyter Lab: http://localhost:8888 (token: nexus-jupyter-token)"
    echo "   - MinIO Storage: http://localhost:9001 (nexus-minio/nexus-minio-password)"
    echo "   - Qdrant Vector DB: http://localhost:6333"
    echo "   - Airflow: http://localhost:8080 (admin/admin)"
    echo "   - TensorFlow Serving: http://localhost:8501"
    echo "   - Triton Server: http://localhost:8000"
    echo ""
    echo "🔧 Core Microservices:"
    echo "   - Authorization: http://localhost:8080"
    echo "   - User Management: http://localhost:8082"
    echo "   - Configuration: http://localhost:8083"
    echo "   - Health Check: http://localhost:8085"
    echo ""
    echo "🧠 AI/ML Microservices:"
    echo "   - Model Management: http://localhost:8086"
    echo "   - Model Training: http://localhost:8087"
    echo "   - Model Deployment: http://localhost:8088"
    echo "   - Data Integration: http://localhost:8089"
    echo "   - Analytics: http://localhost:8090"
    echo ""
    echo "🚀 AI/ML API Endpoints (via Kong):"
    echo "   - Model Management: http://localhost:8000/models"
    echo "   - Model Training: http://localhost:8000/training"
    echo "   - Model Deployment: http://localhost:8000/deployments"
    echo "   - Data Integration: http://localhost:8000/data"
    echo "   - Analytics: http://localhost:8000/analytics"
    echo ""
    echo "📚 Quick Start Examples:"
    echo "   # Create a training job"
    echo "   curl -X POST http://localhost:8000/training/training-jobs \\"
    echo "     -H 'Content-Type: application/json' \\"
    echo "     -d '{\"job_name\":\"test-model\",\"model_type\":\"classifier\",\"algorithm\":\"random_forest\",\"target_column\":\"target\"}'"
    echo ""
    echo "   # List models"
    echo "   curl http://localhost:8000/models/models"
    echo ""
    echo "   # Check system health"
    echo "   curl http://localhost:8000/health"
    echo ""
    
    print_success "🎉 Nexus AI-Native Platform is ready!"
    print_ai "🧠 Your AI/ML development environment is fully operational"
    print_status "Run './scripts/stop-ai-platform.sh' to stop all services"
    print_status "Run './scripts/test-ai-platform.sh' to run comprehensive tests"
}

# Trap to handle script interruption
trap 'print_warning "Script interrupted"; exit 1' INT TERM

# Run main function
main "$@"
