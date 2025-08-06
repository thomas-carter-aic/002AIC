#!/bin/bash

# Deploy Core Services Script for 002AIC Platform
# This script deploys the 5 core services to Kubernetes

set -e

# Configuration
NAMESPACE="aic-platform"
MONITORING_NAMESPACE="aic-monitoring"
KUBECTL_TIMEOUT="300s"

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
    
    # Check if docker is installed (for building images locally)
    if ! command -v docker &> /dev/null; then
        log_warning "Docker is not installed. You'll need to build images separately."
    fi
    
    log_success "Prerequisites check completed"
}

# Create namespaces
create_namespaces() {
    log_info "Creating namespaces..."
    
    kubectl apply -f infra/k8s/core-services/namespace.yaml
    
    log_success "Namespaces created"
}

# Deploy infrastructure (PostgreSQL, Redis)
deploy_infrastructure() {
    log_info "Deploying infrastructure components..."
    
    # Deploy PostgreSQL
    kubectl apply -f infra/k8s/infrastructure/postgresql.yaml
    log_info "PostgreSQL deployment initiated"
    
    # Deploy Redis
    kubectl apply -f infra/k8s/infrastructure/redis.yaml
    log_info "Redis deployment initiated"
    
    # Wait for infrastructure to be ready
    log_info "Waiting for PostgreSQL to be ready..."
    kubectl wait --for=condition=ready pod -l app=postgresql -n $NAMESPACE --timeout=$KUBECTL_TIMEOUT
    
    log_info "Waiting for Redis to be ready..."
    kubectl wait --for=condition=ready pod -l app=redis -n $NAMESPACE --timeout=$KUBECTL_TIMEOUT
    
    log_success "Infrastructure components deployed successfully"
}

# Deploy monitoring stack
deploy_monitoring() {
    log_info "Deploying monitoring stack..."
    
    # Deploy Prometheus
    kubectl apply -f infra/k8s/monitoring/prometheus.yaml
    log_info "Prometheus deployment initiated"
    
    # Deploy Grafana
    kubectl apply -f infra/k8s/monitoring/grafana.yaml
    log_info "Grafana deployment initiated"
    
    # Wait for monitoring components to be ready
    log_info "Waiting for Prometheus to be ready..."
    kubectl wait --for=condition=ready pod -l app=prometheus -n $MONITORING_NAMESPACE --timeout=$KUBECTL_TIMEOUT
    
    log_info "Waiting for Grafana to be ready..."
    kubectl wait --for=condition=ready pod -l app=grafana -n $MONITORING_NAMESPACE --timeout=$KUBECTL_TIMEOUT
    
    log_success "Monitoring stack deployed successfully"
}

# Deploy secrets
deploy_secrets() {
    log_info "Deploying secrets..."
    
    # Check if secrets already exist
    if kubectl get secret database-secret -n $NAMESPACE &> /dev/null; then
        log_warning "Secrets already exist. Skipping secret creation."
        log_warning "If you need to update secrets, delete them first: kubectl delete secret database-secret redis-secret auth-secret api-gateway-tls -n $NAMESPACE"
    else
        kubectl apply -f infra/k8s/core-services/secrets.yaml
        log_success "Secrets deployed successfully"
    fi
}

# Deploy core services
deploy_core_services() {
    log_info "Deploying core services..."
    
    # Deploy services in dependency order
    services=("configuration-service" "service-discovery" "auth-service" "health-check-service" "api-gateway-service")
    
    for service in "${services[@]}"; do
        log_info "Deploying $service..."
        kubectl apply -f "infra/k8s/core-services/$service.yaml"
        
        # Wait for deployment to be ready
        log_info "Waiting for $service to be ready..."
        kubectl wait --for=condition=available deployment/$service -n $NAMESPACE --timeout=$KUBECTL_TIMEOUT
        
        log_success "$service deployed successfully"
    done
}

# Verify deployment
verify_deployment() {
    log_info "Verifying deployment..."
    
    # Check pod status
    log_info "Pod status:"
    kubectl get pods -n $NAMESPACE
    
    # Check service status
    log_info "Service status:"
    kubectl get services -n $NAMESPACE
    
    # Check if all deployments are ready
    deployments=("auth-service" "configuration-service" "service-discovery" "health-check-service" "api-gateway-service")
    
    all_ready=true
    for deployment in "${deployments[@]}"; do
        if ! kubectl get deployment $deployment -n $NAMESPACE -o jsonpath='{.status.conditions[?(@.type=="Available")].status}' | grep -q "True"; then
            log_error "$deployment is not ready"
            all_ready=false
        fi
    done
    
    if $all_ready; then
        log_success "All core services are ready!"
    else
        log_error "Some services are not ready. Check the logs for more details."
        return 1
    fi
    
    # Get API Gateway external IP
    log_info "Getting API Gateway external IP..."
    GATEWAY_IP=$(kubectl get service api-gateway-service -n $NAMESPACE -o jsonpath='{.status.loadBalancer.ingress[0].ip}' 2>/dev/null || echo "pending")
    
    if [ "$GATEWAY_IP" != "pending" ] && [ -n "$GATEWAY_IP" ]; then
        log_success "API Gateway is accessible at: http://$GATEWAY_IP"
    else
        log_warning "API Gateway external IP is still pending. Check with: kubectl get service api-gateway-service -n $NAMESPACE"
    fi
    
    # Get Grafana external IP
    log_info "Getting Grafana external IP..."
    GRAFANA_IP=$(kubectl get service grafana-service -n $MONITORING_NAMESPACE -o jsonpath='{.status.loadBalancer.ingress[0].ip}' 2>/dev/null || echo "pending")
    
    if [ "$GRAFANA_IP" != "pending" ] && [ -n "$GRAFANA_IP" ]; then
        log_success "Grafana is accessible at: http://$GRAFANA_IP:3000"
        log_info "Default credentials: admin/admin123"
    else
        log_warning "Grafana external IP is still pending. Check with: kubectl get service grafana-service -n $MONITORING_NAMESPACE"
    fi
}

# Run health checks
run_health_checks() {
    log_info "Running health checks..."
    
    # Get API Gateway service endpoint
    GATEWAY_SERVICE="api-gateway-service.$NAMESPACE.svc.cluster.local"
    
    # Create a temporary pod for health checks
    kubectl run health-check-pod --image=curlimages/curl:latest --rm -i --restart=Never -- sh -c "
        echo 'Testing API Gateway health...'
        curl -f http://$GATEWAY_SERVICE/health || exit 1
        
        echo 'Testing Auth Service health...'
        curl -f http://auth-service.$NAMESPACE.svc.cluster.local/health || exit 1
        
        echo 'Testing Configuration Service health...'
        curl -f http://configuration-service.$NAMESPACE.svc.cluster.local/health || exit 1
        
        echo 'Testing Service Discovery health...'
        curl -f http://service-discovery.$NAMESPACE.svc.cluster.local/health || exit 1
        
        echo 'Testing Health Check Service health...'
        curl -f http://health-check-service.$NAMESPACE.svc.cluster.local/health || exit 1
        
        echo 'All health checks passed!'
    "
    
    if [ $? -eq 0 ]; then
        log_success "All health checks passed!"
    else
        log_error "Some health checks failed. Check the service logs for more details."
        return 1
    fi
}

# Show useful commands
show_useful_commands() {
    log_info "Useful commands for managing the deployment:"
    echo ""
    echo "# View pod logs"
    echo "kubectl logs -f deployment/auth-service -n $NAMESPACE"
    echo "kubectl logs -f deployment/api-gateway-service -n $NAMESPACE"
    echo ""
    echo "# Scale services"
    echo "kubectl scale deployment auth-service --replicas=5 -n $NAMESPACE"
    echo ""
    echo "# Port forward for local access"
    echo "kubectl port-forward service/api-gateway-service 8080:80 -n $NAMESPACE"
    echo "kubectl port-forward service/grafana-service 3000:3000 -n $MONITORING_NAMESPACE"
    echo ""
    echo "# View service status"
    echo "kubectl get all -n $NAMESPACE"
    echo "kubectl get all -n $MONITORING_NAMESPACE"
    echo ""
    echo "# Delete deployment"
    echo "./scripts/cleanup-core-services.sh"
}

# Main deployment function
main() {
    log_info "Starting 002AIC Core Services deployment..."
    
    check_prerequisites
    create_namespaces
    deploy_secrets
    deploy_infrastructure
    deploy_monitoring
    deploy_core_services
    verify_deployment
    run_health_checks
    
    log_success "🎉 002AIC Core Services deployment completed successfully!"
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
        create_namespaces
        deploy_secrets
        deploy_infrastructure
        ;;
    "monitoring")
        check_prerequisites
        deploy_monitoring
        ;;
    "services")
        check_prerequisites
        deploy_core_services
        verify_deployment
        ;;
    "verify")
        verify_deployment
        run_health_checks
        ;;
    "help")
        echo "Usage: $0 [deploy|infrastructure|monitoring|services|verify|help]"
        echo ""
        echo "Commands:"
        echo "  deploy         - Full deployment (default)"
        echo "  infrastructure - Deploy only PostgreSQL and Redis"
        echo "  monitoring     - Deploy only Prometheus and Grafana"
        echo "  services       - Deploy only the 5 core services"
        echo "  verify         - Verify deployment and run health checks"
        echo "  help           - Show this help message"
        ;;
    *)
        log_error "Unknown command: $1"
        echo "Use '$0 help' for usage information"
        exit 1
        ;;
esac
