#!/bin/bash

echo "👥 Assigning roles to Keycloak users..."

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
else
    ADMIN_TOKEN=$(echo $ADMIN_TOKEN_RESPONSE | grep -o '"access_token":"[^"]*' | cut -d'"' -f4)
fi

if [ -z "$ADMIN_TOKEN" ] || [ "$ADMIN_TOKEN" = "null" ]; then
    echo "❌ Failed to get admin token"
    exit 1
fi

echo "✅ Got admin token"

# Function to get user ID by username
get_user_id() {
    local username=$1
    local response=$(curl -s -X GET \
      "http://localhost:8080/admin/realms/002aic/users?username=$username" \
      -H "Authorization: Bearer $ADMIN_TOKEN")
    
    if command -v jq &> /dev/null; then
        echo $response | jq -r '.[0].id'
    else
        echo $response | grep -o '"id":"[^"]*' | head -1 | cut -d'"' -f4
    fi
}

# Function to get role ID by name
get_role_id() {
    local role_name=$1
    local response=$(curl -s -X GET \
      "http://localhost:8080/admin/realms/002aic/roles/$role_name" \
      -H "Authorization: Bearer $ADMIN_TOKEN")
    
    if command -v jq &> /dev/null; then
        echo $response | jq -r '.id'
    else
        echo $response | grep -o '"id":"[^"]*' | head -1 | cut -d'"' -f4
    fi
}

# Function to assign role to user
assign_role() {
    local user_id=$1
    local role_name=$2
    local role_id=$3
    
    curl -s -X POST \
      "http://localhost:8080/admin/realms/002aic/users/$user_id/role-mappings/realm" \
      -H "Authorization: Bearer $ADMIN_TOKEN" \
      -H "Content-Type: application/json" \
      -d "[{\"id\":\"$role_id\",\"name\":\"$role_name\"}]"
}

# Assign roles to admin user
echo "👤 Assigning roles to admin user..."
ADMIN_USER_ID=$(get_user_id "admin")
ADMIN_ROLE_ID=$(get_role_id "admin")
USER_ROLE_ID=$(get_role_id "user")

if [ -n "$ADMIN_USER_ID" ] && [ "$ADMIN_USER_ID" != "null" ]; then
    assign_role "$ADMIN_USER_ID" "admin" "$ADMIN_ROLE_ID"
    assign_role "$ADMIN_USER_ID" "user" "$USER_ROLE_ID"
    echo "✅ Assigned admin and user roles to admin user"
else
    echo "❌ Could not find admin user"
fi

# Assign roles to test user
echo "👤 Assigning roles to test user..."
TEST_USER_ID=$(get_user_id "testuser")
DEVELOPER_ROLE_ID=$(get_role_id "developer")

if [ -n "$TEST_USER_ID" ] && [ "$TEST_USER_ID" != "null" ]; then
    assign_role "$TEST_USER_ID" "developer" "$DEVELOPER_ROLE_ID"
    assign_role "$TEST_USER_ID" "user" "$USER_ROLE_ID"
    echo "✅ Assigned developer and user roles to test user"
else
    echo "❌ Could not find test user"
fi

# Assign roles to scientist user
echo "👤 Assigning roles to scientist user..."
SCIENTIST_USER_ID=$(get_user_id "scientist")
DATA_SCIENTIST_ROLE_ID=$(get_role_id "data-scientist")

if [ -n "$SCIENTIST_USER_ID" ] && [ "$SCIENTIST_USER_ID" != "null" ]; then
    assign_role "$SCIENTIST_USER_ID" "data-scientist" "$DATA_SCIENTIST_ROLE_ID"
    assign_role "$SCIENTIST_USER_ID" "user" "$USER_ROLE_ID"
    echo "✅ Assigned data-scientist and user roles to scientist user"
else
    echo "❌ Could not find scientist user"
fi

echo ""
echo "🎉 Role assignment completed!"
echo ""
echo "👥 User Role Summary:"
echo "  - admin@002aic.local: admin, user"
echo "  - test@002aic.local: developer, user"  
echo "  - scientist@002aic.local: data-scientist, user"
echo ""
echo "🧪 Test token acquisition with:"
echo "curl -X POST 'http://localhost:8080/realms/002aic/protocol/openid-connect/token' \\"
echo "  -H 'Content-Type: application/x-www-form-urlencoded' \\"
echo "  -d 'grant_type=password' \\"
echo "  -d 'client_id=002aic-api' \\"
echo "  -d 'client_secret=api-client-secret' \\"
echo "  -d 'username=admin' \\"
echo "  -d 'password=admin123!'"
