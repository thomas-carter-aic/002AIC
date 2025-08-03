package handler

import (
	"net/http"

	"github.com/002aic/authorization-service/internal/models"
	"github.com/002aic/authorization-service/internal/service"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type AuthorizationHandler struct {
	authzService    *service.AuthorizationService
	keycloakService *service.KeycloakService
	logger          *zap.Logger
}

func NewAuthorizationHandler(authzService *service.AuthorizationService, keycloakService *service.KeycloakService, logger *zap.Logger) *AuthorizationHandler {
	return &AuthorizationHandler{
		authzService:    authzService,
		keycloakService: keycloakService,
		logger:          logger,
	}
}

func (h *AuthorizationHandler) CheckPermission(c *gin.Context) {
	var req models.AuthorizationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	response, err := h.authzService.CheckPermission(c.Request.Context(), &req)
	if err != nil {
		h.logger.Error("Failed to check permission", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	c.JSON(http.StatusOK, response)
}

func (h *AuthorizationHandler) BatchCheckPermissions(c *gin.Context) {
	var req models.BatchAuthorizationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	response, err := h.authzService.BatchCheckPermissions(c.Request.Context(), &req)
	if err != nil {
		h.logger.Error("Failed to batch check permissions", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	c.JSON(http.StatusOK, response)
}

func (h *AuthorizationHandler) GetUserPermissions(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
		return
	}

	permissions, err := h.authzService.GetUserPermissions(c.Request.Context(), userID.(string))
	if err != nil {
		h.logger.Error("Failed to get user permissions", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	c.JSON(http.StatusOK, permissions)
}

func (h *AuthorizationHandler) GetCurrentUser(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
		return
	}

	user, err := h.keycloakService.GetUser(userID.(string))
	if err != nil {
		h.logger.Error("Failed to get current user", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	c.JSON(http.StatusOK, user)
}

func (h *AuthorizationHandler) GetCurrentUserPermissions(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
		return
	}

	permissions, err := h.authzService.GetUserPermissions(c.Request.Context(), userID.(string))
	if err != nil {
		h.logger.Error("Failed to get current user permissions", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	c.JSON(http.StatusOK, permissions)
}

// Placeholder handlers for admin functionality
func (h *AuthorizationHandler) ListPolicies(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "List policies - not implemented yet"})
}

func (h *AuthorizationHandler) CreatePolicy(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "Create policy - not implemented yet"})
}

func (h *AuthorizationHandler) UpdatePolicy(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "Update policy - not implemented yet"})
}

func (h *AuthorizationHandler) DeletePolicy(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "Delete policy - not implemented yet"})
}

func (h *AuthorizationHandler) ListRoles(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "List roles - not implemented yet"})
}

func (h *AuthorizationHandler) CreateRole(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "Create role - not implemented yet"})
}

func (h *AuthorizationHandler) UpdateRole(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "Update role - not implemented yet"})
}

func (h *AuthorizationHandler) DeleteRole(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "Delete role - not implemented yet"})
}

func (h *AuthorizationHandler) AssignPermissions(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "Assign permissions - not implemented yet"})
}

func (h *AuthorizationHandler) ListUsers(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "List users - not implemented yet"})
}

func (h *AuthorizationHandler) GetUser(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "Get user - not implemented yet"})
}

func (h *AuthorizationHandler) AssignUserRoles(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "Assign user roles - not implemented yet"})
}

func (h *AuthorizationHandler) RemoveUserRole(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "Remove user role - not implemented yet"})
}
