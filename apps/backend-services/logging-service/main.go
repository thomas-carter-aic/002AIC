/**
 * Logging Service - Go
 * Centralized log management and analysis for the 002AIC platform
 * Handles log ingestion, storage, search, and real-time monitoring
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
	"github.com/elastic/go-elasticsearch/v8"
)

// Configuration
type Config struct {
	Port            string
	DatabaseURL     string
	RedisURL        string
	ElasticsearchURL string
	Environment     string
	LogRetentionDays int
	MaxLogSize      int64
	BatchSize       int
	FlushInterval   time.Duration
}

// Log levels
const (
	LogLevelTrace = "trace"
	LogLevelDebug = "debug"
	LogLevelInfo  = "info"
	LogLevelWarn  = "warn"
	LogLevelError = "error"
	LogLevelFatal = "fatal"
)

// Models
type LogEntry struct {
	ID        string                 `json:"id" gorm:"primaryKey"`
	Timestamp time.Time              `json:"timestamp" gorm:"index"`
	Level     string                 `json:"level" gorm:"index"`
	Service   string                 `json:"service" gorm:"index"`
	Message   string                 `json:"message"`
	Fields    map[string]interface{} `json:"fields" gorm:"type:jsonb"`
	TraceID   string                 `json:"trace_id" gorm:"index"`
	SpanID    string                 `json:"span_id"`
	UserID    string                 `json:"user_id" gorm:"index"`
	RequestID string                 `json:"request_id" gorm:"index"`
	Source    string                 `json:"source"`
	Tags      []string               `json:"tags" gorm:"type:text[]"`
	CreatedAt time.Time              `json:"created_at"`
}

type LogAlert struct {
	ID          string                 `json:"id" gorm:"primaryKey"`
	Name        string                 `json:"name" gorm:"not null"`
	Query       string                 `json:"query" gorm:"not null"`
	Threshold   int                    `json:"threshold"`
	TimeWindow  int                    `json:"time_window"`
	Severity    string                 `json:"severity"`
	IsActive    bool                   `json:"is_active" gorm:"default:true"`
	LastTriggered *time.Time           `json:"last_triggered"`
	Config      map[string]interface{} `json:"config" gorm:"type:jsonb"`
	CreatedBy   string                 `json:"created_by"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

// Service struct
type LoggingService struct {
	db         *gorm.DB
	redis      *redis.Client
	es         *elasticsearch.Client
	config     *Config
	router     *gin.Engine
	httpServer *http.Server
	logBuffer  chan *LogEntry
}

// Prometheus metrics
var (
	logsIngested = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "logs_ingested_total",
			Help: "Total number of logs ingested",
		},
		[]string{"service", "level"},
	)

	logProcessingDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "log_processing_duration_seconds",
			Help: "Time taken to process logs",
		},
		[]string{"operation"},
	)

	activeAlerts = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "log_alerts_active",
			Help: "Number of active log alerts",
		},
	)

	logBufferSize = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "log_buffer_size",
			Help: "Current size of log buffer",
		},
	)
)

func init() {
	prometheus.MustRegister(logsIngested)
	prometheus.MustRegister(logProcessingDuration)
	prometheus.MustRegister(activeAlerts)
	prometheus.MustRegister(logBufferSize)
}

func main() {
	config := &Config{
		Port:             getEnv("PORT", "8080"),
		DatabaseURL:      getEnv("DATABASE_URL", "postgres://postgres:password@localhost:5432/logging?sslmode=disable"),
		RedisURL:         getEnv("REDIS_URL", "redis://localhost:6379"),
		ElasticsearchURL: getEnv("ELASTICSEARCH_URL", "http://localhost:9200"),
		Environment:      getEnv("ENVIRONMENT", "development"),
		LogRetentionDays: parseInt(getEnv("LOG_RETENTION_DAYS", "30")),
		MaxLogSize:       parseInt64(getEnv("MAX_LOG_SIZE", "1048576")), // 1MB
		BatchSize:        parseInt(getEnv("BATCH_SIZE", "100")),
		FlushInterval:    time.Duration(parseInt(getEnv("FLUSH_INTERVAL", "5"))) * time.Second,
	}

	service, err := NewLoggingService(config)
	if err != nil {
		log.Fatal("Failed to create logging service:", err)
	}

	if err := service.Start(); err != nil {
		log.Fatal("Failed to start logging service:", err)
	}
}

func NewLoggingService(config *Config) (*LoggingService, error) {
	// Initialize database
	db, err := gorm.Open(postgres.Open(config.DatabaseURL), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Auto-migrate tables
	if err := db.AutoMigrate(&LogEntry{}, &LogAlert{}); err != nil {
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

	// Initialize Elasticsearch
	cfg := elasticsearch.Config{
		Addresses: []string{config.ElasticsearchURL},
	}
	es, err := elasticsearch.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create Elasticsearch client: %w", err)
	}

	service := &LoggingService{
		db:        db,
		redis:     redisClient,
		es:        es,
		config:    config,
		logBuffer: make(chan *LogEntry, config.BatchSize*10),
	}

	service.setupRoutes()
	return service, nil
}

func (s *LoggingService) setupRoutes() {
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
		// Log ingestion
		v1.POST("/logs", s.ingestLog)
		v1.POST("/logs/batch", s.ingestBatchLogs)

		// Log search and retrieval
		v1.GET("/logs", s.searchLogs)
		v1.GET("/logs/:id", s.getLog)
		v1.GET("/logs/stream", s.streamLogs)

		// Log alerts
		v1.POST("/alerts", s.createLogAlert)
		v1.GET("/alerts", s.listLogAlerts)
		v1.GET("/alerts/:id", s.getLogAlert)
		v1.PUT("/alerts/:id", s.updateLogAlert)
		v1.DELETE("/alerts/:id", s.deleteLogAlert)

		// Log analytics
		v1.GET("/analytics/summary", s.getLogSummary)
		v1.GET("/analytics/trends", s.getLogTrends)
		v1.GET("/analytics/errors", s.getErrorAnalytics)
	}
}

func (s *LoggingService) Start() error {
	// Start background workers
	go s.startLogProcessor()
	go s.startAlertProcessor()
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

		log.Println("Shutting down logging service...")
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := s.httpServer.Shutdown(ctx); err != nil {
			log.Printf("Server shutdown error: %v", err)
		}

		s.cleanup()
	}()

	log.Printf("🚀 Logging Service starting on port %s", s.config.Port)
	log.Printf("📊 Health check: http://localhost:%s/health", s.config.Port)
	log.Printf("📈 Metrics: http://localhost:%s/metrics", s.config.Port)

	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("failed to start HTTP server: %w", err)
	}

	return nil
}

func (s *LoggingService) cleanup() {
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
func (s *LoggingService) healthCheck(c *gin.Context) {
	status := gin.H{
		"status":    "healthy",
		"service":   "logging-service",
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

	// Check Elasticsearch connection
	_, err = s.es.Info()
	if err != nil {
		status["status"] = "unhealthy"
		status["elasticsearch"] = "disconnected"
		c.JSON(http.StatusServiceUnavailable, status)
		return
	}
	status["elasticsearch"] = "connected"

	status["log_buffer_size"] = len(s.logBuffer)

	c.JSON(http.StatusOK, status)
}

// Log ingestion endpoint
func (s *LoggingService) ingestLog(c *gin.Context) {
	var logData map[string]interface{}
	if err := c.ShouldBindJSON(&logData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid log data"})
		return
	}

	// Create log entry
	logEntry := &LogEntry{
		ID:        uuid.New().String(),
		Timestamp: time.Now().UTC(),
		Level:     getString(logData, "level", LogLevelInfo),
		Service:   getString(logData, "service", "unknown"),
		Message:   getString(logData, "message", ""),
		Fields:    getMap(logData, "fields"),
		TraceID:   getString(logData, "trace_id", ""),
		SpanID:    getString(logData, "span_id", ""),
		UserID:    getString(logData, "user_id", ""),
		RequestID: getString(logData, "request_id", ""),
		Source:    getString(logData, "source", c.ClientIP()),
		Tags:      getStringSlice(logData, "tags"),
		CreatedAt: time.Now().UTC(),
	}

	// Add to buffer for processing
	select {
	case s.logBuffer <- logEntry:
		// Update metrics
		logsIngested.WithLabelValues(logEntry.Service, logEntry.Level).Inc()
		logBufferSize.Set(float64(len(s.logBuffer)))

		c.JSON(http.StatusAccepted, gin.H{
			"log_id": logEntry.ID,
			"status": "accepted",
		})
	default:
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Log buffer full, please try again later",
		})
	}
}

// Background workers
func (s *LoggingService) startLogProcessor() {
	batch := make([]*LogEntry, 0, s.config.BatchSize)
	ticker := time.NewTicker(s.config.FlushInterval)
	defer ticker.Stop()

	for {
		select {
		case logEntry := <-s.logBuffer:
			batch = append(batch, logEntry)
			if len(batch) >= s.config.BatchSize {
				s.processBatch(batch)
				batch = batch[:0]
			}

		case <-ticker.C:
			if len(batch) > 0 {
				s.processBatch(batch)
				batch = batch[:0]
			}
		}
	}
}

func (s *LoggingService) processBatch(batch []*LogEntry) {
	start := time.Now()

	// Store in database
	if err := s.db.CreateInBatches(batch, len(batch)).Error; err != nil {
		log.Printf("Error storing logs in database: %v", err)
	}

	// Index in Elasticsearch
	if err := s.indexLogsInElasticsearch(batch); err != nil {
		log.Printf("Error indexing logs in Elasticsearch: %v", err)
	}

	// Update metrics
	duration := time.Since(start).Seconds()
	logProcessingDuration.WithLabelValues("batch").Observe(duration)
	logBufferSize.Set(float64(len(s.logBuffer)))
}

func (s *LoggingService) indexLogsInElasticsearch(logs []*LogEntry) error {
	// Implementation would index logs in Elasticsearch
	// This is a placeholder
	return nil
}

// Utility functions
func getString(data map[string]interface{}, key, defaultValue string) string {
	if value, ok := data[key].(string); ok {
		return value
	}
	return defaultValue
}

func getMap(data map[string]interface{}, key string) map[string]interface{} {
	if value, ok := data[key].(map[string]interface{}); ok {
		return value
	}
	return make(map[string]interface{})
}

func getStringSlice(data map[string]interface{}, key string) []string {
	if value, ok := data[key].([]interface{}); ok {
		result := make([]string, len(value))
		for i, v := range value {
			if str, ok := v.(string); ok {
				result[i] = str
			}
		}
		return result
	}
	return []string{}
}

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

func parseInt64(s string) int64 {
	if i, err := strconv.ParseInt(s, 10, 64); err == nil {
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
