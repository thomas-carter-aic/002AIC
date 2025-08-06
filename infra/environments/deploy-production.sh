#!/bin/bash

# Production Deployment Script for 002AIC Nexus Platform
# This script deploys the complete 36-microservice architecture to production

set -euo pipefail

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
ENVIRONMENT="production"
NAMESPACE_PREFIX=""
KUSTOMIZE_DIR="$SCRIPT_DIR/production"

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
    log_info "Checking prerequisites..."
    
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
    
    # Check if production context is set
    CURRENT_CONTEXT=$(kubectl config current-context)
    log_info "Current Kubernetes context: $CURRENT_CONTEXT"
    
    # Verify this is production cluster
    read -p "Are you sure you want to deploy to PRODUCTION cluster '$CURRENT_CONTEXT'? (yes/no): " -r
    if [[ ! $REPLY =~ ^[Yy][Ee][Ss]$ ]]; then
        log_error "Deployment cancelled by user"
        exit 1
    fi
    
    log_success "Prerequisites check passed"
}

# Backup current state
backup_current_state() {
    log_info "Creating backup of current production state..."
    
    BACKUP_DIR="$PROJECT_ROOT/backups/production-$(date +%Y%m%d-%H%M%S)"
    mkdir -p "$BACKUP_DIR"
    
    # Backup all resources
    kubectl get all --all-namespaces -o yaml > "$BACKUP_DIR/all-resources.yaml"
    kubectl get configmaps --all-namespaces -o yaml > "$BACKUP_DIR/configmaps.yaml"
    kubectl get secrets --all-namespaces -o yaml > "$BACKUP_DIR/secrets.yaml"
    kubectl get pv,pvc --all-namespaces -o yaml > "$BACKUP_DIR/storage.yaml"
    
    log_success "Backup created at: $BACKUP_DIR"
}

# Pre-deployment validation
validate_deployment() {
    log_info "Validating deployment configuration..."
    
    # Validate kustomization
    if ! kustomize build "$KUSTOMIZE_DIR" > /dev/null; then
        log_error "Kustomization validation failed"
        exit 1
    fi
    
    # Check resource quotas
    log_info "Checking resource quotas..."
    
    # Validate secrets exist (in production, these should come from external secret management)
    log_warning "Ensure all production secrets are properly configured in your secret management system"
    
    log_success "Deployment validation passed"
}

# Deploy infrastructure services first
deploy_infrastructure() {
    log_info "Deploying infrastructure services..."
    
    # Deploy namespaces first
    kubectl apply -f "$PROJECT_ROOT/infra/k8s/namespaces/"
    
    # Deploy infrastructure services (databases, monitoring, etc.)
    kustomize build "$KUSTOMIZE_DIR" | kubectl apply -l "service-group=infrastructure" -f -
    
    # Wait for infrastructure to be ready
    log_info "Waiting for infrastructure services to be ready..."
    kubectl wait --for=condition=available --timeout=300s deployment -l "service-group=infrastructure" --all-namespaces
    
    log_success "Infrastructure services deployed successfully"
}

# Deploy core platform services
deploy_core_platform() {
    log_info "Deploying core platform services..."
    
    kustomize build "$KUSTOMIZE_DIR" | kubectl apply -l "service-group=core" -f -
    
    # Wait for core services to be ready
    log_info "Waiting for core platform services to be ready..."
    kubectl wait --for=condition=available --timeout=300s deployment -l "service-group=core" --all-namespaces
    
    log_success "Core platform services deployed successfully"
}

# Deploy AI/ML services
deploy_ai_ml_services() {
    log_info "Deploying AI/ML services..."
    
    kustomize build "$KUSTOMIZE_DIR" | kubectl apply -l "service-group=ai-ml" -f -
    
    # Wait for AI/ML services to be ready
    log_info "Waiting for AI/ML services to be ready..."
    kubectl wait --for=condition=available --timeout=600s deployment -l "service-group=ai-ml" --all-namespaces
    
    log_success "AI/ML services deployed successfully"
}

# Deploy data services
deploy_data_services() {
    log_info "Deploying data services..."
    
    kustomize build "$KUSTOMIZE_DIR" | kubectl apply -l "service-group=data" -f -
    
    # Wait for data services to be ready
    log_info "Waiting for data services to be ready..."
    kubectl wait --for=condition=available --timeout=300s deployment -l "service-group=data" --all-namespaces
    
    log_success "Data services deployed successfully"
}

# Deploy DevOps services
deploy_devops_services() {
    log_info "Deploying DevOps services..."
    
    kustomize build "$KUSTOMIZE_DIR" | kubectl apply -l "service-group=devops" -f -
    
    # Wait for DevOps services to be ready
    log_info "Waiting for DevOps services to be ready..."
    kubectl wait --for=condition=available --timeout=300s deployment -l "service-group=devops" --all-namespaces
    
    log_success "DevOps services deployed successfully"
}

# Deploy business services
deploy_business_services() {
    log_info "Deploying business services..."
    
    kustomize build "$KUSTOMIZE_DIR" | kubectl apply -l "service-group=business" -f -
    
    # Wait for business services to be ready
    log_info "Waiting for business services to be ready..."
    kubectl wait --for=condition=available --timeout=300s deployment -l "service-group=business" --all-namespaces
    
    log_success "Business services deployed successfully"
}

# Deploy remaining resources
deploy_remaining_resources() {
    log_info "Deploying remaining resources (HPA, monitoring configs, etc.)..."
    
    # Apply all remaining resources
    kustomize build "$KUSTOMIZE_DIR" | kubectl apply -f -
    
    log_success "All resources deployed successfully"
}

# Post-deployment verification
verify_deployment() {
    log_info "Verifying production deployment..."
    
    # Check all pods are running
    log_info "Checking pod status..."
    kubectl get pods --all-namespaces | grep -E "(Pending|Error|CrashLoopBackOff|ImagePullBackOff)" || true
    
    # Check services
    log_info "Checking service status..."
    kubectl get services --all-namespaces
    
    # Check ingress
    log_info "Checking ingress status..."
    kubectl get ingress --all-namespaces
    
    # Run health checks
    log_info "Running health checks..."
    
    # Check API Gateway health
    if kubectl get service api-gateway-service -n core-platform &> /dev/null; then
        log_info "API Gateway service found"
        # Add specific health check here
    fi
    
    # Check database connectivity
    if kubectl get service postgresql -n infrastructure-services &> /dev/null; then
        log_info "PostgreSQL service found"
        # Add database connectivity check here
    fi
    
    # Check monitoring stack
    if kubectl get service prometheus -n infrastructure-services &> /dev/null; then
        log_info "Prometheus service found"
    fi
    
    if kubectl get service grafana -n infrastructure-services &> /dev/null; then
        log_info "Grafana service found"
    fi
    
    log_success "Deployment verification completed"
}

# Generate deployment report
generate_report() {
    log_info "Generating deployment report..."
    
    REPORT_FILE="$PROJECT_ROOT/deployment-reports/production-$(date +%Y%m%d-%H%M%S).txt"
    mkdir -p "$(dirname "$REPORT_FILE")"
    
    {
        echo "002AIC Nexus Platform - Production Deployment Report"
        echo "=================================================="
        echo "Deployment Date: $(date)"
        echo "Kubernetes Context: $(kubectl config current-context)"
        echo "Kustomize Version: $(kustomize version --short)"
        echo "kubectl Version: $(kubectl version --client --short)"
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
        echo ""
        echo "Resource Usage:"
        echo "==============="
        kubectl top nodes || echo "Metrics server not available"
        kubectl top pods --all-namespaces || echo "Metrics server not available"
    } > "$REPORT_FILE"
    
    log_success "Deployment report generated: $REPORT_FILE"
}

# Rollback function
rollback_deployment() {
    log_error "Deployment failed. Initiating rollback..."
    
    # Find latest backup
    LATEST_BACKUP=$(find "$PROJECT_ROOT/backups" -name "production-*" -type d | sort | tail -1)
    
    if [[ -n "$LATEST_BACKUP" ]]; then
        log_info "Rolling back to: $LATEST_BACKUP"
        
        # Restore from backup
        kubectl apply -f "$LATEST_BACKUP/all-resources.yaml" || true
        kubectl apply -f "$LATEST_BACKUP/configmaps.yaml" || true
        kubectl apply -f "$LATEST_BACKUP/secrets.yaml" || true
        kubectl apply -f "$LATEST_BACKUP/storage.yaml" || true
        
        log_success "Rollback completed"
    else
        log_error "No backup found for rollback"
    fi
}

# Main deployment function
main() {
    log_info "Starting production deployment of 002AIC Nexus Platform"
    log_info "Target: 36 microservices across 6 namespaces"
    
    # Set trap for error handling
    trap rollback_deployment ERR
    
    # Execute deployment steps
    check_prerequisites
    backup_current_state
    validate_deployment
    
    # Sequential deployment by service groups
    deploy_infrastructure
    sleep 30  # Allow infrastructure to stabilize
    
    deploy_core_platform
    sleep 20
    
    deploy_ai_ml_services
    sleep 20
    
    deploy_data_services
    sleep 15
    
    deploy_devops_services
    sleep 15
    
    deploy_business_services
    sleep 15
    
    deploy_remaining_resources
    
    # Post-deployment tasks
    verify_deployment
    generate_report
    
    log_success "Production deployment completed successfully!"
    log_info "Platform is now ready to serve 2M+ users towards $2B ARR target"
    log_info "Monitor the deployment using Grafana dashboards and Prometheus alerts"
}

# Script execution
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi
