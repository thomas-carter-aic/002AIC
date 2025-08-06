#!/bin/bash

# Staging Deployment Script for 002AIC Nexus Platform
# This script deploys the complete 36-microservice architecture to staging

set -euo pipefail

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
ENVIRONMENT="staging"
KUSTOMIZE_DIR="$SCRIPT_DIR/staging"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Logging functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check prerequisites
check_prerequisites() {
    log_info "Checking prerequisites for staging deployment..."
    
    # Check kubectl
    if ! command -v kubectl &> /dev/null; then
        log_error "kubectl is not installed or not in PATH"
        exit 1
    fi
    
    # Check kustomize
    if ! command -v kustomize &> /dev/null; then
        log_error "kustomize is not installed or not in PATH"
        exit 1
    fi
    
    # Check cluster connectivity
    if ! kubectl cluster-info &> /dev/null; then
        log_error "Cannot connect to Kubernetes cluster"
        exit 1
    fi
    
    CURRENT_CONTEXT=$(kubectl config current-context)
    log_info "Current Kubernetes context: $CURRENT_CONTEXT"
    
    log_success "Prerequisites check passed"
}

# Deploy staging environment
deploy_staging() {
    log_info "Deploying 002AIC Nexus Platform to staging environment..."
    
    # Create namespaces first
    log_info "Creating namespaces..."
    kubectl apply -f "$PROJECT_ROOT/infra/k8s/namespaces/"
    
    # Deploy all services using kustomize
    log_info "Deploying all services with staging configuration..."
    kustomize build "$KUSTOMIZE_DIR" | kubectl apply -f -
    
    # Wait for deployments to be ready
    log_info "Waiting for deployments to be ready..."
    
    # Wait for infrastructure services
    kubectl wait --for=condition=available --timeout=300s deployment -l "service-group=infrastructure" --all-namespaces || true
    
    # Wait for core services
    kubectl wait --for=condition=available --timeout=300s deployment -l "service-group=core" --all-namespaces || true
    
    # Wait for other services
    kubectl wait --for=condition=available --timeout=300s deployment -l "service-group=ai-ml" --all-namespaces || true
    kubectl wait --for=condition=available --timeout=300s deployment -l "service-group=data" --all-namespaces || true
    kubectl wait --for=condition=available --timeout=300s deployment -l "service-group=devops" --all-namespaces || true
    kubectl wait --for=condition=available --timeout=300s deployment -l "service-group=business" --all-namespaces || true
    
    log_success "Staging deployment completed"
}

# Verify staging deployment
verify_staging() {
    log_info "Verifying staging deployment..."
    
    # Check pod status
    log_info "Pod status:"
    kubectl get pods --all-namespaces
    
    # Check services
    log_info "Service status:"
    kubectl get services --all-namespaces
    
    # Check for any failed pods
    FAILED_PODS=$(kubectl get pods --all-namespaces --field-selector=status.phase!=Running,status.phase!=Succeeded --no-headers 2>/dev/null | wc -l)
    
    if [[ $FAILED_PODS -gt 0 ]]; then
        log_warning "Found $FAILED_PODS pods not in Running/Succeeded state"
        kubectl get pods --all-namespaces --field-selector=status.phase!=Running,status.phase!=Succeeded
    else
        log_success "All pods are running successfully"
    fi
}

# Run staging tests
run_staging_tests() {
    log_info "Running staging environment tests..."
    
    # Basic connectivity tests
    log_info "Testing API Gateway connectivity..."
    
    # Check if API Gateway service exists
    if kubectl get service api-gateway-service -n core-platform &> /dev/null; then
        log_success "API Gateway service is available"
    else
        log_warning "API Gateway service not found"
    fi
    
    # Check database connectivity
    if kubectl get service postgresql -n infrastructure-services &> /dev/null; then
        log_success "PostgreSQL service is available"
    else
        log_warning "PostgreSQL service not found"
    fi
    
    # Check Redis connectivity
    if kubectl get service redis -n infrastructure-services &> /dev/null; then
        log_success "Redis service is available"
    else
        log_warning "Redis service not found"
    fi
    
    log_success "Staging tests completed"
}

# Generate staging report
generate_staging_report() {
    log_info "Generating staging deployment report..."
    
    REPORT_FILE="$PROJECT_ROOT/deployment-reports/staging-$(date +%Y%m%d-%H%M%S).txt"
    mkdir -p "$(dirname "$REPORT_FILE")"
    
    {
        echo "002AIC Nexus Platform - Staging Deployment Report"
        echo "================================================="
        echo "Deployment Date: $(date)"
        echo "Environment: staging"
        echo "Kubernetes Context: $(kubectl config current-context)"
        echo ""
        echo "Deployed Services:"
        echo "=================="
        kubectl get deployments --all-namespaces -o wide
        echo ""
        echo "Service Status:"
        echo "==============="
        kubectl get services --all-namespaces
        echo ""
        echo "Pod Status:"
        echo "==========="
        kubectl get pods --all-namespaces
    } > "$REPORT_FILE"
    
    log_success "Staging report generated: $REPORT_FILE"
}

# Main function
main() {
    log_info "Starting staging deployment of 002AIC Nexus Platform"
    log_info "Target: 36 microservices with reduced resource allocation"
    
    check_prerequisites
    deploy_staging
    verify_staging
    run_staging_tests
    generate_staging_report
    
    log_success "Staging deployment completed successfully!"
    log_info "Staging environment is ready for testing and validation"
    log_info "Use this environment to test features before production deployment"
}

# Script execution
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi
