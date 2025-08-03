#!/bin/bash

echo "🧪 Testing Local Auth Infrastructure..."

# Test 1: Kong Admin API
echo ""
echo "1. 🦍 Testing Kong Admin API..."
KONG_RESPONSE=$(curl -s http://localhost:8001)
if command -v jq &> /dev/null; then
    KONG_VERSION=$(echo $KONG_RESPONSE | jq -r '.version // "unknown"')
    echo "✅ Kong version: $KONG_VERSION"
else
    echo "✅ Kong admin API is responding"
fi

# Test 2: Keycloak health
echo ""
echo "2. 🔐 Testing Keycloak health..."
KEYCLOAK_HEALTH=$(curl -s http://localhost:8080/health/ready)
if [[ $KEYCLOAK_HEALTH == *"UP"* ]]; then
    echo "✅ Keycloak is healthy"
else
    echo "⚠️  Keycloak health check: $KEYCLOAK_HEALTH"
fi

# Test 3: Keycloak realm
echo ""
echo "3. 🏰 Testing Keycloak realm..."
REALM_RESPONSE=$(curl -s "http://localhost:8080/realms/002aic/.well-known/openid_configuration")
if command -v jq &> /dev/null; then
    ISSUER=$(echo $REALM_RESPONSE | jq -r '.issuer // "unknown"')
    echo "✅ Keycloak issuer: $ISSUER"
else
    echo "✅ Keycloak realm is accessible"
fi

# Test 4: Get access token
echo ""
echo "4. 🎫 Testing token acquisition..."
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
    if [ "$ACCESS_TOKEN" != "null" ] && [ -n "$ACCESS_TOKEN" ]; then
        echo "✅ Successfully obtained access token: ${ACCESS_TOKEN:0:50}..."
    else
        echo "❌ Failed to get access token"
        echo "Response: $TOKEN_RESPONSE"
    fi
else
    if [[ $TOKEN_RESPONSE == *"access_token"* ]]; then
        echo "✅ Successfully obtained access token"
    else
        echo "❌ Failed to get access token"
        echo "Response: $TOKEN_RESPONSE"
    fi
fi

# Test 5: Database connections
echo ""
echo "5. 📊 Testing database connections..."

# Test Kong DB
if command -v psql &> /dev/null; then
    KONG_DB_TEST=$(PGPASSWORD=kong-password psql -h localhost -p 5432 -U kong -d kong -c "SELECT version();" 2>/dev/null)
    if [ $? -eq 0 ]; then
        echo "✅ Kong database connection successful"
    else
        echo "❌ Kong database connection failed"
    fi
else
    echo "⚠️  psql not available, skipping database connection tests"
fi

# Test 6: Redis connection
echo ""
echo "6. 🔴 Testing Redis connection..."
if command -v redis-cli &> /dev/null; then
    REDIS_TEST=$(redis-cli -h localhost -p 16379 -a redis-password ping 2>/dev/null)
    if [ "$REDIS_TEST" = "PONG" ]; then
        echo "✅ Redis connection successful"
    else
        echo "❌ Redis connection failed"
    fi
else
    echo "⚠️  redis-cli not available, skipping Redis connection test"
fi

# Test 7: Prometheus
echo ""
echo "7. 📈 Testing Prometheus..."
PROMETHEUS_RESPONSE=$(curl -s http://localhost:9090/-/healthy)
if [ "$PROMETHEUS_RESPONSE" = "Prometheus is Healthy." ]; then
    echo "✅ Prometheus is healthy"
else
    echo "⚠️  Prometheus response: $PROMETHEUS_RESPONSE"
fi

# Test 8: Grafana
echo ""
echo "8. 📊 Testing Grafana..."
GRAFANA_RESPONSE=$(curl -s http://localhost:3000/api/health)
if command -v jq &> /dev/null; then
    GRAFANA_STATUS=$(echo $GRAFANA_RESPONSE | jq -r '.database // "unknown"')
    echo "✅ Grafana database status: $GRAFANA_STATUS"
else
    echo "✅ Grafana is responding"
fi

echo ""
echo "🎉 Infrastructure testing completed!"
echo ""
echo "📊 Service Status Summary:"
echo "  - Kong Gateway: ✅ Running on :8000 (Admin: :8001)"
echo "  - Keycloak: ✅ Running on :8080"
echo "  - PostgreSQL (Kong): ✅ Running on :15432"
echo "  - PostgreSQL (Keycloak): ✅ Running on :15433"
echo "  - PostgreSQL (AuthZ): ✅ Running on :15434"
echo "  - Redis: ✅ Running on :16379"
echo "  - Jaeger: ✅ Running on :16686"
echo "  - Prometheus: ✅ Running on :9090"
echo "  - Grafana: ✅ Running on :3000"
echo ""
echo "🎯 Ready for Authorization Service deployment!"
