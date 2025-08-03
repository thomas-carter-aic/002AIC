package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// AuthConfig holds the configuration for the auth middleware
type AuthConfig struct {
	AuthorizationServiceURL string
	JWTPublicKeyURL         string
	JWTIssuer              string
	JWTAudience            string
	ServiceName            string
}

// AuthMiddleware provides authentication and authorization for AI services
type AuthMiddleware struct {
	config     AuthConfig
	httpClient *http.Client
}

// AuthorizationRequest represents a request to check permissions
type AuthorizationRequest struct {
	UserID   string                 `json:"user_id"`
	Resource string                 `json:"resource"`
	Action   string                 `json:"action"`
	Context  map[string]interface{} `json:"context,omitempty"`
}

// AuthorizationResponse represents the response from authorization service
type AuthorizationResponse struct {
	Allowed bool   `json:"allowed"`
	Reason  string `json:"reason,omitempty"`
}

// UserContext holds user information extracted from JWT
type UserContext struct {
	UserID   string   `json:"user_id"`
	Username string   `json:"username"`
	Email    string   `json:"email"`
	Roles    []string `json:"roles"`
}

// NewAuthMiddleware creates a new auth middleware instance
func NewAuthMiddleware(config AuthConfig) *AuthMiddleware {
	return &AuthMiddleware{
		config: config,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// ValidateJWT validates a JWT token and extracts user context
func (am *AuthMiddleware) ValidateJWT(tokenString string) (*UserContext, error) {
	// Remove Bearer prefix if present
	tokenString = strings.TrimPrefix(tokenString, "Bearer ")
	
	// Parse JWT token (simplified - in production, verify with Keycloak public key)
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// In production, fetch and cache Keycloak's public key from JWTPublicKeyURL
		// For now, we'll use a placeholder
		return []byte("placeholder-key"), nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse JWT: %w", err)
	}

	if !token.Valid {
		return nil, fmt.Errorf("invalid JWT token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("invalid JWT claims")
	}

	// Extract user context from claims
	userContext := &UserContext{
		UserID:   getStringClaim(claims, "sub"),
		Username: getStringClaim(claims, "preferred_username"),
		Email:    getStringClaim(claims, "email"),
	}

	// Extract roles from realm_access
	if realmAccess, ok := claims["realm_access"].(map[string]interface{}); ok {
		if roles, ok := realmAccess["roles"].([]interface{}); ok {
			for _, role := range roles {
				if roleStr, ok := role.(string); ok {
					userContext.Roles = append(userContext.Roles, roleStr)
				}
			}
		}
	}

	return userContext, nil
}

// CheckPermission checks if a user has permission for a specific resource and action
func (am *AuthMiddleware) CheckPermission(ctx context.Context, userID, resource, action string, context map[string]interface{}) (*AuthorizationResponse, error) {
	req := AuthorizationRequest{
		UserID:   userID,
		Resource: resource,
		Action:   action,
		Context:  context,
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", 
		am.config.AuthorizationServiceURL+"/v1/auth/check", 
		bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Service-Name", am.config.ServiceName)

	resp, err := am.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("authorization service returned status %d", resp.StatusCode)
	}

	var authResp AuthorizationResponse
	if err := json.NewDecoder(resp.Body).Decode(&authResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &authResp, nil
}

// HTTPMiddleware returns an HTTP middleware function for Gin/Echo/etc
func (am *AuthMiddleware) HTTPMiddleware(resource, action string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract JWT from Authorization header
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, "Authorization header required", http.StatusUnauthorized)
				return
			}

			// Validate JWT and extract user context
			userContext, err := am.ValidateJWT(authHeader)
			if err != nil {
				http.Error(w, "Invalid token: "+err.Error(), http.StatusUnauthorized)
				return
			}

			// Check permission
			authResp, err := am.CheckPermission(r.Context(), userContext.UserID, resource, action, nil)
			if err != nil {
				http.Error(w, "Authorization check failed: "+err.Error(), http.StatusInternalServerError)
				return
			}

			if !authResp.Allowed {
				http.Error(w, "Access denied: "+authResp.Reason, http.StatusForbidden)
				return
			}

			// Add user context to request context
			ctx := context.WithValue(r.Context(), "user", userContext)
			r = r.WithContext(ctx)

			// Add user headers for downstream services
			r.Header.Set("X-User-ID", userContext.UserID)
			r.Header.Set("X-User-Email", userContext.Email)
			r.Header.Set("X-User-Roles", strings.Join(userContext.Roles, ","))

			next.ServeHTTP(w, r)
		})
	}
}

// GinMiddleware returns a Gin middleware function
func (am *AuthMiddleware) GinMiddleware(resource, action string) func(c interface{}) {
	// This would be implemented for Gin framework
	// Return a placeholder for now
	return func(c interface{}) {
		// Gin-specific implementation
	}
}

// Helper function to safely extract string claims
func getStringClaim(claims jwt.MapClaims, key string) string {
	if val, ok := claims[key].(string); ok {
		return val
	}
	return ""
}

// GetUserFromContext extracts user context from request context
func GetUserFromContext(ctx context.Context) (*UserContext, bool) {
	user, ok := ctx.Value("user").(*UserContext)
	return user, ok
}
