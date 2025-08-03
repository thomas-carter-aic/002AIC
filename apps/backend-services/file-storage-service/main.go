/**
 * File Storage Service - Go
 * Distributed file management for the 002AIC platform
 * Handles file upload, download, versioning, and metadata management
 */

package main

import (
	"context"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
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
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// Configuration
type Config struct {
	Port         string
	DatabaseURL  string
	RedisURL     string
	MinioURL     string
	MinioUser    string
	MinioPass    string
	MinioBucket  string
	StoragePath  string
	MaxFileSize  int64
	Environment  string
}

// File status constants
const (
	FileStatusUploading = "uploading"
	FileStatusActive    = "active"
	FileStatusArchived  = "archived"
	FileStatusDeleted   = "deleted"
	FileStatusCorrupted = "corrupted"
)

// Storage types
const (
	StorageTypeLocal = "local"
	StorageTypeS3    = "s3"
	StorageTypeMinio = "minio"
)

// Models
type FileMetadata struct {
	ID              string            `json:"id" gorm:"primaryKey"`
	OriginalName    string            `json:"original_name"`
	StoredName      string            `json:"stored_name"`
	Path            string            `json:"path"`
	Size            int64             `json:"size"`
	MimeType        string            `json:"mime_type"`
	Extension       string            `json:"extension"`
	MD5Hash         string            `json:"md5_hash" gorm:"index"`
	SHA256Hash      string            `json:"sha256_hash" gorm:"index"`
	StorageType     string            `json:"storage_type"`
	StorageLocation string            `json:"storage_location"`
	Status          string            `json:"status" gorm:"index"`
	Version         int               `json:"version"`
	ParentID        string            `json:"parent_id" gorm:"index"`
	UserID          string            `json:"user_id" gorm:"index"`
	ProjectID       string            `json:"project_id" gorm:"index"`
	Tags            []string          `json:"tags" gorm:"type:text[]"`
	Metadata        map[string]string `json:"metadata" gorm:"type:jsonb"`
	ExpiresAt       *time.Time        `json:"expires_at"`
	DownloadCount   int64             `json:"download_count"`
	LastAccessedAt  *time.Time        `json:"last_accessed_at"`
	CreatedAt       time.Time         `json:"created_at"`
	UpdatedAt       time.Time         `json:"updated_at"`
}

type FileShare struct {
	ID          string     `json:"id" gorm:"primaryKey"`
	FileID      string     `json:"file_id" gorm:"index"`
	ShareToken  string     `json:"share_token" gorm:"uniqueIndex"`
	ShareType   string     `json:"share_type"` // public, private, password
	Password    string     `json:"password,omitempty"`
	Permissions []string   `json:"permissions" gorm:"type:text[]"`
	ExpiresAt   *time.Time `json:"expires_at"`
	MaxDownloads int       `json:"max_downloads"`
	DownloadCount int      `json:"download_count"`
	CreatedBy   string     `json:"created_by"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type FileChunk struct {
	ID         string    `json:"id" gorm:"primaryKey"`
	FileID     string    `json:"file_id" gorm:"index"`
	ChunkIndex int       `json:"chunk_index"`
	Size       int64     `json:"size"`
	MD5Hash    string    `json:"md5_hash"`
	Path       string    `json:"path"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// Service struct
type FileStorageService struct {
	db         *gorm.DB
	redis      *redis.Client
	minioClient *minio.Client
	config     *Config
	router     *gin.Engine
	httpServer *http.Server
}

// Prometheus metrics
var (
	filesUploaded = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "files_uploaded_total",
			Help: "Total number of files uploaded",
		},
		[]string{"storage_type", "mime_type"},
	)

	filesDownloaded = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "files_downloaded_total",
			Help: "Total number of files downloaded",
		},
		[]string{"storage_type"},
	)

	storageUsed = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "storage_used_bytes",
			Help: "Total storage used in bytes",
		},
		[]string{"storage_type", "user_id"},
	)

	uploadDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "file_upload_duration_seconds",
			Help: "Time taken to upload files",
		},
		[]string{"storage_type", "size_category"},
	)

	downloadDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "file_download_duration_seconds",
			Help: "Time taken to download files",
		},
		[]string{"storage_type", "size_category"},
	)
)

func init() {
	prometheus.MustRegister(filesUploaded)
	prometheus.MustRegister(filesDownloaded)
	prometheus.MustRegister(storageUsed)
	prometheus.MustRegister(uploadDuration)
	prometheus.MustRegister(downloadDuration)
}

func main() {
	config := &Config{
		Port:         getEnv("PORT", "8080"),
		DatabaseURL:  getEnv("DATABASE_URL", "postgres://postgres:password@localhost:5432/filestorage?sslmode=disable"),
		RedisURL:     getEnv("REDIS_URL", "redis://localhost:6379"),
		MinioURL:     getEnv("MINIO_URL", "localhost:9000"),
		MinioUser:    getEnv("MINIO_USER", "minioadmin"),
		MinioPass:    getEnv("MINIO_PASS", "minioadmin"),
		MinioBucket:  getEnv("MINIO_BUCKET", "002aic-files"),
		StoragePath:  getEnv("STORAGE_PATH", "/tmp/002aic-storage"),
		MaxFileSize:  parseSize(getEnv("MAX_FILE_SIZE", "100MB")),
		Environment:  getEnv("ENVIRONMENT", "development"),
	}

	service, err := NewFileStorageService(config)
	if err != nil {
		log.Fatal("Failed to create file storage service:", err)
	}

	if err := service.Start(); err != nil {
		log.Fatal("Failed to start file storage service:", err)
	}
}

func NewFileStorageService(config *Config) (*FileStorageService, error) {
	// Initialize database
	db, err := gorm.Open(postgres.Open(config.DatabaseURL), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Auto-migrate tables
	if err := db.AutoMigrate(&FileMetadata{}, &FileShare{}, &FileChunk{}); err != nil {
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

	// Initialize MinIO client
	minioClient, err := minio.New(config.MinioURL, &minio.Options{
		Creds:  credentials.NewStaticV4(config.MinioUser, config.MinioPass, ""),
		Secure: false, // Set to true for HTTPS
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize MinIO client: %w", err)
	}

	// Create bucket if it doesn't exist
	ctx = context.Background()
	exists, err := minioClient.BucketExists(ctx, config.MinioBucket)
	if err != nil {
		return nil, fmt.Errorf("failed to check bucket existence: %w", err)
	}
	if !exists {
		err = minioClient.MakeBucket(ctx, config.MinioBucket, minio.MakeBucketOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to create bucket: %w", err)
		}
	}

	// Create local storage directory
	if err := os.MkdirAll(config.StoragePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create storage directory: %w", err)
	}

	service := &FileStorageService{
		db:          db,
		redis:       redisClient,
		minioClient: minioClient,
		config:      config,
	}

	service.setupRoutes()
	return service, nil
}

func (s *FileStorageService) setupRoutes() {
	if s.config.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	s.router = gin.Default()

	// Middleware
	s.router.Use(gin.Recovery())
	s.router.Use(corsMiddleware())
	s.router.Use(loggingMiddleware())

	// Set max multipart memory
	s.router.MaxMultipartMemory = 32 << 20 // 32 MiB

	// Health check
	s.router.GET("/health", s.healthCheck)
	s.router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// API routes
	v1 := s.router.Group("/v1")
	{
		// File operations
		v1.POST("/files/upload", s.uploadFile)
		v1.POST("/files/upload/chunked", s.uploadChunkedFile)
		v1.GET("/files/:id", s.getFileMetadata)
		v1.GET("/files/:id/download", s.downloadFile)
		v1.PUT("/files/:id", s.updateFileMetadata)
		v1.DELETE("/files/:id", s.deleteFile)
		v1.POST("/files/:id/versions", s.createFileVersion)
		v1.GET("/files/:id/versions", s.getFileVersions)

		// File listing and search
		v1.GET("/files", s.listFiles)
		v1.GET("/files/search", s.searchFiles)
		v1.GET("/files/duplicates", s.findDuplicates)

		// File sharing
		v1.POST("/files/:id/share", s.createFileShare)
		v1.GET("/files/:id/shares", s.getFileShares)
		v1.DELETE("/shares/:token", s.deleteFileShare)
		v1.GET("/shared/:token", s.getSharedFile)
		v1.GET("/shared/:token/download", s.downloadSharedFile)

		// Batch operations
		v1.POST("/files/batch/upload", s.batchUpload)
		v1.POST("/files/batch/delete", s.batchDelete)
		v1.POST("/files/batch/move", s.batchMove)

		// Storage management
		v1.GET("/storage/stats", s.getStorageStats)
		v1.POST("/storage/cleanup", s.cleanupStorage)
		v1.POST("/storage/migrate", s.migrateStorage)
	}
}

func (s *FileStorageService) Start() error {
	// Start background workers
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

		log.Println("Shutting down file storage service...")
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := s.httpServer.Shutdown(ctx); err != nil {
			log.Printf("Server shutdown error: %v", err)
		}

		s.cleanup()
	}()

	log.Printf("🚀 File Storage Service starting on port %s", s.config.Port)
	log.Printf("📊 Health check: http://localhost:%s/health", s.config.Port)
	log.Printf("📈 Metrics: http://localhost:%s/metrics", s.config.Port)
	log.Printf("💾 Storage path: %s", s.config.StoragePath)
	log.Printf("📦 MinIO bucket: %s", s.config.MinioBucket)

	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("failed to start HTTP server: %w", err)
	}

	return nil
}

func (s *FileStorageService) cleanup() {
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
func (s *FileStorageService) healthCheck(c *gin.Context) {
	status := gin.H{
		"status":    "healthy",
		"service":   "file-storage-service",
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

	// Check MinIO connection
	_, err = s.minioClient.BucketExists(context.Background(), s.config.MinioBucket)
	if err != nil {
		status["status"] = "unhealthy"
		status["minio"] = "disconnected"
		c.JSON(http.StatusServiceUnavailable, status)
		return
	}
	status["minio"] = "connected"

	c.JSON(http.StatusOK, status)
}

// Utility functions
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func parseSize(sizeStr string) int64 {
	sizeStr = strings.ToUpper(sizeStr)
	var multiplier int64 = 1

	if strings.HasSuffix(sizeStr, "KB") {
		multiplier = 1024
		sizeStr = strings.TrimSuffix(sizeStr, "KB")
	} else if strings.HasSuffix(sizeStr, "MB") {
		multiplier = 1024 * 1024
		sizeStr = strings.TrimSuffix(sizeStr, "MB")
	} else if strings.HasSuffix(sizeStr, "GB") {
		multiplier = 1024 * 1024 * 1024
		sizeStr = strings.TrimSuffix(sizeStr, "GB")
	}

	size, err := strconv.ParseInt(sizeStr, 10, 64)
	if err != nil {
		return 100 * 1024 * 1024 // Default 100MB
	}

	return size * multiplier
}

func getSizeCategory(size int64) string {
	if size < 1024*1024 {
		return "small" // < 1MB
	} else if size < 100*1024*1024 {
		return "medium" // < 100MB
	} else {
		return "large" // >= 100MB
	}
}

func calculateHashes(file multipart.File) (string, string, error) {
	md5Hash := md5.New()
	sha256Hash := sha256.New()
	
	// Create a multi-writer to calculate both hashes simultaneously
	multiWriter := io.MultiWriter(md5Hash, sha256Hash)
	
	// Reset file pointer
	file.Seek(0, 0)
	
	// Copy file content to both hash calculators
	if _, err := io.Copy(multiWriter, file); err != nil {
		return "", "", err
	}
	
	// Reset file pointer again for actual storage
	file.Seek(0, 0)
	
	return hex.EncodeToString(md5Hash.Sum(nil)), hex.EncodeToString(sha256Hash.Sum(nil)), nil
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
