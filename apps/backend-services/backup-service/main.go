/**
 * Backup Service - Go
 * Data backup and recovery management for the 002AIC platform
 * Handles automated backups, point-in-time recovery, and disaster recovery
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
)

// Configuration
type Config struct {
	Port                string
	DatabaseURL         string
	RedisURL            string
	BackupStoragePath   string
	S3Bucket            string
	S3Region            string
	RetentionDays       int
	BackupInterval      time.Duration
	MaxConcurrentBackups int
	Environment         string
}

// Backup types
const (
	BackupTypeFull        = "full"
	BackupTypeIncremental = "incremental"
	BackupTypeDifferential = "differential"
	BackupTypeSnapshot    = "snapshot"
)

// Backup status
const (
	BackupStatusPending    = "pending"
	BackupStatusRunning    = "running"
	BackupStatusCompleted  = "completed"
	BackupStatusFailed     = "failed"
	BackupStatusCancelled  = "cancelled"
)

// Recovery status
const (
	RecoveryStatusPending   = "pending"
	RecoveryStatusRunning   = "running"
	RecoveryStatusCompleted = "completed"
	RecoveryStatusFailed    = "failed"
)

// Models
type BackupJob struct {
	ID              string                 `json:"id" gorm:"primaryKey"`
	Name            string                 `json:"name" gorm:"not null"`
	Type            string                 `json:"type" gorm:"not null"`
	Source          string                 `json:"source" gorm:"not null"`
	Destination     string                 `json:"destination" gorm:"not null"`
	Schedule        string                 `json:"schedule"`
	Status          string                 `json:"status" gorm:"index"`
	Progress        float64                `json:"progress" gorm:"default:0"`
	Size            int64                  `json:"size" gorm:"default:0"`
	CompressedSize  int64                  `json:"compressed_size" gorm:"default:0"`
	StartedAt       *time.Time             `json:"started_at"`
	CompletedAt     *time.Time             `json:"completed_at"`
	Duration        int64                  `json:"duration_seconds"`
	ErrorMessage    string                 `json:"error_message"`
	Config          map[string]interface{} `json:"config" gorm:"type:jsonb"`
	Metadata        map[string]interface{} `json:"metadata" gorm:"type:jsonb"`
	RetentionPolicy string                 `json:"retention_policy"`
	IsActive        bool                   `json:"is_active" gorm:"default:true"`
	CreatedBy       string                 `json:"created_by"`
	CreatedAt       time.Time              `json:"created_at"`
	UpdatedAt       time.Time              `json:"updated_at"`
}

type BackupFile struct {
	ID           string                 `json:"id" gorm:"primaryKey"`
	JobID        string                 `json:"job_id" gorm:"index"`
	Job          BackupJob              `json:"job" gorm:"foreignKey:JobID"`
	Filename     string                 `json:"filename" gorm:"not null"`
	Path         string                 `json:"path" gorm:"not null"`
	Size         int64                  `json:"size"`
	Checksum     string                 `json:"checksum"`
	Encrypted    bool                   `json:"encrypted" gorm:"default:false"`
	Compressed   bool                   `json:"compressed" gorm:"default:false"`
	StorageType  string                 `json:"storage_type"`
	Metadata     map[string]interface{} `json:"metadata" gorm:"type:jsonb"`
	ExpiresAt    *time.Time             `json:"expires_at"`
	CreatedAt    time.Time              `json:"created_at"`
}

type RecoveryJob struct {
	ID            string                 `json:"id" gorm:"primaryKey"`
	BackupFileID  string                 `json:"backup_file_id" gorm:"index"`
	BackupFile    BackupFile             `json:"backup_file" gorm:"foreignKey:BackupFileID"`
	Name          string                 `json:"name" gorm:"not null"`
	Type          string                 `json:"type" gorm:"not null"`
	Destination   string                 `json:"destination" gorm:"not null"`
	Status        string                 `json:"status" gorm:"index"`
	Progress      float64                `json:"progress" gorm:"default:0"`
	StartedAt     *time.Time             `json:"started_at"`
	CompletedAt   *time.Time             `json:"completed_at"`
	Duration      int64                  `json:"duration_seconds"`
	ErrorMessage  string                 `json:"error_message"`
	Config        map[string]interface{} `json:"config" gorm:"type:jsonb"`
	PointInTime   *time.Time             `json:"point_in_time"`
	CreatedBy     string                 `json:"created_by"`
	CreatedAt     time.Time              `json:"created_at"`
	UpdatedAt     time.Time              `json:"updated_at"`
}

// Service struct
type BackupService struct {
	db         *gorm.DB
	redis      *redis.Client
	config     *Config
	router     *gin.Engine
	httpServer *http.Server
}

// Prometheus metrics
var (
	backupJobsTotal = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "backup_jobs_total",
			Help: "Total number of backup jobs",
		},
		[]string{"type", "status"},
	)

	backupDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "backup_duration_seconds",
			Help: "Backup job duration in seconds",
		},
		[]string{"job_id", "type"},
	)

	backupSize = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "backup_size_bytes",
			Help: "Backup size in bytes",
		},
		[]string{"type"},
	)

	recoveryJobsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "recovery_jobs_total",
			Help: "Total number of recovery jobs",
		},
		[]string{"type", "status"},
	)

	storageUsed = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "backup_storage_used_bytes",
			Help: "Total backup storage used in bytes",
		},
		[]string{"storage_type"},
	)
)

func init() {
	prometheus.MustRegister(backupJobsTotal)
	prometheus.MustRegister(backupDuration)
	prometheus.MustRegister(backupSize)
	prometheus.MustRegister(recoveryJobsTotal)
	prometheus.MustRegister(storageUsed)
}

func main() {
	config := &Config{
		Port:                 getEnv("PORT", "8080"),
		DatabaseURL:          getEnv("DATABASE_URL", "postgres://postgres:password@localhost:5432/backup?sslmode=disable"),
		RedisURL:             getEnv("REDIS_URL", "redis://localhost:6379"),
		BackupStoragePath:    getEnv("BACKUP_STORAGE_PATH", "/tmp/backups"),
		S3Bucket:             getEnv("S3_BUCKET", "002aic-backups"),
		S3Region:             getEnv("S3_REGION", "us-east-1"),
		RetentionDays:        parseInt(getEnv("RETENTION_DAYS", "30")),
		BackupInterval:       time.Duration(parseInt(getEnv("BACKUP_INTERVAL", "3600"))) * time.Second,
		MaxConcurrentBackups: parseInt(getEnv("MAX_CONCURRENT_BACKUPS", "3")),
		Environment:          getEnv("ENVIRONMENT", "development"),
	}

	service, err := NewBackupService(config)
	if err != nil {
		log.Fatal("Failed to create backup service:", err)
	}

	if err := service.Start(); err != nil {
		log.Fatal("Failed to start backup service:", err)
	}
}

func NewBackupService(config *Config) (*BackupService, error) {
	// Initialize database
	db, err := gorm.Open(postgres.Open(config.DatabaseURL), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Auto-migrate tables
	if err := db.AutoMigrate(&BackupJob{}, &BackupFile{}, &RecoveryJob{}); err != nil {
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

	// Create backup storage directory
	if err := os.MkdirAll(config.BackupStoragePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create backup storage directory: %w", err)
	}

	service := &BackupService{
		db:     db,
		redis:  redisClient,
		config: config,
	}

	service.setupRoutes()
	return service, nil
}

func (s *BackupService) setupRoutes() {
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
		// Backup jobs
		v1.POST("/backup/jobs", s.createBackupJob)
		v1.GET("/backup/jobs", s.listBackupJobs)
		v1.GET("/backup/jobs/:id", s.getBackupJob)
		v1.PUT("/backup/jobs/:id", s.updateBackupJob)
		v1.DELETE("/backup/jobs/:id", s.deleteBackupJob)
		v1.POST("/backup/jobs/:id/start", s.startBackupJob)
		v1.POST("/backup/jobs/:id/cancel", s.cancelBackupJob)

		// Backup files
		v1.GET("/backup/files", s.listBackupFiles)
		v1.GET("/backup/files/:id", s.getBackupFile)
		v1.DELETE("/backup/files/:id", s.deleteBackupFile)
		v1.GET("/backup/files/:id/download", s.downloadBackupFile)

		// Recovery jobs
		v1.POST("/recovery/jobs", s.createRecoveryJob)
		v1.GET("/recovery/jobs", s.listRecoveryJobs)
		v1.GET("/recovery/jobs/:id", s.getRecoveryJob)
		v1.POST("/recovery/jobs/:id/start", s.startRecoveryJob)
		v1.POST("/recovery/jobs/:id/cancel", s.cancelRecoveryJob)

		// Point-in-time recovery
		v1.POST("/recovery/point-in-time", s.pointInTimeRecovery)
		v1.GET("/recovery/available-points", s.getAvailableRecoveryPoints)

		// Backup validation
		v1.POST("/backup/validate/:id", s.validateBackup)
		v1.GET("/backup/integrity/:id", s.checkBackupIntegrity)

		// Analytics
		v1.GET("/analytics/backup", s.getBackupAnalytics)
		v1.GET("/analytics/storage", s.getStorageAnalytics)
		v1.GET("/analytics/recovery", s.getRecoveryAnalytics)
	}
}

func (s *BackupService) Start() error {
	// Start background workers
	go s.startBackupScheduler()
	go s.startBackupWorker()
	go s.startRecoveryWorker()
	go s.startCleanupWorker()
	go s.startMetricsUpdater()

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

		log.Println("Shutting down backup service...")
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := s.httpServer.Shutdown(ctx); err != nil {
			log.Printf("Server shutdown error: %v", err)
		}

		s.cleanup()
	}()

	log.Printf("🚀 Backup Service starting on port %s", s.config.Port)
	log.Printf("📊 Health check: http://localhost:%s/health", s.config.Port)
	log.Printf("📈 Metrics: http://localhost:%s/metrics", s.config.Port)
	log.Printf("💾 Storage path: %s", s.config.BackupStoragePath)
	log.Printf("🔄 Backup interval: %v", s.config.BackupInterval)

	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("failed to start HTTP server: %w", err)
	}

	return nil
}

func (s *BackupService) cleanup() {
	if s.redis != nil {
		s.redis.Close()
	}
	if s.db != nil {
		sqlDB, _ := s.db.DB()
		if sqlDB != nil {
			sqlDB.Close()
		}
	}
}

// Health check endpoint
func (s *BackupService) healthCheck(c *gin.Context) {
	status := gin.H{
		"status":    "healthy",
		"service":   "backup-service",
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

	// Check storage directory
	if _, err := os.Stat(s.config.BackupStoragePath); os.IsNotExist(err) {
		status["status"] = "unhealthy"
		status["storage"] = "directory_not_found"
		c.JSON(http.StatusServiceUnavailable, status)
		return
	}
	status["storage"] = "accessible"

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
