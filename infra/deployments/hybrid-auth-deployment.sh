#!/bin/bash

# Hybrid Authentication & Authorization Deployment Script
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
INFRA_DIR="$(dirname "$SCRIPT_DIR")"

echo "🚀 Starting Hybrid Auth (Keycloak + Go Authorization Service) deployment..."

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

# Ensure namespaces exist
echo "📦 Creating/updating namespaces..."
kubectl apply -f "$INFRA_DIR/k8s/namespaces.yaml"

# Add Helm repositories
echo "📚 Adding Helm repositories..."
helm repo add bitnami https://charts.bitnami.com/bitnami
helm repo update

# Deploy Keycloak
echo "🔐 Deploying Keycloak..."
helm upgrade --install keycloak bitnami/keycloak \
    --namespace auth-services \
    --values "$INFRA_DIR/helm/keycloak/values.yaml" \
    --wait \
    --timeout 10m

# Wait for Keycloak to be ready
echo "⏳ Waiting for Keycloak to be ready..."
kubectl wait --for=condition=available --timeout=600s deployment/keycloak -n auth-services

# Deploy Authorization Service and dependencies
echo "🛡️  Deploying Authorization Service..."
kubectl apply -f "$INFRA_DIR/k8s/authorization-service.yaml"

# Wait for Authorization Service to be ready
echo "⏳ Waiting for Authorization Service to be ready..."
kubectl wait --for=condition=available --timeout=300s deployment/authorization-service -n auth-services

# Apply Kong Keycloak plugins
echo "🔌 Applying Kong Keycloak integration plugins..."
kubectl apply -f "$INFRA_DIR/k8s/kong-keycloak-plugins.yaml"

# Update Kong ingress with authorization routes
echo "🌐 Updating Kong ingress configuration..."
kubectl apply -f "$INFRA_DIR/k8s/kong-ingress-updated.yaml"

# Get service information
echo "📊 Getting service information..."
echo ""
echo "Keycloak Service:"
kubectl get svc keycloak -n auth-services

echo ""
echo "Authorization Service:"
kubectl get svc authorization-service -n auth-services

echo ""
echo "Kong Proxy Service:"
kubectl get svc kong-kong-proxy -n kong-system

# Create test user and assign roles
echo "👤 Setting up test user..."
kubectl exec -n auth-services deployment/keycloak -- \
    /opt/bitnami/keycloak/bin/kcadm.sh create users \
    -r 002aic \
    -s username=testuser \
    -s email=test@002aic.local \
    -s firstName=Test \
    -s lastName=User \
    -s enabled=true \
    --server http://localhost:8080/auth \
    --realm master \
    --user admin \
    --password admin-password || true

# Set password for test user
kubectl exec -n auth-services deployment/keycloak -- \
    /opt/bitnami/keycloak/bin/kcadm.sh set-password \
    -r 002aic \
    --username testuser \
    --new-password testpassword \
    --server http://localhost:8080/auth \
    --realm master \
    --user admin \
    --password admin-password || true

# Port forward commands for local access
echo ""
echo "🔗 To access services locally, run these commands:"
echo ""
echo "Keycloak Admin Console:"
echo "kubectl port-forward svc/keycloak 8080:80 -n auth-services"
echo "Access: http://localhost:8080/auth/admin (admin/admin-password)"
echo ""
echo "Authorization Service API:"
echo "kubectl port-forward svc/authorization-service 8081:8080 -n auth-services"
echo "Access: http://localhost:8081"
echo ""
echo "Kong API Gateway:"
echo "kubectl port-forward svc/kong-kong-proxy 8000:80 -n kong-system"
echo "Access: http://localhost:8000"

echo ""
echo "✅ Hybrid Auth deployment completed successfully!"
echo ""
echo "🔐 Authentication Flow:"
echo "  1. Users authenticate with Keycloak (OIDC/OAuth2)"
echo "  2. Keycloak issues JWT tokens"
echo "  3. Kong validates JWT tokens"
echo "  4. Authorization Service checks fine-grained permissions"
echo "  5. Requests are forwarded to AI services"
echo ""
echo "🧪 Test the setup:"
echo "  1. Get JWT token from Keycloak"
echo "  2. Use token to access /v1/authz/check endpoint"
echo "  3. Check authorization for AI service access"
echo ""
echo "📖 Next steps:"
echo "  1. Configure your AI services to call authorization service"
echo "  2. Set up role-based access control policies"
echo "  3. Integrate with your frontend applications"
echo "  4. Configure monitoring and alerting"

# Test script
cat > /tmp/test-auth.sh << 'EOF'
#!/bin/bash
echo "🧪 Testing Hybrid Auth Setup..."

# Get access token from Keycloak
echo "1. Getting access token from Keycloak..."
TOKEN_RESPONSE=$(curl -s -X POST \
  "http://localhost:8080/auth/realms/002aic/protocol/openid-connect/token" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "grant_type=password" \
  -d "client_id=002aic-api" \
  -d "client_secret=api-client-secret" \
  -d "username=testuser" \
  -d "password=testpassword")

ACCESS_TOKEN=$(echo $TOKEN_RESPONSE | jq -r '.access_token')

if [ "$ACCESS_TOKEN" = "null" ]; then
    echo "❌ Failed to get access token"
    echo "Response: $TOKEN_RESPONSE"
    exit 1
fi

echo "✅ Got access token"

# Test authorization check
echo "2. Testing authorization check..."
AUTH_RESPONSE=$(curl -s -X POST \
  "http://localhost:8081/v1/auth/check" \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "testuser",
    "resource": "model",
    "action": "read"
  }')

echo "Authorization response: $AUTH_RESPONSE"

# Test user permissions
echo "3. Getting user permissions..."
PERM_RESPONSE=$(curl -s -X GET \
  "http://localhost:8081/v1/auth/permissions" \
  -H "Authorization: Bearer $ACCESS_TOKEN")

echo "User permissions: $PERM_RESPONSE"

echo "✅ Auth testing completed!"
EOF

chmod +x /tmp/test-auth.sh
echo ""
echo "🧪 Test script created at /tmp/test-auth.sh"
echo "Run it after port-forwarding to test the auth flow"
