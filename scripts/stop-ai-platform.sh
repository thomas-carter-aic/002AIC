#!/bin/bash

# Nexus Platform - AI/ML Platform Stop Script
# This script stops the complete AI-native platform

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

print_ai() {
    echo -e "${PURPLE}[AI/ML]${NC} $1"
}

# Main execution
main() {
    echo "🛑 Stopping Nexus AI-Native Platform"
    echo "===================================="
    
    # Change to compose directory
    cd "$COMPOSE_DIR"
    
    # Stop AI/ML microservices
    print_ai "🤖 Stopping AI/ML microservices..."
    docker-compose -f nexus-ai-services.yml down
    
    # Stop core microservices
    print_status "🔧 Stopping core microservices..."
    docker-compose -f nexus-core.yml down
    
    # Stop AI/ML infrastructure
    print_ai "🧠 Stopping AI/ML infrastructure..."
    docker-compose -f ai-ml-stack.yml down
    
    # Stop infrastructure services
    print_status "🏗️  Stopping infrastructure services..."
    docker-compose -f kong-kuma-local.yml down
    
    # Optional: Remove volumes (uncomment if you want to clean data)
    # print_warning "🗑️  Removing all data volumes..."
    # docker-compose -f nexus-ai-services.yml down -v
    # docker-compose -f nexus-core.yml down -v
    # docker-compose -f ai-ml-stack.yml down -v
    # docker-compose -f kong-kuma-local.yml down -v
    
    # Optional: Remove network (uncomment if you want to clean network)
    # print_status "🌐 Removing nexus-net network..."
    # docker network rm nexus-net 2>/dev/null || true
    
    print_success "🎉 Nexus AI-Native Platform stopped successfully!"
    print_status "📦 Data volumes are preserved. Use 'docker-compose down -v' to remove data."
    print_ai "🧠 ML models and training data are preserved in volumes."
}

# Run main function
main "$@"
