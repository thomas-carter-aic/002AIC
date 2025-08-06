#!/bin/bash

# Cleanup Core Services Script for 002AIC Platform
# This script removes all core services and infrastructure from Kubernetes

set -e

# Configuration
NAMESPACE="aic-platform"
MONITORING_NAMESPACE="aic-monitoring"

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

# Confirm cleanup
confirm_cleanup() {
    echo -e "${RED}⚠️  WARNING: This will delete all 002AIC core services and data!${NC}"
    echo ""
    echo "This will remove:"
    echo "- All 5 core services (auth, config, discovery, health-check, api-gateway)"
    echo "- PostgreSQL database and all data"
    echo "- Redis cache and all data"
    echo "- Prometheus monitoring data"
    echo "- Grafana dashboards and data"
    echo "- All persistent volumes and data"
    echo ""
    read -p "Are you sure you want to continue? (type 'yes' to confirm): " confirmation
    
    if [ "$confirmation" != "yes" ]; then
        log_info "Cleanup cancelled."
        exit 0
    fi
}

# Delete core services
delete_core_services() {
    log_info "Deleting core services..."
    
    services=("api-gateway-service" "health-check-service" "auth-service" "service-discovery" "configuration-service")
    
    for service in "${services[@]}"; do
        if kubectl get deployment $service -n $NAMESPACE &> /dev/null; then
            log_info "Deleting $service..."
            kubectl delete -f "infra/k8s/core-services/$service.yaml" --ignore-not-found=true
        else
            log_warning "$service not found, skipping..."
        fi
    done
    
    log_success "Core services deleted"
}

# Delete infrastructure
delete_infrastructure() {
    log_info "Deleting infrastructure components..."
    
    # Delete PostgreSQL
    if kubectl get statefulset postgresql -n $NAMESPACE &> /dev/null; then
        log_info "Deleting PostgreSQL..."
        kubectl delete -f infra/k8s/infrastructure/postgresql.yaml --ignore-not-found=true
    fi
    
    # Delete Redis
    if kubectl get deployment redis -n $NAMESPACE &> /dev/null; then
        log_info "Deleting Redis..."
        kubectl delete -f infra/k8s/infrastructure/redis.yaml --ignore-not-found=true
    fi
    
    log_success "Infrastructure components deleted"
}

# Delete monitoring
delete_monitoring() {
    log_info "Deleting monitoring stack..."
    
    # Delete Prometheus
    if kubectl get deployment prometheus -n $MONITORING_NAMESPACE &> /dev/null; then
        log_info "Deleting Prometheus..."
        kubectl delete -f infra/k8s/monitoring/prometheus.yaml --ignore-not-found=true
    fi
    
    # Delete Grafana
    if kubectl get deployment grafana -n $MONITORING_NAMESPACE &> /dev/null; then
        log_info "Deleting Grafana..."
        kubectl delete -f infra/k8s/monitoring/grafana.yaml --ignore-not-found=true
    fi
    
    log_success "Monitoring stack deleted"
}

# Delete secrets
delete_secrets() {
    log_info "Deleting secrets..."
    
    kubectl delete -f infra/k8s/core-services/secrets.yaml --ignore-not-found=true
    
    log_success "Secrets deleted"
}

# Delete persistent volumes
delete_persistent_volumes() {
    log_info "Deleting persistent volumes..."
    
    # Delete PVCs in platform namespace
    kubectl delete pvc --all -n $NAMESPACE --ignore-not-found=true
    
    # Delete PVCs in monitoring namespace
    kubectl delete pvc --all -n $MONITORING_NAMESPACE --ignore-not-found=true
    
    log_warning "Note: Persistent Volumes may still exist depending on your storage class reclaim policy"
    log_info "Check with: kubectl get pv"
    
    log_success "Persistent volume claims deleted"
}

# Delete namespaces
delete_namespaces() {
    log_info "Deleting namespaces..."
    
    # Delete platform namespace
    if kubectl get namespace $NAMESPACE &> /dev/null; then
        log_info "Deleting namespace $NAMESPACE..."
        kubectl delete namespace $NAMESPACE --ignore-not-found=true
    fi
    
    # Delete monitoring namespace
    if kubectl get namespace $MONITORING_NAMESPACE &> /dev/null; then
        log_info "Deleting namespace $MONITORING_NAMESPACE..."
        kubectl delete namespace $MONITORING_NAMESPACE --ignore-not-found=true
    fi
    
    # Delete system namespace
    if kubectl get namespace aic-system &> /dev/null; then
        log_info "Deleting namespace aic-system..."
        kubectl delete namespace aic-system --ignore-not-found=true
    fi
    
    log_success "Namespaces deleted"
}

# Verify cleanup
verify_cleanup() {
    log_info "Verifying cleanup..."
    
    # Check if any resources remain
    remaining_pods=$(kubectl get pods -n $NAMESPACE 2>/dev/null | wc -l)
    remaining_monitoring_pods=$(kubectl get pods -n $MONITORING_NAMESPACE 2>/dev/null | wc -l)
    
    if [ $remaining_pods -eq 0 ] && [ $remaining_monitoring_pods -eq 0 ]; then
        log_success "All resources have been cleaned up successfully!"
    else
        log_warning "Some resources may still be terminating. Check with:"
        echo "kubectl get all -n $NAMESPACE"
        echo "kubectl get all -n $MONITORING_NAMESPACE"
    fi
    
    # Check for any remaining PVs
    remaining_pvs=$(kubectl get pv 2>/dev/null | grep -E "(aic-platform|aic-monitoring)" | wc -l)
    if [ $remaining_pvs -gt 0 ]; then
        log_warning "Some persistent volumes may still exist:"
        kubectl get pv | grep -E "(aic-platform|aic-monitoring)" || true
        log_info "You may need to manually delete them if they have a 'Retain' reclaim policy"
    fi
}

# Main cleanup function
main() {
    log_info "Starting 002AIC Core Services cleanup..."
    
    confirm_cleanup
    
    delete_core_services
    delete_infrastructure
    delete_monitoring
    delete_secrets
    delete_persistent_volumes
    delete_namespaces
    verify_cleanup
    
    log_success "🧹 002AIC Core Services cleanup completed!"
    echo ""
    log_info "To redeploy, run: ./scripts/deploy-core-services.sh"
}

# Handle script arguments
case "${1:-cleanup}" in
    "cleanup")
        main
        ;;
    "services-only")
        log_info "Deleting only core services (keeping infrastructure and monitoring)..."
        delete_core_services
        ;;
    "infrastructure-only")
        log_info "Deleting only infrastructure components..."
        delete_infrastructure
        delete_persistent_volumes
        ;;
    "monitoring-only")
        log_info "Deleting only monitoring stack..."
        delete_monitoring
        ;;
    "force")
        log_warning "Force cleanup without confirmation..."
        delete_core_services
        delete_infrastructure
        delete_monitoring
        delete_secrets
        delete_persistent_volumes
        delete_namespaces
        verify_cleanup
        ;;
    "help")
        echo "Usage: $0 [cleanup|services-only|infrastructure-only|monitoring-only|force|help]"
        echo ""
        echo "Commands:"
        echo "  cleanup              - Full cleanup with confirmation (default)"
        echo "  services-only        - Delete only the 5 core services"
        echo "  infrastructure-only  - Delete only PostgreSQL and Redis"
        echo "  monitoring-only      - Delete only Prometheus and Grafana"
        echo "  force                - Force cleanup without confirmation"
        echo "  help                 - Show this help message"
        ;;
    *)
        log_error "Unknown command: $1"
        echo "Use '$0 help' for usage information"
        exit 1
        ;;
esac
