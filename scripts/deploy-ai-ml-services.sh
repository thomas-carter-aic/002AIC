#!/bin/bash

# Deploy AI/ML Services Script for 002AIC Platform
# This script deploys the 6 AI/ML services to Kubernetes

set -e

# Configuration
NAMESPACE="aic-ai-ml"
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

# Create AI/ML namespace
create_namespace() {
    log_info "Creating AI/ML namespace..."
    
    kubectl apply -f infra/k8s/ai-ml-services/namespace.yaml
    
    log_success "AI/ML namespace created"
}

# Update PostgreSQL with AI/ML databases
update_postgresql() {
    log_info "Updating PostgreSQL with AI/ML databases..."
    
    # Apply AI/ML database initialization script
    kubectl apply -f infra/k8s/infrastructure/postgresql-ai-ml-init.yaml
    
    # Restart PostgreSQL to pick up new init scripts
    kubectl rollout restart statefulset/postgresql -n $PLATFORM_NAMESPACE
    kubectl rollout status statefulset/postgresql -n $PLATFORM_NAMESPACE --timeout=$KUBECTL_TIMEOUT
    
    # Wait a bit for databases to be created
    sleep 30
    
    log_success "PostgreSQL updated with AI/ML databases"
}

# Deploy secrets
deploy_secrets() {
    log_info "Deploying AI/ML secrets..."
    
    # Check if secrets already exist
    if kubectl get secret ai-ml-database-secret -n $NAMESPACE &> /dev/null; then
        log_warning "AI/ML secrets already exist. Skipping secret creation."
        log_warning "If you need to update secrets, delete them first: kubectl delete secret ai-ml-database-secret ai-ml-redis-secret mlflow-secret ai-ml-storage-secret huggingface-secret -n $NAMESPACE"
    else
        kubectl apply -f infra/k8s/ai-ml-services/secrets.yaml
        log_success "AI/ML secrets deployed successfully"
    fi
}

# Deploy MLflow
deploy_mlflow() {
    log_info "Deploying MLflow tracking server..."
    
    kubectl apply -f infra/k8s/ai-ml-services/mlflow.yaml
    
    # Wait for MLflow to be ready
    log_info "Waiting for MLflow to be ready..."
    kubectl wait --for=condition=available deployment/mlflow-service -n $NAMESPACE --timeout=$KUBECTL_TIMEOUT
    
    log_success "MLflow deployed successfully"
}

# Deploy AI/ML services
deploy_ai_ml_services() {
    log_info "Deploying AI/ML services..."
    
    # Deploy services in dependency order
    services=("agent-orchestration-service" "data-integration-service" "pipeline-execution-service" "model-training-service" "model-tuning-service" "model-deployment-service")
    
    for service in "${services[@]}"; do
        log_info "Deploying $service..."
        kubectl apply -f "infra/k8s/ai-ml-services/$service.yaml"
        
        # Wait for deployment to be ready (longer timeout for AI/ML services)
        log_info "Waiting for $service to be ready..."
        kubectl wait --for=condition=available deployment/$service -n $NAMESPACE --timeout=$KUBECTL_TIMEOUT
        
        log_success "$service deployed successfully"
    done
}

# Verify deployment
verify_deployment() {
    log_info "Verifying AI/ML deployment..."
    
    # Check pod status
    log_info "Pod status:"
    kubectl get pods -n $NAMESPACE
    
    # Check service status
    log_info "Service status:"
    kubectl get services -n $NAMESPACE
    
    # Check if all deployments are ready
    deployments=("agent-orchestration-service" "model-training-service" "model-tuning-service" "model-deployment-service" "data-integration-service" "pipeline-execution-service" "mlflow-service")
    
    all_ready=true
    for deployment in "${deployments[@]}"; do
        if ! kubectl get deployment $deployment -n $NAMESPACE -o jsonpath='{.status.conditions[?(@.type=="Available")].status}' | grep -q "True"; then
            log_error "$deployment is not ready"
            all_ready=false
        fi
    done
    
    if $all_ready; then
        log_success "All AI/ML services are ready!"
    else
        log_error "Some AI/ML services are not ready. Check the logs for more details."
        return 1
    fi
    
    # Get MLflow UI external IP
    log_info "Getting MLflow UI external IP..."
    MLFLOW_IP=$(kubectl get service mlflow-ui -n $NAMESPACE -o jsonpath='{.status.loadBalancer.ingress[0].ip}' 2>/dev/null || echo "pending")
    
    if [ "$MLFLOW_IP" != "pending" ] && [ -n "$MLFLOW_IP" ]; then
        log_success "MLflow UI is accessible at: http://$MLFLOW_IP:5000"
    else
        log_warning "MLflow UI external IP is still pending. Check with: kubectl get service mlflow-ui -n $NAMESPACE"
    fi
    
    # Check resource usage
    log_info "Resource usage:"
    kubectl top pods -n $NAMESPACE 2>/dev/null || log_warning "Metrics server not available for resource usage"
}

# Run health checks
run_health_checks() {
    log_info "Running AI/ML health checks..."
    
    # Create a temporary pod for health checks
    kubectl run ai-ml-health-check-pod --image=curlimages/curl:latest --rm -i --restart=Never -- sh -c "
        echo 'Testing Agent Orchestration Service health...'
        curl -f http://agent-orchestration-service.$NAMESPACE.svc.cluster.local/health || exit 1
        
        echo 'Testing Model Training Service health...'
        curl -f http://model-training-service.$NAMESPACE.svc.cluster.local/health || exit 1
        
        echo 'Testing Model Tuning Service health...'
        curl -f http://model-tuning-service.$NAMESPACE.svc.cluster.local/health || exit 1
        
        echo 'Testing Model Deployment Service health...'
        curl -f http://model-deployment-service.$NAMESPACE.svc.cluster.local/health || exit 1
        
        echo 'Testing Data Integration Service health...'
        curl -f http://data-integration-service.$NAMESPACE.svc.cluster.local/health || exit 1
        
        echo 'Testing Pipeline Execution Service health...'
        curl -f http://pipeline-execution-service.$NAMESPACE.svc.cluster.local/health || exit 1
        
        echo 'Testing MLflow Service health...'
        curl -f http://mlflow-service.$NAMESPACE.svc.cluster.local:5000/health || exit 1
        
        echo 'All AI/ML health checks passed!'
    "
    
    if [ $? -eq 0 ]; then
        log_success "All AI/ML health checks passed!"
    else
        log_error "Some AI/ML health checks failed. Check the service logs for more details."
        return 1
    fi
}

# Check GPU availability
check_gpu_availability() {
    log_info "Checking GPU availability..."
    
    # Check if GPU nodes are available
    gpu_nodes=$(kubectl get nodes -l accelerator=nvidia-tesla-gpu --no-headers 2>/dev/null | wc -l)
    
    if [ $gpu_nodes -gt 0 ]; then
        log_success "Found $gpu_nodes GPU-enabled nodes"
        kubectl get nodes -l accelerator=nvidia-tesla-gpu
    else
        log_warning "No GPU-enabled nodes found. AI/ML training services may not function optimally."
        log_info "Consider adding GPU nodes or updating node labels:"
        log_info "kubectl label nodes <node-name> accelerator=nvidia-tesla-gpu"
    fi
    
    # Check GPU resource availability
    gpu_capacity=$(kubectl describe nodes -l accelerator=nvidia-tesla-gpu 2>/dev/null | grep "nvidia.com/gpu" | head -1 | awk '{print $2}' || echo "0")
    if [ "$gpu_capacity" != "0" ]; then
        log_success "GPU capacity: $gpu_capacity"
    else
        log_warning "No GPU resources detected on nodes"
    fi
}

# Show useful commands
show_useful_commands() {
    log_info "Useful commands for managing AI/ML services:"
    echo ""
    echo "# View AI/ML service logs"
    echo "kubectl logs -f deployment/model-training-service -n $NAMESPACE"
    echo "kubectl logs -f deployment/mlflow-service -n $NAMESPACE"
    echo ""
    echo "# Scale AI/ML services"
    echo "kubectl scale deployment agent-orchestration-service --replicas=3 -n $NAMESPACE"
    echo ""
    echo "# Port forward for local access"
    echo "kubectl port-forward service/mlflow-service 5000:5000 -n $NAMESPACE"
    echo "kubectl port-forward service/agent-orchestration-service 8080:80 -n $NAMESPACE"
    echo ""
    echo "# View AI/ML service status"
    echo "kubectl get all -n $NAMESPACE"
    echo ""
    echo "# Check GPU usage"
    echo "kubectl describe nodes -l accelerator=nvidia-tesla-gpu"
    echo ""
    echo "# View persistent volumes"
    echo "kubectl get pvc -n $NAMESPACE"
    echo ""
    echo "# Delete AI/ML deployment"
    echo "./scripts/cleanup-ai-ml-services.sh"
}

# Main deployment function
main() {
    log_info "Starting 002AIC AI/ML Services deployment..."
    
    check_prerequisites
    create_namespace
    update_postgresql
    deploy_secrets
    deploy_mlflow
    deploy_ai_ml_services
    verify_deployment
    check_gpu_availability
    run_health_checks
    
    log_success "🤖 002AIC AI/ML Services deployment completed successfully!"
    echo ""
    show_useful_commands
}

# Handle script arguments
case "${1:-deploy}" in
    "deploy")
        main
        ;;
    "mlflow")
        check_prerequisites
        create_namespace
        deploy_secrets
        deploy_mlflow
        ;;
    "services")
        check_prerequisites
        deploy_ai_ml_services
        verify_deployment
        ;;
    "verify")
        verify_deployment
        check_gpu_availability
        run_health_checks
        ;;
    "gpu-check")
        check_gpu_availability
        ;;
    "help")
        echo "Usage: $0 [deploy|mlflow|services|verify|gpu-check|help]"
        echo ""
        echo "Commands:"
        echo "  deploy     - Full AI/ML deployment (default)"
        echo "  mlflow     - Deploy only MLflow tracking server"
        echo "  services   - Deploy only the 6 AI/ML services"
        echo "  verify     - Verify deployment and run health checks"
        echo "  gpu-check  - Check GPU availability and resources"
        echo "  help       - Show this help message"
        ;;
    *)
        log_error "Unknown command: $1"
        echo "Use '$0 help' for usage information"
        exit 1
        ;;
esac
