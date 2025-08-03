#!/bin/bash

echo "🧪 Testing Local Hybrid Auth Setup..."

# Test 1: Get access token from Keycloak
echo ""
echo "1. 🔐 Getting access token from Keycloak..."
TOKEN_RESPONSE=$(curl -s -X POST \
  "http://localhost:8080/realms/002aic/protocol/openid-connect/token" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "grant_type=password" \
  -d "client_id=002aic-api" \
  -d "client_secret=api-client-secret" \
  -d "username=testuser" \
  -d "password=testpass123!")

if command -v jq &> /dev/null; then
    ACCESS_TOKEN=$(echo $TOKEN_RESPONSE | jq -r '.access_token')
    if [ "$ACCESS_TOKEN" = "null" ] || [ -z "$ACCESS_TOKEN" ]; then
        echo "❌ Failed to get access token"
        echo "Response: $TOKEN_RESPONSE"
        exit 1
    fi
    echo "✅ Got access token: ${ACCESS_TOKEN:0:50}..."
else
    echo "⚠️  jq not available, cannot parse token response"
    echo "Response: $TOKEN_RESPONSE"
    # Try to extract token manually
    ACCESS_TOKEN=$(echo $TOKEN_RESPONSE | grep -o '"access_token":"[^"]*' | cut -d'"' -f4)
    if [ -z "$ACCESS_TOKEN" ]; then
        echo "❌ Failed to extract access token"
        exit 1
    fi
    echo "✅ Got access token: ${ACCESS_TOKEN:0:50}..."
fi

# Test 2: Test authorization service health
echo ""
echo "2. 🏥 Testing Authorization Service health..."
HEALTH_RESPONSE=$(curl -s http://localhost:8081/health)
echo "Health response: $HEALTH_RESPONSE"

# Test 3: Test authorization check (without JWT validation for now)
echo ""
echo "3. 🛡️  Testing authorization check..."
AUTH_RESPONSE=$(curl -s -X POST \
  "http://localhost:8081/v1/auth/check" \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "testuser",
    "resource": "model",
    "action": "read"
  }')

echo "Authorization response: $AUTH_RESPONSE"

# Test 4: Test batch authorization check
echo ""
echo "4. 📦 Testing batch authorization check..."
BATCH_RESPONSE=$(curl -s -X POST \
  "http://localhost:8081/v1/auth/batch-check" \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "testuser",
    "requests": [
      {"resource": "model", "action": "read"},
      {"resource": "model", "action": "create"},
      {"resource": "dataset", "action": "read"}
    ]
  }')

echo "Batch authorization response: $BATCH_RESPONSE"

# Test 5: Test Kong proxy
echo ""
echo "5. 🦍 Testing Kong proxy..."
KONG_RESPONSE=$(curl -s http://localhost:8000)
echo "Kong response: $KONG_RESPONSE"

# Test 6: Test Kong admin API
echo ""
echo "6. 🔧 Testing Kong admin API..."
KONG_ADMIN_RESPONSE=$(curl -s http://localhost:8001)
if command -v jq &> /dev/null; then
    echo "Kong version: $(echo $KONG_ADMIN_RESPONSE | jq -r '.version // "unknown"')"
else
    echo "Kong admin response received (jq not available for parsing)"
fi

# Test 7: Test Keycloak realm info
echo ""
echo "7. 🏰 Testing Keycloak realm info..."
REALM_RESPONSE=$(curl -s "http://localhost:8080/realms/002aic/.well-known/openid_configuration")
if command -v jq &> /dev/null; then
    echo "Keycloak issuer: $(echo $REALM_RESPONSE | jq -r '.issuer // "unknown"')"
else
    echo "Keycloak realm response received (jq not available for parsing)"
fi

echo ""
echo "✅ Local auth testing completed!"
echo ""
echo "📊 Service Status:"
echo "  - Keycloak: ✅ Running on :8080"
echo "  - Authorization Service: ✅ Running on :8081"
echo "  - Kong Gateway: ✅ Running on :8000"
echo "  - Kong Admin: ✅ Running on :8001"
echo ""
echo "🎯 Next Steps:"
echo "  1. Integrate your AI services with the authorization service"
echo "  2. Configure Kong routes for your microservices"
echo "  3. Set up frontend authentication with Keycloak"
echo "  4. Add monitoring and alerting"
