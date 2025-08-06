#!/bin/bash

# Nexus Platform - AI/ML Platform Test Script
# This script tests the complete AI-native platform

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    echo -e "${BLUE}[TEST]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[PASS]${NC} $1"
}

print_error() {
    echo -e "${RED}[FAIL]${NC} $1"
}

print_ai() {
    echo -e "${PURPLE}[AI/ML]${NC} $1"
}

# Test function
test_endpoint() {
    local name=$1
    local url=$2
    local expected_status=${3:-200}
    
    print_status "Testing $name..."
    
    local status_code=$(curl -s -o /dev/null -w "%{http_code}" "$url" || echo "000")
    
    if [ "$status_code" = "$expected_status" ]; then
        print_success "$name is responding (HTTP $status_code)"
        return 0
    else
        print_error "$name failed (HTTP $status_code, expected $expected_status)"
        return 1
    fi
}

# Test API endpoint with JSON response
test_api_endpoint() {
    local name=$1
    local url=$2
    local method=${3:-GET}
    local data=${4:-""}
    
    print_status "Testing $name API..."
    
    local response
    if [ "$method" = "POST" ] && [ -n "$data" ]; then
        response=$(curl -s -X POST "$url" \
            -H "Content-Type: application/json" \
            -d "$data" || echo "ERROR")
    else
        response=$(curl -s "$url" || echo "ERROR")
    fi
    
    if [[ "$response" != "ERROR" ]] && [[ "$response" == *"{"* ]]; then
        print_success "$name API is working"
        return 0
    else
        print_error "$name API failed: $response"
        return 1
    fi
}

# Main test execution
main() {
    echo "🧪 Testing Nexus AI-Native Platform"
    echo "==================================="
    
    local failed_tests=0
    local total_tests=0
    
    # Infrastructure tests
    echo ""
    echo "🏗️  Infrastructure Services:"
    
    tests=(
        "Kong Admin API:http://localhost:8001/status:200"
        "Kong Proxy:http://localhost:8000:404"
        "Kuma Control Plane:http://localhost:5681:200"
        "Prometheus:http://localhost:9090/-/healthy:200"
        "Grafana:http://localhost:3000/api/health:200"
        "Jaeger:http://localhost:16686:200"
    )
    
    for test in "${tests[@]}"; do
        IFS=':' read -r name url expected <<< "$test"
        total_tests=$((total_tests + 1))
        if ! test_endpoint "$name" "$url" "$expected"; then
            failed_tests=$((failed_tests + 1))
        fi
    done
    
    # Database tests
    echo ""
    echo "🗄️  Database Services:"
    
    # Test PostgreSQL
    total_tests=$((total_tests + 1))
    print_status "Testing PostgreSQL connection..."
    if docker exec nexus-postgres-1 pg_isready -U nexus > /dev/null 2>&1; then
        print_success "PostgreSQL is ready"
    else
        print_error "PostgreSQL connection failed"
        failed_tests=$((failed_tests + 1))
    fi
    
    # Test Redis
    total_tests=$((total_tests + 1))
    print_status "Testing Redis connection..."
    if docker exec nexus-redis-1 redis-cli -a nexus-password ping > /dev/null 2>&1; then
        print_success "Redis is ready"
    else
        print_error "Redis connection failed"
        failed_tests=$((failed_tests + 1))
    fi
    
    # Test MongoDB
    total_tests=$((total_tests + 1))
    print_status "Testing MongoDB connection..."
    if docker exec nexus-mongodb-1 mongosh --quiet --eval "db.adminCommand('ping')" > /dev/null 2>&1; then
        print_success "MongoDB is ready"
    else
        print_error "MongoDB connection failed"
        failed_tests=$((failed_tests + 1))
    fi
    
    # AI/ML Infrastructure tests
    echo ""
    echo "🤖 AI/ML Infrastructure:"
    
    ai_infrastructure=(
        "MLflow Server:http://localhost:5000/health:200"
        "Jupyter Lab:http://localhost:8888:200"
        "MinIO:http://localhost:9000/minio/health/live:200"
        "Qdrant Vector DB:http://localhost:6333/health:200"
        "Airflow:http://localhost:8080/health:200"
    )
    
    for test in "${ai_infrastructure[@]}"; do
        IFS=':' read -r name url expected <<< "$test"
        total_tests=$((total_tests + 1))
        if ! test_endpoint "$name" "$url" "$expected"; then
            failed_tests=$((failed_tests + 1))
        fi
    done
    
    # Core Microservices tests
    echo ""
    echo "🔧 Core Microservices:"
    
    core_services=(
        "Authorization Service:http://localhost:8080/health:200"
        "User Management Service:http://localhost:8082/health:200"
        "Configuration Service:http://localhost:8083/health:200"
        "Health Check Service:http://localhost:8085/health:200"
    )
    
    for service in "${core_services[@]}"; do
        IFS=':' read -r name url expected <<< "$service"
        total_tests=$((total_tests + 1))
        if ! test_endpoint "$name" "$url" "$expected"; then
            failed_tests=$((failed_tests + 1))
        fi
    done
    
    # AI/ML Microservices tests
    echo ""
    echo "🧠 AI/ML Microservices:"
    
    ai_services=(
        "Model Management Service:http://localhost:8086/health:200"
        "Model Training Service:http://localhost:8087/health:200"
    )
    
    for service in "${ai_services[@]}"; do
        IFS=':' read -r name url expected <<< "$service"
        total_tests=$((total_tests + 1))
        if ! test_endpoint "$name" "$url" "$expected"; then
            failed_tests=$((failed_tests + 1))
        fi
    done
    
    # API Gateway routing tests
    echo ""
    echo "🌐 API Gateway Routing:"
    
    gateway_routes=(
        "Health Route:http://localhost:8000/health:200"
        "Models Route:http://localhost:8000/models/health:200"
        "Training Route:http://localhost:8000/training/health:200"
    )
    
    for route in "${gateway_routes[@]}"; do
        IFS=':' read -r name url expected <<< "$route"
        total_tests=$((total_tests + 1))
        if ! test_endpoint "$name" "$url" "$expected"; then
            failed_tests=$((failed_tests + 1))
        fi
    done
    
    # AI/ML API functionality tests
    echo ""
    echo "🤖 AI/ML API Functionality:"
    
    # Test Model Management API
    total_tests=$((total_tests + 1))
    if test_api_endpoint "Model Management - List Models" "http://localhost:8086/models"; then
        print_success "Model Management API is functional"
    else
        print_error "Model Management API failed"
        failed_tests=$((failed_tests + 1))
    fi
    
    # Test Model Training API
    total_tests=$((total_tests + 1))
    if test_api_endpoint "Model Training - List Jobs" "http://localhost:8087/training-jobs"; then
        print_success "Model Training API is functional"
    else
        print_error "Model Training API failed"
        failed_tests=$((failed_tests + 1))
    fi
    
    # Test MLflow API
    total_tests=$((total_tests + 1))
    if test_api_endpoint "MLflow - List Experiments" "http://localhost:5000/api/2.0/mlflow/experiments/search" "POST" '{}'; then
        print_success "MLflow API is functional"
    else
        print_error "MLflow API failed"
        failed_tests=$((failed_tests + 1))
    fi
    
    # End-to-end AI workflow test
    echo ""
    echo "🔄 End-to-End AI Workflow Test:"
    
    # Create a simple training job
    total_tests=$((total_tests + 1))
    print_ai "Creating a test training job..."
    
    training_job_data='{
        "job_name": "test-classifier",
        "model_type": "classifier",
        "algorithm": "random_forest",
        "target_column": "target",
        "hyperparameter_tuning": false,
        "test_size": 0.2
    }'
    
    job_response=$(curl -s -X POST "http://localhost:8087/training-jobs" \
        -H "Content-Type: application/json" \
        -d "$training_job_data" || echo "ERROR")
    
    if [[ "$job_response" != "ERROR" ]] && [[ "$job_response" == *"job_id"* ]]; then
        print_success "Training job created successfully"
        
        # Extract job ID
        job_id=$(echo "$job_response" | grep -o '"job_id":"[^"]*"' | cut -d'"' -f4)
        print_ai "Job ID: $job_id"
        
        # Wait a moment and check job status
        sleep 2
        job_status=$(curl -s "http://localhost:8087/training-jobs/$job_id" || echo "ERROR")
        
        if [[ "$job_status" != "ERROR" ]] && [[ "$job_status" == *"status"* ]]; then
            print_success "Training job status retrieved successfully"
        else
            print_error "Failed to retrieve training job status"
            failed_tests=$((failed_tests + 1))
        fi
    else
        print_error "Failed to create training job: $job_response"
        failed_tests=$((failed_tests + 1))
    fi
    
    # Summary
    echo ""
    echo "📊 Test Summary:"
    echo "================"
    
    local passed_tests=$((total_tests - failed_tests))
    echo "Total Tests: $total_tests"
    echo "Passed: $passed_tests"
    echo "Failed: $failed_tests"
    
    if [ $failed_tests -eq 0 ]; then
        print_success "All tests passed! 🎉"
        echo ""
        echo "✅ Your Nexus AI-Native Platform is fully operational!"
        echo ""
        echo "🚀 Next steps:"
        echo "   - Open MLflow UI: http://localhost:5000"
        echo "   - Open Jupyter Lab: http://localhost:8888 (token: nexus-jupyter-token)"
        echo "   - Open Airflow: http://localhost:8080 (admin/admin)"
        echo "   - Open Grafana: http://localhost:3000 (admin/admin)"
        echo "   - Start training ML models!"
        echo ""
        echo "📚 Example API calls:"
        echo "   # List models:"
        echo "   curl http://localhost:8000/models/models"
        echo ""
        echo "   # Create training job:"
        echo "   curl -X POST http://localhost:8000/training/training-jobs \\"
        echo "     -H 'Content-Type: application/json' \\"
        echo "     -d '{\"job_name\":\"my-model\",\"model_type\":\"classifier\",\"algorithm\":\"random_forest\",\"target_column\":\"target\"}'"
        echo ""
        return 0
    else
        print_error "Some tests failed. Please check the services and try again."
        echo ""
        echo "🔧 Troubleshooting:"
        echo "   - Check service logs: docker-compose logs [service-name]"
        echo "   - Restart services: ./scripts/stop-ai-platform.sh && ./scripts/start-ai-platform.sh"
        echo "   - Check port conflicts: lsof -i :[port]"
        echo "   - Verify Docker resources: docker system df"
        return 1
    fi
}

# Run main function
main "$@"
