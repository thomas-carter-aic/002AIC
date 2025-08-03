/**
 * Deployment Service - Go
 * CI/CD pipeline management and deployment orchestration for the 002AIC platform
 * Handles build pipelines, deployments, rollbacks, and environment management
 */

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"github.com/go-redis/redis/v8"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"github.com/docker/docker/client"
	"gopkg.in/yaml.v2"
)

// Configuration
type Config struct {
	Port         string
	DatabaseURL  string
	RedisURL     string
	KubeConfig   string
	DockerHost   string
	GitlabToken  string
	GithubToken  string
	Environment  string
	MaxBuilds    int
	BuildTimeout int
}

// Pipeline status constants
const (
	PipelineStatusPending   = "pending"
	PipelineStatusRunning   = "running"
	PipelineStatusSuccess   = "success"
	PipelineStatusFailed    = "failed"
	PipelineStatusCancelled = "cancelled"
)

// Deployment status constants
const (
	DeploymentStatusPending    = "pending"
	DeploymentStatusDeploying  = "deploying"
	DeploymentStatusDeployed   = "deployed"
	DeploymentStatusFailed     = "failed"
	DeploymentStatusRolledBack = "rolled_back"
)

// Environment types
const (
	EnvironmentDevelopment = "development"
	EnvironmentStaging     = "staging"
	EnvironmentProduction  = "production"
)

// Models
type Pipeline struct {
	ID          string                 `json:"id" gorm:"primaryKey"`
	Name        string                 `json:"name" gorm:"not null"`
	Repository  string                 `json:"repository" gorm:"not null"`
	Branch      string                 `json:"branch" gorm:"not null"`
	Config      map[string]interface{} `json:"config" gorm:"type:jsonb"`
	Status      string                 `json:"status" gorm:"index"`
	Triggers    []string               `json:"triggers" gorm:"type:text[]"`
	Environment string                 `json:"environment" gorm:"index"`
	UserID      string                 `json:"user_id" gorm:"index"`
	ProjectID   string                 `json:"project_id" gorm:"index"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

type Build struct {
	ID           string                 `json:"id" gorm:"primaryKey"`
	PipelineID   string                 `json:"pipeline_id" gorm:"index"`
	Pipeline     Pipeline               `json:"pipeline" gorm:"foreignKey:PipelineID"`
	Number       int                    `json:"number" gorm:"not null"`
	Status       string                 `json:"status" gorm:"index"`
	CommitSHA    string                 `json:"commit_sha"`
	CommitMsg    string                 `json:"commit_message"`
	Author       string                 `json:"author"`
	Config       map[string]interface{} `json:"config" gorm:"type:jsonb"`
	Logs         string                 `json:"logs" gorm:"type:text"`
	Artifacts    []string               `json:"artifacts" gorm:"type:text[]"`
	StartedAt    *time.Time             `json:"started_at"`
	CompletedAt  *time.Time             `json:"completed_at"`
	Duration     int64                  `json:"duration_seconds"`
	TriggeredBy  string                 `json:"triggered_by"`
	CreatedAt    time.Time              `json:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at"`
}

type Deployment struct {
	ID            string                 `json:"id" gorm:"primaryKey"`
	BuildID       string                 `json:"build_id" gorm:"index"`
	Build         Build                  `json:"build" gorm:"foreignKey:BuildID"`
	Environment   string                 `json:"environment" gorm:"index"`
	Status        string                 `json:"status" gorm:"index"`
	Version       string                 `json:"version"`
	Config        map[string]interface{} `json:"config" gorm:"type:jsonb"`
	Resources     map[string]interface{} `json:"resources" gorm:"type:jsonb"`
	HealthChecks  []string               `json:"health_checks" gorm:"type:text[]"`
	RollbackID    *string                `json:"rollback_id"`
	DeployedAt    *time.Time             `json:"deployed_at"`
	RolledBackAt  *time.Time             `json:"rolled_back_at"`
	DeployedBy    string                 `json:"deployed_by"`
	CreatedAt     time.Time              `json:"created_at"`
	UpdatedAt     time.Time              `json:"updated_at"`
}

type Environment struct {
	ID          string                 `json:"id" gorm:"primaryKey"`
	Name        string                 `json:"name" gorm:"uniqueIndex"`
	Type        string                 `json:"type" gorm:"index"`
	Config      map[string]interface{} `json:"config" gorm:"type:jsonb"`
	Variables   map[string]string      `json:"variables" gorm:"type:jsonb"`
	Secrets     map[string]string      `json:"secrets" gorm:"type:jsonb"`
	Resources   map[string]interface{} `json:"resources" gorm:"type:jsonb"`
	IsActive    bool                   `json:"is_active" gorm:"default:true"`
	ProjectID   string                 `json:"project_id" gorm:"index"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

// Service struct
type DeploymentService struct {
	db           *gorm.DB
	redis        *redis.Client
	kubeClient   kubernetes.Interface
	dockerClient *client.Client
	config       *Config
	router       *gin.Engine
	httpServer   *http.Server
}

// Prometheus metrics
var (
	pipelinesTotal = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "pipelines_total",
			Help: "Total number of pipelines",
		},
		[]string{"status", "environment"},
	)

	buildsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "builds_total",
			Help: "Total number of builds",
		},
		[]string{"pipeline_id", "status"},
	)

	deploymentsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "deployments_total",
			Help: "Total number of deployments",
		},
		[]string{"environment", "status"},
	)

	buildDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "build_duration_seconds",
			Help: "Build duration in seconds",
		},
		[]string{"pipeline_id"},
	)

	deploymentDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "deployment_duration_seconds",
			Help: "Deployment duration in seconds",
		},
		[]string{"environment"},
	)

	activeBuilds = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "active_builds",
			Help: "Number of currently active builds",
		},
	)
)

func init() {
	prometheus.MustRegister(pipelinesTotal)
	prometheus.MustRegister(buildsTotal)
	prometheus.MustRegister(deploymentsTotal)
	prometheus.MustRegister(buildDuration)
	prometheus.MustRegister(deploymentDuration)
	prometheus.MustRegister(activeBuilds)
}

func main() {
	config := &Config{
		Port:         getEnv("PORT", "8080"),
		DatabaseURL:  getEnv("DATABASE_URL", "postgres://postgres:password@localhost:5432/deployment?sslmode=disable"),
		RedisURL:     getEnv("REDIS_URL", "redis://localhost:6379"),
		KubeConfig:   getEnv("KUBECONFIG", ""),
		DockerHost:   getEnv("DOCKER_HOST", "unix:///var/run/docker.sock"),
		GitlabToken:  getEnv("GITLAB_TOKEN", ""),
		GithubToken:  getEnv("GITHUB_TOKEN", ""),
		Environment:  getEnv("ENVIRONMENT", "development"),
		MaxBuilds:    parseInt(getEnv("MAX_BUILDS", "10")),
		BuildTimeout: parseInt(getEnv("BUILD_TIMEOUT", "3600")),
	}

	service, err := NewDeploymentService(config)
	if err != nil {
		log.Fatal("Failed to create deployment service:", err)
	}

	if err := service.Start(); err != nil {
		log.Fatal("Failed to start deployment service:", err)
	}
}

func NewDeploymentService(config *Config) (*DeploymentService, error) {
	// Initialize database
	db, err := gorm.Open(postgres.Open(config.DatabaseURL), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Auto-migrate tables
	if err := db.AutoMigrate(&Pipeline{}, &Build{}, &Deployment{}, &Environment{}); err != nil {
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	// Initialize Redis
	opt, err := redis.ParseURL(config.RedisURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Redis URL: %w", err)
	}
	redisClient := redis.NewClient(opt)

	// Test Redis connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	// Initialize Kubernetes client
	var kubeClient kubernetes.Interface
	if config.KubeConfig != "" {
		kubeConfig, err := clientcmd.BuildConfigFromFlags("", config.KubeConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to build kube config: %w", err)
		}
		kubeClient, err = kubernetes.NewForConfig(kubeConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create kube client: %w", err)
		}
	} else {
		// Use in-cluster config
		kubeConfig, err := rest.InClusterConfig()
		if err != nil {
			log.Printf("Warning: Failed to get in-cluster config: %v", err)
		} else {
			kubeClient, err = kubernetes.NewForConfig(kubeConfig)
			if err != nil {
				log.Printf("Warning: Failed to create kube client: %v", err)
			}
		}
	}

	// Initialize Docker client
	dockerClient, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		log.Printf("Warning: Failed to create Docker client: %v", err)
	}

	service := &DeploymentService{
		db:           db,
		redis:        redisClient,
		kubeClient:   kubeClient,
		dockerClient: dockerClient,
		config:       config,
	}

	service.setupRoutes()
	return service, nil
}

func (s *DeploymentService) setupRoutes() {
	if s.config.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	s.router = gin.Default()

	// Middleware
	s.router.Use(gin.Recovery())
	s.router.Use(corsMiddleware())
	s.router.Use(loggingMiddleware())

	// Health check
	s.router.GET("/health", s.healthCheck)
	s.router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// API routes
	v1 := s.router.Group("/v1")
	{
		// Pipeline management
		v1.POST("/pipelines", s.createPipeline)
		v1.GET("/pipelines", s.listPipelines)
		v1.GET("/pipelines/:id", s.getPipeline)
		v1.PUT("/pipelines/:id", s.updatePipeline)
		v1.DELETE("/pipelines/:id", s.deletePipeline)

		// Build management
		v1.POST("/pipelines/:id/builds", s.triggerBuild)
		v1.GET("/pipelines/:id/builds", s.listBuilds)
		v1.GET("/builds/:id", s.getBuild)
		v1.POST("/builds/:id/cancel", s.cancelBuild)
		v1.GET("/builds/:id/logs", s.getBuildLogs)
		v1.GET("/builds/:id/artifacts", s.getBuildArtifacts)

		// Deployment management
		v1.POST("/builds/:id/deploy", s.deployBuild)
		v1.GET("/deployments", s.listDeployments)
		v1.GET("/deployments/:id", s.getDeployment)
		v1.POST("/deployments/:id/rollback", s.rollbackDeployment)
		v1.GET("/deployments/:id/status", s.getDeploymentStatus)

		// Environment management
		v1.POST("/environments", s.createEnvironment)
		v1.GET("/environments", s.listEnvironments)
		v1.GET("/environments/:id", s.getEnvironment)
		v1.PUT("/environments/:id", s.updateEnvironment)
		v1.DELETE("/environments/:id", s.deleteEnvironment)

		// Webhook endpoints
		v1.POST("/webhooks/github", s.handleGitHubWebhook)
		v1.POST("/webhooks/gitlab", s.handleGitLabWebhook)

		// Analytics
		v1.GET("/analytics/pipelines", s.getPipelineAnalytics)
		v1.GET("/analytics/deployments", s.getDeploymentAnalytics)
		v1.GET("/analytics/environments", s.getEnvironmentAnalytics)
	}
}

func (s *DeploymentService) Start() error {
	// Start background workers
	go s.startBuildWorker()
	go s.startDeploymentWorker()
	go s.startMetricsUpdater()
	go s.startCleanupWorker()

	// Start HTTP server
	s.httpServer = &http.Server{
		Addr:    ":" + s.config.Port,
		Handler: s.router,
	}

	// Graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		log.Println("Shutting down deployment service...")
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := s.httpServer.Shutdown(ctx); err != nil {
			log.Printf("Server shutdown error: %v", err)
		}

		s.cleanup()
	}()

	log.Printf("🚀 Deployment Service starting on port %s", s.config.Port)
	log.Printf("📊 Health check: http://localhost:%s/health", s.config.Port)
	log.Printf("📈 Metrics: http://localhost:%s/metrics", s.config.Port)

	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("failed to start HTTP server: %w", err)
	}

	return nil
}

func (s *DeploymentService) cleanup() {
	if s.redis != nil {
		s.redis.Close()
	}
	if s.dockerClient != nil {
		s.dockerClient.Close()
	}
	if s.db != nil {
		sqlDB, _ := s.db.DB()
		if sqlDB != nil {
			sqlDB.Close()
		}
	}
}

// Health check endpoint
func (s *DeploymentService) healthCheck(c *gin.Context) {
	status := gin.H{
		"status":    "healthy",
		"service":   "deployment-service",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"version":   "1.0.0",
	}

	// Check database connection
	sqlDB, err := s.db.DB()
	if err != nil || sqlDB.Ping() != nil {
		status["status"] = "unhealthy"
		status["database"] = "disconnected"
		c.JSON(http.StatusServiceUnavailable, status)
		return
	}
	status["database"] = "connected"

	// Check Redis connection
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := s.redis.Ping(ctx).Err(); err != nil {
		status["status"] = "unhealthy"
		status["redis"] = "disconnected"
		c.JSON(http.StatusServiceUnavailable, status)
		return
	}
	status["redis"] = "connected"

	// Check Kubernetes connection
	if s.kubeClient != nil {
		_, err := s.kubeClient.CoreV1().Namespaces().List(context.Background(), metav1.ListOptions{Limit: 1})
		if err != nil {
			status["kubernetes"] = "disconnected"
		} else {
			status["kubernetes"] = "connected"
		}
	} else {
		status["kubernetes"] = "not_configured"
	}

	// Check Docker connection
	if s.dockerClient != nil {
		_, err := s.dockerClient.Ping(context.Background())
		if err != nil {
			status["docker"] = "disconnected"
		} else {
			status["docker"] = "connected"
		}
	} else {
		status["docker"] = "not_configured"
	}

	c.JSON(http.StatusOK, status)
}

// Utility functions
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func parseInt(s string) int {
	if i, err := strconv.Atoi(s); err == nil {
		return i
	}
	return 0
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization, X-Requested-With")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

func loggingMiddleware() gin.HandlerFunc {
	return gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		return fmt.Sprintf("%s - [%s] \"%s %s %s %d %s \"%s\" %s\"\n",
			param.ClientIP,
			param.TimeStamp.Format(time.RFC1123),
			param.Method,
			param.Path,
			param.Request.Proto,
			param.StatusCode,
			param.Latency,
			param.Request.UserAgent(),
			param.ErrorMessage,
		)
	})
}
