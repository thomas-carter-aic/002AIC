/**
 * Audit Service - Go
 * Compliance and security tracking for the 002AIC platform
 * Handles audit logging, compliance reporting, and security event tracking
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
	"github.com/streadway/amqp"
)

// Configuration
type Config struct {
	Port        string
	DatabaseURL string
	RedisURL    string
	RabbitMQURL string
	Environment string
}

// Audit Event Types
const (
	EventTypeUserAction     = "user_action"
	EventTypeSystemAction   = "system_action"
	EventTypeSecurityEvent  = "security_event"
	EventTypeDataAccess     = "data_access"
	EventTypeConfigChange   = "config_change"
	EventTypeAPICall        = "api_call"
	EventTypeAuthentication = "authentication"
	EventTypeAuthorization  = "authorization"
)

// Risk Levels
const (
	RiskLevelLow      = "low"
	RiskLevelMedium   = "medium"
	RiskLevelHigh     = "high"
	RiskLevelCritical = "critical"
)

// Compliance Standards
const (
	ComplianceSOX     = "sox"
	ComplianceGDPR    = "gdpr"
	ComplianceHIPAA   = "hipaa"
	ComplianceSOC2    = "soc2"
	CompliancePCIDSS  = "pci_dss"
	ComplianceISO27001 = "iso_27001"
)

// Models
type AuditEvent struct {
	ID               string                 `json:"id" gorm:"primaryKey"`
	Timestamp        time.Time              `json:"timestamp" gorm:"index"`
	EventType        string                 `json:"event_type" gorm:"index"`
	Action           string                 `json:"action"`
	Resource         string                 `json:"resource"`
	ResourceID       string                 `json:"resource_id"`
	UserID           string                 `json:"user_id" gorm:"index"`
	SessionID        string                 `json:"session_id"`
	IPAddress        string                 `json:"ip_address"`
	UserAgent        string                 `json:"user_agent"`
	RiskLevel        string                 `json:"risk_level" gorm:"index"`
	ComplianceFlags  []string               `json:"compliance_flags" gorm:"type:text[]"`
	Metadata         map[string]interface{} `json:"metadata" gorm:"type:jsonb"`
	Success          bool                   `json:"success"`
	ErrorMessage     string                 `json:"error_message,omitempty"`
	Duration         int64                  `json:"duration_ms"`
	ServiceName      string                 `json:"service_name"`
	ServiceVersion   string                 `json:"service_version"`
	TraceID          string                 `json:"trace_id"`
	SpanID           string                 `json:"span_id"`
	CreatedAt        time.Time              `json:"created_at"`
	UpdatedAt        time.Time              `json:"updated_at"`
}

type ComplianceReport struct {
	ID               string                 `json:"id" gorm:"primaryKey"`
	Standard         string                 `json:"standard"`
	ReportType       string                 `json:"report_type"`
	Period           string                 `json:"period"`
	StartDate        time.Time              `json:"start_date"`
	EndDate          time.Time              `json:"end_date"`
	Status           string                 `json:"status"`
	TotalEvents      int64                  `json:"total_events"`
	ComplianceScore  float64                `json:"compliance_score"`
	Violations       int64                  `json:"violations"`
	Recommendations  []string               `json:"recommendations" gorm:"type:text[]"`
	GeneratedBy      string                 `json:"generated_by"`
	GeneratedAt      time.Time              `json:"generated_at"`
	Data             map[string]interface{} `json:"data" gorm:"type:jsonb"`
	CreatedAt        time.Time              `json:"created_at"`
	UpdatedAt        time.Time              `json:"updated_at"`
}

type SecurityAlert struct {
	ID          string                 `json:"id" gorm:"primaryKey"`
	AlertType   string                 `json:"alert_type"`
	Severity    string                 `json:"severity" gorm:"index"`
	Title       string                 `json:"title"`
	Description string                 `json:"description"`
	EventIDs    []string               `json:"event_ids" gorm:"type:text[]"`
	UserID      string                 `json:"user_id"`
	IPAddress   string                 `json:"ip_address"`
	Status      string                 `json:"status"`
	AssignedTo  string                 `json:"assigned_to"`
	ResolvedAt  *time.Time             `json:"resolved_at"`
	Resolution  string                 `json:"resolution"`
	Metadata    map[string]interface{} `json:"metadata" gorm:"type:jsonb"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

// Service struct
type AuditService struct {
	db          *gorm.DB
	redis       *redis.Client
	rabbitmq    *amqp.Connection
	config      *Config
	router      *gin.Engine
	httpServer  *http.Server
}

// Prometheus metrics
var (
	auditEventsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "audit_events_total",
			Help: "Total number of audit events processed",
		},
		[]string{"event_type", "risk_level", "success"},
	)

	complianceScore = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "compliance_score",
			Help: "Current compliance score by standard",
		},
		[]string{"standard"},
	)

	securityAlertsActive = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "security_alerts_active",
			Help: "Number of active security alerts",
		},
		[]string{"severity"},
	)

	auditProcessingDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "audit_processing_duration_seconds",
			Help: "Time taken to process audit events",
		},
		[]string{"event_type"},
	)
)

func init() {
	prometheus.MustRegister(auditEventsTotal)
	prometheus.MustRegister(complianceScore)
	prometheus.MustRegister(securityAlertsActive)
	prometheus.MustRegister(auditProcessingDuration)
}

func main() {
	config := &Config{
		Port:        getEnv("PORT", "8080"),
		DatabaseURL: getEnv("DATABASE_URL", "postgres://postgres:password@localhost:5432/audit?sslmode=disable"),
		RedisURL:    getEnv("REDIS_URL", "redis://localhost:6379"),
		RabbitMQURL: getEnv("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/"),
		Environment: getEnv("ENVIRONMENT", "development"),
	}

	service, err := NewAuditService(config)
	if err != nil {
		log.Fatal("Failed to create audit service:", err)
	}

	// Start the service
	if err := service.Start(); err != nil {
		log.Fatal("Failed to start audit service:", err)
	}
}

func NewAuditService(config *Config) (*AuditService, error) {
	// Initialize database
	db, err := gorm.Open(postgres.Open(config.DatabaseURL), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Auto-migrate tables
	if err := db.AutoMigrate(&AuditEvent{}, &ComplianceReport{}, &SecurityAlert{}); err != nil {
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

	// Initialize RabbitMQ
	rabbitmqConn, err := amqp.Dial(config.RabbitMQURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	service := &AuditService{
		db:       db,
		redis:    redisClient,
		rabbitmq: rabbitmqConn,
		config:   config,
	}

	service.setupRoutes()
	return service, nil
}

func (s *AuditService) setupRoutes() {
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
		// Audit events
		v1.POST("/audit/events", s.createAuditEvent)
		v1.GET("/audit/events", s.getAuditEvents)
		v1.GET("/audit/events/:id", s.getAuditEvent)
		v1.POST("/audit/events/batch", s.createBatchAuditEvents)

		// Compliance reports
		v1.POST("/audit/compliance/reports", s.generateComplianceReport)
		v1.GET("/audit/compliance/reports", s.getComplianceReports)
		v1.GET("/audit/compliance/reports/:id", s.getComplianceReport)
		v1.GET("/audit/compliance/standards/:standard/score", s.getComplianceScore)

		// Security alerts
		v1.GET("/audit/security/alerts", s.getSecurityAlerts)
		v1.GET("/audit/security/alerts/:id", s.getSecurityAlert)
		v1.PUT("/audit/security/alerts/:id", s.updateSecurityAlert)
		v1.POST("/audit/security/alerts/:id/resolve", s.resolveSecurityAlert)

		// Analytics
		v1.GET("/audit/analytics/dashboard", s.getAnalyticsDashboard)
		v1.GET("/audit/analytics/trends", s.getAuditTrends)
		v1.GET("/audit/analytics/risk-assessment", s.getRiskAssessment)
	}
}

func (s *AuditService) Start() error {
	// Start background workers
	go s.startEventProcessor()
	go s.startSecurityMonitor()
	go s.startComplianceMonitor()

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

		log.Println("Shutting down audit service...")
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := s.httpServer.Shutdown(ctx); err != nil {
			log.Printf("Server shutdown error: %v", err)
		}

		s.cleanup()
	}()

	log.Printf("🚀 Audit Service starting on port %s", s.config.Port)
	log.Printf("📊 Health check: http://localhost:%s/health", s.config.Port)
	log.Printf("📈 Metrics: http://localhost:%s/metrics", s.config.Port)

	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("failed to start HTTP server: %w", err)
	}

	return nil
}

func (s *AuditService) cleanup() {
	if s.redis != nil {
		s.redis.Close()
	}
	if s.rabbitmq != nil {
		s.rabbitmq.Close()
	}
	if s.db != nil {
		sqlDB, _ := s.db.DB()
		if sqlDB != nil {
			sqlDB.Close()
		}
	}
}

// Health check endpoint
func (s *AuditService) healthCheck(c *gin.Context) {
	status := gin.H{
		"status":    "healthy",
		"service":   "audit-service",
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

	c.JSON(http.StatusOK, status)
}

// Utility functions
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
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
