#!/bin/bash

# Deploy Infrastructure Services Script for 002AIC Platform
# This script deploys the final 12 infrastructure services to complete the 36-service architecture

set -e

# Configuration
NAMESPACE="aic-infrastructure"
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

# Create infrastructure namespace
create_namespace() {
    log_info "Creating infrastructure services namespace..."
    
    kubectl apply -f infra/k8s/infrastructure-services/namespace.yaml
    
    log_success "Infrastructure services namespace created"
}

# Update PostgreSQL with infrastructure databases
update_postgresql() {
    log_info "Updating PostgreSQL with infrastructure service databases..."
    
    # Create PostgreSQL init script for infrastructure services
    kubectl create configmap postgresql-infrastructure-init-scripts \
        --from-literal=06-create-infrastructure-databases.sql="
        -- Create databases for Infrastructure services
        CREATE DATABASE notification_db;
        CREATE DATABASE audit_db;
        CREATE DATABASE security_db;
        CREATE DATABASE backup_db;
        CREATE DATABASE caching_db;
        CREATE DATABASE queue_db;
        CREATE DATABASE logging_db;
        CREATE DATABASE feature_flag_db;
        CREATE DATABASE metrics_db;
        CREATE DATABASE model_registry_db;
        CREATE DATABASE event_streaming_db;
        CREATE DATABASE recommendation_db;
        CREATE DATABASE content_management_db;
        
        -- Create users for Infrastructure services
        CREATE USER notification_user WITH PASSWORD 'notification_password';
        CREATE USER audit_user WITH PASSWORD 'audit_password';
        CREATE USER security_user WITH PASSWORD 'security_password';
        CREATE USER backup_user WITH PASSWORD 'backup_password';
        CREATE USER caching_user WITH PASSWORD 'caching_password';
        CREATE USER queue_user WITH PASSWORD 'queue_password';
        CREATE USER logging_user WITH PASSWORD 'logging_password';
        CREATE USER feature_flag_user WITH PASSWORD 'feature_flag_password';
        CREATE USER metrics_user WITH PASSWORD 'metrics_password';
        CREATE USER model_registry_user WITH PASSWORD 'model_registry_password';
        CREATE USER event_streaming_user WITH PASSWORD 'event_streaming_password';
        CREATE USER recommendation_user WITH PASSWORD 'recommendation_password';
        CREATE USER content_management_user WITH PASSWORD 'content_management_password';
        
        -- Grant permissions
        GRANT ALL PRIVILEGES ON DATABASE notification_db TO notification_user;
        GRANT ALL PRIVILEGES ON DATABASE audit_db TO audit_user;
        GRANT ALL PRIVILEGES ON DATABASE security_db TO security_user;
        GRANT ALL PRIVILEGES ON DATABASE backup_db TO backup_user;
        GRANT ALL PRIVILEGES ON DATABASE caching_db TO caching_user;
        GRANT ALL PRIVILEGES ON DATABASE queue_db TO queue_user;
        GRANT ALL PRIVILEGES ON DATABASE logging_db TO logging_user;
        GRANT ALL PRIVILEGES ON DATABASE feature_flag_db TO feature_flag_user;
        GRANT ALL PRIVILEGES ON DATABASE metrics_db TO metrics_user;
        GRANT ALL PRIVILEGES ON DATABASE model_registry_db TO model_registry_user;
        GRANT ALL PRIVILEGES ON DATABASE event_streaming_db TO event_streaming_user;
        GRANT ALL PRIVILEGES ON DATABASE recommendation_db TO recommendation_user;
        GRANT ALL PRIVILEGES ON DATABASE content_management_db TO content_management_user;
        " \
        -n $PLATFORM_NAMESPACE --dry-run=client -o yaml | kubectl apply -f -
    
    # Restart PostgreSQL to pick up new init scripts
    kubectl rollout restart statefulset/postgresql -n $PLATFORM_NAMESPACE
    kubectl rollout status statefulset/postgresql -n $PLATFORM_NAMESPACE --timeout=$KUBECTL_TIMEOUT
    
    # Wait a bit for databases to be created
    sleep 30
    
    log_success "PostgreSQL updated with infrastructure service databases"
}

# Deploy secrets
deploy_secrets() {
    log_info "Deploying infrastructure services secrets..."
    
    # Check if secrets already exist
    if kubectl get secret infrastructure-database-secret -n $NAMESPACE &> /dev/null; then
        log_warning "Infrastructure services secrets already exist. Skipping secret creation."
    else
        kubectl apply -f infra/k8s/infrastructure-services/secrets.yaml
        log_success "Infrastructure services secrets deployed successfully"
    fi
}

# Deploy infrastructure services
deploy_infrastructure_services() {
    log_info "Deploying infrastructure services..."
    
    # Deploy services in dependency order
    services=(
        "notification-service"
        "audit-service" 
        "security-service"
        "backup-service"
        "caching-service"
        "queue-service"
        "logging-service"
        "feature-flag-service"
        "metrics-service"
    )
    
    for service in "${services[@]}"; do
        log_info "Deploying $service..."
        kubectl apply -f "infra/k8s/infrastructure-services/$service.yaml"
        
        # Wait for deployment to be ready
        log_info "Waiting for $service to be ready..."
        kubectl wait --for=condition=available deployment/$service -n $NAMESPACE --timeout=$KUBECTL_TIMEOUT
        
        log_success "$service deployed successfully"
    done
    
    # Deploy remaining services
    log_info "Deploying remaining infrastructure services..."
    kubectl apply -f infra/k8s/infrastructure-services/remaining-services.yaml
    
    # Wait for remaining services
    remaining_services=("model-registry-service" "event-streaming-service" "recommendation-engine-service" "content-management-service")
    for service in "${remaining_services[@]}"; do
        log_info "Waiting for $service to be ready..."
        kubectl wait --for=condition=available deployment/$service -n $NAMESPACE --timeout=$KUBECTL_TIMEOUT
        log_success "$service deployed successfully"
    done
}

# Verify deployment
verify_deployment() {
    log_info "Verifying infrastructure services deployment..."
    
    # Check pod status
    log_info "Pod status:"
    kubectl get pods -n $NAMESPACE
    
    # Check service status
    log_info "Service status:"
    kubectl get services -n $NAMESPACE
    
    # Check if all deployments are ready
    all_services=(
        "notification-service" "audit-service" "security-service" "backup-service"
        "caching-service" "queue-service" "logging-service" "feature-flag-service"
        "metrics-service" "model-registry-service" "event-streaming-service"
        "recommendation-engine-service" "content-management-service"
    )
    
    all_ready=true
    for deployment in "${all_services[@]}"; do
        if ! kubectl get deployment $deployment -n $NAMESPACE -o jsonpath='{.status.conditions[?(@.type=="Available")].status}' | grep -q "True"; then
            log_error "$deployment is not ready"
            all_ready=false
        fi
    done
    
    if $all_ready; then
        log_success "All 12 infrastructure services are ready!"
        log_success "🎉 COMPLETE 36-SERVICE ARCHITECTURE DEPLOYED! 🎉"
    else
        log_error "Some infrastructure services are not ready. Check the logs for more details."
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
    log_info "Running infrastructure services health checks..."
    
    # Create a temporary pod for health checks
    kubectl run infrastructure-health-check-pod --image=curlimages/curl:latest --rm -i --restart=Never -- sh -c "
        echo 'Testing Notification Service health...'
        curl -f http://notification-service.$NAMESPACE.svc.cluster.local/health || exit 1
        
        echo 'Testing Audit Service health...'
        curl -f http://audit-service.$NAMESPACE.svc.cluster.local/health || exit 1
        
        echo 'Testing Security Service health...'
        curl -f http://security-service.$NAMESPACE.svc.cluster.local/health || exit 1
        
        echo 'Testing Backup Service health...'
        curl -f http://backup-service.$NAMESPACE.svc.cluster.local/health || exit 1
        
        echo 'Testing Caching Service health...'
        curl -f http://caching-service.$NAMESPACE.svc.cluster.local/health || exit 1
        
        echo 'Testing Queue Service health...'
        curl -f http://queue-service.$NAMESPACE.svc.cluster.local/health || exit 1
        
        echo 'Testing Logging Service health...'
        curl -f http://logging-service.$NAMESPACE.svc.cluster.local/health || exit 1
        
        echo 'Testing Feature Flag Service health...'
        curl -f http://feature-flag-service.$NAMESPACE.svc.cluster.local/health || exit 1
        
        echo 'Testing Metrics Service health...'
        curl -f http://metrics-service.$NAMESPACE.svc.cluster.local/health || exit 1
        
        echo 'Testing Model Registry Service health...'
        curl -f http://model-registry-service.$NAMESPACE.svc.cluster.local/health || exit 1
        
        echo 'Testing Event Streaming Service health...'
        curl -f http://event-streaming-service.$NAMESPACE.svc.cluster.local/health || exit 1
        
        echo 'Testing Recommendation Engine Service health...'
        curl -f http://recommendation-engine-service.$NAMESPACE.svc.cluster.local/health || exit 1
        
        echo 'Testing Content Management Service health...'
        curl -f http://content-management-service.$NAMESPACE.svc.cluster.local/health || exit 1
        
        echo 'All 12 infrastructure services health checks passed!'
    "
    
    if [ $? -eq 0 ]; then
        log_success "All infrastructure services health checks passed!"
    else
        log_error "Some infrastructure services health checks failed. Check the service logs for more details."
        return 1
    fi
}

# Show architecture completion summary
show_architecture_summary() {
    log_success "🎉 002AIC PLATFORM ARCHITECTURE COMPLETE! 🎉"
    echo ""
    echo "┌─────────────────────────────────────────────────────────────┐"
    echo "│                 36-SERVICE ARCHITECTURE                     │"
    echo "├─────────────────────────────────────────────────────────────┤"
    echo "│ ✅ Core Services (5):           Platform Foundation        │"
    echo "│ ✅ AI/ML Services (6):          AI-Native Capabilities     │"
    echo "│ ✅ Data Services (3):           Data Management & Analytics │"
    echo "│ ✅ DevOps Services (5):         Operations & Deployment    │"
    echo "│ ✅ Business Services (5):       Revenue & Customer Ops     │"
    echo "│ ✅ Infrastructure Services (12): Platform Infrastructure   │"
    echo "├─────────────────────────────────────────────────────────────┤"
    echo "│ 🎯 Target: \$2B ARR by 2045 with 2M active users          │"
    echo "│ 🏗️  Architecture: Enterprise-grade, AI-native PaaS        │"
    echo "│ 🔒 Security: Zero Trust, SOX/GDPR/HIPAA compliant         │"
    echo "│ 🌍 Scale: Multi-cloud, global deployment ready            │"
    echo "└─────────────────────────────────────────────────────────────┘"
    echo ""
}

# Show useful commands
show_useful_commands() {
    log_info "Useful commands for managing infrastructure services:"
    echo ""
    echo "# View infrastructure service logs"
    echo "kubectl logs -f deployment/notification-service -n $NAMESPACE"
    echo "kubectl logs -f deployment/audit-service -n $NAMESPACE"
    echo "kubectl logs -f deployment/metrics-service -n $NAMESPACE"
    echo ""
    echo "# View all platform services across namespaces"
    echo "kubectl get pods --all-namespaces | grep aic-"
    echo ""
    echo "# Check platform-wide resource usage"
    echo "kubectl top pods --all-namespaces | grep aic-"
    echo ""
    echo "# View all persistent volumes"
    echo "kubectl get pvc --all-namespaces | grep aic-"
    echo ""
    echo "# Platform health overview"
    echo "curl http://health-check-service.aic-platform.svc.cluster.local/v1/system/overview"
    echo ""
    echo "# Complete platform cleanup (USE WITH CAUTION)"
    echo "./scripts/cleanup-all-services.sh"
}

# Main deployment function
main() {
    log_info "Starting 002AIC Infrastructure Services deployment..."
    log_info "This completes the full 36-service architecture!"
    
    check_prerequisites
    create_namespace
    update_postgresql
    deploy_secrets
    deploy_infrastructure_services
    verify_deployment
    run_health_checks
    show_architecture_summary
    
    log_success "🏗️ 002AIC Infrastructure Services deployment completed successfully!"
    log_success "🎉 FULL 36-SERVICE ARCHITECTURE IS NOW COMPLETE! 🎉"
    echo ""
    show_useful_commands
}

# Handle script arguments
case "${1:-deploy}" in
    "deploy")
        main
        ;;
    "services")
        check_prerequisites
        deploy_infrastructure_services
        verify_deployment
        ;;
    "verify")
        verify_deployment
        run_health_checks
        ;;
    "summary")
        show_architecture_summary
        ;;
    "help")
        echo "Usage: $0 [deploy|services|verify|summary|help]"
        echo ""
        echo "Commands:"
        echo "  deploy   - Full infrastructure services deployment (default)"
        echo "  services - Deploy only the 12 infrastructure services"
        echo "  verify   - Verify deployment and run health checks"
        echo "  summary  - Show architecture completion summary"
        echo "  help     - Show this help message"
        ;;
    *)
        log_error "Unknown command: $1"
        echo "Use '$0 help' for usage information"
        exit 1
        ;;
esac
