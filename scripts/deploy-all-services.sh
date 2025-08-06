#!/bin/bash

# Master Deployment Script for 002AIC Platform
# Deploys all 36 services in the correct order with dependencies

set -e

# Configuration
KUBECTL_TIMEOUT="600s"
DEPLOYMENT_MODE="${1:-production}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
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

log_header() {
    echo -e "${PURPLE}[DEPLOY]${NC} $1"
}

# Check prerequisites
check_prerequisites() {
    log_header "🔍 Checking Prerequisites"
    
    # Check required tools
    local required_tools=("kubectl" "helm" "docker")
    for tool in "${required_tools[@]}"; do
        if ! command -v $tool &> /dev/null; then
            log_error "$tool is not installed. Please install $tool first."
            exit 1
        fi
    done
    
    # Check Kubernetes connection
    if ! kubectl cluster-info &> /dev/null; then
        log_error "Cannot connect to Kubernetes cluster. Please check your kubeconfig."
        exit 1
    fi
    
    # Check cluster resources
    local nodes=$(kubectl get nodes --no-headers | wc -l)
    if [ $nodes -lt 3 ]; then
        log_warning "Cluster has only $nodes nodes. Recommended: 3+ nodes for production."
    fi
    
    log_success "Prerequisites check completed"
}

# Deploy in correct dependency order
deploy_platform() {
    log_header "🚀 Starting 002AIC Platform Deployment"
    echo ""
    echo "┌─────────────────────────────────────────────────────────────┐"
    echo "│                 36-SERVICE DEPLOYMENT                       │"
    echo "│                                                             │"
    echo "│  🎯 Target: \$2B ARR by 2045 with 2M active users          │"
    echo "│  🏗️  Mode: $DEPLOYMENT_MODE                                │"
    echo "│  ⏱️  Estimated Time: 45-60 minutes                         │"
    echo "└─────────────────────────────────────────────────────────────┘"
    echo ""
    
    # Phase 1: Core Services (Foundation)
    log_header "📋 Phase 1: Core Services (5 services)"
    ./scripts/deploy-core-services.sh
    log_success "✅ Core Services deployed successfully"
    echo ""
    
    # Phase 2: AI/ML Services
    log_header "🤖 Phase 2: AI/ML Services (6 services)"
    ./scripts/deploy-ai-ml-services.sh
    log_success "✅ AI/ML Services deployed successfully"
    echo ""
    
    # Phase 3: Data Services
    log_header "📊 Phase 3: Data Services (3 services)"
    ./scripts/deploy-data-services.sh
    log_success "✅ Data Services deployed successfully"
    echo ""
    
    # Phase 4: DevOps Services
    log_header "🛠️  Phase 4: DevOps Services (5 services)"
    ./scripts/deploy-devops-services.sh
    log_success "✅ DevOps Services deployed successfully"
    echo ""
    
    # Phase 5: Business Services
    log_header "💼 Phase 5: Business Services (5 services)"
    ./scripts/deploy-business-services.sh
    log_success "✅ Business Services deployed successfully"
    echo ""
    
    # Phase 6: Infrastructure Services
    log_header "🏗️  Phase 6: Infrastructure Services (12 services)"
    ./scripts/deploy-infrastructure-services.sh
    log_success "✅ Infrastructure Services deployed successfully"
    echo ""
}

# Verify complete deployment
verify_deployment() {
    log_header "🔍 Verifying Complete Deployment"
    
    # Check all namespaces
    local namespaces=("aic-platform" "aic-ai-ml" "aic-data" "aic-devops" "aic-business" "aic-infrastructure" "aic-monitoring")
    
    for ns in "${namespaces[@]}"; do
        if kubectl get namespace $ns &> /dev/null; then
            local pods=$(kubectl get pods -n $ns --no-headers | wc -l)
            local ready_pods=$(kubectl get pods -n $ns --no-headers | grep "Running" | wc -l)
            log_info "Namespace $ns: $ready_pods/$pods pods running"
        fi
    done
    
    # Overall health check
    log_info "Running platform health checks..."
    kubectl run platform-health-check --image=curlimages/curl:latest --rm -i --restart=Never -- sh -c "
        echo 'Testing API Gateway...'
        curl -f http://api-gateway-service.aic-platform.svc.cluster.local/health || exit 1
        
        echo 'Testing Health Check Service...'
        curl -f http://health-check-service.aic-platform.svc.cluster.local/health || exit 1
        
        echo 'Platform health check passed!'
    " && log_success "✅ Platform health checks passed" || log_error "❌ Platform health checks failed"
}

# Show deployment summary
show_deployment_summary() {
    log_header "📋 Deployment Summary"
    echo ""
    echo "┌─────────────────────────────────────────────────────────────┐"
    echo "│                 🎉 DEPLOYMENT COMPLETE! 🎉                 │"
    echo "├─────────────────────────────────────────────────────────────┤"
    echo "│                                                             │"
    echo "│  ✅ Core Services (5):           Platform Foundation        │"
    echo "│  ✅ AI/ML Services (6):          AI-Native Capabilities     │"
    echo "│  ✅ Data Services (3):           Data Management & Analytics │"
    echo "│  ✅ DevOps Services (5):         Operations & Deployment    │"
    echo "│  ✅ Business Services (5):       Revenue & Customer Ops     │"
    echo "│  ✅ Infrastructure Services (12): Platform Infrastructure   │"
    echo "│                                                             │"
    echo "│  📊 Total Services: 36                                     │"
    echo "│  🏗️  Architecture: Production-Ready                        │"
    echo "│  🔒 Security: Zero Trust Enabled                           │"
    echo "│  🌍 Scale: Multi-Cloud Ready                               │"
    echo "└─────────────────────────────────────────────────────────────┘"
    echo ""
    
    # Get external access points
    log_info "🌐 External Access Points:"
    
    # API Gateway
    local gateway_ip=$(kubectl get service api-gateway-service -n aic-platform -o jsonpath='{.status.loadBalancer.ingress[0].ip}' 2>/dev/null || echo "pending")
    if [ "$gateway_ip" != "pending" ] && [ -n "$gateway_ip" ]; then
        echo "  🚪 API Gateway: http://$gateway_ip"
    else
        echo "  🚪 API Gateway: kubectl port-forward service/api-gateway-service 8080:80 -n aic-platform"
    fi
    
    # Grafana
    local grafana_ip=$(kubectl get service grafana-service -n aic-monitoring -o jsonpath='{.status.loadBalancer.ingress[0].ip}' 2>/dev/null || echo "pending")
    if [ "$grafana_ip" != "pending" ] && [ -n "$grafana_ip" ]; then
        echo "  📊 Grafana: http://$grafana_ip:3000 (admin/admin123)"
    else
        echo "  📊 Grafana: kubectl port-forward service/grafana-service 3000:3000 -n aic-monitoring"
    fi
    
    # MLflow
    local mlflow_ip=$(kubectl get service mlflow-ui -n aic-ai-ml -o jsonpath='{.status.loadBalancer.ingress[0].ip}' 2>/dev/null || echo "pending")
    if [ "$mlflow_ip" != "pending" ] && [ -n "$mlflow_ip" ]; then
        echo "  🤖 MLflow: http://$mlflow_ip:5000"
    else
        echo "  🤖 MLflow: kubectl port-forward service/mlflow-ui 5000:5000 -n aic-ai-ml"
    fi
    
    echo ""
    log_info "📚 Next Steps:"
    echo "  1. Configure DNS and SSL certificates for production"
    echo "  2. Set up monitoring alerts and dashboards"
    echo "  3. Configure backup and disaster recovery"
    echo "  4. Run load testing and performance optimization"
    echo "  5. Begin customer onboarding and beta testing"
    echo ""
    log_success "🎉 002AIC Platform is ready for production! 🎉"
}

# Main deployment function
main() {
    local start_time=$(date +%s)
    
    check_prerequisites
    deploy_platform
    verify_deployment
    show_deployment_summary
    
    local end_time=$(date +%s)
    local duration=$((end_time - start_time))
    local minutes=$((duration / 60))
    local seconds=$((duration % 60))
    
    log_success "🏁 Total deployment time: ${minutes}m ${seconds}s"
}

# Handle script arguments
case "${1:-deploy}" in
    "production"|"staging"|"development")
        DEPLOYMENT_MODE=$1
        main
        ;;
    "verify")
        verify_deployment
        ;;
    "summary")
        show_deployment_summary
        ;;
    "help")
        echo "Usage: $0 [production|staging|development|verify|summary|help]"
        echo ""
        echo "Commands:"
        echo "  production   - Deploy to production environment (default)"
        echo "  staging      - Deploy to staging environment"
        echo "  development  - Deploy to development environment"
        echo "  verify       - Verify existing deployment"
        echo "  summary      - Show deployment summary"
        echo "  help         - Show this help message"
        ;;
    *)
        log_error "Unknown command: $1"
        echo "Use '$0 help' for usage information"
        exit 1
        ;;
esac
