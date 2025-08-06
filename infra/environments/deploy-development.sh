#!/bin/bash

# Development Deployment Script for 002AIC Nexus Platform
# This script deploys the complete 36-microservice architecture to development

set -euo pipefail

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
ENVIRONMENT="development"
KUSTOMIZE_DIR="$SCRIPT_DIR/development"

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
    log_info "Checking prerequisites for development deployment..."
    
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

# Setup development environment
setup_development() {
    log_info "Setting up development environment..."
    
    # Create development namespace if it doesn't exist
    kubectl create namespace development --dry-run=client -o yaml | kubectl apply -f -
    
    # Label namespace for development
    kubectl label namespace development environment=development --overwrite
    
    log_success "Development environment setup completed"
}

# Deploy development environment
deploy_development() {
    log_info "Deploying 002AIC Nexus Platform to development environment..."
    
    # Create namespaces first
    log_info "Creating namespaces..."
    kubectl apply -f "$PROJECT_ROOT/infra/k8s/namespaces/"
    
    # Deploy all services using kustomize
    log_info "Deploying all services with development configuration..."
    kustomize build "$KUSTOMIZE_DIR" | kubectl apply -f -
    
    # Wait for deployments to be ready (shorter timeout for dev)
    log_info "Waiting for deployments to be ready..."
    
    # Wait for infrastructure services
    kubectl wait --for=condition=available --timeout=180s deployment -l "service-group=infrastructure" --all-namespaces || true
    
    # Wait for core services
    kubectl wait --for=condition=available --timeout=180s deployment -l "service-group=core" --all-namespaces || true
    
    # Wait for other services (with relaxed timeout for development)
    kubectl wait --for=condition=available --timeout=120s deployment -l "service-group=ai-ml" --all-namespaces || true
    kubectl wait --for=condition=available --timeout=120s deployment -l "service-group=data" --all-namespaces || true
    kubectl wait --for=condition=available --timeout=120s deployment -l "service-group=devops" --all-namespaces || true
    kubectl wait --for=condition=available --timeout=120s deployment -l "service-group=business" --all-namespaces || true
    
    log_success "Development deployment completed"
}

# Setup port forwarding for development
setup_port_forwarding() {
    log_info "Setting up port forwarding for development access..."
    
    # Create port forwarding script
    PORT_FORWARD_SCRIPT="$PROJECT_ROOT/scripts/dev-port-forward.sh"
    mkdir -p "$(dirname "$PORT_FORWARD_SCRIPT")"
    
    cat > "$PORT_FORWARD_SCRIPT" << 'EOF'
#!/bin/bash

# Development Port Forwarding Script
# Run this script to access services locally during development

echo "Setting up port forwarding for development services..."

# API Gateway
kubectl port-forward -n core-platform service/api-gateway-service 8080:8080 &
echo "API Gateway: http://localhost:8080"

# PostgreSQL
kubectl port-forward -n infrastructure-services service/postgresql 5432:5432 &
echo "PostgreSQL: localhost:5432"

# Redis
kubectl port-forward -n infrastructure-services service/redis 6379:6379 &
echo "Redis: localhost:6379"

# Grafana
kubectl port-forward -n infrastructure-services service/grafana 3000:3000 &
echo "Grafana: http://localhost:3000"

# Prometheus
kubectl port-forward -n infrastructure-services service/prometheus 9090:9090 &
echo "Prometheus: http://localhost:9090"

echo ""
echo "Port forwarding is now active. Press Ctrl+C to stop all port forwards."
echo "Services are accessible at the URLs shown above."

# Wait for interrupt
trap 'echo "Stopping port forwards..."; kill $(jobs -p); exit' INT
wait
EOF
    
    chmod +x "$PORT_FORWARD_SCRIPT"
    
    log_success "Port forwarding script created: $PORT_FORWARD_SCRIPT"
    log_info "Run '$PORT_FORWARD_SCRIPT' to access services locally"
}

# Verify development deployment
verify_development() {
    log_info "Verifying development deployment..."
    
    # Check pod status
    log_info "Pod status:"
    kubectl get pods --all-namespaces | head -20
    
    # Check services
    log_info "Service status:"
    kubectl get services --all-namespaces | head -15
    
    # Check for any failed pods
    FAILED_PODS=$(kubectl get pods --all-namespaces --field-selector=status.phase!=Running,status.phase!=Succeeded --no-headers 2>/dev/null | wc -l)
    
    if [[ $FAILED_PODS -gt 0 ]]; then
        log_warning "Found $FAILED_PODS pods not in Running/Succeeded state"
        kubectl get pods --all-namespaces --field-selector=status.phase!=Running,status.phase!=Succeeded
    else
        log_success "All pods are running successfully"
    fi
}

# Create development utilities
create_dev_utilities() {
    log_info "Creating development utilities..."
    
    # Create development helper scripts directory
    DEV_SCRIPTS_DIR="$PROJECT_ROOT/scripts/development"
    mkdir -p "$DEV_SCRIPTS_DIR"
    
    # Create service logs script
    cat > "$DEV_SCRIPTS_DIR/logs.sh" << 'EOF'
#!/bin/bash

# Development Logs Script
# Usage: ./logs.sh [service-name] [namespace]

SERVICE_NAME=${1:-""}
NAMESPACE=${2:-""}

if [[ -z "$SERVICE_NAME" ]]; then
    echo "Available services:"
    kubectl get deployments --all-namespaces -o custom-columns=NAMESPACE:.metadata.namespace,NAME:.metadata.name --no-headers
    echo ""
    echo "Usage: $0 <service-name> [namespace]"
    exit 1
fi

if [[ -n "$NAMESPACE" ]]; then
    kubectl logs -f deployment/$SERVICE_NAME -n $NAMESPACE
else
    # Try to find the service in any namespace
    FOUND_NAMESPACE=$(kubectl get deployment $SERVICE_NAME --all-namespaces -o jsonpath='{.items[0].metadata.namespace}' 2>/dev/null)
    if [[ -n "$FOUND_NAMESPACE" ]]; then
        echo "Found $SERVICE_NAME in namespace: $FOUND_NAMESPACE"
        kubectl logs -f deployment/$SERVICE_NAME -n $FOUND_NAMESPACE
    else
        echo "Service $SERVICE_NAME not found in any namespace"
        exit 1
    fi
fi
EOF
    
    # Create service shell access script
    cat > "$DEV_SCRIPTS_DIR/shell.sh" << 'EOF'
#!/bin/bash

# Development Shell Access Script
# Usage: ./shell.sh [service-name] [namespace]

SERVICE_NAME=${1:-""}
NAMESPACE=${2:-""}

if [[ -z "$SERVICE_NAME" ]]; then
    echo "Available services:"
    kubectl get deployments --all-namespaces -o custom-columns=NAMESPACE:.metadata.namespace,NAME:.metadata.name --no-headers
    echo ""
    echo "Usage: $0 <service-name> [namespace]"
    exit 1
fi

if [[ -n "$NAMESPACE" ]]; then
    kubectl exec -it deployment/$SERVICE_NAME -n $NAMESPACE -- /bin/bash
else
    # Try to find the service in any namespace
    FOUND_NAMESPACE=$(kubectl get deployment $SERVICE_NAME --all-namespaces -o jsonpath='{.items[0].metadata.namespace}' 2>/dev/null)
    if [[ -n "$FOUND_NAMESPACE" ]]; then
        echo "Found $SERVICE_NAME in namespace: $FOUND_NAMESPACE"
        kubectl exec -it deployment/$SERVICE_NAME -n $FOUND_NAMESPACE -- /bin/bash
    else
        echo "Service $SERVICE_NAME not found in any namespace"
        exit 1
    fi
fi
EOF
    
    # Create restart service script
    cat > "$DEV_SCRIPTS_DIR/restart.sh" << 'EOF'
#!/bin/bash

# Development Service Restart Script
# Usage: ./restart.sh [service-name] [namespace]

SERVICE_NAME=${1:-""}
NAMESPACE=${2:-""}

if [[ -z "$SERVICE_NAME" ]]; then
    echo "Available services:"
    kubectl get deployments --all-namespaces -o custom-columns=NAMESPACE:.metadata.namespace,NAME:.metadata.name --no-headers
    echo ""
    echo "Usage: $0 <service-name> [namespace]"
    exit 1
fi

if [[ -n "$NAMESPACE" ]]; then
    kubectl rollout restart deployment/$SERVICE_NAME -n $NAMESPACE
    kubectl rollout status deployment/$SERVICE_NAME -n $NAMESPACE
else
    # Try to find the service in any namespace
    FOUND_NAMESPACE=$(kubectl get deployment $SERVICE_NAME --all-namespaces -o jsonpath='{.items[0].metadata.namespace}' 2>/dev/null)
    if [[ -n "$FOUND_NAMESPACE" ]]; then
        echo "Found $SERVICE_NAME in namespace: $FOUND_NAMESPACE"
        kubectl rollout restart deployment/$SERVICE_NAME -n $FOUND_NAMESPACE
        kubectl rollout status deployment/$SERVICE_NAME -n $FOUND_NAMESPACE
    else
        echo "Service $SERVICE_NAME not found in any namespace"
        exit 1
    fi
fi
EOF
    
    # Make scripts executable
    chmod +x "$DEV_SCRIPTS_DIR"/*.sh
    
    log_success "Development utilities created in: $DEV_SCRIPTS_DIR"
}

# Generate development report
generate_development_report() {
    log_info "Generating development deployment report..."
    
    REPORT_FILE="$PROJECT_ROOT/deployment-reports/development-$(date +%Y%m%d-%H%M%S).txt"
    mkdir -p "$(dirname "$REPORT_FILE")"
    
    {
        echo "002AIC Nexus Platform - Development Deployment Report"
        echo "====================================================="
        echo "Deployment Date: $(date)"
        echo "Environment: development"
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
        echo ""
        echo "Development Access:"
        echo "==================="
        echo "Port Forward Script: $PROJECT_ROOT/scripts/dev-port-forward.sh"
        echo "Development Utilities: $PROJECT_ROOT/scripts/development/"
    } > "$REPORT_FILE"
    
    log_success "Development report generated: $REPORT_FILE"
}

# Main function
main() {
    log_info "Starting development deployment of 002AIC Nexus Platform"
    log_info "Target: 36 microservices with minimal resource allocation for local development"
    
    check_prerequisites
    setup_development
    deploy_development
    setup_port_forwarding
    verify_development
    create_dev_utilities
    generate_development_report
    
    log_success "Development deployment completed successfully!"
    log_info "Development environment is ready for local development and testing"
    log_info ""
    log_info "Next steps:"
    log_info "1. Run port forwarding: $PROJECT_ROOT/scripts/dev-port-forward.sh"
    log_info "2. Access services locally via the forwarded ports"
    log_info "3. Use development utilities in: $PROJECT_ROOT/scripts/development/"
    log_info "4. View logs: $PROJECT_ROOT/scripts/development/logs.sh <service-name>"
    log_info "5. Access service shell: $PROJECT_ROOT/scripts/development/shell.sh <service-name>"
}

# Script execution
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi
