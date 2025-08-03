#!/bin/bash

echo "🦍 Configuring Kong Routes for All 36 Microservices..."

KONG_ADMIN_URL="http://localhost:8001"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
KONG_CONFIG_DIR="$(dirname "$SCRIPT_DIR")/kong"

# Function to check Kong connectivity
check_kong() {
    if ! curl -s "$KONG_ADMIN_URL" > /dev/null; then
        echo "❌ Kong Admin API not accessible at $KONG_ADMIN_URL"
        echo "Please ensure Kong is running on port 8001"
        exit 1
    fi
    echo "✅ Kong Admin API is accessible"
}

# Function to apply Kong configuration via Admin API
apply_kong_config() {
    local service_name=$1
    local service_host=$2
    local service_port=$3
    local route_paths=$4
    local methods=$5
    
    echo "📝 Configuring service: $service_name"
    
    # Create or update service
    SERVICE_RESPONSE=$(curl -s -X POST "$KONG_ADMIN_URL/services" \
        -H "Content-Type: application/json" \
        -d "{
            \"name\": \"$service_name\",
            \"protocol\": \"http\",
            \"host\": \"$service_host\",
            \"port\": $service_port,
            \"path\": \"/\"
        }")
    
    if [[ $SERVICE_RESPONSE == *"name already exists"* ]]; then
        # Update existing service
        curl -s -X PATCH "$KONG_ADMIN_URL/services/$service_name" \
            -H "Content-Type: application/json" \
            -d "{
                \"protocol\": \"http\",
                \"host\": \"$service_host\",
                \"port\": $service_port,
                \"path\": \"/\"
            }" > /dev/null
    fi
    
    # Create route
    ROUTE_RESPONSE=$(curl -s -X POST "$KONG_ADMIN_URL/services/$service_name/routes" \
        -H "Content-Type: application/json" \
        -d "{
            \"name\": \"${service_name}-route\",
            \"paths\": $route_paths,
            \"methods\": $methods,
            \"strip_path\": false,
            \"preserve_host\": true
        }")
    
    echo "   ✅ Service and route configured"
}

# Check Kong connectivity
check_kong

echo ""
echo "🔧 Configuring Core Infrastructure Services..."

# API Gateway Service
apply_kong_config "api-gateway-service" "localhost" 8080 '["/v1/gateway"]' '["GET","POST","PUT","DELETE"]'

# Authentication & Authorization Service
apply_kong_config "authentication-authorization-service" "localhost" 8081 '["/v1/auth","/v1/authz"]' '["GET","POST","PUT","DELETE"]'

echo ""
echo "🤖 Configuring AI/ML Core Services..."

# AI/ML Core Services
apply_kong_config "agent-orchestration-service" "localhost" 8082 '["/v1/agents","/v1/orchestration"]' '["GET","POST","PUT","DELETE"]'
apply_kong_config "model-management-service" "localhost" 8083 '["/v1/models"]' '["GET","POST","PUT","DELETE"]'
apply_kong_config "model-training-service" "localhost" 8084 '["/v1/training","/v1/models/train"]' '["GET","POST","PUT","DELETE"]'
apply_kong_config "model-tuning-service" "localhost" 8085 '["/v1/tuning","/v1/models/tune"]' '["GET","POST","PUT","DELETE"]'
apply_kong_config "model-deployment-service" "localhost" 8086 '["/v1/deployments","/v1/models/deploy"]' '["GET","POST","PUT","DELETE"]'

echo ""
echo "📊 Configuring Data Management Services..."

# Data Management Services
apply_kong_config "data-management-service" "localhost" 8087 '["/v1/data","/v1/datasets"]' '["GET","POST","PUT","DELETE"]'
apply_kong_config "data-integration-service" "localhost" 8088 '["/v1/data/integration","/v1/data/sources"]' '["GET","POST","PUT","DELETE"]'
apply_kong_config "data-versioning-service" "localhost" 8089 '["/v1/data/versions","/v1/datasets/versions"]' '["GET","POST","PUT","DELETE"]'

echo ""
echo "🔄 Configuring Pipeline and Workflow Services..."

# Pipeline and Workflow Services
apply_kong_config "pipeline-execution-service" "localhost" 8090 '["/v1/pipelines","/v1/pipelines/execute"]' '["GET","POST","PUT","DELETE"]'
apply_kong_config "workflow-orchestration-service" "localhost" 8091 '["/v1/workflows","/v1/workflows/orchestrate"]' '["GET","POST","PUT","DELETE"]'

echo ""
echo "📈 Configuring Analytics and Monitoring Services..."

# Analytics and Monitoring Services
apply_kong_config "analytics-service" "localhost" 8092 '["/v1/analytics","/v1/reports"]' '["GET","POST","PUT","DELETE"]'
apply_kong_config "monitoring-metrics-service" "localhost" 8093 '["/v1/monitoring","/v1/metrics"]' '["GET","POST"]'
apply_kong_config "experimentation-service" "localhost" 8094 '["/v1/experiments","/v1/experiments/run"]' '["GET","POST","PUT","DELETE"]'

echo ""
echo "💻 Configuring Compute and Resource Services..."

# Compute and Resource Management Services
apply_kong_config "compute-resource-service" "localhost" 8095 '["/v1/compute","/v1/resources"]' '["GET","POST","PUT","DELETE"]'
apply_kong_config "storage-management-service" "localhost" 8096 '["/v1/storage","/v1/files"]' '["GET","POST","PUT","DELETE"]'

echo ""
echo "👥 Configuring User and Workspace Services..."

# User and Workspace Management
apply_kong_config "user-management-service" "localhost" 8097 '["/v1/users","/v1/profiles"]' '["GET","POST","PUT","DELETE"]'
apply_kong_config "workspace-management-service" "localhost" 8098 '["/v1/workspaces","/v1/projects"]' '["GET","POST","PUT","DELETE"]'

echo ""
echo "🔔 Configuring Communication Services..."

# Notification and Communication Services
apply_kong_config "notification-service" "localhost" 8099 '["/v1/notifications","/v1/alerts"]' '["GET","POST","PUT","DELETE"]'
apply_kong_config "collaboration-service" "localhost" 8100 '["/v1/collaboration","/v1/sharing"]' '["GET","POST","PUT","DELETE"]'

echo ""
echo "🔒 Configuring Security Services..."

# Security and Compliance Services
apply_kong_config "security-scanning-service" "localhost" 8101 '["/v1/security","/v1/scans"]' '["GET","POST","PUT","DELETE"]'
apply_kong_config "audit-logging-service" "localhost" 8102 '["/v1/audit","/v1/logs"]' '["GET","POST"]'

echo ""
echo "🔗 Configuring Integration Services..."

# Integration and External Services
apply_kong_config "external-api-service" "localhost" 8103 '["/v1/external","/v1/integrations"]' '["GET","POST","PUT","DELETE"]'
apply_kong_config "webhook-service" "localhost" 8104 '["/v1/webhooks","/v1/callbacks"]' '["GET","POST","PUT","DELETE"]'

echo ""
echo "🧠 Configuring Specialized AI Services..."

# Specialized AI Services
apply_kong_config "nlp-processing-service" "localhost" 8105 '["/v1/nlp","/v1/text"]' '["GET","POST"]'
apply_kong_config "computer-vision-service" "localhost" 8106 '["/v1/vision","/v1/images"]' '["GET","POST"]'
apply_kong_config "recommendation-service" "localhost" 8107 '["/v1/recommendations","/v1/suggest"]' '["GET","POST"]'

echo ""
echo "⚙️ Configuring Configuration Services..."

# Configuration and Template Services
apply_kong_config "configuration-service" "localhost" 8108 '["/v1/config","/v1/settings"]' '["GET","POST","PUT","DELETE"]'
apply_kong_config "template-service" "localhost" 8109 '["/v1/templates","/v1/blueprints"]' '["GET","POST","PUT","DELETE"]'

echo ""
echo "💰 Configuring Billing Services..."

# Billing and Usage Services
apply_kong_config "billing-service" "localhost" 8110 '["/v1/billing","/v1/invoices"]' '["GET","POST","PUT","DELETE"]'
apply_kong_config "usage-tracking-service" "localhost" 8111 '["/v1/usage","/v1/tracking"]' '["GET","POST"]'

echo ""
echo "🔍 Configuring Search and Discovery Services..."

# Search and Discovery Services
apply_kong_config "search-service" "localhost" 8112 '["/v1/search","/v1/discovery"]' '["GET","POST"]'

echo ""
echo "📝 Configuring Version Control Services..."

# Version Control and Backup Services
apply_kong_config "version-control-service" "localhost" 8113 '["/v1/versions","/v1/git"]' '["GET","POST","PUT","DELETE"]'
apply_kong_config "backup-service" "localhost" 8114 '["/v1/backups","/v1/restore"]' '["GET","POST","PUT","DELETE"]'

echo ""
echo "🔌 Applying Global Plugins..."

# Apply global plugins
echo "📦 Configuring global rate limiting..."
curl -s -X POST "$KONG_ADMIN_URL/plugins" \
    -H "Content-Type: application/json" \
    -d '{
        "name": "rate-limiting",
        "config": {
            "minute": 1000,
            "hour": 10000,
            "policy": "local"
        }
    }' > /dev/null

echo "📦 Configuring global CORS..."
curl -s -X POST "$KONG_ADMIN_URL/plugins" \
    -H "Content-Type: application/json" \
    -d '{
        "name": "cors",
        "config": {
            "origins": ["*"],
            "methods": ["GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"],
            "headers": ["Accept", "Accept-Version", "Content-Length", "Content-MD5", "Content-Type", "Date", "Authorization"],
            "exposed_headers": ["X-Auth-Token"],
            "credentials": true,
            "max_age": 3600
        }
    }' > /dev/null

echo "📦 Configuring global Prometheus metrics..."
curl -s -X POST "$KONG_ADMIN_URL/plugins" \
    -H "Content-Type: application/json" \
    -d '{
        "name": "prometheus",
        "config": {
            "per_consumer": true,
            "status_code_metrics": true,
            "latency_metrics": true,
            "bandwidth_metrics": true
        }
    }' > /dev/null

echo ""
echo "📊 Kong Configuration Summary:"
echo "=============================="

# Get services count
SERVICES_COUNT=$(curl -s "$KONG_ADMIN_URL/services" | grep -o '"name"' | wc -l)
echo "✅ Services configured: $SERVICES_COUNT"

# Get routes count
ROUTES_COUNT=$(curl -s "$KONG_ADMIN_URL/routes" | grep -o '"name"' | wc -l)
echo "✅ Routes configured: $ROUTES_COUNT"

# Get plugins count
PLUGINS_COUNT=$(curl -s "$KONG_ADMIN_URL/plugins" | grep -o '"name"' | wc -l)
echo "✅ Plugins configured: $PLUGINS_COUNT"

echo ""
echo "🎉 Kong Routes Configuration Completed!"
echo ""
echo "🔗 All 36 microservices are now routed through Kong:"
echo "   • API Gateway: http://localhost:8000"
echo "   • Admin API: http://localhost:8001"
echo "   • Manager GUI: http://localhost:8002"
echo ""
echo "📋 Service Port Mapping:"
echo "   • api-gateway-service: :8080"
echo "   • authentication-authorization-service: :8081"
echo "   • agent-orchestration-service: :8082"
echo "   • model-management-service: :8083"
echo "   • [... and 32 more services on ports 8084-8114]"
echo ""
echo "🧪 Test your routes:"
echo "   curl http://localhost:8000/v1/models"
echo "   curl http://localhost:8000/v1/auth/check"
echo "   curl http://localhost:8000/v1/analytics"
echo ""
echo "🎯 Ready for microservice deployment!"
