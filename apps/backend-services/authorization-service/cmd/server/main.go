package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/002aic/authorization-service/internal/config"
	"github.com/002aic/authorization-service/internal/handler"
	"github.com/002aic/authorization-service/internal/middleware"
	"github.com/002aic/authorization-service/internal/repository"
	"github.com/002aic/authorization-service/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize logger
	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer logger.Sync()

	// Initialize database
	db, err := repository.NewPostgresDB(cfg.Database)
	if err != nil {
		logger.Fatal("Failed to connect to database", zap.Error(err))
	}

	// Initialize Redis
	redisClient, err := repository.NewRedisClient(cfg.Redis)
	if err != nil {
		logger.Fatal("Failed to connect to Redis", zap.Error(err))
	}

	// Initialize repositories
	policyRepo := repository.NewPolicyRepository(db)
	cacheRepo := repository.NewCacheRepository(redisClient)

	// Initialize services
	authzService := service.NewAuthorizationService(policyRepo, cacheRepo, logger)
	keycloakService := service.NewKeycloakService(cfg.Keycloak, logger)

	// Initialize handlers
	authzHandler := handler.NewAuthorizationHandler(authzService, keycloakService, logger)

	// Setup Gin router
	if cfg.Server.Mode == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(middleware.Logger(logger))
	router.Use(middleware.CORS())
	router.Use(middleware.Metrics())

	// Health check endpoints
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})

	router.GET("/ready", func(c *gin.Context) {
		// Check database connection
		if err := repository.PingDB(db); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not ready", "error": "database unavailable"})
			return
		}
		
		// Check Redis connection
		if err := redisClient.Ping(context.Background()).Err(); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not ready", "error": "redis unavailable"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	})

	// Metrics endpoint
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// API routes
	v1 := router.Group("/v1")
	{
		// Authorization endpoints
		auth := v1.Group("/auth")
		auth.Use(middleware.JWTAuth(cfg.JWT))
		{
			auth.POST("/check", authzHandler.CheckPermission)
			auth.POST("/batch-check", authzHandler.BatchCheckPermissions)
			auth.GET("/permissions", authzHandler.GetUserPermissions)
		}

		// Policy management endpoints (admin only)
		policies := v1.Group("/policies")
		policies.Use(middleware.JWTAuth(cfg.JWT))
		policies.Use(middleware.RequireRole("admin"))
		{
			policies.GET("", authzHandler.ListPolicies)
			policies.POST("", authzHandler.CreatePolicy)
			policies.PUT("/:id", authzHandler.UpdatePolicy)
			policies.DELETE("/:id", authzHandler.DeletePolicy)
		}

		// Role management endpoints (admin only)
		roles := v1.Group("/roles")
		roles.Use(middleware.JWTAuth(cfg.JWT))
		roles.Use(middleware.RequireRole("admin"))
		{
			roles.GET("", authzHandler.ListRoles)
			roles.POST("", authzHandler.CreateRole)
			roles.PUT("/:id", authzHandler.UpdateRole)
			roles.DELETE("/:id", authzHandler.DeleteRole)
			roles.POST("/:id/permissions", authzHandler.AssignPermissions)
		}

		// User management endpoints
		users := v1.Group("/users")
		users.Use(middleware.JWTAuth(cfg.JWT))
		{
			users.GET("/me", authzHandler.GetCurrentUser)
			users.GET("/me/permissions", authzHandler.GetCurrentUserPermissions)
			
			// Admin-only user management
			adminUsers := users.Group("")
			adminUsers.Use(middleware.RequireRole("admin"))
			{
				adminUsers.GET("", authzHandler.ListUsers)
				adminUsers.GET("/:id", authzHandler.GetUser)
				adminUsers.POST("/:id/roles", authzHandler.AssignUserRoles)
				adminUsers.DELETE("/:id/roles/:roleId", authzHandler.RemoveUserRole)
			}
		}
	}

	// Start server
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Server.Port),
		Handler: router,
	}

	// Graceful shutdown
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Failed to start server", zap.Error(err))
		}
	}()

	logger.Info("Authorization service started", 
		zap.Int("port", cfg.Server.Port),
		zap.String("mode", cfg.Server.Mode))

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Fatal("Server forced to shutdown", zap.Error(err))
	}

	logger.Info("Server exited")
}
