#!/bin/bash

# Nexus Platform - Production Deployment Script
# This script deploys the Nexus Platform to production Kubernetes clusters

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
NC='\033[0m' # No Color

# Configuration
PROJECT_ROOT="/home/oss/002AIC"
TERRAFORM_DIR="$PROJECT_ROOT/infra/terraform/aws"
HELM_CHART_DIR="$PROJECT_ROOT/infra/helm/nexus-platform"
K8S_DIR="$PROJECT_ROOT/infra/k8s"

# Default values
ENVIRONMENT=${ENVIRONMENT:-production}
AWS_REGION=${AWS_REGION:-us-west-2}
CLUSTER_NAME=${CLUSTER_NAME:-nexus-production-cluster}
NAMESPACE=${NAMESPACE:-nexus-platform}
IMAGE_TAG=${IMAGE_TAG:-latest}
DRY_RUN=${DRY_RUN:-false}

# Function to print colored output
print_status() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

print_deploy() {
    echo -e "${PURPLE}[DEPLOY]${NC} $1"
}

# Function to check prerequisites
check_prerequisites() {
    print_status "Checking prerequisites..."
    
    # Check required tools
    local tools=("kubectl" "helm" "terraform" "aws")
    for tool in "${tools[@]}"; do
        if ! command -v "$tool" &> /dev/null; then
            print_error "$tool is not installed or not in PATH"
            exit 1
        fi
    done
    
    # Check AWS credentials
    if ! aws sts get-caller-identity &> /dev/null; then
        print_error "AWS credentials not configured"
        exit 1
    fi
    
    # Check Kubernetes context
    if ! kubectl cluster-info &> /dev/null; then
        print_error "Kubernetes cluster not accessible"
        exit 1
    fi
    
    print_success "All prerequisites met"
}

# Function to deploy infrastructure with Terraform
deploy_infrastructure() {
    print_deploy "Deploying infrastructure with Terraform..."
    
    cd "$TERRAFORM_DIR"
    
    # Initialize Terraform
    print_status "Initializing Terraform..."
    terraform init -upgrade
    
    # Plan infrastructure changes
    print_status "Planning infrastructure changes..."
    terraform plan \
        -var="environment=$ENVIRONMENT" \
        -var="aws_region=$AWS_REGION" \
        -out=tfplan
    
    if [ "$DRY_RUN" = "true" ]; then
        print_warning "Dry run mode - skipping infrastructure apply"
        return 0
    fi
    
    # Apply infrastructure changes
    print_status "Applying infrastructure changes..."
    terraform apply tfplan
    
    # Update kubeconfig
    print_status "Updating kubeconfig..."
    aws eks update-kubeconfig --region "$AWS_REGION" --name "$CLUSTER_NAME"
    
    print_success "Infrastructure deployment completed"
}

# Function to create Kubernetes resources
create_k8s_resources() {
    print_deploy "Creating Kubernetes resources..."
    
    # Create namespaces
    print_status "Creating namespaces..."
    kubectl apply -f "$K8S_DIR/namespace.yaml"
    
    # Create RBAC resources
    print_status "Creating RBAC resources..."
    kubectl apply -f "$K8S_DIR/rbac.yaml"
    
    # Create network policies
    print_status "Creating network policies..."
    kubectl apply -f "$K8S_DIR/security/network-policies.yaml"
    
    # Create secrets
    print_status "Creating secrets..."
    create_secrets
    
    print_success "Kubernetes resources created"
}

# Function to create secrets
create_secrets() {
    print_status "Creating application secrets..."
    
    # Database secrets
    kubectl create secret generic nexus-database-secret \
        --from-literal=database-url="postgresql://nexus_admin:$(get_rds_password)@$(get_rds_endpoint):5432/nexus" \
        --namespace="$NAMESPACE" \
        --dry-run=client -o yaml | kubectl apply -f -
    
    # Redis secrets
    kubectl create secret generic nexus-redis-secret \
        --from-literal=redis-url="redis://:$(get_redis_auth_token)@$(get_redis_endpoint):6379/0" \
        --namespace="$NAMESPACE" \
        --dry-run=client -o yaml | kubectl apply -f -
    
    # MLflow secrets
    kubectl create secret generic nexus-mlflow-secret \
        --from-literal=s3-access-key="$(get_s3_access_key)" \
        --from-literal=s3-secret-key="$(get_s3_secret_key)" \
        --from-literal=s3-bucket="$(get_s3_bucket_name)" \
        --namespace="$NAMESPACE" \
        --dry-run=client -o yaml | kubectl apply -f -
    
    print_success "Secrets created"
}

# Function to deploy with Helm
deploy_with_helm() {
    print_deploy "Deploying Nexus Platform with Helm..."
    
    cd "$PROJECT_ROOT"
    
    # Add required Helm repositories
    print_status "Adding Helm repositories..."
    helm repo add bitnami https://charts.bitnami.com/bitnami
    helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
    helm repo add grafana https://grafana.github.io/helm-charts
    helm repo add kong https://charts.konghq.com
    helm repo update
    
    # Create values file for environment
    create_helm_values_file
    
    if [ "$DRY_RUN" = "true" ]; then
        print_warning "Dry run mode - showing Helm template output"
        helm template nexus-platform "$HELM_CHART_DIR" \
            --namespace="$NAMESPACE" \
            --values="$HELM_CHART_DIR/values-$ENVIRONMENT.yaml"
        return 0
    fi
    
    # Deploy with Helm
    print_status "Installing/upgrading Nexus Platform..."
    helm upgrade --install nexus-platform "$HELM_CHART_DIR" \
        --namespace="$NAMESPACE" \
        --create-namespace \
        --values="$HELM_CHART_DIR/values-$ENVIRONMENT.yaml" \
        --set image.tag="$IMAGE_TAG" \
        --set environment="$ENVIRONMENT" \
        --wait \
        --timeout=20m
    
    print_success "Helm deployment completed"
}

# Function to create environment-specific values file
create_helm_values_file() {
    print_status "Creating Helm values file for $ENVIRONMENT..."
    
    cat > "$HELM_CHART_DIR/values-$ENVIRONMENT.yaml" << EOF
# Environment-specific values for $ENVIRONMENT
environment: $ENVIRONMENT

# High availability configuration
replicaCount:
  authorizationService: 3
  apiGatewayService: 3
  modelManagementService: 3
  modelTrainingService: 2

# Auto-scaling
autoscaling:
  enabled: true
  minReplicas: 2
  maxReplicas: 10

# External services (managed by Terraform)
postgresql:
  enabled: false
  external:
    host: $(get_rds_endpoint)
    port: 5432
    database: nexus
    username: nexus_admin

redis:
  enabled: false
  external:
    host: $(get_redis_endpoint)
    port: 6379

mongodb:
  enabled: false
  external:
    host: $(get_docdb_endpoint)
    port: 27017

kafka:
  enabled: false
  external:
    brokers: $(get_msk_brokers)

# Storage
persistence:
  enabled: true
  storageClass: gp3

# Ingress
ingress:
  enabled: true
  className: nginx
  hosts:
    - host: api.nexus.ai
      paths:
        - path: /
          pathType: Prefix
  tls:
    - secretName: nexus-tls-cert
      hosts:
        - api.nexus.ai

# Monitoring
monitoring:
  enabled: true
  prometheus:
    enabled: true
  grafana:
    enabled: true

# Security
security:
  enabled: true
  rbac:
    create: true
  networkPolicies:
    enabled: true
EOF

    print_success "Helm values file created"
}

# Function to run post-deployment tests
run_post_deployment_tests() {
    print_deploy "Running post-deployment tests..."
    
    # Wait for deployments to be ready
    print_status "Waiting for deployments to be ready..."
    kubectl rollout status deployment/authorization-service -n "$NAMESPACE" --timeout=300s
    kubectl rollout status deployment/model-management-service -n "$NAMESPACE" --timeout=300s
    kubectl rollout status deployment/kong-proxy -n "$NAMESPACE" --timeout=300s
    
    # Run health checks
    print_status "Running health checks..."
    
    # Port forward for testing
    kubectl port-forward svc/kong-proxy 8080:80 -n "$NAMESPACE" &
    PORT_FORWARD_PID=$!
    sleep 10
    
    # Test endpoints
    local endpoints=(
        "http://localhost:8080/health"
        "http://localhost:8080/models/health"
        "http://localhost:8080/training/health"
    )
    
    for endpoint in "${endpoints[@]}"; do
        if curl -f -s "$endpoint" > /dev/null; then
            print_success "Health check passed: $endpoint"
        else
            print_error "Health check failed: $endpoint"
            kill $PORT_FORWARD_PID 2>/dev/null || true
            exit 1
        fi
    done
    
    # Clean up port forward
    kill $PORT_FORWARD_PID 2>/dev/null || true
    
    print_success "All post-deployment tests passed"
}

# Function to setup monitoring and alerting
setup_monitoring() {
    print_deploy "Setting up monitoring and alerting..."
    
    # Deploy Prometheus monitoring rules
    kubectl apply -f "$K8S_DIR/monitoring/prometheus-rules.yaml" -n nexus-monitoring
    
    # Deploy Grafana dashboards
    kubectl apply -f "$K8S_DIR/monitoring/grafana-dashboards.yaml" -n nexus-monitoring
    
    # Setup alerting
    kubectl apply -f "$K8S_DIR/monitoring/alertmanager-config.yaml" -n nexus-monitoring
    
    print_success "Monitoring and alerting configured"
}

# Function to create backup
create_backup() {
    print_deploy "Creating pre-deployment backup..."
    
    local backup_name="nexus-backup-$(date +%Y%m%d-%H%M%S)"
    
    # Backup using Velero (if available)
    if command -v velero &> /dev/null; then
        velero backup create "$backup_name" \
            --include-namespaces "$NAMESPACE,nexus-ai,nexus-monitoring" \
            --wait
        print_success "Backup created: $backup_name"
    else
        print_warning "Velero not available - skipping backup"
    fi
}

# Helper functions to get infrastructure values
get_rds_endpoint() {
    cd "$TERRAFORM_DIR"
    terraform output -raw rds_cluster_endpoint 2>/dev/null || echo "localhost"
}

get_rds_password() {
    cd "$TERRAFORM_DIR"
    terraform output -raw rds_cluster_master_password 2>/dev/null || echo "password"
}

get_redis_endpoint() {
    cd "$TERRAFORM_DIR"
    terraform output -raw redis_cluster_cache_nodes 2>/dev/null | jq -r '.[0].address' || echo "localhost"
}

get_redis_auth_token() {
    cd "$TERRAFORM_DIR"
    terraform output -raw redis_auth_token 2>/dev/null || echo "password"
}

get_docdb_endpoint() {
    cd "$TERRAFORM_DIR"
    terraform output -raw docdb_cluster_endpoint 2>/dev/null || echo "localhost"
}

get_msk_brokers() {
    cd "$TERRAFORM_DIR"
    terraform output -raw msk_bootstrap_brokers 2>/dev/null || echo "localhost:9092"
}

get_s3_access_key() {
    cd "$TERRAFORM_DIR"
    terraform output -raw s3_access_key 2>/dev/null || echo "access_key"
}

get_s3_secret_key() {
    cd "$TERRAFORM_DIR"
    terraform output -raw s3_secret_key 2>/dev/null || echo "secret_key"
}

get_s3_bucket_name() {
    cd "$TERRAFORM_DIR"
    terraform output -raw s3_mlflow_bucket 2>/dev/null || echo "nexus-mlflow-artifacts"
}

# Function to display deployment status
display_status() {
    echo ""
    echo "🎉 Nexus Platform Production Deployment Status"
    echo "=============================================="
    echo ""
    echo "🌐 Environment: $ENVIRONMENT"
    echo "🏗️  Cluster: $CLUSTER_NAME"
    echo "📦 Namespace: $NAMESPACE"
    echo "🏷️  Image Tag: $IMAGE_TAG"
    echo ""
    echo "🔗 Access URLs:"
    echo "   - API Gateway: https://api.nexus.ai"
    echo "   - Admin Portal: https://admin.nexus.ai"
    echo "   - MLflow: https://mlflow.nexus.ai"
    echo "   - Grafana: https://grafana.nexus.ai"
    echo "   - Airflow: https://airflow.nexus.ai"
    echo ""
    echo "📊 Monitoring:"
    echo "   - Prometheus: https://prometheus.nexus.ai"
    echo "   - Grafana: https://grafana.nexus.ai"
    echo "   - Jaeger: https://jaeger.nexus.ai"
    echo ""
    echo "🔧 Management Commands:"
    echo "   - View pods: kubectl get pods -n $NAMESPACE"
    echo "   - View services: kubectl get svc -n $NAMESPACE"
    echo "   - View logs: kubectl logs -l app=authorization-service -n $NAMESPACE"
    echo "   - Scale service: kubectl scale deployment authorization-service --replicas=5 -n $NAMESPACE"
    echo ""
}

# Main execution function
main() {
    echo "🚀 Nexus Platform Production Deployment"
    echo "======================================="
    echo ""
    echo "Environment: $ENVIRONMENT"
    echo "AWS Region: $AWS_REGION"
    echo "Cluster: $CLUSTER_NAME"
    echo "Image Tag: $IMAGE_TAG"
    echo "Dry Run: $DRY_RUN"
    echo ""
    
    # Confirmation for production
    if [ "$ENVIRONMENT" = "production" ] && [ "$DRY_RUN" != "true" ]; then
        echo "⚠️  WARNING: You are about to deploy to PRODUCTION!"
        read -p "Are you sure you want to continue? (yes/no): " -r
        if [[ ! $REPLY =~ ^[Yy][Ee][Ss]$ ]]; then
            print_warning "Deployment cancelled"
            exit 0
        fi
    fi
    
    # Execute deployment steps
    check_prerequisites
    
    if [ "$DRY_RUN" != "true" ]; then
        create_backup
    fi
    
    deploy_infrastructure
    create_k8s_resources
    deploy_with_helm
    
    if [ "$DRY_RUN" != "true" ]; then
        setup_monitoring
        run_post_deployment_tests
    fi
    
    display_status
    
    print_success "🎉 Nexus Platform deployment completed successfully!"
    
    if [ "$ENVIRONMENT" = "production" ]; then
        print_deploy "🚀 Production deployment is live!"
        print_status "Monitor the deployment at: https://grafana.nexus.ai"
    fi
}

# Handle script interruption
trap 'print_warning "Deployment interrupted"; exit 1' INT TERM

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --environment)
            ENVIRONMENT="$2"
            shift 2
            ;;
        --cluster-name)
            CLUSTER_NAME="$2"
            shift 2
            ;;
        --image-tag)
            IMAGE_TAG="$2"
            shift 2
            ;;
        --dry-run)
            DRY_RUN="true"
            shift
            ;;
        --help)
            echo "Usage: $0 [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  --environment ENV     Target environment (default: production)"
            echo "  --cluster-name NAME   Kubernetes cluster name"
            echo "  --image-tag TAG       Docker image tag to deploy"
            echo "  --dry-run            Show what would be deployed without applying"
            echo "  --help               Show this help message"
            echo ""
            echo "Environment Variables:"
            echo "  ENVIRONMENT          Target environment"
            echo "  AWS_REGION           AWS region"
            echo "  CLUSTER_NAME         Kubernetes cluster name"
            echo "  IMAGE_TAG            Docker image tag"
            echo "  DRY_RUN              Enable dry run mode"
            exit 0
            ;;
        *)
            print_error "Unknown option: $1"
            exit 1
            ;;
    esac
done

# Run main function
main "$@"
