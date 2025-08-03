/**
 * Metrics Service - Go
 * Performance metrics and analytics collection for the 002AIC platform
 * Handles custom metrics, dashboards, alerting, and performance analytics
 */

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"os/signal"
	"sort"
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
	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

// Configuration
type Config struct {
	Port           string
	DatabaseURL    string
	RedisURL       string
	PrometheusURL  string
	Environment    string
	RetentionDays  int
	SampleInterval time.Duration
	AlertThreshold float64
}

// Metric types
const (
	MetricTypeCounter   = "counter"
	MetricTypeGauge     = "gauge"
	MetricTypeHistogram = "histogram"
	MetricTypeSummary   = "summary"
)

// Aggregation types
const (
	AggregationSum   = "sum"
	AggregationAvg   = "avg"
	AggregationMin   = "min"
	AggregationMax   = "max"
	AggregationCount = "count"
	AggregationRate  = "rate"
)

// Models
type CustomMetric struct {
	ID          string                 `json:"id" gorm:"primaryKey"`
	Name        string                 `json:"name" gorm:"uniqueIndex;not null"`
	Type        string                 `json:"type" gorm:"not null"`
	Description string                 `json:"description"`
	Labels      []string               `json:"labels" gorm:"type:text[]"`
	Unit        string                 `json:"unit"`
	Config      map[string]interface{} `json:"config" gorm:"type:jsonb"`
	IsActive    bool                   `json:"is_active" gorm:"default:true"`
	CreatedBy   string                 `json:"created_by"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

type MetricData struct {
	ID         string                 `json:"id" gorm:"primaryKey"`
	MetricName string                 `json:"metric_name" gorm:"index"`
	Value      float64                `json:"value"`
	Labels     map[string]interface{} `json:"labels" gorm:"type:jsonb"`
	Timestamp  time.Time              `json:"timestamp" gorm:"index"`
	CreatedAt  time.Time              `json:"created_at"`
}

type Dashboard struct {
	ID          string                 `json:"id" gorm:"primaryKey"`
	Name        string                 `json:"name" gorm:"not null"`
	Description string                 `json:"description"`
	Config      map[string]interface{} `json:"config" gorm:"type:jsonb"`
	Widgets     []DashboardWidget      `json:"widgets" gorm:"foreignKey:DashboardID"`
	IsPublic    bool                   `json:"is_public" gorm:"default:false"`
	CreatedBy   string                 `json:"created_by"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

type DashboardWidget struct {
	ID          string                 `json:"id" gorm:"primaryKey"`
	DashboardID string                 `json:"dashboard_id" gorm:"index"`
	Name        string                 `json:"name" gorm:"not null"`
	Type        string                 `json:"type" gorm:"not null"`
	Query       string                 `json:"query"`
	Config      map[string]interface{} `json:"config" gorm:"type:jsonb"`
	Position    map[string]interface{} `json:"position" gorm:"type:jsonb"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

type Alert struct {
	ID          string                 `json:"id" gorm:"primaryKey"`
	Name        string                 `json:"name" gorm:"not null"`
	Query       string                 `json:"query" gorm:"not null"`
	Condition   string                 `json:"condition" gorm:"not null"`
	Threshold   float64                `json:"threshold"`
	Severity    string                 `json:"severity"`
	IsActive    bool                   `json:"is_active" gorm:"default:true"`
	Config      map[string]interface{} `json:"config" gorm:"type:jsonb"`
	LastFired   *time.Time             `json:"last_fired"`
	CreatedBy   string                 `json:"created_by"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

// Service struct
type MetricsService struct {
	db             *gorm.DB
	redis          *redis.Client
	prometheusAPI  v1.API
	config         *Config
	router         *gin.Engine
	httpServer     *http.Server
	customMetrics  map[string]*prometheus.MetricVec
}

// Prometheus metrics for the service itself
var (
	customMetricsTotal = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "custom_metrics_total",
			Help: "Total number of custom metrics",
		},
	)

	metricIngestionRate = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "metric_ingestion_total",
			Help: "Total metrics ingested",
		},
		[]string{"metric_name", "status"},
	)

	queryExecutionDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "metric_query_duration_seconds",
			Help: "Time taken to execute metric queries",
		},
		[]string{"query_type"},
	)

	activeDashboards = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "dashboards_active_total",
			Help: "Total number of active dashboards",
		},
	)

	activeAlerts = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "alerts_active_total",
			Help: "Total number of active alerts",
		},
	)
)

func init() {
	prometheus.MustRegister(customMetricsTotal)
	prometheus.MustRegister(metricIngestionRate)
	prometheus.MustRegister(queryExecutionDuration)
	prometheus.MustRegister(activeDashboards)
	prometheus.MustRegister(activeAlerts)
}

func main() {
	config := &Config{
		Port:           getEnv("PORT", "8080"),
		DatabaseURL:    getEnv("DATABASE_URL", "postgres://postgres:password@localhost:5432/metrics?sslmode=disable"),
		RedisURL:       getEnv("REDIS_URL", "redis://localhost:6379"),
		PrometheusURL:  getEnv("PROMETHEUS_URL", "http://localhost:9090"),
		Environment:    getEnv("ENVIRONMENT", "development"),
		RetentionDays:  parseInt(getEnv("RETENTION_DAYS", "30")),
		SampleInterval: time.Duration(parseInt(getEnv("SAMPLE_INTERVAL", "15"))) * time.Second,
		AlertThreshold: parseFloat(getEnv("ALERT_THRESHOLD", "0.8")),
	}

	service, err := NewMetricsService(config)
	if err != nil {
		log.Fatal("Failed to create metrics service:", err)
	}

	if err := service.Start(); err != nil {
		log.Fatal("Failed to start metrics service:", err)
	}
}

func NewMetricsService(config *Config) (*MetricsService, error) {
	// Initialize database
	db, err := gorm.Open(postgres.Open(config.DatabaseURL), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Auto-migrate tables
	if err := db.AutoMigrate(&CustomMetric{}, &MetricData{}, &Dashboard{}, &DashboardWidget{}, &Alert{}); err != nil {
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

	// Initialize Prometheus API client
	client, err := api.NewClient(api.Config{
		Address: config.PrometheusURL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Prometheus client: %w", err)
	}
	prometheusAPI := v1.NewAPI(client)

	service := &MetricsService{
		db:            db,
		redis:         redisClient,
		prometheusAPI: prometheusAPI,
		config:        config,
		customMetrics: make(map[string]*prometheus.MetricVec),
	}

	service.setupRoutes()
	return service, nil
}

func (s *MetricsService) setupRoutes() {
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
		// Custom metrics
		v1.POST("/metrics/custom", s.createCustomMetric)
		v1.GET("/metrics/custom", s.listCustomMetrics)
		v1.GET("/metrics/custom/:name", s.getCustomMetric)
		v1.PUT("/metrics/custom/:name", s.updateCustomMetric)
		v1.DELETE("/metrics/custom/:name", s.deleteCustomMetric)

		// Metric data ingestion
		v1.POST("/metrics/data", s.ingestMetricData)
		v1.POST("/metrics/data/batch", s.ingestBatchMetricData)

		// Metric queries
		v1.GET("/metrics/query", s.queryMetrics)
		v1.POST("/metrics/query", s.queryMetricsAdvanced)
		v1.GET("/metrics/range", s.queryMetricsRange)

		// Dashboards
		v1.POST("/dashboards", s.createDashboard)
		v1.GET("/dashboards", s.listDashboards)
		v1.GET("/dashboards/:id", s.getDashboard)
		v1.PUT("/dashboards/:id", s.updateDashboard)
		v1.DELETE("/dashboards/:id", s.deleteDashboard)

		// Dashboard widgets
		v1.POST("/dashboards/:id/widgets", s.createWidget)
		v1.PUT("/dashboards/:id/widgets/:widget_id", s.updateWidget)
		v1.DELETE("/dashboards/:id/widgets/:widget_id", s.deleteWidget)

		// Alerts
		v1.POST("/alerts", s.createAlert)
		v1.GET("/alerts", s.listAlerts)
		v1.GET("/alerts/:id", s.getAlert)
		v1.PUT("/alerts/:id", s.updateAlert)
		v1.DELETE("/alerts/:id", s.deleteAlert)
		v1.POST("/alerts/:id/test", s.testAlert)

		// Analytics
		v1.GET("/analytics/summary", s.getMetricsSummary)
		v1.GET("/analytics/trends", s.getMetricsTrends)
		v1.GET("/analytics/performance", s.getPerformanceMetrics)
	}
}

func (s *MetricsService) Start() error {
	// Load existing custom metrics
	if err := s.loadCustomMetrics(); err != nil {
		return fmt.Errorf("failed to load custom metrics: %w", err)
	}

	// Start background workers
	go s.startMetricsSampler()
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

		log.Println("Shutting down metrics service...")
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := s.httpServer.Shutdown(ctx); err != nil {
			log.Printf("Server shutdown error: %v", err)
		}

		s.cleanup()
	}()

	log.Printf("🚀 Metrics Service starting on port %s", s.config.Port)
	log.Printf("📊 Health check: http://localhost:%s/health", s.config.Port)
	log.Printf("📈 Metrics: http://localhost:%s/metrics", s.config.Port)
	log.Printf("🔍 Prometheus: %s", s.config.PrometheusURL)

	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("failed to start HTTP server: %w", err)
	}

	return nil
}

func (s *MetricsService) cleanup() {
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
func (s *MetricsService) healthCheck(c *gin.Context) {
	status := gin.H{
		"status":    "healthy",
		"service":   "metrics-service",
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

	// Check Prometheus connection
	ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, err := s.prometheusAPI.Config(ctx); err != nil {
		status["prometheus"] = "disconnected"
	} else {
		status["prometheus"] = "connected"
	}

	status["custom_metrics"] = len(s.customMetrics)

	c.JSON(http.StatusOK, status)
}

// Custom metrics management
func (s *MetricsService) createCustomMetric(c *gin.Context) {
	var metric CustomMetric
	if err := c.ShouldBindJSON(&metric); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Set ID and timestamps
	metric.ID = uuid.New().String()
	metric.CreatedAt = time.Now().UTC()
	metric.UpdatedAt = time.Now().UTC()

	// Create in database
	if err := s.db.Create(&metric).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create custom metric"})
		return
	}

	// Register Prometheus metric
	if err := s.registerPrometheusMetric(&metric); err != nil {
		log.Printf("Failed to register Prometheus metric: %v", err)
	}

	// Update metrics
	customMetricsTotal.Inc()

	c.JSON(http.StatusCreated, gin.H{
		"metric_id": metric.ID,
		"message":   "Custom metric created successfully",
	})
}

// Metric data ingestion
func (s *MetricsService) ingestMetricData(c *gin.Context) {
	var data struct {
		MetricName string                 `json:"metric_name" binding:"required"`
		Value      float64                `json:"value" binding:"required"`
		Labels     map[string]interface{} `json:"labels"`
		Timestamp  *time.Time             `json:"timestamp"`
	}

	if err := c.ShouldBindJSON(&data); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Set timestamp if not provided
	if data.Timestamp == nil {
		now := time.Now().UTC()
		data.Timestamp = &now
	}

	// Create metric data entry
	metricData := &MetricData{
		ID:         uuid.New().String(),
		MetricName: data.MetricName,
		Value:      data.Value,
		Labels:     data.Labels,
		Timestamp:  *data.Timestamp,
		CreatedAt:  time.Now().UTC(),
	}

	// Store in database
	if err := s.db.Create(metricData).Error; err != nil {
		metricIngestionRate.WithLabelValues(data.MetricName, "error").Inc()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to store metric data"})
		return
	}

	// Update custom Prometheus metric if exists
	if promMetric, exists := s.customMetrics[data.MetricName]; exists {
		s.updatePrometheusMetric(promMetric, data.Value, data.Labels)
	}

	// Update metrics
	metricIngestionRate.WithLabelValues(data.MetricName, "success").Inc()

	c.JSON(http.StatusCreated, gin.H{
		"metric_id": metricData.ID,
		"message":   "Metric data ingested successfully",
	})
}

// Metric queries
func (s *MetricsService) queryMetrics(c *gin.Context) {
	query := c.Query("query")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Query parameter is required"})
		return
	}

	start := time.Now()

	// Execute Prometheus query
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, warnings, err := s.prometheusAPI.Query(ctx, query, time.Now())
	if err != nil {
		queryExecutionDuration.WithLabelValues("instant").Observe(time.Since(start).Seconds())
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Update metrics
	queryExecutionDuration.WithLabelValues("instant").Observe(time.Since(start).Seconds())

	c.JSON(http.StatusOK, gin.H{
		"query":     query,
		"result":    result,
		"warnings":  warnings,
		"timestamp": time.Now().UTC(),
	})
}

// Background workers
func (s *MetricsService) startMetricsSampler() {
	ticker := time.NewTicker(s.config.SampleInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.sampleSystemMetrics()
		}
	}
}

func (s *MetricsService) sampleSystemMetrics() {
	// Sample system metrics and store them
	// This is a simplified implementation
	metrics := map[string]float64{
		"system_cpu_usage":    s.getCPUUsage(),
		"system_memory_usage": s.getMemoryUsage(),
		"system_disk_usage":   s.getDiskUsage(),
	}

	for name, value := range metrics {
		metricData := &MetricData{
			ID:         uuid.New().String(),
			MetricName: name,
			Value:      value,
			Labels:     map[string]interface{}{"source": "system"},
			Timestamp:  time.Now().UTC(),
			CreatedAt:  time.Now().UTC(),
		}

		if err := s.db.Create(metricData).Error; err != nil {
			log.Printf("Failed to store system metric %s: %v", name, err)
		}
	}
}

// Helper functions for system metrics (simplified)
func (s *MetricsService) getCPUUsage() float64 {
	// In a real implementation, this would read from /proc/stat or similar
	return 45.5 // Placeholder
}

func (s *MetricsService) getMemoryUsage() float64 {
	// In a real implementation, this would read from /proc/meminfo or similar
	return 67.8 // Placeholder
}

func (s *MetricsService) getDiskUsage() float64 {
	// In a real implementation, this would use syscalls to get disk usage
	return 23.4 // Placeholder
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

func parseFloat(s string) float64 {
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return f
	}
	return 0.0
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
