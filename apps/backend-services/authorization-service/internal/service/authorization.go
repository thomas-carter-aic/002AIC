package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/002aic/authorization-service/internal/models"
	"github.com/002aic/authorization-service/internal/repository"
	"github.com/casbin/casbin/v2"
	gormadapter "github.com/casbin/gorm-adapter/v3"
	"go.uber.org/zap"
)

type AuthorizationService struct {
	policyRepo repository.PolicyRepository
	cacheRepo  repository.CacheRepository
	enforcer   *casbin.Enforcer
	logger     *zap.Logger
}

func NewAuthorizationService(policyRepo repository.PolicyRepository, cacheRepo repository.CacheRepository, logger *zap.Logger) *AuthorizationService {
	// Initialize Casbin enforcer with GORM adapter
	adapter, err := gormadapter.NewAdapterByDB(policyRepo.GetDB())
	if err != nil {
		logger.Fatal("Failed to initialize Casbin adapter", zap.Error(err))
	}

	enforcer, err := casbin.NewEnforcer("config/rbac_model.conf", adapter)
	if err != nil {
		logger.Fatal("Failed to initialize Casbin enforcer", zap.Error(err))
	}

	// Load policy from database
	enforcer.LoadPolicy()

	// Enable auto-save
	enforcer.EnableAutoSave(true)

	service := &AuthorizationService{
		policyRepo: policyRepo,
		cacheRepo:  cacheRepo,
		enforcer:   enforcer,
		logger:     logger,
	}

	// Initialize default policies if needed
	service.initializeDefaultPolicies()

	return service
}

func (s *AuthorizationService) CheckPermission(ctx context.Context, req *models.AuthorizationRequest) (*models.AuthorizationResponse, error) {
	// Check cache first
	cacheKey := fmt.Sprintf("authz:%s:%s:%s", req.UserID, req.Resource, req.Action)
	if cached, err := s.cacheRepo.Get(ctx, cacheKey); err == nil {
		var response models.AuthorizationResponse
		if err := json.Unmarshal([]byte(cached), &response); err == nil {
			s.logger.Debug("Authorization check served from cache", 
				zap.String("user_id", req.UserID),
				zap.String("resource", req.Resource),
				zap.String("action", req.Action),
				zap.Bool("allowed", response.Allowed))
			return &response, nil
		}
	}

	// Check permission using Casbin
	allowed, err := s.enforcer.Enforce(req.UserID, req.Resource, req.Action)
	if err != nil {
		s.logger.Error("Failed to check permission", 
			zap.String("user_id", req.UserID),
			zap.String("resource", req.Resource),
			zap.String("action", req.Action),
			zap.Error(err))
		return nil, fmt.Errorf("failed to check permission: %w", err)
	}

	response := &models.AuthorizationResponse{
		Allowed: allowed,
	}

	if !allowed {
		response.Reason = "Access denied by policy"
	}

	// Cache the result for 5 minutes
	if responseBytes, err := json.Marshal(response); err == nil {
		s.cacheRepo.Set(ctx, cacheKey, string(responseBytes), 5*time.Minute)
	}

	s.logger.Info("Authorization check completed",
		zap.String("user_id", req.UserID),
		zap.String("resource", req.Resource),
		zap.String("action", req.Action),
		zap.Bool("allowed", allowed))

	return response, nil
}

func (s *AuthorizationService) BatchCheckPermissions(ctx context.Context, req *models.BatchAuthorizationRequest) (*models.BatchAuthorizationResponse, error) {
	results := make([]models.AuthorizationResult, len(req.Requests))

	for i, resourceAction := range req.Requests {
		authReq := &models.AuthorizationRequest{
			UserID:   req.UserID,
			Resource: resourceAction.Resource,
			Action:   resourceAction.Action,
			Context:  req.Context,
		}

		authResp, err := s.CheckPermission(ctx, authReq)
		if err != nil {
			results[i] = models.AuthorizationResult{
				Resource: resourceAction.Resource,
				Action:   resourceAction.Action,
				Allowed:  false,
				Reason:   fmt.Sprintf("Error checking permission: %v", err),
			}
		} else {
			results[i] = models.AuthorizationResult{
				Resource: resourceAction.Resource,
				Action:   resourceAction.Action,
				Allowed:  authResp.Allowed,
				Reason:   authResp.Reason,
			}
		}
	}

	return &models.BatchAuthorizationResponse{
		Results: results,
	}, nil
}

func (s *AuthorizationService) GetUserPermissions(ctx context.Context, userID string) (*models.UserPermissions, error) {
	// Check cache first
	cacheKey := fmt.Sprintf("user_permissions:%s", userID)
	if cached, err := s.cacheRepo.Get(ctx, cacheKey); err == nil {
		var permissions models.UserPermissions
		if err := json.Unmarshal([]byte(cached), &permissions); err == nil {
			return &permissions, nil
		}
	}

	// Get user roles from Casbin
	roles, err := s.enforcer.GetRolesForUser(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get roles for user: %w", err)
	}
	
	// Get permissions for each role
	var allPermissions []models.Permission
	permissionMap := make(map[string]models.Permission)

	for _, role := range roles {
		permissions := s.enforcer.GetPermissionsForUser(role)
		for _, perm := range permissions {
			if len(perm) >= 3 {
				permKey := fmt.Sprintf("%s:%s", perm[1], perm[2])
				if _, exists := permissionMap[permKey]; !exists {
					permission := models.Permission{
						Name:     permKey,
						Resource: perm[1],
						Action:   perm[2],
					}
					permissionMap[permKey] = permission
					allPermissions = append(allPermissions, permission)
				}
			}
		}
	}

	userPermissions := &models.UserPermissions{
		UserID:      userID,
		Roles:       roles,
		Permissions: allPermissions,
	}

	// Cache the result for 10 minutes
	if permBytes, err := json.Marshal(userPermissions); err == nil {
		s.cacheRepo.Set(ctx, cacheKey, string(permBytes), 10*time.Minute)
	}

	return userPermissions, nil
}

func (s *AuthorizationService) AddRoleForUser(ctx context.Context, userID, role string) error {
	_, err := s.enforcer.AddRoleForUser(userID, role)
	if err != nil {
		return fmt.Errorf("failed to add role for user: %w", err)
	}

	// Invalidate cache
	cacheKey := fmt.Sprintf("user_permissions:%s", userID)
	s.cacheRepo.Delete(ctx, cacheKey)

	s.logger.Info("Role added for user",
		zap.String("user_id", userID),
		zap.String("role", role))

	return nil
}

func (s *AuthorizationService) RemoveRoleForUser(ctx context.Context, userID, role string) error {
	_, err := s.enforcer.DeleteRoleForUser(userID, role)
	if err != nil {
		return fmt.Errorf("failed to remove role for user: %w", err)
	}

	// Invalidate cache
	cacheKey := fmt.Sprintf("user_permissions:%s", userID)
	s.cacheRepo.Delete(ctx, cacheKey)

	s.logger.Info("Role removed for user",
		zap.String("user_id", userID),
		zap.String("role", role))

	return nil
}

func (s *AuthorizationService) AddPermissionForRole(ctx context.Context, role, resource, action string) error {
	_, err := s.enforcer.AddPermissionForUser(role, resource, action)
	if err != nil {
		return fmt.Errorf("failed to add permission for role: %w", err)
	}

	s.logger.Info("Permission added for role",
		zap.String("role", role),
		zap.String("resource", resource),
		zap.String("action", action))

	return nil
}

func (s *AuthorizationService) RemovePermissionForRole(ctx context.Context, role, resource, action string) error {
	_, err := s.enforcer.DeletePermissionForUser(role, resource, action)
	if err != nil {
		return fmt.Errorf("failed to remove permission for role: %w", err)
	}

	s.logger.Info("Permission removed for role",
		zap.String("role", role),
		zap.String("resource", resource),
		zap.String("action", action))

	return nil
}

func (s *AuthorizationService) initializeDefaultPolicies() {
	// Add default role hierarchy
	s.enforcer.AddRoleForUser("admin", "user")
	s.enforcer.AddRoleForUser("developer", "user")
	s.enforcer.AddRoleForUser("data-scientist", "user")
	s.enforcer.AddRoleForUser("model-manager", "user")

	// Add default permissions for admin role
	adminPermissions := [][]string{
		{"admin", models.ResourceModel, models.ActionCreate},
		{"admin", models.ResourceModel, models.ActionRead},
		{"admin", models.ResourceModel, models.ActionUpdate},
		{"admin", models.ResourceModel, models.ActionDelete},
		{"admin", models.ResourceModel, models.ActionTrain},
		{"admin", models.ResourceModel, models.ActionDeploy},
		{"admin", models.ResourceDataset, models.ActionCreate},
		{"admin", models.ResourceDataset, models.ActionRead},
		{"admin", models.ResourceDataset, models.ActionUpdate},
		{"admin", models.ResourceDataset, models.ActionDelete},
		{"admin", models.ResourcePipeline, models.ActionCreate},
		{"admin", models.ResourcePipeline, models.ActionRead},
		{"admin", models.ResourcePipeline, models.ActionUpdate},
		{"admin", models.ResourcePipeline, models.ActionDelete},
		{"admin", models.ResourcePipeline, models.ActionExecute},
		{"admin", models.ResourceWorkspace, models.ActionCreate},
		{"admin", models.ResourceWorkspace, models.ActionRead},
		{"admin", models.ResourceWorkspace, models.ActionUpdate},
		{"admin", models.ResourceWorkspace, models.ActionDelete},
		{"admin", models.ResourceCompute, models.ActionCreate},
		{"admin", models.ResourceCompute, models.ActionRead},
		{"admin", models.ResourceCompute, models.ActionUpdate},
		{"admin", models.ResourceCompute, models.ActionDelete},
	}

	// Add default permissions for developer role
	developerPermissions := [][]string{
		{"developer", models.ResourceModel, models.ActionCreate},
		{"developer", models.ResourceModel, models.ActionRead},
		{"developer", models.ResourceModel, models.ActionUpdate},
		{"developer", models.ResourceModel, models.ActionTrain},
		{"developer", models.ResourceDataset, models.ActionRead},
		{"developer", models.ResourcePipeline, models.ActionCreate},
		{"developer", models.ResourcePipeline, models.ActionRead},
		{"developer", models.ResourcePipeline, models.ActionUpdate},
		{"developer", models.ResourcePipeline, models.ActionExecute},
		{"developer", models.ResourceWorkspace, models.ActionRead},
		{"developer", models.ResourceCompute, models.ActionRead},
	}

	// Add default permissions for data-scientist role
	dataScientistPermissions := [][]string{
		{"data-scientist", models.ResourceModel, models.ActionRead},
		{"data-scientist", models.ResourceModel, models.ActionTrain},
		{"data-scientist", models.ResourceDataset, models.ActionCreate},
		{"data-scientist", models.ResourceDataset, models.ActionRead},
		{"data-scientist", models.ResourceDataset, models.ActionUpdate},
		{"data-scientist", models.ResourceExperiment, models.ActionCreate},
		{"data-scientist", models.ResourceExperiment, models.ActionRead},
		{"data-scientist", models.ResourceExperiment, models.ActionUpdate},
		{"data-scientist", models.ResourceWorkspace, models.ActionRead},
	}

	// Add default permissions for model-manager role
	modelManagerPermissions := [][]string{
		{"model-manager", models.ResourceModel, models.ActionRead},
		{"model-manager", models.ResourceModel, models.ActionUpdate},
		{"model-manager", models.ResourceModel, models.ActionDeploy},
		{"model-manager", models.ResourcePipeline, models.ActionRead},
		{"model-manager", models.ResourcePipeline, models.ActionExecute},
		{"model-manager", models.ResourceWorkspace, models.ActionRead},
	}

	// Add basic user permissions
	userPermissions := [][]string{
		{"user", models.ResourceWorkspace, models.ActionRead},
		{"user", models.ResourceModel, models.ActionRead},
		{"user", models.ResourceDataset, models.ActionRead},
	}

	// Apply all permissions
	allPermissions := [][]string{}
	allPermissions = append(allPermissions, adminPermissions...)
	allPermissions = append(allPermissions, developerPermissions...)
	allPermissions = append(allPermissions, dataScientistPermissions...)
	allPermissions = append(allPermissions, modelManagerPermissions...)
	allPermissions = append(allPermissions, userPermissions...)

	for _, perm := range allPermissions {
		s.enforcer.AddPermissionForUser(perm[0], perm[1], perm[2])
	}

	s.logger.Info("Default policies initialized")
}
