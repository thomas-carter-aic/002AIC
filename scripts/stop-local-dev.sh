#!/bin/bash

# Nexus Platform - Local Development Environment Stop Script
# This script stops the complete local development environment

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

# Main execution
main() {
    print_status "Stopping Nexus Platform Local Development Environment..."
    
    # Change to compose directory
    cd "$COMPOSE_DIR"
    
    # Stop all services
    print_status "Stopping microservices..."
    docker-compose -f nexus-core.yml down
    
    print_status "Stopping infrastructure services..."
    docker-compose -f kong-kuma-local.yml down
    
    # Optional: Remove volumes (uncomment if you want to clean data)
    # print_warning "Removing all data volumes..."
    # docker-compose -f nexus-core.yml down -v
    # docker-compose -f kong-kuma-local.yml down -v
    
    # Optional: Remove network (uncomment if you want to clean network)
    # print_status "Removing nexus-net network..."
    # docker network rm nexus-net 2>/dev/null || true
    
    print_success "Nexus Platform stopped successfully!"
    print_status "Data volumes are preserved. Use 'docker-compose down -v' to remove data."
}

# Run main function
main "$@"
