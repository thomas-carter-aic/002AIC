#!/bin/bash

# Deploy Data Services Script for 002AIC Platform
# This script deploys the 3 data services to Kubernetes

set -e

# Configuration
NAMESPACE="aic-data"
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

# Create data namespace
create_namespace() {
    log_info "Creating data services namespace..."
    
    kubectl apply -f infra/k8s/data-services/namespace.yaml
    
    log_success "Data services namespace created"
}

# Update PostgreSQL with data databases
update_postgresql() {
    log_info "Updating PostgreSQL with data service databases..."
    
    # Apply data database initialization script
    kubectl apply -f infra/k8s/infrastructure/postgresql-data-init.yaml
    
    # Restart PostgreSQL to pick up new init scripts
    kubectl rollout restart statefulset/postgresql -n $PLATFORM_NAMESPACE
    kubectl rollout status statefulset/postgresql -n $PLATFORM_NAMESPACE --timeout=$KUBECTL_TIMEOUT
    
    # Wait a bit for databases to be created
    sleep 30
    
    log_success "PostgreSQL updated with data service databases"
}

# Deploy supporting databases
deploy_supporting_databases() {
    log_info "Deploying supporting databases..."
    
    # Deploy MongoDB
    log_info "Deploying MongoDB..."
    kubectl apply -f infra/k8s/data-services/mongodb.yaml
    kubectl wait --for=condition=ready pod -l app=mongodb -n $NAMESPACE --timeout=$KUBECTL_TIMEOUT
    
    # Deploy ClickHouse
    log_info "Deploying ClickHouse..."
    kubectl apply -f infra/k8s/data-services/clickhouse.yaml
    kubectl wait --for=condition=ready pod -l app=clickhouse -n $NAMESPACE --timeout=$KUBECTL_TIMEOUT
    
    # Deploy Elasticsearch
    log_info "Deploying Elasticsearch..."
    kubectl apply -f infra/k8s/data-services/elasticsearch.yaml
    kubectl wait --for=condition=ready pod -l app=elasticsearch -n $NAMESPACE --timeout=$KUBECTL_TIMEOUT
    
    log_success "Supporting databases deployed successfully"
}

# Deploy secrets
deploy_secrets() {
    log_info "Deploying data services secrets..."
    
    # Check if secrets already exist
    if kubectl get secret data-database-secret -n $NAMESPACE &> /dev/null; then
        log_warning "Data services secrets already exist. Skipping secret creation."
        log_warning "If you need to update secrets, delete them first: kubectl delete secret data-database-secret data-redis-secret user-management-secret data-storage-secret email-service-secret analytics-secret -n $NAMESPACE"
    else
        kubectl apply -f infra/k8s/data-services/secrets.yaml
        log_success "Data services secrets deployed successfully"
    fi
}

# Deploy data services
deploy_data_services() {
    log_info "Deploying data services..."
    
    # Deploy services in dependency order
    services=("user-management-service" "data-management-service" "analytics-service")
    
    for service in "${services[@]}"; do
        log_info "Deploying $service..."
        kubectl apply -f "infra/k8s/data-services/$service.yaml"
        
        # Wait for deployment to be ready
        log_info "Waiting for $service to be ready..."
        kubectl wait --for=condition=available deployment/$service -n $NAMESPACE --timeout=$KUBECTL_TIMEOUT
        
        log_success "$service deployed successfully"
    done
}

# Verify deployment
verify_deployment() {
    log_info "Verifying data services deployment..."
    
    # Check pod status
    log_info "Pod status:"
    kubectl get pods -n $NAMESPACE
    
    # Check service status
    log_info "Service status:"
    kubectl get services -n $NAMESPACE
    
    # Check if all deployments are ready
    deployments=("user-management-service" "data-management-service" "analytics-service")
    databases=("mongodb" "clickhouse" "elasticsearch")
    
    all_ready=true
    for deployment in "${deployments[@]}"; do
        if ! kubectl get deployment $deployment -n $NAMESPACE -o jsonpath='{.status.conditions[?(@.type=="Available")].status}' | grep -q "True"; then
            log_error "$deployment is not ready"
            all_ready=false
        fi
    done
    
    for database in "${databases[@]}"; do
        if ! kubectl get statefulset $database -n $NAMESPACE -o jsonpath='{.status.readyReplicas}' | grep -q "1"; then
            log_error "$database is not ready"
            all_ready=false
        fi
    done
    
    if $all_ready; then
        log_success "All data services and databases are ready!"
    else
        log_error "Some data services are not ready. Check the logs for more details."
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
    log_info "Running data services health checks..."
    
    # Create a temporary pod for health checks
    kubectl run data-health-check-pod --image=curlimages/curl:latest --rm -i --restart=Never -- sh -c "
        echo 'Testing User Management Service health...'
        curl -f http://user-management-service.$NAMESPACE.svc.cluster.local/health || exit 1
        
        echo 'Testing Data Management Service health...'
        curl -f http://data-management-service.$NAMESPACE.svc.cluster.local/health || exit 1
        
        echo 'Testing Analytics Service health...'
        curl -f http://analytics-service.$NAMESPACE.svc.cluster.local/health || exit 1
        
        echo 'All data services health checks passed!'
    "
    
    if [ $? -eq 0 ]; then
        log_success "All data services health checks passed!"
    else
        log_error "Some data services health checks failed. Check the service logs for more details."
        return 1
    fi
}

# Test database connectivity
test_database_connectivity() {
    log_info "Testing database connectivity..."
    
    # Test PostgreSQL connectivity
    log_info "Testing PostgreSQL connectivity..."
    kubectl exec -it postgresql-0 -n $PLATFORM_NAMESPACE -- psql -U postgres -c "\l" | grep -E "(user_management_db|data_management_db|analytics_db)" && log_success "PostgreSQL databases accessible" || log_error "PostgreSQL database access failed"
    
    # Test MongoDB connectivity
    log_info "Testing MongoDB connectivity..."
    kubectl exec -it mongodb-0 -n $NAMESPACE -- mongosh --eval "db.adminCommand('listDatabases')" | grep -q "user_management_db" && log_success "MongoDB accessible" || log_error "MongoDB access failed"
    
    # Test ClickHouse connectivity
    log_info "Testing ClickHouse connectivity..."
    kubectl exec -it clickhouse-0 -n $NAMESPACE -- clickhouse-client --query "SHOW DATABASES" | grep -q "analytics" && log_success "ClickHouse accessible" || log_error "ClickHouse access failed"
    
    # Test Elasticsearch connectivity
    log_info "Testing Elasticsearch connectivity..."
    kubectl run elasticsearch-test --image=curlimages/curl:latest --rm -i --restart=Never -- curl -f http://elasticsearch-service.$NAMESPACE.svc.cluster.local:9200/_cluster/health && log_success "Elasticsearch accessible" || log_error "Elasticsearch access failed"
}

# Check data compliance
check_data_compliance() {
    log_info "Checking data compliance features..."
    
    # Check GDPR compliance features
    log_info "Verifying GDPR compliance features..."
    
    # Check if user management service has encryption enabled
    if kubectl get configmap user-management-service-config -n $NAMESPACE -o yaml | grep -q "gdpr_compliance: true"; then
        log_success "GDPR compliance enabled in user management service"
    else
        log_warning "GDPR compliance not explicitly enabled"
    fi
    
    # Check if data management service has encryption at rest
    if kubectl get configmap data-management-service-config -n $NAMESPACE -o yaml | grep -q "encryption_at_rest: true"; then
        log_success "Data encryption at rest enabled"
    else
        log_warning "Data encryption at rest not explicitly enabled"
    fi
    
    # Check audit logging
    if kubectl get configmap analytics-service-config -n $NAMESPACE -o yaml | grep -q "audit_queries: true"; then
        log_success "Audit logging enabled in analytics service"
    else
        log_warning "Audit logging not explicitly enabled"
    fi
}

# Show useful commands
show_useful_commands() {
    log_info "Useful commands for managing data services:"
    echo ""
    echo "# View data service logs"
    echo "kubectl logs -f deployment/user-management-service -n $NAMESPACE"
    echo "kubectl logs -f deployment/analytics-service -n $NAMESPACE"
    echo ""
    echo "# Scale data services"
    echo "kubectl scale deployment user-management-service --replicas=5 -n $NAMESPACE"
    echo ""
    echo "# Port forward for local access"
    echo "kubectl port-forward service/user-management-service 8080:80 -n $NAMESPACE"
    echo "kubectl port-forward service/mongodb-service 27017:27017 -n $NAMESPACE"
    echo "kubectl port-forward service/clickhouse-service 8123:8123 -n $NAMESPACE"
    echo "kubectl port-forward service/elasticsearch-service 9200:9200 -n $NAMESPACE"
    echo ""
    echo "# View data service status"
    echo "kubectl get all -n $NAMESPACE"
    echo ""
    echo "# Check database connectivity"
    echo "./scripts/deploy-data-services.sh test-db"
    echo ""
    echo "# View persistent volumes"
    echo "kubectl get pvc -n $NAMESPACE"
    echo ""
    echo "# Delete data services deployment"
    echo "./scripts/cleanup-data-services.sh"
}

# Main deployment function
main() {
    log_info "Starting 002AIC Data Services deployment..."
    
    check_prerequisites
    create_namespace
    update_postgresql
    deploy_secrets
    deploy_supporting_databases
    deploy_data_services
    verify_deployment
    test_database_connectivity
    check_data_compliance
    run_health_checks
    
    log_success "📊 002AIC Data Services deployment completed successfully!"
    echo ""
    show_useful_commands
}

# Handle script arguments
case "${1:-deploy}" in
    "deploy")
        main
        ;;
    "databases")
        check_prerequisites
        create_namespace
        deploy_secrets
        deploy_supporting_databases
        ;;
    "services")
        check_prerequisites
        deploy_data_services
        verify_deployment
        ;;
    "verify")
        verify_deployment
        test_database_connectivity
        check_data_compliance
        run_health_checks
        ;;
    "test-db")
        test_database_connectivity
        ;;
    "compliance")
        check_data_compliance
        ;;
    "help")
        echo "Usage: $0 [deploy|databases|services|verify|test-db|compliance|help]"
        echo ""
        echo "Commands:"
        echo "  deploy     - Full data services deployment (default)"
        echo "  databases  - Deploy only supporting databases (MongoDB, ClickHouse, Elasticsearch)"
        echo "  services   - Deploy only the 3 data services"
        echo "  verify     - Verify deployment and run health checks"
        echo "  test-db    - Test database connectivity"
        echo "  compliance - Check data compliance features"
        echo "  help       - Show this help message"
        ;;
    *)
        log_error "Unknown command: $1"
        echo "Use '$0 help' for usage information"
        exit 1
        ;;
esac
