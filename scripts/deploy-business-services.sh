#!/bin/bash

# Deploy Business Services Script for 002AIC Platform
# This script deploys the 5 business services to Kubernetes

set -e

# Configuration
NAMESPACE="aic-business"
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
    
    # Check if data services are running (recommended)
    if ! kubectl get namespace aic-data &> /dev/null; then
        log_warning "Data services namespace not found. Some business services may have limited functionality."
        log_info "Consider running: ./scripts/deploy-data-services.sh"
    fi
    
    log_success "Prerequisites check completed"
}

# Create business namespace
create_namespace() {
    log_info "Creating business services namespace..."
    
    kubectl apply -f infra/k8s/business-services/namespace.yaml
    
    log_success "Business services namespace created"
}

# Update PostgreSQL with business databases
update_postgresql() {
    log_info "Updating PostgreSQL with business service databases..."
    
    # Apply business database initialization script
    kubectl apply -f infra/k8s/infrastructure/postgresql-business-init.yaml
    
    # Restart PostgreSQL to pick up new init scripts
    kubectl rollout restart statefulset/postgresql -n $PLATFORM_NAMESPACE
    kubectl rollout status statefulset/postgresql -n $PLATFORM_NAMESPACE --timeout=$KUBECTL_TIMEOUT
    
    # Wait a bit for databases to be created
    sleep 30
    
    log_success "PostgreSQL updated with business service databases"
}

# Deploy supporting infrastructure
deploy_supporting_infrastructure() {
    log_info "Deploying supporting infrastructure..."
    
    # Deploy Temporal workflow engine
    log_info "Deploying Temporal workflow engine..."
    kubectl apply -f infra/k8s/business-services/temporal.yaml
    
    # Wait for Temporal services to be ready
    kubectl wait --for=condition=available deployment/temporal-frontend -n $NAMESPACE --timeout=$KUBECTL_TIMEOUT
    kubectl wait --for=condition=available deployment/temporal-history -n $NAMESPACE --timeout=$KUBECTL_TIMEOUT
    kubectl wait --for=condition=available deployment/temporal-matching -n $NAMESPACE --timeout=$KUBECTL_TIMEOUT
    kubectl wait --for=condition=available deployment/temporal-worker -n $NAMESPACE --timeout=$KUBECTL_TIMEOUT
    
    log_success "Supporting infrastructure deployed successfully"
}

# Deploy secrets
deploy_secrets() {
    log_info "Deploying business services secrets..."
    
    # Check if secrets already exist
    if kubectl get secret business-database-secret -n $NAMESPACE &> /dev/null; then
        log_warning "Business services secrets already exist. Skipping secret creation."
        log_warning "If you need to update secrets, delete them first and re-run deployment."
    else
        kubectl apply -f infra/k8s/business-services/secrets-database.yaml
        kubectl apply -f infra/k8s/business-services/secrets-payment.yaml
        kubectl apply -f infra/k8s/business-services/secrets-integrations.yaml
        kubectl apply -f infra/k8s/business-services/secrets-marketplace.yaml
        log_success "Business services secrets deployed successfully"
    fi
}

# Deploy business services
deploy_business_services() {
    log_info "Deploying business services..."
    
    # Deploy services in dependency order
    services=("billing-service" "integration-service" "support-service" "marketplace-service" "workflow-service")
    
    for service in "${services[@]}"; do
        log_info "Deploying $service..."
        kubectl apply -f "infra/k8s/business-services/$service.yaml"
        
        # Wait for deployment to be ready
        log_info "Waiting for $service to be ready..."
        kubectl wait --for=condition=available deployment/$service -n $NAMESPACE --timeout=$KUBECTL_TIMEOUT
        
        log_success "$service deployed successfully"
    done
}

# Verify deployment
verify_deployment() {
    log_info "Verifying business services deployment..."
    
    # Check pod status
    log_info "Pod status:"
    kubectl get pods -n $NAMESPACE
    
    # Check service status
    log_info "Service status:"
    kubectl get services -n $NAMESPACE
    
    # Check if all deployments are ready
    deployments=("billing-service" "workflow-service" "integration-service" "support-service" "marketplace-service")
    temporal_services=("temporal-frontend" "temporal-history" "temporal-matching" "temporal-worker")
    
    all_ready=true
    for deployment in "${deployments[@]}"; do
        if ! kubectl get deployment $deployment -n $NAMESPACE -o jsonpath='{.status.conditions[?(@.type=="Available")].status}' | grep -q "True"; then
            log_error "$deployment is not ready"
            all_ready=false
        fi
    done
    
    for temporal_service in "${temporal_services[@]}"; do
        if ! kubectl get deployment $temporal_service -n $NAMESPACE -o jsonpath='{.status.conditions[?(@.type=="Available")].status}' | grep -q "True"; then
            log_error "$temporal_service is not ready"
            all_ready=false
        fi
    done
    
    if $all_ready; then
        log_success "All business services and infrastructure are ready!"
    else
        log_error "Some business services are not ready. Check the logs for more details."
        return 1
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
    log_info "Running business services health checks..."
    
    # Create a temporary pod for health checks
    kubectl run business-health-check-pod --image=curlimages/curl:latest --rm -i --restart=Never -- sh -c "
        echo 'Testing Billing Service health...'
        curl -f http://billing-service.$NAMESPACE.svc.cluster.local/health || exit 1
        
        echo 'Testing Workflow Service health...'
        curl -f http://workflow-service.$NAMESPACE.svc.cluster.local/health || exit 1
        
        echo 'Testing Integration Service health...'
        curl -f http://integration-service.$NAMESPACE.svc.cluster.local/health || exit 1
        
        echo 'Testing Support Service health...'
        curl -f http://support-service.$NAMESPACE.svc.cluster.local/health || exit 1
        
        echo 'Testing Marketplace Service health...'
        curl -f http://marketplace-service.$NAMESPACE.svc.cluster.local/health || exit 1
        
        echo 'All business services health checks passed!'
    "
    
    if [ $? -eq 0 ]; then
        log_success "All business services health checks passed!"
    else
        log_error "Some business services health checks failed. Check the service logs for more details."
        return 1
    fi
}

# Test infrastructure connectivity
test_infrastructure_connectivity() {
    log_info "Testing infrastructure connectivity..."
    
    # Test PostgreSQL connectivity
    log_info "Testing PostgreSQL connectivity..."
    kubectl exec -it postgresql-0 -n $PLATFORM_NAMESPACE -- psql -U postgres -c "\l" | grep -E "(billing_db|workflow_db|integration_db|support_db|marketplace_db|temporal)" && log_success "PostgreSQL databases accessible" || log_error "PostgreSQL database access failed"
    
    # Test Temporal connectivity
    log_info "Testing Temporal connectivity..."
    kubectl run temporal-test --image=curlimages/curl:latest --rm -i --restart=Never -- sh -c "
        # Test Temporal frontend gRPC health (using HTTP health check if available)
        nc -z temporal-frontend.$NAMESPACE.svc.cluster.local 7233 && echo 'Temporal frontend accessible' || exit 1
    " && log_success "Temporal accessible" || log_error "Temporal access failed"
}

# Check business capabilities
check_business_capabilities() {
    log_info "Checking business service capabilities..."
    
    # Check billing service payment integration
    log_info "Verifying billing service payment integration..."
    if kubectl get secret business-payment-secret -n $NAMESPACE &> /dev/null; then
        log_success "Billing service has payment provider credentials"
    else
        log_warning "Billing service payment credentials not found"
    fi
    
    # Check integration service connectors
    log_info "Verifying integration service connectors..."
    if kubectl get secret business-integration-secret -n $NAMESPACE &> /dev/null; then
        log_success "Integration service has third-party connector credentials"
    else
        log_warning "Integration service connector credentials not found"
    fi
    
    # Check support service AI capabilities
    log_info "Verifying support service AI capabilities..."
    if kubectl get secret business-support-secret -n $NAMESPACE &> /dev/null; then
        log_success "Support service has AI and helpdesk integration credentials"
    else
        log_warning "Support service integration credentials not found"
    fi
    
    # Check marketplace service container registry
    log_info "Verifying marketplace service capabilities..."
    if kubectl get secret business-marketplace-secret -n $NAMESPACE &> /dev/null; then
        log_success "Marketplace service has container registry and storage credentials"
    else
        log_warning "Marketplace service credentials not found"
    fi
    
    # Check workflow service temporal integration
    log_info "Verifying workflow service temporal integration..."
    if kubectl get service temporal-frontend -n $NAMESPACE &> /dev/null; then
        log_success "Workflow service can integrate with Temporal"
    else
        log_warning "Temporal service not found for workflow integration"
    fi
}

# Show useful commands
show_useful_commands() {
    log_info "Useful commands for managing business services:"
    echo ""
    echo "# View business service logs"
    echo "kubectl logs -f deployment/billing-service -n $NAMESPACE"
    echo "kubectl logs -f deployment/workflow-service -n $NAMESPACE"
    echo "kubectl logs -f deployment/marketplace-service -n $NAMESPACE"
    echo ""
    echo "# Scale business services"
    echo "kubectl scale deployment billing-service --replicas=5 -n $NAMESPACE"
    echo ""
    echo "# Port forward for local access"
    echo "kubectl port-forward service/billing-service 8080:80 -n $NAMESPACE"
    echo "kubectl port-forward service/temporal-frontend 7233:7233 -n $NAMESPACE"
    echo "kubectl port-forward service/marketplace-service 8080:80 -n $NAMESPACE"
    echo ""
    echo "# View business service status"
    echo "kubectl get all -n $NAMESPACE"
    echo ""
    echo "# Test infrastructure connectivity"
    echo "./scripts/deploy-business-services.sh test-infra"
    echo ""
    echo "# Check business capabilities"
    echo "./scripts/deploy-business-services.sh check-capabilities"
    echo ""
    echo "# View persistent volumes"
    echo "kubectl get pvc -n $NAMESPACE"
    echo ""
    echo "# Delete business services deployment"
    echo "./scripts/cleanup-business-services.sh"
}

# Main deployment function
main() {
    log_info "Starting 002AIC Business Services deployment..."
    
    check_prerequisites
    create_namespace
    update_postgresql
    deploy_secrets
    deploy_supporting_infrastructure
    deploy_business_services
    verify_deployment
    test_infrastructure_connectivity
    check_business_capabilities
    run_health_checks
    
    log_success "💼 002AIC Business Services deployment completed successfully!"
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
        deploy_business_services
        verify_deployment
        ;;
    "verify")
        verify_deployment
        test_infrastructure_connectivity
        check_business_capabilities
        run_health_checks
        ;;
    "test-infra")
        test_infrastructure_connectivity
        ;;
    "check-capabilities")
        check_business_capabilities
        ;;
    "help")
        echo "Usage: $0 [deploy|infrastructure|services|verify|test-infra|check-capabilities|help]"
        echo ""
        echo "Commands:"
        echo "  deploy            - Full business services deployment (default)"
        echo "  infrastructure    - Deploy only supporting infrastructure (Temporal)"
        echo "  services          - Deploy only the 5 business services"
        echo "  verify            - Verify deployment and run health checks"
        echo "  test-infra        - Test infrastructure connectivity"
        echo "  check-capabilities - Check business service capabilities"
        echo "  help              - Show this help message"
        ;;
    *)
        log_error "Unknown command: $1"
        echo "Use '$0 help' for usage information"
        exit 1
        ;;
esac
