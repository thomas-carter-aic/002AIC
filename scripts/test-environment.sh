#!/bin/bash

# Nexus Platform - Environment Test Script
# This script tests the local development environment

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
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

# Main test execution
main() {
    echo "🧪 Testing Nexus Platform Local Development Environment"
    echo "=================================================="
    
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
    
    # Microservices tests (if running)
    echo ""
    echo "🔧 Microservices:"
    
    microservices=(
        "Authorization Service:http://localhost:8080/health:200"
        "API Gateway Service:http://localhost:8081/health:200"
        "User Management Service:http://localhost:8082/health:200"
        "Configuration Service:http://localhost:8083/health:200"
        "Discovery Service:http://localhost:8084/health:200"
        "Health Check Service:http://localhost:8085/health:200"
    )
    
    for service in "${microservices[@]}"; do
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
        "Auth Route:http://localhost:8000/auth/health:200"
        "Users Route:http://localhost:8000/users/health:200"
        "Config Route:http://localhost:8000/config/health:200"
        "Discovery Route:http://localhost:8000/discovery/health:200"
        "Health Route:http://localhost:8000/health:200"
    )
    
    for route in "${gateway_routes[@]}"; do
        IFS=':' read -r name url expected <<< "$route"
        total_tests=$((total_tests + 1))
        if ! test_endpoint "$name" "$url" "$expected"; then
            failed_tests=$((failed_tests + 1))
        fi
    done
    
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
        echo "✅ Your Nexus Platform development environment is ready!"
        echo ""
        echo "🚀 Next steps:"
        echo "   - Open Grafana: http://localhost:3000 (admin/admin)"
        echo "   - View API docs: http://localhost:8001 (Kong Admin)"
        echo "   - Check service health: http://localhost:8000/health"
        echo "   - Start developing your microservices!"
        return 0
    else
        print_error "Some tests failed. Please check the services and try again."
        echo ""
        echo "🔧 Troubleshooting:"
        echo "   - Check service logs: docker-compose logs [service-name]"
        echo "   - Restart services: ./scripts/stop-local-dev.sh && ./scripts/start-local-dev.sh"
        echo "   - Check port conflicts: lsof -i :[port]"
        return 1
    fi
}

# Run main function
main "$@"
