#!/bin/bash

# Production Deployment Script for 002AIC Hybrid Auth System
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
K8S_DIR="$(dirname "$SCRIPT_DIR")/k8s"
PRODUCTION_DIR="$K8S_DIR/production"

echo "🚀 Starting Production Deployment of 002AIC Hybrid Auth System"
echo "=============================================================="

# Function to check prerequisites
check_prerequisites() {
    echo "📋 Checking prerequisites..."
    
    if ! command -v kubectl &> /dev/null; then
        echo "❌ kubectl is required but not installed"
        exit 1
    fi
    
    if ! command -v helm &> /dev/null; then
        echo "❌ helm is required but not installed"
        exit 1
    fi
    
    # Check if cluster is accessible
    if ! kubectl cluster-info &> /dev/null; then
        echo "❌ Cannot connect to Kubernetes cluster"
        exit 1
    fi
    
    # Check if this is a production cluster
    CLUSTER_NAME=$(kubectl config current-context)
    echo "🎯 Deploying to cluster: $CLUSTER_NAME"
    
    read -p "⚠️  Are you sure you want to deploy to production? (yes/no): " confirm
    if [ "$confirm" != "yes" ]; then
        echo "❌ Deployment cancelled"
        exit 1
    fi
    
    echo "✅ Prerequisites check passed"
}

# Function to create secrets
create_secrets() {
    echo "🔐 Creating production secrets..."
    
    # Create namespace first
    kubectl apply -f "$PRODUCTION_DIR/namespace-production.yaml"
    
    # Kong PostgreSQL secret
    kubectl create secret generic kong-postgres-secret \
        --namespace=kong-system-prod \
        --from-literal=host="${KONG_DB_HOST:-kong-postgres.auth-services-prod.svc.cluster.local}" \
        --from-literal=username="${KONG_DB_USER:-kong}" \
        --from-literal=password="${KONG_DB_PASSWORD:-$(openssl rand -base64 32)}" \
        --dry-run=client -o yaml | kubectl apply -f -
    
    # Authorization service database secret
    kubectl create secret generic authorization-db-secret \
        --namespace=auth-services-prod \
        --from-literal=host="${AUTHZ_DB_HOST:-authz-postgres.auth-services-prod.svc.cluster.local}" \
        --from-literal=username="${AUTHZ_DB_USER:-authorization}" \
        --from-literal=password="${AUTHZ_DB_PASSWORD:-$(openssl rand -base64 32)}" \
        --dry-run=client -o yaml | kubectl apply -f -
    
    # Redis secret
    kubectl create secret generic redis-secret \
        --namespace=auth-services-prod \
        --from-literal=host="${REDIS_HOST:-redis.auth-services-prod.svc.cluster.local}" \
        --from-literal=password="${REDIS_PASSWORD:-$(openssl rand -base64 32)}" \
        --dry-run=client -o yaml | kubectl apply -f -
    
    # Keycloak client secret
    kubectl create secret generic keycloak-client-secret \
        --namespace=auth-services-prod \
        --from-literal=client-secret="${KEYCLOAK_CLIENT_SECRET:-$(openssl rand -base64 32)}" \
        --dry-run=client -o yaml | kubectl apply -f -
    
    echo "✅ Secrets created"
}

# Function to deploy databases
deploy_databases() {
    echo "🗄️  Deploying production databases..."
    
    # Add Bitnami Helm repository
    helm repo add bitnami https://charts.bitnami.com/bitnami
    helm repo update
    
    # Deploy PostgreSQL for Kong
    helm upgrade --install kong-postgres bitnami/postgresql \
        --namespace auth-services-prod \
        --set auth.postgresPassword="$(kubectl get secret kong-postgres-secret -n kong-system-prod -o jsonpath='{.data.password}' | base64 -d)" \
        --set auth.database=kong \
        --set primary.persistence.size=50Gi \
        --set primary.resources.requests.memory=2Gi \
        --set primary.resources.requests.cpu=1 \
        --set primary.resources.limits.memory=4Gi \
        --set primary.resources.limits.cpu=2 \
        --set metrics.enabled=true \
        --set metrics.serviceMonitor.enabled=true \
        --wait
    
    # Deploy PostgreSQL for Authorization Service
    helm upgrade --install authz-postgres bitnami/postgresql \
        --namespace auth-services-prod \
        --set auth.postgresPassword="$(kubectl get secret authorization-db-secret -n auth-services-prod -o jsonpath='{.data.password}' | base64 -d)" \
        --set auth.database=authorization \
        --set primary.persistence.size=20Gi \
        --set primary.resources.requests.memory=1Gi \
        --set primary.resources.requests.cpu=500m \
        --set primary.resources.limits.memory=2Gi \
        --set primary.resources.limits.cpu=1 \
        --set metrics.enabled=true \
        --set metrics.serviceMonitor.enabled=true \
        --wait
    
    # Deploy Redis
    helm upgrade --install redis bitnami/redis \
        --namespace auth-services-prod \
        --set auth.password="$(kubectl get secret redis-secret -n auth-services-prod -o jsonpath='{.data.password}' | base64 -d)" \
        --set master.persistence.size=10Gi \
        --set master.resources.requests.memory=512Mi \
        --set master.resources.requests.cpu=250m \
        --set master.resources.limits.memory=1Gi \
        --set master.resources.limits.cpu=500m \
        --set replica.replicaCount=2 \
        --set replica.persistence.size=10Gi \
        --set metrics.enabled=true \
        --set metrics.serviceMonitor.enabled=true \
        --wait
    
    echo "✅ Databases deployed"
}

# Function to deploy Keycloak
deploy_keycloak() {
    echo "🏰 Deploying production Keycloak..."
    
    # Deploy Keycloak
    helm upgrade --install keycloak bitnami/keycloak \
        --namespace auth-services-prod \
        --set auth.adminUser=admin \
        --set auth.adminPassword="${KEYCLOAK_ADMIN_PASSWORD:-$(openssl rand -base64 32)}" \
        --set production=true \
        --set proxy=edge \
        --set httpRelativePath="/auth/" \
        --set postgresql.enabled=false \
        --set externalDatabase.host=authz-postgres-postgresql.auth-services-prod.svc.cluster.local \
        --set externalDatabase.port=5432 \
        --set externalDatabase.user=postgres \
        --set externalDatabase.password="$(kubectl get secret authorization-db-secret -n auth-services-prod -o jsonpath='{.data.password}' | base64 -d)" \
        --set externalDatabase.database=keycloak \
        --set replicaCount=3 \
        --set resources.requests.memory=2Gi \
        --set resources.requests.cpu=1 \
        --set resources.limits.memory=4Gi \
        --set resources.limits.cpu=2 \
        --set ingress.enabled=true \
        --set ingress.ingressClassName=kong \
        --set ingress.hostname="${KEYCLOAK_HOSTNAME:-keycloak.002aic.com}" \
        --set ingress.tls=true \
        --set metrics.enabled=true \
        --set metrics.serviceMonitor.enabled=true \
        --wait
    
    echo "✅ Keycloak deployed"
}

# Function to deploy Kong
deploy_kong() {
    echo "🦍 Deploying production Kong..."
    
    # Run Kong migrations
    kubectl apply -f "$PRODUCTION_DIR/kong-migration-job.yaml"
    kubectl wait --for=condition=complete job/kong-migration -n kong-system-prod --timeout=300s
    
    # Deploy Kong
    kubectl apply -f "$PRODUCTION_DIR/kong-production.yaml"
    
    # Wait for Kong to be ready
    kubectl wait --for=condition=available deployment/kong-gateway -n kong-system-prod --timeout=300s
    
    echo "✅ Kong deployed"
}

# Function to deploy authorization service
deploy_authorization_service() {
    echo "🛡️  Deploying production Authorization Service..."
    
    kubectl apply -f "$PRODUCTION_DIR/authorization-service-production.yaml"
    
    # Wait for authorization service to be ready
    kubectl wait --for=condition=available deployment/authorization-service -n auth-services-prod --timeout=300s
    
    echo "✅ Authorization Service deployed"
}

# Function to deploy monitoring
deploy_monitoring() {
    echo "📊 Deploying production monitoring..."
    
    # Add Prometheus Helm repository
    helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
    helm repo update
    
    # Deploy Prometheus Operator
    helm upgrade --install kube-prometheus-stack prometheus-community/kube-prometheus-stack \
        --namespace observability-prod \
        --create-namespace \
        --set prometheus.prometheusSpec.retention=30d \
        --set prometheus.prometheusSpec.storageSpec.volumeClaimTemplate.spec.resources.requests.storage=100Gi \
        --set grafana.adminPassword="${GRAFANA_ADMIN_PASSWORD:-$(openssl rand -base64 32)}" \
        --set grafana.ingress.enabled=true \
        --set grafana.ingress.ingressClassName=kong \
        --set grafana.ingress.hosts[0]="grafana.002aic.com" \
        --wait
    
    # Deploy Jaeger
    kubectl apply -f "$K8S_DIR/jaeger-production.yaml"
    
    echo "✅ Monitoring deployed"
}

# Function to configure Kong routes
configure_kong_routes() {
    echo "🔗 Configuring Kong routes for production..."
    
    # Apply Kong services and routes
    kubectl apply -f "$K8S_DIR/kong-services-config.yaml"
    kubectl apply -f "$K8S_DIR/kong-services-extended.yaml"
    
    # Apply Kong plugins
    kubectl apply -f "$K8S_DIR/kong-keycloak-plugins.yaml"
    
    echo "✅ Kong routes configured"
}

# Function to run post-deployment tests
run_tests() {
    echo "🧪 Running post-deployment tests..."
    
    # Test Kong connectivity
    KONG_PROXY_IP=$(kubectl get svc kong-proxy -n kong-system-prod -o jsonpath='{.status.loadBalancer.ingress[0].ip}')
    if [ -n "$KONG_PROXY_IP" ]; then
        echo "✅ Kong Proxy IP: $KONG_PROXY_IP"
    else
        echo "⚠️  Kong Proxy IP not yet assigned"
    fi
    
    # Test authorization service
    kubectl run test-pod --image=curlimages/curl --rm -it --restart=Never -- \
        curl -f http://authorization-service.auth-services-prod.svc.cluster.local:8080/health
    
    echo "✅ Tests completed"
}

# Function to display deployment summary
display_summary() {
    echo ""
    echo "🎉 PRODUCTION DEPLOYMENT COMPLETED!"
    echo "=================================="
    echo ""
    echo "🔗 Access Points:"
    
    # Get LoadBalancer IPs
    KONG_PROXY_IP=$(kubectl get svc kong-proxy -n kong-system-prod -o jsonpath='{.status.loadBalancer.ingress[0].ip}' 2>/dev/null || echo "Pending")
    
    echo "  • Kong Proxy: http://$KONG_PROXY_IP"
    echo "  • Keycloak: https://${KEYCLOAK_HOSTNAME:-keycloak.002aic.com}"
    echo "  • Grafana: https://grafana.002aic.com"
    
    echo ""
    echo "🔐 Credentials:"
    echo "  • Keycloak Admin: admin / $(kubectl get secret keycloak -n auth-services-prod -o jsonpath='{.data.admin-password}' | base64 -d 2>/dev/null || echo 'Check secret')"
    echo "  • Grafana Admin: admin / $(kubectl get secret kube-prometheus-stack-grafana -n observability-prod -o jsonpath='{.data.admin-password}' | base64 -d 2>/dev/null || echo 'Check secret')"
    
    echo ""
    echo "📊 Deployment Status:"
    kubectl get pods -n kong-system-prod
    kubectl get pods -n auth-services-prod
    kubectl get pods -n observability-prod
    
    echo ""
    echo "🎯 Next Steps:"
    echo "  1. Configure DNS records for your domains"
    echo "  2. Set up SSL certificates"
    echo "  3. Configure Keycloak realm and users"
    echo "  4. Deploy your AI microservices"
    echo "  5. Set up monitoring alerts"
    
    echo ""
    echo "🚀 Your production hybrid auth system is ready!"
}

# Main deployment flow
main() {
    check_prerequisites
    create_secrets
    deploy_databases
    deploy_keycloak
    deploy_kong
    deploy_authorization_service
    deploy_monitoring
    configure_kong_routes
    run_tests
    display_summary
}

# Run main function
main "$@"
