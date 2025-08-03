#!/bin/bash

# Kong + Kuma Deployment Script
set -e

NAMESPACE_KONG="kong-system"
NAMESPACE_KUMA="kuma-system"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
INFRA_DIR="$(dirname "$SCRIPT_DIR")"

echo "🚀 Starting Kong + Kuma deployment..."

# Function to check if command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Check prerequisites
echo "📋 Checking prerequisites..."
if ! command_exists kubectl; then
    echo "❌ kubectl is required but not installed"
    exit 1
fi

if ! command_exists helm; then
    echo "❌ helm is required but not installed"
    exit 1
fi

# Check if cluster is accessible
if ! kubectl cluster-info >/dev/null 2>&1; then
    echo "❌ Cannot connect to Kubernetes cluster"
    exit 1
fi

echo "✅ Prerequisites check passed"

# Create namespaces
echo "📦 Creating namespaces..."
kubectl apply -f "$INFRA_DIR/k8s/namespaces.yaml"

# Add Helm repositories
echo "📚 Adding Helm repositories..."
helm repo add kong https://charts.konghq.com
helm repo add kuma https://kumahq.github.io/charts
helm repo update

# Install Kuma Control Plane first
echo "🕸️  Installing Kuma Control Plane..."
helm upgrade --install kuma-control-plane kuma/kuma \
    --namespace $NAMESPACE_KUMA \
    --create-namespace \
    --values "$INFRA_DIR/helm/kong/values.yaml" \
    --set kuma.controlPlane.mode=standalone \
    --wait

# Wait for Kuma to be ready
echo "⏳ Waiting for Kuma Control Plane to be ready..."
kubectl wait --for=condition=available --timeout=300s deployment/kuma-control-plane -n $NAMESPACE_KUMA

# Install Kong Gateway
echo "🦍 Installing Kong Gateway..."
helm upgrade --install kong kong/kong \
    --namespace $NAMESPACE_KONG \
    --create-namespace \
    --values "$INFRA_DIR/helm/kong/values.yaml" \
    --wait

# Wait for Kong to be ready
echo "⏳ Waiting for Kong Gateway to be ready..."
kubectl wait --for=condition=available --timeout=300s deployment/kong-kong -n $NAMESPACE_KONG

# Apply Kong plugins
echo "🔌 Applying Kong plugins..."
kubectl apply -f "$INFRA_DIR/k8s/kong-plugins.yaml"

# Apply Kuma mesh policies
echo "🛡️  Applying Kuma mesh policies..."
kubectl apply -f "$INFRA_DIR/k8s/kuma-mesh-policies.yaml"

# Apply Kong ingress
echo "🌐 Applying Kong ingress configuration..."
kubectl apply -f "$INFRA_DIR/k8s/kong-ingress.yaml"

# Get service information
echo "📊 Getting service information..."
echo ""
echo "Kong Proxy Service:"
kubectl get svc kong-kong-proxy -n $NAMESPACE_KONG

echo ""
echo "Kong Admin Service:"
kubectl get svc kong-kong-admin -n $NAMESPACE_KONG

echo ""
echo "Kuma Control Plane:"
kubectl get svc kuma-control-plane -n $NAMESPACE_KUMA

# Port forward commands for local access
echo ""
echo "🔗 To access services locally, run these commands:"
echo ""
echo "Kong Admin API:"
echo "kubectl port-forward svc/kong-kong-admin 8001:8001 -n $NAMESPACE_KONG"
echo ""
echo "Kong Manager (Admin GUI):"
echo "kubectl port-forward svc/kong-kong-manager 8002:8002 -n $NAMESPACE_KONG"
echo ""
echo "Kuma Control Plane GUI:"
echo "kubectl port-forward svc/kuma-control-plane 5681:5681 -n $NAMESPACE_KUMA"
echo ""
echo "Kong Proxy (API Gateway):"
echo "kubectl port-forward svc/kong-kong-proxy 8000:80 -n $NAMESPACE_KONG"

echo ""
echo "✅ Kong + Kuma deployment completed successfully!"
echo ""
echo "🌐 Access URLs (after port-forwarding):"
echo "  - Kong Admin API: http://localhost:8001"
echo "  - Kong Manager: http://localhost:8002"
echo "  - Kuma GUI: http://localhost:5681/gui"
echo "  - API Gateway: http://localhost:8000"
echo ""
echo "📖 Next steps:"
echo "  1. Configure your microservices to use Kuma sidecar injection"
echo "  2. Set up authentication service integration"
echo "  3. Configure monitoring and observability"
