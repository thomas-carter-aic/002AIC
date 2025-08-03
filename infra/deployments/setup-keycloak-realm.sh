#!/bin/bash

echo "🏰 Setting up Keycloak realm..."

# Wait for Keycloak to be ready
echo "⏳ Waiting for Keycloak to be ready..."
timeout=60
counter=0
while ! curl -f http://localhost:8080/health/ready &>/dev/null; do
    sleep 2
    counter=$((counter + 2))
    if [ $counter -ge $timeout ]; then
        echo "❌ Keycloak not ready within $timeout seconds"
        exit 1
    fi
done

echo "✅ Keycloak is ready!"

# Get admin access token
echo "🔑 Getting admin access token..."
ADMIN_TOKEN_RESPONSE=$(curl -s -X POST \
  "http://localhost:8080/realms/master/protocol/openid-connect/token" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "grant_type=password" \
  -d "client_id=admin-cli" \
  -d "username=admin" \
  -d "password=admin-password")

if command -v jq &> /dev/null; then
    ADMIN_TOKEN=$(echo $ADMIN_TOKEN_RESPONSE | jq -r '.access_token')
    if [ "$ADMIN_TOKEN" = "null" ] || [ -z "$ADMIN_TOKEN" ]; then
        echo "❌ Failed to get admin token"
        echo "Response: $ADMIN_TOKEN_RESPONSE"
        exit 1
    fi
else
    echo "⚠️  jq not available, extracting token manually"
    ADMIN_TOKEN=$(echo $ADMIN_TOKEN_RESPONSE | grep -o '"access_token":"[^"]*' | cut -d'"' -f4)
    if [ -z "$ADMIN_TOKEN" ]; then
        echo "❌ Failed to extract admin token"
        exit 1
    fi
fi

echo "✅ Got admin token"

# Create 002aic realm
echo "🏗️  Creating 002aic realm..."
REALM_RESPONSE=$(curl -s -X POST \
  "http://localhost:8080/admin/realms" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "realm": "002aic",
    "enabled": true,
    "displayName": "002AIC Platform",
    "registrationAllowed": true,
    "registrationEmailAsUsername": true,
    "rememberMe": true,
    "verifyEmail": false,
    "loginWithEmailAllowed": true,
    "duplicateEmailsAllowed": false,
    "resetPasswordAllowed": true,
    "editUsernameAllowed": false,
    "bruteForceProtected": true,
    "sslRequired": "none",
    "passwordPolicy": "length(8) and digits(1) and lowerCase(1) and upperCase(1)"
  }')

if [ $? -eq 0 ]; then
    echo "✅ Realm created successfully"
else
    echo "❌ Failed to create realm"
    exit 1
fi

# Create API client
echo "🔧 Creating API client..."
CLIENT_RESPONSE=$(curl -s -X POST \
  "http://localhost:8080/admin/realms/002aic/clients" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "clientId": "002aic-api",
    "name": "002AIC API Client",
    "enabled": true,
    "clientAuthenticatorType": "client-secret",
    "secret": "api-client-secret",
    "redirectUris": ["*"],
    "webOrigins": ["*"],
    "protocol": "openid-connect",
    "publicClient": false,
    "bearerOnly": false,
    "standardFlowEnabled": true,
    "implicitFlowEnabled": false,
    "directAccessGrantsEnabled": true,
    "serviceAccountsEnabled": true,
    "fullScopeAllowed": true
  }')

echo "✅ API client created"

# Create frontend client
echo "🖥️  Creating frontend client..."
FRONTEND_CLIENT_RESPONSE=$(curl -s -X POST \
  "http://localhost:8080/admin/realms/002aic/clients" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "clientId": "002aic-frontend",
    "name": "002AIC Frontend Client",
    "enabled": true,
    "publicClient": true,
    "redirectUris": ["http://localhost:3000/*", "https://app.002aic.local/*"],
    "webOrigins": ["http://localhost:3000", "https://app.002aic.local"],
    "protocol": "openid-connect",
    "standardFlowEnabled": true,
    "implicitFlowEnabled": false,
    "directAccessGrantsEnabled": false,
    "fullScopeAllowed": true
  }')

echo "✅ Frontend client created"

# Create roles
echo "👥 Creating roles..."
for role in admin user developer data-scientist model-manager; do
    ROLE_RESPONSE=$(curl -s -X POST \
      "http://localhost:8080/admin/realms/002aic/roles" \
      -H "Authorization: Bearer $ADMIN_TOKEN" \
      -H "Content-Type: application/json" \
      -d "{\"name\": \"$role\", \"description\": \"$role role\"}")
    echo "✅ Created role: $role"
done

# Create test users
echo "👤 Creating test users..."

# Admin user
ADMIN_USER_RESPONSE=$(curl -s -X POST \
  "http://localhost:8080/admin/realms/002aic/users" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "username": "admin",
    "email": "admin@002aic.local",
    "firstName": "Admin",
    "lastName": "User",
    "enabled": true,
    "emailVerified": true,
    "credentials": [{
      "type": "password",
      "value": "admin123!",
      "temporary": false
    }]
  }')

echo "✅ Created admin user"

# Test user
TEST_USER_RESPONSE=$(curl -s -X POST \
  "http://localhost:8080/admin/realms/002aic/users" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "username": "testuser",
    "email": "test@002aic.local",
    "firstName": "Test",
    "lastName": "User",
    "enabled": true,
    "emailVerified": true,
    "credentials": [{
      "type": "password",
      "value": "testpass123!",
      "temporary": false
    }]
  }')

echo "✅ Created test user"

# Data scientist user
SCIENTIST_USER_RESPONSE=$(curl -s -X POST \
  "http://localhost:8080/admin/realms/002aic/users" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "username": "scientist",
    "email": "scientist@002aic.local",
    "firstName": "Data",
    "lastName": "Scientist",
    "enabled": true,
    "emailVerified": true,
    "credentials": [{
      "type": "password",
      "value": "science123!",
      "temporary": false
    }]
  }')

echo "✅ Created scientist user"

echo ""
echo "🎉 Keycloak realm setup completed successfully!"
echo ""
echo "🔐 Test Users Created:"
echo "  - admin@002aic.local / admin123!"
echo "  - test@002aic.local / testpass123!"
echo "  - scientist@002aic.local / science123!"
echo ""
echo "🎯 You can now test token acquisition with:"
echo "curl -X POST 'http://localhost:8080/realms/002aic/protocol/openid-connect/token' \\"
echo "  -H 'Content-Type: application/x-www-form-urlencoded' \\"
echo "  -d 'grant_type=password' \\"
echo "  -d 'client_id=002aic-api' \\"
echo "  -d 'client_secret=api-client-secret' \\"
echo "  -d 'username=testuser' \\"
echo "  -d 'password=testpass123!'"
