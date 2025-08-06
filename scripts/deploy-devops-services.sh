#!/bin/bash

# Deploy DevOps Services Script for 002AIC Platform
# This script deploys the 5 DevOps services to Kubernetes

set -e

# Configuration
NAMESPACE="aic-devops"
PLATFORM_NAMESPACE="aic-platform"
KUBECTL_TIMEOUT="600s"

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
    
    # Check if kubectl is installed
    if ! command -v kubectl &> /dev/null; then
        log_error "kubectl is not installed. Please install kubectl first."
        exit 1
    fi
    
    # Check if kubectl can connect to cluster
    if ! kubectl cluster-info &> /dev/null; then
        log_error "Cannot connect to Kubernetes cluster. Please check your kubeconfig."
        exit 1
    fi
    
    # Check if core services are running
    if ! kubectl get namespace $PLATFORM_NAMESPACE &> /dev/null; then
        log_error "Core platform namespace not found. Please deploy core services first."
        log_info "Run: ./scripts/deploy-core-services.sh"
        exit 1
    fi
    
    # Check if PostgreSQL is running
    if ! kubectl get service postgresql-service -n $PLATFORM_NAMESPACE &> /dev/null; then
        log_error "PostgreSQL service not found. Please deploy core infrastructure first."
        exit 1
    fi
    
    log_success "Prerequisites check completed"
}

# Create DevOps namespace
create_namespace() {
    log_info "Creating DevOps services namespace..."
    
    kubectl apply -f infra/k8s/devops-services/namespace.yaml
    
    log_success "DevOps services namespace created"
}

# Update PostgreSQL with DevOps databases
update_postgresql() {
    log_info "Updating PostgreSQL with DevOps service databases..."
    
    # Apply DevOps database initialization script
    kubectl apply -f infra/k8s/infrastructure/postgresql-devops-init.yaml
    
    # Restart PostgreSQL to pick up new init scripts
    kubectl rollout restart statefulset/postgresql -n $PLATFORM_NAMESPACE
    kubectl rollout status statefulset/postgresql -n $PLATFORM_NAMESPACE --timeout=$KUBECTL_TIMEOUT
    
    # Wait a bit for databases to be created
    sleep 30
    
    log_success "PostgreSQL updated with DevOps service databases"
}

# Deploy supporting infrastructure
deploy_supporting_infrastructure() {
    log_info "Deploying supporting infrastructure..."
    
    # Deploy MinIO for object storage
    log_info "Deploying MinIO..."
    kubectl apply -f infra/k8s/devops-services/minio.yaml
    kubectl wait --for=condition=ready pod -l app=minio -n $NAMESPACE --timeout=$KUBECTL_TIMEOUT
    
    # Deploy AlertManager for monitoring
    log_info "Deploying AlertManager..."
    kubectl apply -f infra/k8s/devops-services/alertmanager.yaml
    kubectl wait --for=condition=available deployment/alertmanager -n $NAMESPACE --timeout=$KUBECTL_TIMEOUT
    
    log_success "Supporting infrastructure deployed successfully"
}

# Deploy secrets
deploy_secrets() {
    log_info "Deploying DevOps services secrets..."
    
    # Check if secrets already exist
    if kubectl get secret devops-database-secret -n $NAMESPACE &> /dev/null; then
        log_warning "DevOps services secrets already exist. Skipping secret creation."
        log_warning "If you need to update secrets, delete them first: kubectl delete secret devops-database-secret devops-redis-secret devops-registry-secret devops-storage-secret devops-monitoring-secret devops-deployment-secret devops-search-secret devops-cdn-secret -n $NAMESPACE"
    else
        kubectl apply -f infra/k8s/devops-services/secrets.yaml
        log_success "DevOps services secrets deployed successfully"
    fi
}

# Deploy DevOps services
deploy_devops_services() {
    log_info "Deploying DevOps services..."
    
    # Deploy services in dependency order
    services=("file-storage-service" "search-service" "monitoring-service" "runtime-management-service" "deployment-service")
    
    for service in "${services[@]}"; do
        log_info "Deploying $service..."
        kubectl apply -f "infra/k8s/devops-services/$service.yaml"
        
        # Wait for deployment to be ready
        log_info "Waiting for $service to be ready..."
        kubectl wait --for=condition=available deployment/$service -n $NAMESPACE --timeout=$KUBECTL_TIMEOUT
        
        log_success "$service deployed successfully"
    done
}

# Verify deployment
verify_deployment() {
    log_info "Verifying DevOps services deployment..."
    
    # Check pod status
    log_info "Pod status:"
    kubectl get pods -n $NAMESPACE
    
    # Check service status
    log_info "Service status:"
    kubectl get services -n $NAMESPACE
    
    # Check if all deployments are ready
    deployments=("runtime-management-service" "monitoring-service" "file-storage-service" "search-service" "deployment-service")
    infrastructure=("minio" "alertmanager")
    
    all_ready=true
    for deployment in "${deployments[@]}"; do
        if ! kubectl get deployment $deployment -n $NAMESPACE -o jsonpath='{.status.conditions[?(@.type=="Available")].status}' | grep -q "True"; then
            log_error "$deployment is not ready"
            all_ready=false
        fi
    done
    
    for infra in "${infrastructure[@]}"; do
        if ! kubectl get deployment $infra -n $NAMESPACE -o jsonpath='{.status.conditions[?(@.type=="Available")].status}' | grep -q "True"; then
            log_error "$infra is not ready"
            all_ready=false
        fi
    done
    
    if $all_ready; then
        log_success "All DevOps services and infrastructure are ready!"
    else
        log_error "Some DevOps services are not ready. Check the logs for more details."
        return 1
    fi
    
    # Get external service IPs
    log_info "Getting external service access points..."
    MINIO_IP=$(kubectl get service minio-console -n $NAMESPACE -o jsonpath='{.status.loadBalancer.ingress[0].ip}' 2>/dev/null || echo "pending")
    
    if [ "$MINIO_IP" != "pending" ] && [ -n "$MINIO_IP" ]; then
        log_success "MinIO Console is accessible at: http://$MINIO_IP:9001"
        log_info "Default credentials: minioadmin/minio123456"
    else
        log_warning "MinIO Console external IP is still pending. Check with: kubectl get service minio-console -n $NAMESPACE"
    fi
    
    # Check resource usage
    log_info "Resource usage:"
    kubectl top pods -n $NAMESPACE 2>/dev/null || log_warning "Metrics server not available for resource usage"
    
    # Check persistent volumes
    log_info "Persistent volumes:"
    kubectl get pvc -n $NAMESPACE
}

# Run health checks
run_health_checks() {
    log_info "Running DevOps services health checks..."
    
    # Create a temporary pod for health checks
    kubectl run devops-health-check-pod --image=curlimages/curl:latest --rm -i --restart=Never -- sh -c "
        echo 'Testing Runtime Management Service health...'
        curl -f http://runtime-management-service.$NAMESPACE.svc.cluster.local/health || exit 1
        
        echo 'Testing Monitoring Service health...'
        curl -f http://monitoring-service.$NAMESPACE.svc.cluster.local/health || exit 1
        
        echo 'Testing File Storage Service health...'
        curl -f http://file-storage-service.$NAMESPACE.svc.cluster.local/health || exit 1
        
        echo 'Testing Search Service health...'
        curl -f http://search-service.$NAMESPACE.svc.cluster.local/health || exit 1
        
        echo 'Testing Deployment Service health...'
        curl -f http://deployment-service.$NAMESPACE.svc.cluster.local/health || exit 1
        
        echo 'All DevOps services health checks passed!'
    "
    
    if [ $? -eq 0 ]; then
        log_success "All DevOps services health checks passed!"
    else
        log_error "Some DevOps services health checks failed. Check the service logs for more details."
        return 1
    fi
}

# Test infrastructure connectivity
test_infrastructure_connectivity() {
    log_info "Testing infrastructure connectivity..."
    
    # Test PostgreSQL connectivity
    log_info "Testing PostgreSQL connectivity..."
    kubectl exec -it postgresql-0 -n $PLATFORM_NAMESPACE -- psql -U postgres -c "\l" | grep -E "(runtime_db|monitoring_db|file_storage_db|search_db|deployment_db)" && log_success "PostgreSQL databases accessible" || log_error "PostgreSQL database access failed"
    
    # Test MinIO connectivity
    log_info "Testing MinIO connectivity..."
    kubectl run minio-test --image=curlimages/curl:latest --rm -i --restart=Never -- curl -f http://minio-service.$NAMESPACE.svc.cluster.local:9000/minio/health/live && log_success "MinIO accessible" || log_error "MinIO access failed"
    
    # Test AlertManager connectivity
    log_info "Testing AlertManager connectivity..."
    kubectl run alertmanager-test --image=curlimages/curl:latest --rm -i --restart=Never -- curl -f http://alertmanager-service.$NAMESPACE.svc.cluster.local:9093/-/healthy && log_success "AlertManager accessible" || log_error "AlertManager access failed"
}

# Check DevOps capabilities
check_devops_capabilities() {
    log_info "Checking DevOps capabilities..."
    
    # Check runtime management capabilities
    log_info "Verifying runtime management capabilities..."
    if kubectl get clusterrole runtime-management-service &> /dev/null; then
        log_success "Runtime management service has proper RBAC permissions"
    else
        log_warning "Runtime management service RBAC not found"
    fi
    
    # Check deployment service capabilities
    log_info "Verifying deployment service capabilities..."
    if kubectl get clusterrole deployment-service &> /dev/null; then
        log_success "Deployment service has proper RBAC permissions"
    else
        log_warning "Deployment service RBAC not found"
    fi
    
    # Check monitoring integration
    log_info "Verifying monitoring integration..."
    if kubectl get service prometheus-service -n aic-monitoring &> /dev/null; then
        log_success "Monitoring service can integrate with existing Prometheus"
    else
        log_warning "Prometheus service not found for monitoring integration"
    fi
    
    # Check file storage backends
    log_info "Verifying file storage backends..."
    if kubectl get secret devops-storage-secret -n $NAMESPACE &> /dev/null; then
        log_success "File storage service has storage backend credentials"
    else
        log_warning "File storage backend credentials not found"
    fi
}

# Show useful commands
show_useful_commands() {
    log_info "Useful commands for managing DevOps services:"
    echo ""
    echo "# View DevOps service logs"
    echo "kubectl logs -f deployment/runtime-management-service -n $NAMESPACE"
    echo "kubectl logs -f deployment/monitoring-service -n $NAMESPACE"
    echo "kubectl logs -f deployment/file-storage-service -n $NAMESPACE"
    echo ""
    echo "# Scale DevOps services"
    echo "kubectl scale deployment runtime-management-service --replicas=3 -n $NAMESPACE"
    echo ""
    echo "# Port forward for local access"
    echo "kubectl port-forward service/minio-service 9000:9000 -n $NAMESPACE"
    echo "kubectl port-forward service/minio-console 9001:9001 -n $NAMESPACE"
    echo "kubectl port-forward service/alertmanager-service 9093:9093 -n $NAMESPACE"
    echo "kubectl port-forward service/file-storage-service 8080:80 -n $NAMESPACE"
    echo ""
    echo "# View DevOps service status"
    echo "kubectl get all -n $NAMESPACE"
    echo ""
    echo "# Test infrastructure connectivity"
    echo "./scripts/deploy-devops-services.sh test-infra"
    echo ""
    echo "# Check DevOps capabilities"
    echo "./scripts/deploy-devops-services.sh check-capabilities"
    echo ""
    echo "# View persistent volumes"
    echo "kubectl get pvc -n $NAMESPACE"
    echo ""
    echo "# Delete DevOps services deployment"
    echo "./scripts/cleanup-devops-services.sh"
}

# Main deployment function
main() {
    log_info "Starting 002AIC DevOps Services deployment..."
    
    check_prerequisites
    create_namespace
    update_postgresql
    deploy_secrets
    deploy_supporting_infrastructure
    deploy_devops_services
    verify_deployment
    test_infrastructure_connectivity
    check_devops_capabilities
    run_health_checks
    
    log_success "🚀 002AIC DevOps Services deployment completed successfully!"
    echo ""
    show_useful_commands
}

# Handle script arguments
case "${1:-deploy}" in
    "deploy")
        main
        ;;
    "infrastructure")
        check_prerequisites
        create_namespace
        deploy_secrets
        deploy_supporting_infrastructure
        ;;
    "services")
        check_prerequisites
        deploy_devops_services
        verify_deployment
        ;;
    "verify")
        verify_deployment
        test_infrastructure_connectivity
        check_devops_capabilities
        run_health_checks
        ;;
    "test-infra")
        test_infrastructure_connectivity
        ;;
    "check-capabilities")
        check_devops_capabilities
        ;;
    "help")
        echo "Usage: $0 [deploy|infrastructure|services|verify|test-infra|check-capabilities|help]"
        echo ""
        echo "Commands:"
        echo "  deploy            - Full DevOps services deployment (default)"
        echo "  infrastructure    - Deploy only supporting infrastructure (MinIO, AlertManager)"
        echo "  services          - Deploy only the 5 DevOps services"
        echo "  verify            - Verify deployment and run health checks"
        echo "  test-infra        - Test infrastructure connectivity"
        echo "  check-capabilities - Check DevOps service capabilities"
        echo "  help              - Show this help message"
        ;;
    *)
        log_error "Unknown command: $1"
        echo "Use '$0 help' for usage information"
        exit 1
        ;;
esac
