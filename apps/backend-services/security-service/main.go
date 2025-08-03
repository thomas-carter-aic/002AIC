/**
 * Security Service - Go
 * Advanced security management for the 002AIC platform
 * Handles threat detection, vulnerability scanning, security policies, and incident response
 */

package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"regexp"
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
	"golang.org/x/crypto/bcrypt"
	"github.com/golang-jwt/jwt/v4"
)

// Configuration
type Config struct {
	Port                    string
	DatabaseURL             string
	RedisURL                string
	JWTSecret               string
	Environment             string
	ThreatDetectionEnabled  bool
	VulnerabilityScanEnabled bool
	MaxLoginAttempts        int
	LockoutDuration         time.Duration
	PasswordMinLength       int
	PasswordComplexity      bool
}

// Security event types
const (
	EventTypeLogin              = "login"
	EventTypeLogout             = "logout"
	EventTypeFailedLogin        = "failed_login"
	EventTypePasswordChange     = "password_change"
	EventTypePermissionDenied   = "permission_denied"
	EventTypeSuspiciousActivity = "suspicious_activity"
	EventTypeDataAccess         = "data_access"
	EventTypeSecurityViolation  = "security_violation"
	EventTypeThreatDetected     = "threat_detected"
	EventTypeVulnerabilityFound = "vulnerability_found"
)

// Threat levels
const (
	ThreatLevelLow      = "low"
	ThreatLevelMedium   = "medium"
	ThreatLevelHigh     = "high"
	ThreatLevelCritical = "critical"
)

// Security policy types
const (
	PolicyTypePassword    = "password"
	PolicyTypeAccess      = "access"
	PolicyTypeEncryption  = "encryption"
	PolicyTypeAudit       = "audit"
	PolicyTypeCompliance  = "compliance"
)

// Models
type SecurityEvent struct {
	ID          string                 `json:"id" gorm:"primaryKey"`
	Type        string                 `json:"type" gorm:"index;not null"`
	Severity    string                 `json:"severity" gorm:"index"`
	UserID      string                 `json:"user_id" gorm:"index"`
	IPAddress   string                 `json:"ip_address" gorm:"index"`
	UserAgent   string                 `json:"user_agent"`
	Resource    string                 `json:"resource"`
	Action      string                 `json:"action"`
	Result      string                 `json:"result"`
	Details     map[string]interface{} `json:"details" gorm:"type:jsonb"`
	Metadata    map[string]interface{} `json:"metadata" gorm:"type:jsonb"`
	Timestamp   time.Time              `json:"timestamp" gorm:"index"`
	ProcessedAt *time.Time             `json:"processed_at"`
	CreatedAt   time.Time              `json:"created_at"`
}

type ThreatDetection struct {
	ID            string                 `json:"id" gorm:"primaryKey"`
	Type          string                 `json:"type" gorm:"index"`
	ThreatLevel   string                 `json:"threat_level" gorm:"index"`
	Source        string                 `json:"source"`
	Target        string                 `json:"target"`
	Description   string                 `json:"description"`
	Indicators    []string               `json:"indicators" gorm:"type:text[]"`
	Evidence      map[string]interface{} `json:"evidence" gorm:"type:jsonb"`
	Status        string                 `json:"status" gorm:"index"`
	AssignedTo    string                 `json:"assigned_to"`
	ResolvedAt    *time.Time             `json:"resolved_at"`
	Resolution    string                 `json:"resolution"`
	CreatedAt     time.Time              `json:"created_at"`
	UpdatedAt     time.Time              `json:"updated_at"`
}

type SecurityPolicy struct {
	ID          string                 `json:"id" gorm:"primaryKey"`
	Name        string                 `json:"name" gorm:"uniqueIndex;not null"`
	Type        string                 `json:"type" gorm:"index"`
	Description string                 `json:"description"`
	Rules       map[string]interface{} `json:"rules" gorm:"type:jsonb"`
	IsActive    bool                   `json:"is_active" gorm:"default:true"`
	Priority    int                    `json:"priority" gorm:"default:0"`
	CreatedBy   string                 `json:"created_by"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

type VulnerabilityReport struct {
	ID            string                 `json:"id" gorm:"primaryKey"`
	Title         string                 `json:"title" gorm:"not null"`
	Description   string                 `json:"description"`
	Severity      string                 `json:"severity" gorm:"index"`
	CVEId         string                 `json:"cve_id" gorm:"index"`
	Component     string                 `json:"component"`
	Version       string                 `json:"version"`
	FixedVersion  string                 `json:"fixed_version"`
	Status        string                 `json:"status" gorm:"index"`
	ReportedBy    string                 `json:"reported_by"`
	AssignedTo    string                 `json:"assigned_to"`
	ResolvedAt    *time.Time             `json:"resolved_at"`
	Details       map[string]interface{} `json:"details" gorm:"type:jsonb"`
	CreatedAt     time.Time              `json:"created_at"`
	UpdatedAt     time.Time              `json:"updated_at"`
}

type SecurityIncident struct {
	ID          string                 `json:"id" gorm:"primaryKey"`
	Title       string                 `json:"title" gorm:"not null"`
	Description string                 `json:"description"`
	Severity    string                 `json:"severity" gorm:"index"`
	Status      string                 `json:"status" gorm:"index"`
	Category    string                 `json:"category"`
	Reporter    string                 `json:"reporter"`
	AssignedTo  string                 `json:"assigned_to"`
	Timeline    []map[string]interface{} `json:"timeline" gorm:"type:jsonb"`
	Evidence    map[string]interface{} `json:"evidence" gorm:"type:jsonb"`
	Impact      string                 `json:"impact"`
	Resolution  string                 `json:"resolution"`
	ResolvedAt  *time.Time             `json:"resolved_at"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

// Service struct
type SecurityService struct {
	db         *gorm.DB
	redis      *redis.Client
	config     *Config
	router     *gin.Engine
	httpServer *http.Server
}

// Prometheus metrics
var (
	securityEventsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "security_events_total",
			Help: "Total number of security events",
		},
		[]string{"type", "severity"},
	)

	threatsDetected = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "threats_detected_total",
			Help: "Total number of threats detected",
		},
		[]string{"type", "threat_level"},
	)

	vulnerabilitiesFound = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "vulnerabilities_found_total",
			Help: "Total number of vulnerabilities found",
		},
		[]string{"severity", "component"},
	)

	securityIncidents = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "security_incidents_active",
			Help: "Number of active security incidents",
		},
		[]string{"severity", "status"},
	)

	failedLoginAttempts = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "failed_login_attempts_total",
			Help: "Total number of failed login attempts",
		},
		[]string{"user_id", "ip_address"},
	)

	securityPolicies = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "security_policies_active",
			Help: "Number of active security policies",
		},
	)
)

func init() {
	prometheus.MustRegister(securityEventsTotal)
	prometheus.MustRegister(threatsDetected)
	prometheus.MustRegister(vulnerabilitiesFound)
	prometheus.MustRegister(securityIncidents)
	prometheus.MustRegister(failedLoginAttempts)
	prometheus.MustRegister(securityPolicies)
}

func main() {
	config := &Config{
		Port:                     getEnv("PORT", "8080"),
		DatabaseURL:              getEnv("DATABASE_URL", "postgres://postgres:password@localhost:5432/security?sslmode=disable"),
		RedisURL:                 getEnv("REDIS_URL", "redis://localhost:6379"),
		JWTSecret:                getEnv("JWT_SECRET", "your-secret-key"),
		Environment:              getEnv("ENVIRONMENT", "development"),
		ThreatDetectionEnabled:   getBool(getEnv("THREAT_DETECTION_ENABLED", "true")),
		VulnerabilityScanEnabled: getBool(getEnv("VULNERABILITY_SCAN_ENABLED", "true")),
		MaxLoginAttempts:         parseInt(getEnv("MAX_LOGIN_ATTEMPTS", "5")),
		LockoutDuration:          time.Duration(parseInt(getEnv("LOCKOUT_DURATION", "300"))) * time.Second,
		PasswordMinLength:        parseInt(getEnv("PASSWORD_MIN_LENGTH", "8")),
		PasswordComplexity:       getBool(getEnv("PASSWORD_COMPLEXITY", "true")),
	}

	service, err := NewSecurityService(config)
	if err != nil {
		log.Fatal("Failed to create security service:", err)
	}

	if err := service.Start(); err != nil {
		log.Fatal("Failed to start security service:", err)
	}
}

func NewSecurityService(config *Config) (*SecurityService, error) {
	// Initialize database
	db, err := gorm.Open(postgres.Open(config.DatabaseURL), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Auto-migrate tables
	if err := db.AutoMigrate(
		&SecurityEvent{},
		&ThreatDetection{},
		&SecurityPolicy{},
		&VulnerabilityReport{},
		&SecurityIncident{},
	); err != nil {
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

	service := &SecurityService{
		db:     db,
		redis:  redisClient,
		config: config,
	}

	service.setupRoutes()
	return service, nil
}

func (s *SecurityService) setupRoutes() {
	if s.config.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	s.router = gin.Default()

	// Middleware
	s.router.Use(gin.Recovery())
	s.router.Use(corsMiddleware())
	s.router.Use(loggingMiddleware())
	s.router.Use(s.securityMiddleware())

	// Health check
	s.router.GET("/health", s.healthCheck)
	s.router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// API routes
	v1 := s.router.Group("/v1")
	{
		// Security events
		v1.POST("/events", s.logSecurityEvent)
		v1.GET("/events", s.listSecurityEvents)
		v1.GET("/events/:id", s.getSecurityEvent)

		// Threat detection
		v1.POST("/threats", s.reportThreat)
		v1.GET("/threats", s.listThreats)
		v1.GET("/threats/:id", s.getThreat)
		v1.PUT("/threats/:id", s.updateThreat)
		v1.POST("/threats/:id/resolve", s.resolveThreat)

		// Security policies
		v1.POST("/policies", s.createSecurityPolicy)
		v1.GET("/policies", s.listSecurityPolicies)
		v1.GET("/policies/:id", s.getSecurityPolicy)
		v1.PUT("/policies/:id", s.updateSecurityPolicy)
		v1.DELETE("/policies/:id", s.deleteSecurityPolicy)

		// Vulnerability management
		v1.POST("/vulnerabilities", s.reportVulnerability)
		v1.GET("/vulnerabilities", s.listVulnerabilities)
		v1.GET("/vulnerabilities/:id", s.getVulnerability)
		v1.PUT("/vulnerabilities/:id", s.updateVulnerability)
		v1.POST("/vulnerabilities/scan", s.triggerVulnerabilityScan)

		// Incident management
		v1.POST("/incidents", s.createSecurityIncident)
		v1.GET("/incidents", s.listSecurityIncidents)
		v1.GET("/incidents/:id", s.getSecurityIncident)
		v1.PUT("/incidents/:id", s.updateSecurityIncident)
		v1.POST("/incidents/:id/resolve", s.resolveSecurityIncident)

		// Security validation
		v1.POST("/validate/password", s.validatePassword)
		v1.POST("/validate/access", s.validateAccess)
		v1.POST("/validate/token", s.validateToken)

		// Security analytics
		v1.GET("/analytics/events", s.getSecurityAnalytics)
		v1.GET("/analytics/threats", s.getThreatAnalytics)
		v1.GET("/analytics/vulnerabilities", s.getVulnerabilityAnalytics)
	}
}

func (s *SecurityService) Start() error {
	// Initialize default security policies
	if err := s.initializeDefaultPolicies(); err != nil {
		return fmt.Errorf("failed to initialize default policies: %w", err)
	}

	// Start background workers
	go s.startThreatDetectionWorker()
	go s.startVulnerabilityScanWorker()
	go s.startSecurityEventProcessor()
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

		log.Println("Shutting down security service...")
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := s.httpServer.Shutdown(ctx); err != nil {
			log.Printf("Server shutdown error: %v", err)
		}

		s.cleanup()
	}()

	log.Printf("🚀 Security Service starting on port %s", s.config.Port)
	log.Printf("📊 Health check: http://localhost:%s/health", s.config.Port)
	log.Printf("📈 Metrics: http://localhost:%s/metrics", s.config.Port)
	log.Printf("🔒 Threat detection: %v", s.config.ThreatDetectionEnabled)
	log.Printf("🔍 Vulnerability scanning: %v", s.config.VulnerabilityScanEnabled)

	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("failed to start HTTP server: %w", err)
	}

	return nil
}

func (s *SecurityService) cleanup() {
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
func (s *SecurityService) healthCheck(c *gin.Context) {
	status := gin.H{
		"status":    "healthy",
		"service":   "security-service",
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

	// Add security status
	status["threat_detection"] = s.config.ThreatDetectionEnabled
	status["vulnerability_scanning"] = s.config.VulnerabilityScanEnabled

	c.JSON(http.StatusOK, status)
}

// Security event logging
func (s *SecurityService) logSecurityEvent(c *gin.Context) {
	var eventData struct {
		Type      string                 `json:"type" binding:"required"`
		Severity  string                 `json:"severity"`
		UserID    string                 `json:"user_id"`
		Resource  string                 `json:"resource"`
		Action    string                 `json:"action"`
		Result    string                 `json:"result"`
		Details   map[string]interface{} `json:"details"`
		Metadata  map[string]interface{} `json:"metadata"`
	}

	if err := c.ShouldBindJSON(&eventData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Create security event
	event := &SecurityEvent{
		ID:        uuid.New().String(),
		Type:      eventData.Type,
		Severity:  eventData.Severity,
		UserID:    eventData.UserID,
		IPAddress: c.ClientIP(),
		UserAgent: c.GetHeader("User-Agent"),
		Resource:  eventData.Resource,
		Action:    eventData.Action,
		Result:    eventData.Result,
		Details:   eventData.Details,
		Metadata:  eventData.Metadata,
		Timestamp: time.Now().UTC(),
		CreatedAt: time.Now().UTC(),
	}

	// Store event
	if err := s.db.Create(event).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to log security event"})
		return
	}

	// Update metrics
	securityEventsTotal.WithLabelValues(event.Type, event.Severity).Inc()

	// Process event for threat detection
	go s.processSecurityEvent(event)

	c.JSON(http.StatusCreated, gin.H{
		"event_id": event.ID,
		"message":  "Security event logged successfully",
	})
}

// Password validation
func (s *SecurityService) validatePassword(c *gin.Context) {
	var request struct {
		Password string `json:"password" binding:"required"`
		UserID   string `json:"user_id"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate password strength
	validation := s.validatePasswordStrength(request.Password)

	c.JSON(http.StatusOK, gin.H{
		"valid":        validation.Valid,
		"score":        validation.Score,
		"requirements": validation.Requirements,
		"suggestions":  validation.Suggestions,
	})
}

// Password strength validation
type PasswordValidation struct {
	Valid        bool              `json:"valid"`
	Score        int               `json:"score"`
	Requirements map[string]bool   `json:"requirements"`
	Suggestions  []string          `json:"suggestions"`
}

func (s *SecurityService) validatePasswordStrength(password string) PasswordValidation {
	validation := PasswordValidation{
		Requirements: make(map[string]bool),
		Suggestions:  []string{},
	}

	// Check minimum length
	validation.Requirements["min_length"] = len(password) >= s.config.PasswordMinLength
	if !validation.Requirements["min_length"] {
		validation.Suggestions = append(validation.Suggestions, 
			fmt.Sprintf("Password must be at least %d characters long", s.config.PasswordMinLength))
	}

	if s.config.PasswordComplexity {
		// Check for uppercase letters
		validation.Requirements["uppercase"] = regexp.MustCompile(`[A-Z]`).MatchString(password)
		if !validation.Requirements["uppercase"] {
			validation.Suggestions = append(validation.Suggestions, "Include at least one uppercase letter")
		}

		// Check for lowercase letters
		validation.Requirements["lowercase"] = regexp.MustCompile(`[a-z]`).MatchString(password)
		if !validation.Requirements["lowercase"] {
			validation.Suggestions = append(validation.Suggestions, "Include at least one lowercase letter")
		}

		// Check for numbers
		validation.Requirements["numbers"] = regexp.MustCompile(`[0-9]`).MatchString(password)
		if !validation.Requirements["numbers"] {
			validation.Suggestions = append(validation.Suggestions, "Include at least one number")
		}

		// Check for special characters
		validation.Requirements["special"] = regexp.MustCompile(`[!@#$%^&*(),.?":{}|<>]`).MatchString(password)
		if !validation.Requirements["special"] {
			validation.Suggestions = append(validation.Suggestions, "Include at least one special character")
		}
	}

	// Calculate score
	score := 0
	for _, met := range validation.Requirements {
		if met {
			score += 20
		}
	}

	// Bonus points for length
	if len(password) > 12 {
		score += 10
	}
	if len(password) > 16 {
		score += 10
	}

	validation.Score = score
	validation.Valid = score >= 80

	return validation
}

// Security middleware
func (s *SecurityService) securityMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Add security headers
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")

		// Rate limiting check
		if s.isRateLimited(c) {
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "Rate limit exceeded"})
			c.Abort()
			return
		}

		// IP blocking check
		if s.isIPBlocked(c.ClientIP()) {
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
			c.Abort()
			return
		}

		c.Next()
	}
}

// Check if IP is rate limited
func (s *SecurityService) isRateLimited(c *gin.Context) bool {
	// Simple rate limiting implementation
	key := fmt.Sprintf("rate_limit:%s", c.ClientIP())
	ctx := context.Background()

	count, err := s.redis.Incr(ctx, key).Result()
	if err != nil {
		return false
	}

	if count == 1 {
		s.redis.Expire(ctx, key, time.Minute)
	}

	return count > 100 // 100 requests per minute
}

// Check if IP is blocked
func (s *SecurityService) isIPBlocked(ip string) bool {
	ctx := context.Background()
	key := fmt.Sprintf("blocked_ip:%s", ip)
	
	exists, err := s.redis.Exists(ctx, key).Result()
	return err == nil && exists > 0
}

// Background workers
func (s *SecurityService) startThreatDetectionWorker() {
	if !s.config.ThreatDetectionEnabled {
		return
	}

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.detectThreats()
		}
	}
}

func (s *SecurityService) startVulnerabilityScanWorker() {
	if !s.config.VulnerabilityScanEnabled {
		return
	}

	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.scanVulnerabilities()
		}
	}
}

func (s *SecurityService) startSecurityEventProcessor() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.processUnprocessedEvents()
		}
	}
}

func (s *SecurityService) startMetricsUpdater() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.updateSecurityMetrics()
		}
	}
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

func getBool(s string) bool {
	return strings.ToLower(s) == "true"
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
