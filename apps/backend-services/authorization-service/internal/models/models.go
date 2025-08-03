package models

import (
	"time"
	"gorm.io/gorm"
)

// User represents a user in the system
type User struct {
	ID          string    `json:"id" gorm:"primaryKey"`
	Email       string    `json:"email" gorm:"uniqueIndex"`
	Username    string    `json:"username" gorm:"uniqueIndex"`
	FirstName   string    `json:"first_name"`
	LastName    string    `json:"last_name"`
	Enabled     bool      `json:"enabled" gorm:"default:true"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Roles       []Role    `json:"roles" gorm:"many2many:user_roles;"`
}

// Role represents a role in the system
type Role struct {
	ID          uint         `json:"id" gorm:"primaryKey"`
	Name        string       `json:"name" gorm:"uniqueIndex"`
	Description string       `json:"description"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
	Users       []User       `json:"users" gorm:"many2many:user_roles;"`
	Permissions []Permission `json:"permissions" gorm:"many2many:role_permissions;"`
}

// Permission represents a permission in the system
type Permission struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	Name        string    `json:"name" gorm:"uniqueIndex"`
	Resource    string    `json:"resource"`
	Action      string    `json:"action"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Roles       []Role    `json:"roles" gorm:"many2many:role_permissions;"`
}

// Policy represents a Casbin policy
type Policy struct {
	ID        uint           `json:"id" gorm:"primaryKey"`
	PType     string         `json:"p_type"`
	V0        string         `json:"v0"`
	V1        string         `json:"v1"`
	V2        string         `json:"v2"`
	V3        string         `json:"v3"`
	V4        string         `json:"v4"`
	V5        string         `json:"v5"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"deleted_at" gorm:"index"`
}

// AuthorizationRequest represents an authorization check request
type AuthorizationRequest struct {
	UserID   string `json:"user_id" binding:"required"`
	Resource string `json:"resource" binding:"required"`
	Action   string `json:"action" binding:"required"`
	Context  map[string]interface{} `json:"context,omitempty"`
}

// AuthorizationResponse represents an authorization check response
type AuthorizationResponse struct {
	Allowed bool   `json:"allowed"`
	Reason  string `json:"reason,omitempty"`
}

// BatchAuthorizationRequest represents a batch authorization check request
type BatchAuthorizationRequest struct {
	UserID   string                   `json:"user_id" binding:"required"`
	Requests []ResourceActionRequest  `json:"requests" binding:"required"`
	Context  map[string]interface{}   `json:"context,omitempty"`
}

// ResourceActionRequest represents a single resource-action pair
type ResourceActionRequest struct {
	Resource string `json:"resource" binding:"required"`
	Action   string `json:"action" binding:"required"`
}

// BatchAuthorizationResponse represents a batch authorization check response
type BatchAuthorizationResponse struct {
	Results []AuthorizationResult `json:"results"`
}

// AuthorizationResult represents a single authorization result
type AuthorizationResult struct {
	Resource string `json:"resource"`
	Action   string `json:"action"`
	Allowed  bool   `json:"allowed"`
	Reason   string `json:"reason,omitempty"`
}

// UserPermissions represents user permissions response
type UserPermissions struct {
	UserID      string       `json:"user_id"`
	Roles       []string     `json:"roles"`
	Permissions []Permission `json:"permissions"`
}

// AI Platform specific resources and actions
const (
	// Resources
	ResourceModel      = "model"
	ResourceDataset    = "dataset"
	ResourcePipeline   = "pipeline"
	ResourceWorkflow   = "workflow"
	ResourceExperiment = "experiment"
	ResourceProject    = "project"
	ResourceWorkspace  = "workspace"
	ResourceCompute    = "compute"
	ResourceStorage    = "storage"
	ResourceBilling    = "billing"

	// Actions
	ActionCreate = "create"
	ActionRead   = "read"
	ActionUpdate = "update"
	ActionDelete = "delete"
	ActionExecute = "execute"
	ActionDeploy  = "deploy"
	ActionTrain   = "train"
	ActionTune    = "tune"
	ActionShare   = "share"
	ActionPublish = "publish"
	ActionBill    = "bill"
)

// Default roles for AI platform
var DefaultRoles = []Role{
	{
		Name:        "admin",
		Description: "Platform Administrator with full access",
	},
	{
		Name:        "user",
		Description: "Basic platform user",
	},
	{
		Name:        "developer",
		Description: "AI Developer with model and pipeline access",
	},
	{
		Name:        "data-scientist",
		Description: "Data Scientist with experiment and dataset access",
	},
	{
		Name:        "model-manager",
		Description: "Model Manager with model lifecycle access",
	},
}

// Default permissions for AI platform
var DefaultPermissions = []Permission{
	// Model permissions
	{Name: "model:create", Resource: ResourceModel, Action: ActionCreate, Description: "Create models"},
	{Name: "model:read", Resource: ResourceModel, Action: ActionRead, Description: "Read models"},
	{Name: "model:update", Resource: ResourceModel, Action: ActionUpdate, Description: "Update models"},
	{Name: "model:delete", Resource: ResourceModel, Action: ActionDelete, Description: "Delete models"},
	{Name: "model:train", Resource: ResourceModel, Action: ActionTrain, Description: "Train models"},
	{Name: "model:deploy", Resource: ResourceModel, Action: ActionDeploy, Description: "Deploy models"},
	
	// Dataset permissions
	{Name: "dataset:create", Resource: ResourceDataset, Action: ActionCreate, Description: "Create datasets"},
	{Name: "dataset:read", Resource: ResourceDataset, Action: ActionRead, Description: "Read datasets"},
	{Name: "dataset:update", Resource: ResourceDataset, Action: ActionUpdate, Description: "Update datasets"},
	{Name: "dataset:delete", Resource: ResourceDataset, Action: ActionDelete, Description: "Delete datasets"},
	
	// Pipeline permissions
	{Name: "pipeline:create", Resource: ResourcePipeline, Action: ActionCreate, Description: "Create pipelines"},
	{Name: "pipeline:read", Resource: ResourcePipeline, Action: ActionRead, Description: "Read pipelines"},
	{Name: "pipeline:update", Resource: ResourcePipeline, Action: ActionUpdate, Description: "Update pipelines"},
	{Name: "pipeline:delete", Resource: ResourcePipeline, Action: ActionDelete, Description: "Delete pipelines"},
	{Name: "pipeline:execute", Resource: ResourcePipeline, Action: ActionExecute, Description: "Execute pipelines"},
	
	// Experiment permissions
	{Name: "experiment:create", Resource: ResourceExperiment, Action: ActionCreate, Description: "Create experiments"},
	{Name: "experiment:read", Resource: ResourceExperiment, Action: ActionRead, Description: "Read experiments"},
	{Name: "experiment:update", Resource: ResourceExperiment, Action: ActionUpdate, Description: "Update experiments"},
	{Name: "experiment:delete", Resource: ResourceExperiment, Action: ActionDelete, Description: "Delete experiments"},
	
	// Workspace permissions
	{Name: "workspace:create", Resource: ResourceWorkspace, Action: ActionCreate, Description: "Create workspaces"},
	{Name: "workspace:read", Resource: ResourceWorkspace, Action: ActionRead, Description: "Read workspaces"},
	{Name: "workspace:update", Resource: ResourceWorkspace, Action: ActionUpdate, Description: "Update workspaces"},
	{Name: "workspace:delete", Resource: ResourceWorkspace, Action: ActionDelete, Description: "Delete workspaces"},
	
	// Compute permissions
	{Name: "compute:create", Resource: ResourceCompute, Action: ActionCreate, Description: "Create compute resources"},
	{Name: "compute:read", Resource: ResourceCompute, Action: ActionRead, Description: "Read compute resources"},
	{Name: "compute:update", Resource: ResourceCompute, Action: ActionUpdate, Description: "Update compute resources"},
	{Name: "compute:delete", Resource: ResourceCompute, Action: ActionDelete, Description: "Delete compute resources"},
}
